package supersubtitles

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

// moviePage is the site's search page of a movie, as Bazarr's test of
// the provider's year.
func moviePage(rows ...string) string {
	return `<table><tr><td>header</td></tr><tr><td>header</td></tr>` + strings.Join(rows, "") + `</table>`
}

func movieRow(title, lang, id, magyar string) string {
	return fmt.Sprintf(`<tr id="vilagit">
                    <td></td>
                    <td><small>%s</small></td>
                    <td>
                        <div class="eredeti">
                            %s
                        </div>
                        <div class="magyar">%s</div>
                    </td>
                    <td>Anonymus</td>
                    <td>2019-03-02</td>
                    <td>
                        <a href="/index.php?action=letolt&amp;fnev=Obey 2018.srt&amp;felirat=%s">
                            Download
                        </a>
                    </td>
                </tr>`, lang, title, magyar, id)
}

type fake struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	requests []string
	// movie and series answer the search of a movie and the list of a
	// series' subtitles.
	movie  string
	series map[string]string
}

func newFake(t *testing.T) (*Provider, *fake) {
	t.Helper()
	f := &fake{t: t, series: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL + "/")
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).pause = 0
	return p.(*Provider), f
}

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/index.php" {
		http.NotFound(w, r)
		return
	}
	if !strings.HasSuffix(r.Header.Get("Referer"), "/index.php") {
		f.t.Errorf("referer = %q", r.Header.Get("Referer"))
	}
	q := r.URL.Query()
	f.mu.Lock()
	f.requests = append(f.requests, r.URL.RawQuery)
	f.mu.Unlock()
	switch {
	case q.Get("action") == "autoname":
		switch q.Get("term") {
		case "La Brea", "Brea":
			_, _ = fmt.Fprint(w, `[{"name":"La Brea (2021)","ID":"5678"},{"name":"La Brea (2001)","ID":"1"}]`)
		case "Loki":
			_, _ = fmt.Fprint(w, `[{"name":"Loki (aka. Marvel's Loki) (2021)","ID":"4321"}]`)
		default:
			_, _ = fmt.Fprint(w, `<html>not JSON</html>`)
		}
	case q.Get("action") == "xbmc":
		key := q.Get("sid") + "/" + q.Get("ev") + "/" + q.Get("rtol")
		_, _ = fmt.Fprint(w, f.series[key])
	case q.Get("tipus") == "adatlap":
		_, _ = fmt.Fprint(w, `<html><body><a href="index.php">x</a><a alt="iMDB" href="http://www.imdb.com/title/tt11640018/" target="_blank"><img alt="iMDB" src="img/adatlap/imdb.png"/></a></body></html>`)
	case q.Get("tab") == "film":
		if q.Get("search") != "Obey" {
			f.t.Errorf("search = %q, want = Obey", q.Get("search"))
		}
		_, _ = fmt.Fprint(w, f.movie)
	case q.Get("action") == "letolt":
		switch q.Get("felirat") {
		case "1643361676":
			_, _ = w.Write(zipped(f.t, "La.Brea.S02E12.srt", srt("12"), "La.Brea.S02E13.srt", srt("13")))
		case "1634579718":
			_, _ = w.Write([]byte(srt("Dune")))
		case "1681841063":
			_, _ = w.Write(zipped(f.t, "Dune.hu.srt", srt("Dűne")))
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

// video2 is the episode of Bazarr's test_list_video_2.
func video2() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "La Brea", Season: 2, Number: 13, Title: "The Journey (1)", Year: 2021,
		ReleaseGroup: "CAKES", Resolution: "720p", Source: "Web", VideoCodec: "H.264",
		SeriesExternalIDs: map[string]string{"imdb": "tt11640018", "tvdb": "395029"},
	}
}

const episodeList = `{
	"10": {"language":"Magyar","nev":"La Brea - 2x13 (WEB.720p-CAKES)","baselink":"x","fnev":"a.zip","felirat":"1643361676","evad":"2","ep":"13","feltolto":"J1GG4","pontos_talalat":"111","evadpakk":"0"},
	"11": {"language":"Magyar","nev":"La Brea - 2x13 (WEB.1080p-NTb)","fnev":"a.zip","felirat":"1643361676","evad":"2","ep":"13","feltolto":"J1GG4","evadpakk":"0"},
	"12": {"language":"Angol","nev":"La Brea - 2x13 (WEB.720p-CAKES)","fnev":"b.srt","felirat":"1643361677","evad":"2","ep":"13","feltolto":"x","evadpakk":"0"},
	"13": {"language":"Magyar","nev":"La Brea (Season 2) (WEB.720p-CAKES)","fnev":"c.zip","felirat":"1643361678","evad":"2","ep":"-1","feltolto":"y","evadpakk":"1"}
}`

func TestSearchEpisode(t *testing.T) {
	p, f := newFake(t)
	f.series["5678/2/13"] = episodeList
	subs, err := p.Search(t.Context(), video2(), []provider.Language{{Tag: "hu"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subtitles = %+v, want = 1", subs)
	}
	s := subs[0]
	if s.ID != "/index.php?action=letolt&felirat=1643361676" || s.Language != (provider.Language{Tag: "hu"}) || s.Uploader != "J1GG4" {
		t.Errorf("subtitle = %+v", s)
	}
	// The site's season and episode are in the release names the server
	// guesses from, as Bazarr's update_matches.
	if want := []string{"La Brea (2021) S02E13", "WEB.720p-CAKES", "WEB.1080p-NTb"}; !slices.Equal(s.ReleaseNames, want) {
		t.Errorf("release names = %v, want = %v", s.ReleaseNames, want)
	}
	if want := []string{"series", "series_imdb_id", "year", "release_group", "resolution"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches = %v, want = %v", s.Matches, want)
	}

	// Without the episode's subtitles, the season's packs.
	f.series["5678/2/13"] = `[]`
	f.series["5678/2/"] = episodeList
	subs, err = p.Search(t.Context(), video2(), []provider.Language{{Tag: "hu"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].ID != "/index.php?action=letolt&felirat=1643361678" || subs[0].ReleaseNames[0] != "La Brea (2021) S02E13" {
		t.Errorf("subtitles = %+v, want the season's pack", subs)
	}
}

func TestFindID(t *testing.T) {
	p, f := newFake(t)
	for _, c := range []struct {
		series string
		year   int
		want   string
		terms  []string
	}{
		{"La Brea", 2021, "5678", []string{"La%20Brea"}},
		{"La Brea", 0, "5678", []string{"La%20Brea"}},
		{"La Brea", 2001, "1", []string{"La%20Brea"}},
		{"La Brea", 1999, "", []string{"La%20Brea", "La+Brea", "Brea"}},
		{"Loki", 2021, "4321", []string{"Loki"}},
	} {
		f.mu.Lock()
		f.requests = nil
		f.mu.Unlock()
		if _, err := p.query(t.Context(), &provider.Video{Episode: true, Series: c.series, Year: c.year, Season: 1, Number: 1}, c.series, nil); err != nil {
			t.Fatal(err)
		}
		var terms []string
		var sid string
		for _, r := range f.requests {
			q, _ := url.ParseQuery(r)
			if q.Get("action") == "autoname" {
				term := strings.TrimSuffix(strings.TrimPrefix(r, "term="), "&nyelv=0&action=autoname")
				terms = append(terms, term)
			}
			if q.Get("action") == "xbmc" {
				sid = q.Get("sid")
			}
		}
		if sid != c.want || !slices.Equal(terms, c.terms) {
			t.Errorf("%s (%d): series ID = %q from %v, want = %q from %v", c.series, c.year, sid, terms, c.want, c.terms)
		}
	}
}

func TestSearchMovie(t *testing.T) {
	p, f := newFake(t)
	f.movie = moviePage(
		movieRow("Obey (2018) (WEBRip.720p-YIFY, WEB.1080p-NTb)", "Angol", "1551531078", ""),
		movieRow("Obey (2018) (WEBRip.720p-YIFY)", "Magyar", "1551531079", "szinkronoshoz"),
		movieRow("Other (2018) (WEBRip.720p-YIFY)", "Angol", "1551531080", ""),
	)
	v := &provider.Video{Title: "Obey", Year: 2018, ReleaseGroup: "NTb", ExternalIDs: map[string]string{"imdb": "tt11640018"}}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}, {Tag: "hu", Forced: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("subtitles = %+v, want = 2", subs)
	}
	s := subs[0]
	if s.ID != "index.php?action=letolt&fnev=Obey 2018.srt&felirat=1551531078" || s.Uploader != "Anonymus" || s.Language != (provider.Language{Tag: "en"}) ||
		!slices.Equal(s.ReleaseNames, []string{"Obey (2018)", "WEBRip.720p-YIFY", "WEB.1080p-NTb"}) {
		t.Errorf("subtitle = %+v", s)
	}
	if want := []string{"title", "imdb_id", "year", "release_group"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches = %v, want = %v", s.Matches, want)
	}
	if subs[1].Language != (provider.Language{Tag: "hu", Forced: true}) {
		t.Errorf("language = %+v, want = forced Hungarian", subs[1].Language)
	}
}

func TestDownload(t *testing.T) {
	p, _ := newFake(t)
	for _, c := range []struct {
		id   string
		v    *provider.Video
		want string
	}{
		// Bazarr's test_download_episode_subtitle, test_download_movie_subtitle
		// and test_download_movie_subtitle_hungarian.
		{"/index.php?action=letolt&felirat=1643361676", video2(), "13"},
		{"index.php?action=letolt&felirat=1634579718", &provider.Video{Title: "Dune"}, "Dune"},
		{"/index.php?action=letolt&felirat=1681841063", &provider.Video{Title: "Dune"}, "Dűne"},
	} {
		f, err := p.Download(t.Context(), c.v, c.id)
		if err != nil {
			t.Fatalf("download %s: %v", c.id, err)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\n" + c.want + "\n"; string(f.Data) != want || f.Format != "srt" {
			t.Errorf("download %s = %q, want = %q", c.id, f.Data, want)
		}
	}
	if _, err := p.Download(t.Context(), video2(), "index.php?action=letolt&felirat=1"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("download of a missing subtitle: err = %v, want = ProviderError", err)
	}
}

func TestMatches(t *testing.T) {
	// Bazarr's test_download_episode_subtitle matches its subtitle.
	r := &result{series: "All of us are dead", season: 1, episode: 11, releases: []string{"NF.WEB-DL.1080p-TEPES", "WEBRip.1080p-RARBG"}}
	v := &provider.Video{Episode: true, Series: "All of Us Are Dead", Season: 1, Number: 11, OriginalSeries: true, Resolution: "1080p", Source: "Web"}
	if got, want := r.matches(v), []string{"series", "year"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want = %v", got, want)
	}
	if got, want := r.releaseInfo(), []string{"All of us are dead S01E11", "NF.WEB-DL.1080p-TEPES", "WEBRip.1080p-RARBG"}; !slices.Equal(got, want) {
		t.Errorf("release info = %v, want = %v", got, want)
	}
	v.ReleaseGroup = "rarbg"
	if got, want := r.matches(v), []string{"series", "year", "release_group", "resolution"}; !slices.Equal(got, want) {
		t.Errorf("matches = %v, want = %v", got, want)
	}
}

func TestPorted(t *testing.T) {
	ported.Run(t, "supersubtitles.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_movie_result_uses_provider_year": func(t *testing.T, args map[string]json.RawMessage) {
				title := ported.Arg[string](t, args, "provider_title")
				want := ported.Arg[*int](t, args, "expected_year")
				p, f := newFake(t)
				f.movie = moviePage(movieRow(title, "Angol", "1551531078", ""))
				movie := &provider.Video{Title: "Obey", Year: 2013, FileName: "Obey (2013) - [720p][AAC 2.0][AVC].mp4"}
				results, err := p.processSubs(t.Context(), movie, []provider.Language{{Tag: "en"}}, p.link("index.php?search=Obey&soriSorszam=&nyelv=&tab=film"))
				if err != nil {
					t.Fatal(err)
				}
				if len(results) != 1 {
					t.Fatalf("subtitles = %d, want = 1", len(results))
				}
				r := results[0]
				if wantYear := 0; want != nil && r.year != *want || want == nil && r.year != wantYear {
					t.Errorf("year = %d, want = %v", r.year, want)
				}
				if info := strings.Join(r.releaseInfo(), "\n"); strings.Contains(info, "(None)") || strings.Contains(info, "(0)") {
					t.Errorf("release info = %q, want no year", info)
				}
				if slices.Contains(r.matches(movie), "year") {
					t.Errorf("matches = %v, want no year", r.matches(movie))
				}
			},
		},
		Facts: map[string]string{
			"test_list_episode_subtitles":            "TestSearchEpisode",
			"test_download_episode_subtitle":         "TestDownload",
			"test_list_and_download_movie_subtitles": "TestSearchMovie",
			"test_download_movie_subtitle":           "TestDownload",
			"test_download_movie_subtitle_hungarian": "TestDownload",
			"test_list_video_2":                      "TestSearchEpisode",
		},
		Skip: map[string]string{
			"test_subtitle_reprs": "subtitles are plain values without Python's repr and str",
		},
	})
}
