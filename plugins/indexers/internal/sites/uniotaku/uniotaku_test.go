package uniotaku

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
	return &indexer.Indexer{ID: "1", Name: "UniOtaku", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "p&ss"}}

func page(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchRequests(t *testing.T) {
	free := testSettings
	free.FreeleechOnly = true
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			"recent", testSettings,
			indexer.Query{},
			"https://tracker.uniotaku.com/torrents_.php?categoria=0&grupo=0&status=0&ordenar=0&start=0&length=100&search[value]=&search[regex]=false",
		},
		{
			"episode in one category, freeleech", free,
			indexer.Query{Type: indexer.SearchTV, Term: "Shingeki no-Kyojin", Season: indexer.Ptr(1), Episode: "2", Categories: []int{indexer.MoviesOther.ID}},
			"https://tracker.uniotaku.com/torrents_.php?categoria=47&grupo=0&status=1&ordenar=0&start=0&length=100&search[value]=%25Shingeki%25no%25Kyojin%25&search[regex]=false",
		},
		{
			"several categories", testSettings,
			indexer.Query{Term: "a_b", Categories: []int{indexer.TVAnime.ID}},
			"https://tracker.uniotaku.com/torrents_.php?categoria=0&grupo=0&status=0&ordenar=0&start=0&length=100&search[value]=%25a%25b%25&search[regex]=false",
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

// results is the torrents table as UniOtaku serves it.
const results = `{"draw":0,"recordsTotal":2,"data":[
["<a href=\"torrents-details.php?id=10\"> Kyojin 01 </a><img src=\"images/free.gif\">","<img alt=\"OVA\" src=\"x.png\">","",5,"2","40","1.5 GB","<a href=\"teams-view.php?id=3\">Fansub</a>"],
["<a href=\"torrents-details.php?id=11\">Kyojin 02</a><img src=\"images/silverdownload.gif\">","",null,3,1,7,"700 MB",""]
]}`

func loggedIn(resp *http.Response) *http.Response {
	resp.Header["Set-Cookie"] = []string{"uid=1; path=/", "pass=abc; path=/"}
	return resp
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return loggedIn(page(results)) })
	before := time.Now()
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
		{"Title", r.Title, "Kyojin 01 [Fansub]"},
		{"GUID", r.GUID, "https://tracker.uniotaku.com/torrents-details.php?id=10"},
		{"DownloadURL", r.DownloadURL, "https://tracker.uniotaku.com/torrents-details.php?id=10"},
		{"Category", r.Categories[0], indexer.TVAnime.ID},
		{"Size", *r.Size, int64(1610612736)},
		{"Grabs", *r.Grabs, 40},
		{"Seeders", *r.Seeders, 5},
		{"Peers", *r.Peers, 7},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"MinimumRatio", *r.MinimumRatio, 0.7},
		{"second Title", r2.Title, "Kyojin 02"},
		// Rows without a category image are anime.
		{"second Category", r2.Categories[0], indexer.TVAnime.ID},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 0.5},
		{"dates a minute apart", r.PublishDate.Sub(r2.PublishDate), time.Minute},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if d := before.Sub(r.PublishDate); d < 0 || d > time.Minute {
		t.Errorf("PublishDate = %v, want = a minute before %v", r.PublishDate, before)
	}
}

func TestLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/account-login.php":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), "username=user&password=p%26ss&manter=1"; got != want {
					t.Errorf("login form = %q, want = %q", got, want)
				}
				if got := r.Header.Get("Referer"); got != "https://tracker.uniotaku.com/" {
					t.Errorf("Referer = %q", got)
				}
				return loggedIn(&http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/index.php"}}, Body: http.NoBody})
			case "/torrents_.php":
				if c, err := r.Cookie("pass"); err != nil || c.Value != "abc" {
					return page(`{"data":[]}`)
				}
				return page(results)
			case "/torrents-details.php":
				return page(`<a href="download.php?id=10&amp;f=x.torrent">Baixar</a>`)
			case "/download.php":
				if c, err := r.Cookie("uid"); err != nil || c.Value != "1" {
					t.Errorf("download without the login cookies")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"application/x-bittorrent"}},
					Body:       io.NopCloser(strings.NewReader("d8:announce0:e")),
				}
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
		data, err := ix.Download(context.Background(), releases[0].DownloadURL)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "d8:announce0:e" {
			t.Errorf("download = %q", data)
		}
	})
}

func TestLoginFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
			return page(`<div class="login-content"><span class="text-red"> Senha incorreta </span></div>`)
		})
		_, err := ix.Search(context.Background(), &indexer.Query{})
		if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), "Senha incorreta") {
			t.Errorf("err = %v, want = an authentication error with the site's message", err)
		}
	})
}

func TestDownloadWithoutLink(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return page(`<p>Not found</p>`) })
	link := "https://tracker.uniotaku.com/torrents-details.php?id=10"
	if _, err := ix.Download(context.Background(), link); err == nil || !strings.Contains(err.Error(), "Failed to fetch download link from "+link) {
		t.Errorf("err = %v, want = a missing link error", err)
	}
}
