package immortalseed

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	return &indexer.Indexer{ID: "1", Name: "ImmortalSeed", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "pass"}}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{
			"RSS",
			indexer.Query{},
			"https://immortalseed.me/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc",
		},
		{
			"term with separators",
			indexer.Query{Term: "Some.Film_2020 - Part (2)", Categories: []int{2040}},
			"https://immortalseed.me/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc&do=search&keywords=Some+Film+2020+Part+2&search_type=t_name&selectedcats2=16",
		},
		{
			"TV episode",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2", Categories: []int{5045}},
			"https://immortalseed.me/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc&do=search&keywords=The+Show+S01E02&search_type=t_name&selectedcats2=64%2C63",
		},
		{
			"movie",
			indexer.Query{Type: indexer.SearchMovie, Term: "A Film"},
			"https://immortalseed.me/browse.php?category=0&include_dead_torrents=yes&sort=added&order=desc&do=search&keywords=A+Film&search_type=t_name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

// page is a search page as ImmortalSeed shows it.
const page = `<html><body><a href="logout.php">Logout</a>
<table id="sortabletable">
<tr><td>Type</td><td>Name</td><td>DL</td><td>Comments</td><td>Size</td><td>Snatched</td><td>Seeders</td><td>Leechers</td></tr>
<tr>
<td><a href="browse.php?category=8"><img alt="TV HD"></a></td>
<td><div><a href="https://immortalseed.me/details.php?id=1001">The.Show.S01E02.1080p...</a></div>
<div class="tooltip-content"><div> The.Show.S01E02.1080p.WEB.h264-GRP </div><div>Drama | Comedy</div></div>
<img title="Free Torrent"><img title="x2 Torrent">
<div>Uploaded 2023-05-01 03:04 pm by someone</div></td>
<td><a href="https://immortalseed.me/download.php?id=1001&amp;name=x.torrent"><img></a></td>
<td>0</td>
<td>1.5 GB</td>
<td>1,234</td>
<td>87</td>
<td> 4 </td>
</tr>
<tr>
<td><a href="browse.php?category=16"><img alt="Movies HD"></a></td>
<td><div><a href="https://immortalseed.me/details.php?id=1002">A.Film.2020.1080p.BluRay.x264-GRP</a></div>
<img title="Silver Torrent">
<div>2023-04-30 11:59 AM</div></td>
<td><a href="https://immortalseed.me/download.php?id=1002"><img></a></td>
<td>2</td>
<td>8.5 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
</table></body></html>`

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(page))
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
		{"Title", r.Title, "The.Show.S01E02.1080p.WEB.h264-GRP"},
		{"Description", r.Description, "Drama,Comedy"},
		{"GUID", r.GUID, "https://immortalseed.me/details.php?id=1001"},
		{"InfoURL", r.InfoURL, "https://immortalseed.me/details.php?id=1001"},
		{"DownloadURL", r.DownloadURL, "https://immortalseed.me/download.php?id=1001&name=x.torrent"},
		{"Categories", r.Categories, []int{5040, 100008}},
		{"Size", *r.Size, int64(1610612736)},
		{"Grabs", *r.Grabs, 1234},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"PublishDate", r.PublishDate, time.Date(2023, 5, 1, 15, 4, 0, 0, time.Local)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 2.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(86400)},
		{"second Title", r2.Title, "A.Film.2020.1080p.BluRay.x264-GRP"},
		{"second Description", r2.Description, ""},
		{"second PublishDate", r2.PublishDate, time.Date(2023, 4, 30, 11, 59, 0, 0, time.Local)},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 0.5},
		{"second UploadVolumeFactor", *r2.UploadVolumeFactor, 1.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestFreeleechOnly(t *testing.T) {
	s := testSettings
	s.FreeleechOnly = true
	ix := newIndexer(t, s, respond(page))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].GUID != "https://immortalseed.me/details.php?id=1001" {
		t.Errorf("releases = %d, want = only the freeleech one", len(releases))
	}
}

func equal(a, b any) bool {
	if x, ok := a.(time.Time); ok {
		return x.Equal(b.(time.Time))
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name, body string
		ok         bool
	}{
		{"success", page, true},
		{"failure", `<html><body><form action="takelogin.php"></form></body></html>`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testLogin(t, tt.body, tt.ok) })
		})
	}
}

// testLogin searches with a login answered with body, wanting success or
// an authentication failure.
func testLogin(t *testing.T, body string, ok bool) {
	var searched int
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		if r.URL.Path == "/takelogin.php" {
			form, _ := io.ReadAll(r.Body)
			if got, want := string(form), "username=user&password=pass"; got != want {
				t.Errorf("login form = %q, want = %q", got, want)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Set-Cookie": {"uid=42; path=/", "pass=secret; path=/"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}
		}
		searched++
		if searched == 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<html>Login</html>"))}
		}
		if c, err := r.Cookie("pass"); err != nil || c.Value != "secret" {
			t.Errorf("cookie pass = %v, want = secret", c)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if ok {
		if err != nil || len(releases) != 2 {
			t.Errorf("Search() = %d releases, %v, want = 2 releases", len(releases), err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "ImmortalSeed Auth Failed") || !errors.Is(err, indexer.ErrAuth) {
		t.Errorf("err = %v, want = ImmortalSeed Auth Failed", err)
	}
}
