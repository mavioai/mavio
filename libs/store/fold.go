package store

import (
	"strings"
	"unicode"

	"github.com/mozillazg/go-unidecode"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// cleanValue is Jellyfin's clean form of a name (GetCleanValue), which search
// matches and value lists group by: diacritics removed, lower case, every
// character that is not a letter, digit or space replaced by a space, and
// whitespace collapsed. Full-width forms are also folded to half-width.
// "Ｓｐｉｄｅｒ-Ｍａｎ: Ｌｏｔｕｓ" becomes "spider man lotus"; CJK text is kept.
func cleanValue(s string) string {
	s = strings.ToLower(removeDiacritics(width.Fold.String(s)))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ")
}

// lowerFold lower-cases s and removes diacritics, keeping punctuation; it is
// the form LIKE patterns of raw search terms are matched against.
func lowerFold(s string) string {
	return strings.ToLower(removeDiacritics(width.Fold.String(s)))
}

// diacriticScripts are the scripts whose combining marks are diacritics.
// Marks of other scripts, such as the Japanese voicing marks, change the
// letter and are kept.
var diacriticScripts = []*unicode.RangeTable{unicode.Latin, unicode.Greek, unicode.Cyrillic}

// letterReplacements spell out letters that do not decompose into a base
// letter and a combining mark.
var letterReplacements = strings.NewReplacer(
	"Æ", "AE", "æ", "ae", "Œ", "OE", "œ", "oe", "Ø", "O", "ø", "o",
	"Ł", "L", "ł", "l", "Đ", "D", "đ", "d", "Ð", "D", "ð", "d",
	"Þ", "TH", "þ", "th", "ß", "ss", "ı", "i",
)

// removeDiacritics removes the diacritics of Latin, Greek and Cyrillic
// letters, like Jellyfin's RemoveDiacritics: "Cidadão" becomes "Cidadao" and
// "cœur" becomes "coeur", while Korean and Japanese text is unchanged.
func removeDiacritics(s string) string {
	s = letterReplacements.Replace(s)
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	strip := false
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			if strip {
				continue
			}
		} else {
			strip = unicode.IsOneOf(diacriticScripts, r)
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}

// Sort-name cleaning, as in Jellyfin's default server configuration.
var (
	sortRemoveWords      = []string{"the", "a", "an"}
	sortRemoveCharacters = []string{",", "&", "-", "{", "}", "'"}
	sortReplaceChars     = []string{".", "+", "%"}
)

// sortKey derives the sort key of a name the way Jellyfin derives sort
// names: lower case; the articles "the", "a" and "an" removed at the start,
// middle and end; some punctuation removed or turned into spaces; runs of
// digits zero-padded to ten places so numbers sort naturally; diacritics
// removed; and non-ASCII text transliterated to Latin so that, for example,
// Chinese titles sort by pinyin.
func sortKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for _, w := range sortRemoveWords {
		if rest, ok := strings.CutPrefix(s, w+" "); ok {
			s = rest
		}
		s = strings.ReplaceAll(s, " "+w+" ", " ")
		if rest, ok := strings.CutSuffix(s, " "+w); ok {
			s = rest
		}
	}
	for _, c := range sortRemoveCharacters {
		s = strings.ReplaceAll(s, c, "")
	}
	for _, c := range sortReplaceChars {
		s = strings.ReplaceAll(s, c, " ")
	}
	return sortChunks(s)
}

// sortChunks zero-pads digit runs shorter than ten digits, removes
// diacritics and transliterates non-ASCII text.
func sortChunks(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	rs := []rune(s)
	start := 0
	for i := 1; i <= len(rs); i++ {
		if i < len(rs) && unicode.IsDigit(rs[i]) == unicode.IsDigit(rs[start]) {
			continue
		}
		chunk := string(rs[start:i])
		if unicode.IsDigit(rs[start]) && i-start < 10 {
			b.WriteString(strings.Repeat("0", 10-(i-start)))
		}
		b.WriteString(chunk)
		start = i
	}
	out := removeDiacritics(b.String())
	if !isASCII(out) {
		out = transliterate(out)
	}
	return out
}

// transliterate converts text to lower-case ASCII Latin and drops
// punctuation, like the ICU chain "Any-Latin; Latin-Ascii; Lower; NFD;
// [:Nonspacing Mark:] Remove; [:Punctuation:] Remove".
func transliterate(s string) string {
	latin := strings.ToLower(unidecode.Unidecode(width.Fold.String(s)))
	var b strings.Builder
	for _, r := range latin {
		if !unicode.IsPunct(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// likeEscape escapes LIKE wildcards in a literal; patterns use ESCAPE '\'.
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// likeRaw escapes only the escape character, so % and _ in a raw search
// term act as wildcards, as in Jellyfin.
func likeRaw(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
