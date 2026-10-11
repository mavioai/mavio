package torrentscsv

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

func newIndexer(t *testing.T, settings indexer.BaseSettings, h handler) *indexer.Indexer {
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
	return &indexer.Indexer{ID: "1", Name: "TorrentsCSV", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		s    indexer.BaseSettings
		q    indexer.Query
		want string
	}{
		{"basic", indexer.BaseSettings{}, indexer.Query{Term: "ubuntu 22.04"}, "https://torrents-csv.com/service/search?size=100&q=ubuntu+22.04"},
		{
			"legacy link",
			indexer.BaseSettings{BaseURL: "https://torrents-csv.ml/"},
			indexer.Query{Type: indexer.SearchMovie, Term: "Film (2020)", IMDbID: "tt0076759"},
			"https://torrents-csv.com/service/search?size=100&q=Film+(2020)",
		},
		{
			"tv season",
			indexer.BaseSettings{},
			indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(3)},
			"https://torrents-csv.com/service/search?size=100&q=Show+S03",
		},
		{"rss", indexer.BaseSettings{}, indexer.Query{}, ""},
		{"too short", indexer.BaseSettings{}, indexer.Query{Term: "ab"}, ""},
		{"music", indexer.BaseSettings{}, indexer.Query{Type: indexer.SearchMusic, Term: "artist"}, ""},
		{"book", indexer.BaseSettings{}, indexer.Query{Type: indexer.SearchBook, Term: "author"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.s, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
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

const page = `{"torrents":[
{"infohash":"0123456789abcdef0123456789abcdef01234567","name":"Old Release","size_bytes":1048576,"created_unix":1600000000,"seeders":3,"leechers":2,"completed":10},
{"infohash":"89abcdef0123456789abcdef0123456789abcdef","name":"New Release 1080p","size_bytes":4294967296,"created_unix":1700000000,"seeders":null,"leechers":5},
null
]}`

func TestParse(t *testing.T) {
	ix := newIndexer(t, indexer.BaseSettings{}, func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "release"})
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
		{"Title", r.Title, "New Release 1080p"},
		{"GUID", r.GUID, "magnet:?xt=urn:btih:89abcdef0123456789abcdef0123456789abcdef"},
		{"InfoURL", r.InfoURL, "https://torrents-csv.com/search?q=New Release 1080p"},
		{"InfoHash", r.InfoHash, "89abcdef0123456789abcdef0123456789abcdef"},
		{"DownloadURL", r.DownloadURL, ""},
		{"MagnetURL", r.MagnetURL, ""},
		{"PublishDate", r.PublishDate, time.Unix(1700000000, 0).UTC()},
		{"Size", *r.Size, int64(4294967296)},
		{"Seeders", *r.Seeders, 0},
		{"Peers", *r.Peers, 5},
		{"Grabs", *r.Grabs, 0},
		{"Categories", r.Categories[0], indexer.Other.ID},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"old Title", old.Title, "Old Release"},
		{"old Seeders", *old.Seeders, 3},
		{"old Peers", *old.Peers, 5},
		{"old Grabs", *old.Grabs, 10},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}
