package subscenter

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nשלום\r\n"

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(srt))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// list is the site's list of subtitles: by language, of a kind, by
// quality, the same subtitle under two releases.
const list = `{
	"he": {"1": {"720p": {
		"a": {"hearing_impaired": 0, "id": 10, "key": "k10", "h_version": "1", "downloaded": 5, "subtitle_version": "Show.S01E02.720p.HDTV.x264-KILLERS"},
		"b": {"hearing_impaired": 1, "id": 11, "key": "k11", "h_version": "2", "downloaded": 1, "subtitle_version": "Show.S01E02.720p.WEB-DL"}},
		"1080p": {"c": {"hearing_impaired": 0, "id": 10, "key": "k10", "h_version": "1", "downloaded": 2, "subtitle_version": "Show.S01E02.1080p.HDTV"}}}},
	"en": {"1": {"720p": {"d": {"hearing_impaired": false, "id": 12, "key": "k12", "h_version": "1", "downloaded": 0, "subtitle_version": "x"}}}}
}`

type fakeSite struct {
	srv      *httptest.Server
	searches atomic.Int32
	download []byte
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{download: zipOf(t, "Show.S01E02.srt", "info.txt")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /he/subtitle/search/", func(w http.ResponseWriter, r *http.Request) {
		f.searches.Add(1)
		switch r.URL.Query().Get("q") {
		case "The Show":
			_, _ = w.Write([]byte(`<div id="processes">
				<div class="generalWindowTop"><a href="/he/subtitle/series/the-show/">The Show</a></div></div>`))
		case "Up":
			// A single result redirects to its page.
			http.Redirect(w, r, "/he/subtitle/movie/up/", http.StatusFound)
		}
	})
	mux.HandleFunc("GET /he/subtitle/movie/up/", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /he/cst/data/series/sb/the-show/1/2/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list))
	})
	mux.HandleFunc("GET /he/cst/data/movie/sb/up/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"he": {"1": {"720p": {"a": {"hearing_impaired": 0, "id": 20, "key": "k", "h_version": "3", "downloaded": 9, "subtitle_version": "Up.2009.720p"}}}}}`))
	})
	mux.HandleFunc("GET /he/subtitle/download/he/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" || r.URL.Query().Get("v") == "" || !strings.Contains(r.Header.Get("Referer"), "/he/subtitle/") {
			t.Errorf("download request got = %s %v", r.URL, r.Header)
		}
		_, _ = w.Write(f.download)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	hebrew := []provider.Language{{Tag: "he"}}
	for _, c := range []struct {
		name  string
		video *provider.Video
		want  []string
	}{
		{
			"episode", &provider.Video{Episode: true, Series: "The Show", Title: "Pilot", Season: 1, Number: 2},
			[]string{
				"10 false [Show.S01E02.1080p.HDTV Show.S01E02.720p.HDTV.x264-KILLERS] 7 [series season episode]",
				"11 true [Show.S01E02.720p.WEB-DL] 1 [series season episode]",
			},
		},
		{
			// A movie the search redirects to.
			"movie", &provider.Video{Title: "Up", Year: 2009},
			[]string{"20 false [Up.2009.720p] 9 [title]"},
		},
		{"not found", &provider.Video{Title: "Nothing"}, nil},
		// The series is no movie.
		{"no movie", &provider.Video{Title: "The Show"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), c.video, hebrew)
			var got []string
			for _, s := range subs {
				q, _ := url.ParseQuery(s.ID)
				got = append(got, fmt.Sprintf("%s %v %v %d %v", q.Get("id"), s.Language.HearingImpaired, s.ReleaseNames, s.Downloads, s.Matches))
			}
			if err != nil || !slices.Equal(got, c.want) {
				t.Errorf("got = %v, %v, want = %v", got, err, c.want)
			}
		})
	}
	// The URL titles are cached.
	n := f.searches.Load()
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, hebrew); err != nil || f.searches.Load() != n {
		t.Errorf("searches got = %d, %v, want = %d", f.searches.Load(), err, n)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "The Show", Season: 1, Number: 2}, []provider.Language{{Tag: "he"}})
	if err != nil || len(subs) == 0 {
		t.Fatalf("Search got = %v, %v", subs, err)
	}
	file, err := p.Download(t.Context(), nil, subs[0].ID)
	if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" || file.Language.Tag != "he" {
		t.Errorf("Download got = %+v, %v, want = the subtitle", file, err)
	}
	for _, c := range []struct {
		data []byte
		kind provider.Kind
	}{
		{[]byte("<html>limit</html>"), provider.ProviderError},
		{zipOf(t, "a.srt", "b.srt"), provider.ProviderError},
		{zipOf(t, "readme.txt"), provider.BadSubtitle},
	} {
		f.download = c.data
		if _, err := p.Download(t.Context(), nil, subs[0].ID); provider.KindOf(err) != c.kind {
			t.Errorf("Download got = %v, want = %v", err, c.kind)
		}
	}
	if _, err := p.Download(t.Context(), nil, "id=1"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download of a malformed ID got = %v, want = %v", err, provider.BadSubtitle)
	}
}
