package shazbat

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
	return &indexer.Indexer{ID: "1", Name: "Shazbat", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "p&ss"}}

func page(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestFixSearchTerm(t *testing.T) {
	tests := []struct{ term, want string }{
		{"The Show S01E02", "the show s01e02"},
		{"Show E05", "show"},
		{"The Show S01", "the show"},
		{"The.Show.(2019)", "the show"},
		{"The Show 2023.05.01", "the show"},
		{"1923", "1923"},
		{"Show [UK]", "show uk"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := fixSearchTerm(tt.term); got != tt.want {
			t.Errorf("fixSearchTerm(%q) = %q, want = %q", tt.term, got, tt.want)
		}
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name      string
		q         indexer.Query
		want      string // "" for no request
		xRequests bool
	}{
		{"RSS", indexer.Query{}, "https://www.shazbat.tube/torrents", false},
		{"episode", indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2"}, "https://www.shazbat.tube/search?search=the%20show&portlet=true", true},
		{"basic", indexer.Query{Term: "The Show (2019)"}, "https://www.shazbat.tube/search?search=the%20show&portlet=true", true},
		{"movie", indexer.Query{Type: indexer.SearchMovie, Term: "Film"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if tt.want == "" {
				if r != nil {
					t.Errorf("URL = %q, want = none", r.URL)
				}
				return
			}
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("Referer"); got != "https://www.shazbat.tube/" {
				t.Errorf("Referer = %q, want = %q", got, "https://www.shazbat.tube/")
			}
			if got := r.Header.Get("X-Requested-With") != ""; got != tt.xRequests {
				t.Errorf("X-Requested-With set = %v, want = %v", got, tt.xRequests)
			}
		})
	}
}

func TestLoginNeeded(t *testing.T) {
	site := &Site{}
	tests := []struct {
		name     string
		status   int
		location string
		body     string
		want     bool
	}{
		{"login redirect", http.StatusFound, "/Login?x=1", "", true},
		{"other redirect", http.StatusFound, "/torrents", "", false},
		{"sign in", http.StatusOK, "", "Please Sign In Now", true},
		{"full redirect", http.StatusOK, "", `<script>fullRedirect("/login")</script>`, true},
		{"page", http.StatusOK, "", "<table></table>", false},
	}
	for _, tt := range tests {
		r := &indexer.Response{URL: "https://www.shazbat.tube/torrents", StatusCode: tt.status, Header: http.Header{}, Body: []byte(tt.body)}
		if tt.location != "" {
			r.Header.Set("Location", tt.location)
		}
		if got := site.LoginNeeded(r); got != tt.want {
			t.Errorf("%s: LoginNeeded() = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

func TestLogin(t *testing.T) {
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		body, _ := io.ReadAll(r.Body)
		if got, want := string(body), "referer=&query=&tv_timezone=0&username=user&password=p%26ss"; got != want {
			t.Errorf("form = %q, want = %q", got, want)
		}
		if got := r.Header.Get("Referer"); got != "https://www.shazbat.tube/login" {
			t.Errorf("Referer = %q", got)
		}
		resp := page(http.StatusOK, "<a href=logout>Log out</a>")
		resp.Header.Add("Set-Cookie", "session=s1; Path=/")
		return resp
	})
	if err := ix.Site.(*Site).Login(context.Background(), ix); err != nil {
		t.Fatal(err)
	}
	if got := ix.Cookies()["session"]; got != "s1" {
		t.Errorf("session = %q, want = %q", got, "s1")
	}

	for _, tt := range []struct{ body, want string }{
		{`<p>Sign in now</p><div id="fail"><div class="modal-body"> Wrong password </div></div>`, "Wrong password"},
		{`<p>Sign in now</p>`, "Unknown error message, please report."},
	} {
		ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(http.StatusOK, tt.body) })
		err := ix.Site.(*Site).Login(context.Background(), ix)
		if !errors.Is(err, indexer.ErrAuth) || !strings.HasSuffix(err.Error(), tt.want) {
			t.Errorf("Login() = %v, want = ErrAuth ending with %q", err, tt.want)
		}
	}
}

const searchPage = `<html><body>
<span>Freeleech until: <span class="datetime" data-timestamp="1700000000">soon</span></span>
<table id="torrent-table"><tbody>
<tr class="eprow"><td></td><td></td>
 <td> <b>new</b> The.Show.S01E02.1080p.WEB.h264-GRP <label class="label label-tag"> Drama </label><label class="label-tag">Crime</label></td>
 <td>(1073741824) :10 / :2</td>
 <td><a href="load_torrent?id=7&amp;hash=x">dl</a><a href="torrent_info?id=7">info</a><span class="datetime" data-timestamp="1672628645"></span></td></tr>
<tr class="eprow"><td></td><td></td><td>The.Show.S01E01.HDTV.x264-GRP</td><td>garbage</td>
 <td><a href="load_torrent?id=6">dl</a><a href="torrent_info?id=6">info</a></td></tr>
</tbody></table>
<div class="show" data-id="42">The Show</div><div class="show" data-id="43">The Show (US)</div><div class="show" data-id="44">Other</div>
</body></html>`

const showPage = `<table><tr class="eprow"><td></td><td></td><td>The.Show.S02E01.2160p.WEB.h265-GRP</td><td>(2048) :1 / :0</td>
<td><a href="load_torrent?id=9">dl</a><a href="torrent_info?id=9">info</a><span class="datetime" data-timestamp="1672000000"></span></td></tr></table>`

func TestParse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var shows []string
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/show" {
				shows = append(shows, r.URL.RawQuery)
				if c, err := r.Cookie("session"); err != nil || c.Value != "s1" {
					t.Errorf("show page session = %v, want = s1", c)
				}
				if got, want := r.Header.Get("Referer"), "https://www.shazbat.tube/show?id="+r.URL.Query().Get("id"); got != want {
					t.Errorf("Referer = %q, want = %q", got, want)
				}
				return page(http.StatusOK, strings.ReplaceAll(showPage, "id=9", "id=9"+r.URL.Query().Get("id")))
			}
			return page(http.StatusOK, searchPage)
		})
		ix.SetCookies(map[string]string{"session": "s1"}, time.Time{})
		releases, err := ix.Search(context.Background(), &indexer.Query{Term: "The Show"})
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"id=42&show_mode=torrents&portlet=true&tab=true", "id=43&show_mode=torrents&portlet=true&tab=true"}; !equal(shows, want) {
			t.Errorf("show pages = %v, want = %v", shows, want)
		}
		if len(releases) != 4 {
			t.Fatalf("releases = %d, want = 4", len(releases))
		}
		r, r2, r3 := releases[0], releases[1], releases[2]
		for _, c := range []struct {
			name      string
			got, want any
		}{
			{"Title", r.Title, "The.Show.S01E02.1080p.WEB.h264-GRP"},
			{"GUID", r.GUID, "https://www.shazbat.tube/torrent_info?id=7"},
			{"InfoURL", r.InfoURL, "https://www.shazbat.tube/torrent_info?id=7"},
			{"DownloadURL", r.DownloadURL, "https://www.shazbat.tube/load_torrent?id=7&hash=x"},
			{"Categories", r.Categories, []int{5000, 5040}},
			{"Size", *r.Size, int64(1073741824)},
			{"Seeders", *r.Seeders, 10},
			{"Peers", *r.Peers, 12},
			{"PublishDate", r.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
			{"Genres", r.Genres, []string{"Drama", "Crime"}},
			{"Scene", r.Scene, true},
			{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
			{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
			{"MinimumRatio", *r.MinimumRatio, 1.0},
			{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
			{"second Categories", r2.Categories, []int{5000, 5030}},
			{"second Size", *r2.Size, int64(0)},
			{"second GUID", r2.GUID, "https://www.shazbat.tube/torrent_info?id=6"},
			{"second PublishDate", r2.PublishDate, time.Date(2023, 1, 2, 3, 3, 5, 0, time.UTC)},
			{"second Genres", r2.Genres, []string{}},
			{"show Title", r3.Title, "The.Show.S02E01.2160p.WEB.h265-GRP"},
			{"show Categories", r3.Categories, []int{5000, 5045}},
			{"show DownloadVolumeFactor", *r3.DownloadVolumeFactor, 0.0},
		} {
			if !equal(c.got, c.want) {
				t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
			}
		}
	})
}

func TestParseDates(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
		return page(http.StatusOK, `<table><tr class="eprow"><td></td><td></td><td>A 720p</td><td></td><td><a href="torrent_info?id=1"></a></td></tr><tr class="eprow"><td></td><td></td><td>B</td><td></td><td><a href="torrent_info?id=2"></a></td></tr></table>`)
	})
	start := time.Now().UTC()
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	if d := start.Sub(releases[0].PublishDate); d < 59*time.Second || d > 2*time.Minute {
		t.Errorf("first date %v before the search, want = a minute", d)
	}
	if d := releases[0].PublishDate.Sub(releases[1].PublishDate); d != time.Minute {
		t.Errorf("second date %v before the first, want = a minute", d)
	}
	if got := *releases[0].DownloadVolumeFactor; got != 1 {
		t.Errorf("DownloadVolumeFactor = %v, want = 1", got)
	}
}

func TestShowPageErrors(t *testing.T) {
	tests := []struct {
		name     string
		show     func() *http.Response
		cookies  bool
		want     string
		auth     bool
		rateLimt bool
	}{
		{"no cookies", nil, false, "Invalid cookies. Most likely your session expired or was killed.", false, false},
		{
			"login redirect",
			func() *http.Response {
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login"}}, Body: io.NopCloser(strings.NewReader(""))}
			},
			true, "We are being redirected to the Shazbat login page", true, false,
		},
		{"too many requests", func() *http.Response { return page(http.StatusTooManyRequests, "") }, true, "request limit reached", false, true},
		{"server error", func() *http.Response { return page(http.StatusBadGateway, "") }, true, "HTTP Error - 502. https://www.shazbat.tube/show?id=42&show_mode=torrents&portlet=true&tab=true", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
					if r.URL.Path == "/show" {
						return tt.show()
					}
					return page(http.StatusOK, `<div class="show" data-id="42"></div>`)
				})
				if tt.cookies {
					ix.SetCookies(map[string]string{"session": "s1"}, time.Time{})
				}
				_, err := ix.Search(context.Background(), &indexer.Query{Term: "show"})
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Errorf("err = %v, want = containing %q", err, tt.want)
				}
				if got := errors.Is(err, indexer.ErrAuth); got != tt.auth {
					t.Errorf("errors.Is(err, ErrAuth) = %v, want = %v", got, tt.auth)
				}
				var rl *indexer.RateLimitError
				if got := errors.As(err, &rl); got != tt.rateLimt {
					t.Errorf("errors.As(err, RateLimitError) = %v, want = %v", got, tt.rateLimt)
				}
				if tt.auth && len(ix.Cookies()) != 0 {
					t.Errorf("cookies = %v, want = cleared", ix.Cookies())
				}
			})
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		data string
		want string
	}{
		{`{"password":"p"}`, "username"},
		{`{"username":"u","password":"p","show_pages_fetch_limit":0}`, "greater than zero"},
		{`{"username":"u","password":"p","show_pages_fetch_limit":6}`, "less than or equal to 5"},
		{`{"username":"u","password":"p","show_pages_fetch_limit":5}`, ""},
		{`{"username":"u","password":"p","base_url":"https://www.shazbat.tv/"}`, ""},
	}
	for _, tt := range tests {
		site, err := Definition.New([]byte(tt.data))
		if tt.want == "" {
			if err != nil {
				t.Errorf("New(%s) error = %v, want = nil", tt.data, err)
			} else if got := site.(*Site).baseURL; got != "https://www.shazbat.tube/" {
				t.Errorf("baseURL = %q, want = the first link", got)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("New(%s) error = %v, want = containing %q", tt.data, err, tt.want)
		}
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
