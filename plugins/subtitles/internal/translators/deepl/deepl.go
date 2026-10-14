// Package deepl is the translator of DeepL, ported from Lingarr's
// DeepLService: cues go fifty at a time to DeepL's API with the
// account's key, between the languages DeepL lists.
package deepl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// Name is the translator's name.
const Name = "deepl"

// batchSize is how many texts DeepL translates in a request.
const batchSize = 50

// Definition returns the translator's definition.
func Definition() translator.Definition {
	return translator.Definition{
		Name: Name, DisplayName: "DeepL",
		Settings: []provider.Setting{
			{Name: "api_key", Label: "API key", Help: "The DeepL API key; keys of free accounts end with :fx.", Type: provider.Password},
		},
		New: New,
	}
}

type settings struct {
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
	if s.APIKey = strings.TrimSpace(s.APIKey); s.APIKey == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "an API key is required")
	}
	base := "https://api.deepl.com"
	if strings.HasSuffix(s.APIKey, ":fx") {
		base = "https://api-free.deepl.com"
	}
	return &Translator{env: env, key: s.APIKey, base: base}, nil
}

// Translator is DeepL.
type Translator struct {
	env   provider.Env
	key   string
	base  string
	langs translator.LanguageCache
}

// SetBaseURL points the translator at another server, for tests.
func (t *Translator) SetBaseURL(u string) { t.base = u }

// Credit implements translator.Translator.
func (*Translator) Credit() string { return "DeepL" }

// BatchSize implements translator.Translator.
func (*Translator) BatchSize() int { return batchSize }

func (t *Translator) now() time.Time {
	if t.env.Now != nil {
		return t.env.Now()
	}
	return time.Now()
}

// do sends a request to DeepL's API, decoding its answer into out.
func (t *Translator) do(ctx context.Context, method, path string, body, out any) error {
	r := bytes.NewReader(nil)
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.base+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "DeepL-Auth-Key "+t.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("DeepL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return translator.StatusError("DeepL", resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return provider.Errorf(provider.ParseResponseError, "DeepL: %w", err)
	}
	return nil
}

// list lists DeepL's languages, without the generic Portuguese, as
// Lingarr's.
func (t *Translator) list(ctx context.Context) (source, target []translator.Language, err error) {
	read := func(kind string) ([]translator.Language, error) {
		var langs []struct {
			Language string `json:"language"`
		}
		if err := t.do(ctx, http.MethodGet, "/v2/languages?type="+kind, nil, &langs); err != nil {
			return nil, err
		}
		out := []translator.Language{}
		for _, l := range langs {
			if !strings.EqualFold(l.Language, "pt") {
				out = append(out, translator.Language{Code: l.Language})
			}
		}
		return out, nil
	}
	if source, err = read("source"); err != nil {
		return nil, nil, err
	}
	if target, err = read("target"); err != nil {
		return nil, nil, err
	}
	return source, target, nil
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
	sources, targets, err := t.langs.Get(ctx, t.now(), t.list)
	if err != nil {
		return nil, err
	}
	from, ok := translator.Match(req.Source, sources)
	if !ok {
		return nil, provider.Errorf(provider.ConfigurationError, "DeepL does not translate from %s", req.Source)
	}
	to, ok := translator.Match(req.Target, targets)
	if !ok {
		return nil, provider.Errorf(provider.ConfigurationError, "DeepL does not translate to %s", req.Target)
	}
	out := make([]string, len(req.Lines))
	texts, indexes := translator.Texts(req.Lines)
	for len(texts) > 0 {
		n := min(len(texts), batchSize)
		var resp struct {
			Translations []struct {
				Text string `json:"text"`
			} `json:"translations"`
		}
		body := map[string]any{"text": texts[:n], "source_lang": from, "target_lang": to}
		if err := t.do(ctx, http.MethodPost, "/v2/translate", body, &resp); err != nil {
			return nil, err
		}
		if len(resp.Translations) != n {
			return nil, provider.Errorf(provider.ParseResponseError, "DeepL translated %d texts of %d", len(resp.Translations), n)
		}
		for i, tr := range resp.Translations {
			out[indexes[i]] = tr.Text
		}
		texts, indexes = texts[n:], indexes[n:]
	}
	return out, nil
}
