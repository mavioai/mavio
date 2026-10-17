package challenge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestIsProtected(t *testing.T) {
	for _, tt := range []struct {
		server string
		status int
		vary   string
		body   string
		want   bool
	}{
		{"cloudflare", 503, "", "<title>Just a moment...</title>", true},
		{"cloudflare-nginx", 403, "", " error code: 1020 ", true},
		{"ddos-guard", 403, "", "<title>DDoS-Guard</title>", true},
		{"cloudflare", 200, "Accept-Encoding,User-Agent", "DDoS protection", true},
		{"cloudflare", 200, "", "<title>Just a moment...</title>", false},
		{"nginx", 503, "", "<title>Just a moment...</title>", false},
	} {
		h := http.Header{"Server": {tt.server}}
		if tt.vary != "" {
			h.Set("Vary", tt.vary)
		}
		if got := IsProtected(tt.status, h, []byte(tt.body)); got != tt.want {
			t.Errorf("IsProtected(%s %d %q) got = %v, want = %v", tt.server, tt.status, tt.body, got, tt.want)
		}
	}
}

type solver struct {
	reqs []*pluginv1.SolveChallengeRequest
	err  error
}

func (s *solver) SolveChallenge(_ context.Context, req *pluginv1.SolveChallengeRequest) (*pluginv1.SolveChallengeResponse, error) {
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return pluginv1.SolveChallengeResponse_builder{Solution: pluginv1.ChallengeSolution_builder{
		UserAgent: new("browser"),
		Cookies:   []*pluginv1.Cookie{pluginv1.Cookie_builder{Name: new("cf_clearance"), Value: new("ok")}.Build()},
	}.Build()}.Build(), nil
}

func TestTransport(t *testing.T) {
	var hits atomic.Int32
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Server", "cloudflare")
		if ck, err := r.Cookie("cf_clearance"); err == nil && ck.Value == "ok" && r.UserAgent() == "browser" {
			b, _ := io.ReadAll(r.Body)
			_, _ = io.WriteString(w, "page "+string(b))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "<html><title>Just a moment...</title></html>")
	}))
	defer site.Close()
	s := &solver{}
	tr := &Transport{Next: http.DefaultTransport, Solver: s, ProxyID: "p"}
	c := &http.Client{Transport: tr}
	resp, err := c.Post(site.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("a=1"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(b) != "page a=1" {
		t.Errorf("body got = %q, want = page a=1", b)
	}
	if len(s.reqs) != 1 {
		t.Fatalf("solved %d times, want = 1", len(s.reqs))
	}
	ch := s.reqs[0].GetChallenge()
	if s.reqs[0].GetProxyId() != "p" || ch.GetMethod() != "POST" || string(ch.GetBody()) != "a=1" ||
		ch.GetContentType() != "application/x-www-form-urlencoded" || ch.GetUrl() != site.URL+"/login" {
		t.Errorf("request got = %v", s.reqs[0])
	}
	// The host's later requests send the solution at once.
	hits.Store(0)
	resp, err = c.Get(site.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if hits.Load() != 1 || len(s.reqs) != 1 {
		t.Errorf("later request: %d hits, %d solves; want = 1, 1", hits.Load(), len(s.reqs))
	}
}

func TestTransportUnsolved(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "<title>Just a moment...</title>")
	}))
	defer site.Close()
	s := &solver{err: connect.NewError(connect.CodeDeadlineExceeded, nil)}
	_, err := (&http.Client{Transport: &Transport{Next: http.DefaultTransport, Solver: s}}).Get(site.URL)
	if err == nil || !strings.Contains(err.Error(), ErrUnsolved.Error()) {
		t.Errorf("got = %v, want = ErrUnsolved", err)
	}
	// Without a solver, or without a solver plugin, the challenge is
	// returned.
	for _, s := range []Solver{nil, &solver{err: connect.NewError(connect.CodeUnavailable, nil)}} {
		resp, err := (&http.Client{Transport: &Transport{Next: http.DefaultTransport, Solver: s}}).Get(site.URL)
		if err != nil {
			t.Fatalf("solver %v: got = %v, want = 503", s, err)
		}
		page, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(page), "Just a moment") {
			t.Errorf("solver %v: got = %d %q, want = 503 and the challenge", s, resp.StatusCode, page)
		}
	}
}

func TestApply(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://site/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "s"})
	r.AddCookie(&http.Cookie{Name: "cf_clearance", Value: "old"})
	Apply(r, pluginv1.ChallengeSolution_builder{
		UserAgent: new("browser"),
		Cookies:   []*pluginv1.Cookie{pluginv1.Cookie_builder{Name: new("cf_clearance"), Value: new("new")}.Build()},
	}.Build())
	if got, want := r.Header.Get("Cookie"), "session=s; cf_clearance=new"; got != want {
		t.Errorf("Cookie got = %q, want = %q", got, want)
	}
	if r.UserAgent() != "browser" {
		t.Errorf("User-Agent got = %q, want = browser", r.UserAgent())
	}
}
