package translator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// StatusError returns the failure of a service's answer of an unsuccessful
// status: too many requests and quotas, which the server retries or not,
// rejected keys, and the rest.
func StatusError(service string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := strings.TrimSpace(string(body))
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return provider.Errorf(provider.TooManyRequests, "%s: too many requests", service)
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return provider.Errorf(provider.ServiceUnavailable, "%s: %s", service, resp.Status)
	case http.StatusUnauthorized, http.StatusForbidden:
		return provider.Errorf(provider.AuthenticationError, "%s: %s: %s", service, resp.Status, msg)
	case 456: // DeepL's quota exceeded.
		return provider.Errorf(provider.SearchLimitReached, "%s: quota exceeded", service)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return provider.Errorf(provider.ConfigurationError, "%s: %s: %s", service, resp.Status, msg)
	}
	return fmt.Errorf("%s: %s: %s", service, resp.Status, msg)
}

// PerCue translates a request's lines a cue at a time, workers at once,
// for services translating a text per request; blank lines are not sent.
// Any cue failing fails them all, as Lingarr's.
func PerCue(ctx context.Context, lines []string, workers int, translate func(ctx context.Context, text string) (string, error)) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]string, len(lines))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var once sync.Once
	var failed error
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Go(func() {
			defer func() { <-sem }()
			got, err := translate(ctx, line)
			if err != nil {
				once.Do(func() { failed = err; cancel() })
				return
			}
			out[i] = got
		})
	}
	wg.Wait()
	if failed != nil {
		return nil, failed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LanguageCache keeps the languages a service lists for a day, as
// Lingarr's.
type LanguageCache struct {
	mu             sync.Mutex
	source, target []Language
	until          time.Time
}

// Get returns the languages, listing them again with list once a day
// passed.
func (c *LanguageCache) Get(ctx context.Context, now time.Time, list func(context.Context) (source, target []Language, err error)) (source, target []Language, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.source != nil && now.Before(c.until) {
		return c.source, c.target, nil
	}
	source, target, err = list(ctx)
	if err != nil {
		return nil, nil, err
	}
	c.source, c.target, c.until = source, target, now.Add(24*time.Hour)
	return source, target, nil
}

// Texts returns the lines with text and their indexes, what services
// translating several texts at once are sent.
func Texts(lines []string) (texts []string, indexes []int) {
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			texts, indexes = append(texts, l), append(indexes, i)
		}
	}
	return texts, indexes
}
