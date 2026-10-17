package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// httpProxy returns the server's HTTP proxy at a URL.
func httpProxy(t *testing.T, id, rawURL string) *pluginv1.Proxy {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())
	return pluginv1.Proxy_builder{
		Id: proto.String(id), Kind: pluginv1.ProxyKind_PROXY_KIND_HTTP.Enum(), Host: proto.String(u.Hostname()), Port: proto.Int32(int32(port)),
		Username: proto.String("u"), Password: proto.String("p"),
	}.Build()
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
	p := httpProxy(t, "p1", srv.URL)
	c := NewClient(Config{Proxy: p, Exclude: []string{"excluded.test"}}, 5*time.Second)
	if got := get(t, c, "http://subtitles.test/a"); got != "proxied" {
		t.Errorf("got = %q, want = proxied", got)
	}
	if got, want := proxied.Load(), "http://subtitles.test/a Basic dTpw"; got != want {
		t.Errorf("proxied got = %q, want = %q", got, want)
	}
	// Excluded hosts, by default 127.0.0.1, are reached directly.
	if got := get(t, NewClient(Config{Proxy: p}, 5*time.Second), site.URL); got != "direct" {
		t.Errorf("excluded got = %q, want = direct", got)
	}
	if got := get(t, NewClient(Config{Proxy: p, Exclude: []string{}}, 5*time.Second), site.URL); got != "proxied" {
		t.Errorf("none excluded got = %q, want = proxied", got)
	}
}

// solver solves challenges with a cookie.
type solver struct {
	reqs []*pluginv1.SolveChallengeRequest
}

func (s *solver) SolveChallenge(_ context.Context, req *pluginv1.SolveChallengeRequest) (*pluginv1.SolveChallengeResponse, error) {
	s.reqs = append(s.reqs, req)
	sol := pluginv1.ChallengeSolution_builder{
		UserAgent: proto.String("browser"),
		Cookies:   []*pluginv1.Cookie{pluginv1.Cookie_builder{Name: proto.String("cf_clearance"), Value: proto.String("ok")}.Build()},
	}.Build()
	return pluginv1.SolveChallengeResponse_builder{Solution: sol}.Build(), nil
}

func TestChallenge(t *testing.T) {
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
	s := &solver{}
	// The site is excluded, but its challenge is solved through the
	// proxy all the same.
	c := NewClient(Config{Proxy: httpProxy(t, "p1", "http://127.0.0.1:1"), Solver: s}, 5*time.Second)
	for range 2 {
		if got := get(t, c, site.URL+"/search"); got != "subtitles" {
			t.Fatalf("got = %q, want = subtitles", got)
		}
	}
	if len(s.reqs) != 1 {
		t.Fatalf("got %d solves, want = 1", len(s.reqs))
	}
	if got := s.reqs[0]; got.GetProxyId() != "p1" || got.GetChallenge().GetUrl() != site.URL+"/search" || got.GetChallenge().GetMethod() != http.MethodGet {
		t.Errorf("got request = %v", got)
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
