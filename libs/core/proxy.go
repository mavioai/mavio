package core

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// ProxyKind is the protocol of a proxy.
type ProxyKind string

// Kinds of proxies.
const (
	ProxyHTTP   ProxyKind = "http"
	ProxySOCKS4 ProxyKind = "socks4"
	ProxySOCKS5 ProxyKind = "socks5"
)

// SystemProxyID is the ID of the system proxy, which every server has
// without storing it: the proxy its environment or operating system sets.
const SystemProxyID = "system"

// Proxy is one of the server's proxies, which plugins reach sites
// through.
type Proxy struct {
	ID ID
	// Name is unique ignoring case.
	Name string
	Kind ProxyKind
	// Host is a name or an IP address, without brackets.
	Host string
	Port int
	// Username and Password are optional; SOCKS4 takes no password.
	Username, Password string
}

// Validate checks the proxy's invariants.
func (p *Proxy) Validate() error {
	switch {
	case p.ID.IsZero() || strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("%w: proxy requires an ID and a name", ErrInvalid)
	case p.Kind != ProxyHTTP && p.Kind != ProxySOCKS4 && p.Kind != ProxySOCKS5:
		return fmt.Errorf("%w: proxy %q has an unknown kind %q", ErrInvalid, p.Name, p.Kind)
	case !validHost(p.Host):
		return fmt.Errorf("%w: proxy %q has an invalid host %q", ErrInvalid, p.Name, p.Host)
	case p.Port < 1 || p.Port > 65535:
		return fmt.Errorf("%w: proxy %q has an invalid port %d", ErrInvalid, p.Name, p.Port)
	case p.Kind == ProxySOCKS4 && p.Password != "":
		return fmt.Errorf("%w: SOCKS4 proxy %q takes no password", ErrInvalid, p.Name)
	case p.Password != "" && p.Username == "":
		return fmt.Errorf("%w: proxy %q has a password without a username", ErrInvalid, p.Name)
	}
	return nil
}

// validHost reports whether host is an IP address or a host name.
func validHost(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	if host == "" || len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasPrefix(host, "-") {
		return false
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.', r == '_':
		default:
			return false
		}
	}
	return true
}

// NameKey is the proxy's name as compared for uniqueness.
func (p *Proxy) NameKey() string { return strings.ToLower(strings.TrimSpace(p.Name)) }

// Key tells duplicates apart: proxies with the same host, port and
// username are the same proxy, whatever their kinds. Hosts are compared
// lowercased and IP addresses in their normal form, never resolved.
func (p *Proxy) Key() string {
	return url.QueryEscape(p.Username) + "@" + net.JoinHostPort(normalHost(p.Host), strconv.Itoa(p.Port))
}

func normalHost(host string) string {
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Unmap().String()
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// Address is the proxy's host and port.
func (p *Proxy) Address() string { return net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) }

// URL is the proxy as a URL, kind://user:password@host:port.
func (p *Proxy) URL() *url.URL {
	u := &url.URL{Scheme: string(p.Kind), Host: p.Address()}
	switch {
	case p.Password != "":
		u.User = url.UserPassword(p.Username, p.Password)
	case p.Username != "":
		u.User = url.User(p.Username)
	}
	return u
}
