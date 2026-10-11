package anidub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

func newIndexer(t *testing.T, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(indexer.UserPassSettings{Username: "user", Password: "pass word"})
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
	return &indexer.Indexer{ID: "1", Name: "Anidub", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func html200(body string, cookies ...string) *http.Response {
	h := http.Header{"Content-Type": {"text/html; charset=utf-8"}}
	for _, c := range cookies {
		h.Add("Set-Cookie", c)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSettings(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil {
		t.Error("New() without credentials error = nil, want an error")
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name  string
		q     indexer.Query
		story string // "" for the main page
	}{
		{"rss", indexer.Query{}, ""},
		{"tv rss", indexer.Query{Type: indexer.SearchTV}, ""},
		{"search", indexer.Query{Term: "Naruto Shippuden"}, "Naruto Shippuden"},
		{"tv episode", indexer.Query{Type: indexer.SearchTV, Term: "Naruto", Season: indexer.Ptr(1), Episode: "5"}, "Naruto"},
		{"tv season", indexer.Query{Type: indexer.SearchTV, Term: "Naruto", Season: indexer.Ptr(2)}, "Naruto"},
		{"movie", indexer.Query{Type: indexer.SearchMovie, Term: "Akira 1988"}, "Akira"},
		{"book", indexer.Query{Type: indexer.SearchBook, Term: "Берсерк"}, "Берсерк"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
			r := c.First()
			if tt.story == "" {
				if r.URL != "https://tr.anidub.com/" || r.Method != "" && r.Method != http.MethodGet {
					t.Errorf("request = %s %s, want = GET https://tr.anidub.com/", r.Method, r.URL)
				}
				return
			}
			if r.URL != "https://tr.anidub.com/index.php?do=search" || r.Method != http.MethodPost {
				t.Errorf("request = %s %s, want = POST https://tr.anidub.com/index.php?do=search", r.Method, r.URL)
			}
			want := "do=search&subaction=search&search_start=1&full_search=1&result_from=1&story=" +
				url.QueryEscape(tt.story) +
				"&titleonly=3&searchuser=&replyless=0&replylimit=0&searchdate=0&beforeafter=after&sortby=&resorder=desc&showposts=1&catlist[]=0"
			if got := string(r.Body); got != want {
				t.Errorf("body = %q, want = %q", got, want)
			}
			if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %q, want = application/x-www-form-urlencoded", got)
			}
		})
	}
}

const listPage = `<html><body><a href="index.php?action=logout">Выход</a>
<div id="dle-content">
<div class="story"><div class="story_h"><div class="lcol"><h2><a href="https://tr.anidub.com/anime_tv/full/11111-naruto.html">Naruto</a></h2></div></div></div>
<div class="story"><div class="story_h"><div class="lcol"><h2><a href="/anime_movie/22222-akira.html">Akira</a></h2></div></div></div>
</div></body></html>`

const tvPage = `<html><body><a href="index.php?action=logout">Выход</a>
<h1 id="news-title">Наруто / Naruto [01-220 из 220]</h1>
<ul class="story_inf"><li>Автор</li><li><b>Дата:</b> Вчера, 18:30</li></ul>
<div id="tabs">
<div class="torrent_c" id="tv720"><div id="torrent_1001_info">
<ul class="list down"><li class="red">45.3 GB</li><li class="li_distribute_m">12</li><li class="li_swing_m">3</li><li class="li_download_m">1500</li></ul>
</div></div>
<div class="torrent_c" id="hwp"><div id="torrent_1002_info">
<ul class="list down"><li class="red">700 MB</li><li class="li_distribute_m">x</li><li class="li_swing_m"> 4 </li><li class="li_download_m">9</li></ul>
</div></div>
</div></body></html>`

const moviePage = `<html><body><a href="index.php?action=logout">Выход</a>
<h1 id="news-title">Акира / Akira</h1>
<ul class="story_inf"><li>Автор</li><li><b>Дата:</b> 5-03-2021, 07:15</li></ul>
<div id="tabs"><div class="torrent_c" id="web"><div id="torrent_2001_info">
<ul class="list down"><li class="red">8 GB</li><li class="li_distribute_m">5</li><li class="li_swing_m">1</li><li class="li_download_m">77</li></ul>
</div></div></div></body></html>`

func TestParse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var referers []string
		ix := newIndexer(t, func(r *http.Request) *http.Response {
			mu.Lock()
			defer mu.Unlock()
			switch r.URL.Path {
			case "/":
				return html200(listPage)
			case "/anime_tv/full/11111-naruto.html":
				referers = append(referers, r.Header.Get("Referer"))
				return html200(tvPage)
			case "/anime_movie/22222-akira.html":
				return html200(moviePage)
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}
		})
		now := time.Date(2023, 4, 10, 22, 0, 0, 0, time.UTC) // 11 April in Moscow
		ix.Now = func() time.Time { return now }
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if len(releases) != 3 {
			t.Fatalf("releases = %d, want = 3", len(releases))
		}
		tv, sd, movie := releases[0], releases[1], releases[2]
		const tvURL = "https://tr.anidub.com/anime_tv/full/11111-naruto.html"
		for _, c := range []struct {
			name      string
			got, want any
		}{
			{"Title", tv.Title, "Наруто / Naruto [01-220 из 220] [HDTV 720p]"},
			{"InfoURL", tv.InfoURL, tvURL},
			{"GUID", tv.GUID, tvURL + "torrent_1001_info"},
			{"DownloadURL", tv.DownloadURL, "https://tr.anidub.com/engine/download.php?id=1001"},
			{"Size", *tv.Size, int64(48640504627)},
			{"Seeders", *tv.Seeders, 12},
			{"Peers", *tv.Peers, 15},
			{"Grabs", *tv.Grabs, 1500},
			{"Category", tv.Categories[0], indexer.TVAnime.ID},
			{"PublishDate", tv.PublishDate, time.Date(2023, 4, 10, 15, 30, 0, 0, time.UTC)},
			{"DownloadVolumeFactor", *tv.DownloadVolumeFactor, 0.0},
			{"UploadVolumeFactor", *tv.UploadVolumeFactor, 1.0},
			{"Resolution", tv.Resolution, "HDTV 720p"},
			{"sd Title", sd.Title, "Наруто / Naruto [01-220 из 220] [SDTV]"},
			{"sd Resolution", sd.Resolution, "SDTV"},
			{"sd Seeders", *sd.Seeders, 0},
			{"sd Peers", *sd.Peers, 4},
			{"movie Title", movie.Title, "Акира / Akira [WEB]"},
			{"movie Category", movie.Categories[0], indexer.Movies.ID},
			{"movie PublishDate", movie.PublishDate, time.Date(2021, 3, 5, 4, 15, 0, 0, time.UTC)},
			{"Referer", referers[0], "https://tr.anidub.com/"},
		} {
			if c.got != c.want {
				t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
			}
		}
	})
}

func TestParseRateLimited(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, func(r *http.Request) *http.Response {
			if r.URL.Path == "/" {
				return html200(listPage)
			}
			return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(""))}
		})
		_, err := ix.Search(context.Background(), &indexer.Query{})
		var rl *indexer.RateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("Search() error = %v, want = a RateLimitError", err)
		}
	})
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		wantErr string
	}{
		{"success", `<a href="index.php?action=logout">Выход</a>`, ""},
		{"error", `<div id="content"><div class="berror"><div class="berror_c"> Неверный пароль </div></div></div>`, "Неверный пароль"},
		{"unknown", `<html></html>`, "Unknown error message, please report."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var form url.Values
				var cookie string
				ix := newIndexer(t, func(r *http.Request) *http.Response {
					if r.Method == http.MethodGet {
						return html200("<html></html>", "PHPSESSID=abc; Path=/")
					}
					if r.URL.String() != "https://tr.anidub.com/index.php" {
						t.Errorf("login URL = %s", r.URL)
					}
					cookie = r.Header.Get("Cookie")
					b, _ := io.ReadAll(r.Body)
					form, _ = url.ParseQuery(string(b))
					return html200(tt.answer, "dle_user_id=42; Path=/")
				})
				ix.SetCookies(map[string]string{"old": "1"}, time.Time{})
				err := ix.Site.(indexer.LoginSite).Login(context.Background(), ix)
				if form.Get("login_name") != "user" || form.Get("login_password") != "pass word" || form.Get("login") != "submit" {
					t.Errorf("form = %v", form)
				}
				if !strings.Contains(cookie, "PHPSESSID=abc") {
					t.Errorf("Cookie = %q, want = the main page's", cookie)
				}
				if tt.wantErr == "" {
					if err != nil {
						t.Fatal(err)
					}
					if got := ix.Cookies(); got["dle_user_id"] != "42" || got["old"] != "" {
						t.Errorf("cookies = %v, want = the login's", got)
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

func TestLoginNeeded(t *testing.T) {
	s := &Site{}
	for _, tt := range []struct {
		body string
		want bool
	}{
		{`<a href="index.php?action=logout">`, false},
		{`<form name="login">`, true},
	} {
		if got := s.LoginNeeded(&indexer.Response{Body: []byte(tt.body)}); got != tt.want {
			t.Errorf("LoginNeeded(%q) = %v, want = %v", tt.body, got, tt.want)
		}
	}
}

// TestLoginMainPageFollowsRedirects checks that the main page is fetched
// following redirects, as Prowlarr's new HttpRequest(BaseUrl) is.
func TestLoginMainPageFollowsRedirects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cookie string
		ix := newIndexer(t, func(r *http.Request) *http.Response {
			switch {
			case r.Method == http.MethodPost:
				cookie = r.Header.Get("Cookie")
				return html200(`<a href="index.php?action=logout">Выход</a>`, "dle_user_id=42; Path=/")
			case r.URL.Path == "/":
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/home/"}}, Body: http.NoBody}
			}
			return html200("<html></html>", "PHPSESSID=abc; Path=/")
		})
		if err := ix.Site.(indexer.LoginSite).Login(context.Background(), ix); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cookie, "PHPSESSID=abc") {
			t.Errorf("Cookie = %q, want = the main page's", cookie)
		}
	})
}
