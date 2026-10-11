package series

import (
	"errors"
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestRetention ports Sonarr's RetentionSpecificationFixture.
func TestRetention(t *testing.T) {
	tests := []struct {
		name            string
		torrent         bool
		retention, days int
		want            bool
	}{
		{name: "should_return_true_when_retention_is_set_to_zero", retention: 0, days: 100, want: true},
		{name: "should_return_true_when_release_if_younger_than_retention", retention: 1000, days: 100, want: true},
		{name: "should_return_true_when_release_and_retention_are_the_same", retention: 100, days: 100, want: true},
		{name: "should_return_false_when_old_than_retention", retention: 10, days: 100, want: false},
		{name: "should_return_true_if_release_came_out_today_and_retention_is_zero", retention: 0, days: 100, want: true},
		{name: "should_return_true_when_release_is_not_usenet", torrent: true, retention: 10, days: 100, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.Config.Retention = tt.retention
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Release.PublishDate = testNow.AddDate(0, 0, -tt.days)
			if tt.torrent {
				c.Release.Protocol = acquire.ProtocolTorrent
			}
			want(t, Root(acquire.Retention{}), e, c, nil, tt.want)
		})
	}
}

// TestRawDisk ports Sonarr's RawDiskSpecificationFixture.
func TestRawDisk(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		container string
		want      bool
	}{
		{name: "should_return_true_if_no_container_specified_and_does_not_match_disc_release_pattern", want: true},
		{name: "should_return_true_if_mkv", container: "MKV", want: true},
		{name: "should_return_false_if_vob", container: "VOB", want: false},
		{name: "should_return_false_if_iso", container: "ISO", want: false},
		{name: "should_return_false_if_m2ts", container: "M2TS", want: false},
		{name: "should_compare_case_insensitive", container: "vob", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series Title S02 Disc 1 1080i Blu-ray DTS-HD MA 2.0 AVC-TrollHD", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series Title S03 Disc 1 1080p Blu-ray LPCM 2.0 AVC-TrollHD", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "SERIES TITLE S02 1080P FULL BLURAY AVC DTS-HD MA 5 1", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series.Title.S06.2016.DISC.3.BluRay.1080p.AVC.Atmos.TrueHD7.1-MTeam", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series Title S05 Disc 1 BluRay 1080p AVC Atmos TrueHD 7 1-MTeam", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series Title S05 Disc 1 BluRay 1080p AVC Atmos TrueHD 7 1-MTeam", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Someone.the.Entertainer.Presents.S01.NTSC.3xDVD9.MPEG-2.DD2.0", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series.Title.S00.The.Christmas.Special.2011.PAL.DVD5.DD2.0", want: false},
		{name: "should_return_false_if_matches_disc_format", title: "Series.of.Desire.2000.S1_D01.NTSC.DVD5", want: false},
		{name: "should_return_true_if_dvdrip", title: "Series Title EP50 USLT NTSC DVDRemux DD2.0", want: true},
		{name: "should_return_true_if_dvdrip", title: "Series.Title.S01.NTSC.DVDRip.DD2.0.x264-PLAiD", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Release.Title = "Series.title.s01e01"
			if tt.title != "" {
				c.Release.Title = tt.title
			}
			c.Release.Protocol = acquire.ProtocolTorrent
			c.Release.Container = tt.container
			want(t, Root(acquire.RawDisk{}), e, c, nil, tt.want)
		})
	}
}

// TestFreeSpace ports Sonarr's FreeSpaceSpecificationFixture. A root
// folder that is not available is free space that is unknown.
func TestFreeSpace(t *testing.T) {
	tests := []struct {
		name                string
		skip                bool
		minimum, free, size int64
		unknown             bool
		want                bool
	}{
		{name: "should_return_true_when_available_space_is_more_than_size", minimum: 0, free: 200, size: 100, want: true},
		{name: "should_return_true_when_available_space_minus_size_is_more_than_minimum_free_space", minimum: 50, free: 200, size: 100, want: true},
		{name: "should_return_false_available_space_is_less_than_size", minimum: 0, free: 200, size: 1000, want: false},
		{name: "should_return_false_when_available_space_minus_size_is_less_than_minimum_free_space", minimum: 150, free: 200, size: 100, want: false},
		{name: "should_return_true_if_skip_free_space_check_is_true", skip: true, minimum: 150, free: 200, size: 100, want: true},
		{name: "should_return_true_if_root_folder_is_not_available", minimum: 150, size: 100, unknown: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			e.Config.SkipFreeSpaceCheck = tt.skip
			e.Config.MinimumFreeSpace = tt.minimum << 20
			if !tt.unknown {
				free := tt.free << 20
				ports.free = &free
			}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			c.Series.Path = "/Test/TV/Series"
			c.Release.Size = tt.size << 20
			want(t, Root(acquire.FreeSpace{}), e, c, nil, tt.want)
		})
	}
}

// TestTorrentSeeding ports Sonarr's Search/TorrentSeedingSpecificationFixture.
// An indexer that no longer exists is one the port does not return.
func TestTorrentSeeding(t *testing.T) {
	tests := []struct {
		name       string
		notTorrent bool
		noIndexer  bool
		gone       bool
		seeders    *int
		want       bool
	}{
		{name: "should_return_true_if_not_torrent", notTorrent: true, seeders: intPtr(0), want: true},
		{name: "should_return_true_if_indexer_not_specified", noIndexer: true, seeders: intPtr(0), want: true},
		{name: "should_return_true_if_indexer_no_longer_exists", gone: true, seeders: intPtr(0), want: true},
		{name: "should_return_true_if_seeds_unknown", want: true},
		{name: "should_return_true_if_seeds_above_or_equal_to_limit(5)", seeders: intPtr(5), want: true},
		{name: "should_return_true_if_seeds_above_or_equal_to_limit(6)", seeders: intPtr(6), want: true},
		{name: "should_return_false_if_seeds_belove_limit(0)", seeders: intPtr(0), want: false},
		{name: "should_return_false_if_seeds_belove_limit(4)", seeders: intPtr(4), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			if !tt.gone {
				ports.indexers = map[string]*acquire.IndexerInfo{ixID(1): {ID: ixID(1), MinimumSeeders: 5}}
			}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.Bluray720p, 1), ep(1, 1, 1))
			c.Release.Title = "Series.Title.S01.720p.BluRay.X264-RlsGrp"
			c.Release.Protocol = acquire.ProtocolTorrent
			c.Release.Seeders = tt.seeders
			if tt.notTorrent {
				c.Release.Protocol = acquire.ProtocolUsenet
				c.Release.Seeders = nil
			}
			if tt.noIndexer {
				c.Release.IndexerID = ""
			}
			want(t, Root(acquire.TorrentSeeding{}), e, c, nil, tt.want)
		})
	}
}

// TestCustomFormatAllowedByProfile ports Sonarr's
// CustomFormatAllowedByProfileSpecificationFixture.
func TestCustomFormatAllowedByProfile(t *testing.T) {
	format1 := acquire.CustomFormat{ID: tid(1), Name: "Awesome Format"}
	format2 := acquire.CustomFormat{ID: tid(2), Name: "Cool Format"}
	formats := []acquire.CustomFormat{format1, format2}
	tests := []struct {
		name     string
		matched  []acquire.CustomFormat
		allowed  []string
		minScore int
		want     bool
	}{
		{name: "should_allow_if_format_score_greater_than_min", matched: []acquire.CustomFormat{format1}, allowed: []string{format1.Name}, minScore: 1, want: true},
		{name: "should_deny_if_format_score_not_greater_than_min", matched: []acquire.CustomFormat{format2}, allowed: []string{format1.Name}, minScore: 1, want: false},
		{name: "should_deny_if_format_score_not_greater_than_min_2", matched: []acquire.CustomFormat{format2, format1}, allowed: []string{format1.Name}, minScore: 1, want: false},
		{name: "should_allow_if_all_format_is_defined_in_profile", matched: []acquire.CustomFormat{format2, format1}, allowed: []string{format1.Name, format2.Name}, minScore: 1, want: true},
		{name: "should_deny_if_no_format_was_parsed_and_min_score_positive", allowed: []string{format1.Name, format2.Name}, minScore: 1, want: false},
		{name: "should_allow_if_no_format_was_parsed_min_score_is_zero", allowed: []string{format1.Name, format2.Name}, minScore: 0, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			p := &acquire.QualityProfile{
				Cutoff: acquire.QualityID(release.Bluray1080p), MinFormatScore: tt.minScore,
				FormatItems: sampleFormatItems(formats, tt.allowed...),
			}
			c := testCandidate(testSeries(p), model(release.DVD, 2), ep(1, 1, 1))
			c.Formats = tt.matched
			c.FormatScore = p.FormatScore(tt.matched)
			r := check(t, CustomFormatAllowedByProfile{}, e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("accepted = %v (%+v), want = %v", got, r, tt.want)
			}
		})
	}
}

// TestAnimeVersionUpgrade ports Sonarr's
// AnimeVersionUpgradeSpecificationFixture. The release is version 2 of
// an HDTV-720p anime episode by DRONE, whose file is version 1 by
// DRONE2. A blank release group is given as " ", which Sonarr
// treats as it does an empty one.
func TestAnimeVersionUpgrade(t *testing.T) {
	tests := []struct {
		name         string
		noFile       bool
		standard     bool
		noUpgrade    bool
		fileGroup    string
		releaseGroup string
		propers      acquire.ProperDownload
		want         bool
	}{
		{name: "should_be_true_when_no_existing_file", noFile: true, want: true},
		{name: "should_be_true_if_series_is_not_anime", standard: true, want: true},
		{name: "should_be_true_if_is_not_a_version_upgrade_for_existing_file", noUpgrade: true, want: true},
		{name: "should_be_true_when_release_group_matches", fileGroup: "DRONE", want: true},
		{name: "should_be_false_when_existing_file_doesnt_have_a_release_group", fileGroup: " ", want: false},
		{name: "should_should_be_false_when_release_doesnt_have_a_release_group", releaseGroup: " ", want: false},
		{name: "should_be_false_when_release_group_does_not_match", want: false},
		{name: "should_return_true_when_repacks_are_not_preferred", propers: acquire.PropersDoNotPrefer, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			if tt.propers != 0 {
				e.Config.Propers = tt.propers
			}
			s := testSeries(testProfile(release.HDTV720p, release.HDTV720p))
			s.Type = Anime
			if tt.standard {
				s.Type = Standard
			}
			q := model(release.HDTV720p, 2)
			if tt.noUpgrade {
				q = model(release.HDTV720p, 1)
			}
			episode := ep(1, 1, 1)
			if !tt.noFile {
				f := file(1, model(release.HDTV720p, 1))
				f.ReleaseGroup = "DRONE2"
				if tt.fileGroup != "" {
					f.ReleaseGroup = tt.fileGroup
				}
				episode.File = f
			}
			c := testCandidate(s, q, episode)
			c.Parsed.ReleaseGroup = "DRONE"
			if tt.releaseGroup != "" {
				c.Parsed.ReleaseGroup = tt.releaseGroup
			}
			want(t, AnimeVersionUpgrade{}, e, c, nil, tt.want)
		})
	}
}

// TestRootSpecError checks that a root specification's error rejects
// the candidate with acquire.ReasonDecisionError.
func TestRootSpecError(t *testing.T) {
	e, ports, _ := newTestEnv()
	ports.err = errors.New("boom")
	c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
	d := (&Engine{Env: e, Specs: []Spec{Root(acquire.Blocklist{})}}).Evaluate(t.Context(), c, nil)
	if len(d.Rejections) != 1 || d.Rejections[0].Reason != acquire.ReasonDecisionError || d.Rejections[0].Message != "Blocklist: boom" {
		t.Errorf("Rejections = %+v, want = one Blocklist decision error", d.Rejections)
	}
}
