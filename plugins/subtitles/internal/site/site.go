// Package site is what the providers share to reach their sites: sessions
// with cookies, the sites' answers mapped to the failures Bazarr
// throttles by, and HTML pages queried with CSS selectors.
package site

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// UserAgent is the browser's the sites are reached as, as Bazarr's
// providers' that pass for one.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// MaxBody bounds an answer: a page, or a subtitle or archive.
const MaxBody = 64 << 20

// Session reaches a site, keeping its cookies.
type Session struct {
	http *http.Client
	// Header is sent with every request; the User-Agent is UserAgent
	// unless it sets one.
	Header http.Header
	// Check maps an answer to a failure; Status when nil.
	Check func(*Response) error
}

// New returns a session reaching a site with the plugin's client, with a
// cookie jar of its own.
func New(hc *http.Client) *Session {
	jar, _ := cookiejar.New(nil)
	c := *hc
	c.Jar = jar
	return &Session{http: &c, Header: http.Header{}}
}

// NewBrowser returns a session as New does that passes for a browser
// picked at random, as Bazarr's providers that pick a user agent at random
// for their sessions.
func NewBrowser(hc *http.Client) *Session {
	s := New(hc)
	s.Header.Set("User-Agent", RandomUserAgent())
	return s
}

// Jar returns the session's cookies.
func (s *Session) Jar() http.CookieJar { return s.http.Jar }

// Response is a site's answer, read whole.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	// URL is the URL answered, after redirects.
	URL *url.URL
}

// JSON decodes the answer.
func (r *Response) JSON(out any) error {
	if err := json.Unmarshal(r.Body, out); err != nil {
		return provider.Errorf(provider.ParseResponseError, "%s: %w", r.URL.Path, err)
	}
	return nil
}

// Status maps the status codes of an answer to failures, as Bazarr's
// providers mostly do: 429 is too many requests, 401 an authentication
// failure, 403 forbidden, 5xx the service unavailable, any other code but
// 2xx a provider error.
func Status(r *Response) error {
	switch c := r.Status; {
	case c >= 200 && c < 300:
		return nil
	case c == http.StatusTooManyRequests:
		return provider.Errorf(provider.TooManyRequests, "%s: too many requests", r.URL.Path)
	case c == http.StatusUnauthorized:
		return provider.Errorf(provider.AuthenticationError, "%s: unauthorized", r.URL.Path)
	case c == http.StatusForbidden:
		return provider.Errorf(provider.ForbiddenError, "%s: forbidden", r.URL.Path)
	case c >= 500:
		return provider.Errorf(provider.ServiceUnavailable, "%s: %d", r.URL.Path, c)
	}
	return provider.Errorf(provider.ProviderError, "%s: bad status code: %d", r.URL.Path, r.Status)
}

// Do sends a request and reads its answer, checked with Check. A network
// failure is the service unavailable, or a timeout.
func (s *Session) Do(req *http.Request) (*Response, error) {
	for k, v := range s.Header {
		if req.Header.Get(k) == "" {
			req.Header[k] = v
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, netError(req, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return nil, netError(req, err)
	}
	if len(body) > MaxBody {
		return nil, provider.Errorf(provider.ProviderError, "%s: larger than %d bytes", req.URL.Path, MaxBody)
	}
	r := &Response{Status: resp.StatusCode, Header: resp.Header, Body: body, URL: resp.Request.URL}
	check := s.Check
	if check == nil {
		check = Status
	}
	if err := check(r); err != nil {
		return r, err
	}
	return r, nil
}

func netError(req *http.Request, err error) error {
	if ctxErr := req.Context().Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return provider.Errorf(provider.Timeout, "%s: %w", req.URL.Host, err)
	}
	return provider.Errorf(provider.ServiceUnavailable, "%s: %w", req.URL.Host, err)
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// Get fetches a URL with a query.
func (s *Session) Get(ctx context.Context, u string, query url.Values) (*Response, error) {
	if len(query) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	return s.Do(req)
}

// PostForm posts a form.
func (s *Session) PostForm(ctx context.Context, u string, form url.Values) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.Do(req)
}

// PostJSON posts a JSON body.
func (s *Session) PostJSON(ctx context.Context, u string, body any) (*Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return s.Do(req)
}

// Retry calls f as Bazarr's ProviderRetryMixin: 3 times, waiting the
// environment's RetryWait between them.
func Retry[T any](ctx context.Context, env provider.Env, f func() (T, error)) (T, error) {
	return provider.Retry(ctx, 3, env.RetryWait, f)
}

// Sleep waits for d or until ctx is done, for the sites that ask for
// pauses between requests.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
