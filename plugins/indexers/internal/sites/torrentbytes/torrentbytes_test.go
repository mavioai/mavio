package torrentbytes

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
	return &indexer.Indexer{ID: "1", Name: "Torrent Bytes", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "péss"}}

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
		{"recent", indexer.Query{}, "https://www.torrentbytes.net/browse.php?incldead=1&search=&sc=1"},
		{
			"movie by IMDb ID",
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "tt0076759", Categories: []int{2040}},
			"https://www.torrentbytes.net/browse.php?incldead=1&search=tt0076759&sc=2&c5=1&c46=1",
		},
		{
			"episode",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2"},
			"https://www.torrentbytes.net/browse.php?incldead=1&search=The+Show+S01E02&sc=1",
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
			if !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
		})
	}
	ix := newIndexer(t, testSettings, nil)
	c, err := ix.Site.SearchRequests(context.Background(), ix, &indexer.Query{Type: indexer.SearchBook, Term: "x"})
	if err != nil || c.First() != nil {
		t.Errorf("book search = %v, %v, want = no requests", c.First(), err)
	}
}

// results is a search page as TorrentBytes shows it, in Latin-1.
const results = "<html><body><a href=\"my.php\">Profile</a>\n" +
	"<table><tbody>\n" +
	"<tr><td class=\"colhead\">Type</td><td class=\"colhead\">Name</td></tr>\n" +
	"<tr>\n" +
	"<td><a href=\"browse.php?cat=38\"><img alt=\"TV/HD\"></a></td>\n" +
	"<td><a href=\"download.php?id=77&amp;name=The.Show.torrent\"><img></a> <a href=\"details.php?id=77&amp;hit=1\" title=\"The.Show.S01E02.Caf\xe9.720p.HDTV.x264-GRP\"><b>The.Show.S01E02.Caf\xe9...</b></a> <font color=\"green\">[F/L]</font></td>\n" +
	"<td>4</td><td>0</td><td>2015-04-25<br>23:38:12</td><td>x</td><td>1.37 GB</td><td>----</td><td>87</td><td>4</td>\n" +
	"</tr>\n" +
	"<tr>\n" +
	"<td></td>\n" +
	"<td><a href=\"download.php?id=78\"><img></a> <a href=\"details.php?id=78\">Short.Title-GRP</a> <font color=\"green\">fl</font></td>\n" +
	"<td>1</td><td>0</td><td>2015-04-26<br>01:02:03</td><td>x</td><td>100 MB</td><td>12</td><td>5</td><td>0</td>\n" +
	"</tr>\n" +
	"<tr><td></td><td><a href=\"download.php\"></a><a href=\"details.php\"></a></td></tr>\n" +
	"<tr><td></td><td><a href=\"download.php?id=79\"><img></a> <a href=\"details.php?id=79\">After.The.Empty.Row</a></td></tr>\n" +
	"</tbody></table></body></html>"

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(results) })
	releases, err := ix.Search(context.Background(), &indexer.Query{Interactive: true})
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
		{"Title", r.Title, "The.Show.S01E02.Café.720p.HDTV.x264-GRP"},
		{"Description", r.Description, "The.Show.S01E02.Café.720p.HDTV.x264-GRP"},
		{"GUID", r.GUID, "https://www.torrentbytes.net/details.php?id=77&hit=1"},
		{"DownloadURL", r.DownloadURL, "https://www.torrentbytes.net/download.php?id=77&name=The.Show.torrent"},
		{"PublishDate", r.PublishDate, time.Date(2015, 4, 25, 23, 38, 12, 0, time.Local)},
		{"Size", *r.Size, int64(1471026298)},
		{"Files", *r.Files, 4},
		{"Grabs", r.Grabs == nil, true},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"Category", r.Categories[0], indexer.TVHD.ID},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"second Title", r2.Title, "Short.Title-GRP"},
		{"second Categories", len(r2.Categories), 0},
		{"second Grabs", *r2.Grabs, 12},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
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
			case "/takelogin.php":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), "username=user&password=p%E9ss&returnto=%2F&login=Log+in!"; got != want {
					t.Errorf("login form = %q, want = %q", got, want)
				}
				if got, want := r.Header.Get("Referer"), "https://www.torrentbytes.net/takelogin.php"; got != want {
					t.Errorf("Referer = %q, want = %q", got, want)
				}
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"/my.php"}, "Set-Cookie": {"uid=1; path=/", "pass=abc; path=/"}},
					Body:       http.NoBody,
				}
			case "/my.php":
				return page(`<a href="my.php">Profile</a>`)
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
		if logins != 1 || len(releases) != 1 {
			t.Errorf("logins, releases = %d, %d, want = 1, 1", logins, len(releases))
		}
	})
}

func TestLoginFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			return page(`<table><tr><td class="embedded"> Username or password incorrect </td></tr></table>`)
		})
		_, err := ix.Search(context.Background(), &indexer.Query{})
		if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), ": Username or password incorrect") {
			t.Errorf("err = %v, want = an authentication error with the site's message", err)
		}
	})
}
