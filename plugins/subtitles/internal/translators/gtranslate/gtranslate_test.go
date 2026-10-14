package gtranslate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// fakeServices answers as the four services do, upper-casing texts and
// noting the language pairs asked.
type fakeServices struct {
	t *testing.T

	mu      sync.Mutex
	pairs   []string
	pages   int
	limited bool
}

func (f *fakeServices) note(pair string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pairs = append(f.pairs, pair)
}

var signature = regexp.MustCompile(`^MSTranslatorAndroidApp::[A-Za-z0-9+/]+=*::[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2}GMT::[0-9a-f]{32}$`)

func (f *fakeServices) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	switch r.URL.Path {
	case "/translate_a/single":
		if r.URL.Query().Get("client") != "gtx" || !regexp.MustCompile(`^\d+\.\d+$`).MatchString(r.URL.Query().Get("tk")) {
			f.t.Errorf("Google request = %s, want the extension's", r.URL)
		}
		f.note(r.URL.Query().Get("sl") + ">" + r.URL.Query().Get("tl"))
		if f.limited {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Google answers a text's sentences apart.
		text := strings.ToUpper(r.PostForm.Get("q"))
		half := len(text) / 2
		_ = json.NewEncoder(w).Encode(map[string]any{"sentences": []map[string]string{{"trans": text[:half]}, {"trans": text[half:]}}, "src": "en"})
	case "/translator":
		f.mu.Lock()
		f.pages++
		f.mu.Unlock()
		fmt.Fprintf(w, `<script>var params_AbusePreventionHelper = [%d,"tok",3600000];</script>`, time.Now().UnixMilli())
	case "/ttranslatev3":
		if r.PostForm.Get("token") != "tok" || r.URL.Query().Get("IID") != bingIID || len(r.URL.Query().Get("IG")) != 32 {
			_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 400, "message": "bad credentials"})
			return
		}
		f.note(r.PostForm.Get("fromLang") + ">" + r.PostForm.Get("to"))
		_ = json.NewEncoder(w).Encode([]any{map[string]any{"translations": []map[string]string{{"text": strings.ToUpper(r.PostForm.Get("text"))}}}})
	case "/translate":
		if !signature.MatchString(r.Header.Get("X-MT-Signature")) || r.URL.Query().Get("api-version") != "3.0" {
			f.t.Errorf("Microsoft signature = %q, want the app's", r.Header.Get("X-MT-Signature"))
		}
		var body []map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.note(r.URL.Query().Get("from") + ">" + r.URL.Query().Get("to"))
		_ = json.NewEncoder(w).Encode([]any{map[string]any{"translations": []map[string]string{{"text": strings.ToUpper(body[0]["Text"])}}}})
	case "/api/v1/tr.json/translate":
		if r.URL.Query().Get("srv") != "android" || len(r.URL.Query().Get("ucid")) != 32 || r.UserAgent() != yandexUserAgent {
			f.t.Errorf("Yandex request = %s, want the app's", r.URL)
		}
		f.note(strings.Replace(r.PostForm.Get("lang"), "-", ">", 1))
		if r.PostForm.Get("lang") == "en-xx" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 501, "message": "The specified translation direction is not supported"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "lang": r.PostForm.Get("lang"), "text": []string{strings.ToUpper(r.PostForm.Get("text"))}})
	default:
		http.NotFound(w, r)
	}
}

func TestTranslators(t *testing.T) {
	f := &fakeServices{t: t}
	srv := httptest.NewServer(f)
	defer srv.Close()
	env := provider.Env{HTTP: srv.Client()}
	defs := Definitions()
	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	if strings.Join(names, ",") != "google,bing,microsoft,yandex" {
		t.Errorf("definitions = %v, want Lingarr's GTranslate services", names)
	}
	for _, c := range []struct {
		name, source, target, pair string
	}{
		{"google", "en", "zh", "en>zh-CN"},
		{"google", "en", "pt", "en>pt"},
		{"bing", "en-US", "zh-TW", "en>zh-Hant"},
		{"microsoft", "en", "no", "en>nb"},
		{"microsoft", "en", "pt-PT", "en>pt-PT"},
		{"yandex", "en", "pt-PT", "en>pt"},
		{"yandex", "en", "pt-BR", "en>pt-BR"},
	} {
		f.pairs = nil
		var def translator.Definition
		for _, d := range defs {
			if d.Name == c.name {
				def = d
			}
		}
		tr, err := def.New(env, nil)
		if err != nil {
			t.Fatal(err)
		}
		tr.(*Translator).SetBaseURL(srv.URL)
		got, err := tr.Translate(t.Context(), &translator.Request{Source: c.source, Target: c.target, Lines: []string{"hello", " ", "bye now"}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if strings.Join(got, "|") != "HELLO||BYE NOW" {
			t.Errorf("%s translated = %q, want the texts upper-cased and the blank kept", c.name, got)
		}
		if len(f.pairs) != 2 || f.pairs[0] != c.pair {
			t.Errorf("%s %s>%s asked %v, want %s twice", c.name, c.source, c.target, f.pairs, c.pair)
		}
	}
	if f.pages != 1 {
		t.Errorf("Bing's page read %d times, want once while its credentials last", f.pages)
	}

	// Languages, failures and limits.
	g, _ := defs[0].New(env, nil)
	g.(*Translator).SetBaseURL(srv.URL)
	src, tgt, err := g.(translator.Languages).Languages(t.Context())
	if err != nil || len(src) < 100 || len(tgt) != len(src) {
		t.Errorf("Google's languages = %d, %d, %v; want its hundred and more", len(src), len(tgt), err)
	}
	if _, err := g.Translate(t.Context(), &translator.Request{Source: "en", Target: "tlh", Lines: []string{"a"}}); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("Google to Klingon: err = %v, want a configuration error", err)
	}
	f.limited = true
	if _, err := g.Translate(t.Context(), &translator.Request{Source: "en", Target: "fr", Lines: []string{"a", "b"}}); provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("Google rate limited: err = %v, want too many requests", err)
	}
	m, _ := defs[2].New(env, nil)
	if _, err := m.Translate(t.Context(), &translator.Request{Source: "en", Target: "fr", Lines: []string{strings.Repeat("a", 1001)}}); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("Microsoft with 1001 characters: err = %v, want a configuration error", err)
	}
	y, _ := defs[3].New(env, nil)
	y.(*Translator).svc.(*yandexService).base = srv.URL
	y.(*Translator).spec.langs = append(y.(*Translator).spec.langs, translator.Language{Code: "xx", Tag: "und"})
	if _, err := y.Translate(t.Context(), &translator.Request{Source: "en", Target: "und", Lines: []string{"a"}}); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Yandex failing: err = %v, want its message", err)
	}
}

func TestGoogleToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tk := googleToken("Hello, 世界 🎬", now)
	if !regexp.MustCompile(`^\d{1,6}\.\d+$`).MatchString(tk) || tk != googleToken("Hello, 世界 🎬", now) || tk == googleToken("Hello", now) {
		t.Errorf("token = %q, want a stable one of the text", tk)
	}
}

func TestMicrosoftSignature(t *testing.T) {
	got := microsoftSignature("api.cognitive.microsofttranslator.com/translate?api-version=3.0&to=fr", time.Date(2026, 10, 14, 8, 5, 9, 0, time.UTC), strings.Repeat("a", 32))
	if !signature.MatchString(got) || !strings.Contains(got, "::Wed, 14 Oct 2026 08:05:09GMT::") {
		t.Errorf("signature = %q, want the app's format", got)
	}
	if e := escapeDataString("a.b/c?d=e&f~g h"); e != "a.b%2Fc%3Fd%3De%26f~g%20h" {
		t.Errorf("escaped = %q", e)
	}
}

func TestPerCueCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := translator.PerCue(ctx, []string{"a", "b"}, 1, func(context.Context, string) (string, error) { return "x", nil }); err == nil {
		t.Error("PerCue canceled: err = nil, want the cancellation")
	}
}
