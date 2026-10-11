package torrentsyndikat

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
	return &indexer.Indexer{ID: "1", Name: "TorrentSyndikat", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{APIKey: "key"}

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

const api = "https://torrent-syndikat.org/api_9djWe8Tb2NE3p6opyqnh/v1/browse.php?"

func TestSearchRequests(t *testing.T) {
	typed := testSettings
	typed.ProductsOnly = true
	typed.ReleaseTypes = []ReleaseType{ReleaseP2P, ReleaseOScene}
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"recent", testSettings, indexer.Query{}, api + "apikey=key&limit=50&ponly=false"},
		{
			"movie by IMDb ID, typed", typed,
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "tt0076759", Categories: []int{2040}},
			api + "apikey=key&limit=50&ponly=true&release_type=P2P%2CO-Scene&imdbId=tt0076759&cats=9%2C20",
		},
		{
			"season", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Categories: []int{5040}},
			api + "apikey=key&limit=50&ponly=false&searchstring=The+Show+S01*&cats=53%2C54%2C55%2C56",
		},
		{
			"episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2"},
			api + "apikey=key&limit=50&ponly=false&searchstring=The+Show+S01E02",
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

const results = `{"rows":[
{"id":"321","name":"Der.Film.2020.German.1080p.BluRay.x264-GRP","category":9,"added":1700000000,"size":8589934592,"numfiles":3,
 "seeders":25,"leechers":2,"snatched":"40","imdbId":76759,"tvdbId":null,"tmdbId":"11","poster":"/posters/321.jpg",
 "title":"Der Film","title_origin":"The Film","year":2020,"pid":77,"release_type":"Scene","tags":["German","DL"],"genres":["Action",1]},
{"id":"322","name":"Plain","category":"53","added":"1700000001","size":100,"numfiles":1,"seeders":1,"leechers":0,"snatched":0}
]}`

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(results))
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
		{"Title", r.Title, "Der.Film.2020.German.1080p.BluRay.x264-GRP"},
		{"GUID", r.GUID, "https://torrent-syndikat.org/details.php?id=321"},
		{"DownloadURL", r.DownloadURL, "https://torrent-syndikat.org/download.php?id=321&apikey=key"},
		{"PosterURL", r.PosterURL, "https://torrent-syndikat.org/posters/321.jpg"},
		{"PublishDate", r.PublishDate.Unix(), int64(1700000000)},
		{"Size", *r.Size, int64(8589934592)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 40},
		{"Seeders", *r.Seeders, 25},
		{"Peers", *r.Peers, 27},
		{"IMDbID", r.IMDbID, 76759},
		{"TVDBID", r.TVDBID, 0},
		{"TMDBID", r.TMDBID, 11},
		{"Category", r.Categories[0], indexer.MoviesHD.ID},
		{"Description", r.Description, "Title: Der Film\nOriginal Title: The Film\nYear: 2020\nProduct-Link: https://torrent-syndikat.org/product.php?pid=77\nGenres: Action, 1\nTags: German, DL\nRelease Type: Scene"},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(345600)},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"second Description", r2.Description, ""},
		{"second PosterURL", r2.PosterURL, ""},
		{"second Category", r2.Categories[0], indexer.TVHD.ID},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseError(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(`{"error":"invalid key"}`))
	if _, err := ix.Search(context.Background(), &indexer.Query{}); err == nil {
		t.Errorf("err = nil, want = an error")
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{}).Validate(); err == nil {
		t.Errorf("Validate() = nil, want = an api_key error")
	}
}
