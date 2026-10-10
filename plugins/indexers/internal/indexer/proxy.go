package indexer

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
	"time"
)

// ProxyKind is the kind of a proxy.
type ProxyKind string

// The kinds of proxies, as Prowlarr's indexer proxies are.
const (
	ProxyHTTP         ProxyKind = "http"
	ProxySOCKS4       ProxyKind = "socks4"
	ProxySOCKS5       ProxyKind = "socks5"
	ProxyFlareSolverr ProxyKind = "flaresolverr"
)

// Proxy is a proxy indexers sharing a tag with it send requests through.
type Proxy struct {
	Kind ProxyKind `json:"kind"`
	// host:port, or FlareSolverr's URL.
	Host     string   `json:"host"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	// How long FlareSolverr may take to solve a challenge, in seconds;
	// 60 when 0.
	RequestTimeout int `json:"request_timeout,omitempty"`
}

// Validate checks the proxy's settings.
func (p *Proxy) Validate() error {
	switch p.Kind {
	case ProxyHTTP, ProxySOCKS4, ProxySOCKS5:
		if _, _, err := net.SplitHostPort(p.Host); err != nil {
			return fmt.Errorf("%s proxy host %q: %w", p.Kind, p.Host, err)
		}
	case ProxyFlareSolverr:
		if u, err := url.Parse(p.Host); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("FlareSolverr URL %q is not an HTTP URL", p.Host)
		}
	default:
		return fmt.Errorf("unknown proxy kind %q", p.Kind)
	}
	return nil
}

// URL returns the proxy's URL, with its credentials; nil for FlareSolverr.
func (p *Proxy) URL() *url.URL {
	if p == nil || p.Kind == ProxyFlareSolverr {
		return nil
	}
	u := &url.URL{Scheme: string(p.Kind), Host: p.Host}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u
}

// SelectProxies returns the first proxy and the first FlareSolverr sharing
// a tag with the indexer, either nil when there is none.
func SelectProxies(proxies []Proxy, tags []string) (proxy, solver *Proxy) {
	for i := range proxies {
		p := &proxies[i]
		if !slices.ContainsFunc(p.Tags, func(t string) bool { return slices.Contains(tags, t) }) {
			continue
		}
		if p.Kind == ProxyFlareSolverr {
			if solver == nil {
				solver = p
			}
		} else if proxy == nil {
			proxy = p
		}
	}
	return proxy, solver
}

// newTransport returns a transport through the proxy, nil for none.
func newTransport(p *Proxy) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	switch {
	case p == nil:
	case p.Kind == ProxySOCKS4:
		d := &socks4Dialer{proxy: p.Host, user: p.Username}
		t.Proxy = nil
		t.DialContext = d.DialContext
	default:
		// net/http speaks HTTP and SOCKS5 proxies itself.
		t.Proxy = http.ProxyURL(p.URL())
	}
	return t
}

// socks4Dialer connects through a SOCKS4a proxy, which net/http does not
// speak.
type socks4Dialer struct {
	proxy, user string
}

func (d *socks4Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("port %q: %w", portStr, err)
	}
	var nd net.Dialer
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
	req := []byte{4, 1, 0, 0}
	req = binary.BigEndian.AppendUint16(req[:2], port)
	ip := net.ParseIP(host).To4()
	if ip == nil {
		// SOCKS4a: the proxy resolves the name.
		ip = net.IPv4(0, 0, 0, 1).To4()
	}
	req = append(req, ip...)
	req = append(append(req, d.user...), 0)
	if net.ParseIP(host).To4() == nil {
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
