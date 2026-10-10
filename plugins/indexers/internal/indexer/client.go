package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

// DefaultUserAgent is the User-Agent of requests that set none.
const DefaultUserAgent = "Mavio"

const (
	defaultTimeout = 100 * time.Second
	maxBodySize    = 64 << 20
	maxRedirects   = 5
)

// Client sends the requests of all indexers: it spaces requests to each
// host by their rate limits, as Prowlarr's RateLimitService does, sends
// them through proxies and solves challenges with FlareSolverr.
type Client struct {
	// Transport replaces the transports through proxies, for tests.
	Transport http.RoundTripper
	UserAgent string

	mu         sync.Mutex
	next       map[string]time.Time
	transports map[string]http.RoundTripper
	// User agents FlareSolverr solved hosts' challenges with.
	solvedAgents map[string]string
}

// NewClient returns a client.
func NewClient() *Client {
	return &Client{UserAgent: DefaultUserAgent}
}

// Session returns a session sending requests through the proxies given.
func (c *Client) Session(proxy, solver *Proxy) *Session {
	return &Session{client: c, proxy: proxy, solver: solver}
}

// wait waits until the host may be sent another request, at least
// interval after the last.
func (c *Client) wait(ctx context.Context, host string, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	c.mu.Lock()
	if c.next == nil {
		c.next = map[string]time.Time{}
	}
	now := time.Now()
	at := now
	if t := c.next[host]; t.After(now) {
		at = t
	}
	c.next[host] = at.Add(interval)
	c.mu.Unlock()
	return sleep(ctx, at.Sub(now))
}

func (c *Client) transport(p *Proxy) http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	key := ""
	if u := p.URL(); u != nil {
		key = u.String()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transports == nil {
		c.transports = map[string]http.RoundTripper{}
	}
	t := c.transports[key]
	if t == nil {
		t = newTransport(p)
		c.transports[key] = t
	}
	return t
}

func (c *Client) solvedAgent(host string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.solvedAgents[host]
}

func (c *Client) setSolvedAgent(host, agent string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.solvedAgents == nil {
		c.solvedAgents = map[string]string{}
	}
	c.solvedAgents[host] = agent
}

// Session sends an indexer's requests.
type Session struct {
	client        *Client
	proxy, solver *Proxy
	// Encoding decodes responses whose requests set none; the response's
	// charset, else UTF-8, when nil.
	Encoding encoding.Encoding
	// RateLimit is the indexer's minimum time between requests.
	RateLimit time.Duration
}

// Do sends a request, following redirects when it asks to, and solving
// Cloudflare's challenges when the indexer has a FlareSolverr. Error
// statuses are returned as responses.
func (s *Session) Do(ctx context.Context, r *Request) (*Response, error) {
	req := *r
	req.Cookies = maps.Clone(r.Cookies)
	if req.Cookies == nil {
		req.Cookies = map[string]string{}
	}
	received := map[string]string{}
	for range maxRedirects + 1 {
		resp, err := s.do(ctx, &req)
		if err != nil {
			return nil, err
		}
		maps.Copy(received, resp.Cookies)
		maps.Copy(req.Cookies, resp.Cookies)
		if !r.FollowRedirects || !resp.IsRedirect() {
			resp.Request = r
			resp.Cookies = received
			return resp, nil
		}
		next := resp.RedirectURL()
		if next == "" {
			return nil, fmt.Errorf("%s redirected without a location", req.URL)
		}
		if resp.StatusCode != http.StatusTemporaryRedirect && resp.StatusCode != http.StatusPermanentRedirect {
			req.Method, req.Body = http.MethodGet, nil
		}
		req.URL = next
	}
	return nil, fmt.Errorf("%s redirected more than %d times", r.URL, maxRedirects)
}

// do sends a request once, solving a challenge if one answers.
func (s *Session) do(ctx context.Context, r *Request) (*Response, error) {
	resp, err := s.send(ctx, r, "")
	if err != nil || s.solver == nil || !IsCloudflareProtected(resp) {
		return resp, err
	}
	solution, err := s.solve(ctx, r)
	if err != nil {
		return nil, err
	}
	host := hostOf(r.URL)
	s.client.setSolvedAgent(host, solution.UserAgent)
	maps.Copy(r.Cookies, solution.cookies())
	return s.send(ctx, r, solution.UserAgent)
}

func (s *Session) send(ctx context.Context, r *Request, agent string) (*Response, error) {
	host := hostOf(r.URL)
	if err := s.client.wait(ctx, host, max(r.RateLimit, s.RateLimit)); err != nil {
		return nil, err
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if b := r.bodyReader(); b != nil {
		body = b
	}
	req, err := http.NewRequestWithContext(ctx, method, r.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range r.Header {
		req.Header[k] = slices.Clone(v)
	}
	if agent == "" && req.Header.Get("User-Agent") == "" && s.solver != nil {
		agent = s.client.solvedAgent(host)
	}
	switch {
	case agent != "":
		req.Header.Set("User-Agent", agent)
	case req.Header.Get("User-Agent") == "":
		req.Header.Set("User-Agent", s.client.UserAgent)
	}
	if len(r.Cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(r.Cookies))
	}

	hc := &http.Client{
		Transport:     s.client.transport(s.proxy),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", r.URL, err)
	}
	resp := &Response{
		Request:    r,
		URL:        r.URL,
		StatusCode: res.StatusCode,
		Header:     res.Header,
		Body:       data,
		Cookies:    map[string]string{},
		encoding:   r.Encoding,
	}
	if resp.encoding == nil {
		resp.encoding = s.Encoding
	}
	if resp.encoding == nil {
		resp.encoding = charset(res.Header.Get("Content-Type"))
	}
	for _, c := range res.Cookies() {
		if c.MaxAge >= 0 && c.Value != "" {
			resp.Cookies[c.Name] = c.Value
		}
	}
	return resp, nil
}

// cookieHeader returns the Cookie header of cookies, in name order.
func cookieHeader(cookies map[string]string) string {
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(cookies)) {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(cookies[k])
	}
	return b.String()
}

// charset returns the encoding a Content-Type names, nil for none or
// UTF-8.
func charset(contentType string) encoding.Encoding {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["charset"] == "" {
		return nil
	}
	e, err := htmlindex.Get(params["charset"])
	if err != nil || e == encoding.Nop {
		return nil
	}
	if name, _ := htmlindex.Name(e); name == "utf-8" {
		return nil
	}
	return e
}

func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Host
	}
	return ""
}

// sleep waits for d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// solution is FlareSolverr's solution of a challenge.
type solution struct {
	UserAgent string `json:"userAgent"`
	Cookies   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"cookies"`
}

func (s *solution) cookies() map[string]string {
	m := map[string]string{}
	for _, c := range s.Cookies {
		m[c.Name] = c.Value
	}
	return m
}

// ErrFlareSolverr is returned when FlareSolverr fails to solve a challenge.
var ErrFlareSolverr = errors.New("FlareSolverr failed")

// solve asks FlareSolverr to solve the challenge answering a request, as
// Prowlarr's FlareSolverr proxy does.
func (s *Session) solve(ctx context.Context, r *Request) (*solution, error) {
	timeout := s.solver.RequestTimeout
	if timeout <= 0 {
		timeout = 60
	}
	cmd := map[string]any{"url": r.URL, "maxTimeout": timeout * 1000}
	if u := s.proxy.URL(); u != nil {
		u.User = nil
		p := map[string]any{"url": u.String()}
		if s.proxy.Username != "" {
			p["username"], p["password"] = s.proxy.Username, s.proxy.Password
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
		cmd["postData"] = string(r.Body)
		cmd["headers"] = map[string]any{"contentType": ct}
	default:
		return nil, fmt.Errorf("%w: unsupported method %s", ErrFlareSolverr, r.Method)
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	req := &Request{
		Method:  http.MethodPost,
		URL:     strings.TrimRight(s.solver.Host, "/") + "/v1",
		Header:  http.Header{"Content-Type": {"application/json"}, "Accept": {AcceptJSON}},
		Body:    body,
		Timeout: time.Duration(timeout+5) * time.Second,
	}
	direct := &Session{client: s.client}
	resp, err := direct.send(ctx, req, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFlareSolverr, err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFlareSolverr, resp.StatusCode)
	}
	var result struct {
		Message  string    `json:"message"`
		Solution *solution `json:"solution"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFlareSolverr, err)
	}
	if result.Solution == nil || len(result.Solution.Cookies) == 0 {
		return nil, fmt.Errorf("%w: no cookies returned: %s", ErrFlareSolverr, result.Message)
	}
	return result.Solution, nil
}
