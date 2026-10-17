package indexer

import (
	"context"
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

	"connectrpc.com/connect"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/mavioai/mavio/libs/plugin/challenge"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
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
// them through the server's proxies and has the server solve challenges.
type Client struct {
	// Transport replaces the transports through proxies, for tests.
	Transport http.RoundTripper
	UserAgent string

	mu sync.Mutex
	// next is when each host may be sent another request.
	next map[string]time.Time
	// transports are by proxy ID, "" for none.
	transports map[string]http.RoundTripper
	// solvedAgents are the user agents hosts' challenges were solved
	// with, by solvedKey.
	solvedAgents map[string]string
}

// NewClient returns a client.
func NewClient() *Client {
	return &Client{UserAgent: DefaultUserAgent}
}

// Session returns a session sending requests through the server's proxy
// p (nil for none) and solving challenges with solver (nil to leave them
// unsolved).
func (c *Client) Session(p *pluginv1.Proxy, solver challenge.Solver) *Session {
	return &Session{client: c, proxy: p, solver: solver}
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

// transport returns the transport through a proxy. A configuration
// replaces its plugin's proxies, sessions with them, and their
// transports.
func (c *Client) transport(p *pluginv1.Proxy) http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	key := p.GetId()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transports == nil {
		c.transports = map[string]http.RoundTripper{}
	}
	t := c.transports[key]
	if t == nil {
		t = proxy.Transport(p, proxy.Local)
		c.transports[key] = t
	}
	return t
}

// Forget drops the transports through proxies, for a configuration
// whose proxies may have changed.
func (c *Client) Forget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range c.transports {
		if t, ok := t.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
	}
	c.transports = nil
}

// solvedKey keys a host's solved agent: a solution holds only through the
// proxy it was solved through.
func solvedKey(host string, p *pluginv1.Proxy) string { return host + "\x00" + p.GetId() }

func (c *Client) solvedAgent(host string, p *pluginv1.Proxy) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.solvedAgents[solvedKey(host, p)]
}

func (c *Client) setSolvedAgent(host string, p *pluginv1.Proxy, agent string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.solvedAgents == nil {
		c.solvedAgents = map[string]string{}
	}
	c.solvedAgents[solvedKey(host, p)] = agent
}

// Session sends an indexer's requests.
type Session struct {
	client *Client
	proxy  *pluginv1.Proxy
	solver challenge.Solver
	// Encoding decodes responses whose requests set none; the response's
	// charset, else UTF-8, when nil.
	Encoding encoding.Encoding
	// RateLimit is the indexer's minimum time between requests.
	RateLimit time.Duration
}

// Do sends a request, following redirects when it asks to, and solving
// Cloudflare's and DDoS-Guard's challenges with the session's solver.
// Error statuses are returned as responses.
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

// do sends a request once, solving a challenge if one answers. Without a
// solver plugin, the challenge is returned as it is.
func (s *Session) do(ctx context.Context, r *Request) (*Response, error) {
	resp, err := s.send(ctx, r, "")
	if err != nil || s.solver == nil || !IsCloudflareProtected(resp) {
		return resp, err
	}
	sol, err := s.solve(ctx, r)
	if connect.CodeOf(err) == connect.CodeUnavailable {
		return resp, nil
	}
	if err != nil {
		return nil, err
	}
	s.client.setSolvedAgent(hostOf(r.URL), s.proxy, sol.GetUserAgent())
	for _, c := range sol.GetCookies() {
		r.Cookies[c.GetName()] = c.GetValue()
	}
	return s.send(ctx, r, sol.GetUserAgent())
}

// solve has the server solve the challenge answering a request, through
// the session's proxy.
func (s *Session) solve(ctx context.Context, r *Request) (*pluginv1.ChallengeSolution, error) {
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	return challenge.Solve(ctx, s.solver, req, r.Body, s.proxy.GetId(), 0)
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
		agent = s.client.solvedAgent(host, s.proxy)
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
