package organize

import (
	"strings"
	"unicode"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/libs/release"
)

var (
	// scenifyRemoveChars matches the characters CleanTitle removes: a
	// punctuation character standing alone between spaces, apostrophes,
	// colons, question marks and commas ending a word, and brackets.
	scenifyRemoveChars = re(`(?<=\s)(,|<|>|\/|\\|;|:|'|"|\||`+"`"+`|’|~|!|\?|@|$|%|^|\*|-|_|=){1}(?=\s)|('|`+"`"+`|’|:|\?|,)(?=(?:(?:s|m|t|ve|ll|d|re)\s)|\s|$)|(\(|\)|\[|\]|\{|\})`, regexp2.IgnoreCase)
	// titlePrefixRegex splits a leading article from a title and its
	// trailing parenthesized qualifiers.
	titlePrefixRegex = re(`^(The|An|A) (.*?)((?: *\([^)]+\))*)$`, regexp2.IgnoreCase)
	// reservedDeviceNamesRegex matches a Windows device name ending in a
	// period at the start of a component.
	reservedDeviceNamesRegex = re(`^(?:aux|com[1-9]|con|lpt[1-9]|nul|prn)\.`, regexp2.IgnoreCase)
)

// badCharacters are illegal in file names; goodCharacters replace them
// index by index.
var (
	badCharacters  = []string{`\`, "/", "<", ">", "?", "*", "|", `"`}
	goodCharacters = []string{"+", "+", "", "", "!", "-", "", ""}
)

// CleanTitle makes a title scene-like: "&" becomes "and", slashes become
// spaces, brackets, lone punctuation and apostrophes are removed, and
// diacritics are stripped.
func CleanTitle(title string) string {
	return removeDiacritics(scenify(title))
}

// scenify is CleanTitle without stripping diacritics, as Sonarr cleans
// titles: "&" becomes "and", slashes become spaces, and brackets, lone
// punctuation and apostrophes are removed.
func scenify(title string) string {
	title = strings.ReplaceAll(title, "&", "and")
	title = strings.ReplaceAll(title, "/", " ")
	return scenifyRemoveChars.replace(title, "")
}

// TitleThe moves a leading "The", "An" or "A" behind the title, before
// any trailing parenthesized qualifiers: "The Mist (2007)" becomes
// "Mist, The (2007)".
func TitleThe(title string) string {
	if strings.TrimSpace(title) == "" {
		return ""
	}
	return titlePrefixRegex.replace(title, "$2, $1$3")
}

// CleanTitleThe is TitleThe of a CleanTitle.
func CleanTitleThe(title string) string {
	return cleanTitleTheWith(title, CleanTitle)
}

// cleanTitleTheWith is TitleThe of a title cleaned with clean.
func cleanTitleTheWith(title string, clean func(string) string) string {
	if strings.TrimSpace(title) == "" {
		return ""
	}
	if m := titlePrefixRegex.match(title); m != nil {
		g := m.Groups()
		return strings.TrimSpace(clean(g[2].String())) + ", " + g[1].String() + clean(g[3].String())
	}
	return clean(title)
}

// TitleFirstCharacter returns the upper-case first letter or digit of a
// title, without diacritics, looking at its first two characters; it is
// "_" when neither is a letter or digit.
func TitleFirstCharacter(title string) string {
	runes := []rune(title)
	for i := 0; i < 2 && i < len(runes); i++ {
		if r := runes[i]; unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string([]rune(removeDiacritics(string(unicode.ToUpper(r))))[0])
		}
	}
	return "_"
}

// CleanFileName replaces or removes the characters of name that are
// illegal in file names with the default naming configuration.
func CleanFileName(name string) string {
	return cleanFileName(name, DefaultNaming().cleaning())
}

// CleanFolderName collapses repeated separators of name and trims spaces
// and periods from its ends.
func CleanFolderName(name string) string {
	return strings.Trim(collapseSeparators(name), " .")
}

// charCleaning is how illegal characters are replaced in file names.
type charCleaning struct {
	// replaceIllegal replaces illegal characters instead of removing them.
	replaceIllegal bool
	// colon is how colons are replaced when replaceIllegal is set.
	colon ColonReplacement
	// customColon replaces colons for ColonCustom.
	customColon string
}

// cleanFileName replaces or removes the characters of name that are
// illegal in file names as c says, and trims leading spaces and periods
// and trailing spaces.
func cleanFileName(name string, c charCleaning) string {
	result := name
	if c.replaceIllegal {
		if c.colon == ColonSmart {
			result = strings.ReplaceAll(result, ": ", " - ")
			result = strings.ReplaceAll(result, ":", "-")
		} else {
			replacement := ""
			switch c.colon {
			case ColonDash:
				replacement = "-"
			case ColonSpaceDash:
				replacement = " -"
			case ColonSpaceDashSpace:
				replacement = " - "
			case ColonCustom:
				replacement = c.customColon
			}
			result = strings.ReplaceAll(result, ":", replacement)
		}
	} else {
		result = strings.ReplaceAll(result, ":", "")
	}
	for i, bad := range badCharacters {
		good := ""
		if c.replaceIllegal {
			good = goodCharacters[i]
		}
		result = strings.ReplaceAll(result, bad, good)
	}
	return strings.TrimRight(strings.TrimLeft(result, " ."), " ")
}

// isSeparator reports whether r is one of the separators a token can use.
func isSeparator(r rune) bool { return r == '-' || r == ' ' || r == '.' || r == '_' }

// collapseSeparators replaces runs of the same separator with one.
func collapseSeparators(s string) string {
	var b strings.Builder
	var prev rune = -1
	for _, r := range s {
		if r == prev && isSeparator(r) {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// replaceReservedDeviceNames replaces the period after a leading Windows
// device name with an underscore.
func replaceReservedDeviceNames(s string) string {
	return reservedDeviceNamesRegex.replaceFunc(s, func(m regexp2.Match) string {
		return strings.ReplaceAll(m.String(), ".", "_")
	})
}

// transliterations are the letters without a canonical decomposition that
// lose their diacritics by transliteration.
var transliterations = map[rune]string{
	'Æ': "AE", 'æ': "ae", 'Œ': "OE", 'œ': "oe", 'Ø': "O", 'ø': "o", 'ß': "ss",
	'Đ': "D", 'đ': "d", 'Ð': "D", 'ð': "d", 'Ł': "L", 'ł': "l", 'Þ': "TH", 'þ': "th",
	'Ħ': "H", 'ħ': "h", 'ı': "i", 'Ŀ': "L", 'ŀ': "l", 'Ŧ': "T", 'ŧ': "t",
}

// removeDiacritics strips the diacritics of Latin, Greek and Cyrillic
// letters and transliterates ligatures and stroked letters; other scripts
// are kept as they are.
func removeDiacritics(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case transliterations[r] != "":
			b.WriteString(transliterations[r])
		case unicode.Is(unicode.Mn, r):
		case r >= 0x80 && (r < 0x0530 || (r >= 0x1E00 && r <= 0x1FFF)):
			b.WriteString(release.RemoveAccent(string(r)))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
