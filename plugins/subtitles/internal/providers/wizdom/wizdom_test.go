package wizdom

import (
	"archive/zip"
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func makeZip(t *testing.T, files map[string]string, order ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nשלום\r\n"

// fakeSite is Wizdom and TMDB.
type fakeSite struct {
	srv  *httptest.Server
	mu   sync.Mutex
	hits []string
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api_key") != tmdbKey || q.Get("language") != "en" {
			t.Errorf("TMDB query got = %v", q)
		}
		switch q.Get("query") {
		case "Its a Wonderful Life":
			if q.Get("year") != "1946" {
				t.Errorf("year got = %q, want = 1946", q.Get("year"))
			}
			_, _ = w.Write([]byte(`{"results":[{"id":1585}]}`))
		default:
			_, _ = w.Write([]byte(`{"results":[]}`))
		}
	})
	mux.HandleFunc("GET /3/movie/1585", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"imdb_id":"tt0038650"}`))
	})
	mux.HandleFunc("GET /3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":1396}]}`))
	})
	mux.HandleFunc("GET /3/tv/1396/external_ids", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"imdb_id":"tt0903747"}`))
	})
	mux.HandleFunc("GET /api/releases/tt0038650", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"subs":[{"id":11,"version":"Its.a.Wonderful.Life.1946.1080p.BluRay"},{"id":12,"version":"Its.a.Wonderful.Life.1946.DVDRip"},{"id":11,"version":"dup"}]}`))
	})
	mux.HandleFunc("GET /api/releases/tt0903747", func(w http.ResponseWriter, r *http.Request) {
		// Seasons as a list, indexed by season.
		_, _ = w.Write([]byte(`{"subs":[{}, {"1":[{"id":"101","version":"Breaking.Bad.S01E01.720p"}],"2":[{"id":"102","version":"x"}]}]}`))
	})
	mux.HandleFunc("GET /api/releases/tt2", func(w http.ResponseWriter, r *http.Request) {
		// Seasons as an object, keyed by season.
		_, _ = w.Write([]byte(`{"subs":{"3":{"4":[{"id":7,"version":"Show.S03E04"}]}}}`))
	})
	mux.HandleFunc("GET /api/releases/tt500", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("GET /api/releases/tt429", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	mux.HandleFunc("GET /api/files/sub/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			t.Errorf("download without Referer")
		}
		switch r.PathValue("id") {
		case "11":
			_, _ = w.Write(makeZip(t, map[string]string{"a.nfo": "x", "a.srt": srt}, "a.nfo", "a.srt"))
		case "12":
			_, _ = w.Write(makeZip(t, map[string]string{"bad.srt": "garbage", "good.srt": srt}, "bad.srt", "good.srt"))
		case "13":
			_, _ = w.Write(makeZip(t, map[string]string{"a.txt": "x"}, "a.txt"))
		}
	})
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits = append(f.hits, r.URL.Path)
		f.mu.Unlock()
		if r.Header.Get("User-Agent") != "Subliminal/2.1" {
			t.Errorf("User-Agent got = %q", r.Header.Get("User-Agent"))
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSite) provider(t *testing.T) *Provider {
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
	he := []provider.Language{{Tag: "he"}}
	cases := []struct {
		name     string
		v        *provider.Video
		langs    []provider.Language
		wantIDs  []string
		wantRel  []string
		wantMat  []string
		wantPage string
		wantKind provider.Kind
	}{
		{
			name: "movie by TMDB", v: &provider.Video{Title: "Unknown", AlternativeTitles: []string{"It's a Wonderful Life"}, Year: 1946}, langs: he,
			wantIDs: []string{"11", "12"}, wantRel: []string{"Its.a.Wonderful.Life.1946.1080p.BluRay"}, wantMat: []string{"title"}, wantPage: "/movies/tt0038650",
		},
		{
			name: "episode, seasons in a list", v: &provider.Video{Episode: true, Series: "Breaking Bad", Season: 1, Number: 1}, langs: he,
			wantIDs: []string{"101"}, wantRel: []string{"Breaking.Bad.S01E01.720p"}, wantMat: []string{"series", "season", "episode"}, wantPage: "/series/tt0903747",
		},
		{
			name: "episode, seasons in an object", v: &provider.Video{Episode: true, Series: "Show", Season: 3, Number: 4, SeriesExternalIDs: map[string]string{"imdb": "tt2"}}, langs: he,
			wantIDs: []string{"7"}, wantRel: []string{"Show.S03E04"}, wantMat: []string{"series", "season", "episode", "series_imdb_id"}, wantPage: "/series/tt2",
		},
		{name: "episode missing", v: &provider.Video{Episode: true, Series: "Show", Season: 9, Number: 4, SeriesExternalIDs: map[string]string{"imdb": "tt2"}}, langs: he},
		{name: "no subtitles", v: &provider.Video{Title: "X", ExternalIDs: map[string]string{"imdb": "tt500"}}, langs: he},
		{name: "no IMDb ID", v: &provider.Video{Title: "Nothing"}, langs: he},
		{name: "other language", v: &provider.Video{Title: "X", ExternalIDs: map[string]string{"imdb": "tt500"}}, langs: []provider.Language{{Tag: "en"}}},
		{name: "too many requests", v: &provider.Video{Title: "X", ExternalIDs: map[string]string{"imdb": "tt429"}}, langs: he, wantKind: provider.TooManyRequests},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			got, err := f.provider(t).Search(t.Context(), c.v, c.langs)
			if provider.KindOf(err) != c.wantKind || c.wantKind == provider.Other && err != nil {
				t.Fatalf("Search got = %v, want = %v", err, c.wantKind)
			}
			var ids []string
			for _, s := range got {
				ids = append(ids, s.ID)
				if s.Language.Tag != "he" || s.PageLink != f.srv.URL+c.wantPage {
					t.Errorf("subtitle got = %+v, want = Hebrew of page %s", s, c.wantPage)
				}
			}
			if !slices.Equal(ids, c.wantIDs) {
				t.Fatalf("IDs got = %v, want = %v", ids, c.wantIDs)
			}
			if len(got) > 0 && (!slices.Equal(got[0].ReleaseNames, c.wantRel) || !slices.Equal(got[0].Matches, c.wantMat)) {
				t.Errorf("release names, matches got = %v, %v, want = %v, %v", got[0].ReleaseNames, got[0].Matches, c.wantRel, c.wantMat)
			}
		})
	}
}

func TestSearchCachesIMDbID(t *testing.T) {
	f := newFake(t)
	p := f.provider(t)
	v := &provider.Video{Title: "It's a Wonderful Life", Year: 1946}
	for range 2 {
		if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "he"}}); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, h := range f.hits {
		if h == "/3/search/movie" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("TMDB searches got = %d, want = 1", n)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := f.provider(t)
	for _, id := range []string{"11", "12"} {
		got, err := p.Download(t.Context(), nil, id)
		if err != nil {
			t.Fatal(err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\nשלום\n"; string(got.Data) != want || got.Format != "srt" || got.Language.Tag != "he" {
			t.Errorf("Download(%s) got = %q %q, want = %q srt", id, got.Data, got.Format, want)
		}
	}
	for _, id := range []string{"13", "14", "15"} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != provider.BadSubtitle {
			t.Errorf("Download(%s) got = %v, want = %v", id, err, provider.BadSubtitle)
		}
	}
}
