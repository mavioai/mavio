package series

import (
	"context"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestBlockedIndexer ports Sonarr's BlockedIndexerSpecificationFixture.
// The indexer is blocked until an hour after testNow: the root
// specification blocks an indexer only while its DisabledUntil is ahead.
func TestBlockedIndexer(t *testing.T) {
	tests := []struct {
		name    string
		blocked bool
		want    bool
	}{
		{name: "should_return_true_if_no_blocked_indexer", want: true},
		{name: "should_return_false_if_blocked_indexer", blocked: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			ix := &acquire.IndexerInfo{ID: ixID(1)}
			if tt.blocked {
				ix.DisabledUntil = testNow.Add(time.Hour)
			}
			ports.indexers = map[string]*acquire.IndexerInfo{ixID(1): ix}
			spec := Root(acquire.BlockedIndexer{})
			want(t, spec, e, testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1)), nil, tt.want)
			if !spec.Temporary() {
				t.Errorf("Temporary() = false, want = true")
			}
		})
	}
}

// TestMultiSeason ports Sonarr's MultiSeasonSpecificationFixture.
func TestMultiSeason(t *testing.T) {
	tests := []struct {
		name        string
		multiSeason bool
		want        bool
	}{
		{name: "should_return_true_if_is_not_a_multi_season_release", want: true},
		{name: "should_return_false_if_is_a_multi_season_release", multiSeason: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.Bluray720p, 1), ep(1, 1, 1), ep(2, 1, 2), ep(3, 1, 3))
			c.Parsed.FullSeason = true
			c.Parsed.IsMultiSeason = tt.multiSeason
			c.Episodes[2].AirDateUTC = testNow.AddDate(0, 0, 2)
			want(t, MultiSeason{}, e, c, nil, tt.want)
		})
	}
}

// TestMinimumAge ports Sonarr's MinimumAgeSpecificationFixture.
func TestMinimumAge(t *testing.T) {
	tests := []struct {
		name               string
		minimumAge, ageMin int
		want               bool
	}{
		{name: "should_return_true_when_minimum_age_is_set_to_zero", minimumAge: 0, ageMin: 100, want: true},
		{name: "should_return_true_when_age_is_greater_than_minimum_age", minimumAge: 30, ageMin: 100, want: true},
		{name: "should_return_false_when_age_is_less_than_minimum_age", minimumAge: 30, ageMin: 10, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.Config.MinimumAge = time.Duration(tt.minimumAge) * time.Minute
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Release.PublishDate = testNow.Add(-time.Duration(tt.ageMin) * time.Minute)
			want(t, Root(acquire.MinimumAge{}), e, c, nil, tt.want)
		})
	}
}

// TestQualityAllowedByProfile ports Sonarr's
// QualityAllowedByProfileSpecificationFixture.
func TestQualityAllowedByProfile(t *testing.T) {
	tests := []struct {
		name    string
		quality release.Quality
		want    bool
	}{
		{name: "should_allow_if_quality_is_defined_in_profile(DVD)", quality: release.DVD, want: true},
		{name: "should_allow_if_quality_is_defined_in_profile(HDTV720p)", quality: release.HDTV720p, want: true},
		{name: "should_allow_if_quality_is_defined_in_profile(Bluray1080p)", quality: release.Bluray1080p, want: true},
		{name: "should_not_allow_if_quality_is_not_defined_in_profile(SDTV)", quality: release.SDTV, want: false},
		{name: "should_not_allow_if_quality_is_not_defined_in_profile(WEBDL720p)", quality: release.WEBDL720p, want: false},
		{name: "should_not_allow_if_quality_is_not_defined_in_profile(Bluray720p)", quality: release.Bluray720p, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			p := &acquire.QualityProfile{
				Cutoff: acquire.QualityID(release.Bluray1080p),
				Items:  defaultQualities(release.DVD, release.HDTV720p, release.Bluray1080p),
			}
			c := testCandidate(testSeries(p), model(tt.quality, 2), ep(1, 1, 1))
			want(t, Root(acquire.QualityAllowedByProfile{}), e, c, nil, tt.want)
		})
	}
}

// TestMaximumSize ports Sonarr's MaximumSizeSpecificationFixture.
func TestMaximumSize(t *testing.T) {
	tests := []struct {
		name            string
		maximum, sizeMB int64
		want            bool
	}{
		{name: "should_return_true_when_maximum_size_is_set_to_zero", maximum: 0, sizeMB: 1000, want: true},
		{name: "should_return_true_when_size_is_smaller_than_maximum_size", maximum: 2000, sizeMB: 1999, want: true},
		{name: "should_return_true_when_size_is_equals_to_maximum_size", maximum: 2000, sizeMB: 2000, want: true},
		{name: "should_return_false_when_size_is_bigger_than_maximum_size", maximum: 2000, sizeMB: 2001, want: false},
		{name: "should_return_true_when_size_is_zero", maximum: 2000, sizeMB: 0, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.Config.MaximumSize = tt.maximum << 20
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Release.Size = tt.sizeMB << 20
			want(t, Root(acquire.MaximumSize{}), e, c, nil, tt.want)
		})
	}
}

// TestProtocol ports Sonarr's ProtocolSpecificationFixture.
func TestProtocol(t *testing.T) {
	tests := []struct {
		name          string
		protocol      acquire.Protocol
		usenet, torrt bool
		want          bool
	}{
		{name: "should_be_true_if_usenet_and_usenet_is_enabled", protocol: acquire.ProtocolUsenet, usenet: true, want: true},
		{name: "should_be_true_if_torrent_and_torrent_is_enabled", protocol: acquire.ProtocolTorrent, torrt: true, want: true},
		{name: "should_be_false_if_usenet_and_usenet_is_disabled", protocol: acquire.ProtocolUsenet, want: false},
		{name: "should_be_false_if_torrent_and_torrent_is_disabled", protocol: acquire.ProtocolTorrent, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.DelayProfiles = []acquire.DelayProfile{{Default: true, EnableUsenet: tt.usenet, EnableTorrent: tt.torrt}}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Release.Protocol = tt.protocol
			r := check(t, Root(acquire.ProtocolAllowed{}), e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("accepted = %v, want = %v", got, tt.want)
			}
			if r != nil && r.Message != "Usenet is not enabled for this series" && r.Message != "Torrent is not enabled for this series" {
				t.Errorf("message = %q, want = the series message", r.Message)
			}
		})
	}
}

// TestSameEpisodes ports Sonarr's SameEpisodesSpecificationFixture.
func TestSameEpisodes(t *testing.T) {
	// Builder numbers the episodes 1 and 2; the fixture puts both in
	// file 1 unless the case says otherwise.
	newEpisodes := func(sameFile bool) []*Episode {
		eps := []*Episode{ep(1, 1, 1), ep(2, 1, 2)}
		eps[0].File = file(1, model(release.HDTV720p, 1))
		eps[1].File = file(1, model(release.HDTV720p, 1))
		if !sameFile {
			eps[1].File = file(2, model(release.HDTV720p, 1))
		}
		return eps
	}
	tests := []struct {
		name     string
		sameFile bool
		inFile   map[core.ID][]core.ID
		release  func(eps []*Episode) []*Episode
		want     bool
	}{
		{
			name: "should_not_upgrade_when_new_release_contains_less_episodes", sameFile: true,
			inFile:  map[core.ID][]core.ID{tid(1): ids(1, 2)},
			release: func(eps []*Episode) []*Episode { return eps[:1] },
			want:    false,
		},
		{
			name: "should_upgrade_when_new_release_contains_more_episodes", sameFile: true,
			inFile:  map[core.ID][]core.ID{tid(1): ids(1)},
			release: func(eps []*Episode) []*Episode { return eps },
			want:    true,
		},
		{
			name: "should_upgrade_when_new_release_contains_the_same_episodes", sameFile: true,
			inFile:  map[core.ID][]core.ID{tid(1): ids(1, 2)},
			release: func(eps []*Episode) []*Episode { return eps },
			want:    true,
		},
		{
			name:    "should_upgrade_when_release_contains_the_same_episodes_as_multiple_files",
			inFile:  map[core.ID][]core.ID{tid(1): ids(1), tid(2): ids(2)},
			release: func(eps []*Episode) []*Episode { return eps },
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, eps := newTestEnv()
			eps.inFile = tt.inFile
			got, err := SameEpisodes(context.Background(), eps, tt.release(newEpisodes(tt.sameFile)))
			if err != nil {
				t.Fatalf("SameEpisodes() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("SameEpisodes() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// TestFullSeason ports Sonarr's FullSeasonSpecificationFixture. The
// episodes aired 8 days before testNow unless the case changes the
// last one.
func TestFullSeason(t *testing.T) {
	tests := []struct {
		name       string
		notFull    bool
		lastAirDay time.Time
		set        bool
		want       bool
	}{
		{name: "should_return_true_if_is_not_a_full_season", notFull: true, set: true, lastAirDay: testNow.AddDate(0, 0, 2), want: true},
		{name: "should_return_true_if_all_episodes_have_aired", want: true},
		{name: "should_return_true_if_all_episodes_will_have_aired_in_the_next_24_hours", set: true, lastAirDay: testNow.Add(23 * time.Hour), want: true},
		{name: "should_return_false_if_one_episode_has_not_aired", set: true, lastAirDay: testNow.AddDate(0, 0, 2), want: false},
		{name: "should_return_false_if_an_episode_does_not_have_an_air_date", set: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.Bluray720p, 1), ep(1, 1, 1), ep(2, 1, 2), ep(3, 1, 3))
			c.Parsed.FullSeason = !tt.notFull
			for _, e := range c.Episodes {
				e.AirDateUTC = testNow.AddDate(0, 0, -8)
			}
			if tt.set {
				c.Episodes[2].AirDateUTC = tt.lastAirDay
			}
			want(t, FullSeason{}, e, c, nil, tt.want)
		})
	}
}
