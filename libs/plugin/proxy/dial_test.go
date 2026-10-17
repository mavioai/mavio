package proxy

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// serve accepts one connection on a new listener and hands it to f.
func serve(t *testing.T, f func(conn net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		f(conn)
	}()
	return ln.Addr().String()
}

// echo answers what it reads, upper-cased.
func echo(conn net.Conn) {
	b := make([]byte, 4)
	if _, err := io.ReadFull(conn, b); err == nil {
		_, _ = conn.Write([]byte(strings.ToUpper(string(b))))
	}
}

func roundTrip(t *testing.T, conn net.Conn) string {
	t.Helper()
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4)
	if _, err := io.ReadFull(conn, b); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDialHTTP(t *testing.T) {
	asked := make(chan *http.Request, 1)
	addr := serve(t, func(conn net.Conn) {
		r, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		asked <- r
		if r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("u:pw")) {
			_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
			return
		}
		// What the target sends at once reaches the caller after the answer.
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\nhi")
		echo(conn)
	})
	conn, err := Dial(t.Context(), newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, addr, "u", "pw"), nil, "indexer.test:443")
	if err != nil {
		t.Fatal(err)
	}
	r := <-asked
	if r.Method != http.MethodConnect || r.Host != "indexer.test:443" {
		t.Errorf("request got = %s %s, want = CONNECT indexer.test:443", r.Method, r.Host)
	}
	b := make([]byte, 2)
	if _, err := io.ReadFull(conn, b); err != nil || string(b) != "hi" {
		t.Errorf("first bytes got = %q, %v; want = hi", b, err)
	}
	if got := roundTrip(t, conn); got != "PING" {
		t.Errorf("got = %q, want = PING", got)
	}

	addr = serve(t, func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err == nil {
			_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
		}
	})
	if _, err := Dial(t.Context(), newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, addr, "", ""), nil, "indexer.test:443"); err == nil || !strings.Contains(err.Error(), "407") {
		t.Errorf("refused got = %v, want = 407", err)
	}
}

func TestDialSOCKS5(t *testing.T) {
	for _, tc := range []struct {
		name, user, host string
		want             []string // what the proxy reads: greeting, sign-in, request
	}{
		{"name", "", "indexer.test", []string{"\x05\x01\x00", "", "\x05\x01\x00\x03\x0cindexer.test\x01\xbb"}},
		{"ip with sign-in", "u", "10.1.2.3", []string{"\x05\x01\x02", "\x01\x01u\x02pw", "\x05\x01\x00\x01\x0a\x01\x02\x03\x01\xbb"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := make(chan []string, 1)
			addr := serve(t, func(conn net.Conn) {
				var got []string
				buf := make([]byte, 256)
				n, _ := conn.Read(buf)
				got = append(got, string(buf[:n]))
				_, _ = conn.Write([]byte{5, buf[2]})
				if buf[2] == 2 {
					n, _ = conn.Read(buf)
					got = append(got, string(buf[:n]))
					_, _ = conn.Write([]byte{1, 0})
				} else {
					got = append(got, "")
				}
				n, _ = conn.Read(buf)
				read <- append(got, string(buf[:n]))
				// Bound to a name, which the caller skips.
				_, _ = conn.Write([]byte("\x05\x00\x00\x03\x05proxy\x04\x38"))
				echo(conn)
			})
			pw := ""
			if tc.user != "" {
				pw = "pw"
			}
			conn, err := Dial(t.Context(), newProxy(pluginv1.ProxyKind_PROXY_KIND_SOCKS5, addr, tc.user, pw), nil, net.JoinHostPort(tc.host, "443"))
			if err != nil {
				t.Fatal(err)
			}
			if got := roundTrip(t, conn); got != "PING" {
				t.Errorf("got = %q, want = PING", got)
			}
			got := <-read
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("read %d got = %q, want = %q", i, got[i], tc.want[i])
				}
			}
		})
	}

	addr := serve(t, func(conn net.Conn) {
		buf := make([]byte, 256)
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte{5, 0})
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
	})
	if _, err := Dial(t.Context(), newProxy(pluginv1.ProxyKind_PROXY_KIND_SOCKS5, addr, "", ""), nil, "indexer.test:443"); err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("refused got = %v, want = connection refused", err)
	}
}

func TestDialDirect(t *testing.T) {
	target := serve(t, echo)
	// The proxy is never reached: nothing listens there.
	unreachable := newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, "127.0.0.1:1", "", "")
	for _, tc := range []struct {
		name   string
		p      *pluginv1.Proxy
		bypass func(string) bool
	}{
		{"no proxy", nil, nil},
		{"system proxy setting none", pluginv1.Proxy_builder{Id: new(SystemID)}.Build(), nil},
		{"bypassed", unreachable, Local},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := Dial(t.Context(), tc.p, tc.bypass, target)
			if err != nil {
				t.Fatal(err)
			}
			if got := roundTrip(t, conn); got != "PING" {
				t.Errorf("got = %q, want = PING", got)
			}
		})
		target = serve(t, echo)
	}
}
