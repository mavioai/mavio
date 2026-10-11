package scenehd

import (
	"context"
	"encoding/json"
	"errors"
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
	return &indexer.Indexer{ID: "1", Name: "SceneHD", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

const passkey = "0123456789abcdef0123456789abcdef"

var testSettings = Settings{Passkey: passkey}

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{
			"recent",
			indexer.Query{},
			"https://scenehd.org/browse.php?api=&passkey=" + passkey + "&search=",
		},
		{
			"movie by IMDb ID and term",
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars", IMDbID: "76759", Categories: []int{2040}},
			"https://scenehd.org/browse.php?api=&passkey=" + passkey + "&search=tt0076759+Star+Wars&cat=1%2C4",
		},
		{
			"episode",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2", Categories: []int{5000}},
			"https://scenehd.org/browse.php?api=&passkey=" + passkey + "&search=The+Show+S01E02&cat=6%2C5%2C7",
		},
		{
			"unmapped categories",
			indexer.Query{Term: "x", Categories: []int{7000}},
			"https://scenehd.org/browse.php?api=&passkey=" + passkey + "&search=x&cat=",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("Accept"); got != indexer.AcceptJSON {
				t.Errorf("Accept = %q, want = %q", got, indexer.AcceptJSON)
			}
		})
	}
}

const results = `[
{"id":"1234","name":"The.Movie.2020.1080p.BluRay.x264-GRP","added":"2023-05-06 07:08:09","is_freeleech":"1","category":"1","size":"8589934592","times_completed":"12","numfiles":"3","seeders":"25","leechers":"2","imdbid":"tt0076759"},
{"id":1235,"name":"Other","added":"2023-05-06 07:08:10","is_freeleech":0,"category":9,"size":100,"times_completed":0,"numfiles":1,"seeders":1,"leechers":0,"imdbid":null}
]`

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
		{"Title", r.Title, "The.Movie.2020.1080p.BluRay.x264-GRP"},
		{"GUID", r.GUID, "https://scenehd.org/details.php?id=1234"},
		{"DownloadURL", r.DownloadURL, "https://scenehd.org/download.php?id=1234&passkey=" + passkey},
		{"PublishDate", r.PublishDate, time.Date(2023, 5, 6, 7, 8, 9, 0, time.Local)},
		{"Size", *r.Size, int64(8589934592)},
		{"Grabs", *r.Grabs, 12},
		{"Files", *r.Files, 3},
		{"Seeders", *r.Seeders, 25},
		{"Peers", *r.Peers, 27},
		{"IMDbID", r.IMDbID, 76759},
		{"Category", r.Categories[0], indexer.MoviesHD.ID},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(0)},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
		{"second Category", r2.Categories[0], indexer.Other.ID},
		{"second IMDbID", r2.IMDbID, 0},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestInvalidPasskey(t *testing.T) {
	ix := newIndexer(t, testSettings, respond("User not found or passkey not set"))
	_, err := ix.Search(context.Background(), &indexer.Query{})
	if !errors.Is(err, indexer.ErrAuth) || !strings.Contains(err.Error(), "The passkey is invalid") {
		t.Errorf("err = %v, want = an authentication error", err)
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{Passkey: "short"}).Validate(); err == nil || !strings.Contains(err.Error(), "32") {
		t.Errorf("Validate() = %v, want = a length error", err)
	}
}
