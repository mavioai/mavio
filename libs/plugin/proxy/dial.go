package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Dial connects to addr (host:port) through p, a TCP stream as any
// protocol uses: an HTTP proxy is asked to CONNECT, SOCKS proxies to
// connect. It connects directly for nil, for a system proxy setting none,
// and for the hosts p's bypass list and bypass report; bypass may be nil.
func Dial(ctx context.Context, p *pluginv1.Proxy, bypass func(host string) bool, addr string) (net.Conn, error) {
	var nd net.Dialer
	if URL(p) == nil || bypass != nil && bypass(addr) || Bypass(p.GetBypass())(addr) {
		return nd.DialContext(ctx, "tcp", addr)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("port %q: %w", portStr, err)
	}
	if p.GetKind() == pluginv1.ProxyKind_PROXY_KIND_SOCKS4 {
		d := &socks4Dialer{proxy: Address(p), user: p.GetUsername(), direct: func(string) bool { return false }}
		return d.DialContext(ctx, "tcp", addr)
	}
	conn, err := nd.DialContext(ctx, "tcp", Address(p))
	if err != nil {
		return nil, err
	}
	// The handshake ends with ctx, or at its deadline.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	out := conn
	if p.GetKind() == pluginv1.ProxyKind_PROXY_KIND_SOCKS5 {
		err = socks5Connect(conn, p.GetUsername(), p.GetPassword(), host, uint16(port))
	} else {
		out, err = httpConnect(conn, p.GetUsername(), p.GetPassword(), addr)
	}
	if !stop() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%s proxy %s: %w", schemes[p.GetKind()], Address(p), err)
	}
	_ = conn.SetDeadline(time.Time{})
	return out, nil
}

// httpConnect asks an HTTP proxy to tunnel to addr.
func httpConnect(conn net.Conn, user, password, addr string) (net.Conn, error) {
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: addr}, Host: addr, Header: http.Header{}}
	if user != "" || password != "" {
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
	}
	if err := req.Write(conn); err != nil {
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CONNECT answered %s", resp.Status)
	}
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufferedConn reads first what the proxy sent past its answer.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.r.Read(b) }

// socks5Errors are the replies of a SOCKS5 proxy refusing to connect.
var socks5Errors = map[byte]string{
	1: "general failure", 2: "connection not allowed by ruleset", 3: "network unreachable", 4: "host unreachable",
	5: "connection refused", 6: "TTL expired", 7: "command not supported", 8: "address type not supported",
}

// socks5Connect asks a SOCKS5 proxy to connect to host:port, signing in
// with a username and password when given (RFC 1928, RFC 1929); the proxy
// resolves names.
func socks5Connect(conn net.Conn, user, password, host string, port uint16) error {
	methods := []byte{0}
	if user != "" || password != "" {
		if len(user) > 255 || len(password) > 255 {
			return errors.New("username or password longer than 255 bytes")
		}
		methods = []byte{2}
	}
	if _, err := conn.Write(append([]byte{5, byte(len(methods))}, methods...)); err != nil {
		return err
	}
	var choice [2]byte
	if _, err := io.ReadFull(conn, choice[:]); err != nil {
		return err
	}
	switch {
	case choice[0] != 5:
		return fmt.Errorf("version %d is not SOCKS5", choice[0])
	case choice[1] == 2 && methods[0] == 2:
		auth := append(append([]byte{1, byte(len(user))}, user...), byte(len(password)))
		if _, err := conn.Write(append(auth, password...)); err != nil {
			return err
		}
		var status [2]byte
		if _, err := io.ReadFull(conn, status[:]); err != nil {
			return err
		}
		if status[1] != 0 {
			return errors.New("username or password rejected")
		}
	case choice[1] != methods[0]:
		return errors.New("no acceptable way to sign in")
	}

	req := []byte{5, 1, 0}
	if ip, err := netip.ParseAddr(host); err == nil && ip.Unmap().Is4() {
		req = append(append(req, 1), ip.Unmap().AsSlice()...)
	} else if err == nil {
		req = append(append(req, 4), ip.AsSlice()...)
	} else {
		if len(host) > 255 {
			return errors.New("host name longer than 255 bytes")
		}
		req = append(append(req, 3, byte(len(host))), host...)
	}
	if _, err := conn.Write(binary.BigEndian.AppendUint16(req, port)); err != nil {
		return err
	}
	var reply [4]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return err
	}
	if reply[1] != 0 {
		if msg, ok := socks5Errors[reply[1]]; ok {
			return errors.New(msg)
		}
		return fmt.Errorf("reply %d", reply[1])
	}
	// The address the proxy bound, which nothing needs.
	skip := 0
	switch reply[3] {
	case 1:
		skip = 4
	case 4:
		skip = 16
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return err
		}
		skip = int(n[0])
	default:
		return fmt.Errorf("address type %d", reply[3])
	}
	_, err := io.CopyN(io.Discard, conn, int64(skip+2))
	return err
}
