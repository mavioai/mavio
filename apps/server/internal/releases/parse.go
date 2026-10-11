package releases

import (
	"slices"

	"github.com/mavioai/mavio/libs/release"
)

// parse parses a release's title as a movie or an episode by kind; without
// one, by its categories, and else as an episode when it numbers episodes
// or a season by other than a movie's year, which Sonarr's rules read as
// a season and an episode.
func parse(r Release, kind release.Kind) Release {
	title := r.GetTitle()
	if kind == 0 {
		switch {
		case slices.ContainsFunc(r.GetCategories(), func(c int32) bool { return c/1000 == 5 }):
			kind = release.Series
		case slices.ContainsFunc(r.GetCategories(), func(c int32) bool { return c/1000 == 2 }):
			kind = release.Movie
		}
	}
	switch kind {
	case release.Movie:
		r.Movie = release.ParseMovieTitle(title, false)
		return r
	case release.Series:
		r.Episode = release.ParseSeriesTitle(title)
		return r
	}
	movie := release.ParseMovieTitle(title, false)
	if e := release.ParseSeriesTitle(title); e != nil && numbered(e) && (movie == nil || !fromYear(e, movie.Year)) {
		r.Episode = e
		return r
	}
	r.Movie = movie
	return r
}

// numbered reports whether an episode release names its episodes or
// season.
func numbered(e *release.ParsedEpisodeInfo) bool {
	return len(e.EpisodeNumbers) > 0 || len(e.AbsoluteEpisodeNumbers) > 0 || e.IsDaily() || e.FullSeason
}

// fromYear reports whether an episode release's season and episode are a
// year, as "Name.2010" reads as season 20, episode 10.
func fromYear(e *release.ParsedEpisodeInfo, year int) bool {
	return year > 0 && len(e.EpisodeNumbers) == 1 && e.SeasonNumber*100+e.EpisodeNumbers[0] == year
}
