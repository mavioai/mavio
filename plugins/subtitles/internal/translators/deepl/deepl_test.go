package deepl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// fakeDeepL lists a few languages and upper-cases texts.
type fakeDeepL struct {
	t        *testing.T
	listed   int
	requests []map[string]any
	status   int
}

func (f *fakeDeepL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "DeepL-Auth-Key k:fx" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/v2/languages":
		f.listed++
		langs := []string{"EN", "FR", "PT", "ZH"}
		if r.URL.Query().Get("type") == "target" {
			langs = []string{"EN-GB", "EN-US", "FR", "PT", "PT-BR", "PT-PT", "ZH-HANS", "ZH-HANT"}
		}
		var out []map[string]string
		for _, l := range langs {
			out = append(out, map[string]string{"language": l, "name": l})
		}
		_ = json.NewEncoder(w).Encode(out)
	case "/v2/translate":
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.requests = append(f.requests, body)
		var out []map[string]string
		for _, t := range body["text"].([]any) {
			out = append(out, map[string]string{"text": strings.ToUpper(t.(string))})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"translations": out})
	default:
		http.NotFound(w, r)
	}
}

func TestTranslate(t *testing.T) {
	if _, err := New(provider.Env{}, nil); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New without a key: err = %v, want a configuration error", err)
	}
	f := &fakeDeepL{t: t}
	srv := httptest.NewServer(f)
	defer srv.Close()
	tr, err := New(provider.Env{HTTP: srv.Client()}, json.RawMessage(`{"api_key":" k:fx "}`))
	if err != nil {
		t.Fatal(err)
	}
	d := tr.(*Translator)
	if d.base != "https://api-free.deepl.com" {
		t.Errorf("base = %s, want the free API for :fx keys", d.base)
	}
	d.SetBaseURL(srv.URL)
	src, tgt, err := d.Languages(t.Context())
	if err != nil || strings.Join(src, ",") != "EN,FR,ZH" || strings.Join(tgt, ",") != "EN-GB,EN-US,FR,PT-BR,PT-PT,ZH-HANS,ZH-HANT" {
		t.Errorf("languages = %v, %v, %v; want DeepL's without the generic Portuguese", src, tgt, err)
	}
	lines := make([]string, 0, 60)
	for range 30 {
		lines = append(lines, "hi", "")
	}
	got, err := d.Translate(t.Context(), &translator.Request{Source: "en", Target: "zh-TW", Lines: lines})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 60 || got[0] != "HI" || got[1] != "" || got[58] != "HI" {
		t.Errorf("translated = %q, want the texts upper-cased and the blanks kept", got)
	}
	if len(f.requests) != 1 || f.requests[0]["source_lang"] != "EN" || f.requests[0]["target_lang"] != "ZH-HANT" || len(f.requests[0]["text"].([]any)) != 30 {
		t.Errorf("requests = %v, want the 30 texts at once from EN to ZH-HANT", f.requests)
	}
	if f.listed != 2 {
		t.Errorf("languages listed %d times, want once a day", f.listed)
	}
	if _, err := d.Translate(t.Context(), &translator.Request{Source: "en", Target: "de", Lines: []string{"a"}}); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("to German: err = %v, want a configuration error", err)
	}
	for status, want := range map[int]provider.Kind{456: provider.SearchLimitReached, 429: provider.TooManyRequests, 503: provider.ServiceUnavailable} {
		f.status = status
		if _, err := d.Translate(t.Context(), &translator.Request{Source: "en", Target: "fr", Lines: []string{"a"}}); provider.KindOf(err) != want {
			t.Errorf("answered %d: err = %v, want %v", status, err, want)
		}
	}
}
