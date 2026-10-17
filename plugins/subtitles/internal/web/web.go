// Package web is how providers reach their sites: through one of the
// server's proxies, solving Cloudflare's and DDoS-Guard's challenges
// through the server.
package web

import (
	"net/http"
	"time"

	"github.com/mavioai/mavio/libs/plugin/challenge"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// DefaultExclude are the hosts reached directly when a configuration
// lists none, as Bazarr's default.
var DefaultExclude = []string{"localhost", "127.0.0.1"}

// Config is how providers reach their sites.
type Config struct {
	// Proxy is the server's proxy requests go through; nil for none.
	Proxy *pluginv1.Proxy
	// Exclude are the hosts reached directly, as NO_PROXY lists them;
	// DefaultExclude when nil.
	Exclude []string
	// Solver solves challenges; nil leaves them unsolved.
	Solver challenge.Solver
}

// NewClient returns the client providers reach their sites with, through
// the proxy and solving challenges with the solver; requests time out
// after timeout.
func NewClient(c Config, timeout time.Duration) *http.Client {
	exclude := c.Exclude
	if exclude == nil {
		exclude = DefaultExclude
	}
	var rt http.RoundTripper = proxy.Transport(c.Proxy, proxy.Bypass(exclude))
	if c.Solver != nil {
		rt = &challenge.Transport{Next: rt, Solver: c.Solver, ProxyID: c.Proxy.GetId()}
	}
	return &http.Client{Transport: rt, Timeout: timeout}
}
