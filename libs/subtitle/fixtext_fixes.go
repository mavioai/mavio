package subtitle

import (
	"slices"
	"strings"
)

// ftfy's individual fixes, from ftfy/fixes.py.

// unescapeHTML is fixes.unescape_html: it decodes the HTML entities ftfy
// knows (chardata.HTML_ENTITIES: HTML5's names that end in a semicolon,
// and the uppercased names of some), and numeric character references that
// end in a semicolon, leaving everything else alone.
func (f *textFixer) unescapeHTML(text []rune) []rune {
	if !slices.Contains(text, '&') {
		return text
	}
	return replaceRunes(f.htmlEntity, text, func(m []rune) []rune {
		s := string(m)
		if v, ok := f.htmlEntities[s]; ok {
			return []rune(v)
		}
		if strings.HasPrefix(s, "&#") {
			if v, ok := unescapeCharRef(s); ok && !strings.Contains(v, ";") {
				return []rune(v)
			}
		}
		return m
	})
}

// unescapeCharRef is what Python's html.unescape makes of s, a match of
// HTML_ENTITY_RE that starts with "&#", when the character reference takes
// all of it; ok is false when it does not, in which case ftfy keeps s.
func unescapeCharRef(s string) (v string, ok bool) {
	digits := strings.TrimSuffix(s[2:], ";")
	base := 10
	if digits != "" && (digits[0] == 'x' || digits[0] == 'X') {
		digits, base = digits[1:], 16
	}
	if digits == "" {
		return "", false
	}
	num := 0
	for _, c := range digits {
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case base == 16 && c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return "", false
		}
		if num <= 0x10FFFF {
			num = num*base + d // beyond U+10FFFF the value no longer matters
		}
	}
	return pyCharRef(num), true
}

// pyCharRef is the numeric branch of Python's html._replace_charref.
func pyCharRef(num int) string {
	if num == 0 {
		return "�"
	}
	if num == 0x0D {
		return "\r"
	}
	if num >= 0x80 && num <= 0x9F {
		// _invalid_charrefs: Windows-1252, with its five holes kept.
		return string(windows1252C1[num-0x80])
	}
	switch {
	case num >= 0xD800 && num <= 0xDFFF || num > 0x10FFFF:
		return "�"
	case num >= 0x01 && num <= 0x08, num == 0x0B, num >= 0x0E && num <= 0x1F, num == 0x7F,
		num >= 0xFDD0 && num <= 0xFDEF, num&0xFFFE == 0xFFFE:
		// _invalid_codepoints
		return ""
	}
	return string(rune(num))
}

// windows1252C1 is what HTML makes of the character references &#x80; to
// &#x9F; (Python's html._invalid_charrefs).
var windows1252C1 = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

// removeTerminalEscapes is fixes.remove_terminal_escapes: it strips ANSI
// escape sequences.
func (f *textFixer) removeTerminalEscapes(text []rune) []rune {
	if !slices.Contains(text, '\x1b') {
		return text
	}
	return replaceRunes(f.ansi, text, func([]rune) []rune { return nil })
}

// latinLigatures is chardata.LIGATURES.
var latinLigatures = map[rune]string{
	'Ĳ': "IJ", // Dutch ligatures
	'ĳ': "ij",
	'ŉ': "ʼn", // Afrikaans digraph meant to avoid auto-curled quote
	'Ǳ': "DZ", // Serbian/Croatian digraphs for Cyrillic conversion
	'ǲ': "Dz",
	'ǳ': "dz",
	'Ǆ': "DŽ",
	'ǅ': "Dž",
	'ǆ': "dž",
	'Ǉ': "LJ",
	'ǈ': "Lj",
	'ǉ': "lj",
	'Ǌ': "NJ",
	'ǋ': "Nj",
	'ǌ': "nj",
	'ﬀ': "ff", // Latin typographical ligatures
	'ﬁ': "fi",
	'ﬂ': "fl",
	'ﬃ': "ffi",
	'ﬄ': "ffl",
	'ﬅ': "ſt",
	'ﬆ': "st",
}

// translate is Python's str.translate with a table from characters to
// strings; a character without an entry stays.
func translate(text []rune, table func(rune) (string, bool)) []rune {
	var out []rune
	for i, r := range text {
		v, ok := table(r)
		if !ok {
			if out != nil {
				out = append(out, r)
			}
			continue
		}
		if out == nil {
			out = append(make([]rune, 0, len(text)), text[:i]...)
		}
		out = append(out, []rune(v)...)
	}
	if out == nil {
		return text
	}
	return out
}

// fixLatinLigatures is fixes.fix_latin_ligatures.
func fixLatinLigatures(text []rune) []rune {
	return translate(text, func(r rune) (string, bool) {
		v, ok := latinLigatures[r]
		return v, ok
	})
}

// fixLineBreaks is fixes.fix_line_breaks: CRLF, CR, U+2028, U+2029 and
// U+0085 become LF.
func fixLineBreaks(text []rune) []rune {
	if !slices.ContainsFunc(text, func(r rune) bool { return r == '\r' || r == 0x2028 || r == 0x2029 || r == 0x85 }) {
		return text
	}
	out := make([]rune, 0, len(text))
	for i := 0; i < len(text); i++ {
		switch r := text[i]; r {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			out = append(out, '\n')
		case 0x2028, 0x2029, 0x85:
			out = append(out, '\n')
		default:
			out = append(out, r)
		}
	}
	return out
}

func isSurrogate(r rune) bool { return r >= 0xD800 && r <= 0xDFFF }

// fixSurrogates is fixes.fix_surrogates: UTF-16 surrogate pairs become the
// character they encode, and lone surrogates U+FFFD.
func fixSurrogates(text []rune) []rune {
	if !slices.ContainsFunc(text, isSurrogate) {
		return text
	}
	out := make([]rune, 0, len(text))
	for i := 0; i < len(text); i++ {
		r := text[i]
		switch {
		case r >= 0xD800 && r <= 0xDBFF && i+1 < len(text) && text[i+1] >= 0xDC00 && text[i+1] <= 0xDFFF:
			out = append(out, 0x10000+(r-0xD800)*0x400+(text[i+1]-0xDC00))
			i++
		case isSurrogate(r):
			out = append(out, '�')
		default:
			out = append(out, r)
		}
	}
	return out
}

// isRemovedControl reports whether r is in chardata.CONTROL_CHARS.
func isRemovedControl(r rune) bool {
	return r <= 0x08 || r == 0x0B || r >= 0x0E && r <= 0x1F || r == 0x7F ||
		r >= 0x206A && r <= 0x206F || r == 0xFEFF || r >= 0xFFF9 && r <= 0xFFFC
}

// removeControlChars is fixes.remove_control_chars.
func removeControlChars(text []rune) []rune {
	if !slices.ContainsFunc(text, isRemovedControl) {
		return text
	}
	return slices.DeleteFunc(slices.Clone(text), isRemovedControl)
}
