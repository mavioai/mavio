package metadata

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
)

// Lyrics are the words of a track, synced when their lines have times.
type Lyrics struct {
	Metadata LyricMetadata
	Lines    []LyricLine
}

// LyricMetadata are the ID tags of an LRC file.
type LyricMetadata struct {
	Artist, Album, Title, Author, By, Creator, Version string
	Length                                             time.Duration
	// Offset was added to every time.
	Offset time.Duration
	// Synced says the lines have times.
	Synced bool
}

// LyricLine is a line of lyrics, with when it starts in synced lyrics and
// the times of its words in enhanced (ELRC) ones.
type LyricLine struct {
	Text  string
	Start *time.Duration
	Cues  []LyricCue
}

// LyricCue times part of a line: the UTF-16 code units from Position to
// EndPosition, as JavaScript indexes strings. End is nil for the last
// word of the last line.
type LyricCue struct {
	Position, EndPosition int
	Start                 time.Duration
	End                   *time.Duration
}

// LyricExtensions are the extensions of lyric files, preferred first.
var LyricExtensions = []string{".elrc", ".lrc", ".txt"}

// ParseLyrics reads a lyric file: LRC and enhanced LRC as Jellyfin's
// LrcLyricParser reads them, else, and for .txt files, one line per line
// of text as its TxtLyricParser does. It reports false for other files
// and for LRC files without lines.
func ParseLyrics(name, content string) (*Lyrics, bool) {
	ext := strings.ToLower(path.Ext(name))
	if !slices.Contains(LyricExtensions, ext) {
		return nil, false
	}
	if ext != ".txt" {
		if l, ok := parseLRC(content); ok {
			return l, true
		}
	}
	return parseText(content), true
}

func parseText(content string) *Lyrics {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	var out Lyrics
	for line := range strings.SplitSeq(content, "\n") {
		out.Lines = append(out.Lines, LyricLine{Text: strings.TrimSpace(line)})
	}
	return &out
}

var (
	// lineTime is a line's time tag, [mm:ss.xx]; idTag an ID tag, [ar:…].
	lineTime = regexp.MustCompile(`^\[(\d+):(\d{1,2}(?:[.:]\d{1,3})?)\]`)
	idTag    = regexp.MustCompile(`^\[([A-Za-z]+):(.*)\]\s*$`)
	// wordTime is a word's time tag, <mm:ss.xx>.
	wordTime = regexp.MustCompile(`<(\d+):(\d{1,2}(?:[.:]\d{1,3})?)>`)
)

// parseTime reads minutes and seconds with an optional fraction.
func parseTime(min, sec string) (time.Duration, bool) {
	m, err := strconv.Atoi(min)
	if err != nil {
		return 0, false
	}
	sec = strings.Replace(sec, ":", ".", 1)
	s, err := strconv.ParseFloat(sec, 64)
	if err != nil || s >= 60 {
		return 0, false
	}
	return time.Duration(m)*time.Minute + time.Duration(s*1000+0.5)*time.Millisecond, true
}

// timedLine is a parsed LRC line before its cues are made.
type timedLine struct {
	start time.Duration
	text  []rune
	tags  []wordTag
	order int
}

// wordTag is a word's time at a position of its line's text.
type wordTag struct {
	pos  int
	time time.Duration
}

func parseLRC(content string) (*Lyrics, bool) {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	var out Lyrics
	var lines []timedLine
	for raw := range strings.SplitSeq(content, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var starts []time.Duration
		for {
			m := lineTime.FindStringSubmatch(raw)
			if m == nil {
				break
			}
			if t, ok := parseTime(m[1], m[2]); ok {
				starts = append(starts, t)
			}
			raw = raw[len(m[0]):]
		}
		if len(starts) == 0 {
			if m := idTag.FindStringSubmatch(raw); m != nil {
				out.Metadata.set(strings.ToLower(m[1]), strings.TrimSpace(m[2]))
			}
			continue
		}
		text, tags := wordTags(raw)
		for _, s := range starts {
			lines = append(lines, timedLine{start: s, text: text, tags: tags, order: len(lines)})
		}
	}
	if len(lines) == 0 {
		return nil, false
	}
	slices.SortStableFunc(lines, func(a, b timedLine) int { return int(a.start - b.start) })
	offset := out.Metadata.Offset
	at := func(d time.Duration) time.Duration { return max(0, d+offset) }
	out.Metadata.Synced = true
	for i, l := range lines {
		start := at(l.start)
		line := LyricLine{Text: string(l.text), Start: &start}
		units := utf16Offsets(l.text)
		for k, tag := range l.tags {
			end := len(l.text)
			if k+1 < len(l.tags) {
				end = l.tags[k+1].pos
			}
			if strings.TrimSpace(string(l.text[tag.pos:end])) == "" {
				continue
			}
			cue := LyricCue{Position: units[tag.pos], EndPosition: units[end], Start: at(tag.time)}
			switch {
			case k+1 < len(l.tags):
				e := at(l.tags[k+1].time)
				cue.End = &e
			case i+1 < len(lines):
				e := at(lines[i+1].start)
				cue.End = &e
			}
			line.Cues = append(line.Cues, cue)
		}
		out.Lines = append(out.Lines, line)
	}
	return &out, true
}

// utf16Offsets maps rune positions of text to UTF-16 code unit positions,
// the end included.
func utf16Offsets(text []rune) []int {
	out := make([]int, len(text)+1)
	for i, r := range text {
		out[i+1] = out[i] + utf16.RuneLen(r)
	}
	return out
}

// wordTags removes the word time tags from a line, collapsing the spaces
// around them, and returns where each tag stands in the text: a tag before
// a word marks its start, one after a word, followed by no word before
// the next tag, its end.
func wordTags(raw string) ([]rune, []wordTag) {
	locs := wordTime.FindAllStringSubmatchIndex(raw, -1)
	if len(locs) == 0 {
		return []rune(collapse(raw)), nil
	}
	var pieces []string
	var times []time.Duration
	prev := 0
	for _, loc := range locs {
		pieces = append(pieces, raw[prev:loc[0]])
		t, _ := parseTime(raw[loc[2]:loc[3]], raw[loc[4]:loc[5]])
		times = append(times, t)
		prev = loc[1]
	}
	pieces = append(pieces, raw[prev:])
	// pieces[k] precedes tag k; pieces[k+1] follows it.
	var text []rune
	space := false
	var tags []wordTag
	for k, piece := range pieces {
		if k > 0 {
			pos := len(text)
			if strings.TrimSpace(piece) != "" && space && len(text) > 0 {
				pos++ // the start of the next word, after its space
			}
			tags = append(tags, wordTag{pos: pos, time: times[k-1]})
		}
		for _, r := range piece {
			if unicode.IsSpace(r) {
				space = len(text) > 0
				continue
			}
			if space {
				text = append(text, ' ')
				space = false
			}
			text = append(text, r)
		}
	}
	return text, tags
}

// collapse trims a text and collapses its runs of spaces.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func (m *LyricMetadata) set(key, value string) {
	switch key {
	case "ar":
		m.Artist = value
	case "al":
		m.Album = value
	case "ti":
		m.Title = value
	case "au":
		m.Author = value
	case "by":
		m.By = value
	case "re", "tool":
		m.Creator = value
	case "ve":
		m.Version = value
	case "length":
		if min, sec, ok := strings.Cut(value, ":"); ok {
			if d, ok := parseTime(min, sec); ok {
				m.Length = d
			}
		}
	case "offset":
		// A positive offset shows lines earlier.
		if ms, err := strconv.Atoi(strings.TrimPrefix(value, "+")); err == nil {
			m.Offset = -time.Duration(ms) * time.Millisecond
		}
	}
}
