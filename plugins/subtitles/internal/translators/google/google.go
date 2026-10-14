// Package google is the translator of Google Translate, ported from
// Bazarr's GoogleTranslatorService and deep_translator's GoogleTranslator:
// cues are translated one at a time, ten at once, through the free page
// of Google Translate for phones.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// Name is the translator's name.
const Name = "google_translate"

const (
	// workers is how many cues are translated at once.
	workers = 10
	// tries is how many times a cue is requested, waiting 1, 2, 4, 8 and
	// 16 seconds and up to a second more between tries, as Bazarr's
	// retry(tries=6, delay=1, backoff=2, jitter=(0, 1)).
	tries = 6
	// maxChars bounds the length of a cue, as deep_translator's.
	maxChars = 5000
)

// convert are the codes Google Translate takes for languages, as Bazarr's
// language_code_convert_dict.
var convert = map[string]string{"he": "iw", "zh": "zh-CN", "zt": "zh-TW"}

// Definition returns the translator's definition.
func Definition() translator.Definition {
	return translator.Definition{Name: Name, DisplayName: "Google Translate", New: New}
}

// New makes the translator.
func New(env provider.Env, _ json.RawMessage) (translator.Translator, error) {
	return &Translator{env: env, base: "https://translate.google.com/m", sleep: site.Sleep}, nil
}

// Translator is Google Translate.
type Translator struct {
	env   provider.Env
	base  string
	sleep func(context.Context, time.Duration) error
}

// SetBaseURL points the translator at another server, for tests.
func (t *Translator) SetBaseURL(u string) { t.base = u }

// SetSleep replaces how the translator waits between tries, for tests.
func (t *Translator) SetSleep(f func(context.Context, time.Duration) error) { t.sleep = f }

// Credit implements translator.Translator.
func (*Translator) Credit() string { return "Google Translate" }

// BatchSize implements translator.Translator.
func (*Translator) BatchSize() int { return 0 }

// Translate implements translator.Translator. As Bazarr's, any cue failing
// fails the translation, so that none is saved partly translated.
func (t *Translator) Translate(ctx context.Context, req *translator.Request) ([]string, error) {
	target := translator.Code(req.Target, convert)
	out := make([]string, len(req.Lines))
	errs := make([]error, len(req.Lines))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i, line := range req.Lines {
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			defer func() { <-sem }()
			out[i], errs[i] = t.line(ctx, target, line)
		})
	}
	wg.Wait()
	var failed []error
	for _, err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	if len(failed) > 0 {
		return nil, fmt.Errorf("google translation failed for %d/%d line(s): %w", len(failed), len(req.Lines), failed[0])
	}
	return out, nil
}

// errRetry is a failure the cue is requested again for.
var errRetry = errors.New("retry")

// line translates a cue, trying again for too many requests and failed
// requests.
func (t *Translator) line(ctx context.Context, target, text string) (string, error) {
	if len(text) >= maxChars {
		return "", fmt.Errorf("%d characters, at most %d are translated", len(text), maxChars-1)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	wait := time.Second
	for try := 1; ; try++ {
		got, err := t.request(ctx, target, text)
		if err == nil || !errors.Is(err, errRetry) || try == tries {
			return got, err
		}
		if err := t.sleep(ctx, wait+rand.N(time.Second)); err != nil {
			return "", err
		}
		wait *= 2
	}
}

// request asks for a cue's translation; "" when it is the cue's text,
// which had nothing to translate.
func (t *Translator) request(ctx context.Context, target, text string) (string, error) {
	q := url.Values{"tl": {target}, "sl": {"auto"}, "q": {text}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.base+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", site.RandomUserAgent())
	resp, err := t.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%w: %w", errRetry, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", provider.Errorf(provider.TooManyRequests, "%w: too many requests", errRetry)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("%w: request failed with %s", errRetry, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errRetry, err)
	}
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errRetry, err)
	}
	n := find(doc, "t0")
	if n == nil {
		n = find(doc, "result-container")
	}
	if n == nil {
		// deep_translator's TranslationNotFound, which Bazarr keeps the
		// cue for.
		return "", nil
	}
	got := strippedText(n)
	if got == text {
		return "", nil
	}
	if looksLikeError(got) {
		return "", fmt.Errorf("%w: invalid response %q", errRetry, got)
	}
	return got, nil
}

// find returns the first div of a class.
func find(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && n.Data == "div" {
		for _, a := range n.Attr {
			if a.Key == "class" && strings.Contains(" "+a.Val+" ", " "+class+" ") {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, class); f != nil {
			return f
		}
	}
	return nil
}

// strippedText returns a node's text as BeautifulSoup's
// get_text(strip=True): its strings stripped and joined.
func strippedText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(strings.TrimSpace(n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

var (
	htmlTag   = regexp.MustCompile(`(?i)<\s*(html|body|div|br|p|DOCTYPE)[^>]*>`)
	errorCode = regexp.MustCompile(`\b(error|http)[ _-]?\d{3}\b`)
	// errorMarkers are the texts of the error pages the free endpoint may
	// answer with instead of a translation.
	errorMarkers = []string{
		"too many requests", "server error", "internal server error", "service unavailable", "bad gateway",
		"temporarily unavailable", "captcha", "unusual traffic", "rate limit",
	}
)

// looksLikeError reports whether a translation is an error or captcha
// page Google answered with, as Bazarr's _looks_like_error_response.
func looksLikeError(s string) bool {
	if htmlTag.MatchString(s) {
		return true
	}
	lower := strings.ToLower(s)
	for _, m := range errorMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return errorCode.MatchString(lower)
}
