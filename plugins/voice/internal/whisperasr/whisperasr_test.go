package whisperasr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

// fakeWhisper is whisper-asr-webservice detecting Javanese, recording
// the requests and the sizes of the audio it is sent.
type fakeWhisper struct {
	t *testing.T

	mu       sync.Mutex
	requests []string
	sizes    []int
}

func (f *fakeWhisper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, _, err := r.FormFile("audio_file")
	if err != nil {
		f.t.Fatalf("audio_file: %v", err)
	}
	audio, _ := io.ReadAll(file)
	f.sizes = append(f.sizes, len(audio))
	f.requests = append(f.requests, r.URL.Path+"?"+r.URL.RawQuery)
	switch r.URL.Path {
	case "/detect-language":
		_, _ = io.WriteString(w, `{"detected_language":"javanese","language_code":"jw"}`)
	case "/asr":
		_, _ = io.WriteString(w, `{"text":" Halo. Piye kabare?","language":"jw","segments":[{"start":0.0,"end":1.25,"text":" Halo."},{"start":1.5,"end":2.0,"text":" Piye kabare?"}]}`)
	default:
		http.NotFound(w, r)
	}
}

func audio(seconds int) service.Audio {
	data := make([]byte, seconds*service.BytesPerSecond)
	return service.Audio{Name: "/films/Film.mkv", Open: func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}}
}

func TestRecognizer(t *testing.T) {
	f := &fakeWhisper{t: t}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	r, err := New(service.Env{}, json.RawMessage(`{"endpoint":"`+srv.URL+`/","pass_name":true}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Recognize(t.Context(), service.Request{Audio: audio(3), Language: "jv", Hints: []string{"Yogyakarta"}, Timestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []service.Segment{{Start: 0, End: 1250 * time.Millisecond, Text: "Halo."}, {Start: 1500 * time.Millisecond, End: 2 * time.Second, Text: "Piye kabare?"}}
	if res.Text != "Halo. Piye kabare?" || res.Language != "jv" || !slices.Equal(res.Segments, want) {
		t.Errorf("Recognize = %+v", res)
	}
	if _, err := r.Recognize(t.Context(), service.Request{Audio: audio(1), Language: "gsw-CH", Translate: true}); err != nil {
		t.Fatal(err)
	}
	lang, err := r.DetectLanguage(t.Context(), audio(60))
	if err != nil || lang != "jv" {
		t.Errorf("DetectLanguage = %q, %v; want jv", lang, err)
	}
	wantRequests := []string{
		"/asr?encode=false&initial_prompt=Yogyakarta&language=jw&output=json&task=transcribe&video_file=%2Ffilms%2FFilm.mkv",
		"/asr?encode=false&language=de&output=json&task=translate&video_file=%2Ffilms%2FFilm.mkv",
		"/detect-language?encode=false&video_file=%2Ffilms%2FFilm.mkv",
	}
	if !slices.Equal(f.requests, wantRequests) || !slices.Equal(f.sizes, []int{3 * service.BytesPerSecond, service.BytesPerSecond, 30 * service.BytesPerSecond}) {
		t.Errorf("requests = %q of %v bytes, want = %q of 3 s, 1 s and 30 s", f.requests, f.sizes, wantRequests)
	}
	if _, err := r.Recognize(t.Context(), service.Request{Audio: audio(1), Language: "tlh"}); err == nil {
		t.Error("Recognize in Klingon: err = nil, want one")
	}
	if _, err := New(service.Env{}, json.RawMessage(`{"timeout":0}`)); err == nil {
		t.Error("New without a timeout: err = nil, want one")
	}
}
