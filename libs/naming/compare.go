package naming

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// String ordering as in .NET's culture-aware comparison (ICU root
// collation), which Jellyfin sorts file names with: white space, then
// punctuation and symbols in collation order, then digits, then letters;
// case and accents only break ties, lower case first.

// punctuationOrder is the root collation order of ASCII punctuation and
// symbols.
const punctuationOrder = "_-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$"

// collationKey is the primary weight of a rune: its class, then its base
// letter or position.
type collationKey struct {
	class int
	value rune
}

func primaryKey(r rune) collationKey {
	switch {
	case unicode.IsSpace(r):
		return collationKey{0, r}
	case r < 0x80 && strings.ContainsRune(punctuationOrder, r):
		return collationKey{1, rune(strings.IndexRune(punctuationOrder, r))}
	case unicode.IsPunct(r) || unicode.IsSymbol(r):
		return collationKey{2, r}
	case unicode.IsDigit(r):
		return collationKey{3, r - '0'}
	}
	return collationKey{4, unicode.ToLower(baseRune(r))}
}

// baseRune strips accents from r.
func baseRune(r rune) rune {
	d := []rune(norm.NFD.String(string(r)))
	if len(d) == 0 {
		return r
	}
	return d[0]
}

func compareKeys(a, b collationKey) int {
	switch {
	case a.class != b.class:
		return a.class - b.class
	case a.value < b.value:
		return -1
	case a.value > b.value:
		return 1
	}
	return 0
}

// compareCulture compares strings like .NET's culture-aware comparison.
func compareCulture(a, b string) int {
	return compareCollated(a, b, false)
}

// compareNumeric compares strings like .NET's culture-aware comparison
// with numeric ordering, so "Part 2" sorts before "Part 10".
func compareNumeric(a, b string) int {
	return compareCollated(a, b, true)
}

func compareCollated(a, b string, numeric bool) int {
	ra, rb := []rune(a), []rune(b)
	// Primary: base characters, numbers by value when numeric.
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if numeric && isASCIIDigit(ra[i]) && isASCIIDigit(rb[j]) {
			ei, ej := digitRunEnd(ra, i), digitRunEnd(rb, j)
			if c := compareDigits(ra[i:ei], rb[j:ej]); c != 0 {
				return c
			}
			i, j = ei, ej
			continue
		}
		if c := compareKeys(primaryKey(ra[i]), primaryKey(rb[j])); c != 0 {
			return c
		}
		i++
		j++
	}
	switch {
	case i < len(ra):
		return 1
	case j < len(rb):
		return -1
	}
	// Secondary: accents.
	for k := 0; k < min(len(ra), len(rb)); k++ {
		ba, bb := unicode.ToLower(ra[k]), unicode.ToLower(rb[k])
		if ba != bb && baseRune(ba) == baseRune(bb) {
			if baseRune(ba) == ba {
				return -1
			}
			if baseRune(bb) == bb {
				return 1
			}
		}
	}
	// Tertiary: lower case first.
	for k := 0; k < min(len(ra), len(rb)); k++ {
		if ra[k] != rb[k] && unicode.ToLower(ra[k]) == unicode.ToLower(rb[k]) {
			if unicode.IsLower(ra[k]) {
				return -1
			}
			return 1
		}
	}
	return strings.Compare(a, b)
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

func digitRunEnd(rs []rune, i int) int {
	for i < len(rs) && isASCIIDigit(rs[i]) {
		i++
	}
	return i
}

// compareDigits compares two digit runs by value.
func compareDigits(a, b []rune) int {
	trim := func(d []rune) []rune {
		for len(d) > 1 && d[0] == '0' {
			d = d[1:]
		}
		return d
	}
	a, b = trim(a), trim(b)
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(string(a), string(b))
}
