package subtitle

import (
	"fmt"
	"strconv"
	"strings"
)

// parseVTT reads WebVTT cues; NOTE, STYLE and REGION blocks are skipped.
func parseVTT(lines []string) []Cue {
	var cues []Cue
	// Blocks are separated by blank lines.
	var block []string
	flush := func() {
		defer func() { block = block[:0] }()
		if len(block) == 0 {
			return
		}
		first := strings.TrimSpace(block[0])
		if strings.HasPrefix(first, "WEBVTT") || strings.HasPrefix(first, "NOTE") ||
			strings.HasPrefix(first, "STYLE") || strings.HasPrefix(first, "REGION") {
			return
		}
		timing := 0
		if _, _, ok := parseTiming(block[0]); !ok {
			timing = 1 // an identifier line comes first
		}
		if timing >= len(block) {
			return
		}
		start, end, ok := parseTiming(block[timing])
		if !ok {
			return
		}
		index := len(cues) + 1
		if timing == 1 {
			if n, err := strconv.Atoi(first); err == nil {
				index = n
			}
		}
		cues = append(cues, Cue{Index: index, Start: start, End: end, Text: strings.Join(block[timing+1:], "\n")})
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		block = append(block, line)
	}
	flush()
	return cues
}

func writeVTT(s *Subtitle) []byte {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, c := range s.Cues {
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", timestamp(c.Start, "."), timestamp(c.End, "."), vttText(c.Text))
	}
	return []byte(b.String())
}

// vttText makes cue text safe for WebVTT: override blocks are removed, and
// blank lines, which would end the cue, are dropped.
func vttText(text string) string {
	var lines []string
	for line := range strings.SplitSeq(plainText(text), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.ReplaceAll(line, "-->", "->"))
		}
	}
	return strings.Join(lines, "\n")
}
