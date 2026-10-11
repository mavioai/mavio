package organize

import (
	"errors"
	"testing"
)

// Ported from Sonarr's GetSeasonFolderFixture.
func TestSeasonFolder(t *testing.T) {
	cases := []struct {
		title  string
		season int
		format string
		want   string
	}{
		{"Venture Bros.", 1, "{Series.Title}.{season:00}", "Venture.Bros.01"},
		{"Venture Bros.", 1, "{Series Title} Season {season:00}", "Venture Bros. Season 01"},
		{"Series Title?", 1, "{Series Title} Season {season:00}", "Series Title! Season 01"},
	}
	for _, c := range cases {
		n := DefaultSeriesNaming()
		n.SeasonFolderFormat = c.format
		if got := SeasonFolder(n, Series{Title: c.title}, c.season); got != c.want {
			t.Errorf("SeasonFolder(%q, %q) = %q, want = %q", c.format, c.title, got, c.want)
		}
	}
	n := DefaultSeriesNaming()
	if got, want := SeasonFolder(n, Series{Title: "Title"}, 0), "Specials"; got != want {
		t.Errorf("SeasonFolder(season 0) = %q, want = %q", got, want)
	}
}

// Ported from Sonarr's GetSeriesFolderFixture.
func TestSeriesFolder(t *testing.T) {
	var cases []episodeCase
	for _, c := range []struct{ title, format, want string }{
		{"30 Rock", "{Series Title}", "30 Rock"},
		{"30 Rock", "{Series.Title}", "30.Rock"},
		{"24/7 Road to the NHL Winter Classic", "{Series Title}", "24+7 Road to the NHL Winter Classic"},
		{"Venture Bros.", "{Series.Title}", "Venture.Bros"},
		{".hack", "{Series.Title}", "hack"},
		{"30 Rock", ".{Series.Title}.", "30.Rock"},
	} {
		cases = append(cases, episodeCase{name: c.title + " " + c.format, format: c.format, series: withSeriesTitle(c.title), want: c.want})
	}
	runSeriesFolderCases(t, episodeFixture{naming: DefaultSeriesNaming()}, cases)
}

// Ported from Sonarr's BuildFilePathFixture.
func TestBuildEpisodeFilePath(t *testing.T) {
	cases := []struct {
		name         string
		fileName     string
		title        string
		path         string
		season       int
		seasonFolder bool
		format       string
		want         string
	}{
		{"season folder 00", "30 Rock - S01E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 1, true, "Season {season:00}", "/C/Test/30 Rock/Season 01/30 Rock - S01E05 - Episode Title.mkv"},
		{"season folder", "30 Rock - S01E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 1, true, "Season {season}", "/C/Test/30 Rock/Season 1/30 Rock - S01E05 - Episode Title.mkv"},
		{"no season folder 00", "30 Rock - S01E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 1, false, "Season {season:00}", "/C/Test/30 Rock/30 Rock - S01E05 - Episode Title.mkv"},
		{"no season folder", "30 Rock - S01E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 1, false, "Season {season}", "/C/Test/30 Rock/30 Rock - S01E05 - Episode Title.mkv"},
		{"ugly season folder", "30 Rock - S01E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 1, true, "ReallyUglySeasonFolder {season}", "/C/Test/30 Rock/ReallyUglySeasonFolder 1/30 Rock - S01E05 - Episode Title.mkv"},
		{"specials folder", "30 Rock - S00E05 - Episode Title", "30 Rock", "/C/Test/30 Rock", 0, true, "Season {season}", "/C/Test/30 Rock/MySpecials/30 Rock - S00E05 - Episode Title.mkv"},
		{
			"clean season folder with illegal characters in series title", "S01E05 - Episode Title", "NCIS: Los Angeles",
			"/C/Test/NCIS - Los Angeles", 1, true, "{Series Title} Season {season:0}",
			"/C/Test/NCIS - Los Angeles/NCIS - Los Angeles Season 1/S01E05 - Episode Title.mkv",
		},
	}
	for _, c := range cases {
		n := DefaultSeriesNaming()
		n.SeasonFolderFormat = c.format
		n.SpecialsFolderFormat = "MySpecials"
		s := Series{Title: c.title, Path: c.path, SeasonFolder: c.seasonFolder}
		eps := []Episode{{Title: "Episode Title", SeasonNumber: c.season, EpisodeNumber: 5}}
		got, err := BuildEpisodeFilePath(n, s, eps, EpisodeFile{SceneName: c.fileName, Imported: true}, ".mkv")
		if err != nil || got != c.want {
			t.Errorf("%s: BuildEpisodeFilePath() = %q, %v, want = %q", c.name, got, err, c.want)
		}
	}
}

// Ported from Sonarr's CleanFilenameFixture.
func TestSeriesCleanFileName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Law & Order: Criminal Intent - S10E07 - Icarus [HDTV-720p]", "Law & Order - Criminal Intent - S10E07 - Icarus [HDTV-720p]"},
		{".hack s01e01", "hack s01e01"},
		{" Series Title - S01E01 - Episode Title", "Series Title - S01E01 - Episode Title"},
		{"Series Title - S01E01 - Episode Title ", "Series Title - S01E01 - Episode Title"},
	}
	for _, c := range cases {
		if got := CleanFileName(c.name); got != c.want {
			t.Errorf("CleanFileName(%q) = %q, want = %q", c.name, got, c.want)
		}
	}
}

// Ported from Sonarr's OriginalTitleFixture.
func TestEpisodeOriginalTitle(t *testing.T) {
	daily := func(s *Series) { s.Type = SeriesDaily }
	airDate := func(eps []Episode) []Episode {
		eps[0].AirDate = "2022-04-28"
		return eps
	}
	absolute := func(eps []Episode) []Episode {
		eps[0].AbsoluteEpisodeNumber = 123
		return eps
	}
	relative := func(p string) func(f *EpisodeFile) { return func(f *EpisodeFile) { f.RelativePath = p } }
	newFile := func(p string) func(f *EpisodeFile) {
		return func(f *EpisodeFile) { f.RelativePath, f.Imported = p, false }
	}
	cases := []episodeCase{
		{
			name: "include original title if not current file name", format: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {[Original Title]}",
			file: func(f *EpisodeFile) {
				f.SceneName, f.RelativePath = "my.series.s15e06", "My Series - S15E06 - City Sushi"
			},
			want: "My Series - S15E06 - City Sushi [my.series.s15e06]",
		},
		{
			name: "include current filename if not renaming files", naming: notRenamingEpisodes,
			file: func(f *EpisodeFile) { f.SceneName = "my.series.s15e06" }, want: "my.series.s15e06",
		},
		{
			name: "include current filename without season and episode tokens for standard series", format: "{Original Title} {Quality Title}",
			file: relative("My Series - S15E06 - City Sushi"), want: "My Series - S15E06 - City Sushi HDTV-720p",
		},
		{
			name: "include current filename without air date token for daily series", format: "{Original Title} {Quality Title}",
			series: daily, episodes: airDate, file: relative("My Series - 2022-04-28 - City Sushi"),
			want: "My Series - 2022-04-28 - City Sushi HDTV-720p",
		},
		{
			name: "include current filename without absolute number token for anime series", format: "{Original Title} {Quality Title}",
			series: anime, episodes: absolute, file: relative("My Series - 123 - City Sushi"), want: "My Series - 123 - City Sushi HDTV-720p",
		},
		{
			name: "not include current filename with season and episode tokens for standard series", format: "{Series Title} - S{season:00}E{episode:00} {[Original Title]}",
			file: relative("My Series - S15E06 - City Sushi"), want: "My Series - S15E06",
		},
		{
			name: "not include current filename with air date token for daily series", format: "{Series Title} - {Air-Date} {[Original Title]}",
			series: daily, episodes: airDate, file: relative("My Series - 2022-04-28 - City Sushi"), want: "My Series - 2022-04-28",
		},
		{
			name: "not include current filename with absolute number token for anime series", format: "{Series Title} - {absolute:00} {[Original Title]}",
			series: anime, episodes: absolute, file: relative("My Series - 123 - City Sushi"), want: "My Series - 123",
		},
		{
			name: "include current filename for new file with season and episode tokens for standard series", format: "{Series Title} - S{season:00}E{episode:00} {[Original Title]}",
			file: newFile("My Series - S15E06 - City Sushi"), want: "My Series - S15E06 [My Series - S15E06 - City Sushi]",
		},
		{
			name: "include current filename for new file with air date token for daily series", format: "{Series Title} - {Air-Date} {[Original Title]}",
			series: daily, episodes: airDate, file: newFile("My Series - 2022-04-28 - City Sushi"),
			want: "My Series - 2022-04-28 [My Series - 2022-04-28 - City Sushi]",
		},
		{
			name: "include current filename for new file with absolute number token for anime series", format: "{Series Title} - {absolute:00} {[Original Title]}",
			series: anime, episodes: absolute, file: newFile("My Series - 123 - City Sushi"), want: "My Series - 123 [My Series - 123 - City Sushi]",
		},
	}
	f := sonarrHDTV720p()
	f.Imported = true
	runEpisodeCases(t, episodeFixture{naming: seriesRenaming(), series: Series{Title: "My Series"}, episodes: []Episode{citySushi()}, file: f}, cases)
}

// Ported from Sonarr's CustomFormatsFixture.
func TestEpisodeCustomFormats(t *testing.T) {
	formats := []CustomFormat{
		{Name: "INTERNAL", IncludeWhenRenaming: true},
		{Name: "AMZN", IncludeWhenRenaming: true},
		{Name: "NAME WITH SPACES", IncludeWhenRenaming: true},
		{Name: "NotIncludedFormat", IncludeWhenRenaming: false},
	}
	withFormats := func(f *EpisodeFile) { f.CustomFormats = formats }
	var cases []episodeCase
	for _, c := range []struct {
		format string
		with   bool
		want   string
	}{
		{"{Custom Formats}", true, "INTERNAL AMZN NAME WITH SPACES"},
		{"{Custom Formats}", false, ""},
		{"{Custom Formats:-INTERNAL}", true, "AMZN NAME WITH SPACES"},
		{"{Custom Formats:-NAME WITH SPACES}", true, "INTERNAL AMZN"},
		{"{Custom Formats:-INTERNAL,NAME WITH SPACES}", true, "AMZN"},
		{"{Custom Formats:INTERNAL}", true, "INTERNAL"},
		{"{Custom Formats:NAME WITH SPACES}", true, "NAME WITH SPACES"},
		{"{Custom Formats:INTERNAL,NAME WITH SPACES}", true, "INTERNAL NAME WITH SPACES"},
		{"{Custom Formats:-}", true, "{Custom Formats:-}"},
		{"{Custom Formats:}", true, "{Custom Formats:}"},
		{"{Custom Format}", true, ""},
		{"{Custom Format:INTERNAL}", true, "INTERNAL"},
		{"{Custom Format:AMZN}", true, "AMZN"},
		{"{Custom Format:NAME WITH SPACES}", true, "NAME WITH SPACES"},
		{"{Custom Format:DOESNOTEXIST}", true, ""},
		{"{Custom Format:INTERNAL} - {Custom Format:AMZN}", true, "INTERNAL - AMZN"},
		{"{Custom Format:AMZN} - {Custom Format:INTERNAL}", true, "AMZN - INTERNAL"},
		{"{Custom Format}", false, ""},
		{"{Custom Format:INTERNAL}", false, ""},
		{"{Custom Format:AMZN}", false, ""},
	} {
		ec := episodeCase{name: c.format, format: c.format, want: c.want}
		if c.with {
			ec.file = withFormats
		} else {
			ec.name += " without formats"
		}
		cases = append(cases, ec)
	}
	runEpisodeCases(t, southParkEpisodeFixture(), cases)
}

// Ported from Sonarr's RequiresEpisodeTitleFixture.
func TestRequiresEpisodeTitle(t *testing.T) {
	s := Series{Title: "South Park"}
	eps := []Episode{citySushi()}
	n := seriesRenaming()
	n.StandardEpisodeFormat = "{Series Title} S{season:00}E{episode:00}"
	if RequiresEpisodeTitle(n, s, eps) {
		t.Errorf("RequiresEpisodeTitle(%q) = true, want = false", n.StandardEpisodeFormat)
	}
	n = DefaultSeriesNaming()
	if RequiresEpisodeTitle(n, s, eps) {
		t.Errorf("RequiresEpisodeTitle(not renaming) = true, want = false")
	}
	n = seriesRenaming()
	if !RequiresEpisodeTitle(n, s, eps) {
		t.Errorf("RequiresEpisodeTitle(%q) = false, want = true", n.StandardEpisodeFormat)
	}
}

// Ported from Sonarr's RequiresAbsoluteEpisodeNumberFixture.
func TestRequiresAbsoluteEpisodeNumber(t *testing.T) {
	cases := []struct {
		format string
		want   bool
	}{
		{"{Series Title} S{season:00}E{episode:00}", false},
		{"{Series Title} {absolute:00}", true},
	}
	for _, c := range cases {
		n := seriesRenaming()
		n.AnimeEpisodeFormat = c.format
		if got := RequiresAbsoluteEpisodeNumber(n); got != c.want {
			t.Errorf("RequiresAbsoluteEpisodeNumber(%q) = %v, want = %v", c.format, got, c.want)
		}
	}
}

// The cases follow the validators of Sonarr's FileNameValidation and
// NamingConfigController, which have no fixture of their own.
func TestSeriesNamingValidate(t *testing.T) {
	cases := []struct {
		name   string
		naming func(n *SeriesNaming)
		want   error
	}{
		{"default", func(*SeriesNaming) {}, nil},
		{"invalid colon", func(n *SeriesNaming) { n.ColonReplacement = "colon" }, ErrInvalidColonReplacement},
		{"custom colon", func(n *SeriesNaming) { n.ColonReplacement, n.CustomColonReplacement = ColonCustom, "∶" }, nil},
		{"custom colon empty", func(n *SeriesNaming) { n.ColonReplacement = ColonCustom }, nil},
		{"custom colon with colon", func(n *SeriesNaming) { n.ColonReplacement, n.CustomColonReplacement = ColonCustom, " : " }, ErrIllegalCharacters},
		{"custom colon with slash", func(n *SeriesNaming) { n.ColonReplacement, n.CustomColonReplacement = ColonCustom, "/" }, ErrIllegalCharacters},
		{"invalid style", func(n *SeriesNaming) { n.MultiEpisodeStyle = "triple" }, ErrInvalidMultiEpisodeStyle},
		{"standard {season}x{episode}", func(n *SeriesNaming) { n.StandardEpisodeFormat = "{season}x{episode}" }, nil},
		{"standard season and episode apart", func(n *SeriesNaming) { n.StandardEpisodeFormat = "Season {season} Episode {episode}" }, nil},
		{"standard original title", func(n *SeriesNaming) { n.StandardEpisodeFormat = "{Original Title}" }, nil},
		{"standard without numbers", func(n *SeriesNaming) { n.StandardEpisodeFormat = "{Series Title} {Episode Title}" }, ErrMissingTokens},
		{"standard air date", func(n *SeriesNaming) { n.StandardEpisodeFormat = "{Series Title} {Air-Date}" }, ErrMissingTokens},
		{"standard empty", func(n *SeriesNaming) { n.StandardEpisodeFormat = "" }, ErrEmptyFormat},
		{"standard control character", func(n *SeriesNaming) { n.StandardEpisodeFormat += "\x00" }, ErrIllegalCharacters},
		{"daily air date", func(n *SeriesNaming) { n.DailyEpisodeFormat = "{Series Title} {Air.Date}" }, nil},
		{"daily without date", func(n *SeriesNaming) { n.DailyEpisodeFormat = "{Series Title}" }, ErrMissingTokens},
		{"anime absolute", func(n *SeriesNaming) { n.AnimeEpisodeFormat = "{Series Title} - {absolute:000}" }, nil},
		{"anime without numbers", func(n *SeriesNaming) { n.AnimeEpisodeFormat = "{Series Title} {Air-Date}" }, ErrMissingTokens},
		{"series folder clean title year", func(n *SeriesNaming) { n.SeriesFolderFormat = "{Series CleanTitleYear}" }, nil},
		{"series folder without title", func(n *SeriesNaming) { n.SeriesFolderFormat = "{TvdbId}" }, ErrMissingTokens},
		{"season folder padded", func(n *SeriesNaming) { n.SeasonFolderFormat = "S{season:00}" }, nil},
		{"season folder without season", func(n *SeriesNaming) { n.SeasonFolderFormat = "Season" }, ErrMissingTokens},
		{"specials folder empty", func(n *SeriesNaming) { n.SpecialsFolderFormat = " " }, ErrEmptyFormat},
	}
	for _, c := range cases {
		n := DefaultSeriesNaming()
		c.naming(&n)
		if err := n.Validate(); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: Validate() = %v, want = %v", c.name, err, c.want)
		}
	}
}

func TestNamingValidateRejectsCustomColon(t *testing.T) {
	n := DefaultNaming()
	n.ColonReplacement = ColonCustom
	if err := n.Validate(); !errors.Is(err, ErrInvalidColonReplacement) {
		t.Errorf("Validate() = %v, want = %v", err, ErrInvalidColonReplacement)
	}
}
