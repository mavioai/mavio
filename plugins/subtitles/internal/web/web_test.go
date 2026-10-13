package web

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExcluded(t *testing.T) {
	cases := []struct {
		exclude []string
		host    string
		want    bool
	}{
		{nil, "localhost:8080", true},
		{nil, "127.0.0.1", true},
		{nil, "api.opensubtitles.com", false},
		{[]string{"opensubtitles.com"}, "api.opensubtitles.com:443", true},
		{[]string{".opensubtitles.com"}, "opensubtitles.com", true},
		{[]string{"opensubtitles.com"}, "notopensubtitles.com", false},
		{[]string{"*"}, "podnapisi.net", true},
		{[]string{}, "localhost", false},
	}
	for _, c := range cases {
		p := &Proxy{Kind: ProxyHTTP, Host: "proxy:8080", Exclude: c.exclude}
		if got := p.excluded(c.host); got != c.want {
			t.Errorf("excluded(%v, %q) got = %v, want = %v", c.exclude, c.host, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		p    Proxy
		want bool
	}{
		{Proxy{Kind: ProxyHTTP, Host: "proxy:8080"}, true},
		{Proxy{Kind: ProxySOCKS4, Host: "proxy:1080"}, true},
		{Proxy{Kind: ProxySOCKS5, Host: "[::1]:1080"}, true},
		{Proxy{Kind: "https", Host: "proxy:8080"}, false},
		{Proxy{Kind: ProxyHTTP, Host: "proxy"}, false},
	}
	for _, c := range cases {
		if got := c.p.Validate() == nil; got != c.want {
			t.Errorf("Validate(%+v) got = %v, want = %v", c.p, got, c.want)
		}
	}
}

func TestHTTPProxy(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "direct") }))
	defer site.Close()
	var proxied atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Store(r.URL.String() + " " + r.Header.Get("Proxy-Authorization"))
		_, _ = io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	u, _ := url.Parse(proxy.URL)
	p := &Proxy{Kind: ProxyHTTP, Host: u.Host, Username: "u", Password: "p", Exclude: []string{"excluded.test"}}
	c := NewClient(Config{Proxy: p}, 5*time.Second)
	if got := get(t, c, "http://subtitles.test/a"); got != "proxied" {
		t.Errorf("got = %q, want = proxied", got)
	}
	if got, want := proxied.Load(), "http://subtitles.test/a Basic dTpw"; got != want {
		t.Errorf("proxied got = %q, want = %q", got, want)
	}
	// Excluded hosts are reached directly.
	p.Exclude = []string{"127.0.0.1"}
	if got := get(t, NewClient(Config{Proxy: p}, 5*time.Second), site.URL); got != "direct" {
		t.Errorf("excluded got = %q, want = direct", got)
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
	p := &Proxy{Kind: ProxySOCKS4, Host: ln.Addr().String(), Username: "u", Exclude: []string{}}
	c := NewClient(Config{Proxy: p}, 5*time.Second)
	if got := get(t, c, "http://subtitles.test:8080/"); got != "hello subtitles.test:8080" {
		t.Errorf("got = %q, want = hello subtitles.test:8080", got)
	}
	// SOCKS4a: version, CONNECT, port 8080, 0.0.0.1, user, host.
	want := "\x04\x01\x1f\x90\x00\x00\x00\x01u\x00subtitles.test\x00"
	if got := string(<-requests); got != want {
		t.Errorf("request got = %q, want = %q", got, want)
	}
}

func TestFlareSolverr(t *testing.T) {
	var hits atomic.Int32
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Server", "cloudflare")
		if ck, err := r.Cookie("cf_clearance"); err == nil && ck.Value == "ok" && r.UserAgent() == "browser" {
			_, _ = io.WriteString(w, "subtitles")
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "<html><title>Just a moment...</title></html>")
	}))
	defer site.Close()
	var cmds []map[string]any
	solver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cmd map[string]any
		_ = json.NewDecoder(r.Body).Decode(&cmd)
		cmds = append(cmds, cmd)
		_, _ = io.WriteString(w, `{"status":"ok","solution":{"userAgent":"browser","cookies":[{"name":"cf_clearance","value":"ok"}]}}`)
	}))
	defer solver.Close()
	c := NewClient(Config{FlareSolverr: &FlareSolverr{URL: solver.URL, RequestTimeout: 30}}, 5*time.Second)
	for range 2 {
		if got := get(t, c, site.URL+"/search"); got != "subtitles" {
			t.Fatalf("got = %q, want = subtitles", got)
		}
	}
	if len(cmds) != 1 {
		t.Fatalf("got %d FlareSolverr requests, want = 1", len(cmds))
	}
	if cmds[0]["cmd"] != "request.get" || cmds[0]["url"] != site.URL+"/search" || cmds[0]["maxTimeout"] != 30000.0 {
		t.Errorf("got command = %v", cmds[0])
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("got %d requests to the site, want = 3", got)
	}

	resp, err := c.Post(site.URL+"/api", "application/json", strings.NewReader("{}"))
	if err == nil {
		_ = resp.Body.Close()
	}
	// The host's solution is reused, so the POST passes.
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("POST got = %v, %v, want = 200", resp, err)
	}
}

func TestIsCloudflareProtected(t *testing.T) {
	cf := http.Header{"Server": {"cloudflare"}}
	cases := []struct {
		status int
		header http.Header
		body   string
		want   bool
	}{
		{403, cf, "<title>Just a moment...</title>", true},
		{503, cf, "<title>Attention Required! | Cloudflare</title>", true},
		{403, cf, "error code: 1020", true},
		{200, cf, "<title>Just a moment...</title>", false},
		{403, http.Header{"Server": {"nginx"}}, "<title>Just a moment...</title>", false},
		{403, http.Header{"Server": {"ddos-guard"}}, "<title>DDoS-Guard</title>", true},
	}
	for _, c := range cases {
		if got := IsCloudflareProtected(c.status, c.header, []byte(c.body)); got != c.want {
			t.Errorf("IsCloudflareProtected(%d, %v, %q) got = %v, want = %v", c.status, c.header, c.body, got, c.want)
		}
	}
}

func get(t *testing.T, c *http.Client, u string) string {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
