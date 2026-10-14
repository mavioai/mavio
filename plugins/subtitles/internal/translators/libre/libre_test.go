package libre

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

func TestTranslate(t *testing.T) {
	for _, raw := range []string{``, `{"url":"ftp://x"}`} {
		if _, err := New(provider.Env{}, json.RawMessage(raw)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s): err = %v, want a configuration error", raw, err)
		}
	}
	var requests []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/languages":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"code": "en"}, {"code": "pb"}, {"code": "zh-Hans"}})
		case "/translate":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["api_key"] != "secret" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			requests = append(requests, body)
			var out []string
			for _, q := range body["q"].([]any) {
				out = append(out, strings.ToUpper(q.(string)))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"translatedText": out})
		}
	}))
	defer srv.Close()
	tr, err := New(provider.Env{HTTP: srv.Client()}, json.RawMessage(`{"url":"`+srv.URL+`/","api_key":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	src, _, err := tr.(translator.Languages).Languages(t.Context())
	if err != nil || strings.Join(src, ",") != "en,pt-BR,zh-Hans" {
		t.Errorf("languages = %v, %v; want the server's, pb as Brazilian Portuguese", src, err)
	}
	got, err := tr.Translate(t.Context(), &translator.Request{Source: "en", Target: "pt", Lines: []string{"a", " ", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "A||B" || len(requests) != 1 || requests[0]["target"] != "pb" || requests[0]["format"] != "text" {
		t.Errorf("translated = %q with %v, want the texts to pb at once", got, requests)
	}
	tr, _ = New(provider.Env{HTTP: srv.Client()}, json.RawMessage(`{"url":"`+srv.URL+`"}`))
	if _, err := tr.Translate(t.Context(), &translator.Request{Source: "en", Target: "zh", Lines: []string{"a"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("without the key: err = %v, want an authentication error", err)
	}
}
