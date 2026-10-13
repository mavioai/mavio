// Package web is how providers reach their sites: through the proxy
// Bazarr's settings give, solving Cloudflare's challenges with
// FlareSolverr.
package web

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ProxyKind is the kind of a proxy, as Bazarr's proxy types.
type ProxyKind string

// The kinds of proxies.
const (
	ProxyHTTP   ProxyKind = "http"
	ProxySOCKS4 ProxyKind = "socks4"
	ProxySOCKS5 ProxyKind = "socks5"
)

// Proxy is the proxy providers' requests go through, as Bazarr's proxy
// settings.
type Proxy struct {
	Kind ProxyKind `json:"kind"`
	// Host is host:port.
	Host     string `json:"host"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// Exclude are the hosts reached directly, as NO_PROXY; localhost and
	// 127.0.0.1 when nil.
	Exclude []string `json:"exclude,omitempty"`
}

// Validate checks the proxy's settings.
func (p *Proxy) Validate() error {
	switch p.Kind {
	case ProxyHTTP, ProxySOCKS4, ProxySOCKS5:
	default:
		return fmt.Errorf("unknown proxy kind %q", p.Kind)
	}
	if _, _, err := net.SplitHostPort(p.Host); err != nil {
		return fmt.Errorf("%s proxy host %q: %w", p.Kind, p.Host, err)
	}
	return nil
}

// URL returns the proxy's URL, with its credentials.
func (p *Proxy) URL() *url.URL {
	u := &url.URL{Scheme: string(p.Kind), Host: p.Host}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u
}

// excluded reports whether a host is reached directly.
func (p *Proxy) excluded(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	exclude := p.Exclude
	if exclude == nil {
		exclude = []string{"localhost", "127.0.0.1"}
	}
	return slices.ContainsFunc(exclude, func(e string) bool {
		e = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(e)), ".")
		h := strings.ToLower(host)
		return e != "" && (e == "*" || h == e || strings.HasSuffix(h, "."+e))
	})
}

// newTransport returns a transport through the proxy; a direct one for
// nil.
func newTransport(p *Proxy) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	switch {
	case p == nil:
		t.Proxy = nil
	case p.Kind == ProxySOCKS4:
		d := &socks4Dialer{proxy: p.Host, user: p.Username, direct: p.excluded}
		t.Proxy = nil
		t.DialContext = d.DialContext
	default:
		// net/http speaks HTTP and SOCKS5 proxies itself.
		u := p.URL()
		t.Proxy = func(r *http.Request) (*url.URL, error) {
			if p.excluded(r.URL.Host) {
				return nil, nil
			}
			return u, nil
		}
	}
	return t
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
