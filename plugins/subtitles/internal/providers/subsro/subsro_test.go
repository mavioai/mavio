package subsro

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nBună\r\n"

// fakeAPI is Subs.ro: keys in limited answer with 429; statuses answer
// the next requests.
type fakeAPI struct {
	srv *httptest.Server

	mu       sync.Mutex
	limited  map[string]bool
	statuses []int
	results  string
	requests []string
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{limited: map[string]bool{}}
	answer := func(w http.ResponseWriter, r *http.Request) bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		key := r.Header.Get("X-Subs-Api-Key")
		f.requests = append(f.requests, key+" "+r.URL.RequestURI())
		if f.limited[key] {
			w.WriteHeader(http.StatusTooManyRequests)
			return true
		}
		if len(f.statuses) > 0 {
			w.WriteHeader(f.statuses[0])
			f.statuses = f.statuses[1:]
			return true
		}
		return false
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/search/imdbid/{id}", func(w http.ResponseWriter, r *http.Request) {
		if answer(w, r) {
			return
		}
		_, _ = w.Write([]byte(strings.ReplaceAll(f.results, "BASE", f.srv.URL)))
	})
	mux.HandleFunc("GET /v1.0/subtitle/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if answer(w, r) {
			return
		}
		switch r.PathValue("id") {
		case "1":
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, n := range []string{"Show.S01E01.srt", "Show.S01E02.srt", "Show.S01E02.forced.srt"} {
				fw, _ := zw.Create(n)
				_, _ = fw.Write([]byte(srt + n))
			}
			_ = zw.Close()
			_, _ = w.Write(buf.Bytes())
		case "2":
			_, _ = w.Write([]byte(srt))
		default:
			_, _ = w.Write([]byte(`{"status":404}`))
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeAPI, keys string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"api_key":"`+keys+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

const results = `{"status":200,"count":3,"items":[
	{"id":1,"title":"Show - Sezonul 1","year":2019,"description":"Show.S01.1080p.WEB-DL","downloadLink":"BASE/v1.0/subtitle/1/download"},
	{"id":2,"title":"Show","year":"2019","description":"Show.S02E05.720p","downloadLink":"BASE/v1.0/subtitle/2/download"},
	{"id":3,"title":"Show","year":2019,"description":"no link"}]}`

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.results = results
	p := newProvider(t, f, "k1")
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, Year: 2019, SeriesExternalIDs: map[string]string{"imdb": "tt0123456"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"k1 /v1.0/search/imdbid/0123456?language=ro", "k1 /v1.0/search/imdbid/0123456?language=en"}
	if !slices.Equal(f.requests, want) {
		t.Errorf("requests got = %v, want = %v", f.requests, want)
	}
	// Each language is searched; subtitles without links are left out.
	if len(subs) != 4 || subs[0].Language.Tag != "ro" || subs[2].Language.Tag != "en" {
		t.Fatalf("subtitles got = %+v", subs)
	}
	if got, want := subs[0].Matches, []string{"year", "series", "imdb_id", "season", "episode"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}
	if got, want := subs[0].ReleaseNames, []string{"Show - Sezonul 1 2019 Show.S01.1080p.WEB-DL"}; !slices.Equal(got, want) {
		t.Errorf("release names got = %v, want = %v", got, want)
	}
	// The season of the release: the second season's.
	if got, want := subs[1].Matches, []string{"year", "series", "imdb_id"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}
	if subs[0].ID != f.srv.URL+"/v1.0/subtitle/1/download" {
		t.Errorf("ID got = %s", subs[0].ID)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.results = `{"status":200,"items":[{"id":2,"title":"Up","year":2009,"description":"Up.2009.BluRay","downloadLink":"BASE/v1.0/subtitle/2/download"}]}`
	p := newProvider(t, f, "k1")
	v := &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"year", "title", "imdb_id"}) ||
		!slices.Equal(subs[0].ReleaseNames, []string{"Up 2009 Up.2009.BluRay"}) {
		t.Errorf("subtitles got = %+v", subs)
	}
	if subs, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, []provider.Language{{Tag: "ro"}}); err != nil || subs != nil {
		t.Errorf("search without an IMDb ID got = %v, %v, want none", subs, err)
	}
	for _, body := range []string{`{"status":404,"items":[]}`, `oops`} {
		f.results = body
		if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}}); err != nil || len(subs) != 0 {
			t.Errorf("search answered %s got = %v, %v, want none", body, subs, err)
		}
	}
}

func TestKeys(t *testing.T) {
	f := newFake(t)
	f.results = `{"status":200,"items":[]}`
	f.limited["k1"] = true
	p := newProvider(t, f, " k1, k2 ,")
	v := &provider.Video{Title: "Up", ExternalIDs: map[string]string{"imdb": "tt1049413"}}
	for range 2 {
		if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}}); err != nil {
			t.Fatal(err)
		}
	}
	// The limited key is left for the next one, which is kept.
	want := []string{"k1 /v1.0/search/imdbid/1049413?language=ro", "k2 /v1.0/search/imdbid/1049413?language=ro", "k2 /v1.0/search/imdbid/1049413?language=ro"}
	if !slices.Equal(f.requests, want) {
		t.Errorf("requests got = %v, want = %v", f.requests, want)
	}
	f.limited["k2"] = true
	if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}}); provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("search with every key limited got = %v, want = TooManyRequests", err)
	}
}

func TestStatusCodes(t *testing.T) {
	for code, kind := range map[int]provider.Kind{401: provider.AuthenticationError, 500: provider.APIThrottled, 503: provider.APIThrottled} {
		f := newFake(t)
		f.statuses = []int{code}
		p := newProvider(t, f, "k1")
		_, err := p.Search(t.Context(), &provider.Video{ExternalIDs: map[string]string{"imdb": "tt1"}}, []provider.Language{{Tag: "ro"}})
		if got := provider.KindOf(err); got != kind {
			t.Errorf("status %d: kind got = %v, want = %v", code, got, kind)
		}
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{"api_key":" , "}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New without keys got = %v, want = ConfigurationError", err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, "k1")
	const sub = "1\n00:00:01,000 --> 00:00:02,000\nBună\n"
	got, err := p.Download(t.Context(), &provider.Video{Episode: true, Season: 1, Number: 2}, f.srv.URL+"/v1.0/subtitle/1/download")
	if err != nil {
		t.Fatal(err)
	}
	// The episode's, not forced.
	if want := sub + "Show.S01E02.srt"; string(got.Data) != want || got.Format != "srt" {
		t.Errorf("download got = %q, want = %q", got.Data, want)
	}
	if got, err := p.Download(t.Context(), &provider.Video{}, f.srv.URL+"/v1.0/subtitle/2/download"); err != nil || string(got.Data) != sub {
		t.Errorf("download of a subtitle got = %q, %v", got.Data, err)
	}
	if _, err := p.Download(t.Context(), &provider.Video{}, f.srv.URL+"/v1.0/subtitle/3/download"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("download of no subtitle got = %v, want = BadSubtitle", err)
	}
}
