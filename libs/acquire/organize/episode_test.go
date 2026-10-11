package organize

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// episodeMediaInfo formats media information as Sonarr's MediaInfoModel
// describes it, with ffprobe codec names.
func episodeMediaInfo(f EpisodeFile, videoFormat, audioFormat string, audioChannels float64, hdrType string, audio, subtitles []string) *MediaInfo {
	return &MediaInfo{
		VideoCodec:            FormatVideoCodec(videoFormat, "", f.SceneOrFileName()),
		VideoBitDepth:         8,
		VideoDynamicRangeType: hdrType,
		AudioCodec:            FormatAudioCodec(audioFormat, "", ""),
		AudioChannels:         audioChannels,
		AudioLanguages:        audio,
		SubtitleLanguages:     subtitles,
	}
}

// givenEpisodeMediaInfo is GivenMediaInfoModel of Sonarr's
// FileNameBuilderFixture.
func givenEpisodeMediaInfo(hdrType, audio, subtitles string) func(f *EpisodeFile) {
	return func(f *EpisodeFile) {
		f.ReleaseGroup = ""
		f.MediaInfo = episodeMediaInfo(*f, "h264", "dts", 6, hdrType, strings.Split(audio, "/"), strings.Split(subtitles, "/"))
	}
}

func episodeReal(f *EpisodeFile) { f.Revision.Real = 1 }

func notRenamingEpisodes(n *SeriesNaming) { n.RenameEpisodes = false }

func seasonOne(eps []Episode) []Episode {
	eps[0].SeasonNumber = 1
	return eps
}

// Ported from Sonarr's FileNameBuilderFixture.
func TestBuildEpisodeFileName(t *testing.T) {
	dailyShow := func(s *Series) { s.Title, s.Type = "The Daily Show with Jon Stewart", SeriesDaily }
	kristenStewart := func(season int, airDate string) func([]Episode) []Episode {
		return func(eps []Episode) []Episode {
			eps[0].AirDate, eps[0].Title, eps[0].SeasonNumber, eps[0].EpisodeNumber = airDate, "Kristen Stewart", season, 5
			return eps
		}
	}
	part1 := func(title string) func([]Episode) []Episode {
		return func([]Episode) []Episode { return []Episode{{Title: title, SeasonNumber: 6, EpisodeNumber: 6}} }
	}
	sceneFile := func(f *EpisodeFile) {
		f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid-LOL", "30 Rock - S01E01 - Test"
	}
	cases := []episodeCase{
		{name: "replace Series space Title", format: "{Series Title}", want: "South Park"},
		{name: "replace Series underscore Title", format: "{Series_Title}", want: "South_Park"},
		{name: "replace Series dot Title", format: "{Series.Title}", want: "South.Park"},
		{name: "replace Series dash Title", format: "{Series-Title}", want: "South-Park"},
		{name: "SERIES TITLE all caps", format: "{SERIES TITLE}", want: "SOUTH PARK"},
		{name: "random casing keeps original casing", format: "{sErIES-tItLE}", want: "South-Park"},
		{name: "series title all lower case", format: "{series title}", want: "south park"},
		{name: "cleanup Series Title", format: "{Series.CleanTitle}", series: withSeriesTitle("South Park (1997)"), want: "South.Park.1997"},
		{name: "replace episode title", format: "{Episode Title}", want: "City Sushi"},
		{name: "episode title random casing", format: "{ePisOde-TitLe}", want: "City-Sushi"},
		{name: "season number single digit", format: "{season}x{episode}", episodes: seasonOne, want: "1x6"},
		{name: "season00 number two digits", format: "{season:00}x{episode}", episodes: seasonOne, want: "01x6"},
		{name: "episode number single digit", format: "{season}x{episode}", episodes: seasonOne, want: "1x6"},
		{name: "episode00 number two digits", format: "{season}x{episode:00}", episodes: seasonOne, want: "1x06"},
		{name: "quality title", format: "{Quality Title}", want: "HDTV-720p"},
		{name: "quality proper with proper", format: "{Quality Proper}", file: episodeProper, want: "Proper"},
		{name: "quality real with real", format: "{Quality Real}", file: episodeReal, want: "REAL"},
		{
			name:   "all contents in pattern",
			format: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} [{Quality Title}]",
			want:   "South Park - S15E06 - City Sushi [HDTV-720p]",
		},
		{name: "escape opening brace", format: "Some Escaped {{ String", want: "Some Escaped { String"},
		{name: "escape closing brace", format: "Some Escaped }} String", want: "Some Escaped } String"},
		{name: "escape token", format: "Some Escaped {{Series Title}} String", want: "Some Escaped {Series Title} String"},
		{name: "escape braces around token", format: "Some Escaped {{{Series Title}}} String", want: "Some Escaped {South Park} String"},
		{
			name: "escape token in title", format: "Some Unescaped {Series Title} String",
			series: withSeriesTitle("My {Quality Full} Title"), want: "Some Unescaped My {Quality Full} Title String",
		},
		{
			name: "use file name when sceneName is null", naming: notRenamingEpisodes,
			file: func(f *EpisodeFile) { f.RelativePath = "30 Rock - S01E01 - Test" }, want: "30 Rock - S01E01 - Test",
		},
		{
			name: "use path when sceneName and relative path are null", naming: notRenamingEpisodes,
			file: func(f *EpisodeFile) { f.Path = "/C/Test/Unsorted/Series - S01E01 - Test" }, want: "Series - S01E01 - Test",
		},
		{name: "use file name when sceneName is not null", naming: notRenamingEpisodes, file: sceneFile, want: "30.Rock.S01E01.xvid-LOL"},
		{
			name: "replace illegal characters when renaming is disabled",
			naming: func(n *SeriesNaming) {
				n.RenameEpisodes, n.ReplaceIllegalCharacters, n.ColonReplacement = false, true, ColonSmart
			},
			file: func(f *EpisodeFile) {
				f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid:LOL", "30 Rock - S01E01 - Test"
			},
			want: "30.Rock.S01E01.xvid-LOL",
		},
		{
			name: "use airDate if series isDaily and not a special", format: "{Series Title} - {air-date} - {Episode Title}",
			series: dailyShow, episodes: kristenStewart(1, "2012-12-13"),
			want: "The Daily Show with Jon Stewart - 2012-12-13 - Kristen Stewart",
		},
		{
			name: "use standard if series isDaily special",
			naming: func(n *SeriesNaming) {
				n.StandardEpisodeFormat = "{Series Title} - S{season:00}E{episode:00} - {Episode Title}"
			},
			series: dailyShow, episodes: kristenStewart(0, "2012-12-13"),
			want: "The Daily Show with Jon Stewart - S00E05 - Kristen Stewart",
		},
		{
			name: "set airdate to unknown if not available", format: "{Series Title} - {Air-Date} - {Episode Title}",
			series: dailyShow, episodes: kristenStewart(1, ""),
			want: "The Daily Show with Jon Stewart - Unknown - Kristen Stewart",
		},
		{name: "not clean episode title if there is only one", format: "{Episode Title}", episodes: episodeTitled("City Sushi (1)"), want: "City Sushi (1)"},
		{name: "replace release group", format: "{Release Group}", want: "SonarrTest"},
		{
			name: "use original title", format: "{Series Title} - {Original Title}", series: withSeriesTitle("30 Rock"),
			file: sceneFile, want: "30 Rock - 30.Rock.S01E01.xvid-LOL",
		},
		{
			name: "trim periods from end of episode title", format: "{Series Title} - S{season:00}E{episode:00} - {Episode Title}",
			naming: withStyle(MultiEpisodeScene), series: withSeriesTitle("30 Rock"), episodes: part1("Part 1."),
			want: "30 Rock - S06E06 - Part 1",
		},
		{
			name: "trim question marks from end of episode title", format: "{Series Title} - S{season:00}E{episode:00} - {Episode Title}",
			naming: withStyle(MultiEpisodeScene), series: withSeriesTitle("30 Rock"), episodes: part1("Part 1?"),
			want: "30 Rock - S06E06 - Part 1",
		},
		{
			name: "replace double period with single period", format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}",
			series: withSeriesTitle("Chicago P.D."), episodes: part1("Part 1"), want: "Chicago.P.D.S06E06.Part.1",
		},
		{
			name: "replace triple period with single period", format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}",
			series: withSeriesTitle("Chicago P.D.."), episodes: part1("Part 1"), want: "Chicago.P.D.S06E06.Part.1",
		},
		{
			name:   "not replace absolute numbering when series is not anime",
			format: "{Series.Title}.S{season:00}E{episode:00}.{absolute:00}.{Episode.Title}",
			want:   "South.Park.S15E06.City.Sushi",
		},
		{
			name:   "replace standard and absolute numbering when series is anime",
			format: "{Series.Title}.S{season:00}E{episode:00}.{absolute:00}.{Episode.Title}",
			series: anime, want: "South.Park.S15E06.100.City.Sushi",
		},
		{
			name:   "replace standard numbering when series is anime",
			format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}",
			series: anime, want: "South.Park.S15E06.City.Sushi",
		},
		{
			name: "replace absolute numbering when series is anime", format: "{Series.Title}.{absolute:00}.{Episode.Title}",
			series: anime, want: "South.Park.100.City.Sushi",
		},
		{
			name:   "replace duplicate numbering individually",
			format: `{Series.Title}.{season}x{episode:00}.{absolute:000}\{Series.Title}.S{season:00}E{episode:00}.{absolute:00}.{Episode.Title}`,
			series: anime, want: "South.Park.15x06.100/South.Park.S15E06.100.City.Sushi",
		},
		{
			name:   "replace individual season episode tokens",
			format: `{Series Title} Season {season:0000} Episode {episode:0000}\{Series.Title}.S{season:00}E{episode:00}.{absolute:00}.{Episode.Title}`,
			series: anime, want: "South Park Season 0015 Episode 0006/South.Park.S15E06.100.City.Sushi",
		},
		{
			name: "use standard naming when anime episode has no absolute number",
			naming: func(n *SeriesNaming) {
				n.StandardEpisodeFormat = "{Series Title} - {season:0}x{episode:00} - {Episode Title}"
				n.AnimeEpisodeFormat = "{Series Title} - {absolute:000} - {Episode Title}"
			},
			series: anime,
			episodes: func(eps []Episode) []Episode {
				eps[0].AbsoluteEpisodeNumber = 0
				return eps
			},
			want: "South Park - 15x06 - City Sushi",
		},
		{
			name:   "include affixes if value not empty",
			format: "{Series.Title}.S{season:00}E{episode:00}{_Episode.Title_}{Quality.Title}",
			want:   "South.Park.S15E06_City.Sushi_HDTV-720p",
		},
		{
			name: "not include affixes if value empty", format: "{Series.Title}.S{season:00}E{episode:00}{_Episode.Title_}",
			episodes: episodeTitled(""), want: "South.Park.S15E06",
		},
		{
			name:   "format mediainfo properly",
			format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}.{MEDIAINFO.FULL}",
			file: func(f *EpisodeFile) {
				f.MediaInfo = episodeMediaInfo(*f, "h264", "dts", 0, "", []string{"eng", "spa"}, []string{"eng", "spa", "ita"})
			},
			want: "South.Park.S15E06.City.Sushi.H264.DTS[EN+ES].[EN+ES+IT]",
		},
		{
			name:   "exclude english in mediainfo audio language",
			format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}.{MEDIAINFO.FULL}",
			file: func(f *EpisodeFile) {
				f.MediaInfo = episodeMediaInfo(*f, "h264", "dts", 0, "", []string{"eng"}, []string{"eng", "spa", "ita"})
			},
			want: "South.Park.S15E06.City.Sushi.H264.DTS.[EN+ES+IT]",
		},
		{
			name: "use existing filename when scene name is not available", format: "{Original Title}",
			file: func(f *EpisodeFile) { f.RelativePath = "existing.file.mkv" }, want: "existing.file",
		},
		{name: "use only original title", format: "{Original Title}", series: withSeriesTitle("30 Rock"), file: sceneFile, want: "30.Rock.S01E01.xvid-LOL"},
		{
			name: "allow period between season and episode", format: "{Series.Title}.S{season:00}.E{episode:00}.{Episode.Title}",
			want: "South.Park.S15.E06.City.Sushi",
		},
		{
			name: "allow space between season and episode", format: "{Series Title} - S{season:00} E{episode:00} - {Episode Title}",
			want: "South Park - S15 E06 - City Sushi",
		},
		{name: "quality proper with v2 for anime v2", format: "{Quality Proper}", series: anime, file: episodeProper, want: "v2"},
		{name: "no quality proper when not a proper", format: "{Quality Title} {Quality Proper}", want: "HDTV-720p"},
		{
			name: "wrap proper in square brackets", format: "{Series Title} - S{season:00}E{episode:00} [{Quality Title}] {[Quality Proper]}",
			file: episodeProper, want: "South Park - S15E06 [HDTV-720p] [Proper]",
		},
		{
			name:   "no wrapped proper when not a proper",
			format: "{Series Title} - S{season:00}E{episode:00} [{Quality Title}] {[Quality Proper]}",
			want:   "South Park - S15E06 [HDTV-720p]",
		},
		{
			name: "quality full with title only when not a proper", format: "{Series Title} - S{season:00}E{episode:00} [{Quality Full}]",
			want: "South Park - S15E06 [HDTV-720p]",
		},
		{
			name: "quality full with title and proper when a proper", format: "{Series Title} - S{season:00}E{episode:00} [{Quality Full}]",
			file: episodeProper, want: "South Park - S15E06 [HDTV-720p Proper]",
		},
		{
			name: "quality full with title and real when a real", format: "{Series Title} - S{season:00}E{episode:00} [{Quality Full}]",
			file: episodeReal, want: "South Park - S15E06 [HDTV-720p REAL]",
		},
		{
			name: "no separator required between tokens", format: "[{Release Group}]{Series.CleanTitle}.{absolute:000}",
			series: anime, want: "[SonarrTest]South.Park.100",
		},
		{
			name: "use original filename only", format: "{Original Filename}", series: withSeriesTitle("30 Rock"),
			file: sceneFile, want: "30 Rock - S01E01 - Test",
		},
		{name: "default release group when not available", format: "{Release Group}", file: func(f *EpisodeFile) { f.ReleaseGroup = "" }, want: DefaultReleaseGroup},
		{
			name: "all tokens for culture en-US", format: "{MediaInfo AudioLanguages}",
			file: givenEpisodeMediaInfo("", "eng/deu", "eng/spa/ita"), want: "[EN+DE]",
		},
		{name: "replace release hash with stored hash", format: "{Release Hash}", file: func(f *EpisodeFile) { f.ReleaseHash = "ABCDEFGH" }, want: "ABCDEFGH"},
		{name: "replace null release hash with empty string", format: "{Release Hash}", want: ""},
		{
			name: "maintain ellipsis in naming format", format: "{Series.Title}.S{season:00}.E{episode:00}...{Episode.CleanTitle}",
			want: "South.Park.S15.E06...City.Sushi",
		},
	}
	for _, sep := range []string{" ", "-", ".", "_"} {
		cases = append(cases,
			episodeCase{
				name:   fmt.Sprintf("trim extra separators from end %q", sep),
				format: fmt.Sprintf("{Quality%[1]sTitle}%[1]s{Quality%[1]sProper}", sep),
				want:   "HDTV-720p",
			},
			episodeCase{
				name:   fmt.Sprintf("trim extra separators from middle %q", sep),
				format: fmt.Sprintf("{Quality%[1]sTitle}%[1]s{Quality%[1]sProper}%[1]s{Episode%[1]sTitle}", sep),
				want:   fmt.Sprintf("HDTV-720p%[1]sCity%[1]sSushi", sep),
			})
	}
	for _, format := range []string{"{Episode Title}{-Release Group}", "{Episode Title}{ Release Group}", "{Episode Title}{ [Release Group]}"} {
		cases = append(cases, episodeCase{
			name: "no default release group with separator " + format, format: format,
			file: func(f *EpisodeFile) { f.ReleaseGroup = "" }, want: "City Sushi",
		})
	}
	for _, group := range []string{"0SEC", "2HD", "IMMERSE"} {
		cases = append(cases, episodeCase{
			name: "existing casing for release group " + group, format: "{Release Group}",
			file: func(f *EpisodeFile) { f.ReleaseGroup = group }, want: group,
		})
	}
	for _, lc := range []struct{ language, code string }{
		{"nob", "NB"},
		{"swe", "SV"},
		{"zho", "ZH"},
		{"chi", "ZH"},
		{"fre", "FR"},
		{"rum", "RO"},
		{"per", "FA"},
		{"ger", "DE"},
		{"cze", "CS"},
		{"ice", "IS"},
		{"dut", "NL"},
		{"nor", "NO"},
	} {
		cases = append(cases, episodeCase{
			name: "format language code " + lc.language, format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}.{MEDIAINFO.FULL}",
			file: func(f *EpisodeFile) {
				f.MediaInfo = episodeMediaInfo(*f, "h264", "dts", 6, "", []string{"eng"}, []string{lc.language})
			},
			want: "South.Park.S15E06.City.Sushi.H264.DTS.[" + lc.code + "]",
		})
	}
	for _, a := range []struct{ format, languages, want string }{
		{"{MediaInfo AudioLanguages}", "eng", ""},
		{"{MediaInfo AudioLanguages}", "eng/deu", "[EN+DE]"},
		{"{MediaInfo AudioLanguagesAll}", "eng", "[EN]"},
		{"{MediaInfo AudioLanguagesAll}", "eng/deu", "[EN+DE]"},
	} {
		cases = append(cases, episodeCase{
			name: "format audio languages " + a.format + " " + a.languages, format: a.format,
			file: givenEpisodeMediaInfo("", a.languages, "eng/spa/ita"), want: a.want,
		})
	}
	for _, s := range []struct{ languages, filter, want string }{
		{"eng/deu", "", "[EN+DE]"},
		{"eng/nld/deu", "", "[EN+NL+DE]"},
		{"eng/deu", ":DE", "[DE]"},
		{"eng/nld/deu", ":EN+NL", "[EN+NL]"},
		{"eng/nld/deu", ":NL+EN", "[NL+EN]"},
		{"eng/nld/deu", ":-NL", "[EN+DE]"},
		{"eng/nld/deu", ":DE+", "[DE+-]"},
		{"eng/nld/deu", ":DE+NO.", "[DE]."},
		{"eng/nld/deu", ":-EN-", "[NL+DE]-"},
	} {
		cases = append(cases, episodeCase{
			name: "format subtitle languages " + s.languages + s.filter, format: "{MediaInfo SubtitleLanguages" + s.filter + "}End",
			file: givenEpisodeMediaInfo("", "eng", s.languages), want: s.want + "End",
		})
	}
	// HdrFormat.None, Hlg10 and Hdr10, as their dynamic range types.
	for _, h := range []struct{ hdrType, want string }{
		{"", "South.Park.S15E06.City.Sushi"},
		{"HLG", "South.Park.S15E06.City.Sushi.HDR"},
		{"HDR10", "South.Park.S15E06.City.Sushi.HDR"},
	} {
		cases = append(cases, episodeCase{
			name: "hdr for videodynamicrange " + h.hdrType, format: "{Series.Title}.S{season:00}E{episode:00}.{Episode.Title}.{MediaInfo VideoDynamicRange}",
			file: givenEpisodeMediaInfo(h.hdrType, "eng", "eng/spa/ita"), want: h.want,
		})
	}
	runEpisodeCases(t, southParkEpisodeFixture(), cases)
}

// Sonarr's FileNameBuilderFixture cases that are not ported.
func TestBuildEpisodeFileNameSkipped(t *testing.T) {
	const mediaInfoRefresh = "media info is an input; refreshing stale media info belongs to the caller"
	skipped := []struct{ name, reason string }{
		{"should_remove_duplicate_non_word_characters", "ignored in Sonarr as not currently supported"},
		{"should_replace_all_tokens_for_different_cultures(fr-FR, az, tr-TR)", "Go case mapping is culture-invariant; the en-US case is ported as \"all tokens for culture en-US\""},
		{"should_update_media_info_if_token_configured_and_revision_is_old", mediaInfoRefresh},
		{"should_not_update_media_info_if_no_series_path_available", mediaInfoRefresh},
		{"should_not_update_media_info_if_token_not_configured_and_revision_is_old", mediaInfoRefresh},
		{"should_not_update_media_info_if_token_configured_and_revision_is_current", mediaInfoRefresh},
		{"should_not_update_media_info_if_token_configured_and_revision_is_newer", mediaInfoRefresh},
		{"should_use_updated_media_info_if_token_configured_and_revision_is_old(2 cases)", mediaInfoRefresh},
	}
	for _, s := range skipped {
		t.Run(s.name, func(t *testing.T) { t.Skip(s.reason) })
	}
}

func TestBuildEpisodeFileNameErrors(t *testing.T) {
	cases := []struct {
		name   string
		naming func(n *SeriesNaming)
		series Series
		eps    []Episode
		want   error
	}{
		{"empty standard format", func(n *SeriesNaming) { n.StandardEpisodeFormat = " " }, Series{}, []Episode{citySushi()}, ErrEmptyFormat},
		{"empty daily format", func(n *SeriesNaming) { n.DailyEpisodeFormat = "" }, Series{Type: SeriesDaily}, []Episode{citySushi()}, ErrEmptyFormat},
		{"empty anime format", func(n *SeriesNaming) { n.AnimeEpisodeFormat = "" }, Series{Type: SeriesAnime}, []Episode{citySushi()}, ErrEmptyFormat},
		{"no episodes", func(*SeriesNaming) {}, Series{}, nil, ErrNoEpisodes},
	}
	for _, c := range cases {
		n := seriesRenaming()
		c.naming(&n)
		if _, err := BuildEpisodeFileName(n, c.series, c.eps, sonarrHDTV720p()); !errors.Is(err, c.want) {
			t.Errorf("%s: BuildEpisodeFileName() error = %v, want = %v", c.name, err, c.want)
		}
	}
	if _, err := BuildEpisodeFilePath(seriesRenaming(), Series{}, []Episode{citySushi()}, sonarrHDTV720p(), ""); !errors.Is(err, ErrNoExtension) {
		t.Errorf("BuildEpisodeFilePath() error = %v, want = %v", err, ErrNoExtension)
	}
}
