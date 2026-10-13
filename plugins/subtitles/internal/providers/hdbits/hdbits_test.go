package hdbits

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
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

// fakeAPI is HDBits' API: torrents and their subtitles.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	lookups  []string
	torrents string
	status   int
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t}
	decode := func(r *http.Request) map[string]any {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("%s: %v", r.URL.Path, err)
		}
		if body["username"] != "me" || body["passkey"] != "pk" {
			t.Errorf("%s: credentials %v", r.URL.Path, body)
		}
		return body
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/torrents", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		delete(body, "username")
		delete(body, "passkey")
		data, _ := json.Marshal(body)
		f.mu.Lock()
		f.lookups = append(f.lookups, string(data))
		f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
		}
		_, _ = w.Write([]byte(f.torrents))
	})
	mux.HandleFunc("POST /api/subtitles", func(w http.ResponseWriter, r *http.Request) {
		switch id := decode(r)["torrent_id"]; id {
		case "1", 1.0:
			_, _ = w.Write([]byte(`{"status":0,"data":[
				{"id":11,"title":"Show S01E02 720p","filename":"show.s01e02.srt","language":"uk"},
				{"id":12,"title":"Show S01E03 720p","filename":"show.s01e03.srt","language":"uk"},
				{"id":13,"title":"Show S01 Pack","filename":"show.s01.zip","language":"br"},
				{"id":14,"title":"Show S01E02 Commentary","filename":"show.s01e02.srt","language":"en"},
				{"id":15,"title":"Show S01E02","filename":"show.s01e02.sup","language":"en"},
				{"id":16,"title":"Show S01E02","filename":"show.s01e02.ass","language":"fr"},
				{"id":17,"title":"Show S01E02","filename":"show.s01e02.srt","language":"xx"},
				{"id":18,"title":"Show S01E02 Forced","filename":"show.forced.srt","language":"en"}]}`))
		case "2", 2.0:
			_, _ = w.Write([]byte(`<html>`))
		default:
			t.Errorf("torrent_id = %v", id)
		}
	})
	mux.HandleFunc("GET /getdox.php", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("passkey") != "pk" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Query().Get("id") {
		case "11":
			_, _ = w.Write([]byte(srt))
		case "13":
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for _, n := range []string{"Show.S01E01.srt", "Show.S01E02.srt"} {
				fw, _ := zw.Create(n)
				_, _ = fw.Write([]byte(srt + n))
			}
			_ = zw.Close()
			_, _ = w.Write(buf.Bytes())
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeAPI) (*Provider, *[]time.Duration) {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"username":"me","passkey":"pk"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	hp := p.(*Provider)
	hp.SetBaseURL(u)
	var slept []time.Duration
	hp.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return hp, &slept
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.torrents = `{"status":0,"data":[{"id":1},{"id":2}]}`
	p, slept := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"tvdb": "81189"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}, {Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`{"tvdb":{"id":81189,"season":1}}`}; !slices.Equal(f.lookups, want) {
		t.Errorf("lookups got = %v, want = %v", f.lookups, want)
	}
	// Other episodes, extras, other formats and languages are left out.
	if got := ids(subs); !slices.Equal(got, []string{"11", "13"}) {
		t.Fatalf("subtitles got = %v, want = [11 13]", got)
	}
	if subs[0].Language.Tag != "en" || subs[1].Language.Tag != "pt-BR" || subs[0].ReleaseNames[0] != "Show S01E02 720p" {
		t.Errorf("subtitles got = %+v", subs)
	}
	if want := []string{"tvdb_id", "imdb_id", "series", "title", "episode", "season"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("matches got = %v, want = %v", subs[0].Matches, want)
	}
	if want := []time.Duration{500 * time.Millisecond, 500 * time.Millisecond}; !slices.Equal(*slept, want) {
		t.Errorf("waits got = %v, want = %v", *slept, want)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.torrents = `{"status":5,"message":"no results"}`
	p, _ := newProvider(t, f)
	v := &provider.Video{Title: "Up", ExternalIDs: map[string]string{"imdb": "tt1049413"}}
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}}); err != nil || subs != nil {
		t.Errorf("search without data got = %v, %v, want none", subs, err)
	}
	if want := []string{`{"imdb":{"id":"1049413"}}`}; !slices.Equal(f.lookups, want) {
		t.Errorf("lookups got = %v, want = %v", f.lookups, want)
	}
	f.torrents = `{"data":[{"id":1}]}`
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "fr"}})
	if err != nil {
		t.Fatal(err)
	}
	// Movies take every subtitle of the torrent.
	if got := ids(subs); !slices.Equal(got, []string{"16"}) || !slices.Equal(subs[0].Matches, []string{"imdb_id", "title", "year"}) {
		t.Errorf("subtitles got = %+v, want = [16]", subs)
	}
	f.status = http.StatusServiceUnavailable
	if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "fr"}}); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("search failing got = %v, want = ProviderError", err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p, _ := newProvider(t, f)
	ep := &provider.Video{Episode: true, Season: 1, Number: 2}
	const sub = "1\n00:00:01,000 --> 00:00:02,000\nHello\n"
	for id, want := range map[string]string{"11": sub, "13": sub + "Show.S01E02.srt"} {
		got, err := p.Download(t.Context(), ep, id)
		if err != nil {
			t.Fatal(err)
		}
		if string(got.Data) != want || got.Format != "srt" {
			t.Errorf("download %s got = %q, want = %q", id, got.Data, want)
		}
	}
	if _, err := p.Download(t.Context(), ep, "99"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("download of a missing subtitle got = %v, want = ProviderError", err)
	}
}

func TestLanguages(t *testing.T) {
	langs := Definition().Languages
	for _, tag := range []string{"pt-BR", "en", "pt", "sv", "zu"} {
		if !slices.Contains(langs, tag) {
			t.Errorf("languages lack %s", tag)
		}
	}
	for code, want := range map[string]string{"uk": "en", "br": "pt-BR", "gr": "el", "se": "sv", "fr": "fr", "xx": ""} {
		if got := fromSite(code); got != want {
			t.Errorf("fromSite(%s) got = %q, want = %q", code, got, want)
		}
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
		out = append(out, fmt.Sprint(s.ID))
	}
	return out
}
