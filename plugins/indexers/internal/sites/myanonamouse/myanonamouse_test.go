package myanonamouse

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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
	return &indexer.Indexer{ID: "1", Name: "MyAnonamouse", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{MamID: "settings-id"}

func TestSearchRequests(t *testing.T) {
	const base = "https://www.myanonamouse.net/tor/js/loadSearchJSONbasic.php?"
	const fixed = "&tor[srchIn][title]=true&tor[srchIn][author]=true&tor[srchIn][narrator]=true&tor[searchIn]=torrents&tor[sortType]=default"
	full := testSettings
	full.SearchType = 3
	full.SearchInDescription = true
	full.SearchInSeries = true
	full.SearchInFilenames = true
	full.SearchLanguages = []int{1, 36}
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string // "" for no request
	}{
		{
			"RSS", testSettings,
			indexer.Query{},
			base + "tor[text]=&tor[searchType]=all" + fixed + "&tor[startNumber]=0&perpage=100&thumbnails=1&description=1&tor[cat][]=0",
		},
		{
			"book with paging and settings", full,
			indexer.Query{Type: indexer.SearchBook, Term: "Dune: Messiah!", Categories: []int{7020}, Offset: 100, Limit: 50},
			base + "tor[text]=Dune+Messiah&tor[searchType]=fl-VIP" + fixed + "&tor[startNumber]=100&perpage=50&thumbnails=1&description=1" +
				"&tor[srchIn][description]=true&tor[srchIn][series]=true&tor[srchIn][filenames]=true&tor[browse_lang][0]=1&tor[browse_lang][1]=36" +
				"&tor[cat][0]=14&tor[cat][1]=60&tor[cat][2]=71&tor[cat][3]=72",
		},
		{"term empty once sanitized", testSettings, indexer.Query{Term: "!!!"}, ""},
		{"TV", testSettings, indexer.Query{Type: indexer.SearchTV, Term: "Show"}, ""},
		{"movie", testSettings, indexer.Query{Type: indexer.SearchMovie, Term: "Film"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			got := ""
			if r != nil {
				got = r.URL
			}
			if tt.want == "" {
				if got != "" {
					t.Errorf("URL = %q, want = none", got)
				}
				return
			}
			// Only the leading categories of the book search are checked.
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("URL = %q, want = starting with %q", got, tt.want)
			}
			if r.Cookies["mam_id"] != "settings-id" {
				t.Errorf("mam_id = %q, want = %q", r.Cookies["mam_id"], "settings-id")
			}
		})
	}
}

func TestCookies(t *testing.T) {
	ix := newIndexer(t, testSettings, nil)
	site := ix.Site.(*Site)
	ix.SetCookies(map[string]string{"other": "x"}, time.Time{})
	if got := site.cookies(ix); got["mam_id"] != "settings-id" || got["other"] != "" {
		t.Errorf("cookies() = %v, want = the settings' mam_id only", got)
	}
	ix.SetCookies(map[string]string{"mam_id": "renewed", "other": "x"}, time.Time{})
	if got := site.cookies(ix); got["mam_id"] != "renewed" || got["other"] != "x" {
		t.Errorf("cookies() = %v, want = the stored ones", got)
	}
}

const feed = `{"perpage":100,"start":0,"data":[
{"id":1001,"title":"Dune","author_info":"{\"12\":\"Frank Herbert\",\"13\":\"Brian &amp; Kevin\"}","description":"A desert planet",
 "lang_code":"ENG","filetype":"epub mobi","vip":1,"free":0,"personal_freeleech":0,"fl_vip":1,"category":"63",
 "added":"2023-01-02 03:04:05","times_completed":42,"seeders":10,"leechers":2,"numfiles":3,"size":"1.5 MiB"},
{"id":"1002","title":"Audio Dune","author_info":null,"description":"","lang_code":"","filetype":"","vip":false,"free":false,
 "personal_freeleech":false,"fl_vip":false,"category":"41","added":"2023-01-01 00:00:00","times_completed":"0","seeders":"1","leechers":"0","numfiles":"20","size":"512 MiB"}
],"found":2,"message":""}`

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		class        string
		wedge        bool
		wantFactor   float64
		wantDownload string
	}{
		{"VIP user", "VIP", false, 0, "https://www.myanonamouse.net/tor/download.php?tid=1001"},
		{"user with wedges", "User", true, 1, "https://www.myanonamouse.net/tor/download.php?tid=1001&fl=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				settings := testSettings
				settings.UseFreeleechWedge = tt.wedge
				var userLoads atomic.Int32
				ix := newIndexer(t, settings, func(r *http.Request) *http.Response {
					if r.URL.Path == "/jsonLoad.php" {
						userLoads.Add(1)
						if c, err := r.Cookie("mam_id"); err != nil || c.Value != "renewed" {
							t.Errorf("user data mam_id = %v, want = renewed", c)
						}
						return jsonResponse(http.StatusOK, `{"classname":" `+tt.class+` "}`)
					}
					resp := jsonResponse(http.StatusOK, feed)
					resp.Header.Add("Set-Cookie", "mam_id=renewed; Path=/")
					return resp
				})
				for range 2 {
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
						{"Title", r.Title, "Dune by Frank Herbert, Brian & Kevin [ENG / EPUB MOBI] [VIP]"},
						{"Author", r.Author, "Frank Herbert, Brian & Kevin"},
						{"BookTitle", r.BookTitle, "Dune"},
						{"Description", r.Description, "A desert planet"},
						{"GUID", r.GUID, "https://www.myanonamouse.net/t/1001"},
						{"InfoURL", r.InfoURL, "https://www.myanonamouse.net/t/1001"},
						{"DownloadURL", r.DownloadURL, tt.wantDownload},
						{"Categories", r.Categories, []int{7020, 100063}},
						{"PublishDate", r.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
						{"Grabs", *r.Grabs, 42},
						{"Files", *r.Files, 3},
						{"Seeders", *r.Seeders, 10},
						{"Peers", *r.Peers, 12},
						{"Size", *r.Size, int64(1572864)},
						{"DownloadVolumeFactor", *r.DownloadVolumeFactor, tt.wantFactor},
						{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
						{"MinimumRatio", *r.MinimumRatio, 1.0},
						{"MinimumSeedTime", *r.MinimumSeedTime, int64(259200)},
						{"second Title", r2.Title, "Audio Dune"},
						{"second Categories", r2.Categories, []int{3030, 100041}},
						{"second Files", *r2.Files, 20},
					} {
						if !equal(c.got, c.want) {
							t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
						}
					}
				}
				if n := userLoads.Load(); n != 1 {
					t.Errorf("user data loads = %d, want = 1", n)
				}
				if got := ix.Cookies()["mam_id"]; got != "renewed" {
					t.Errorf("stored mam_id = %q, want = %q", got, "renewed")
				}
			})
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name        string
		resp        func() *http.Response
		want        string
		auth        bool
		clearCookie bool
	}{
		{"forbidden", func() *http.Response { return jsonResponse(http.StatusForbidden, "") }, "[403 Forbidden] - mam_id expired or invalid", true, false},
		{"status", func() *http.Response { return jsonResponse(http.StatusNotFound, "") }, "Unexpected response status 404 code", false, true},
		{
			"HTML",
			func() *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<html>"))}
			},
			"Unexpected response header text/html from indexer request, expected application/json", false, true,
		},
		{"no data", func() *http.Response { return jsonResponse(http.StatusOK, `{"message":"Bad search"}`) }, "Unexpected response content from indexer request: Bad search", false, false},
		{"no data or message", func() *http.Response { return jsonResponse(http.StatusOK, `{}`) }, "Unexpected response content from indexer request: Check the logs", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return tt.resp() })
			ix.SetCookies(map[string]string{"mam_id": "stored"}, time.Time{})
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want = containing %q", err, tt.want)
			}
			if got := errors.Is(err, indexer.ErrAuth); got != tt.auth {
				t.Errorf("errors.Is(err, ErrAuth) = %v, want = %v", got, tt.auth)
			}
			if got := ix.Cookies()["mam_id"] == ""; got != tt.clearCookie {
				t.Errorf("cookies cleared = %v, want = %v", got, tt.clearCookie)
			}
		})
	}
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
		return jsonResponse(http.StatusOK, `{"error":"Nothing returned, out of 123456 torrents"}`)
	})
	if releases, err := ix.Search(context.Background(), &indexer.Query{}); err != nil || len(releases) != 0 {
		t.Errorf("Search() = %d, %v, want = 0, nil", len(releases), err)
	}
}

func TestDownloadCookies(t *testing.T) {
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		if got, want := r.Header.Get("Cookie"), "mam_id=settings-id"; got != want {
			t.Errorf("Cookie = %q, want = %q", got, want)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infod"))}
	})
	if data, err := ix.Download(context.Background(), "https://www.myanonamouse.net/tor/download.php?tid=1"); err != nil || string(data) != "d4:infod" {
		t.Errorf("Download() = %q, %v, want = %q, nil", data, err, "d4:infod")
	}
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "mam_id") {
		t.Errorf("New() error = %v, want = a mam_id error", err)
	}
}
