package subf2m

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

const userAgent = "Mozilla/5.0 (Linux; Android 10; SM-G996U Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Mobile Safari/537.36"

// titles are the site's search results: what each finds when a word of
// the query is in it.
var titles = []struct{ href, text string }{
	{"/subtitles/pirates-of-the-caribbean-2-dead-mans-chest", "Pirates of the Caribbean 2: Dead Man's Chest (2006)"},
	{"/subtitles/dead-man", "Dead Man (1995)"},
	{"/subtitles/dune-2021", "Dune (2021)"},
	{"/subtitles/dune", "Dune (1984)"},
	{"/subtitles/dune-part-two", "Dune: Part Two (2024)"},
	{"/subtitles/cure-kyua", "Cure (Kyua) (1997)"},
	{"/subtitles/the-goonies", "The Goonies (1985)"},
	{"/subtitles/breaking-bad-first-season", "Breaking Bad - First Season (2008)"},
	{"/subtitles/breaking-bad-second-season", "Breaking Bad - Second Season (2009)"},
	{"/subtitles/house-of-the-dragon-first-season", "House of the Dragon - First Season (2022)"},
	{"/subtitles/the-bear-first-season", "The Bear - First Season (2022)"},
	{"/subtitles/courage-the-cowardly-dog", "Courage the Cowardly Dog - Complete Series (1999)"},
	{"/subtitles/the-twilight-zone-the-complete-original-series", "The Twilight Zone (The Complete Original Series) (1959)"},
	{"/subtitles/the-twilight-zone-second-season", "The Twilight Zone - Second Season (2020)"},
	{"/subtitles/sam-max-freelance-police", "Sam & Max: Freelance Police (Complete Series) (1997)"},
	{"/subtitles/dr-romantic-third-season", "Dr. Romantic - Third Season (2023)"},
}

// subtitlesPage is a page of subtitles of a title.
func subtitlesPage(imdb string, items ...string) string {
	return fmt.Sprintf(`<html><body><div id="content"><div class="subtitles byFilm"><div class="box clearfix">
		<div class="top left"><div class="header"><h2><a href="https://www.imdb.com/title/%s">IMDb</a></h2></div></div></div>
		<ul class="sublist">%s</ul></div></div></body></html>`, imdb, strings.Join(items, ""))
}

// subItem is a subtitle of a page.
func subItem(id string, releases []string, comment string) string {
	var lis string
	for _, r := range releases {
		lis += "<li>" + r + "</li>\n"
	}
	link := ""
	if id != "" {
		link = `<a class="download icon-download" href="` + id + `">Download</a>`
	}
	return fmt.Sprintf(`<li class="item"><ul class="scrolllist">%s</ul><div class="comment-col"><p>%s</p></div>%s</li>`, lis, comment, link)
}

type fakeSite struct {
	t   *testing.T
	srv *httptest.Server
	// results overrides the search results.
	results string

	mu       sync.Mutex
	statuses []int
	pages    map[string]string
	files    map[string][]byte
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

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{t: t, pages: map[string]string{}, files: map[string][]byte{}}
	dune := subtitlesPage("tt1160419",
		subItem("/subtitles/dune-2021/LANG/1", []string{"Dune.2021.WEBRip.x264-ION10", "Dune.2021.1080p.HDRip.X264-EVO"}, "Synced\nfor all"),
		subItem("", []string{"No.Link"}, ""))
	bb := subtitlesPage("tt0903747",
		subItem("/subtitles/breaking-bad-first-season/LANG/161227", []string{"Breaking.Bad.S01E01-7.DSR-HDTV.eng"}, ""),
		subItem("/subtitles/breaking-bad-first-season/LANG/2", []string{"Breaking.Bad.S01E03.720p.BluRay.x264"}, ""),
		subItem("/subtitles/breaking-bad-first-season/LANG/3", []string{"Breaking.Bad.S01.1080p.BluRay.x264-ROVERS"}, ""),
		subItem("/subtitles/breaking-bad-first-season/LANG/4", []string{"Something else"}, ""))
	for _, lang := range []string{"english", "brazillian-portuguese"} {
		f.pages["/subtitles/dune-2021/"+lang] = strings.ReplaceAll(dune, "LANG", lang)
		f.pages["/subtitles/breaking-bad-first-season/"+lang] = strings.ReplaceAll(bb, "LANG", lang)
	}
	f.pages["/subtitles/the-goonies/english"] = subtitlesPage("tt0089218", subItem("/subtitles/the-goonies/english/1", []string{"The.Goonies.1985.1080p.BluRay"}, ""))
	f.pages["/subtitles/dune-part-two/english"] = subtitlesPage("tt15239678", subItem("/subtitles/dune-part-two/english/1", []string{"Dune.Part.Two"}, ""))
	f.pages["/subtitles/courage-the-cowardly-dog/english"] = subtitlesPage("tt0220880",
		subItem("/subtitles/courage-the-cowardly-dog/english/1", []string{"Courage the Cowardly Dog complete series"}, ""),
		subItem("/subtitles/courage-the-cowardly-dog/english/2232402", []string{"Courage.The.Cowardly.Dog.S03.DVDRip"}, "Season 3 complete."))
	f.pages["/subtitles/sam-max-freelance-police/english"] = subtitlesPage("tt0125646",
		subItem("/subtitles/sam-max-freelance-police/english/1", []string{"Sam & Max Freelance Police - Complete Series"}, ""))
	f.pages["/subtitles/dr-romantic-third-season/english"] = subtitlesPage("tt6157190",
		subItem("/subtitles/dr-romantic-third-season/english/1", []string{"Dr.Romantic.S03E16.SBS.x265.1080p-thon"}, ""))
	// The subtitles' pages and their archives.
	archives := map[string][]byte{
		"/subtitles/dune-2021/farsi_persian/2604701":          zipOf(t, "Dune.2021.srt"),
		"/subtitles/breaking-bad-first-season/english/161227": zipOf(t, "Breaking.Bad.S01E01.srt", "Breaking.Bad.S01E07.srt"),
		"/subtitles/courage-the-cowardly-dog/english/2232402": zipOf(t, "Courage 3x12 - Snowman's Revenge.srt", "Courage - Feast of the Bullfrogs.srt"),
		"/subtitles/rick-and-morty-sixth-season/english/3060783": zipOf(t,
			"Rick.and.Morty.S06E06.Juricksic.Mort.srt", "Rick.and.Morty.S06E07.Full.Meta.Jackrick.srt"),
		"/subtitles/sam-max-freelance-police/english/1": zipOf(t, "Sam & Max 1x20.srt", "Sam & Max 1x21 The Glazed McGuffin Affair.srt"),
		"/subtitles/bad/english/1":                      []byte("<html>not an archive</html>"),
	}
	for page, data := range archives {
		f.pages[page] = `<html><body><a id="downloadButton" href="/download` + page + `">Download</a></body></html>`
		f.files["/download"+page] = data
	}
	f.pages["/subtitles/nolink/english/1"] = `<html><body>gone</body></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("GET /subtitles/searchbytitle", func(w http.ResponseWriter, r *http.Request) {
		if f.status(w) {
			return
		}
		if r.Header.Get("User-Agent") != userAgent || r.Header.Get("Authority") != "subf2m.co" {
			t.Errorf("headers got = %v", r.Header)
		}
		if f.results != "" {
			_, _ = w.Write([]byte(f.results))
			return
		}
		query := strings.ToLower(r.URL.Query().Get("query"))
		_, _ = w.Write([]byte("<html><body><div class=\"search-result\"><ul>"))
		for _, title := range titles {
			for word := range strings.FieldsSeq(query) {
				if len(word) > 3 && strings.Contains(strings.ToLower(title.text), word) ||
					strings.HasPrefix(word, "tt") && word == "tt0089218" && title.href == "/subtitles/the-goonies" {
					_, _ = fmt.Fprintf(w, `<li><div class="title"><a href="%s">%s</a></div></li>`, title.href, title.text)
					break
				}
			}
		}
		_, _ = w.Write([]byte("</ul></div></body></html>"))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if f.status(w) {
			return
		}
		if data, ok := f.files[r.URL.Path]; ok {
			_, _ = w.Write(data)
			return
		}
		page, ok := f.pages[r.URL.Path]
		if !ok {
			// The site's answer to inexistent pages.
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write([]byte(page))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// status answers with the next status set, if any.
func (f *fakeSite) status(w http.ResponseWriter) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.statuses) == 0 {
		return false
	}
	w.WriteHeader(f.statuses[0])
	f.statuses = f.statuses[1:]
	return true
}

func newProvider(t *testing.T, f *fakeSite) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)},
		json.RawMessage(`{"verify_ssl":true,"user_agent":"`+userAgent+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

// language reads a case's language expression.
func language(t *testing.T, args map[string]json.RawMessage) string {
	expr := ported.Arg[map[string]string](t, args, "language")["$expr"]
	switch expr {
	case `Language.fromalpha2("en")`:
		return "en"
	case `Language("por", "BR")`:
		return "pt-BR"
	}
	t.Fatalf("language %s", expr)
	return ""
}

func TestPorted(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	ported.Run(t, "subf_2m.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_search_movie": func(t *testing.T, args map[string]json.RawMessage) {
				got, err := p.searchMovie(t.Context(), ported.Arg[string](t, args, "title"), ported.Arg[int](t, args, "year"))
				if want := ported.Arg[string](t, args, "expected_url"); err != nil || !slices.Contains(got, want) {
					t.Errorf("got = %v, %v, want = %s in them", got, err, want)
				}
			},
			"test_search_tv_show_season": func(t *testing.T, args map[string]json.RawMessage) {
				got, err := p.searchSeason(t.Context(), ported.Arg[string](t, args, "series_title"), ported.Arg[int](t, args, "season"), ported.Arg[int](t, args, "year"))
				if want := ported.Arg[string](t, args, "expected_url"); err != nil || !slices.Contains(got, want) {
					t.Errorf("got = %v, %v, want = %s in them", got, err, want)
				}
			},
			"test_search_tv_show_season_ordinal_spelling": func(t *testing.T, args map[string]json.RawMessage) {
				word := ported.Arg[string](t, args, "season_word")
				href := "/subtitles/greys-anatomy-" + strings.ToLower(word) + "-season"
				f := newFake(t)
				f.results = fmt.Sprintf(`<li><div class='title'><a href="%s">Grey&#39;s Anatomy - %s Season  (2017)</a></div></li>`, href, word)
				got, err := newProvider(t, f).searchSeason(t.Context(), "grey's anatomy", ported.Arg[int](t, args, "season"), 0)
				if err != nil || !slices.Equal(got, []string{href}) {
					t.Errorf("got = %v, %v, want = %s", got, err, href)
				}
			},
			"test_find_movie_subtitles": func(t *testing.T, args map[string]json.RawMessage) {
				items, err := p.findMovie(t.Context(), "/subtitles/dune-2021", language(t, args), "tt1160419")
				if err != nil || len(items) != 1 || !strings.Contains(items[0].pageLink, languages[language(t, args)]) {
					t.Errorf("got = %v, %v, want = the subtitle in the language", items, err)
				}
			},
			"test_find_episode_subtitles": func(t *testing.T, args map[string]json.RawMessage) {
				items, err := p.findEpisode(t.Context(), "/subtitles/breaking-bad-first-season", 1, 1, language(t, args), "tt0903747")
				if err != nil || len(items) != 2 || !strings.Contains(items[0].pageLink, languages[language(t, args)]) {
					t.Errorf("got = %v, %v, want = the episodes' and the season pack's subtitles in the language", items, err)
				}
			},
			"test_download_subtitle_episode_with_title": func(t *testing.T, args map[string]json.RawMessage) {
				link := strings.Replace(ported.Arg[string](t, args, "page_link"), "https://subf2m.co", f.srv.URL, 1)
				v := &provider.Video{Episode: true, Number: ported.Arg[int](t, args, "episode_number"), Title: ported.Arg[string](t, args, "episode_title")}
				file, err := p.Download(t.Context(), v, link)
				words := strings.Fields(v.Title)
				if err != nil || !strings.Contains(string(file.Data), words[len(words)-1]) || file.Format != "srt" {
					t.Errorf("got = %q, %v, want = the subtitle of %q", file.Data, err, v.Title)
				}
			},
		},
		Facts: map[string]string{
			"test_list_subtitles_movie_falls_back_to_imdb_id":         "TestSearchMovie",
			"test_init_empty_user_agent_raises_configurationerror":    "TestNew",
			"test_find_episode_subtitles_from_complete_series_path":   "TestSearchEpisode",
			"test_list_and_download_subtitles_complete_series_pack":   "TestSearchEpisode",
			"test_subtitle_get_matches":                               "TestSearchMovie",
			"test_subtitle_get_matches_episode":                       "TestSearchEpisode",
			"test_list_subtitles_movie":                               "TestSearchMovie",
			"test_list_subtitles_inexistent_movie":                    "TestSearchMovie",
			"test_list_subtitles_episode":                             "TestSearchEpisode",
			"test_list_subtitles_inexistent_episode":                  "TestSearchEpisode",
			"test_download_subtitle":                                  "TestDownload",
			"test_download_subtitle_episode":                          "TestDownload",
			"test_get_episode_from_release":                           "TestEpisodeFromRelease",
			"test_get_episode_from_release_return_none":               "TestEpisodeFromRelease",
			"test_get_episode_from_release_w_empty_match_return_none": "TestEpisodeFromRelease",
			"test_complex_episode_name":                               "TestSearchEpisode",
		},
	})
}

func TestNew(t *testing.T) {
	for _, raw := range []string{``, `{"user_agent":" "}`, `{"user_agent":1}`} {
		if _, err := New(provider.Env{}, json.RawMessage(raw)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = %v", raw, err, provider.ConfigurationError)
		}
	}
	// Verify SSL off makes a client of its own.
	hc := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
	if got := insecure(hc); got == hc || !got.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Errorf("insecure got = %v, want = a client skipping verification", got)
	}
}

func TestEpisodeFromRelease(t *testing.T) {
	for _, c := range []struct {
		release         string
		season, episode int
		ok              bool
	}{
		{"Vinland Saga Season 2 - 05 [Crunchyroll][Crunchyroll] Vinland Saga Season 2 - 05", 2, 5, true},
		{"Vinland Saga Season 2 - Foo", 0, 0, false},
		{"Vinland Saga - 02", 0, 0, false},
	} {
		s, e, ok := episodeFromRelease(c.release)
		if s != c.season || e != c.episode || ok != c.ok {
			t.Errorf("episodeFromRelease(%q) got = %d, %d, %v, want = %d, %d, %v", c.release, s, e, ok, c.season, c.episode, c.ok)
		}
	}
}

func ids(subs []provider.Subtitle, base string) []string {
	var out []string
	for _, s := range subs {
		out = append(out, strings.TrimPrefix(s.ID, base)+" "+s.Language.Tag)
	}
	return out
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	for _, c := range []struct {
		name  string
		video *provider.Video
		langs []provider.Language
		want  []string
	}{
		{
			"dune", &provider.Video{Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"}},
			[]provider.Language{{Tag: "en"}, {Tag: "pt-BR"}, {Tag: "xx"}},
			[]string{"/subtitles/dune-2021/english/1 en", "/subtitles/dune-2021/brazillian-portuguese/1 pt-BR"},
		},
		{
			// A localized title falls back to the IMDb ID.
			"goonierne", &provider.Video{Title: "Goonierne", Year: 1985, ExternalIDs: map[string]string{"imdb": "tt0089218"}},
			[]provider.Language{{Tag: "en"}},
			[]string{"/subtitles/the-goonies/english/1 en"},
		},
		{
			// Another IMDb ID's page has none.
			"wrong imdb", &provider.Video{Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1"}},
			[]provider.Language{{Tag: "en"}},
			nil,
		},
		{"inexistent", &provider.Video{Title: "Inexistent Movie", Year: 2021}, []provider.Language{{Tag: "en"}}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), c.video, c.langs)
			if got := ids(subs, f.srv.URL); err != nil || !slices.Equal(got, c.want) {
				t.Fatalf("got = %v, %v, want = %v", got, err, c.want)
			}
			for _, s := range subs {
				if !slices.Equal(s.Matches, []string{"title", "year", "imdb_id"}) || len(s.ReleaseNames) == 0 || s.PageLink != s.ID {
					t.Errorf("%s: got = %+v, want = the movie's matches and releases", s.ID, s)
				}
			}
		})
	}
	subs, _ := p.Search(t.Context(), &provider.Video{Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"}}, []provider.Language{{Tag: "en"}})
	if want := []string{"Dune.2021.WEBRip.x264-ION10", "Dune.2021.1080p.HDRip.X264-EVO", "Synced for all"}; len(subs) != 1 || !slices.Equal(subs[0].ReleaseNames, want) {
		t.Errorf("release names got = %v, want = %v", subs, want)
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	matches := []string{"title", "series", "year", "season", "episode", "imdb_id"}
	for _, c := range []struct {
		name  string
		video *provider.Video
		want  []string
	}{
		{
			// The episode's subtitles and the season pack's.
			"breaking bad", &provider.Video{Episode: true, Series: "Breaking Bad", Season: 1, Number: 1, SeriesExternalIDs: map[string]string{"imdb": "tt0903747"}},
			[]string{"/subtitles/breaking-bad-first-season/english/161227 en", "/subtitles/breaking-bad-first-season/english/3 en"},
		},
		{
			// A complete series page.
			"courage", &provider.Video{Episode: true, Series: "Courage the Cowardly Dog", Season: 1, Number: 1, SeriesExternalIDs: map[string]string{"imdb": "tt0220880"}},
			[]string{"/subtitles/courage-the-cowardly-dog/english/1 en"},
		},
		{
			"sam & max", &provider.Video{Episode: true, Series: "Sam & Max: Freelance Police", Title: "The Glazed McGuffin Affair", Season: 1, Number: 21, SeriesExternalIDs: map[string]string{"imdb": "tt0125646"}},
			[]string{"/subtitles/sam-max-freelance-police/english/1 en"},
		},
		{
			"dr. romantic", &provider.Video{Episode: true, Series: "Dr. Romantic", Season: 3, Number: 16, Year: 2016, SeriesExternalIDs: map[string]string{"imdb": "tt6157190"}},
			[]string{"/subtitles/dr-romantic-third-season/english/1 en"},
		},
		{"inexistent", &provider.Video{Episode: true, Series: "121361asdfgh", Season: 1, Number: 1}, nil},
		{"season 21", &provider.Video{Episode: true, Series: "Breaking Bad", Season: 21, Number: 1}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), c.video, []provider.Language{{Tag: "en"}})
			if got := ids(subs, f.srv.URL); err != nil || !slices.Equal(got, c.want) {
				t.Fatalf("got = %v, %v, want = %v", got, err, c.want)
			}
			for _, s := range subs {
				if !slices.Equal(s.Matches, matches) {
					t.Errorf("%s: matches got = %v, want = %v", s.ID, s.Matches, matches)
				}
			}
			if len(subs) > 0 && c.video.Title != "" {
				file, err := p.Download(t.Context(), c.video, subs[0].ID)
				if err != nil || !strings.Contains(string(file.Data), c.video.Title) {
					t.Errorf("Download got = %q, %v, want = the episode's subtitle", file.Data, err)
				}
			}
		})
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	for _, c := range []struct {
		page string
		v    *provider.Video
		want string
		kind provider.Kind
	}{
		{"/subtitles/dune-2021/farsi_persian/2604701", &provider.Video{Title: "Dune"}, "Dune.2021.srt", 0},
		{"/subtitles/breaking-bad-first-season/english/161227", &provider.Video{Episode: true, Number: 7}, "Breaking.Bad.S01E07.srt", 0},
		{"/subtitles/nolink/english/1", nil, "", provider.APIThrottled},
		{"/subtitles/bad/english/1", nil, "", provider.APIThrottled},
		{"/subtitles/breaking-bad-first-season/english/161227", &provider.Video{Episode: true, Number: 9}, "", provider.BadSubtitle},
	} {
		file, err := p.Download(t.Context(), c.v, f.srv.URL+c.page)
		if c.kind != 0 {
			if provider.KindOf(err) != c.kind {
				t.Errorf("Download(%s) got = %v, want = %v", c.page, err, c.kind)
			}
			continue
		}
		if err != nil || !strings.Contains(string(file.Data), c.want) || strings.Contains(string(file.Data), "\r") {
			t.Errorf("Download(%s) got = %q, %v, want = %s", c.page, file.Data, err, c.want)
		}
	}
}

func TestStatus(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	// 404 and 503 are tried again, 3 times at most.
	f.statuses = []int{http.StatusServiceUnavailable, http.StatusNotFound}
	got, err := p.searchMovie(t.Context(), "Dune", 2021)
	if err != nil || !slices.Contains(got, "/subtitles/dune-2021") {
		t.Errorf("after 503 and 404 got = %v, %v, want = the results", got, err)
	}
	f.statuses = []int{http.StatusForbidden}
	if got, err := p.searchMovie(t.Context(), "Dune", 2021); err != nil || len(got) != 0 {
		t.Errorf("403 got = %v, %v, want = none", got, err)
	}
	for _, c := range []struct {
		status int
		kind   provider.Kind
	}{{http.StatusTooManyRequests, provider.TooManyRequests}, {http.StatusBadGateway, provider.ServiceUnavailable}} {
		f.statuses = []int{c.status}
		if _, err := p.searchMovie(t.Context(), "Dune", 2021); provider.KindOf(err) != c.kind {
			t.Errorf("%d got = %v, want = %v", c.status, err, c.kind)
		}
	}
}
