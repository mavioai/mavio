package speedcd

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
	return &indexer.Indexer{ID: "1", Name: "SpeedCD", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "p@ss"}}

func page(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequests(t *testing.T) {
	filtered := testSettings
	filtered.FreeleechOnly = true
	filtered.ExcludeArchives = true
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"recent", testSettings, indexer.Query{}, "https://speed.cd/browse/"},
		{
			"movie by IMDb ID, filtered", filtered,
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "tt0076759", Categories: []int{2040}},
			"https://speed.cd/browse/43/freeleech/norar/deep/q/tt0076759",
		},
		{
			"season by IMDb ID", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", IMDbID: "tt0123456", Season: indexer.Ptr(2)},
			"https://speed.cd/browse/deep/q/tt0123456+S02*",
		},
		{
			"episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2), Episode: "3", Categories: []int{5040}},
			"https://speed.cd/browse/52/49/q/The+Show+S02E03",
		},
		{
			"page", testSettings,
			indexer.Query{Term: "linux (iso)", Limit: 100, Offset: 200},
			"https://speed.cd/browse/q/linux+(iso)/p/3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
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

// results is a search page as SpeedCD shows it.
const results = `<html><body>
<a href="/browse.php">Browse</a>
<div class="boxContent"><table><tbody>
<tr>
<td><a href="/browse/49"><img alt="TV/HD"></a></td>
<td><div><a href="/t/987">[REQ] The.Show.S02E03.720p.HDTV.x264-GRP.</a></div>
<span class="elapsedDate x" title="Monday, January 2, 2023 at 3:04PM">1 day ago</span> <span class="tag">[Freeleech]</span></td>
<td></td>
<td><a href="/download/987/The.Show.torrent">DL</a></td>
<td>0</td>
<td>1.37 GB</td>
<td>1,204</td>
<td>87</td>
<td>4</td>
</tr>
<tr>
<td><a href="/browse/43"><img alt="Movies/HD"></a></td>
<td><div><a href="/t/988">A.Film.2020.1080p.BluRay.x264-GRP</a></div>
<span class="elapsedDate" title="Sunday, December 31, 2023 at 11:59AM">now</span> <span>[freeleech]</span></td>
<td></td>
<td><a href="/download/988/A.Film.torrent">DL</a></td>
<td>0</td>
<td>8 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
</tbody></table></div></body></html>`

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
		{"GUID", r.GUID, "https://speed.cd/t/987"},
		{"DownloadURL", r.DownloadURL, "https://speed.cd/download/987/The.Show.torrent"},
		{"PublishDate", r.PublishDate, time.Date(2023, 1, 2, 15, 4, 0, 0, time.UTC)},
		{"Size", *r.Size, int64(1471026298)},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"Category", r.Categories[0], indexer.TVHD.ID},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(259200)},
		{"second PublishDate", r2.PublishDate, time.Date(2023, 12, 31, 11, 59, 0, 0, time.UTC)},
		// The site's tag is case sensitive.
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
			var body []byte
			if r.Body != nil {
				body, _ = io.ReadAll(r.Body)
			}
			switch r.URL.Path {
			case "/checkpoint/API":
				if got, want := string(body), "username=user"; got != want {
					t.Errorf("user form = %q, want = %q", got, want)
				}
				resp := page(`{"html":"<form><input type=\"hidden\" name=\"a\" value=\"tok123\"><input name=\"pwd\"></form>"}`)
				resp.Header.Set("Set-Cookie", "PHPSESSID=s1; path=/")
				return resp
			case "/checkpoint/":
				logins++
				if got, want := string(body), "pwd=p%40ss&a=tok123"; got != want {
					t.Errorf("password form = %q, want = %q", got, want)
				}
				if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "s1" {
					t.Errorf("login without the session cookie")
				}
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"/index.php"}, "Set-Cookie": {"inSpeed_uid=1; path=/"}},
					Body:       http.NoBody,
				}
			case "/index.php":
				return page(`<a href="/browse.php">Browse</a>`)
			case "/browse/":
				if c, err := r.Cookie("inSpeed_uid"); err != nil || c.Value != "1" {
					return page(`<form action="/checkpoint/"></form>`)
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
	})
}

func TestLoginFailure(t *testing.T) {
	tests := []struct{ name, page, want string }{
		{"no token", `{"html":""}`, "Error parsing the login form"},
		{"site message", `<h5> Incorrect password </h5>`, "Incorrect password"},
		{"captcha", `<h5>Error</h5><p>Wrong Captcha!</p>`, "Captcha required"},
		{"unknown", `<p>Error</p>`, "Unknown error message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
					switch {
					case r.URL.Path == "/checkpoint/API" && tt.name == "no token":
						return page(tt.page)
					case r.URL.Path == "/checkpoint/API":
						return page(`{"html":"<input name=\"a\" value=\"tok\">"}`)
					case r.URL.Path == "/checkpoint/":
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
