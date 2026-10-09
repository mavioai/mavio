package main

import (
	"fmt"
	"slices"
	"testing"
)

func TestTMDBUtilsCases(t *testing.T) {
	portedCases(t, "tmdb_utils.json", ported{
		run: map[string]func(*testing.T, args){
			"NormalizeLanguage_Valid_Success": checkNormalizeLanguage,
			"NormalizeLanguage_Invalid_Equal": checkNormalizeLanguage,
			"GetImageLanguage_Valid_Success": func(t *testing.T, a args) {
				got := imageLanguage(a.str(t, "imageLanguage"), a.str(t, "imageRegion"), a.str(t, "requestLanguage"))
				if want := a.str(t, "expected"); got != want {
					t.Errorf("imageLanguage() = %q, want = %q", got, want)
				}
			},
			"TryParseTmdbId_OnlyAcceptsTmdbIds": checkParseTMDBID,
			// Items keep their IDs in a map keyed by provider.
			"TryGetTmdbId_OnlyAcceptsTmdbIds": func(t *testing.T, a args) {
				ids := map[string]string{keyTMDB: a.str(t, "value")}
				id, ok := parseTMDBID(ids[keyTMDB])
				if want := a.boolean(t, "expected"); ok != want || id != a.integer(t, "expectedId") {
					t.Errorf("parseTMDBID() = %d, %v, want = %d, %v", id, ok, a.integer(t, "expectedId"), want)
				}
			},
			"CleanName_Valid_Success": func(t *testing.T, a args) {
				if got, want := cleanName(a.str(t, "name")), a.str(t, "expected"); got != want {
					t.Errorf("cleanName() = %q, want = %q", got, want)
				}
			},
			"NormalizeTitle_Valid_Success": func(t *testing.T, a args) {
				if got, want := normalizeTitle(a.str(t, "title")), a.str(t, "expected"); got != want {
					t.Errorf("normalizeTitle() = %q, want = %q", got, want)
				}
			},
			"IsOriginalImageSize_Valid_Success": func(t *testing.T, a args) {
				if got, want := isOriginalImageSize(a.str(t, "size")), a.boolean(t, "expected"); got != want {
					t.Errorf("isOriginalImageSize() = %v, want = %v", got, want)
				}
			},
		},
		facts: map[string]string{
			"TryGetTmdbId_NoId_False":                        "TestParseTMDBIDMissing",
			"FindBestMatch_Movies_PicksExpected":             "TestFindBestMatchMovies",
			"FindBestMatch_Series_PicksMatchingFirstAirYear": "TestFindBestMatchSeries",
			"FindBestMatch_NoResults_ReturnsNull":            "TestFindBestMatchNoResults",
		},
	})
}

func checkNormalizeLanguage(t *testing.T, a args) {
	if got, want := normalizeLanguage(a.str(t, "input"), ""), a.str(t, "expected"); got != want {
		t.Errorf("normalizeLanguage() = %q, want = %q", got, want)
	}
}

func checkParseTMDBID(t *testing.T, a args) {
	id, ok := parseTMDBID(a.str(t, "value"))
	if want, wantID := a.boolean(t, "expected"), a.integer(t, "expectedId"); ok != want || id != wantID {
		t.Errorf("parseTMDBID() = %d, %v, want = %d, %v", id, ok, wantID, want)
	}
}

func TestParseTMDBIDMissing(t *testing.T) {
	if id, ok := parseTMDBID(map[string]string{}[keyTMDB]); ok {
		t.Errorf("parseTMDBID() = %d, true, want = false", id)
	}
}

func TestNormalizeLanguageLatinAmericanSpanish(t *testing.T) {
	for _, tt := range []struct{ lang, country, want string }{
		{"es-419", "AR", "es-AR"},
		{"es-419", "CO", "es-MX"},
		{"es-419", "", "es-419"},
	} {
		if got := normalizeLanguage(tt.lang, tt.country); got != tt.want {
			t.Errorf("normalizeLanguage(%q, %q) = %q, want = %q", tt.lang, tt.country, got, tt.want)
		}
	}
}

func movie(id int, title, original string, year int) (int, candidate) {
	return id, candidate{title: title, originalTitle: original, date: fmt.Sprintf("%04d-06-01", year)}
}

// TestFindBestMatchMovies ports FindBestMatch_Movies_TestData, which keeps
// results in the order the live API returned them.
func TestFindBestMatchMovies(t *testing.T) {
	type result struct {
		id int
		c  candidate
	}
	m := func(id int, title, original string, year int) result {
		id, c := movie(id, title, original, year)
		return result{id, c}
	}
	tests := []struct {
		desc    string
		name    string
		year    int
		results []result
		want    int
	}{
		{"Mulan (2020)", "Mulan", 2020, []result{m(10674, "Mulan", "Mulan", 1998), m(337401, "Mulan", "Mulan", 2020), m(752662, "Hua Mulan", "花木兰", 2020)}, 337401},
		{"Mulan (1998)", "Mulan", 1998, []result{m(10674, "Mulan", "Mulan", 1998), m(337401, "Mulan", "Mulan", 2020), m(752662, "Hua Mulan", "花木兰", 2020)}, 10674},
		{"Aladdin (2019)", "Aladdin", 2019, []result{m(812, "Aladdin", "Aladdin", 1992), m(420817, "Aladdin", "Aladdin", 2019), m(602411, "Adventures of Aladdin", "Adventures of Aladdin", 2019)}, 420817},
		{"The Lion King (2019)", "The Lion King", 2019, []result{m(8587, "The Lion King", "The Lion King", 1994), m(420818, "The Lion King", "The Lion King", 2019)}, 420818},
		{"The Amityville Horror (1979)", "The Amityville Horror", 1979, []result{m(10065, "The Amityville Horror", "The Amityville Horror", 2005), m(11449, "The Amityville Horror", "The Amityville Horror", 1979)}, 11449},
		{"WALL-E (2008)", "WALL-E", 2008, []result{m(877268, "WALL·E's Treasures & Trinkets", "WALL·E's Treasures & Trinkets", 2008), m(10681, "WALL·E", "WALL·E", 2008), m(10673, "Wall Street", "Wall Street", 1987)}, 10681},
		{"8½ (1963)", "8½", 1963, []result{m(422801, "Interpol Code 8", "国際秘密警察　指令第８号", 1963), m(520251, "Um 8 Uhr kommt Sadowski", "Um 8 Uhr kommt Sadowski", 1963), m(422, "8½", "8½", 1963)}, 422},
		{"Ściany mają uszy (1966)", "Ściany mają uszy", 1966, []result{m(1, "Something Else", "Something Else", 1966), m(2, "Walls Have Ears", "Ściany mają uszy", 1966)}, 2},
		{"Off by one year", "Some Movie", 2011, []result{m(1, "Some Movie", "Some Movie", 2015), m(2, "Some Movie", "Some Movie", 2010)}, 2},
		{"A Christmas No. 1 (2021)", "A Christmas No. 1", 2021, []result{m(878111, "A Christmas Number One", "A Christmas Number One", 2021), m(2, "Ten Hours for Christmas", "10 Horas para o Natal", 2021)}, 878111},
		{"Title outranks year", "Some Movie", 2020, []result{m(1, "A Different Movie", "A Different Movie", 2020), m(2, "Some Movie", "Some Movie", 1994)}, 2},
		{"No year known", "Mulan", 0, []result{m(10674, "Mulan", "Mulan", 1998), m(337401, "Mulan", "Mulan", 2020)}, 10674},
		{"Empty name", "  ", 2020, []result{m(1, "Some Movie", "Some Movie", 1994), m(2, "Some Movie", "Some Movie", 2020)}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cands := make([]candidate, len(tt.results))
			for i, r := range tt.results {
				cands[i] = r.c
			}
			i := findBestMatch(cands, tt.name, tt.year)
			if i < 0 {
				t.Fatalf("findBestMatch() = %d, want = a match", i)
			}
			if got := tt.results[i].id; got != tt.want {
				t.Errorf("findBestMatch() = %d, want = %d", got, tt.want)
			}
		})
	}
}

func TestFindBestMatchSeries(t *testing.T) {
	ids := []int{10042, 101048, 255055, 2430}
	results := []candidate{
		{title: "Doc", originalTitle: "Doc", date: "2001-06-01"},
		{title: "Doc", originalTitle: "Doc", date: "2020-06-01"},
		{title: "Doc", originalTitle: "Doc", date: "2025-06-01"},
		{title: "Doc Martin", originalTitle: "Doc Martin", date: "2004-06-01"},
	}
	i := findBestMatch(results, "Doc", 2025)
	if i < 0 || ids[i] != 255055 {
		t.Errorf("findBestMatch() = %d, want = index of 255055", i)
	}
}

func TestFindBestMatchNoResults(t *testing.T) {
	for _, results := range [][]candidate{nil, {}} {
		if got := findBestMatch(results, "Mulan", 2020); got != -1 {
			t.Errorf("findBestMatch(%v) = %d, want = -1", results, got)
		}
	}
}

func TestParentalRating(t *testing.T) {
	for _, tt := range []struct{ country, rating, want string }{
		{"US", "PG-13", "PG-13"},
		{"DE", "12", "FSK-12"},
		{"GB", "15", "GB-15"},
	} {
		if got := parentalRating(tt.country, tt.rating); got != tt.want {
			t.Errorf("parentalRating(%q, %q) = %q, want = %q", tt.country, tt.rating, got, tt.want)
		}
	}
}

func TestImageLanguages(t *testing.T) {
	for _, tt := range []struct{ lang, want string }{
		{"de-ch", "de,null,en"},
		{"en", "en,null"},
		{"", "null,en"},
	} {
		if got := imageLanguages(tt.lang, ""); got != tt.want {
			t.Errorf("imageLanguages(%q) = %q, want = %q", tt.lang, got, tt.want)
		}
	}
}

func TestTMDBUtilsCastCases(t *testing.T) {
	cfg := castConfig{maxCast: 10}
	portedCases(t, "tmdb_utils_cast.json", ported{
		run: map[string]func(*testing.T, args){
			"MapCast_NoCast_YieldsNothing": func(t *testing.T, a args) {
				var got []credit
				if a.boolean(t, "aggregate") {
					got = mapAggregateCast(nil, cfg)
				} else {
					got = mapCast(nil, cfg)
				}
				if len(got) != 0 {
					t.Errorf("credits = %v, want = none", got)
				}
			},
		},
		facts: map[string]string{
			"MapAggregateCast_MemberWithSeveralRoles_YieldsOneCreditPerRole":      "TestMapAggregateCast/several_roles",
			"MapAggregateCast_MemberWithoutARole_IsStillCredited":                 "TestMapAggregateCast/without_a_role",
			"MapAggregateCast_MoreThanConfigured_KeepsTheTopBilled":               "TestMapAggregateCast/top_billed",
			"MapAggregateCast_HideMissingCastMembers_DropsTheOnesWithoutAProfile": "TestMapAggregateCast/hide_missing",
			"MapCast_FlatCredits_YieldOneCreditEach":                              "TestMapCast",
		},
	})
}

func aggregate(name string, id, order int, roles ...tmdbRole) tmdbAggregateCast {
	return tmdbAggregateCast{Name: name, ID: id, Order: order, Roles: roles}
}

func TestMapAggregateCast(t *testing.T) {
	cfg := castConfig{maxCast: 10}
	t.Run("several roles", func(t *testing.T) {
		got := mapAggregateCast([]tmdbAggregateCast{
			aggregate("Megumi Toyoguchi", 1, 0, tmdbRole{"Tabby (voice)", 3}, tmdbRole{"Mimiru (voice)", 12}),
		}, cfg)
		// The character they played the longest comes first.
		want := []credit{{name: "Megumi Toyoguchi", role: "Mimiru (voice)", id: 1}, {name: "Megumi Toyoguchi", role: "Tabby (voice)", id: 1}}
		if !slices.Equal(got, want) {
			t.Errorf("mapAggregateCast() = %+v, want = %+v", got, want)
		}
	})
	t.Run("without a role", func(t *testing.T) {
		got := mapAggregateCast([]tmdbAggregateCast{aggregate("Uncredited Actor", 2, 0)}, cfg)
		if want := []credit{{name: "Uncredited Actor", id: 2}}; !slices.Equal(got, want) {
			t.Errorf("mapAggregateCast() = %+v, want = %+v", got, want)
		}
	})
	t.Run("top billed", func(t *testing.T) {
		var cast []tmdbAggregateCast
		for i := range 5 {
			cast = append(cast, aggregate(fmt.Sprintf("Actor %d", 4-i), i+1, 4-i, tmdbRole{fmt.Sprintf("Role %d", 4-i), 1}))
		}
		var got []string
		for _, c := range mapAggregateCast(cast, castConfig{maxCast: 2}) {
			got = append(got, c.name)
		}
		if want := []string{"Actor 0", "Actor 1"}; !slices.Equal(got, want) {
			t.Errorf("names = %v, want = %v", got, want)
		}
	})
	t.Run("hide missing", func(t *testing.T) {
		with := aggregate("Has Profile", 1, 0, tmdbRole{"Hero", 1})
		with.ProfilePath = "/profile.jpg"
		got := mapAggregateCast([]tmdbAggregateCast{with, aggregate("No Profile", 2, 1, tmdbRole{"Villain", 1})}, castConfig{maxCast: 10, hideMissing: true})
		if len(got) != 1 || got[0].name != "Has Profile" {
			t.Errorf("mapAggregateCast() = %+v, want = Has Profile only", got)
		}
	})
}

func TestMapCast(t *testing.T) {
	got := mapCast([]tmdbCast{
		{Name: "Kevin Conroy", ID: 1, Order: 0, Character: " Batman (voice) "},
		{Name: "  ", ID: 2, Order: 1, Character: "Nobody"},
	}, castConfig{maxCast: 10})
	if want := []credit{{name: "Kevin Conroy", role: "Batman (voice)", id: 1}}; !slices.Equal(got, want) {
		t.Errorf("mapCast() = %+v, want = %+v", got, want)
	}
}

func TestMissingEpisodeProviderCases(t *testing.T) {
	const reason = "virtual episodes for missing and unaired episodes are not part of the domain model yet"
	skip := map[string]string{"ShouldImportEpisode_RespectsAirDateAndOptions": reason}
	for _, fact := range []string{
		"ShouldPrune_AgedOutVirtualTmdbEpisode_ReturnsTrue", "ShouldPrune_NotInPruningMode_ReturnsFalse",
		"ShouldPrune_StillUpcoming_ReturnsFalse", "ShouldPrune_VirtualEpisodeFromAnotherProvider_ReturnsFalse",
		"ShouldPrune_PhysicalEpisode_ReturnsFalse", "ShouldPrune_AiredWithinGracePeriod_ReturnsFalse",
		"ShouldPrune_AiredBeyondGracePeriod_ReturnsTrue", "ShouldPrune_SpecialWithSpecialsDisabled_ReturnsTrue",
		"ShouldPrune_SpecialWithSpecialsEnabled_FollowsNormalRules", "GetPremiereDate_NullAirDate_ReturnsNull",
		"GetPremiereDate_AirDate_ReturnsUtc", "UpdateVirtualEpisode_PlaceholderTitleReplaced_UpdatesAndReturnsTrue",
		"UpdateVirtualEpisode_NoChanges_ReturnsFalse", "UpdateVirtualEpisode_EmptyTmdbValues_DoNotOverwrite",
		"UpdateVirtualEpisode_RescheduledAirDate_UpdatesPremiereAndYear", "BuildSeasonSortNameTemplate_NoNameSortedPhysicalSeason_ReturnsNull",
		"BuildSeasonSortNameTemplate_MirrorsSiblingConventionAndSwapsNumber", "BuildSeasonSortNameTemplate_PreservesNonEnglishToken",
		"BuildSeasonSortNameTemplate_SiblingWithoutDigits_ReturnsNull", "BuildSeasonSortNameTemplate_IgnoresVirtualSeasonsAsReference",
	} {
		skip[fact] = reason
	}
	portedCases(t, "tmdb_missing_episode_provider.json", ported{skip: skip})
}
