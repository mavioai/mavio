package gazellegames

import (
	"context"
	"encoding/json"
	"errors"
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

// handler answers the site's requests.
type handler func(r *http.Request) *http.Response

func (h handler) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := h(r)
	resp.Request = r
	return resp, nil
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
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
	return &indexer.Indexer{ID: "1", Name: "GazelleGames", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestCases(t *testing.T) {
	ported.Run(t, "gazelle_games_tests/gazelle_games.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_GazelleGames": "TestRecentFeed",
		"should_not_error_if_empty_response":         "TestEmptyResponse",
	}})
}

func TestRecentFeed(t *testing.T) {
	feed := string(ported.Response(t, "GazelleGames/recentfeed.json"))
	ix := newIndexer(t, Settings{APIKey: "somekey"}, func(*http.Request) *http.Response { return jsonResponse(feed) })
	releases, err := ix.Search(context.Background(), &indexer.Query{Categories: []int{2000}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(releases), 1462; got != want {
		t.Fatalf("len(releases) = %d, want = %d", got, want)
	}
	r := releases[0]
	checks := []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Microsoft_Flight_Simulator-HOODLUM (2020) [Windows / Multi-Language / Full ISO]"},
		{"Protocol", r.Protocol, indexer.ProtocolTorrent},
		{"DownloadURL", r.DownloadURL, "https://gazellegames.net/torrents.php?action=download&id=303216&authkey=prowlarr&torrent_pass="},
		{"InfoURL", r.InfoURL, "https://gazellegames.net/torrents.php?id=84781&torrentid=303216"},
		{"CommentsURL", r.CommentsURL, ""},
		{"PublishDate", r.PublishDate, time.Date(2022, 7, 25, 6, 39, 11, 0, time.UTC)},
		{"Size", *r.Size, int64(80077617780)},
		{"InfoHash", r.InfoHash, ""},
		{"MagnetURL", r.MagnetURL, ""},
		{"Peers", *r.Peers, 383},
		{"Seeders", *r.Seeders, 383},
		{"IMDbID", r.IMDbID, 0},
		{"TMDBID", r.TMDBID, 0},
		{"TVDBID", r.TVDBID, 0},
		{"Languages", len(r.Languages), 0},
		{"Subtitles", len(r.Subtitles), 0},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 1.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestEmptyResponse(t *testing.T) {
	feed := string(ported.Response(t, "GazelleGames/recentfeed-empty.json"))
	ix := newIndexer(t, Settings{APIKey: "somekey"}, func(*http.Request) *http.Response { return jsonResponse(feed) })
	releases, err := ix.Search(context.Background(), &indexer.Query{Categories: []int{2000}})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 0 {
		t.Errorf("len(releases) = %d, want = 0", len(releases))
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			name: "term and platforms", settings: Settings{APIKey: "k"},
			q:    indexer.Query{Term: "Some.Game", Categories: []int{indexer.ConsoleXBox360.ID, indexer.PCGames.ID}},
			want: "https://gazellegames.net/api.php?request=search&search_type=torrents&empty_groups=filled&order_by=time&order_way=desc&searchstr=Some+Game&artistcheck[]=DOS&artistcheck[]=Windows&artistcheck[]=Xbox+360",
		},
		{
			name: "group names and freeleech", settings: Settings{APIKey: "k", SearchGroupNames: true, FreeleechOnly: true},
			q:    indexer.Query{Type: indexer.SearchMovie, Term: "Game"},
			want: "https://gazellegames.net/api.php?request=search&search_type=torrents&empty_groups=filled&order_by=time&order_way=desc&groupname=Game&freetorrent=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			chain, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := chain.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("X-API-Key"); got != "k" {
				t.Errorf("X-API-Key = %q, want = %q", got, "k")
			}
		})
	}
}

func TestParse(t *testing.T) {
	const feed = `{"status":"success","response":{
		"2":{"Artists":[],"Year":"2001","Torrents":{
			"20":{"CategoryID":"3","ReleaseTitle":"A Book","Format":"PDF","Encoding":"","RemasterTitle":"Second &amp; Last","RemasterYear":"2003","Dupable":"1","GameDOXType":" Update ","Time":"2024-01-02 00:00:00","TorrentType":"Torrent","Size":"5","Seeders":"1","Leechers":"2","FreeTorrent":"1"},
			"21":{"CategoryID":"1","ReleaseTitle":"A Book 1999","Time":"2024-01-01 00:00:00","TorrentType":"Torrent","Size":"5","Seeders":"1","Leechers":"2","FreeTorrent":"Neutral"},
			"22":{"CategoryID":"3","ReleaseTitle":"Link","Time":"2024-01-03 00:00:00","TorrentType":"Link","Size":"5"}}},
		"3":{"Artists":[{"name":"Switch"}],"Torrents":[]},
		"4":{"Artists":[{"name":"Wii"},{"name":"Wii U"}],"Year":"2010","Torrents":{
			"40":{"CategoryID":"1","ReleaseTitle":"Game","Region":"PAL","Time":"2023-01-01 00:00:00","TorrentType":"Torrent","Size":"5","Seeders":"1","Leechers":"2","FreeTorrent":"0","LowSeedFL":true},
			"41":{"CategoryID":"1","ReleaseTitle":"Game 2","Time":"2022-01-01 00:00:00","TorrentType":"Torrent","Size":"5","Seeders":"1","Leechers":"2","FreeTorrent":"0"}}}
	}}`
	wii := []int{indexer.ConsoleWii.ID, 122059, indexer.ConsoleWiiU.ID, 128932}
	type want struct {
		title      string
		down, up   float64
		categories []int
	}
	tests := []struct {
		name     string
		settings Settings
		want     []want
	}{
		{
			name: "all", settings: Settings{APIKey: "k"},
			want: []want{
				{"A Book (2001) [Second & Last 2003] [PDF / Trumpable] [Update]", 0, 1, []int{7020, 100003}},
				{"A Book 1999", 0, 0, []int{7020, 100003}},
				{"Game (2010) [Wii, Wii U / PAL]", 0, 1, wii},
				{"Game 2 (2010) [Wii, Wii U]", 1, 1, wii},
			},
		},
		{
			name: "freeleech only", settings: Settings{APIKey: "k", FreeleechOnly: true},
			want: []want{
				{"A Book (2001) [Second & Last 2003] [PDF / Trumpable] [Update]", 0, 1, []int{7020, 100003}},
				{"A Book 1999", 0, 0, []int{7020, 100003}},
				{"Game (2010) [Wii, Wii U / PAL]", 0, 1, wii},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			resp := &indexer.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(feed)}
			releases, err := ix.Site.ParseResponse(context.Background(), ix, resp)
			if err != nil {
				t.Fatal(err)
			}
			var got []want
			for _, r := range releases {
				got = append(got, want{r.Title, *r.DownloadVolumeFactor, *r.UploadVolumeFactor, r.Categories})
			}
			if !slices.EqualFunc(got, tt.want, func(a, b want) bool {
				return a.title == b.title && a.down == b.down && a.up == b.up && slices.Equal(a.categories, b.categories)
			}) {
				t.Errorf("releases = %+v, want = %+v", got, tt.want)
			}
		})
	}
}

func TestResponseErrors(t *testing.T) {
	tests := []struct {
		name string
		resp *indexer.Response
	}{
		{"status", &indexer.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{"Content-Type": {"application/json"}}}},
		{"content type", &indexer.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte("<html>")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, Settings{APIKey: "k"}, nil)
			if _, err := ix.Site.ParseResponse(context.Background(), ix, tt.resp); err == nil {
				t.Errorf("ParseResponse() = nil error, want = an error")
			}
		})
	}
}

// TestDownload runs in a bubble: the requests wait for the rate limit.
func TestDownload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var got []string
		ix := newIndexer(t, Settings{APIKey: "k"}, func(r *http.Request) *http.Response {
			got = append(got, r.URL.String())
			if r.URL.Path == "/api.php" {
				return jsonResponse(`{"status":"success","response":{"passkey":"pk"}}`)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infoe"))}
		})
		link := "https://gazellegames.net/torrents.php?action=download&id=1&authkey=prowlarr&torrent_pass="
		for range 2 {
			if _, err := ix.Download(context.Background(), link); err != nil {
				t.Fatal(err)
			}
		}
		want := []string{
			"https://gazellegames.net/api.php?request=quick_user",
			"https://gazellegames.net/torrents.php?action=download&id=1&authkey=prowlarr&torrent_pass=pk",
			"https://gazellegames.net/torrents.php?action=download&id=1&authkey=prowlarr&torrent_pass=pk",
		}
		if !slices.Equal(got, want) {
			t.Errorf("requests = %q, want = %q", got, want)
		}
	})
}

func TestPasskeyFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, Settings{APIKey: "k"}, func(*http.Request) *http.Response {
			return jsonResponse(`{"status":"failure","error":"bad key"}`)
		})
		err := ix.Site.(indexer.SiteTester).TestSite(context.Background(), ix)
		if !errors.Is(err, indexer.ErrAuth) {
			t.Errorf("TestSite() = %v, want = %v", err, indexer.ErrAuth)
		}
	})
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil {
		t.Errorf("New(no api_key) = nil error, want = an error")
	}
}
