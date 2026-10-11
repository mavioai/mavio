package acquire

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// spCandidate returns testMovie's candidate for an SDTV release under a
// profile allowing SDTV to Bluray-1080p.
func spCandidate() *Candidate {
	return testMovie(testProfile(release.Bluray1080p, release.SDTV, release.DVD, release.HDTV720p, release.Bluray1080p), release.SDTV)
}

// spWant fails the test when a spec's verdict on a candidate is not want.
func spWant(t *testing.T, spec Spec, e *Env, c *Candidate, s *Search, want bool) {
	t.Helper()
	r := check(t, spec, e, c, s)
	if got := r == nil; got != want {
		t.Errorf("%s accepted = %v (%+v), want = %v", spec.Name(), got, r, want)
	}
}

// TestSearchTargetSpec ports Radarr's Search/MovieSpecificationFixture.
func TestSearchTargetSpec(t *testing.T) {
	tests := []struct {
		name   string
		search *Search
		want   bool
	}{
		{"should_return_false_if_series_doesnt_match", &Search{TargetID: tid(2), ItemIDs: []core.ID{tid(2)}}, false},
		{"should_return_true_when_series_ids_match", &Search{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}}, true},
		// Not in the fixture: RSS syncs have no search to match.
		{"rss sync", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			spWant(t, SearchTarget{}, e, spCandidate(), tt.search, tt.want)
		})
	}
}

// TestBlockedIndexerSpec ports Radarr's BlockedIndexerSpecificationFixture.
func TestBlockedIndexerSpec(t *testing.T) {
	tests := []struct {
		name    string
		blocked bool
		want    bool
	}{
		{"should_return_true_if_no_blocked_indexer", false, true},
		{"should_return_false_if_blocked_indexer", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			ix := &IndexerInfo{ID: ixID(1), Name: "Indexer"}
			if tt.blocked {
				// Radarr's mock lists the indexer among the blocked ones with
				// DisabledTill = now; here a status is blocked while
				// DisabledUntil is after now.
				ix.DisabledUntil = testNow.Add(time.Hour)
			}
			ports.indexers = map[string]*IndexerInfo{ixID(1): ix}
			spWant(t, BlockedIndexer{}, e, spCandidate(), nil, tt.want)
		})
	}
	if !(BlockedIndexer{}).Temporary() {
		t.Error("BlockedIndexer.Temporary() = false, want = true")
	}
}

// TestMinimumAgeSpec ports Radarr's MinimumAgeSpecificationFixture.
func TestMinimumAgeSpec(t *testing.T) {
	tests := []struct {
		name            string
		minimum, ageMin int
		want            bool
	}{
		{"should_return_true_when_minimum_age_is_set_to_zero", 0, 100, true},
		{"should_return_true_when_age_is_greater_than_minimum_age", 30, 100, true},
		{"should_return_false_when_age_is_less_than_minimum_age", 30, 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.MinimumAge = time.Duration(tt.minimum) * time.Minute
			c := spCandidate()
			c.Release.PublishDate = testNow.Add(-time.Duration(tt.ageMin) * time.Minute)
			spWant(t, MinimumAge{}, e, c, nil, tt.want)
		})
	}
}

// TestMaximumSizeSpec ports Radarr's MaximumSizeSpecificationFixture.
func TestMaximumSizeSpec(t *testing.T) {
	tests := []struct {
		name          string
		maximum, size int64
		want          bool
	}{
		{"should_return_true_when_maximum_size_is_set_to_zero", 0, 1000, true},
		{"should_return_true_when_size_is_smaller_than_maximum_size", 2000, 1999, true},
		{"should_return_true_when_size_is_equals_to_maximum_size", 2000, 2000, true},
		{"should_return_false_when_size_is_bigger_than_maximum_size", 2000, 2001, false},
		{"should_return_true_when_size_is_zero", 2000, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.MaximumSize = tt.maximum << 20
			c := spCandidate()
			c.Release.Size = tt.size << 20
			spWant(t, MaximumSize{}, e, c, nil, tt.want)
		})
	}
}

// TestProtocolAllowedSpec ports Radarr's ProtocolSpecificationFixture.
func TestProtocolAllowedSpec(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		enabled  bool
		want     bool
	}{
		{"should_be_true_if_usenet_and_usenet_is_enabled", ProtocolUsenet, true, true},
		{"should_be_true_if_torrent_and_torrent_is_enabled", ProtocolTorrent, true, true},
		{"should_be_false_if_usenet_and_usenet_is_disabled", ProtocolUsenet, false, false},
		{"should_be_false_if_torrent_and_torrent_is_disabled", ProtocolTorrent, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			// Radarr's fixture starts from a delay profile with both protocols
			// disabled and enables the one under test.
			dp := DelayProfile{ID: tid(1), Default: true, Order: 1<<31 - 1}
			if tt.protocol == ProtocolUsenet {
				dp.EnableUsenet = tt.enabled
			} else {
				dp.EnableTorrent = tt.enabled
			}
			e.DelayProfiles = []DelayProfile{dp}
			c := spCandidate()
			c.Release.Protocol = tt.protocol
			spWant(t, ProtocolAllowed{}, e, c, nil, tt.want)
		})
	}
}

// TestMonitoredSpec ports Radarr's MonitoredMovieSpecificationFixture.
func TestMonitoredSpec(t *testing.T) {
	tests := []struct {
		name      string
		monitored bool
		search    *Search
		want      bool
	}{
		// Radarr checks a "single" and a "multi" remote movie of the same
		// movie; movies have no multi-item releases, so they are one case.
		{"setup_should_return_monitored_episode_should_return_true", true, nil, true},
		{"not_monitored_series_should_be_skipped", false, nil, false},
		{"only_episode_not_monitored_should_return_false", false, nil, false},
		{"should_return_true_for_single_episode_search", false, &Search{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, UserInvoked: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			c := spCandidate()
			c.Target.Monitored = tt.monitored
			spWant(t, Monitored{}, e, c, tt.search, tt.want)
		})
	}
}

// TestRetentionSpec ports Radarr's RetentionSpecificationFixture.
func TestRetentionSpec(t *testing.T) {
	tests := []struct {
		name           string
		protocol       Protocol
		retention, age int
		want           bool
	}{
		{"should_return_true_when_retention_is_set_to_zero", ProtocolUsenet, 0, 100, true},
		{"should_return_true_when_release_if_younger_than_retention", ProtocolUsenet, 1000, 100, true},
		{"should_return_true_when_release_and_retention_are_the_same", ProtocolUsenet, 100, 100, true},
		{"should_return_false_when_old_than_retention", ProtocolUsenet, 10, 100, false},
		{"should_return_true_if_release_came_out_today_and_retention_is_zero", ProtocolUsenet, 0, 100, true},
		{"should_return_true_when_release_is_not_usenet", ProtocolTorrent, 10, 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.Retention = tt.retention
			c := spCandidate()
			c.Release.Protocol = tt.protocol
			c.Release.PublishDate = testNow.AddDate(0, 0, -tt.age)
			spWant(t, Retention{}, e, c, nil, tt.want)
		})
	}
}

// TestIndexerTagSpec ports Radarr's RssSync/IndexerTagSpecificationFixture.
func TestIndexerTagSpec(t *testing.T) {
	tests := []struct {
		name                  string
		indexerID             int
		indexerTags, itemTags []string
		want                  bool
	}{
		{"indexer_and_movie_without_tags_should_return_true", 1, nil, nil, true},
		{"indexer_with_tags_movie_without_tags_should_return_false", 1, []string{"123"}, nil, false},
		{"indexer_without_tags_movie_with_tags_should_return_true", 1, nil, []string{"123"}, true},
		{"indexer_with_tags_movie_with_matching_tags_should_return_true", 1, []string{"123", "456"}, []string{"123", "789"}, true},
		{"indexer_with_tags_movie_with_different_tags_should_return_false", 1, []string{"456"}, []string{"123", "789"}, false},
		{"release_without_indexerid_should_return_true", 0, []string{"456"}, []string{"123", "789"}, true},
		{"release_with_invalid_indexerid_should_return_true", 2, []string{"456"}, []string{"123", "789"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			ports.indexers = map[string]*IndexerInfo{ixID(1): {ID: ixID(1), Tags: tt.indexerTags}}
			c := spCandidate()
			c.Release.IndexerID = ixID(tt.indexerID)
			c.Target.Tags = tt.itemTags
			spWant(t, IndexerTag{}, e, c, &Search{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}}, tt.want)
		})
	}
}

// TestTorrentSeedingSpec ports Radarr's
// Search/TorrentSeedingSpecificationFixture.
func TestTorrentSeedingSpec(t *testing.T) {
	tests := []struct {
		name      string
		protocol  Protocol
		indexerID int
		noIndexer bool
		seeders   *int
		want      bool
	}{
		{"should_return_true_if_not_torrent", ProtocolUsenet, 1, false, nil, true},
		{"should_return_true_if_indexer_not_specified", ProtocolTorrent, 0, false, intPtr(0), true},
		{"should_return_true_if_indexer_no_longer_exists", ProtocolTorrent, 1, true, intPtr(0), true},
		{"should_return_true_if_seeds_unknown", ProtocolTorrent, 1, false, nil, true},
		{"should_return_true_if_seeds_above_or_equal_to_limit(5)", ProtocolTorrent, 1, false, intPtr(5), true},
		{"should_return_true_if_seeds_above_or_equal_to_limit(6)", ProtocolTorrent, 1, false, intPtr(6), true},
		{"should_return_false_if_seeds_belove_limit(0)", ProtocolTorrent, 1, false, intPtr(0), false},
		{"should_return_false_if_seeds_belove_limit(4)", ProtocolTorrent, 1, false, intPtr(4), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			if !tt.noIndexer {
				ports.indexers = map[string]*IndexerInfo{ixID(1): {ID: ixID(1), MinimumSeeders: 5}}
			}
			c := spCandidate()
			c.Release.Title = "Series.Title.S01.720p.BluRay.X264-RlsGrp"
			c.Release.Protocol = tt.protocol
			c.Release.IndexerID = ixID(tt.indexerID)
			c.Release.Seeders = tt.seeders
			spWant(t, TorrentSeeding{}, e, c, nil, tt.want)
		})
	}
}
