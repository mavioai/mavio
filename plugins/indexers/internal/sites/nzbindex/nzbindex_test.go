package nzbindex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	return &indexer.Indexer{ID: "1", Name: "NzbIndex", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{BaseSettings: indexer.BaseSettings{BaseURL: "https://nzbindex.com/"}}

func TestNzbIndexCases(t *testing.T) {
	ported.Run(t, "nzb_index_tests/nzb_index.json", ported.Cases{Facts: map[string]string{
		"should_parse_search_results":             "TestParseSearchResults",
		"should_build_search_url_without_api_key": "TestSearchRequests",
		"should_build_search_url_with_api_key":    "TestSearchRequests",
	}})
}

func respond(status int, contentType, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParseSearchResults(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(http.StatusOK, "application/json", string(ported.Response(t, "NzbIndex/search.json"))))
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "test"})
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
		{"Title", r.Title, "Ubuntu Gubuntu 11.10 Unity Edition (64bit)"},
		{"DownloadURL", r.DownloadURL, "https://nzbindex.com/api/download/a1b2c3d4-e5f6-7890-abcd-ef1234567890.nzb"},
		{"InfoURL", r.InfoURL, "https://nzbindex.com/collection/a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"GUID", r.GUID, "https://nzbindex.com/collection/a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"PublishDate", r.PublishDate, time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC)},
		{"Size", *r.Size, int64(8589934592)},
		{"Files", *r.Files, 42},
		{"Categories", r.Categories[0], indexer.Other.ID},
		{"Protocol", r.Protocol, indexer.ProtocolUsenet},
		{"second Title", releases[1].Title, "Some.TV.Show.S01E01.720p.WEB-DL-GROUP"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestSearchRequests(t *testing.T) {
	keyed := testSettings
	keyed.APIKey = " myapikey "
	tests := []struct {
		name string
		s    Settings
		q    indexer.Query
		want string
	}{
		{"without API key", testSettings, indexer.Query{Term: "test", Limit: 25}, "https://nzbindex.com/api/search?max=25&q=test"},
		{"with API key", keyed, indexer.Query{Term: "test", Limit: 25}, "https://nzbindex.com/api/search?max=25&key=myapikey&q=test"},
		{"rss", testSettings, indexer.Query{}, "https://nzbindex.com/api/search?max=100"},
		{"page", testSettings, indexer.Query{Term: "a b", Limit: 50, Offset: 100}, "https://nzbindex.com/api/search?max=50&q=a+b&p=2"},
		{
			"tv episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2"},
			"https://nzbindex.com/api/search?max=100&q=The+Show+S01E02",
		},
		{
			"movie",
			Settings{},
			indexer.Query{Type: indexer.SearchMovie, Term: "Film: Part (2020)"},
			"https://nzbindex.com/api/search?max=100&q=Film+Part+(2020)",
		},
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

func TestResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		status                  int
		want                    string
	}{
		{"api error", "application/json", `{"error":true,"errorMessage":"Invalid key"}`, http.StatusOK, "NZBIndex API error: Invalid key"},
		{"html", "text/html", "<html>", http.StatusOK, "Unexpected response header text/html"},
		{"status", "application/json", "{}", http.StatusForbidden, "Unexpected response status 403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, respond(tt.status, tt.contentType, tt.body))
			_, err := ix.Search(context.Background(), &indexer.Query{Term: "test"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = containing %q", err, tt.want)
			}
		})
	}
	ix := newIndexer(t, testSettings, respond(http.StatusOK, "application/json", `{"data":{"content":null},"error":false}`))
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "test"})
	if err != nil || len(releases) != 0 {
		t.Errorf("Search() = %d releases, %v, want = 0 releases, nil", len(releases), err)
	}
}
