package lingarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// fakeLingarr translates by upper-casing, answering its first requests
// with fails' statuses.
type fakeLingarr struct {
	t     *testing.T
	fails []int
	got   []request
}

func (f *fakeLingarr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/translate/content" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("X-Api-Key") != "secret" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		f.t.Errorf("decode request: %v", err)
	}
	f.got = append(f.got, req)
	if len(f.fails) > 0 {
		code := f.fails[0]
		f.fails = f.fails[1:]
		w.WriteHeader(code)
		return
	}
	var out []map[string]any
	for _, l := range req.Lines {
		out = append(out, map[string]any{"position": *l.Position, "line": strings.ToUpper(*l.Line)})
	}
	_ = json.NewEncoder(w).Encode(out)
}

func newTranslator(t *testing.T, f *fakeLingarr, token string) (*Translator, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	tr, err := New(provider.Env{HTTP: srv.Client()}, json.RawMessage(`{"url":"`+srv.URL+`/","token":"`+token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	l := tr.(*Translator)
	var waits []time.Duration
	l.SetSleep(func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	})
	return l, &waits
}

func TestTranslate(t *testing.T) {
	f := &fakeLingarr{t: t, fails: []int{http.StatusBadGateway, http.StatusTooManyRequests}}
	l, waits := newTranslator(t, f, "secret")
	got, err := l.Translate(t.Context(), &translator.Request{
		Source: "en", Target: "pt-BR", Lines: []string{"Hello", " ", "Bye"},
		Media: translator.Media{Episode: true, Title: "Lost", SourceFileName: "Lost - S01E01.en.srt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "HELLO||BYE" {
		t.Errorf("Translate got = %q, want = HELLO||BYE", got)
	}
	if len(f.got) != 3 || len(*waits) != 2 {
		t.Fatalf("requests = %d, waits = %v; want three tries", len(f.got), *waits)
	}
	r := f.got[2]
	if r.Title != "Lost · Lost - S01E01.en.srt" || r.SourceLanguage != "en" || r.TargetLanguage != "pt-BR" || r.MediaType != "Episode" ||
		len(r.Lines) != 2 || *r.Lines[1].Position != 2 {
		t.Errorf("request = %+v, want the episode's non-blank cues", r)
	}
}

func TestTranslateFailure(t *testing.T) {
	f := &fakeLingarr{t: t}
	l, _ := newTranslator(t, f, "wrong")
	if _, err := l.Translate(t.Context(), &translator.Request{Source: "en", Target: "fr", Lines: []string{"a"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("Translate with a wrong key: err = %v, want an authentication error", err)
	}
	l, _ = newTranslator(t, f, "secret")
	f.fails = []int{http.StatusBadRequest}
	if _, err := l.Translate(t.Context(), &translator.Request{Source: "en", Target: "fr", Lines: []string{"a"}}); err == nil || len(f.fails) != 0 {
		t.Errorf("Translate answered 400: err = %v, want a failure without retrying", err)
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{"url":"lingarr"}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New with an invalid URL: err = %v, want a configuration error", err)
	}
}
