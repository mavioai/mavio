package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// upDefaultQualities returns the qualities of Radarr's
// QualityFixture.GetDefaultQualities, lowest first.
func upDefaultQualities() []release.Quality {
	return []release.Quality{
		release.Unknown, release.Cam, release.Telecine, release.DVDSCR, release.Regional, release.SDTV,
		release.DVD, release.DVDR, release.HDTV720p, release.HDTV1080p, release.HDTV2160p, release.WEBDL480p,
		release.WEBDL720p, release.WEBDL1080p, release.WEBDL2160p, release.Bluray480p, release.Bluray576p,
		release.Bluray720p, release.Bluray1080p, release.Bluray2160p, release.BRDisk, release.RawHD,
	}
}

// upProfile returns a profile of upDefaultQualities, all allowed, as
// Radarr's fixtures build one. A cutoff of release.Unknown is Radarr's
// unset cutoff, ID 0.
func upProfile(upgradeAllowed bool, cutoff release.Quality) *QualityProfile {
	p := testProfile(cutoff, upDefaultQualities()...)
	p.UpgradeAllowed = upgradeAllowed
	return p
}

// upModel returns a quality at a revision version.
func upModel(q release.Quality, version int) QualityModel {
	return QualityModel{Quality: q, Revision: release.Revision{Version: version}}
}

// upFormats returns custom formats with the IDs.
func upFormats(ids ...int) []CustomFormat {
	formats := make([]CustomFormat, 0, len(ids))
	for _, id := range ids {
		formats = append(formats, CustomFormat{ID: tid(id), Name: "Format"})
	}
	return formats
}

// upReasonName names an upgrade rejection in failure messages.
func upReasonName(r UpgradeRejection) string {
	return [...]string{
		"Accepted", "BetterQuality", "BetterRevision", "QualityCutoff",
		"CustomFormatScore", "CustomFormatCutoff", "MinCustomFormatScore", "NotAllowed",
	}[r]
}

// TestUpgradesIsUpgradable ports the IsUpgradeTest cases of Radarr's
// UpgradeSpecificationFixture.
func TestUpgradesIsUpgradable(t *testing.T) {
	// Format 1 scores 1 and format 2 scores 2, as
	// GetSampleFormatItems("My Format 1", "My Format 2") gives them.
	const cf1, cf2 = 1, 2
	tests := []struct {
		name           string
		current        release.Quality
		currentVersion int
		currentFormats []int
		newQuality     release.Quality
		newVersion     int
		newFormats     []int
		want           UpgradeRejection
	}{
		// Quality upgrade trumps custom format.
		{"sdtv revision no formats", release.SDTV, 1, nil, release.SDTV, 2, nil, UpgradeAccepted},
		{"sdtv revision same format", release.SDTV, 1, []int{cf1}, release.SDTV, 2, []int{cf1}, UpgradeAccepted},
		{"sdtv revision lower format", release.SDTV, 1, []int{cf1}, release.SDTV, 2, []int{cf2}, UpgradeAccepted},
		{"sdtv revision higher format", release.SDTV, 1, []int{cf2}, release.SDTV, 2, []int{cf1}, UpgradeAccepted},
		// Revision upgrade trumps custom format.
		{"webdl revision no formats", release.WEBDL720p, 1, nil, release.WEBDL720p, 2, nil, UpgradeAccepted},
		{"webdl revision same format", release.WEBDL720p, 1, []int{cf1}, release.WEBDL720p, 2, []int{cf1}, UpgradeAccepted},
		{"webdl revision lower format", release.WEBDL720p, 1, []int{cf1}, release.WEBDL720p, 2, []int{cf2}, UpgradeAccepted},
		{"webdl revision higher format", release.WEBDL720p, 1, []int{cf2}, release.WEBDL720p, 2, []int{cf1}, UpgradeAccepted},
		// Custom formats apply if quality same.
		{"same no formats", release.SDTV, 1, nil, release.SDTV, 1, nil, UpgradeCustomFormatScore},
		{"same format", release.SDTV, 1, []int{cf1}, release.SDTV, 1, []int{cf1}, UpgradeCustomFormatScore},
		{"format upgrade over cutoff", release.SDTV, 1, []int{cf1}, release.SDTV, 1, []int{cf2}, UpgradeCustomFormatCutoff},
		{"format downgrade", release.SDTV, 1, []int{cf2}, release.SDTV, 1, []int{cf1}, UpgradeCustomFormatScore},

		{"lower quality", release.WEBDL720p, 1, nil, release.HDTV720p, 2, nil, UpgradeBetterQuality},
		{"lower quality again", release.WEBDL720p, 1, nil, release.HDTV720p, 2, nil, UpgradeBetterQuality},
		{"same webdl720p", release.WEBDL720p, 1, nil, release.WEBDL720p, 1, nil, UpgradeCustomFormatScore},
		{"same webdl1080p", release.WEBDL1080p, 1, nil, release.WEBDL1080p, 1, nil, UpgradeCustomFormatScore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := upProfile(true, release.Unknown)
			p.FormatItems = []FormatScore{{FormatID: tid(cf1), Score: 1}, {FormatID: tid(cf2), Score: 2}}
			u := Upgrades{Propers: PropersPreferAndUpgrade}
			got := u.IsUpgradable(p,
				upModel(tt.current, tt.currentVersion), upFormats(tt.currentFormats...),
				upModel(tt.newQuality, tt.newVersion), upFormats(tt.newFormats...))
			if got != tt.want {
				t.Errorf("IsUpgradable() got = %s, want = %s", upReasonName(got), upReasonName(tt.want))
			}
		})
	}
}

// TestUpgradesIsUpgradableSettings ports the remaining tests of Radarr's
// UpgradeSpecificationFixture.
func TestUpgradesIsUpgradableSettings(t *testing.T) {
	// The profile scores of the custom format score tests: One 10, Two 20.
	scored := func(minUpgrade int) *QualityProfile {
		p := upProfile(true, release.Unknown)
		p.MinUpgradeFormatScore = minUpgrade
		p.CutoffFormatScore = 100
		p.FormatItems = []FormatScore{{FormatID: tid(1), Score: 10}, {FormatID: tid(2), Score: 20}}
		return p
	}
	tests := []struct {
		name           string
		propers        ProperDownload
		profile        *QualityProfile
		current        QualityModel
		currentFormats []CustomFormat
		newQuality     QualityModel
		newFormats     []CustomFormat
		want           UpgradeRejection
	}{
		{
			name: "should_return_true_if_proper_and_download_propers_is_do_not_download", propers: PropersDoNotUpgrade,
			profile: upProfile(false, release.Unknown),
			current: upModel(release.DVD, 1), newQuality: upModel(release.DVD, 2), want: UpgradeAccepted,
		},
		{
			name: "should_return_false_if_proper_and_autoDownloadPropers_is_do_not_prefer", propers: PropersDoNotPrefer,
			profile: upProfile(false, release.Unknown),
			current: upModel(release.DVD, 1), newQuality: upModel(release.DVD, 2), want: UpgradeNotAllowed,
		},
		{
			name: "should_return_false_if_release_and_existing_file_are_the_same", propers: PropersPreferAndUpgrade,
			profile: upProfile(false, release.Unknown),
			current: upModel(release.HDTV720p, 1), newQuality: upModel(release.HDTV720p, 1), want: UpgradeNotAllowed,
		},
		{
			name: "should_return_true_if_release_has_higher_quality_and_cutoff_is_not_already_met", propers: PropersPreferAndUpgrade,
			profile: upProfile(true, release.HDTV1080p),
			current: upModel(release.HDTV720p, 1), newQuality: upModel(release.HDTV1080p, 1), want: UpgradeAccepted,
		},
		{
			name: "should_return_false_if_release_has_higher_quality_and_cutoff_is_already_met", propers: PropersPreferAndUpgrade,
			profile: upProfile(true, release.HDTV720p),
			current: upModel(release.HDTV720p, 1), newQuality: upModel(release.HDTV1080p, 1), want: UpgradeQualityCutoff,
		},
		{
			name: "should_return_false_if_minimum_custom_score_is_not_met", propers: PropersPreferAndUpgrade,
			profile: scored(11),
			current: upModel(release.DVD, 1), currentFormats: upFormats(1),
			newQuality: upModel(release.DVD, 1), newFormats: upFormats(2), want: UpgradeMinCustomFormatScore,
		},
		{
			name: "should_return_true_if_minimum_custom_score_is_met", propers: PropersPreferAndUpgrade,
			profile: scored(10),
			current: upModel(release.DVD, 1), currentFormats: upFormats(1),
			newQuality: upModel(release.DVD, 1), newFormats: upFormats(2), want: UpgradeAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := Upgrades{Propers: tt.propers}
			got := u.IsUpgradable(tt.profile, tt.current, tt.currentFormats, tt.newQuality, tt.newFormats)
			if got != tt.want {
				t.Errorf("IsUpgradable() got = %s, want = %s", upReasonName(got), upReasonName(tt.want))
			}
		})
	}
}
