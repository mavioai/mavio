package series

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestPrioritize ports Sonarr's PrioritizeDownloadDecisionFixture. Each
// release is for series 1 of Sonarr's default qualities and a runtime of
// 60 minutes; it is a usenet release of indexer priority 25 published
// now unless the case says otherwise, and its episodes are numbered as
// their IDs. Languages play no part in Sonarr's comparer and are left
// out. want lists the positions, in the input, of the leading results.
func TestPrioritize(t *testing.T) {
	type rel struct {
		episodes       []int
		quality        acquire.QualityModel
		ageDays        int
		size           int64
		torrent        bool
		priority       int
		seeders, peers *int
		fullSeason     bool
		anime          bool
		score          int
	}
	hd := func(r rel) rel {
		r.quality = model(release.HDTV720p, 1)
		if r.episodes == nil {
			r.episodes = []int{1}
		}
		return r
	}
	q := func(quality release.Quality, version, real int) rel {
		return rel{episodes: []int{1}, quality: acquire.QualityModel{Quality: quality, Revision: release.Revision{Version: version, Real: real}}}
	}
	scored := func(r rel, score int) rel { r.score = score; return r }
	prio := func(r rel, p int) rel { r.priority = p; return r }
	torrent := func(r rel, seeders, peers int, sizeMB float64) rel {
		r.torrent, r.seeders, r.peers, r.size = true, intPtr(seeders), intPtr(peers), megabytes(sizeMB)
		return r
	}
	mb := megabytes
	tests := []struct {
		name      string
		preferred float64 // the preferred size of every quality, 0 for none
		torrents  bool    // torrents are preferred
		propers   acquire.ProperDownload
		releases  []rel
		want      []int
	}{
		{name: "should_put_reals_before_non_reals", releases: []rel{q(release.HDTV720p, 1, 0), q(release.HDTV720p, 1, 1)}, want: []int{1}},
		{name: "should_put_propers_before_non_propers", releases: []rel{q(release.HDTV720p, 1, 0), q(release.HDTV720p, 2, 0)}, want: []int{1}},
		{name: "should_put_higher_quality_before_lower", releases: []rel{q(release.SDTV, 1, 0), q(release.HDTV720p, 1, 0)}, want: []int{1}},
		{name: "should_order_by_lowest_number_of_episodes", releases: []rel{hd(rel{episodes: []int{2}}), hd(rel{episodes: []int{1}})}, want: []int{1}},
		{name: "should_order_by_lowest_number_of_episodes_with_multiple_episodes", releases: []rel{hd(rel{episodes: []int{2, 3}}), hd(rel{episodes: []int{1, 2}})}, want: []int{1}},
		{
			name: "should_order_by_age_then_largest_rounded_to_200mb",
			releases: []rel{
				{episodes: []int{1}, quality: model(release.SDTV, 1), size: mb(100), ageDays: 1},
				hd(rel{size: mb(1200), ageDays: 1000}),
				hd(rel{size: mb(1250), ageDays: 10}),
				hd(rel{size: mb(3000), ageDays: 1}),
			},
			want: []int{3},
		},
		{name: "should_order_by_closest_to_preferred_size_if_both_under", preferred: 200, releases: []rel{hd(rel{size: mb(1200), ageDays: 1}), hd(rel{size: mb(10000), ageDays: 1})}, want: []int{1}},
		{
			name: "should_order_by_closest_to_preferred_size_if_preferred_is_in_between", preferred: 46,
			releases: []rel{hd(rel{size: mb(500), ageDays: 1}), hd(rel{size: mb(2000), ageDays: 1}), hd(rel{size: mb(3000), ageDays: 1}), hd(rel{size: mb(5000), ageDays: 1})},
			want:     []int{2},
		},
		{name: "should_order_by_youngest", releases: []rel{hd(rel{ageDays: 10}), hd(rel{ageDays: 5})}, want: []int{1}},
		{name: "should_not_throw_if_no_episodes_are_found", releases: []rel{hd(rel{episodes: []int{}, size: mb(500)}), hd(rel{size: mb(500)})}},
		{name: "should_put_usenet_above_torrent_when_usenet_is_preferred", releases: []rel{hd(rel{torrent: true}), hd(rel{})}, want: []int{1}},
		{name: "should_put_torrent_above_usenet_when_torrent_is_preferred", torrents: true, releases: []rel{hd(rel{torrent: true}), hd(rel{})}, want: []int{0}},
		{name: "should_prefer_season_pack_above_single_episode", releases: []rel{hd(rel{episodes: []int{1, 2}, fullSeason: true}), hd(rel{})}, want: []int{0}},
		{name: "should_prefer_multiepisode_over_single_episode_for_anime", releases: []rel{hd(rel{episodes: []int{1, 2}, anime: true}), hd(rel{anime: true})}, want: []int{0}},
		{name: "should_prefer_single_episode_over_multi_episode_for_non_anime", releases: []rel{hd(rel{episodes: []int{1, 2}}), hd(rel{})}, want: []int{1}},
		{name: "should_prefer_releases_with_more_seeders", releases: []rel{torrent(hd(rel{}), 10, 0, 0), torrent(hd(rel{}), 100, 0, 0)}, want: []int{1}},
		{name: "should_prefer_releases_with_more_peers_given_equal_number_of_seeds", releases: []rel{torrent(hd(rel{}), 10, 10, 0), torrent(hd(rel{}), 10, 100, 0)}, want: []int{1}},
		{name: "should_prefer_releases_with_more_peers_no_seeds", releases: []rel{torrent(hd(rel{}), 0, 10, 0), torrent(hd(rel{}), 0, 100, 0)}, want: []int{1}},
		// Sonarr's case clones the first torrent before setting its size
		// to 250 MB, so the second keeps 200 MB.
		{name: "should_prefer_first_release_if_peers_and_size_are_too_similar", releases: []rel{torrent(hd(rel{}), 1000, 10, 250), torrent(hd(rel{}), 1100, 10, 200)}, want: []int{0}},
		{name: "should_prefer_first_release_if_age_and_size_are_too_similar", releases: []rel{hd(rel{ageDays: 100, size: mb(200)}), hd(rel{ageDays: 150, size: mb(250)})}, want: []int{0}},
		{
			name: "should_prefer_quality_over_the_number_of_peers",
			releases: []rel{
				torrent(rel{episodes: []int{1}, quality: model(release.Bluray1080p, 1)}, 100, 10, 250),
				torrent(rel{episodes: []int{1}, quality: model(release.SDTV, 1)}, 1100, 10, 200),
			},
			want: []int{0},
		},
		{name: "should_put_higher_quality_before_lower_always", releases: []rel{q(release.SDTV, 1, 0), q(release.HDTV720p, 1, 0)}, want: []int{1}},
		{name: "should_prefer_higher_score_over_lower_score", releases: []rel{scored(q(release.WEBDL1080p, 1, 0), 10), q(release.WEBDL1080p, 1, 0)}, want: []int{0}},
		{
			name: "should_prefer_proper_over_score_when_download_propers_is_prefer_and_upgrade", propers: acquire.PropersPreferAndUpgrade,
			releases: []rel{scored(q(release.WEBDL1080p, 1, 0), 10), q(release.WEBDL1080p, 2, 0)}, want: []int{1},
		},
		{
			name: "should_prefer_proper_over_score_when_download_propers_is_do_not_upgrade", propers: acquire.PropersDoNotUpgrade,
			releases: []rel{scored(q(release.WEBDL1080p, 1, 0), 10), q(release.WEBDL1080p, 2, 0)}, want: []int{1},
		},
		{
			name: "should_prefer_score_over_proper_when_download_propers_is_do_not_prefer", propers: acquire.PropersDoNotPrefer,
			releases: []rel{scored(q(release.WEBDL1080p, 1, 0), 10), q(release.WEBDL1080p, 2, 0)}, want: []int{0},
		},
		{
			name: "should_prefer_score_over_real_when_download_propers_is_do_not_prefer", propers: acquire.PropersDoNotPrefer,
			releases: []rel{scored(q(release.WEBDL1080p, 1, 0), 10), q(release.WEBDL1080p, 1, 1)}, want: []int{0},
		},
		{
			name:     "sort_download_decisions_based_on_indexer_priority",
			releases: []rel{prio(q(release.WEBDL1080p, 1, 0), 25), prio(q(release.WEBDL1080p, 1, 0), 50), prio(q(release.WEBDL1080p, 1, 0), 1)},
			want:     []int{2, 0, 1},
		},
		{
			name: "ensure_download_decisions_indexer_priority_is_not_perfered_over_quality",
			releases: []rel{
				prio(q(release.HDTV720p, 1, 0), 25), prio(q(release.WEBDL1080p, 1, 0), 50),
				prio(q(release.SDTV, 1, 0), 1), prio(q(release.WEBDL1080p, 1, 0), 25),
			},
			want: []int{3, 1, 0, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.Config.Propers = tt.propers
			e.Definitions = nil
			if tt.preferred != 0 {
				for _, quality := range sonarrQualities {
					e.Definitions = append(e.Definitions, acquire.Definition{Quality: quality, PreferredSize: tt.preferred})
				}
			}
			if tt.torrents {
				e.DelayProfiles[0].PreferredProtocol = acquire.ProtocolTorrent
			}
			var decisions []Decision
			for i, r := range tt.releases {
				s := testSeries(&acquire.QualityProfile{Items: defaultQualities()})
				s.Runtime = 60
				if r.anime {
					s.Type = Anime
				}
				var episodes []*Episode
				for _, n := range r.episodes {
					episodes = append(episodes, ep(n, 1, n))
				}
				c := testCandidate(s, r.quality, episodes...)
				c.Release.GUID = strconv.Itoa(i)
				c.Release.PublishDate = testNow.Add(-time.Duration(r.ageDays) * 24 * time.Hour)
				c.Release.Size = r.size
				c.Release.IndexerPriority = 25
				if r.priority != 0 {
					c.Release.IndexerPriority = r.priority
				}
				if r.torrent {
					c.Release.Protocol = acquire.ProtocolTorrent
				}
				c.Release.Seeders, c.Release.Peers = r.seeders, r.peers
				c.Parsed.FullSeason = r.fullSeason
				c.FormatScore = r.score
				decisions = append(decisions, Decision{Candidate: c})
			}
			got := e.Prioritize(decisions)
			if len(got) != len(decisions) {
				t.Fatalf("len(Prioritize()) = %d, want = %d", len(got), len(decisions))
			}
			var order []int
			for _, d := range got[:len(tt.want)] {
				n, _ := strconv.Atoi(d.Candidate.Release.GUID)
				order = append(order, n)
			}
			if !slices.Equal(order, tt.want) {
				t.Errorf("Prioritize() leads with %v, want = %v", order, tt.want)
			}
		})
	}
}
