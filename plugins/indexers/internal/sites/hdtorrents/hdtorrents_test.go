package hdtorrents

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
	return &indexer.Indexer{ID: "1", Name: "HD-Torrents", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{Username: "user", Password: "pass"}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{
			"RSS",
			indexer.Query{},
			"https://hdts.ru/torrents.php?search=&active=0&options=0",
		},
		{
			"term with points and parentheses",
			indexer.Query{Term: "Some.Film (2020)", Categories: []int{2045}},
			"https://hdts.ru/torrents.php?category[]=71&category[]=64&search=Some%20Film+%282020%29&active=0&options=0",
		},
		{
			"TV season",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2), Categories: []int{5045}},
			"https://hdts.ru/torrents.php?category[]=72&category[]=73&category[]=65&search=The+Show+S02&active=0&options=0",
		},
		{
			"movie by IMDb ID, term as is",
			indexer.Query{Type: indexer.SearchMovie, Term: "Film: Part 2", IMDbID: "76759"},
			"https://hdts.ru/torrents.php?search=tt0076759+Film%3A+Part+2&active=0&options=0",
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

// page is a search page as HD-Torrents shows it to a moderator, with a row
// having the uploader's column and the moderators' links and one having
// neither.
const page = `<html><body>
<table class="navus"><tr><td>Welcome user</td><td>Rank: Power User</td></tr></table>
<table class="mainblockcontenttt">
<tr><td class="mainblockcontenthead">Type</td><td class="mainblockcontenthead">x</td></tr>
<tr><td class="mainblockcontent">Header</td></tr>
<tr>
<td class="mainblockcontent"><a href="torrents.php?category=30"><img src="images/categories/tv1080.png"></a></td>
<td class="mainblockcontent"><img src="images/internal.png"></td>
<td class="mainblockcontent"><a href="details.php?id=abc123&amp;hit=1" onmouseover="return overlib('&lt;img src=\'./images\posters/abc.jpg\'&gt;')">The.Show.S02E01.1080p.BluRay.x264-GRP</a><br><span> Genre: Drama </span> <img src="images/50.png"></td>
<td class="mainblockcontent"><a href="https://www.imdb.com/title/tt1234567/">IMDb</a></td>
<td class="mainblockcontent"><a href="download.php?id=abc123&amp;f=The.Show.torrent"><img src="images/download.png"></a></td>
<td class="mainblockcontent">0</td>
<td class="mainblockcontent"><span 06 oct 2023 12:34:56>Oct 06</span></td>
<td class="mainblockcontent">4.37 GB</td>
<td class="mainblockcontent">uploader</td>
<td class="mainblockcontent">12</td>
<td class="mainblockcontent">3</td>
<td class="mainblockcontent">150</td>
<td class="mainblockcontent">Edit</td>
<td class="mainblockcontent">Delete</td>
<td class="mainblockcontent">Recommend</td>
<td class="mainblockcontent">Like</td>
</tr>
<tr>
<td class="mainblockcontent"><a href="torrents.php?category=64"><img></a></td>
<td class="mainblockcontent"></td>
<td class="mainblockcontent"><a href="details.php?id=def456" onmouseover="">A.Film.2020.2160p.UHD.BluRay.x265-GRP</a> <img src="images/no_ratio.png"></td>
<td class="mainblockcontent"></td>
<td class="mainblockcontent"><a href="download.php?id=def456"><img></a></td>
<td class="mainblockcontent">1</td>
<td class="mainblockcontent"><span 01 jan 2024 00:00:01></span></td>
<td class="mainblockcontent">20 GB</td>
<td class="mainblockcontent">--</td>
<td class="mainblockcontent">1</td>
<td class="mainblockcontent">7</td>
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
		{"Title", r.Title, "The.Show.S02E01.1080p.BluRay.x264-GRP"},
		{"Description", r.Description, "Genre: Drama"},
		{"GUID", r.GUID, "https://hdts.ru/details.php?id=abc123&hit=1"},
		{"InfoURL", r.InfoURL, "https://hdts.ru/details.php?id=abc123&hit=1"},
		{"DownloadURL", r.DownloadURL, "https://hdts.ru/download.php?id=abc123&f=The.Show.torrent"},
		{"PosterURL", r.PosterURL, "https://hdts.ru/images/posters/abc.jpg"},
		{"PublishDate", r.PublishDate, time.Date(2023, 10, 6, 12, 34, 56, 0, time.Local)},
		{"Categories", r.Categories, []int{5040, 100030}},
		{"IMDbID", r.IMDbID, 1234567},
		{"Size", *r.Size, int64(4692251770)},
		{"Seeders", *r.Seeders, 12},
		{"Peers", *r.Peers, 15},
		{"Grabs", *r.Grabs, 150},
		{"Flags", r.Flags, []indexer.Flag{indexer.FlagInternal, indexer.FlagHalfLeech}},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.5},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
		{"second PosterURL", r2.PosterURL, ""},
		{"second Description", r2.Description, ""},
		{"second Categories", r2.Categories, []int{2045, 100064}},
		{"second Seeders", *r2.Seeders, 0},
		{"second Peers", *r2.Peers, 1},
		{"second Grabs", *r2.Grabs, 7},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 0.0},
		{"second UploadVolumeFactor", *r2.UploadVolumeFactor, 0.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseFreeleechRank(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(strings.Replace(page, "Rank: Power User", "Rank: vip ", 1)))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got := *releases[0].DownloadVolumeFactor; got != 0 {
		t.Errorf("DownloadVolumeFactor = %v, want = 0", got)
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
		{"success", `<html><body>If your browser doesn't have javascript enabled, click <a href="index.php">here</a></body></html>`, ""},
		{"site's error", `<html><body><div><font color="#FF0000"> Error: Username or password incorrect! </font></div></body></html>`, "Error: Username or password incorrect!"},
		{"unknown error", `<html><body>Login</body></html>`, "Couldn't login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { synctest.Test(t, func(t *testing.T) { testLogin(t, tt.body, tt.want) }) })
	}
}

// testLogin searches with a login answered with body, wanting an error
// containing want, or success when want is "".
func testLogin(t *testing.T, body, want string) {
	var searched int
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		if r.URL.Path == "/login.php" {
			form, _ := io.ReadAll(r.Body)
			if got, want := string(form), "uid=user&pwd=pass"; got != want {
				t.Errorf("login form = %q, want = %q", got, want)
			}
			if got, want := r.Header.Get("Referer"), "https://hdts.ru/login.php"; got != want {
				t.Errorf("Referer = %q, want = %q", got, want)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Set-Cookie": {"uid=42; path=/", "pass=secret; path=/"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}
		}
		searched++
		if searched == 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("Error:You're not authorized"))}
		}
		if c, err := r.Cookie("pass"); err != nil || c.Value != "secret" {
			t.Errorf("cookie pass = %v, want = secret", c)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if want == "" {
		if err != nil || len(releases) != 2 {
			t.Errorf("Search() = %d releases, %v, want = 2 releases", len(releases), err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) || !errors.Is(err, indexer.ErrAuth) {
		t.Errorf("err = %v, want = an auth error containing %q", err, want)
	}
}
