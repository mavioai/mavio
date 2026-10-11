package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// TestUpgradeAllowed ports Radarr's UpgradeAllowedSpecificationFixture.
// The fixture tests UpgradableSpecification.IsUpgradeAllowed; each case
// also runs through the UpgradeAllowed spec with the current quality as
// the existing file.
func TestUpgradeAllowed(t *testing.T) {
	const one, two = 1, 2
	tests := []struct {
		name           string
		upgradeAllowed bool
		current        QualityModel
		currentFormats []int
		newQuality     QualityModel
		newFormats     []int
		want           bool
	}{
		{
			name:    "should_return_false_when_quality_is_better_custom_formats_are_the_same_and_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), newQuality: upModel(release.Bluray1080p, 1), want: false,
		},
		{
			name:    "should_return_false_when_quality_is_same_and_custom_format_is_upgrade_and_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), currentFormats: []int{one},
			newQuality: upModel(release.DVD, 1), newFormats: []int{two}, want: false,
		},
		{
			name: "should_return_true_for_custom_format_upgrade_when_upgrading_is_allowed", upgradeAllowed: true,
			current: upModel(release.DVD, 1), currentFormats: []int{one},
			newQuality: upModel(release.DVD, 1), newFormats: []int{two}, want: true,
		},
		{
			name:    "should_return_true_for_same_custom_format_score_when_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), currentFormats: []int{one},
			newQuality: upModel(release.DVD, 1), newFormats: []int{one}, want: true,
		},
		{
			name: "should_return_true_for_lower_custom_format_score_when_upgrading_is_allowed", upgradeAllowed: true,
			current: upModel(release.DVD, 1), currentFormats: []int{two},
			newQuality: upModel(release.DVD, 1), newFormats: []int{one}, want: true,
		},
		{
			name:    "should_return_true_for_lower_language_when_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), currentFormats: []int{two},
			newQuality: upModel(release.DVD, 1), newFormats: []int{one}, want: true,
		},
		{
			name: "should_return_true_for_quality_upgrade_when_upgrading_is_allowed", upgradeAllowed: true,
			current: upModel(release.DVD, 1), newQuality: upModel(release.Bluray1080p, 1), want: true,
		},
		{
			name: "should_return_true_for_same_quality_when_upgrading_is_allowed", upgradeAllowed: true,
			current: upModel(release.DVD, 1), newQuality: upModel(release.DVD, 1), want: true,
		},
		{
			name:    "should_return_true_for_same_quality_when_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), newQuality: upModel(release.DVD, 1), want: true,
		},
		{
			name: "should_return_true_for_lower_quality_when_upgrading_is_allowed", upgradeAllowed: true,
			current: upModel(release.DVD, 1), newQuality: upModel(release.SDTV, 1), want: true,
		},
		{
			name:    "should_return_true_for_lower_quality_when_upgrading_is_not_allowed",
			current: upModel(release.DVD, 1), newQuality: upModel(release.SDTV, 1), want: true,
		},
		{
			name:    "should_returntrue_when_quality_is_revision_upgrade_for_same_quality",
			current: upModel(release.DVD, 1), currentFormats: []int{one},
			newQuality: upModel(release.DVD, 2), newFormats: []int{one}, want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := upProfile(tt.upgradeAllowed, release.Bluray1080p)
			p.CutoffFormatScore = 100
			p.FormatItems = []FormatScore{{FormatID: tid(one), Score: 50}, {FormatID: tid(two), Score: 100}}
			currentFormats, newFormats := upFormats(tt.currentFormats...), upFormats(tt.newFormats...)

			u := Upgrades{Propers: PropersPreferAndUpgrade}
			if got := u.IsUpgradeAllowed(p, tt.current, currentFormats, tt.newQuality, newFormats); got != tt.want {
				t.Errorf("IsUpgradeAllowed() got = %v, want = %v", got, tt.want)
			}

			e, _ := newTestEnv()
			c := testMovie(p, tt.newQuality.Quality)
			c.Quality = tt.newQuality
			c.Formats = newFormats
			c.Items[0].File = &File{ID: tid(1), Quality: tt.current, Formats: currentFormats}
			r := check(t, UpgradeAllowed{}, e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("UpgradeAllowed accepted got = %v (%v), want = %v", got, r, tt.want)
			}
			if r != nil && r.Reason != ReasonQualityUpgradesDisabled {
				t.Errorf("UpgradeAllowed reason got = %s, want = %s", r.Reason, ReasonQualityUpgradesDisabled)
			}
		})
	}

	t.Run("no existing file", func(t *testing.T) {
		// Radarr's spec accepts when the movie has no file; the fixture
		// does not test it, as it tests IsUpgradeAllowed only.
		e, _ := newTestEnv()
		c := testMovie(upProfile(false, release.Bluray1080p), release.Bluray1080p)
		if !accepted(t, UpgradeAllowed{}, e, c, nil) {
			t.Errorf("UpgradeAllowed accepted got = false, want = true")
		}
	})
}
