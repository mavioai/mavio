package subs4free

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

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nΓεια\r\n"

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

// moviePage is a movie's page of subtitles.
const moviePage = `<html><body><table><tr><td id="dates_header"><table><tr><td>
	<u>The Matrix</u><div><b>Year</b><i>:</i>(1999)</div></td></tr></table></td></tr></table>
	<div class="movie-details"><span>The.Matrix.1999.720p.BluRay.x264-REFiNED</span><a href="/el-subs/1">dl</a>
		<div class="movie-info"><p>By <a href="/u/1">greek</a></p></div><i class="sprite elgif"></i></div>
	<div class="movie-details"><span>The.Matrix.1999.DVDRip.XviD</span><a href="/en-subs/2">dl</a>
		<div class="movie-info"><p>By <a href="/u/2">english</a></p></div><i class="sprite engif"></i></div>
	</body></html>`

type fakeSite struct {
	srv *httptest.Server

	mu       sync.Mutex
	requests []string
	posted   url.Values
	archive  []byte
	status   int
}

func (f *fakeSite) log(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.URL.Path+" "+r.Header.Get("Referer"))
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{archive: zipOf(t, ".hidden.srt", "readme.nfo", "The.Matrix.srt")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search_report.php", func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		f.log(r)
		switch r.URL.Query().Get("search") {
		case "the matrix":
			_, _ = w.Write([]byte(`<select name="Mov_sel"><option>Pick</option>
				<option value="/movie.php?p=matrix-1999">The Matrix</option>
				<option value="/movie.php?p=matrix-2">The Matrix 2003</option></select>`))
		case "matrix 1999":
			_, _ = w.Write([]byte(moviePage))
		}
	})
	mux.HandleFunc("GET /matrix-1999", func(w http.ResponseWriter, r *http.Request) {
		f.log(r)
		_, _ = w.Write([]byte(moviePage))
	})
	mux.HandleFunc("GET /el-subs/1", func(w http.ResponseWriter, r *http.Request) {
		f.log(r)
		_, _ = w.Write([]byte(`<form><input name="id" value="77"><input type="image" width="120px" height="30px"></form>`))
	})
	mux.HandleFunc("GET /en-subs/2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<form>none</form>`))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { f.log(r) })
	mux.HandleFunc("POST /getSub.php", func(w http.ResponseWriter, r *http.Request) {
		f.log(r)
		_ = r.ParseForm()
		f.mu.Lock()
		f.posted = r.PostForm
		f.mu.Unlock()
		_, _ = w.Write(f.archive)
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

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	for _, c := range []struct {
		name  string
		video *provider.Video
		langs []provider.Language
		want  []string
	}{
		{
			// The page the site suggests for the title.
			"suggested", &provider.Video{Title: "The Matrix", Year: 1999},
			[]provider.Language{{Tag: "el"}, {Tag: "en"}},
			[]string{"el The.Matrix.1999.720p.BluRay.x264-REFiNED [title year]", "en The.Matrix.1999.DVDRip.XviD [title year]"},
		},
		{
			// Only the languages asked for.
			"greek", &provider.Video{Title: "The Matrix", Year: 1999},
			[]provider.Language{{Tag: "el"}},
			[]string{"el The.Matrix.1999.720p.BluRay.x264-REFiNED [title year]"},
		},
		{
			// No suggestion: the search for the title and year.
			"searched", &provider.Video{Title: "Matrix", AlternativeTitles: []string{"Matriks"}, Year: 1999},
			[]provider.Language{{Tag: "en"}},
			[]string{"en The.Matrix.1999.DVDRip.XviD [year]"},
		},
		{"episode", &provider.Video{Episode: true, Series: "The Matrix"}, []provider.Language{{Tag: "en"}}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), c.video, c.langs)
			var got []string
			for _, s := range subs {
				got = append(got, fmt.Sprintf("%s %s %v", s.Language.Tag, s.ReleaseNames[0], s.Matches))
			}
			if err != nil || !slices.Equal(got, c.want) {
				t.Errorf("got = %v, %v, want = %v", got, err, c.want)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	subs, err := p.Search(t.Context(), &provider.Video{Title: "The Matrix", Year: 1999}, []provider.Language{{Tag: "el"}, {Tag: "en"}})
	if err != nil || len(subs) != 2 {
		t.Fatalf("Search got = %v, %v", subs, err)
	}
	f.requests = nil
	file, err := p.Download(t.Context(), nil, subs[0].ID)
	if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" {
		t.Errorf("Download got = %q, %v, want = the archive's subtitle", file.Data, err)
	}
	link := f.srv.URL + "/el-subs/1"
	want := []string{
		"/el-subs/1 " + f.srv.URL + "/matrix-1999",
		"/favicon.ico " + link, "/includes/anti-block-layover.php " + link, "/includes/anti-block.php " + link,
		"/getSub.php " + link,
	}
	if !slices.Equal(f.requests, want) {
		t.Errorf("requests got = %v, want = %v", f.requests, want)
	}
	if f.posted.Get("id") != "77" || f.posted.Get("x") == "" || f.posted.Get("y") == "" {
		t.Errorf("form got = %v, want = the id and a click", f.posted)
	}
	// A bare subtitle is taken as it is.
	f.archive = []byte(srt)
	if file, err := p.Download(t.Context(), nil, subs[0].ID); err != nil || file.Format != "srt" {
		t.Errorf("bare Download got = %q, %v, want = the subtitle", file.Data, err)
	}
	for name, data := range map[string][]byte{"html": []byte("<html>blocked</html>"), "empty archive": zipOf(t, "readme.nfo"), "empty": nil} {
		f.archive = data
		if _, err := p.Download(t.Context(), nil, subs[0].ID); provider.KindOf(err) != provider.BadSubtitle {
			t.Errorf("Download of %s got = %v, want = %v", name, err, provider.BadSubtitle)
		}
	}
	// No form.
	if _, err := p.Download(t.Context(), nil, subs[1].ID); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download without a form got = %v, want = %v", err, provider.BadSubtitle)
	}
	if _, err := p.Download(t.Context(), nil, "%zz"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download of a malformed ID got = %v, want = %v", err, provider.BadSubtitle)
	}
}

func TestStatus(t *testing.T) {
	f := newFake(t)
	for _, c := range []struct {
		status int
		kind   provider.Kind
	}{{http.StatusTooManyRequests, provider.TooManyRequests}, {http.StatusServiceUnavailable, provider.ServiceUnavailable}, {http.StatusNotFound, provider.ProviderError}} {
		f.status = c.status
		if _, err := newProvider(t, f).Search(t.Context(), &provider.Video{Title: "The Matrix"}, []provider.Language{{Tag: "en"}}); provider.KindOf(err) != c.kind {
			t.Errorf("%d got = %v, want = %v", c.status, err, c.kind)
		}
	}
}
