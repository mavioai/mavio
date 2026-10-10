package hdbits

import (
	"context"
	"encoding/json"
	"errors"
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
	return &indexer.Indexer{ID: "1", Name: "HdBits", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func respond(status int, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
	}
}

var (
	testSettings = Settings{Username: "somename", APIKey: "abcd"}
	movieQuery   = indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000, 2010}, IMDbID: "0076759"}
)

func TestHDBitsCases(t *testing.T) {
	ported.Run(t, "hd_bits_tests/hd_bits.json", ported.Cases{
		Run: map[string]func(*testing.T, ported.Args){
			"should_parse_recent_feed_from_HDBits": func(t *testing.T, a ported.Args) {
				name, _ := a.Str(t, "fileName")
				testRecentFeed(t, strings.TrimPrefix(name, "Files/Indexers/"))
			},
		},
		Facts: map[string]string{"should_warn_on_wrong_passkey": "TestResponses"},
	})
	ported.Run(t, "hd_bits_tests/hd_bits_request_generator.json", ported.Cases{Facts: map[string]string{
		"should_search_by_imdbid_if_supported":                "TestSearchRequests",
		"should_search_by_tvdbid_season_episode_if_supported": "TestSearchRequests",
		"should_search_by_tvdbid_daily_episode_if_supported":  "TestSearchRequests",
	}})
}

func testRecentFeed(t *testing.T, file string) {
	ix := newIndexer(t, Settings{Username: "u", APIKey: "fakekey"}, respond(http.StatusOK, string(ported.Response(t, file))))
	releases, err := ix.Search(context.Background(), &movieQuery)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"GUID", r.GUID, "HDBits-257142"},
		{"Title", r.Title, "Supernatural.S10E17.1080p.WEB-DL.DD5.1.H.264-ECI"},
		{"Protocol", r.Protocol, indexer.ProtocolTorrent},
		{"DownloadURL", r.DownloadURL, "https://hdbits.org/download.php?id=257142&passkey=fakekey"},
		{"InfoURL", r.InfoURL, "https://hdbits.org/details.php?id=257142"},
		{"PublishDate", r.PublishDate, time.Date(2015, 4, 4, 20, 30, 46, 0, time.UTC)},
		{"Size", *r.Size, int64(1718009717)},
		{"InfoHash", r.InfoHash, "EABC50AEF9F53CEDED84ADF14144D3368E586F3A"},
		{"MagnetURL", r.MagnetURL, ""},
		{"Peers", *r.Peers, 47},
		{"Seeders", *r.Seeders, 46},
		{"TVDBID", r.TVDBID, 78901},
		{"Categories", r.Categories[0], indexer.TV.ID},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.5},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

// body returns the query of a search's request.
func body(t *testing.T, s Settings, q *indexer.Query) map[string]any {
	t.Helper()
	ix := newIndexer(t, s, nil)
	c, err := ix.Site.SearchRequests(context.Background(), ix, q)
	if err != nil {
		t.Fatal(err)
	}
	r := c.First()
	if r == nil {
		return nil
	}
	if r.Method != http.MethodPost || r.URL != "https://hdbits.org/api/torrents" || r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("request = %s %s %s, want = a POST of JSON to the API", r.Method, r.URL, r.Header.Get("Content-Type"))
	}
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		s    Settings
		q    indexer.Query
		want string
	}{
		{"imdb", testSettings, movieQuery, `{"category":[1],"imdb":{"id":76759},"limit":100,"passkey":"abcd","username":"somename"}`},
		{
			"tvdb season episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Categories: []int{5000, 5010}, TVDBID: 392256, Season: indexer.Ptr(1), Episode: "3"},
			`{"category":[2,3,5],"limit":100,"passkey":"abcd","tvdb":{"episode":"3","id":392256,"season":1},"username":"somename"}`,
		},
		{
			"tvdb daily", testSettings,
			indexer.Query{Type: indexer.SearchTV, Categories: []int{5000, 5010}, TVDBID: 289574, Season: indexer.Ptr(2023), Episode: "01/03"},
			`{"category":[2,3,5],"limit":100,"passkey":"abcd","search":"2023-01-03","tvdb":{"id":289574},"username":"somename"}`,
		},
		{
			"tv term", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1), Episode: "2"},
			`{"limit":100,"passkey":"abcd","search":"Show S01E02","username":"somename"}`,
		},
		{
			"movie term paged",
			Settings{Username: "u", APIKey: "k", Codecs: []int{1, 5}, Mediums: []int{6}, Origins: []int{1}},
			indexer.Query{Type: indexer.SearchMovie, Term: "Star Wars: Episode IV", Limit: 100, Offset: 200},
			`{"codec":[1,5],"limit":100,"medium":[6],"origin":[1],"page":2,"passkey":"k","search":"Star Wars Episode IV","username":"u"}`,
		},
		{"basic", testSettings, indexer.Query{Term: "x", IMDbID: "1"}, `{"limit":100,"passkey":"abcd","search":"x","username":"somename"}`},
		{"music", testSettings, indexer.Query{Type: indexer.SearchMusic, Term: "x"}, `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(body(t, tt.s, &tt.q))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("body = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	const data = `{"status":0,"data":[
		{"id":1,"name":"Disc","filename":"Disc.TORRENT","type_category":1,"type_medium":1,"added":"2015-04-04T20:30:46+0000","imdb":{"id":5,"year":1999}},
		{"id":2,"name":"XXX","filename":"x.torrent","type_category":7,"type_medium":6,"type_origin":1,"type_exclusive":1,"added":"2015-04-04T20:30:46+0000"},
		{"id":3,"name":"Free","filename":"Free.Torrent","freeleech":"yes","type_category":1,"type_medium":3,"added":"2015-04-04T20:30:46+0000"},
		{"id":4,"name":"Encode","filename":"","type_category":1,"type_medium":3,"added":"2015-04-04T20:30:46+0000"}
	]}`
	type want struct {
		title          string
		down, up       float64
		internal, excl bool
	}
	tests := []struct {
		name string
		s    Settings
		want []want
	}{
		{"filenames", testSettings, []want{
			{"Disc", 0.5, 1, false, false},
			{"XXX", 0, 0, true, true},
			{"Free", 0, 1, false, false},
			{"Encode", 1, 1, false, false},
		}},
		{"names, freeleech only", Settings{Username: "u", APIKey: "k", FreeleechOnly: true, UseFilenames: indexer.Ptr(false)}, []want{
			{"Free", 0, 1, false, false},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.s, respond(http.StatusOK, data))
			releases, err := ix.Search(context.Background(), &indexer.Query{})
			if err != nil {
				t.Fatal(err)
			}
			var got []want
			for _, r := range releases {
				got = append(got, want{r.Title, *r.DownloadVolumeFactor, *r.UploadVolumeFactor, r.HasFlag(indexer.FlagInternal), r.HasFlag(indexer.FlagExclusive)})
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("releases = %+v, want = %+v", got, tt.want)
			}
			if tt.name == "filenames" && (releases[0].IMDbID != 5 || releases[0].Year != 1999) {
				t.Errorf("IMDbID, Year = %d, %d, want = 5, 1999", releases[0].IMDbID, releases[0].Year)
			}
		})
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  string
		wantAuth bool
	}{
		{"wrong passkey", http.StatusOK, `{"status":5,"message":"Invalid authentication credentials"}`, "status code 5: Invalid authentication credentials", true},
		{"failure", http.StatusOK, `{"status":1}`, "status code 1", false},
		{"no data", http.StatusOK, `{"status":0,"data":{}}`, "missing result data", false},
		{"limit", http.StatusForbidden, ``, "Query Limit Reached", false},
		{"status", http.StatusBadRequest, ``, "Unexpected response status 400", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, respond(tt.status, tt.body))
			_, err := ix.Search(context.Background(), &movieQuery)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Search() error = %v, want = %q", err, tt.wantErr)
			}
			if got := errors.Is(err, indexer.ErrAuth); got != tt.wantAuth {
				t.Errorf("errors.Is(ErrAuth) = %v, want = %v", got, tt.wantAuth)
			}
		})
	}
}
