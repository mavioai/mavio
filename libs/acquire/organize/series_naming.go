package organize

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
)

// MultiEpisodeStyle is how a file holding several episodes writes their
// numbers.
type MultiEpisodeStyle string

// Multi-episode styles, shown for episodes 6 to 8 of season 15 in the
// format "S{season:00}E{episode:00}".
const (
	// MultiEpisodeExtend appends the other episode numbers: S15E06-07-08.
	MultiEpisodeExtend MultiEpisodeStyle = "extend"
	// MultiEpisodeDuplicate repeats the whole pattern, joined by the
	// separator before or after it: S15E06 - S15E07 - S15E08.
	MultiEpisodeDuplicate MultiEpisodeStyle = "duplicate"
	// MultiEpisodeRepeat repeats the episode part: S15E06E07E08.
	MultiEpisodeRepeat MultiEpisodeStyle = "repeat"
	// MultiEpisodeScene appends the episode parts after dashes:
	// S15E06-E07-E08.
	MultiEpisodeScene MultiEpisodeStyle = "scene"
	// MultiEpisodeRange writes the first and last episode: S15E06-08.
	MultiEpisodeRange MultiEpisodeStyle = "range"
	// MultiEpisodePrefixedRange writes the first and last episode with
	// the episode prefix: S15E06-E08.
	MultiEpisodePrefixedRange MultiEpisodeStyle = "prefixed_range"
)

// Valid reports whether s is one of the multi-episode styles.
func (s MultiEpisodeStyle) Valid() bool {
	switch s {
	case MultiEpisodeExtend, MultiEpisodeDuplicate, MultiEpisodeRepeat, MultiEpisodeScene,
		MultiEpisodeRange, MultiEpisodePrefixedRange:
		return true
	}
	return false
}

// SeriesNaming is the series naming configuration.
type SeriesNaming struct {
	// RenameEpisodes renames imported files with the episode formats;
	// when false a file keeps its scene or original file name.
	RenameEpisodes bool
	// ReplaceIllegalCharacters replaces characters that are illegal in
	// file names with legal ones instead of removing them.
	ReplaceIllegalCharacters bool
	// ColonReplacement is how colons are replaced when
	// ReplaceIllegalCharacters is set; otherwise they are removed.
	ColonReplacement ColonReplacement
	// CustomColonReplacement replaces colons for ColonCustom.
	CustomColonReplacement string
	// MultiEpisodeStyle is how a file of several episodes writes their
	// numbers.
	MultiEpisodeStyle MultiEpisodeStyle
	// StandardEpisodeFormat names an episode file of a standard series,
	// without its extension; "/" or "\" separate subfolders.
	StandardEpisodeFormat string
	// DailyEpisodeFormat names an episode file of a daily series outside
	// the specials.
	DailyEpisodeFormat string
	// AnimeEpisodeFormat names an episode file of an anime series.
	AnimeEpisodeFormat string
	// SeriesFolderFormat names the folder of a series.
	SeriesFolderFormat string
	// SeasonFolderFormat names the folder of a season.
	SeasonFolderFormat string
	// SpecialsFolderFormat names the folder of season 0.
	SpecialsFolderFormat string
}

// DefaultSeriesNaming returns Sonarr's default naming configuration.
func DefaultSeriesNaming() SeriesNaming {
	return SeriesNaming{
		RenameEpisodes:           false,
		ReplaceIllegalCharacters: true,
		ColonReplacement:         ColonSmart,
		CustomColonReplacement:   "",
		MultiEpisodeStyle:        MultiEpisodePrefixedRange,
		StandardEpisodeFormat:    "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
		DailyEpisodeFormat:       "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
		AnimeEpisodeFormat:       "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
		SeriesFolderFormat:       "{Series Title}",
		SeasonFolderFormat:       "Season {season}",
		SpecialsFolderFormat:     "Specials",
	}
}

// cleaning returns how n replaces illegal characters.
func (n SeriesNaming) cleaning() charCleaning {
	return charCleaning{
		replaceIllegal: n.ReplaceIllegalCharacters,
		colon:          n.ColonReplacement,
		customColon:    n.CustomColonReplacement,
	}
}

// ErrInvalidMultiEpisodeStyle is returned for an unknown multi-episode
// style.
var ErrInvalidMultiEpisodeStyle = errors.New("organize: invalid multi-episode style")

var (
	// seasonEpisodePatternRegex matches a season and episode pair such as
	// "S{season:00}E{episode:00}" with the separators around it.
	seasonEpisodePatternRegex = re(`(?<separator>(?<=})[- ._]+?)?(?<seasonEpisode>s?{season(?:\:0+)?}(?<episodeSeparator>[- ._]?[ex])(?<episode>{episode(?:\:0+)?}))(?<separator>[- ._]+?(?={))?`, regexp2.IgnoreCase)
	// absoluteEpisodePatternRegex matches an absolute episode number with
	// the separators around it.
	absoluteEpisodePatternRegex = re(`(?<separator>(?<=})[- ._]+?)?(?<absolute>{absolute(?:\:0+)?})(?<separator>[- ._]+?(?={))?`, regexp2.IgnoreCase)
	// seasonRegex matches a season number.
	seasonRegex = re(`(?<season>\{season(?:\:0+)?})`, regexp2.IgnoreCase)
	// episodeRegex matches an episode number.
	episodeRegex = re(`(?<episode>\{episode(?:\:0+)?})`, regexp2.IgnoreCase)
	// absoluteEpisodeRegex matches an absolute episode number.
	absoluteEpisodeRegex = re(`(?<absolute>\{absolute(?:\:0+)?})`, regexp2.IgnoreCase)
	// airDateRegex matches the air date token.
	airDateRegex = re(`\{Air(\s|\W|_)Date\}`, regexp2.IgnoreCase)
	// seriesTitleRegex matches the title tokens that identify a series.
	seriesTitleRegex = re(`(?<token>\{(?:Series)(?<separator>[- ._])(Clean)?Title(The)?(Without)?(Year)?(?::(?<customFormat>[0-9-]+))?\})`, regexp2.IgnoreCase)
	// seasonFolderRegex matches the season number of a season folder.
	seasonFolderRegex = re(`(\{season(\:\d+)?\})`, regexp2.IgnoreCase)
)

// Validate checks the formats, the colon replacement and the
// multi-episode style.
func (n SeriesNaming) Validate() error {
	if !n.ColonReplacement.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidColonReplacement, n.ColonReplacement)
	}
	if n.ColonReplacement == ColonCustom {
		if err := ValidateCustomColonReplacement(n.CustomColonReplacement); err != nil {
			return fmt.Errorf("custom colon replacement: %w", err)
		}
	}
	if !n.MultiEpisodeStyle.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidMultiEpisodeStyle, n.MultiEpisodeStyle)
	}
	checks := []struct {
		name     string
		format   string
		validate func(string) error
	}{
		{"standard episode format", n.StandardEpisodeFormat, ValidateStandardEpisodeFormat},
		{"daily episode format", n.DailyEpisodeFormat, ValidateDailyEpisodeFormat},
		{"anime episode format", n.AnimeEpisodeFormat, ValidateAnimeEpisodeFormat},
		{"series folder format", n.SeriesFolderFormat, ValidateSeriesFolderFormat},
		{"season folder format", n.SeasonFolderFormat, ValidateSeasonFolderFormat},
		{"specials folder format", n.SpecialsFolderFormat, ValidateSpecialsFolderFormat},
	}
	for _, c := range checks {
		if err := c.validate(c.format); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}

// hasSeasonAndEpisode reports whether format numbers an episode by season
// and episode or keeps the original title.
func hasSeasonAndEpisode(format string) bool {
	return seasonEpisodePatternRegex.isMatch(format) ||
		(seasonRegex.isMatch(format) && episodeRegex.isMatch(format)) ||
		originalTokenRegex.isMatch(format)
}

// ValidateStandardEpisodeFormat checks a standard episode format: it must
// contain season and episode numbers or the original title or file name.
func ValidateStandardEpisodeFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if !hasSeasonAndEpisode(format) {
		return fmt.Errorf("%w: must contain season and episode numbers or original title", ErrMissingTokens)
	}
	return nil
}

// ValidateDailyEpisodeFormat checks a daily episode format: it must
// contain the air date, season and episode numbers, or the original title
// or file name.
func ValidateDailyEpisodeFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if !hasSeasonAndEpisode(format) && !airDateRegex.isMatch(format) {
		return fmt.Errorf("%w: must contain air date or season and episode or original title", ErrMissingTokens)
	}
	return nil
}

// ValidateAnimeEpisodeFormat checks an anime episode format: it must
// contain the absolute episode number, season and episode numbers, or the
// original title or file name.
func ValidateAnimeEpisodeFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if !hasSeasonAndEpisode(format) && !absoluteEpisodePatternRegex.isMatch(format) {
		return fmt.Errorf("%w: must contain absolute episode number or season and episode or original title", ErrMissingTokens)
	}
	return nil
}

// ValidateSeriesFolderFormat checks a series folder format: it must
// contain a series title.
func ValidateSeriesFolderFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if !seriesTitleRegex.isMatch(format) {
		return fmt.Errorf("%w: must contain series title", ErrMissingTokens)
	}
	return nil
}

// ValidateSeasonFolderFormat checks a season folder format: it must
// contain the season number.
func ValidateSeasonFolderFormat(format string) error {
	if err := validateCommon(format); err != nil {
		return err
	}
	if !seasonFolderRegex.isMatch(format) {
		return fmt.Errorf("%w: must contain season number", ErrMissingTokens)
	}
	return nil
}

// ValidateSpecialsFolderFormat checks a specials folder format, which
// needs no token.
func ValidateSpecialsFolderFormat(format string) error {
	return validateCommon(format)
}

// ValidateCustomColonReplacement checks a custom colon replacement: it may
// be empty but must not contain a colon or a character illegal in file
// names.
func ValidateCustomColonReplacement(replacement string) error {
	if strings.TrimSpace(replacement) == "" {
		return nil
	}
	var illegal []string
	for _, bad := range append(badCharacters[:len(badCharacters):len(badCharacters)], ":") {
		if strings.Contains(replacement, bad) {
			illegal = append(illegal, bad)
		}
	}
	for _, r := range replacement {
		if r < 0x20 {
			illegal = append(illegal, string(r))
		}
	}
	if len(illegal) > 0 {
		return fmt.Errorf("%w: %q", ErrIllegalCharacters, strings.Join(illegal, ""))
	}
	return nil
}
