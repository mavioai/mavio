package server_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/types/known/durationpb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1/assistantv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/tools/fixtures"
)

// assistantRoutes are the answers of the services for the assistant
// tests, besides the series acquisition test's: TMDB finds The Matrix by
// any name, and no film by a film's file name.
func assistantRoutes(t *testing.T) map[string]string {
	routes := requestRoutes(t)
	routes["api.themoviedb.org/3/search/multi"] = `{"page": 1, "total_pages": 1, "total_results": 1, "results": [
		{"id": 603, "media_type": "movie", "title": "The Matrix", "release_date": "1999-03-30", "popularity": 90}]}`
	routes["api.themoviedb.org/3/search/movie"] = `{"page": 1, "total_pages": 0, "total_results": 0, "results": []}`
	return routes
}

// assistantClient is the assistant of an acquisition end-to-end test's
// server, as its administrator on the device e2e.
func (s *acquisitionServer) assistant() assistantv1connect.AssistantServiceClient {
	return assistantv1connect.NewAssistantServiceClient(http.DefaultClient, s.url, s.token)
}

// ask asks the assistant and returns the events of its answer.
func ask(t *testing.T, a assistantv1connect.AssistantServiceClient, req *assistantv1.AskRequest) []*assistantv1.AskResponse {
	t.Helper()
	st, err := a.Ask(t.Context(), req)
	if err != nil {
		t.Fatalf("Ask %q: %v", req.GetText(), err)
	}
	defer st.Close()
	var out []*assistantv1.AskResponse
	for st.Receive() {
		out = append(out, st.Msg())
	}
	if err := st.Err(); err != nil {
		t.Fatalf("Ask %q: %v", req.GetText(), err)
	}
	if len(out) == 0 || !out[len(out)-1].HasDone() {
		t.Fatalf("Ask %q answered %v, want it to end with done", req.GetText(), out)
	}
	return out
}

// done returns the end of an answer.
func done(events []*assistantv1.AskResponse) *assistantv1.Done {
	return events[len(events)-1].GetDone()
}

// actions returns the client actions of an answer.
func actions(events []*assistantv1.AskResponse) []*assistantv1.ClientAction {
	var out []*assistantv1.ClientAction
	for _, e := range events {
		if e.HasAction() {
			out = append(out, e.GetAction())
		}
	}
	return out
}

// understoodByAI tells whether an answer went to the AI path.
func understoodByAI(events []*assistantv1.AskResponse) bool {
	for _, e := range events {
		if e.HasUnderstood() && e.GetUnderstood().GetAi() {
			return true
		}
	}
	return false
}

// fakeASR is a whisper-asr-webservice hearing English and recognizing
// what it is told to, recording what it was asked.
type fakeASR struct {
	*httptest.Server

	mu    sync.Mutex
	text  string
	tasks []string
}

func newFakeASR(t *testing.T) *fakeASR {
	f := &fakeASR{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("audio_file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, _ = io.Copy(io.Discard, file)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/detect-language":
			_, _ = io.WriteString(w, `{"language_code":"en","detected_language":"english"}`)
		case "/asr":
			f.tasks = append(f.tasks, r.URL.Query().Get("task"))
			text, _ := json.Marshal(f.text)
			fmt.Fprintf(w, `{"text":%s,"language":"en","segments":[{"start":1,"end":2,"text":%s}]}`, text, text)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeASR) say(text string) {
	f.mu.Lock()
	f.text = text
	f.mu.Unlock()
}

// listen speaks a recording to the assistant's server recognizer, as
// 16 kHz PCM sent in pieces, and returns what it heard.
func (s *acquisitionServer) listen(wav string) string {
	t := s.t
	t.Helper()
	pcm := filepath.Join(t.TempDir(), "speech.pcm")
	run(t, s.ffmpeg, "-loglevel", "error", "-i", wav, "-f", "s16le", "-ac", "1", "-ar", "16000", pcm)
	audio, err := os.ReadFile(pcm)
	if err != nil {
		t.Fatal(err)
	}
	started, err := s.assistant().StartListening(t.Context(), assistantv1.StartListeningRequest_builder{Language: new("en")}.Build())
	if err != nil {
		t.Fatalf("StartListening: %v", err)
	}
	conn, err := websocket.Dial("ws"+strings.TrimPrefix(s.url, "http")+started.GetUrl(), "", s.url)
	if err != nil {
		t.Fatalf("dial %s: %v", started.GetUrl(), err)
	}
	defer conn.Close()
	// The server ends the utterance itself after a second of silence, and
	// stops reading.
	go func() {
		for chunk := range slices.Chunk(audio, 3200) {
			if websocket.Message.Send(conn, chunk) != nil {
				return
			}
		}
		_ = websocket.Message.Send(conn, "end")
	}()
	var heard struct {
		Text, Language, Error string
		Final                 bool
	}
	if err := websocket.JSON.Receive(conn, &heard); err != nil {
		t.Fatalf("receive: %v", err)
	}
	if heard.Error != "" || !heard.Final {
		t.Fatalf("heard %+v, want the final text", heard)
	}
	return heard.Text
}

// streamIndex returns the index of an item's stream of a kind in a
// language, by its first letters.
func (s *acquisitionServer) streamIndex(itemID string, kind libraryv1.StreamKind, language string) int32 {
	s.t.Helper()
	index := int32(-1)
	var sources []*libraryv1.MediaSource
	waitFor(s.t, fmt.Sprintf("a %v stream in %s", kind, language), func() bool {
		resp, err := s.items.GetItem(s.t.Context(), libraryv1.GetItemRequest_builder{Id: new(itemID)}.Build())
		if err != nil {
			s.t.Fatalf("GetItem: %v", err)
		}
		sources = resp.GetMediaSources()
		for _, src := range sources {
			for _, st := range src.GetStreams() {
				if st.GetKind() == kind && strings.HasPrefix(st.GetLanguage(), language) {
					index = st.GetIndex()
				}
			}
		}
		return index >= 0
	}, func() any { return sources })
	return index
}

// requested lists the titles requested.
func (s *acquisitionServer) requested() []string {
	s.t.Helper()
	resp, err := s.requests.ListRequests(s.t.Context(), &acquisitionv1.ListRequestsRequest{})
	if err != nil {
		s.t.Fatalf("ListRequests: %v", err)
	}
	var out []string
	for _, r := range resp.GetRequests() {
		out = append(out, r.GetTitle())
	}
	return out
}

// inLibrary lists the items of a kind in a library.
func (s *acquisitionServer) inLibrary(libraryID string, kind libraryv1.ItemKind) []*libraryv1.Item {
	s.t.Helper()
	resp, err := s.items.ListItems(s.t.Context(), libraryv1.ListItemsRequest_builder{
		LibraryIds: []string{libraryID}, Kinds: []libraryv1.ItemKind{kind},
	}.Build())
	if err != nil {
		s.t.Fatalf("ListItems: %v", err)
	}
	return resp.GetItems()
}

// movie returns the only movie of a library.
func (s *acquisitionServer) movie(libraryID string) *libraryv1.Item {
	s.t.Helper()
	var movies []*libraryv1.Item
	waitFor(s.t, "the library's movie", func() bool {
		movies = s.inLibrary(libraryID, libraryv1.ItemKind_ITEM_KIND_MOVIE)
		return len(movies) == 1
	}, func() any { return names(movies) })
	return movies[0]
}

// TestAssistant runs the assistant's fast path through the API, without
// an AI gateway plugin, with the voice plugin against a fake
// whisper-asr-webservice on the loopback interface (P29's done
// criteria): English and Chinese requests pause, seek and switch the
// subtitles of the requesting device; a film named by a homophone of its
// Chinese name plays; a request of a title is canceled, another is
// confirmed by voice; the recognizer that listened transcribes an
// episode's subtitles.
func TestAssistant(t *testing.T) {
	t.Parallel()
	speechWAV := fixtures.Require(t, "speech.wav")
	s := startAcquisition(t, assistantRoutes(t), false, voicePlugin)
	ctx := t.Context()
	a := s.assistant()

	// A film with English and Chinese subtitles, and an episode.
	movies, shows := t.TempDir(), t.TempDir()
	srt := func(text string) string {
		path := filepath.Join(t.TempDir(), "sub.srt")
		if err := os.WriteFile(path, []byte("1\n00:00:01,000 --> 00:00:03,000\n"+text+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	filmDir := filepath.Join(movies, "盗梦空间 (2010)")
	if err := os.MkdirAll(filmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	film := filepath.Join(filmDir, "盗梦空间 (2010).mkv")
	run(t, s.ffmpeg, "-loglevel", "error", "-i", s.video("640x360", 30), "-i", srt("Hello."), "-i", srt("你好。"),
		"-map", "0", "-map", "1", "-map", "2", "-c", "copy", "-c:s", "srt",
		"-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=chi", film)
	settled(t, film)
	episodeDir := filepath.Join(shows, "Breaking Bad", "Season 01")
	if err := os.MkdirAll(episodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(s.video("320x240", 20), filepath.Join(episodeDir, "Breaking Bad S01E01.mkv")); err != nil {
		t.Fatal(err)
	}
	movieLib := s.library("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies)
	showLib := s.library("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	movie := s.movie(movieLib)

	// whisper-asr is the server's recognizer, and transcribes subtitles.
	asr := newFakeASR(t)
	if _, err := s.pluginAdmin.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId:   new("dev.mavio.voice"),
		ConfigJson: new(`{"recognizers": [{"name": "whisper-asr", "enabled": true, "settings": {"endpoint": "` + asr.URL + `", "subtitles": true}}]}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	waitFor(t, "whisper-asr recognizing", func() bool {
		got, err := a.GetAssistant(ctx, &assistantv1.GetAssistantRequest{})
		return err == nil && len(got.GetRecognizers()) > 0
	})

	// 1. The device plays the film without subtitles.
	english := s.streamIndex(movie.GetId(), libraryv1.StreamKind_STREAM_KIND_SUBTITLE, "en")
	chinese := s.streamIndex(movie.GetId(), libraryv1.StreamKind_STREAM_KIND_SUBTITLE, "chi")
	playing := func(language string) *assistantv1.AskContext {
		return assistantv1.AskContext_builder{
			Languages: []string{language}, Screen: new("player"),
			Playing: assistantv1.AskPlaying_builder{
				ItemId: new(movie.GetId()), Position: durationpb.New(10 * time.Second), Speed: new(1.0),
				AudioStreamIndex: new(int32(1)), SubtitleStreamIndex: new(int32(-1)), Volume: new(int32(80)),
			}.Build(),
		}.Build()
	}
	control := func(text, language string, want assistantv1.ControlCommand, value float64) {
		t.Helper()
		events := ask(t, a, assistantv1.AskRequest_builder{Text: new(text), Context: playing(language)}.Build())
		acts := actions(events)
		if len(acts) != 1 || acts[0].GetControl().GetCommand() != want || acts[0].GetControl().GetValue() != value {
			t.Errorf("%q acts %v, want = %v %v", text, acts, want, value)
		}
	}
	control("pause", "en", assistantv1.ControlCommand_CONTROL_COMMAND_PAUSE, 0)
	control("暂停", "zh", assistantv1.ControlCommand_CONTROL_COMMAND_PAUSE, 0)
	control("go back 30 seconds", "en", assistantv1.ControlCommand_CONTROL_COMMAND_SEEK_BY, -30)
	control("快进30秒", "zh", assistantv1.ControlCommand_CONTROL_COMMAND_SEEK_BY, 30)
	subtitles := func(text, language string, want int32) {
		t.Helper()
		events := ask(t, a, assistantv1.AskRequest_builder{Text: new(text), Context: playing(language)}.Build())
		acts := actions(events)
		if len(acts) != 1 || !acts[0].HasStreams() || acts[0].GetStreams().GetSubtitleStreamIndex() != want {
			t.Errorf("%q acts %v, want = subtitles %d", text, acts, want)
		}
	}
	subtitles("switch the subtitles to Chinese", "en", chinese)
	subtitles("字幕换成英文", "zh", english)

	// 2. 道梦空间 sounds as 盗梦空间, which plays.
	events := ask(t, a, assistantv1.AskRequest_builder{
		Text: new("播放道梦空间"), Context: assistantv1.AskContext_builder{Languages: []string{"zh"}, Screen: new("home")}.Build(),
	}.Build())
	if acts := actions(events); len(acts) != 1 || !slices.Equal(acts[0].GetPlay().GetItemIds(), []string{movie.GetId()}) {
		t.Errorf("播放道梦空间 acts %v, want = play %s", acts, movie.GetId())
	}

	// 3. A request is asked to be confirmed, and canceled.
	home := func(language string) *assistantv1.AskContext {
		return assistantv1.AskContext_builder{Languages: []string{language}, Screen: new("home")}.Build()
	}
	events = ask(t, a, assistantv1.AskRequest_builder{Text: new("求黑客帝国"), Context: home("zh")}.Build())
	if !done(events).GetConfirmation() {
		t.Fatalf("求黑客帝国 answered %v, want = a confirmation", events)
	}
	if d := done(ask(t, a, assistantv1.AskRequest_builder{Text: new("不要"), Context: home("zh")}.Build())); d.GetConfirmation() || d.GetAnswer() == "" {
		t.Errorf("不要 answered %v, want = canceled", d)
	}
	if got := s.requested(); len(got) != 0 {
		t.Errorf("requests after canceling = %q, want = none", got)
	}

	// 4. Another is confirmed by voice: the recognizer hears the recording.
	events = ask(t, a, assistantv1.AskRequest_builder{Text: new("request The Matrix"), Context: home("en")}.Build())
	if !done(events).GetConfirmation() {
		t.Fatalf("request The Matrix answered %v, want = a confirmation", events)
	}
	asr.say("Yes.")
	heard := s.listen(speechWAV)
	if heard != "Yes." {
		t.Fatalf("heard %q, want = Yes.", heard)
	}
	d := done(ask(t, a, assistantv1.AskRequest_builder{Text: new(heard), Spoken: new(true), Context: home("en")}.Build()))
	if d.GetConfirmation() || !strings.Contains(d.GetAnswer(), "The Matrix") {
		t.Errorf("Yes. answered %v, want = The Matrix requested", d)
	}
	if got := s.requested(); !slices.Equal(got, []string{"The Matrix"}) {
		t.Errorf("requests = %q, want = The Matrix", got)
	}

	// 5. The same recognizer transcribes the episode's subtitles.
	var episode *libraryv1.Item
	waitFor(t, "the episode", func() bool {
		eps := s.inLibrary(showLib, libraryv1.ItemKind_ITEM_KIND_EPISODE)
		if len(eps) == 1 {
			episode = eps[0]
		}
		return episode != nil
	})
	waitFor(t, "whisper-asr transcribing subtitles", func() bool {
		resp, err := s.subtitles.ListSubtitleProviders(ctx, &acquisitionv1.ListSubtitleProvidersRequest{})
		if err != nil {
			return false
		}
		return slices.ContainsFunc(resp.GetProviders(), func(p *acquisitionv1.SubtitleProviderState) bool {
			return p.GetId() == transcriptionID && p.GetEnabled()
		})
	})
	asr.say("Say my name.")
	en := acquisitionv1.SubtitleLanguageSpec_builder{Language: new("en")}.Build()
	found, err := s.subtitles.SearchItemSubtitles(ctx, acquisitionv1.SearchItemSubtitlesRequest_builder{
		ItemId: new(episode.GetId()), Languages: []*acquisitionv1.SubtitleLanguageSpec{en},
	}.Build())
	if err != nil {
		t.Fatalf("SearchItemSubtitles: %v", err)
	}
	i := slices.IndexFunc(found.GetSubtitles(), func(sub *acquisitionv1.ScoredSubtitle) bool { return sub.GetProvider() == transcriptionID })
	if i < 0 {
		t.Fatalf("subtitles found = %v, want = whisper-asr's transcription", found.GetSubtitles())
	}
	transcribed := found.GetSubtitles()[i]
	saved, err := s.subtitles.DownloadItemSubtitle(ctx, acquisitionv1.DownloadItemSubtitleRequest_builder{
		ItemId: new(episode.GetId()), Provider: new(transcribed.GetProvider()), SubtitleId: new(transcribed.GetSubtitleId()),
		Language: transcribed.GetLanguage(),
	}.Build())
	if err != nil {
		t.Fatalf("DownloadItemSubtitle: %v", err)
	}
	if data, err := os.ReadFile(saved.GetPath()); err != nil || !strings.Contains(string(data), "Say my name.") {
		t.Errorf("transcription = %q, %v; want = whisper-asr's", data, err)
	}
	asr.mu.Lock()
	if !slices.Equal(asr.tasks, []string{"transcribe", "transcribe"}) {
		t.Errorf("whisper-asr was asked to %q, want = transcribe the recording, then the episode", asr.tasks)
	}
	asr.mu.Unlock()
}

// modelCall is a call of a tool a fake model makes.
type modelCall struct {
	Name string
	Args any
}

// modelRequest is what a fake model is asked: its instructions, the
// messages' roles and texts, and the tools offered.
type modelRequest struct {
	System   string
	Roles    []string
	Messages []string
	Tools    []string
}

// lastUser returns the last message of the user, and whether a tool's
// result followed it.
func (r modelRequest) lastUser() (string, bool) {
	for i := len(r.Roles) - 1; i >= 0; i-- {
		if r.Roles[i] == "user" {
			return r.Messages[i], slices.Contains(r.Roles[i+1:], "tool")
		}
	}
	return "", false
}

// fakeModel serves an OpenAI-compatible chat completions API whose model
// answers with the text or the call answer returns for a request.
func fakeModel(t *testing.T, answer func(modelRequest) (string, *modelCall)) *httptest.Server {
	t.Helper()
	var (
		mu    sync.Mutex
		calls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req modelRequest
		for _, m := range body.Messages {
			var s string
			if json.Unmarshal(m.Content, &s) != nil {
				var parts []struct {
					Text string `json:"text"`
				}
				_ = json.Unmarshal(m.Content, &parts)
				for _, p := range parts {
					s += p.Text
				}
			}
			if m.Role == "system" || m.Role == "developer" {
				req.System += s + "\n"
				continue
			}
			req.Roles = append(req.Roles, m.Role)
			req.Messages = append(req.Messages, s)
		}
		for _, tl := range body.Tools {
			req.Tools = append(req.Tools, tl.Function.Name)
		}
		if n := len(req.Messages); n > 0 {
			t.Logf("model asked after %s: %.300s", req.Roles[n-1], req.Messages[n-1])
		}
		text, call := answer(req)
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(delta any, finish string) {
			choice := map[string]any{"index": 0, "delta": delta}
			if finish != "" {
				choice["finish_reason"] = finish
			}
			b, _ := json.Marshal(map[string]any{
				"id": "1", "object": "chat.completion.chunk", "model": "m", "choices": []any{choice},
				"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			})
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		if call != nil {
			args, _ := json.Marshal(call.Args)
			mu.Lock()
			calls++
			id := fmt.Sprintf("call_%d", calls)
			mu.Unlock()
			chunk(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"index": 0, "id": id, "type": "function", "function": map[string]any{"name": call.Name, "arguments": string(args)},
			}}}, "")
			chunk(map[string]any{}, "tool_calls")
		} else {
			chunk(map[string]any{"role": "assistant", "content": text}, "")
			chunk(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAssistantAI runs the assistant's AI path through the API with the
// built agent plugin and a fake OpenAI-compatible model on the loopback
// interface (P29's done criteria): a request the fast path does not
// match runs tools as the user through a script of code mode, whose
// QuickJS sandbox loads in the built executable; a call of a script
// interrupted by restarting the plugin runs once; what the user tells is
// remembered in the next conversation.
func TestAssistantAI(t *testing.T) {
	t.Parallel()
	agent := processPlugin{name: "agent", source: agentSource, build: agentBuild}
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("integration test: bun not found on PATH")
	}
	s := startAcquisition(t, assistantRoutes(t), false, agent)
	ctx := t.Context()
	a := s.assistant()

	movies := t.TempDir()
	filmDir := filepath.Join(movies, "Inception (2010)")
	if err := os.MkdirAll(filmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(s.video("320x240", 20), filepath.Join(filmDir, "Inception (2010).mkv")); err != nil {
		t.Fatal(err)
	}
	movie := s.movie(s.library("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies))

	codemode := func(code string) *modelCall {
		return &modelCall{Name: "codemode", Args: map[string]string{"code": code}}
	}
	model := fakeModel(t, func(r modelRequest) (string, *modelCall) {
		text, answered := r.lastUser()
		switch {
		case strings.Contains(r.System, "You maintain what an assistant"):
			return "{}", nil
		case strings.Contains(text, "dreams") && !answered:
			return "", codemode(`const found = await tools.search_library({ query: "Inception" });
await tools.play({ item_ids: [found.items[0].id] });
return found.items[0].name;`)
		case strings.Contains(text, "dreams"):
			return "Playing Inception.", nil
		case strings.Contains(text, "Tonight") && !answered:
			return "", codemode(`const found = await tools.search_library({ query: "Inception" });
await tools.add_to_playlist({ playlist: "Tonight", item_ids: [found.items[0].id] });
return await tools.request_title({ query: "The Matrix" });`)
		case strings.Contains(text, "Tonight"):
			return "Added, and requested The Matrix.", nil
		case strings.Contains(text, "daughter") && !answered:
			if slices.Contains(r.Tools, "remember") {
				return "", &modelCall{Name: "remember", Args: map[string]string{"text": "The user watches with their daughter."}}
			}
			return "", codemode(`return await tools.remember({ text: "The user watches with their daughter." });`)
		case strings.Contains(text, "daughter"):
			return "I will keep that in mind.", nil
		case strings.Contains(text, "know about me"):
			if strings.Contains(r.System, "watches with their daughter") {
				return "You watch with your daughter.", nil
			}
			return "Nothing yet.", nil
		}
		return "OK.", nil
	})

	// The agent plugin serves the model, which the assistant uses with code
	// mode and memory.
	waitFor(t, "the agent plugin started", func() bool {
		_, err := s.pluginAdmin.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
			PluginId:   new("dev.mavio.agent"),
			ConfigJson: new(fmt.Sprintf(`{"providers":[{"id":"local","name":"Local","base_url":%q,"models":[{"id":"assistant","name":"Assistant"}]}]}`, model.URL+"/v1")),
		}.Build())
		return err == nil
	})
	got, err := a.GetAssistantSettings(ctx, &assistantv1.GetAssistantSettingsRequest{})
	if err != nil {
		t.Fatalf("GetAssistantSettings: %v", err)
	}
	settings := got.GetSettings()
	settings.SetEnabled(true)
	settings.SetModel("ai/local/assistant")
	settings.SetCodeMode(true)
	settings.SetMemory(true)
	if _, err := a.UpdateAssistantSettings(ctx, assistantv1.UpdateAssistantSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		t.Fatalf("UpdateAssistantSettings: %v", err)
	}
	home := assistantv1.AskContext_builder{Languages: []string{"en"}, Screen: new("home")}.Build()

	// 1. A request the fast path does not match is the model's, whose
	// script finds the film and plays it on the requesting device.
	events := ask(t, a, assistantv1.AskRequest_builder{Text: new("Something about dreams tonight?"), Context: home}.Build())
	if !understoodByAI(events) {
		t.Fatalf("answer = %v, want = the AI path's", events)
	}
	if acts := actions(events); len(acts) != 1 || !slices.Equal(acts[0].GetPlay().GetItemIds(), []string{movie.GetId()}) {
		t.Errorf("acts %v, want = play %s", acts, movie.GetId())
	}
	if d := done(events); d.GetAnswer() != "Playing Inception." || d.GetConversationId() == "" {
		t.Errorf("done = %v, want = the model's answer in a conversation", d)
	}

	// 2. A script adds the film to a playlist and asks to request a title,
	// which waits for confirmation while the plugin restarts.
	events = ask(t, a, assistantv1.AskRequest_builder{Text: new("Tonight list for this film, and The Matrix asked for"), Context: home}.Build())
	if !done(events).GetConfirmation() {
		t.Fatalf("answer = %v, want = a confirmation", events)
	}
	for _, enabled := range []bool{false, true} {
		if _, err := s.pluginAdmin.SetPluginEnabled(ctx, systemv1.SetPluginEnabledRequest_builder{
			PluginId: new("dev.mavio.agent"), Enabled: new(enabled),
		}.Build()); err != nil {
			t.Fatalf("SetPluginEnabled %v: %v", enabled, err)
		}
	}
	waitFor(t, "the agent plugin restarted", func() bool {
		_, err := a.ListAssistantConversations(ctx, &assistantv1.ListAssistantConversationsRequest{})
		return err == nil
	})
	// The restart ended the model's turn, so the server answers for it.
	d := done(ask(t, a, assistantv1.AskRequest_builder{Text: new("yes"), Context: home}.Build()))
	if d.GetConfirmation() || !strings.Contains(d.GetAnswer(), "The Matrix") || d.GetAnswer() == "Added, and requested The Matrix." {
		t.Errorf("yes answered %v, want = the server's answer that The Matrix is requested", d)
	}
	playlists := libraryv1connect.NewPlaylistServiceClient(http.DefaultClient, s.url, s.token)
	waitFor(t, "the script finished", func() bool {
		conv, err := a.GetAssistantConversation(ctx, assistantv1.GetAssistantConversationRequest_builder{Id: new(done(events).GetConversationId())}.Build())
		if err != nil {
			return false
		}
		msgs := conv.GetMessages()
		return len(msgs) > 0 && msgs[len(msgs)-1].GetText() == "Added, and requested The Matrix."
	})
	list, err := playlists.ListPlaylists(ctx, &libraryv1.ListPlaylistsRequest{})
	if err != nil || len(list.GetPlaylists()) != 1 || list.GetPlaylists()[0].GetName() != "Tonight" {
		t.Fatalf("playlists = %v, %v; want = Tonight", list.GetPlaylists(), err)
	}
	resp, err := playlists.ListPlaylistEntries(ctx, libraryv1.ListPlaylistEntriesRequest_builder{Id: new(list.GetPlaylists()[0].GetId())}.Build())
	if err != nil {
		t.Fatalf("ListPlaylistEntries: %v", err)
	}
	if n := resp.GetTotal(); n != 1 {
		t.Errorf("Tonight's entries = %d, want = 1", n)
	}
	if got := s.requested(); !slices.Equal(got, []string{"The Matrix"}) {
		t.Errorf("requests = %q, want = The Matrix once", got)
	}

	// 3. What the user tells is remembered in the next conversation.
	events = ask(t, a, assistantv1.AskRequest_builder{Text: new("By the way, my daughter is always with me on the sofa."), Context: home}.Build())
	if d := done(events); d.GetAnswer() != "I will keep that in mind." {
		t.Errorf("done = %v, want = the model's answer", d)
	}
	memories, err := a.ListAssistantMemories(ctx, &assistantv1.ListAssistantMemoriesRequest{})
	if err != nil || len(memories.GetMemories()) != 1 || memories.GetMemories()[0].GetText() != "The user watches with their daughter." {
		t.Errorf("memories = %v, %v; want = the daughter", memories.GetMemories(), err)
	}
	if _, err := a.DeleteAssistantConversation(ctx, assistantv1.DeleteAssistantConversationRequest_builder{Id: new(done(events).GetConversationId())}.Build()); err != nil {
		t.Fatalf("DeleteAssistantConversation: %v", err)
	}
	events = ask(t, a, assistantv1.AskRequest_builder{Text: new("What do you know about me?"), Context: home}.Build())
	if d := done(events); d.GetAnswer() != "You watch with your daughter." || d.GetConversationId() == "" {
		t.Errorf("done = %v, want = the memory used in a new conversation", d)
	}
	if _, err := a.GetAssistantConversation(ctx, assistantv1.GetAssistantConversationRequest_builder{Id: new(done(events).GetConversationId())}.Build()); err != nil {
		t.Errorf("GetAssistantConversation of the new conversation: %v", err)
	}
}
