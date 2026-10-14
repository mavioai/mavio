package subtitle

import (
	"bytes"
	"slices"
	"unicode/utf8"
)

// ftfy's fix_encoding: detecting mojibake and decoding it again, from
// ftfy/__init__.py, ftfy/fixes.py and ftfy/bad_codecs.

// isBad is badness.is_bad: whether text looks like it contains mojibake.
func (f *textFixer) isBad(text []rune) bool {
	_, _, ok := findRunes(f.badness, text, 0)
	return ok
}

// fixEncoding is ftfy.fix_encoding: it applies fixEncodingOneStep until the
// text stops changing.
func (f *textFixer) fixEncoding(text []rune) []rune {
	for {
		fixed := f.fixEncodingOneStep(text)
		if slices.Equal(fixed, text) {
			return fixed
		}
		text = fixed
	}
}

// fixEncodingOneStep is ftfy._fix_encoding_one_step_and_explain with the
// default options.
func (f *textFixer) fixEncodingOneStep(text []rune) []rune {
	if len(text) == 0 || isASCII(text) || !f.isBad(text) {
		return text
	}

	// Suppose the text was UTF-8 decoded with a single-byte encoding.
	latin1 := false
	for _, c := range f.charmaps {
		if !c.possible(text) {
			continue
		}
		if c.name == "latin-1" {
			latin1 = true
		}
		b := c.encodeRunes(text)
		if c.name != "macroman" {
			if _, _, ok := findRunes(f.alteredUTF8, bytesAsRunes(b), 0); ok {
				b = f.restoreByteA0(b)
			}
		}
		if c.sloppy {
			b = f.replaceLossySequences(b)
		}
		var fixed []rune
		var ok bool
		if bytes.IndexByte(b, 0xED) >= 0 || bytes.IndexByte(b, 0xC0) >= 0 {
			fixed, ok = decodeUTF8Variants(b)
		} else if ok = utf8.Valid(b); ok {
			fixed = []rune(string(b))
		}
		if ok {
			return fixed
		}
	}

	// Fix sequences that look like UTF-8 mojibake in isolation.
	if _, _, ok := f.findUTF8Mojibake(text, 0); ok {
		if fixed := f.decodeInconsistentUTF8(text); !slices.Equal(fixed, text) {
			return fixed
		}
	}

	// Latin-1 that was meant to be Windows-1252. ftfy also checks for
	// "windows-1252" among the possible encodings here, which never holds
	// because it only tries "sloppy-windows-1252"; so the text is always
	// decoded as Windows-1252, strictly.
	if latin1 {
		if fixed, ok := f.windows1252.decodeBytes(f.charmaps[0].encodeRunes(text)); ok && !slices.Equal(fixed, text) {
			return fixed
		}
	}

	if slices.ContainsFunc(text, isC1Control) {
		return f.fixC1Controls(text)
	}
	return text
}

func isASCII(text []rune) bool {
	return !slices.ContainsFunc(text, func(r rune) bool { return r < 0 || r > 0x7F })
}

// restoreByteA0 is fixes.restore_byte_a0: it turns spaces back into byte A0
// where that makes a UTF-8 sequence.
func (f *textFixer) restoreByteA0(b []byte) []byte {
	b = restoreAGrave(b)
	return runesAsBytes(replaceRunes(f.alteredUTF8, bytesAsRunes(b), func(m []rune) []rune {
		out := slices.Clone(m)
		for i, r := range out {
			if r == ' ' {
				out[i] = 0xA0
			}
		}
		return out
	}))
}

// restoreAGrave is fixes.A_GRAVE_WORD_RE.sub(b"\xc3\xa0 ", b): byte C3
// followed by a space is "à" as a word of its own, unless the next bytes
// are a space or a Portuguese contraction ("às", "àquele", "àquela",
// "àquilo").
func restoreAGrave(b []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i+1 < len(b); i++ {
		if b[i] != 0xC3 || b[i+1] != ' ' {
			continue
		}
		rest := b[i+2:]
		if hasAnyPrefix(rest, " ", "quele", "quela", "quilo", "s ") {
			continue
		}
		out = append(out, b[last:i]...)
		out = append(out, 0xC3, 0xA0, ' ')
		last = i + 2
		i++
	}
	if out == nil {
		return b
	}
	return append(out, b[last:]...)
}

func hasAnyPrefix(b []byte, prefixes ...string) bool {
	for _, p := range prefixes {
		if bytes.HasPrefix(b, []byte(p)) {
			return true
		}
	}
	return false
}

// replaceLossySequences is fixes.replace_lossy_sequences: UTF-8 sequences
// some of whose bytes were lost (byte 1A, or '?') become U+FFFD.
func (f *textFixer) replaceLossySequences(b []byte) []byte {
	replacement := []rune{0xEF, 0xBF, 0xBD} // U+FFFD in UTF-8
	return runesAsBytes(replaceRunes(f.lossyUTF8, bytesAsRunes(b), func([]rune) []rune { return replacement }))
}

// findUTF8Mojibake finds the first match of chardata.UTF8_DETECTOR_RE in
// text at or after from: a run of UTF-8 mojibake that does not follow a
// character in utf8_continuation_strict. Like Python's engine, it checks
// the lookbehind at each position before matching the rest there.
func (f *textFixer) findUTF8Mojibake(text []rune, from int) (start, end int, ok bool) {
	for start = from; start < len(text); start++ {
		if !f.utf8First.contains(text[start]) || start > 0 && f.utf8Strict.contains(text[start-1]) {
			continue
		}
		if _, n, ok := findRunes(f.utf8Detector, text, start); ok {
			return start, n, true
		}
	}
	return 0, 0, false
}

// decodeInconsistentUTF8 is fixes.decode_inconsistent_utf8: it fixes the
// runs of UTF-8 mojibake in text one by one.
func (f *textFixer) decodeInconsistentUTF8(text []rune) []rune {
	var out []rune
	last := 0
	for pos := 0; ; {
		start, end, ok := f.findUTF8Mojibake(text, pos)
		if !ok {
			break
		}
		out = append(out, text[last:start]...)
		sub := text[start:end]
		// The match must be shorter than the text, so that this does not
		// recurse forever.
		if len(sub) < len(text) && f.isBad(sub) {
			out = append(out, f.fixEncoding(slices.Clone(sub))...)
		} else {
			out = append(out, sub...)
		}
		last, pos = end, end
	}
	if out == nil {
		return text
	}
	return append(out, text[last:]...)
}

// isC1Control reports whether r matches chardata.C1_CONTROL_RE.
func isC1Control(r rune) bool { return r >= 0x80 && r <= 0x9F }

// fixC1Controls is fixes.fix_c1_controls: C1 controls become their
// Windows-1252 characters, as Web browsers decode them.
func (f *textFixer) fixC1Controls(text []rune) []rune {
	if !slices.ContainsFunc(text, isC1Control) {
		return text
	}
	out := slices.Clone(text)
	for i, r := range out {
		if isC1Control(r) {
			out[i] = f.sloppy1252.decode[r]
		}
	}
	return out
}

// decodeUTF8Variants decodes b with ftfy's "utf-8-variants" codec
// (bad_codecs/utf8_variants.py): UTF-8 that may also contain CESU-8
// surrogate pairs and Java's two-byte NUL (C0 80). Like Python's strict
// decoder, it fails on anything else that is not UTF-8.
func decodeUTF8Variants(b []byte) ([]rune, bool) {
	var out []rune
	n := len(b)
	// atEnd reports whether Python's $ matches at i in b: at its end or
	// before a final newline.
	atEnd := func(i int) bool { return i == n || i == n-1 && b[i] == '\n' }
	for i := 0; i < n; {
		switch {
		case b[i] == 0xC0 && (i+1 < n && b[i+1] == 0x80 || atEnd(i+1)):
			// NULL_EXPR. A lone C0 at the very end fails to decode; C0
			// before a final newline decodes, with the newline, as NUL.
			if i+1 == n {
				return nil, false
			}
			out = append(out, 0)
			i += 2
		case b[i] == 0xED && (i+1 < n && b[i+1] >= 0xA0 && b[i+1] <= 0xBF || atEnd(i+1)):
			// CESU8_EXPR or SURROGATE_EXPR. Only a complete CESU-8
			// pair decodes; a lone surrogate fails.
			if n-i < 6 || !isCESU8Pair(b[i:i+6], i+6 == n) {
				return nil, false
			}
			cp := rune(b[i+1]&0x0F)<<16 + rune(b[i+2]&0x3F)<<10 + rune(b[i+4]&0x0F)<<6 + rune(b[i+5]&0x3F) + 0x10000
			out = append(out, cp)
			i += 6
		default:
			r, size := utf8.DecodeRune(b[i:])
			if r == utf8.RuneError && size <= 1 {
				return nil, false
			}
			out = append(out, r)
			i += size
		}
	}
	return out, true
}

// isCESU8Pair reports whether p, six bytes, match CESU8_RE: a high and a low
// surrogate encoded in UTF-8. last tells whether p ends the input, where
// Python's $ also lets a final newline stand for the last byte.
func isCESU8Pair(p []byte, last bool) bool {
	cont := func(x byte) bool { return x >= 0x80 && x <= 0xBF }
	return p[0] == 0xED && p[1] >= 0xA0 && p[1] <= 0xAF && cont(p[2]) &&
		p[3] == 0xED && p[4] >= 0xB0 && p[4] <= 0xBF &&
		(cont(p[5]) || last && p[5] == '\n')
}
