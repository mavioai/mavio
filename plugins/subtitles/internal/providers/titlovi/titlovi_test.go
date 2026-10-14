package titlovi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "titlovi.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_and_download_from_pack": "TestListAndDownloadFromPack",
		},
	})
}

// sites answers the requests to any host, as requests_mock does.
type sites struct {
	t *testing.T

	mu       sync.Mutex
	routes   map[string]func(*http.Request) (int, []byte)
	requests []*http.Request
}

func newSites(t *testing.T) *sites {
	return &sites{t: t, routes: map[string]func(*http.Request) (int, []byte){}}
}

// on answers a method, host and path.
func (s *sites) on(route string, h func(*http.Request) (int, []byte)) {
	s.mu.Lock()
	s.routes[route] = h
	s.mu.Unlock()
}

func (s *sites) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, r)
	h := s.routes[r.Method+" "+r.URL.Host+r.URL.Path]
	s.mu.Unlock()
	code, body := http.StatusNotFound, []byte("not found")
	if h != nil {
		code, body = h(r)
	}
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
}

func (s *sites) count(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.requests {
		if r.Method+" "+r.URL.Host+r.URL.Path == route {
			n++
		}
	}
	return n
}

func reply(code int, body []byte) func(*http.Request) (int, []byte) {
	return func(*http.Request) (int, []byte) { return code, body }
}

// now is before the recorded token expires.
var now = time.Date(2024, 10, 1, 12, 0, 0, 0, time.Local)

func newProvider(t *testing.T, s *sites, dataDir string) *Provider {
	t.Helper()
	env := provider.Env{HTTP: &http.Client{Transport: s}, Now: func() time.Time { return now }, Log: slog.New(slog.DiscardHandler), DataDir: dataDir}
	p, err := New(env, json.RawMessage(`{"username":"user1","password":"pass1"}`))
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

const (
	gettoken = "POST kodi.titlovi.com/api/subtitles/gettoken"
	search   = "GET kodi.titlovi.com/api/subtitles/search"
)

func nikita() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "La Femme Nikita", Season: 1, Number: 13, VideoCodec: "H.265",
		SeriesExternalIDs: map[string]string{"imdb": "tt21209876"},
		FileName:          "La Femme Nikita (1997) - S01E13 - Recruit [HDTV-720p][Opus 2.0][x265].mkv",
	}
}

func TestListAndDownloadFromPack(t *testing.T) {
	s := newSites(t)
	s.on(gettoken, func(r *http.Request) (int, []byte) {
		q := r.URL.Query()
		if q.Get("username") != "user1" || q.Get("password") != "pass1" || q.Get("json") != "True" {
			t.Errorf("gettoken query = %v", q)
		}
		return http.StatusOK, ported.Response(t, "titlovi_gettoken_response.json")
	})
	s.on(search, func(r *http.Request) (int, []byte) {
		q := r.URL.Query()
		if q.Get("token") != "asdf1234" || q.Get("userid") != "111" || q.Get("query") != "la femme nikita" || q.Get("lang") != "Srpski" ||
			q.Get("json") != "True" || q.Get("season") != "1" {
			t.Errorf("search query = %v", q)
		}
		return http.StatusOK, ported.Response(t, "titlovi_search_response.json")
	})
	pack := ported.Response(t, "titlovi_some_subtitle_pack.zip")
	s.on("GET titlovi.com/download/", func(r *http.Request) (int, []byte) {
		if r.URL.Query().Get("mediaid") != "81022" {
			return http.StatusNotFound, nil
		}
		return http.StatusOK, pack
	})
	p := newProvider(t, s, t.TempDir())
	subs, err := p.Search(t.Context(), nikita(), []provider.Language{{Tag: "sr-Latn"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subtitles = %d, want = 1", len(subs))
	}
	if sub := subs[0]; sub.Language != (provider.Language{Tag: "sr-Latn"}) || !slices.Equal(sub.Matches, []string{"series", "season", "episode"}) {
		t.Errorf("subtitle = %+v", sub)
	}
	file, err := p.Download(t.Context(), nikita(), subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(pack), int64(len(pack)))
	if err != nil {
		t.Fatal(err)
	}
	want := readZip(t, zr, "La Femme Nikita - 01x13 - Recruit.srt")
	if !bytes.Equal(file.Data, archive.FixLineEndings(want)) || file.Format != "srt" {
		t.Errorf("file = %d bytes %s, want = Recruit.srt's %d", len(file.Data), file.Format, len(want))
	}
}

func readZip(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	f, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestToken(t *testing.T) {
	dir := t.TempDir()
	s := newSites(t)
	s.on(gettoken, reply(http.StatusOK, ported.Response(t, "titlovi_gettoken_response.json")))
	s.on(search, reply(http.StatusOK, []byte(`{"PagesAvailable":0,"SubtitleResults":[]}`)))
	sr := []provider.Language{{Tag: "sr"}}
	if _, err := newProvider(t, s, dir).Search(t.Context(), nikita(), sr); err != nil {
		t.Fatal(err)
	}
	// The token survives restarts until it expires.
	if _, err := newProvider(t, s, dir).Search(t.Context(), nikita(), sr); err != nil {
		t.Fatal(err)
	}
	if got := s.count(gettoken); got != 1 {
		t.Errorf("logins = %d, want = 1", got)
	}
	now = now.AddDate(0, 1, 0)
	_, err := newProvider(t, s, dir).Search(t.Context(), nikita(), sr)
	now = now.AddDate(0, -1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.count(gettoken); got != 2 {
		t.Errorf("logins = %d, want = 2", got)
	}

	// An unauthorized account fails, and so does a refused token, which
	// is then forgotten.
	s.on(gettoken, reply(http.StatusUnauthorized, nil))
	if _, err := newProvider(t, s, "").Search(t.Context(), nikita(), sr); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("err = %v, want = AuthenticationError", err)
	}
	s.on(search, reply(http.StatusUnauthorized, nil))
	// Searches kept across restarts would answer without the site.
	if err := os.Remove(filepath.Join(dir, "cache.json")); err != nil {
		t.Fatal(err)
	}
	p := newProvider(t, s, dir)
	if _, err := p.Search(t.Context(), nikita(), sr); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("err = %v, want = AuthenticationError", err)
	}
	if _, err := os.Stat(filepath.Join(dir, tokenFile)); !os.IsNotExist(err) {
		t.Errorf("token file: %v, want none", err)
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{"username":"a"}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("err = %v, want = ConfigurationError", err)
	}
}

func hit(id int, title, lang string, year, season, episode int, release string) map[string]any {
	return map[string]any{
		"Id": id, "Title": title, "Year": year, "Link": "https://titlovi.com/download/?type=1&mediaid=" + strings.Repeat("1", id),
		"Season": season, "Episode": episode, "Lang": lang, "DownloadCount": 7, "Release": release,
	}
}

func TestSearch(t *testing.T) {
	s := newSites(t)
	s.on(gettoken, reply(http.StatusOK, ported.Response(t, "titlovi_gettoken_response.json")))
	pages := map[string][]any{
		"": {
			hit(1, "Nikita aka La Femme Nikita", "Srpski", 1990, -1, -1, "720p BluRay x264 DTS-PRoDJi"),
			hit(2, "Nikita", "Cirilica", 1990, -1, -1, ""),
		},
		"2": {hit(3, "Nikita", "Hrvatski", 1991, -1, -1, "")},
		"3": {hit(4, "Nikita", "English", 1990, -1, -1, "")},
	}
	s.on(search, func(r *http.Request) (int, []byte) {
		q := r.URL.Query()
		if q.Get("imdbID") != "tt0100263" || q.Get("lang") != "Srpski|Cirilica|Hrvatski|English" || q.Has("season") {
			t.Errorf("query = %v", q)
		}
		body, _ := json.Marshal(map[string]any{"PagesAvailable": 5, "SubtitleResults": pages[q.Get("pg")]})
		return http.StatusOK, body
	})
	v := &provider.Video{Title: "La Femme Nikita", Year: 1990, ExternalIDs: map[string]string{"imdb": "tt0100263"}}
	langs := []provider.Language{{Tag: "sr"}, {Tag: "sr-Latn"}, {Tag: "sr-Cyrl"}, {Tag: "hr"}, {Tag: "en", HearingImpaired: true}}
	p := newProvider(t, s, "")
	subs, err := p.Search(t.Context(), v, langs)
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	for _, sub := range subs {
		tags = append(tags, sub.Language.Tag)
	}
	// Three pages are searched; English is not asked for as such.
	if want := []string{"sr", "sr-Cyrl", "hr"}; !slices.Equal(tags, want) {
		t.Fatalf("languages = %v, want = %v", tags, want)
	}
	if !slices.Equal(subs[0].Matches, []string{"title", "year"}) || !slices.Equal(subs[0].ReleaseNames, []string{"720p BluRay x264 DTS-PRoDJi"}) ||
		subs[0].Downloads != 7 || subs[1].ReleaseNames != nil || !slices.Equal(subs[2].Matches, nil) {
		t.Errorf("subtitles = %+v", subs)
	}
	if got := s.count(search); got != 3 {
		t.Errorf("searches = %d, want = 3", got)
	}
	// Searches are cached.
	if _, err := p.Search(t.Context(), v, langs); err != nil || s.count(search) != 3 {
		t.Errorf("searches = %d, %v, want = 3", s.count(search), err)
	}

	s.on(search, reply(http.StatusTooManyRequests, nil))
	v.Year = 1991
	if _, err := p.Search(t.Context(), v, langs[:1]); provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("err = %v, want = TooManyRequests", err)
	}
	// Other failures are logged.
	s.on(search, reply(http.StatusOK, []byte("<html>")))
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "hr"}}); err != nil || subs != nil {
		t.Errorf("search = %v, %v, want none", subs, err)
	}
}

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
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
	return buf.Bytes()
}

func TestDownload(t *testing.T) {
	s := newSites(t)
	s.on(gettoken, reply(http.StatusOK, ported.Response(t, "titlovi_gettoken_response.json")))
	files := map[string][]byte{
		"bundle": zipOf(t, "Movie.cyr.srt", "Movie.lat.srt"),
		"single": zipOf(t, "Movie.srt"),
		"bare":   []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nbare\r\n"),
		"junk":   []byte("junk"),
		"pack":   zipOf(t, "Show.S01E01.srt", "Show.S01E02.srt"),
	}
	s.on("GET titlovi.com/download/", func(r *http.Request) (int, []byte) {
		switch m := r.URL.Query().Get("mediaid"); m {
		case "limited":
			return http.StatusTooManyRequests, nil
		case "gone":
			return http.StatusNotFound, nil
		default:
			return http.StatusOK, files[m]
		}
	})
	id := func(media, lang string, extra ...string) string {
		v := url.Values{"link": {"https://titlovi.com/download/?type=1&mediaid=" + media}, "lang": {lang}}
		for i := 0; i+1 < len(extra); i += 2 {
			v.Set(extra[i], extra[i+1])
		}
		return v.Encode()
	}
	p := newProvider(t, s, "")
	for _, c := range []struct {
		id, want string
		kind     provider.Kind
	}{
		{id("bundle", "sr"), "Movie.lat.srt", 0},
		{id("bundle", "sr-Cyrl"), "Movie.cyr.srt", 0},
		{id("single", "hr"), "Movie.srt", 0},
		{id("bare", "hr"), "bare", 0},
		{id("pack", "sr", "season", "1", "episode", "2", "pack", "1"), "Show.S01E02.srt", 0},
		{id("pack", "sr", "season", "1", "episode", "3", "pack", "1"), "", provider.BadSubtitle},
		{id("junk", "hr"), "", provider.BadSubtitle},
		{id("limited", "hr"), "", provider.TooManyRequests},
		{id("gone", "hr"), "", provider.ProviderError},
	} {
		file, err := p.Download(t.Context(), nil, c.id)
		if c.kind != 0 {
			if k := provider.KindOf(err); k != c.kind {
				t.Errorf("%s: err = %v, want = %v", c.id, err, c.kind)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.id, err)
			continue
		}
		if !strings.Contains(string(file.Data), c.want) || strings.Contains(string(file.Data), "\r") {
			t.Errorf("%s: file = %q, want = %s", c.id, file.Data, c.want)
		}
	}
}
