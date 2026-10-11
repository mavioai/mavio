package torrentrss

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

const feedURL = "http://my.indexer.tv/recent"

// feedTransport answers every request with a recorded feed.
type feedTransport struct{ body []byte }

func (t feedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       ioNopCloser{strings.NewReader(string(t.body))},
		Request:    r,
	}, nil
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }

func feed(t *testing.T, name string) []byte {
	t.Helper()
	return ported.Response(t, name)
}

func response(t *testing.T, name string) *indexer.Response {
	t.Helper()
	return &indexer.Response{
		Request:    indexer.NewRequest(feedURL, indexer.AcceptRSS),
		URL:        feedURL,
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       feed(t, name),
	}
}

func newIndexer(t *testing.T, name string, s Settings) *indexer.Indexer {
	t.Helper()
	site, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = feedTransport{feed(t, name)}
	return &indexer.Indexer{ID: "1", Name: "TorrentRssIndexer", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestTorrentRSSCases(t *testing.T) {
	facts := map[string]string{}
	for _, f := range []string{
		"should_parse_recent_feed_from_ImmortalSeed", "should_parse_recent_feed_from_Ezrss",
		"should_parse_recent_feed_from_ShowRSS_info", "should_parse_recent_feed_from_Doki",
		"should_parse_recent_feed_from_ExtraTorrents", "should_parse_recent_feed_from_LimeTorrents",
		"should_parse_recent_feed_from_AnimeTosho_without_size", "should_parse_multi_enclosure_from_AnimeTosho",
		"should_parse_recent_feed_from_AlphaRatio", "should_parse_recent_feed_from_EveolutionWorld_without_size",
		"should_record_indexer_failure_if_unsupported_feed",
	} {
		facts[f] = "TestRecentFeeds"
	}
	ported.Run(t, "torrent_rss_tests/torrent_rss_indexer.json", ported.Cases{Facts: facts})

	detectFacts := map[string]string{}
	for _, f := range []string{
		"should_detect_rss_settings_for_ezrss", "should_detect_rss_settings_for_speed_cd",
		"should_detect_rss_settings_for_ImmortalSeed", "should_detect_rss_settings_for_ShowRSS_info",
		"should_detect_rss_settings_for_TransmitTheNet", "should_detect_rss_settings_for_BitHdtv",
		"should_detect_rss_settings_for_Doki", "should_detect_rss_settings_for_ExtraTorrents",
		"should_detect_rss_settings_for_LimeTorrents", "should_detect_rss_settings_for_AlphaRatio",
		"should_detect_rss_settings_for_AnimeTosho_without_size",
	} {
		detectFacts[f] = "TestDetect"
	}
	ported.Run(t, "torrent_rss_tests/torrent_rss_settings_detector.json", ported.Cases{
		Facts: detectFacts,
		Skip: map[string]string{
			"should_reject_rss_settings_for_AwesomeHD": "ignored upstream: cannot reliably reject unparseable titles",
		},
		Run: map[string]func(*testing.T, ported.Args){
			"should_detect_recent_feed": func(t *testing.T, a ported.Args) {
				file, _ := a.Str(t, "rssXmlFile")
				if ps, err := Detect(response(t, file), Settings{BaseURL: feedURL}); err != nil || ps == nil {
					t.Errorf("Detect(%s) got = %v, %v, want = settings", file, ps, err)
				}
			},
			"should_reject_recent_feed_with_invalid_downloadurl": func(t *testing.T, a ported.Args) {
				checkRejected(t, a, false, "download url")
			},
			"should_reject_recent_feed_without_pubDate": func(t *testing.T, a ported.Args) {
				checkRejected(t, a, false, "RSS feed must have a pubDate")
			},
			"should_detect_feed_without_size": func(t *testing.T, a ported.Args) {
				file, _ := a.Str(t, "rssXmlFile")
				ps, err := Detect(response(t, file), Settings{BaseURL: feedURL, AllowZeroSize: true})
				if err != nil || ps == nil || ps.UseEnclosureLength || ps.ParseSizeInDescription || ps.SizeElementName != "" {
					t.Errorf("Detect(%s) got = %+v, %v, want = settings without sizes", file, ps, err)
				}
			},
			"should_reject_feed_without_size": func(t *testing.T, a ported.Args) {
				checkRejected(t, a, false, "content size")
			},
		},
	})

	ported.Run(t, "torrent_rss_tests/torrent_rss_parser_factory.json", ported.Cases{Facts: map[string]string{
		"should_return_ezrssparser":                        "TestParserFactory",
		"should_return_generic_torrentrssparser":           "TestParserFactory",
		"should_throw_on_failure":                          "TestParserFactory",
		"should_cache_settings_for_same_baseurl":           "TestParserFactory",
		"should_not_cache_failure":                         "TestParserFactory",
		"should_not_cache_settings_for_different_baseurl":  "TestParserFactory",
		"should_not_cache_settings_for_different_settings": "TestParserFactory",
	}})
}

func checkRejected(t *testing.T, a ported.Args, allowZero bool, want string) {
	t.Helper()
	file, _ := a.Str(t, "rssXmlFile")
	_, err := Detect(response(t, file), Settings{BaseURL: feedURL, AllowZeroSize: allowZero})
	if !errors.Is(err, indexer.ErrUnsupportedFeed) || !strings.Contains(err.Error(), want) {
		t.Errorf("Detect(%s) error got = %v, want = unsupported feed with %q", file, err, want)
	}
}

func TestRecentFeeds(t *testing.T) {
	type want struct {
		title, download, info, comments string
		date                            string
		size                            int64
		infoHash, magnet                string
		peers, seeders                  int
	}
	for _, tt := range []struct {
		file      string
		allowZero bool
		count     int
		last      bool
		want      want
	}{
		{"TorrentRss/ImmortalSeed.xml", false, 50, false, want{
			title: "Conan.2015.02.05.Jeff.Bridges.720p.HDTV.X264-CROOKS", download: "https://immortalseed.me/download.php?type=rss&secret_key=12345678910&id=374534",
			date: "2015-02-06T13:32:26Z", size: 984078090, peers: 8, seeders: 6,
		}},
		{"TorrentRss/Ezrss.xml", false, 3, false, want{
			title: "S4C I Grombil Cyfandir Pell American Interior [PDTV - MVGROUP]", download: "http://re.zoink.it/20a4ed4eFC",
			info: "http://eztv.it/ep/58439/s4c-i-grombil-cyfandir-pell-american-interior-pdtv-x264-mvgroup/", comments: "http://eztv.it/forum/discuss/58439/",
			date: "2014-09-15T18:39:00Z", size: 796606175, infoHash: "20FC4FBFA88272274AC671F857CC15144E9AA83E",
			magnet: "magnet:?xt=urn:btih:ED6E7P5IQJZCOSWGOH4FPTAVCRHJVKB6&dn=S4C.I.Grombil.Cyfandir.Pell.American.Interior.PDTV.x264-MVGroup",
			peers:  -1, seeders: -1,
		}},
		{"TorrentRss/ShowRSS.info.xml", true, 5, false, want{
			title:    "The Voice 8x25",
			download: "magnet:?xt=urn:btih:96CD620BEDA3EFD7C4D7746EF94549D03A2EB13B&dn=The+Voice+S08E25+WEBRip+x264+WNN&tr=udp://tracker.coppersurfer.tk:6969/announce&tr=udp://tracker.leechers-paradise.org:6969&tr=udp://open.demonii.com:1337",
			date:     "2015-05-15T08:30:01Z", infoHash: "96CD620BEDA3EFD7C4D7746EF94549D03A2EB13B",
			magnet: "magnet:?xt=urn:btih:96CD620BEDA3EFD7C4D7746EF94549D03A2EB13B&dn=The+Voice+S08E25+WEBRip+x264+WNN&tr=udp://tracker.coppersurfer.tk:6969/announce&tr=udp://tracker.leechers-paradise.org:6969&tr=udp://open.demonii.com:1337",
			peers:  1, seeders: 1,
		}},
		{"TorrentRss/Doki.xml", true, 5, false, want{
			title:    "[Doki] PriPara   50 (848x480 h264 AAC) [6F0B49FD] mkv",
			download: "http://tracker.anime-index.org/download.php?id=82d8ad84403e01a7786130905ca169a3429e657f&f=%5BDoki%5D+PriPara+-+50+%28848x480+h264+AAC%29+%5B6F0B49FD%5D.mkv.torrent",
			date:     "2015-07-02T08:18:29Z", peers: 1, seeders: 1,
		}},
		{"TorrentRss/ExtraTorrents.xml", false, 5, false, want{
			title:    "One.Piece.E334.D ED.720p.HDTV.x264-W4F-={SPARROW}=-",
			download: "http://ac.me/download/4722030/One.Piece.E334.D+ED.720p.HDTV.x264-W4F-%3D%7BSPARROW%7D%3D-.torrent",
			date:     "2016-02-21T09:51:54Z", size: 562386947, peers: 1, seeders: 1,
		}},
		{"TorrentRss/LimeTorrents.xml", false, 5, false, want{
			title:    "The Expanse 2x04 (720p-HDTV-x264-SVA)[VTV]",
			download: "http://itorrents.org/torrent/51C578C9823DD58F6EEA287C368ED935843D63AB.torrent?title=The-Expanse-2x04-(720p-HDTV-x264-SVA)[VTV]",
			comments: "http://www.limetorrents.cc/The-Expanse-2x04-(720p-HDTV-x264-SVA)[VTV]-torrent-8643587.html",
			date:     "2017-02-16T02:24:26Z", size: 880496711, peers: 1, seeders: 1,
		}},
		{"TorrentRss/AnimeTosho_NoSize.xml", false, 2, false, want{
			title:    "[FFF] Ore Monogatari!! - Vol.01 [BD][720p-AAC]",
			download: "http://storage.animetosho.org/torrents/85a570f25067f69b3c83b901ce6c00c491345288/%5BFFF%5D%20Ore%20Monogatari%21%21%20-%20Vol.01%20%5BBD%5D%5B720p-AAC%5D.torrent",
			comments: "https://animetosho.org/view/fff-ore-monogatari-vol-01-bd-720p-aac.1009077",
			date:     "2016-08-02T13:48:04Z", size: int64(math.Round(1.366 * 1024 * 1024 * 1024)), peers: 1, seeders: 1,
		}},
		{"TorrentRss/AnimeTosho_NoSize.xml", false, 2, true, want{
			title:    "DAYS - 05 (1280x720 HEVC2 AAC).mkv",
			download: "http://storage.animetosho.org/torrents/4b58360143d59a55cbd922397a3eaa378165f3ff/DAYS%20-%2005%20%281280x720%20HEVC2%20AAC%29.torrent",
		}},
		{"TorrentRss/AlphaRatio.xml", false, 2, true, want{
			title:    "TvHD 465860 465831 WWE.RAW.2016.11.28.720p.HDTV.x264-KYR",
			download: "https://alpharatio.cc/torrents.php?action=download&authkey=private_auth_key&torrent_pass=private_torrent_pass&id=465831",
		}},
		{"TorrentRss/EvolutionWorld.xml", true, 2, false, want{
			title:    "[TVShow --> TVShow Bluray 720p] Fargo S01 Complete Season 1 720p BRRip DD5.1 x264-PSYPHER [SEEDERS (3)/LEECHERS (0)]",
			download: "http://ew.pw/download.php?id=dea071a7a62a0d662538d46402fb112f30b8c9fa&f=Fargo%20S01%20Complete%20Season%201%20720p%20BRRip%20DD5.1%20x264-PSYPHER.torrent&auth=secret",
			date:     "2017-08-13T22:21:43Z", peers: 1, seeders: 1,
		}},
	} {
		t.Run(tt.file, func(t *testing.T) {
			ix := newIndexer(t, tt.file, Settings{BaseURL: feedURL, AllowZeroSize: tt.allowZero})
			rs, err := ix.Search(t.Context(), &indexer.Query{})
			if err != nil {
				t.Fatal(err)
			}
			if len(rs) != tt.count {
				t.Fatalf("releases got = %d, want = %d", len(rs), tt.count)
			}
			r := rs[0]
			if tt.last {
				r = rs[len(rs)-1]
			}
			w := tt.want
			if r.Title != w.title || r.DownloadURL != w.download || r.Protocol != indexer.ProtocolTorrent {
				t.Errorf("got = %q %q %v, want = %q %q torrent", r.Title, r.DownloadURL, r.Protocol, w.title, w.download)
			}
			if w.date == "" {
				return
			}
			if r.InfoURL != w.info || r.CommentsURL != w.comments {
				t.Errorf("info, comments got = %q %q, want = %q %q", r.InfoURL, r.CommentsURL, w.info, w.comments)
			}
			if got := r.PublishDate.Format(time.RFC3339); got != w.date {
				t.Errorf("date got = %s, want = %s", got, w.date)
			}
			if *r.Size != w.size || r.InfoHash != w.infoHash || r.MagnetURL != w.magnet {
				t.Errorf("size, hash, magnet got = %d %q %q, want = %d %q %q", *r.Size, r.InfoHash, r.MagnetURL, w.size, w.infoHash, w.magnet)
			}
			// Prowlarr gives releases without them one seeder and peer,
			// where Sonarr's cases expect none (-1 here).
			if got := intOr(r.Peers); got != w.peers {
				t.Errorf("peers got = %d, want = %d", got, w.peers)
			}
			if got := intOr(r.Seeders); got != w.seeders {
				t.Errorf("seeders got = %d, want = %d", got, w.seeders)
			}
		})
	}

	ix := newIndexer(t, "TorrentRss/invalid/TorrentDay_NoPubDate.xml", Settings{BaseURL: feedURL})
	if _, err := ix.Search(t.Context(), &indexer.Query{}); !errors.Is(err, indexer.ErrUnsupportedFeed) {
		t.Errorf("unsupported feed error got = %v, want = ErrUnsupportedFeed", err)
	}
	if st := ix.Statuses.Get("1"); st.EscalationLevel != 1 {
		t.Errorf("unsupported feed escalation got = %d, want = 1", st.EscalationLevel)
	}
}

func intOr(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func TestDetect(t *testing.T) {
	for _, tt := range []struct {
		file      string
		allowZero bool
		want      ParserSettings
	}{
		{"TorrentRss/Ezrss.xml", false, ParserSettings{UseEZTVFormat: true}},
		{"TorrentRss/speed.cd.xml", false, ParserSettings{ParseSizeInDescription: true}},
		{"TorrentRss/ImmortalSeed.xml", false, ParserSettings{ParseSizeInDescription: true, ParseSeedersInDescription: true}},
		{"TorrentRss/ShowRSS.info.xml", true, ParserSettings{UseEnclosureURL: true}},
		{"TorrentRss/TransmitTheNet.xml", false, ParserSettings{UseEnclosureURL: true, ParseSizeInDescription: true}},
		{"TorrentRss/BitHdtv.xml", false, ParserSettings{SizeElementName: "size"}},
		{"TorrentRss/Doki.xml", true, ParserSettings{UseEnclosureURL: true}},
		{"TorrentRss/ExtraTorrents.xml", true, ParserSettings{UseEnclosureURL: true, UseEnclosureLength: true}},
		{"TorrentRss/LimeTorrents.xml", true, ParserSettings{UseEnclosureURL: true, UseEnclosureLength: true}},
		{"TorrentRss/AlphaRatio.xml", true, ParserSettings{ParseSizeInDescription: true}},
		{"TorrentRss/AnimeTosho_NoSize.xml", true, ParserSettings{UseEnclosureURL: true, ParseSizeInDescription: true}},
	} {
		got, err := Detect(response(t, tt.file), Settings{BaseURL: feedURL, AllowZeroSize: tt.allowZero})
		if err != nil || got == nil || *got != tt.want {
			t.Errorf("Detect(%s) got = %+v, %v, want = %+v", tt.file, got, err, tt.want)
		}
	}
}

func TestParserFactory(t *testing.T) {
	calls := 0
	var result *ParserSettings
	var resultErr error
	detect := func(*indexer.Response, Settings) (*ParserSettings, error) {
		calls++
		return result, resultErr
	}
	site := func(s Settings) *Site {
		st, err := New(s)
		if err != nil {
			t.Fatal(err)
		}
		st.detect = detect
		return st
	}
	r := response(t, "TorrentRss/Ezrss.xml")
	one := Settings{BaseURL: "http://my.indexer.com/"}

	result = &ParserSettings{UseEZTVFormat: true}
	s := site(one)
	ps, err := s.parserSettings(nil, r)
	if _, ok := s.Parser(ps).(*ezrssParser); err != nil || !ok {
		t.Errorf("EZTV parser got = %T, %v, want = *ezrssParser", s.Parser(ps), err)
	}
	result = &ParserSettings{UseEnclosureLength: true}
	s = site(one)
	ps, _ = s.parserSettings(nil, r)
	if p, ok := s.Parser(ps).(*indexer.RSSParser); !ok || !p.UseEnclosureLength || p.ParseSizeInDescription {
		t.Errorf("generic parser got = %+v, want = enclosure lengths", s.Parser(ps))
	}

	calls = 0
	result, resultErr = nil, nil
	s = site(one)
	if _, err := s.parserSettings(nil, r); !errors.Is(err, indexer.ErrUnsupportedFeed) {
		t.Errorf("failed detection error got = %v, want = ErrUnsupportedFeed", err)
	}
	result = &ParserSettings{}
	for range 2 {
		if _, err := s.parserSettings(nil, r); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Errorf("detections got = %d, want = 2 (failures are not kept)", calls)
	}
	calls = 0
	for _, other := range []Settings{{BaseURL: "http://my.other.indexer.com/"}, {BaseURL: one.BaseURL, AllowZeroSize: true}} {
		if _, err := site(other).parserSettings(nil, r); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Errorf("detections for other settings got = %d, want = 2", calls)
	}
}

// TestSearchFollowsRedirects checks that the feed is fetched following
// redirects, as Prowlarr's IndexerRequest of its URL is.
func TestSearchFollowsRedirects(t *testing.T) {
	site, err := New(Settings{BaseURL: feedURL})
	if err != nil {
		t.Fatal(err)
	}
	c, err := site.SearchRequests(context.Background(), nil, &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	r := c.First()
	if r == nil {
		t.Fatal("no request, want = one")
	}
	if !r.FollowRedirects {
		t.Errorf("FollowRedirects = false, want = true")
	}
}
