package store

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// fold normalizes text for case- and accent-insensitive matching and
// ordering: full-width characters become half-width, diacritics are removed,
// letters are lower-cased and whitespace runs collapse to one space.
// "Ｃａｆé  Ｌｕｃｅ" folds to "cafe luce"; CJK text is kept as is.
func fold(s string) string {
	t := transform.Chain(width.Fold, norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToLower(out)), " ")
}

// searchKey is what substring search matches for an item.
func searchKey(name, originalTitle string) string {
	key := fold(name)
	if ot := fold(originalTitle); ot != "" && ot != key {
		key += " " + ot
	}
	return key
}
