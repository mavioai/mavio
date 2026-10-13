package subtitle

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Helpers that behave like the Python string operations Bazarr's code
// relies on.

// isPySpace reports whether Python's str.isspace holds for r.
func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f
}

// pyStrip is Python's str.strip().
func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

// isPyWord reports whether r matches \w in a Python str pattern.
func isPyWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}

// isPyCased reports whether r is a cased character, for the final sigma
// rule of str.lower.
func isPyCased(r rune) bool { return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) }

// pyLower is Python's str.lower(), with its final sigma and dotted capital
// I special cases.
func pyLower(s string) string {
	if !strings.ContainsAny(s, "Σİ") {
		return strings.ToLower(s)
	}
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		switch r {
		case 'Σ':
			before := i > 0 && isPyCased(rs[i-1])
			after := i+1 < len(rs) && isPyCased(rs[i+1])
			if before && !after {
				b.WriteRune('ς')
			} else {
				b.WriteRune('σ')
			}
		case 'İ':
			b.WriteString("i̇")
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// pyCapitalize is Python's str.capitalize(): the first character in title
// case, the rest lower case.
func pyCapitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToTitle(r)) + pyLower(s[size:])
}

// pyRegex compiles a .NET-syntax regular expression ported from a Python
// one, with a timeout so that a pathological line cannot stall a job.
func pyRegex(pattern string) *regexp2.Regexp {
	re := regexp2.MustCompile(pattern, regexp2.None)
	re.MatchTimeout = 5 * time.Second
	return re
}

// group returns a group of a match, "" when it did not participate, as
// Python's re.sub templates do.
func group(m regexp2.Match, i int) string {
	g := m.GroupByNumber(i)
	if g == nil || len(g.Captures) == 0 {
		return ""
	}
	return g.String()
}

// pySub is Python's re.sub with a function. A timed-out match leaves s
// unchanged.
func pySub(re *regexp2.Regexp, s string, repl func(m regexp2.Match) string) string {
	out, err := re.ReplaceFunc(s, repl, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// pySubString is Python's re.sub with a replacement without references.
func pySubString(re *regexp2.Regexp, s, repl string) string {
	return pySub(re, s, func(regexp2.Match) string { return repl })
}

// pySearch is Python's re.search, as a boolean.
func pySearch(re *regexp2.Regexp, s string) bool {
	ok, err := re.MatchString(s)
	return err == nil && ok
}
