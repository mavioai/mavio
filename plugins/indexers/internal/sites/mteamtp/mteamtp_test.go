package mteamtp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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
	return &indexer.Indexer{ID: "1", Name: "M-Team - TP", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{APIKey: "key"}

func respond(status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSearchRequests(t *testing.T) {
	free := testSettings
	free.FreeleechOnly = true
	free.BaseURL = "https://tp.m-team.cc/"
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     []string
	}{
		{
			"RSS", testSettings,
			indexer.Query{},
			[]string{`{"mode":"normal","categories":[],"pageNumber":1,"pageSize":100}`},
		},
		{
			"basic with adult categories", free,
			indexer.Query{Term: "a  b", Categories: []int{2040, 6000}},
			[]string{
				`{"mode":"normal","categories":["419","439"],"discount":"FREE","keyword":"a  b","pageNumber":1,"pageSize":100}`,
				`{"mode":"adult","categories":["410","424","437","431","429","430","426","432","436","440","425","433","411","412","413"],"discount":"FREE","keyword":"a  b","pageNumber":1,"pageSize":100}`,
			},
		},
		{
			"basic with only adult categories", testSettings,
			indexer.Query{Categories: []int{6080}},
			[]string{`{"mode":"adult","categories":["424","430"],"pageNumber":1,"pageSize":100}`},
		},
		{
			"movie with only adult categories", testSettings,
			indexer.Query{Type: indexer.SearchMovie, IMDbID: "tt0133093", Categories: []int{6000}},
			nil,
		},
		{
			"movie by IMDb ID", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "The Matrix", IMDbID: "tt0133093"},
			[]string{`{"mode":"normal","categories":[],"imdb":"tt0133093","keyword":"The Matrix","pageNumber":1,"pageSize":100}`},
		},
		{
			"episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1), Episode: "2", Categories: []int{5040}},
			[]string{`{"mode":"normal","categories":["402","438"],"keyword":"Show S01E02","pageNumber":1,"pageSize":100}`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, tier := range c.Tiers() {
				for _, pages := range tier {
					for r := range pages {
						if r.URL != "https://api.m-team.cc/api/torrent/search" || r.Method != http.MethodPost {
							t.Errorf("request = %s %s, want = POST https://api.m-team.cc/api/torrent/search", r.Method, r.URL)
						}
						for k, v := range map[string]string{"x-api-key": "key", "Content-Type": "application/json", "Accept": "application/json", "User-Agent": UserAgent} {
							if r.Header.Get(k) != v {
								t.Errorf("%s = %q, want = %q", k, r.Header.Get(k), v)
							}
						}
						got = append(got, string(r.Body))
					}
				}
			}
			if !equal(got, tt.want) {
				t.Errorf("bodies = %q, want = %q", got, tt.want)
			}
		})
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

const feed = `{"code":"0","message":"SUCCESS","data":{"pageNumber":"1","pageSize":"100","total":"2","data":[
{"id":"100","name":"Old  Movie\t2001 1080p ","smallDescr":"An old movie","category":"419","numfiles":"3","imdb":"tt0133093","size":"8589934592",
 "status":{"createdDate":"2023-01-02 11:04:05","discount":"_2x_free","timesCompleted":"12","seeders":"30","leechers":"2"}},
{"id":"101","name":"New Show S01E01","smallDescr":"","category":"402","numfiles":"1","size":"1073741824",
 "status":{"createdDate":"2023-05-06 07:08:09","discount":"PERCENT_70","timesCompleted":"0","seeders":"5","leechers":"1"}}
]}}`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return respond(http.StatusOK, "application/json", feed) })
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	show, movie := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", movie.Title, "Old Movie 2001 1080p"},
		{"GUID", movie.GUID, "https://kp.m-team.cc/detail/100"},
		{"InfoURL", movie.InfoURL, "https://kp.m-team.cc/detail/100"},
		{"DownloadURL", movie.DownloadURL, "https://api.m-team.cc/api/torrent/genDlToken?id=100"},
		{"Description", movie.Description, "An old movie"},
		{"Categories", movie.Categories, []int{2040, 100419}},
		{"Files", *movie.Files, 3},
		{"Size", *movie.Size, int64(8589934592)},
		{"Grabs", *movie.Grabs, 12},
		{"Seeders", *movie.Seeders, 30},
		{"Peers", *movie.Peers, 32},
		{"DownloadVolumeFactor", *movie.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *movie.UploadVolumeFactor, 2.0},
		{"MinimumRatio", *movie.MinimumRatio, 1.0},
		{"MinimumSeedTime", *movie.MinimumSeedTime, int64(172800)},
		{"PublishDate", movie.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"show DownloadVolumeFactor", *show.DownloadVolumeFactor, 0.3},
		{"show UploadVolumeFactor", *show.UploadVolumeFactor, 1.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		want                    string
	}{
		{"status", "application/json", "{}", http.StatusForbidden, "Unexpected response status 403"},
		{"content type", "text/html", "{}", http.StatusOK, "Unexpected response header text/html"},
		{"not JSON", "application/json", "<html>", http.StatusOK, "Invalid response received from M-Team, not a valid JSON"},
		{"message", "application/json", `{"code":"1","message":"key is invalid","data":null}`, http.StatusOK, "Response from API: key is invalid"},
		{"bad number", "application/json", `{"message":"SUCCESS","data":{"data":[{"id":"x","status":{}}]}}`, http.StatusOK, "mteamtp: id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return respond(tt.status, tt.contentType, tt.body) })
			_, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want = containing %q", err, tt.want)
			}
		})
	}
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
		return respond(http.StatusOK, "application/json", `{"code":"0","message":"SUCCESS","data":null}`)
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
	if err != nil || len(releases) != 0 {
		t.Errorf("Search() = %d, %v, want = 0, nil", len(releases), err)
	}
}

func TestDownload(t *testing.T) {
	var calls []string
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		calls = append(calls, r.Method+" "+r.URL.String())
		switch r.URL.Host {
		case "api.m-team.cc":
			if r.Header.Get("x-api-key") != "key" || r.Header.Get("User-Agent") != UserAgent {
				return respond(http.StatusUnauthorized, "application/json", `{}`)
			}
			return respond(http.StatusOK, "application/json", `{"code":"0","message":"SUCCESS","data":"https://dl.m-team.cc/torrent?token=abc"}`)
		case "dl.m-team.cc":
			if r.Header.Get("Cookie") != "" || r.Header.Get("User-Agent") != UserAgent {
				t.Errorf("Cookie, User-Agent = %q, %q, want = none, %q", r.Header.Get("Cookie"), r.Header.Get("User-Agent"), UserAgent)
			}
			return respond(http.StatusOK, "application/x-bittorrent", "d4:infod")
		}
		return respond(http.StatusNotFound, "text/plain", "")
	})
	ix.SetCookies(map[string]string{"a": "b"}, time.Time{})
	data, err := ix.Download(context.Background(), "https://api.m-team.cc/api/torrent/genDlToken?id=100")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"POST https://api.m-team.cc/api/torrent/genDlToken?id=100", "GET https://dl.m-team.cc/torrent?token=abc"}
	if string(data) != "d4:infod" || !equal(calls, want) {
		t.Errorf("Download() = %q after %q, want = %q after %q", data, calls, "d4:infod", want)
	}
}

func TestDownloadErrors(t *testing.T) {
	tests := []struct{ body, want string }{
		{"<html>", "not a valid JSON"},
		{`{"data":""}`, "unable to find download link for: https://api.m-team.cc/api/torrent/genDlToken?id=1"},
	}
	for _, tt := range tests {
		ix := newIndexer(t, testSettings, func(*http.Request) *http.Response { return respond(http.StatusOK, "application/json", tt.body) })
		_, err := ix.Download(context.Background(), "https://api.m-team.cc/api/torrent/genDlToken?id=1")
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Download() error = %v, want = containing %q", err, tt.want)
		}
	}
}

func TestAPIURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://kp.m-team.cc/": "https://api.m-team.cc/",
		"https://xp.m-team.io/": "https://api.m-team.io/",
	} {
		if got := apiURL(in); got != want {
			t.Errorf("apiURL(%q) = %q, want = %q", in, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Errorf("New() error = %v, want = an api_key error", err)
	}
}
