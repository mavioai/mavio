package organize

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// SeriesType decides which episode format names a series' files.
type SeriesType string

// Series types.
const (
	// SeriesStandard numbers episodes by season and episode.
	SeriesStandard SeriesType = "standard"
	// SeriesDaily identifies episodes by air date.
	SeriesDaily SeriesType = "daily"
	// SeriesAnime also numbers episodes absolutely.
	SeriesAnime SeriesType = "anime"
)

// Series is the series an episode file or folder is named after.
type Series struct {
	// Title is the series title.
	Title string
	// Year is the year the series started; zero when unknown.
	Year int
	// Type is the series type; "" counts as SeriesStandard.
	Type SeriesType
	// TvdbID is the TheTVDB ID.
	TvdbID int
	// TvMazeID is the TVmaze ID; zero when unknown.
	TvMazeID int
	// TmdbID is the TMDB ID; zero when unknown.
	TmdbID int
	// ImdbID is the IMDb ID, such as "tt0121955".
	ImdbID string
	// Path is the folder of the series.
	Path string
	// SeasonFolder puts episode files in season folders.
	SeasonFolder bool
}

// Episode is an episode an episode file holds.
type Episode struct {
	// SeasonNumber is the season number; 0 is the specials.
	SeasonNumber int
	// EpisodeNumber is the episode number within the season.
	EpisodeNumber int
	// AbsoluteEpisodeNumber is the episode number across seasons; zero
	// when unknown.
	AbsoluteEpisodeNumber int
	// AirDate is the date the episode first aired as YYYY-MM-DD; empty
	// when unknown.
	AirDate string
	// Title is the episode title.
	Title string
}

// EpisodeFile is the episode file being named.
type EpisodeFile struct {
	// Quality is the title of the quality definition, such as
	// "HDTV-720p"; empty when the quality is unknown.
	Quality string
	// Revision is the revision of the quality; an anime proper is written
	// as "v2" and so on.
	Revision Revision
	// ReleaseGroup is the release group.
	ReleaseGroup string
	// ReleaseHash is the hash some anime releases carry in their names.
	ReleaseHash string
	// SceneName is the release name the file was grabbed as.
	SceneName string
	// RelativePath is the path of the file relative to the series folder.
	RelativePath string
	// Path is the absolute path of the file.
	Path string
	// Imported reports a file already in the library, whose current file
	// name came from an earlier rename: a format that identifies the
	// episode then leaves {Original Title} and {Original Filename} empty.
	Imported bool
	// CustomFormats are the custom formats the file matches.
	CustomFormats []CustomFormat
	// MediaInfo is the media information of the file; nil when unknown,
	// which leaves every MediaInfo token empty.
	MediaInfo *MediaInfo
}

// SceneOrFileName returns the scene name of the file, or its file name
// without the extension.
func (f EpisodeFile) SceneOrFileName() string {
	return sceneOrFileName(f.SceneName, f.RelativePath, f.Path)
}

// Path limits in bytes, as Sonarr applies them on Linux.
const (
	maxFileNameBytes = 255
	maxFilePathBytes = 4096
)

// Episode naming errors.
var (
	// ErrNoEpisodes is returned when naming a file of no episodes.
	ErrNoEpisodes = errors.New("organize: no episodes to name the file after")
	// ErrNoExtension is returned when building a file path without an
	// extension.
	ErrNoExtension = errors.New("organize: file extension is empty")
)

var (
	// episodeTitleRegex matches a token as Sonarr writes them, or an
	// escaped "{{" or "}}".
	episodeTitleRegex = re(`(?<escaped>\{\{|\}\})|\{(?<prefix>[- ._\[(]*)(?<token>(?:[a-z0-9]+)(?:(?<separator>[- ._]+)(?:[a-z0-9]+))?)(?::(?<customFormat>[ ,a-z0-9+-]+(?<![- ])))?(?<suffix>[- ._)\]]*)\}`, regexp2.IgnoreCase)
	// multiPartCleanupRegex matches the part number ending the title of
	// one part of a multi-part episode.
	multiPartCleanupRegex = re(`(?:\:?\s?(?:\(\d+\)|(Part|Pt\.?)\s?\d+))$`, regexp2.IgnoreCase)
	// yearRegex matches a parenthesized year ending a title.
	yearRegex = re(`\(\d{4}\)$`, regexp2.IgnoreCase)
)

// escapedEllipsis marks an ellipsis of the format through the first pass
// of token replacement.
const escapedEllipsis = "{{ellipsis}}"

// BuildEpisodeFileName names an episode file, without its extension, with
// the format of the series type; components of a format with "/" or "\"
// are joined with "/". When n.RenameEpisodes is false the file keeps its
// scene or original file name.
func BuildEpisodeFileName(n SeriesNaming, s Series, episodes []Episode, f EpisodeFile) (string, error) {
	return buildEpisodeFileName(n, s, episodes, f, "", maxFilePathBytes)
}

// BuildEpisodeFilePath returns the path of an episode file in the folder
// of its season (see SeasonPath), named by BuildEpisodeFileName with the
// extension, such as ".mkv". Long episode titles are shortened so that
// the file name with its extension stays within 255 bytes.
func BuildEpisodeFilePath(n SeriesNaming, s Series, episodes []Episode, f EpisodeFile, extension string) (string, error) {
	if strings.TrimSpace(extension) == "" {
		return "", ErrNoExtension
	}
	if len(episodes) == 0 {
		return "", ErrNoEpisodes
	}
	seasonPath := SeasonPath(n, s, episodes[0].SeasonNumber)
	name, err := buildEpisodeFileName(n, s, episodes, f, extension, maxFilePathBytes-len(seasonPath)-1)
	if err != nil {
		return "", err
	}
	if seasonPath == "" {
		return name, nil
	}
	return strings.TrimRight(seasonPath, "/") + "/" + name, nil
}

// SeasonPath returns the folder of a season: the series path, joined with
// the season folder when s.SeasonFolder is set.
func SeasonPath(n SeriesNaming, s Series, seasonNumber int) string {
	if !s.SeasonFolder {
		return s.Path
	}
	var components []string
	for _, c := range strings.Split(SeasonFolder(n, s, seasonNumber), "/") {
		if c = CleanFileName(c); c != "" {
			components = append(components, c)
		}
	}
	folder := strings.Join(components, "/")
	switch {
	case s.Path == "":
		return folder
	case folder == "":
		return s.Path
	}
	return strings.TrimRight(s.Path, "/") + "/" + folder
}

// SeriesFolder names the folder of a series with n.SeriesFolderFormat;
// components of a format with "/" or "\" are joined with "/".
func SeriesFolder(n SeriesNaming, s Series) string {
	h := handlers{}
	addSeriesTokens(h, s)
	addSeriesIDTokens(h, s)
	return folderName(n.SeriesFolderFormat, h, n.cleaning())
}

// SeasonFolder names the folder of a season with n.SeasonFolderFormat, or
// n.SpecialsFolderFormat for season 0; components of a format with "/" or
// "\" are joined with "/".
func SeasonFolder(n SeriesNaming, s Series, seasonNumber int) string {
	h := handlers{}
	addSeriesTokens(h, s)
	addSeriesIDTokens(h, s)
	addSeasonTokens(h, seasonNumber)
	format := n.SeasonFolderFormat
	if seasonNumber == 0 {
		format = n.SpecialsFolderFormat
	}
	return folderName(format, h, n.cleaning())
}

// RequiresEpisodeTitle reports whether the format that names the episodes
// of s contains an episode title, so that a file waits for the title to
// be known before it is renamed.
func RequiresEpisodeTitle(n SeriesNaming, s Series, episodes []Episode) bool {
	if !n.RenameEpisodes {
		return false
	}
	pattern := n.StandardEpisodeFormat
	switch {
	case s.Type == SeriesDaily:
		pattern = n.DailyEpisodeFormat
	case s.Type == SeriesAnime && allAbsolute(episodes):
		pattern = n.AnimeEpisodeFormat
	}
	for _, m := range episodeTitleRegex.all(pattern) {
		switch simplifyToken(group(*m, "token")) {
		case "episodetitle", "episodecleantitle":
			return true
		}
	}
	return false
}

// RequiresAbsoluteEpisodeNumber reports whether the anime episode format
// contains the absolute episode number.
func RequiresAbsoluteEpisodeNumber(n SeriesNaming) bool {
	return absoluteEpisodeRegex.isMatch(n.AnimeEpisodeFormat)
}

func allAbsolute(episodes []Episode) bool {
	for _, e := range episodes {
		if e.AbsoluteEpisodeNumber == 0 {
			return false
		}
	}
	return true
}

func buildEpisodeFileName(n SeriesNaming, s Series, episodes []Episode, f EpisodeFile, extension string, maxPath int) (string, error) {
	if !n.RenameEpisodes {
		return originalTitle(f.SceneName, f.RelativePath, f.Path, true) + extension, nil
	}
	switch {
	case s.Type == SeriesDaily && strings.TrimSpace(n.DailyEpisodeFormat) == "":
		return "", fmt.Errorf("daily episode format: %w", ErrEmptyFormat)
	case s.Type == SeriesAnime && strings.TrimSpace(n.AnimeEpisodeFormat) == "":
		return "", fmt.Errorf("anime episode format: %w", ErrEmptyFormat)
	case s.Type != SeriesDaily && s.Type != SeriesAnime && strings.TrimSpace(n.StandardEpisodeFormat) == "":
		return "", fmt.Errorf("standard episode format: %w", ErrEmptyFormat)
	}
	if len(episodes) == 0 {
		return "", ErrNoEpisodes
	}
	eps := slices.Clone(episodes)
	slices.SortStableFunc(eps, func(a, b Episode) int {
		return cmp.Or(cmp.Compare(a.SeasonNumber, b.SeasonNumber), cmp.Compare(a.EpisodeNumber, b.EpisodeNumber))
	})

	pattern := n.StandardEpisodeFormat
	if s.Type == SeriesDaily && eps[0].SeasonNumber > 0 {
		pattern = n.DailyEpisodeFormat
	}
	if s.Type == SeriesAnime && (allAbsolute(eps) || !RequiresAbsoluteEpisodeNumber(n)) {
		pattern = n.AnimeEpisodeFormat
	}

	c := n.cleaning()
	parts := splitPattern(pattern)
	var components []string
	for i, p := range parts {
		h := handlers{}
		hasIdentifier := seasonEpisodePatternRegex.isMatch(p) || absoluteEpisodePatternRegex.isMatch(p) || airDateRegex.isMatch(p)

		p = addSeasonEpisodeNumberingTokens(p, h, eps, n.MultiEpisodeStyle)
		p = addAbsoluteNumberingTokens(p, h, s, eps, n.MultiEpisodeStyle)
		p = strings.ReplaceAll(p, "...", escapedEllipsis)

		addSeriesTokens(h, s)
		addSeriesIDTokens(h, s)
		addAirDateTokens(h, eps)
		// The episode titles are written in a second pass, once the length
		// left for them is known.
		h.preserve("{Episode Title}")
		h.preserve("{Episode CleanTitle}")
		addEpisodeFileTokens(h, f, !hasIdentifier || !f.Imported)
		addQualityTokens(h, f.Quality, f.Revision, s.Type == SeriesAnime)
		addMediaInfoTokens(h, f.MediaInfo)
		addCustomFormatTokens(h, f.CustomFormats)

		component := strings.TrimSpace(replaceEpisodeTokens(p, h, c, true))
		maxSegment := min(maxFileNameBytes, maxPath)
		if i == len(parts)-1 {
			maxSegment -= len(extension)
		}
		addEpisodeTitleTokens(h, eps, maxSegment-lengthWithoutEpisodeTitle(component, c))
		component = strings.TrimSpace(replaceEpisodeTokens(component, h, c, false))

		component = collapseSeparators(component)
		component = strings.TrimRight(component, "- ._")
		component = strings.ReplaceAll(component, ellipsisPlaceholder, "...")
		component = replaceReservedDeviceNames(component)
		if strings.TrimSpace(component) != "" {
			components = append(components, component)
		}
	}
	return strings.Join(components, "/") + extension, nil
}

// folderName fills a series or season folder format.
func folderName(format string, h handlers, c charCleaning) string {
	var components []string
	for _, p := range splitPattern(format) {
		name := CleanFolderName(replaceEpisodeTokens(p, h, c, false))
		name = replaceReservedDeviceNames(name)
		name = strings.ReplaceAll(name, ellipsisPlaceholder, "...")
		if strings.TrimSpace(name) != "" {
			components = append(components, name)
		}
	}
	return strings.Join(components, "/")
}

// preserve makes a token keep its text when replaced, to be filled by a
// later pass.
func (h handlers) preserve(token string) { h[simplifyToken(token)] = nil }

// replaceEpisodeTokens replaces the tokens of a series format. "{{" and
// "}}" write a brace; with escape they are kept, and braces in the token
// values are doubled, for a second pass.
func replaceEpisodeTokens(pattern string, h handlers, c charCleaning, escape bool) string {
	return episodeTitleRegex.replaceFunc(pattern, func(m regexp2.Match) string {
		if g := m.GroupByName("escaped"); g != nil && len(g.Captures) > 0 {
			switch {
			case escape:
				return m.String()
			case m.String() == "{{":
				return "{"
			}
			return "}"
		}
		t := newTokenMatch(m)
		f, ok := h[simplifyToken(t.token)]
		if ok && f == nil {
			return m.String()
		}
		text := ""
		if ok {
			text = f(t)
		}
		text = formatTokenValue(t, text, c)
		if escape {
			text = strings.ReplaceAll(strings.ReplaceAll(text, "{", "{{"), "}", "}}")
		}
		return text
	})
}

// lengthWithoutEpisodeTitle returns the length in bytes of a component
// whose episode titles are empty.
func lengthWithoutEpisodeTitle(component string, c charCleaning) int {
	h := handlers{}
	h.set("{Episode Title}", func(tokenMatch) string { return "" })
	h.set("{Episode CleanTitle}", func(tokenMatch) string { return "" })
	h.set("{ellipsis}", func(tokenMatch) string { return "..." })
	return len(replaceEpisodeTokens(component, h, c, false))
}

func addSeriesTokens(h handlers, s Series) {
	title, year := s.Title, s.Year
	h.set("{Series Title}", func(t tokenMatch) string { return truncate(title, t.customFormat) })
	h.set("{Series CleanTitle}", func(t tokenMatch) string { return truncate(scenify(title), t.customFormat) })
	h.set("{Series TitleYear}", func(t tokenMatch) string { return truncate(titleYear(title, year), t.customFormat) })
	h.set("{Series CleanTitleYear}", func(t tokenMatch) string {
		return truncate(scenify(titleYear(title, year)), t.customFormat)
	})
	h.set("{Series TitleWithoutYear}", func(t tokenMatch) string { return truncate(titleWithoutYear(title), t.customFormat) })
	h.set("{Series CleanTitleWithoutYear}", func(t tokenMatch) string {
		return truncate(scenify(titleWithoutYear(title)), t.customFormat)
	})
	h.set("{Series TitleThe}", func(t tokenMatch) string { return truncate(TitleThe(title), t.customFormat) })
	h.set("{Series CleanTitleThe}", func(t tokenMatch) string {
		return truncate(cleanTitleTheWith(title, scenify), t.customFormat)
	})
	h.set("{Series TitleTheYear}", func(t tokenMatch) string {
		return truncate(titleYear(TitleThe(title), year), t.customFormat)
	})
	h.set("{Series CleanTitleTheYear}", func(t tokenMatch) string {
		return truncate(cleanTitleTheYear(title, year), t.customFormat)
	})
	h.set("{Series TitleTheWithoutYear}", func(t tokenMatch) string {
		return truncate(titleWithoutYear(TitleThe(title)), t.customFormat)
	})
	h.set("{Series CleanTitleTheWithoutYear}", func(t tokenMatch) string {
		return truncate(cleanTitleTheWith(titleWithoutYear(title), scenify), t.customFormat)
	})
	h.set("{Series TitleFirstCharacter}", func(t tokenMatch) string {
		return truncate(TitleFirstCharacter(TitleThe(title)), t.customFormat)
	})
	h.set("{Series Year}", func(tokenMatch) string { return strconv.Itoa(year) })
}

// titleYear appends the year in parentheses to a title that does not end
// in a parenthesized year; a zero year is not written.
func titleYear(title string, year int) string {
	if year == 0 || yearRegex.isMatch(title) {
		return title
	}
	return fmt.Sprintf("%s (%d)", title, year)
}

// cleanTitleTheYear is the cleaned TitleThe of a title followed by its
// year, or by the year the title ends with.
func cleanTitleTheYear(title string, year int) string {
	if year == 0 {
		return cleanTitleTheWith(title, scenify)
	}
	if m := yearRegex.match(title); m != nil {
		return cleanTitleTheWith(strings.TrimSpace(title[:m.Index]), scenify) + " " + m.String()[1:5]
	}
	return cleanTitleTheWith(title, scenify) + " " + strconv.Itoa(year)
}

// titleWithoutYear removes a parenthesized year ending a title.
func titleWithoutYear(title string) string { return yearRegex.replace(title, "") }

func addSeriesIDTokens(h handlers, s Series) {
	h.set("{ImdbId}", func(tokenMatch) string { return s.ImdbID })
	h.set("{TvdbId}", func(tokenMatch) string { return strconv.Itoa(s.TvdbID) })
	h.set("{TvMazeId}", func(tokenMatch) string { return positive(s.TvMazeID) })
	h.set("{TmdbId}", func(tokenMatch) string { return positive(s.TmdbID) })
}

// positive formats a positive number, and writes nothing otherwise.
func positive(n int) string {
	if n > 0 {
		return strconv.Itoa(n)
	}
	return ""
}

func addSeasonTokens(h handlers, seasonNumber int) {
	h.set("{Season}", func(t tokenMatch) string { return formatNumber(seasonNumber, t.customFormat) })
}

func addAirDateTokens(h handlers, episodes []Episode) {
	airDate := "Unknown"
	if d := strings.TrimSpace(episodes[0].AirDate); d != "" {
		airDate = strings.ReplaceAll(d, "-", " ")
	}
	h.set("{Air Date}", func(tokenMatch) string { return airDate })
}

func addEpisodeFileTokens(h handlers, f EpisodeFile, useFileName bool) {
	h.set("{Original Title}", func(tokenMatch) string {
		return originalTitle(f.SceneName, f.RelativePath, f.Path, useFileName)
	})
	h.set("{Original Filename}", func(tokenMatch) string {
		return originalFileName(f.RelativePath, f.Path, useFileName)
	})
	addReleaseGroupToken(h, f.ReleaseGroup)
	h.set("{Release Hash}", func(tokenMatch) string { return f.ReleaseHash })
}

func addEpisodeTitleTokens(h handlers, episodes []Episode, maxLength int) {
	h.set("{Episode Title}", func(t tokenMatch) string {
		return joinEpisodeTitles(episodeTitles(episodes), "+", maxLength, t.customFormat)
	})
	h.set("{Episode CleanTitle}", func(t tokenMatch) string {
		titles := episodeTitles(episodes)
		for i, title := range titles {
			titles[i] = scenify(title)
		}
		return joinEpisodeTitles(titles, "and", maxLength, t.customFormat)
	})
}

// episodeTitles returns the distinct titles of the episodes, without
// trailing spaces, periods and question marks; titles of several episodes
// lose their part numbers unless no title is left.
func episodeTitles(episodes []Episode) []string {
	trim := func(title string) string { return strings.TrimRight(title, " .?") }
	if len(episodes) == 1 {
		return []string{trim(episodes[0].Title)}
	}
	var titles, trimmed []string
	blank := true
	for _, e := range episodes {
		title := strings.TrimSpace(multiPartCleanupRegex.replace(trim(e.Title), ""))
		titles = append(titles, title)
		trimmed = append(trimmed, trim(e.Title))
		blank = blank && strings.TrimSpace(title) == ""
	}
	if blank {
		return distinct(trimmed)
	}
	return distinct(titles)
}

// joinEpisodeTitles joins titles with " separator " within maxLength
// bytes, or the token's length when shorter; titles too long keep the
// first and last title around an ellipsis, then the first title, then
// its beginning.
func joinEpisodeTitles(titles []string, separator string, maxLength int, formatter string) string {
	if formatterLength := maxLengthFromFormatter(formatter); formatterLength > 0 {
		maxLength = min(maxLength, formatterLength)
	}
	joined := strings.Join(titles, " "+strings.TrimSpace(separator)+" ")
	if len(joined) <= maxLength {
		return joined
	}
	first := titles[0]
	if len(titles) >= 2 {
		last := titles[len(titles)-1]
		if len(first)+len(last)+3 <= maxLength {
			return strings.TrimRight(first, " .") + ellipsisPlaceholder + last
		}
	}
	if len(titles) > 1 && len(first)+3 <= maxLength {
		return strings.TrimRight(first, " .") + ellipsisPlaceholder
	}
	if len(titles) == 1 && len(first) <= maxLength {
		return first
	}
	return strings.TrimRight(truncateBytes(first, maxLength-3), " .") + ellipsisPlaceholder
}

// truncateBytes returns the longest prefix of s of whole characters within
// maxBytes bytes.
func truncateBytes(s string, maxBytes int) string {
	n := 0
	for i, r := range s {
		size := utf8.RuneLen(r)
		if size < 0 {
			size = 3 // U+FFFD, as an invalid byte is written.
		}
		if n+size > maxBytes {
			return s[:i]
		}
		n += size
	}
	return s
}

// episodeFormat is a season and episode pattern found in a format.
type episodeFormat struct {
	separator, episodeSeparator, episodePattern, seasonEpisodePattern string
}

// absoluteFormat is an absolute episode pattern found in a format.
type absoluteFormat struct {
	separator, pattern string
}

// addSeasonEpisodeNumberingTokens replaces each season and episode pattern
// of a format with a token writing the numbers of the episodes in style,
// and adds the {Season} and {Episode} tokens.
func addSeasonEpisodeNumberingTokens(pattern string, h handlers, episodes []Episode, style MultiEpisodeStyle) string {
	var formats []episodeFormat
	for _, m := range seasonEpisodePatternRegex.all(pattern) {
		ef := episodeFormat{
			separator:            group(*m, "separator"),
			episodeSeparator:     group(*m, "episodeSeparator"),
			episodePattern:       group(*m, "episode"),
			seasonEpisodePattern: group(*m, "seasonEpisode"),
		}
		if !slices.ContainsFunc(formats, func(f episodeFormat) bool { return f.seasonEpisodePattern == ef.seasonEpisodePattern }) {
			formats = append(formats, ef)
		}
	}
	for i, ef := range formats {
		base := ef.seasonEpisodePattern
		var value string
		switch style {
		case MultiEpisodeDuplicate:
			value = formatNumberTokens(base, ef.separator+base, episodes)
		case MultiEpisodeRepeat:
			value = formatNumberTokens(base, ef.episodeSeparator+ef.episodePattern, episodes)
		case MultiEpisodeScene:
			value = formatNumberTokens(base, "-"+ef.episodeSeparator+ef.episodePattern, episodes)
		case MultiEpisodeRange:
			value = formatNumberTokens(base, "-"+ef.episodePattern, firstAndLast(episodes))
		case MultiEpisodePrefixedRange:
			value = formatNumberTokens(base, "-"+ef.episodeSeparator+ef.episodePattern, firstAndLast(episodes))
		default:
			value = formatNumberTokens(base, "-"+ef.episodePattern, episodes)
		}
		token := fmt.Sprintf("{Season Episode%d}", i+1)
		pattern = strings.ReplaceAll(pattern, base, token)
		h.set(token, func(tokenMatch) string { return value })
	}
	addSeasonTokens(h, episodes[0].SeasonNumber)
	first, last := episodes[0].EpisodeNumber, episodes[len(episodes)-1].EpisodeNumber
	h.set("{Episode}", func(t tokenMatch) string {
		if len(episodes) > 1 {
			return formatNumber(first, t.customFormat) + "-" + formatNumber(last, t.customFormat)
		}
		return formatNumber(first, t.customFormat)
	})
	return pattern
}

// addAbsoluteNumberingTokens replaces each absolute episode pattern of a
// format with a token writing the absolute numbers of the episodes in
// style; outside anime, or when an absolute number is unknown, the
// patterns are removed.
func addAbsoluteNumberingTokens(pattern string, h handlers, s Series, episodes []Episode, style MultiEpisodeStyle) string {
	var formats []absoluteFormat
	for _, m := range absoluteEpisodePatternRegex.all(pattern) {
		af := absoluteFormat{separator: group(*m, "separator"), pattern: group(*m, "absolute")}
		if strings.TrimSpace(af.separator) == "" {
			af.separator = "-"
		}
		if !slices.ContainsFunc(formats, func(f absoluteFormat) bool { return f.pattern == af.pattern }) {
			formats = append(formats, af)
		}
	}
	index := 1
	for _, af := range formats {
		if s.Type != SeriesAnime || !allAbsolute(episodes) {
			pattern = strings.ReplaceAll(pattern, af.pattern, "")
			continue
		}
		var value string
		switch style {
		case MultiEpisodeDuplicate:
			value = formatAbsoluteNumberTokens(af.pattern, af.separator+af.pattern, episodes)
		case MultiEpisodeRepeat:
			separator := strings.TrimSpace(af.separator)
			if separator == "" {
				separator = " "
			}
			value = formatAbsoluteNumberTokens(af.pattern, separator+af.pattern, episodes)
		case MultiEpisodeRange, MultiEpisodePrefixedRange:
			value = formatAbsoluteNumberTokens(af.pattern, "-"+af.pattern, firstAndLast(episodes))
		default:
			value = formatAbsoluteNumberTokens(af.pattern, "-"+af.pattern, episodes)
		}
		token := fmt.Sprintf("{Absolute Pattern%d}", index)
		index++
		pattern = strings.ReplaceAll(pattern, af.pattern, token)
		h.set(token, func(tokenMatch) string { return value })
	}
	return pattern
}

// firstAndLast returns the first and, for several episodes, the last
// episode.
func firstAndLast(episodes []Episode) []Episode {
	if len(episodes) == 1 {
		return episodes[:1]
	}
	return []Episode{episodes[0], episodes[len(episodes)-1]}
}

// formatNumberTokens writes base for the first episode and format for
// each other, with their episode numbers and the season of the first.
func formatNumberTokens(base, format string, episodes []Episode) string {
	var b strings.Builder
	for i, e := range episodes {
		p := format
		if i == 0 {
			p = base
		}
		b.WriteString(episodeRegex.replaceFunc(p, func(m regexp2.Match) string {
			return replaceNumberToken(group(m, "episode"), e.EpisodeNumber)
		}))
	}
	return replaceSeasonTokens(b.String(), episodes[0].SeasonNumber)
}

// formatAbsoluteNumberTokens is formatNumberTokens for absolute numbers.
func formatAbsoluteNumberTokens(base, format string, episodes []Episode) string {
	var b strings.Builder
	for i, e := range episodes {
		p := format
		if i == 0 {
			p = base
		}
		b.WriteString(absoluteEpisodeRegex.replaceFunc(p, func(m regexp2.Match) string {
			return replaceNumberToken(group(m, "absolute"), e.AbsoluteEpisodeNumber)
		}))
	}
	return replaceSeasonTokens(b.String(), episodes[0].SeasonNumber)
}

func replaceSeasonTokens(pattern string, seasonNumber int) string {
	return seasonRegex.replaceFunc(pattern, func(m regexp2.Match) string {
		return replaceNumberToken(group(m, "season"), seasonNumber)
	})
}

// replaceNumberToken writes value as a number token such as "{episode:00}"
// formats it.
func replaceNumberToken(token string, value int) string {
	_, format, _ := strings.Cut(strings.Trim(token, "{}"), ":")
	return formatNumber(value, format)
}

// formatNumber writes n zero-padded to the number of digit placeholders
// of a .NET custom numeric format such as "00"; other formats write n as
// it is.
func formatNumber(n int, format string) string {
	s := strconv.Itoa(n)
	if format == "" || strings.Trim(format, "0#") != "" {
		return s
	}
	width := strings.Count(format, "0")
	if n < 0 {
		return "-" + padLeft(s[1:], width)
	}
	return padLeft(s, width)
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}
