package bakabt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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
	settings.Username, settings.Password = "user", "pass"
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
	return &indexer.Indexer{ID: "1", Name: "BakaBT", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func html200(body string, cookies ...string) *http.Response {
	h := http.Header{"Content-Type": {"text/html; charset=utf-8"}}
	for _, c := range cookies {
		h.Add("Set-Cookie", c)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSettings(t *testing.T) {
	if _, err := Definition.New([]byte(`{"freeleech_only":true}`)); err == nil {
		t.Error("New() without credentials error = nil, want an error")
	}
}

func TestSearchRequests(t *testing.T) {
	const prefix = "https://bakabt.me/browse.php?only=0&incomplete=1&lossless=1&hd=1&multiaudio=1&bonus=1&reorder=1&q="
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"rss", Settings{}, indexer.Query{}, prefix},
		{"search", Settings{}, indexer.Query{Term: "Cowboy Bebop"}, prefix + "Cowboy+Bebop"},
		{"trailing episode", Settings{}, indexer.Query{Term: "One Piece 1059"}, prefix + "One+Piece"},
		{"trailing E episode", Settings{}, indexer.Query{Term: "One Piece E12"}, prefix + "One+Piece"},
		// TV searches are by the term alone, without season or episode.
		{"tv", Settings{}, indexer.Query{Type: indexer.SearchTV, Term: "Naruto", Season: indexer.Ptr(1), Episode: "5"}, prefix + "Naruto"},
		{"movie", Settings{}, indexer.Query{Type: indexer.SearchMovie, Term: "Akira (1988)"}, prefix + "Akira+(1988)"},
		{
			"adult",
			Settings{AdultContent: true},
			indexer.Query{Term: "x"},
			"https://bakabt.me/browse.php?only=0&hentai=1&incomplete=1&lossless=1&hd=1&multiaudio=1&bonus=1&reorder=1&q=x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("Accept"); got != "" {
				t.Errorf("Accept = %q, want none", got)
			}
		})
	}
}

const page = `<html><body><a href="logout.php">Logout</a>
<table class="torrents">
<tr><th>Category</th></tr>
<tr class="torrent">
 <td class="category"><span title="Anime Series" class="icon"></span></td>
 <td class="name"><a class="title" href="torrent/1001/cowboy-bebop">Cowboy Bebop | Kaubōi Bibappu (Dual Audio) [BD 1080p]</a>
  <span class="tags">bd 1080p dual-audio</span> <span class="freeleech">Freeleech</span></td>
 <td class="added">'5 Jul '15</td>
 <td class="size">45.3 GB</td>
 <td class="peers">12k / <a href="download/1001/x.torrent">12</a> / <a href="#">3</a></td>
</tr>
<tr class="torrent_alt">
 <td class="category"></td>
 <td class="name"><a class="alt_title" href="torrent/1002/naruto">Naruto Season 2 [DVD]</a></td>
 <td class="added">yesterday</td>
 <td class="size">700 MB</td>
 <td class="peers">15 / <a href="download/1002/x.torrent">4</a> / <a href="#">1</a></td>
</tr>
<tr class="torrent">
 <td class="category"><span title="Anime Movie"></span></td>
 <td class="name"><a class="title" href="torrent/1003/akira">Akira (1988)</a></td>
 <td class="added">22 Jul 15</td>
 <td class="size">8 GB</td>
 <td class="peers">7 / <a href="download/1003/x.torrent">5</a> / <a href="#">0</a></td>
</tr>
<tr class="torrent"><td class="category"><span title="Unknown"></span></td><td class="name">no link</td></tr>
</table></body></html>`

func search(t *testing.T, settings Settings) []*indexer.Release {
	t.Helper()
	ix := newIndexer(t, settings, func(*http.Request) *http.Response { return html200(page) })
	now := time.Date(2023, 4, 10, 12, 0, 0, 0, time.Local)
	ix.Now = func() time.Time { return now }
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	return releases
}

func TestParse(t *testing.T) {
	releases := search(t, Settings{})
	if len(releases) != 3 {
		t.Fatalf("releases = %d, want = 3", len(releases))
	}
	bebop, naruto, akira := releases[0], releases[1], releases[2]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", bebop.Title, "Kaubōi Bibappu (Dual Audio) [BD 1080p]"},
		{"GUID", bebop.GUID, "https://bakabt.me/torrent/1001/cowboy-bebop"},
		{"InfoURL", bebop.InfoURL, "https://bakabt.me/torrent/1001/cowboy-bebop"},
		{"DownloadURL", bebop.DownloadURL, "https://bakabt.me/download/1001/x.torrent"},
		{"Description", bebop.Description, "bd 1080p dual-audio"},
		{"Category", bebop.Categories[0], indexer.TVAnime.ID},
		{"Grabs", *bebop.Grabs, 12000},
		{"Seeders", *bebop.Seeders, 12},
		{"Peers", *bebop.Peers, 15},
		{"PublishDate", bebop.PublishDate, time.Date(2015, 7, 5, 0, 0, 0, 0, time.Local)},
		{"DownloadVolumeFactor", *bebop.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *bebop.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *bebop.MinimumRatio, 1.0},
		{"MinimumSeedTime", *bebop.MinimumSeedTime, int64(172800)},
		{"naruto Title", naruto.Title, "Naruto Season 2 [DVD]"},
		{"naruto Category", naruto.Categories[0], indexer.TVAnime.ID},
		{"naruto Grabs", *naruto.Grabs, 15},
		{"naruto PublishDate", naruto.PublishDate, time.Date(2023, 4, 9, 12, 0, 0, 0, time.Local)},
		{"naruto DownloadVolumeFactor", *naruto.DownloadVolumeFactor, 1.0},
		{"akira Title", akira.Title, "Akira (1988)"},
		{"akira Category", akira.Categories[0], indexer.Movies.ID},
		{"akira PublishDate", akira.PublishDate, time.Date(2015, 7, 22, 0, 0, 0, 0, time.Local)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseSettings(t *testing.T) {
	// The titles of a row share its GUID, so searches keep the first
	// only, as Prowlarr's do; the parser returns them all.
	ix := newIndexer(t, Settings{AddRomajiTitle: true, AppendSeason: true, FreeleechOnly: true}, nil)
	releases, err := ix.Site.ParseResponse(context.Background(), ix, &indexer.Response{Body: []byte(page)})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, r := range releases {
		titles = append(titles, r.Title)
	}
	want := []string{
		"Cowboy Bebop Season 1 (Dual Audio) [BD 1080p]",
		"Kaubōi Bibappu  Season 1 (Dual Audio) [BD 1080p]",
	}
	if strings.Join(titles, "\n") != strings.Join(want, "\n") {
		t.Errorf("titles = %q, want = %q", titles, want)
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name, page, want string
	}{
		{"link", `<a class="download_link" href="download/1001/x.torrent?key=k">Download</a>`, "torrent"},
		{"no link", `<html></html>`, "Unable to find download link."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, Settings{}, func(r *http.Request) *http.Response {
					if r.URL.Path == "/download/1001/x.torrent" {
						if r.URL.RawQuery != "key=k" {
							t.Errorf("query = %q", r.URL.RawQuery)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": {"application/x-bittorrent"}},
							Body:       io.NopCloser(strings.NewReader("d8:announce4:test4:infod4:name1:xee")),
						}
					}
					if c, err := r.Cookie("uid"); err != nil || c.Value != "7" {
						t.Errorf("page cookie uid = %v, want = 7", c)
					}
					return html200(tt.page)
				})
				ix.SetCookies(map[string]string{"uid": "7"}, time.Time{})
				data, err := ix.Download(context.Background(), "https://bakabt.me/torrent/1001/cowboy-bebop")
				if tt.want != "torrent" {
					if err == nil || err.Error() != tt.want {
						t.Errorf("Download() error = %v, want = %q", err, tt.want)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(string(data), "d8:announce") {
					t.Errorf("Download() = %q, want the torrent", data)
				}
			})
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name, answer, wantErr string
	}{
		{"success", `<a href="logout.php">Logout</a>`, ""},
		{"error", `<form id="loginForm"><div id="loginError"> Invalid password </div></form>`, "Invalid password"},
		{"unknown", `<form id="loginForm"></form>`, "Unknown error message, please report."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var form url.Values
				var cookie string
				ix := newIndexer(t, Settings{}, func(r *http.Request) *http.Response {
					if r.URL.String() != "https://bakabt.me/login.php" {
						t.Errorf("login URL = %s", r.URL)
					}
					if r.Method == http.MethodGet {
						return html200(`<form id="loginForm"><input name="loginKey" value="k3y"></form>`, "PHPSESSID=abc; Path=/")
					}
					cookie = r.Header.Get("Cookie")
					b, _ := io.ReadAll(r.Body)
					form, _ = url.ParseQuery(string(b))
					return html200(tt.answer, "uid=7; Path=/")
				})
				err := ix.Site.(indexer.LoginSite).Login(context.Background(), ix)
				want := url.Values{"loginKey": {"k3y"}, "username": {"user"}, "password": {"pass"}, "returnto": {"/index.php"}}
				if form.Encode() != want.Encode() {
					t.Errorf("form = %v, want = %v", form, want)
				}
				if !strings.Contains(cookie, "PHPSESSID=abc") {
					t.Errorf("Cookie = %q, want = the login page's", cookie)
				}
				if tt.wantErr == "" {
					if err != nil {
						t.Fatal(err)
					}
					if got := ix.Cookies()["uid"]; got != "7" {
						t.Errorf("cookie uid = %q, want = 7", got)
					}
					return
				}
				if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Login() error = %v, want = %q", err, tt.wantErr)
				}
			})
		})
	}
}

// TestLoginPageFollowsRedirects checks that the login page is fetched
// following redirects, as Prowlarr's new HttpRequest(loginUrl) is.
func TestLoginPageFollowsRedirects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var form url.Values
		ix := newIndexer(t, Settings{}, func(r *http.Request) *http.Response {
			switch {
			case r.Method == http.MethodPost:
				b, _ := io.ReadAll(r.Body)
				form, _ = url.ParseQuery(string(b))
				return html200(`<a href="logout.php">Logout</a>`, "uid=7; Path=/")
			case r.URL.Path == "/login.php":
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login2.php"}}, Body: http.NoBody}
			}
			return html200(`<form id="loginForm"><input name="loginKey" value="k3y"></form>`, "PHPSESSID=abc; Path=/")
		})
		if err := ix.Site.(indexer.LoginSite).Login(context.Background(), ix); err != nil {
			t.Fatal(err)
		}
		if got := form.Get("loginKey"); got != "k3y" {
			t.Errorf("loginKey = %q, want = k3y", got)
		}
	})
}
