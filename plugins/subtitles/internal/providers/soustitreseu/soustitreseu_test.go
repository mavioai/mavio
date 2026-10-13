package soustitreseu

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range files {
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

func newProvider(t *testing.T) (*Provider, string) {
	t.Helper()
	season := zipOf(t, "Lost.1x02.HDTV.XviD-LOL.fr.srt", "Lost.1x02.HDTV.XviD-LOL.en.srt", "Lost.1x03.HDTV.XviD-LOL.fr.srt",
		".Lost.1x02.fr.srt", "Lost.1x02.notes.txt", "Lost.102.720p.tag.ass")
	files := map[string][]byte{
		"/series/lost.S01.zip":       season,
		"/series/lost.1x02.zip":      zipOf(t, "Lost.S01E02.WEB.VOSTFR.srt"),
		"/series/lost.1x05.zip":      zipOf(t, "Lost.S01E05.srt"),
		"/series/lost.S02.zip":       zipOf(t, "Lost.2x02.srt"),
		"/series/lost.broken.zip":    []byte("<html>no archive</html>"),
		"/films/inception.zip":       zipOf(t, "Inception.2010.720p.BluRay.x264-REFiNED.fr.srt", "Inception.2010.720p.BluRay.x264-REFiNED.en.srt"),
		"/films/inception-other.zip": zipOf(t, "Inception.DVDRip.fr.sub"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			t.Errorf("referer got = none, want = the site")
		}
		switch r.URL.Query().Get("q") {
		case "Lost":
			_, _ = w.Write([]byte(`<div class="serie"><h3><a href="lost.html">Lost</a></h3></div>` +
				`<div class="serie"><h3><a href="lostgirl.html">Lost Girl</a></h3></div>` +
				`<div class="serie"><h3><a href="other.html">Other</a></h3></div>`))
		case "Inception":
			_, _ = w.Write([]byte(`<div class="film"><h3><a href="inception.html">INCEPTION (2010)</a></h3></div>`))
		}
	})
	mux.HandleFunc("GET /lost.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="subList" href="lost.S01.zip"><span class="episodenum">S1</span></a>` +
			`<a class="subList" href="lost.1x02.zip"><span class="episodenum">1×02</span></a>` +
			`<a class="subList" href="lost.1x05.zip"><span class="episodenum">1×05</span></a>` +
			`<a class="subList" href="lost.S02.zip"><span class="episodenum">S2</span></a>` +
			`<a class="subList" href="lost.broken.zip"><span class="episodenum">1×02</span></a>`))
	})
	mux.HandleFunc("GET /lostgirl.html", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /inception.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="subList" href="inception.zip">a</a><a class="subList" href="inception-other.zip">b</a>`))
	})
	for name, data := range files {
		mux.HandleFunc("GET "+name, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) })
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider), srv.URL
}

func names(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.Language.Tag+":"+s.ReleaseNames[0])
	}
	return out
}

func TestSearchEpisode(t *testing.T) {
	p, base := newProvider(t)
	v := &provider.Video{Episode: true, Series: "Lost", Season: 1, Number: 2}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "fr"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fr:Lost.1x02.HDTV.XviD-LOL.fr.srt", "en:Lost.1x02.HDTV.XviD-LOL.en.srt", "fr:Lost.102.720p.tag.ass", "fr:Lost.S01E02.WEB.VOSTFR.srt"}
	if got := names(subs); !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
	for _, s := range subs {
		if !slices.Equal(s.Matches, []string{"series"}) || s.PageLink != base+"/lost.html" {
			t.Errorf("got = %v at %s, want = [series] at the series' page", s.Matches, s.PageLink)
		}
	}
	subs, err = p.Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 1 {
		t.Fatalf("english got = %v, %v, want = 1 subtitle", names(subs), err)
	}
	f, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "1\n00:00:01,000 --> 00:00:02,000\nLost.1x02.HDTV.XviD-LOL.en.srt\n" || f.Format != "srt" || f.Language.Tag != "en" {
		t.Errorf("got = %q, %q, %q, want = the English srt", f.Data, f.Format, f.Language.Tag)
	}
}

func TestSearchMovie(t *testing.T) {
	p, _ := newProvider(t)
	v := &provider.Video{Title: "Inception", Year: 2010}
	subs, err := p.Search(t.Context(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fr:Inception.2010.720p.BluRay.x264-REFiNED.fr.srt", "en:Inception.2010.720p.BluRay.x264-REFiNED.en.srt", "fr:Inception.DVDRip.fr.sub"}
	if got := names(subs); !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
	if !slices.Equal(subs[0].Matches, []string{"title"}) {
		t.Errorf("matches got = %v, want = [title]", subs[0].Matches)
	}
	f, err := p.Download(t.Context(), v, subs[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "srt" || f.Language.Tag != "fr" {
		t.Errorf("got = %q in %q, want = srt content in fr", f.Format, f.Language.Tag)
	}
	if subs, err := p.Search(t.Context(), &provider.Video{Title: "Nothing"}, nil); err != nil || subs != nil {
		t.Errorf("got = %v, %v, want = none", subs, err)
	}
}

func TestDownloadFailures(t *testing.T) {
	p, base := newProvider(t)
	for id, want := range map[string]provider.Kind{
		url.Values{"u": {base + "/series/lost.broken.zip"}, "f": {"a.srt"}}.Encode(): provider.BadSubtitle,
		url.Values{"u": {base + "/series/lost.S02.zip"}, "f": {"a.srt"}}.Encode():    provider.BadSubtitle,
		url.Values{"u": {base + "/series/gone.zip"}, "f": {"a.srt"}}.Encode():        provider.ProviderError,
		url.Values{"u": {"http://elsewhere/a.zip"}, "f": {"a.srt"}}.Encode():         provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != want {
			t.Errorf("%s: got = %v, want = %v", id, err, want)
		}
	}
}

func TestArchiveEpisode(t *testing.T) {
	for _, c := range []struct {
		href, num       string
		season, episode int
		ok              bool
	}{
		{"x.zip", "1×02", 1, 2, true},
		{"x.zip", "S3", 3, 0, true},
		{"Show.3x04.zip", "S3", 3, 4, true},
		{"x.zip", "?", 0, 0, false},
	} {
		s, e, ok := archiveEpisode(c.href, c.num)
		if s != c.season || e != c.episode || ok != c.ok {
			t.Errorf("archiveEpisode(%q, %q) got = %d, %d, %v, want = %d, %d, %v", c.href, c.num, s, e, ok, c.season, c.episode, c.ok)
		}
	}
}
