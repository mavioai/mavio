package subsplease

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
	return &indexer.Indexer{ID: "1", Name: "SubsPlease", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name     string
		settings indexer.BaseSettings
		q        indexer.Query
		want     string
	}{
		{"rss", indexer.BaseSettings{}, indexer.Query{}, "https://subsplease.org/api/?tz=UTC&f=latest"},
		{"tv rss", indexer.BaseSettings{}, indexer.Query{Type: indexer.SearchTV, Season: indexer.Ptr(2)}, "https://subsplease.org/api/?tz=UTC&f=latest"},
		{
			"tv episode",
			indexer.BaseSettings{},
			indexer.Query{Type: indexer.SearchTV, Term: "Shingeki no Kyojin", Season: indexer.Ptr(2), Episode: "5"},
			"https://subsplease.org/api/?tz=UTC&f=search&s=Shingeki+no+Kyojin+S2+05",
		},
		{
			"first season",
			indexer.BaseSettings{},
			indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1), Episode: "12"},
			"https://subsplease.org/api/?tz=UTC&f=search&s=Show+12",
		},
		{
			"group and resolution",
			indexer.BaseSettings{BaseURL: "https://subsplease.nocensor.space/"},
			indexer.Query{Term: "[SubsPlease] One Piece 1080p"},
			"https://subsplease.org/api/?tz=UTC&f=search&s=One+Piece",
		},
		{
			"movie",
			indexer.BaseSettings{BaseURL: "https://subsplease.mrunblock.bond/"},
			indexer.Query{Type: indexer.SearchMovie, Term: "Suzume"},
			"https://subsplease.mrunblock.bond/api/?tz=UTC&f=search&s=Suzume",
		},
		{"music", indexer.BaseSettings{}, indexer.Query{Type: indexer.SearchMusic, Term: "x"}, ""},
		{"book", indexer.BaseSettings{}, indexer.Query{Type: indexer.SearchBook, Term: "x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
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

const feed = `{
"Suzume - Movie": {"time":"New","release_date":"Tue, 25 Apr 2023 16:31:20 +0000","show":"Suzume","episode":"Movie",
 "downloads":[{"res":"1080","magnet":"magnet:?xt=urn:btih:AAAA&dn=x&xl=3221225472&tr=t"}],"xdcc":"","image_url":"/wp-content/uploads/suzume.jpg","page":"suzume"},
"One Piece - 1059": {"time":"New","release_date":"Sun, 23 Apr 2023 02:06:08 +0000","show":"One Piece","episode":"1059",
 "downloads":[{"res":"480","magnet":"magnet:?xt=urn:btih:BBBB"},{"res":"720","magnet":"magnet:?xt=urn:btih:CCCC"},{"res":"1080","magnet":"magnet:?xt=urn:btih:DDDD"},{"res":"2160","magnet":""}],
 "xdcc":"","image_url":"","page":"one-piece"}
}`

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, indexer.BaseSettings{}, respond(feed))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 5 {
		t.Fatalf("releases = %d, want = 5", len(releases))
	}
	movie, sd := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", movie.Title, "[SubsPlease] Suzume - Movie (1080p)"},
		{"GUID", movie.GUID, "magnet:?xt=urn:btih:AAAA&dn=x&xl=3221225472&tr=t"},
		{"MagnetURL", movie.MagnetURL, "magnet:?xt=urn:btih:AAAA&dn=x&xl=3221225472&tr=t"},
		{"DownloadURL", movie.DownloadURL, ""},
		{"InfoURL", movie.InfoURL, "https://subsplease.org/shows/suzume/"},
		{"PosterURL", movie.PosterURL, "https://subsplease.org/wp-content/uploads/suzume.jpg"},
		{"Size", *movie.Size, int64(3221225472)},
		{"Categories", len(movie.Categories), 2},
		{"Category", movie.Categories[1], 2020},
		{"PublishDate", movie.PublishDate.UTC(), time.Date(2023, 4, 25, 16, 31, 20, 0, time.UTC)},
		{"Seeders", *movie.Seeders, 1},
		{"Peers", *movie.Peers, 2},
		{"Files", *movie.Files, 1},
		{"MinimumSeedTime", *movie.MinimumSeedTime, int64(172800)},
		{"DownloadVolumeFactor", *movie.DownloadVolumeFactor, 0.0},
		{"sd Title", sd.Title, "[SubsPlease] One Piece - 1059 (480p)"},
		{"sd Size", *sd.Size, int64(350 << 20)},
		{"sd PosterURL", sd.PosterURL, ""},
		{"sd Categories", len(sd.Categories), 1},
		{"hd Size", *releases[2].Size, int64(700 << 20)},
		{"fhd Size", *releases[3].Size, int64(1395864371)},
		{"uhd Size", *releases[4].Size, int64(1 << 30)},
		{"uhd GUID", releases[4].GUID, "https://subsplease.org/shows/one-piece/"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestEmpty(t *testing.T) {
	for _, body := range []string{"", "[]"} {
		ix := newIndexer(t, indexer.BaseSettings{}, respond(body))
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil || len(releases) != 0 {
			t.Errorf("Search() of %q = %d releases, %v, want = none", body, len(releases), err)
		}
	}
}
