// Package challenge passes the challenges of sites behind Cloudflare or
// DDoS-Guard: it tells them apart and solves them through
// HostService.SolveChallenge, which the server passes on to the
// CHALLENGE_SOLVER plugins.
package challenge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ErrUnsolved is a challenge no solver solved.
var ErrUnsolved = errors.New("challenge not solved")

// maxBody bounds the pages read to look for a challenge.
const maxBody = 64 << 20

// Guarded reports whether Cloudflare or DDoS-Guard answered, by the
// Server header; only their answers may be challenges.
func Guarded(header http.Header) bool {
	switch strings.ToLower(header.Get("Server")) {
	case "cloudflare", "cloudflare-nginx", "ddos-guard":
		return true
	}
	return false
}

// IsProtected reports whether Cloudflare or DDoS-Guard answered with a
// challenge instead of the site, as Prowlarr's CloudFlareDetectionService.
func IsProtected(status int, header http.Header, body []byte) bool {
	if !Guarded(header) {
		return false
	}
	html := string(body)
	if status == http.StatusServiceUnavailable || status == http.StatusForbidden {
		for _, title := range []string{"<title>Just a moment...</title>", "<title>Access denied</title>", "<title>Attention Required! | Cloudflare</title>"} {
			if strings.Contains(html, title) {
				return true
			}
		}
		if strings.TrimSpace(html) == "error code: 1020" || strings.Contains(strings.ToLower(html), "<title>ddos-guard</title>") {
			return true
		}
	}
	return header.Get("Vary") == "Accept-Encoding,User-Agent" && header.Get("Content-Encoding") == "" &&
		strings.Contains(strings.ToLower(html), "ddos")
}

// Solver solves challenges; pluginv1connect.HostServiceClient is one.
type Solver interface {
	SolveChallenge(ctx context.Context, req *pluginv1.SolveChallengeRequest) (*pluginv1.SolveChallengeResponse, error)
}

var _ Solver = pluginv1connect.HostServiceClient(nil)

// Solve solves the challenge answering a request with body, through the
// server's proxy of the ID ("" for none), taking at most timeout (0 for
// the solver's own).
func Solve(ctx context.Context, s Solver, r *http.Request, body []byte, proxyID string, timeout time.Duration) (*pluginv1.ChallengeSolution, error) {
	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	c := pluginv1.Challenge_builder{
		Url: new(r.URL.String()), Method: &method, TimeoutSeconds: new(int32(timeout / time.Second)),
	}.Build()
	switch method {
	case http.MethodGet:
	case http.MethodPost:
		c.SetBody(body)
		c.SetContentType(r.Header.Get("Content-Type"))
	default:
		return nil, fmt.Errorf("%w: unsupported method %s", ErrUnsolved, method)
	}
	resp, err := s.SolveChallenge(ctx, pluginv1.SolveChallengeRequest_builder{Challenge: c, ProxyId: &proxyID}.Build())
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUnsolved, r.URL.Host, err)
	}
	sol := resp.GetSolution()
	if len(sol.GetCookies()) == 0 || sol.GetUserAgent() == "" {
		return nil, fmt.Errorf("%w: %s: no cookies or user agent", ErrUnsolved, r.URL.Host)
	}
	return sol, nil
}

// Apply makes a request send a solution's user agent and cookies, which
// replace the request's cookies of the same names.
func Apply(r *http.Request, s *pluginv1.ChallengeSolution) {
	if s == nil {
		return
	}
	r.Header.Set("User-Agent", s.GetUserAgent())
	names := map[string]bool{}
	for _, c := range s.GetCookies() {
		names[c.GetName()] = true
	}
	kept := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range kept {
		if !names[c.Name] {
			r.AddCookie(c)
		}
	}
	for _, c := range s.GetCookies() {
		r.AddCookie(&http.Cookie{Name: c.GetName(), Value: c.GetValue()})
	}
}

// Transport solves the challenges answering its requests, sending a
// host's later requests with the solution and retrying the challenged
// request once.
type Transport struct {
	// Next sends requests, through the proxy of ProxyID.
	Next http.RoundTripper
	// Solver solves challenges; nil returns them as they are.
	Solver Solver
	// ProxyID is the server's proxy Next goes through, "" for none.
	ProxyID string
	// Timeout bounds solving, 0 for the solver's own.
	Timeout time.Duration

	mu     sync.Mutex
	solved map[string]*pluginv1.ChallengeSolution
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil && r.Body != http.NoBody {
		var err error
		if body, err = io.ReadAll(r.Body); err != nil {
			return nil, err
		}
		_ = r.Body.Close()
	}
	resp, err := t.Next.RoundTrip(t.with(r, body))
	if err != nil || t.Solver == nil || !Guarded(resp.Header) {
		return resp, err
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(page))
	if !IsProtected(resp.StatusCode, resp.Header, page) {
		return resp, nil
	}
	s, err := Solve(r.Context(), t.Solver, r, body, t.ProxyID, t.Timeout)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	if t.solved == nil {
		t.solved = map[string]*pluginv1.ChallengeSolution{}
	}
	t.solved[r.URL.Host] = s
	t.mu.Unlock()
	return t.Next.RoundTrip(t.with(r, body))
}

// Solution returns the solution of a host's challenge, nil for none.
func (t *Transport) Solution(host string) *pluginv1.ChallengeSolution {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.solved[host]
}

// with returns a request with its body and its host's solution.
func (t *Transport) with(r *http.Request, body []byte) *http.Request {
	r2 := r.Clone(r.Context())
	if body != nil {
		r2.Body = io.NopCloser(bytes.NewReader(body))
		r2.ContentLength = int64(len(body))
	}
	Apply(r2, t.Solution(r.URL.Host))
	return r2
}
