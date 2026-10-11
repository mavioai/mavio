package xspeeds

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

type handler func(r *http.Request) *http.Response

func (h handler) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := h(r)
	resp.Request = r
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp, nil
}

func newIndexer(t *testing.T, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := Definition.New(data)
	if err != nil {
		t.Fatal(err)
	}
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = h
	return &indexer.Indexer{ID: "1", Name: "XSpeeds", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "p&ss"}}

func page(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{"recent", indexer.Query{}, "https://www.xspeeds.eu/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc"},
		{
			"episode in one category",
			indexer.Query{Type: indexer.SearchTV, Term: "The.Show", Season: indexer.Ptr(2), Episode: "3", Categories: []int{indexer.TVSD.ID}},
			"https://www.xspeeds.eu/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc&do=search&keywords=The+Show+S02E03&search_type=t_name",
		},
		{
			"movie in one category",
			indexer.Query{Type: indexer.SearchMovie, Term: "A_Film - 2020", Categories: []int{indexer.MoviesBluRay.ID}},
			"https://www.xspeeds.eu/browse.php?category=109&include_dead_torrents=yes&sort=added&order=desc&do=search&keywords=A+Film+2020&search_type=t_name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if r := c.First(); r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
		})
	}
}

// results is a search page as XSpeeds shows it.
const results = `<html><body>
<a href="logout.php">Logout</a>
<table id="sortabletable"><tbody>
<tr><td>Type</td><td>Name</td></tr>
<tr>
<td><a href="browse.php?category=129"><img alt="TV HD"></a></td>
<td><div><a href="https://www.xspeeds.eu/details.php?id=55">The.Show.S02E03.720p.HDTV.x264-GRP</a></div>
<div class="tooltip-content"><div>x</div><div> Drama | Comedy </div></div>
<img title="Free Torrent (FL)"><img title="x2 Torrent">
<div>Added 25-04-2015 23:38 by someone</div></td>
<td><a href="https://www.xspeeds.eu/download.php?torrent=55">DL</a></td>
<td></td>
<td>1.5 GB</td>
<td>1,204</td>
<td>87</td>
<td> 4 </td>
</tr>
<tr>
<td><a href="browse.php?category=139"><img alt="TOTM"></a></td>
<td><div><a href="https://www.xspeeds.eu/details.php?id=56">A.Film.2020.1080p.BluRay.x264-GRP</a></div>
<img title="Silver Torrent">
<div>no date</div></td>
<td><a href="https://www.xspeeds.eu/download.php?torrent=56">DL</a></td>
<td></td>
<td>8 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
</tbody></table></body></html>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(results) })
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r, r2 := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "The.Show.S02E03.720p.HDTV.x264-GRP"},
		{"GUID", r.GUID, "https://www.xspeeds.eu/details.php?id=55"},
		{"DownloadURL", r.DownloadURL, "https://www.xspeeds.eu/download.php?torrent=55"},
		{"Description", r.Description, "Drama,Comedy"},
		{"PublishDate", r.PublishDate, time.Date(2015, 4, 25, 23, 38, 0, 0, time.Local)},
		{"Size", *r.Size, int64(1610612736)},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"has TV/HD", slices.Contains(r.Categories, indexer.TVHD.ID), true},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 2.0},
		{"MinimumRatio", *r.MinimumRatio, 0.8},
		{"has TV and Movies", slices.Contains(r2.Categories, indexer.TV.ID) && slices.Contains(r2.Categories, indexer.Movies.ID), true},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 0.5},
		{"second UploadVolumeFactor", *r2.UploadVolumeFactor, 1.0},
		{"second PublishDate", r2.PublishDate.IsZero(), true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseFreeleechOnly(t *testing.T) {
	s := testSettings
	s.FreeleechOnly = true
	ix := newIndexer(t, s, func(*http.Request) *http.Response { return page(results) })
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].GUID != "https://www.xspeeds.eu/details.php?id=55" {
		t.Errorf("releases = %v, want = the freeleech one", releases)
	}
}

func TestLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/login.php":
				resp := page(`<form action="takelogin.php"></form>`)
				resp.Header.Set("Set-Cookie", "PHPSESSID=s1; path=/")
				return resp
			case "/takelogin.php":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), "username=user&password=p%26ss"; got != want {
					t.Errorf("login form = %q, want = %q", got, want)
				}
				if got := r.Header.Get("Referer"); got != "https://www.xspeeds.eu/login.php" {
					t.Errorf("Referer = %q", got)
				}
				if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "s1" {
					t.Errorf("login without the login page's cookie")
				}
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"/index.php"}, "Set-Cookie": {"uid=1; path=/", "pass=abc; path=/"}},
					Body:       http.NoBody,
				}
			case "/index.php":
				return page(`<a href="logout.php">Logout</a>`)
			case "/browse.php":
				if c, err := r.Cookie("pass"); err != nil || c.Value != "abc" {
					return page(`<form action="takelogin.php"></form>`)
				}
				return page(results)
			}
			t.Errorf("unexpected request %s", r.URL)
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if logins != 1 || len(releases) != 2 {
			t.Errorf("logins, releases = %d, %d, want = 1, 2", logins, len(releases))
		}
		if got := ix.Cookies(); got["PHPSESSID"] != "s1" || got["uid"] != "1" {
			t.Errorf("cookies = %v, want = the login page's and the login's", got)
		}
	})
}

func TestLoginFailure(t *testing.T) {
	tests := []struct{ name, page, want string }{
		{"table", "<div class=\"left_side\"><table><tr><td>Error</td></tr><tr><td> Wrong\n\tpassword </td></tr></table></div>", "Wrong password"},
		{"notification", `<div class="left_side"><table><tr><td>Error</td></tr><tr><td> </td></tr></table></div><div class="notification-body"> Banned </div>`, "Banned"},
		{"unknown", `<p>Error</p>`, "Unknown error message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
					if r.URL.Path == "/takelogin.php" {
						return page(tt.page)
					}
					return page(`<form></form>`)
				})
				_, err := ix.Search(context.Background(), &indexer.Query{})
				if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), tt.want) {
					t.Errorf("err = %v, want = an authentication error containing %q", err, tt.want)
				}
			})
		})
	}
}
