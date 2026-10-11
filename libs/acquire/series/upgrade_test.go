package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// upgradeName names an upgrade rejection in failure messages.
func upgradeName(r acquire.UpgradeRejection) string {
	return [...]string{
		"None", "BetterQuality", "BetterRevision", "QualityCutoff",
		"CustomFormatScore", "CustomFormatCutoff", "MinCustomFormatScore", "UpgradesNotAllowed",
	}[r]
}

// TestUpgradable ports Sonarr's UpgradeSpecificationFixture against
// acquire.Upgrades, with Sonarr's default qualities. The cutoff column
// of IsUpgradeTest plays no part in Sonarr's test, whose profile has
// none; neither does it here.
func TestUpgradable(t *testing.T) {
	one := acquire.CustomFormat{ID: tid(1), Name: "One"}
	two := acquire.CustomFormat{ID: tid(2), Name: "Two"}
	minScore := func(increment int) *acquire.QualityProfile {
		return &acquire.QualityProfile{
			Items: defaultQualities(), UpgradeAllowed: true, MinUpgradeFormatScore: increment, CutoffFormatScore: 100,
			FormatItems: []acquire.FormatScore{{FormatID: one.ID, Score: 10}, {FormatID: two.ID, Score: 20}},
		}
	}
	upgrades := &acquire.QualityProfile{Items: defaultQualities(), UpgradeAllowed: true}
	bare := &acquire.QualityProfile{Items: defaultQualities()}
	withCutoff := func(cutoff release.Quality) *acquire.QualityProfile {
		return &acquire.QualityProfile{Items: defaultQualities(), UpgradeAllowed: true, Cutoff: acquire.QualityID(cutoff)}
	}
	tests := []struct {
		name                   string
		propers                acquire.ProperDownload
		profile                *acquire.QualityProfile
		current, newQuality    acquire.QualityModel
		currentFmts, newFormat []acquire.CustomFormat
		want                   acquire.UpgradeRejection
	}{
		{name: "IsUpgradeTest(SDTV,1,SDTV,2,SDTV,None)", profile: upgrades, current: model(release.SDTV, 1), newQuality: model(release.SDTV, 2), want: acquire.UpgradeAccepted},
		{name: "IsUpgradeTest(WEBDL720p,1,WEBDL720p,2,WEBDL720p,None)", profile: upgrades, current: model(release.WEBDL720p, 1), newQuality: model(release.WEBDL720p, 2), want: acquire.UpgradeAccepted},
		{name: "IsUpgradeTest(SDTV,1,SDTV,1,SDTV,CustomFormatScore)", profile: upgrades, current: model(release.SDTV, 1), newQuality: model(release.SDTV, 1), want: acquire.UpgradeCustomFormatScore},
		{name: "IsUpgradeTest(WEBDL720p,1,HDTV720p,2,Bluray720p,BetterQuality)", profile: upgrades, current: model(release.WEBDL720p, 1), newQuality: model(release.HDTV720p, 2), want: acquire.UpgradeBetterQuality},
		{name: "IsUpgradeTest(WEBDL720p,1,HDTV720p,2,WEBDL720p,BetterQuality)", profile: upgrades, current: model(release.WEBDL720p, 1), newQuality: model(release.HDTV720p, 2), want: acquire.UpgradeBetterQuality},
		{name: "IsUpgradeTest(WEBDL720p,1,WEBDL720p,1,WEBDL720p,CustomFormatScore)", profile: upgrades, current: model(release.WEBDL720p, 1), newQuality: model(release.WEBDL720p, 1), want: acquire.UpgradeCustomFormatScore},
		{name: "IsUpgradeTest(WEBDL1080p,1,WEBDL1080p,1,WEBDL1080p,CustomFormatScore)", profile: upgrades, current: model(release.WEBDL1080p, 1), newQuality: model(release.WEBDL1080p, 1), want: acquire.UpgradeCustomFormatScore},
		{
			name: "should_return_true_if_proper_and_download_propers_is_do_not_download", propers: acquire.PropersDoNotUpgrade, profile: bare,
			current: model(release.DVD, 1), newQuality: model(release.DVD, 2), want: acquire.UpgradeAccepted,
		},
		{
			name: "should_return_false_if_proper_and_autoDownloadPropers_is_do_not_prefer", propers: acquire.PropersDoNotPrefer, profile: bare,
			current: model(release.DVD, 1), newQuality: model(release.DVD, 2), want: acquire.UpgradeNotAllowed,
		},
		{name: "should_return_false_if_release_and_existing_file_are_the_same", profile: bare, current: model(release.HDTV720p, 1), newQuality: model(release.HDTV720p, 1), want: acquire.UpgradeNotAllowed},
		{
			name: "should_return_true_if_release_has_higher_quality_and_cutoff_is_not_already_met", profile: withCutoff(release.HDTV1080p),
			current: model(release.HDTV720p, 1), newQuality: model(release.HDTV1080p, 1), want: acquire.UpgradeAccepted,
		},
		{
			name: "should_return_false_if_release_has_higher_quality_and_cutoff_is_already_met", profile: withCutoff(release.HDTV720p),
			current: model(release.HDTV720p, 1), newQuality: model(release.HDTV1080p, 1), want: acquire.UpgradeQualityCutoff,
		},
		{
			name: "should_return_false_if_minimum_custom_score_is_not_met", profile: minScore(11),
			current: model(release.DVD, 1), currentFmts: []acquire.CustomFormat{one}, newQuality: model(release.DVD, 1), newFormat: []acquire.CustomFormat{two},
			want: acquire.UpgradeMinCustomFormatScore,
		},
		{
			name: "should_return_true_if_minimum_custom_score_is_met", profile: minScore(10),
			current: model(release.DVD, 1), currentFmts: []acquire.CustomFormat{one}, newQuality: model(release.DVD, 1), newFormat: []acquire.CustomFormat{two},
			want: acquire.UpgradeAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := acquire.Upgrades{Propers: tt.propers}
			if got := u.IsUpgradable(tt.profile, tt.current, tt.currentFmts, tt.newQuality, tt.newFormat); got != tt.want {
				t.Errorf("IsUpgradable() = %s, want = %s", upgradeName(got), upgradeName(tt.want))
			}
		})
	}
}

// TestUpgradeAllowed ports Sonarr's UpgradeAllowedSpecificationFixture
// against acquire.Upgrades: a profile of Sonarr's default qualities with
// a Bluray-1080p cutoff, where format One scores 50 and Two 100.
func TestUpgradeAllowed(t *testing.T) {
	one := acquire.CustomFormat{ID: tid(1), Name: "One"}
	two := acquire.CustomFormat{ID: tid(2), Name: "Two"}
	fmts := func(f ...acquire.CustomFormat) []acquire.CustomFormat { return f }
	dvd, sdtv, bluray := model(release.DVD, 1), model(release.SDTV, 1), model(release.Bluray1080p, 1)
	tests := []struct {
		name                string
		upgradeAllowed      bool
		current, newQuality acquire.QualityModel
		currentFmts, newFmt []acquire.CustomFormat
		want                bool
	}{
		{name: "should_return_false_when_quality_is_better_custom_formats_are_the_same_and_upgrading_is_not_allowed", current: dvd, newQuality: bluray, want: false},
		{name: "should_return_false_when_quality_is_same_and_custom_format_is_upgrade_and_upgrading_is_not_allowed", current: dvd, currentFmts: fmts(one), newQuality: dvd, newFmt: fmts(two), want: false},
		{name: "should_return_true_for_custom_format_upgrade_when_upgrading_is_allowed", upgradeAllowed: true, current: dvd, currentFmts: fmts(one), newQuality: dvd, newFmt: fmts(two), want: true},
		{name: "should_return_true_for_same_custom_format_score_when_upgrading_is_not_allowed", current: dvd, currentFmts: fmts(one), newQuality: dvd, newFmt: fmts(one), want: true},
		{name: "should_return_true_for_lower_custom_format_score_when_upgrading_is_allowed", upgradeAllowed: true, current: dvd, currentFmts: fmts(two), newQuality: dvd, newFmt: fmts(one), want: true},
		{name: "should_return_true_for_lower_language_when_upgrading_is_not_allowed", current: dvd, currentFmts: fmts(two), newQuality: dvd, newFmt: fmts(one), want: true},
		{name: "should_return_true_for_quality_upgrade_when_upgrading_is_allowed", upgradeAllowed: true, current: dvd, newQuality: bluray, want: true},
		{name: "should_return_true_for_same_quality_when_upgrading_is_allowed", upgradeAllowed: true, current: dvd, newQuality: dvd, want: true},
		{name: "should_return_true_for_same_quality_when_upgrading_is_not_allowed", current: dvd, newQuality: dvd, want: true},
		{name: "should_return_true_for_lower_quality_when_upgrading_is_allowed", upgradeAllowed: true, current: dvd, newQuality: sdtv, want: true},
		{name: "should_return_true_for_lower_quality_when_upgrading_is_not_allowed", current: dvd, newQuality: sdtv, want: true},
		{name: "should_returntrue_when_quality_is_revision_upgrade_for_same_quality", current: dvd, currentFmts: fmts(one), newQuality: model(release.DVD, 2), newFmt: fmts(one), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &acquire.QualityProfile{
				Cutoff: acquire.QualityID(release.Bluray1080p), Items: defaultQualities(), UpgradeAllowed: tt.upgradeAllowed, CutoffFormatScore: 100,
				FormatItems: []acquire.FormatScore{{FormatID: one.ID, Score: 50}, {FormatID: two.ID, Score: 100}},
			}
			if got := (acquire.Upgrades{}).IsUpgradeAllowed(p, tt.current, tt.currentFmts, tt.newQuality, tt.newFmt); got != tt.want {
				t.Errorf("IsUpgradeAllowed() = %v, want = %v", got, tt.want)
			}
		})
	}
}
