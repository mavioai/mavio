package revolutiontt

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
	return &indexer.Indexer{ID: "1", Name: "RevolutionTT", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{indexer.UserPassSettings{Username: "user", Password: "p&ss"}}

func page(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequests(t *testing.T) {
	legacy := testSettings
	legacy.BaseURL = "https://revolutiontt.me/"
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			"recent at a legacy address", legacy,
			indexer.Query{},
			"https://revott.me/browse.php?incldead=1&titleonly=1&search=",
		},
		{
			"movie by IMDb ID", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "tt0076759", Categories: []int{2040}},
			"https://revott.me/browse.php?incldead=1&titleonly=0&search=tt0076759&c12=1",
		},
		{
			"season", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(3), Categories: []int{5000}},
			"https://revott.me/browse.php?incldead=1&titleonly=1&search=The+Show+S03&c23=1&c43=1&c42=1&c45=1&c41=1&c7=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
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

// results is a search page as RevolutionTT shows it.
const results = `<html><body>
<a href="/logout.php">Logout</a>
<table id="torrents-table"><tbody>
<tr><th>Type</th><th>Name</th></tr>
<tr><td colspan="10">Please donate!</td></tr>
<tr>
<td class="br_type"><a href="browse.php?cat=42"><img alt="TV/HD"></a></td>
<td class="br_right"><a href="details.php?id=55"><b>[REQ] The.Show.S03E01.1080p.WEB.h264-GRP</b></a> <a href="https://www.imdb.com/title/tt1234567">IMDb</a></td>
<td></td>
<td><a href="download.php/55/The.Show.torrent">DL</a></td>
<td>2</td>
<td><nobr>2015-04-25<br>23:38:12</nobr></td>
<td>1.37 GB<br><a href="filelist.php?id=55">12 files</a></td>
<td>40 times</td>
<td>17</td>
<td>3</td>
</tr>
</tbody></table></body></html>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(results) })
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
		{"Title", r.Title, "The.Show.S03E01.1080p.WEB.h264-GRP"},
		{"GUID", r.GUID, "https://revott.me/details.php?id=55"},
		{"DownloadURL", r.DownloadURL, "https://revott.me/download.php/55/The.Show.torrent"},
		{"PublishDate", r.PublishDate, time.Date(2015, 4, 25, 23, 38, 12, 0, time.Local)},
		{"Size", *r.Size, int64(1471026298)},
		{"Files", *r.Files, 12},
		{"Grabs", *r.Grabs, 40},
		{"Seeders", *r.Seeders, 17},
		{"Peers", *r.Peers, 20},
		{"IMDbID", r.IMDbID, 1234567},
		{"Categories", len(r.Categories) == 1 && r.Categories[0] == indexer.TVHD.ID, true},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
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
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"/index.php"}, "Set-Cookie": {"uid=1; path=/", "pass=abc; path=/"}},
					Body:       http.NoBody,
				}
			case "/index.php":
				return page(`<a href="/logout.php">Logout</a>`)
			case "/browse.php":
				if c, err := r.Cookie("pass"); err != nil || c.Value != "abc" {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login.php"}}, Body: http.NoBody}
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
		if logins != 1 || len(releases) != 1 {
			t.Errorf("logins, releases = %d, %d, want = 1, 1", logins, len(releases))
		}
		if got := ix.Cookies(); got["PHPSESSID"] != "s1" || got["uid"] != "1" {
			t.Errorf("cookies = %v, want = the login page's and the login's", got)
		}
	})
}

func TestLoginFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
			return page(`<form action="takelogin.php">Wrong password</form>`)
		})
		_, err := ix.Search(context.Background(), &indexer.Query{})
		if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), "RevolutionTT authentication failed") {
			t.Errorf("err = %v, want = an authentication error", err)
		}
	})
}
