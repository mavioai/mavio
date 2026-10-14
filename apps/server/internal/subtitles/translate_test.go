package subtitles

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/tools/fixtures"
)

// fakeTranslator is a translator plugin with the translator "upper",
// which upper-cases lines, batch at a time.
type fakeTranslator struct {
	enabled bool
	markup  bool
	batch   int32

	mu       sync.Mutex
	requests []*pluginv1.TranslateSubtitleLinesRequest
}

func (f *fakeTranslator) ListSubtitleTranslators(context.Context, *pluginv1.ListSubtitleTranslatorsRequest) (*pluginv1.ListSubtitleTranslatorsResponse, error) {
	return pluginv1.ListSubtitleTranslatorsResponse_builder{Translators: []*pluginv1.SubtitleTranslatorInfo{
		pluginv1.SubtitleTranslatorInfo_builder{
			Name: proto.String("upper"), DisplayName: proto.String("Upper"), Enabled: proto.Bool(f.enabled),
			BatchSize: proto.Int32(f.batch), Markup: proto.Bool(f.markup),
		}.Build(),
	}}.Build(), nil
}

func (f *fakeTranslator) TranslateSubtitleLines(_ context.Context, req *pluginv1.TranslateSubtitleLinesRequest) (*pluginv1.TranslateSubtitleLinesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	var out []string
	for _, l := range req.GetLines() {
		if l == "♪" {
			// Nothing to translate.
			out = append(out, "")
			continue
		}
		out = append(out, strings.ToUpper(l))
	}
	return pluginv1.TranslateSubtitleLinesResponse_builder{Lines: out, Credit: proto.String("Upper 1.0")}.Build(), nil
}

func TestTranslate(t *testing.T) {
	e := newEnv(t)
	tr := &fakeTranslator{batch: 2}
	e.rt.Translators = func() []Translator { return []Translator{{ID: "plug", Client: tr}} }
	en, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:00,500 --> 00:00:02,000\n<i>Hello</i>\nthere\n\n"+
		"2\n00:00:03,000 --> 00:00:04,000\n♪\n\n3\n00:00:05,000 --> 00:00:06,000\nBye\n"), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	fr := core.SubtitleLanguage{Language: "fr"}
	if _, err := e.rt.QueueTranslate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("QueueTranslate without an enabled translator: err = %v, want ErrUnavailable", err)
	}
	tr.enabled = true
	for _, c := range []struct {
		name string
		p    TranslatePayload
		want error
	}{
		{"unknown file", TranslatePayload{ItemID: e.movie.ID, Path: "/elsewhere/x.srt", Language: fr}, core.ErrNotFound},
		{"no source", TranslatePayload{ItemID: e.movie.ID, Language: fr}, core.ErrInvalid},
		{"audio stream", TranslatePayload{ItemID: e.movie.ID, StreamIndex: new(1), Language: fr}, core.ErrInvalid},
		{"unknown translator", TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr, Translator: "plug/other"}, core.ErrNotFound},
	} {
		if _, err := e.rt.QueueTranslate(e.ctx, c.p); !errors.Is(err, c.want) {
			t.Errorf("QueueTranslate(%s): err = %v, want %v", c.name, err, c.want)
		}
	}
	id, err := e.rt.QueueTranslate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr, Translator: "plug/upper"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := e.db.Jobs().Lease(e.ctx, "test", []string{JobTranslate}, time.Minute)
	if err != nil || job.ID != id {
		t.Fatalf("leased job = %+v, %v; want the %s job %s", job, err, JobTranslate, id)
	}
	if _, err := e.rt.Handlers()[JobTranslate](e.ctx, job); err != nil {
		t.Fatal(err)
	}

	// The plain text of the cues with text went in batches of two; the
	// translation is SubRip, credited before its first cue.
	if len(tr.requests) != 2 || strings.Join(tr.requests[0].GetLines(), "|") != "Hello\nthere|♪" ||
		tr.requests[0].GetSourceLanguage() != "en" || tr.requests[0].GetTargetLanguage() != "fr" ||
		tr.requests[0].GetMedia().GetTitle() != "The Matrix" || tr.requests[0].GetMedia().GetFileName() != "The Matrix (1999).fr.srt" {
		t.Errorf("requests = %v, want the cues' plain text in batches of two", tr.requests)
	}
	got := e.file("The Matrix (1999).fr.srt")
	want := "1\n00:00:00,000 --> 00:00:00,500\n# Subtitles translated with Upper 1.0 # \n\n" +
		"2\n00:00:00,500 --> 00:00:02,000\nHELLO\nTHERE\n\n3\n00:00:03,000 --> 00:00:04,000\n♪\n\n4\n00:00:05,000 --> 00:00:06,000\nBYE\n"
	if strings.TrimSpace(strings.ReplaceAll(got, "\r\n", "\n")) != strings.TrimSpace(want) {
		t.Errorf("translation = %q, want = %q", got, want)
	}
	h := e.history(core.SubtitleTranslated)
	if len(h) != 1 || h[0].Language != fr || h[0].Score != 90 || h[0].MaxScore != 180 ||
		h[0].Description != "en subtitles translated to fr with Upper 1.0." {
		t.Errorf("history = %+v, want one translation to fr scored half the maximum", h)
	}

	// A translator keeping markup takes SubRip's; without the credit, an
	// embedded stream translated keeps its cues.
	tr.markup, tr.batch, tr.requests = true, 0, nil
	e.s.Translation.Credit = false
	e.rt.Extracted = func(context.Context, core.ID, *core.MediaSource, *core.MediaStream) ([]byte, string, error) {
		return []byte("1\n00:00:01,000 --> 00:00:02,000\n<i>Hi</i>\n"), "srt", nil
	}
	es := core.SubtitleLanguage{Language: "es", Forced: true}
	if _, err := e.rt.Translate(e.ctx, TranslatePayload{ItemID: e.movie.ID, StreamIndex: new(2), Language: es}); err != nil {
		t.Fatal(err)
	}
	if len(tr.requests) != 1 || strings.Join(tr.requests[0].GetLines(), "|") != "<i>Hi</i>" {
		t.Errorf("requests = %v, want SubRip's markup", tr.requests)
	}
	if got := e.file("The Matrix (1999).es.forced.srt"); !strings.Contains(got, "<I>HI</I>") || strings.Contains(got, "translated with") {
		t.Errorf("translation = %q, want the markup kept and no credit", got)
	}

	// Embedded subtitles of videos on cloud drives are not read.
	e.rt.Class = func(context.Context, string) core.StorageClass { return core.StorageCloud }
	if _, err := e.rt.QueueTranslate(e.ctx, TranslatePayload{ItemID: e.movie.ID, StreamIndex: new(2), Language: es}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("QueueTranslate of a stream on a cloud drive: err = %v, want ErrUnavailable", err)
	}
}

func TestAudio(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found")
	}
	speech := fixtures.Require(t, "speech.wav")
	e := newEnv(t)
	e.rt.FFmpeg = ffmpeg
	video := path.Join(e.dir, "The Matrix (1999).mkv")
	data, err := os.ReadFile(speech)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(video, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := core.DefaultSubtitleSettings()
	tg, err := e.rt.load(e.ctx, e.movie.ID, &s)
	if err != nil {
		t.Fatal(err)
	}
	v := e.rt.pluginVideo(e.ctx, tg, &s)
	if len(v.GetAudioStreams()) != 1 || v.GetAudioStreams()[0].GetLanguage() != "eng" {
		t.Errorf("audio streams = %v, want one in eng", v.GetAudioStreams())
	}
	u := v.GetAudioUrl()
	if !strings.HasPrefix(u, audioBase) {
		t.Fatalf("audio URL = %q, want one of the host API", u)
	}
	if again := e.rt.pluginVideo(e.ctx, tg, &s).GetAudioUrl(); again != u {
		t.Errorf("audio URL = %q, then %q; want the same", u, again)
	}
	mux := http.NewServeMux()
	mux.Handle(AudioPath, e.rt.AudioHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	get := func(path string) (int, []byte) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, body
	}
	path := strings.TrimPrefix(u, "http://mavio.host")
	if code, body := get(path + "?seconds=2"); code != http.StatusOK || len(body) != 2*16000*2 {
		t.Errorf("GET %s?seconds=2 = %d with %d bytes, want 2 seconds of 16 kHz samples", path, code, len(body))
	}
	if code, body := get(path); code != http.StatusOK || len(body) < 50*16000*2 {
		t.Errorf("GET %s = %d with %d bytes, want the whole minute", path, code, len(body))
	}
	for _, c := range []struct {
		path string
		want int
	}{
		{path + "?stream=1", http.StatusUnprocessableEntity},
		{path + "?seconds=x", http.StatusBadRequest},
		{"/subtitles/audio/unknown", http.StatusNotFound},
	} {
		if code, _ := get(c.path); code != c.want {
			t.Errorf("GET %s = %d, want = %d", c.path, code, c.want)
		}
	}

	// The audio of videos on cloud drives is not read.
	e.rt.Class = func(context.Context, string) core.StorageClass { return core.StorageCloud }
	if u := e.rt.pluginVideo(e.ctx, tg, &s).GetAudioUrl(); u != "" {
		t.Errorf("audio URL on a cloud drive = %q, want none", u)
	}
}
