package subtitle

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors returned by the package.
var (
	// ErrUnsupportedFormat is returned for a format the package cannot read
	// or write.
	ErrUnsupportedFormat = errors.New("unsupported subtitle format")
	// ErrNoCues is returned when a file yields no cues.
	ErrNoCues = errors.New("no subtitle cues")
)

// Cue is one timed piece of text.
type Cue struct {
	// Index is the cue number from the file, or its position from 1.
	Index int
	Start time.Duration
	End   time.Duration
	// Text has lines separated by "\n" and keeps the source's inline
	// markup: HTML-like tags for SRT and WebVTT, override blocks such as
	// {\pos(400,570)} for SSA and ASS.
	Text string
}

// Subtitle is a parsed subtitle track.
type Subtitle struct {
	Cues []Cue
}

// Format names, as file extensions without the dot.
const (
	SRT  = "srt"
	SSA  = "ssa"
	ASS  = "ass"
	VTT  = "vtt"
	TTML = "ttml"
	// JSON is Jellyfin's JSON track format.
	JSON = "json"
)

// normalize maps format aliases and extensions to a format name.
func normalize(format string) string {
	f := strings.ToLower(strings.TrimPrefix(format, "."))
	switch f {
	case "subrip":
		return SRT
	case "webvtt":
		return VTT
	}
	return f
}

// CanParse reports whether Parse reads format.
func CanParse(format string) bool {
	switch normalize(format) {
	case SRT, SSA, ASS, VTT:
		return true
	}
	return false
}

// CanWrite reports whether Write produces format.
func CanWrite(format string) bool {
	switch normalize(format) {
	case SRT, SSA, ASS, VTT, TTML, JSON:
		return true
	}
	return false
}

// Parse reads a UTF-8 subtitle file of the given format ("srt", "ssa",
// "ass", "vtt" or an alias such as "subrip"). A byte order mark and CRLF
// line endings are accepted.
func Parse(data []byte, format string) (*Subtitle, error) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	var cues []Cue
	switch normalize(format) {
	case SRT:
		cues = parseSRT(lines)
	case SSA, ASS:
		cues = parseSSA(lines)
	case VTT:
		cues = parseVTT(lines)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
	}
	if len(cues) == 0 {
		return nil, fmt.Errorf("%s: %w", format, ErrNoCues)
	}
	return &Subtitle{Cues: cues}, nil
}

// Write renders s in the given format.
func Write(s *Subtitle, format string) ([]byte, error) {
	switch normalize(format) {
	case SRT:
		return writeSRT(s), nil
	case VTT:
		return writeVTT(s), nil
	case SSA:
		return writeSSA(s, false), nil
	case ASS:
		return writeSSA(s, true), nil
	case TTML:
		return writeTTML(s), nil
	case JSON:
		return writeJSON(s)
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
}

// Filter keeps the cues of a time window, as for a stream starting at
// start: cues that ended before start and, when end is positive, cues that
// start after end are dropped. Unless preserveTimestamps is set, the
// remaining cues are shifted by -start, clamped at zero.
func (s *Subtitle) Filter(start, end time.Duration, preserveTimestamps bool) {
	kept := s.Cues[:0]
	for _, c := range s.Cues {
		if c.Start < start && c.End < start {
			continue
		}
		if end > 0 && c.Start > end {
			continue
		}
		if !preserveTimestamps {
			c.Start, c.End = max(0, c.Start-start), max(0, c.End-start)
		}
		kept = append(kept, c)
	}
	s.Cues = kept
}

// Convert parses data in format from, filters it (see Filter) and writes
// it in format to.
func Convert(data []byte, from, to string, start, end time.Duration, preserveTimestamps bool) ([]byte, error) {
	if !CanWrite(to) {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, to)
	}
	s, err := Parse(data, from)
	if err != nil {
		return nil, err
	}
	s.Filter(start, end, preserveTimestamps)
	return Write(s, to)
}
