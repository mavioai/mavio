// Package gtranslate is the translators of Google Translate, Bing
// Translator, Microsoft Translator and Yandex.Translate, ported from
// Lingarr's GTranslatorService and the GTranslate library it uses: the
// free endpoints of their web pages and apps, without keys, a cue per
// request.
package gtranslate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

const (
	// batchSize is how many cues a request of the server takes.
	batchSize = 20
	// workers is how many cues are translated at once.
	workers = 4
	// userAgent is GTranslate's.
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
)

// service is a translation service.
type service interface {
	// text translates a text between the service's language codes.
	text(ctx context.Context, from, to, text string) (string, error)
}

// spec describes a service.
type spec struct {
	name, display string
	langs         []translator.Language
	// api is the code the service's API takes for a language's code in
	// langs, when another.
	api map[string]string
	// maxChars bounds a cue's length; 0 for no bound.
	maxChars int
	new      func(env provider.Env) service
}

// definition returns the translator definition of a service.
func (s *spec) definition() translator.Definition {
	return translator.Definition{
		Name: s.name, DisplayName: s.display,
		New: func(env provider.Env, _ json.RawMessage) (translator.Translator, error) {
			return &Translator{spec: s, svc: s.new(env)}, nil
		},
	}
}

// Translator is a service's translator.
type Translator struct {
	spec *spec
	svc  service
}

// SetBaseURL points the translator at another server, for tests.
func (t *Translator) SetBaseURL(u string) {
	if s, ok := t.svc.(interface{ setBase(string) }); ok {
		s.setBase(u)
	}
}

// Credit implements translator.Translator.
func (t *Translator) Credit() string { return t.spec.display }

// BatchSize implements translator.Translator.
func (*Translator) BatchSize() int { return batchSize }

// Languages implements translator.Languages.
func (t *Translator) Languages(context.Context) (source, target []string, err error) {
	tags := translator.Tags(t.spec.langs)
	return tags, tags, nil
}

// code returns the code the service's API takes for a BCP 47 tag.
func (t *Translator) code(tag string) (string, error) {
	c, ok := translator.Match(tag, t.spec.langs)
	if !ok {
		return "", provider.Errorf(provider.ConfigurationError, "%s does not translate %s", t.spec.display, tag)
	}
	if a, ok := t.spec.api[c]; ok {
		return a, nil
	}
	return c, nil
}

// Translate implements translator.Translator.
func (t *Translator) Translate(ctx context.Context, req *translator.Request) ([]string, error) {
	from, err := t.code(req.Source)
	if err != nil {
		return nil, err
	}
	to, err := t.code(req.Target)
	if err != nil {
		return nil, err
	}
	return translator.PerCue(ctx, req.Lines, workers, func(ctx context.Context, text string) (string, error) {
		if n := len([]rune(text)); t.spec.maxChars > 0 && n > t.spec.maxChars {
			return "", provider.Errorf(provider.ConfigurationError, "%s translates at most %d characters, not %d", t.spec.display, t.spec.maxChars, n)
		}
		return t.svc.text(ctx, from, to, text)
	})
}

// do sends a request, returning the answer when it succeeded.
func do(env provider.Env, display string, req *http.Request) (*http.Response, error) {
	resp, err := env.HTTP.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return nil, fmt.Errorf("%s: %w", display, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		return nil, translator.StatusError(display, resp)
	}
	return resp, nil
}

// Definitions returns the translators' definitions.
func Definitions() []translator.Definition {
	return []translator.Definition{googleSpec().definition(), bingSpec().definition(), microsoftSpec().definition(), yandexSpec().definition()}
}

// newGUID returns a random GUID as .NET's ToString("N"): 32 hexadecimal
// digits.
func newGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return hex.EncodeToString(b)
}
