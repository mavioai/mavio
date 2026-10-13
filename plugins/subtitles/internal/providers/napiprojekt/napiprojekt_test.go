package napiprojekt

import (
	"encoding/json"
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
	ported.Run(t, "napiprojekt.json", ported.Cases{Facts: map[string]string{
		"test_list_subtitles_episode": "TestCassetteEpisode",
		"test_list_subtitles_movie":   "TestCassetteMovie",
	}})
}

func cassetteProvider(t *testing.T, name string) *Provider {
	t.Helper()
	env := provider.Env{
		HTTP: &http.Client{Transport: ported.Cassette(t, "test_napiprojekt/"+name)},
		Now:  time.Now, Log: slog.New(slog.DiscardHandler),
	}
	p, err := New(env, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func TestCassetteEpisode(t *testing.T) {
	v := &provider.Video{
		Episode: true, Series: "Attack on Titan", Season: 2, Number: 1, Source: "Web",
		ReleaseName:       "Attack on Titan - S02E01 - Beast Titan Bluray-1080p",
		SeriesExternalIDs: map[string]string{"imdb": "tt2560140"},
		ExternalIDs:       map[string]string{HashID: "fe93bb3a7743c39e12c8d7c4a864dff1"},
	}
	got, err := cassetteProvider(t, "test_list_subtitles_episode.yaml").Search(t.Context(), v, []provider.Language{{Tag: "pl"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got = %d subtitles, want = 3", len(got))
	}
	if got[0].ID != "fe93bb3a7743c39e12c8d7c4a864dff1" || !slices.Equal(got[0].Matches, []string{"hash"}) {
		t.Errorf("hash subtitle got = %+v", got[0])
	}
	want := provider.Subtitle{
		ID: "fe93bb3a7743c39e12c8d7c4a864dff1", Language: provider.Language{Tag: "pl"},
		ReleaseNames: []string{authorLabel + ": Brak | 1280x720 | 23.976 | 247.6 MiB | 2017-09-24 | 00:24:11.560"},
		Matches:      []string{"series", "year", "series_imdb_id", "season", "episode", "hash"},
		Uploader:     "Brak",
	}
	if !equal(got[1], want) {
		t.Errorf("scraped subtitle got = %+v, want = %+v", got[1], want)
	}
	if got[2].ID != "e791ae70db9fcff44c0cb6cff7e74db4" || slices.Contains(got[2].Matches, "hash") {
		t.Errorf("other scraped subtitle got = %+v", got[2])
	}
}

func TestCassetteMovie(t *testing.T) {
	v := &provider.Video{
		Title: "Shrek", Year: 2001, ReleaseName: "Shrek",
		ExternalIDs: map[string]string{"imdb": "tt0126029", HashID: "444563eef63f83d47cabb888f7a45113"},
	}
	got, err := cassetteProvider(t, "test_list_subtitles_movie.yaml").Search(t.Context(), v, []provider.Language{{Tag: "pl"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 28 {
		t.Fatalf("got = %d subtitles, want = 28", len(got))
	}
	for _, s := range got[1:] {
		if !slices.Equal(s.Matches[:3], []string{"title", "year", "imdb_id"}) || len(s.ID) != 32 {
			t.Errorf("scraped subtitle got = %+v, want = one matching the IMDb ID", s)
		}
	}
}

func equal(a, b provider.Subtitle) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestSubhash(t *testing.T) {
	for hash, want := range map[string]string{
		"444563eef63f83d47cabb888f7a45113": "a6f09",
		"fe93bb3a7743c39e12c8d7c4a864dff1": "8410a",
	} {
		if got, ok := subhash(hash); !ok || got != want {
			t.Errorf("subhash(%s) got = %q, want = %q", hash, got, want)
		}
	}
	for _, hash := range []string{"", "xyz", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		if _, ok := subhash(hash); ok {
			t.Errorf("subhash(%q) got = ok, want = not", hash)
		}
	}
}

const catalogue = `<div class="greyBoxCatcher">
<a class="movieTitleCat" href="napisy-1-Other-(2001)"><h3>Other</h3></a>
<a href="https://www.imdb.com/title/tt0000001/">imdb</a>
</div>
<div class="greyBoxCatcher">
<a class="movieTitleCat" href="napisy-2-Film-(2020)"><h3>Film</h3></a>
<a href="https://www.imdb.com/title/tt0000002/?ref_=x">imdb</a>
</div>`

var page = strings.ReplaceAll(`<table>
<tr title="<b>{author}:</b> Brak (dodał: x)<br><b>Video FPS:</b> 25<br><b>Video rozdzielczość:</b> 1920x1080<br>">
<td><p class="blue"> <a class="tableA" href="napiprojekt:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa">a.mkv</a></p></td>
<td><p>1 GiB</p></td><td><p>25</p></td><td><p>01:30:00</p></td><td><p>Brak</p></td><td><p>2020-01-01</p></td></tr>
<tr title="<b>{author}:</b> Jan Kowalski (dodał: y)<br>">
<td><p class="blue"> <a class="tableA" href="napiprojekt:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb">b.mkv</a></p></td>
<td><p>2 GiB</p></td><td><p>24</p></td><td><p>01:31:00</p></td><td><p></p></td><td><p>2020-02-02</p></td></tr>
<tr title="<b>{author}:</b> anon (dodał: z)<br>">
<td><p class="blue"> <a class="tableA" href="napiprojekt:cccccccccccccccccccccccccccccccc">c.mkv</a></p></td>
<td><p>3 GiB</p></td><td><p>24</p></td><td><p>01:32:00</p></td><td><p>anon</p></td><td><p>2020-03-03</p></td></tr>
<tr><td><a class="tableA" href="napiprojekt:dddddddddddddddddddddddddddddddd">untitled row</a></td></tr>
</table>`, "{author}", authorLabel)

// newServer is NapiProjekt: dl.php has a subtitle for hash aaa…, none for
// others.
func newServer(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/unit_napisy/dl.php":
			if r.URL.Query().Get("f") == "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
				_, _ = w.Write([]byte("{1}{56}Napisy\r\n"))
				return
			}
			_, _ = w.Write([]byte("NPc0"))
		case "/ajax/search_catalog.php":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(pages[r.PostForm.Encode()]))
		default:
			body, ok := pages[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(t *testing.T, srv *httptest.Server, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	movieForm := url.Values{"queryString": {"Film"}, "queryKind": {"2"}, "queryYear": {"2020"}, "associate": {""}}.Encode()
	episodeForm := url.Values{"queryString": {"Film"}, "queryKind": {"1"}, "queryYear": {""}, "associate": {""}}.Encode()
	pages := map[string]string{
		movieForm:                        catalogue,
		episodeForm:                      `<a class="movieTitleCat" href="napisy-3-Film"><h3>Film</h3></a>`,
		"/napisy1,7,0-dla-2-Film-(2020)": page,
		"/napisy1,7,0-dla-3-Film-s01e02": page,
	}
	movie := &provider.Video{Title: "Film", Year: 2020, ExternalIDs: map[string]string{"imdb": "tt0000002", HashID: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}}
	episode := &provider.Video{Episode: true, Series: "Film", Season: 1, Number: 2}
	pl := []provider.Language{{Tag: "pl"}}
	cases := []struct {
		name     string
		settings string
		v        *provider.Video
		wantIDs  []string
		wantM    []string
	}{
		{
			name: "movie", v: movie,
			wantIDs: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc"},
			wantM:   []string{"title", "year", "imdb_id"},
		},
		{name: "hash only", settings: `{"hash_only":true,"only_authors":true}`, v: movie, wantIDs: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
		{
			name: "only authors", settings: `{"only_authors":true}`, v: movie,
			wantIDs: []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc"},
		},
		{name: "only real names", settings: `{"only_real_names":true}`, v: movie, wantIDs: []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		{
			name: "episode without hash or IMDb ID", v: episode,
			wantIDs: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc"},
			wantM:   []string{"series", "season", "episode"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := newProvider(t, newServer(t, pages), c.settings).Search(t.Context(), c.v, pl)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, s := range got {
				ids = append(ids, s.ID)
			}
			if !slices.Equal(ids, c.wantIDs) {
				t.Fatalf("IDs got = %v, want = %v", ids, c.wantIDs)
			}
			if c.wantM != nil && !slices.Equal(got[len(got)-1].Matches, c.wantM) {
				t.Errorf("matches got = %v, want = %v", got[len(got)-1].Matches, c.wantM)
			}
		})
	}
	got, _ := newProvider(t, newServer(t, pages), "").Search(t.Context(), movie, pl)
	if want := authorLabel + ": Jan Kowalski |  |  | 2 GiB | 2020-02-02 | 01:31:00"; got[2].ReleaseNames[0] != want || got[2].Uploader != "Jan Kowalski" {
		t.Errorf("release got = %q by %q, want = %q", got[2].ReleaseNames[0], got[2].Uploader, want)
	}
	if want := authorLabel + ": Brak | 1920x1080 | 25 | 1 GiB | 2020-01-01 | 01:30:00"; got[1].ReleaseNames[0] != want || !slices.Contains(got[1].Matches, "hash") {
		t.Errorf("release got = %q %v, want = %q matching the hash", got[1].ReleaseNames[0], got[1].Matches, want)
	}
}

func TestSearchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	_, err := newProvider(t, srv, "").Search(t.Context(), &provider.Video{Title: "Film"}, []provider.Language{{Tag: "pl"}})
	if provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("Search got = %v, want = %v", err, provider.TooManyRequests)
	}
}

func TestDownload(t *testing.T) {
	p := newProvider(t, newServer(t, nil), "")
	f, err := p.Download(t.Context(), nil, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "{1}{56}Napisy\r\n" || f.Format != "sub" || f.Language.Tag != "pl" {
		t.Errorf("Download got = %q %q, want = the MicroDVD subtitle", f.Data, f.Format)
	}
	for _, id := range []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "bad"} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != provider.BadSubtitle {
			t.Errorf("Download(%s) got = %v, want = %v", id, err, provider.BadSubtitle)
		}
	}
}
