package release

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParsedMovieInfo is a movie release parsed with Radarr's rules.
type ParsedMovieInfo struct {
	// MovieTitles lists the title and its alternatives ("AKA").
	MovieTitles        []string
	OriginalTitle      string
	ReleaseTitle       string
	SimpleReleaseTitle string
	Quality            ParsedQuality
	Languages          []Language
	ReleaseGroup       string
	ReleaseHash        string
	Edition            string
	Year               int
	ImdbID             string
	TmdbID             int
	HardcodedSubs      string
}

// PrimaryMovieTitle returns the first title, or "".
func (p *ParsedMovieInfo) PrimaryMovieTitle() string {
	if len(p.MovieTitles) > 0 {
		return p.MovieTitles[0]
	}
	return ""
}

// ParseMoviePath parses a movie file, using its folder name when the file
// name alone is not enough.
func ParseMoviePath(path string) *ParsedMovieInfo {
	dir, name := splitPath(path)
	if result := ParseMovieTitle(name, true); result != nil {
		return result
	}
	if result := ParseMovieTitle(dir+" "+name, false); result != nil {
		return result
	}
	return ParseMovieTitle(dir+pathExtension(name), false)
}

// ParseMovieTitle parses a movie release title, or returns nil. A folder
// name may also start with the year.
func ParseMovieTitle(title string, isDir bool) *ParsedMovieInfo {
	originalTitle := title
	if !validateBeforeParsing(title, Movie) {
		return nil
	}
	title = unreverse(title, movieReversedTitleRegex, Movie)
	releaseTitle := strings.Trim(RemoveFileExtension(title, Movie), "-_")
	releaseTitle = strings.NewReplacer("【", "[", "】", "]").Replace(releaseTitle)

	simpleTitle := movieSimpleTitleRegex.apply(releaseTitle)
	simpleTitle = websitePrefixRegex.apply(simpleTitle)
	simpleTitle = websitePostfixRegex.apply(simpleTitle)
	simpleTitle = cleanTorrentSuffixRegex.apply(simpleTitle)
	simpleTitle = cleanQualityBrackets(simpleTitle, Movie)

	regexes := movieTitleRegexes
	if isDir {
		regexes = append(regexes[:len(regexes):len(regexes)], movieFolderRegexes...)
	}
	for _, r := range regexes {
		matches := r.matches(simpleTitle)
		if len(matches) == 0 {
			continue
		}
		result := parseMovieMatches(matches)
		if result == nil {
			continue
		}
		first := matches[0]
		simpleReleaseTitle := simpleReleaseTitleRegex.replace(releaseTitle, "")
		titleCapture, hasTitle := firstCapture(first, "title")
		replaceString := result.PrimaryMovieTitle()
		if hasTitle {
			replaceString = titleCapture.Value
		}
		if strings.TrimSpace(replaceString) != "" {
			placeholder := "A Movie"
			if strings.Contains(replaceString, ".") {
				placeholder = "A.Movie"
			}
			if hasTitle {
				runes := []rune(simpleReleaseTitle)
				if titleCapture.end() > len(runes) {
					return nil
				}
				simpleReleaseTitle = string(runes[:titleCapture.Index]) + placeholder + string(runes[titleCapture.end():])
			} else {
				simpleReleaseTitle = strings.ReplaceAll(simpleReleaseTitle, replaceString, placeholder)
			}
		}

		result.ReleaseGroup = ParseReleaseGroup(simpleReleaseTitle, Movie)
		if subGroup := first.value("subgroup"); strings.TrimSpace(subGroup) != "" {
			result.ReleaseGroup = subGroup
		}
		result.HardcodedSubs = ParseHardcodedSubs(title)
		languageTitle := simpleReleaseTitle
		if strings.TrimSpace(result.ReleaseGroup) != "" {
			languageTitle = strings.ReplaceAll(simpleReleaseTitle, result.ReleaseGroup, "RlsGrp")
		}
		result.Languages = ParseLanguages(languageTitle, Movie)
		result.Quality = ParseQuality(title, Movie)
		if strings.TrimSpace(result.Edition) == "" {
			result.Edition = ParseEdition(simpleReleaseTitle)
		}
		result.ReleaseHash = releaseHash(first, "[]")
		result.OriginalTitle = originalTitle
		result.ReleaseTitle = releaseTitle
		result.SimpleReleaseTitle = simpleReleaseTitle
		result.ImdbID = ParseImdbID(simpleReleaseTitle)
		result.TmdbID = ParseTmdbID(simpleReleaseTitle)
		return result
	}
	return nil
}

// firstCapture returns the last capture of a group, which .NET reports as
// the group's value and position.
func firstCapture(m *match, name string) (capture, bool) {
	c := m.captures(name)
	if len(c) == 0 {
		return capture{}, false
	}
	return c[len(c)-1], true
}

func parseMovieMatches(matches []*match) *ParsedMovieInfo {
	first := matches[0]
	title, ok := first.group("title")
	if !ok || title == "(" {
		return nil
	}
	movieName := strings.ReplaceAll(title, "_", " ")
	movieName = normalizeAlternativeTitleRegex.replace(movieName, " AKA ")
	movieName = strings.Trim(requestInfoRegex.replace(movieName, ""), " ")

	// Keep the dots of acronyms such as "U.N.C.L.E.".
	parts := strings.Split(movieName, ".")
	var b strings.Builder
	previousAcronym := false
	for n, part := range parts {
		nextPart := ""
		if len(parts) >= n+2 {
			nextPart = parts[n+1]
		}
		lower := strings.ToLower(part)
		switch {
		case utf8.RuneCountInString(part) == 1 && lower != "a" && !isInt(part) &&
			(previousAcronym || n < len(parts)-1) &&
			(previousAcronym || utf8.RuneCountInString(nextPart) != 1 || !isInt(nextPart)):
			b.WriteString(part + ".")
			previousAcronym = true
		case lower == "a" && (previousAcronym || utf8.RuneCountInString(nextPart) == 1):
			b.WriteString(part + ".")
			previousAcronym = true
		case lower == "dr":
			b.WriteString(part + ".")
			previousAcronym = true
		default:
			if previousAcronym {
				b.WriteString(" ")
				previousAcronym = false
			}
			b.WriteString(part + " ")
		}
	}
	movieName = strings.Trim(b.String(), " ")

	result := &ParsedMovieInfo{MovieTitles: []string{movieName}}
	result.Year, _ = strconv.Atoi(first.value("year"))
	if edition, ok := first.group("edition"); ok {
		result.Edition = strings.ReplaceAll(edition, ".", " ")
	}
	// Remove parentheses of the form "(aka ...)" and split by AKA.
	unbracketed := bracketedAlternativeTitleRegex.replace(movieName, "$1 AKA $2")
	for _, name := range alternativeTitleRegex.split(unbracketed) {
		if strings.TrimSpace(name) != "" && name != movieName {
			result.MovieTitles = append(result.MovieTitles, name)
		}
	}
	return result
}

func isInt(s string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil
}

// ParseImdbID returns the IMDb ID in a release title, or "".
func ParseImdbID(title string) string {
	if m := reportImdbIDRegex.match(title); m != nil {
		if id := m.value("imdbid"); len(id) == 9 || len(id) == 10 {
			return id
		}
	}
	return ""
}

// ParseTmdbID returns the TMDB ID in a release title such as
// "{tmdbid-123}", or 0.
func ParseTmdbID(title string) int {
	if m := reportTmdbIDRegex.match(title); m != nil {
		id, _ := strconv.Atoi(m.value("tmdbid"))
		return id
	}
	return 0
}

// ParseEdition returns the edition of a movie release, such as
// "Directors Cut", or "".
func ParseEdition(title string) string {
	if m := reportEditionRegex.match(title); m != nil {
		if edition := m.value("edition"); strings.TrimSpace(edition) != "" {
			return strings.ReplaceAll(edition, ".", " ")
		}
	}
	return ""
}

// ParseHardcodedSubs returns the hardcoded subtitles tag of a release, such
// as "KORSUB", "Generic Hardcoded Subs" for an untyped tag, or "".
func ParseHardcodedSubs(title string) string {
	matches := hardcodedSubsRegex.matches(title)
	if len(matches) == 0 {
		return ""
	}
	m := matches[len(matches)-1]
	if sub, ok := m.group("hcsub"); ok {
		return sub
	}
	if m.has("hc") {
		return "Generic Hardcoded Subs"
	}
	return ""
}

// ReplaceGermanUmlauts spells out German umlauts and ß.
func ReplaceGermanUmlauts(s string) string {
	return strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss").Replace(s)
}

// CleanMovieTitle reduces a movie title to a lowercase key for matching,
// without punctuation, accents and common words.
func CleanMovieTitle(title string) string {
	if strings.TrimSpace(title) == "" || isInt64(title) {
		return title
	}
	return RemoveAccent(ReplaceGermanUmlauts(strings.ToLower(movieNormalizeRegex.replace(title, ""))))
}

// NormalizeMovieTitle reduces a movie title to lowercase words.
func NormalizeMovieTitle(title string) string {
	title = specialEpisodeWordRegex.replace(title, "")
	title = punctuationRegex.replace(title, " ")
	title = duplicateSpacesRegex.replace(title, " ")
	return strings.ToLower(strings.TrimSpace(title))
}

// NormalizeTitle removes punctuation and a leading article from a title.
func NormalizeTitle(title string, kind Kind) string {
	title = punctuationRegex.replace(title, "")
	title = articleWordRegex.replace(title, "")
	title = duplicateSpacesRegex.replace(title, " ")
	if kind == Movie {
		title = specialCharRegex.replace(title, "")
	}
	return strings.ToLower(strings.TrimSpace(title))
}

// SimplifyReleaseTitle removes characters that are invalid in file names.
func SimplifyReleaseTitle(title string) string { return simpleReleaseTitleRegex.replace(title, "") }
