// Package titles compares the titles of the scraped subtitle sites with
// the video's, as subliminal's utilities.
package titles

import (
	"regexp"
	"strings"
)

var (
	spaced  = regexp.MustCompile(`[-:().]`)
	removed = regexp.MustCompile("['´`’]")
	spaces  = regexp.MustCompile(`\s+`)
)

// Sanitize strips a title of special characters and lower cases it, as
// subliminal's sanitize: dashes, colons, parentheses and dots become
// spaces, apostrophes go, and spaces are collapsed.
func Sanitize(s string) string {
	s = spaced.ReplaceAllString(s, " ")
	s = removed.ReplaceAllString(s, "")
	s = spaces.ReplaceAllString(s, " ")
	return strings.ToLower(strings.TrimSpace(s))
}
