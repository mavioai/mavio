package indexer

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrAuth is returned when the indexer rejects the credentials.
	ErrAuth = errors.New("invalid credentials")
	// ErrUnsupportedFeed is returned for feeds the parser cannot read.
	ErrUnsupportedFeed = errors.New("unsupported feed")
	// ErrCloudflare is returned when Cloudflare or DDoS-Guard blocks the
	// request; a FlareSolverr proxy gets past them.
	ErrCloudflare = errors.New("blocked by Cloudflare protection, FlareSolverr may be required")
	// ErrReleaseUnavailable is returned when a release's file is gone.
	ErrReleaseUnavailable = errors.New("release no longer available")
	// ErrDisabled is returned for indexers left out after failing.
	ErrDisabled = errors.New("indexer disabled after failing")
	// ErrLimitReached is returned for indexers that reached their query
	// or grab limit.
	ErrLimitReached = errors.New("limit reached")
)

// HTTPError is an error status from an indexer.
type HTTPError struct {
	Response *Response
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: HTTP %d", e.Response.Request.URL, e.Response.StatusCode)
}

// ServerError reports whether the indexer's server failed.
func (e *HTTPError) ServerError() bool {
	return e.Response.StatusCode >= 500
}

// RateLimitError is returned when the indexer limits requests: HTTP 429
// or a limit it reports itself.
type RateLimitError struct {
	Message string
	// When to try again; zero when the indexer does not tell.
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.Message == "" {
		return "request limit reached"
	}
	return "request limit reached: " + e.Message
}

// Error is an error the indexer reports in a response.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// Errorf returns an Error.
func Errorf(format string, args ...any) error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}
