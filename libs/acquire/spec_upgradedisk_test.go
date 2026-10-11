package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// TestUpgradeDisk ports Radarr's UpgradeDiskSpecificationFixture.
func TestUpgradeDisk(t *testing.T) {
	// The custom format of the format tests scores 50 where the profile
	// scores it; GivenProfile drops the profile's sample format items,
	// so it scores nothing elsewhere. The existing file matches no
	// format in any test.
	const format = 1
	tests := []struct {
		name    string
		propers ProperDownload
		// profile is nil for the fixture's: upgrades allowed to
		// Bluray-1080p.
		profile    *QualityProfile
		noFile     bool
		file       QualityModel
		newQuality QualityModel
		newFormats []CustomFormat
		want       bool
	}{
		{
			name: "should_return_true_if_movie_has_no_existing_file", noFile: true,
			newQuality: upModel(release.DVD, 2), want: true,
		},
		{
			name: "should_be_upgradable_if_only_movie_is_upgradable",
			file: upModel(release.SDTV, 1), newQuality: upModel(release.DVD, 2), want: true,
		},
		{
			name: "should_not_be_upgradable_if_qualities_are_the_same",
			file: upModel(release.WEBDL1080p, 1), newQuality: upModel(release.WEBDL1080p, 1), want: false,
		},
		{
			name: "should_not_be_upgradable_if_revision_downgrade_if_propers_are_preferred",
			file: upModel(release.WEBDL1080p, 2), newQuality: upModel(release.WEBDL1080p, 1), want: false,
		},
		{
			name: "should_return_false_if_current_movie_is_equal_to_cutoff", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 2), newQuality: upModel(release.DVD, 2), want: false,
		},
		{
			name: "should_return_false_if_current_movie_is_greater_than_cutoff", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.Bluray1080p, 2), newQuality: upModel(release.DVD, 2), want: false,
		},
		{
			name: "should_return_true_when_new_movie_is_proper_but_existing_is_not", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 1), newQuality: upModel(release.HDTV720p, 2), want: true,
		},
		{
			name: "should_return_false_if_cutoff_is_met_and_quality_is_higher", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 2), newQuality: upModel(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_false_if_quality_cutoff_is_met_and_quality_is_higher_but_language_is_met", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 2), newQuality: upModel(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_false_if_cutoff_is_met_and_quality_is_higher_and_language_is_higher", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 2), newQuality: upModel(release.Bluray1080p, 2), want: false,
		},
		{
			name: "should_return_true_if_cutoff_is_not_met_and_new_quality_is_higher_and_language_is_higher", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.SDTV, 2), newQuality: upModel(release.Bluray1080p, 2), want: true,
		},
		{
			name: "should_return_true_if_cutoff_is_not_met_and_language_is_higher", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.SDTV, 2), newQuality: upModel(release.DVD, 2), want: true,
		},
		{
			name: "should_return_false_if_custom_formats_is_met_and_quality_and_format_higher", profile: upProfile(true, release.HDTV720p),
			file: upModel(release.HDTV720p, 1), newQuality: upModel(release.Bluray1080p, 1), newFormats: upFormats(format), want: false,
		},
		{
			name: "should_return_true_if_cutoffs_are_met_but_is_a_revision_upgrade", profile: upProfile(true, release.HDTV1080p),
			file: upModel(release.WEBDL1080p, 1), newQuality: upModel(release.WEBDL1080p, 2), want: true,
		},
		{
			name: "should_return_false_if_quality_profile_does_not_allow_upgrades_but_cutoff_is_set_to_highest_quality", profile: upProfile(false, release.RawHD),
			file: upModel(release.WEBDL1080p, 1), newQuality: upModel(release.Bluray1080p, 1), want: false,
		},
		{
			name:    "should_return_false_if_quality_profile_does_not_allow_upgrades_but_format_cutoff_is_above_current_score",
			profile: upFormatCutoffProfile(format),
			file:    upModel(release.WEBDL1080p, 1), newQuality: upModel(release.WEBDL1080p, 1), newFormats: upFormats(format), want: false,
		},
		{
			name:    "should_return_false_if_quality_profile_does_not_allow_upgrades_but_format_cutoff_is_above_current_score_and_is_revision_upgrade",
			propers: PropersDoNotPrefer, profile: upFormatCutoffProfile(format),
			file: upModel(release.WEBDL1080p, 1), newQuality: upModel(release.WEBDL1080p, 2), newFormats: upFormats(format), want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.Propers = tt.propers
			p := tt.profile
			if p == nil {
				p = upProfile(true, release.Bluray1080p)
			}
			c := testMovie(p, release.DVD)
			c.Quality = tt.newQuality
			c.Formats = tt.newFormats
			if !tt.noFile {
				c.Items[0].File = &File{ID: tid(1), Quality: tt.file, DateAdded: testNow}
			}
			r := check(t, UpgradeDisk{}, e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("UpgradeDisk accepted got = %v (%v), want = %v", got, r, tt.want)
			}
		})
	}
}

// upFormatCutoffProfile returns the profile of Radarr's format cutoff
// tests: no upgrades, a cutoff of SDTV, a custom format cutoff of 10000
// and the format scored 50.
func upFormatCutoffProfile(format int) *QualityProfile {
	p := upProfile(false, release.SDTV)
	p.CutoffFormatScore = 10000
	p.FormatItems = []FormatScore{{FormatID: tid(format), Score: 50}}
	return p
}
