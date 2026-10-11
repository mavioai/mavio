package torrentday

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	return &indexer.Indexer{ID: "1", Name: "TorrentDay", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{
	BaseSettings: indexer.BaseSettings{BaseURL: "https://www.torrentday.com/"},
	Cookie:       "uid=1; pass=abc",
}

func TestSearchRequests(t *testing.T) {
	free := testSettings
	free.FreeleechOnly = true
	all := "25;96;11;5;103;3;21;22;13;44;48;1;24;104;32;31;33;46;82;14;26;7;34;2;10;4;18;8;9;17;27;23;41;16;29;42;20;102;30;95;47;43;45;28;12;19;6;15"
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"RSS", testSettings, indexer.Query{}, "https://www.torrentday.com/t.json?" + all + ";q="},
		{
			"movie by IMDb ID", free,
			indexer.Query{Type: indexer.SearchMovie, Term: "The Matrix (1999)", IMDbID: "133093", Categories: []int{2045}},
			"https://www.torrentday.com/t.json?96;free;q=tt0133093+The+Matrix+(1999)",
		},
		{
			"season", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show!", Season: indexer.Ptr(2), Categories: []int{5045}},
			"https://www.torrentday.com/t.json?104;q=The+Show+S02",
		},
		{
			"basic keeps the term as is", testSettings,
			indexer.Query{Term: "a&b", Categories: []int{3000}},
			"https://www.torrentday.com/t.json?17;27;23;41;16;42;45;q=a%26b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Cookies["pass"]; got != "abc" {
				t.Errorf("cookie pass = %q, want = %q", got, "abc")
			}
		})
	}
}

const feed = `[
{"c":7,"name":"The.Show.S02E01.720p.HDTV.x264-GRP","t":4123456,"seeders":87,"leechers":4,"imdb-id":"tt1234567","download-multiplier":0,"ctime":1672628645,"size":1471026298,"files":3,"completed":1204},
{"c":"48","name":"A Film 2020 1080p x265-GRP","t":"4123457","seeders":"5","leechers":"1","imdb-id":"","ctime":"1672628000","size":"9126805504","files":"1","completed":"10"}
]`

func respond(status int, header http.Header, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(http.StatusOK, http.Header{"Content-Type": {"application/json"}}, feed))
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
		{"Title", r.Title, "The.Show.S02E01.720p.HDTV.x264-GRP"},
		{"GUID", r.GUID, "https://www.torrentday.com/details.php?id=4123456"},
		{"InfoURL", r.InfoURL, "https://www.torrentday.com/details.php?id=4123456"},
		{"DownloadURL", r.DownloadURL, "https://www.torrentday.com/download.php/4123456/4123456.torrent"},
		{"PublishDate", r.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"Categories", r.Categories, []int{5040, 100007}},
		{"Size", *r.Size, int64(1471026298)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"IMDbID", r.IMDbID, 1234567},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(259200)},
		{"second Categories", r2.Categories, []int{2000, 100048}},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
		{"second IMDbID", r2.IMDbID, 0},
		{"second Size", *r2.Size, int64(9126805504)},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

// TestSearchFollowsRedirects checks that searches follow redirects, as
// Prowlarr's IndexerRequests of URLs do in production.
func TestSearchFollowsRedirects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/t.json" {
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/moved.json"}}, Body: http.NoBody}
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(feed))}
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if len(releases) != 2 {
			t.Errorf("releases = %d, want = 2", len(releases))
		}
	})
}

// TestParseRedirects checks the errors of redirects the parser is given,
// which searches only return when redirects are not followed.
func TestParseRedirects(t *testing.T) {
	tests := []struct{ location, want string }{
		{"https://www.torrentday.com/login.php?returnto=%2Ft.json", "We are being redirected to the login page"},
		{"https://www.torrentday.com/elsewhere", "Redirected to https://www.torrentday.com/elsewhere from indexer request"},
	}
	for _, tt := range tests {
		ix := newIndexer(t, testSettings, nil)
		r := &indexer.Response{
			URL:        "https://www.torrentday.com/t.json",
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {tt.location}},
		}
		_, err := ix.Site.ParseResponse(context.Background(), ix, r)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("err = %v, want = containing %q", err, tt.want)
		}
	}
}

func TestDownloadCookies(t *testing.T) {
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		if got, want := r.Header.Get("Cookie"), "pass=abc; uid=1"; got != want {
			t.Errorf("Cookie = %q, want = %q", got, want)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infod"))}
	})
	if data, err := ix.Download(context.Background(), "https://www.torrentday.com/download.php/1/1.torrent"); err != nil || string(data) != "d4:infod" {
		t.Errorf("Download() = %q, %v, want = %q, nil", data, err, "d4:infod")
	}
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "cookie") {
		t.Errorf("New() error = %v, want = a cookie error", err)
	}
}
