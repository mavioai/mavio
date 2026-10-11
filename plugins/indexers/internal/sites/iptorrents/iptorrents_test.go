package iptorrents

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	return &indexer.Indexer{ID: "1", Name: "IPTorrents", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{
	BaseSettings: indexer.BaseSettings{BaseURL: "https://iptorrents.com/"},
	Cookie:       "uid=123; pass=abc",
	UserAgent:    "Mozilla/5.0",
}

func TestIPTorrentsCases(t *testing.T) {
	ported.Run(t, "ip_torrents_tests/ip_torrents_request_generator.json", ported.Cases{Facts: map[string]string{
		"should_not_sort_by_seeders_by_default":      "TestSearchRequests",
		"should_sort_by_seeders_when_enabled":        "TestSearchRequests",
		"should_not_sort_rss_feed_even_when_enabled": "TestSearchRequests",
	}})
}

func TestSearchRequests(t *testing.T) {
	sorted := testSettings
	sorted.SearchSortBy = SortSeeders
	free := testSettings
	free.FreeleechOnly = true
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			"movie by IMDb ID, default sort", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000}, IMDbID: "0076759"},
			"https://iptorrents.com/t?72=&87=&77=&101=&89=&90=&96=&6=&48=&54=&62=&38=&68=&20=&7=&100=&q=%2B(tt0076759)&qf=all",
		},
		{
			"movie sorted by seeders", sorted,
			indexer.Query{Type: indexer.SearchMovie, Categories: []int{2030}, IMDbID: "0076759"},
			"https://iptorrents.com/t?77=&90=&96=&62=&7=&o=seeders&q=%2B(tt0076759)&qf=all",
		},
		{
			"RSS not sorted", sorted,
			indexer.Query{Categories: []int{2030}},
			"https://iptorrents.com/t?77=&90=&96=&62=&7=",
		},
		{
			"season search", free,
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(2), Categories: []int{5040}},
			"https://iptorrents.com/t?23=&5=&99=&free=on&q=%2B(The+Show+S02*)",
		},
		{
			"page", testSettings,
			indexer.Query{Term: "linux", Limit: 50, Offset: 100},
			"https://iptorrents.com/t?q=%2B(linux)&p=3",
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
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("User-Agent"); got != "Mozilla/5.0" {
				t.Errorf("User-Agent = %q, want = %q", got, "Mozilla/5.0")
			}
			if got := r.Cookies["pass"]; got != "abc" {
				t.Errorf("cookie pass = %q, want = %q", got, "abc")
			}
		})
	}
}

// page is a search page as IPTorrents shows it, with the category
// column showing icons.
const page = `<!DOCTYPE html><html><body>
<a href="/lout.php">Logout</a>
<table id="torrents">
<thead><tr><th>Type</th><th>Name</th><th>Sort by files</th><th>DL</th><th>Sort by comments</th><th>Sort by size</th><th>Sort by snatches</th><th>Sort by seeders</th><th>Sort by leechers</th></tr></thead>
<tbody>
<tr>
<td><a href="?5"><img alt="TV/x264"></a></td>
<td><a class="hv" href="/t/4123456">[REQ] The.Show.S02E01.720p.HDTV.x264-GRP</a> <span class="free">FreeLeech</span><div class="sub">720p, x264 | 2.5 hours ago by Uploader</div></td>
<td>3 Go to files</td>
<td><a href="/download.php/4123456/The.Show.S02E01.720p.HDTV.x264-GRP.torrent"><i></i></a></td>
<td>0</td>
<td>1.37 GB</td>
<td>1,204</td>
<td>87</td>
<td>4</td>
</tr>
<tr>
<td><a href="?72"><img alt="Movies"></a></td>
<td><a class="hv" href="/t/4123457">[ita] A Film 2020 1080p BluRay x264-GRP</a><div class="sub">1 day ago</div></td>
<td>1 Go to files</td>
<td><a href="/download.php/4123457/A.Film.torrent"><i></i></a></td>
<td>2</td>
<td>8.5 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
</tbody>
</table></body></html>`

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(page))
	before := time.Now()
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
		{"GUID", r.GUID, "https://iptorrents.com/t/4123456"},
		{"InfoURL", r.InfoURL, "https://iptorrents.com/t/4123456"},
		{"DownloadURL", r.DownloadURL, "https://iptorrents.com/download.php/4123456/The.Show.S02E01.720p.HDTV.x264-GRP.torrent"},
		{"Description", r.Description, "Tags: 720p, x264"},
		{"Categories", r.Categories, []int{5040, 100005}},
		{"Size", *r.Size, int64(1471026298)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(1209600)},
		{"second Title", r2.Title, "A Film 2020 1080p BluRay x264-GRP"},
		{"second Description", r2.Description, ""},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if want := before.Add(-150 * time.Minute); r.PublishDate.Sub(want).Abs() > time.Minute {
		t.Errorf("PublishDate = %v, want = %v", r.PublishDate, want)
	}
}

// TestSearchFollowsRedirects checks that searches follow redirects, as
// Prowlarr's IndexerRequests of URLs do in production.
func TestSearchFollowsRedirects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
			if r.URL.Path == "/t" {
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/t/moved"}}, Body: http.NoBody}
			}
			return respond(page)(r)
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if len(releases) != 2 {
			t.Fatalf("releases = %d, want = 2", len(releases))
		}
		if got, want := releases[0].DownloadURL, "https://iptorrents.com/download.php/4123456/The.Show.S02E01.720p.HDTV.x264-GRP.torrent"; got != want {
			t.Errorf("DownloadURL = %q, want = %q", got, want)
		}
	})
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, body string
		want       string
		auth       bool
	}{
		{"logged out", `<html><body><form action="/take_login.php"></form></body></html>`, "authentication", true},
		{"text categories", strings.ReplaceAll(page, `<a href="?5">`, `<a href="/t?5">`), "Icons", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, respond(tt.body))
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

func TestCleanTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"[REQUESTED] Some.Title-GRP", "Some.Title-GRP"},
		{"[eng] Show S01E01 1080p-GRP", "Show S01E01 1080p-GRP"},
		{" - Title Ω: ", "Title"},
		{"[SubsPlease] Anime - 01 (1080p)", "[SubsPlease] Anime - 01 (1080p)"},
	}
	for _, tt := range tests {
		if got := cleanTitle(tt.in); got != tt.want {
			t.Errorf("cleanTitle(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{Cookie: "a=b"}).Validate(); err == nil || !strings.Contains(err.Error(), "user_agent") {
		t.Errorf("Validate() = %v, want = a user_agent error", err)
	}
}
