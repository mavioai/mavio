package proxy

import (
	"context"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func newProxy(kind pluginv1.ProxyKind, hostport, user, password string) *pluginv1.Proxy {
	host, portStr, _ := net.SplitHostPort(hostport)
	port, _ := strconv.Atoi(portStr)
	return pluginv1.Proxy_builder{
		Id: new("p"), Kind: &kind, Host: &host, Port: new(int32(port)), Username: &user, Password: &password,
	}.Build()
}

func TestBypass(t *testing.T) {
	for _, tc := range []struct {
		hosts []string
		host  string
		want  bool
	}{
		{[]string{"localhost", "127.0.0.1"}, "localhost:8080", true},
		{[]string{"localhost", "127.0.0.1"}, "127.0.0.1", true},
		{[]string{"localhost"}, "api.opensubtitles.com", false},
		{[]string{"opensubtitles.com"}, "api.opensubtitles.com:443", true},
		{[]string{".opensubtitles.com"}, "opensubtitles.com", true},
		{[]string{"opensubtitles.com"}, "notopensubtitles.com", false},
		{[]string{"::1"}, "[::1]:80", true},
		{[]string{"*"}, "podnapisi.net", true},
		{nil, "localhost", false},
		{[]string{"192.168.0.0/16"}, "192.168.31.1:80", true},
		{[]string{"192.168.0.0/16"}, "10.0.0.1", false},
		{[]string{"127.0.0.1/32"}, "127.0.0.1", true},
		{[]string{"10.0.0.0/8"}, "10.example.com", false},
		{[]string{"*.local"}, "nas.local", true},
		{[]string{"*.local"}, "local", false},
		{[]string{"10.*"}, "10.1.2.3:8080", true},
		{[]string{"<local>"}, "nas:8080", true},
		{[]string{"<local>"}, "nas.lan", false},
		{[]string{"<local>"}, "[::1]", false},
	} {
		if got := Bypass(tc.hosts)(tc.host); got != tc.want {
			t.Errorf("Bypass(%v)(%q) got = %v, want = %v", tc.hosts, tc.host, got, tc.want)
		}
	}
}

func TestLocal(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost:8080":     true,
		"nas:8080":           true,
		"127.0.0.1":          true,
		"[::1]:80":           true,
		"10.1.2.3":           true,
		"172.20.0.5:9696":    true,
		"192.168.31.1":       true,
		"[fd00::1]:80":       true,
		"172.32.0.1":         false,
		"api.themoviedb.org": false,
		"8.8.8.8":            false,
	} {
		if got := Local(host); got != want {
			t.Errorf("Local(%q) got = %v, want = %v", host, got, want)
		}
	}
}

func TestURL(t *testing.T) {
	for _, tc := range []struct {
		p    *pluginv1.Proxy
		want string
	}{
		{newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, "h:8080", "", ""), "http://h:8080"},
		{newProxy(pluginv1.ProxyKind_PROXY_KIND_SOCKS5, "[::1]:1080", "u", "p@ss"), "socks5://u:p%40ss@[::1]:1080"},
		{newProxy(pluginv1.ProxyKind_PROXY_KIND_SOCKS4, "h:1080", "u", ""), "socks4://u@h:1080"},
	} {
		if got := URL(tc.p).String(); got != tc.want {
			t.Errorf("URL() got = %q, want = %q", got, tc.want)
		}
	}
	if URL(nil) != nil {
		t.Error("URL(nil) got = non-nil, want = nil")
	}
	if got := URL(pluginv1.Proxy_builder{Id: new(SystemID)}.Build()); got != nil {
		t.Errorf("URL(system proxy setting none) got = %v, want = nil", got)
	}
}

func get(t *testing.T, rt http.RoundTripper, u string) string {
	t.Helper()
	resp, err := (&http.Client{Transport: rt}).Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestHTTPProxy(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "direct") }))
	defer site.Close()
	var proxied atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Store(r.URL.String() + " " + r.Header.Get("Proxy-Authorization"))
		_, _ = io.WriteString(w, "proxied")
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	p := newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, u.Host, "u", "p")
	if got := get(t, Transport(p, nil), "http://subtitles.test/a"); got != "proxied" {
		t.Errorf("got = %q, want = proxied", got)
	}
	if got, want := proxied.Load(), "http://subtitles.test/a Basic dTpw"; got != want {
		t.Errorf("proxied got = %q, want = %q", got, want)
	}
	if got := get(t, Transport(p, Bypass([]string{"127.0.0.1"})), site.URL); got != "direct" {
		t.Errorf("bypassed got = %q, want = direct", got)
	}
	if got := get(t, Transport(nil, nil), site.URL); got != "direct" {
		t.Errorf("without a proxy got = %q, want = direct", got)
	}
	system := newProxy(pluginv1.ProxyKind_PROXY_KIND_HTTP, u.Host, "", "")
	system.SetId(SystemID)
	system.SetBypass([]string{"127.0.0.0/8"})
	if got := get(t, Transport(system, nil), site.URL); got != "direct" {
		t.Errorf("bypassed by the proxy got = %q, want = direct", got)
	}
	if got := get(t, Transport(system, nil), "http://subtitles.test/b"); got != "proxied" {
		t.Errorf("not bypassed by the proxy got = %q, want = proxied", got)
	}
	none := pluginv1.Proxy_builder{Id: new(SystemID)}.Build()
	if got := get(t, Transport(none, nil), site.URL); got != "direct" {
		t.Errorf("system proxy setting none got = %q, want = direct", got)
	}
}

func TestSOCKS4(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "hello "+r.Host) }))
	defer site.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	requests := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		requests <- buf[:n]
		_, _ = conn.Write([]byte{0, 0x5a, 0, 0, 0, 0, 0, 0})
		up, err := net.Dial("tcp", site.Listener.Addr().String())
		if err != nil {
			return
		}
		defer up.Close()
		go func() { _, _ = io.Copy(up, conn) }()
		_, _ = io.Copy(conn, up)
	}()
	p := newProxy(pluginv1.ProxyKind_PROXY_KIND_SOCKS4, ln.Addr().String(), "u", "")
	if got := get(t, Transport(p, nil), "http://subtitles.test:8080/"); got != "hello subtitles.test:8080" {
		t.Errorf("got = %q, want = hello subtitles.test:8080", got)
	}
	// SOCKS4a: version, CONNECT, port 8080, 0.0.0.1, user, host.
	want := "\x04\x01\x1f\x90\x00\x00\x00\x01u\x00subtitles.test\x00"
	if got := string(<-requests); got != want {
		t.Errorf("request got = %q, want = %q", got, want)
	}
}

type lister struct {
	asked []string
	err   error
}

func (l *lister) ListProxies(_ context.Context, req *pluginv1.ListProxiesRequest) (*pluginv1.ListProxiesResponse, error) {
	l.asked = req.GetIds()
	if l.err != nil {
		return nil, l.err
	}
	var out []*pluginv1.Proxy
	for _, id := range req.GetIds() {
		if id != "gone" {
			out = append(out, pluginv1.Proxy_builder{Id: new(id)}.Build())
		}
	}
	return pluginv1.ListProxiesResponse_builder{Proxies: out}.Build(), nil
}

func TestList(t *testing.T) {
	l := &lister{}
	got, err := List(t.Context(), l, "b", "", "a", "b", "gone")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "gone"}; !reflect.DeepEqual(l.asked, want) {
		t.Errorf("asked got = %v, want = %v", l.asked, want)
	}
	ids := slices.Sorted(maps.Keys(got))
	if want := []string{"a", "b"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("List got = %v, want = %v", ids, want)
	}
	l = &lister{}
	if got, err := List(t.Context(), l, ""); err != nil || len(got) != 0 || l.asked != nil {
		t.Errorf("List of none got = %v, %v, asked %v; want = nothing asked", got, err, l.asked)
	}
	l = &lister{err: connect.NewError(connect.CodePermissionDenied, nil)}
	if _, err := List(t.Context(), l, "a"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("List denied got = %v, want = permission denied", err)
	}
}
