package filelist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
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
	return &indexer.Indexer{ID: "1", Name: "FileList", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{Username: "someuser", Passkey: "somepass"}

func TestFileListCases(t *testing.T) {
	ported.Run(t, "file_list_tests/file_list.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_FileList": "TestRecentFeed",
	}})
	ported.Run(t, "file_list_tests/file_list_request_generator.json", ported.Cases{Facts: map[string]string{
		"should_use_categories_for_feed":                   "TestSearchRequests",
		"should_not_search_by_imdbid_if_not_supported":     "TestSearchRequests",
		"should_search_by_name_and_year_if_missing_imdbid": "TestSearchRequests",
	}})
}

func respond(contentType string, body []byte) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(string(body)))}
	}
}

func TestRecentFeed(t *testing.T) {
	ix := newIndexer(t, testSettings, respond("application/json", ported.Response(t, "FileList/recentfeed.json")))
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000}})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 4 {
		t.Fatalf("releases = %d, want = 4", len(releases))
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Storming.Juno.2010.1080p.BluRay.x264-GUACAMOLE"},
		{"Protocol", r.Protocol, indexer.ProtocolTorrent},
		{"DownloadURL", r.DownloadURL, "https://filelist.io/download.php?id=665873&passkey=somepass"},
		{"InfoURL", r.InfoURL, "https://filelist.io/details.php?id=665873"},
		{"CommentsURL", r.CommentsURL, ""},
		{"PublishDate", r.PublishDate, time.Date(2020, 1, 25, 19, 20, 19, 0, time.UTC)},
		{"Size", *r.Size, int64(8300512414)},
		{"InfoHash", r.InfoHash, ""},
		{"MagnetURL", r.MagnetURL, ""},
		{"Peers", *r.Peers, 2 + 12},
		{"Seeders", *r.Seeders, 12},
		{"IMDbID", r.IMDbID, 1776305},
		{"Genres", strings.Join(r.Genres, "|"), "History|War"},
		{"Categories", r.Categories[0], indexer.MoviesHD.ID},
		{"third DownloadVolumeFactor", *releases[2].DownloadVolumeFactor, 0.0},
		{"any Internal", slices.ContainsFunc(releases, func(r *indexer.Release) bool { return r.HasFlag(indexer.FlagInternal) }), true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestFreeleechOnly(t *testing.T) {
	s := testSettings
	s.FreeleechOnly = true
	ix := newIndexer(t, s, respond("application/json", ported.Response(t, "FileList/recentfeed.json")))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range releases {
		if *r.DownloadVolumeFactor != 0 {
			t.Errorf("%s DownloadVolumeFactor = %v, want = 0", r.Title, *r.DownloadVolumeFactor)
		}
	}
	if len(releases) == 0 || len(releases) == 4 {
		t.Errorf("releases = %d, want = only the freeleech ones", len(releases))
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		s    Settings
		q    indexer.Query
		want string
	}{
		// Prowlarr's test maps only 1 and 2, in that order; the site's
		// categories map 2 first.
		{
			"feed categories", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Categories: []int{indexer.MoviesSD.ID, indexer.MoviesDVD.ID}},
			"https://filelist.io/api.php?action=latest-torrents&category=2%2C1",
		},
		{
			"movie IMDb", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "0076759", Categories: []int{2000}},
			"https://filelist.io/api.php?action=search-torrents&type=imdb&query=tt0076759&category=25%2C6%2C26%2C20%2C2%2C3%2C4%2C19%2C1%2C28",
		},
		{
			"movie name",
			Settings{Username: "u", Passkey: "p", FreeleechOnly: true},
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars"},
			"https://filelist.io/api.php?action=search-torrents&type=name&query=Star+Wars&freeleech=1",
		},
		{
			"tv episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1), Episode: "2"},
			"https://filelist.io/api.php?action=search-torrents&season=1&episode=2&type=name&query=Show",
		},
		{
			"tv IMDb", testSettings,
			indexer.Query{Type: indexer.SearchTV, IMDbID: "tt123", Season: indexer.Ptr(1)},
			"https://filelist.io/api.php?action=search-torrents&season=1&type=imdb&query=tt0000123",
		},
		{
			"tv daily", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Late Show", Season: indexer.Ptr(2023), Episode: "01/03"},
			"https://filelist.io/api.php?action=search-torrents&type=name&query=Late+Show+2023.01.03",
		},
		{"tv daily by ID", testSettings, indexer.Query{Type: indexer.SearchTV, IMDbID: "tt1", Season: indexer.Ptr(2023), Episode: "01/03"}, ""},
		{
			"tv rss",
			Settings{BaseSettings: indexer.BaseSettings{BaseURL: "https://filelist.ro/"}, Username: "u", Passkey: "p"},
			indexer.Query{Type: indexer.SearchTV, Season: indexer.Ptr(1)},
			"https://filelist.io/api.php?action=latest-torrents",
		},
		{
			"music", testSettings,
			indexer.Query{Type: indexer.SearchMusic, Term: "x"},
			"https://filelist.io/api.php?action=search-torrents&type=name&query=x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.s, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			got := ""
			if r != nil {
				got = r.URL
			}
			if got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
	ix := newIndexer(t, Settings{Username: " someuser ", Passkey: "somepass"}, nil)
	c, err := ix.Site.SearchRequests(context.Background(), ix, &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.First().Header.Get("Authorization"), "Basic c29tZXVzZXI6c29tZXBhc3M="; got != want {
		t.Errorf("Authorization = %q, want = %q", got, want)
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		wantErr                 string
	}{
		{"error", "application/json", `{"error":"Invalid passkey"}`, http.StatusOK, "Unexpected response from indexer request: Invalid passkey"},
		{"html", "text/html", "<html>", http.StatusOK, "Unexpected response header text/html"},
		{"status", "application/json", "", http.StatusForbidden, "Unexpected response status 403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
				return &http.Response{StatusCode: tt.status, Header: http.Header{"Content-Type": {tt.contentType}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			})
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Search() error = %v, want = %q", err, tt.wantErr)
			}
		})
	}
}
