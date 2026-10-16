package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

// fakeAPI is a server of the OpenAI audio API answering every piece of
// audio with a segment from 0.5 to 0.9 seconds, in French.
type fakeAPI struct {
	t *testing.T

	mu     sync.Mutex
	sizes  []int
	fields []string
	speech map[string]any
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/v1/audio/transcriptions", "/v1/audio/translations":
		file, _, err := r.FormFile("file")
		if err != nil {
			f.t.Fatalf("file: %v", err)
		}
		wav, _ := io.ReadAll(file)
		if !bytes.HasPrefix(wav, []byte("RIFF")) {
			f.t.Errorf("audio is not WAV")
		}
		f.sizes = append(f.sizes, len(wav)-44)
		f.fields = append(f.fields, strings.Join([]string{
			r.URL.Path, r.FormValue("model"), r.FormValue("language"), r.FormValue("prompt"), r.FormValue("response_format"),
		}, " "))
		_, _ = io.WriteString(w, `{"text":" Bonjour.","language":"french","segments":[{"start":0.5,"end":0.9,"text":" Bonjour."}]}`)
	case "/v1/audio/speech":
		_ = json.NewDecoder(r.Body).Decode(&f.speech)
		_, _ = io.WriteString(w, "OggS")
	default:
		http.NotFound(w, r)
	}
}

func newAPI(t *testing.T) (*fakeAPI, string) {
	f := &fakeAPI{t: t}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv.URL + "/v1"
}

func pcm(d time.Duration) service.Audio {
	data := make([]byte, int(d/time.Millisecond)*service.BytesPerSecond/1000)
	return service.Audio{Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }}
}

func TestRecognize(t *testing.T) {
	f, base := newAPI(t)
	r, err := NewRecognizer(service.Env{}, json.RawMessage(`{"base_url":"`+base+`/","api_key":"key"}`))
	if err != nil {
		t.Fatal(err)
	}
	r.(*Recognizer).chunk = time.Second
	res, err := r.Recognize(t.Context(), service.Request{Audio: pcm(2500 * time.Millisecond), Language: "fr-CA", Hints: []string{"Amélie", "Paris"}, Timestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	var starts []time.Duration
	for _, s := range res.Segments {
		starts = append(starts, s.Start)
	}
	if res.Language != "fr" || res.Text != "Bonjour. Bonjour. Bonjour." || !slices.Equal(starts, []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond, 2500 * time.Millisecond}) {
		t.Errorf("Recognize = %+v, want three segments a second apart in French", res)
	}
	if want := []int{32000, 32000, 16000}; !slices.Equal(f.sizes, want) {
		t.Errorf("sizes sent = %v, want = %v", f.sizes, want)
	}
	if got, want := f.fields[0], "/v1/audio/transcriptions whisper-1 fr Amélie, Paris verbose_json"; got != want {
		t.Errorf("fields = %q, want = %q", got, want)
	}

	f.sizes, f.fields = nil, nil
	res, err = r.Recognize(t.Context(), service.Request{Audio: pcm(time.Second), Translate: true})
	if err != nil || res.Language != "en" || res.Segments != nil || f.fields[0] != "/v1/audio/translations whisper-1   verbose_json" {
		t.Errorf("translation = %+v, %v, sent %q; want English without segments", res, err, f.fields)
	}

	f.sizes = nil
	if lang, err := r.DetectLanguage(t.Context(), pcm(time.Minute)); err != nil || lang != "fr" || !slices.Equal(f.sizes, []int{30 * service.BytesPerSecond}) {
		t.Errorf("DetectLanguage = %q, %v from %v bytes; want fr from 30 seconds", lang, err, f.sizes)
	}

	plain, err := NewRecognizer(service.Env{}, json.RawMessage(`{"base_url":"`+base+`","api_key":"key","timestamps":false,"translates":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if c := plain.Caps(); c.Timestamps || c.Detects || c.Translates {
		t.Errorf("caps = %+v, want neither timestamps, detection nor translation", c)
	}
	if _, err := plain.Recognize(t.Context(), service.Request{Audio: pcm(time.Second), Translate: true}); err == nil {
		t.Error("translation without it: err = nil, want one")
	}
	if _, err := NewRecognizer(service.Env{}, json.RawMessage(`{"base_url":"ftp://x"}`)); err == nil {
		t.Error("NewRecognizer with an FTP URL: err = nil, want one")
	}
}

func TestSynthesize(t *testing.T) {
	f, base := newAPI(t)
	s, err := NewSynthesizer(service.Env{}, json.RawMessage(`{"base_url":"`+base+`","api_key":"key","model":"kokoro","voices":"af_heart, bf_emma"}`))
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Voices(); len(v) != 2 || v[0].ID != "af_heart" || v[0].Name != "Af_heart" {
		t.Errorf("voices = %+v", v)
	}
	speech, err := s.Synthesize(t.Context(), "bf_emma", "Hello.", "en", service.MP3, 1.5)
	if err != nil {
		t.Fatal(err)
	}
	if speech.ContentType != "audio/mpeg" || string(speech.Audio) != "OggS" {
		t.Errorf("speech = %+v", speech)
	}
	if f.speech["model"] != "kokoro" || f.speech["voice"] != "bf_emma" || f.speech["response_format"] != "mp3" || f.speech["speed"] != 1.5 {
		t.Errorf("request = %v", f.speech)
	}
	f.speech = nil
	if _, err := s.Synthesize(t.Context(), "af_heart", "Hello.", "en", service.OggOpus, 0); err != nil || f.speech["response_format"] != "opus" || f.speech["speed"] != nil {
		t.Errorf("Opus request = %v, %v", f.speech, err)
	}
}
