package subsynchro

import (
	"archive/zip"
	"bytes"
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

func makeZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range names {
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

const results = `{"status":200,"data":[
	{"titre":"Le Parrain","titre_original":"The Godfather","release":"The.Godfather.1972.1080p.BluRay.x264-AMIABLE","filename":"godfather.srt","telechargement":"%s/dl/1.zip","fichier":"zip"},
	{"titre":"Autre","titre_original":"Other","filename":"The.Godfather.1972.REMASTERED.720p.BluRay.x264-SiNNERS.srt","telechargement":"/dl/2.zip"}
]}`

func newServer(t *testing.T, search string, query *url.Values) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Bazarr" || r.Header.Get("Referer") != srv.URL {
			t.Errorf("headers got = %v", r.Header)
		}
		switch r.URL.Path {
		case "/include/ajax/subMarin.php":
			*query = r.URL.Query()
			_, _ = w.Write([]byte(strings.ReplaceAll(search, "%s", srv.URL)))
		case "/dl/1.zip":
			_, _ = w.Write(makeZip(t, "__MACOSX/._godfather.srt", "readme.nfo", "godfather.srt"))
		case "/dl/nosub.zip":
			_, _ = w.Write(makeZip(t, "readme.nfo"))
		case "/dl/plain.srt":
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nx\n"))
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
	godfather := &provider.Video{Title: "The Godfather", Year: 1972}
	cases := []struct {
		name      string
		v         *provider.Video
		search    string
		langs     []provider.Language
		wantQuery url.Values
		want      []provider.Subtitle
	}{
		{
			name: "movie", v: godfather, search: results, langs: []provider.Language{{Tag: "fr"}},
			wantQuery: url.Values{"title": {"The Godfather"}, "year": {"1972"}},
			want: []provider.Subtitle{
				{
					ID: "/dl/1.zip", Language: provider.Language{Tag: "fr"}, PageLink: "/dl/1.zip", Matches: []string{"title", "year"},
					ReleaseNames: []string{"godfather.srt", "The.Godfather.1972.1080p.BluRay.x264-AMIABLE"},
				},
				{
					ID: "/dl/2.zip", Language: provider.Language{Tag: "fr"}, PageLink: "/dl/2.zip",
					ReleaseNames: []string{"The.Godfather.1972.REMASTERED.720p.BluRay.x264-SiNNERS.srt"},
				},
			},
		},
		{
			name: "no subtitles", v: &provider.Video{Title: "Up"}, search: `{"status":404,"data":[]}`, langs: []provider.Language{{Tag: "fr"}},
			wantQuery: url.Values{"title": {"Up"}, "year": {"None"}},
		},
		{name: "not JSON", v: godfather, search: `<html>`, langs: []provider.Language{{Tag: "fr"}}, wantQuery: url.Values{"title": {"The Godfather"}, "year": {"1972"}}},
		{name: "other language", v: godfather, langs: []provider.Language{{Tag: "en"}}},
		{name: "episode", v: &provider.Video{Episode: true}, langs: []provider.Language{{Tag: "fr"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var query url.Values
			srv := newServer(t, c.search, &query)
			got, err := newProvider(t, srv).Search(t.Context(), c.v, c.langs)
			if err != nil {
				t.Fatal(err)
			}
			if query.Encode() != c.wantQuery.Encode() {
				t.Errorf("query got = %v, want = %v", query, c.wantQuery)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got = %+v, want = %+v", got, c.want)
			}
			for i, s := range got {
				w := c.want[i]
				if strings.TrimPrefix(s.ID, srv.URL) != w.ID || strings.TrimPrefix(s.PageLink, srv.URL) != w.PageLink || s.Language != w.Language ||
					!slices.Equal(s.Matches, w.Matches) || !slices.Equal(s.ReleaseNames, w.ReleaseNames) {
					t.Errorf("subtitle %d got = %+v, want = %+v", i, s, w)
				}
			}
		})
	}
}

func TestDownload(t *testing.T) {
	var query url.Values
	srv := newServer(t, "", &query)
	p := newProvider(t, srv)
	f, err := p.Download(t.Context(), nil, srv.URL+"/dl/1.zip")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\ngodfather.srt\n"; string(f.Data) != want || f.Format != "srt" || f.Language.Tag != "fr" {
		t.Errorf("Download got = %q %q, want = %q srt", f.Data, f.Format, want)
	}
	for _, c := range []struct {
		path string
		want provider.Kind
	}{
		{"/dl/nosub.zip", provider.APIThrottled},
		{"/dl/plain.srt", provider.APIThrottled},
		{"/dl/missing", provider.ProviderError},
	} {
		if _, err := p.Download(t.Context(), nil, srv.URL+c.path); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.path, err, c.want)
		}
	}
}
