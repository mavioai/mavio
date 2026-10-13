package web

import (
	"net/http"
	"time"
)

// Config is how providers reach their sites.
type Config struct {
	// Proxy is the proxy requests go through; nil for none.
	Proxy *Proxy `json:"proxy,omitempty"`
	// FlareSolverr solves Cloudflare's challenges; nil for none.
	FlareSolverr *FlareSolverr `json:"flaresolverr,omitempty"`
}

// NewClient returns the client providers reach their sites with,
// through the proxy and solving challenges with FlareSolverr; requests
// time out after timeout.
func NewClient(c Config, timeout time.Duration) *http.Client {
	var rt http.RoundTripper = newTransport(c.Proxy)
	if c.FlareSolverr != nil && c.FlareSolverr.URL != "" {
		rt = &cloudflare{next: rt, solver: c.FlareSolverr, proxy: c.Proxy, direct: newTransport(nil)}
	}
	return &http.Client{Transport: rt, Timeout: timeout}
}
