package opensubtitlescom

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/cache"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fakeAPI is OpenSubtitles.com with an account whose first session
// expires after the first search.
type fakeAPI struct {
	t      *testing.T
	srv    *httptest.Server
	logins atomic.Int32
	// status answers the next request of a path with a status.
	status map[string]int

	mu       sync.Mutex
	queries  []string
	features []string
	results  string
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, status: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "me" || body["password"] != "pw" {
			http.Error(w, `{"message":"bad"}`, http.StatusUnauthorized)
			return
		}
		n := f.logins.Add(1)
		_, _ = fmt.Fprintf(w, `{"token":"t%d","base_url":"vip-api.opensubtitles.com"}`, n)
	})
	mux.HandleFunc("GET /api/v1/features", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		f.mu.Lock()
		f.features = append(f.features, r.URL.RawQuery)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"data":[
			{"id":"7","attributes":{"title":"Up","year":"1990"}},
			{"id":"8","attributes":{"title":"Up","year":"2009"}}]}`))
	})
	mux.HandleFunc("GET /api/v1/subtitles", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		if f.answer(w, "subtitles") {
			return
		}
		f.mu.Lock()
		f.queries = append(f.queries, r.URL.RawQuery)
		res := f.results
		f.mu.Unlock()
		_, _ = w.Write([]byte(res))
	})
	mux.HandleFunc("POST /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer t1" {
			http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
			return
		}
		f.check(r)
		if f.answer(w, "download") {
			return
		}
		var body struct {
			FileID    int64  `json:"file_id"`
			SubFormat string `json:"sub_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.SubFormat != "srt" {
			f.t.Errorf("sub_format = %q, want = srt", body.SubFormat)
		}
		_, _ = fmt.Fprintf(w, `{"link":"%s/file/%d","file_name":"b.srt"}`, f.srv.URL, body.FileID)
	})
	mux.HandleFunc("GET /file/22", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n你好\r\n"))
	})
	mux.HandleFunc("GET /file/33", func(w http.ResponseWriter, r *http.Request) {})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// answer answers a request with the status set for its path, once.
func (f *fakeAPI) answer(w http.ResponseWriter, path string) bool {
	f.mu.Lock()
	code, ok := f.status[path]
	delete(f.status, path)
	f.mu.Unlock()
	if !ok {
		return false
	}
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"message":"no","requests":20,"remaining":0,"reset_time":"5 hours"}`))
	return true
}

func (f *fakeAPI) check(r *http.Request) {
	if r.Header.Get("Api-Key") != "key" || r.Header.Get("User-Agent") != userAgent || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer t") {
		f.t.Errorf("%s: headers %v", r.URL.Path, r.Header)
	}
}

func newProvider(t *testing.T, f *fakeAPI, extra string) *Provider {
	t.Helper()
	settings := `{"api_key":"key","username":"me","password":"pw"` + extra + `}`
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

const results = `{"data":[
	{"id":"1","attributes":{"language":"zh-cn","download_count":10,"release":"Other.Release","url":"https://www.opensubtitles.com/1","uploader":{"name":"u"},
		"feature_details":{"season_number":1,"episode_number":3,"parent_imdb_id":12345,"year":2009},"files":[{"file_id":11,"file_name":"a.srt"}]}},
	{"id":"2","attributes":{"language":"zh-cn","download_count":5,"moviehash_match":true,"hearing_impaired":true,"release":"Show.S01E02",
		"feature_details":{"season_number":1,"episode_number":2,"parent_imdb_id":12345},"files":[{"file_id":22,"file_name":"b.srt"}]}},
	{"id":"3","attributes":{"language":"pt-br","foreign_parts_only":true,"release":"Forced","feature_details":{},"files":[{"file_id":33}]}},
	{"id":"4","attributes":{"language":"en","ai_translated":true,"release":"AI","feature_details":{},"files":[{"file_id":44}]}},
	{"id":"5","attributes":{"language":"en","machine_translated":true,"release":"Machine","feature_details":{},"files":[{"file_id":55}]}},
	{"id":"6","attributes":{"language":"en","release":"No files","feature_details":{},"files":[]}}]}`

func episode() *provider.Video {
	return &provider.Video{
		Episode: true, Title: "Pilot", Series: "Show", SeriesExternalIDs: map[string]string{"imdb": "tt0012345"},
		Season: 1, Number: 2, Year: 2009, FileHash: "0123456789abcdef",
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.results = results
	p := newProvider(t, f, "")
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "zh-Hans"}, {Tag: "pt"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "ai_translated=exclude&episode_number=2&languages=pt-pt%2Czh-cn&moviehash=0123456789abcdef&parent_imdb_id=12345&season_number=1"
	if len(f.queries) != 1 || f.queries[0] != want {
		t.Errorf("query = %v, want = %s", f.queries, want)
	}
	// Forced subtitles are left out when no forced ones are asked for, as
	// are translated ones and those without files.
	if got := ids(subs); !slices.Equal(got, []string{"11", "22"}) {
		t.Fatalf("subtitles = %v, want = [11 22]", got)
	}
	if got, want := subs[0].Matches, []string{"series", "season", "series_imdb_id", "year"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want = %v", got, want)
	}
	if got, want := subs[1].Matches, []string{"series", "season", "episode", "series_imdb_id", "year", "hash"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want = %v", got, want)
	}
	s := subs[0]
	if s.Language != (provider.Language{Tag: "zh-Hans"}) || s.PageLink != "https://www.opensubtitles.com/1" || s.Uploader != "u" || s.Downloads != 10 {
		t.Errorf("subtitle = %+v", s)
	}
	if !subs[1].Language.HearingImpaired {
		t.Errorf("hearing impaired = false, want = true")
	}
}

func TestSearchForced(t *testing.T) {
	f := newFake(t)
	f.results = results
	p := newProvider(t, f, `,"use_hash":false,"include_ai_translated":true,"include_machine_translated":true`)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "pt-BR", Forced: true}})
	if err != nil {
		t.Fatal(err)
	}
	want := "episode_number=2&languages=pt-br&machine_translated=include&parent_imdb_id=12345&season_number=1"
	if f.queries[0] != want {
		t.Errorf("query = %s, want = %s", f.queries[0], want)
	}
	if got := ids(subs); !slices.Equal(got, []string{"33"}) {
		t.Errorf("subtitles = %v, want = [33]", got)
	}
	if subs[0].Language != (provider.Language{Tag: "pt-BR", Forced: true}) {
		t.Errorf("language = %+v", subs[0].Language)
	}
	f.mu.Lock()
	f.queries = nil
	f.mu.Unlock()
	subs, err = p.Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(subs); !slices.Equal(got, []string{"11", "22", "44", "55"}) {
		t.Errorf("subtitles = %v, want = [11 22 44 55]", got)
	}
}

func TestSearchMovieByTitle(t *testing.T) {
	f := newFake(t)
	f.results = `{"data":[{"attributes":{"language":"fr","release":"Up.2009","feature_details":{"year":2009},"files":[{"file_id":9}]}}]}`
	p := newProvider(t, f, "")
	v := &provider.Video{Title: "Up", Year: 2009}
	for range 2 {
		subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "fr"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"title", "year"}) {
			t.Errorf("subtitles = %+v", subs)
		}
	}
	// The title's ID is looked up once and kept.
	if want := []string{"query=up"}; !slices.Equal(f.features, want) {
		t.Errorf("features = %v, want = %v", f.features, want)
	}
	if want := "ai_translated=exclude&id=8&languages=fr"; f.queries[0] != want {
		t.Errorf("query = %s, want = %s", f.queries[0], want)
	}
	unknown := &provider.Video{Title: "Down", Year: 2009}
	if subs, err := p.Search(t.Context(), unknown, []provider.Language{{Tag: "fr"}}); err != nil || len(subs) != 0 {
		t.Errorf("search of an unknown title = %v, %v, want none", subs, err)
	}
}

func TestKeptAcrossRestarts(t *testing.T) {
	f := newFake(t)
	f.results = `{"data":[{"attributes":{"language":"fr","release":"Up.2009","feature_details":{"year":2009},"files":[{"file_id":9}]}}]}`
	dir := t.TempDir()
	v := &provider.Video{Title: "Up", Year: 2009}
	for range 2 {
		p := newProvider(t, f, "")
		p.cache = cache.New(provider.Env{Now: time.Now, Log: slog.New(slog.DiscardHandler), DataDir: dir})
		if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "fr"}}); err != nil || len(subs) != 1 {
			t.Fatalf("search = %v, %v, want one subtitle", subs, err)
		}
	}
	// The session and the title's ID outlive the provider.
	if n := f.logins.Load(); n != 1 {
		t.Errorf("logins = %d, want = 1", n)
	}
	if want := []string{"query=up"}; !slices.Equal(f.features, want) {
		t.Errorf("features = %v, want = %v", f.features, want)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	f.results = results
	p := newProvider(t, f, "")
	if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "zh-Hans"}}); err != nil {
		t.Fatal(err)
	}
	// The first session has expired by the download, which logs in again.
	got, err := p.Download(t.Context(), episode(), "22")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\n你好\n"; got.Format != "srt" || string(got.Data) != want {
		t.Errorf("download = %q, want = %q", got.Data, want)
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("logins = %d, want = 2", n)
	}
	if _, err := p.Download(t.Context(), episode(), "x"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("download of a malformed ID: kind = %v, want = BadSubtitle", provider.KindOf(err))
	}
	if _, err := p.Download(t.Context(), episode(), "33"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("download of an empty file: err = %v, want = ProviderError", err)
	}
}

func TestStatusCodes(t *testing.T) {
	for _, c := range []struct {
		code  int
		kind  provider.Kind
		tries int
	}{
		{http.StatusBadRequest, provider.ConfigurationError, 1},
		{http.StatusForbidden, provider.ProviderError, 2},
		{http.StatusNotAcceptable, provider.DownloadLimitExceeded, 1},
		{http.StatusGone, provider.ProviderError, 2},
		{http.StatusTooManyRequests, provider.TooManyRequests, 1},
		{http.StatusBadGateway, provider.APIThrottled, 2},
		{http.StatusServiceUnavailable, provider.ProviderError, 2},
	} {
		t.Run(fmt.Sprint(c.code), func(t *testing.T) {
			f := newFake(t)
			f.results = `{"data":[]}`
			p := newProvider(t, f, "")
			f.status["subtitles"] = c.code
			_, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
			// Retried failures succeed on the second try.
			if c.tries > 1 {
				if err != nil || len(f.queries) != 1 {
					t.Errorf("err = %v, queries = %d, want = nil, 1", err, len(f.queries))
				}
				return
			}
			if got := provider.KindOf(err); got != c.kind {
				t.Errorf("kind = %v, want = %v (%v)", got, c.kind, err)
			}
		})
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{}`, `{"username":"me","password":"pw"}`, `{"api_key":"k","username":"me"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s): err = %v, want = ConfigurationError", s, err)
		}
	}
}

func TestLanguages(t *testing.T) {
	for tag, site := range map[string]string{"pt": "pt-pt", "pt-BR": "pt-br", "zh-Hans": "zh-cn", "zh-Hant": "zh-tw", "es-MX": "ea", "en": "en"} {
		if got := toSite(tag); got != site {
			t.Errorf("toSite(%s) = %s, want = %s", tag, got, site)
		}
		if got := fromSite(site); got != tag {
			t.Errorf("fromSite(%s) = %s, want = %s", site, got, tag)
		}
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}
