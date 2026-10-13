package tvsubtitles

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

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

func zipped(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(srt))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fake struct {
	mu    sync.Mutex
	paths []string
}

func (f *fake) hits(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.paths {
		if p == path {
			n++
		}
	}
	return n
}

func newFake(t *testing.T) (*Provider, *fake) {
	t.Helper()
	f := &fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		switch r.URL.Path {
		case "/search1.php":
			if r.Method != http.MethodPost || r.FormValue("qs") != "the office" && r.FormValue("qs") != "The Office" {
				t.Errorf("search %s %q", r.Method, r.FormValue("qs"))
			}
			_, _ = fmt.Fprint(w, `<html><body><div class="left"><ul>
				<li><div><a href="/tvshow-1.html">The Office (UK) (2001-2003)</a></div></li>
				<li><div><a href="/tvshow-12.html">The Office (US) (2005-2013)</a></div></li>
				<li><div><a href="/tvshow-13.html">Not a show</a></div></li>
				</ul></div></body></html>`)
		case "/tvshow-1-2.html":
			_, _ = fmt.Fprint(w, `<html><body><table id="table5">
				<tr><td>Episode</td><td>Title</td></tr>
				<tr><td>2x1</td><td><a href="episode-100.html">Merger</a></td></tr>
				<tr><td>2x2</td><td><a href="episode-101.html">Interview</a></td></tr>
				</table></body></html>`)
		case "/episode-101.html":
			_, _ = fmt.Fprint(w, `<html><body>
				<a href="/subtitle-501.html"><div class="subtitlen"><h5><img src="images/flags/en.gif"> The.Office.S02E02.HDTV.XviD-LOL</h5><p title="rip">HDTV</p></div></a>
				<a href="/subtitle-502.html"><div class="subtitlen"><h5><img src="images/flags/br.gif"> The.Office.S02E02.WEB-FLEET</h5><p title="rip"></p></div></a>
				<a href="/subtitle-503.html"><div class="subtitlen"><h5><img src="images/flags/fr.gif"></h5><p title="rip">DVD</p></div></a>
				</body></html>`)
		case "/download-501.html":
			_, _ = fmt.Fprint(w, "<script>\nvar s1= 'files/The';\nvar s2= '_Office_2x02.zip';\n</script>")
		case "/download-502.html":
			_, _ = fmt.Fprint(w, "<script>\nvar s1= 'files/two.zip';\n</script>")
		case "/download-503.html":
			_, _ = fmt.Fprint(w, "<html>no link</html>")
		case "/files/The_Office_2x02.zip":
			_, _ = w.Write(zipped(t, "The.Office.S02E02.en.srt"))
		case "/files/two.zip":
			_, _ = w.Write(zipped(t, "a.srt", "b.srt"))
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
	return p.(*Provider), f
}

func TestSearch(t *testing.T) {
	p, f := newFake(t)
	v := &provider.Video{Episode: true, Series: "The Office", Year: 2001, Season: 2, Number: 2, ReleaseGroup: "DIMENSION"}
	langs := []provider.Language{{Tag: "en"}, {Tag: "pt-BR"}}
	for range 2 {
		subs, err := p.Search(t.Context(), v, langs)
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 2 {
			t.Fatalf("subtitles = %+v, want = 2", subs)
		}
		if s := subs[0]; s.ID != "501" || s.Language.Tag != "en" || !slices.Equal(s.ReleaseNames, []string{"The.Office.S02E02.HDTV.XviD-LOL", "HDTV"}) ||
			!slices.Equal(s.Matches, []string{"series", "season", "episode", "year", "release_group"}) || s.PageLink != p.link("subtitle-501.html") {
			t.Errorf("subtitle = %+v", s)
		}
		if s := subs[1]; s.ID != "502" || s.Language.Tag != "pt-BR" || slices.Contains(s.Matches, "release_group") || len(s.ReleaseNames) != 1 {
			t.Errorf("subtitle = %+v", s)
		}
	}
	// The show's and the episodes' IDs are kept.
	if n := f.hits("/search1.php") + f.hits("/tvshow-1-2.html"); n != 2 {
		t.Errorf("show and season requests = %d, want = 2", n)
	}
	for _, c := range []*provider.Video{
		{Episode: true, Series: "The Office", Year: 2002, Season: 2, Number: 2},
		{Episode: true, Series: "The Office", Year: 2001, Season: 2, Number: 9},
		{Title: "The Office"},
	} {
		if subs, err := p.Search(t.Context(), c, langs); err != nil || len(subs) != 0 {
			t.Errorf("search(%+v) = %v, %v, want = none", c, subs, err)
		}
	}
}

func TestDownload(t *testing.T) {
	p, _ := newFake(t)
	f, err := p.Download(t.Context(), nil, "501")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"; string(f.Data) != want || f.Format != "srt" {
		t.Errorf("download = %q %s, want = %q srt", f.Data, f.Format, want)
	}
	for id, kind := range map[string]provider.Kind{
		"502": provider.ProviderError, "503": provider.ProviderError, "504": provider.ProviderError, "x": provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != kind {
			t.Errorf("download %s: err = %v, want = %v", id, err, kind)
		}
	}
}

func TestLanguages(t *testing.T) {
	for code, tag := range map[string]string{"br": "pt-BR", "ua": "uk", "gr": "el", "cn": "zh", "jp": "ja", "cz": "cs", "en": "en", "pt": "pt"} {
		if got := fromSite(code); got != tag {
			t.Errorf("fromSite(%s) = %s, want = %s", code, got, tag)
		}
		if tag != "" && !slices.Contains(Definition().Languages, tag) {
			t.Errorf("%s is not among the languages", tag)
		}
	}
}
