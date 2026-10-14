// Package lingarr is the translator of Lingarr, ported from Bazarr's
// LingarrTranslatorService: a subtitle's cues go at once to a Lingarr
// server's content translation API, which translates them with the
// service it is set up with.
package lingarr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// Name is the translator's name.
const Name = "lingarr"

const (
	defaultURL = "http://lingarr:9876"
	// timeout bounds a translation, longer than Lingarr's own 30 minutes
	// so that its failures are reported.
	timeout = 32 * time.Minute
	// tries is how many times a translation is requested, waiting 1 and 2
	// seconds and up to a second more between tries, as Bazarr's
	// retry(tries=3, delay=1, backoff=2, jitter=(0, 1)).
	tries = 3
)

// convert are the codes Lingarr takes for languages, as Bazarr's
// language_code_convert_dict.
var convert = map[string]string{"zh": "zh-CN", "zt": "zh-TW", "pb": "pt-BR"}

// Definition returns the translator's definition.
func Definition() translator.Definition {
	return translator.Definition{
		Name: Name, DisplayName: "Lingarr",
		Settings: []provider.Setting{
			{Name: "url", Label: "Lingarr URL", Type: provider.Text, Default: defaultURL},
			{Name: "token", Label: "API key", Help: "Lingarr's API key, when its authentication is enabled.", Type: provider.Password},
		},
		New: New,
	}
}

type settings struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// New makes the translator from its settings.
func New(env provider.Env, raw json.RawMessage) (translator.Translator, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.URL = strings.TrimSuffix(strings.TrimSpace(s.URL), "/"); s.URL == "" {
		s.URL = defaultURL
	}
	if u, err := url.Parse(s.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "invalid URL %q", s.URL)
	}
	return &Translator{env: env, s: s, sleep: site.Sleep}, nil
}

// Translator is Lingarr.
type Translator struct {
	env   provider.Env
	s     settings
	sleep func(context.Context, time.Duration) error
}

// SetSleep replaces how the translator waits between tries, for tests.
func (t *Translator) SetSleep(f func(context.Context, time.Duration) error) { t.sleep = f }

// Credit implements translator.Translator.
func (*Translator) Credit() string { return "Lingarr" }

// BatchSize implements translator.Translator.
func (*Translator) BatchSize() int { return 0 }

// line is a cue as Lingarr takes and gives it.
type line struct {
	Position *int    `json:"position"`
	Line     *string `json:"line"`
}

// request is a translation as Lingarr's content API takes it.
type request struct {
	ArrMediaID     int    `json:"arrMediaId"`
	Title          string `json:"title"`
	SourceLanguage string `json:"sourceLanguage"`
	TargetLanguage string `json:"targetLanguage"`
	MediaType      string `json:"mediaType"`
	Lines          []line `json:"lines"`
}

// Translate implements translator.Translator; blank cues are not sent.
func (t *Translator) Translate(ctx context.Context, req *translator.Request) ([]string, error) {
	out := make([]string, len(req.Lines))
	body := request{
		Title:          title(req.Media),
		SourceLanguage: translator.Code(req.Source, convert),
		TargetLanguage: translator.Code(req.Target, convert),
		MediaType:      "Movie",
		Lines:          []line{},
	}
	if req.Media.Episode {
		body.MediaType = "Episode"
	}
	for i, l := range req.Lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		body.Lines = append(body.Lines, line{Position: new(i), Line: new(l)})
	}
	if len(body.Lines) == 0 {
		return out, nil
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	wait := time.Second
	var got []line
	for try := 1; ; try++ {
		got, err = t.request(ctx, payload)
		if err == nil || !errors.Is(err, errRetry) || try == tries {
			break
		}
		if err := t.sleep(ctx, wait+rand.N(time.Second)); err != nil {
			return nil, err
		}
		wait *= 2
	}
	if err != nil {
		return nil, err
	}
	for _, l := range got {
		if p := *l.Position; p >= 0 && p < len(out) {
			out[p] = *l.Line
		}
	}
	return out, nil
}

// title is what Lingarr shows the translation as: the title and the
// subtitle's file name, as Bazarr's.
func title(m translator.Media) string {
	name := m.SourceFileName
	if name == "" {
		name = "subtitle.srt"
	}
	if strings.TrimSpace(m.Title) == "" {
		return name
	}
	return m.Title + " · " + name
}

// errRetry is a failure the translation is requested again for.
var errRetry = errors.New("retry")

func (t *Translator) request(ctx context.Context, payload []byte) ([]line, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.s.URL+"/api/translate/content", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if t.s.Token != "" {
		req.Header.Set("X-Api-Key", t.s.Token)
	}
	resp, err := t.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%w: connection error: %w", errRetry, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRetry, err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, provider.Errorf(provider.AuthenticationError, "%w: authentication failed: invalid or missing API key", errRetry)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, provider.Errorf(provider.TooManyRequests, "%w: rate limit exceeded", errRetry)
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: server error: %d", errRetry, resp.StatusCode)
	default:
		return nil, fmt.Errorf("lingarr API error: %s: %s", resp.Status, bytes.TrimSpace(data))
	}
	var lines []line
	if err := json.Unmarshal(data, &lines); err != nil {
		return nil, fmt.Errorf("unexpected response format from Lingarr API: %w", err)
	}
	for _, l := range lines {
		if l.Position == nil || l.Line == nil {
			return nil, errors.New("invalid response format from Lingarr API")
		}
	}
	return lines, nil
}
