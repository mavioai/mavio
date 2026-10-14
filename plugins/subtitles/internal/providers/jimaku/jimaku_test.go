package jimaku

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nこんにちは\r\n"

// fakeSite is Jimaku's API: answers by path and query, in order for
// repeated requests.
type fakeSite struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	answers  map[string][]answer
	requests []string
}

type answer struct {
	code int
	body string
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{t: t, answers: map[string][]answer{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "key" && !strings.HasPrefix(r.URL.Path, "/anime-list") {
			t.Errorf("Authorization = %q, want = key", r.Header.Get("Authorization"))
		}
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		f.mu.Lock()
		f.requests = append(f.requests, key)
		as := f.answers[key]
		if len(as) > 1 {
			f.answers[key] = as[1:]
		}
		f.mu.Unlock()
		if len(as) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(as[0].code)
		_, _ = w.Write([]byte(as[0].body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSite) set(key string, as ...answer) { f.answers[key] = as }

func ok(body string) answer { return answer{200, body} }

func newProvider(t *testing.T, f *fakeSite, extra string) (*Provider, *[]time.Duration) {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)},
		json.RawMessage(`{"api_key":"key"`+extra+`}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	jp := p.(*Provider)
	jp.SetBaseURL(u)
	jp.AniDB().SetURLs(f.srv.URL+"/anime-list.xml", f.srv.URL+"/anime-list-mini.json", f.srv.URL+"/httpapi")
	var slept []time.Duration
	jp.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return jp, &slept
}

func episode() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Frieren", Title: "The Journey's End", Season: 1, Number: 3, Year: 2023,
		SeriesExternalIDs: map[string]string{"anilist": "154587"},
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.set("/api/entries/search?anilist_id=154587", ok(`[
		{"id":1,"name":"Other","anilist_id":1,"flags":{}},
		{"id":7,"name":"Sousou no Frieren","anilist_id":154587,"flags":{"anime":true,"unverified":true}}]`))
	f.set("/api/entries/7/files?episode=3", ok(fmt.Sprintf(`[
		{"url":"%[1]s/f/a.srt","name":"Frieren.S01E03.WEBRip.Netflix.ja[cc].srt","size":40000},
		{"url":"%[1]s/f/b.ass","name":"[Group] Frieren - 03 (1080p).ass","size":40000},
		{"url":"%[1]s/f/c.srt","name":"Frieren.S01E03.ja,en.srt","size":40000},
		{"url":"%[1]s/f/d.srt","name":"Frieren - 03 [WhisperAI].srt","size":40000},
		{"url":"%[1]s/f/e.srt","name":"Frieren - 03.srt","size":120},
		{"url":"%[1]s/f/f.zip","name":"Frieren 01-12.zip","size":400000},
		{"url":"%[1]s/f/g.7z","name":"Frieren 01-12.7z","size":400000}]`, f.srv.URL)))
	p, _ := newProvider(t, f, "")
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "ja"}})
	if err != nil {
		t.Fatal(err)
	}
	// Archives are left out when subtitles are found, as are those of
	// several languages, made by Whisper, or too small.
	want := []string{f.srv.URL + "/f/a.srt", f.srv.URL + "/f/b.ass"}
	if got := ids(subs); !slices.Equal(got, want) {
		t.Fatalf("subtitles got = %v, want = %v", got, want)
	}
	if got, want := subs[0].Matches, []string{"series", "season", "year", "audio_codec"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}
	if got, want := subs[1].Matches, []string{"series", "season", "year"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}
	if s := subs[1]; s.Language.Tag != "ja" || !slices.Equal(s.ReleaseNames, []string{"[Group] Frieren - 03 (1080p).ass"}) {
		t.Errorf("subtitle got = %+v", s)
	}

	// With archives enabled, the archive is listed too.
	p, _ = newProvider(t, f, `,"enable_archives_download":true,"enable_ai_subs":true`)
	subs, err = p.Search(t.Context(), episode(), []provider.Language{{Tag: "ja"}})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{f.srv.URL + "/f/a.srt", f.srv.URL + "/f/b.ass", f.srv.URL + "/f/d.srt", f.srv.URL + "/f/f.zip"}
	if got := ids(subs); !slices.Equal(got, want) {
		t.Errorf("subtitles got = %v, want = %v", got, want)
	}
}

func TestSearchCached(t *testing.T) {
	f := newFake(t)
	f.set("/api/entries/search?anilist_id=154587", ok(`[{"id":7,"anilist_id":154587,"flags":{}}]`))
	f.set("/api/entries/7/files?episode=3", ok(`[{"url":"u","name":"a.srt"}]`))
	p, _ := newProvider(t, f, "")
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	p.env.Now = func() time.Time { return now }
	p.cache = cache.New(p.env)
	for range 2 {
		if _, err := p.Search(t.Context(), episode(), nil); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(2 * time.Minute)
	if _, err := p.Search(t.Context(), episode(), nil); err != nil {
		t.Fatal(err)
	}
	// Entries are kept a week, files a minute.
	want := []string{"/api/entries/search?anilist_id=154587", "/api/entries/7/files?episode=3", "/api/entries/7/files?episode=3"}
	if !slices.Equal(f.requests, want) {
		t.Errorf("requests got = %v, want = %v", f.requests, want)
	}
}

func TestSearchByName(t *testing.T) {
	f := newFake(t)
	f.set("/api/entries/search?query=drive+my+car", ok(`[]`))
	f.set("/api/entries/search?anime=false&query=drive+my+car", ok(`[{"id":9,"anilist_id":null,"flags":{"movie":true}}]`))
	f.set("/api/entries/9/files", ok(`[{"url":"u.zip","name":"Drive My Car.zip"}]`))
	p, _ := newProvider(t, f, "")
	movie := &provider.Video{Title: "Drive My Car", Year: 2021}
	subs, err := p.Search(t.Context(), movie, nil)
	if err != nil {
		t.Fatal(err)
	}
	// An archive alone is listed even with archives disabled.
	if got := ids(subs); !slices.Equal(got, []string{"u.zip"}) {
		t.Errorf("subtitles got = %v, want = [u.zip]", got)
	}
	if got, want := subs[0].Matches, []string{"title", "year"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}

	// Episodes are not searched by name when that is disabled.
	p, _ = newProvider(t, f, `,"enable_name_search_fallback":false`)
	v := episode()
	v.SeriesExternalIDs = nil
	f.requests = nil
	if subs, err := p.Search(t.Context(), v, nil); err != nil || len(subs) != 0 || len(f.requests) != 0 {
		t.Errorf("search got = %v, %v, %v, want none", subs, err, f.requests)
	}
}

func TestSearchEpisodeFallbacks(t *testing.T) {
	f := newFake(t)
	f.set("/api/entries/search?anilist_id=5", ok(`[{"id":5,"anilist_id":5,"flags":{}}]`))
	f.set("/api/entries/5/files?episode=14", ok(`[]`))
	f.set("/api/entries/5/files?episode=2", ok(`[]`))
	p, _ := newProvider(t, f, "")
	v := &provider.Video{Episode: true, Series: "Show", Season: 2, Number: 2, AbsoluteNumber: 14, ExternalIDs: map[string]string{"anilist": "5"}}
	if subs, err := p.Search(t.Context(), v, nil); err != nil || len(subs) != 0 {
		t.Errorf("search got = %v, %v, want none", subs, err)
	}
	// The absolute number is tried first, then the season's.
	want := []string{"/api/entries/search?anilist_id=5", "/api/entries/5/files?episode=14", "/api/entries/5/files?episode=2"}
	if !slices.Equal(f.requests, want) {
		t.Errorf("requests got = %v, want = %v", f.requests, want)
	}

	f.set("/api/entries/5/files?episode=14", ok(`[{"url":"`+f.srv.URL+`/f/pack.zip","name":"pack.zip"}]`))
	p, _ = newProvider(t, f, "")
	subs, err := p.Search(t.Context(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(subs), []string{f.srv.URL + "/f/pack.zip#episode=14"}; !slices.Equal(got, want) {
		t.Fatalf("subtitles got = %v, want = %v", got, want)
	}
	// The absolute number picks the subtitle of the pack.
	f.set("/f/pack.zip", ok(string(zipOf(t, map[string]string{"Show - 13.srt": "x", "Show - 14.srt": srt}))))
	file, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.ReplaceAll(srt, "\r\n", "\n"); string(file.Data) != want || file.Format != "srt" || file.Language.Tag != "ja" {
		t.Errorf("download got = %q %s %v, want = %q", file.Data, file.Format, file.Language, want)
	}

	// Without files of the episode, archives of the entry are looked for.
	f.set("/api/entries/5/files?episode=3", ok(`[]`))
	f.set("/api/entries/5/files", ok(`[{"url":"a","name":"a.srt"},{"url":"b","name":"b.rar"}]`))
	v = &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 3, ExternalIDs: map[string]string{"anilist": "5"}}
	p, _ = newProvider(t, f, `,"enable_archives_download":true`)
	subs, err = p.Search(t.Context(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(subs); !slices.Equal(got, []string{"b"}) {
		t.Errorf("subtitles got = %v, want = [b]", got)
	}
}

func TestSearchAnimeIDs(t *testing.T) {
	f := newFake(t)
	f.set("/anime-list.xml", ok(`<anime-list>
  <anime anidbid="300" tvdbid="700" defaulttvdbseason="a" episodeoffset="">
    <mapping-list><mapping anidbseason="1" tvdbseason="2" offset="13"></mapping></mapping-list>
  </anime>
</anime-list>`))
	f.set("/anime-list-mini.json", ok(`[{"anidb_id": 300, "anilist_id": 9}, {"imdb_id": "tt1", "anilist_id": 8}]`))
	f.set("/api/entries/search?anilist_id=9", ok(`[{"id":9,"anilist_id":9,"flags":{}}]`))
	f.set("/api/entries/9/files?episode=15", ok(`[{"url":"a","name":"Show - 15.srt"}]`))
	f.set("/api/entries/search?anilist_id=8", ok(`[{"id":8,"anilist_id":8,"flags":{"movie":true}}]`))
	f.set("/api/entries/8/files", ok(`[{"url":"m","name":"Movie.srt"}]`))
	p, _ := newProvider(t, f, "")

	// An episode's AniList ID is its AniDB anime's, and AniDB's offset of
	// its season gives its number in the anime.
	v := &provider.Video{Episode: true, Series: "Show", Season: 2, Number: 2, SeriesExternalIDs: map[string]string{"tvdb": "700"}}
	subs, err := p.Search(t.Context(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(subs), []string{"a#episode=15"}; !slices.Equal(got, want) {
		t.Errorf("episode subtitles got = %v, want = %v", got, want)
	}

	// A movie's is its IMDb ID's.
	subs, err = p.Search(t.Context(), &provider.Video{Title: "Movie", ExternalIDs: map[string]string{"imdb": "tt1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(subs), []string{"m"}; !slices.Equal(got, want) {
		t.Errorf("movie subtitles got = %v, want = %v", got, want)
	}

	// The lists failing leaves the name to search by.
	f.set("/anime-list.xml")
	f.set("/api/entries/search?query=other", ok(`[]`))
	f.set("/api/entries/search?anime=false&query=other", ok(`[]`))
	p, _ = newProvider(t, f, "")
	v = &provider.Video{Episode: true, Series: "Other", Season: 1, Number: 1, SeriesExternalIDs: map[string]string{"tvdb": "701"}}
	if subs, err := p.Search(t.Context(), v, nil); err != nil || len(subs) != 0 {
		t.Errorf("search got = %v, %v, want none", subs, err)
	}
	if !slices.Contains(f.requests, "/api/entries/search?query=other") {
		t.Errorf("requests got = %v, want a search by name", f.requests)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	f.set("/f/a.srt", ok(srt))
	f.set("/f/gone.srt", answer{404, ""})
	p, _ := newProvider(t, f, "")
	file, err := p.Download(t.Context(), episode(), f.srv.URL+"/f/a.srt")
	if err != nil {
		t.Fatal(err)
	}
	if file.Format != "srt" || !strings.Contains(string(file.Data), "こんにちは") {
		t.Errorf("download got = %q %s", file.Data, file.Format)
	}
	if _, err := p.Download(t.Context(), episode(), f.srv.URL+"/f/gone.srt"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("download of a missing file got = %v, want = ProviderError", err)
	}
}

func TestStatusCodes(t *testing.T) {
	const path = "/api/entries/search?anilist_id=154587"
	for _, c := range []struct {
		name    string
		answers []answer
		kind    provider.Kind
		slept   int
	}{
		{"rate limited once", []answer{{429, ""}, ok(`[{"id":7,"anilist_id":154587,"flags":{}}]`)}, -1, 1},
		{"rate limited", []answer{{429, ""}}, provider.APIThrottled, 3},
		{"unauthorized", []answer{{401, ""}}, provider.AuthenticationError, 0},
		{"error", []answer{ok(`{"error":"bad","code":500}`)}, provider.ServiceUnavailable, 0},
		{"not JSON", []answer{ok(`<html>`)}, provider.ParseResponseError, 0},
		{"server error", []answer{{502, ""}}, provider.ProviderError, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.set(path, c.answers...)
			f.set("/api/entries/7/files?episode=3", ok(`[]`))
			f.set("/api/entries/7/files", ok(`[]`))
			p, slept := newProvider(t, f, "")
			_, err := p.Search(t.Context(), episode(), nil)
			if c.kind < 0 {
				if err != nil {
					t.Errorf("err got = %v, want = nil", err)
				}
			} else if got := provider.KindOf(err); got != c.kind {
				t.Errorf("kind got = %v, want = %v (%v)", got, c.kind, err)
			}
			if len(*slept) != c.slept || c.slept > 0 && (*slept)[0] != 5*time.Second {
				t.Errorf("waits got = %v, want = %d of 5s", *slept, c.slept)
			}
		})
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{}`, `{"api_key":""}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = ConfigurationError", s, err)
		}
	}
}

func TestLanguages(t *testing.T) {
	for name, want := range map[string][]string{
		"Show.S01E01.srt":               {"ja"},
		"Show.S01E01.ja.srt":            {"ja"},
		"Show.S01E01.ja,en.srt":         {"ja", "en"},
		"Show.S01E01.JPSC.ass":          {"ja", "sc"},
		"Show.S01E01.chs&jp.ass":        {"zh", "ja"},
		"Show.S01E01.en,fr.srt":         {"ja"},
		"[Group] Show - 01 [ja+en].ass": {"ja", "en"},
		"Show.S01E01.ja[cc],en[cc].srt": {"ja"},
		"Show.S01E01.1080p.srt":         {"ja"},
		"Show.S01E01.Netflix,ja.srt":    {"ja"},
		"Show - 01 [jpn,eng].srt":       {"ja", "en"},
		"Show - 01 [jpn,xyz].srt":       {"ja"},
		"Show - 01.srt":                 {"ja"},
	} {
		if got := languages(name); !slices.Equal(got, want) {
			t.Errorf("languages(%q) got = %v, want = %v", name, got, want)
		}
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	}) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}
