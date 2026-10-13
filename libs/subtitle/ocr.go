package subtitle

import (
	"strings"
)

// ocrDictionary is Subzero's OCR fix data of one language. Subzero matches
// most groups with one regular expression alternating every key in order;
// literalSet does the same without compiling thousands of alternatives.
type ocrDictionary struct {
	wholeLines         ocrGroup
	wholeWords         *literalSet
	beginLines         *literalSet
	endLines           *literalSet
	partialLines       *literalSet
	partialWordsAlways ocrGroup
}

func newOCRDictionary(groups map[string]ocrGroup) *ocrDictionary {
	return &ocrDictionary{
		wholeLines:         groups["WholeLines"],
		wholeWords:         newLiteralSet(groups["WholeWords"]),
		beginLines:         newLiteralSet(groups["BeginLines"]),
		endLines:           newLiteralSet(groups["EndLines"]),
		partialLines:       newLiteralSet(groups["PartialLines"]),
		partialWordsAlways: groups["PartialWordsAlways"],
	}
}

// literalSet replaces keys, tried in order at each position from the
// left, as the regular expression (?:key1|key2|...) between two zero-width
// conditions does in Python's re.sub.
type literalSet struct {
	keys    [][]rune
	values  []string
	byFirst map[rune][]int
}

func newLiteralSet(pairs ocrGroup) *literalSet {
	l := &literalSet{byFirst: map[rune][]int{}}
	for _, p := range pairs {
		k := []rune(p[0])
		if len(k) == 0 {
			continue
		}
		l.byFirst[k[0]] = append(l.byFirst[k[0]], len(l.keys))
		l.keys = append(l.keys, k)
		l.values = append(l.values, p[1])
	}
	return l
}

// boundary reports whether \b holds between rs[i-1] and rs[i].
func boundary(rs []rune, i int) bool {
	before := i > 0 && isPyWord(rs[i-1])
	after := i < len(rs) && isPyWord(rs[i])
	return before != after
}

// lineStart and lineEnd are ^ and $ of a multi-line Python pattern.
func lineStart(rs []rune, i int) bool { return i == 0 || rs[i-1] == '\n' }
func lineEnd(rs []rune, i int) bool   { return i == len(rs) || rs[i] == '\n' }

// replace replaces the keys found where before holds at their start and
// after at their end.
func (l *literalSet) replace(s string, before, after func(rs []rune, i int) bool) string {
	if len(l.keys) == 0 {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	changed := false
	for i := 0; i < len(rs); {
		matched := false
		if before(rs, i) {
			for _, k := range l.byFirst[rs[i]] {
				key := l.keys[k]
				if len(rs)-i >= len(key) && runesEqual(rs[i:i+len(key)], key) && after(rs, i+len(key)) {
					b.WriteString(l.values[k])
					i += len(key)
					matched, changed = true, true
					break
				}
			}
		}
		if !matched {
			b.WriteRune(rs[i])
			i++
		}
	}
	if !changed {
		return s
	}
	return b.String()
}

func runesEqual(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The conditions around each group's keys, from Subzero's make_data.py.
func wholeWordBefore(rs []rune, i int) bool { return boundary(rs, i) || lineStart(rs, i) }
func wholeWordAfter(rs []rune, i int) bool  { return boundary(rs, i) || lineEnd(rs, i) }
func anywhere([]rune, int) bool             { return true }

func partialLineBefore(rs []rune, i int) bool {
	return i > 0 && isPySpace(rs[i-1]) || lineStart(rs, i) || boundary(rs, i)
}

func partialLineAfter(rs []rune, i int) bool {
	return i < len(rs) && isPySpace(rs[i]) || lineEnd(rs, i) || boundary(rs, i)
}

// ocrProcessors are FixOCR's processors after OCR_fix_F, in order.
func (d *ocrDictionary) processors() []processor {
	return []processor{
		{name: "OCR_replace_line", apply: func(content string, _ *lineContext) (string, error) {
			if len(d.wholeLines) == 0 {
				return content, nil
			}
			content = pyStrip(content)
			for _, p := range d.wholeLines {
				if content == p[0] {
					return p[1], nil
				}
			}
			return content, nil
		}},
		{name: "OCR_replace_word", apply: func(content string, _ *lineContext) (string, error) {
			return d.wholeWords.replace(content, wholeWordBefore, wholeWordAfter), nil
		}},
		{name: "OCR_replace_beginline", apply: func(content string, _ *lineContext) (string, error) {
			return d.beginLines.replace(content, lineStart, anywhere), nil
		}},
		{name: "OCR_replace_endline", apply: func(content string, _ *lineContext) (string, error) {
			return d.endLines.replace(content, anywhere, lineEnd), nil
		}},
		{name: "OCR_replace_partialline", apply: func(content string, _ *lineContext) (string, error) {
			return d.partialLines.replace(content, partialLineBefore, partialLineAfter), nil
		}},
		{name: "OCR_replace_partialwordsalways", apply: func(content string, _ *lineContext) (string, error) {
			for _, p := range d.partialWordsAlways {
				content = strings.ReplaceAll(content, p[0], p[1])
			}
			return content, nil
		}},
	}
}
