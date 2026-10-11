package acquire

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// MovieRef is a movie releases are matched to.
type MovieRef struct {
	ID            core.ID
	Title         string
	OriginalTitle string
	Year          int
	// SecondaryYear is another year releases may give the movie; 0 for
	// none.
	SecondaryYear int
	// AlternativeTitles and TranslatedTitles are the movie's other titles
	// and its titles in other languages.
	AlternativeTitles, TranslatedTitles []string
	// ImdbID is as "tt0111161"; empty when unknown.
	ImdbID string
	// TmdbID is 0 when unknown.
	TmdbID int
}

// hasYear reports whether year is the movie's year or secondary year.
func (m *MovieRef) hasYear(year int) bool {
	return m.Year == year || m.SecondaryYear != 0 && m.SecondaryYear == year
}

// ErrMultipleMovies is returned when a release's titles match more than
// one movie, as Radarr's MultipleMoviesFoundException.
var ErrMultipleMovies = errors.New("multiple movies match")

// arabicRoman is a number from 1 to 20 as Radarr's RomanNumeralParser
// maps them: in Arabic numerals, in upper case and in lower case Roman
// numerals.
type arabicRoman struct{ arabic, roman, romanLower string }

var arabicRomans = func() []arabicRoman {
	out := make([]arabicRoman, 20)
	for i := range out {
		r := release.RomanNumeral(i + 1)
		out[i] = arabicRoman{strconv.Itoa(i + 1), r, strings.ToLower(r)}
	}
	return out
}()

// MatchMovie finds the movie a parsed release is for, as Radarr's
// ParsingService.FindMovie: by the IMDb ID, then the TMDB ID the indexer
// gave, when the parsed year agrees; then, for a search, by the searched
// movie's titles or IDs; else by title and year among movies. It returns
// nil when nothing matches, and ErrMultipleMovies when titles match more
// than one movie. searched is nil for RSS syncs.
func MatchMovie(parsed *release.ParsedMovieInfo, imdbID string, tmdbID int, searched *MovieRef, movies []MovieRef) (*MovieRef, error) {
	imdbID = release.NormalizeImdbID(imdbID)
	if imdbID != "" {
		if i := slices.IndexFunc(movies, func(m MovieRef) bool { return m.ImdbID != "" && release.NormalizeImdbID(m.ImdbID) == imdbID }); i >= 0 && yearMatches(parsed.Year, &movies[i]) {
			return &movies[i], nil
		}
	}
	if tmdbID > 0 {
		if i := slices.IndexFunc(movies, func(m MovieRef) bool { return m.TmdbID == tmdbID }); i >= 0 && yearMatches(parsed.Year, &movies[i]) {
			return &movies[i], nil
		}
	}
	if searched != nil {
		return matchSearched(parsed, imdbID, tmdbID, searched), nil
	}
	return matchTitle(parsed, movies)
}

// yearMatches reports whether a parsed year, which is below 1800 when
// unknown, is the movie's year or secondary year.
func yearMatches(year int, m *MovieRef) bool { return year < 1800 || m.hasYear(year) }

// cleanTitles returns the clean titles of titles.
func cleanTitles(titles []string) []string {
	out := make([]string, 0, len(titles))
	for _, t := range titles {
		out = append(out, release.CleanMovieTitle(t))
	}
	return out
}

// matchSearched matches a release to the movie searched for, as Radarr's
// TryGetMovieBySearchCriteria: by its title, original title, alternative
// titles or translations.
func matchSearched(parsed *release.ParsedMovieInfo, imdbID string, tmdbID int, m *MovieRef) *MovieRef {
	possible := []string{release.CleanMovieTitle(m.Title)}
	if m.OriginalTitle != "" {
		possible = append(possible, release.CleanMovieTitle(m.OriginalTitle))
	}
	possible = append(possible, cleanTitles(m.AlternativeTitles)...)
	possible = append(possible, cleanTitles(m.TranslatedTitles)...)
	clean := make([]string, len(parsed.MovieTitles))
	for i, t := range parsed.MovieTitles {
		clean[i] = release.CleanMovieTitle(t)
	}
	matches := slices.ContainsFunc(possible, func(pt string) bool {
		return slices.Contains(clean, pt) || slices.ContainsFunc(arabicRomans, func(n arabicRoman) bool {
			return slices.Contains(clean, strings.ReplaceAll(pt, n.arabic, n.romanLower)) ||
				slices.ContainsFunc(clean, func(t string) bool { return strings.ReplaceAll(t, n.arabic, n.romanLower) == pt })
		})
	})
	switch {
	case matches && yearMatches(parsed.Year, m):
		return m
	case tmdbID > 0 && tmdbID == m.TmdbID:
		return m
	case imdbID != "" && imdbID == release.NormalizeImdbID(m.ImdbID):
		return m
	}
	return nil
}

// matchTitle matches a release among movies by its titles and year, as
// Radarr's TryGetMovieByTitleAndOrYear and MovieService.FindByTitle: by
// a movie's title or original title, then by its title with numbers
// written in Roman or Arabic numerals, then by its alternative titles,
// then by its translations; in the year or secondary year parsed.
func matchTitle(parsed *release.ParsedMovieInfo, movies []MovieRef) (*MovieRef, error) {
	var clean, other []string
	for _, t := range parsed.MovieTitles {
		c := strings.ToLower(release.CleanMovieTitle(t))
		roman, arabic := c, c
		for _, n := range arabicRomans {
			roman = strings.ReplaceAll(roman, n.arabic, n.roman)
			arabic = strings.ReplaceAll(arabic, n.roman, n.arabic)
		}
		clean = append(clean, c)
		other = append(other, arabic, strings.ToLower(roman))
	}
	year := 0
	if parsed.Year > 1800 {
		year = parsed.Year
	}
	withYear := func(match func(m *MovieRef) bool) []*MovieRef {
		var out []*MovieRef
		for i := range movies {
			m := &movies[i]
			if match(m) && (year == 0 || m.hasYear(year)) {
				out = append(out, m)
			}
		}
		return out
	}
	found := withYear(func(m *MovieRef) bool {
		return slices.Contains(clean, release.CleanMovieTitle(m.Title)) ||
			m.OriginalTitle != "" && slices.Contains(clean, release.CleanMovieTitle(m.OriginalTitle))
	})
	if len(found) == 0 {
		found = withYear(func(m *MovieRef) bool { return slices.Contains(other, release.CleanMovieTitle(m.Title)) })
	}
	anyOf := func(titles []string) bool {
		return slices.ContainsFunc(cleanTitles(titles), func(t string) bool { return slices.Contains(clean, t) || slices.Contains(other, t) })
	}
	if len(found) == 0 {
		found = withYear(func(m *MovieRef) bool { return anyOf(m.AlternativeTitles) })
	}
	if len(found) == 0 {
		found = withYear(func(m *MovieRef) bool { return anyOf(m.TranslatedTitles) })
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("%w: %d movies match %q", ErrMultipleMovies, len(found), parsed.PrimaryMovieTitle())
}
