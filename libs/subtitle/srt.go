package subtitle

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// srtTiming matches an SRT timing line; WebVTT timings share the syntax
// with a dot before the milliseconds and optional hours.
var srtTiming = regexp.MustCompile(`^\s*(?:(\d+):)?(\d{1,2}):(\d{1,2})[,.](\d{1,3})\s*-->\s*(?:(\d+):)?(\d{1,2}):(\d{1,2})[,.](\d{1,3})`)

// parseTiming parses the start and end of a timing line.
func parseTiming(line string) (start, end time.Duration, ok bool) {
	m := srtTiming.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	return clock(m[1], m[2], m[3], m[4]), clock(m[5], m[6], m[7], m[8]), true
}

// clock converts hours, minutes, seconds and a fraction of a second.
func clock(h, m, s, frac string) time.Duration {
	n := func(v string) time.Duration {
		i, _ := strconv.Atoi(v)
		return time.Duration(i)
	}
	// The fraction is milliseconds when it has three digits, hundredths
	// when it has two.
	ms := n(frac)
	for range 3 - len(frac) {
		ms *= 10
	}
	return n(h)*time.Hour + n(m)*time.Minute + n(s)*time.Second + ms*time.Millisecond
}

func isIndex(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	_, err := strconv.Atoi(line)
	return err == nil
}

// parseSRT reads SubRip cues. A cue starts at a timing line, optionally
// preceded by its number; its text runs until the next cue, so blank lines
// inside the text are kept.
func parseSRT(lines []string) []Cue {
	var cues []Cue
	// A header is a cue's timing line and its number line, -1 if none.
	type header struct{ number, timing int }
	var headers []header
	for i, line := range lines {
		if _, _, ok := parseTiming(line); !ok {
			continue
		}
		h := header{number: -1, timing: i}
		if i > 0 && isIndex(lines[i-1]) {
			h.number = i - 1
		}
		headers = append(headers, h)
	}
	for k, h := range headers {
		end := len(lines)
		if k+1 < len(headers) {
			next := headers[k+1]
			end = next.timing
			if next.number >= 0 {
				end = next.number
			}
		}
		start, stop, _ := parseTiming(lines[h.timing])
		text := lines[h.timing+1 : end]
		for len(text) > 0 && strings.TrimSpace(text[len(text)-1]) == "" {
			text = text[:len(text)-1]
		}
		index := len(cues) + 1
		if h.number >= 0 {
			index, _ = strconv.Atoi(strings.TrimSpace(lines[h.number]))
		}
		cues = append(cues, Cue{Index: index, Start: start, End: stop, Text: strings.Join(text, "\n")})
	}
	return cues
}

// timestamp formats d as HH:MM:SS followed by sep and milliseconds.
func timestamp(d time.Duration, sep string) string {
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, sep, ms%1000)
}

func writeSRT(s *Subtitle) []byte {
	var b strings.Builder
	for i, c := range s.Cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, timestamp(c.Start, ","), timestamp(c.End, ","), plainText(c.Text))
	}
	return []byte(b.String())
}
