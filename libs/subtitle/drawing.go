package subtitle

import (
	"regexp"
	"strings"
)

var (
	// drawingBlocks are SSA drawing-mode blocks, from {\p1} to {\p0}.
	drawingBlocks = regexp.MustCompile(`(?is)\\p[0-9].*?\\p[0-9]`)
	// vectorPrefix is a line that is, or begins with, a vector path: a
	// drawing command followed by at least two commands or coordinates.
	vectorPrefix = regexp.MustCompile(`^[mlcbsnMLCBSN](?:\s+(?:[mlcbsnMLCBSN]|[+-]?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)){2,}(?:\s|$)`)
	// spaces are runs of spaces and tabs.
	spaces = regexp.MustCompile(`[ \t]+`)
)

// SpokenText returns a cue's plain text without SSA drawings, as Lingarr's
// RemoveMarkup: drawing-mode blocks removed with the markup, spaces
// collapsed, and each line that is, or begins with, a vector path left
// empty. Fansub scripts draw signs and karaoke with such cues, which have
// nothing to translate.
func SpokenText(text string) string {
	lines := strings.Split(PlainText(drawingBlocks.ReplaceAllString(text, "")), "\n")
	for i, l := range lines {
		l = strings.TrimSpace(spaces.ReplaceAllString(l, " "))
		if vectorPrefix.MatchString(l) {
			l = ""
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}
