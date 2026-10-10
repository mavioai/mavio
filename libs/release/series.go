package release

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ReleaseType classifies an episode release.
type ReleaseType int

// Release types.
const (
	ReleaseUnknown ReleaseType = iota
	SingleEpisode
	MultiEpisode
	SeasonPack
)

// SeriesTitleInfo describes the series title of a release.
type SeriesTitleInfo struct {
	Title            string
	TitleWithoutYear string
	Year             int
	// AllTitles lists the alternative titles of a release that names
	// several, such as "Title (Alt Title)".
	AllTitles []string
}

// ParsedEpisodeInfo is an episode release parsed with Sonarr's rules.
type ParsedEpisodeInfo struct {
	ReleaseTitle                  string
	SeriesTitle                   string
	SeriesTitleInfo               SeriesTitleInfo
	Quality                       ParsedQuality
	SeasonNumber                  int
	EpisodeNumbers                []int
	AbsoluteEpisodeNumbers        []int
	SpecialAbsoluteEpisodeNumbers []float64
	// AirDate is the air date of a daily episode as "2006-01-02".
	AirDate         string
	Languages       []Language
	FullSeason      bool
	IsPartialSeason bool
	IsMultiSeason   bool
	IsSeasonExtra   bool
	IsSplitEpisode  bool
	IsMiniSeries    bool
	Special         bool
	ReleaseGroup    string
	ReleaseHash     string
	SeasonPart      int
	// ReleaseTokens is the part of the release title after the episode
	// numbers.
	ReleaseTokens string
	DailyPart     *int
}

// IsDaily reports whether the release is identified by air date.
func (p *ParsedEpisodeInfo) IsDaily() bool { return strings.TrimSpace(p.AirDate) != "" }

// IsAbsoluteNumbering reports whether the release has absolute numbers.
func (p *ParsedEpisodeInfo) IsAbsoluteNumbering() bool { return len(p.AbsoluteEpisodeNumbers) > 0 }

// IsPossibleSpecialEpisode reports whether the release may be a special.
func (p *ParsedEpisodeInfo) IsPossibleSpecialEpisode() bool {
	blankTitle := strings.TrimSpace(p.SeriesTitle) == ""
	return (strings.TrimSpace(p.AirDate) == "" && blankTitle && (len(p.EpisodeNumbers) == 0 || p.SeasonNumber == 0)) ||
		(!blankTitle && p.Special) ||
		(len(p.EpisodeNumbers) == 1 && p.EpisodeNumbers[0] == 0)
}

// IsPossibleSceneSeasonSpecial reports whether the release is episode 0 of
// a regular season, which scene groups use for season specials.
func (p *ParsedEpisodeInfo) IsPossibleSceneSeasonSpecial() bool {
	return p.SeasonNumber != 0 && len(p.EpisodeNumbers) == 1 && p.EpisodeNumbers[0] == 0
}

// ReleaseType classifies the release.
func (p *ParsedEpisodeInfo) ReleaseType() ReleaseType {
	switch {
	case len(p.EpisodeNumbers) > 1 || len(p.AbsoluteEpisodeNumbers) > 1:
		return MultiEpisode
	case len(p.EpisodeNumbers) == 1 || len(p.AbsoluteEpisodeNumbers) == 1:
		return SingleEpisode
	case p.FullSeason:
		return SeasonPack
	}
	return ReleaseUnknown
}

// errInvalidDate stops parsing: a daily release with an impossible date is
// not retried with the remaining expressions.
var errInvalidDate = errors.New("invalid air date")

// ParseSeriesPath parses an episode file, using its folder names when the
// file name alone is not enough.
func ParseSeriesPath(path string) *ParsedEpisodeInfo {
	dir, name := splitPath(path)
	result := ParseSeriesTitle(name)

	// Parse using the folder and file separately, but combine if they both
	// parse correctly.
	if m := simpleEpisodeNumberRegex.match(name); m != nil && (result == nil || result.IsMiniSeries || len(result.AbsoluteEpisodeNumbers) > 0) {
		if season := seasonFolderRegex.match(dir); season != nil && season.has("season") {
			episodes := m.captures("episode")
			first, err1 := parseNumber(episodes[0].Value)
			last, err2 := parseNumber(episodes[len(episodes)-1].Value)
			if err1 == nil && err2 == nil {
				pathTitle := fmt.Sprintf("S%sE%02d", season.value("season"), first)
				if first != last {
					pathTitle += fmt.Sprintf("-E%02d", last)
				}
				if remaining, ok := m.group("remaining"); ok {
					pathTitle += " " + remaining
				}
				return ParseSeriesTitle(pathTitle)
			}
		}
	}

	ext := pathExtension(name)
	if number, err := strconv.Atoi(strings.TrimSuffix(name, ext)); result == nil && err == nil {
		result = ParseSeriesTitle(dir)
		switch {
		case result != nil && slices.Contains(result.AbsoluteEpisodeNumbers, number):
			result.AbsoluteEpisodeNumbers = []int{number}
		case result != nil && slices.Contains(result.EpisodeNumbers, number):
			result.EpisodeNumbers = []int{number}
		default:
			result = nil
		}
	}
	if result == nil {
		result = ParseSeriesTitle(dir + " " + name)
	}
	if result == nil {
		result = ParseSeriesTitle(dir + ext)
	}
	return result
}

// splitPath returns the name of the parent folder and the file name of a
// Unix or Windows path.
func splitPath(path string) (dir, name string) {
	i := strings.LastIndexAny(path, `/\`)
	if i < 0 {
		return "", path
	}
	name = path[i+1:]
	path = path[:i]
	return path[strings.LastIndexAny(path, `/\`)+1:], name
}

// SimplifySeriesTitle removes reversed naming, websites and torrent site
// suffixes from a release title.
func SimplifySeriesTitle(title string) string {
	if !validateBeforeParsing(title, Series) {
		return title
	}
	title = unreverse(title, seriesReversedTitleRegex, Series)
	title = websitePrefixRegex.apply(title)
	title = websitePostfixRegex.apply(title)
	return cleanTorrentSuffixRegex.apply(title)
}

// ParseSeriesTitle parses an episode release title, or returns nil.
func ParseSeriesTitle(title string) *ParsedEpisodeInfo {
	if !validateBeforeParsing(title, Series) {
		return nil
	}
	title = unreverse(title, seriesReversedTitleRegex, Series)
	releaseTitle := RemoveFileExtension(title, Series)
	releaseTitle = strings.NewReplacer("【", "[", "】", "]").Replace(releaseTitle)
	for _, r := range seriesPreSubstitutions {
		r.tryReplace(&releaseTitle)
	}

	simpleTitle := seriesSimpleTitleRegex.apply(releaseTitle)
	simpleTitle = websitePrefixRegex.apply(simpleTitle)
	simpleTitle = websitePostfixRegex.apply(simpleTitle)
	simpleTitle = cleanTorrentSuffixRegex.apply(simpleTitle)
	simpleTitle = cleanQualityBrackets(simpleTitle, Series)

	if m := sixDigitAirDateRegex.match(simpleTitle); m != nil {
		year, month, day := m.value("airyear"), m.value("airmonth"), m.value("airday")
		if month != "00" || day != "00" {
			simpleTitle = strings.ReplaceAll(simpleTitle, m.value("airdate"), fmt.Sprintf("20%s.%s.%s", year, month, day))
		}
	}

	for _, r := range seriesTitleRegexes {
		matches := r.matches(simpleTitle)
		if len(matches) == 0 {
			continue
		}
		result, err := parseEpisodeMatches(matches, releaseTitle)
		if err != nil {
			return nil
		}
		if result == nil {
			continue
		}
		if result.FullSeason && containsFold(result.ReleaseTokens, "Special") {
			result.FullSeason = false
			result.Special = true
		}
		result.Languages = ParseLanguages(result.ReleaseTokens, Series)
		result.Quality = ParseQuality(title, Series)
		result.ReleaseGroup = ParseReleaseGroup(releaseTitle, Series)
		if subGroup := matches[0].value("subgroup"); strings.TrimSpace(subGroup) != "" {
			result.ReleaseGroup = subGroup
		}
		result.ReleaseHash = releaseHash(matches[0], "[]()")
		return result
	}
	return nil
}

// ParseSeriesName returns the cleaned series title of a release.
func ParseSeriesName(title string) string {
	if result := ParseSeriesTitle(title); result != nil {
		return result.SeriesTitle
	}
	return CleanSeriesTitle(title)
}

// CleanSeriesTitle reduces a series title to a lowercase key for matching,
// without punctuation, accents and common words.
func CleanSeriesTitle(title string) string {
	if strings.TrimSpace(title) == "" || isInt64(title) {
		return title
	}
	title = percentRegex.replace(title, "percent")
	return RemoveAccent(strings.ToLower(seriesNormalizeRegex.apply(title)))
}

// NormalizeEpisodeTitle reduces an episode title to a lowercase key for
// matching.
func NormalizeEpisodeTitle(title string) string {
	for _, r := range specialEpisodeTitleRegexes {
		if m := r.match(title); m != nil {
			title = m.value("episodetitle")
			break
		}
	}
	title = partRegex.replace(title, "")
	title = punctuationRegex.replace(title, " ")
	title = duplicateSpacesRegex.replace(title, " ")
	return strings.ToLower(strings.TrimSpace(title))
}

func parseEpisodeMatches(matches []*match, releaseTitle string) (*ParsedEpisodeInfo, error) {
	first := matches[0]
	seriesName := strings.NewReplacer(".", " ", "_", " ").Replace(first.value("title"))
	seriesName = strings.Trim(requestInfoRegex.replace(seriesName, ""), " ")
	airYear, _ := strconv.Atoi(first.value("airyear"))
	lastIndex := first.groupEnd("title")

	var result *ParsedEpisodeInfo
	if airYear < 1900 {
		result = &ParsedEpisodeInfo{ReleaseTitle: releaseTitle, EpisodeNumbers: []int{}, AbsoluteEpisodeNumbers: []int{}}
		for _, m := range matches {
			episodes := m.captures("episode")
			absolutes := m.captures("absoluteepisode")
			// An empty list of episodes is a full season release.
			if len(episodes) > 0 {
				firstEpisode, err := parseNumber(episodes[0].Value)
				if err != nil {
					return nil, err
				}
				lastEpisode, err := parseNumber(episodes[len(episodes)-1].Value)
				if err != nil {
					return nil, err
				}
				if firstEpisode > lastEpisode {
					return nil, nil
				}
				result.EpisodeNumbers = intRange(firstEpisode, lastEpisode)
				lastIndex = max(lastIndex, episodes[len(episodes)-1].end())
				if m.has("special") {
					result.Special = true
				}
				if m.has("splitepisode") {
					result.IsSplitEpisode = true
				}
			}
			if len(absolutes) > 0 {
				firstAbsolute, err := parseDecimal(absolutes[0].Value)
				if err != nil {
					return nil, err
				}
				lastAbsolute, err := parseDecimal(absolutes[len(absolutes)-1].Value)
				if err != nil {
					return nil, err
				}
				if firstAbsolute > lastAbsolute {
					return nil, nil
				}
				if firstAbsolute != math.Trunc(firstAbsolute) || lastAbsolute != math.Trunc(lastAbsolute) {
					if len(absolutes) != 1 {
						// Several numbers are not allowed for specials.
						return nil, nil
					}
					result.SpecialAbsoluteEpisodeNumbers = []float64{firstAbsolute}
					result.Special = true
					lastIndex = max(lastIndex, absolutes[0].end())
				} else {
					result.AbsoluteEpisodeNumbers = intRange(int(firstAbsolute), int(lastAbsolute))
					if m.has("special") {
						result.Special = true
					}
					lastIndex = max(lastIndex, absolutes[len(absolutes)-1].end())
				}
			}
			if len(episodes) == 0 && len(absolutes) == 0 {
				// Extras and subtitle packs are flagged so they can be
				// filtered out.
				if strings.TrimSpace(first.value("extras")) != "" {
					result.IsSeasonExtra = true
				}
				// Partial season packs have a season part.
				if part := first.value("seasonpart"); strings.TrimSpace(part) != "" {
					n, err := strconv.Atoi(part)
					if err != nil {
						return nil, err
					}
					result.SeasonPart = n
					result.IsPartialSeason = true
				} else {
					result.FullSeason = true
				}
			}
			if count, ok := first.group("episodecount"); ok && len(episodes) == 2 && episodes[1].Value == count {
				result.EpisodeNumbers = []int{}
				result.FullSeason = true
			}
		}

		var seasons []int
		for _, c := range first.captures("season") {
			if season, err := strconv.Atoi(c.Value); err == nil {
				seasons = append(seasons, season)
				lastIndex = max(lastIndex, c.end())
			}
		}
		// A release of several seasons is rejected later.
		if len(distinct(seasons)) > 1 {
			result.IsMultiSeason = true
		}
		switch {
		case len(seasons) > 0:
			result.SeasonNumber = seasons[0]
		case len(result.AbsoluteEpisodeNumbers) == 0 && len(result.EpisodeNumbers) > 0:
			// Without a season, a release that is not absolutely numbered
			// is a mini-series.
			result.SeasonNumber = 1
			result.IsMiniSeries = true
		}
	} else {
		// A daily show.
		var month, day int
		if first.has("ambiguousairmonth") && first.has("ambiguousairday") {
			m, _ := strconv.Atoi(first.value("ambiguousairmonth"))
			d, _ := strconv.Atoi(first.value("ambiguousairday"))
			if d <= 12 && m <= 12 {
				return nil, fmt.Errorf("%w: ambiguous month and day %d and %d", errInvalidDate, m, d)
			}
			month, day = m, d
		} else {
			var err error
			if month, err = strconv.Atoi(first.value("airmonth")); err != nil {
				return nil, err
			}
			if day, err = strconv.Atoi(first.value("airday")); err != nil {
				return nil, err
			}
			// Swap day and month when the month is out of range.
			if month > 12 {
				month, day = day, month
			}
		}
		airDate := time.Date(airYear, time.Month(month), day, 0, 0, 0, 0, time.Local)
		if airDate.Year() != airYear || airDate.Month() != time.Month(month) || airDate.Day() != day {
			return nil, fmt.Errorf("%w: %d-%d-%d", errInvalidDate, airYear, month, day)
		}
		// A date in the future is most likely a parse error.
		now := time.Now()
		if airDate.After(time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)) {
			return nil, fmt.Errorf("%w: %s", errInvalidDate, airDate.Format(time.DateOnly))
		}
		// Before 1970 only the Plex DVR format, with a year in the title, is
		// trusted.
		if airDate.Before(time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)) && strings.TrimSpace(first.value("titleyear")) == "" {
			return nil, fmt.Errorf("%w: %s", errInvalidDate, airDate.Format(time.DateOnly))
		}
		lastIndex = max(lastIndex, first.groupEnd("airyear"), first.groupEnd("airmonth"), first.groupEnd("airday"))
		result = &ParsedEpisodeInfo{ReleaseTitle: releaseTitle, AirDate: airDate.Format(time.DateOnly), EpisodeNumbers: []int{}, AbsoluteEpisodeNumbers: []int{}}
		if part, ok := first.group("part"); ok {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			result.DailyPart = &n
		}
	}

	if runes := []rune(releaseTitle); lastIndex < len(runes) {
		result.ReleaseTokens = string(runes[lastIndex:])
	} else {
		result.ReleaseTokens = releaseTitle
	}
	result.SeriesTitle = seriesName
	result.SeriesTitleInfo = seriesTitleInfo(seriesName, first)
	return result, nil
}

func seriesTitleInfo(title string, first *match) SeriesTitleInfo {
	info := SeriesTitleInfo{Title: title, TitleWithoutYear: title}
	if m := seriesYearInTitleRegex.match(title); m != nil {
		info.TitleWithoutYear = m.value("title")
		info.Year, _ = strconv.Atoi(m.value("year"))
	}
	if m := titleComponentsRegex.match(info.TitleWithoutYear); m != nil {
		for _, c := range m.captures("title") {
			info.AllTitles = append(info.AllTitles, c.Value)
		}
	} else if titles := first.captures("title"); len(titles) > 1 {
		for _, c := range titles {
			info.AllTitles = append(info.AllTitles, strings.NewReplacer(".", " ", "_", " ").Replace(c.Value))
		}
	}
	return info
}

func intRange(first, last int) []int {
	out := make([]int, 0, last-first+1)
	for n := first; n <= last; n++ {
		out = append(out, n)
	}
	return out
}

var numberWords = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}

// parseNumber parses an episode number written with any digits, or as a
// word from "zero" to "nine".
func parseNumber(value string) (int, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(toNumerals(norm.NFKC.String(value)))); err == nil {
		return n, nil
	}
	if i := slices.Index(numberWords, strings.ToLower(value)); i >= 0 {
		return i, nil
	}
	return 0, fmt.Errorf("%q isn't a number", value)
}

func parseDecimal(value string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(toNumerals(norm.NFKC.String(value))), 64)
	if err != nil {
		return 0, fmt.Errorf("%q isn't a number", value)
	}
	return n, nil
}

// toNumerals replaces decimal digits of any script with ASCII digits.
func toNumerals(s string) string {
	return strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII && unicode.IsDigit(r) {
			// Decimal digits come in runs from zero to nine.
			zero := r
			for unicode.IsDigit(zero - 1) {
				zero--
			}
			return '0' + (r-zero)%10
		}
		return r
	}, s)
}

func isInt64(s string) bool {
	_, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return err == nil
}

// RemoveAccent removes diacritics from s.
func RemoveAccent(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(s))
}

// validateBeforeParsing rejects password-protected, hashed and season
// folder names.
func validateBeforeParsing(title string, kind Kind) bool {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "password") && strings.Contains(lower, "yenc") {
		return false
	}
	if !strings.ContainsFunc(title, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return false
	}
	withoutExtension := RemoveFileExtension(title, kind)
	hashed := seriesRejectHashedRegexes
	if kind == Movie {
		hashed = movieRejectHashedRegexes
	}
	for _, r := range hashed {
		if r.isMatch(withoutExtension) {
			return false
		}
	}
	return kind == Movie || !seasonFolderReleaseRegex.isMatch(withoutExtension)
}

// unreverse undoes a release name written backwards.
//
// Radarr formats the reversed char array into the title instead of its
// characters, so its reversed titles never parse; both kinds reverse the
// title here.
func unreverse(title string, reversed *regex, kind Kind) string {
	if !reversed.isMatch(title) {
		return title
	}
	name := []rune(RemoveFileExtension(title, kind))
	slices.Reverse(name)
	return string(name) + string([]rune(title)[len(name):])
}

// RemoveFileExtension removes a media or Usenet file extension.
func RemoveFileExtension(title string, kind Kind) string {
	return fileExtensionRegex.replaceFunc(title, func(m *match) string {
		ext := strings.ToLower(m.String())
		if IsMediaExtension(ext, kind) || ext == ".par2" || ext == ".nzb" {
			return ""
		}
		return m.String()
	})
}

// cleanQualityBrackets removes a trailing bracketed quality such as
// "[WEBDL-720p]".
func cleanQualityBrackets(title string, kind Kind) string {
	return cleanQualityBracketsRegex.replaceFunc(title, func(m *match) string {
		if ParseQualityName(m.String(), kind).Quality != Unknown {
			return ""
		}
		return m.String()
	})
}

func releaseHash(m *match, trim string) string {
	hash, ok := m.group("hash")
	if !ok {
		return ""
	}
	hash = strings.Trim(hash, trim)
	if hash == "1280x720" {
		return ""
	}
	return hash
}

// HasMultipleLanguages reports whether a release title is tagged "multi".
func HasMultipleLanguages(title string) bool { return multiRegex.isMatch(title) }

// NormalizeImdbID returns an IMDb ID as "tt" and at least seven digits, or
// "" when id is not one.
func NormalizeImdbID(id string) string {
	if !imdbIDRegex.isMatch(id) || len(id) <= 2 {
		return ""
	}
	digits := strings.ReplaceAll(id, "tt", "")
	if len(digits) < 7 {
		digits = strings.Repeat("0", 7-len(digits)) + digits
	}
	return "tt" + digits
}

var imdbIDRegex = re(`^(\d{1,10}|(tt)\d{1,10})$`)
