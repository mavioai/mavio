package proxies

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// dialTimeout bounds connecting a tunnel, through a proxy or not.
const dialTimeout = 30 * time.Second

// noProxy are the hosts programs sent to the Gateway reach directly: this
// machine and the local network, as the Gateway itself would. It names no
// IPv6 block, which Python's httpx fails on (fc00::/7 stops it creating a
// client); the Gateway reaches those addresses directly all the same.
var noProxy = []string{"localhost", "127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", ".local"}

// Gateway is an HTTP proxy on the server's own listener, through which
// the programs the server starts, its process plugins and their managed
// containers, reach sites: each signs in with its own name and a password
// derived from the server's key, and its traffic goes through the proxy
// the settings choose for it, decided at each connection. Only this
// machine and the local network may use it, and they are reached directly.
type Gateway struct {
	s      *Service
	key    []byte
	choose func(ctx context.Context, user string) (string, bool)
	log    *slog.Logger
	fwd    *httputil.ReverseProxy

	mu      sync.Mutex
	tunnels map[string]map[*tunnel]struct{} // by user
	closed  bool
}

// tunnel is a CONNECT tunnel, the client's connection and the one it
// reaches.
type tunnel struct{ client, upstream net.Conn }

func (t *tunnel) close() {
	_ = t.client.Close()
	_ = t.upstream.Close()
}

type userKey struct{}

// NewGateway returns a Gateway whose passwords are derived from key and
// whose users' proxies choose returns, by ID ("" for none, SystemID for
// the system proxy); choose reports false for a user that is no longer
// one. Changes of the proxies end the tunnels through them.
func NewGateway(s *Service, key []byte, choose func(ctx context.Context, user string) (string, bool), log *slog.Logger) *Gateway {
	g := &Gateway{s: s, key: key, choose: choose, log: cmp.Or(log, slog.New(slog.DiscardHandler)), tunnels: map[string]map[*tunnel]struct{}{}}
	through := s.Through(func(r *http.Request) string {
		id, _ := r.Context().Value(userKey{}).(string)
		return id
	})
	g.fwd = &httputil.ReverseProxy{
		Rewrite:   func(pr *httputil.ProxyRequest) { pr.Out.Host = pr.In.URL.Host },
		Transport: through,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadGateway)
		},
	}
	s.onChange(func() { g.Reset("") })
	return g
}

// Password returns user's password.
func (g *Gateway) Password(user string) string {
	mac := hmac.New(sha256.New, g.key)
	_, _ = io.WriteString(mac, user)
	return hex.EncodeToString(mac.Sum(nil))
}

// Env returns the environment sending a program's HTTP and HTTPS requests
// to the Gateway as user, which it reaches at address (host:port); the
// local network and the hosts of local are reached directly. ALL_PROXY is
// emptied, the Gateway taking no SOCKS.
func (g *Gateway) Env(user, address string, local ...string) []string {
	u := (&url.URL{Scheme: "http", User: url.UserPassword(user, g.Password(user)), Host: address}).String()
	host, _, _ := net.SplitHostPort(address)
	var hosts []string
	for _, h := range slices.Concat(noProxy, []string{host}, local) {
		if !slices.Contains(hosts, h) {
			hosts = append(hosts, h)
		}
	}
	no := strings.Join(hosts, ",")
	var out []string
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY"} {
		v := ""
		switch name {
		case "HTTP_PROXY", "HTTPS_PROXY":
			v = u
		case "NO_PROXY":
			v = no
		}
		out = append(out, name+"="+v, strings.ToLower(name)+"="+v)
	}
	return out
}

// Wrap returns next serving also the Gateway's requests: CONNECT and
// absolute-form requests from this machine and the local network.
func (g *Gateway) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect && !r.URL.IsAbs() || !remoteLocal(r.RemoteAddr) {
			next.ServeHTTP(w, r)
			return
		}
		user, ok := g.authenticate(r)
		if !ok {
			w.Header().Set("Proxy-Authenticate", `Basic realm="mavio"`)
			http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
			return
		}
		choice, ok := g.choose(r.Context(), user)
		if !ok {
			http.Error(w, "unknown proxy user "+user, http.StatusForbidden)
			return
		}
		if r.Method == http.MethodConnect {
			g.connect(w, r, user, choice)
			return
		}
		g.fwd.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, choice)))
	})
}

// remoteLocal reports whether a request comes from this machine or the
// local network.
func remoteLocal(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	return err == nil && net.ParseIP(host) != nil && proxy.Local(host)
}

func (g *Gateway) authenticate(r *http.Request) (string, bool) {
	scheme, cred, ok := strings.Cut(r.Header.Get("Proxy-Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", false
	}
	// Reuse net/http's decoding of Basic credentials.
	user, pw, ok := (&http.Request{Header: http.Header{"Authorization": {"Basic " + cred}}}).BasicAuth()
	if !ok || user == "" {
		return "", false
	}
	return user, subtle.ConstantTimeCompare([]byte(pw), []byte(g.Password(user))) == 1
}

// connect tunnels a CONNECT request to its target.
func (g *Gateway) connect(w http.ResponseWriter, r *http.Request, user, choice string) {
	ctx, cancel := context.WithTimeout(r.Context(), dialTimeout)
	defer cancel()
	var p *pluginv1.Proxy
	if !proxy.Local(r.Host) {
		var err error
		if p, err = g.s.Resolve(ctx, choice); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	up, err := proxy.Dial(ctx, p, proxy.Local, r.Host)
	if err != nil {
		g.log.DebugContext(ctx, "proxy gateway tunnel", "user", user, "host", r.Host, "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	client, brw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		_ = up.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = client.SetDeadline(time.Time{})
	t := &tunnel{client: client, upstream: up}
	if !g.track(user, t) {
		t.close()
		return
	}
	defer g.untrack(user, t)
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		t.close()
		return
	}
	done := make(chan struct{})
	go func() {
		// What the client sent past its request is in brw.
		_, _ = io.Copy(up, brw)
		closeWrite(up)
		close(done)
	}()
	_, _ = io.Copy(client, up)
	closeWrite(client)
	<-done
	t.close()
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

func (g *Gateway) track(user string, t *tunnel) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	if g.tunnels[user] == nil {
		g.tunnels[user] = map[*tunnel]struct{}{}
	}
	g.tunnels[user][t] = struct{}{}
	return true
}

func (g *Gateway) untrack(user string, t *tunnel) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.tunnels[user], t)
	if len(g.tunnels[user]) == 0 {
		delete(g.tunnels, user)
	}
}

// Reset ends the tunnels of user, every user's for "", for them to be
// opened again through the proxy chosen now; plain HTTP requests take it
// at once.
func (g *Gateway) Reset(user string) {
	g.mu.Lock()
	var ts []*tunnel
	for u, set := range g.tunnels {
		if user == "" || u == user {
			for t := range set {
				ts = append(ts, t)
			}
		}
	}
	g.mu.Unlock()
	for _, t := range ts {
		t.close()
	}
}

// Close ends every tunnel and refuses new ones.
func (g *Gateway) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	g.Reset("")
}
