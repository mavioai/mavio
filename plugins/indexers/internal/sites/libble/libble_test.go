package libble

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
	return &indexer.Indexer{ID: "1", Name: "Libble", Site: site, Session: c.Session(nil, nil), Statuses: st}
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
			"https://libble.me/torrents.php?action=advanced&order_by=time&order_way=desc",
		},
		{
			"term, category and page",
			indexer.Query{Term: "Some Artist", Categories: []int{3030}, Limit: 50, Offset: 100},
			"https://libble.me/torrents.php?action=advanced&order_by=time&order_way=desc&searchstr=Some+Artist&page=3",
		},
		{
			"categories",
			indexer.Query{Categories: []int{3000}},
			"https://libble.me/torrents.php?action=advanced&order_by=time&order_way=desc&filter_cat[1]=1&filter_cat[2]=1&filter_cat[7]=1",
		},
		{
			"music",
			indexer.Query{
				Type: indexer.SearchMusic, Artist: "The Band", Album: "Greatest Hits 2020", Label: "Label & Co",
				Year: 2020, Genre: "rock", Categories: []int{3020},
			},
			"https://libble.me/torrents.php?artistname=The+Band&groupname=Greatest+Hits&recordlabel=Label+%26+Co&year=2020&taglist=rock&tags_type=0&action=advanced&order_by=time&order_way=desc&filter_cat[7]=1",
		},
		{
			"various artists",
			indexer.Query{Type: indexer.SearchMusic, Artist: "VA", Album: "1999"},
			"https://libble.me/torrents.php?groupname=1999&action=advanced&order_by=time&order_way=desc",
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
	for _, typ := range []indexer.SearchType{indexer.SearchMovie, indexer.SearchTV, indexer.SearchBook} {
		ix := newIndexer(t, testSettings, nil)
		c, err := ix.Site.SearchRequests(context.Background(), ix, &indexer.Query{Type: typ, Term: "x"})
		if err != nil || c.First() != nil {
			t.Errorf("%s SearchRequests() = %v, %v, want = no requests", typ, c.First(), err)
		}
	}
}

// page is a search page as Libble shows it: groups of torrents, each
// followed by its torrents' rows.
const page = `<html><body><a href="logout.php?auth=x">Logout</a>
<table id="torrent_table">
<tr class="colhead"><td></td><td>Name</td></tr>
<tr class="group">
<td class="center"></td>
<td class="cats_col"><div title="Music" class="tooltip cat_icon cats_music"></div></td>
<td><img class="thumbnail" title=" https://libble.me/covers/1.jpg "></td>
<td><strong><a href="artist.php?id=5">The Band</a> &amp; <a href="artist.php?id=6">Guest</a> - <a href="torrents.php?id=100">Greatest Hits</a> [2020]</strong>
<div class="tags">rock, pop, </div></td>
</tr>
<tr class="group_torrent groupid_100">
<td><a href="torrents.php?action=download&amp;id=1001&amp;authkey=a&amp;torrent_pass=b">DL</a> <a href="torrents.php?id=100&amp;torrentid=1001">FLAC / Lossless / CD / <strong>Freeleech!</strong></a></td>
<td>12</td>
<td><span class="time" title="Jan 05 2024, 14:22">1 hour ago</span></td>
<td>412.5 MB</td>
<td>30</td>
<td>20</td>
<td>2</td>
</tr>
<tr class="group_torrent groupid_100">
<td><a href="torrents.php?action=download&amp;id=1002">DL</a> <a href="torrents.php?id=100&amp;torrentid=1002">MP3 / 320 / WEB</a></td>
<td>12</td>
<td><span class="time">2 days ago</span></td>
<td>120 MB</td>
<td>5</td>
<td>4</td>
<td>0</td>
</tr>
<tr class="group">
<td class="center"></td>
<td class="cats_col"><div class="cat_icon cats_musicvideos"></div></td>
<td></td>
<td><strong><a href="torrents.php?id=200">Live Concert</a> [2019]</strong></td>
</tr>
<tr class="group_torrent groupid_200">
<td><a href="torrents.php?action=download&amp;id=2001">DL</a> <a href="torrents.php?id=200&amp;torrentid=2001">MKV / 1080p / <strong>Neutral!</strong></a></td>
<td>1</td>
<td><span class="time" title="Dec 31 2023, 23:59">x</span></td>
<td>4 GB</td>
<td>1</td>
<td>1</td>
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
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	ix.Now = func() time.Time { return now }
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 3 {
		t.Fatalf("releases = %d, want = 3", len(releases))
	}
	r, r2, r3 := releases[0], releases[1], releases[2]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "The Band, Guest - Greatest Hits [2020] FLAC / Lossless / CD"},
		{"Artist", r.Artist, "The Band, Guest"},
		{"Album", r.Album, "Greatest Hits"},
		{"GUID", r.GUID, "https://libble.me/torrents.php?id=100&torrentid=1001"},
		{"InfoURL", r.InfoURL, "https://libble.me/torrents.php?id=100&torrentid=1001"},
		{"DownloadURL", r.DownloadURL, "https://libble.me/torrents.php?action=download&id=1001&authkey=a&torrent_pass=b"},
		{"Categories", r.Categories, []int{3000}},
		{"Description", r.Description, "rock, pop,"},
		{"Genres", r.Genres, []string{"rock", "pop"}},
		{"PosterURL", r.PosterURL, "https://libble.me/covers/1.jpg"},
		{"Size", *r.Size, int64(432537600)},
		{"Files", *r.Files, 12},
		{"Grabs", *r.Grabs, 30},
		{"Seeders", *r.Seeders, 20},
		{"Peers", *r.Peers, 22},
		{"PublishDate", r.PublishDate, time.Date(2024, 1, 5, 14, 22, 0, 0, time.UTC)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(259200)},
		{"second Title", r2.Title, "The Band, Guest - Greatest Hits [2020] MP3 / 320 / WEB"},
		{"second PublishDate", r2.PublishDate, now.Add(-48 * time.Hour)},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
		{"third Title", r3.Title, "Various Artists - Live Concert [2019] MKV / 1080p"},
		{"third Categories", r3.Categories, []int{3020}},
		{"third PosterURL", r3.PosterURL, ""},
		{"third DownloadVolumeFactor", *r3.DownloadVolumeFactor, 0.0},
		{"third UploadVolumeFactor", *r3.UploadVolumeFactor, 0.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
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
		want       string
	}{
		{"success", page, ""},
		{"site's warning", `<html><body><form id="loginform"><span class="warning"> Your username or password was incorrect. </span></form></body></html>`, "Your username or password was incorrect."},
		{"unknown error", `<html><body><form id="loginform"></form></body></html>`, "Unknown error message, please report."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testLogin(t, tt.body, tt.want) })
		})
	}
}

// testLogin searches with a login answered with body, wanting an error
// containing want, or success when want is "".
func testLogin(t *testing.T, body, want string) {
	s := testSettings
	s.TwoFactorAuthCode = "123456"
	var searched int
	ix := newIndexer(t, s, func(r *http.Request) *http.Response {
		if r.URL.Path == "/login.php" {
			form, _ := io.ReadAll(r.Body)
			if got, want := string(form), "username=user&password=pass&code=123456&keeplogged=1&login=Login"; got != want {
				t.Errorf("login form = %q, want = %q", got, want)
			}
			if got, want := r.Header.Get("Referer"), "https://libble.me/login.php"; got != want {
				t.Errorf("Referer = %q, want = %q", got, want)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Set-Cookie": {"session=secret; path=/"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}
		}
		searched++
		if searched == 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<html>Login</html>"))}
		}
		if c, err := r.Cookie("session"); err != nil || c.Value != "secret" {
			t.Errorf("cookie session = %v, want = secret", c)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if want == "" {
		if err != nil || len(releases) != 3 {
			t.Errorf("Search() = %d releases, %v, want = 3 releases", len(releases), err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) || !errors.Is(err, indexer.ErrAuth) {
		t.Errorf("err = %v, want = an auth error containing %q", err, want)
	}
}
