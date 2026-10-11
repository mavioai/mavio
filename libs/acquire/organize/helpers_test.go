package organize

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// nameCase is a file or folder naming case: format replaces the format
// under test, and the functions adjust the fixture's movie, file and
// configuration.
type nameCase struct {
	name   string
	format string
	naming func(n *Naming)
	movie  func(m *Movie)
	file   func(f *File)
	want   string
}

// fixture is the starting state of a ported Radarr fixture.
type fixture struct {
	naming Naming
	movie  Movie
	file   File
}

func (c nameCase) apply(fx fixture) (Naming, Movie, File) {
	n, m, f := fx.naming, fx.movie, fx.file
	m.Translations = append([]Translation(nil), m.Translations...)
	f.CustomFormats = append([]CustomFormat(nil), f.CustomFormats...)
	if c.naming != nil {
		c.naming(&n)
	}
	if c.movie != nil {
		c.movie(&m)
	}
	if c.file != nil {
		c.file(&f)
	}
	return n, m, f
}

// runFileCases builds file names with the cases' formats as
// StandardMovieFormat.
func runFileCases(t *testing.T, fx fixture, cases []nameCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, m, f := c.apply(fx)
			if c.format != "" {
				n.StandardMovieFormat = c.format
			}
			got, err := BuildFileName(n, m, f)
			if err != nil {
				t.Fatalf("BuildFileName(%q) error = %v", n.StandardMovieFormat, err)
			}
			if got != c.want {
				t.Errorf("BuildFileName(%q) = %q, want = %q", n.StandardMovieFormat, got, c.want)
			}
		})
	}
}

// runFolderCases builds folder names with the cases' formats as
// MovieFolderFormat.
func runFolderCases(t *testing.T, fx fixture, cases []nameCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, m, _ := c.apply(fx)
			if c.format != "" {
				n.MovieFolderFormat = c.format
			}
			if got := MovieFolder(n, m); got != c.want {
				t.Errorf("MovieFolder(%q) = %q, want = %q", n.MovieFolderFormat, got, c.want)
			}
		})
	}
}

// renaming returns the default configuration with renaming enabled.
func renaming() Naming {
	n := DefaultNaming()
	n.RenameMovies = true
	return n
}

// hdtv720p is the file most Radarr fixtures name.
func hdtv720p() File { return File{Quality: "HDTV-720p", ReleaseGroup: "RadarrTest"} }

func withTitle(title string) func(m *Movie) { return func(m *Movie) { m.Title = title } }

func withCollection(title string) func(m *Movie) { return func(m *Movie) { m.CollectionTitle = title } }

// mediaInfo formats media information as Radarr's GivenMediaInfoModel
// helper describes it, with ffprobe codec names.
func mediaInfo(f File, videoFormat, audioFormat string, audioChannels float64, hdrType string, audio, subtitles []string) *MediaInfo {
	scene := f.SceneOrFileName()
	return &MediaInfo{
		VideoCodec:            FormatVideoCodec(videoFormat, "", scene),
		VideoBitDepth:         8,
		VideoDynamicRangeType: hdrType,
		AudioCodec:            FormatAudioCodec(audioFormat, "", ""),
		AudioChannels:         audioChannels,
		AudioLanguages:        audio,
		SubtitleLanguages:     subtitles,
	}
}

var southParkTranslations = []Translation{
	{Language: release.German, Title: "German South Park"},
	{Language: release.French, Title: "French South Park"},
}
