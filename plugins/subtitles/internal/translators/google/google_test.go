package google

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// fakeGoogle translates by upper-casing, failing as fails says.
type fakeGoogle struct {
	mu    sync.Mutex
	tries map[string]int
	// fails answers the first tries of a text with a status, or with an
	// error page when the status is 200.
	fails map[string]int
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("sl") != "auto" || r.Header.Get("User-Agent") == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	text := q.Get("q")
	f.mu.Lock()
	f.tries[text]++
	n := f.tries[text]
	f.mu.Unlock()
	switch {
	case text == "unknown":
		_, _ = w.Write([]byte(`<html><body><div class="other">?</div></body></html>`))
		return
	case text == "♪":
		_, _ = w.Write([]byte(`<div class="result-container">♪</div>`))
		return
	case n <= 2 && f.fails[text] == http.StatusOK:
		_, _ = w.Write([]byte(`<div class="t0">Our systems have detected unusual traffic</div>`))
		return
	case n <= 2 && f.fails[text] != 0:
		w.WriteHeader(f.fails[text])
		return
	}
	_, _ = w.Write([]byte(`<html><body><div class="t0 x">` + html.EscapeString(q.Get("tl")+":"+strings.ToUpper(text)) + `</div></body></html>`))
}

func newTranslator(t *testing.T, f *fakeGoogle) (*Translator, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	tr, err := New(provider.Env{HTTP: srv.Client()}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	g := tr.(*Translator)
	g.SetBaseURL(srv.URL + "/m")
	var (
		mu    sync.Mutex
		waits []time.Duration
	)
	g.SetSleep(func(_ context.Context, d time.Duration) error {
		mu.Lock()
		defer mu.Unlock()
		waits = append(waits, d)
		return nil
	})
	return g, &waits
}

func TestTranslate(t *testing.T) {
	f := &fakeGoogle{tries: map[string]int{}, fails: map[string]int{"busy": http.StatusTooManyRequests, "page": http.StatusOK}}
	g, waits := newTranslator(t, f)
	got, err := g.Translate(t.Context(), &translator.Request{
		Source: "en", Target: "he", Lines: []string{" hello & bye ", "", "♪", "unknown", "busy", "page"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"iw:HELLO & BYE", "", "", "", "iw:BUSY", "iw:PAGE"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Translate got = %q, want = %q", got, want)
	}
	if f.tries["busy"] != 3 || f.tries["page"] != 3 || len(*waits) != 4 {
		t.Errorf("tries = %v, waits = %v; want three tries of the failing cues", f.tries, *waits)
	}
	for _, d := range *waits {
		if d < time.Second || d >= 3*time.Second {
			t.Errorf("wait = %v, want 1 or 2 seconds and up to one more", d)
		}
	}
	if got, err := g.Translate(t.Context(), &translator.Request{Target: "zh-Hant", Lines: []string{"a"}}); err != nil || got[0] != "zh-TW:A" {
		t.Errorf("Translate to zh-Hant = %q, %v; want zh-TW", got, err)
	}
}

func TestTranslateFailure(t *testing.T) {
	f := &fakeGoogle{tries: map[string]int{}, fails: map[string]int{"down": http.StatusBadGateway}}
	g, waits := newTranslator(t, f)
	// The cue fails all of its tries.
	f.tries["down"] = -10
	_, err := g.Translate(t.Context(), &translator.Request{Target: "fr", Lines: []string{"ok", "down"}})
	if err == nil || !strings.Contains(err.Error(), "1/2 line(s)") {
		t.Errorf("Translate with a failing cue: err = %v, want the cue's failure", err)
	}
	if f.tries["down"] != -10+tries || len(*waits) != tries-1 {
		t.Errorf("tries = %d, waits = %v; want %d tries", f.tries["down"]+10, *waits, tries)
	}
	if _, err := g.Translate(t.Context(), &translator.Request{Target: "fr", Lines: []string{strings.Repeat("a", maxChars)}}); err == nil {
		t.Error("Translate of a cue too long: err = nil, want one")
	}
}

func TestLooksLikeError(t *testing.T) {
	for _, c := range []struct {
		s    string
		want bool
	}{
		{"Bonjour", false},
		{"Error 500", true},
		{"HTTP-429", true},
		{"<div>x</div>", true},
		{"Too Many Requests", true},
		{"Le 500e jour", false},
	} {
		if got := looksLikeError(c.s); got != c.want {
			t.Errorf("looksLikeError(%q) got = %v, want = %v", c.s, got, c.want)
		}
	}
}
