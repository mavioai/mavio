package gazelle

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

// handler answers a site's requests.
type handler func(r *http.Request) *http.Response

func (h handler) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := h(r)
	resp.Request = r
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp, nil
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func definitionOf(t *testing.T, id string) indexer.Definition {
	t.Helper()
	i := slices.IndexFunc(Definitions, func(d indexer.Definition) bool { return d.ID == id })
	if i < 0 {
		t.Fatalf("no definition %q", id)
	}
	return Definitions[i]
}

func newIndexer(t *testing.T, id string, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := definitionOf(t, id).New(data)
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
	return &indexer.Indexer{ID: "1", Name: id, Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var userPass = Settings{UserPassSettings: indexer.UserPassSettings{Username: "somekey", Password: "somekey"}}

func TestSecretCinemaCases(t *testing.T) {
	ported.Run(t, "secret_cinema_tests/secret_cinema.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_SecretCinema": "TestSecretCinemaRecentFeed",
	}})
}

func TestSecretCinemaRecentFeed(t *testing.T) {
	feed := string(ported.Response(t, "SecretCinema/recentfeed.json"))
	ix := newIndexer(t, "secretcinema", userPass, func(*http.Request) *http.Response { return jsonResponse(feed) })
	releases, err := ix.Search(context.Background(), &indexer.Query{Categories: []int{2000}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(releases), 3; got != want {
		t.Fatalf("len(releases) = %d, want = %d", got, want)
	}
	r := releases[0]
	checks := []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Singin' in the Rain (1952) 2160p"},
		{"Protocol", r.Protocol, indexer.ProtocolTorrent},
		{"DownloadURL", r.DownloadURL, "https://secret-cinema.pw/torrents.php?action=download&id=45068"},
		{"InfoURL", r.InfoURL, "https://secret-cinema.pw/torrents.php?id=2497&torrentid=45068"},
		{"CommentsURL", r.CommentsURL, ""},
		{"PublishDate", r.PublishDate, time.Date(2022, 12, 15, 17, 37, 29, 0, time.UTC)},
		{"Size", *r.Size, int64(57473058680)},
		{"InfoHash", r.InfoHash, ""},
		{"MagnetURL", r.MagnetURL, ""},
		{"Peers", *r.Peers, 8},
		{"Seeders", *r.Seeders, 8},
		{"IMDbID", r.IMDbID, 0},
		{"TMDBID", r.TMDBID, 0},
		{"TVDBID", r.TVDBID, 0},
		{"Languages", len(r.Languages), 0},
		{"Subtitles", len(r.Subtitles), 0},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			name: "basic", id: "brokenstones", settings: userPass,
			q:    indexer.Query{Term: "Some.Title", Categories: []int{4000}},
			want: "https://brokenstones.is/ajax.php?action=browse&order_by=time&order_way=desc&searchstr=Some+Title&filter_cat[1]=1&filter_cat[2]=1&filter_cat[3]=1&filter_cat[4]=1",
		},
		{
			name: "alpharatio tv", id: "alpharatio",
			settings: Settings{UserPassSettings: userPass.UserPassSettings, FreeleechOnly: true, ExcludeScene: true},
			q:        indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1), Episode: "2", Limit: 50, Offset: 100},
			want:     "https://alpharatio.cc/ajax.php?action=browse&order_by=time&order_way=desc&searchstr=Show+S01E02&freetorrent=1&scene=0&page=3",
		},
		{
			name: "alpharatio imdb in tags", id: "alpharatio", settings: userPass,
			q:    indexer.Query{Type: indexer.SearchMovie, IMDbID: "123456"},
			want: "https://alpharatio.cc/ajax.php?action=browse&order_by=time&order_way=desc&taglist=tt0123456",
		},
		{
			name: "music", id: "dicmusic", settings: userPass,
			q:    indexer.Query{Type: indexer.SearchMusic, Artist: "VA", Album: "An Album", Label: "Label"},
			want: "https://dicmusic.com/ajax.php?action=browse&order_by=time&order_way=desc&groupname=An+Album&recordlabel=Label",
		},
		{
			name: "secret cinema imdb", id: "secretcinema", settings: userPass,
			q:    indexer.Query{Type: indexer.SearchMovie, Term: "Title", IMDbID: "tt0045152"},
			want: "https://secret-cinema.pw/ajax.php?action=browse&order_by=time&order_way=desc&searchstr=Title&cataloguenumber=tt0045152",
		},
		{
			name: "greatposterwall imdb", id: "greatposterwall",
			settings: Settings{UserPassSettings: userPass.UserPassSettings, FreeleechOnly: true},
			q:        indexer.Query{Type: indexer.SearchMovie, Term: "Title: Sub", IMDbID: "45152"},
			want:     "https://greatposterwall.com/ajax.php?action=browse&order_by=time&order_way=desc&searchstr=tt0045152&freetorrent=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.id, tt.settings, nil)
			chain, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := chain.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, "brokenstones", userPass, func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/login.php":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), "username=somekey&password=somekey&keeplogged=1"; got != want {
					t.Errorf("login form = %q, want = %q", got, want)
				}
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": {"/index.php"}, "Set-Cookie": {"session=abc; path=/"}},
					Body:       http.NoBody,
				}
			case "/ajax.php":
				if c, err := r.Cookie("session"); err != nil || c.Value != "abc" {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login.php"}}, Body: http.NoBody}
				}
				return jsonResponse(`{"status":"success","response":{"results":[{"groupId":"1","groupName":"App &amp; Co","torrentId":7,"size":"100","seeders":"2","leechers":"1","groupTime":"1700000000","category":"MacOS Apps","snatches":"3"}]}}`)
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
			t.Fatalf("len(releases) = %d, want = 1", len(releases))
		}
		r := releases[0]
		if got, want := r.Title, "App & Co"; got != want {
			t.Errorf("Title = %q, want = %q", got, want)
		}
		if got, want := r.PublishDate, time.Unix(1700000000, 0).UTC(); !got.Equal(want) {
			t.Errorf("PublishDate = %v, want = %v", got, want)
		}
		if got, want := r.Categories, []int{indexer.PCMac.ID, 100001}; !slices.Equal(got, want) {
			t.Errorf("Categories = %v, want = %v", got, want)
		}
		if got, want := *r.Grabs, 3; got != want {
			t.Errorf("Grabs = %d, want = %d", got, want)
		}
	})
}

func TestCookieSetting(t *testing.T) {
	var cookie string
	s := Settings{Cookie: "session=xyz; other=1"}
	ix := newIndexer(t, "dicmusic", s, func(r *http.Request) *http.Response {
		cookie = r.Header.Get("Cookie")
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login.php"}}, Body: http.NoBody}
	})
	_, err := ix.Search(context.Background(), &indexer.Query{})
	if err == nil {
		t.Fatal("Search() error = nil, want the status error")
	}
	if !strings.Contains(cookie, "session=xyz") || !strings.Contains(cookie, "other=1") {
		t.Errorf("Cookie = %q, want the cookie setting's", cookie)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		id       string
		settings Settings
		ok       bool
	}{
		{"brokenstones", userPass, true},
		{"brokenstones", Settings{Cookie: "a=b"}, false},
		{"dicmusic", Settings{Cookie: "a=b"}, true},
		{"dicmusic", Settings{}, false},
		{"dicmusic", Settings{UserPassSettings: indexer.UserPassSettings{Username: "u"}}, false},
	}
	for _, tt := range tests {
		data, _ := json.Marshal(tt.settings)
		_, err := definitionOf(t, tt.id).New(data)
		if got := err == nil; got != tt.ok {
			t.Errorf("%s New(%s) error = %v, want ok = %v", tt.id, data, err, tt.ok)
		}
	}
}

func TestParseTorrents(t *testing.T) {
	body := `{"status":"success","response":{"results":[
		{"groupId":5,"groupName":"Album","artist":"Artist","groupYear":2001,"cover":"/static/c.jpg","torrents":[
			{"torrentId":10,"format":"FLAC","encoding":"Lossless","media":"CD","hasCue":true,"time":"2020-01-02 03:04:05","size":"500","seeders":"4","leechers":"0","fileCount":12,"isFreeLeech":true,"canUseToken":true,"category":"Music"},
			{"torrentId":11,"format":"MP3","encoding":"320","media":"WEB","time":"2021-01-02 03:04:05","size":"50","seeders":"1","leechers":"1","canUseToken":true,"category":"Select Category"},
			{"torrentId":12,"format":"MP3","encoding":"V0","media":"WEB","time":"2019-01-02 03:04:05","size":"40","seeders":"1","leechers":"1","canUseToken":false}
		]}]}}`
	ix := newIndexer(t, "desigaane", Settings{UserPassSettings: userPass.UserPassSettings, UseFreeleechToken: TokenRequired}, func(*http.Request) *http.Response { return jsonResponse(body) })
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range releases {
		got = append(got, r.Title+" | "+r.DownloadURL)
	}
	want := []string{
		"Artist - Album (2001) [MP3 320] [WEB] | https://desigaane.rocks/torrents.php?action=download&id=11&usetoken=1",
		"Artist - Album (2001) [FLAC Lossless] [CD] [Cue] | https://desigaane.rocks/torrents.php?action=download&id=10",
	}
	if !slices.Equal(got, want) {
		t.Errorf("releases = %q, want = %q", got, want)
	}
	if got, want := releases[1].PosterURL, "https://desigaane.rocks/static/c.jpg"; got != want {
		t.Errorf("PosterURL = %q, want = %q", got, want)
	}
	if got, want := releases[0].Categories, []int{indexer.Audio.ID, 100001}; !slices.Equal(got, want) {
		t.Errorf("Categories = %v, want = %v", got, want)
	}
}

func TestParseGreatPosterWall(t *testing.T) {
	body := `{"status":"success","response":{"currentPage":1,"pages":1,"results":[
		{"groupId":9,"groupName":"Movie","cover":"https://img/c.jpg","imdbId":"tt0045152","torrents":[
			{"torrentId":1,"time":"2023-05-06 10:00:00","size":1000,"seeders":3,"leechers":2,"snatches":5,"fileCount":1,"resolution":"2160p","fileName":"Movie.2023.2160p &amp; More","freeType":"12","canUseToken":true},
			{"torrentId":2,"time":"2023-05-06 09:00:00","size":900,"seeders":1,"leechers":0,"snatches":0,"fileCount":1,"resolution":"720p","fileName":"Movie.2023.720p","freeType":"2","isNeutralLeech":false}
		]}]}}`
	ix := newIndexer(t, "greatposterwall", userPass, func(*http.Request) *http.Response { return jsonResponse(body) })
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("len(releases) = %d, want = 2", len(releases))
	}
	r := releases[0]
	checks := []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Movie.2023.2160p & More"},
		{"PublishDate", r.PublishDate, time.Date(2023, 5, 6, 2, 0, 0, 0, time.UTC)},
		{"Categories", len(r.Categories) == 2 && r.Categories[1] == indexer.MoviesUHD.ID, true},
		{"IMDbID", r.IMDbID, 45152},
		{"Peers", *r.Peers, 5},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.5},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
		{"DownloadURL", r.DownloadURL, "https://greatposterwall.com/torrents.php?action=download&id=1"},
		{"second Categories", releases[1].Categories[1], indexer.MoviesHD.ID},
		{"second DownloadVolumeFactor", *releases[1].DownloadVolumeFactor, 0.0},
		{"second UploadVolumeFactor", *releases[1].UploadVolumeFactor, 0.0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestResponseErrors(t *testing.T) {
	tests := []struct {
		name string
		id   string
		resp func() *http.Response
		want string
	}{
		{"status", "brokenstones", func() *http.Response {
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"error":"bad request"}`))}
		}, "Unexpected response status 400 code from indexer request: bad request"},
		{"content type", "brokenstones", func() *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<html>"))}
		}, "Unexpected response header text/html from indexer request, expected application/json"},
		{"gpw login redirect", "greatposterwall", func() *http.Response {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/login.php"}}, Body: http.NoBody}
		}, "We are being redirected to the login page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.id, Settings{Cookie: "a=b", UserPassSettings: userPass.UserPassSettings}, func(*http.Request) *http.Response { return tt.resp() })
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = %q", err, tt.want)
			}
		})
	}
}

func TestRetryDownload(t *testing.T) {
	site, err := newSite(brokenStones, Settings{UserPassSettings: userPass.UserPassSettings, UseFreeleechToken: TokenPreferred})
	if err != nil {
		t.Fatal(err)
	}
	link := "https://brokenstones.is/torrents.php?action=download&id=1&usetoken=1"
	tests := []struct {
		name string
		link string
		data string
		want string
	}{
		{"torrent", link, "d8:announce", ""},
		{"no tokens", link, "<p>You do not have any freeleech tokens left.</p>", "https://brokenstones.is/torrents.php?action=download&id=1"},
		{"other page", link, "<p>Error</p>", ""},
		{"no token used", "https://brokenstones.is/torrents.php?action=download&id=1", "This torrent is too large.", ""},
	}
	for _, tt := range tests {
		got, ok := site.RetryDownload(tt.link, []byte(tt.data))
		if ok != (tt.want != "") || got != tt.want {
			t.Errorf("%s: RetryDownload() = %q, %v, want = %q", tt.name, got, ok, tt.want)
		}
	}
}
