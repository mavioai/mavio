package release

import (
	"slices"
	"strings"
	"testing"
)

// parseMovie parses a title and fails the test when it returns nil.
func parseMovie(t *testing.T, title string, isDir bool) *ParsedMovieInfo {
	t.Helper()
	p := ParseMovieTitle(title, isDir)
	if p == nil {
		t.Fatalf("ParseMovieTitle(%q, %v): got = nil", title, isDir)
	}
	return p
}

func TestMovieAnimeVersionCases(t *testing.T) {
	portedCases(t, "movie/anime_version.json", ported{run: map[string]func(*testing.T, args){
		"should_be_able_to_parse_repack": checkVersion(Movie),
	}})
}

func movieNotParsed(name string) func(*testing.T, args) {
	return func(t *testing.T, a args) {
		title := a.str(t, name)
		if p := ParseMovieTitle(title, false); p != nil {
			t.Errorf("ParseMovieTitle(%q): got = %+v, want = nil", title, p)
		}
	}
}

func TestMovieCrapCases(t *testing.T) {
	portedCases(t, "movie/crap_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_not_parse_crap": movieNotParsed("title"),
			"should_not_parse_random": func(t *testing.T, a args) {
				randomTitles(t, a, func(s string) bool { return ParseMovieTitle(s, false) != nil })
			},
			"should_not_parse_file_name_without_proper_spacing": movieNotParsed("fileName"),
			"should_not_parse_invalid_release_name":             movieNotParsed("fileName"),
		},
		facts: map[string]string{"should_not_parse_md5": "TestMovieNotParseMD5"},
	})
}

func TestMovieNotParseMD5(t *testing.T) {
	md5Titles(t, func(s string) bool { return ParseMovieTitle(s, false) != nil })
}

func TestMovieEditionCases(t *testing.T) {
	edition := func(t *testing.T, a args) {
		check(t, "Edition", parseMovie(t, a.str(t, "postTitle"), false).Edition, a.str(t, "edition"))
	}
	portedCases(t, "movie/edition_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_edition":     edition,
		"should_not_parse_edition": edition,
	}})
}

func TestMovieHashedReleaseCases(t *testing.T) {
	portedCases(t, "movie/hashed_release.json", ported{run: map[string]func(*testing.T, args){
		"should_properly_parse_hashed_releases": func(t *testing.T, a args) {
			path := osAgnostic(t, a, "path")
			p := ParseMoviePath(path)
			if p == nil {
				t.Fatalf("ParseMoviePath(%q): got = nil", path)
			}
			check(t, "PrimaryMovieTitle", p.PrimaryMovieTitle(), a.str(t, "title"))
			check(t, "Quality", p.Quality.Quality, quality(t, a, "quality"))
			check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "releaseGroup"))
		},
	}})
}

func TestMovieIsoLanguageCases(t *testing.T) {
	run := map[string]func(*testing.T, args){
		"should_return_iso_language_for_English":     isoLanguage(Movie, English),
		"unknown_or_invalid_code_should_return_null": isoLanguage(Movie, LanguageUnknown),
		"should_not_return_portuguese":               isoLanguage(Movie, LanguageUnknown),
	}
	for _, l := range []Language{Portuguese, Telugu, Afrikaans, Marathi, Tagalog, Urdu, Romansh, Mongolian, Bengali, Georgian} {
		run["should_return_"+strings.ToLower(l.String())] = isoLanguage(Movie, l)
	}
	portedCases(t, "movie/iso_languages.json", ported{run: run})
}

func TestMovieLanguageCases(t *testing.T) {
	dir := func(t *testing.T, title string) []Language { return parseMovie(t, title, true).Languages }
	file := func(t *testing.T, title string) []Language { return parseMovie(t, title, false).Languages }
	languages := func(_ *testing.T, title string) []Language { return ParseLanguages(title, Movie) }
	languagesOf := map[string]func(*testing.T, string) []Language{"": dir, "release": file}
	for _, l := range []string{"arabic", "persian", "bengali", "telugu", "slovak", "latvian"} {
		languagesOf["should_parse_language_"+l] = file
	}
	for _, l := range []string{"malayalam", "kannada", "albanian", "afrikaans", "marathi", "tagalog", "urdu", "romansh", "mongolian", "georgian"} {
		languagesOf["should_parse_language_"+l] = languages
	}
	run := languageChecks(t, "movie/language_parser.json", Movie, languagesOf)
	contain := func(want ...Language) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			got := dir(t, title)
			for _, l := range want {
				if !contains(got, l) {
					t.Errorf("languages of %q: got = %v, want to contain %v", title, got, l)
				}
			}
		}
	}
	run["should_parse_language_french_english"] = contain(French, English)
	run["should_parse_language_ukrainian_multi"] = contain(Ukrainian)
	run["should_parse_language_polish_subbed"] = contain(LanguageUnknown)
	run["should_parse_language_spanish_latino"] = func(t *testing.T, a args) {
		title := a.str(t, "postTitle")
		got := ParseLanguages(title, Movie)
		if len(got) == 0 || got[0] != SpanishLatino {
			t.Errorf("ParseLanguages(%q): got = %v, want SpanishLatino first", title, got)
		}
	}
	portedCases(t, "movie/language_parser.json", ported{run: run})
}

func TestCleanMovieTitleCases(t *testing.T) {
	portedCases(t, "movie/normalize_title.json", ported{
		run: cleanTitleChecks(CleanMovieTitle),
		facts: map[string]string{
			"should_remove_a_from_middle_of_title":         "TestCleanMovieTitleArticleA",
			"should_not_remove_a_when_at_start_of_acronym": "TestCleanMovieTitleArticleA",
			"should_not_remove_a_when_at_end_of_acronym":   "TestCleanMovieTitleArticleA",
			"should_not_clean_trailing_a":                  "TestCleanMovieTitleArticleA",
		},
	})
}

func TestCleanMovieTitleArticleA(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"word.a.word", "wordword"},
		{"word a word", "wordword"},
		{"word-a-word", "wordword"},
		{"word.a.N.K.L.E.word", "wordankleword"},
		{"word a N K L E word", "wordankleword"},
		{"word-a-N-K-L-E-word", "wordankleword"},
		{"word.N.K.L.E.a.word", "wordnkleaword"},
		{"word N K L E a word", "wordnkleaword"},
		{"word-N-K-L-E-a-word", "wordnkleaword"},
		{"Tokyo Ghoul A", "tokyoghoula"},
	} {
		check(t, "CleanMovieTitle("+c.in+")", CleanMovieTitle(c.in), c.want)
	}
}

func TestMovieParserCases(t *testing.T) {
	title := func(isDir bool) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			check(t, "PrimaryMovieTitle", parseMovie(t, a.str(t, "postTitle"), isDir).PrimaryMovieTitle(), a.str(t, "title"))
		}
	}
	portedCases(t, "movie/parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_parse_movie_title": title(false),
			"should_parse_anime_movie_title": func(t *testing.T, a args) {
				p := parseMovie(t, a.str(t, "postTitle"), false)
				check(t, "PrimaryMovieTitle", p.PrimaryMovieTitle(), a.str(t, "title"))
				check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "releaseGroup"))
				check(t, "Year", p.Year, a.integer(t, "year"))
			},
			"should_parse_anime_movie_title_without_year": func(t *testing.T, a args) {
				p := parseMovie(t, a.str(t, "postTitle"), false)
				check(t, "PrimaryMovieTitle", p.PrimaryMovieTitle(), a.str(t, "title"))
				check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "releaseGroup"))
			},
			"should_parse_german_movie": func(t *testing.T, a args) {
				p := parseMovie(t, a.str(t, "postTitle"), false)
				check(t, "PrimaryMovieTitle", p.PrimaryMovieTitle(), a.str(t, "title"))
				check(t, "Edition", p.Edition, a.str(t, "edition"))
				check(t, "Year", p.Year, a.integer(t, "year"))
			},
			"should_parse_movie_alternative_titles": func(t *testing.T, a args) {
				got := parseMovie(t, a.str(t, "postTitle"), true).MovieTitles
				if want := a.strs(t, "parsedTitles"); !slices.Equal(got, want) {
					t.Errorf("MovieTitles: got = %q, want = %q", got, want)
				}
			},
			"should_parse_movie_folder_name": title(true),
			"should_parse_movie_year": func(t *testing.T, a args) {
				check(t, "Year", parseMovie(t, a.str(t, "postTitle"), false).Year, a.integer(t, "year"))
			},
			"should_parse_tmdb_id": func(t *testing.T, a args) {
				check(t, "TmdbID", parseMovie(t, a.str(t, "postTitle"), false).TmdbID, a.integer(t, "tmdbId"))
			},
			"should_not_parse_wrong_language_in_title": func(t *testing.T, a args) {
				got := parseMovie(t, a.str(t, "postTitle"), true).Languages
				if !slices.Equal(got, []Language{LanguageUnknown}) {
					t.Errorf("Languages: got = %v, want = [Unknown]", got)
				}
			},
			"should_parse_hardcoded_subs": func(t *testing.T, a args) {
				check(t, "HardcodedSubs", parseMovie(t, a.str(t, "postTitle"), false).HardcodedSubs, a.str(t, "sub"))
			},
			"should_parse_imdb_in_title": func(t *testing.T, a args) {
				check(t, "ImdbID", parseMovie(t, a.str(t, "postTitle"), true).ImdbID, a.str(t, "imdb"))
			},
			"should_normalize_imdbid": func(t *testing.T, a args) {
				id := a.str(t, "imdbid")
				check(t, "NormalizeImdbID("+id+")", NormalizeImdbID(id), a.str(t, "normalized"))
			},
		},
		facts: map[string]string{"should_remove_accents_from_title": "TestCleanMovieTitleAccents"},
	})
}

func TestCleanMovieTitleAccents(t *testing.T) {
	check(t, "CleanMovieTitle", CleanMovieTitle("Carnivàle"), "carnivale")
}

func TestMovieReleaseGroupCases(t *testing.T) {
	run := map[string]func(*testing.T, args){}
	for _, m := range []string{
		"should_parse_release_group", "should_parse_exception_release_group", "should_not_include_extension_in_release_group",
		"should_not_include_language_in_release_group", "should_not_include_bad_suffix_in_release_group",
		"should_parse_anime_release_groups", "should_not_parse_anime_hash_as_release_group",
	} {
		run[m] = releaseGroup(Movie)
	}
	portedCases(t, "movie/release_group_parser.json", ported{run: run})
}

func TestMovieSceneCheckerCases(t *testing.T) {
	scene := func(want bool) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "title")
			check(t, "IsSceneTitle("+title+")", IsSceneTitle(title, Movie), want)
		}
	}
	portedCases(t, "movie/scene_checker.json", ported{run: map[string]func(*testing.T, args){
		"should_return_true_for_scene_names":      scene(true),
		"should_return_false_for_non_scene_names": scene(false),
		"should_correctly_parse_scene_names": func(t *testing.T, a args) {
			title := a.str(t, "title")
			check(t, "SceneTitle("+title+")", SceneTitle(title, Movie), a.str(t, "result"))
		},
	}})
}

func TestURLSlugCases(t *testing.T) {
	slug := func(invalidDash bool, trim, dedupe string, want func(a args, t *testing.T) string) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			in := a.str(t, "input")
			if a.has("trimList") {
				trim = a.str(t, "trimList")
			}
			if a.has("deduplicateChars") {
				dedupe = a.str(t, "deduplicateChars")
			}
			check(t, "URLSlug("+in+")", URLSlug(in, invalidDash, trim, dedupe), want(a, t))
		}
	}
	result := func(a args, t *testing.T) string { return a.str(t, "result") }
	fixed := func(s string) func(args, *testing.T) string { return func(args, *testing.T) string { return s } }
	portedCases(t, "movie/slug_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_replace_accents":                                         slug(false, "-_", "-_", result),
			"should_replace_special_characters":                              slug(false, "-_", "-_", fixed("testresult")),
			"should_lowercase_capitals":                                      slug(false, "-_", "-_", fixed("this-is-a-mixed-case-sensitive-value")),
			"should_trim_trailing_dashes_and_underscores":                    slug(false, "-_", "-_", fixed("test")),
			"should_replace_spaces_with_dash":                                slug(false, "-_", "-_", fixed("test-result")),
			"should_replace_double_occurence":                                slug(false, "-_", "-_", result), //nolint:misspell // upstream test name
			"should_replace_special_characters_with_dash_when_enabled":       slug(true, "-_", "-_", fixed("test-result")),
			"should__not_replace_special_characters_with_dash_when_disabled": slug(false, "-_", "-_", fixed("testresult")),
			"should_trim_trailing_dashes_and_underscores_based_on_list":      slug(false, "", "", result),
			"should_replace_duplicate_characters_based_on_list":              slug(false, "", "", result),
		},
		facts: map[string]string{
			"should_handle_null_trim_parameters":    "TestURLSlugEmptyParameters",
			"should_handle_null_dedupe_parameters":  "TestURLSlugEmptyParameters",
			"should_handle_empty_trim_parameters":   "TestURLSlugEmptyParameters",
			"should_handle_empty_dedupe_parameters": "TestURLSlugEmptyParameters",
		},
	})
}

// TestURLSlugEmptyParameters covers both null and empty parameters, which
// are the same in Go.
func TestURLSlugEmptyParameters(t *testing.T) {
	check(t, "URLSlug without trimming", URLSlug("test", false, "", "-_"), "test")
	check(t, "URLSlug without deduplication", URLSlug("test", false, "-_", ""), "test")
}

func TestMovieURLCases(t *testing.T) {
	portedCases(t, "movie/url.json", ported{run: map[string]func(*testing.T, args){
		"should_not_parse_url_in_name": func(t *testing.T, a args) {
			p := parseMovie(t, a.str(t, "postTitle"), false)
			check(t, "PrimaryMovieTitle", CleanMovieTitle(p.PrimaryMovieTitle()), CleanMovieTitle(a.str(t, "title")))
		},
		"should_not_parse_url_in_group": releaseGroup(Movie),
	}})
}

// romanNumerals1To20 is the part of Radarr's ArabicRomanNumeralDictionary
// that its fixture tests.
var romanNumerals1To20 = []string{
	"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X",
	"XI", "XII", "XIII", "XIV", "XV", "XVI", "XVII", "XVIII", "XIX", "XX",
}

func TestRomanNumeralCases(t *testing.T) {
	portedCases(t, "movie/roman_numeral_tests/roman_numeral_conversion.json", ported{facts: map[string]string{
		"should_convert_arabic_numeral_to_roman_numeral": "TestRomanNumeral",
		"should_convert_roman_numeral_to_arabic_numeral": "TestRomanNumeral",
	}})
}

func TestRomanNumeral(t *testing.T) {
	for i, want := range romanNumerals1To20 {
		check(t, "RomanNumeral", RomanNumeral(i+1), want)
		n, ok := ParseRomanNumeral(want)
		if !ok || n != i+1 {
			t.Errorf("ParseRomanNumeral(%s): got = %d, %v, want = %d", want, n, ok, i+1)
		}
	}
}
