package subtitriid

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nSveiki\r\n"

func zipped(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	}) {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(files[name]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newFake(t *testing.T) (*Provider, *[]string) {
	t.Helper()
	var searches []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			t.Errorf("referer is not set")
		}
		searches = append(searches, r.URL.Query().Get("q"))
		if r.URL.Query().Get("q") != "Up" {
			return
		}
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="eBlock"><div class="eTitle"><a href="/news/up/1">Augšā / Up</a></div></div>
			<div class="eBlock"><div class="eTitle"><a href="/news/up-2/2">Augšā 2</a></div></div>
		</body></html>`)
	})
	mux.HandleFunc("GET /news/up/1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body><h1 class="main-header"> Augšā / Up </h1>
			<span id="film-page-year">2009</span>
			<div id="actors-page"><a href="https://www.imdb.com/title/tt1049413/">IMDb</a></div>
			<a class="hvr" href="/load/up.zip">Lejupielādēt</a></body></html>`)
	})
	mux.HandleFunc("GET /news/up-2/2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body><h1 class="main-header">Up 2</h1>
			<span id="film-page-year">2012</span>
			<div id="actors-page"><a href="https://www.imdb.com/title/tt2/">IMDb</a></div>
			<a class="hvr" href="/load/up2.srt">Lejupielādēt</a></body></html>`)
	})
	mux.HandleFunc("GET /load/up.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipped(t, map[string]string{"Up.2009.lv.srt": srt, "readme.txt": "x"}))
	})
	mux.HandleFunc("GET /load/up2.srt", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(srt)) })
	mux.HandleFunc("GET /load/bad", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not a subtitle")) })
	mux.HandleFunc("GET /load/gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL + "/")
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider), &searches
}

func TestSearch(t *testing.T) {
	p, searches := newFake(t)
	v := &provider.Video{Title: "Up", AlternativeTitles: []string{"Augšā"}, Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "lv"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Up", "Augšā"}; !slices.Equal(*searches, want) {
		t.Errorf("searches = %v, want = %v", *searches, want)
	}
	if len(subs) != 2 {
		t.Fatalf("subtitles = %d, want = 2", len(subs))
	}
	for i, c := range []struct {
		id      string
		matches []string
	}{
		{"/load/up.zip", []string{"title", "year", "imdb_id"}},
		{"/load/up2.srt", nil},
	} {
		if subs[i].ID != c.id || !slices.Equal(subs[i].Matches, c.matches) || subs[i].Language.Tag != "lv" {
			t.Errorf("subtitle %d = %+v, want = %s %v", i, subs[i], c.id, c.matches)
		}
	}
	for _, c := range []struct {
		v     *provider.Video
		langs []provider.Language
	}{
		{&provider.Video{Episode: true, Series: "Up"}, []provider.Language{{Tag: "lv"}}},
		{v, []provider.Language{{Tag: "lv", Forced: true}}},
	} {
		if subs, err := p.Search(t.Context(), c.v, c.langs); err != nil || len(subs) != 0 {
			t.Errorf("search(%+v, %v) = %v, %v, want = none", c.v, c.langs, subs, err)
		}
	}
}

func TestDownload(t *testing.T) {
	p, _ := newFake(t)
	for _, c := range []struct {
		id   string
		kind provider.Kind
	}{
		{"/load/up.zip", -1},
		{"/load/up2.srt", -1},
		{"/load/bad", provider.ProviderError},
		{"/load/gone", provider.ProviderError},
	} {
		f, err := p.Download(t.Context(), &provider.Video{Title: "Up"}, c.id)
		if c.kind >= 0 {
			if got := provider.KindOf(err); err == nil || got != c.kind {
				t.Errorf("download %s: err = %v, want = %v", c.id, err, c.kind)
			}
			continue
		}
		if err != nil {
			t.Fatalf("download %s: %v", c.id, err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\nSveiki\n"; string(f.Data) != want && string(f.Data) != srt || f.Format != "srt" || f.Language.Tag != "lv" {
			t.Errorf("download %s = %q %s %v", c.id, f.Data, f.Format, f.Language)
		}
	}
}
