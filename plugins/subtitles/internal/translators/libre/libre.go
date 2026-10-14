// Package libre is the translator of LibreTranslate, ported from Lingarr's
// LibreService: cues go to a LibreTranslate server, between the languages
// it lists.
package libre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// Name is the translator's name.
const Name = "libretranslate"

// batchSize is how many texts a request translates.
const batchSize = 25

// Definition returns the translator's definition.
func Definition() translator.Definition {
	return translator.Definition{
		Name: Name, DisplayName: "LibreTranslate",
		Settings: []provider.Setting{
			{Name: "url", Label: "LibreTranslate URL", Type: provider.Text},
			{Name: "api_key", Label: "API key", Help: "The server's API key, when it requires one.", Type: provider.Password},
		},
		New: New,
	}
}

type settings struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// New makes the translator from its settings.
func New(env provider.Env, raw json.RawMessage) (translator.Translator, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	s.URL = strings.TrimSuffix(strings.TrimSpace(s.URL), "/")
	if u, err := url.Parse(s.URL); s.URL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "invalid URL %q", s.URL)
	}
	return &Translator{env: env, s: s}, nil
}

// Translator is LibreTranslate.
type Translator struct {
	env   provider.Env
	s     settings
	langs translator.LanguageCache
}

// Credit implements translator.Translator.
func (*Translator) Credit() string { return "LibreTranslate" }

// BatchSize implements translator.Translator.
func (*Translator) BatchSize() int { return batchSize }

func (t *Translator) now() time.Time {
	if t.env.Now != nil {
		return t.env.Now()
	}
	return time.Now()
}

// do sends a request to the server, decoding its answer into out.
func (t *Translator) do(ctx context.Context, method, path string, body, out any) error {
	r := bytes.NewReader(nil)
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.s.URL+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("LibreTranslate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return translator.StatusError("LibreTranslate", resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return provider.Errorf(provider.ParseResponseError, "LibreTranslate: %w", err)
	}
	return nil
}

// list lists the server's languages; Brazilian Portuguese is "pb".
func (t *Translator) list(ctx context.Context) (source, target []translator.Language, err error) {
	var langs []struct {
		Code string `json:"code"`
	}
	if err := t.do(ctx, http.MethodGet, "/languages", nil, &langs); err != nil {
		return nil, nil, err
	}
	out := []translator.Language{}
	for _, l := range langs {
		lang := translator.Language{Code: l.Code}
		if l.Code == "pb" {
			lang.Tag = "pt-BR"
		}
		out = append(out, lang)
	}
	return out, out, nil
}

// Languages implements translator.Languages.
func (t *Translator) Languages(ctx context.Context) (source, target []string, err error) {
	s, tg, err := t.langs.Get(ctx, t.now(), t.list)
	if err != nil {
		return nil, nil, err
	}
	return translator.Tags(s), translator.Tags(tg), nil
}

// Translate implements translator.Translator.
func (t *Translator) Translate(ctx context.Context, req *translator.Request) ([]string, error) {
	langs, _, err := t.langs.Get(ctx, t.now(), t.list)
	if err != nil {
		return nil, err
	}
	from, ok := translator.Match(req.Source, langs)
	if !ok {
		return nil, provider.Errorf(provider.ConfigurationError, "LibreTranslate does not translate from %s", req.Source)
	}
	to, ok := translator.Match(req.Target, langs)
	if !ok {
		return nil, provider.Errorf(provider.ConfigurationError, "LibreTranslate does not translate to %s", req.Target)
	}
	out := make([]string, len(req.Lines))
	texts, indexes := translator.Texts(req.Lines)
	for len(texts) > 0 {
		n := min(len(texts), batchSize)
		body := map[string]any{"q": texts[:n], "source": from, "target": to, "format": "text"}
		if t.s.APIKey != "" {
			body["api_key"] = t.s.APIKey
		}
		var resp struct {
			TranslatedText []string `json:"translatedText"`
		}
		if err := t.do(ctx, http.MethodPost, "/translate", body, &resp); err != nil {
			return nil, err
		}
		if len(resp.TranslatedText) != n {
			return nil, provider.Errorf(provider.ParseResponseError, "LibreTranslate translated %d texts of %d", len(resp.TranslatedText), n)
		}
		for i, tr := range resp.TranslatedText {
			out[indexes[i]] = tr
		}
		texts, indexes = texts[n:], indexes[n:]
	}
	return out, nil
}
