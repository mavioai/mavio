package release

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// parseSeries parses a title and fails the test when it returns nil.
func parseSeries(t *testing.T, title string) *ParsedEpisodeInfo {
	t.Helper()
	p := ParseSeriesTitle(title)
	if p == nil {
		t.Fatalf("ParseSeriesTitle(%q): got = nil", title)
	}
	return p
}

// checkEpisode checks the fields most series theories assert.
func checkEpisode(t *testing.T, p *ParsedEpisodeInfo, title string, season int, episodes []int, fullSeason bool) {
	t.Helper()
	check(t, "SeriesTitle", p.SeriesTitle, title)
	check(t, "SeasonNumber", p.SeasonNumber, season)
	checkSame(t, "EpisodeNumbers", p.EpisodeNumbers, episodes)
	checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, nil)
	check(t, "FullSeason", p.FullSeason, fullSeason)
}

func TestSeriesAbsoluteEpisodeNumberCases(t *testing.T) {
	absolute := func(t *testing.T, p *ParsedEpisodeInfo, want int) {
		t.Helper()
		checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, []int{want})
	}
	portedCases(t, "series/absolute_episode_number_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_absolute_numbers": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			absolute(t, p, a.integer(t, "absoluteEpisodeNumber"))
			check(t, "SeasonNumber", p.SeasonNumber, a.integer(t, "seasonNumber"))
			episode := 0
			if len(p.EpisodeNumbers) == 1 {
				episode = p.EpisodeNumbers[0]
			} else if len(p.EpisodeNumbers) > 1 {
				t.Errorf("EpisodeNumbers: got = %v, want at most one", p.EpisodeNumbers)
			}
			check(t, "EpisodeNumber", episode, a.integer(t, "episodeNumber"))
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, false)
		},
		"should_parse_absolute_specials": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			absolute(t, p, a.integer(t, "absoluteEpisodeNumber"))
			check(t, "SeasonNumber", p.SeasonNumber, 0)
			if len(p.EpisodeNumbers) > 1 || len(p.EpisodeNumbers) == 1 && p.EpisodeNumbers[0] != 0 {
				t.Errorf("EpisodeNumbers: got = %v, want = [] or [0]", p.EpisodeNumbers)
			}
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, false)
			check(t, "Special", p.Special, true)
		},
		"should_parse_multi_episode_absolute_numbers": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers,
				intRange(a.integer(t, "firstAbsoluteEpisodeNumber"), a.integer(t, "lastAbsoluteEpisodeNumber")))
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, false)
		},
		"should_parse_anime_season_packs": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, nil)
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, true)
			check(t, "SeasonNumber", p.SeasonNumber, a.integer(t, "seasonNumber"))
		},
		"should_handle_anime_recap_numbering": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, nil)
			checkSame(t, "SpecialAbsoluteEpisodeNumbers", p.SpecialAbsoluteEpisodeNumbers, []float64{a.number(t, "specialEpisodeNumber")})
			check(t, "FullSeason", p.FullSeason, false)
		},
		"should_handle_ambiguously_named_anime_files_in_batch_release": func(t *testing.T, a args) {
			path := `C:\Test\` + a.str(t, "releaseName") + `\` + a.str(t, "filename")
			p := ParseSeriesPath(path)
			if p == nil {
				t.Fatalf("ParseSeriesPath(%q): got = nil", path)
			}
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, []int{a.integer(t, "absoluteEpisodeNumber")})
			check(t, "SeasonNumber", p.SeasonNumber, 0)
			checkSame(t, "EpisodeNumbers", p.EpisodeNumbers, nil)
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, false)
		},
		"should_parse_absolute_followed_by_standard_as_standard": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "releaseName"))
			checkEpisode(t, p, a.str(t, "title"), a.integer(t, "seasonNumber"), []int{a.integer(t, "episodeNumber")}, false)
			checkSame(t, "SpecialAbsoluteEpisodeNumbers", p.SpecialAbsoluteEpisodeNumbers, []float64{a.number(t, "specialEpisodeNumber")})
		},
	}})
}

func TestSeriesAnimeMetadataCases(t *testing.T) {
	portedCases(t, "series/anime_metadata_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_releasegroup_and_hash": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "subGroup"))
			check(t, "ReleaseHash", p.ReleaseHash, a.str(t, "hash"))
		},
		"should_parse_release_hashes_with_10b_or_8b": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			check(t, "ReleaseHash", p.ReleaseHash, a.str(t, "hash"))
		},
	}})
}

func checkVersion(kind Kind) func(*testing.T, args) {
	return func(t *testing.T, a args) {
		title := a.str(t, "title")
		check(t, "ParseQuality("+title+").Revision.Version", ParseQuality(title, kind).Revision.Version, a.integer(t, "version"))
	}
}

func TestSeriesAnimeVersionCases(t *testing.T) {
	portedCases(t, "series/anime_version.json", ported{run: map[string]func(*testing.T, args){
		"should_be_able_to_parse_repack": checkVersion(Series),
	}})
}

// notParsed returns a check that a title argument does not parse.
func notParsed(name string) func(*testing.T, args) {
	return func(t *testing.T, a args) {
		title := a.str(t, name)
		if p := ParseSeriesTitle(title); p != nil {
			t.Errorf("ParseSeriesTitle(%q): got = %+v, want = nil", title, p)
		}
	}
}

// randomTitles ports should_not_parse_random: none of 500 random
// alphanumeric strings of a length parses.
func randomTitles(t *testing.T, a args, parses func(string) bool) {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	length := a.integer(t, "length")
	r := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		b := make([]byte, length)
		for i := range b {
			b[i] = charset[r.IntN(len(charset))]
		}
		if parses(string(b)) {
			t.Errorf("%q parsed, want = nil", b)
		}
	}
}

// md5Titles ports should_not_parse_md5: none of 100 chained MD5 hashes
// parses.
func md5Titles(t *testing.T, parses func(string) bool) {
	hash := "CRAPPY TEST SEED"
	for range 100 {
		sum := md5.Sum([]byte(hash))
		hash = strings.ToUpper(hex.EncodeToString(sum[:]))
		if parses(hash) {
			t.Errorf("%q parsed, want = nil", hash)
		}
	}
}

func TestSeriesCrapCases(t *testing.T) {
	portedCases(t, "series/crap_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_not_parse_crap": notParsed("title"),
			"should_parse_unknown_formats_without_error": func(t *testing.T, a args) {
				parseSeries(t, a.str(t, "title"))
			},
			"should_not_parse_random": func(t *testing.T, a args) {
				randomTitles(t, a, func(s string) bool { return ParseSeriesTitle(s) != nil })
			},
			"should_not_parse_file_name_without_proper_spacing": notParsed("fileName"),
			"should_not_parse_invalid_release_name":             notParsed("fileName"),
			"should_not_parse_special_with_part_number":         notParsed("fileName"),
		},
		facts: map[string]string{"should_not_parse_md5": "TestSeriesNotParseMD5"},
	})
}

func TestSeriesNotParseMD5(t *testing.T) {
	md5Titles(t, func(s string) bool { return ParseSeriesTitle(s) != nil })
}

func TestSeriesDailyEpisodeCases(t *testing.T) {
	daily := func(t *testing.T, a args) *ParsedEpisodeInfo {
		p := parseSeries(t, a.str(t, "postTitle"))
		airDate := time.Date(a.integer(t, "year"), time.Month(a.integer(t, "month")), a.integer(t, "day"), 0, 0, 0, 0, time.UTC)
		check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
		check(t, "AirDate", p.AirDate, airDate.Format(time.DateOnly))
		checkSame(t, "EpisodeNumbers", p.EpisodeNumbers, nil)
		checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, nil)
		check(t, "FullSeason", p.FullSeason, false)
		return p
	}
	expand := func(title string, year int, month, day string) string {
		return strings.NewReplacer("{year}", fmt.Sprint(year), "{month}", month, "{day}", day).Replace(title)
	}
	portedCases(t, "series/daily_episode_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_parse_daily_episode": func(t *testing.T, a args) { daily(t, a) },
			"should_parse_daily_episode_with_multiple_parts": func(t *testing.T, a args) {
				p := daily(t, a)
				if p.DailyPart == nil || *p.DailyPart != a.integer(t, "part") {
					t.Errorf("DailyPart: got = %v, want = %d", p.DailyPart, a.integer(t, "part"))
				}
			},
			"should_not_accept_ancient_daily_series": func(t *testing.T, a args) {
				title := expand(a.str(t, "title"), 1950, "10", "14")
				if p := ParseSeriesTitle(title); p != nil {
					t.Errorf("ParseSeriesTitle(%q): got = %+v, want = nil", title, p)
				}
			},
			"should_not_accept_future_dates": func(t *testing.T, a args) {
				d := time.Now().AddDate(0, 0, 2)
				title := expand(a.str(t, "title"), d.Year(), fmt.Sprintf("%02d", d.Month()), fmt.Sprintf("%02d", d.Day()))
				if p := ParseSeriesTitle(title); p != nil {
					t.Errorf("ParseSeriesTitle(%q): got = %+v, want = nil", title, p)
				}
			},
			"should_not_parse_ambiguous_daily_episode": notParsed("postTitle"),
		},
		facts: map[string]string{"should_fail_if_episode_is_far_in_future": "TestSeriesFarFutureEpisode"},
	})
}

func TestSeriesFarFutureEpisode(t *testing.T) {
	title := time.Now().AddDate(0, 0, 2).Format("2006.01.02") + " - A Talk Show - HD TV.mkv"
	if p := ParseSeriesTitle(title); p != nil {
		t.Errorf("ParseSeriesTitle(%q): got = %+v, want = nil", title, p)
	}
}

func TestSeriesHashedReleaseCases(t *testing.T) {
	portedCases(t, "series/hashed_release.json", ported{run: map[string]func(*testing.T, args){
		"should_properly_parse_hashed_releases": func(t *testing.T, a args) {
			path := osAgnostic(t, a, "path")
			p := ParseSeriesPath(path)
			if p == nil {
				t.Fatalf("ParseSeriesPath(%q): got = nil", path)
			}
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "Quality", p.Quality.Quality, quality(t, a, "quality"))
			check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "releaseGroup"))
		},
	}})
}

func TestSeriesIsPossibleSpecialEpisodeCases(t *testing.T) {
	special := func(want bool) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			check(t, "IsPossibleSpecialEpisode", parseSeries(t, a.str(t, "title")).IsPossibleSpecialEpisode(), want)
		}
	}
	portedCases(t, "series/is_possible_special_episode.json", ported{
		run: map[string]func(*testing.T, args){
			"IsPossibleSpecialEpisode_should_be_true":                       special(true),
			"IsPossibleSpecialEpisode_should_be_true_if_e00_special":        special(true),
			"IsPossibleSpecialEpisode_should_be_false_for_Special_in_title": special(false),
		},
		facts: map[string]string{
			"should_not_treat_files_without_a_series_title_as_a_special": "TestIsPossibleSpecialEpisode",
			"should_return_true_when_episode_numbers_is_empty":           "TestIsPossibleSpecialEpisode",
		},
	})
}

func TestIsPossibleSpecialEpisode(t *testing.T) {
	p := ParsedEpisodeInfo{EpisodeNumbers: []int{7}, SeasonNumber: 1}
	check(t, "IsPossibleSpecialEpisode with an episode", p.IsPossibleSpecialEpisode(), false)
	p = ParsedEpisodeInfo{SeasonNumber: 1}
	check(t, "IsPossibleSpecialEpisode without episodes", p.IsPossibleSpecialEpisode(), true)
}

// isoLanguage returns a check of FindIsoLanguage; Unknown expects no
// result.
func isoLanguage(kind Kind, want Language) func(*testing.T, args) {
	return func(t *testing.T, a args) {
		code := a.str(t, "isoCode")
		got, ok := FindIsoLanguage(code, kind)
		if want == LanguageUnknown {
			if ok {
				t.Errorf("FindIsoLanguage(%q): got = %v, want none", code, got.Language)
			}
			return
		}
		if !ok || got.Language != want {
			t.Errorf("FindIsoLanguage(%q): got = %v (%v), want = %v", code, got.Language, ok, want)
		}
	}
}

func TestSeriesIsoLanguageCases(t *testing.T) {
	portedCases(t, "series/iso_languages.json", ported{run: map[string]func(*testing.T, args){
		"should_return_iso_language_for_English":           isoLanguage(Series, English),
		"unknown_or_invalid_code_should_return_null":       isoLanguage(Series, LanguageUnknown),
		"should_return_portuguese":                         isoLanguage(Series, Portuguese),
		"should_not_return_portuguese":                     isoLanguage(Series, LanguageUnknown),
		"should_lookup_cze_via_alternate_iso_code_mapping": isoLanguage(Series, Czech),
	}})
}

// languageChecks returns the language theories shared by both apps.
// languagesOf parses a postTitle for the should_parse_language_* theories.
func languageChecks(t *testing.T, name string, kind Kind, languagesOf map[string]func(t *testing.T, title string) []Language) map[string]func(*testing.T, args) {
	t.Helper()
	run := map[string]func(*testing.T, args){}
	f := loadCaseFile(t, name)
	for _, th := range f.Theories {
		if !strings.HasPrefix(th.Method, "should_parse_language_") {
			continue
		}
		parse, ok := languagesOf[th.Method]
		if !ok {
			parse = languagesOf[""]
		}
		want := LanguageUnknown
		if !strings.HasSuffix(th.Method, "_subbed") && !strings.HasSuffix(th.Method, "_multi") && !strings.HasSuffix(th.Method, "_french_english") {
			want = languageOfMethod(t, th.Method)
		}
		run[th.Method] = func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			got := parse(t, title)
			if !contains(got, want) {
				t.Errorf("languages of %q: got = %v, want to contain %v", title, got, want)
			}
		}
	}
	subtitle := func(want Language) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			name := a.str(t, "fileName")
			check(t, "ParseSubtitleLanguage("+name+")", ParseSubtitleLanguage(name, kind), want)
		}
	}
	run["should_parse_subtitle_language_unknown"] = subtitle(LanguageUnknown)
	run["should_parse_subtitle_language_english"] = subtitle(English)
	run["should_parse_subtitle_language"] = subtitle(English)
	run["should_parse_title_and_tags"] = func(t *testing.T, a args) {
		info := ParseSubtitleLanguageInformation(a.str(t, "postTitle"), kind)
		checkSame(t, "LanguageTags", info.LanguageTags, a.strs(t, "expectedTags"))
		title := ""
		if info.Title != nil {
			title = *info.Title
		}
		check(t, "Title", strings.ToLower(title), strings.ToLower(a.str(t, "expectedTitle")))
		check(t, "Language", info.Language, languageNamed(t, a.str(t, "expectedLanguage")))
	}
	run["should_not_parse_false_title"] = func(t *testing.T, a args) {
		info := ParseSubtitleLanguageInformation(a.str(t, "postTitle"), kind)
		check(t, "Language", info.Language, LanguageUnknown)
		checkSame(t, "LanguageTags", info.LanguageTags, nil)
		if info.RawTitle != nil {
			t.Errorf("RawTitle: got = %q, want = nil", *info.RawTitle)
		}
	}
	// counted checks that the languages of a release are exactly want.
	counted := func(languagesOf func(t *testing.T, title string) []Language, want ...Language) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			checkSame(t, "languages of "+title, languagesOf(t, title), want)
		}
	}
	release := languagesOf["release"]
	run["should_add_original_language_to_german_release_with_dl_tag"] = counted(release, German, Original)
	run["should_not_add_original_language_to_german_release_when_title_contains_web_dl"] = counted(release, German)
	run["should_add_original_language_and_english_to_german_release_with_ml_tag"] = counted(release, German, Original, English)
	run["should_parse_original_title_from_release_name"] = counted(release, Original)
	return run
}

func loadCaseFile(t *testing.T, name string) caseFile {
	t.Helper()
	f, err := readCaseFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestSeriesLanguageCases(t *testing.T) {
	languages := func(_ *testing.T, title string) []Language { return ParseLanguages(title, Series) }
	release := func(t *testing.T, title string) []Language { return parseSeries(t, title).Languages }
	run := languageChecks(t, "series/language_parser.json", Series, map[string]func(*testing.T, string) []Language{
		"": languages, "release": release,
	})
	exactly := func(want ...Language) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			checkSame(t, "ParseLanguages("+title+")", ParseLanguages(title, Series), want)
		}
	}
	run["should_parse_hindi_and_english"] = exactly(Hindi, English)
	run["should_parse_spanish_and_catalan"] = exactly(Spanish, Catalan)
	run["should_parse_english_spanish_and_catalan"] = exactly(English, Spanish, Catalan)
	releaseLanguages := func(want ...Language) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			checkSame(t, "languages of "+title, release(t, title), want)
		}
	}
	run["should_parse_english_and_ukranian"] = releaseLanguages(Ukrainian, English) //nolint:misspell // upstream test name
	run["should_parse_turkish_and_ukranian"] = releaseLanguages(Ukrainian, Turkish) //nolint:misspell // upstream test name
	portedCases(t, "series/language_parser.json", ported{run: run})
}

func TestSeriesMiniSeriesCases(t *testing.T) {
	mini := func(t *testing.T, a args) {
		p := parseSeries(t, a.str(t, "postTitle"))
		checkEpisode(t, p, a.str(t, "title"), 1, []int{a.integer(t, "episodeNumber")}, false)
	}
	portedCases(t, "series/mini_series_episode_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_mini_series_episode":   mini,
		"should_parse_korean_series_episode": mini,
	}})
}

func TestSeriesMultiEpisodeCases(t *testing.T) {
	portedCases(t, "series/multi_episode_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_multiple_episodes": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			checkEpisode(t, p, a.str(t, "title"), a.integer(t, "season"), a.ints(t, "episodes"), false)
		},
	}})
}

func TestNormalizeEpisodeTitleCases(t *testing.T) {
	portedCases(t, "series/normalize_episode_title.json", ported{run: map[string]func(*testing.T, args){
		"should_normalize_episode_title": func(t *testing.T, a args) {
			in := a.str(t, "input")
			check(t, "NormalizeEpisodeTitle("+in+")", NormalizeEpisodeTitle(in), a.str(t, "expected"))
		},
	}})
}

// cleanTitleChecks returns the theories shared by Sonarr's
// NormalizeSeriesTitleFixture and Radarr's NormalizeTitleFixture.
func cleanTitleChecks(clean func(string) string) map[string]func(*testing.T, args) {
	checkClean := func(in, want string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			check(t, "clean("+in+")", clean(in), want)
		}
	}
	formats := func(formats []string, want func(word string) string) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			word := a.str(t, "word")
			for _, f := range formats {
				checkClean(strings.ReplaceAll(f, "{0}", word), want(word))(t)
			}
		}
	}
	pair := func(in, out string) func(*testing.T, args) {
		return func(t *testing.T, a args) { checkClean(a.str(t, in), a.str(t, out))(t) }
	}
	return map[string]func(*testing.T, args){
		"should_normalize_series_title":                              pair("parsedSeriesName", "seriesName"),
		"should_not_remove_from_the_beginning_of_the_title":          pair("parsedSeriesName", "seriesName"),
		"should_remove_special_characters_and_casing":                pair("dirty", "clean"),
		"should_replace_percent_sign_with_percent_following_numbers": pair("input", "expected"),
		"should_remove_common_words_from_middle_of_title": formats(
			[]string{"word.{0}.word", "word {0} word", "word-{0}-word"},
			func(string) string { return "wordword" }),
		"should_not_remove_common_words_from_end_of_title": formats(
			[]string{"word.word.{0}", "word-word-{0}", "word-word {0}"},
			func(w string) string { return "wordword" + strings.ToLower(w) }),
		"should_not_remove_common_words_in_the_middle_of_word": formats(
			[]string{"word.{0}word", "word {0}word", "word-{0}word", "word{0}.word", "word{0}-word", "word{0}-word"},
			func(w string) string { return "word" + strings.ToLower(w) + "word" }),
		"should_not_clean_word_from_beginning_of_string": formats(
			[]string{"{0}.word.word", "{0}-word-word", "{0} word word"},
			func(w string) string { return w + "wordword" }),
	}
}

func TestCleanSeriesTitleCases(t *testing.T) {
	portedCases(t, "series/normalize_series_title.json", ported{
		run: cleanTitleChecks(CleanSeriesTitle),
		facts: map[string]string{
			"should_remove_a_from_middle_of_title": "TestCleanSeriesTitleArticleA",
			"should_not_clean_trailing_a":          "TestCleanSeriesTitleArticleA",
		},
	})
}

func TestCleanSeriesTitleArticleA(t *testing.T) {
	for _, in := range []string{"word.a.word", "word a word", "word-a-word"} {
		check(t, "CleanSeriesTitle("+in+")", CleanSeriesTitle(in), "wordword")
	}
	check(t, "CleanSeriesTitle(Series Title A)", CleanSeriesTitle("Series Title A"), "seriestitlea")
}

func TestSeriesParserCases(t *testing.T) {
	portedCases(t, "series/parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_parse_series_name": func(t *testing.T, a args) {
				title := a.str(t, "postTitle")
				check(t, "ParseSeriesName("+title+")", CleanSeriesTitle(ParseSeriesName(title)), CleanSeriesTitle(a.str(t, "title")))
			},
			"should_parse_series_title_info": func(t *testing.T, a args) {
				info := parseSeries(t, a.str(t, "postTitle")).SeriesTitleInfo
				check(t, "TitleWithoutYear", info.TitleWithoutYear, a.str(t, "titleWithoutYear"))
				year := 0
				if a.has("year") {
					year = a.integer(t, "year")
				}
				check(t, "Year", info.Year, year)
			},
			"should_clean_up_invalid_path_characters": func(t *testing.T, a args) {
				ParseSeriesTitle(a.str(t, "postTitle"))
			},
			"should_remove_request_info_from_title": func(t *testing.T, a args) {
				check(t, "SeriesTitle", parseSeries(t, a.str(t, "postTitle")).SeriesTitle, a.str(t, "title"))
			},
			"should_parse_quality_from_extension": func(t *testing.T, a args) {
				q := parseSeries(t, a.str(t, "title")).Quality
				if q.Quality == Unknown {
					t.Errorf("Quality: got = Unknown")
				}
				check(t, "SourceDetection", q.SourceDetection, DetectedExtension)
				check(t, "ResolutionDetection", q.ResolutionDetection, DetectedExtension)
			},
			"should_parse_releasetitle": func(t *testing.T, a args) {
				check(t, "ReleaseTitle", parseSeries(t, a.str(t, "path")).ReleaseTitle, a.str(t, "releaseTitle"))
			},
			"should_parse_multiple_series_titles": func(t *testing.T, a args) {
				checkSame(t, "AllTitles", parseSeries(t, a.str(t, "postTitle")).SeriesTitleInfo.AllTitles, a.strs(t, "titles"))
			},
			"should_parse_language_after_parsing_title": func(t *testing.T, a args) {
				checkSame(t, "Languages", parseSeries(t, a.str(t, "postTitle")).Languages,
					[]Language{languageNamed(t, a.str(t, "expectedLanguage"))})
			},
		},
		facts: map[string]string{"should_remove_accents_from_title": "TestCleanSeriesTitleAccents"},
	})
}

func TestCleanSeriesTitleAccents(t *testing.T) {
	check(t, "CleanSeriesTitle", CleanSeriesTitle("Seri\u00E0es"), "seriaes")
}

func TestSeriesPathCases(t *testing.T) {
	parsePath := func(t *testing.T, a args) *ParsedEpisodeInfo {
		path := osAgnostic(t, a, "path")
		p := ParseSeriesPath(path)
		if p == nil {
			t.Fatalf("ParseSeriesPath(%q): got = nil", path)
		}
		return p
	}
	portedCases(t, "series/path_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_from_path": func(t *testing.T, a args) {
			p := parsePath(t, a)
			check(t, "SeasonNumber", p.SeasonNumber, a.integer(t, "season"))
			checkSame(t, "EpisodeNumbers", p.EpisodeNumbers, []int{a.integer(t, "episode")})
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, nil)
			check(t, "FullSeason", p.FullSeason, false)
		},
		"should_parse_multi_episode_from_path": func(t *testing.T, a args) {
			checkEpisode(t, parsePath(t, a), a.str(t, "title"), a.integer(t, "season"), a.ints(t, "episodes"), false)
		},
	}})
}

// releaseGroup returns a check of ParseReleaseGroup; a null expectation is
// "".
func releaseGroup(kind Kind) func(*testing.T, args) {
	return func(t *testing.T, a args) {
		title := a.str(t, "title")
		check(t, "ParseReleaseGroup("+title+")", ParseReleaseGroup(title, kind), a.str(t, "expected"))
	}
}

func TestSeriesReleaseGroupCases(t *testing.T) {
	portedCases(t, "series/release_group_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_parse_release_group":                   releaseGroup(Series),
			"should_parse_exception_release_group":         releaseGroup(Series),
			"should_not_include_language_in_release_group": releaseGroup(Series),
			"should_not_include_repost_in_release_group":   releaseGroup(Series),
			"should_parse_anime_release_groups":            releaseGroup(Series),
			"should_not_parse_anime_hash_as_release_group": releaseGroup(Series),
		},
		facts: map[string]string{"should_not_include_extension_in_release_group": "TestSeriesReleaseGroupOfPath"},
	})
}

func TestSeriesReleaseGroupOfPath(t *testing.T) {
	p := ParseSeriesPath(`C:\Test\Doctor.Series.2005.s01e01.internal.bdrip.x264-archivist.mkv`)
	if p == nil {
		t.Fatal("ParseSeriesPath: got = nil")
	}
	check(t, "ReleaseGroup", p.ReleaseGroup, "archivist")
}

func TestSeriesSceneCheckerCases(t *testing.T) {
	scene := func(want bool) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			title := a.str(t, "title")
			check(t, "IsSceneTitle("+title+")", IsSceneTitle(title, Series), want)
		}
	}
	portedCases(t, "series/scene_checker.json", ported{run: map[string]func(*testing.T, args){
		"should_return_true_for_scene_names":      scene(true),
		"should_return_false_for_non_scene_names": scene(false),
	}})
}

func TestSeriesSeasonCases(t *testing.T) {
	season := func(t *testing.T, a args, fullSeason bool) *ParsedEpisodeInfo {
		p := parseSeries(t, a.str(t, "postTitle"))
		want := 0
		if a.has("season") {
			want = a.integer(t, "season")
		} else {
			want = a.integer(t, "firstSeason")
		}
		checkEpisode(t, p, a.str(t, "title"), want, nil, fullSeason)
		return p
	}
	extra := func(t *testing.T, a args) {
		check(t, "IsSeasonExtra", season(t, a, true).IsSeasonExtra, true)
	}
	portedCases(t, "series/season_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"should_parse_full_season_release": func(t *testing.T, a args) { season(t, a, true) },
			"should_parse_season_extras":       extra,
			"should_parse_season_subpack":      extra,
			"should_parse_partial_season_release": func(t *testing.T, a args) {
				p := season(t, a, false)
				check(t, "IsPartialSeason", p.IsPartialSeason, true)
				check(t, "SeasonPart", p.SeasonPart, a.integer(t, "seasonPart"))
			},
			"should_parse_multi_season_release": func(t *testing.T, a args) {
				p := season(t, a, true)
				check(t, "IsPartialSeason", p.IsPartialSeason, false)
				check(t, "IsMultiSeason", p.IsMultiSeason, true)
			},
		},
		facts: map[string]string{"should_not_parse_season_folders": "TestSeriesSeasonFolder"},
	})
}

func TestSeriesSeasonFolder(t *testing.T) {
	if p := ParseSeriesTitle("Season 3"); p != nil {
		t.Errorf("ParseSeriesTitle(Season 3): got = %+v, want = nil", p)
	}
}

func TestSeriesSingleEpisodeCases(t *testing.T) {
	single := func(t *testing.T, a args) *ParsedEpisodeInfo {
		p := parseSeries(t, a.str(t, "postTitle"))
		checkEpisode(t, p, a.str(t, "title"), a.integer(t, "seasonNumber"), []int{a.integer(t, "episodeNumber")}, false)
		return p
	}
	portedCases(t, "series/single_episode_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_single_episode":               func(t *testing.T, a args) { single(t, a) },
		"should_parse_japanese_variety_show_format": func(t *testing.T, a args) { single(t, a) },
		"should_parse_decimal_number_as_special": func(t *testing.T, a args) {
			check(t, "Special", single(t, a).Special, true)
		},
		"should_parse_split_episode": func(t *testing.T, a args) {
			check(t, "IsSplitEpisode", single(t, a).IsSplitEpisode, true)
		},
	}})
}

// replaceUnicode ports XmlCleaner.ReplaceUnicode, which drops characters
// that are invalid in XML.
func replaceUnicode(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD {
			return r
		}
		return -1
	}, s)
}

func TestSeriesUnicodeReleaseCases(t *testing.T) {
	anime := func(t *testing.T, a args) {
		p := parseSeries(t, replaceUnicode(a.str(t, "postTitle")))
		check(t, "ReleaseGroup", p.ReleaseGroup, a.str(t, "subgroup"))
		var want []int
		if a.has("absoluteEpisodeNumbers") {
			want = a.ints(t, "absoluteEpisodeNumbers")
		} else {
			want = []int{a.integer(t, "absoluteEpisodeNumber")}
		}
		checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, want)
		check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
		check(t, "FullSeason", p.FullSeason, false)
	}
	portedCases(t, "series/unicode_release_parser.json", ported{run: map[string]func(*testing.T, args){
		"should_parse_chinese_anime_releases":                anime,
		"should_parse_chinese_anime_season_episode_releases": anime,
		"should_parse_unbracketed_chinese_anime_releases":    anime,
		"should_parse_chinese_multiepisode_releases":         anime,
		"should_parse_false_positive_chinese_anime_releases": anime,
		"should_parse_gm_team_releases_and_files": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, []int{a.integer(t, "absoluteEpisodeNumber")})
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			check(t, "FullSeason", p.FullSeason, false)
			check(t, "ReleaseGroup", p.ReleaseGroup, "GM-Team")
		},
		"should_parse_unicode_digits": func(t *testing.T, a args) {
			p := parseSeries(t, a.str(t, "postTitle"))
			check(t, "SeriesTitle", p.SeriesTitle, a.str(t, "title"))
			checkSame(t, "AbsoluteEpisodeNumbers", p.AbsoluteEpisodeNumbers, []int{a.integer(t, "absoluteEpisodeNumber")})
			checkSame(t, "SpecialAbsoluteEpisodeNumbers", p.SpecialAbsoluteEpisodeNumbers, nil)
			check(t, "FullSeason", p.FullSeason, false)
		},
	}})
}

func TestSeriesURLCases(t *testing.T) {
	portedCases(t, "series/url.json", ported{run: map[string]func(*testing.T, args){
		"should_not_parse_url_in_name": func(t *testing.T, a args) {
			title := a.str(t, "postTitle")
			check(t, "ParseSeriesName("+title+")", CleanSeriesTitle(ParseSeriesName(title)), CleanSeriesTitle(a.str(t, "title")))
		},
		"should_not_parse_url_in_group": releaseGroup(Series),
	}})
}

func TestSeriesTitleInfoCases(t *testing.T) {
	portedCases(t, "series/series_title_info.json", ported{facts: map[string]string{
		"should_have_year_zero_when_title_doesnt_have_a_year":                                   "TestSeriesTitleInfo",
		"should_have_same_title_for_title_and_title_without_year_when_title_doesnt_have_a_year": "TestSeriesTitleInfo",
		"should_have_year_when_title_has_a_year":                                                "TestSeriesTitleInfo",
		"should_have_year_in_title_when_title_has_a_year":                                       "TestSeriesTitleInfo",
		"should_title_without_year_should_not_contain_year":                                     "TestSeriesTitleInfo",
	}})
}

func TestSeriesTitleInfo(t *testing.T) {
	info := parseSeries(t, "Series.Title.S01E01.pilot.720p.hdtv").SeriesTitleInfo
	check(t, "Year", info.Year, 0)
	check(t, "Title", info.Title, info.TitleWithoutYear)
	info = parseSeries(t, "Series.Title.2004.S01E01.pilot.720p.hdtv").SeriesTitleInfo
	check(t, "Year", info.Year, 2004)
	check(t, "Title", info.Title, "Series Title 2004")
	check(t, "TitleWithoutYear", info.TitleWithoutYear, "Series Title")
}

func TestValidateParsedEpisodeInfoCases(t *testing.T) {
	const reason = "ValidateForSeriesType checks a release against a series' type, which libs/acquire does"
	portedCases(t, "series/validate_parsed_episode_info.json", ported{skip: map[string]string{
		"should_return_true_if_episode_info_is_not_daily":                  reason,
		"should_return_true_if_episode_info_is_daily_for_daily_series":     reason,
		"should_return_false_if_episode_info_is_daily_for_standard_series": reason,
		"should_not_log_warning_if_warnIfInvalid_is_false":                 reason,
	}})
}
