package subsarr

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

// fakeSite is a Subsarr: results by query, statuses to answer the next
// searches with.
type fakeSite struct {
	srv *httptest.Server

	mu       sync.Mutex
	results  map[string]string
	statuses []int
	queries  []string
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{results: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sub/api/v1/subtitles/search", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.queries = append(f.queries, r.URL.RawQuery)
		if len(f.statuses) > 0 {
			w.WriteHeader(f.statuses[0])
			f.statuses = f.statuses[1:]
			return
		}
		q := r.URL.Query()
		res, ok := f.results[q.Get("language")+"|"+q.Get("imdb_id")+q.Get("query")]
		if !ok {
			res = `{"items":[]}`
		}
		_, _ = w.Write([]byte(res))
	})
	mux.HandleFunc("GET /sub/api/v1/subtitles/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(srt))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"base_url":"`+f.srv.URL+`/sub/"}`))
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.results["english|tt0903747"] = `{"items":[
		{"id":1,"language":"english","hi":true,"download_url":"/sub/api/v1/subtitles/1/download","title":"breaking bad","releases":["Breaking.Bad.S01E02.720p","Breaking.Bad.S01E02.1080p"],"filename":"a.srt"},
		{"id":2,"language":"klingon","download_url":"/x","title":"Breaking Bad"},
		{"id":3,"language":"french","download_url":"/y","title":"Breaking Bad"}]}`
	f.results["brazilian_portuguese|Breaking Bad"] = `{"items":[
		{"id":4,"language":"brazilian_portuguese","download_url":"http://elsewhere/4","title":"Other","releases":null,"filename":"b.srt"}]}`
	p := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Breaking Bad", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0903747"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}, {Tag: "en", HearingImpaired: true}, {Tag: "pt-BR"}, {Tag: "tlh"}})
	if err != nil {
		t.Fatal(err)
	}
	// Each name once, by IMDb ID then title.
	want := []string{
		"episode=2&imdb_id=tt0903747&language=english&per_page=100&season=1",
		"episode=2&imdb_id=tt0903747&language=brazilian_portuguese&per_page=100&season=1",
		"episode=2&language=brazilian_portuguese&per_page=100&query=Breaking+Bad&season=1",
	}
	if !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	if got := ids(subs); !slices.Equal(got, []string{f.srv.URL + "/sub/api/v1/subtitles/1/download", "http://elsewhere/4"}) {
		t.Fatalf("subtitles got = %v", got)
	}
	s := subs[0]
	if s.Language != (provider.Language{Tag: "en", HearingImpaired: true}) || !slices.Equal(s.ReleaseNames, []string{"Breaking.Bad.S01E02.720p", "Breaking.Bad.S01E02.1080p"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	if want := []string{"series", "season", "episode"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	if s := subs[1]; s.Language.Tag != "pt-BR" || !slices.Equal(s.ReleaseNames, []string{"b.srt"}) || !slices.Equal(s.Matches, []string{"season", "episode"}) {
		t.Errorf("subtitle got = %+v", s)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.results["spanish|Up"] = `{"items":[{"id":5,"language":"spanish","download_url":"/5","title":"UP"}]}`
	p := newProvider(t, f)
	subs, err := p.Search(t.Context(), &provider.Video{Title: "Up", Year: 2009}, []provider.Language{{Tag: "es-MX"}, {Tag: "es"}})
	if err != nil {
		t.Fatal(err)
	}
	// Latin American Spanish is Spanish to the site.
	if want := []string{"language=spanish&per_page=100&query=Up"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	if len(subs) != 1 || subs[0].Language.Tag != "es" || !slices.Equal(subs[0].Matches, []string{"title"}) {
		t.Errorf("subtitles got = %+v", subs)
	}
	f.queries = nil
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}}, []provider.Language{{Tag: "es"}}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"imdb_id=tt1049413&language=spanish&per_page=100&year=2009", "language=spanish&per_page=100&query=Up"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
}

func TestStatusAndDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	f.statuses = []int{500}
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("search failing got = %v, want = ProviderError", err)
	}
	got, err := p.Download(t.Context(), &provider.Video{}, f.srv.URL+"/sub/api/v1/subtitles/1/download")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"; string(got.Data) != want || got.Format != "srt" {
		t.Errorf("download got = %q, want = %q", got.Data, want)
	}
	if _, err := p.Download(t.Context(), &provider.Video{}, f.srv.URL+"/sub/api/v1/subtitles/2/download"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("download failing got = %v, want = ProviderError", err)
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{}`, `{"base_url":"subsarr:8080"}`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = ConfigurationError", s, err)
		}
	}
	for tag, name := range map[string]string{"pt-BR": "brazilian_portuguese", "pt": "portuguese", "zh-Hans": "chinese_bg_code", "uk": "ukranian", "km": "cambodian-khmer", "tlh": ""} { //nolint:misspell // Subscene's spelling
		if got := siteName(tag); got != name {
			t.Errorf("siteName(%s) got = %q, want = %q", tag, got, name)
		}
	}
	if n := len(Definition().Languages); n != len(fromSite) {
		t.Errorf("languages got = %d, want = %d", n, len(fromSite))
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}
