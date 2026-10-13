package subsunacs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nЗдравей\r\n"

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range files {
			if !yield(k) {
				return
			}
		}
	}) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// resultRow is a row of the site's search results.
func resultRow(id, title, year, cds string) string {
	return fmt.Sprintf(`<tr onmouseover="x"><td class="tdMovie"><a class="tooltip" href="/subtitles/%s/"
		title="&lt;img src=&quot;/x.jpg&quot;&gt;Notes of %s">%s</a> <span class="smGray">&nbsp;(%s)</span></td>
		<td>%s</td><td>23.976</td><td><img title="4.5"></td><td>x</td><td>uploader</td></tr>`, id, id, title, year, cds)
}

type fakeSite struct {
	srv *httptest.Server

	mu        sync.Mutex
	searches  []url.Values
	downloads int
	status    int
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	files := map[string][]byte{
		"1": zipOf(t, map[string]string{
			"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt": srt,
			"Прочети.txt":                                  "read me", "info.txt": "not a subtitle",
		}),
		"2": zipOf(t, map[string]string{"The.Matrix.1999.CD1.srt": srt, "The.Matrix.1999.CD2.srt": srt}),
		"3": []byte("<html>no archive</html>"),
		"4": zipOf(t, map[string]string{"Show.S01E02.HDTV.srt": srt, "Show.S01E02.WEB.txt": srt}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /search.php", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.searches = append(f.searches, r.URL.Query())
		status := f.status
		f.mu.Unlock()
		if r.Header.Get("Referer") != "http://"+r.Host+"/index.php" {
			t.Errorf("search Referer got = %q, want = the index", r.Header.Get("Referer"))
		}
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		var rows string
		// Bazarr's sanitize lowercases the titles.
		switch r.URL.Query().Get("m") {
		case "the matrix":
			rows = resultRow("1", "The Matrix", "1999", "1") + resultRow("2", "The Matrix", "1999", "2") + resultRow("3", "The Matrix", "1999", "1")
		case "show 01 02":
			rows = resultRow("4", "Show", "2010", "1")
		}
		_, _ = fmt.Fprintf(w, "<table>%s<tr><td>header</td></tr></table>", rows)
	})
	mux.HandleFunc("GET /subtitles/{id}/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.downloads++
		f.mu.Unlock()
		if r.Header.Get("Referer") != "http://"+r.Host+"/search.php" {
			t.Errorf("download Referer got = %q, want = the search", r.Header.Get("Referer"))
		}
		_, _ = w.Write(files[r.PathValue("id")])
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func describe(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		q, _ := url.ParseQuery(s.ID)
		out = append(out, fmt.Sprintf("%s %s %v %v %s", q.Get("file"), s.Language.Tag, s.ReleaseNames, s.Matches, s.Format))
	}
	return out
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := &provider.Video{Title: "The Matrix", Year: 1999, FileName: "/movies/The.Matrix.1999.720p.BluRay.x264-REFiNED.mkv"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}, {Tag: "en"}})
	want := []string{
		"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt bg [The.Matrix.1999.720p.BluRay.x264-REFiNED.srt The Matrix] [hash year] srt",
		"The.Matrix.1999.CD1.srt bg [] [] srt",
		"The.Matrix.1999.CD2.srt bg [] [] srt",
		"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt en [The.Matrix.1999.720p.BluRay.x264-REFiNED.srt The Matrix] [hash year] srt",
		"The.Matrix.1999.CD1.srt en [] [] srt",
		"The.Matrix.1999.CD2.srt en [] [] srt",
	}
	if got := describe(subs); err != nil || !slices.Equal(got, want) {
		t.Errorf("Search got = %v, %v, want = %v", got, err, want)
	}
	if len(subs) > 0 && (subs[0].Uploader != "uploader" || !strings.HasPrefix(subs[0].Comment, "Notes of 1") || subs[0].PageLink != f.srv.URL+"/subtitles/1/") {
		t.Errorf("subtitle got = %+v, want = the uploader, notes and page", subs[0])
	}
	if got := []string{f.searches[0].Get("l"), f.searches[0].Get("y"), f.searches[0].Get("imdbcheck"), f.searches[1].Get("l")}; !slices.Equal(got, []string{"0", "1999", "1", "1"}) {
		t.Errorf("search parameters got = %v, want = [0 1999 1 1]", got)
	}
	// The archives are cached; the page that is no archive is not.
	if f.downloads != 4 {
		t.Errorf("downloads got = %d, want = 4", f.downloads)
	}
	file, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" {
		t.Errorf("Download got = %+v, %v, want = the subtitle", file, err)
	}
	if _, err := p.Download(t.Context(), v, url.Values{"link": {f.srv.URL + "/subtitles/1/"}, "file": {"x.srt"}}.Encode()); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download of a missing file got = %v, want = %v", err, provider.BadSubtitle)
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, Year: 2010, FileName: "Show.S01E02.HDTV.mkv"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}})
	want := []string{
		"Show.S01E02.HDTV.srt bg [Show.S01E02.HDTV.srt Show] [hash year] srt",
		"Show.S01E02.WEB.txt bg [Show.S01E02.WEB.txt Show] [year] srt",
	}
	if got := describe(subs); err != nil || !slices.Equal(got, want) {
		t.Errorf("Search got = %v, %v, want = %v", got, err, want)
	}
	if got := f.searches[0].Get("y"); got != "" {
		t.Errorf("episode year got = %q, want = none", got)
	}
}

func TestSearchFailures(t *testing.T) {
	for _, c := range []struct {
		status  int
		wantErr bool
		kind    provider.Kind
	}{
		// A redirect is no subtitles.
		{http.StatusFound, false, provider.Other},
		{http.StatusForbidden, true, provider.ForbiddenError},
		{http.StatusTooManyRequests, true, provider.TooManyRequests},
		{http.StatusServiceUnavailable, true, provider.ServiceUnavailable},
	} {
		f := newFake(t)
		f.status = c.status
		p := newProvider(t, f)
		subs, err := p.Search(t.Context(), &provider.Video{Title: "The Matrix"}, []provider.Language{{Tag: "bg"}})
		if (err != nil) != c.wantErr || provider.KindOf(err) != c.kind || len(subs) != 0 {
			t.Errorf("Search of %d got = %v, %v, want = %v", c.status, subs, err, c.kind)
		}
	}
}
