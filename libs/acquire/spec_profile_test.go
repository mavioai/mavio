package acquire

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// spDefaultQualities returns the items of Radarr's test helper
// QualityFixture.GetDefaultQualities: its qualities not allowed first,
// then those allowed.
func spDefaultQualities(allowed ...release.Quality) []ProfileItem {
	qualities := []release.Quality{
		release.Unknown, release.Cam, release.Telecine, release.DVDSCR, release.Regional, release.SDTV,
		release.DVD, release.DVDR, release.HDTV720p, release.HDTV1080p, release.HDTV2160p, release.WEBDL480p,
		release.WEBDL720p, release.WEBDL1080p, release.WEBDL2160p, release.Bluray480p, release.Bluray576p,
		release.Bluray720p, release.Bluray1080p, release.Bluray2160p, release.BRDisk, release.RawHD,
	}
	var items []ProfileItem
	for _, q := range qualities {
		if !slices.Contains(allowed, q) {
			items = append(items, ProfileItem{Quality: q})
		}
	}
	for _, q := range allowed {
		items = append(items, ProfileItem{Quality: q, Allowed: true})
	}
	return items
}

// spSampleFormatItems returns the format items of Radarr's test helper
// CustomFormatsTestHelpers.GetSampleFormatItems over formats: the allowed
// ones score 1, 2, 4… in order, the others minus the next power of two.
func spSampleFormatItems(formats []CustomFormat, allowed ...string) []FormatScore {
	var allowedItems, disallowed []FormatScore
	for _, f := range formats {
		if slices.Contains(allowed, f.Name) {
			allowedItems = append(allowedItems, FormatScore{FormatID: f.ID, Score: 1 << len(allowedItems)})
		}
	}
	for _, f := range formats {
		if !slices.Contains(allowed, f.Name) {
			disallowed = append(disallowed, FormatScore{FormatID: f.ID, Score: -(1 << len(allowedItems))})
		}
	}
	return append(disallowed, allowedItems...)
}

// TestQualityAllowedByProfileSpec ports Radarr's
// QualityAllowedByProfileSpecificationFixture.
func TestQualityAllowedByProfileSpec(t *testing.T) {
	tests := []struct {
		name    string
		quality release.Quality
		want    bool
	}{
		{"should_allow_if_quality_is_defined_in_profile(DVD)", release.DVD, true},
		{"should_allow_if_quality_is_defined_in_profile(HDTV720p)", release.HDTV720p, true},
		{"should_allow_if_quality_is_defined_in_profile(Bluray1080p)", release.Bluray1080p, true},
		{"should_not_allow_if_quality_is_not_defined_in_profile(SDTV)", release.SDTV, false},
		{"should_not_allow_if_quality_is_not_defined_in_profile(WEBDL720p)", release.WEBDL720p, false},
		{"should_not_allow_if_quality_is_not_defined_in_profile(Bluray720p)", release.Bluray720p, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			p := &QualityProfile{
				ID: tid(1), Cutoff: QualityID(release.Bluray1080p),
				Items: spDefaultQualities(release.DVD, release.HDTV720p, release.Bluray1080p),
			}
			c := testMovie(p, tt.quality)
			c.Quality.Revision = release.Revision{Version: 2}
			spWant(t, QualityAllowedByProfile{}, e, c, nil, tt.want)
		})
	}
}

// TestCustomFormatAllowedByProfileSpec ports Radarr's
// CustomFormatAllowedByProfileSpecificationFixture.
func TestCustomFormatAllowedByProfileSpec(t *testing.T) {
	format1 := CustomFormat{ID: tid(1), Name: "Awesome Format"}
	format2 := CustomFormat{ID: tid(2), Name: "Cool Format"}
	all := []CustomFormat{format1, format2}
	tests := []struct {
		name     string
		formats  []CustomFormat
		allowed  []string
		minScore int
		want     bool
	}{
		{"should_allow_if_format_score_greater_than_min", []CustomFormat{format1}, []string{format1.Name}, 1, true},
		{"should_deny_if_format_score_not_greater_than_min", []CustomFormat{format2}, []string{format1.Name}, 1, false},
		{"should_deny_if_format_score_not_greater_than_min_2", []CustomFormat{format2, format1}, []string{format1.Name}, 1, false},
		{"should_allow_if_all_format_is_defined_in_profile", []CustomFormat{format2, format1}, []string{format1.Name, format2.Name}, 1, true},
		{"should_deny_if_no_format_was_parsed_and_min_score_positive", nil, []string{format1.Name, format2.Name}, 1, false},
		{"should_allow_if_no_format_was_parsed_min_score_is_zero", nil, []string{format1.Name, format2.Name}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			p := testProfile(release.Bluray1080p, release.DVD, release.Bluray1080p)
			p.MinFormatScore = tt.minScore
			p.FormatItems = spSampleFormatItems(all, tt.allowed...)
			c := testMovie(p, release.DVD)
			c.Quality.Revision = release.Revision{Version: 2}
			c.Formats = tt.formats
			c.FormatScore = p.FormatScore(c.Formats)
			spWant(t, CustomFormatAllowedByProfile{}, e, c, nil, tt.want)
		})
	}
}

// TestLanguageWantedSpec ports Radarr's LanguageSpecificationFixture.
func TestLanguageWantedSpec(t *testing.T) {
	tests := []struct {
		name      string
		wanted    release.Language
		languages []release.Language
		want      bool
	}{
		{"should_return_true_if_language_is_english", release.English, []release.Language{release.English}, true},
		{"should_return_false_if_language_is_german", release.English, []release.Language{release.German}, false},
		{"should_return_false_if_release_is_german_and_profile_original", release.Original, []release.Language{release.German}, false},
		{"should_return_true_if_release_is_french_and_profile_original", release.Original, []release.Language{release.French}, true},
		{"should_return_true_if_allowed_language_any (German)", release.Any, []release.Language{release.German}, true},
		{"should_return_true_if_allowed_language_any (English)", release.Any, []release.Language{release.English}, true},
		// Unknown is a language of its own, not any.
		{"unknown wanted, german found", release.LanguageUnknown, []release.Language{release.German}, false},
		{"unknown wanted, unknown found", release.LanguageUnknown, []release.Language{release.LanguageUnknown}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			p := testProfile(release.Bluray1080p, release.SDTV, release.Bluray1080p)
			p.Language = tt.wanted
			c := testMovie(p, release.SDTV)
			c.Target.OriginalLanguage = release.French
			c.Languages = tt.languages
			spWant(t, LanguageWanted{}, e, c, nil, tt.want)
		})
	}
}
