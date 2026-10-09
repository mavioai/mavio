package metadata

import (
	"encoding/json"
	"testing"
)

func (a args) optInt(t *testing.T, name string) (int, bool) {
	t.Helper()
	var n *int
	if err := json.Unmarshal(a[name], &n); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
	if n == nil {
		return 0, false
	}
	return *n, true
}

// TestPortedRatingScores runs the rating cases of Jellyfin's
// LocalizationManagerTests. Jellyfin's tests configure the server's
// metadata country, which RatingScore takes as its country; cases without
// one use Jellyfin's default, US. Mavio keeps no sub-scores, so the
// expected sub-scores are not compared.
func TestPortedRatingScores(t *testing.T) {
	score := func(value, country string) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			rating := a.str(t, value)
			c := "US"
			if country != "" {
				c = a.str(t, country)
			}
			want, rated := a.optInt(t, "expectedScore")
			got, ok := RatingScore(rating, c)
			if ok != rated || got != want {
				t.Errorf("RatingScore(%q, %q) got = %d %v, want = %d %v", rating, c, got, ok, want, rated)
			}
		}
	}
	unrated := func(value, country string) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			rating := a.str(t, value)
			c := "US"
			if country != "" {
				c = a.str(t, country)
			}
			if got, ok := RatingScore(rating, c); ok {
				t.Errorf("RatingScore(%q, %q) got = %d, want = unrated", rating, c, got)
			}
		}
	}
	localization := "localization data (languages, countries, translations) comes with server administration in P10"
	portedCases(t, "localization/localization_manager.json", ported{
		run: map[string]func(*testing.T, args){
			"GetRatingLevel_GivenValidString_Success":                     score("value", "countryCode"),
			"GetRatingScore_IsCaseInsensitive_Success":                    score("value", "countryCode"),
			"GetRatingLevel_GivenValidAge_Success":                        score("value", ""),
			"GetRatingLevel_SkipsUnratedListEntries_Success":              score("value", ""),
			"GetRatingScore_RatingContainingSlash_IsNotSplit":             score("value", "countryCode"),
			"GetRatingScore_CountryPrefixedList_UsesFirstResolvingEntry":  score("value", "countryCode"),
			"GetRatingLevel_Split_Success":                                unrated("value", ""),
			"GetRatingScore_FallbackPrioritizesUS_Success":                score("rating", "countryCode"),
			"GetRatingScore_UnknownRatingWithKnownCountry_ReturnsNull":    unrated("rating", "countryCode"),
			"GetRatingScore_ValidRatingWithCountrySeparator_ReturnsScore": score("rating", "countryCode"),
		},
		skip: map[string]string{
			"FindLanguageInfo_Valid_Success":                                                localization,
			"FindLanguageInfo_ISO6392Only_Success":                                          localization,
			"GetLanguageDisplayName_DelimitedName_ReturnsTruncatedName":                     localization,
			"GetLanguageDisplayName_InvalidInput_ReturnsNull":                               localization,
			"GetLocalizedString_Valid_Success":                                              localization,
			"GetCountries_All_Success":                                                      localization,
			"GetCultures_All_Success":                                                       localization,
			"TryGetISO6392TFromB_Success":                                                   localization,
			"GetParentalRatings_Default_Success":                                            "lists of a country's ratings come with localization data in P10",
			"GetParentalRatings_ConfiguredCountryCode_Success":                              "lists of a country's ratings come with localization data in P10",
			"GetLocalizedString_Invalid_Success":                                            localization,
			"GetLocalizedString_WithCulture_ReturnsTranslation":                             localization,
			"GetLocalizedString_WithCulture_FallsBackToEnUs":                                localization,
			"GetLocalizedString_WithBcp47Normalization_ReturnsTranslation":                  localization,
			"GetLocalizedString_WithBcp47NormalizationToUppercaseRegion_ReturnsTranslation": localization,
			"GetServerLocalizedString_UsesServerCulture":                                    localization,
			"GetLocalizedString_UsesCurrentUICulture":                                       localization,
			"GetSupportedUICultures_IncludesCommonCultures":                                 localization,
		},
		facts: map[string]string{
			"GetRatingLevel_GivenUnratedString_Success":              "TestRatingScoreFacts",
			"GetRatingScore_ResolvedCountryPrefixedList_DoesNotWarn": "TestRatingScoreFacts (RatingScore logs nothing)",
			"GetRatingScore_ListWithoutKnownRating_WarnsOnce":        "TestRatingScoreFacts (RatingScore logs nothing)",
		},
	})
}

// TestRatingScoreFacts ports the rating facts of Jellyfin's
// LocalizationManagerTests.
func TestRatingScoreFacts(t *testing.T) {
	for _, rating := range []string{"NR", "unrated", "Not Rated", "n/a", "N/A", " n/a "} {
		if score, ok := RatingScore(rating, "US"); ok {
			t.Errorf("RatingScore(%q) got = %d, want = unrated", rating, score)
		}
	}
	if score, ok := RatingScore("DE:FSK 18 / DE:FSK-18 / DE:FSK18 / DE:18 / DE:ab 18", "US"); !ok || score != 18 {
		t.Errorf("resolved country-prefixed list got = %d %v, want = 18", score, ok)
	}
	if score, ok := RatingScore("DE:Unbekannt / DE:Unsinn", "US"); ok {
		t.Errorf("list without a known rating got = %d, want = unrated", score)
	}
	// Scores start at zero: a G rating is rated.
	if score, ok := RatingScore("G", "US"); !ok || score != 0 {
		t.Errorf("G got = %d %v, want = 0 true", score, ok)
	}
}
