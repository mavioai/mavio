package subtitulamostv

import (
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

// seasonPage is the page of a show's season with its episodes.
func seasonPage(season, episode int) string {
	sel := func(cond bool) string {
		if cond {
			return "selected"
		}
		return ""
	}
	return fmt.Sprintf(`<html><body>
		<div id="season-choices"><a class="choice %s" href="/shows/7/season/1">1</a><a class="choice %s" href="/shows/7/season/2">2</a></div>
		<div id="episode-choices"><a class="choice %s" href="/episodes/10/show-1x01">1</a><a class="choice %s" href="/episodes/11/show-1x02">2</a></div>
		</body></html>`, sel(season == 1), sel(season == 2), sel(episode == 1), sel(episode == 2))
}

const episodePage = `<html><body>
	<div id="season-choices"><a class="choice selected" href="/shows/7/season/1">1</a></div>
	<div id="episode-choices"><a class="choice" href="/episodes/10/show-1x01">1</a><a class="choice selected" href="/episodes/11/show-1x02">2</a></div>
	<div class="language-container"><div class="language-name">English</div>
		<div class="version-container"><p>Versión</p><p>WEB-NTb</p><a href="/subtitles/1/download">Descargar</a></div>
		<div class="version-container"><p>Versión</p><p>HDTV-KILLERS</p><a href="/subtitles/2/download">Descargar</a></div>
	</div>
	<div class="language-container"><div class="language-name">Español (España)</div>
		<div class="version-container"><p>Versión</p><p>WEB-NTb</p><a href="/subtitles/3/download">Descargar</a></div>
		<div class="version-container"><p>Versión</p><p>WEB-NTb</p><span>50% traducido</span></div>
	</div>
	<div class="language-container"><div class="language-name">Català</div>
		<div class="version-container"><p>Versió</p><p>WEB-NTb</p><a href="/subtitles/4/download">Descarrega</a></div>
	</div>
	</body></html>`

func newFake(t *testing.T) (*Provider, *[]string) {
	t.Helper()
	var paths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != userAgent {
			t.Errorf("user agent = %q, want = %q", ua, userAgent)
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/search/query":
			if q := r.URL.Query().Get("q"); q != "The Show" {
				t.Errorf("q = %q, want = The Show", q)
			}
			_, _ = fmt.Fprint(w, `[{"show_id":6,"show_name":"The Show (US)"},{"show_id":7,"show_name":"the show"}]`)
		case "/shows/7":
			_, _ = fmt.Fprint(w, seasonPage(2, 0))
		case "/shows/7/season/1":
			_, _ = fmt.Fprint(w, seasonPage(1, 1))
		case "/episodes/11/show-1x02":
			_, _ = fmt.Fprint(w, episodePage)
		case "/subtitles/1/download":
			_, _ = fmt.Fprint(w, "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n")
		case "/subtitles/9/download":
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL + "/")
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider), &paths
}

func TestSearch(t *testing.T) {
	p, paths := newFake(t)
	v := &provider.Video{Episode: true, Series: "The Show", Season: 1, Number: 2, ReleaseGroup: "ntb"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}, {Tag: "es"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/search/query", "/shows/7", "/shows/7/season/1", "/episodes/11/show-1x02"}; !slices.Equal(*paths, want) {
		t.Errorf("paths = %v, want = %v", *paths, want)
	}
	for i, c := range []struct {
		id, tag, release string
		group            bool
	}{
		{"/subtitles/1/download", "en", "WEB-NTb", true},
		{"/subtitles/2/download", "en", "HDTV-KILLERS", false},
		{"/subtitles/3/download", "es", "WEB-NTb", true},
	} {
		if i >= len(subs) {
			t.Fatalf("subtitles = %d, want = 3", len(subs))
		}
		s := subs[i]
		if s.ID != c.id || s.Language.Tag != c.tag || !slices.Equal(s.ReleaseNames, []string{c.release}) || slices.Contains(s.Matches, "release_group") != c.group {
			t.Errorf("subtitle %d = %+v, want = %s %s %s", i, s, c.id, c.tag, c.release)
		}
		if !slices.Contains(s.Matches, "episode") || s.PageLink == "" {
			t.Errorf("subtitle %d = %+v, want episode matched", i, s)
		}
	}
	if len(subs) != 3 {
		t.Errorf("subtitles = %d, want = 3", len(subs))
	}

	subs, err = p.Search(t.Context(), v, []provider.Language{{Tag: "es"}})
	if err != nil || len(subs) != 1 || subs[0].ID != "/subtitles/3/download" {
		t.Errorf("search in Spanish = %+v, %v, want = /subtitles/3/download", subs, err)
	}
	for _, v := range []*provider.Video{
		{Episode: true, Series: "The Show", Season: 3, Number: 2},
		{Episode: true, Series: "The Show", Season: 1, Number: 5},
		{Title: "The Show"},
	} {
		if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
			t.Errorf("search(%+v) = %v, %v, want = none", v, subs, err)
		}
	}
}

func TestDownload(t *testing.T) {
	p, _ := newFake(t)
	f, err := p.Download(t.Context(), nil, "/subtitles/1/download")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"; string(f.Data) != want || f.Format != "srt" {
		t.Errorf("download = %q %s, want = %q srt", f.Data, f.Format, want)
	}
	for id, kind := range map[string]provider.Kind{"/subtitles/9/download": provider.ProviderError, "/subtitles/8/download": provider.ProviderError} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != kind {
			t.Errorf("download %s: err = %v, want = %v", id, err, kind)
		}
	}
}
