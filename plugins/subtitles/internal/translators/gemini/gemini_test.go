package gemini

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// fakeGemini translates by upper-casing, answering batches larger than
// limit with an error and the keys of limited with a rate limit.
type fakeGemini struct {
	t       *testing.T
	limit   int
	limited map[string]bool
	// rtl translates to Hebrew.
	rtl bool
	// fenced wraps answers in a Markdown fence.
	fenced bool

	mu      sync.Mutex
	keys    []string
	batches []int
	system  string
}

func (f *fakeGemini) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/models/gemini-pro:generateContent" {
		http.NotFound(w, r)
		return
	}
	key := r.URL.Query().Get("key")
	var req struct {
		SystemInstruction struct {
			Parts []struct{ Text string } `json:"parts"`
		} `json:"system_instruction"`
		Contents []struct {
			Role  string                  `json:"role"`
			Parts []struct{ Text string } `json:"parts"`
		} `json:"contents"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil || len(req.Contents) != 1 || req.Contents[0].Role != "user" {
		f.t.Errorf("request = %s, want one user content", body)
	}
	var items []item
	_ = json.Unmarshal([]byte(req.Contents[0].Parts[0].Text), &items)
	f.mu.Lock()
	f.keys = append(f.keys, key)
	f.system = req.SystemInstruction.Parts[0].Text
	limited := f.limited[key]
	if !limited {
		f.batches = append(f.batches, len(items))
	}
	f.mu.Unlock()
	switch {
	case limited:
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED"}}`))
		return
	case len(items) > f.limit:
		http.Error(w, `{"error":{"message":"output too long"}}`, http.StatusInternalServerError)
		return
	}
	var out []map[string]any
	for _, it := range items {
		c := strings.ToUpper(it.Content)
		if f.rtl {
			c = "שלום"
		}
		out = append(out, map[string]any{"index": it.Index, "content": c})
	}
	data, _ := json.Marshal(out)
	text := string(data)
	if f.fenced {
		text = "```json\n" + text + "\n```"
	}
	half := len([]rune(text)) / 2
	answer, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{
		"parts": []any{map[string]any{"text": string([]rune(text)[:half])}, map[string]any{"text": string([]rune(text)[half:])}},
	}}}})
	_, _ = w.Write(answer)
}

func newTranslator(t *testing.T, f *fakeGemini, settings string, now *time.Time) *Translator {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	tr, err := New(provider.Env{HTTP: srv.Client(), Now: func() time.Time { return *now }, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	g := tr.(*Translator)
	g.SetBaseURL(srv.URL)
	return g
}

func TestTranslate(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	f := &fakeGemini{t: t, limit: 2, limited: map[string]bool{"k1": true}, fenced: true}
	g := newTranslator(t, f, `{"api_keys":" k1, k2 ,k1","model":"gemini-pro","batch_size":0}`, &now)
	if g.BatchSize() != 1 || g.Credit() != "gemini-pro" {
		t.Errorf("batch size = %d, credit = %q; want 1 and the model", g.BatchSize(), g.Credit())
	}
	req := &translator.Request{
		Source: "en", Target: "fr", Lines: []string{"<i>one</i>", "two", "three", "four"},
		Media: translator.Media{Title: "The Matrix", Year: 1999, ExternalIDs: map[string]string{"imdb": "tt0133093"}, Overview: "Neo."},
	}
	got, err := g.Translate(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<I>ONE</I>|TWO|THREE|FOUR"; strings.Join(got, "|") != want {
		t.Errorf("Translate got = %q, want = %q", got, want)
	}
	// k1 rested; the batch of four failed and went in halves.
	if strings.Join(f.keys, ",") != "k1,k2,k2,k2" || len(f.batches) != 3 || f.batches[0] != 4 || f.batches[1] != 2 {
		t.Errorf("keys = %v, batches = %v; want k1 rate limited, then halves of the batch", f.keys, f.batches)
	}
	if !strings.Contains(f.system, "translates subtitles to French.") ||
		!strings.HasSuffix(f.system, "Additional user instruction: 'You will translate movie that is called The Matrix from 1999 and it has IMDB ID = tt0133093. Its overview: Neo.'") {
		t.Errorf("instruction = %q, want French and the movie's description", f.system)
	}

	// Both keys rate limited fail the translation until k1 rested.
	f.limited["k2"] = true
	f.keys = nil
	if _, err := g.Translate(t.Context(), req); provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("Translate with every key rate limited: err = %v, want too many requests", err)
	}
	if _, err := g.Translate(t.Context(), req); provider.KindOf(err) != provider.TooManyRequests || len(f.keys) != 1 {
		t.Errorf("Translate with every key resting: err = %v after %v, want too many requests without a request", err, f.keys)
	}
	now = now.Add(2*time.Minute + time.Second)
	f.limited = map[string]bool{}
	f.limit, f.rtl, f.fenced = 10, true, false
	got, err = g.Translate(t.Context(), &translator.Request{Target: "he", Lines: []string{"hello"}})
	if err != nil || got[0] != "\u202bשלום\u202c" {
		t.Errorf("Translate to Hebrew = %q, %v; want the line embedded right to left", got, err)
	}
}

func TestTranslateFailure(t *testing.T) {
	now := time.Now()
	f := &fakeGemini{t: t, limit: 0, limited: map[string]bool{}}
	g := newTranslator(t, f, `{"api_keys":["k"],"model":"gemini-pro"}`, &now)
	_, err := g.Translate(t.Context(), &translator.Request{Target: "fr", Lines: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}})
	var ae *apiError
	if !errors.As(err, &ae) || ae.status != http.StatusInternalServerError {
		t.Errorf("Translate failing: err = %v, want the answer's error", err)
	}
	// The batch, its first half, its first quarter and its first eighth.
	if len(f.keys) != 4 {
		t.Errorf("requests = %d, want 4", len(f.keys))
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{"api_keys":" , "}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New without keys: err = %v, want a configuration error", err)
	}
}

func TestDescription(t *testing.T) {
	m := translator.Media{Episode: true, Title: "Lost", Year: 2004, Overview: "Survivors."}
	if got, want := Description(m), "You will translate TV show that is called Lost from 2004 and it has IMDB ID = . Its overview: Survivors."; got != want {
		t.Errorf("Description got = %q, want = %q", got, want)
	}
	if got := Description(translator.Media{}); got != "" {
		t.Errorf("Description without a title got = %q, want none", got)
	}
}
