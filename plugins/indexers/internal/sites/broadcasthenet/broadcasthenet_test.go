package broadcasthenet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
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

func newIndexer(t *testing.T, h handler) *indexer.Indexer {
	t.Helper()
	site, err := Definition.New([]byte(`{"api_key":"abc"}`))
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
	return &indexer.Indexer{ID: "1", Name: "btn", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

// call is a getTorrents call.
type call struct {
	key     string
	query   torrentQuery
	results int
	offset  int
}

// calls returns the calls of a query's searches, one per search.
func calls(t *testing.T, q *indexer.Query) []call {
	t.Helper()
	ix := newIndexer(t, nil)
	c, err := ix.Site.SearchRequests(context.Background(), ix, q)
	if err != nil {
		t.Fatal(err)
	}
	var out []call
	for _, tier := range c.Tiers() {
		for _, pages := range tier {
			for r := range pages {
				var body struct {
					JSONRPC string            `json:"jsonrpc"`
					Method  string            `json:"method"`
					Params  []json.RawMessage `json:"params"`
					ID      string            `json:"id"`
				}
				if err := json.Unmarshal(r.Body, &body); err != nil {
					t.Fatal(err)
				}
				if body.JSONRPC != "2.0" || body.Method != "getTorrents" || len(body.Params) != 4 || len(body.ID) != 8 {
					t.Fatalf("body = %s, want = a getTorrents call", r.Body)
				}
				var cl call
				for i, v := range []any{&cl.key, &cl.query, &cl.results, &cl.offset} {
					if err := json.Unmarshal(body.Params[i], v); err != nil {
						t.Fatal(err)
					}
				}
				out = append(out, cl)
				break
			}
		}
		if len(c.Tiers()) != 1 {
			t.Errorf("tiers = %d, want = 1", len(c.Tiers()))
		}
	}
	return out
}

var tvCats = []int{indexer.TV.ID, indexer.TVHD.ID}

func TestRequestGeneratorCases(t *testing.T) {
	episode := func(term string, tvdb int) func(*testing.T, ported.Args) {
		return func(t *testing.T, a ported.Args) {
			var season int
			a.Decode(t, "season", &season)
			ep, _ := a.Str(t, "episode")
			want, _ := a.Str(t, "expectedQuery")
			search := ""
			if term != "" {
				search = strings.ReplaceAll(term, " ", "%")
			}
			got := calls(t, &indexer.Query{Type: indexer.SearchTV, Categories: tvCats, Term: term, TVDBID: tvdb, Season: &season, Episode: ep})
			wantCalls := []call{{"abc", torrentQuery{Category: "Episode", Name: want, Search: search, Tvdb: map[bool]string{true: "371980"}[tvdb > 0]}, 100, 0}}
			if !slices.Equal(got, wantCalls) {
				t.Errorf("calls = %+v, want = %+v", got, wantCalls)
			}
		}
	}
	ported.Run(t, "broadcasthe_net_tests/broadcasthe_net_request_generator.json", ported.Cases{
		Run: map[string]func(*testing.T, ported.Args){
			"should_search_by_tvdbid_season_episode_if_supported": episode("", 371980),
			"should_search_by_term_season_episode_if_supported":   episode("Malcolm in the Middle", 0),
		},
		Facts: map[string]string{
			"should_have_empty_parameters_if_rss_search":         "TestSearchRequests",
			"should_search_by_tvdbid_season_if_supported":        "TestSearchRequests",
			"should_search_by_tvdbid_daily_episode_if_supported": "TestSearchRequests",
			"should_prefer_search_by_tvdbid_if_rid_supported":    "TestSearchRequests",
			"should_search_by_term_supported":                    "TestSearchRequests",
			"should_search_by_term_season_if_supported":          "TestSearchRequests",
			"should_search_by_term_daily_episode_if_supported":   "TestSearchRequests",
		},
	})
}

func TestSearchRequests(t *testing.T) {
	malcolm := "Malcolm%in%the%Middle"
	tests := []struct {
		name string
		q    indexer.Query
		want []call
	}{
		{"rss", indexer.Query{Type: indexer.SearchTV, Categories: tvCats}, []call{{"abc", torrentQuery{}, 100, 0}}},
		{
			"tvdb season",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, TVDBID: 371980, Season: indexer.Ptr(1)},
			[]call{
				{"abc", torrentQuery{Tvdb: "371980", Category: "Season", Name: "Season 1%"}, 100, 0},
				{"abc", torrentQuery{Tvdb: "371980", Category: "Episode", Name: "S01E%"}, 100, 0},
			},
		},
		{
			"tvdb daily",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, TVDBID: 289574, Season: indexer.Ptr(2023), Episode: "01/03"},
			[]call{{"abc", torrentQuery{Tvdb: "289574", Category: "Episode", Name: "2023.01.03%"}, 100, 0}},
		},
		{
			"tvdb before rid",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, TVDBID: 371980, RID: 12345},
			[]call{{"abc", torrentQuery{Tvdb: "371980"}, 100, 0}},
		},
		{
			"rid",
			indexer.Query{Type: indexer.SearchTV, RID: 12345, Limit: 50, Offset: 100},
			[]call{{"abc", torrentQuery{Tvrage: "12345"}, 50, 100}},
		},
		{
			"term",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, Term: "Malcolm in the Middle"},
			[]call{{"abc", torrentQuery{Search: malcolm}, 100, 0}},
		},
		{
			"term season",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, Term: "Malcolm in the Middle", Season: indexer.Ptr(2)},
			[]call{
				{"abc", torrentQuery{Search: malcolm, Category: "Season", Name: "Season 2%"}, 100, 0},
				{"abc", torrentQuery{Search: malcolm, Category: "Episode", Name: "S02E%"}, 100, 0},
			},
		},
		{
			"term daily",
			indexer.Query{Type: indexer.SearchTV, Categories: tvCats, Term: "The Late Show with Stephen Colbert", Season: indexer.Ptr(2023), Episode: "01/03"},
			[]call{{"abc", torrentQuery{Search: "The%Late%Show%with%Stephen%Colbert", Category: "Episode", Name: "2023.01.03%"}, 100, 0}},
		},
		{"absolute episode by ID", indexer.Query{Type: indexer.SearchTV, TVDBID: 1, Term: "123"}, nil},
		{"basic", indexer.Query{Term: "a b", Season: indexer.Ptr(1)}, []call{{"abc", torrentQuery{Search: "a%b"}, 100, 0}}},
		{"movie", indexer.Query{Type: indexer.SearchMovie, Term: "a"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calls(t, &tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("calls = %+v, want = %+v", got, tt.want)
			}
		})
	}
}

const results = `{"id":"x","result":{"results":"2","torrents":{
"20":{"GroupID":"5","TorrentID":"20","Category":"Season","Snatched":"3","Seeders":"10","Leechers":"2",
 "Container":"M2TS","Resolution":"1080p","Origin":"Internal","ReleaseName":"Show.S01.1080p.BluRay.H.264-GRP\\\\",
 "Size":"1000","Time":"1600000000","TvdbID":"71","TvrageID":"0","ImdbID":"0123","InfoHash":"ABC",
 "DownloadURL":"https://broadcasthe.net/torrents.php?action=download&id=20"},
"10":{"GroupID":"4","TorrentID":"10","Category":"Episode","Container":"MKV","Resolution":"Unknown",
 "Origin":"Scene","ReleaseName":"Show.S01E01.H.264-GRP","Size":"1","Time":"1600000001",
 "DownloadURL":"https://broadcasthe.net/d"}}}}`

func TestParseResponse(t *testing.T) {
	ix := newIndexer(t, func(r *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(results))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	a, b := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"GUID", a.GUID, "BTN-20"},
		{"InfoURL", a.InfoURL, "https://broadcasthe.net/torrents.php?id=5&torrentid=20"},
		{"DownloadURL", a.DownloadURL, "https://broadcasthe.net/torrents.php?action=download&id=20"},
		{"Title", a.Title, "Show.S01.1080p.BluRay.AVC-GRP"},
		{"Categories", a.Categories[0], indexer.TVHD.ID},
		{"InfoHash", a.InfoHash, "ABC"},
		{"Size", *a.Size, int64(1000)},
		{"Grabs", *a.Grabs, 3},
		{"Seeders", *a.Seeders, 10},
		{"Peers", *a.Peers, 12},
		{"PublishDate", a.PublishDate, time.Unix(1600000000, 0).UTC()},
		{"Flags", a.HasFlag(indexer.FlagInternal), true},
		{"Scene", a.Scene, false},
		{"MinimumSeedTime", *a.MinimumSeedTime, int64(432000)},
		{"DownloadVolumeFactor", *a.DownloadVolumeFactor, 0.0},
		{"TVDBID", a.TVDBID, 71},
		{"TVRageID", a.TVRageID, 0},
		{"IMDbID", a.IMDbID, 123},
		{"second GUID", b.GUID, "BTN-10"},
		{"second Title", b.Title, "Show.S01E01.H.264-GRP"},
		{"second Categories", b.Categories[0], indexer.TV.ID},
		{"second Scene", b.Scene, true},
		{"second Seeders", b.Seeders == nil, true},
		{"second MinimumSeedTime", *b.MinimumSeedTime, int64(86400)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		wantErr                 string
	}{
		{"unauthorized", "application/json", "", http.StatusUnauthorized, "API Key invalid"},
		{"not found", "application/json", "", http.StatusNotFound, "API may have changed"},
		{"unavailable", "application/json", "", http.StatusServiceUnavailable, "150 API requests per hour"},
		{"html", "text/html", "<html>", http.StatusOK, "html content"},
		{"call limit", "application/json", "Call Limit Exceeded", http.StatusOK, "150 API requests per hour"},
		{"interrupted", "application/json", "Query execution was interrupted", http.StatusOK, "internal server error"},
		{"rpc error", "application/json", `{"error":{"code":-32001,"message":"Invalid API Key"}}`, http.StatusOK, "Invalid API Key"},
		{"no results", "application/json", `{"result":{"results":"0","torrents":[]}}`, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, func(*http.Request) *http.Response {
					return &http.Response{StatusCode: tt.status, Header: http.Header{"Content-Type": {tt.contentType}}, Body: io.NopCloser(strings.NewReader(tt.body))}
				})
				releases, err := ix.Search(context.Background(), &indexer.Query{})
				if tt.wantErr == "" {
					if err != nil || len(releases) != 0 {
						t.Errorf("Search() = %d, %v, want = 0, nil", len(releases), err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Search() error = %v, want = %q", err, tt.wantErr)
				}
			})
		})
	}
}
