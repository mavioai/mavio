package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxBody bounds the responses of clients read.
const maxBody = 64 << 20

// Response is a client's answer.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Send sends a request to a client and reads the answer; failures to
// reach the client are UnavailableErrors.
func Send(hc *http.Client, req *http.Request) (*Response, error) {
	resp, err := hc.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return nil, &UnavailableError{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, &UnavailableError{Err: err}
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}, nil
}

// Err returns nil for a successful answer, an AuthError for 401 and 403,
// and an HTTPError otherwise.
func (r *Response) Err() error {
	switch {
	case r.StatusCode >= 200 && r.StatusCode < 300:
		return nil
	case r.StatusCode == http.StatusUnauthorized || r.StatusCode == http.StatusForbidden:
		return &AuthError{Err: &HTTPError{StatusCode: r.StatusCode, Body: snippet(r.Body)}}
	}
	return &HTTPError{StatusCode: r.StatusCode, Body: snippet(r.Body)}
}

// HTTPError is a client answering with a failure status.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// NoRedirect returns hc not following redirects, for clients whose
// requests Radarr sends with AllowAutoRedirect off.
func NoRedirect(hc *http.Client) *http.Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}
