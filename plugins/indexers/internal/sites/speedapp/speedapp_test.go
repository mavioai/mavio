package speedapp

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

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

var testSettings = Settings{Email: "me@example.com", Password: "secret"}

func newIndexer(t *testing.T, id string, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	i := slices.IndexFunc(Definitions, func(d indexer.Definition) bool { return d.ID == id })
	if i < 0 {
		t.Fatalf("no definition %q", id)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := Definitions[i].New(data)
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

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		s    Settings
		want string
	}{
		{"valid", testSettings, ""},
		{"no email", Settings{Password: "p"}, "email: must not be empty"},
		{"no password", Settings{Email: "e"}, "password: must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ""
			if err := tt.s.Validate(); err != nil {
				got = err.Error()
			}
			if got != tt.want {
				t.Errorf("Validate() = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name, id string
		s        Settings
		q        indexer.Query
		want     string
	}{
		{
			"rss", "speedapp", testSettings,
			indexer.Query{},
			"https://speedapp.io/api/torrent?itemsPerPage=100&sort=torrent.createdAt&direction=desc&search=",
		},
		{
			"movie by IMDb", "speedapp", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "ignored", IMDbID: "123", Categories: []int{2040}},
			"https://speedapp.io/api/torrent?itemsPerPage=100&sort=torrent.createdAt&direction=desc&imdbId=tt0000123&categories%5B%5D=8&categories%5B%5D=29",
		},
		{
			"tv paged", "speedapp", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Show Name", Season: indexer.Ptr(2), Episode: "3", Limit: 20, Offset: 40},
			"https://speedapp.io/api/torrent?itemsPerPage=20&sort=torrent.createdAt&direction=desc&page=3&search=Show+Name&season=2&episode=3",
		},
		{
			"legacy link", "retroflix",
			Settings{BaseSettings: indexer.BaseSettings{BaseURL: "https://retroflix.net/"}, Email: "e", Password: "p"},
			indexer.Query{Term: "a b", Categories: []int{7000}},
			"https://retroflix.club/api/torrent?itemsPerPage=100&sort=torrent.createdAt&direction=desc&search=a+b&categories%5B%5D=409",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.id, tt.s, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

const torrents = `[{
	"download_volume_factor": 0, "upload_volume_factor": 1,
	"url": "https://speedapp.io/browse/1", "id": 1, "name": "Some.Movie.2020.1080p.WEB-DL [REQUESTED]. ",
	"category": {"id": 8, "name": "Movies: HD"}, "size": 1073741824,
	"created_at": "2021-05-01T10:20:30+03:00", "times_completed": 5, "leechers": 2, "seeders": 7,
	"short_description": "Some Movie", "poster": "https://speedapp.io/p.jpg", "imdb_id": "tt1234567"
}, {
	"url": "https://speedapp.io/browse/2", "id": 2, "name": "Other Release",
	"category": {"id": 5}, "size": 10, "created_at": "2021-05-01T09:00:00+00:00", "imdb_id": null
}]`

func TestLoginAndParse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, "speedapp", testSettings, func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/api/login":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), `{"username":"me@example.com","password":"secret"}`; got != want {
					t.Errorf("login body = %s, want = %s", got, want)
				}
				if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
					t.Errorf("login Content-Type = %q, want = %q", got, want)
				}
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			case "/api/torrent":
				if r.Header.Get("Authorization") != "Bearer tok" {
					return jsonResponse(http.StatusUnauthorized, `{}`)
				}
				return jsonResponse(http.StatusOK, torrents)
			}
			return jsonResponse(http.StatusNotFound, "")
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
		if err != nil {
			t.Fatal(err)
		}
		if logins != 1 || len(releases) != 2 {
			t.Fatalf("logins, releases = %d, %d, want = 1, 2", logins, len(releases))
		}
		r := releases[0]
		for _, c := range []struct {
			name      string
			got, want any
		}{
			{"Title", r.Title, "Some.Movie.2020.1080p.WEB-DL"},
			{"GUID", r.GUID, "https://speedapp.io/browse/1"},
			{"InfoURL", r.InfoURL, "https://speedapp.io/browse/1"},
			{"DownloadURL", r.DownloadURL, "https://speedapp.io/api/torrent/1/download"},
			{"Description", r.Description, "Some Movie"},
			{"PosterURL", r.PosterURL, "https://speedapp.io/p.jpg"},
			{"PublishDate", r.PublishDate, time.Date(2021, 5, 1, 7, 20, 30, 0, time.UTC)},
			{"Size", *r.Size, int64(1073741824)},
			{"Grabs", *r.Grabs, 5},
			{"Seeders", *r.Seeders, 7},
			{"Peers", *r.Peers, 9},
			{"IMDbID", r.IMDbID, 1234567},
			{"Categories", r.Categories, []int{indexer.MoviesHD.ID, 100008}},
			{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
			{"MinimumRatio", *r.MinimumRatio, 1.0},
			{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
			{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		} {
			if !equal(c.got, c.want) {
				t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
			}
		}
		if got := releases[1].IMDbID; got != 0 {
			t.Errorf("IMDbID = %d, want = 0", got)
		}
	})
}

func equal(a, b any) bool {
	if x, ok := a.([]int); ok {
		y, ok := b.([]int)
		return ok && slices.Equal(x, y)
	}
	if x, ok := a.(time.Time); ok {
		y, ok := b.(time.Time)
		return ok && x.Equal(y)
	}
	return a == b
}

func TestFilterByQuery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, "speedapp", testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/api/login" {
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			}
			return jsonResponse(http.StatusOK, torrents)
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{Term: "other release"})
		if err != nil {
			t.Fatal(err)
		}
		if len(releases) != 1 || releases[0].Title != "Other Release" {
			t.Errorf("Search() = %d releases, want = only Other Release", len(releases))
		}
	})
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		wantErr     string
	}{
		{"server error", http.StatusInternalServerError, "application/json", "500"},
		{"bad request", http.StatusBadRequest, "application/json", "Unexpected response status 400"},
		{"html", http.StatusOK, "text/html", "Unexpected response header text/html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, "speedapp", testSettings, func(r *http.Request) *http.Response {
					if r.URL.Path == "/api/login" {
						return jsonResponse(http.StatusOK, `{"token":"tok"}`)
					}
					resp := jsonResponse(tt.status, "[]")
					resp.Header.Set("Content-Type", tt.contentType)
					return resp
				})
				_, err := ix.Search(context.Background(), &indexer.Query{})
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Search() error = %v, want = %q", err, tt.wantErr)
				}
			})
		})
	}
}

func TestLoginFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, "speedapp", testSettings, func(*http.Request) *http.Response {
			return jsonResponse(http.StatusUnauthorized, `{}`)
		})
		if err := ix.Test(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
			t.Errorf("Test() error = %v, want = HTTP 401", err)
		}
	})
}

func TestDownload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, "retroflix", testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/api/login" {
				logins++
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			}
			if r.Header.Get("Authorization") != "Bearer tok" {
				return jsonResponse(http.StatusUnauthorized, "")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infod"))}
		})
		data, err := ix.Download(context.Background(), "https://retroflix.club/api/torrent/1/download")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "d4:infod" || logins != 1 {
			t.Errorf("Download() = %q with %d logins, want = %q with 1", data, logins, "d4:infod")
		}
	})
}
