package organize

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
)

// ColonReplacement is how a colon in a token value is written to a file
// name when illegal characters are replaced.
type ColonReplacement string

// Colon replacements.
const (
	// ColonDelete removes colons: "Movie: Title" becomes "Movie Title".
	ColonDelete ColonReplacement = "delete"
	// ColonDash replaces a colon with a dash: "Movie- Title".
	ColonDash ColonReplacement = "dash"
	// ColonSpaceDash replaces a colon with a space and a dash: "Movie - Title".
	ColonSpaceDash ColonReplacement = "space_dash"
	// ColonSpaceDashSpace replaces a colon with a space, a dash and a space:
	// "Movie:Title" becomes "Movie - Title".
	ColonSpaceDashSpace ColonReplacement = "space_dash_space"
	// ColonSmart replaces ": " with " - " and any other colon with "-".
	ColonSmart ColonReplacement = "smart"
)

// Valid reports whether c is one of the colon replacements.
func (c ColonReplacement) Valid() bool {
	switch c {
	case ColonDelete, ColonDash, ColonSpaceDash, ColonSpaceDashSpace, ColonSmart:
		return true
	}
	return false
}

// Naming is the movie naming configuration.
type Naming struct {
	// RenameMovies renames imported files with StandardMovieFormat; when
	// false a file keeps its scene or original file name.
	RenameMovies bool
	// ReplaceIllegalCharacters replaces characters that are illegal in
	// file names with legal ones instead of removing them.
	ReplaceIllegalCharacters bool
	// ColonReplacement is how colons are replaced when
	// ReplaceIllegalCharacters is set; otherwise they are removed.
	ColonReplacement ColonReplacement
	// MovieFolderFormat names the folder of a movie; "/" or "\" separate
	// nested folders.
	MovieFolderFormat string
	// StandardMovieFormat names a movie file, without its extension; "/"
	// or "\" separate subfolders.
	StandardMovieFormat string
}

// DefaultNaming returns Radarr's default naming configuration.
func DefaultNaming() Naming {
	return Naming{
		RenameMovies:             false,
		ReplaceIllegalCharacters: true,
		ColonReplacement:         ColonSmart,
		MovieFolderFormat:        "{Movie Title} ({Release Year})",
		StandardMovieFormat:      "{Movie Title} ({Release Year}) {Quality Full}",
	}
}

// Validation errors.
var (
	// ErrEmptyFormat is returned for an empty naming format.
	ErrEmptyFormat = errors.New("organize: naming format is empty")
	// ErrIllegalCharacters is returned for a format with characters that
	// are illegal in paths.
	ErrIllegalCharacters = errors.New("organize: naming format contains illegal characters")
	// ErrMissingTokens is returned for a format without the tokens that
	// identify a movie.
	ErrMissingTokens = errors.New("organize: naming format lacks required tokens")
	// ErrDeprecatedFolderTokens is returned for a folder format with
	// tokens derived from file properties.
	ErrDeprecatedFolderTokens = errors.New("organize: folder format contains tokens derived from file properties")
	// ErrInvalidColonReplacement is returned for an unknown colon
	// replacement.
	ErrInvalidColonReplacement = errors.New("organize: invalid colon replacement")
)

var (
	// movieTitleRegex matches the title tokens that identify a movie.
	movieTitleRegex = re(`(?<token>\{(?:Movie)(?<separator>[- ._])(?:Clean)?(?:OriginalTitle|Title(?:The)?)(?::(?<customFormat>[a-z0-9|-]+))?\})`, regexp2.IgnoreCase)
	// releaseYearRegex matches the release year token.
	releaseYearRegex = re(`\{[-{ ._\[(]*Release[- ._]Year[-} ._)\]]*\}`, regexp2.IgnoreCase)
	// originalTokenRegex matches the original title and file name tokens.
	originalTokenRegex = re(`(\{Original[- ._](?:Title|Filename)\})`, regexp2.IgnoreCase)
	// deprecatedMovieFolderTokensRegex matches tokens a folder cannot use.
	deprecatedMovieFolderTokensRegex = re(`(\{[- ._\[\(]?(?:Original[- ._](?:Title|Filename)|Release[- ._]Group|Edition[- ._]Tags|Quality[- ._](?:Full|Title|Proper|Real)|MediaInfo[- ._](?:Video|VideoCodec|VideoBitDepth|Audio|AudioCodec|AudioChannels|AudioLanguages|AudioLanguagesAll|SubtitleLanguages|SubtitleLanguagesAll|3D|Simple|Full|VideoDynamicRange|VideoDynamicRangeType))[- ._\]\)]?\})`, regexp2.IgnoreCase)
)

// Validate checks both formats and the colon replacement.
func (n Naming) Validate() error {
	if !n.ColonReplacement.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidColonReplacement, n.ColonReplacement)
	}
	if err := ValidateMovieFormat(n.StandardMovieFormat); err != nil {
		return fmt.Errorf("standard movie format: %w", err)
	}
	if err := ValidateMovieFolderFormat(n.MovieFolderFormat); err != nil {
		return fmt.Errorf("movie folder format: %w", err)
	}
	return nil
}

// ValidateMovieFormat checks a movie file format: it must contain a movie
// title and the release year, or the original title or file name.
func ValidateMovieFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	original := originalTokenRegex.isMatch(format)
	if !original && (!movieTitleRegex.isMatch(format) || !releaseYearRegex.isMatch(format)) {
		return fmt.Errorf("%w: must contain either movie title and release year or original title/filename", ErrMissingTokens)
	}
	return nil
}

// ValidateMovieFolderFormat checks a movie folder format: it must contain
// a movie title and no token derived from the file.
func ValidateMovieFolderFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if tokens := deprecatedMovieFolderTokensRegex.matches(format); len(tokens) > 0 {
		return fmt.Errorf("%w: %s", ErrDeprecatedFolderTokens, strings.Join(tokens, ", "))
	}
	if !movieTitleRegex.isMatch(format) {
		return fmt.Errorf("%w: must contain movie title", ErrMissingTokens)
	}
	return nil
}

func validateCommon(format string) error {
	if strings.TrimSpace(format) == "" {
		return ErrEmptyFormat
	}
	var illegal []rune
	for _, r := range format {
		if r < 0x20 {
			illegal = append(illegal, r)
		}
	}
	if len(illegal) > 0 {
		return fmt.Errorf("%w: %q", ErrIllegalCharacters, string(illegal))
	}
	return nil
}
