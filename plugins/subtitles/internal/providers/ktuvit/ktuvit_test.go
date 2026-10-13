package ktuvit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "ktuvit.json", ported.Cases{
		Facts: map[string]string{
			"test_query_falls_back_to_yearless_search":               "TestYearlessFallback",
			"test_query_does_not_fall_back_when_year_search_matches": "TestYearlessFallback",
		},
	})
}

const (
	imdbID = "tt2866360"
	// movieInfo is a movie's page with one subtitle, as Bazarr's test's.
	movieInfo = `<table id="subtitlesList"><tbody><tr>` +
		"<td>Coherence.2014.1080p.WEB\n<small>by someone</small></td><td></td><td></td><td></td><td></td>" +
		`<td><a data-subtitle-id="SUB_1">download</a></td>` +
		"</tr></tbody></table>"
)

// d wraps a value as Ktuvit's services answer.
func d(v any) map[string]any {
	inner, _ := json.Marshal(v)
	return map[string]any{"d": string(inner)}
}

// fake is Ktuvit, with TMDB.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	routes   map[string]http.HandlerFunc
	requests []string
	bodies   []map[string]any
}

func newFake(t *testing.T) *fake {
	f := &fake{routes: map[string]http.HandlerFunc{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.bodies = append(f.bodies, body)
		h := f.routes[r.Method+" "+r.URL.Path]
		f.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), bodyKey{}, body)))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type bodyKey struct{}

// bodyOf returns the JSON body a request posted.
func bodyOf(r *http.Request) map[string]any {
	body, _ := r.Context().Value(bodyKey{}).(map[string]any)
	return body
}

func (f *fake) on(route string, h http.HandlerFunc) {
	f.mu.Lock()
	f.routes[route] = h
	f.mu.Unlock()
}

func jsonOf(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(v) }
}

func text(s string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(s)) }
}

func (f *fake) provider(t *testing.T, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	sp := p.(*Provider)
	u, _ := url.Parse(f.srv.URL)
	sp.SetBaseURL(u)
	return sp
}

// loggedIn returns a provider logged in without an account, as Bazarr's
// test's.
func (f *fake) loggedIn(t *testing.T) *Provider {
	p := f.provider(t, `{}`)
	p.initialized, p.loggedIn = true, true
	return p
}

func film(id string) map[string]any {
	return map[string]any{"IMDB_Link": "http://www.imdb.com/title/" + imdbID + "/", "ID": id}
}

func coherence() *provider.Video {
	return &provider.Video{Title: "Coherence", Year: 2014, ExternalIDs: map[string]string{"imdb": imdbID}}
}

var hebrew = []provider.Language{{Tag: "he"}}

func TestYearlessFallback(t *testing.T) {
	for _, c := range []struct {
		name  string
		films func(year any) []any
		years []any
	}{
		{"falls back", func(year any) []any {
			if year != "" {
				return []any{}
			}
			return []any{film("KT_ID_1")}
		}, []any{2014.0, ""}},
		{"matches with the year", func(any) []any { return []any{film("KT_ID_1")} }, []any{2014.0}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.on("POST /Services/ContentProvider.svc/SearchPage_search", func(w http.ResponseWriter, r *http.Request) {
				req := bodyOf(r)["request"].(map[string]any)
				_ = json.NewEncoder(w).Encode(d(map[string]any{"Films": c.films(req["Year"])}))
			})
			f.on("GET /MovieInfo.aspx", text(movieInfo))
			subs, err := f.loggedIn(t).Search(t.Context(), coherence(), hebrew)
			if err != nil {
				t.Fatal(err)
			}
			var years []any
			for _, b := range f.bodies {
				if b != nil {
					years = append(years, b["request"].(map[string]any)["Year"])
				}
			}
			if !slices.Equal(years, c.years) {
				t.Errorf("years = %v, want = %v", years, c.years)
			}
			want := provider.Subtitle{
				ID: "KT_ID_1/SUB_1", Language: provider.Language{Tag: "he"}, ReleaseNames: []string{"Coherence.2014.1080p.WEB"},
				Matches: []string{"title"}, PageLink: f.srv.URL + "/MovieInfo.aspx?ID=KT_ID_1",
			}
			if len(subs) != 1 || !equal(subs[0], want) {
				t.Errorf("subtitles = %+v, want = %+v", subs, want)
			}
		})
	}
}

func equal(a, b provider.Subtitle) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestLogin(t *testing.T) {
	if _, err := New(provider.Env{}, json.RawMessage(`{"email":"a@b.c"}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("err = %v, want = ConfigurationError", err)
	}

	// Without an account nothing is searched.
	f := newFake(t)
	if subs, err := f.provider(t, `{}`).Search(t.Context(), coherence(), hebrew); err != nil || subs != nil || len(f.requests) != 0 {
		t.Errorf("search = %v, %v after %v", subs, err, f.requests)
	}

	for _, c := range []struct {
		name   string
		login  http.HandlerFunc
		kind   provider.Kind
		logged bool
	}{
		{"success", func(w http.ResponseWriter, r *http.Request) {
			req := bodyOf(r)["request"].(map[string]any)
			if req["Email"] != "a@b.c" || req["Password"] != "hash" {
				t.Errorf("login = %v", req)
			}
			http.SetCookie(w, &http.Cookie{Name: "Login", Value: "cookie", Path: "/"})
			_ = json.NewEncoder(w).Encode(d(map[string]any{"IsSuccess": true}))
		}, 0, true},
		{"failure", jsonOf(d(map[string]any{"IsSuccess": false, "ErrorMessage": "bad password"})), provider.AuthenticationError, false},
		{"garbage", text("<html>"), provider.AuthenticationError, false},
		{"no cookie", jsonOf(d(map[string]any{"IsSuccess": true})), provider.AuthenticationError, false},
		{"empty", text(""), 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.on("POST /Services/MembershipService.svc/Login", c.login)
			f.on("POST /Services/ContentProvider.svc/SearchPage_search", func(w http.ResponseWriter, r *http.Request) {
				if cookie, err := r.Cookie("Login"); err != nil || cookie.Value != "cookie" {
					t.Errorf("Login cookie = %v, %v", cookie, err)
				}
				_ = json.NewEncoder(w).Encode(d(map[string]any{"Films": []any{}}))
			})
			_, err := f.provider(t, `{"email":"a@b.c","hashed_password":"hash"}`).Search(t.Context(), coherence(), hebrew)
			if k := provider.KindOf(err); err != nil && k != c.kind || err == nil && c.kind != 0 {
				t.Errorf("err = %v, want = %v", err, c.kind)
			}
			if searched := slices.Contains(f.requests, "POST /Services/ContentProvider.svc/SearchPage_search"); searched != c.logged {
				t.Errorf("searched = %v, want = %v", searched, c.logged)
			}
		})
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.on("GET /3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api_key") != tmdbAPIKey || q.Get("query") != "Greys Anatomy" || q.Get("year") != "2005" {
			t.Errorf("TMDB query = %v", q)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": 1416}}})
	})
	f.on("GET /3/tv/1416/external_ids", jsonOf(map[string]any{"imdb_id": imdbID}))
	f.on("POST /Services/ContentProvider.svc/SearchPage_search", func(w http.ResponseWriter, r *http.Request) {
		req := bodyOf(r)["request"].(map[string]any)
		if req["SearchType"] != "1" || req["FilmName"] != "Grey's Anatomy" {
			t.Errorf("search = %v", req)
		}
		_ = json.NewEncoder(w).Encode(d(map[string]any{"Films": []any{film("S1"), map[string]any{"IMDB_Link": "http://www.imdb.com/title/tt1", "ID": "S2"}}}))
	})
	f.on("GET /Services/GetModuleAjax.ashx", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("moduleName") != "SubtitlesList" || q.Get("SeriesID") != "S1" || q.Get("Season") != "2" || q.Get("Episode") != "5" {
			t.Errorf("episode query = %v", q)
		}
		_, _ = w.Write([]byte(`<tr><td>Greys.Anatomy.S02E05.720p
extra</td><td></td><td></td><td></td><td></td><td><input data-sub-id="E1"></td></tr>
<tr><td>Greys.Anatomy.S02E05.HDTV</td><td></td><td></td><td></td><td></td><td><input data-sub-id="E2"></td></tr>`))
	})
	v := &provider.Video{Episode: true, Series: "Grey's Anatomy", Year: 2005, Season: 2, Number: 5}
	p := f.loggedIn(t)
	subs, err := p.Search(t.Context(), v, hebrew)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 || subs[0].ID != "S1/E1" || !slices.Equal(subs[0].ReleaseNames, []string{"Greys.Anatomy.S02E05.720p"}) ||
		!slices.Equal(subs[0].Matches, []string{"series", "season", "episode"}) {
		t.Errorf("subtitles = %+v", subs)
	}

	// No subtitles, and languages Ktuvit has not.
	f.on("GET /Services/GetModuleAjax.ashx", text("<tr><td>"+noSubtitles+"</td></tr>"))
	v.Number = 6
	if subs, err := p.Search(t.Context(), v, hebrew); err != nil || subs != nil {
		t.Errorf("search = %v, %v, want none", subs, err)
	}
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "he", HearingImpaired: true}}); err != nil || subs != nil {
		t.Errorf("search = %v, %v, want none", subs, err)
	}
	// The IMDb ID is cached.
	var lookups int
	for _, r := range f.requests {
		if r == "GET /3/search/tv" {
			lookups++
		}
	}
	if lookups != 1 {
		t.Errorf("TMDB searches = %d, want = 1", lookups)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	f.on("POST /Services/ContentProvider.svc/RequestSubtitleDownload", func(w http.ResponseWriter, r *http.Request) {
		req := bodyOf(r)["request"].(map[string]any)
		if req["FilmID"] != "KT_ID_1" || req["SubtitleID"] != "SUB_1" || req["PredefinedLayout"] != -1.0 {
			t.Errorf("request = %v", req)
		}
		_ = json.NewEncoder(w).Encode(d(map[string]any{"DownloadIdentifier": "ident"}))
	})
	f.on("GET /Services/DownloadFile.ashx", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("DownloadIdentifier") == "ident" {
			_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nשלום\r\n"))
		}
	})
	p := f.loggedIn(t)
	file, err := p.Download(t.Context(), nil, "KT_ID_1/SUB_1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nשלום\n"; string(file.Data) != want || file.Format != "srt" {
		t.Errorf("file = %q %s, want = %q srt", file.Data, file.Format, want)
	}

	f.on("POST /Services/ContentProvider.svc/RequestSubtitleDownload", jsonOf(d(map[string]any{"DownloadIdentifier": "other"})))
	if _, err := p.Download(t.Context(), nil, "KT_ID_1/SUB_1"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("err = %v, want = BadSubtitle", err)
	}
	f.on("POST /Services/ContentProvider.svc/RequestSubtitleDownload", jsonOf(d(map[string]any{})))
	if _, err := p.Download(t.Context(), nil, "KT_ID_1/SUB_1"); provider.KindOf(err) != provider.ParseResponseError {
		t.Errorf("err = %v, want = ParseResponseError", err)
	}
	f.on("POST /Services/ContentProvider.svc/RequestSubtitleDownload", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := p.Download(t.Context(), nil, "KT_ID_1/SUB_1"); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("err = %v, want = ServiceUnavailable", err)
	}
}
