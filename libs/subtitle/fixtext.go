package subtitle

import (
	"slices"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// maxDecodeLength is ftfy's default max_decode_length: the longest segment,
// in characters, that is fixed at once.
const maxDecodeLength = 1_000_000

// FixText repairs text as ftfy's fix_text (ftfy 6.3.1) does with Bazarr's
// options, which it applies to every subtitle it saves: ftfy's defaults
// except that curly quotes stay curly and full-width characters keep their
// width.
//
// It fixes the text line by line (in segments of at most a million
// characters). Each segment goes through these steps, again and again until
// they change nothing: decoding HTML entities (unless a "<" has been seen,
// in this segment or an earlier one, which suggests real HTML), repairing
// mojibake (text decoded with the wrong encoding, such as "Ã©" for "é"),
// replacing C1 control characters with their Windows-1252 characters,
// splitting Latin ligatures such as "ﬁ", turning every kind of line break
// into "\n", joining UTF-16 surrogates, removing terminal escapes and
// invisible control characters, and NFC normalization.
//
// Python strings are sequences of code points that may include lone UTF-16
// surrogates, which a Go string cannot hold. FixText reads text as UTF-8
// in which a surrogate code point may appear in its generalized UTF-8 form
// (WTF-8, the bytes ED A0–BF 80–BF), standing for that surrogate; any
// other byte that is not valid UTF-8 stands for U+FFFD. The result is
// always valid UTF-8.
func FixText(text string) string {
	f := getTextFixer()
	rs := decodeWTF8(text)
	var out []rune
	unescape := true // unescape_html="auto"
	for pos := 0; pos < len(rs); {
		brk := len(rs)
		if i := slices.Index(rs[pos:], '\n'); i >= 0 {
			brk = pos + i + 1
		}
		brk = min(brk, pos+maxDecodeLength)
		segment := rs[pos:brk]
		// Once a segment contains "<", ftfy stops decoding entities for the
		// rest of the text.
		if unescape && slices.Contains(segment, '<') {
			unescape = false
		}
		out = append(out, f.fixSegment(segment, unescape)...)
		pos = brk
	}
	return string(out)
}

// fixSegment is ftfy.fix_and_explain without the explanation.
func (f *textFixer) fixSegment(text []rune, unescape bool) []rune {
	for {
		orig := text
		if unescape {
			text = f.unescapeHTML(text)
		}
		text = f.fixEncoding(text)
		text = f.fixC1Controls(text)
		text = fixLatinLigatures(text)
		text = fixLineBreaks(text)
		text = fixSurrogates(text)
		text = f.removeTerminalEscapes(text)
		text = removeControlChars(text)
		if s := string(text); !norm.NFC.IsNormalString(s) {
			text = []rune(norm.NFC.String(s))
		}
		if slices.Equal(text, orig) {
			return text
		}
	}
}

// decodeWTF8 decodes s into code points as FixText documents: UTF-8 with
// surrogates in their three-byte form, and U+FFFD for each other invalid
// byte.
func decodeWTF8(s string) []rune {
	rs := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2] >= 0x80 && s[i+2] <= 0xBF {
			rs = append(rs, 0xD000|rune(s[i+1]&0x3F)<<6|rune(s[i+2]&0x3F))
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rs = append(rs, r)
		i += size
	}
	return rs
}
