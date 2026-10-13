package subsource

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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// pyBytes decodes the Python bytes literals of the ported cases.
func pyBytes(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var e struct {
		Expr string `json:"$expr"`
	}
	if err := json.Unmarshal(raw, &e); err != nil || len(e.Expr) < 3 || e.Expr[0] != 'b' {
		t.Fatalf("not a bytes literal: %s", raw)
	}
	s := e.Expr[1:]
	if s[0] == '\'' {
		return []byte(s[1 : len(s)-1])
	}
	u, err := strconv.Unquote(s)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(u)
}

func TestPorted(t *testing.T) {
	ported.Run(t, "subsource.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_retry_after_is_none_when_the_server_does_not_say": func(t *testing.T, args map[string]json.RawMessage) {
				r := rateLimited(pyBytes(t, args["body"]), nil)
				if n, ok := retryAfter(r, time.Now()); ok {
					t.Errorf("retryAfter = %d, want = none", n)
				}
			},
		},
		Facts: map[string]string{
			"test_convert_brazilian_portuguese":                                 "TestLanguageNames",
			"test_convert_plain_portuguese_still_works":                         "TestLanguageNames",
			"test_convert_brazilian_portuguese_round_trip":                      "TestLanguageNames",
			"test_retry_after_prefers_the_reset_header":                         "TestRetryAfter",
			"test_retry_after_is_never_zero":                                    "TestRetryAfter",
			"test_checked_waits_out_a_rate_limit_that_resets_within_the_minute": "TestRateLimits",
			"test_checked_throttles_on_a_rate_limit_that_outlasts_the_minute":   "TestRateLimits",
			"test_checked_throttles_when_the_server_reports_no_delay":           "TestRateLimits",
			"test_checked_waits_only_once":                                      "TestRateLimits",
		},
	})
}

func rateLimited(body []byte, resetIn *time.Duration) *site.Response {
	r := &site.Response{Status: http.StatusTooManyRequests, Header: http.Header{}, Body: body, URL: &url.URL{Path: "/"}}
	if resetIn != nil {
		r.Header.Set("X-RateLimit-Reset", time.Now().UTC().Add(*resetIn).Format("2006-01-02T15:04:05.000000Z"))
	}
	return r
}

// TestLanguageNames ports the converter's facts with its names: Bazarr's
// tests expect "Brazillian Portuguese", which its converter does not have.
func TestLanguageNames(t *testing.T) {
	if got := names["pt-BR"]; got != "Brazilian_portuguese" {
		t.Errorf("pt-BR = %q, want = Brazilian_portuguese", got)
	}
	if got := names["pt"]; got != "Portuguese" {
		t.Errorf("pt = %q, want = Portuguese", got)
	}
	for _, name := range []string{"Brazilian_portuguese", "brazilian_portuguese"} {
		if got := tagOf(name); got != "pt-BR" {
			t.Errorf("tagOf(%q) = %q, want = pt-BR", name, got)
		}
	}
	if tagOf("farsi_persian") != "fa" || tagOf("northen sami") != "se" || tagOf("Klingon") != "" { //nolint:misspell // the site's spelling
		t.Error("language names misread")
	}
}

func TestRetryAfter(t *testing.T) {
	in := 26 * time.Second
	if n, ok := retryAfter(rateLimited([]byte(`{"retryAfter": 3}`), &in), time.Now()); !ok || n != 26 {
		t.Errorf("retryAfter = %d %v, want = 26", n, ok)
	}
	if n, ok := retryAfter(rateLimited([]byte(`{"retryAfter": 3.2}`), nil), time.Now()); !ok || n != 4 {
		t.Errorf("retryAfter = %d %v, want = 4", n, ok)
	}
	past := -5 * time.Second
	if n, ok := retryAfter(rateLimited(nil, &past), time.Now()); !ok || n != 1 {
		t.Errorf("retryAfter = %d %v, want = 1", n, ok)
	}
}

// fake is SubSource's API.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	routes   map[string][]http.HandlerFunc
	requests []*http.Request
}

func newFake(t *testing.T) *fake {
	f := &fake{routes: map[string][]http.HandlerFunc{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("api_key"); got != "key" {
			t.Errorf("api_key = %q, want = key", got)
		}
		f.mu.Lock()
		f.requests = append(f.requests, r)
		hs := f.routes[r.URL.Path]
		var h http.HandlerFunc
		if len(hs) > 0 {
			h = hs[0]
			if len(hs) > 1 {
				f.routes[r.URL.Path] = hs[1:]
			}
		}
		f.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// on answers a path with the handlers in turn, the last one repeated.
func (f *fake) on(path string, hs ...http.HandlerFunc) {
	f.mu.Lock()
	f.routes[path] = hs
	f.mu.Unlock()
}

func jsonOf(body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(body) }
}

func limited(resetIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", time.Now().UTC().Add(resetIn).Format(time.RFC3339Nano))
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

func (f *fake) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		out = append(out, r.URL.Path)
	}
	return out
}

func (f *fake) provider(t *testing.T) (*Provider, *[]time.Duration) {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"apikey":"key"}`))
	if err != nil {
		t.Fatal(err)
	}
	sp := p.(*Provider)
	u, _ := url.Parse(f.srv.URL)
	sp.SetBaseURL(u)
	var slept []time.Duration
	sp.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return sp, &slept
}

var ok = jsonOf(map[string]any{"data": []any{}})

func TestRateLimits(t *testing.T) {
	for _, c := range []struct {
		name    string
		answers []http.HandlerFunc
		kind    provider.Kind
		tries   int
	}{
		{"within the minute", []http.HandlerFunc{limited(maxInlineWait * time.Second), ok}, 0, 2},
		{"outlasting the minute", []http.HandlerFunc{limited((maxInlineWait + 2) * time.Second)}, provider.TooManyRequests, 1},
		{"no delay", []http.HandlerFunc{func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }}, provider.TooManyRequests, 1},
		{"waits once", []http.HandlerFunc{limited(5 * time.Second)}, provider.TooManyRequests, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.on("/api/v1/movies/search", c.answers...)
			p, slept := f.provider(t)
			_, err := p.titles(t.Context(), url.Values{})
			if k := provider.KindOf(err); err != nil && k != c.kind || err == nil && c.kind != 0 {
				t.Errorf("err = %v, want = %v", err, c.kind)
			}
			if got := len(f.paths()); got != c.tries {
				t.Errorf("tries = %d, want = %d", got, c.tries)
			}
			if c.tries == 2 && (len(*slept) != 1 || (*slept)[0] > maxInlineWait*time.Second) {
				t.Errorf("slept = %v", *slept)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	for code, kind := range map[int]provider.Kind{
		400: provider.APIThrottled, 401: provider.AuthenticationError, 403: provider.ForbiddenError,
		404: provider.ProviderError, 502: provider.ServiceUnavailable,
	} {
		f := newFake(t)
		f.on("/api/v1/movies/search", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
		p, _ := f.provider(t)
		if _, err := p.Search(t.Context(), dune(), english); provider.KindOf(err) != kind {
			t.Errorf("%d: err = %v, want = %v", code, err, kind)
		}
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("err = %v, want = ConfigurationError", err)
	}
}

func TestNetworkRetries(t *testing.T) {
	var tries int
	hc := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		tries++
		return nil, errNetwork
	})}
	p, err := New(provider.Env{HTTP: hc, Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"apikey":"key"}`))
	if err != nil {
		t.Fatal(err)
	}
	p.(*Provider).sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := p.Search(t.Context(), dune(), english); provider.KindOf(err) != provider.ServiceUnavailable || tries != retryAmount {
		t.Errorf("err = %v after %d tries, want = ServiceUnavailable after %d", err, tries, retryAmount)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type netErr struct{}

func (netErr) Error() string { return "connection refused" }

var errNetwork = netErr{}

var english = []provider.Language{{Tag: "en"}}

func dune() *provider.Video {
	return &provider.Video{Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"}, ReleaseGroup: "SHITBOX"}
}

func got() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Game of Thrones", Season: 3, Number: 10, Year: 2011, ReleaseGroup: "NTb",
		SeriesExternalIDs: map[string]string{"imdb": "tt0944947"},
	}
}

func sub(id int, lang string, releases []string, extra map[string]any) map[string]any {
	m := map[string]any{
		"subtitleId": id, "link": "/subtitle/x/" + strconv.Itoa(id), "language": lang, "releaseInfo": releases,
		"commentary": "", "hearingImpaired": false, "foreignParts": false, "uploaderId": 7,
		"contributors": []any{map[string]any{"id": 3, "displayname": "other"}, map[string]any{"id": 7, "displayname": "uploader"}},
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.on("/api/v1/movies/search", jsonOf(map[string]any{"data": []any{
		map[string]any{"title": "Dune", "releaseYear": "1984", "movieId": 1},
		map[string]any{"title": "Dune: Part One", "releaseYear": 2021, "movieId": 2},
	}}))
	f.on("/api/v1/subtitles", jsonOf(map[string]any{"success": true, "data": []any{
		sub(10, "english", []string{"Dune.2021.1080p.WEBRip.x264-SHITBOX"}, nil),
		sub(11, "english", []string{"Dune.2021.Forced"}, map[string]any{"commentary": "Foreign parts only"}),
		sub(12, "english", []string{"Dune.2021.SDH"}, map[string]any{"commentary": "SDH version"}),
		sub(13, "english", []string{"Dune.2021.NonHI"}, map[string]any{"commentary": "Non-SDH, hi removed", "hearingImpaired": false}),
	}}))
	p, _ := f.provider(t)
	subs, err := p.Search(t.Context(), dune(), english)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"10:0:0:false:false", "12:0:0:false:false", "13:0:0:false:false"}; !slices.Equal(ids(subs), want) {
		t.Fatalf("subtitles = %v, want = %v", ids(subs), want)
	}
	s := subs[0]
	if s.Language != (provider.Language{Tag: "en"}) || s.Uploader != "uploader" || s.PageLink != "https://subsource.net/subtitle/x/10" ||
		!slices.Equal(s.Matches, []string{"title", "imdb_id"}) || !slices.Equal(s.ReleaseNames, []string{"Dune.2021.1080p.WEBRip.x264-SHITBOX"}) {
		t.Errorf("subtitle = %+v", s)
	}
	if !subs[1].Language.HearingImpaired || subs[2].Language.HearingImpaired {
		t.Errorf("hearing impaired = %v %v, want = true false", subs[1].Language.HearingImpaired, subs[2].Language.HearingImpaired)
	}
	q := f.requests[0].URL.Query()
	if q.Get("searchType") != "imdb" || q.Get("imdb") != "tt1160419" || q.Has("season") {
		t.Errorf("title query = %v", q)
	}
	q = f.requests[1].URL.Query()
	if q.Get("language") != "english" || q.Get("movieId") != "2" || q.Get("limit") != "100" {
		t.Errorf("subtitles query = %v", q)
	}

	// Searches are cached.
	if _, err := p.Search(t.Context(), dune(), english); err != nil || len(f.paths()) != 2 {
		t.Errorf("requests = %v, %v, want = 2", f.paths(), err)
	}

	// Forced subtitles answer forced languages only, HI ones HI languages.
	f.on("/api/v1/subtitles", jsonOf(map[string]any{"data": []any{
		sub(11, "english", []string{"Dune.2021"}, map[string]any{"foreignParts": true}),
		sub(12, "english", []string{"Dune.2021"}, map[string]any{"hearingImpaired": true}),
		sub(14, "english", []string{"Dune.2021"}, nil),
	}}))
	subs, err = p.Search(t.Context(), dune(), []provider.Language{{Tag: "en", Forced: true}})
	if err != nil || !slices.Equal(ids(subs), []string{"11:0:0:false:true", "12:0:0:false:false", "14:0:0:false:false"}) {
		t.Errorf("forced = %v, %v", ids(subs), err)
	}
	subs, err = p.Search(t.Context(), dune(), []provider.Language{{Tag: "en", HearingImpaired: true}})
	if err != nil || !slices.Equal(ids(subs), []string{"12:0:0:false:false"}) {
		t.Errorf("HI = %v, %v", ids(subs), err)
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.on("/api/v1/movies/search",
		jsonOf(map[string]any{"data": []any{}}),
		jsonOf(map[string]any{"data": []any{map[string]any{"title": "Game of Thrones - Third Season", "releaseYear": 2011, "movieId": 5}}}))
	f.on("/api/v1/subtitles", jsonOf(map[string]any{"data": []any{
		sub(20, "brazilian_portuguese", []string{"Game.of.Thrones.S03E10.720p.WEB-DL.DD5.1.H.264-NTb"}, nil),
		sub(21, "brazilian_portuguese", []string{"Game.of.Thrones.S03.Complete"}, nil),
		sub(22, "brazilian_portuguese", []string{"Game.of.Thrones.S03E09"}, nil),
		sub(23, "brazilian_portuguese", []string{"Game.of.Thrones.S02E10"}, nil),
		sub(24, "brazilian_portuguese", []string{"Game.of.Thrones.S03E09E10"}, nil),
	}}))
	p, _ := f.provider(t)
	subs, err := p.Search(t.Context(), got(), []provider.Language{{Tag: "pt-BR"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"20:3:10:false:false", "21:3:0:true:false"}; !slices.Equal(ids(subs), want) {
		t.Fatalf("subtitles = %v, want = %v", ids(subs), want)
	}
	if subs[0].Language.Tag != "pt-BR" || !slices.Equal(subs[0].Matches, []string{"series", "series_imdb_id"}) ||
		!slices.Equal(subs[1].Matches, []string{"series", "series_imdb_id", "episode"}) {
		t.Errorf("subtitles = %+v", subs)
	}
	// The IMDb search found nothing, so the title was searched.
	q := f.requests[1].URL.Query()
	if q.Get("searchType") != "text" || q.Get("q") != "game of thrones" || q.Get("season") != "3" {
		t.Errorf("text query = %v", q)
	}
	q = f.requests[2].URL.Query()
	if q.Get("language") != "brazilian_portuguese" || q.Get("seasonNumber") != "3" || q.Get("episodeNumber") != "10" {
		t.Errorf("subtitles query = %v", q)
	}

	// Without an IMDb ID nothing is searched.
	v := got()
	v.SeriesExternalIDs = nil
	if subs, err := p.Search(t.Context(), v, english); err != nil || subs != nil || len(f.paths()) != 3 {
		t.Errorf("search = %v, %v after %v", subs, err, f.paths())
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
	f := newFake(t)
	pack := zipOf(t, "Game.of.Thrones.S03E09.NTb.srt", "Game.of.Thrones.S03E10.NTb.srt", "Game.of.Thrones.S03E10.NTb.forced.srt")
	f.on("/api/v1/subtitles/21/download", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(pack) })
	f.on("/api/v1/subtitles/22/download", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not a zip")) })
	f.on("/api/v1/subtitles/23/download", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(zipOf(t, "a.srt", "b.srt")) })
	p, _ := f.provider(t)
	for range 2 {
		file, err := p.Download(t.Context(), got(), "21:3:0:true:false")
		if err != nil {
			t.Fatal(err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\nGame.of.Thrones.S03E10.NTb.srt\n"; string(file.Data) != want || file.Format != "srt" {
			t.Errorf("file = %q %s, want = %q srt", file.Data, file.Format, want)
		}
	}
	// The archive is cached.
	if got := len(f.paths()); got != 1 {
		t.Errorf("requests = %d, want = 1", got)
	}
	for _, id := range []string{"22:0:0:false:false", "23:3:10:false:false", "nope"} {
		if _, err := p.Download(t.Context(), got(), id); provider.KindOf(err) != provider.BadSubtitle {
			t.Errorf("%s: err = %v, want = BadSubtitle", id, err)
		}
	}
	if !strings.HasPrefix(f.paths()[1], "/api/v1/subtitles/22") {
		t.Errorf("paths = %v", f.paths())
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}
