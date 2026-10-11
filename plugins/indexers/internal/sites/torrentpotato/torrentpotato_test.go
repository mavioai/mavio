package torrentpotato

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
	return &indexer.Indexer{ID: "1", Name: "TorrentPotato", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{BaseURL: "https://potato.example.org/api", User: "some user", Passkey: "abc123"}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{"rss", indexer.Query{}, "https://potato.example.org/api?passkey=abc123&user=some%20user&search=%20"},
		{"basic", indexer.Query{Term: "Star Wars: A New Hope"}, "https://potato.example.org/api?passkey=abc123&user=some%20user&search=Star%20Wars%3A%20A%20New%20Hope"},
		{
			"movie with IMDb ID",
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "76759"},
			"https://potato.example.org/api?passkey=abc123&user=some%20user&imdbid=tt0076759&search=Star%20Wars",
		},
		{"tv", indexer.Query{Type: indexer.SearchTV, Term: "Show"}, ""},
		{"music", indexer.Query{Type: indexer.SearchMusic, Term: "Artist"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if r := c.First(); r != nil {
				got = r.URL
			}
			if got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

const page = `{"total_results":2,"results":[
{"release_name":"Old.Movie.2001.720p.BluRay.x264-GRP","torrent_id":"1","details_url":"https://potato.example.org/t/1","download_url":"https://potato.example.org/dl/1.torrent","imdb_id":"tt0123456","freeleech":false,"type":"movie","size":"4400","leechers":2,"seeders":10,"publish_date":"2020-01-02 03:04:05"},
{"release_name":"Movie &amp; Friends 2020 1080p","torrent_id":"2","details_url":"https://potato.example.org/t/2","download_url":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=x","imdb_id":"","freeleech":true,"type":"movie","size":8000,"leechers":0,"seeders":3,"publish_date":"2021-05-06T07:08:09+02:00"}
]}`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie, Term: "movie"})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r, old := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Movie & Friends 2020 1080p"},
		{"GUID", r.GUID, "potato-0123456789abcdef0123456789abcdef01234567"},
		{"DownloadURL", r.DownloadURL, "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=x"},
		{"InfoURL", r.InfoURL, "https://potato.example.org/t/2"},
		{"Size", *r.Size, int64(8000000000)},
		{"Seeders", *r.Seeders, 3},
		{"Peers", *r.Peers, 3},
		{"PublishDate", r.PublishDate, time.Date(2021, 5, 6, 5, 8, 9, 0, time.UTC)},
		{"IMDbID", r.IMDbID, 0},
		{"Categories", r.Categories[0], indexer.Movies.ID},
		{"old GUID", old.GUID, "potato-https://potato.example.org/dl/1.torrent"},
		{"old IMDbID", old.IMDbID, 123456},
		{"old Size", *old.Size, int64(4400000000)},
		{"old Peers", *old.Peers, 12},
		{"old PublishDate", old.PublishDate, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"error", http.StatusOK, `{"error":"Invalid passkey"}`, "Indexer API call returned an error [Invalid passkey]"},
		{"status", http.StatusForbidden, `{}`, "Indexer API call returned an unexpected StatusCode [403]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, func(*http.Request) *http.Response {
				return &http.Response{StatusCode: tt.status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			})
			_, err := ix.Search(context.Background(), &indexer.Query{Term: "x"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = containing %q", err, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		s    Settings
		want string
	}{
		{"valid", testSettings, ""},
		{"no user", Settings{BaseURL: "http://127.0.0.1", Passkey: "x"}, "user"},
		{"no passkey", Settings{BaseURL: "http://127.0.0.1", User: "x"}, "passkey"},
		{"bad URL", Settings{BaseURL: "potato.example.org", User: "x", Passkey: "y"}, "base_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validate()
			got := ""
			if err != nil {
				got = err.Error()
			}
			if (tt.want == "") != (err == nil) || !strings.Contains(got, tt.want) {
				t.Errorf("Validate() = %v, want = containing %q", err, tt.want)
			}
		})
	}
}
