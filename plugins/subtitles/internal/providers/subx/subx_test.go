package subx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHola\r\n"

// fakeAPI is SubX: search results by title or IMDb ID, then statuses to
// answer with first.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	results  map[string]string
	statuses []int
	header   http.Header
	queries  []string
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, results: map[string]string{}, header: http.Header{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/subtitles/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("Authorization = %q, want = Bearer key", r.Header.Get("Authorization"))
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.queries = append(f.queries, r.URL.RawQuery)
		if len(f.statuses) > 0 {
			code := f.statuses[0]
			f.statuses = f.statuses[1:]
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(code)
			return
		}
		for k, v := range f.header {
			w.Header()[k] = v
		}
		q := r.URL.Query()
		key := q.Get("title")
		if key == "" {
			key = q.Get("imdb_id")
		}
		res, ok := f.results[key]
		if !ok {
			res = `{"total":0,"items":[]}`
		}
		_, _ = w.Write([]byte(res))
	})
	mux.HandleFunc("GET /api/subtitles/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("id") {
		case "1":
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, n := range []string{"Show.S01E01.srt", "Show.S01E02.srt"} {
				fw, _ := zw.Create(n)
				_, _ = fw.Write([]byte(srt + n))
			}
			_ = zw.Close()
			_, _ = w.Write(buf.Bytes())
		case "2":
			_, _ = w.Write([]byte(srt))
		case "3":
			_, _ = w.Write([]byte("<html>"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeAPI) (*Provider, *[]time.Duration) {
	t.Helper()
	now := time.Unix(1_000_000, 0)
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: func() time.Time { return now }, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"api_key":"key"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	sp := p.(*Provider)
	sp.SetBaseURL(u)
	var slept []time.Duration
	sp.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return sp, &slept
}

var both = []provider.Language{{Tag: "es"}, {Tag: "es-MX"}}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.results["Breaking Bad S03E13"] = `{"total":3,"items":[
		{"id":1,"title":"Breaking Bad S03E13","description":"Fly.720p, castellano","uploader_name":"ana","season":3,"episode":13},
		{"id":2,"title":"Breaking Bad S03E12","season":3,"episode":12},
		{"id":3,"title":"Breaking Bad S02E13","season":2,"episode":13},
		{"id":4,"title":"Breaking Bad S03","description":"temporada completa","season":3,"episode":null,"page_url":"https://www.subdivx.com/4"}]}`
	p, _ := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Breaking.Bad", Season: 3, Number: 13, Year: 2008}
	subs, err := p.Search(t.Context(), v, both)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"limit=200&title=Breaking+Bad+S03E13&video_type=episode&year=2008"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	// The episode's subtitles leave out the season's packs.
	if got := ids(subs); !slices.Equal(got, []string{"1"}) {
		t.Fatalf("subtitles got = %v, want = [1]", got)
	}
	s := subs[0]
	if s.Language.Tag != "es" || s.Uploader != "ana" || s.PageLink != f.srv.URL+"/api/subtitles/1" ||
		!slices.Equal(s.ReleaseNames, []string{"Breaking Bad S03E13 | Fly.720p, castellano"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	if want := []string{"title", "series", "year", "season", "episode"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	// Spain's Spanish is left out when Latin America's alone is asked for.
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "es-MX"}}); err != nil || len(subs) != 0 {
		t.Errorf("Latin American subtitles got = %v, %v, want none", subs, err)
	}
}

func TestSearchSeasonPacks(t *testing.T) {
	f := newFake(t)
	f.results["Show S01"] = `{"items":[{"id":4,"title":"Show S01","season":1,"episode":null},{"id":5,"title":"Show S02","season":2}]}`
	p, slept := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Show", AlternativeSeries: []string{"Show"}, Season: 1, Number: 2}
	subs, err := p.Search(t.Context(), v, both)
	if err != nil {
		t.Fatal(err)
	}
	// The episode, then its season.
	if len(f.queries) != 2 {
		t.Errorf("queries got = %v, want 2", f.queries)
	}
	if got := ids(subs); !slices.Equal(got, []string{"4"}) || subs[0].Language.Tag != "es-MX" || subs[0].Uploader != "unknown" {
		t.Fatalf("subtitles got = %+v, want = [4]", subs)
	}
	if want := []string{"title", "series", "year", "season", "episode"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("matches got = %v, want = %v", subs[0].Matches, want)
	}
	if len(*slept) != 0 {
		t.Errorf("waits got = %v, want none", *slept)
	}

	// Nothing for the titles: three queries each, a pause between them.
	f.queries = nil
	v = &provider.Video{Episode: true, Series: "Other", AlternativeSeries: []string{"Otro", "Autre", "Andere"}, Season: 1, Number: 2}
	if subs, err := p.Search(t.Context(), v, both); err != nil || len(subs) != 0 {
		t.Errorf("search got = %v, %v, want none", subs, err)
	}
	if len(f.queries) != 9 || !slices.Equal(*slept, []time.Duration{time.Second, time.Second}) {
		t.Errorf("queries got = %d, waits = %v, want = 9, [1s 1s]", len(f.queries), *slept)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.results["tt1049413"] = `{"items":[{"id":7,"title":"Up (2009)","description":"BluRay\nWEB-DL","season":null}]}`
	p, _ := newProvider(t, f)
	v := &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}}
	subs, err := p.Search(t.Context(), v, both)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"imdb_id=tt1049413&limit=200&video_type=movie&year=2009"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"title", "year"}) ||
		!slices.Equal(subs[0].ReleaseNames, []string{"Up (2009) | BluRay", "WEB-DL"}) {
		t.Errorf("subtitles got = %+v", subs)
	}
}

func TestSearchStatus(t *testing.T) {
	for _, c := range []struct {
		name     string
		statuses []int
		kind     provider.Kind
		slept    []time.Duration
	}{
		{"rate limited once", []int{429}, -1, []time.Duration{7 * time.Second}},
		{"rate limited", []int{429, 429, 429}, provider.APIThrottled, []time.Duration{7 * time.Second, 7 * time.Second}},
		{"server failing", []int{500, 502, 503}, -1, []time.Duration{time.Second, 2 * time.Second}},
		{"unauthorized", []int{401}, provider.ConfigurationError, nil},
		{"bad request", []int{400}, -1, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.statuses = c.statuses
			f.results["Up"] = `{"items":[{"id":7,"title":"Up"}]}`
			p, slept := newProvider(t, f)
			subs, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, both)
			if c.kind >= 0 {
				if got := provider.KindOf(err); got != c.kind {
					t.Errorf("kind got = %v, want = %v (%v)", got, c.kind, err)
				}
			} else if err != nil {
				t.Errorf("err got = %v, want = nil", err)
			}
			if c.name == "rate limited once" && len(subs) != 1 {
				t.Errorf("subtitles got = %v, want 1", subs)
			}
			if !slices.Equal(*slept, c.slept) {
				t.Errorf("waits got = %v, want = %v", *slept, c.slept)
			}
		})
	}
}

func TestSlowDown(t *testing.T) {
	f := newFake(t)
	f.results["Up"] = `{"items":[]}`
	p, slept := newProvider(t, f)
	f.header.Set("X-RateLimit-Remaining", "1")
	f.header.Set("X-RateLimit-Limit", "10")
	f.header.Set("X-RateLimit-Reset", strconv.FormatInt(1_000_030, 10))
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, both); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{30 * time.Second}; !slices.Equal(*slept, want) {
		t.Errorf("waits got = %v, want = %v", *slept, want)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p, _ := newProvider(t, f)
	ep := &provider.Video{Episode: true, Season: 1, Number: 2}
	got, err := p.Download(t.Context(), ep, "1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHola\nShow.S01E02.srt"; string(got.Data) != want || got.Format != "srt" {
		t.Errorf("download got = %q %s, want = %q", got.Data, got.Format, want)
	}
	if got, err := p.Download(t.Context(), &provider.Video{}, "2"); err != nil || got.Format != "srt" {
		t.Errorf("download of a subtitle got = %q, %v", got.Data, err)
	}
	for id, kind := range map[string]provider.Kind{"3": provider.APIThrottled, "9": provider.APIThrottled} {
		if _, err := p.Download(t.Context(), ep, id); provider.KindOf(err) != kind {
			t.Errorf("download %s got = %v, want = %v", id, err, kind)
		}
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New without a key got = %v, want = ConfigurationError", err)
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}
