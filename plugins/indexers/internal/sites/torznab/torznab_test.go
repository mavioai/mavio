package torznab

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/newznab"
)

type feedTransport []byte

func (f feedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/rss+xml"}},
		Body:       io.NopCloser(strings.NewReader(string(f))),
		Request:    r,
	}, nil
}

// testCaps are the capabilities TorznabFixture gives its indexer.
func testCaps() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Categories.Add("2000", indexer.Movies, "Movies")
	c.Categories.Add("2040", indexer.MoviesHD, "Movies/HD")
	c.Categories.Add("5000", indexer.TV, "TV")
	c.Categories.Add("5040", indexer.TVHD, "TV/HD")
	c.Categories.Add("5070", indexer.TVAnime, "TV/Anime")
	return c
}

func search(t *testing.T, feed string) []*indexer.Release {
	t.Helper()
	site, err := New(newznab.Settings{BaseURL: "http://indexer.local/"})
	if err != nil {
		t.Fatal(err)
	}
	caps := testCaps()
	site.caps.Set(caps, time.Now())
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = feedTransport(ported.Response(t, feed))
	ix := &indexer.Indexer{ID: "1", Name: "Torznab", Site: site, Session: c.Session(nil, nil), Statuses: st}
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie})
	if err != nil {
		t.Fatal(err)
	}
	return releases
}

func TestTorznabCases(t *testing.T) {
	ported.Run(t, "torznab_tests/torznab.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_torznab_hdaccess_net": "TestRecentFeeds",
		"should_parse_recent_feed_from_torznab_tpb":          "TestRecentFeeds",
		"should_parse_recent_feed_from_torznab_animetosho":   "TestRecentFeeds",
		"should_parse_recent_feed_from_torznab_morethantv":   "TestRecentFeeds",
		"should_use_pagesize_reported_by_caps":               "TestPageSizeFromCaps",
	}})
}

type want struct {
	count                                 int
	title, download, info, comments, hash string
	published                             time.Time
	size                                  int64
	seeders, peers                        *int
	categories                            int
	// The release has no TVDB or TVRage ID.
	noIDs bool
}

func TestRecentFeeds(t *testing.T) {
	tests := []struct {
		feed string
		want want
	}{
		{"Torznab/torznab_hdaccess_net.xml", want{
			count:     5,
			title:     "Better Call Saul S01E05 Alpine Shepherd 1080p NF WEBRip DD5.1 x264",
			download:  "https://hdaccess.net/download.php?torrent=11515&passkey=123456",
			info:      "https://hdaccess.net/details.php?id=11515&hit=1",
			comments:  "https://hdaccess.net/details.php?id=11515&hit=1#comments",
			published: time.Date(2015, 3, 14, 21, 10, 42, 0, time.UTC),
			size:      2538463390,
			hash:      "63e07ff523710ca268567dad344ce1e0e6b7e8a3",
			seeders:   indexer.Ptr(7), peers: indexer.Ptr(7),
		}},
		{"Torznab/torznab_tpb.xml", want{count: 0}},
		{"Torznab/torznab_animetosho.xml", want{
			count:     2,
			title:     "[finFAGs]_Frame_Arms_Girl_07_(1280x720_TV_AAC)_[1262B6F7].mkv",
			download:  "http://storage.localhost/torrents/123451.torrent",
			info:      "https://localhost/view/finfags-_frame_arms_girl_07_-1280x720_tv_aac-_-1262b6f7-mkv.123451",
			comments:  "https://localhost/view/finfags-_frame_arms_girl_07_-1280x720_tv_aac-_-1262b6f7-mkv.123451",
			published: time.Date(2017, 5, 17, 20, 36, 6, 0, time.UTC),
			size:      316477946,
			hash:      "2d69a861bef5a9f2cdf791b7328e37b7953205e1",
			noIDs:     true,
		}},
		{"Torznab/torznab_morethantv.xml", want{
			count:     2,
			title:     "Out of the Past 1947 720p BluRay FLAC2.0 x264-CtrlHD.mkv",
			download:  "https://www.morethantv.me/torrents.php?action=download&id=(removed)&authkey=(removed)&torrent_pass=(removed)",
			info:      "https://www.morethantv.me/torrents.php?id=(removed)&torrentid=836164",
			comments:  "https://www.morethantv.me/torrents.php?id=(removed)&torrentid=836164",
			published: time.Date(2022, 12, 20, 21, 32, 17, 0, time.UTC),
			size:      5412993028,
			hash:      "(removed)",
			seeders:   indexer.Ptr(3), peers: indexer.Ptr(3),
			categories: 4,
			noIDs:      true,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.feed, func(t *testing.T) {
			releases := search(t, tt.feed)
			if len(releases) != tt.want.count {
				t.Fatalf("got = %d releases, want = %d", len(releases), tt.want.count)
			}
			if tt.want.count == 0 {
				return
			}
			r, w := releases[0], tt.want
			for _, c := range []struct{ name, got, want string }{
				{"title", r.Title, w.title},
				{"download", r.DownloadURL, w.download},
				{"info", r.InfoURL, w.info},
				{"comments", r.CommentsURL, w.comments},
				{"info hash", r.InfoHash, w.hash},
			} {
				if c.got != c.want {
					t.Errorf("%s: got = %q, want = %q", c.name, c.got, c.want)
				}
			}
			if r.Protocol != indexer.ProtocolTorrent {
				t.Errorf("protocol: got = %v, want = torrent", r.Protocol)
			}
			if !r.PublishDate.Equal(w.published) {
				t.Errorf("published: got = %v, want = %v", r.PublishDate, w.published)
			}
			if r.Size == nil || *r.Size != w.size {
				t.Errorf("size: got = %v, want = %d", r.Size, w.size)
			}
			if w.noIDs && (r.TVDBID != 0 || r.TVRageID != 0) {
				t.Errorf("ids: got = %d/%d, want = 0/0", r.TVDBID, r.TVRageID)
			}
			if !samePtr(r.Seeders, w.seeders) || !samePtr(r.Peers, w.peers) {
				t.Errorf("seeders/peers: got = %v/%v, want = %v/%v", deref(r.Seeders), deref(r.Peers), deref(w.seeders), deref(w.peers))
			}
			if w.categories > 0 && len(r.Categories) != w.categories {
				t.Errorf("categories: got = %v, want = %d", r.Categories, w.categories)
			}
		})
	}
}

func samePtr(a, b *int) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestPageSizeFromCaps(t *testing.T) {
	site, err := New(newznab.Settings{BaseURL: "http://indexer.local/"})
	if err != nil {
		t.Fatal(err)
	}
	caps := testCaps()
	caps.LimitsMax, caps.LimitsDefault = 30, 25
	site.caps.Set(caps, time.Now())
	if got := site.Info().PageSize; got != 25 {
		t.Errorf("got = %d, want = 25", got)
	}
}
