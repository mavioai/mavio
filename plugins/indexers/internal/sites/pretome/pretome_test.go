package pretome

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
	return &indexer.Indexer{ID: "1", Name: "PreToMe", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{
	UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "péss"},
	Pin:              "1234",
}

func page(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{
			"recent",
			indexer.Query{},
			"https://pretome.info/browse.php?st=1&search=&tags=&tf=all",
		},
		{
			"movie by IMDb ID, too many tags",
			indexer.Query{Type: indexer.SearchMovie, Term: "Ignored", IMDbID: "76759", Categories: []int{2000}},
			"https://pretome.info/browse.php?st=1&search=tt0076759&sd=1&cat[]=19",
		},
		{
			"episode with tags",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2), Episode: "1", Categories: []int{5040}},
			"https://pretome.info/browse.php?st=1&search=The+Show+S02E01&cat[]=7&tags=x264%2C720p%2CHDTV%2CBluRay%2C1080p%2Cx265&tf=any",
		},
		{
			"music",
			indexer.Query{Type: indexer.SearchMusic, Term: "Artist (Live)", Categories: []int{3010, 8000}},
			"https://pretome.info/browse.php?st=1&search=Artist+(Live)&cat[]=31&cat[]=6&tags=RARFiX%2CMP3%2CV2%2C320kbps&tf=any",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if r.FollowRedirects {
				t.Errorf("FollowRedirects = true, want = false")
			}
		})
	}
}

// results is a search page as PreToMe shows it.
const results = `<html><body>
<a href="logout.php">Logout</a>
<table><tbody>
<tr><td class="colhead">Type</td></tr>
<tr class="browse">
<td><a href="browse.php?cat[]=7&amp;tags=x264"><img alt="TV"></a></td>
<td><a href="details.php?id=123&amp;hit=1" title="The.Show.S02E01.720p.HDTV.x264-GRP">The.Show.S02E01...</a></td>
<td><a href="download.php/123/The.Show.torrent"><img></a></td>
<td>3</td>
<td>0</td>
<td>2 hours<br>ago</td>
<td></td>
<td>1.5 GB</td>
<td>1,204</td>
<td>87</td>
<td>4</td>
</tr>
</tbody></table></body></html>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(results) })
	before := time.Now()
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 {
		t.Fatalf("releases = %d, want = 1", len(releases))
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "The.Show.S02E01.720p.HDTV.x264-GRP"},
		{"GUID", r.GUID, "https://pretome.info/details.php?id=123&hit=1"},
		{"InfoURL", r.InfoURL, "https://pretome.info/details.php?id=123&hit=1"},
		{"DownloadURL", r.DownloadURL, "https://pretome.info/download.php/123/The.Show.torrent"},
		{"Size", *r.Size, int64(1610612736)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 0.75},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(216000)},
		{"has TV/HD", slices.Contains(r.Categories, indexer.TVHD.ID), true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if want := before.Add(-2 * time.Hour); r.PublishDate.Sub(want).Abs() > time.Minute {
		t.Errorf("PublishDate = %v, want = %v", r.PublishDate, want)
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
				if got, want := string(body), "username=user&password=p%E9ss&login_pin=1234&returnto=%252F&login=Login"; got != want {
					t.Errorf("login form = %q, want = %q", got, want)
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
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login.php"}}, Body: http.NoBody}
				}
				if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "s1" {
					t.Errorf("search without the session cookie")
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
		if logins != 1 {
			t.Errorf("logins = %d, want = 1", logins)
		}
		if len(releases) != 1 {
			t.Errorf("releases = %d, want = 1", len(releases))
		}
	})
}

func TestLoginFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/takelogin.php" {
				return page(`<table class="body_table"><tr><td><font color="red"> Wrong PIN! </font></td></tr></table>`)
			}
			return page(`<form></form>`)
		})
		_, err := ix.Search(context.Background(), &indexer.Query{})
		if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), "Wrong PIN!") {
			t.Errorf("err = %v, want = an authentication error with the site's message", err)
		}
	})
}

func TestValidate(t *testing.T) {
	s := testSettings
	s.Pin = " "
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "pin") {
		t.Errorf("Validate() = %v, want = a pin error", err)
	}
}
