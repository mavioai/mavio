package subssabbz

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

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nЗдравей\r\n"

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(srt))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// resultRow is a row of the site's search results.
func resultRow(base, id, title, year, cds, imdb string) string {
	return fmt.Sprintf(`<tr class="subs-row"><td>1</td><td class="c2field"><a href="%s/files/%s"
		onmouseover="ddrivetip('<div><img src=x /></div>Notes of %s','#555')">%s</a> (%s)</td>
		<td>x</td><td>x</td><td>x</td><td>x</td><td>%s</td><td>23.976</td><td>uploader</td>
		<td><a href="https://www.imdb.com/title/%s/">imdb</a></td></tr>`, base, id, id, title, year, cds, imdb)
}

type fakeSite struct {
	srv *httptest.Server

	mu        sync.Mutex
	searches  []url.Values
	downloads int
	forbidden int
	redirect  bool
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	rar := ported.Response(t, "archive_2.rar")
	files := map[string][]byte{
		"1": zipOf(t, "The.Matrix.1999.720p.BluRay.x264-REFiNED.srt", "readme.txt"),
		"2": zipOf(t, "The.Matrix.1999.CD1.srt", "The.Matrix.1999.CD2.srt"),
		"3": []byte("<html>no archive</html>"),
		"4": zipOf(t, "Show.S01E02.HDTV.srt", "Show.S01E02.WEB.sub"),
		"5": rar,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /index.php", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.searches = append(f.searches, r.URL.Query())
		forbidden, redirect := f.forbidden > 0, f.redirect
		if forbidden {
			f.forbidden--
		}
		f.mu.Unlock()
		switch {
		case forbidden:
			w.WriteHeader(http.StatusForbidden)
			return
		case redirect:
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		base := "http://" + r.Host
		switch r.URL.Query().Get("movie") {
		case "the matrix":
			_, _ = fmt.Fprintf(w, "<html><body><table>%s%s%s%s</table></body></html>",
				resultRow(base, "1", "The Matrix", "1999", "1", "tt0133093"),
				resultRow(base, "2", "The Matrix", "1999", "2", "tt0133093"),
				resultRow(base, "3", "The Matrix", "1999", "1", "tt0133093"),
				resultRow(base, "9", "The Matrix Reloaded", "2003", "1", "tt0234215"))
		case "show":
			_, _ = fmt.Fprintf(w, "<table>%s%s</table>",
				resultRow(base, "4", "Show - 01x02", "2010", "1", "tt0000100"),
				resultRow(base, "5", "Show - 01x04", "2010", "1", "tt0000100"))
		}
	})
	mux.HandleFunc("GET /elsewhere", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the search was redirected")
	})
	mux.HandleFunc("GET /files/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.downloads++
		f.mu.Unlock()
		if r.Header.Get("Referer") == "" {
			t.Errorf("download without a referer")
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
		out = append(out, fmt.Sprintf("%s %s %v", q.Get("file"), s.Language.Tag, s.Matches))
	}
	return out
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := &provider.Video{
		Title: "The Matrix", Year: 1999, ExternalIDs: map[string]string{"imdb": "tt0133093"},
		FileName: "/movies/The Matrix (1999)/The.Matrix.1999.720p.BluRay.x264-REFiNED.mkv",
	}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}, {Tag: "en", HearingImpaired: true}})
	want := []string{
		"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt bg [hash year imdb_id title]",
		// The files of a movie on two discs match nothing.
		"The.Matrix.1999.CD1.srt bg []",
		"The.Matrix.1999.CD2.srt bg []",
		"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt en [hash year imdb_id title]",
		"The.Matrix.1999.CD1.srt en []",
		"The.Matrix.1999.CD2.srt en []",
	}
	if got := describe(subs); err != nil || !slices.Equal(got, want) {
		t.Fatalf("got = %v, %v, want = %v", got, err, want)
	}
	if s := subs[0]; s.Comment != "Notes of 1" || s.Uploader != "uploader" || !slices.Equal(s.ReleaseNames, []string{"The.Matrix.1999.720p.BluRay.x264-REFiNED.srt", "The Matrix"}) || s.Format != "srt" {
		t.Errorf("got = %+v, want = the row's notes, uploader and release names", s)
	}
	if !subs[3].Language.HearingImpaired {
		t.Errorf("language got = %+v, want = the one asked for", subs[3].Language)
	}
	if len(subs[1].ReleaseNames) != 0 {
		t.Errorf("release names of a multi-disc movie got = %v, want = none", subs[1].ReleaseNames)
	}
	if q := f.searches[0]; q.Get("movie") != "the matrix" || q.Get("yr") != "1999" || q.Get("select-language") != "2" || q.Get("act") != "search" {
		t.Errorf("search got = %v", q)
	}
	if q := f.searches[1]; q.Get("select-language") != "1" {
		t.Errorf("English search got = %v", q)
	}
	// The archives were cached: the second language downloads only what
	// was no archive again.
	if f.downloads != 4 {
		t.Errorf("downloads got = %d, want = 4", f.downloads)
	}
	file, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" || f.downloads != 4 {
		t.Errorf("Download got = %q, %v, %d downloads, want = the cached subtitle", file.Data, err, f.downloads)
	}
	// Another provider downloads the archive again.
	if file, err := newProvider(t, f).Download(t.Context(), v, subs[1].ID); err != nil || len(file.Data) == 0 {
		t.Errorf("Download got = %q, %v, want = the subtitle", file.Data, err)
	}
	if _, err := p.Download(t.Context(), v, "link=x"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download of a malformed ID got = %v, want = %v", err, provider.BadSubtitle)
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := &provider.Video{
		Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0000100"},
		FileName: "Show.S01E02.HDTV.mkv",
	}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}})
	a, aerr := archive.Open(ported.Response(t, "archive_2.rar"))
	if aerr != nil {
		t.Fatal(aerr)
	}
	want := []string{"Show.S01E02.HDTV.srt bg [hash series_imdb_id series]", "Show.S01E02.WEB.sub bg [series_imdb_id series]"}
	for _, name := range slices.Sorted(slices.Values(a.Names())) {
		if l := strings.ToLower(name); strings.HasSuffix(l, ".srt") || strings.HasSuffix(l, ".sub") {
			want = append(want, name+" bg [series_imdb_id series]")
		}
	}
	if got := describe(subs); err != nil || !slices.Equal(got, want) {
		t.Errorf("got = %v, %v, want = %v", got, err, want)
	}
	if q := f.searches[0]; q.Get("movie") != "show" || q.Get("yr") != "" {
		t.Errorf("search got = %v", q)
	}
}

func TestSearchTitle(t *testing.T) {
	for _, c := range []struct {
		video *provider.Video
		want  string
	}{
		{&provider.Video{Title: "Shōgun"}, "Shogun"},
		{&provider.Video{Title: "Back to the Future Part II"}, "Back to the Future II"},
		{&provider.Video{Episode: true, Series: "Marvel's Daredevil"}, "Daredevil"},
		{&provider.Video{Episode: true, Series: "Grey's Anatomy: Ok"}, "grey's anatomy ok"},
		{&provider.Video{Title: "Черната чанта", FileName: "/media/Cherna Chanta (2020)/x.mkv"}, "Cherna Chanta"},
		{&provider.Video{Title: "Черната чанта", FileName: "/media/movies/Cherna.Chanta.2020.mkv"}, "Cherna Chanta"},
		{&provider.Video{Title: "Черната чанта", AlternativeTitles: []string{"Чанта", "The Black Bag"}}, "The Black Bag"},
		{&provider.Video{Title: "Черната чанта"}, ""},
	} {
		if got := searchTitle(c.video); !strings.EqualFold(got, c.want) {
			t.Errorf("searchTitle(%+v) got = %q, want = %q", c.video, got, c.want)
		}
	}
}

func TestSearchFailures(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := &provider.Video{Title: "The Matrix", Year: 1999}
	// A forbidden search is tried again.
	f.forbidden = 2
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}}); err != nil || len(subs) == 0 || len(f.searches) != 3 {
		t.Errorf("after two 403 got = %d subtitles, %v, %d searches, want = some after 3 searches", len(subs), err, len(f.searches))
	}
	// Then skipped.
	f.forbidden = 3
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}}); err != nil || len(subs) != 0 {
		t.Errorf("after three 403 got = %v, %v, want = none", subs, err)
	}
	// A redirect is no result.
	f.redirect = true
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}}); err != nil || len(subs) != 0 {
		t.Errorf("redirect got = %v, %v, want = none", subs, err)
	}
	f.redirect = false
	// No title is no search.
	n := len(f.searches)
	if subs, err := p.Search(t.Context(), &provider.Video{Title: "Черна"}, []provider.Language{{Tag: "bg"}}); err != nil || len(subs) != 0 || len(f.searches) != n {
		t.Errorf("empty title got = %v, %v, want = no search", subs, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	p.SetBaseURL(u)
	if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}}); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("503 got = %v, want = %v", err, provider.ServiceUnavailable)
	}
}
