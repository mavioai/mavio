package titrari

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

// result is a result of the site's search.
func result(title, href, uploader, downloads, comment string) string {
	return fmt.Sprintf(`<tr><td><h1><a href="index.php?page=detalii&id=1">%s</a></h1></td>
		<td rowspan="5"><a href="%s">Descarca</a></td>
		<td class="row1 stanga"><a href="profil">x</a><a href="profil">%s</a></td></tr>
		<tr><td><span>Descarcari: %s</span></td></tr>
		<tr><td><a href="https://www.imdb.com/title/tt1049413"><img src="img/imdb.png"></a></td></tr>
		<tr><td class="comment">Comentariu:</td><td class="comment">%s</td></tr>`, title, href, uploader, downloads, comment)
}

type fake struct {
	mu      sync.Mutex
	queries []url.Values
	homes   int
}

func newFake(t *testing.T) (*Provider, *fake) {
	t.Helper()
	f := &fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/" && len(q) == 0:
			f.homes++
			_, _ = fmt.Fprint(w, `<html><body><a href="index.php?page=acasa">Acasa</a><a href="index.php?page=cautaresecreta&x=1">Cautare Avansată</a></body></html>`)
		case r.URL.Path == "/":
			f.queries = append(f.queries, q)
			switch q.Get("z5") {
			case "0000429":
				http.Error(w, "Too many requests", http.StatusNotFound)
				return
			case "0000500":
				http.Error(w, "down", http.StatusInternalServerError)
				return
			}
			_, _ = fmt.Fprint(w, `<html><body><table>`+
				result("Up (2009)", "get.php?id=1", "ana", "12", "Up.2009.720p.BluRay.x264-SPARKS")+
				result("Up (2009)", "get.php?id=2", "ion", "40", "Up.2009.DVDRip")+
				`</table></body></html>`)
		case r.URL.Path == "/get.php":
			if r.Header.Get("Referer") == "" {
				t.Errorf("referer is not set")
			}
			switch r.URL.Query().Get("id") {
			case "1":
				_, _ = w.Write(zipped(t, "Up.2009.srt", srt("Sus")))
			case "2":
				_, _ = w.Write([]byte(srt("Sus")))
			case "3":
				_, _ = w.Write(zipped(t, ".hidden.S01E02.srt", srt("x"), "Show.S01E01.srt", srt("one"), "Show.S01E02.srt", srt("two")))
			case "4":
				_, _ = w.Write([]byte("<html>nope</html>"))
			}
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
	p.(*Provider).pause = 0
	return p.(*Provider), f
}

func TestSearchMovie(t *testing.T) {
	p, f := newFake(t)
	v := &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": "tt1049413"}, ReleaseGroup: "SPARKS"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}, {Tag: "en", Forced: true}})
	if err != nil {
		t.Fatal(err)
	}
	if f.homes != 1 {
		t.Errorf("home page requests = %d, want = 1", f.homes)
	}
	if len(f.queries) != 2 {
		t.Fatalf("queries = %v, want = 2", f.queries)
	}
	for i, z8 := range []string{"1", "-1"} {
		q := f.queries[i]
		if q.Get("page") != "cautaresecreta" || q.Get("z5") != "1049413" || q.Get("z7") != "" || q.Get("z8") != z8 || q.Get("z9") != "All" {
			t.Errorf("query %d = %v, want z8 = %s", i, q, z8)
		}
	}
	if len(subs) != 4 {
		t.Fatalf("subtitles = %d, want = 4", len(subs))
	}
	// The most downloaded first, in the language searched.
	if s := subs[0]; s.ID != "get.php?id=2" || s.Downloads != 40 || s.Uploader != "ion" || s.Language != (provider.Language{Tag: "ro"}) ||
		!slices.Equal(s.Matches, []string{"year", "title", "imdb_id"}) || !slices.Equal(s.ReleaseNames, []string{"Up.2009.DVDRip"}) {
		t.Errorf("subtitle = %+v", s)
	}
	if s := subs[1]; s.ID != "get.php?id=1" || !slices.Equal(s.Matches, []string{"year", "release_group", "title", "imdb_id"}) {
		t.Errorf("subtitle = %+v", s)
	}
	if s := subs[2]; s.Language != (provider.Language{Tag: "en", Forced: true}) {
		t.Errorf("language = %+v, want = forced English", s.Language)
	}
}

func TestSearchEpisode(t *testing.T) {
	p, f := newFake(t)
	v := &provider.Video{Episode: true, Series: "Up", Title: "Pilot", Season: 2009, Number: 2}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if q := f.queries[0]; q.Get("z7") != "pilot" || q.Get("z5") != "" || q.Get("z8") != "2" {
		t.Errorf("query = %v, want the title", q)
	}
	if len(subs) != 2 || !slices.Equal(subs[0].Matches, []string{"series"}) {
		t.Errorf("subtitles = %+v", subs)
	}
	v.SeriesExternalIDs = map[string]string{"imdb": "tt1049413"}
	v.Series, v.Season = "Show", 1
	got := matches(v, "Show - Sezonul 1 ", "tt1049413", "", 0)
	if want := []string{"series", "imdb_id", "season", "episode"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want = %v", got, want)
	}
}

func TestStatus(t *testing.T) {
	p, _ := newFake(t)
	for imdb, kind := range map[string]provider.Kind{"tt0000429": provider.TooManyRequests, "tt0000500": provider.ServiceUnavailable} {
		v := &provider.Video{Title: "Up", ExternalIDs: map[string]string{"imdb": imdb}}
		if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "ro"}}); provider.KindOf(err) != kind {
			t.Errorf("search %s: err = %v, want = %v", imdb, err, kind)
		}
	}
	def := Definition()
	if d := provider.Throttle(&def, provider.TooManyRequests, time.Now()); d != 10*time.Minute {
		t.Errorf("throttle = %v, want = 10m", d)
	}
}

func TestDownload(t *testing.T) {
	p, _ := newFake(t)
	movie := &provider.Video{Title: "Up"}
	episode := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}
	for _, c := range []struct {
		id   string
		v    *provider.Video
		want string
		kind provider.Kind
	}{
		{"get.php?id=1", movie, "Sus", -1},
		{"get.php?id=2", movie, "Sus", -1},
		{"get.php?id=2", episode, "Sus", -1},
		{"get.php?id=3", episode, "two", -1},
		{"get.php?id=3", &provider.Video{Episode: true, Number: 7}, "", provider.BadSubtitle},
		{"get.php?id=4", movie, "", provider.ProviderError},
	} {
		f, err := p.Download(t.Context(), c.v, c.id)
		if c.kind >= 0 {
			if provider.KindOf(err) != c.kind || err == nil {
				t.Errorf("download %s: err = %v, want = %v", c.id, err, c.kind)
			}
			continue
		}
		if err != nil {
			t.Fatalf("download %s: %v", c.id, err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\n" + c.want + "\n"; string(f.Data) != want || f.Format != "srt" {
			t.Errorf("download %s = %q %s, want = %q srt", c.id, f.Data, f.Format, want)
		}
	}
}
