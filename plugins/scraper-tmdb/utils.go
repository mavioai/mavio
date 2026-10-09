package main

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// TMDB naming conventions and matching, ported from Jellyfin's TmdbUtils.

const originalImageSize = "original"

// Scores of FindBestMatch: a title outweighs a year.
const (
	titleExactScore   = 8
	titlePrefixScore  = 4
	yearExactScore    = 2
	yearAdjacentScore = 1
)

var (
	nonSearchTerm = regexp.MustCompile(`[^\p{L}\p{N}\p{M}·]+`)
	nonComparable = regexp.MustCompile(`[^\p{L}\p{N}\p{M}]+`)
)

// parseTMDBID parses a TMDB ID, a positive decimal number; another
// provider's ID filed under the TMDB key, such as an IMDb ID, is not one.
func parseTMDBID(value string) (int, bool) {
	if value == "" || strings.TrimLeft(value, "0123456789") != "" {
		return 0, false
	}
	id, err := strconv.Atoi(value)
	return id, err == nil && id > 0
}

// cleanName turns a name into the space-separated words TMDB searches for.
func cleanName(name string) string {
	return strings.TrimSpace(nonSearchTerm.ReplaceAllString(name, " "))
}

// normalizeTitle reduces a title to lower-case words for comparison, so
// that "WALL-E" and "WALL·E" are equal.
func normalizeTitle(title string) string {
	if title == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(nonComparable.ReplaceAllString(title, " ")))
}

// candidate is a search result as matching sees it.
type candidate struct {
	title, originalTitle string
	date                 string // "2006-01-02"
}

// findBestMatch picks the search result whose title, or original title,
// and year fit best. TMDB's year filter does not exclude results, so a
// remake and its original both come back; ties keep TMDB's order. It
// returns -1 when there are no results.
func findBestMatch(results []candidate, name string, year int) int {
	if len(results) == 0 {
		return -1
	}
	normalized := normalizeTitle(name)
	if normalized == "" {
		return 0
	}
	best, bestScore := 0, 0
	for i, r := range results {
		score := max(scoreTitle(normalized, r.title), scoreTitle(normalized, r.originalTitle)) + scoreYear(year, r.date)
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

func scoreTitle(name, title string) int {
	t := normalizeTitle(title)
	if t == name {
		return titleExactScore
	}
	// Whole words only, otherwise "Wall" half matches "Wall Street".
	if len(t) > len(name) && t[len(name)] == ' ' && strings.HasPrefix(t, name) {
		return titlePrefixScore
	}
	return 0
}

func scoreYear(year int, date string) int {
	ry, ok := yearOf(date)
	if year <= 0 || !ok {
		return 0
	}
	switch ry - year {
	case 0:
		return yearExactScore
	case 1, -1:
		// Regional release dates straddle a new year.
		return yearAdjacentScore
	}
	return 0
}

func yearOf(date string) (int, bool) {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return 0, false
	}
	return t.Year(), true
}

// normalizeLanguage writes a language as TMDB wants it: the region in
// upper case, Latin American Spanish as the requested country's variant,
// and Swiss variants, which TMDB lacks, as the bare language.
func normalizeLanguage(language, country string) string {
	if language == "" {
		return language
	}
	if strings.EqualFold(language, "es-419") && country != "" {
		language = "es-MX"
		if strings.EqualFold(country, "AR") {
			language = "es-AR"
		}
	}
	parts := strings.Split(language, "-")
	if len(parts) == 2 {
		if strings.EqualFold(parts[1], "CH") {
			return parts[0]
		}
		language = parts[0] + "-" + strings.ToUpper(parts[1])
	}
	return language
}

// imageLanguages is the include_image_language parameter: the preferred
// language, images without text, and English as fallback.
func imageLanguages(preferred, country string) string {
	var langs []string
	if preferred != "" {
		preferred = normalizeLanguage(preferred, country)
		langs = append(langs, preferred)
	}
	langs = append(langs, "null")
	if !strings.EqualFold(preferred, "en") {
		langs = append(langs, "en")
	}
	return strings.Join(langs, ",")
}

// imageLanguage returns the language of an image as the request names it:
// the requested regional variant when the image's region matches or is
// unknown, the image's own region otherwise, and "" for images without
// text ("xx").
func imageLanguage(imageLang, imageRegion, requestLang string) string {
	if imageLang == "" || strings.EqualFold(imageLang, "xx") {
		return ""
	}
	if requestLang == "" {
		return imageLang
	}
	parts := strings.Split(requestLang, "-")
	if len(parts) != 2 || !strings.EqualFold(parts[0], imageLang) {
		return imageLang
	}
	if imageRegion == "" || strings.EqualFold(imageRegion, parts[1]) {
		return requestLang
	}
	return imageLang + "-" + strings.ToUpper(imageRegion)
}

func isOriginalImageSize(size string) bool {
	return size == "" || strings.EqualFold(size, originalImageSize)
}

// parentalRating qualifies a certification by country, as "FSK-12" for
// Germany; US ratings stay bare.
func parentalRating(country, rating string) string {
	prefix := ""
	if !strings.EqualFold(country, "US") {
		prefix = country + "-"
	}
	r := prefix + rating
	if len(r) >= 3 && strings.EqualFold(r[:3], "DE-") {
		r = "FSK-" + r[3:]
	}
	return r
}

// crewKind maps a crew job to the credits Mavio keeps: directors,
// producers and writers.
func crewKind(department, job string) string {
	switch {
	case strings.EqualFold(department, "directing") && strings.EqualFold(job, "director"):
		return "director"
	case strings.EqualFold(department, "production") && strings.EqualFold(job, "producer"):
		return "producer"
	case strings.EqualFold(department, "writing") && slices.ContainsFunc([]string{"writer", "screenplay", "novel"}, func(j string) bool {
		return strings.EqualFold(j, job)
	}):
		return "writer"
	}
	return ""
}

// credit is an actor's credit before conversion to the contract.
type credit struct {
	name, role string
	id         int
	order      int
	profile    string
}

// castConfig limits the cast taken over.
type castConfig struct {
	maxCast     int
	hideMissing bool // drop actors without a profile picture
}

func (c castConfig) keep(name, profile string) bool {
	return strings.TrimSpace(name) != "" && (!c.hideMissing || profile != "")
}

// mapCast takes the top-billed actors, one credit each.
func mapCast(cast []tmdbCast, c castConfig) []credit {
	billed := slices.DeleteFunc(slices.Clone(cast), func(m tmdbCast) bool { return !c.keep(m.Name, m.ProfilePath) })
	slices.SortStableFunc(billed, func(a, b tmdbCast) int { return cmp.Compare(a.Order, b.Order) })
	var out []credit
	for _, m := range billed[:min(len(billed), c.maxCast)] {
		out = append(out, credit{name: strings.TrimSpace(m.Name), role: strings.TrimSpace(m.Character), id: m.ID, order: m.Order, profile: m.ProfilePath})
	}
	return out
}

// mapAggregateCast takes the top-billed actors of a series, one credit per
// character they played, the one they played longest first.
func mapAggregateCast(cast []tmdbAggregateCast, c castConfig) []credit {
	billed := slices.DeleteFunc(slices.Clone(cast), func(m tmdbAggregateCast) bool { return !c.keep(m.Name, m.ProfilePath) })
	slices.SortStableFunc(billed, func(a, b tmdbAggregateCast) int { return cmp.Compare(a.Order, b.Order) })
	var out []credit
	for _, m := range billed[:min(len(billed), c.maxCast)] {
		roles := slices.DeleteFunc(slices.Clone(m.Roles), func(r tmdbRole) bool { return strings.TrimSpace(r.Character) == "" })
		slices.SortStableFunc(roles, func(a, b tmdbRole) int { return cmp.Compare(b.EpisodeCount, a.EpisodeCount) })
		characters := []string{""}
		if len(roles) > 0 {
			characters = characters[:0]
			for _, r := range roles {
				characters = append(characters, strings.TrimSpace(r.Character))
			}
		}
		for _, ch := range characters {
			out = append(out, credit{name: strings.TrimSpace(m.Name), role: ch, id: m.ID, order: m.Order, profile: m.ProfilePath})
		}
	}
	return out
}
