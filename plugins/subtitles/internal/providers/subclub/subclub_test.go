package subclub

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
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "subclub.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_title_regex": func(t *testing.T, args map[string]json.RawMessage) {
				text := ported.Arg[string](t, args, "anchor_text")
				want := ported.Arg[map[string]*string](t, args, "expected")
				m := titleRe.FindStringSubmatch(text)
				if m == nil {
					t.Fatalf("%q: got = no match, want = a match", text)
				}
				for i, name := range titleRe.SubexpNames()[1:] {
					w := ""
					if want[name] != nil {
						w = *want[name]
					}
					if m[i+1] != w {
						t.Errorf("%q: %s got = %q, want = %q", text, name, m[i+1], w)
					}
				}
			},
		},
		Facts: map[string]string{
			"test_subtitle_movie_matches":                       "TestMatches",
			"test_subtitle_episode_matches":                     "TestMatches",
			"test_list_subtitles_skips_when_no_estonian":        "TestSearchNoEstonian",
			"test_list_subtitles_movie_inception":               "TestSearch",
			"test_list_subtitles_movie_shrek2_archive_fallback": "TestSearch",
			"test_list_subtitles_movie_simpsons_movie_mixed":    "TestSearch",
			"test_list_subtitles_episode_got_s01e01":            "TestSearch",
		},
	})
}

func TestMatches(t *testing.T) {
	movie := &provider.Video{Title: "Inception", Year: 2010, ExternalIDs: map[string]string{"imdb": "tt1375666"}}
	h := hit{title: "Inception", year: 2010, imdbID: "tt1375666"}
	if got, want := matches(h, movie), []string{"title", "year", "imdb_id"}; !slices.Equal(got, want) {
		t.Errorf("movie matches got = %v, want = %v", got, want)
	}
	ep := &provider.Video{
		Episode: true, Series: "Game of Thrones", Season: 1, Number: 1, Year: 2011,
		SeriesExternalIDs: map[string]string{"imdb": "tt0944947"},
	}
	h = hit{title: "Game of Thrones", year: 2011, season: 1, episode: 1, imdbID: "tt0944947"}
	if got, want := matches(h, ep), []string{"series", "season", "episode", "year", "series_imdb_id"}; !slices.Equal(got, want) {
		t.Errorf("episode matches got = %v, want = %v", got, want)
	}
}

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

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nTere\r\n"

// row is a row of the site's search results.
func row(id, anchor, imdb, rating string) string {
	return fmt.Sprintf(`<tr><td>1</td><td><a class="sc_link" href="../down.php?id=%s">%s</a></td><td>x</td>
		<td><a href="https://www.imdb.com/title/%s/">IMDb</a></td><td></td><td></td><td>23,976</td>
		<td><span>%s</span></td><td> uploader </td></tr>`, id, anchor, imdb, rating)
}

type fakeSite struct {
	srv      *httptest.Server
	archives map[string][]byte
	listings map[string][]string
	searches []string
	status   int
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{
		archives: map[string][]byte{
			"5159":  zipOf(t, map[string]string{"Simpsons/The.Simpsons.Movie.DVDRip.srt": srt, "Simpsons/.hidden.srt": srt, "readme.nfo": "x"}),
			"10200": zipOf(t, map[string]string{"Shrek.2.BRUTUS.srt": srt, "Shrek.2.ShareConnector.CD1.srt": srt}),
		},
		listings: map[string][]string{
			"10100": {"Inception.720p.BluRay.x264-CROSSBOW.srt", "Inception.DVDRip.sub"},
			"5270":  {"The.Simpsons.Movie.720p.srt"},
			"11232": {"Game.of.Thrones.S01E01.720p.BluRay.X264-REWARD.srt"},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /jutud.php", func(w http.ResponseWriter, r *http.Request) {
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		q := r.URL.Query().Get("otsing")
		f.searches = append(f.searches, q)
		var rows []string
		switch q {
		case "Inception":
			rows = []string{row("10100", "Inception (2010)", "tt1375666", "5,0"), row("10101", "Inception (2009)", "tt1", "4")}
		case "Shrek 2":
			rows = []string{row("10200", "Shrek 2 (2004)", "tt0298148", "4")}
		case "The Simpsons Movie":
			rows = []string{row("5159", "The Simpsons Movie (2007)", "tt0462538", "3"), row("5270", "The Simpsons Movie (2007)", "tt0462538", "4.5")}
		case "Game of Thrones":
			rows = []string{
				row("11232", "Game of Thrones (2011) [01x01]", "tt0944947", "5"),
				row("11233", "Game of Thrones (2011) [01x02]", "tt0944947", "5"),
				row("11234", "Game of Thrones (2011)", "tt0944947", "5"),
				`<tr><td>short</td></tr>`,
			}
		}
		_, _ = fmt.Fprintf(w, `<html><body><table id="tale_list"><thead><tr><th>x</th></tr></thead><tbody>%s</tbody></table></body></html>`, strings.Join(rows, ""))
	})
	mux.HandleFunc("GET /subtitles_archivecontent.php", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		for _, name := range f.listings[id] {
			_, _ = fmt.Fprintf(w, `<div><a href="../down.php?id=%s&amp;filename=%s">%s</a></div>`, id, url.QueryEscape(name), name)
		}
		_, _ = fmt.Fprintf(w, `<a href="../down.php?id=%s">all</a>`, id)
	})
	mux.HandleFunc("GET /down.php", func(w http.ResponseWriter, r *http.Request) {
		id, name := r.URL.Query().Get("id"), r.URL.Query().Get("filename")
		if name == "" {
			_, _ = w.Write(f.archives[id])
			return
		}
		if name == "Inception.DVDRip.sub" {
			return
		}
		_, _ = w.Write([]byte(srt))
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

var estonian = []provider.Language{{Tag: "et"}}

func TestSearchNoEstonian(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	subs, err := p.Search(t.Context(), &provider.Video{Title: "Inception", Year: 2010}, []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 0 || len(f.searches) != 0 {
		t.Errorf("got = %v, %v, %d searches, want = none and no search", subs, err, len(f.searches))
	}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	for _, c := range []struct {
		name    string
		video   *provider.Video
		files   []string
		matches []string
	}{
		{
			// The listing links each file: nothing is extracted.
			"inception", &provider.Video{Title: "Inception", Year: 2010, ExternalIDs: map[string]string{"imdb": "tt1375666"}},
			[]string{"10100/Inception.720p.BluRay.x264-CROSSBOW.srt", "10100/Inception.DVDRip.sub"},
			[]string{"title", "year", "imdb_id"},
		},
		{
			// An empty listing falls back to extracting the archive.
			"shrek 2", &provider.Video{Title: "Shrek 2", Year: 2004, ExternalIDs: map[string]string{"imdb": "tt0298148"}},
			[]string{"10200/Shrek.2.BRUTUS.srt", "10200/Shrek.2.ShareConnector.CD1.srt"},
			[]string{"title", "year", "imdb_id"},
		},
		{
			// Two archives, one listed, one extracted, the best rated first.
			"simpsons", &provider.Video{Title: "The Simpsons Movie", Year: 2007},
			[]string{"5270/The.Simpsons.Movie.720p.srt", "5159/The.Simpsons.Movie.DVDRip.srt"},
			[]string{"title", "year"},
		},
		{
			// Only the episode's archive.
			"got", &provider.Video{
				Episode: true, Series: "Game of Thrones", Season: 1, Number: 1, Year: 2011,
				SeriesExternalIDs: map[string]string{"imdb": "tt0944947"},
			},
			[]string{"11232/Game.of.Thrones.S01E01.720p.BluRay.X264-REWARD.srt"},
			[]string{"series", "season", "episode", "year", "series_imdb_id"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), c.video, estonian)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, s := range subs {
				ids = append(ids, s.ID)
				if !slices.Equal(s.Matches, c.matches) || s.Language.Tag != "et" || s.Uploader != "uploader" || !strings.HasPrefix(s.PageLink, f.srv.URL+"/down.php?id=") {
					t.Errorf("%s: got = %+v, want = matches %v", s.ID, s, c.matches)
				}
			}
			if !slices.Equal(ids, c.files) {
				t.Fatalf("got = %v, want = %v", ids, c.files)
			}
			file, err := p.Download(t.Context(), c.video, ids[0])
			if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" {
				t.Errorf("Download(%s) got = %q, %q, %v, want = the subtitle", ids[0], file.Data, file.Format, err)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	// A fresh provider finds the files of an archive again.
	for _, c := range []struct {
		id   string
		kind provider.Kind
	}{
		{"10200/Shrek.2.BRUTUS.srt", 0},
		{"10100/Inception.720p.BluRay.x264-CROSSBOW.srt", 0},
		{"10100/Inception.DVDRip.sub", provider.ProviderError},
		{"10100/Other.srt", provider.BadSubtitle},
		{"nope", provider.BadSubtitle},
	} {
		_, err := newProvider(t, f).Download(t.Context(), nil, c.id)
		if c.kind == 0 && err != nil || c.kind != 0 && provider.KindOf(err) != c.kind {
			t.Errorf("Download(%s) got = %v, want = %v", c.id, err, c.kind)
		}
	}
}

func TestSearchFailures(t *testing.T) {
	f := newFake(t)
	f.status = http.StatusServiceUnavailable
	// Bazarr skips failing searches.
	subs, err := newProvider(t, f).Search(t.Context(), &provider.Video{Title: "Inception", AlternativeTitles: []string{"Origin"}}, estonian)
	if err != nil || len(subs) != 0 {
		t.Errorf("got = %v, %v, want = none", subs, err)
	}
	f.status = 0
	_, err = newProvider(t, f).extract(t.Context(), "11232")
	if provider.KindOf(err) != provider.ProviderError {
		t.Errorf("extract of no archive got = %v, want = %v", err, provider.ProviderError)
	}
}
