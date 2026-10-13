package legendasnet

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nOlá\r\n"

// fakeAPI is Legendas.net with an account whose first session expires
// after the first search.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	logins   int
	searches []map[string]any
	results  string
	status   map[string]int
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, status: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["email"] != "me" || body["password"] != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.logins++
		n := f.logins
		f.mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"access_token":"t%d"}`, n)
	})
	search := func(w http.ResponseWriter, r *http.Request) {
		if f.answer(w, r, "search") {
			return
		}
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("search body %q: %v", data, err)
		}
		body["path"] = r.URL.Path
		f.mu.Lock()
		f.searches = append(f.searches, body)
		f.mu.Unlock()
		_, _ = w.Write([]byte(f.results))
	}
	mux.HandleFunc("GET /api/v1/search/tv", search)
	mux.HandleFunc("GET /api/v1/search/movie", search)
	mux.HandleFunc("GET /uploads/", func(w http.ResponseWriter, r *http.Request) {
		if f.answer(w, r, "download") {
			return
		}
		if r.URL.Path == "/uploads/a.zip" {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, n := range []string{"readme.txt", "a.srt", "b.srt"} {
				fw, _ := zw.Create(n)
				_, _ = fw.Write([]byte(srt + n))
			}
			_ = zw.Close()
			_, _ = w.Write(buf.Bytes())
			return
		}
		_, _ = w.Write([]byte(srt))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// answer checks the session and answers with the status set for a
// request, once; the first session is expired after a search.
func (f *fakeAPI) answer(w http.ResponseWriter, r *http.Request, name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	auth := r.Header.Get("Authorization")
	if auth == "" || auth == "Bearer t1" && len(f.searches) > 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}
	code, ok := f.status[name]
	if !ok {
		return false
	}
	delete(f.status, name)
	w.WriteHeader(code)
	return true
}

func newProvider(t *testing.T, f *fakeAPI) (*Provider, *[]time.Duration) {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"username":"me","password":"pw"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	lp := p.(*Provider)
	lp.SetBaseURL(u)
	var slept []time.Duration
	lp.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return lp, &slept
}

const tvResults = `{"success":true,"tv_shows":[
	{"id":10,"tmdb_id":1399,"path":"/uploads/a.zip","release_name":"Show.S01E02.1080p.WEB-DL","uploader":"ana","comment":"","season":1,"episode":2},
	{"id":11,"tmdb_id":1399,"path":"/uploads/b.srt","release_name":"Show.S01E03.720p","uploader":"bia","comment":"Forced","season":"1","episode":"3"}]}`

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.results = tvResults
	p, _ := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0944947"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"path": "/api/v1/search/tv", "page": 1.0, "per_page": 25.0, "tv_episode": 2.0, "tv_season": 1.0, "imdb_id": "tt0944947"}
	if len(f.searches) != 1 || fmt.Sprint(f.searches[0]) != fmt.Sprint(want) {
		t.Errorf("searches got = %v, want = %v", f.searches, want)
	}
	// Forced subtitles are listed only when asked for.
	if got := ids(subs); !slices.Equal(got, []string{"/uploads/a.zip"}) {
		t.Fatalf("subtitles got = %v, want = [/uploads/a.zip]", got)
	}
	s := subs[0]
	if want := []string{"series", "season", "episode", "series_imdb_id"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	if s.PageLink != f.srv.URL+"/tv_legenda?movie_id=1399&legenda_id=10" || s.Uploader != "ana" || s.ReleaseNames[0] != "Show.S01E02.1080p.WEB-DL" {
		t.Errorf("subtitle got = %+v", s)
	}

	// The session has expired: the search logs in again.
	subs, err = p.Search(t.Context(), v, []provider.Language{{Tag: "pt-BR", Forced: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(subs); !slices.Equal(got, []string{"/uploads/b.srt"}) || !subs[0].Language.Forced {
		t.Fatalf("subtitles got = %+v, want = [/uploads/b.srt] forced", subs)
	}
	if want := []string{"series", "season", "series_imdb_id"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("matches got = %v, want = %v", subs[0].Matches, want)
	}
	if f.logins != 2 {
		t.Errorf("logins got = %d, want = 2", f.logins)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.results = `{"movies":[{"id":5,"tmdb_id":14160,"path":"/uploads/up.srt","release_name":"Up.2009.1080p","uploader":"c"}]}`
	p, _ := newProvider(t, f)
	subs, err := p.Search(t.Context(), &provider.Video{Title: "Up", Year: 2009}, []provider.Language{{Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"path": "/api/v1/search/movie", "page": 1.0, "per_page": 25.0, "imdb_id": nil, "name": "Up"}
	if fmt.Sprint(f.searches[0]) != fmt.Sprint(want) {
		t.Errorf("search got = %v, want = %v", f.searches[0], want)
	}
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"title", "imdb_id"}) || subs[0].PageLink != f.srv.URL+"/legenda?movie_id=14160&legenda_id=5" {
		t.Errorf("subtitles got = %+v", subs)
	}
	f.results = `{"success":false,"error":"nothing"}`
	if subs, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, []provider.Language{{Tag: "pt-BR"}}); err != nil || len(subs) != 0 {
		t.Errorf("failed search got = %v, %v, want none", subs, err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p, _ := newProvider(t, f)
	got, err := p.Download(t.Context(), nil, "/uploads/a.zip")
	if err != nil {
		t.Fatal(err)
	}
	// The archive's first subtitle.
	if want := "1\n00:00:01,000 --> 00:00:02,000\nOlá\na.srt"; string(got.Data) != want || got.Format != "srt" || got.Language.Tag != "pt-BR" {
		t.Errorf("download got = %q %s %v, want = %q", got.Data, got.Format, got.Language, want)
	}
	got, err = p.Download(t.Context(), nil, f.srv.URL+"/uploads/b.srt")
	if err != nil || got.Format != "srt" {
		t.Errorf("download got = %q, %v", got.Data, err)
	}
}

func TestStatusCodes(t *testing.T) {
	for _, c := range []struct {
		name, request string
		code          int
		kind          provider.Kind
	}{
		{"search throttled", "search", 429, -1},
		{"search not found", "search", 404, -1},
		{"search failing", "search", 500, -1},
		{"download limit", "download", 429, provider.DownloadLimitExceeded},
		{"download failing", "download", 502, -1},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.results = tvResults
			p, _ := newProvider(t, f)
			f.status[c.request] = c.code
			var err error
			if c.request == "search" {
				_, err = p.Search(t.Context(), &provider.Video{Title: "Up"}, []provider.Language{{Tag: "pt-BR"}})
			} else {
				_, err = p.Download(t.Context(), nil, "/uploads/b.srt")
			}
			// Other failures are retried, as Bazarr's ProviderRetryMixin.
			if c.kind < 0 {
				if err != nil {
					t.Errorf("err got = %v, want = nil", err)
				}
				return
			}
			if got := provider.KindOf(err); got != c.kind {
				t.Errorf("kind got = %v, want = %v (%v)", got, c.kind, err)
			}
		})
	}
}

func TestAuthentication(t *testing.T) {
	f := newFake(t)
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"username":"me","password":"bad"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, nil); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("search with a bad password got = %v, want = ConfigurationError", err)
	}
	for _, s := range []string{`{}`, `{"username":"me"}`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = ConfigurationError", s, err)
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
