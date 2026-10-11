package nebulance

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

func newIndexer(t *testing.T, h handler) *indexer.Indexer {
	t.Helper()
	site, err := Definition.New([]byte(`{"api_key":"key"}`))
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
	return &indexer.Indexer{ID: "1", Name: "Nebulance", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func respond(status int, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestSearchRequests(t *testing.T) {
	const base = "https://nebulance.io/api.php?action=search&age=%3E0"
	tests := []struct {
		name string
		q    indexer.Query
		want string // "" for no request
	}{
		{"RSS", indexer.Query{}, base + "&api_key=key&per_page=100"},
		{"basic", indexer.Query{Term: "The Show", Limit: 50, Offset: 100}, base + "&release=The+Show&api_key=key&per_page=50&page=2"},
		{"basic too short", indexer.Query{Term: "ab"}, ""},
		{"movie", indexer.Query{Type: indexer.SearchMovie, Term: "The Show"}, ""},
		{
			"episode by TVmaze ID",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", TVMazeID: 7, IMDbID: "1", Season: indexer.Ptr(1), Episode: "02"},
			base + "&tvmaze=7&release=The+Show&season=1&episode=2&api_key=key&per_page=100",
		},
		{
			"season by IMDb ID",
			indexer.Query{Type: indexer.SearchTV, IMDbID: "1234", Season: indexer.Ptr(0)},
			base + "&imdb=tt0001234&season=0&api_key=key&per_page=100",
		},
		{
			"daily episode",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2024), Episode: "03/01"},
			base + "&release=2024.03.01&name=The+Show&api_key=key&per_page=100",
		},
		{"season without name", indexer.Query{Type: indexer.SearchTV, Season: indexer.Ptr(1)}, ""},
		{"TV too short", indexer.Query{Type: indexer.SearchTV, Term: "ab", Season: indexer.Ptr(1)}, ""},
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

const feed = `{"total_results":2,"items":[
{"rls_name":" The.Show.S01E02.1080p.WEB-DL.DDP5.1.H.264-GRP ","cat":"Episode","size":1073741824,"seed":10,"leech":2,"snatch":30,
 "download":"https://nebulance.io/torrents.php?action=download&id=5&authkey=a&torrent_pass=b","file_list":["a.mkv","a.nfo"],
 "group_name":"The Show","group_id":5,"imdb_id":"tt1234567","tvmaze_id":42,"rls_utc":"2023-01-02 03:04:05","tags":["Scene","drama"]},
{"rls_name":"","cat":"Season","size":"5368709120","seed":"1","leech":"0","snatch":"0","download":"https://nebulance.io/dl/6",
 "group_name":"Another Show S01","group_id":"6","imdb_id":null,"tvmaze_id":null,"rls_utc":"2023-01-01T00:00:00Z","tags":null}
]}`

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, respond(http.StatusOK, feed))
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
		{"Title", r.Title, "The.Show.S01E02.1080p.WEB-DL.DDP5.1.H.264-GRP"},
		{"GUID", r.GUID, "https://nebulance.io/torrents.php?id=5"},
		{"InfoURL", r.InfoURL, "https://nebulance.io/torrents.php?id=5"},
		{"DownloadURL", r.DownloadURL, "https://nebulance.io/torrents.php?action=download&id=5&authkey=a&torrent_pass=b"},
		{"Categories", r.Categories, []int{indexer.TVHD.ID}},
		{"Size", *r.Size, int64(1073741824)},
		{"Files", *r.Files, 2},
		{"PublishDate", r.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"Grabs", *r.Grabs, 30},
		{"Seeders", *r.Seeders, 10},
		{"Peers", *r.Peers, 12},
		{"Scene", r.Scene, true},
		{"MinimumRatio", *r.MinimumRatio, 0.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(86400)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"IMDbID", r.IMDbID, 1234567},
		{"TVMazeID", r.TVMazeID, 42},
		{"Flags", r.Flags, []indexer.Flag{indexer.FlagFreeleech, indexer.FlagScene}},
		{"second Title", r2.Title, "Another Show S01"},
		{"second Categories", r2.Categories, []int{indexer.TV.ID}},
		{"second MinimumSeedTime", *r2.MinimumSeedTime, int64(432000)},
		{"second Files", *r2.Files, 0},
		{"second Scene", r2.Scene, false},
		{"second TVMazeID", r2.TVMazeID, 0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"status", http.StatusForbidden, "", "Unexpected response status '403' code"},
		{"error", http.StatusOK, `{"error":{"code":1,"message":"Invalid API key"}}`, "Indexer API call returned an error [Invalid API key]"},
		{"RPC result", http.StatusOK, `{"total_results":"many","result":"Bad request"}`, "Unexpected response from indexer request: Bad request"},
		{"not JSON", http.StatusOK, `<html>`, "Unexpected response from indexer request: invalid character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, respond(tt.status, tt.body))
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want = containing %q", err, tt.want)
			}
		})
	}
	ix := newIndexer(t, respond(http.StatusOK, `{"total_results":0,"items":[]}`))
	if releases, err := ix.Search(context.Background(), &indexer.Query{}); err != nil || len(releases) != 0 {
		t.Errorf("Search() = %d, %v, want = 0, nil", len(releases), err)
	}
}

func TestDownloadWithoutCookies(t *testing.T) {
	ix := newIndexer(t, func(r *http.Request) *http.Response {
		if c := r.Header.Get("Cookie"); c != "" {
			t.Errorf("Cookie = %q, want = none", c)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infod"))}
	})
	ix.SetCookies(map[string]string{"session": "x"}, time.Time{})
	if data, err := ix.Download(context.Background(), "https://nebulance.io/dl/6"); err != nil || string(data) != "d4:infod" {
		t.Errorf("Download() = %q, %v, want = %q, nil", data, err, "d4:infod")
	}
}

func TestTVCategory(t *testing.T) {
	tests := []struct {
		title string
		want  *indexer.Category
	}{
		{"Show.S01E01.2160p.WEB-DL.x265-GRP", indexer.TVUHD},
		{"Show.S01E01.720p.WEBRip.x264-GRP", indexer.TVHD},
		{"Show.S01E01.480p.WEB-DL.x264-GRP", indexer.TVSD},
		{"Show.S01E01.WEB-DL.x264-GRP", indexer.TV},
		{"Show.S01E01.HDTV.x264-GRP", indexer.TVSD},
		{"Show.S01E01.1080p.HDTV.x264-GRP", indexer.TVHD},
		{"Show.S01.1080p.BluRay.x264-GRP", indexer.TVHD},
		{"Show.S01.BDRip.XviD-GRP", indexer.TVSD},
		{"Show.S01.2160p.BluRay.REMUX-GRP", indexer.TVUHD},
		{"Show.S01.DVDRip.x264-GRP", indexer.TVSD},
		{"Show.S01E01.HR.WS.PDTV.x264-GRP", indexer.TVHD},
		{"Show.S01E01.PDTV.x264-GRP", indexer.TVSD},
		{"Show.S01E01.1080i.HDTV.MPEG2-GRP", indexer.TVSD},
		{"Show.S01E01.TrollHD", indexer.TVHD},
		{"Show_S01E01_RawHD", indexer.TVHD},
		{"Show S01E01", indexer.TV},
		{"", indexer.TV},
	}
	for _, tt := range tests {
		if got := tvCategory(tt.title); got.ID != tt.want.ID {
			t.Errorf("tvCategory(%q) = %d, want = %d", tt.title, got.ID, tt.want.ID)
		}
	}
}
