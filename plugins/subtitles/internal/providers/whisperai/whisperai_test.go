package whisperai

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fakeWhisper is whisper-asr-webservice detecting lang, and the host API
// serving a video's audio as its stream's index and seconds.
type fakeWhisper struct {
	t    *testing.T
	lang string

	mu      sync.Mutex
	audio   []string
	detects int
	asr     []string
}

func (f *fakeWhisper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/subtitles/audio/tok":
		f.audio = append(f.audio, r.URL.RawQuery)
		_, _ = w.Write([]byte("pcm:" + r.URL.RawQuery))
	case "/subtitles/audio/none":
		http.Error(w, "no audio", http.StatusUnprocessableEntity)
	case "/detect-language", "/asr":
		if r.Header.Get("User-Agent") != userAgent || r.URL.Query().Get("encode") != "false" {
			f.t.Errorf("request %s, want subliminal's user agent and encode=false", r.URL)
		}
		file, _, err := r.FormFile("audio_file")
		if err != nil {
			f.t.Fatalf("audio_file: %v", err)
		}
		audio, _ := io.ReadAll(file)
		if r.URL.Path == "/detect-language" {
			f.detects++
			if string(audio) != "pcm:seconds=30&stream=0" {
				f.t.Errorf("detected from %q, want the first 30 seconds of the first stream", audio)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"language_code": f.lang, "detected_language": "whatever"})
			return
		}
		q := r.URL.Query()
		f.asr = append(f.asr, q.Get("task")+" "+q.Get("language")+" "+q.Get("video_file")+" "+string(audio))
		_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"))
	default:
		http.NotFound(w, r)
	}
}

func newProvider(t *testing.T, f *fakeWhisper, settings string) (*Provider, string) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Host: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)},
		json.RawMessage(strings.ReplaceAll(settings, "%s", srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider), srv.URL
}

func TestSearch(t *testing.T) {
	en, fr, de := provider.Language{Tag: "en"}, provider.Language{Tag: "fr", Forced: true}, provider.Language{Tag: "de"}
	cases := []struct {
		name    string
		streams []provider.AudioStream
		detect  string
		langs   []provider.Language
		// want are the release names found.
		want    []string
		detects int
	}{
		{
			name:    "tagged",
			streams: []provider.AudioStream{{Index: 0, Language: "fre"}},
			langs:   []provider.Language{en, fr, de},
			want:    []string{"translate French audio -> English SRT", "transcribe French audio -> French SRT"},
		},
		{
			name:    "untagged",
			streams: []provider.AudioStream{{Index: 0}},
			detect:  "de",
			langs:   []provider.Language{en, de},
			want:    []string{"translate German audio -> English SRT", "transcribe German audio -> German SRT"},
			detects: 1,
		},
		{
			name:    "ambiguous",
			streams: []provider.AudioStream{{Index: 0, Language: "und"}, {Index: 1, Language: "eng"}},
			detect:  "jw",
			langs:   []provider.Language{en, fr},
			want:    []string{"translate Javanese audio -> English SRT"},
			detects: 1,
		},
		{
			name:    "undetected",
			streams: []provider.AudioStream{{Index: 0, Language: "mul"}},
			detect:  "und",
			langs:   []provider.Language{en},
			detects: 1,
		},
		{
			name:    "swiss german",
			streams: []provider.AudioStream{{Index: 0, Language: "gsw"}},
			langs:   []provider.Language{de},
			want:    []string{"transcribe German audio -> German SRT"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeWhisper{t: t, lang: c.detect}
			p, base := newProvider(t, f, `{"endpoint":"%s/"}`)
			v := &provider.Video{Episode: true, AudioURL: base + "/subtitles/audio/tok", AudioStreams: c.streams}
			found, err := p.Search(t.Context(), v, c.langs)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, s := range found {
				got = append(got, s.ReleaseNames...)
				if !slices.Equal(s.Matches, []string{"series", "season", "episode"}) || s.Format != "srt" {
					t.Errorf("subtitle = %+v, want an episode's matches", s)
				}
			}
			if !slices.Equal(got, c.want) || f.detects != c.detects {
				t.Errorf("found %q with %d detections, want %q with %d", got, f.detects, c.want, c.detects)
			}
			// Detections are remembered.
			if _, err := p.Search(t.Context(), v, c.langs); err != nil || f.detects != c.detects {
				t.Errorf("searched again: %d detections, %v; want %d", f.detects, err, c.detects)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	f := &fakeWhisper{t: t}
	p, base := newProvider(t, f, `{"endpoint":"%s","pass_video_name":true}`)
	v := &provider.Video{
		FileName: "/movies/Amélie (2001).mkv", AudioURL: base + "/subtitles/audio/tok",
		AudioStreams: []provider.AudioStream{{Index: 0, Language: "eng"}, {Index: 1, Language: "fre"}},
	}
	fr := provider.Language{Tag: "fr", HearingImpaired: true}
	found, err := p.Search(t.Context(), v, []provider.Language{fr, {Tag: "en"}})
	if err != nil || len(found) != 2 {
		t.Fatalf("Search = %+v, %v; want French and English", found, err)
	}
	file, err := p.Download(t.Context(), v, found[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if file.Format != "srt" || file.Language != fr || !strings.Contains(string(file.Data), "Hi") {
		t.Errorf("Download = %+v, want the French SubRip", file)
	}
	if _, err := p.Download(t.Context(), v, found[1].ID); err != nil {
		t.Fatal(err)
	}
	want := []string{"transcribe fr /movies/Amélie (2001).mkv pcm:stream=1", "transcribe en /movies/Amélie (2001).mkv pcm:stream=0"}
	if !slices.Equal(f.asr, want) {
		t.Errorf("transcriptions = %q, want = %q", f.asr, want)
	}

	// Without audio, nothing is found nor transcribed.
	if found, err := p.Search(t.Context(), &provider.Video{}, []provider.Language{fr}); err != nil || len(found) != 0 {
		t.Errorf("Search without audio = %+v, %v; want nothing", found, err)
	}
	v.AudioURL = base + "/subtitles/audio/none"
	if _, err := p.Download(t.Context(), v, found[0].ID); err == nil {
		t.Error("Download of a video without audio: err = nil, want one")
	}
	v.AudioStreams = nil
	if found, err := p.Search(t.Context(), v, []provider.Language{fr}); err != nil || len(found) != 0 {
		t.Errorf("Search of a video whose audio is missing = %+v, %v; want nothing", found, err)
	}
}

func TestDefinition(t *testing.T) {
	d := Definition()
	for _, tag := range []string{"en", "jv", "gsw", "haw", "no"} {
		if !slices.Contains(d.Languages, tag) {
			t.Errorf("languages = %v, want %s among them", d.Languages, tag)
		}
	}
	if len(d.Languages) != len(languages) {
		t.Errorf("%d languages, want %d", len(d.Languages), len(languages))
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{"timeout":0}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New with no timeout: err = %v, want a configuration error", err)
	}
}
