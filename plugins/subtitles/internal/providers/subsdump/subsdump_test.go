package subsdump

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "subsdump.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_base_url_validation": func(t *testing.T, args map[string]json.RawMessage) {
				u := ported.Arg[*string](t, args, "url")
				s := "{}"
				if u != nil {
					s = fmt.Sprintf(`{"base_url":%q}`, *u)
				}
				if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
					t.Errorf("New(%s): err = %v, want = ConfigurationError", s, err)
				}
			},
			"test_search_http_error": func(t *testing.T, args map[string]json.RawMessage) {
				code := ported.Arg[int](t, args, "status_code")
				f := newFake(t, "")
				f.handle("/api/v1/movies/tt1160419/subtitles", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
				_, err := f.provider(t, "").Search(t.Context(), dune(), arabic)
				wantError(t, err, provider.ProviderError, fmt.Sprintf("HTTP %d", code))
			},
			"test_remote_request_errors": func(t *testing.T, args map[string]json.RawMessage) {
				var e struct {
					Expr string `json:"$expr"`
				}
				_ = json.Unmarshal(args["error"], &e)
				message := ported.Arg[string](t, args, "message")
				kind, failure := provider.ProviderError, errors.New("connection refused")
				switch e.Expr {
				case "Timeout()":
					failure = timeoutError{}
				case "SSLError()":
					kind, failure = provider.ConfigurationError, x509.UnknownAuthorityError{}
				}
				p := failing(t, failure)
				_, err := p.Search(t.Context(), dune(), arabic)
				wantError(t, err, kind, message)
			},
			"test_download_http_errors": func(t *testing.T, args map[string]json.RawMessage) {
				code := ported.Arg[int](t, args, "status_code")
				message := ported.Arg[string](t, args, "message")
				kind := provider.ProviderError
				if code == http.StatusUnauthorized {
					kind = provider.AuthenticationError
				}
				f := newFake(t, "api-secret")
				f.handle("/api/v1/subtitles/42/content", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
				_, err := f.provider(t, "api-secret").Download(t.Context(), nil, "42")
				wantError(t, err, kind, message)
			},
		},
		Facts: map[string]string{
			"test_initialize_uses_optional_api_key_and_cleanup":                  "TestInitialize",
			"test_initialize_without_api_key":                                    "TestInitialize",
			"test_initialize_rejects_an_incompatible_service_and_closes_session": "TestInitialize",
			"test_movie_lookup_returns_matches_and_page_link":                    "TestSearch",
			"test_episode_lookup_maps_season_episode_and_release":                "TestSearch",
			"test_title_fallback_for_episode_without_imdb_id":                    "TestSearch",
			"test_language_mapping_and_hearing_impaired_metadata":                "TestSearch",
			"test_no_results":                            "TestSearch",
			"test_malformed_json":                        "TestSearchFailures",
			"test_malformed_subtitle_record_is_ignored":  "TestSearchFailures",
			"test_invalid_authentication":                "TestSearchFailures",
			"test_download_uses_native_content_endpoint": "TestDownload",
			"test_download_rejects_empty_content":        "TestDownload",
			"test_download_timeout":                      "TestDownload",
		},
	})
}

var arabic = []provider.Language{{Tag: "ar"}}

func dune() *provider.Video {
	return &provider.Video{
		Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"}, ReleaseGroup: "SHITBOX",
		FileName: "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX.mkv",
	}
}

func got() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Game of Thrones", Season: 3, Number: 10, Title: "Mhysa",
		SeriesExternalIDs: map[string]string{"imdb": "tt0944947"},
		FileName:          "/tv/Game of Thrones/Game.of.Thrones.S03E10.Mhysa.720p.WEB-DL.DD5.1.H.264-NTb.mkv",
	}
}

func rec(id int, title, lang string, year, season, episode any, hi bool) map[string]any {
	return map[string]any{
		"id":       id,
		"media":    map[string]any{"type": "movie", "title": title, "imdb_id": "tt1160419", "year": year, "season": season, "episode": episode},
		"language": map[string]any{"code": lang, "name": "Arabic"},
		"releases": []string{"Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX"}, "hearing_impaired": hi, "uploader": "contributor",
		"links": map[string]any{
			"page": fmt.Sprintf("/subtitles/%d", id), "archive": fmt.Sprintf("/api/v1/subtitles/%d/archive", id),
			"content": fmt.Sprintf("/api/v1/subtitles/%d/content", id),
		},
	}
}

// fake is a SubsDump instance.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	routes   map[string]http.HandlerFunc
	requests []*http.Request
}

func newFake(t *testing.T, key string) *fake {
	f := &fake{routes: map[string]http.HandlerFunc{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r)
		h := f.routes[r.URL.Path]
		f.mu.Unlock()
		if got := r.Header.Get("X-API-Key"); got != key {
			t.Errorf("X-API-Key = %q, want = %q", got, key)
		}
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	f.handle("/api/v1/info", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"SubsDump","version":"2.1.0","api_version":"v1"}`))
	})
	return f
}

func (f *fake) handle(path string, h http.HandlerFunc) {
	f.mu.Lock()
	f.routes[path] = h
	f.mu.Unlock()
}

func (f *fake) json(pattern string, body any) {
	f.handle(pattern, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(body) })
}

func (f *fake) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fake) provider(t *testing.T, key string) *Provider {
	t.Helper()
	p, err := New(env(f.srv.Client()), json.RawMessage(fmt.Sprintf(`{"base_url":%q,"api_key":%q}`, f.srv.URL+"/", key)))
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func env(hc *http.Client) provider.Env {
	return provider.Env{HTTP: hc, Now: time.Now, Log: slog.New(slog.DiscardHandler)}
}

type timeoutError struct{}

func (timeoutError) Error() string { return "i/o timeout" }
func (timeoutError) Timeout() bool { return true }

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// failing returns a provider whose initialized instance fails with err.
func failing(t *testing.T, failure error) *Provider {
	hc := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) { return nil, failure })}
	p, err := New(env(hc), json.RawMessage(`{"base_url":"https://subsdump.example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	sp := p.(*Provider)
	sp.ready = true
	return sp
}

func wantError(t *testing.T, err error, kind provider.Kind, message string) {
	t.Helper()
	if k := provider.KindOf(err); k != kind || !strings.Contains(fmt.Sprint(err), message) {
		t.Errorf("err = %v (%v), want = %v matching %q", err, k, kind, message)
	}
}

func TestInitialize(t *testing.T) {
	for _, key := range []string{"api-secret", ""} {
		f := newFake(t, key)
		f.json("/api/v1/movies/tt1160419/subtitles", map[string]any{"subtitles": []any{}})
		p := f.provider(t, key)
		if p.base != f.srv.URL {
			t.Errorf("base = %q, want = %q", p.base, f.srv.URL)
		}
		for range 2 {
			if _, err := p.Search(t.Context(), dune(), arabic); err != nil {
				t.Fatal(err)
			}
		}
		var infos int
		for _, r := range f.requests {
			if r.URL.Path == "/api/v1/info" {
				infos++
			}
		}
		if infos != 1 {
			t.Errorf("info requests = %d, want = 1", infos)
		}
	}

	f := newFake(t, "")
	f.json("/api/v1/info", map[string]any{"name": "Another service", "api_version": "v1"})
	_, err := f.provider(t, "").Search(t.Context(), dune(), arabic)
	wantError(t, err, provider.ConfigurationError, "SubsDump v1 API")
}

func TestSearch(t *testing.T) {
	f := newFake(t, "")
	f.json("/api/v1/movies/tt1160419/subtitles", map[string]any{"subtitles": []any{rec(42, "Dune", "ara", 2021, nil, nil, false)}})
	subs, err := f.provider(t, "").Search(t.Context(), dune(), arabic)
	if err != nil {
		t.Fatal(err)
	}
	want := provider.Subtitle{
		ID: "42", Language: provider.Language{Tag: "ar"}, ReleaseNames: []string{"Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX"},
		Matches: []string{"title", "year"}, PageLink: f.srv.URL + "/subtitles/42", Uploader: "contributor",
	}
	if len(subs) != 1 || fmt.Sprint(subs[0]) != fmt.Sprint(want) {
		t.Errorf("subtitles = %+v, want = %+v", subs, want)
	}
	q := f.last().URL.Query()
	if q.Get("language") != "ara" || q.Get("year") != "2021" || q.Get("per_page") != "100" {
		t.Errorf("query = %v", q)
	}

	f = newFake(t, "")
	f.json("/api/v1/series/tt0944947/seasons/3/episodes/10/subtitles", map[string]any{"subtitles": []any{rec(43, "Game of Thrones", "ara", nil, 3, 10, false)}})
	subs, err = f.provider(t, "").Search(t.Context(), got(), arabic)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"series", "season", "episode"}) {
		t.Errorf("subtitles = %+v, want series, season and episode matched", subs)
	}
	if got := f.last().URL.Query().Get("release"); got != "Game.of.Thrones.S03E10.Mhysa.720p.WEB-DL.DD5.1.H.264-NTb.mkv" {
		t.Errorf("release = %q", got)
	}

	// Episodes without an IMDb ID are searched by title.
	f = newFake(t, "")
	f.json("/api/v1/subtitles", map[string]any{"subtitles": []any{}})
	saul := &provider.Video{Episode: true, Series: "better call saul", Season: 6, Number: 4, FileName: "Better.Call.Saul.S06E04.720p.WEBRIP.X264.mkv"}
	if subs, err := f.provider(t, "").Search(t.Context(), saul, arabic); err != nil || len(subs) != 0 {
		t.Fatalf("search = %v, %v, want none", subs, err)
	}
	q = f.last().URL.Query()
	if q.Get("title") != "better call saul" || q.Get("media_type") != "episode" || q.Get("season") != "6" || q.Get("episode") != "4" {
		t.Errorf("query = %v", q)
	}

	// Brazilian Portuguese is "por-BR"; hearing impaired is the record's.
	f = newFake(t, "")
	f.json("/api/v1/movies/tt1160419/subtitles", map[string]any{"subtitles": []any{rec(42, "Dune", "por-BR", 2021, nil, nil, true)}})
	subs, err = f.provider(t, "").Search(t.Context(), dune(), []provider.Language{{Tag: "pt-BR", HearingImpaired: true}, {Tag: "xx"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Language != (provider.Language{Tag: "pt-BR", HearingImpaired: true}) {
		t.Errorf("subtitles = %+v", subs)
	}
	if got := f.last().URL.Query().Get("language"); got != "por-BR" {
		t.Errorf("language = %q, want = por-BR", got)
	}
	if !slices.Contains(Definition().Languages, "pt-BR") || !slices.Contains(Definition().Languages, "fa") {
		t.Errorf("languages = %v", Definition().Languages)
	}
}

func TestSearchFailures(t *testing.T) {
	f := newFake(t, "")
	f.handle("/api/v1/movies/tt1160419/subtitles", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not JSON")) })
	_, err := f.provider(t, "").Search(t.Context(), dune(), arabic)
	wantError(t, err, provider.ProviderError, "malformed JSON")

	f = newFake(t, "")
	malformed := rec(42, "Dune", "ara", 2021, nil, nil, false)
	malformed["links"].(map[string]any)["page"] = "/subtitles/different"
	f.json("/api/v1/movies/tt1160419/subtitles", map[string]any{"subtitles": []any{
		malformed, rec(43, "Dune", "eng", 2021, nil, nil, false), map[string]any{"id": true}, "x",
	}})
	if subs, err := f.provider(t, "").Search(t.Context(), dune(), arabic); err != nil || len(subs) != 0 {
		t.Errorf("search = %v, %v, want none", subs, err)
	}

	for _, c := range []struct {
		code    int
		kind    provider.Kind
		message string
	}{
		{http.StatusUnauthorized, provider.AuthenticationError, "API key"},
		{http.StatusForbidden, provider.AuthenticationError, "API key"},
		{http.StatusServiceUnavailable, provider.ServiceUnavailable, "not ready"},
		{http.StatusFound, provider.ProviderError, "HTTP 302"},
	} {
		f = newFake(t, "wrong-key")
		f.handle("/api/v1/movies/tt1160419/subtitles", func(w http.ResponseWriter, r *http.Request) {
			if c.code == http.StatusFound {
				http.Redirect(w, r, "/elsewhere", c.code)
				return
			}
			w.WriteHeader(c.code)
		})
		_, err = f.provider(t, "wrong-key").Search(t.Context(), dune(), arabic)
		wantError(t, err, c.kind, c.message)
	}

	f = newFake(t, "")
	f.json("/api/v1/movies/tt1160419/subtitles", map[string]any{"results": []any{}})
	_, err = f.provider(t, "").Search(t.Context(), dune(), arabic)
	wantError(t, err, provider.ProviderError, "missing subtitles")
}

func TestDownload(t *testing.T) {
	f := newFake(t, "api-secret")
	f.handle("/api/v1/subtitles/42/content", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"))
	})
	f.handle("/api/v1/subtitles/43/content", func(http.ResponseWriter, *http.Request) {})
	p := f.provider(t, "api-secret")
	file, err := p.Download(t.Context(), nil, "42")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"; string(file.Data) != want || file.Format != "srt" {
		t.Errorf("file = %q %s, want = %q srt", file.Data, file.Format, want)
	}
	_, err = p.Download(t.Context(), nil, "43")
	wantError(t, err, provider.ProviderError, "invalid subtitle file")

	_, err = failing(t, timeoutError{}).Download(t.Context(), nil, "42")
	wantError(t, err, provider.ProviderError, "download timed out")

	if _, err := p.Download(t.Context(), nil, "x"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("err = %v, want = BadSubtitle", err)
	}
}
