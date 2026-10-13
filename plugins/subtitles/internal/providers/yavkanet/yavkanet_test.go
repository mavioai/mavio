package yavkanet

import (
	"archive/zip"
	"bytes"
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

func srt(text string) string { return "1\r\n00:00:01,000 --> 00:00:02,000\r\n" + text + "\r\n" }

func zipped(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		f, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(files[i+1]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const searchPage = `<html><body><table>
	<tr><th>Субтитри</th></tr>
	<tr><td><a class="balon" href="/subs/1/BG" content="&lt;p&gt;&lt;img src='/img/x.png'&gt;Up.2009.720p.BluRay.x264-SPARKS&lt;/p&gt;">Up</a> <span>(2009)</span>
		<span title="Кадри в секунда">23.976</span> <a class="click" href="/user/ivan">ivan</a></td></tr>
	<tr><td><a class="selector" href="/subs/2/BG" content="notes">Up</a> <span>(2010)</span><a class="click">maria</a></td></tr>
	<tr><td><a class="balon" href="/subs/3/BG" content="">Up</a></td></tr>
	<tr><td><a class="balon" href="/subs/4/BG" content="">Up</a></td></tr>
	</table></body></html>`

type fake struct {
	mu    sync.Mutex
	posts map[string]int
}

func newFake(t *testing.T) (*Provider, *fake) {
	t.Helper()
	f := &fake{posts: map[string]int{}}
	form := `<html><body><form method="POST" action=""><input type="hidden" name="id" value="%s"><input type="submit" value="Изтегли"></form></body></html>`
	mux := http.NewServeMux()
	mux.HandleFunc("GET /imdb/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			t.Errorf("referer is not set")
		}
		switch r.PathValue("id") {
		case "tt1049413":
			_, _ = fmt.Fprint(w, searchPage)
		case "tt0000500":
			http.Error(w, "down", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /subs/{n}/BG", func(w http.ResponseWriter, r *http.Request) {
		switch n := r.PathValue("n"); n {
		case "1", "2", "4":
			_, _ = fmt.Fprintf(w, form, n)
		case "3":
			_, _ = fmt.Fprint(w, `<html><body>no form</body></html>`)
		}
	})
	mux.HandleFunc("POST /subs/{n}/BG/", func(w http.ResponseWriter, r *http.Request) {
		n := r.PathValue("n")
		if r.FormValue("id") != n || r.Header.Get("Referer") == "" {
			t.Errorf("post %s: form %v, headers %v", n, r.Form, r.Header)
		}
		f.mu.Lock()
		f.posts[n]++
		f.mu.Unlock()
		switch n {
		case "1":
			_, _ = w.Write(zipped(t, "Up.2009.720p.BluRay.x264-SPARKS.srt", srt("Горе"), "readme.txt", "x"))
		case "2":
			_, _ = w.Write(zipped(t, "Up.CD1.srt", srt("one"), "Up.CD2.sub", srt("two")))
		case "4":
			http.Redirect(w, r, "/login", http.StatusFound)
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
	p.(*Provider).pause = 0
	return p.(*Provider), f
}

func TestSearchAndDownload(t *testing.T) {
	p, f := newFake(t)
	v := &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}, FileName: "/movies/Up.2009.720p.BluRay.x264-SPARKS.mkv"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 {
		t.Fatalf("subtitles = %+v, want = 3", subs)
	}
	s := subs[0]
	if !slices.Equal(s.ReleaseNames, []string{"Up.2009.720p.BluRay.x264-SPARKS.srt [23.976]", "Up"}) || !slices.Equal(s.Matches, []string{"hash", "year"}) ||
		s.Uploader != "ivan" || s.Comment != "Up.2009.720p.BluRay.x264-SPARKS" || s.Language.Tag != "bg" {
		t.Errorf("subtitle = %+v", s)
	}
	if s := subs[1]; !slices.Equal(s.ReleaseNames, []string{"Up.CD1.srt", "Up"}) || len(s.Matches) != 0 || s.Uploader != "maria" {
		t.Errorf("subtitle = %+v", s)
	}

	for i, want := range []string{"Горе", "two"} {
		id := subs[i*2].ID
		file, err := p.Download(t.Context(), v, id)
		if err != nil {
			t.Fatal(err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\n" + want + "\n"; string(file.Data) != want || file.Language.Tag != "bg" {
			t.Errorf("download %s = %q %v, want = %q", id, file.Data, file.Language, want)
		}
	}
	// The archives are posted for once, from the search to the download.
	if f.posts["1"] != 1 || f.posts["2"] != 1 {
		t.Errorf("posts = %v, want = one of each", f.posts)
	}
	p.archives = map[string]cached{}
	if file, err := p.Download(t.Context(), v, subs[1].ID); err != nil || file.Format != "srt" {
		t.Errorf("download without the archive kept = %v, %v", file, err)
	}
	for id, kind := range map[string]provider.Kind{
		"link=%2Fsubs%2F3%2FBG&file=a.srt": provider.ProviderError,
		"link=%2Fsubs%2F1%2FBG&file=b.srt": provider.BadSubtitle,
		"%":                                provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), v, id); provider.KindOf(err) != kind {
			t.Errorf("download %s: err = %v, want = %v", id, err, kind)
		}
	}
}

func TestSearchFailures(t *testing.T) {
	p, _ := newFake(t)
	for imdb, kind := range map[string]provider.Kind{"tt0000500": provider.ServiceUnavailable, "tt0000404": provider.ProviderError} {
		v := &provider.Video{Title: "Up", ExternalIDs: map[string]string{"imdb": imdb}}
		if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "bg"}}); provider.KindOf(err) != kind {
			t.Errorf("search %s: err = %v, want = %v", imdb, err, kind)
		}
	}
	if subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "Up"}, []provider.Language{{Tag: "bg"}}); err != nil || len(subs) != 0 {
		t.Errorf("search without an IMDb ID = %v, %v, want = none", subs, err)
	}
}

func TestMatches(t *testing.T) {
	v := &provider.Video{Year: 2009, FileName: `C:\movies\Up.2009.1080p.BluRay[rarbg].mkv`}
	for _, c := range []struct {
		name, notes string
		single      bool
		want        []string
	}{
		{"Up.2009.1080p.srt", "", false, []string{"year"}},
		{"Up.2009.1080p.srt", "for up.2009.1080p.bluray", true, []string{"hash", "year"}},
		{"Up.2009.1080p.srt", "for up.2009.1080p.bluray", false, []string{"year"}},
		{"dir/UP.2009.1080P.BLURAY [x].srt", "", false, []string{"hash", "year"}},
	} {
		if got := matches(v, c.name, c.notes, 2009, c.single); !slices.Equal(got, c.want) {
			t.Errorf("matches(%s, %s) = %v, want = %v", c.name, c.notes, got, c.want)
		}
	}
}
