package beyondhd

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
	return &indexer.Indexer{ID: "1", Name: "BeyondHD", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

const (
	apiKey = "0123456789abcdef0123456789abcdef"
	rssKey = "fedcba9876543210fedcba9876543210"
)

var testSettings = Settings{APIKey: apiKey, RSSKey: rssKey}

func TestSearchRequests(t *testing.T) {
	filtered := testSettings
	filtered.FreeleechOnly = true
	filtered.LimitedOnly = true
	filtered.RefundOnly = true
	filtered.RewindOnly = true
	filtered.SearchTypes = []int{8, 0, 8}
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			"RSS", testSettings,
			indexer.Query{},
			`{"action":"search","rsskey":"` + rssKey + `"}`,
		},
		{
			"movie by IMDb ID", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "The Matrix", IMDbID: "133093", TMDBID: 603, Categories: []int{2000}},
			`{"action":"search","rsskey":"` + rssKey + `","imdb_id":"tt0133093","search":"The Matrix","categories":[1]}`,
		},
		{
			"movie by TMDB ID", testSettings,
			indexer.Query{Type: indexer.SearchMovie, TMDBID: 603},
			`{"action":"search","rsskey":"` + rssKey + `","tmdb_id":"movie/603"}`,
		},
		{
			"episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2), Episode: "3", Categories: []int{5000, 2000}},
			`{"action":"search","rsskey":"` + rssKey + `","search":"The Show S02E03","categories":[1,2]}`,
		},
		{
			"daily episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2024), Episode: "03/01"},
			`{"action":"search","rsskey":"` + rssKey + `","search":"The Show 2024-03-01"}`,
		},
		{
			"filters and page", filtered,
			indexer.Query{Term: "linux", Limit: 100, Offset: 200},
			`{"action":"search","rsskey":"` + rssKey + `","freeleech":1,"limited":1,"refund":1,"rewind":1,"search":"linux","types":["1080p","UHD 100"],"page":3}`,
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
			if got, want := r.URL, "https://beyond-hd.me/api/torrents/"+apiKey; got != want {
				t.Errorf("URL = %q, want = %q", got, want)
			}
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" {
				t.Errorf("Method, Content-Type, Accept = %q, %q, %q, want = POST, application/json, application/json", r.Method, r.Header.Get("Content-Type"), r.Header.Get("Accept"))
			}
			if got := string(r.Body); got != tt.want {
				t.Errorf("Body = %s, want = %s", got, tt.want)
			}
		})
	}
}

const feed = `{"status_code":1,"status_message":"OK","page":1,"results":[
{"id":1,"name":"Old.Movie.2001.1080p.BluRay.x264-GRP","info_hash":"aaa","category":"Movies","type":"1080p","size":8589934592,"times_completed":12,"seeders":30,"leechers":2,
 "audios":"English, French ,","subtitles":"English","created_at":"2023-01-02 03:04:05","download_url":"https://beyond-hd.me/torrent/download/1.rsskey","url":"https://beyond-hd.me/details/1",
 "imdb_id":"tt0133093","tmdb_id":"movie/603","freeleech":0,"promo25":0,"promo50":1,"promo75":0,"limited":0,"exclusive":1,"internal":1},
{"id":2,"name":"The.Show.S02E03.2160p.WEB-DL","info_hash":"bbb","category":"TV","type":"2160p","size":"1073741824","times_completed":"3","seeders":"5","leechers":"1",
 "audios":null,"subtitles":"","created_at":"2023-05-06 07:08:09","download_url":"https://beyond-hd.me/torrent/download/2.rsskey","url":"https://beyond-hd.me/details/2",
 "imdb_id":"","tmdb_id":"603","freeleech":true,"promo25":false,"promo50":false,"promo75":false,"limited":false,"exclusive":false,"internal":false}
]}`

func jsonResponse(status int, contentType, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, jsonResponse(http.StatusOK, "application/json; charset=utf-8", feed))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	// Newest first.
	tv, movie := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", movie.Title, "Old.Movie.2001.1080p.BluRay.x264-GRP"},
		{"GUID", movie.GUID, "https://beyond-hd.me/details/1"},
		{"InfoURL", movie.InfoURL, "https://beyond-hd.me/details/1"},
		{"DownloadURL", movie.DownloadURL, "https://beyond-hd.me/torrent/download/1.rsskey"},
		{"InfoHash", movie.InfoHash, "aaa"},
		{"Categories", movie.Categories, []int{2000, 100001}},
		{"PublishDate", movie.PublishDate, time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"Size", *movie.Size, int64(8589934592)},
		{"Grabs", *movie.Grabs, 12},
		{"Seeders", *movie.Seeders, 30},
		{"Peers", *movie.Peers, 32},
		{"IMDbID", movie.IMDbID, 133093},
		{"TMDBID", movie.TMDBID, 603},
		{"DownloadVolumeFactor", *movie.DownloadVolumeFactor, 0.5},
		{"UploadVolumeFactor", *movie.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *movie.MinimumRatio, 1.0},
		{"MinimumSeedTime", *movie.MinimumSeedTime, int64(172800)},
		{"Languages", movie.Languages, []string{"English", "French"}},
		{"Subtitles", movie.Subtitles, []string{"English"}},
		{"Flags", movie.Flags, []indexer.Flag{indexer.FlagInternal, indexer.FlagExclusive, indexer.FlagHalfLeech}},
		{"tv Categories", tv.Categories, []int{5000, 100002}},
		{"tv Size", *tv.Size, int64(1073741824)},
		{"tv TMDBID", tv.TMDBID, 0},
		{"tv IMDbID", tv.IMDbID, 0},
		{"tv DownloadVolumeFactor", *tv.DownloadVolumeFactor, 0.0},
		{"tv Languages", tv.Languages, []string{}},
		{"tv Subtitles", tv.Subtitles, []string{}},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseFilters(t *testing.T) {
	s := testSettings
	s.FreeleechOnly = true
	ix := newIndexer(t, s, jsonResponse(http.StatusOK, "application/json", feed))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].Title != "The.Show.S02E03.2160p.WEB-DL" {
		t.Errorf("releases = %d, want = only the freeleech one", len(releases))
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		want                    string
		auth                    bool
	}{
		{"status", "application/json", "{}", http.StatusBadRequest, "Unexpected response status 400", false},
		{"maintenance", "text/html", "<p>Site Maintenance</p>", http.StatusOK, "Indexer responded with HTML content. Site is under maintenance.", false},
		{"blocked", "text/html", "<p>no</p>", http.StatusOK, "Site is likely blocked or unavailable.", false},
		{"content type", "text/plain", "{}", http.StatusOK, "Unexpected response header text/plain", false},
		{"api key", "application/json", `{"status_code":0,"status_message":"Invalid API Key"}`, http.StatusOK, "API Key invalid or not authorized", true},
		{"error", "application/json", `{"status_code":0,"status_message":"Bad search"}`, http.StatusOK, "Indexer Error: Bad search", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, jsonResponse(tt.status, tt.contentType, tt.body))
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want = containing %q", err, tt.want)
			}
			if got := errors.Is(err, indexer.ErrAuth); got != tt.auth {
				t.Errorf("errors.Is(err, ErrAuth) = %v, want = %v", got, tt.auth)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	ix := newIndexer(t, testSettings, jsonResponse(http.StatusOK, "application/json", feed))
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "old movie"})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].Title != "Old.Movie.2001.1080p.BluRay.x264-GRP" {
		t.Errorf("releases = %d, want = the one matching the query", len(releases))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		s    Settings
		want string
	}{
		{Settings{RSSKey: rssKey}, "api_key: must not be empty"},
		{Settings{APIKey: "short", RSSKey: rssKey}, "api_key: must be 32 characters"},
		{Settings{APIKey: apiKey, RSSKey: "x"}, "rss_key: must be 32 characters"},
		{testSettings, ""},
	}
	for _, tt := range tests {
		got := ""
		if err := tt.s.Validate(); err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("Validate() = %q, want = %q", got, tt.want)
		}
	}
}
