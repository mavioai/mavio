package passthepopcorn

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
	return &indexer.Indexer{ID: "1", Name: "PTP", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func respond(status int, contentType, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

var testSettings = Settings{APIUser: "asdf", APIKey: "sad"}

func TestPTPCases(t *testing.T) {
	ported.Run(t, "ptp_tests/ptp.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"should_parse_feed_from_PTP": func(t *testing.T, a ported.Args) {
			name, _ := a.Str(t, "fileName")
			testFeed(t, strings.TrimPrefix(name, "Files/Indexers/"))
		},
	}})
}

func testFeed(t *testing.T, file string) {
	ix := newIndexer(t, testSettings, respond(http.StatusOK, "application/json", string(ported.Response(t, file))))
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 293 {
		t.Fatalf("releases = %d, want = 293", len(releases))
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"GUID", r.GUID, "PassThePopcorn-452135"},
		{"Title", r.Title, "The.Night.Of.S01.BluRay.AAC2.0.x264-DEPTH"},
		{"Protocol", r.Protocol, indexer.ProtocolTorrent},
		{"DownloadURL", r.DownloadURL, "https://passthepopcorn.me/torrents.php?action=download&id=452135"},
		{"InfoURL", r.InfoURL, "https://passthepopcorn.me/torrents.php?id=148131&torrentid=452135"},
		{"Size", *r.Size, int64(2466170624)},
		{"InfoHash", r.InfoHash, ""},
		{"MagnetURL", r.MagnetURL, ""},
		{"Peers", *r.Peers, 28},
		{"Seeders", *r.Seeders, 26},
		{"any Scene", slices.ContainsFunc(releases, func(r *indexer.Release) bool { return r.HasFlag(indexer.FlagScene) }), true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestSearchRequests(t *testing.T) {
	const base = "https://passthepopcorn.me/torrents.php?action=advanced&json=noredirect&grouping=0&order_by=time&order_way=desc&searchstr="
	tests := []struct {
		name string
		s    Settings
		q    indexer.Query
		want string
	}{
		{"rss categories", testSettings, indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000}}, base + "&filter_cat[1]=1&filter_cat[2]=1&filter_cat[3]=1&filter_cat[4]=1&filter_cat[5]=1&filter_cat[6]=1"},
		{"imdb", testSettings, indexer.Query{Type: indexer.SearchMovie, Term: "x", IMDbID: "123", Categories: []int{2000}}, base + "tt0000123"},
		{"tv", testSettings, indexer.Query{Type: indexer.SearchTV, Term: "Show", Season: indexer.Ptr(1)}, base + "Show+S01"},
		{
			"options paged",
			Settings{APIUser: "u", APIKey: "k", FreeleechOnly: true, GoldenPopcornOnly: true},
			indexer.Query{Term: "a b", Limit: 50, Offset: 100},
			base + "a+b&freetorrent=1&scene=2&page=3",
		},
		{"book", testSettings, indexer.Query{Type: indexer.SearchBook, Term: "x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.s, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r == nil {
				if tt.want != "" {
					t.Errorf("URL = none, want = %q", tt.want)
				}
				return
			}
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if r.Header.Get("ApiUser") != tt.s.APIUser || r.Header.Get("ApiKey") != tt.s.APIKey {
				t.Errorf("headers = %v, want = the API user and key", r.Header)
			}
		})
	}
}

func TestParse(t *testing.T) {
	const data = `{"TotalResults":"3","Movies":[{"GroupId":"7","CategoryId":"2","Year":"1999","Cover":"https://ptp.me/c.jpg","Tags":["drama","comedy"],"ImdbId":"0012345","Torrents":[
		{"Id":1,"Size":"10","UploadTime":"2017-04-17 12:13:42","Snatched":"1","Seeders":"2","Leechers":"3","ReleaseName":"A","GoldenPopcorn":true,"Checked":true,"FreeleechType":"Freeleech"},
		{"Id":2,"Size":"10","UploadTime":"2017-04-17 12:13:42","Snatched":"1","Seeders":"2","Leechers":"3","ReleaseName":"B","FreeleechType":"Neutral Leech"},
		{"Id":3,"Size":"10","UploadTime":"2017-04-17 12:13:42","Snatched":"1","Seeders":"2","Leechers":"3","ReleaseName":"C","FreeleechType":"Half Leech"}]}]}`
	type want struct {
		title    string
		down, up float64
	}
	tests := []struct {
		name string
		s    Settings
		want []want
	}{
		{"all", testSettings, []want{{"A", 0, 1}, {"B", 0, 0}, {"C", 0.5, 1}}},
		{"freeleech only", Settings{APIUser: "u", APIKey: "k", FreeleechOnly: true}, []want{{"A", 0, 1}, {"B", 0, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.s, respond(http.StatusOK, "application/json", data))
			releases, err := ix.Search(context.Background(), &indexer.Query{})
			if err != nil {
				t.Fatal(err)
			}
			var got []want
			for _, r := range releases {
				got = append(got, want{r.Title, *r.DownloadVolumeFactor, *r.UploadVolumeFactor})
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("releases = %+v, want = %+v", got, tt.want)
			}
		})
	}
	ix := newIndexer(t, testSettings, respond(http.StatusOK, "application/json", data))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Year", r.Year, 1999},
		{"IMDbID", r.IMDbID, 12345},
		{"PublishDate", r.PublishDate, time.Date(2017, 4, 17, 12, 13, 42, 0, time.UTC)},
		{"PosterURL", r.PosterURL, "https://ptp.me/c.jpg"},
		{"Genres", strings.Join(r.Genres, ","), "drama,comedy"},
		{"Categories", r.Categories[0], indexer.Movies.ID},
		{"Golden", r.HasFlag(FlagGolden), true},
		{"Approved", r.HasFlag(FlagApproved), true},
		{"Freeleech", r.HasFlag(indexer.FlagFreeleech), true},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(345600)},
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
		{"query limit", "application/json", "", http.StatusForbidden, "PTP Query Limit Reached"},
		{"not tv", "text/html", "We are not a TV indexer", http.StatusTooManyRequests, "We are not a TV indexer"},
		{"request limit", "text/html", "", http.StatusTooManyRequests, "PTP Request Limit Reached"},
		{"status", "text/html", "", http.StatusBadRequest, "Unexpected response status 400"},
		{"html", "text/html", "<html>", http.StatusOK, "Unexpected response header text/html"},
		{"none", "application/json", `{"TotalResults":"0"}`, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ix := newIndexer(t, testSettings, respond(tt.status, tt.contentType, tt.body))
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

func TestDownload(t *testing.T) {
	ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
		if r.Header.Get("ApiUser") != "asdf" || r.Header.Get("ApiKey") != "sad" {
			return respond(http.StatusUnauthorized, "text/html", "")(r)
		}
		return respond(http.StatusOK, "application/x-bittorrent", "d4:infod")(r)
	})
	data, err := ix.Download(context.Background(), "https://passthepopcorn.me/torrents.php?action=download&id=1")
	if err != nil || string(data) != "d4:infod" {
		t.Errorf("Download() = %q, %v, want = %q, nil", data, err, "d4:infod")
	}
}
