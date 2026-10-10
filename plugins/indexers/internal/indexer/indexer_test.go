package indexer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// handlerTransport answers requests with a handler, without a network.
type handlerTransport struct{ h http.Handler }

func (t handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, r)
	res := rec.Result()
	res.Request = r
	return res, nil
}

// fakeSite searches with the given chain and reads one release per line
// of its responses, "title size" with size omitted for none.
type fakeSite struct {
	info  Info
	caps  *Capabilities
	chain func(q *Query) *Chain
	login func(ctx context.Context, ix *Indexer) error
}

func (s *fakeSite) Info() Info { return s.info }

func (s *fakeSite) Capabilities(context.Context, *Indexer) (*Capabilities, error) {
	if s.caps == nil {
		return NewCapabilities(), nil
	}
	return s.caps, nil
}

func (s *fakeSite) SearchRequests(_ context.Context, _ *Indexer, q *Query) (*Chain, error) {
	return s.chain(q), nil
}

func (s *fakeSite) ParseResponse(_ context.Context, _ *Indexer, r *Response) ([]*Release, error) {
	var out []*Release
	for line := range strings.Lines(r.Content()) {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		rel := &Release{Title: f[0], GUID: f[0], Categories: []int{2000}}
		if len(f) > 1 {
			var n int64
			if _, err := fmt.Sscan(f[1], &n); err != nil {
				return nil, err
			}
			rel.Size = &n
		}
		out = append(out, rel)
	}
	return out, nil
}

type loginSite struct{ *fakeSite }

func (s loginSite) LoginNeeded(r *Response) bool { return r.StatusCode == http.StatusUnauthorized }

func (s loginSite) Login(ctx context.Context, ix *Indexer) error { return s.login(ctx, ix) }

func newIndexer(t *testing.T, site Site, h http.Handler) *Indexer {
	t.Helper()
	st, err := NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := NewClient()
	c.Transport = handlerTransport{h}
	return &Indexer{ID: "1", Name: "fake", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func pages(urls ...string) *Chain {
	var c Chain
	var rs []*Request
	for _, u := range urls {
		rs = append(rs, NewRequest(u, AcceptRSS))
	}
	c.AddRequests(rs...)
	return &c
}

func titles(rs []*Release) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Title)
	}
	return out
}

func TestSearchTiersAndPages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var got []string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = append(got, r.URL.Path)
			switch r.URL.Path {
			case "/p1":
				fmt.Fprint(w, "a 1\nb 2\n")
			case "/p2":
				fmt.Fprint(w, "c 3\n")
			case "/empty":
			}
		})
		site := &fakeSite{info: Info{SupportsSearch: true, SupportsRSS: true, PageSize: 2}, chain: func(*Query) *Chain {
			var c Chain
			c.AddRequests(NewRequest("http://site/empty", ""))
			c.AddTier()
			c.AddRequests(NewRequest("http://site/p1", ""), NewRequest("http://site/p2", ""), NewRequest("http://site/p3", ""))
			c.AddTier()
			c.AddRequests(NewRequest("http://site/never", ""))
			return &c
		}}
		ix := newIndexer(t, site, h)
		rs, err := ix.Search(t.Context(), &Query{Term: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"a", "b", "c"}; !slices.Equal(titles(rs), want) {
			t.Errorf("releases got = %v, want = %v", titles(rs), want)
		}
		if want := []string{"/empty", "/p1", "/p2"}; !slices.Equal(got, want) {
			t.Errorf("requests got = %v, want = %v", got, want)
		}
	})
}

func TestSearchValidAndCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "a 1\nnosize\na 2\n")
		})
		site := &fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/") }}
		ix := newIndexer(t, site, h)
		for _, tt := range []struct {
			interactive bool
			want        []string
		}{{false, []string{"a"}}, {true, []string{"a", "nosize"}}} {
			rs, err := ix.Search(t.Context(), &Query{Term: "x", Interactive: tt.interactive})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(titles(rs), tt.want) {
				t.Errorf("interactive %v: got = %v, want = %v", tt.interactive, titles(rs), tt.want)
			}
		}
	})
}

func TestSearchRetries(t *testing.T) {
	for _, tt := range []struct {
		name      string
		failures  int
		status    int
		wantErr   bool
		wantCalls int
	}{
		{"recovers", 2, http.StatusServiceUnavailable, false, 3},
		{"gives up", 3, http.StatusBadGateway, true, 3},
		{"timeout", 1, http.StatusRequestTimeout, false, 2},
		{"client error", 1, http.StatusNotFound, false, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls <= tt.failures {
						w.WriteHeader(tt.status)
						return
					}
					fmt.Fprint(w, "a 1\n")
				})
				site := &fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/") }}
				ix := newIndexer(t, site, h)
				_, err := ix.Search(t.Context(), &Query{Term: "x"})
				if (err != nil) != tt.wantErr {
					t.Errorf("error got = %v, want error = %v", err, tt.wantErr)
				}
				if calls != tt.wantCalls {
					t.Errorf("calls got = %d, want = %d", calls, tt.wantCalls)
				}
				if _, _, disabled := ix.Statuses.Disabled("1"); disabled != tt.wantErr {
					t.Errorf("disabled got = %v, want = %v", disabled, tt.wantErr)
				}
			})
		})
	}
}

func TestSearchRateLimited(t *testing.T) {
	for _, tt := range []struct {
		retryAfter string
		want       time.Duration
	}{{"", time.Hour}, {"120", 5 * time.Minute}, {"7200", 3 * time.Hour}} {
		synctest.Test(t, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			})
			site := &fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/") }}
			ix := newIndexer(t, site, h)
			_, err := ix.Search(t.Context(), &Query{Term: "x"})
			var rl *RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("error got = %v, want = RateLimitError", err)
			}
			until, _, _ := ix.Statuses.Disabled("1")
			if got := time.Until(until); got != tt.want {
				t.Errorf("Retry-After %q: disabled for got = %v, want = %v", tt.retryAfter, got, tt.want)
			}
			if _, err := ix.Search(t.Context(), &Query{Term: "x"}); !errors.Is(err, ErrDisabled) {
				t.Errorf("second search error got = %v, want = ErrDisabled", err)
			}
		})
	}
}

func TestSearchLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login":
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok"})
			default:
				if c, err := r.Cookie("session"); err != nil || c.Value != "ok" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				fmt.Fprint(w, "a 1\n")
			}
		})
		logins := 0
		site := loginSite{&fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/search") }}}
		site.login = func(ctx context.Context, ix *Indexer) error {
			logins++
			resp, err := ix.Do(ctx, NewRequest("http://site/login", ""))
			if err != nil {
				return err
			}
			ix.SetCookies(resp.Cookies, time.Time{})
			return nil
		}
		ix := newIndexer(t, site, h)
		for range 2 {
			rs, err := ix.Search(t.Context(), &Query{Term: "x"})
			if err != nil || len(rs) != 1 {
				t.Fatalf("Search() got = %v, %v, want = 1 release", titles(rs), err)
			}
		}
		if logins != 1 {
			t.Errorf("logins got = %d, want = 1", logins)
		}
	})
}

func TestSearchUnsupportedParams(t *testing.T) {
	caps := NewCapabilities()
	if err := caps.AddParams(SearchMovie, []string{"q", "imdbid"}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		q    Query
		want bool
	}{
		{Query{Type: SearchMovie, IMDbID: "tt0000001"}, true},
		{Query{Type: SearchMovie, TMDBID: 1}, false},
		{Query{Type: SearchBasic, Term: "x"}, true},
	} {
		if got := supports(caps, &tt.q); got != tt.want {
			t.Errorf("supports(%+v) got = %v, want = %v", tt.q, got, tt.want)
		}
	}
}

func TestQueryLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "a 1\n") })
		site := &fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/") }}
		ix := newIndexer(t, site, h)
		ix.Limits = Limits{Queries: 2, Per: time.Hour}
		for i, want := range []error{nil, nil, ErrLimitReached} {
			if _, err := ix.Search(t.Context(), &Query{Term: "x"}); !errors.Is(err, want) {
				t.Errorf("search %d error got = %v, want = %v", i, err, want)
			}
		}
		time.Sleep(time.Hour)
		if _, err := ix.Search(t.Context(), &Query{Term: "x"}); err != nil {
			t.Errorf("search after an hour error got = %v, want = nil", err)
		}
	})
}

func TestCleanup(t *testing.T) {
	ix := &Indexer{Site: &fakeSite{info: Info{Protocol: ProtocolTorrent}}}
	rs := ix.cleanup([]*Release{
		{Title: "a", DownloadURL: "http://d/1", DownloadVolumeFactor: Ptr(0.0), UploadVolumeFactor: Ptr(2.0)},
		{Title: "b", MagnetURL: "magnet:?xt=urn:btih:1", DownloadVolumeFactor: Ptr(0.5), UploadVolumeFactor: Ptr(0.0), Scene: true},
		{Title: "c", InfoURL: "http://i/1"},
		{Title: "a again", DownloadURL: "http://d/1"},
	})
	want := []struct {
		guid  string
		flags []Flag
	}{
		{"http://d/1", []Flag{FlagFreeleech, FlagDoubleUpload}},
		{"magnet:?xt=urn:btih:1", []Flag{FlagHalfLeech, FlagNeutralLeech, FlagScene}},
		{"http://i/1", nil},
	}
	if len(rs) != len(want) {
		t.Fatalf("cleanup() got = %v, want = 3 releases", titles(rs))
	}
	for i, w := range want {
		if rs[i].GUID != w.guid || !slices.Equal(rs[i].Flags, w.flags) || rs[i].Protocol != ProtocolTorrent {
			t.Errorf("release %d got = %q %v, want = %q %v", i, rs[i].GUID, rs[i].Flags, w.guid, w.flags)
		}
	}
}

func TestFilterByQuery(t *testing.T) {
	rs := []*Release{{Title: "The.Matrix.1999"}, {Title: "Matrix Reloaded"}, {Title: "Other", Description: "the matrix 1999"}, {Title: "Nothing"}}
	for _, tt := range []struct {
		term string
		want []string
	}{
		{"matrix", []string{"The.Matrix.1999", "Matrix Reloaded", "Other"}},
		{"The Matrix 1999", []string{"The.Matrix.1999", "Other"}},
		{"", []string{"The.Matrix.1999", "Matrix Reloaded", "Other", "Nothing"}},
	} {
		if got := titles(FilterByQuery(rs, &Query{Term: tt.term})); !slices.Equal(got, tt.want) {
			t.Errorf("FilterByQuery(%q) got = %v, want = %v", tt.term, got, tt.want)
		}
	}
}

func TestDownload(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/file":
			fmt.Fprint(w, "d8:announce")
		case "/redirect":
			http.Redirect(w, r, "/file", http.StatusFound)
		case "/magnet":
			http.Redirect(w, r, "magnet:?xt=urn:btih:abc", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>")
		default:
			http.NotFound(w, r)
		}
	})
	for _, tt := range []struct {
		link    string
		want    string
		wantErr error
	}{
		{"http://site/file", "d8:announce", nil},
		{"http://site/redirect", "d8:announce", nil},
		{"http://site/magnet", "magnet:?xt=urn:btih:abc", nil},
		{"magnet:?xt=urn:btih:def", "magnet:?xt=urn:btih:def", nil},
		{"http://site/gone", "", ErrReleaseUnavailable},
		{"http://site/loop", "", errAny},
		{"http://site/page", "", errAny},
		{"magnet:?dn=x", "", errAny},
	} {
		synctest.Test(t, func(t *testing.T) {
			ix := newIndexer(t, &fakeSite{}, h)
			got, err := ix.Download(t.Context(), tt.link)
			switch {
			case tt.wantErr == nil && err != nil, tt.wantErr != nil && err == nil,
				tt.wantErr != nil && !errors.Is(tt.wantErr, errAny) && !errors.Is(err, tt.wantErr):
				t.Errorf("Download(%q) error got = %v, want = %v", tt.link, err, tt.wantErr)
			case string(got) != tt.want:
				t.Errorf("Download(%q) got = %q, want = %q", tt.link, got, tt.want)
			}
		})
	}
}

var errAny = errors.New("any error")

func TestIsCloudflareProtected(t *testing.T) {
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
		r := &Response{StatusCode: tt.status, Header: http.Header{"Server": {tt.server}}, Body: []byte(tt.body)}
		if tt.vary != "" {
			r.Header.Set("Vary", tt.vary)
		}
		if got := IsCloudflareProtected(r); got != tt.want {
			t.Errorf("IsCloudflareProtected(%s %d %q) got = %v, want = %v", tt.server, tt.status, tt.body, got, tt.want)
		}
	}
}

func TestFlareSolverr(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Host {
			case "solver":
				fmt.Fprint(w, `{"status":"ok","solution":{"userAgent":"Solved/1","cookies":[{"name":"cf_clearance","value":"yes"}]}}`)
			default:
				if c, err := r.Cookie("cf_clearance"); err == nil && c.Value == "yes" && r.UserAgent() == "Solved/1" {
					fmt.Fprint(w, "a 1\n")
					return
				}
				w.Header().Set("Server", "cloudflare")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "<title>Just a moment...</title>")
			}
		})
		site := &fakeSite{info: Info{SupportsSearch: true}, chain: func(*Query) *Chain { return pages("http://site/") }}
		ix := newIndexer(t, site, h)
		if _, err := ix.Search(t.Context(), &Query{Term: "x"}); !errors.Is(err, ErrCloudflare) {
			t.Errorf("without FlareSolverr error got = %v, want = ErrCloudflare", err)
		}
		ix = newIndexer(t, site, h)
		ix.Session.solver = &Proxy{Kind: ProxyFlareSolverr, Host: "http://solver"}
		rs, err := ix.Search(t.Context(), &Query{Term: "x"})
		if err != nil || len(rs) != 1 {
			t.Errorf("with FlareSolverr got = %v, %v, want = 1 release", titles(rs), err)
		}
	})
}

func TestRateLimitSpacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var at []time.Time
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			at = append(at, time.Now())
			fmt.Fprint(w, "a 1\nb 1\n")
		})
		site := &fakeSite{info: Info{SupportsSearch: true, PageSize: 2, RateLimit: 3 * time.Second}, chain: func(*Query) *Chain {
			return pages("http://site/1", "http://site/2", "http://site/3")
		}}
		ix := newIndexer(t, site, h)
		if _, err := ix.Search(t.Context(), &Query{Term: "x"}); err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(at); i++ {
			if got := at[i].Sub(at[i-1]); got != 3*time.Second {
				t.Errorf("gap %d got = %v, want = 3s", i, got)
			}
		}
	})
}

func TestSelectProxies(t *testing.T) {
	proxies := []Proxy{
		{Kind: ProxySOCKS5, Host: "a:1", Tags: []string{"x"}},
		{Kind: ProxyFlareSolverr, Host: "http://fs", Tags: []string{"y"}},
		{Kind: ProxyHTTP, Host: "b:2", Tags: []string{"y"}},
	}
	for _, tt := range []struct {
		tags          []string
		proxy, solver string
	}{
		{[]string{"x", "y"}, "a:1", "http://fs"},
		{[]string{"y"}, "b:2", "http://fs"},
		{nil, "", ""},
	} {
		p, s := SelectProxies(proxies, tt.tags)
		var gp, gs string
		if p != nil {
			gp = p.Host
		}
		if s != nil {
			gs = s.Host
		}
		if gp != tt.proxy || gs != tt.solver {
			t.Errorf("SelectProxies(%v) got = %q %q, want = %q %q", tt.tags, gp, gs, tt.proxy, tt.solver)
		}
	}
}

func TestSOCKS4(t *testing.T) {
	for _, tt := range []struct {
		host string
		want []byte
	}{
		{"10.0.0.1", []byte{4, 1, 0, 80, 10, 0, 0, 1, 'u', 0}},
		{"example.org", append([]byte{4, 1, 0, 80, 0, 0, 0, 1, 'u', 0}, "example.org\x00"...)},
	} {
		client, server := net.Pipe()
		got := make(chan []byte, 1)
		go func() {
			buf := make([]byte, 64)
			n, _ := server.Read(buf)
			got <- buf[:n]
			_, _ = server.Write([]byte{0, 0x5a, 0, 0, 0, 0, 0, 0})
		}()
		d := &socks4Dialer{user: "u"}
		if err := d.connect(client, tt.host, 80); err != nil {
			t.Errorf("connect(%s) error = %v", tt.host, err)
		}
		if g := <-got; !slices.Equal(g, tt.want) {
			t.Errorf("connect(%s) sent got = %v, want = %v", tt.host, g, tt.want)
		}
		client.Close()
		server.Close()
	}
}
