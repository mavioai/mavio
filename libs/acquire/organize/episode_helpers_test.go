package organize

import (
	"slices"
	"testing"
)

// episodeCase is an episode naming case: format replaces the episode
// format of the series type (or the folder format under test), and the
// functions adjust the fixture's configuration, series, episodes and file.
type episodeCase struct {
	name     string
	format   string
	naming   func(n *SeriesNaming)
	series   func(s *Series)
	episodes func(eps []Episode) []Episode
	file     func(f *EpisodeFile)
	want     string
}

// episodeFixture is the starting state of a ported Sonarr fixture.
type episodeFixture struct {
	naming   SeriesNaming
	series   Series
	episodes []Episode
	file     EpisodeFile
}

func (c episodeCase) apply(fx episodeFixture) (SeriesNaming, Series, []Episode, EpisodeFile) {
	n, s, eps, f := fx.naming, fx.series, slices.Clone(fx.episodes), fx.file
	f.CustomFormats = slices.Clone(f.CustomFormats)
	if c.naming != nil {
		c.naming(&n)
	}
	if c.series != nil {
		c.series(&s)
	}
	if c.episodes != nil {
		eps = c.episodes(eps)
	}
	if c.file != nil {
		c.file(&f)
	}
	return n, s, eps, f
}

// setEpisodeFormat sets the episode format of the series type.
func setEpisodeFormat(n *SeriesNaming, t SeriesType, format string) {
	switch t {
	case SeriesDaily:
		n.DailyEpisodeFormat = format
	case SeriesAnime:
		n.AnimeEpisodeFormat = format
	default:
		n.StandardEpisodeFormat = format
	}
}

// runEpisodeCases builds episode file names with the cases' formats as
// the episode format of the series type.
func runEpisodeCases(t *testing.T, fx episodeFixture, cases []episodeCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, s, eps, f := c.apply(fx)
			if c.format != "" {
				setEpisodeFormat(&n, s.Type, c.format)
			}
			got, err := BuildEpisodeFileName(n, s, eps, f)
			if err != nil {
				t.Fatalf("BuildEpisodeFileName() error = %v", err)
			}
			if got != c.want {
				t.Errorf("BuildEpisodeFileName() = %q, want = %q", got, c.want)
			}
		})
	}
}

// runSeriesFolderCases names series folders with the cases' formats as
// SeriesFolderFormat.
func runSeriesFolderCases(t *testing.T, fx episodeFixture, cases []episodeCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, s, _, _ := c.apply(fx)
			if c.format != "" {
				n.SeriesFolderFormat = c.format
			}
			if got := SeriesFolder(n, s); got != c.want {
				t.Errorf("SeriesFolder(%q) = %q, want = %q", n.SeriesFolderFormat, got, c.want)
			}
		})
	}
}

// seriesRenaming returns the default configuration with renaming enabled.
func seriesRenaming() SeriesNaming {
	n := DefaultSeriesNaming()
	n.RenameEpisodes = true
	return n
}

// citySushi is the episode most Sonarr fixtures name.
func citySushi() Episode {
	return Episode{Title: "City Sushi", SeasonNumber: 15, EpisodeNumber: 6, AbsoluteEpisodeNumber: 100}
}

// sonarrHDTV720p is the file most Sonarr fixtures name.
func sonarrHDTV720p() EpisodeFile {
	return EpisodeFile{Quality: "HDTV-720p", ReleaseGroup: "SonarrTest"}
}

// southParkEpisodeFixture is the setup of Sonarr's FileNameBuilderFixture
// and of most fixtures derived from it.
func southParkEpisodeFixture() episodeFixture {
	return episodeFixture{
		naming:   seriesRenaming(),
		series:   Series{Title: "South Park"},
		episodes: []Episode{citySushi()},
		file:     sonarrHDTV720p(),
	}
}

// citySushiEpisodes are the three episodes of Sonarr's MultiEpisodeFixture.
func citySushiEpisodes(n int) func([]Episode) []Episode {
	return func([]Episode) []Episode {
		var eps []Episode
		for i := range n {
			e := citySushi()
			e.EpisodeNumber += i
			e.AbsoluteEpisodeNumber += i
			eps = append(eps, e)
		}
		return eps
	}
}

func withSeriesTitle(title string) func(s *Series) { return func(s *Series) { s.Title = title } }

func withSeriesTitleYear(title string, year int) func(s *Series) {
	return func(s *Series) { s.Title, s.Year = title, year }
}

func anime(s *Series) { s.Type = SeriesAnime }

func withStyle(style MultiEpisodeStyle) func(n *SeriesNaming) {
	return func(n *SeriesNaming) { n.MultiEpisodeStyle = style }
}

func episodeProper(f *EpisodeFile) { f.Revision.Version = 2 }

func episodeTitled(title string) func([]Episode) []Episode {
	return func(eps []Episode) []Episode {
		eps[0].Title = title
		return eps
	}
}
