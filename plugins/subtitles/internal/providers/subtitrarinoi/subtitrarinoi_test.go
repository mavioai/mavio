package subtitrarinoi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nSalut\r\n"

// breakingBad is the conftest's breaking_bad_s01e01.
func breakingBad() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Breaking Bad", Season: 1, Number: 1, ReleaseGroup: "REWARD",
		FileName: "Breaking.Bad.S01E01.720p.BluRay.X264-REWARD.mkv", Source: "Blu-Ray", Resolution: "720p",
		SeriesExternalIDs: map[string]string{"imdb": "tt0903747", "tvdb": "81189"},
	}
}

// fixture is the test's subtitrari_subtitle.
func fixture(comments string) hit {
	return hit{
		link: "https://www.subtitrari-noi.ro/7493-subtitrari noi.ro\\ ", title: "Breaking Bad", imdbID: "tt0903747",
		uploader: "Alice", page: "https://www.subtitrari-noi.ro/index.php?page=movie_details&act=1&id=7493",
		comments: comments, year: 2008, downloads: 4230,
	}
}

func TestPorted(t *testing.T) {
	ported.Run(t, "subtitrarinoi.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_subtitle_get_matches_episode": func(t *testing.T, args map[string]json.RawMessage) {
				got := fixture(ported.Arg[string](t, args, "comment")).matches(breakingBad())
				for _, m := range []string{"season", "episode", "series", "imdb_id"} {
					if !slices.Contains(got, m) {
						t.Errorf("matches got = %v, want = with %s", got, m)
					}
				}
			},
			"test_subtitle_get_matches_episode_false": func(t *testing.T, args map[string]json.RawMessage) {
				got := fixture(ported.Arg[string](t, args, "comment")).matches(breakingBad())
				if slices.Contains(got, "season") && slices.Contains(got, "episode") {
					t.Errorf("matches got = %v, want = without season and episode", got)
				}
			},
		},
		Facts: map[string]string{
			"test_list_subtitles":             "TestSearch",
			"test_provider_download_subtitle": "TestDownload",
		},
	})
}

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(w, "1\r\n00:00:01,000 --> 00:00:02,000\r\n%s\r\n", name)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// result is a subtitle of the search results, followed by its releases.
func result(id, title, uploader, downloads, imdb, comments string) string {
	return fmt.Sprintf(`<div id="round"><div id="content-main"><a href="index.php?page=movie_details&id=%s">%s</a>
		<p>a</p><p>b</p><p>c</p><p>d</p><p>Uploader: %s</p></div>
		<div id="content-right"><p>Descarcari: %s</p><a href="http://other">x</a><a href="https://www.imdb.com/title/%s/"><img src="/images/imdb.png"></a></div>
		<div class="buton"><a href="%s-subtitrari noi.ro">Download</a></div></div>
		<div>%s</div>`, id, title, uploader, downloads, imdb, id, comments)
}

type fakeSite struct {
	srv *httptest.Server

	mu       sync.Mutex
	searches []url.Values
	status   int
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	files := map[string][]byte{
		"7493": zipOf(t, "Breaking.Bad.S01E01.srt", "Breaking.Bad.S01E02.srt"),
		"8000": []byte(srt),
		"9000": []byte("<html>gone</html>"),
		"9001": zipOf(t, "readme.txt"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /paginare_filme.php", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		f.mu.Lock()
		f.searches = append(f.searches, r.PostForm)
		status := f.status
		f.mu.Unlock()
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Errorf("X-Requested-With got = %q, want = XMLHttpRequest", r.Header.Get("X-Requested-With"))
		}
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		switch r.PostForm.Get("cautare") {
		case "0903747":
			_, _ = w.Write([]byte(`<div id="header" class="x">menu</div>` +
				result("8000", "Breaking Bad - Sezonul 2 (2009)", "Bob", "100", "tt0903747", "Sezonul 2;HDTV") +
				result("7493", "Breaking Bad (2008)", "Alice", "4230", "tt0903747", "Sezonul 1 ep. 1-7;BluRay-REWARD")))
		case "inception":
			_, _ = w.Write([]byte(result("8000", "Inception (2010)", "Carol", "7", "tt1375666", "Inception.2010.1080p")))
		default:
			_, _ = w.Write([]byte(`<p>Nimic</p>`))
		}
	})
	mux.HandleFunc("GET /{file}", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.Header.Get("Referer"), "/paginare_filme.php") {
			t.Errorf("download Referer got = %q, want = the search", r.Header.Get("Referer"))
		}
		id, _, _ := strings.Cut(r.PathValue("file"), "-")
		data, ok := files[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
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
	romanian := []provider.Language{{Tag: "ro", HearingImpaired: true}}
	for _, c := range []struct {
		name   string
		video  *provider.Video
		search string
		want   []string
	}{
		{
			"episode", breakingBad(), "0903747",
			[]string{
				"/7493-subtitrari%20noi.ro 4230 Alice [Sezonul 1 ep. 1-7,BluRay-REWARD] [release_group series imdb_id season episode]",
				"/8000-subtitrari%20noi.ro 100 Bob [Sezonul 2,HDTV] [series imdb_id]",
			},
		},
		{
			// A movie without an IMDb ID is searched by its sanitized title.
			"movie by title", &provider.Video{Title: "Inception", Year: 2010, ExternalIDs: map[string]string{}}, "inception",
			[]string{"/8000-subtitrari%20noi.ro 7 Carol [Inception.2010.1080p] [year title]"},
		},
		{"not found", &provider.Video{Title: "Nothing", ExternalIDs: map[string]string{"imdb": "tt1"}}, "1", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			p := newProvider(t, f)
			subs, err := p.Search(t.Context(), c.video, romanian)
			var got []string
			for _, s := range subs {
				got = append(got, fmt.Sprintf("%s %d %s %v %v", strings.TrimPrefix(strings.ReplaceAll(s.ID, " ", "%20"), f.srv.URL), s.Downloads, s.Uploader, s.ReleaseNames, s.Matches))
				if s.Language != romanian[0] {
					t.Errorf("language got = %+v, want = %+v", s.Language, romanian[0])
				}
			}
			if err != nil || !slices.Equal(got, c.want) {
				t.Errorf("Search got = %v, %v, want = %v", got, err, c.want)
			}
			q := f.searches[0]
			if q.Get("cautare") != c.search || q.Get("query_q") != c.search || q.Get("tip") != "2" || q.Get("an") != "Toti anii" {
				t.Errorf("search form got = %v, want = cautare %q", q, c.search)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	for _, c := range []struct {
		status int
		kind   provider.Kind
	}{
		{http.StatusTooManyRequests, provider.TooManyRequests},
		{http.StatusForbidden, provider.ForbiddenError},
		{http.StatusBadGateway, provider.ServiceUnavailable},
		{http.StatusNotFound, provider.ProviderError},
	} {
		f := newFake(t)
		f.status = c.status
		p := newProvider(t, f)
		if _, err := p.Search(t.Context(), breakingBad(), []provider.Language{{Tag: "ro"}}); provider.KindOf(err) != c.kind {
			t.Errorf("Search of %d got = %v, want = %v", c.status, err, c.kind)
		}
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	ep2 := breakingBad()
	ep2.Number = 2
	for _, c := range []struct {
		name  string
		video *provider.Video
		file  string
		want  string
		kind  provider.Kind
	}{
		{"episode of an archive", breakingBad(), "7493", "Breaking.Bad.S01E01.srt", provider.Other},
		{"other episode of an archive", ep2, "7493", "Breaking.Bad.S01E02.srt", provider.Other},
		{"subtitle", breakingBad(), "8000", "Salut", provider.Other},
		{"not a subtitle", breakingBad(), "9000", "", provider.BadSubtitle},
		{"archive without subtitles", breakingBad(), "9001", "", provider.BadSubtitle},
		{"missing", breakingBad(), "404", "", provider.ProviderError},
	} {
		t.Run(c.name, func(t *testing.T) {
			file, err := p.Download(t.Context(), c.video, f.srv.URL+"/"+c.file+"-subtitrari noi.ro")
			if c.want == "" {
				if provider.KindOf(err) != c.kind || err == nil {
					t.Errorf("Download got = %v, want = %v", err, c.kind)
				}
				return
			}
			if err != nil || !strings.Contains(string(file.Data), c.want) || strings.Contains(string(file.Data), "\r") || file.Format != "srt" {
				t.Errorf("Download got = %q, %q, %v, want = %s", file.Data, file.Format, err, c.want)
			}
		})
	}
}
