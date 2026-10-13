package anidb

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const animeList = `<?xml version="1.0" encoding="UTF-8"?>
<anime-list>
  <anime anidbid="100" tvdbid="500" defaulttvdbseason="1" episodeoffset="">
    <name>Show</name>
  </anime>
  <anime anidbid="102" tvdbid="500" defaulttvdbseason="2" episodeoffset="12">
    <name>Show Part 2</name>
  </anime>
  <anime anidbid="101" tvdbid="500" defaulttvdbseason="2" episodeoffset="0">
    <name>Show Season 2</name>
    <mapping-list>
      <mapping anidbseason="0" tvdbseason="0">;1-5;</mapping>
      <mapping anidbseason="1" tvdbseason="2">;3-20+21;</mapping>
    </mapping-list>
  </anime>
  <anime anidbid="200" tvdbid="600" defaulttvdbseason="a" episodeoffset="">
    <name>Long Show</name>
    <mapping-list>
      <mapping anidbseason="1" tvdbseason="3" offset="50"/>
    </mapping-list>
  </anime>
</anime-list>`

const aniList = `[
  {"anidb_id": 100, "anilist_id": 9100, "imdb_id": "tt0000100"},
  {"anidb_id": 300, "imdb_id": "tt0000300", "anilist_id": 9300},
  {"anidb_id": 400, "imdb_id": "tt0000400"}
]`

// fake serves the lists and the AniDB API.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	hits     map[string]int
	apiCode  string
	apiQuery []string
}

func newFake(t *testing.T) *fake {
	f := &fake{hits: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /anime-list.xml", func(w http.ResponseWriter, r *http.Request) {
		f.hit("mappings")
		_, _ = w.Write([]byte(animeList))
	})
	mux.HandleFunc("GET /anime-list-mini.json", func(w http.ResponseWriter, r *http.Request) {
		f.hit("anilist")
		_, _ = w.Write([]byte(aniList))
	})
	mux.HandleFunc("GET /httpapi", func(w http.ResponseWriter, r *http.Request) {
		f.hit("api")
		f.mu.Lock()
		f.apiQuery = append(f.apiQuery, r.URL.RawQuery)
		code := f.apiCode
		f.mu.Unlock()
		if code != "" {
			_, _ = fmt.Fprintf(w, `<error code="%s">no</error>`, code)
			return
		}
		_, _ = fmt.Fprintf(w, `<anime id="%s"><episodes>
			<episode id="7001" update="2020-01-01"><epno type="1">1</epno></episode>
			<episode id="7003" update="2020-01-01"><epno type="1">3</epno></episode>
			<episode id="7090" update="2020-01-01"><epno type="2">S1</epno></episode>
		</episodes></anime>`, r.URL.Query().Get("aid"))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) hit(name string) {
	f.mu.Lock()
	f.hits[name]++
	f.mu.Unlock()
}

func (f *fake) setCode(code string) {
	f.mu.Lock()
	f.apiCode = code
	f.mu.Unlock()
}

func (f *fake) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[name]
}

func newClient(t *testing.T, f *fake, dir string, now *time.Time, cfg Config) *Client {
	t.Helper()
	env := provider.Env{HTTP: f.srv.Client(), Now: func() time.Time { return *now }, Log: slog.New(slog.DiscardHandler), DataDir: dir}
	c := New(env, cfg)
	c.SetURLs(f.srv.URL+"/anime-list.xml", f.srv.URL+"/anime-list-mini.json", f.srv.URL+"/httpapi")
	return c
}

func TestShowInformation(t *testing.T) {
	f := newFake(t)
	now := time.Now()
	c := newClient(t, f, "", &now, Config{})
	for _, tc := range []struct {
		tvdb                   string
		season, episode        int
		series, number, offset int
	}{
		{"500", 1, 4, 100, 4, 0},
		// Season 2 is split in two anime at episode 12.
		{"500", 2, 5, 101, 5, 0},
		{"500", 2, 15, 102, 3, 12},
		// Mapped episodes: TVDB 20 and 21 are AniDB's 3.
		{"500", 2, 21, 101, 3, 0},
		// The specials' mapping is of season 0, not 2.
		{"500", 2, 5, 101, 5, 0},
		// Seasons kept in the mappings of an "a" entry.
		{"600", 3, 55, 200, 5, 50},
		{"600", 4, 2, 200, 2, 0},
		{"500", 3, 1, 0, 0, 0},
		{"999", 1, 1, 0, 0, 0},
	} {
		series, number, offset, err := c.showInformation(t.Context(), tc.tvdb, tc.season, tc.episode)
		if err != nil {
			t.Fatal(err)
		}
		if series != tc.series || series != 0 && (number != tc.number || offset != tc.offset) {
			t.Errorf("showInformation(%s, %d, %d) got = %d, %d, %d, want = %d, %d, %d",
				tc.tvdb, tc.season, tc.episode, series, number, offset, tc.series, tc.number, tc.offset)
		}
	}
	if n := f.count("mappings"); n != 1 {
		t.Errorf("mappings fetched got = %d, want = 1", n)
	}
}

func episode(tvdb string, season, number int) *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Show", Season: season, Number: number,
		SeriesExternalIDs: map[string]string{"tvdb": tvdb}, ExternalIDs: map[string]string{},
	}
}

func TestRefine(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	ver := 3
	c := newClient(t, f, dir, &now, Config{APIClient: "mavio", APIClientVer: &ver})

	ids, err := c.Refine(t.Context(), episode("500", 2, 15), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := (IDs{Series: 102, Episode: 7003, EpisodeNumber: 3, Offset: 12}); ids != want {
		t.Errorf("Refine got = %+v, want = %+v", ids, want)
	}
	if want := "aid=102&client=mavio&clientver=3&protover=1&request=anime"; len(f.apiQuery) != 1 || f.apiQuery[0] != want {
		t.Errorf("API queries got = %v, want = [%s]", f.apiQuery, want)
	}

	// Without episode IDs, the API is not asked.
	ids, err = c.Refine(t.Context(), episode("500", 1, 2), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (IDs{Series: 100, EpisodeNumber: 2}); ids != want {
		t.Errorf("Refine got = %+v, want = %+v", ids, want)
	}

	// The IDs the video carries are kept.
	v := episode("", 1, 3)
	v.SeriesExternalIDs["anidb"], v.ExternalIDs["anidb"] = "42", "4242"
	if ids, err = c.Refine(t.Context(), v, true); err != nil || ids != (IDs{Series: 42, Episode: 4242, EpisodeNumber: 3}) {
		t.Errorf("Refine got = %+v, %v, want = the video's", ids, err)
	}
	// Not an anime.
	if ids, err = c.Refine(t.Context(), episode("999", 1, 3), true); err != nil || ids != (IDs{}) {
		t.Errorf("Refine got = %+v, %v, want = none", ids, err)
	}

	// Another client of the same folder uses the lists and episodes kept
	// there, and the quota.
	c2 := newClient(t, f, dir, &now, Config{APIClient: "mavio"})
	ids, err = c2.Refine(t.Context(), episode("500", 2, 13), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := (IDs{Series: 102, Episode: 7001, EpisodeNumber: 1, Offset: 12}); ids != want {
		t.Errorf("Refine got = %+v, want = %+v", ids, want)
	}
	if got := []int{f.count("mappings"), f.count("api")}; got[0] != 1 || got[1] != 1 {
		t.Errorf("fetched mappings, API got = %v, want = [1 1]", got)
	}
	if q, err := c2.loadQuota(); err != nil || q.Count != 1 {
		t.Errorf("quota got = %+v, %v, want = 1 request", q, err)
	}

	// A day later the mapping is fetched again.
	now = now.Add(25 * time.Hour)
	if _, err := c2.Refine(t.Context(), episode("500", 1, 2), false); err != nil {
		t.Fatal(err)
	}
	if n := f.count("mappings"); n != 2 {
		t.Errorf("mappings fetched got = %d, want = 2", n)
	}
}

func TestRefineLimits(t *testing.T) {
	f := newFake(t)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()

	// Without an API client, there is no episode ID.
	c := newClient(t, f, dir, &now, Config{})
	if ids, err := c.Refine(t.Context(), episode("500", 1, 3), true); err != nil || ids != (IDs{Series: 100, EpisodeNumber: 3}) {
		t.Errorf("Refine got = %+v, %v, want = no episode ID", ids, err)
	}

	c = newClient(t, f, dir, &now, Config{APIClient: "mavio"})
	if err := c.saveQuota(quota{Count: dailyLimit, Set: now}); err != nil {
		t.Fatal(err)
	}
	// The daily limit throttles the API for a day.
	if ids, err := c.Refine(t.Context(), episode("500", 1, 3), true); err != nil || ids.Episode != 0 {
		t.Errorf("Refine got = %+v, %v, want = no episode ID", ids, err)
	}
	if throttled, _ := c.Throttled(); !throttled || f.count("api") != 0 {
		t.Errorf("throttled, API got = %v, %d, want = true, 0", throttled, f.count("api"))
	}
	now = now.Add(23 * time.Hour)
	if ids, _ := c.Refine(t.Context(), episode("500", 1, 3), true); ids.Episode != 0 || f.count("api") != 0 {
		t.Errorf("Refine got = %+v, want = still throttled", ids)
	}
	now = now.Add(2 * time.Hour)
	if ids, err := c.Refine(t.Context(), episode("500", 1, 3), true); err != nil || ids.Episode != 7003 {
		t.Errorf("Refine got = %+v, %v, want = episode 7003", ids, err)
	}

	// A ban throttles; an unknown client is a configuration error.
	for code, want := range map[string]provider.Kind{"500": provider.TooManyRequests, "302": provider.ConfigurationError} {
		f.setCode(code)
		c := newClient(t, f, "", &now, Config{APIClient: "mavio"})
		_, err := c.episodesOf(t.Context(), 300)
		if got := provider.KindOf(err); got != want {
			t.Errorf("answer %s got = %v, want = %v", code, got, want)
		}
	}
	c = newClient(t, f, "", &now, Config{APIClient: "mavio"})
	f.setCode("500")
	if ids, err := c.Refine(t.Context(), episode("600", 1, 3), true); err != nil || ids != (IDs{Series: 200, EpisodeNumber: 3}) {
		t.Errorf("Refine when banned got = %+v, %v, want = no episode ID", ids, err)
	}
	if throttled, _ := c.Throttled(); !throttled {
		t.Errorf("throttled got = false, want = true")
	}
}

func TestCachedStale(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	path := filepath.Join(dir, "anime-list.xml")
	if err := os.WriteFile(path, []byte(`<anime-list><anime anidbid="1" tvdbid="1" defaulttvdbseason="1"/></anime-list>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	c := newClient(t, f, dir, &now, Config{})
	c.mappingsURL = f.srv.URL + "/missing"
	// A list that cannot be fetched again is used stale.
	if series, _, _, err := c.showInformation(t.Context(), "1", 1, 1); err != nil || series != 1 {
		t.Errorf("showInformation got = %d, %v, want = 1", series, err)
	}
	c = newClient(t, f, "", &now, Config{})
	c.mappingsURL = f.srv.URL + "/missing"
	if _, _, _, err := c.showInformation(t.Context(), "1", 1, 1); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("showInformation got = %v, want = ProviderError", err)
	}
}

func TestAniListID(t *testing.T) {
	f := newFake(t)
	now := time.Now()
	c := newClient(t, f, t.TempDir(), &now, Config{})
	for _, tc := range []struct {
		v      *provider.Video
		series int
		want   int
	}{
		{&provider.Video{ExternalIDs: map[string]string{"imdb": "tt0000300"}}, 0, 9300},
		{&provider.Video{ExternalIDs: map[string]string{"imdb": "tt0000400"}}, 0, 0},
		{&provider.Video{ExternalIDs: map[string]string{"imdb": "tt0000300", "anilist": "77"}}, 0, 77},
		{&provider.Video{ExternalIDs: map[string]string{}}, 0, 0},
		{&provider.Video{Episode: true}, 100, 9100},
		{&provider.Video{Episode: true}, 0, 0},
	} {
		got, err := c.AniListID(t.Context(), tc.v, tc.series)
		if err != nil || got != tc.want {
			t.Errorf("AniListID(%+v) got = %d, %v, want = %d", tc.v, got, err, tc.want)
		}
	}
	if n := f.count("anilist"); n != 1 {
		t.Errorf("anime-lists fetched got = %d, want = 1", n)
	}
}

func TestConfig(t *testing.T) {
	for _, tc := range []struct {
		ver  int
		want provider.Kind
	}{{1, provider.Other}, {9, provider.Other}, {0, provider.ConfigurationError}, {10, provider.ConfigurationError}} {
		c := Config{APIClientVer: &tc.ver}
		if err := c.Validate(); (err != nil) != (tc.want == provider.ConfigurationError) || err != nil && provider.KindOf(err) != tc.want {
			t.Errorf("Validate(%d) got = %v, want = %v", tc.ver, err, tc.want)
		}
	}
	if got := (Config{}).Version(); got != 1 {
		t.Errorf("Version got = %d, want = 1", got)
	}
}
