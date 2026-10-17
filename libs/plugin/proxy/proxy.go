// Package proxy reaches sites through the server's proxies, which a
// plugin's configuration names by ID (manifest.OptionsProxies) and
// HostService.ListProxies returns.
package proxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ErrNotFound is a proxy the server does not have, such as one deleted.
var ErrNotFound = errors.New("proxy not found")

// SystemID is the ID of the system proxy, which every server has: the one
// its environment or operating system sets. With none set, its kind is
// unspecified and Transport reaches sites directly.
const SystemID = "system"

var schemes = map[pluginv1.ProxyKind]string{
	pluginv1.ProxyKind_PROXY_KIND_HTTP:   "http",
	pluginv1.ProxyKind_PROXY_KIND_SOCKS4: "socks4",
	pluginv1.ProxyKind_PROXY_KIND_SOCKS5: "socks5",
}

// Address returns the proxy's host:port.
func Address(p *pluginv1.Proxy) string {
	return net.JoinHostPort(p.GetHost(), strconv.Itoa(int(p.GetPort())))
}

// URL returns the proxy as kind://user:password@host:port; nil for nil
// and for a system proxy setting none.
func URL(p *pluginv1.Proxy) *url.URL {
	if p == nil || p.GetKind() == pluginv1.ProxyKind_PROXY_KIND_UNSPECIFIED {
		return nil
	}
	u := &url.URL{Scheme: schemes[p.GetKind()], Host: Address(p)}
	switch {
	case p.GetPassword() != "":
		u.User = url.UserPassword(p.GetUsername(), p.GetPassword())
	case p.GetUsername() != "":
		u.User = url.User(p.GetUsername())
	}
	return u
}

// Bypass returns whether a host is reached directly, as NO_PROXY lists
// them: a host is excluded by its name, a domain by its suffix
// ("example.com" and ".example.com" both exclude its subdomains), names
// by "*" wildcards ("*.local", "10.*"), IP addresses by themselves or
// their CIDR blocks, names without a dot by "<local>" and every host by
// "*".
func Bypass(hosts []string) func(host string) bool {
	return func(hostport string) bool {
		host := hostport
		if h, _, err := net.SplitHostPort(hostport); err == nil {
			host = h
		}
		host = strings.ToLower(strings.Trim(host, "[]"))
		addr, addrErr := netip.ParseAddr(host)
		return slices.ContainsFunc(hosts, func(e string) bool {
			e = strings.ToLower(strings.TrimSpace(e))
			switch {
			case e == "":
				return false
			case e == "*":
				return true
			case e == "<local>":
				return addrErr != nil && !strings.Contains(host, ".")
			case strings.Contains(e, "/"):
				pfx, err := netip.ParsePrefix(e)
				return err == nil && addrErr == nil && pfx.Contains(addr.Unmap())
			case strings.Contains(e, "*"):
				ok, _ := path.Match(e, host)
				return ok
			}
			e = strings.TrimPrefix(e, ".")
			return host == e || strings.HasSuffix(host, "."+e)
		})
	}
}

// Local reports whether a host is one of this machine or of the local
// network, which Prowlarr's BypassLocalAddresses reaches directly
// whichever the proxy: names without a dot, localhost, and loopback and
// private addresses.
func Local(host string) bool { return local(host) }

var local = Bypass([]string{
	"<local>", "localhost", "127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
})

// Transport returns a transport through p, a direct one for nil and for
// a system proxy setting none, reaching directly the hosts p's bypass
// list and bypass report; bypass may be nil.
func Transport(p *pluginv1.Proxy, bypass func(host string) bool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	if URL(p) == nil {
		return t
	}
	if own := p.GetBypass(); len(own) > 0 {
		ownBypass, other := Bypass(own), bypass
		bypass = func(host string) bool { return ownBypass(host) || other != nil && other(host) }
	}
	if bypass == nil {
		bypass = func(string) bool { return false }
	}
	if p.GetKind() == pluginv1.ProxyKind_PROXY_KIND_SOCKS4 {
		d := &socks4Dialer{proxy: Address(p), user: p.GetUsername(), direct: bypass}
		t.DialContext = d.DialContext
		return t
	}
	// net/http speaks HTTP and SOCKS5 proxies itself.
	u := URL(p)
	t.Proxy = func(r *http.Request) (*url.URL, error) {
		if bypass(r.URL.Host) {
			return nil, nil
		}
		return u, nil
	}
	return t
}

// Lister lists the server's proxies; pluginv1connect.HostServiceClient is
// one.
type Lister interface {
	ListProxies(ctx context.Context, req *pluginv1.ListProxiesRequest) (*pluginv1.ListProxiesResponse, error)
}

var _ Lister = pluginv1connect.HostServiceClient(nil)

// List returns the proxies of the IDs by ID, the empty ID left out. An ID
// the server does not know is left out too, for the caller to fail what
// uses it with ErrNotFound.
func List(ctx context.Context, l Lister, ids ...string) (map[string]*pluginv1.Proxy, error) {
	want := slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return id == "" })
	if len(want) == 0 {
		return map[string]*pluginv1.Proxy{}, nil
	}
	slices.Sort(want)
	want = slices.Compact(want)
	resp, err := l.ListProxies(ctx, pluginv1.ListProxiesRequest_builder{Ids: want}.Build())
	if err != nil {
		if connect.CodeOf(err) == connect.CodeUnimplemented {
			return map[string]*pluginv1.Proxy{}, nil
		}
		return nil, fmt.Errorf("list proxies: %w", err)
	}
	out := make(map[string]*pluginv1.Proxy, len(resp.GetProxies()))
	for _, p := range resp.GetProxies() {
		out[p.GetId()] = p
	}
	return out, nil
}

// socks4Dialer connects through a SOCKS4a proxy, which net/http does not
// speak.
type socks4Dialer struct {
	proxy, user string
	direct      func(addr string) bool
}

func (d *socks4Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var nd net.Dialer
	if d.direct(addr) {
		return nd.DialContext(ctx, network, addr)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("port %q: %w", portStr, err)
	}
	conn, err := nd.DialContext(ctx, "tcp", d.proxy)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := d.connect(conn, host, uint16(port)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS4 proxy %s: %w", d.proxy, err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (d *socks4Dialer) connect(conn net.Conn, host string, port uint16) error {
	req := binary.BigEndian.AppendUint16([]byte{4, 1}, port)
	ip := net.ParseIP(host).To4()
	if ip == nil {
		// SOCKS4a: the proxy resolves the name.
		req = append(req, 0, 0, 0, 1)
	} else {
		req = append(req, ip...)
	}
	req = append(append(req, d.user...), 0)
	if ip == nil {
		req = append(append(req, host...), 0)
	}
	if _, err := conn.Write(req); err != nil {
		return err
	}
	var resp [8]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return err
	}
	if resp[1] != 0x5a {
		return errors.New("connection rejected")
	}
	return nil
}
