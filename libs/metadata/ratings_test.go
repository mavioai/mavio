package metadata

import (
	"encoding/json"
	"slices"
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
	translation := "the server does not translate strings; clients localize what they show"
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
			"FindLanguageInfo_Valid_Success": func(t *testing.T, a args) {
				l, ok := FindLanguage(a.str(t, "identifier"))
				if !ok || l.ThreeLetterCodes[0] != "deu" || l.DisplayName != "German" || l.Name != "German" || !slices.Contains(l.ThreeLetterCodes, "ger") {
					t.Errorf("FindLanguage got = %+v %v, want = German", l, ok)
				}
			},
			"FindLanguageInfo_ISO6392Only_Success": func(t *testing.T, a args) {
				code := a.str(t, "code")
				l, ok := FindLanguage(code)
				if want := a.str(t, "expectedDisplayName"); !ok || l.DisplayName != want || l.ThreeLetterCodes[0] != code {
					t.Errorf("FindLanguage(%q) got = %+v %v, want = %s", code, l, ok, want)
				}
			},
			"GetLanguageDisplayName_DelimitedName_ReturnsTruncatedName": func(t *testing.T, a args) {
				if got, ok := LanguageDisplayName(a.str(t, "language")); !ok || got != a.str(t, "expected") {
					t.Errorf("LanguageDisplayName got = %q %v, want = %q", got, ok, a.str(t, "expected"))
				}
			},
			"GetLanguageDisplayName_InvalidInput_ReturnsNull": func(t *testing.T, a args) {
				var lang *string
				if err := json.Unmarshal(a["language"], &lang); err != nil {
					t.Fatal(err)
				}
				if lang == nil {
					lang = new("")
				}
				if got, ok := LanguageDisplayName(*lang); ok {
					t.Errorf("LanguageDisplayName(%q) got = %q, want = none", *lang, got)
				}
			},
		},
		skip: map[string]string{
			"GetLocalizedString_Valid_Success":                                              translation,
			"GetLocalizedString_Invalid_Success":                                            translation,
			"GetLocalizedString_WithCulture_ReturnsTranslation":                             translation,
			"GetLocalizedString_WithCulture_FallsBackToEnUs":                                translation,
			"GetLocalizedString_WithBcp47Normalization_ReturnsTranslation":                  translation,
			"GetLocalizedString_WithBcp47NormalizationToUppercaseRegion_ReturnsTranslation": translation,
			"GetServerLocalizedString_UsesServerCulture":                                    translation,
			"GetLocalizedString_UsesCurrentUICulture":                                       translation,
			"GetSupportedUICultures_IncludesCommonCultures":                                 translation,
		},
		facts: map[string]string{
			"GetCountries_All_Success":                               "TestLocalizationFacts",
			"GetCultures_All_Success":                                "TestLocalizationFacts",
			"TryGetISO6392TFromB_Success":                            "TestLocalizationFacts",
			"GetParentalRatings_Default_Success":                     "TestLocalizationFacts (the default country is US)",
			"GetParentalRatings_ConfiguredCountryCode_Success":       "TestLocalizationFacts",
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

// TestLocalizationFacts ports the localization facts of Jellyfin's
// LocalizationManagerTests.
func TestLocalizationFacts(t *testing.T) {
	countries := Countries()
	i := slices.IndexFunc(countries, func(c Country) bool { return c.Code == "DE" })
	if len(countries) != 140 || i < 0 || countries[i].Name != "Germany" || countries[i].ThreeLetterCode != "DEU" {
		t.Errorf("Countries got = %d, Germany %+v", len(countries), countries[max(i, 0)])
	}
	languages := Languages()
	i = slices.IndexFunc(languages, func(l Language) bool { return l.TwoLetterCode == "de" })
	if len(languages) != 496 || i < 0 || languages[i].ThreeLetterCodes[0] != "deu" || languages[i].Name != "German" ||
		!slices.Contains(languages[i].ThreeLetterCodes, "ger") {
		t.Errorf("Languages got = %d, German %+v", len(languages), languages[max(i, 0)])
	}
	for b, want := range map[string]string{"ger": "deu", "chi": "zho", "eng": ""} {
		if got, ok := ISO6392TFromB(b); got != want || ok != (want != "") {
			t.Errorf("ISO6392TFromB(%q) got = %q %v, want = %q", b, got, ok, want)
		}
	}
	for _, c := range []struct {
		country, name string
		count, score  int
	}{{"US", "TV-MA", 56, 17}, {"DE", "FSK-12", 34, 12}} {
		ratings := ParentalRatings(c.country)
		i := slices.IndexFunc(ratings, func(r ParentalRating) bool { return r.Name == c.name })
		if len(ratings) != c.count || i < 0 || ratings[i].Score == nil || *ratings[i].Score != c.score {
			t.Errorf("ParentalRatings(%s) got = %d ratings, %s at %d", c.country, len(ratings), c.name, i)
		}
	}
}
