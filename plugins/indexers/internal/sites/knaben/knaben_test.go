package knaben

import (
	"context"
	"io"
	"net/http"
	"slices"
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

func newIndexer(t *testing.T, h handler) *indexer.Indexer {
	t.Helper()
	site, err := Definition.New([]byte(`{"base_url":"https://knaben.eu/"}`))
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
	return &indexer.Indexer{ID: "1", Name: "Knaben", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{
			"rss",
			indexer.Query{},
			"{\n  \"order_by\": \"date\",\n  \"order_direction\": \"desc\",\n  \"from\": 0,\n  \"size\": 100,\n  \"hide_unsafe\": true\n}",
		},
		{
			"tv episode in categories",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2", Categories: []int{indexer.TVHD.ID, indexer.TVOther.ID}},
			"{\n  \"order_by\": \"date\",\n  \"order_direction\": \"desc\",\n  \"from\": 0,\n  \"size\": 100,\n  \"hide_unsafe\": true,\n" +
				"  \"search_type\": \"100%\",\n  \"search_field\": \"title\",\n  \"query\": \"The Show S01E02\",\n" +
				"  \"categories\": [\n    2001000,\n    2007000,\n    2008000\n  ]\n}",
		},
		{
			"movie with IMDb ID searches by title",
			indexer.Query{Type: indexer.SearchMovie, Term: "Film & Co", IMDbID: "tt0076759"},
			"{\n  \"order_by\": \"date\",\n  \"order_direction\": \"desc\",\n  \"from\": 0,\n  \"size\": 100,\n  \"hide_unsafe\": true,\n" +
				"  \"search_type\": \"100%\",\n  \"search_field\": \"title\",\n  \"query\": \"Film  Co\"\n}",
		},
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
			if r.URL != "https://api.knaben.org/v1" || r.Method != http.MethodPost {
				t.Errorf("request = %s %s, want = POST https://api.knaben.org/v1", r.Method, r.URL)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want = application/json", got)
			}
			if got := string(r.Body); got != tt.want {
				t.Errorf("body = %s, want = %s", got, tt.want)
			}
		})
	}
}

const page = `{"total":{"relation":"eq","value":3},"hits":[
{"title":"The.Show.S01E02.1080p.WEB.h264-GRP","categoryId":[2001000,2000000],"hash":"0123456789abcdef0123456789abcdef01234567","details":"https://example.org/details/1","link":"","magnetUrl":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567","bytes":1610612736,"seeders":12,"peers":3,"date":"2024-03-01T10:00:00"},
{"title":"Dead","categoryId":[2000000],"hash":"aa","details":"https://example.org/details/2","bytes":1,"seeders":0,"peers":1,"date":"2024-03-01T10:00:00+00:00"},
{"title":"Film 2020 2160p","categoryId":[3003000],"hash":"bb","details":"https://example.org/details/3","link":"https://example.org/dl/3.torrent","bytes":2048,"seeders":1,"peers":0,"date":"2024-02-01T08:30:00.123+00:00"}
]}`

func TestParse(t *testing.T) {
	ix := newIndexer(t, func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r, f := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "The.Show.S01E02.1080p.WEB.h264-GRP"},
		{"GUID", r.GUID, "https://example.org/details/1"},
		{"InfoURL", r.InfoURL, "https://example.org/details/1"},
		{"DownloadURL", r.DownloadURL, ""},
		{"MagnetURL", r.MagnetURL, "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"},
		{"InfoHash", r.InfoHash, "0123456789abcdef0123456789abcdef01234567"},
		{"Size", *r.Size, int64(1610612736)},
		{"Seeders", *r.Seeders, 12},
		{"Peers", *r.Peers, 15},
		{"PublishDate", r.PublishDate, time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"Categories", slices.Equal(r.Categories, []int{indexer.TVHD.ID, 2101000, indexer.TV.ID, 2100000}), true},
		{"film DownloadURL", f.DownloadURL, "https://example.org/dl/3.torrent"},
		{"film PublishDate", f.PublishDate, time.Date(2024, 2, 1, 8, 30, 0, 123000000, time.UTC)},
		{"film Categories", slices.Equal(f.Categories, []int{indexer.MoviesUHD.ID, 3103000}), true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if !r.HasFlag(indexer.FlagFreeleech) {
		t.Errorf("Flags = %v, want = freeleech", r.Flags)
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		want                    string
	}{
		{"html", "text/html", "<html>", http.StatusOK, "Unexpected response header text/html"},
		{"status", "application/json", "{}", http.StatusForbidden, "Unexpected response status 403"},
		{"bad date", "application/json", `{"hits":[{"title":"x","seeders":1,"date":"yesterday"}]}`, http.StatusOK, "unable to parse date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, func(*http.Request) *http.Response {
				return &http.Response{StatusCode: tt.status, Header: http.Header{"Content-Type": {tt.contentType}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			})
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = containing %q", err, tt.want)
			}
		})
	}
}
