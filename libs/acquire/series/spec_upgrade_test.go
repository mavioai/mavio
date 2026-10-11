package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestUpgradeDisk ports Sonarr's UpgradeDiskSpecificationFixture. The
// release is a DVD v2 of episode 1, or of episodes 1 to 3; episodes 1
// and 2 have Bluray-1080p v2 files unless the case changes them, and
// episode 3 has none. The profile allows upgrades of the default
// qualities to Bluray-1080p unless the case gives another; no custom
// format scores unless the case gives "My Format" one.
func TestUpgradeDisk(t *testing.T) {
	myFormat := acquire.CustomFormat{ID: tid(1), Name: "My Format"}
	profile := func(cutoff release.Quality, upgrades bool) *acquire.QualityProfile {
		return &acquire.QualityProfile{UpgradeAllowed: upgrades, Cutoff: acquire.QualityID(cutoff), Items: defaultQualities()}
	}
	formatCutoff := func(cutoff release.Quality) *acquire.QualityProfile {
		p := profile(cutoff, false)
		p.CutoffFormatScore = 10000
		p.FormatItems = []acquire.FormatScore{{FormatID: myFormat.ID, Score: 50}}
		return p
	}
	sdtv := model(release.SDTV, 1)
	tests := []struct {
		name          string
		profile       *acquire.QualityProfile
		propers       acquire.ProperDownload
		multi         bool
		noFile        bool
		noEpisodes    bool
		first, second acquire.QualityModel
		newQuality    acquire.QualityModel
		newFormats    bool
		newScore      int
		want          bool
	}{
		// Sonarr keeps the file but sets its ID to 0; a nil file is
		// Mavio's episode without one.
		{name: "should_return_true_if_episode_has_no_existing_file", noFile: true, want: true},
		{name: "should_return_true_if_single_episode_doesnt_exist_on_disk", noEpisodes: true, want: true},
		{name: "should_be_upgradable_if_only_episode_is_upgradable", first: sdtv, want: true},
		{name: "should_be_upgradable_if_both_episodes_are_upgradable", multi: true, first: sdtv, second: sdtv, want: true},
		{name: "should_be_not_upgradable_if_both_episodes_are_not_upgradable", multi: true, want: false},
		{name: "should_be_not_upgradable_if_only_first_episodes_is_upgradable", multi: true, first: sdtv, want: false},
		{name: "should_be_not_upgradable_if_only_second_episodes_is_upgradable", multi: true, second: sdtv, want: false},
		{name: "should_not_be_upgradable_if_qualities_are_the_same", first: model(release.WEBDL1080p, 1), newQuality: model(release.WEBDL1080p, 1), want: false},
		{
			name:  "should_not_be_upgradable_if_revision_downgrade_and_preferred_word_upgrade_if_propers_are_preferred",
			first: model(release.WEBDL1080p, 2), newQuality: model(release.WEBDL1080p, 1), newScore: 10, want: false,
		},
		{name: "should_return_false_if_current_episode_is_equal_to_cutoff", profile: profile(release.HDTV720p, true), first: model(release.HDTV720p, 2), want: false},
		{name: "should_return_false_if_current_episode_is_greater_than_cutoff", profile: profile(release.HDTV720p, true), first: model(release.Bluray1080p, 2), want: false},
		{
			name: "should_return_true_when_new_episode_is_proper_but_existing_is_not", profile: profile(release.HDTV720p, true),
			first: model(release.HDTV720p, 1), newQuality: model(release.HDTV720p, 2), want: true,
		},
		{
			name: "should_return_false_if_cutoff_is_met_and_quality_is_higher", profile: profile(release.HDTV720p, true),
			first: model(release.HDTV720p, 2), newQuality: model(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_false_if_quality_cutoff_is_met_and_quality_is_higher_but_language_is_met", profile: profile(release.HDTV720p, true),
			first: model(release.HDTV720p, 2), newQuality: model(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_false_if_cutoff_is_met_and_quality_is_higher_and_language_is_higher", profile: profile(release.HDTV720p, true),
			first: model(release.HDTV720p, 2), newQuality: model(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_true_if_cutoff_is_not_met_and_new_quality_is_higher_and_language_is_higher", profile: profile(release.HDTV720p, true),
			first: model(release.SDTV, 2), newQuality: model(release.Bluray1080p, 2), want: true,
		},
		{name: "should_return_true_if_cutoff_is_not_met_and_language_is_higher", profile: profile(release.HDTV720p, true), first: model(release.SDTV, 2), want: true},
		// Sonarr's GivenProfile replaces the profile's format items with
		// none, so "My Format" scores 0.
		{
			name: "should_return_false_if_custom_formats_is_met_and_quality_and_format_higher", profile: profile(release.HDTV720p, true),
			first: model(release.HDTV720p, 1), newQuality: model(release.Bluray1080p, 1), newFormats: true, want: false,
		},
		{
			name: "should_return_true_if_cutoffs_are_met_but_is_a_revision_upgrade", profile: profile(release.HDTV1080p, true),
			first: model(release.WEBDL1080p, 1), newQuality: model(release.WEBDL1080p, 2), want: true,
		},
		{
			name: "should_return_false_if_quality_profile_does_not_allow_upgrades_but_cutoff_is_set_to_highest_quality", profile: profile(release.RawHD, false),
			first: model(release.WEBDL1080p, 1), newQuality: model(release.Bluray1080p, 1), want: false,
		},
		{
			name: "should_return_false_if_quality_profile_does_not_allow_upgrades_but_format_cutoff_is_above_current_score", profile: formatCutoff(release.SDTV),
			first: model(release.WEBDL1080p, 1), newQuality: model(release.WEBDL1080p, 1), newFormats: true, want: false,
		},
		{
			name:    "should_return_false_if_quality_profile_does_not_allow_upgrades_but_format_cutoff_is_above_current_score_and_is_revision_upgrade",
			profile: formatCutoff(release.SDTV), propers: acquire.PropersDoNotPrefer,
			first: model(release.WEBDL1080p, 1), newQuality: model(release.WEBDL1080p, 2), newFormats: true, want: false,
		},
	}
	or := func(q, def acquire.QualityModel) acquire.QualityModel {
		if q.Quality == 0 {
			return def
		}
		return q
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			if tt.propers != 0 {
				e.Config.Propers = tt.propers
			}
			p := tt.profile
			if p == nil {
				p = profile(release.Bluray1080p, true)
			}
			blurayV2 := model(release.Bluray1080p, 2)
			first, second, none := ep(1, 12, 3), ep(2, 12, 4), ep(3, 12, 5)
			first.File = file(1, or(tt.first, blurayV2))
			second.File = file(2, or(tt.second, blurayV2))
			for _, f := range []*acquire.File{first.File, second.File} {
				f.Languages = []release.Language{release.English}
			}
			if tt.noFile {
				first.File = nil
			}
			episodes := []*Episode{first, none}
			switch {
			case tt.noEpisodes:
				episodes = nil
			case tt.multi:
				episodes = []*Episode{first, second, none}
			}
			c := testCandidate(testSeries(p), or(tt.newQuality, model(release.DVD, 2)), episodes...)
			c.Languages = []release.Language{release.English}
			if tt.newFormats {
				c.Formats = []acquire.CustomFormat{myFormat}
			}
			c.FormatScore = tt.newScore
			want(t, UpgradeDisk{}, e, c, nil, tt.want)
		})
	}
}
