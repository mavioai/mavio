package subtitle

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

// The tables and expressions of ftfy (chardata.py and badness.py), built
// once from data/ftfy.json as ftfy builds them at import time.

// charmapEncodings are ftfy's CHARMAP_ENCODINGS, in the order it tries
// them.
var charmapEncodings = []string{
	"latin-1",
	"sloppy-windows-1252",
	"sloppy-windows-1251",
	"sloppy-windows-1250",
	"sloppy-windows-1253",
	"sloppy-windows-1254",
	"sloppy-windows-1257",
	"iso-8859-2",
	"macroman",
	"cp437",
}

// undefinedByte marks a byte a decoding table leaves undefined, as CPython's
// tables do.
const undefinedByte = '￾'

// byteCodec is a single-byte encoding.
type byteCodec struct {
	name   string
	sloppy bool
	decode [256]rune // -1 for an undefined byte
	encode map[rune]byte
}

func newByteCodec(name string, table string, sloppy bool) (*byteCodec, error) {
	rs := []rune(table)
	if len(rs) != 256 {
		return nil, fmt.Errorf("decoding table of %s has %d characters", name, len(rs))
	}
	c := &byteCodec{name: name, sloppy: sloppy, encode: make(map[rune]byte, 256)}
	for i, r := range rs {
		switch {
		case r != undefinedByte:
			c.decode[i] = r
		case sloppy:
			// sloppy.make_sloppy_codec: undefined bytes decode as Latin-1.
			c.decode[i] = rune(i)
		default:
			c.decode[i] = -1
		}
	}
	if sloppy {
		// Byte 1A stands for U+FFFD, the character the sloppy codecs
		// encode it as.
		c.decode[0x1A] = '�'
	}
	for i, r := range c.decode {
		if r >= 0 {
			c.encode[r] = byte(i)
		}
	}
	return c, nil
}

// possible is chardata.possible_encoding: whether text can be encoded.
func (c *byteCodec) possible(text []rune) bool {
	for _, r := range text {
		if _, ok := c.encode[r]; !ok {
			return false
		}
	}
	return true
}

// encodeRunes encodes text, which must be possible in c.
func (c *byteCodec) encodeRunes(text []rune) []byte {
	b := make([]byte, len(text))
	for i, r := range text {
		b[i] = c.encode[r]
	}
	return b
}

// decodeBytes decodes b strictly; ok is false if a byte is undefined.
func (c *byteCodec) decodeBytes(b []byte) (text []rune, ok bool) {
	text = make([]rune, len(b))
	for i, x := range b {
		if text[i] = c.decode[x]; text[i] < 0 {
			return nil, false
		}
	}
	return text, true
}

// textFixer holds what ftfy builds at import time.
type textFixer struct {
	charmaps     []*byteCodec // in charmapEncodings order
	windows1252  *byteCodec   // strict windows-1252
	sloppy1252   *byteCodec
	htmlEntities map[string]string

	badness      *regexp.Regexp // badness.BADNESS_RE
	utf8Detector *regexp.Regexp // chardata.UTF8_DETECTOR_RE without its lookbehind, anchored
	utf8First    runeClass      // the characters a match of utf8Detector starts with
	utf8Strict   runeClass      // the lookbehind's class
	htmlEntity   *regexp.Regexp // chardata.HTML_ENTITY_RE
	ansi         *regexp.Regexp // fixes.ANSI_RE
	alteredUTF8  *regexp.Regexp // chardata.ALTERED_UTF8_RE, on bytes as runes
	lossyUTF8    *regexp.Regexp // chardata.LOSSY_UTF8_RE, on bytes as runes
}

// getTextFixer builds the textFixer once; it is immutable afterwards.
var getTextFixer = sync.OnceValue(func() *textFixer {
	f, err := newTextFixer(loadData[ftfyData]("ftfy.json"))
	if err != nil {
		panic(fmt.Sprintf("subtitle: embedded ftfy.json: %v", err))
	}
	return f
})

// ftfyData is data/ftfy.json.
type ftfyData struct {
	MojibakeCategories map[string]string `json:"mojibake_categories"`
	UTF8Clues          map[string]string `json:"utf8_clues"`
	Codecs             map[string]string `json:"codecs"`
	HTMLEntities       map[string]string `json:"html_entities"`
}

func newTextFixer(d ftfyData) (*textFixer, error) {
	f := &textFixer{htmlEntities: d.HTMLEntities}
	for _, name := range charmapEncodings {
		base, sloppy := strings.CutPrefix(name, "sloppy-")
		table, ok := d.Codecs[base]
		if !ok {
			return nil, fmt.Errorf("no decoding table of %s", base)
		}
		c, err := newByteCodec(name, table, sloppy)
		if err != nil {
			return nil, err
		}
		f.charmaps = append(f.charmaps, c)
		if name == "sloppy-windows-1252" {
			f.sloppy1252 = c
		}
	}
	var err error
	if f.windows1252, err = newByteCodec("windows-1252", d.Codecs["windows-1252"], false); err != nil {
		return nil, err
	}

	// Categories and clues go into character classes as they are, ranges
	// included; escape what would otherwise end or negate a class.
	classEscaper := strings.NewReplacer(`\`, `\\`, `]`, `\]`, `[`, `\[`, `^`, `\^`)
	pairs := []string{
		`\s`, pySpaceClass, // Python's \s and \w on str patterns
		`\w`, `[\p{L}\p{N}_]`,
	}
	for name, chars := range d.MojibakeCategories {
		pairs = append(pairs, "{"+name+"}", classEscaper.Replace(chars))
	}
	expand := strings.NewReplacer(pairs...)
	if f.badness, err = regexp.Compile(expand.Replace(strings.Join(badnessPattern, "|"))); err != nil {
		return nil, fmt.Errorf("badness: %w", err)
	}

	pairs = pairs[:0]
	for name, chars := range d.UTF8Clues {
		pairs = append(pairs, "{"+name+"}", classEscaper.Replace(chars))
	}
	expand = strings.NewReplacer(pairs...)
	if f.utf8Detector, err = regexp.Compile(expand.Replace(utf8DetectorPattern)); err != nil {
		return nil, fmt.Errorf("utf8 detector: %w", err)
	}
	if f.utf8Strict, err = parseRuneClass(d.UTF8Clues["utf8_continuation_strict"]); err != nil {
		return nil, err
	}
	for _, name := range []string{"utf8_first_of_2", "utf8_first_of_3", "utf8_first_of_4"} {
		c, err := parseRuneClass(d.UTF8Clues[name])
		if err != nil {
			return nil, err
		}
		f.utf8First = append(f.utf8First, c...)
	}

	f.htmlEntity = regexp.MustCompile(`&#?[0-9A-Za-z]{1,24};`)
	// Python's \d on str patterns is any decimal digit (Nd).
	f.ansi = regexp.MustCompile(`\x1b\[[\p{Nd};]*[a-zA-Z]`)
	f.alteredUTF8 = regexp.MustCompile(`[\xc2\xc3\xc5\xce\xd0\xd9][ ]` +
		`|[\xe2\xe3][ ][\x80-\x84\x86-\x9f\xa1-\xbf]` +
		`|[\xe0-\xe3][\x80-\x84\x86-\x9f\xa1-\xbf][ ]` +
		`|[\xf0][ ][\x80-\xbf][\x80-\xbf]` +
		`|[\xf0][\x80-\xbf][ ][\x80-\xbf]` +
		`|[\xf0][\x80-\xbf][\x80-\xbf][ ]`)
	f.lossyUTF8 = regexp.MustCompile(`[\xc2-\xdf][\x1a]` +
		`|[\xc2-\xc3][?]` +
		`|\xed[\xa0-\xaf][\x1a?]\xed[\xb0-\xbf][\x1a?\x80-\xbf]` +
		`|\xed[\xa0-\xaf][\x1a?\x80-\xbf]\xed[\xb0-\xbf][\x1a?]` +
		`|[\xe0-\xef][\x1a?][\x1a\x80-\xbf]` +
		`|[\xe0-\xef][\x1a\x80-\xbf][\x1a?]` +
		`|[\xf0-\xf4][\x1a?][\x1a\x80-\xbf][\x1a\x80-\xbf]` +
		`|[\xf0-\xf4][\x1a\x80-\xbf][\x1a?][\x1a\x80-\xbf]` +
		`|[\xf0-\xf4][\x1a\x80-\xbf][\x1a\x80-\xbf][\x1a?]` +
		`|\x1a`)
	return f, nil
}

// pySpaceClass is the class of characters Python's \s matches in a str
// pattern: those for which str.isspace holds.
const pySpaceClass = `[\t\n\v\f\r\x1c-\x20\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// badnessPattern is badness.BADNESS_RE, one alternative per element, with
// its verbose whitespace removed. {name} stands for a mojibake category.
var badnessPattern = []string{
	`[{c1}]`,
	`[{bad}{lower_accented}{upper_accented}{box}{start_punctuation}{end_punctuation}{currency}{numeric}{law}][{bad}]`,
	`[a-zA-Z][{lower_common}{upper_common}][{bad}]`,
	`[{bad}][{lower_accented}{upper_accented}{box}{start_punctuation}{end_punctuation}{currency}{numeric}{law}]`,
	`[{lower_accented}{lower_common}{box}{end_punctuation}{currency}{numeric}][{upper_accented}]`,
	`[{box}{end_punctuation}{currency}{numeric}][{lower_accented}]`,
	`[{lower_accented}{box}{end_punctuation}][{currency}]`,
	`\s[{upper_accented}][{currency}]`,
	`[{upper_accented}{box}][{numeric}{law}]`,
	`[{lower_accented}{upper_accented}{box}{currency}{end_punctuation}][{start_punctuation}][{numeric}]`,
	`[{lower_accented}{upper_accented}{currency}{numeric}{box}{law}][{end_punctuation}][{start_punctuation}]`,
	`[{currency}{numeric}{box}][{start_punctuation}]`,
	`[a-z][{upper_accented}][{start_punctuation}{currency}]`,
	`[{box}][{kaomoji}]`,
	`[{lower_accented}{upper_accented}{currency}{numeric}{start_punctuation}{end_punctuation}{law}][{box}]`,
	`[{box}][{end_punctuation}]`,
	`[{lower_accented}{upper_accented}][{start_punctuation}{end_punctuation}]\w`,
	// The ligature œ when not followed by an unaccented Latin letter
	`[Œœ][^A-Za-z]`,
	// Degree signs after capital letters
	`[{upper_accented}]°`,
	// Common Windows-1252 2-character mojibake that isn't covered by the cases above
	`[ÂÃÎÐ][€œŠš¢£Ÿž\xa0\xad®©°·»{start_punctuation}{end_punctuation}–—´]`,
	`×[²³]`,
	// Windows-1252 mojibake of Arabic words needs to include the 'common' characters.
	`[ØÙ][{common}{currency}{bad}{numeric}{start_punctuation}ŸŠ®°µ»][ØÙ][{common}{currency}{bad}{numeric}{start_punctuation}ŸŠ®°µ»]`,
	// Windows-1252 mojibake that starts 3-character sequences for some South Asian alphabets
	`à[²µ¹¼½¾]`,
	// MacRoman mojibake that isn't covered by the cases above
	`√[±∂†≠®™´≤≥¥µø]`,
	`≈[°¢]`,
	`‚Ä[ìîïòôúùû†°¢π]`,
	`‚[âó][àä°ê]`,
	// Windows-1251 mojibake of characters in the U+2000 range
	`вЂ`,
	// Windows-1251 mojibake of Latin-1 characters and/or the Cyrillic alphabet
	`[ВГРС][{c1}{bad}{start_punctuation}{end_punctuation}{currency}°µ][ВГРС]`,
	// A distinctive five-character sequence of Cyrillic letters
	`ГўВЂВ.[A-Za-z ]`,
	// Windows-1252 encodings of 'à' and 'á', as well as \xa0 itself
	`Ã[\xa0¡]`,
	`[a-z]\s?[ÃÂ][ ]`,
	`^[ÃÂ][ ]`,
	// Cases where Â precedes a character as an encoding of exactly the same character
	`[a-z.,?!{end_punctuation}]Â[ {start_punctuation}{end_punctuation}]`,
	// Windows-1253 mojibake of characters in the U+2000 range
	`β€[™\xa0Ά\xad®°]`,
	// Windows-1253 mojibake of Latin-1 characters and/or the Greek alphabet
	`[ΒΓΞΟ][{c1}{bad}{start_punctuation}{end_punctuation}{currency}°][ΒΓΞΟ]`,
	// Windows-1257 mojibake of characters in the U+2000 range
	`ā€`,
}

// utf8DetectorPattern is chardata.UTF8_DETECTOR_RE without its negative
// lookbehind (?<! [{utf8_continuation_strict}]), which findUTF8Mojibake
// checks before matching the rest at a position.
const utf8DetectorPattern = `^(?:[{utf8_first_of_2}][{utf8_continuation}]` +
	`|[{utf8_first_of_3}][{utf8_continuation}]{2}` +
	`|[{utf8_first_of_4}][{utf8_continuation}]{3})+`

// runeClass is a set of characters given as a regular expression character
// class's contents, with ranges.
type runeClass [][2]rune

func parseRuneClass(s string) (runeClass, error) {
	rs := []rune(s)
	var c runeClass
	for i := 0; i < len(rs); i++ {
		lo, hi := rs[i], rs[i]
		if i+2 < len(rs) && rs[i+1] == '-' {
			hi = rs[i+2]
			i += 2
			if hi < lo {
				return nil, fmt.Errorf("bad range %q-%q", lo, hi)
			}
		}
		c = append(c, [2]rune{lo, hi})
	}
	return c, nil
}

func (c runeClass) contains(r rune) bool {
	for _, x := range c {
		if r >= x[0] && r <= x[1] {
			return true
		}
	}
	return false
}

// runeSliceReader reads a []rune as an io.RuneReader whose runes are one
// unit wide each, so that regexp's offsets on it are indices into the
// slice. Python's str is a sequence of code points, lone surrogates
// included, and so is the []rune ftfy's steps work on here.
type runeSliceReader struct {
	rs []rune
	i  int
}

func (r *runeSliceReader) ReadRune() (rune, int, error) {
	if r.i >= len(r.rs) {
		return 0, 0, io.EOF
	}
	c := r.rs[r.i]
	r.i++
	return c, 1, nil
}

// findRunes finds the first match of re in text[from:], returning indices
// into text. re sees text[from:] alone: ^ matches at from, and nothing
// before from is context.
func findRunes(re *regexp.Regexp, text []rune, from int) (start, end int, ok bool) {
	loc := re.FindReaderIndex(&runeSliceReader{rs: text, i: from})
	if loc == nil {
		return 0, 0, false
	}
	return from + loc[0], from + loc[1], true
}

// replaceRunes is Python's re.sub with a function: it replaces every
// non-overlapping match of re in text, scanning from the left.
func replaceRunes(re *regexp.Regexp, text []rune, repl func([]rune) []rune) []rune {
	var out []rune
	last := 0
	for pos := 0; pos <= len(text); {
		start, end, ok := findRunes(re, text, pos)
		if !ok {
			break
		}
		out = append(out, text[last:start]...)
		out = append(out, repl(text[start:end])...)
		last = end
		pos = end
		if end == start {
			pos++ // ftfy's expressions match no empty string; this only guards the loop
		}
	}
	if out == nil {
		return text
	}
	return append(out, text[last:]...)
}

// bytesAsRunes and runesAsBytes convert between bytes and the runes U+0000
// to U+00FF, so that regexp can match Python's bytes patterns.
func bytesAsRunes(b []byte) []rune {
	rs := make([]rune, len(b))
	for i, x := range b {
		rs[i] = rune(x)
	}
	return rs
}

func runesAsBytes(rs []rune) []byte {
	b := make([]byte, len(rs))
	for i, r := range rs {
		b[i] = byte(r)
	}
	return b
}
