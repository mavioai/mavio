package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/apps/server/internal/dependencies"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/apps/server/internal/rpc"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// proxyEnv names the environment variables of proxies, which programs
// reaching sites directly get empty, not to take the server's own.
var proxyEnv = []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "no_proxy", "all_proxy"}

// gatewayUsers sends process plugins and managed containers to the proxy
// gateway when their proxy is one, and leaves them reaching sites
// directly otherwise, sparing the gateway: they start again when their
// choice goes from one to the other, as the system proxy is set or not.
type gatewayUsers struct {
	gateway  *proxies.Gateway
	proxies  *proxies.Service
	settings *settings.Manager
	names    containers.Names
	// addr is where process plugins reach the server's listener, port
	// where containers do on the host they reach it by.
	addr string
	port int
	log  *slog.Logger

	plugs *plugins.Manager
	deps  *dependencies.Service

	mu sync.Mutex
	// proxied tells, by plugin ID, whether a process plugin started
	// through the gateway.
	proxied map[string]bool
}

// gatewayAddress returns where processes on this machine reach a listener
// at addr: the loopback address for one on every address.
func gatewayAddress(addr net.Addr) (string, int) {
	a, ok := addr.(*net.TCPAddr)
	if !ok {
		return "", 0
	}
	host := "127.0.0.1"
	if !a.IP.IsUnspecified() && !a.IP.IsLoopback() {
		host = a.IP.String()
	}
	return net.JoinHostPort(host, strconv.Itoa(a.Port)), a.Port
}

func (g *gatewayUsers) choice(id string) string { return rpc.PluginProxy(g.settings.Get(), id) }

// choose returns the proxy choice of a gateway user: a managed container
// by its name, or a running process plugin by its ID.
func (g *gatewayUsers) choose(ctx context.Context, user string) (string, bool) {
	if c, ok := g.deps.ProxyOf(ctx, user); ok {
		return c, true
	}
	if g.plugs == nil {
		return "", false
	}
	if m, ok := g.plugs.Manifest(user); ok && m.GetRuntime() == pluginv1.Runtime_RUNTIME_PROCESS {
		return g.choice(user), true
	}
	return "", false
}

// env returns a process plugin's proxy environment as it starts.
func (g *gatewayUsers) env(m *pluginv1.Manifest) []string {
	id := m.GetId()
	direct := g.addr == "" || g.proxies.Direct(context.Background(), g.choice(id))
	g.mu.Lock()
	g.proxied[id] = !direct
	g.mu.Unlock()
	if direct {
		out := make([]string, len(proxyEnv))
		for i, k := range proxyEnv {
			out[i] = k + "="
		}
		return out
	}
	return g.gateway.Env(id, g.addr)
}

// containerEnv returns a managed container's proxy environment, nil when
// it reaches sites directly; it reaches the plugin's other containers
// directly.
func (g *gatewayUsers) containerEnv(ctx context.Context, m *pluginv1.Manifest, c core.PluginContainer) func(host string) map[string]string {
	if g.port == 0 || g.proxies.Direct(ctx, c.Proxy) {
		return nil
	}
	user := g.names.Container(c.PluginID, c.Name)
	var (
		siblings []string
		ipv4Only bool
	)
	for _, spec := range m.GetContainers() {
		siblings = append(siblings, g.names.Container(c.PluginID, spec.GetName()))
		if spec.GetName() == c.Name {
			ipv4Only = spec.GetNoProxyIpv4Only()
		}
	}
	return func(host string) map[string]string {
		out := map[string]string{}
		for _, kv := range g.gateway.Env(user, net.JoinHostPort(host, strconv.Itoa(g.port)), siblings...) {
			k, v, _ := strings.Cut(kv, "=")
			if ipv4Only && strings.EqualFold(k, "NO_PROXY") {
				v = withoutIPv6Blocks(v)
			}
			out[k] = v
		}
		return out
	}
}

// withoutIPv6Blocks leaves the IPv6 blocks, such as fc00::/7, out of a
// NO_PROXY list, keeping single addresses such as ::1.
func withoutIPv6Blocks(noProxy string) string {
	hosts := strings.Split(noProxy, ",")
	hosts = slices.DeleteFunc(hosts, func(h string) bool {
		p, err := netip.ParsePrefix(strings.TrimSpace(h))
		return err == nil && p.Addr().Is6()
	})
	return strings.Join(hosts, ",")
}

// pluginChanged follows a plugin's new proxy: its tunnels end, and it
// starts again when it goes from reaching sites directly to through the
// gateway, or back.
func (g *gatewayUsers) pluginChanged(ctx context.Context, id string) {
	g.gateway.Reset(id)
	g.mu.Lock()
	was, ok := g.proxied[id]
	g.mu.Unlock()
	if ok && was == g.proxies.Direct(ctx, g.choice(id)) {
		g.restart(ctx, id)
	}
}

// proxiesChanged starts again the process plugins and the managed
// containers whose proxy went from none to one or back, the system proxy
// set or no longer.
func (g *gatewayUsers) proxiesChanged(ctx context.Context) {
	g.mu.Lock()
	var flipped []string
	for id, was := range g.proxied {
		if was == g.proxies.Direct(ctx, g.choice(id)) {
			flipped = append(flipped, id)
		}
	}
	g.mu.Unlock()
	for _, id := range flipped {
		g.restart(ctx, id)
	}
	g.deps.ProxiesChanged(ctx)
}

func (g *gatewayUsers) restart(ctx context.Context, id string) {
	if g.plugs == nil {
		return
	}
	g.log.InfoContext(ctx, "restarting plugin for its proxy", "plugin", id)
	if err := g.plugs.Restart(ctx, id); errors.Is(err, core.ErrNotFound) {
		g.mu.Lock()
		delete(g.proxied, id)
		g.mu.Unlock()
	} else if err != nil {
		g.log.WarnContext(ctx, "restart plugin for its proxy", "plugin", id, "err", err)
	}
}
