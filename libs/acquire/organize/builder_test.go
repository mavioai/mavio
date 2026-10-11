package organize

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func southParkFixture() fixture {
	return fixture{
		naming: renaming(),
		movie:  Movie{Title: "South Park", OriginalTitle: "South of the Park", Year: 1, Translations: southParkTranslations},
		file:   hdtv720p(),
	}
}

func proper(f *File) { f.Revision.Version = 2 }

func realRevision(f *File) { f.Revision.Real = 1 }

func notRenaming(n *Naming) { n.RenameMovies = false }

// givenMediaInfo is GivenMediaInfoModel of Radarr's FileNameBuilderFixture.
func givenMediaInfo(hdrType, audio, subtitles string) func(f *File) {
	return func(f *File) {
		f.MediaInfo = mediaInfo(*f, "h264", "dts", 6, hdrType, strings.Split(audio, "/"), strings.Split(subtitles, "/"))
	}
}

// Ported from Radarr's FileNameBuilderFixture.
func TestBuildFileName(t *testing.T) {
	cases := []nameCase{
		{name: "replace Movie space Title", format: "{Movie Title}", want: "South Park"},
		{name: "replace Movie underscore Title", format: "{Movie_Title}", want: "South_Park"},
		{name: "replace Movie dot Title", format: "{Movie.Title}", want: "South.Park"},
		{name: "replace Movie dash Title", format: "{Movie-Title}", want: "South-Park"},
		{name: "MOVIE TITLE all caps", format: "{MOVIE TITLE}", want: "SOUTH PARK"},
		{name: "random casing keeps original casing", format: "{mOvIe-tItLE}", want: "South-Park"},
		{name: "movie title all lower case", format: "{movie title}", want: "south park"},
		{name: "translated movie title", format: "{Movie Title:FR}", want: "French South Park"},
		{name: "translated title with base title if invalid code", format: "{Movie Title:JP}", want: "South Park"},
		{name: "translated title with base title if no translation exists", format: "{Movie Title:JA}", want: "South Park"},
		{name: "translated title with fallback if no translation exists", format: "{Movie Title:JP|FR}", want: "French South Park"},
		{name: "translated title with original if no translation or fallback exists", format: "{Movie Title:JP|CN}", want: "South Park"},
		{name: "cleanup Movie Title", format: "{Movie.CleanTitle}", movie: withTitle("South Park (1997)"), want: "South.Park.1997"},
		{name: "movie original title", format: "{Movie OriginalTitle}", want: "South of the Park"},
		{name: "movie certification", format: "{Movie Certification}", movie: func(m *Movie) { m.Certification = "R" }, want: "R"},
		{name: "movie collection", format: "{Movie Collection}", movie: withCollection("South Part Collection"), want: "South Part Collection"},
		{name: "empty for null collection", format: "{Movie Collection}", want: ""},
		{name: "quality title", format: "{Quality Title}", want: "HDTV-720p"},
		{name: "quality proper with proper", format: "{Quality Proper}", file: proper, want: "Proper"},
		{name: "quality real with real", format: "{Quality Real}", file: realRevision, want: "REAL"},
		{name: "all contents in pattern", format: "{Movie Title} [{Quality Title}]", want: "South Park [HDTV-720p]"},
		{
			name: "use file name when sceneName is null", naming: notRenaming,
			file: func(f *File) { f.RelativePath = "30 Rock - S01E01 - Test" },
			want: "30 Rock - S01E01 - Test",
		},
		{
			name: "use path when sceneName and relative path are null", naming: notRenaming,
			file: func(f *File) { f.Path = "/Test/Unsorted/Movie - S01E01 - Test" },
			want: "Movie - S01E01 - Test",
		},
		{
			name: "use file name when sceneName is not null", naming: notRenaming,
			file: func(f *File) { f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid-LOL", "30 Rock - S01E01 - Test" },
			want: "30.Rock.S01E01.xvid-LOL",
		},
		{
			name: "replace illegal characters when renaming is disabled",
			naming: func(n *Naming) {
				n.RenameMovies, n.ReplaceIllegalCharacters, n.ColonReplacement = false, true, ColonSmart
			},
			file: func(f *File) { f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid:LOL", "30 Rock - S01E01 - Test" },
			want: "30.Rock.S01E01.xvid-LOL",
		},
		{name: "replace release group", format: "{Release Group}", want: "RadarrTest"},
		{
			name: "use original title", format: "{Movie Title} - {Original Title}", movie: withTitle("30 Rock"),
			file: func(f *File) { f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid-LOL", "30 Rock - S01E01 - Test" },
			want: "30 Rock - 30.Rock.S01E01.xvid-LOL",
		},
		{name: "include affixes if value not empty", format: "{Movie.Title}.{_Quality.Title_}", want: "South.Park._HDTV-720p"},
		{
			name: "format mediainfo properly", format: "{Movie.Title}.{MEDIAINFO.FULL}",
			file: func(f *File) {
				f.MediaInfo = mediaInfo(*f, "h264", "dts", 0, "", []string{"eng", "spa"}, []string{"eng", "spa", "ita"})
			},
			want: "South.Park.H264.DTS[EN+ES].[EN+ES+IT]",
		},
		{
			name: "exclude english in mediainfo audio language", format: "{Movie.Title}.{MEDIAINFO.FULL}",
			file: func(f *File) {
				f.MediaInfo = mediaInfo(*f, "h264", "dts", 0, "", []string{"eng"}, []string{"eng", "spa", "ita"})
			},
			want: "South.Park.H264.DTS.[EN+ES+IT]",
		},
		{
			name: "format mediainfo 3d properly", format: "{Movie.Title}.{MEDIAINFO.3D}.{MediaInfo.Simple}",
			file: func(f *File) {
				f.MediaInfo = mediaInfo(*f, "h264", "dts", 0, "", []string{"eng"}, []string{"eng", "spa", "ita"})
				f.MediaInfo.Is3D = true
			},
			want: "South.Park.3D.h264.DTS",
		},
		{name: "remove duplicate non word characters", format: "{Movie.Title}", movie: withTitle("Venture Bros."), want: "Venture.Bros"},
		{
			name: "use existing filename when scene name is not available", format: "{Original Title}",
			file: func(f *File) { f.RelativePath = "existing.file.mkv" },
			want: "existing.file",
		},
		{
			name: "use only original title", format: "{Original Title}", movie: withTitle("30 Rock"),
			file: func(f *File) { f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid-LOL", "30 Rock - S01E01 - Test" },
			want: "30.Rock.S01E01.xvid-LOL",
		},
		{name: "no quality proper when not a proper", format: "{Quality Title} {Quality Proper}", want: "HDTV-720p"},
		{name: "wrap proper in square brackets", format: "{Movie Title} [{Quality Title}] {[Quality Proper]}", file: proper, want: "South Park [HDTV-720p] [Proper]"},
		{name: "no wrapped proper when not a proper", format: "{Movie Title} [{Quality Title}] {[Quality Proper]}", want: "South Park [HDTV-720p]"},
		{name: "quality full with title only when not a proper", format: "{Movie Title} [{Quality Full}]", want: "South Park [HDTV-720p]"},
		{name: "quality full with title and proper when a proper", format: "{Movie Title} [{Quality Full}]", file: proper, want: "South Park [HDTV-720p Proper]"},
		{name: "quality full with title and real when a real", format: "{Movie Title} [{Quality Full}]", file: realRevision, want: "South Park [HDTV-720p REAL]"},
		{
			name:   "replace all tokens",
			format: "{Movie CleanTitle} ({Release Year}) [{Quality Title}] [tmdb-{TmdbId}] [{MediaInfo AudioCodec}]",
			movie:  func(m *Movie) { m.TmdbID, m.Year = 124578, 2020 },
			file:   givenMediaInfo("", "eng", "eng/spa/ita"),
			want:   "South Park (2020) [HDTV-720p] [tmdb-124578] [DTS]",
		},
		{
			name: "use original filename only", format: "{Original Filename}", movie: withTitle("30 Rock"),
			file: func(f *File) { f.SceneName, f.RelativePath = "30.Rock.S01E01.xvid-LOL", "30 Rock - S01E01 - Test" },
			want: "30 Rock - S01E01 - Test",
		},
		{name: "default release group when not available", format: "{Release Group}", file: func(f *File) { f.ReleaseGroup = "" }, want: DefaultReleaseGroup},
	}
	for _, sep := range []string{" ", "-", ".", "_"} {
		cases = append(cases,
			nameCase{
				name:   fmt.Sprintf("trim extra separators from end %q", sep),
				format: fmt.Sprintf("{Quality%[1]sTitle}%[1]s{Quality%[1]sProper}", sep),
				want:   "HDTV-720p",
			},
			nameCase{
				name:   fmt.Sprintf("trim extra separators from middle %q", sep),
				format: fmt.Sprintf("{Quality%[1]sTitle}%[1]s{Quality%[1]sProper}%[1]s{Movie%[1]sTitle}", sep),
				want:   fmt.Sprintf("HDTV-720p%[1]sSouth%[1]sPark", sep),
			})
	}
	for _, format := range []string{"{Movie Title}{-Release Group}", "{Movie Title}{ Release Group}", "{Movie Title}{ [Release Group]}"} {
		cases = append(cases, nameCase{
			name: "no default release group with separator " + format, format: format,
			file: func(f *File) { f.ReleaseGroup = "" }, want: "South Park",
		})
	}
	for _, group := range []string{"0SEC", "2HD", "IMMERSE"} {
		cases = append(cases, nameCase{
			name: "existing casing for release group " + group, format: "{Release Group}",
			file: func(f *File) { f.ReleaseGroup = group }, want: group,
		})
	}
	languageCodes := []struct{ language, code string }{
		{"nob", "NB"},
		{"swe", "SV"},
		{"zho", "ZH"},
		{"chi", "ZH"},
		{"fre", "FR"},
		{"rum", "RO"},
		{"per", "FA"},
		{"ger", "DE"},
		{"gsw", "DE"},
		{"cze", "CS"},
		{"ice", "IS"},
		{"dut", "NL"},
		{"nor", "NO"},
		{"khk", "MN"},
		{"mvf", "MN"},
		{"geo", "KA"},
		{"kat", "KA"},
	}
	for _, lc := range languageCodes {
		cases = append(cases, nameCase{
			name: "format language code " + lc.language, format: "{Movie.Title}.{MEDIAINFO.FULL}",
			file: func(f *File) {
				f.MediaInfo = mediaInfo(*f, "h264", "dts", 6, "", []string{"eng"}, []string{lc.language})
			},
			want: "South.Park.H264.DTS.[" + lc.code + "]",
		})
	}
	audio := []struct{ format, languages, want string }{
		{"{MediaInfo AudioLanguages}", "eng", ""},
		{"{MediaInfo AudioLanguages}", "eng/deu", "[EN+DE]"},
		{"{MediaInfo AudioLanguagesAll}", "eng", "[EN]"},
		{"{MediaInfo AudioLanguagesAll}", "eng/deu", "[EN+DE]"},
	}
	for _, a := range audio {
		cases = append(cases, nameCase{
			name: "format audio languages " + a.format + " " + a.languages, format: a.format,
			file: func(f *File) {
				f.ReleaseGroup = ""
				givenMediaInfo("", a.languages, "eng/spa/ita")(f)
			},
			want: a.want,
		})
	}
	subtitles := []struct{ languages, filter, want string }{
		{"eng/deu", "", "[EN+DE]"},
		{"eng/nld/deu", "", "[EN+NL+DE]"},
		{"eng/deu", ":DE", "[DE]"},
		{"eng/nld/deu", ":EN+NL", "[EN+NL]"},
		{"eng/nld/deu", ":NL+EN", "[NL+EN]"},
		{"eng/nld/deu", ":-NL", "[EN+DE]"},
		{"eng/nld/deu", ":DE+", "[DE+-]"},
		{"eng/nld/deu", ":DE+NO.", "[DE]."},
		{"eng/nld/deu", ":-EN-", "[NL+DE]-"},
	}
	for _, s := range subtitles {
		cases = append(cases, nameCase{
			name: "format subtitle languages " + s.languages + s.filter, format: "{MediaInfo SubtitleLanguages" + s.filter + "}End",
			file: func(f *File) {
				f.ReleaseGroup = ""
				givenMediaInfo("", "eng", s.languages)(f)
			},
			want: s.want + "End",
		})
	}
	// HdrFormat.None, Hlg10 and Hdr10, as their dynamic range types.
	for _, h := range []struct{ hdrType, want string }{{"", "South.Park"}, {"HLG", "South.Park.HDR"}, {"HDR10", "South.Park.HDR"}} {
		cases = append(cases, nameCase{
			name: "hdr for videodynamicrange " + h.hdrType, format: "{Movie.Title}.{MediaInfo VideoDynamicRange}",
			file: givenMediaInfo(h.hdrType, "eng", "eng/spa/ita"), want: h.want,
		})
	}
	runFileCases(t, southParkFixture(), cases)
}

// Radarr's FileNameBuilderFixture cases that are not ported.
func TestBuildFileNameSkipped(t *testing.T) {
	skipped := []struct{ name, reason string }{
		{"should_replace_all_tokens_for_different_cultures(fr-FR, az, tr-TR)", "Go case mapping is culture-invariant; the en-US case is ported as \"replace all tokens\""},
		{"should_update_media_info_if_token_configured_and_revision_is_old", "media info is an input; refreshing stale media info belongs to the caller"},
		{"should_not_update_media_info_if_no_movie_path_available", "media info is an input; refreshing stale media info belongs to the caller"},
		{"should_not_update_media_info_if_token_not_configured_and_revision_is_old", "media info is an input; refreshing stale media info belongs to the caller"},
		{"should_not_update_media_info_if_token_configured_and_revision_is_current", "media info is an input; refreshing stale media info belongs to the caller"},
		{"should_not_update_media_info_if_token_configured_and_revision_is_newer", "media info is an input; refreshing stale media info belongs to the caller"},
	}
	for _, s := range skipped {
		t.Run(s.name, func(t *testing.T) { t.Skip(s.reason) })
	}
}

func TestBuildFileNameEmptyFormat(t *testing.T) {
	n := renaming()
	n.StandardMovieFormat = " "
	if _, err := BuildFileName(n, Movie{Title: "Movie"}, hdtv720p()); !errors.Is(err, ErrEmptyFormat) {
		t.Errorf("BuildFileName() error = %v, want = %v", err, ErrEmptyFormat)
	}
}
