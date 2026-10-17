package flaresolverr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client speaks FlareSolverr's v1 API.
type Client struct {
	// URL is FlareSolverr's base URL, e.g. http://127.0.0.1:8191.
	URL  string
	HTTP *http.Client
}

// Proxy is a proxy as FlareSolverr takes it: its URL with the scheme
// (http, socks4 or socks5), and its credentials, which only sessions take.
type Proxy struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Cookie is a cookie FlareSolverr returns.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// Solution is what passed a challenge.
type Solution struct {
	URL       string   `json:"url"`
	Status    int      `json:"status"`
	Cookies   []Cookie `json:"cookies"`
	UserAgent string   `json:"userAgent"`
}

type command struct {
	Cmd               string `json:"cmd"`
	Session           string `json:"session,omitempty"`
	URL               string `json:"url,omitempty"`
	PostData          string `json:"postData,omitempty"`
	MaxTimeout        int64  `json:"maxTimeout,omitempty"`
	ReturnOnlyCookies bool   `json:"returnOnlyCookies,omitempty"`
	DisableMedia      bool   `json:"disableMedia,omitempty"`
	Proxy             *Proxy `json:"proxy,omitempty"`
}

type answer struct {
	Status   string    `json:"status"`
	Message  string    `json:"message"`
	Solution *Solution `json:"solution"`
	Sessions []string  `json:"sessions"`
}

// Error is FlareSolverr's error answer.
type Error struct{ Message string }

func (e *Error) Error() string { return "FlareSolverr: " + e.Message }

// do sends a command; FlareSolverr answers errors with HTTP 500 and a
// message.
func (c *Client) do(ctx context.Context, cmd command) (answer, error) {
	body, err := json.Marshal(cmd)
	if err != nil {
		return answer{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.URL, "/")+"/v1", bytes.NewReader(body))
	if err != nil {
		return answer{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return answer{}, fmt.Errorf("FlareSolverr %s: %w", cmd.Cmd, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return answer{}, fmt.Errorf("FlareSolverr %s: %w", cmd.Cmd, err)
	}
	var a answer
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return answer{}, fmt.Errorf("FlareSolverr %s: HTTP %d", cmd.Cmd, resp.StatusCode)
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return answer{}, fmt.Errorf("FlareSolverr %s: HTTP %d: %w", cmd.Cmd, resp.StatusCode, err)
	}
	if a.Status != "ok" {
		msg := a.Message
		if msg == "" {
			msg = fmt.Sprintf("%s failed with HTTP %d", cmd.Cmd, resp.StatusCode)
		}
		return answer{}, &Error{Message: msg}
	}
	return a, nil
}

// CreateSession creates a session with its own browser, through the proxy
// when not nil.
func (c *Client) CreateSession(ctx context.Context, name string, p *Proxy) error {
	_, err := c.do(ctx, command{Cmd: "sessions.create", Session: name, Proxy: p})
	return err
}

// DestroySession closes a session's browser.
func (c *Client) DestroySession(ctx context.Context, name string) error {
	_, err := c.do(ctx, command{Cmd: "sessions.destroy", Session: name})
	return err
}

// ListSessions lists the sessions.
func (c *Client) ListSessions(ctx context.Context) ([]string, error) {
	a, err := c.do(ctx, command{Cmd: "sessions.list"})
	return a.Sessions, err
}

// Request is a page to open.
type Request struct {
	URL string
	// PostData is a form-encoded body to POST; empty GETs.
	PostData string
	Post     bool
	// MaxTimeout is in milliseconds.
	MaxTimeout   int64
	DisableMedia bool
}

// Get opens a page in a session, returning only its cookies and the user
// agent.
func (c *Client) Get(ctx context.Context, session string, r Request) (Solution, error) {
	cmd := command{
		Cmd: "request.get", Session: session, URL: r.URL, MaxTimeout: r.MaxTimeout,
		ReturnOnlyCookies: true, DisableMedia: r.DisableMedia,
	}
	if r.Post {
		cmd.Cmd, cmd.PostData = "request.post", r.PostData
	}
	a, err := c.do(ctx, cmd)
	if err != nil {
		return Solution{}, err
	}
	if a.Solution == nil {
		return Solution{}, errors.New("FlareSolverr returned no solution")
	}
	return *a.Solution, nil
}
