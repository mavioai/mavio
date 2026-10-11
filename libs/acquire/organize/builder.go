package organize

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/libs/release"
)

// DefaultReleaseGroup is written by a bare {Release Group} token when the
// file has no release group.
const DefaultReleaseGroup = "Mavio"

// Movie is the movie a file or folder is named after.
type Movie struct {
	// Title is the movie title.
	Title string
	// OriginalTitle is the title in the original language.
	OriginalTitle string
	// Year is the release year; zero when unknown.
	Year int
	// CollectionTitle is the title of the collection the movie belongs
	// to, if any.
	CollectionTitle string
	// Certification is the age rating, such as "R".
	Certification string
	// ImdbID is the IMDb ID, such as "tt0111161".
	ImdbID string
	// TmdbID is the TMDB ID.
	TmdbID int
	// Translations are the translated titles {Movie Title:FR} chooses
	// from.
	Translations []Translation
}

// Translation is the title of a movie in a language.
type Translation struct {
	// Language is the language of the title.
	Language release.Language
	// Title is the translated title.
	Title string
}

// File is the movie file being named.
type File struct {
	// Quality is the title of the quality definition, such as
	// "Bluray-1080p"; empty when the quality is unknown.
	Quality string
	// Revision is the revision of the quality.
	Revision Revision
	// Edition is the edition, such as "Director's Cut".
	Edition string
	// ReleaseGroup is the release group.
	ReleaseGroup string
	// SceneName is the release name the file was grabbed as.
	SceneName string
	// RelativePath is the path of the file relative to the movie folder.
	RelativePath string
	// Path is the absolute path of the file.
	Path string
	// CustomFormats are the custom formats the file matches.
	CustomFormats []CustomFormat
	// MediaInfo is the media information of the file; nil when unknown,
	// which leaves every MediaInfo token empty.
	MediaInfo *MediaInfo
}

// Revision is the revision of a file's quality.
type Revision struct {
	// Version is 1 for the first release and 2 or more for a proper or
	// repack, which {Quality Proper} writes as "Proper".
	Version int
	// Real is positive for a REAL release, which {Quality Real} writes as
	// "REAL".
	Real int
}

// CustomFormat is a custom format a file matches.
type CustomFormat struct {
	// Name is the name of the custom format.
	Name string
	// IncludeWhenRenaming makes the {Custom Formats} tokens write it.
	IncludeWhenRenaming bool
}

// SceneOrFileName returns the scene name of the file, or its file name
// without the extension.
func (f File) SceneOrFileName() string {
	return sceneOrFileName(f.SceneName, f.RelativePath, f.Path)
}

// sceneOrFileName returns sceneName, or the file name without extension
// of relativePath or else of path.
func sceneOrFileName(sceneName, relativePath, path string) string {
	switch {
	case strings.TrimSpace(sceneName) != "":
		return sceneName
	case strings.TrimSpace(relativePath) != "":
		return fileNameWithoutExtension(relativePath)
	}
	return fileNameWithoutExtension(path)
}

// titleRegex matches a token: {prefix Token:format suffix}, optionally in
// an {imdb-…} or {edition-…} tag.
var titleRegex = re(`(?<tag>\{(?<prefix>[-{ ._\[(]*)(?:imdb(?:id)?-|edition-))?\{(?<prefix>[-{ ._\[(]*)(?<token>(?:[a-z0-9]+)(?:(?<separator>[- ._]+)(?:[a-z0-9]+))?)(?::(?<customFormat>[ ,a-z0-9|+-]+(?<![- ])))?(?<suffix>[-} ._)\]]*)\}`, regexp2.IgnoreCase)

var (
	// ordinalRegex matches an ordinal such as "1st" in an edition.
	ordinalRegex = re(`((?:\b|_)\d{1,3}(?:st|th|rd|nd)(?:\b|_))`, regexp2.IgnoreCase)
	// upperEditionRegex matches the edition words written in upper case.
	upperEditionRegex = re(`((?:\b|_)(?:IMAX|3D|SDR|HDR|DV)(?:\b|_))`, regexp2.IgnoreCase)
)

// ellipsisPlaceholder marks a truncation until the separators around it
// have been cleaned up.
const ellipsisPlaceholder = "{ellipsis}"

// BuildFileName names a movie file, without its extension, with
// n.StandardMovieFormat; components of a format with "/" or "\" are
// joined with "/". When n.RenameMovies is false the file keeps its scene
// or original file name.
func BuildFileName(n Naming, m Movie, f File) (string, error) {
	if !n.RenameMovies {
		return originalTitle(f.SceneName, f.RelativePath, f.Path, true), nil
	}
	if strings.TrimSpace(n.StandardMovieFormat) == "" {
		return "", fmt.Errorf("standard movie format: %w", ErrEmptyFormat)
	}
	pattern := n.StandardMovieFormat
	h := handlers{}
	addMovieTokens(h, m)
	addReleaseYearTokens(h, m.Year)
	addIDTokens(h, m)
	addQualityTokens(h, f.Quality, f.Revision, false)
	addMediaInfoTokens(h, f.MediaInfo)
	addMovieFileTokens(h, f, titleRegex.count(pattern) > 1)
	addEditionTagsTokens(h, f)
	addCustomFormatTokens(h, f.CustomFormats)

	var components []string
	for _, p := range splitPattern(pattern) {
		c := strings.TrimSpace(replaceTokens(p, h, n.cleaning()))
		c = collapseSeparators(c)
		c = strings.TrimRight(c, "- ._")
		c = strings.ReplaceAll(c, ellipsisPlaceholder, "...")
		c = replaceReservedDeviceNames(c)
		if strings.TrimSpace(c) != "" {
			components = append(components, c)
		}
	}
	return strings.Join(components, "/"), nil
}

// MovieFolder names the folder of a movie with n.MovieFolderFormat;
// components of a format with "/" or "\" are joined with "/".
func MovieFolder(n Naming, m Movie) string {
	h := handlers{}
	addMovieTokens(h, m)
	addReleaseYearTokens(h, m.Year)
	addIDTokens(h, m)

	var components []string
	for _, p := range splitPattern(n.MovieFolderFormat) {
		c := CleanFolderName(replaceTokens(p, h, n.cleaning()))
		c = strings.ReplaceAll(c, ellipsisPlaceholder, "...")
		c = replaceReservedDeviceNames(c)
		if strings.TrimSpace(c) != "" {
			components = append(components, c)
		}
	}
	return strings.Join(components, "/")
}

// splitPattern splits a format into its path components.
func splitPattern(pattern string) []string {
	return strings.FieldsFunc(pattern, func(r rune) bool { return r == '/' || r == '\\' })
}

// tokenMatch is a token found in a format.
type tokenMatch struct {
	tag, prefix, separator, suffix, token, customFormat string
}

// defaultValue returns v for a token without prefix and suffix, and ""
// otherwise.
func (t tokenMatch) defaultValue(v string) string {
	if t.prefix == "" && t.suffix == "" {
		return v
	}
	return ""
}

// handlers maps simplified token names to the functions writing them.
type handlers map[string]func(t tokenMatch) string

func (h handlers) set(token string, f func(t tokenMatch) string) { h[simplifyToken(token)] = f }

// simplifyToken reduces a token name to its lower-case letters and digits,
// so that "{Movie Title}", "{Movie.Title}" and "{MovieTitle}" are one.
func simplifyToken(token string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) {
			return unicode.ToLower(r)
		}
		return -1
	}, token)
}

func replaceTokens(pattern string, h handlers, c charCleaning) string {
	return titleRegex.replaceFunc(pattern, func(m regexp2.Match) string { return replaceToken(m, h, c) })
}

func replaceToken(m regexp2.Match, h handlers, c charCleaning) string {
	t := newTokenMatch(m)
	text := ""
	if f, ok := h[simplifyToken(t.token)]; ok {
		text = f(t)
	}
	return formatTokenValue(t, text, c)
}

// newTokenMatch reads a token from a match of a title expression.
func newTokenMatch(m regexp2.Match) tokenMatch {
	t := tokenMatch{
		tag:          group(m, "tag"),
		prefix:       group(m, "prefix"),
		separator:    group(m, "separator"),
		suffix:       group(m, "suffix"),
		token:        group(m, "token"),
		customFormat: group(m, "customFormat"),
	}
	if strings.TrimSpace(t.customFormat) == "" {
		t.customFormat = ""
	}
	return t
}

// formatTokenValue writes the value of token t: trimmed, cased as the
// token name is, with the token's separator for spaces, cleaned of
// illegal characters, and with the token's affixes when not empty.
func formatTokenValue(t tokenMatch, text string, c charCleaning) string {
	text = strings.TrimSpace(text)
	switch {
	case !strings.ContainsFunc(t.token, unicode.IsUpper):
		text = strings.ToLower(text)
	case !strings.ContainsFunc(t.token, unicode.IsLower):
		text = strings.ToUpper(text)
	}
	if strings.TrimSpace(t.separator) != "" {
		text = strings.ReplaceAll(text, " ", t.separator)
	}
	text = cleanFileName(text, c)
	if strings.TrimSpace(text) != "" {
		text = t.tag + t.prefix + text + t.suffix
	}
	return text
}

func addMovieTokens(h handlers, m Movie) {
	h.set("{Movie Title}", func(t tokenMatch) string { return truncate(languageTitle(m, t.customFormat), t.customFormat) })
	h.set("{Movie CleanTitle}", func(t tokenMatch) string {
		return truncate(CleanTitle(languageTitle(m, t.customFormat)), t.customFormat)
	})
	h.set("{Movie TitleThe}", func(t tokenMatch) string { return truncate(TitleThe(m.Title), t.customFormat) })
	h.set("{Movie CleanTitleThe}", func(t tokenMatch) string { return truncate(CleanTitleThe(m.Title), t.customFormat) })
	h.set("{Movie TitleFirstCharacter}", func(t tokenMatch) string {
		return TitleFirstCharacter(TitleThe(languageTitle(m, t.customFormat)))
	})
	h.set("{Movie OriginalTitle}", func(t tokenMatch) string { return truncate(m.OriginalTitle, t.customFormat) })
	h.set("{Movie CleanOriginalTitle}", func(t tokenMatch) string {
		return truncate(CleanTitle(m.OriginalTitle), t.customFormat)
	})
	h.set("{Movie Certification}", func(tokenMatch) string { return m.Certification })
	h.set("{Movie Collection}", func(t tokenMatch) string { return truncate(m.CollectionTitle, t.customFormat) })
	h.set("{Movie CollectionThe}", func(t tokenMatch) string { return truncate(TitleThe(m.CollectionTitle), t.customFormat) })
	h.set("{Movie CleanCollectionThe}", func(t tokenMatch) string {
		return truncate(CleanTitleThe(m.CollectionTitle), t.customFormat)
	})
}

// languageTitle returns the title in the first language of the "|"
// separated ISO codes that is known, or the movie title.
func languageTitle(m Movie, isoCodes string) string {
	if strings.TrimSpace(isoCodes) == "" {
		return m.Title
	}
	for _, code := range strings.Split(isoCodes, "|") {
		iso, ok := release.FindIsoLanguage(strings.ToLower(code), release.Movie)
		if !ok {
			continue
		}
		for _, tr := range m.Translations {
			if tr.Language == iso.Language && tr.Title != "" {
				return tr.Title
			}
		}
		return m.Title
	}
	return m.Title
}

func addReleaseYearTokens(h handlers, year int) {
	h.set("{Release Year}", func(tokenMatch) string {
		if year == 0 {
			return ""
		}
		return strconv.Itoa(year)
	})
}

func addIDTokens(h handlers, m Movie) {
	h.set("{ImdbId}", func(tokenMatch) string { return m.ImdbID })
	h.set("{TmdbId}", func(tokenMatch) string { return strconv.Itoa(m.TmdbID) })
}

// addQualityTokens adds the quality tokens of a quality title and
// revision; anime writes a proper as "v2", "v3" and so on.
func addQualityTokens(h handlers, title string, revision Revision, anime bool) {
	proper, realLabel := "", ""
	if title != "" {
		if revision.Version > 1 {
			proper = "Proper"
			if anime {
				proper = "v" + strconv.Itoa(revision.Version)
			}
		}
		if revision.Real > 0 {
			realLabel = "REAL"
		}
	}
	h.set("{Quality Full}", func(tokenMatch) string {
		if title == "" {
			return ""
		}
		return title + " " + proper + " " + realLabel
	})
	h.set("{Quality Title}", func(tokenMatch) string { return title })
	h.set("{Quality Proper}", func(tokenMatch) string { return proper })
	h.set("{Quality Real}", func(tokenMatch) string { return realLabel })
}

func addMediaInfoTokens(h handlers, info *MediaInfo) {
	if info == nil {
		return
	}
	mi := *info
	bitDepth := "8"
	if mi.VideoBitDepth > 0 {
		bitDepth = strconv.Itoa(mi.VideoBitDepth)
	}
	channels := ""
	if mi.AudioChannels > 0 {
		channels = strconv.FormatFloat(mi.AudioChannels, 'f', 1, 64)
	}
	threeD := ""
	if mi.Is3D {
		threeD = "3D"
	}
	h.set("{MediaInfo Video}", func(tokenMatch) string { return mi.VideoCodec })
	h.set("{MediaInfo VideoCodec}", func(tokenMatch) string { return mi.VideoCodec })
	h.set("{MediaInfo VideoBitDepth}", func(tokenMatch) string { return bitDepth })
	h.set("{MediaInfo Audio}", func(tokenMatch) string { return mi.AudioCodec })
	h.set("{MediaInfo AudioCodec}", func(tokenMatch) string { return mi.AudioCodec })
	h.set("{MediaInfo AudioChannels}", func(tokenMatch) string { return channels })
	h.set("{MediaInfo AudioLanguages}", func(t tokenMatch) string {
		return languagesToken(mi.AudioLanguages, t.customFormat, true, true)
	})
	h.set("{MediaInfo AudioLanguagesAll}", func(t tokenMatch) string {
		return languagesToken(mi.AudioLanguages, t.customFormat, false, true)
	})
	h.set("{MediaInfo SubtitleLanguages}", func(t tokenMatch) string {
		return languagesToken(mi.SubtitleLanguages, t.customFormat, false, true)
	})
	h.set("{MediaInfo SubtitleLanguagesAll}", func(t tokenMatch) string {
		return languagesToken(mi.SubtitleLanguages, t.customFormat, false, true)
	})
	h.set("{MediaInfo 3D}", func(tokenMatch) string { return threeD })
	h.set("{MediaInfo Simple}", func(tokenMatch) string { return mi.VideoCodec + " " + mi.AudioCodec })
	h.set("{MediaInfo Full}", func(t tokenMatch) string {
		return mi.VideoCodec + " " + mi.AudioCodec + languagesToken(mi.AudioLanguages, t.customFormat, true, true) +
			" " + languagesToken(mi.SubtitleLanguages, t.customFormat, false, true)
	})
	h.set("{MediaInfo VideoDynamicRange}", func(tokenMatch) string { return mi.VideoDynamicRange() })
	h.set("{MediaInfo VideoDynamicRangeType}", func(tokenMatch) string { return mi.VideoDynamicRangeType })
}

func addMovieFileTokens(h handlers, f File, multipleTokens bool) {
	h.set("{Original Title}", func(tokenMatch) string {
		return originalTitle(f.SceneName, f.RelativePath, f.Path, !multipleTokens)
	})
	h.set("{Original Filename}", func(tokenMatch) string {
		return originalFileName(f.RelativePath, f.Path, !multipleTokens)
	})
	addReleaseGroupToken(h, f.ReleaseGroup)
}

// addReleaseGroupToken adds {Release Group}, which a bare token writes as
// DefaultReleaseGroup when the release group is unknown.
func addReleaseGroupToken(h handlers, releaseGroup string) {
	h.set("{Release Group}", func(t tokenMatch) string {
		if strings.TrimSpace(releaseGroup) == "" {
			return t.defaultValue(DefaultReleaseGroup)
		}
		return truncate(releaseGroup, t.customFormat)
	})
}

func addEditionTagsTokens(h handlers, f File) {
	if strings.TrimSpace(f.Edition) == "" {
		return
	}
	h.set("{Edition Tags}", func(t tokenMatch) string { return truncate(editionToken(f.Edition), t.customFormat) })
}

// editionToken title-cases an edition, keeping ordinals lower case and
// IMAX, 3D, SDR, HDR and DV upper case.
func editionToken(edition string) string {
	e := titleCase(strings.ToLower(edition))
	e = ordinalRegex.replaceFunc(e, func(m regexp2.Match) string { return strings.ToLower(m.GroupByNumber(1).String()) })
	return upperEditionRegex.replaceFunc(e, func(m regexp2.Match) string { return strings.ToUpper(m.GroupByNumber(1).String()) })
}

// titleCase upper-cases the first letter of every word as .NET's
// TextInfo.ToTitleCase does for lower-case text: a word starts at a
// letter and runs through letters, digits, marks and apostrophes.
func titleCase(s string) string {
	var b strings.Builder
	inWord := false
	for _, r := range s {
		if inWord && !unicode.IsLetter(r) && r != '\'' && isWordSeparator(r) {
			inWord = false
		}
		if !inWord && unicode.IsLetter(r) {
			r = unicode.ToTitle(r)
			inWord = true
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isWordSeparator reports whether r ends a word: white space, control and
// format characters, punctuation and symbols.
func isWordSeparator(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) ||
		unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.In(r, unicode.Zl, unicode.Zp)
}

func addCustomFormatTokens(h handlers, formats []CustomFormat) {
	h.set("{Custom Formats}", func(t tokenMatch) string { return customFormatsToken(formats, t.customFormat) })
	h.set("{Custom Format}", func(t tokenMatch) string {
		if t.customFormat == "" {
			return ""
		}
		for _, cf := range formats {
			if cf.IncludeWhenRenaming && cf.Name == t.customFormat {
				return cf.Name
			}
		}
		return ""
	})
}

// customFormatsToken joins the names of the custom formats included when
// renaming; a filter "A,B" keeps the listed ones and "-A,B" removes them.
func customFormatsToken(formats []CustomFormat, filter string) string {
	var exclude bool
	var names []string
	if filter != "" {
		exclude = strings.HasPrefix(filter, "-")
		names = strings.Split(strings.TrimPrefix(filter, "-"), ",")
	}
	var out []string
	for _, cf := range formats {
		if !cf.IncludeWhenRenaming {
			continue
		}
		if filter != "" && slices.Contains(names, cf.Name) == exclude {
			continue
		}
		out = append(out, cf.Name)
	}
	return strings.Join(out, " ")
}

// originalTitle returns the cleaned scene name of a file, or its original
// file name.
func originalTitle(sceneName, relativePath, path string, useFileName bool) string {
	if strings.TrimSpace(sceneName) == "" {
		return CleanFileName(originalFileName(relativePath, path, useFileName))
	}
	return CleanFileName(sceneName)
}

// originalFileName returns the file name of a file without extension, or
// "" unless useFileName is set: renaming must not nest the current name
// in the new one.
func originalFileName(relativePath, path string, useFileName bool) string {
	if !useFileName {
		return ""
	}
	if strings.TrimSpace(relativePath) == "" {
		return fileNameWithoutExtension(path)
	}
	return fileNameWithoutExtension(relativePath)
}

// fileNameWithoutExtension returns the last element of a "/" or "\"
// separated path without its extension.
func fileNameWithoutExtension(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	if i := strings.LastIndex(p, "."); i >= 0 {
		p = p[:i]
	}
	return p
}

// truncate shortens input to the length in formatter, ending it with an
// ellipsis, or for a negative length keeps its end behind an ellipsis.
func truncate(input, formatter string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	maxLength := maxLengthFromFormatter(formatter)
	runes := []rune(input)
	limit := max(maxLength, -maxLength)
	if maxLength == 0 || len(runes) <= limit {
		return input
	}
	keep := max(limit-3, 0)
	if maxLength < 0 {
		return ellipsisPlaceholder + strings.TrimLeft(string(runes[len(runes)-keep:]), " .")
	}
	return strings.TrimRight(string(runes[:keep]), " .") + ellipsisPlaceholder
}

// maxLengthFromFormatter returns the length a token's format sets, or 0.
func maxLengthFromFormatter(formatter string) int {
	n, err := strconv.Atoi(strings.TrimSpace(formatter))
	if err != nil {
		return 0
	}
	return n
}
