package subtitles

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/release"
)

// Guess is what a release or file name says of its video, as the guessit
// properties Bazarr matches; zero values are absent.
type Guess struct {
	// Titles are the movie's or the series' titles.
	Titles []string
	// EpisodeTitle is the episode's title.
	EpisodeTitle string
	Season       int
	Episodes     []int
	Year         int
	ReleaseGroup string
	// Source is a merged source (SourceWeb, …), or "" for one that
	// matches nothing.
	Source           string
	Resolution       string
	VideoCodec       string
	AudioCodecs      []string
	Edition          string
	StreamingService string
}

// ParseGuess parses a release or file name of an episode or a movie.
func ParseGuess(name string, episode bool) Guess {
	name = strings.TrimSpace(name)
	kind := release.Movie
	if episode {
		kind = release.Series
	}
	quality := release.ParseQuality(name, kind)
	g := Guess{
		Source:           SourceOf(quality.Quality),
		Resolution:       ResolutionOf(quality),
		VideoCodec:       release.ParseVideoCodec(name),
		AudioCodecs:      release.ParseAudioCodecs(name),
		StreamingService: release.ParseStreamingService(name),
	}
	if episode {
		if p := release.ParseSeriesTitle(name); p != nil {
			info := p.SeriesTitleInfo
			if title := cmp.Or(info.TitleWithoutYear, p.SeriesTitle); title != "" {
				g.Titles = append(g.Titles, title)
			}
			for _, t := range info.AllTitles {
				if !slices.ContainsFunc(g.Titles, func(x string) bool { return strings.EqualFold(x, t) }) {
					g.Titles = append(g.Titles, t)
				}
			}
			g.Year = info.Year
			g.Season = p.SeasonNumber
			g.Episodes = p.EpisodeNumbers
			g.ReleaseGroup = p.ReleaseGroup
		}
	} else {
		if p := release.ParseMovieTitle(strings.TrimSuffix(name, path.Ext(name)), false); p != nil {
			g.Titles = p.MovieTitles
			g.Year = p.Year
			g.ReleaseGroup = p.ReleaseGroup
			g.Edition = p.Edition
		} else {
			g.Edition = release.ParseEdition(name)
		}
	}
	if g.ReleaseGroup == "" {
		g.ReleaseGroup = release.ParseReleaseGroup(name, kind)
	}
	return g
}

// Refine fills the properties of v that are empty with those of g, as
// Bazarr refines the guess of a file name with its release name.
func (v *Video) Refine(g Guess) {
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&v.ReleaseGroup, g.ReleaseGroup)
	fill(&v.Source, g.Source)
	fill(&v.Resolution, g.Resolution)
	fill(&v.VideoCodec, g.VideoCodec)
	if len(g.AudioCodecs) > 0 {
		fill(&v.AudioCodec, g.AudioCodecs[0])
	}
	fill(&v.Edition, g.Edition)
	fill(&v.StreamingService, g.StreamingService)
}
