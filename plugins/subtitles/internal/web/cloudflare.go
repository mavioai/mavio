package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FlareSolverr is a FlareSolverr server, which solves Cloudflare's
// challenges in a browser.
type FlareSolverr struct {
	// URL is the server's, such as "http://localhost:8191".
	URL string `json:"url"`
	// RequestTimeout is how long a challenge may take, in seconds; 60
	// when 0.
	RequestTimeout int `json:"request_timeout,omitempty"`
}

// maxBody bounds the pages read to look for a challenge.
const maxBody = 64 << 20

// ErrFlareSolverr is a challenge FlareSolverr did not solve.
var ErrFlareSolverr = errors.New("FlareSolverr failed")

// IsCloudflareProtected reports whether Cloudflare or DDoS-Guard answered
// instead of the site, as Prowlarr's CloudFlareDetectionService.
func IsCloudflareProtected(status int, header http.Header, body []byte) bool {
	switch strings.ToLower(header.Get("Server")) {
	case "cloudflare", "cloudflare-nginx", "ddos-guard":
	default:
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

// solution is FlareSolverr's solution of a host's challenge: the cookies
// and the user agent requests to it pass with.
type solution struct {
	UserAgent string `json:"userAgent"`
	Cookies   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"cookies"`
}

// cloudflare solves the challenges answering requests with FlareSolverr,
// sending the host's later requests with the solution.
type cloudflare struct {
	next   http.RoundTripper
	solver *FlareSolverr
	proxy  *Proxy
	// direct reaches FlareSolverr.
	direct http.RoundTripper

	mu     sync.Mutex
	solved map[string]*solution
}

func (c *cloudflare) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		var err error
		if body, err = io.ReadAll(r.Body); err != nil {
			return nil, err
		}
		_ = r.Body.Close()
	}
	resp, err := c.next.RoundTrip(c.with(r, body))
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(resp.Header.Get("Server")) {
	case "cloudflare", "cloudflare-nginx", "ddos-guard":
	default:
		return resp, nil
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(page))
	if !IsCloudflareProtected(resp.StatusCode, resp.Header, page) {
		return resp, nil
	}
	s, err := c.solve(r, body)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.solved == nil {
		c.solved = map[string]*solution{}
	}
	c.solved[r.URL.Host] = s
	c.mu.Unlock()
	return c.next.RoundTrip(c.with(r, body))
}

// with returns a request with its body and the solution of its host's
// challenge.
func (c *cloudflare) with(r *http.Request, body []byte) *http.Request {
	r2 := r.Clone(r.Context())
	if body != nil {
		r2.Body = io.NopCloser(bytes.NewReader(body))
		r2.ContentLength = int64(len(body))
	}
	c.mu.Lock()
	s := c.solved[r.URL.Host]
	c.mu.Unlock()
	if s == nil {
		return r2
	}
	r2.Header.Set("User-Agent", s.UserAgent)
	for _, ck := range s.Cookies {
		r2.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
	}
	return r2
}

// solve asks FlareSolverr to solve the challenge answering a request, as
// Prowlarr's FlareSolverr proxy does.
func (c *cloudflare) solve(r *http.Request, body []byte) (*solution, error) {
	timeout := c.solver.RequestTimeout
	if timeout <= 0 {
		timeout = 60
	}
	cmd := map[string]any{"url": r.URL.String(), "maxTimeout": timeout * 1000}
	if c.proxy != nil && !c.proxy.excluded(r.URL.Host) {
		u := c.proxy.URL()
		u.User = nil
		p := map[string]any{"url": u.String()}
		if c.proxy.Username != "" {
			p["username"], p["password"] = c.proxy.Username, c.proxy.Password
		}
		cmd["proxy"] = p
	}
	switch r.Method {
	case "", http.MethodGet:
		cmd["cmd"] = "request.get"
	case http.MethodPost:
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "application/x-www-form-urlencoded") {
			return nil, fmt.Errorf("%w: unsupported POST content type %q", ErrFlareSolverr, ct)
		}
		cmd["cmd"] = "request.post"
		cmd["postData"] = string(body)
	default:
		return nil, fmt.Errorf("%w: unsupported method %s", ErrFlareSolverr, r.Method)
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout+5)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.solver.URL, "/")+"/v1", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.direct.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFlareSolverr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFlareSolverr, resp.StatusCode)
	}
	var result struct {
		Message  string    `json:"message"`
		Solution *solution `json:"solution"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFlareSolverr, err)
	}
	if result.Solution == nil || len(result.Solution.Cookies) == 0 {
		return nil, fmt.Errorf("%w: no cookies returned: %s", ErrFlareSolverr, result.Message)
	}
	return result.Solution, nil
}
