package avistaz

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

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

var testSettings = Settings{Username: "someuser", Password: "somepass", PID: "somepid"}

func newIndexer(t *testing.T, id string, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	i := slices.IndexFunc(Definitions, func(d indexer.Definition) bool { return d.ID == id })
	if i < 0 {
		t.Fatalf("no definition %q", id)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := Definitions[i].New(data)
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
	return &indexer.Indexer{ID: "1", Name: id, Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestAvistazCases(t *testing.T) {
	for _, tt := range []struct{ file, fact string }{
		{"avistaz.json", "should_parse_recent_feed_from_AvistaZ"},
		{"exoticaz.json", "should_parse_recent_feed_from_ExoticaZ"},
		{"private_hd.json", "should_parse_recent_feed_from_PrivateHD"},
	} {
		ported.Run(t, "avistaz_tests/"+tt.file, ported.Cases{Facts: map[string]string{tt.fact: "TestRecentFeeds"}})
	}
}

type check struct {
	name      string
	got, want any
}

func TestRecentFeeds(t *testing.T) {
	tests := []struct {
		id, feed string
		checks   func(r *indexer.Release) []check
	}{
		{"avistaz", "Avistaz/recentfeed.json", func(r *indexer.Release) []check {
			return []check{
				{"Title", r.Title, "JAPAN SINKS: People of Hope 2021 S01E05 720p NF WEB-DL DDP2.0 x264-SEIKEL"},
				{"DownloadURL", r.DownloadURL, "https://avistaz.to/rss/download/(removed)/(removed).torrent"},
				{"InfoURL", r.InfoURL, "https://avistaz.to/torrent/187240-japan-sinks-people-of-hope-2021-s01e05-720p-nf-web-dl-ddp20-x264-seikel"},
				{"PublishDate", r.PublishDate, time.Date(2021, 11, 14, 21, 26, 21, 0, time.UTC)},
				{"Size", *r.Size, int64(935127615)},
				{"InfoHash", r.InfoHash, "a879261d4e6e792402f92401141a21de70d51bf2"},
				{"Peers", *r.Peers, 20},
				{"Seeders", *r.Seeders, 20},
				{"IMDbID", r.IMDbID, 15569106},
				{"TMDBID", r.TMDBID, 135144},
				{"TVDBID", r.TVDBID, 410548},
				{"Languages", strings.Join(r.Languages, ","), "Japanese"},
				{"Subtitles", len(r.Subtitles), 27},
				{"first Subtitle", r.Subtitles[0], "Arabic"},
			}
		}},
		{"exoticaz", "Exoticaz/recentfeed.json", func(r *indexer.Release) []check {
			return []check{
				{"Title", r.Title, "[SSIS-419] My first experience is Yua Mikami. From the day I lost my virginity, I was devoted to sex."},
				{"DownloadURL", r.DownloadURL, "https://exoticaz.to/rss/download/(removed)/(removed).torrent"},
				{"InfoURL", r.InfoURL, "https://exoticaz.to/torrent/64040-ssis-419-my-first-experience-is-yua-mikami-from-the-day-i-lost-my-virginity-i-was-devoted-to-sex"},
				{"PublishDate", r.PublishDate, time.Date(2022, 6, 11, 9, 4, 50, 0, time.UTC)},
				{"Size", *r.Size, int64(7085405541)},
				{"InfoHash", r.InfoHash, "asdjfiasdf54asd7f4a2sdf544asdf"},
				{"Peers", *r.Peers, 33},
				{"Seeders", *r.Seeders, 33},
				{"first Category", r.Categories[0], 6040},
			}
		}},
		{"privatehd", "PrivateHD/recentfeed.json", func(r *indexer.Release) []check {
			return []check{
				{"Title", r.Title, "Godzilla 2014 2160p UHD BluRay REMUX HDR HEVC Atmos-TRiToN"},
				{"DownloadURL", r.DownloadURL, "https://privatehd.to/rss/download/123456789/78506-a879261d4e6e792402f92401141a21de70d51bf2.torrent"},
				{"InfoURL", r.InfoURL, "https://privatehd.to/torrent/78506-godzilla-2014-2160p-uhd-bluray-remux-hdr-hevc-atmos-triton"},
				{"PublishDate", r.PublishDate, time.Date(2021, 3, 21, 4, 24, 49, 0, time.UTC)},
				{"Size", *r.Size, int64(69914591044)},
				{"InfoHash", r.InfoHash, "a879261d4e6e792402f92401141a21de70d51bf2"},
				{"Peers", *r.Peers, 32},
				{"Seeders", *r.Seeders, 27},
				{"IMDbID", r.IMDbID, 831387},
				{"TMDBID", r.TMDBID, 124905},
				{"TVDBID", r.TVDBID, 0},
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			feed := string(ported.Response(t, tt.feed))
			ix := newIndexer(t, tt.id, testSettings, func(*http.Request) *http.Response { return jsonResponse(http.StatusOK, feed) })
			releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000}})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(releases), 100; got != want {
				t.Fatalf("len(releases) = %d, want = %d", got, want)
			}
			r := releases[0]
			if r.Protocol != indexer.ProtocolTorrent || r.CommentsURL != "" || r.MagnetURL != "" {
				t.Errorf("Protocol, CommentsURL, MagnetURL = %v, %q, %q, want = torrent, \"\", \"\"", r.Protocol, r.CommentsURL, r.MagnetURL)
			}
			for _, c := range tt.checks(r) {
				if c.got != c.want {
					t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
				}
			}
		})
	}
}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		id   string
		s    Settings
		q    indexer.Query
		want string
	}{
		{
			"movie imdb", "avistaz", testSettings,
			indexer.Query{Type: indexer.SearchMovie, IMDbID: "123", TMDBID: 5, Categories: []int{2000}},
			"https://avistaz.to/api/v1/jackett/torrents?in=1&type=1&limit=50&imdb=tt0000123",
		},
		{
			"movie tmdb", "privatehd", testSettings,
			indexer.Query{Type: indexer.SearchMovie, TMDBID: 5, Genre: "drama", Limit: 20, Offset: 40},
			"https://privatehd.to/api/v1/jackett/torrents?in=1&type=0&limit=20&page=3&tags=drama&tmdb=5",
		},
		{
			"qualities", "cinemaz",
			Settings{Username: "u", Password: "p", PID: "x", FreeleechOnly: true},
			indexer.Query{Term: "a b", Categories: []int{2045, 5040, 2030}},
			"https://cinemaz.to/api/v1/jackett/torrents?in=1&type=0&limit=50&discount[]=1&video_quality[]=6&video_quality[]=2&video_quality[]=7&video_quality[]=3&video_quality[]=1&search=a+b",
		},
		{
			"tv tvdb", "privatehd", testSettings,
			indexer.Query{Type: indexer.SearchTV, TVDBID: 7, Season: indexer.Ptr(1), Episode: "2"},
			"https://privatehd.to/api/v1/jackett/torrents?in=1&type=0&limit=50&tvdb=7&search=S01E02",
		},
		{
			"avistaz episode without season", "avistaz", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Running Man", Episode: "323"},
			"https://avistaz.to/api/v1/jackett/torrents?in=1&type=0&limit=50&search=Running+Man+E323",
		},
		{
			"animez", "animez",
			Settings{Username: "u", Password: "p", PID: "x", FreeleechOnly: true},
			indexer.Query{Term: "x", Categories: []int{7000}, Limit: 10, Offset: 10},
			"https://animez.to/api/v1/jackett/torrents?limit=10&format[]=MANGA&format[]=NOVEL&format[]=ONE_SHOT&page=2&freeleech=1&search=x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.id, tt.s, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
			if got := c.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, "privatehd", testSettings, func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/api/v1/jackett/auth":
				logins++
				body, _ := io.ReadAll(r.Body)
				if got, want := string(body), "username=someuser&password=somepass&pid=somepid"; got != want {
					t.Errorf("auth form = %q, want = %q", got, want)
				}
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			case "/api/v1/jackett/torrents":
				if r.Header.Get("Authorization") != "Bearer tok" {
					return jsonResponse(http.StatusUnauthorized, `{"message":"no"}`)
				}
				return jsonResponse(http.StatusOK, `{"data":[{"url":"u","download":"d","created_at_iso":"2021-03-21T00:24:49-04:00","file_name":"F","type":"Music","seed":1,"leech":2,"file_size":107374182400}]}`)
			}
			return jsonResponse(http.StatusNotFound, "")
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if logins != 1 || len(releases) != 1 {
			t.Fatalf("logins, releases = %d, %d, want = 1, 1", logins, len(releases))
		}
		r := releases[0]
		// 100 GiB: (100 ln 100 - 219.2023) hours, truncated.
		if got, want := *r.MinimumSeedTime, int64(241)*3600; got != want {
			t.Errorf("MinimumSeedTime = %d, want = %d", got, want)
		}
		if got, want := r.Categories, []int{indexer.Audio.ID}; !slices.Equal(got, want) {
			t.Errorf("Categories = %v, want = %v", got, want)
		}
	})
}

// TestTokenKept checks that the token outlives the site, as Prowlarr keeps
// it with the indexer's settings.
func TestTokenKept(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		h := func(r *http.Request) *http.Response {
			switch r.URL.Path {
			case "/api/v1/jackett/auth":
				logins++
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			case "/api/v1/jackett/torrents":
				if r.Header.Get("Authorization") != "Bearer tok" {
					return jsonResponse(http.StatusUnauthorized, `{"message":"no"}`)
				}
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			}
			return jsonResponse(http.StatusNotFound, "")
		}
		ix := newIndexer(t, "privatehd", testSettings, h)
		if _, err := ix.Search(context.Background(), &indexer.Query{}); err != nil {
			t.Fatal(err)
		}
		restarted := newIndexer(t, "privatehd", testSettings, h)
		restarted.Statuses = ix.Statuses
		if _, err := restarted.Search(context.Background(), &indexer.Query{}); err != nil {
			t.Fatal(err)
		}
		if logins != 1 {
			t.Errorf("logins = %d, want = 1", logins)
		}
	})
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"not found", http.StatusNotFound, "", ""},
		{"rate limit", http.StatusTooManyRequests, "", "request limit reached"},
		{"bad type", http.StatusOK, `{"data":[{"created_at_iso":"2021-03-21T00:24:49Z","type":"Book"}]}`, `error parsing Avistaz category type "Book"`},
		{"server error", http.StatusBadRequest, "", "Unexpected response status 400"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, "avistaz", testSettings, func(*http.Request) *http.Response { return jsonResponse(tt.status, tt.body) })
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
	}
}

func TestAnimeZDownload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		ix := newIndexer(t, "animez", testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/api/v1/jackett/auth" {
				logins++
				return jsonResponse(http.StatusOK, `{"token":"tok"}`)
			}
			if r.Header.Get("Authorization") != "Bearer tok" {
				return jsonResponse(http.StatusUnauthorized, "")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("d4:infod"))}
		})
		data, err := ix.Download(context.Background(), "https://animez.to/torrent/1/download")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "d4:infod" || logins != 1 {
			t.Errorf("Download() = %q with %d logins, want = %q with 1", data, logins, "d4:infod")
		}
	})
}
