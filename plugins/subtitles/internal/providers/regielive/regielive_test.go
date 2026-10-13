package regielive

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func makeZip(t *testing.T, files ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n" + name + "\r\n"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const results = `{"rezultate":{"123":{"subtitrari":{
	"2":{"titlu":"Show.S01E02.720p.HDTV.x264-KILLERS","url":"%s/dl/2","rating":{"nota":4.5}},
	"1":{"titlu":"Show.S01E02.1080p.WEB-NTb","url":"%s/dl/1","rating":{"nota":"5"}}}}}}`

func newServer(t *testing.T, search string, status int, query *url.Values) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bazarr/search.php":
			if r.Header.Get("RL-API") != apiKey {
				t.Errorf("RL-API got = %q, want = %q", r.Header.Get("RL-API"), apiKey)
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != r.URL.RawQuery {
				t.Errorf("body got = %q, want = %q", body, r.URL.RawQuery)
			}
			*query = r.URL.Query()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(strings.ReplaceAll(search, "%s", srv.URL)))
		case "/dl/1":
			if r.Header.Get("Referer") != "https://subtitrari.regielive.ro" || r.Header.Get("User-Agent") != downloadAgent {
				t.Errorf("download headers got = %v", r.Header)
			}
			_, _ = w.Write(makeZip(t, ".hidden.srt", "readme.txt", "Show.S01E02.SRT"))
		case "/dl/nosub":
			_, _ = w.Write(makeZip(t, "readme.txt"))
		case "/dl/500":
			_, _ = w.Write([]byte("500"))
		case "/dl/html":
			_, _ = w.Write([]byte("<html>\nnope</html>"))
		case "/dl/429":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(t *testing.T, srv *httptest.Server) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	episode := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, Year: 2010, ReleaseGroup: "killers"}
	movie := &provider.Video{Title: "Film"}
	cases := []struct {
		name      string
		v         *provider.Video
		search    string
		status    int
		langs     []provider.Language
		wantQuery url.Values
		wantIDs   []string
		wantMatch [][]string
		wantKind  provider.Kind
	}{
		{
			name: "episode", v: episode, search: results, status: 200, langs: []provider.Language{{Tag: "ro"}},
			wantQuery: url.Values{"nume": {"Show"}, "sezon": {"1"}, "episod": {"2"}, "an": {"2010"}},
			wantIDs:   []string{"/dl/1", "/dl/2"},
			wantMatch: [][]string{
				{"title", "series", "season", "episode", "year"},
				{"title", "series", "season", "episode", "year", "release_group", "hash"},
			},
		},
		{
			name: "movie without year", v: movie, search: `[]`, status: 200, langs: []provider.Language{{Tag: "ro"}},
			wantQuery: url.Values{"nume": {"Film"}, "an": {"None"}},
		},
		{name: "other language", v: movie, langs: []provider.Language{{Tag: "en"}}},
		{name: "status", v: movie, search: results, status: 404, langs: []provider.Language{{Tag: "ro"}}, wantQuery: url.Values{"nume": {"Film"}, "an": {"None"}}},
		{name: "too many requests", v: movie, status: 429, langs: []provider.Language{{Tag: "ro"}}, wantKind: provider.TooManyRequests},
		{name: "bad JSON", v: movie, search: "<html>", status: 200, langs: []provider.Language{{Tag: "ro"}}, wantKind: provider.ProviderError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var query url.Values
			srv := newServer(t, c.search, c.status, &query)
			got, err := newProvider(t, srv).Search(t.Context(), c.v, c.langs)
			if c.wantKind != provider.Other {
				if provider.KindOf(err) != c.wantKind {
					t.Fatalf("Search got = %v, want = %v", err, c.wantKind)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.wantQuery != nil && query.Encode() != c.wantQuery.Encode() {
				t.Errorf("query got = %v, want = %v", query, c.wantQuery)
			}
			var ids []string
			for i, s := range got {
				ids = append(ids, strings.TrimPrefix(s.ID, srv.URL))
				if !slices.Equal(s.Matches, c.wantMatch[i]) {
					t.Errorf("matches of %s got = %v, want = %v", s.ID, s.Matches, c.wantMatch[i])
				}
				if s.Language.Tag != "ro" || len(s.ReleaseNames) != 1 || s.PageLink != s.ID {
					t.Errorf("subtitle got = %+v", s)
				}
			}
			if !slices.Equal(ids, c.wantIDs) {
				t.Errorf("IDs got = %v, want = %v", ids, c.wantIDs)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	var query url.Values
	srv := newServer(t, "", 200, &query)
	p := newProvider(t, srv)
	f, err := p.Download(t.Context(), nil, srv.URL+"/dl/1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nShow.S01E02.SRT\n"; string(f.Data) != want || f.Format != "srt" || f.Language.Tag != "ro" {
		t.Errorf("Download got = %q %q, want = %q srt", f.Data, f.Format, want)
	}
	for _, c := range []struct {
		path string
		want provider.Kind
	}{
		{"/dl/nosub", provider.ProviderError},
		{"/dl/500", provider.ProviderError},
		{"/dl/html", provider.ProviderError},
		{"/dl/missing", provider.ProviderError},
		{"/dl/429", provider.TooManyRequests},
	} {
		if _, err := p.Download(t.Context(), nil, srv.URL+c.path); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.path, err, c.want)
		}
	}
}
