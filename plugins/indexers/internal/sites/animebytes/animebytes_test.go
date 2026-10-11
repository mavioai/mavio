package animebytes

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
	return &indexer.Indexer{ID: "1", Name: "AnimeBytes", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func respond(contentType, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

// passkey is a passkey of a valid length.
const passkey = "somepasssomepasssomepasssomepass"

var testSettings = Settings{
	BaseSettings: indexer.BaseSettings{BaseURL: "https://animebytes.tv/"},
	Username:     "someuser",
	Passkey:      passkey,
}

func TestAnimeBytesCases(t *testing.T) {
	ported.Run(t, "anime_bytes_tests/anime_bytes.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_animebytes": "TestRecentFeed",
	}})
}

// TestRecentFeed ports Prowlarr's should_parse_recent_feed_from_animebytes,
// whose passkey, "somepass", is too short for the settings' validation.
func TestRecentFeed(t *testing.T) {
	ix := newIndexer(t, testSettings, respond("application/json", string(ported.Response(t, "AnimeBytes/recentfeed.json"))))
	ix.Site.(*Site).settings.Passkey = "somepass"
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "test", Categories: []int{2000, 5000}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(releases), 39; got != want {
		t.Fatalf("len(releases) = %d, want = %d", got, want)
	}
	tests := []struct {
		index                       int
		title, download, info, guid string
		published                   time.Time
		size                        int64
		seeders, leechers, files    int
		minimumSeedTime             int64
	}{
		{
			3, "[SubsPlease] One Piece: The Great Gold Pirate - 1059 [Web][MKV][h264][720p][AAC 2.0][Softsubs (SubsPlease)][Episode 1059]",
			"https://animebytes.tv/torrent/1043925/download/", "https://animebytes.tv/torrent/1043925/group",
			"https://animebytes.tv/torrent/1043925/group?nh=0F6BB43603CC07F4C804B9A29139F852",
			time.Date(2023, 4, 23, 2, 6, 8, 0, time.UTC), 743629489, 114, 1, 1, 259200,
		},
		{
			20, "[GHOST] BLEACH S03 [Blu-ray][MKV][h265 10-bit][1080p][AC3 2.0][Dual Audio][Softsubs (GHOST)]",
			"https://animebytes.tv/torrent/1031203/download/", "https://animebytes.tv/torrent/1031203/group",
			"https://animebytes.tv/torrent/1031203/group?nh=F7C73EF631FE269D3A7F10BD12EC99A1",
			time.Date(2023, 4, 3, 3, 14, 35, 0, time.UTC), 24498538059, 12, 2, 22, 655200,
		},
		{
			23, "[Polarwindz] Cowboy Bebop: Tengoku no Tobira 2001 [Blu-ray][MKV][h265 10-bit][1080p][Opus 5.1][Softsubs (Polarwindz)]",
			"https://animebytes.tv/torrent/959397/download/", "https://animebytes.tv/torrent/959397/group",
			"https://animebytes.tv/torrent/959397/group?nh=D63895DA87A25239C11F9823F46000E1",
			time.Date(2023, 4, 2, 5, 0, 43, 0, time.UTC), 13090646841, 5, 1, 1, 475200,
		},
		{
			5, "[SubsPlease] Dr. STONE: NEW WORLD S03E03 - 03 [Web][MKV][h264][720p][AAC 2.0][Softsubs (SubsPlease)][Episode 3]",
			"https://animebytes.tv/torrent/1041495/download/", "https://animebytes.tv/torrent/1041495/group",
			"https://animebytes.tv/torrent/1041495/group?nh=8B78B0DD3BCC6068BFCD927E4AC674F6",
			time.Date(2023, 4, 20, 14, 32, 29, 0, time.UTC), 748209543, 137, 3, 1, 259200,
		},
		{
			28, "[-ZR-] Dr. STONE: STONE WARS 2021 S02 [Web][MKV][h264][1080p][AAC 2.0][Dual Audio][Softsubs (-ZR-)]",
			"https://animebytes.tv/torrent/944509/download/", "https://animebytes.tv/torrent/944509/group",
			"https://animebytes.tv/torrent/944509/group?nh=FDCAA1EAB36D7C802F1E4B13DAE5EED7",
			time.Date(2021, 6, 3, 20, 30, 0, 0, time.UTC), 16611719364, 31, 1, 11, 529200,
		},
		{
			37, "[HorribleSubs] Dr. STONE S01 [Web][MKV][h264][720p][AAC 2.0][Softsubs (HorribleSubs)]",
			"https://animebytes.tv/torrent/430074/download/", "https://animebytes.tv/torrent/430074/group",
			"https://animebytes.tv/torrent/430074/group?nh=32279E138015D8718B2B4B49AEF64574",
			time.Date(2019, 12, 13, 17, 2, 48, 0, time.UTC), 16366224176, 33, 1, 24, 529200,
		},
	}
	for _, tt := range tests {
		r := releases[tt.index]
		checks := []struct {
			name      string
			got, want any
		}{
			{"Title", r.Title, tt.title},
			{"Protocol", r.Protocol, indexer.ProtocolTorrent},
			{"DownloadURL", r.DownloadURL, tt.download + "somepass"},
			{"InfoURL", r.InfoURL, tt.info},
			{"GUID", r.GUID, tt.guid},
			{"CommentsURL", r.CommentsURL, ""},
			{"PublishDate", r.PublishDate, tt.published},
			{"Size", *r.Size, tt.size},
			{"InfoHash", r.InfoHash, ""},
			{"MagnetURL", r.MagnetURL, ""},
			{"Peers", *r.Peers, tt.seeders + tt.leechers},
			{"Seeders", *r.Seeders, tt.seeders},
			{"Files", *r.Files, tt.files},
			{"MinimumSeedTime", *r.MinimumSeedTime, tt.minimumSeedTime},
		}
		for _, c := range checks {
			if c.got != c.want {
				t.Errorf("release %d %s = %v, want = %v", tt.index, c.name, c.got, c.want)
			}
		}
	}
}

func TestSearchRequests(t *testing.T) {
	byYear := testSettings
	byYear.SearchByYear = true
	byYear.ExcludeHentai = true
	byYear.FreeleechOnly = true
	const prefix = "https://animebytes.tv/scrape.php?username=someuser&torrent_pass=" + passkey + "&sort=grouptime&way=desc&type="
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"rss", testSettings, indexer.Query{}, prefix + "anime&searchstr=&limit=20"},
		{
			"tv episode in a season", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Dr. STONE S03E03", Season: indexer.Ptr(3), Categories: []int{5070}},
			prefix + "anime&searchstr=Dr.+STONE&limit=50&anime[tv_series]=1&anime[ona]=1",
		},
		{
			"movie by year", byYear,
			indexer.Query{Type: indexer.SearchMovie, Term: "Cowboy Bebop The Movie 2001"},
			prefix + "anime&searchstr=Cowboy+Bebop&limit=50&year=2001&freeleech=1&hentai=0",
		},
		{
			"music", byYear,
			indexer.Query{Type: indexer.SearchMusic, Term: "Tank!", Artist: "Seatbelts", Album: "Cowboy Bebop", Year: 1998, Categories: []int{3000}},
			prefix + "music&searchstr=Tank&limit=50&artistnames=Seatbelts&groupname=Cowboy+Bebop&year=1998&audio=1&freeleech=1",
		},
		{
			"various artists", testSettings,
			indexer.Query{Type: indexer.SearchMusic, Artist: "VA"},
			prefix + "music&searchstr=&limit=20",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
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

func TestCleanSearchTerm(t *testing.T) {
	tests := []struct{ in, want string }{
		{"One Piece 1059", "One Piece"},
		{"Show 2x05", "Show"},
		{"Show S01E05", "Show"},
		{"Cowboy Bebop The Movie", "Cowboy Bebop"},
		{"Bleach", "Bleach"},
	}
	for _, tt := range tests {
		if got := cleanSearchTerm(tt.in); got != tt.want {
			t.Errorf("cleanSearchTerm(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

func TestSeasonFromTitles(t *testing.T) {
	tests := []struct {
		titles []string
		want   int
		ok     bool
	}{
		{[]string{"Shingeki no Kyojin 3rd Season"}, 3, true},
		{[]string{"Show Season 4"}, 4, true},
		{[]string{"Show III"}, 3, true},
		{[]string{"Show 2"}, 2, true},
		{[]string{"Show Part 2"}, 0, false},
		{[]string{"Show"}, 0, false},
	}
	for _, tt := range tests {
		got, ok := seasonFromTitles(tt.titles)
		if got != tt.want || ok != tt.ok {
			t.Errorf("seasonFromTitles(%q) = %d, %v, want = %d, %v", tt.titles, got, ok, tt.want, tt.ok)
		}
	}
}

func TestParse(t *testing.T) {
	const body = `{"Matches":1,"Groups":[{"ID":1,"CategoryName":"Anime","FullName":"Show &amp; Co","GroupName":"TV Series","SeriesName":"","Year":"2020",
"SynonymnsV2":{"Japanese":"ショー","Romaji":"show","Alternative":"Alt One, Alt Two"},"Description":"desc",
"Torrents":[
{"ID":10,"EditionData":{"EditionTitle":"Season 2"},"RawDownMultiplier":0,"RawUpMultiplier":1,"Link":"https://animebytes.tv/torrent/10/download/x",
"Property":"Blu-ray | M2TS | H.264 | 1080p | Freeleech | RAW","Snatched":3,"Seeders":4,"Leechers":5,"Size":2147483648,"FileCount":2,
"FileList":[{"filename":"BDMV/Show Remux 2020.m2ts","size":1},{"filename":"Show.nfo","size":1}],"UploadTime":"2024-01-02 03:04:05"},
{"ID":11,"RawDownMultiplier":1,"RawUpMultiplier":1,"Link":"https://animebytes.tv/torrent/11/download/x",
"Property":"Web | MKV | h264 | 720p | Softsubs (Grp)","Snatched":0,"Seeders":0,"Leechers":0,"Size":1,"FileCount":1,
"FileList":[{"filename":"[Grp] Show - 01.mkv","size":1}],"UploadTime":"2024-01-03 03:04:05"}]}]}`
	ix := newIndexer(t, testSettings, respond("application/json", body))
	releases, err := ix.Site.ParseResponse(context.Background(), ix, &indexer.Response{
		Request: &indexer.Request{}, StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, r := range releases {
		titles = append(titles, r.Title)
	}
	want := []string{
		"[Grp] Show - 01.mkv",
		"[Grp] Show & Co S01 [Web][MKV][h264][720p][Softsubs (Grp)]",
		"[Grp] ショー S01 [Web][MKV][h264][720p][Softsubs (Grp)]",
		"[Grp] show S01 [Web][MKV][h264][720p][Softsubs (Grp)]",
		"[Grp] Alt One S01 [Web][MKV][h264][720p][Softsubs (Grp)]",
		"[Grp] Alt Two S01 [Web][MKV][h264][720p][Softsubs (Grp)]",
		"BDMV/Show Remux 2020.m2ts",
		"Show & Co 2020 S02 [Blu-ray][M2TS][AVC][1080p Remux][RAW][BR-DISK]",
		"ショー 2020 S02 [Blu-ray][M2TS][AVC][1080p Remux][RAW][BR-DISK]",
		"show 2020 S02 [Blu-ray][M2TS][AVC][1080p Remux][RAW][BR-DISK]",
		"Alt One 2020 S02 [Blu-ray][M2TS][AVC][1080p Remux][RAW][BR-DISK]",
		"Alt Two 2020 S02 [Blu-ray][M2TS][AVC][1080p Remux][RAW][BR-DISK]",
	}
	if strings.Join(titles, "\n") != strings.Join(want, "\n") {
		t.Errorf("titles = %q, want = %q", titles, want)
	}
	r := releases[len(releases)-1]
	if got, want := *r.MinimumSeedTime, int64(259200+2*18000); got != want {
		t.Errorf("MinimumSeedTime = %d, want = %d", got, want)
	}
	if got, want := *r.DownloadVolumeFactor, 0.0; got != want {
		t.Errorf("DownloadVolumeFactor = %v, want = %v", got, want)
	}
	if got, want := r.Year, 2020; got != want {
		t.Errorf("Year = %d, want = %d", got, want)
	}
	if got, want := r.Categories, []int{5070}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("Categories = %v, want = %v", got, want)
	}
	// The Japanese title's MD5 is of its ASCII encoding, question marks.
	if got, want := releases[2].GUID, "https://animebytes.tv/torrent/11/group?nh="+md5Hex("???"); got != want {
		t.Errorf("GUID = %q, want = %q", got, want)
	}
}

func TestParseOptions(t *testing.T) {
	const body = `{"Matches":1,"Groups":[{"CategoryName":"Anime","FullName":"Show","GroupName":"TV Series","Year":2020,"SynonymnsV2":{"Japanese":"ショー"},
"Torrents":[{"ID":10,"RawDownMultiplier":0,"RawUpMultiplier":1,"Link":"l","Property":"Blu-ray | ISO","Size":1,"FileList":[],"UploadTime":"2024-01-02 03:04:05"},
{"ID":11,"RawDownMultiplier":1,"RawUpMultiplier":1,"Link":"l","Property":"Web","Size":1,"FileList":[],"UploadTime":"2024-01-02 03:04:05"}]}]}`
	s := testSettings
	s.ExcludeRaw = true
	s.FreeleechOnly = true
	s.AddJapaneseTitle = indexer.Ptr(false)
	s.EnableSonarrCompatibility = indexer.Ptr(false)
	ix := newIndexer(t, s, nil)
	releases, err := ix.Site.ParseResponse(context.Background(), ix, &indexer.Response{
		Request: &indexer.Request{}, StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 0 {
		t.Errorf("releases = %d, want = 0", len(releases))
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		want                    string
	}{
		{"error", "application/json", `{"error":"Invalid passkey"}`, "Unexpected response from indexer request: Invalid passkey"},
		{"html", "text/html", "<html>", "Unexpected response header text/html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, respond(tt.contentType, tt.body))
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = %q", err, tt.want)
			}
		})
	}
}

func TestCleanupReleases(t *testing.T) {
	now := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	site := &Site{now: func() time.Time { return now }}
	var releases []*indexer.Release
	for i := range 25 {
		releases = append(releases, &indexer.Release{Title: "r", PublishDate: now.Add(-time.Duration(i) * time.Hour * 2)})
	}
	if got := len(site.CleanupReleases(append([]*indexer.Release(nil), releases...), &indexer.Query{})); got != 20 {
		t.Errorf("RSS releases = %d, want = 20", got)
	}
	if got := len(site.CleanupReleases(append([]*indexer.Release(nil), releases...), &indexer.Query{Term: "x"})); got != 25 {
		t.Errorf("search releases = %d, want = 25", got)
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{Username: "u", Passkey: "short"}).Validate(); err == nil || !strings.Contains(err.Error(), "32, 48 or 56") {
		t.Errorf("Validate() = %v, want = a passkey length error", err)
	}
}
