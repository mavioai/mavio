package library

import "strings"

// glob is a path pattern: "**" matches any number of path segments, "*"
// any run of characters within a segment and "?" one character. Matching
// ignores case and accepts '/' and '\\' as separators.
type glob []string

func parseGlob(pattern string) glob { return strings.Split(pattern, "/") }

// match reports whether the whole path matches.
func (g glob) match(path string) bool {
	segs := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	return matchSegments(g, segs)
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			rest := pat[1:]
			for i := 0; i <= len(segs); i++ {
				if matchSegments(rest, segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 || !matchSegment(pat[0], segs[0]) {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// matchSegment matches one segment with "*", "?", "[...]" classes ("!" or
// "^" negates, "a-z" ranges) and backslash escapes, ignoring case.
func matchSegment(pat, s string) bool {
	return matchRunes([]rune(strings.ToLower(pat)), []rune(strings.ToLower(s)))
}

func matchRunes(p, s []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			for i := 0; i <= len(s); i++ {
				if matchRunes(p, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
		case '[':
			if len(s) == 0 {
				return false
			}
			ok, n := matchClass(p, s[0])
			if n == 0 { // no closing bracket: a literal "["
				if s[0] != '[' {
					return false
				}
				n = 1
			} else if !ok {
				return false
			}
			p, s = p[n:], s[1:]
			continue
		case '\\':
			if len(p) > 1 {
				p = p[1:]
			}
			fallthrough
		default:
			if len(s) == 0 || p[0] != s[0] {
				return false
			}
		}
		p, s = p[1:], s[1:]
	}
	return len(s) == 0
}

// matchClass matches c against the class at the start of p and returns the
// class length, 0 when the class is not closed.
func matchClass(p []rune, c rune) (bool, int) {
	i := 1
	negate := i < len(p) && (p[i] == '!' || p[i] == '^')
	if negate {
		i++
	}
	matched := false
	for first := true; i < len(p); first = false {
		if p[i] == ']' && !first {
			return matched != negate, i + 1
		}
		lo := p[i]
		if lo == '\\' && i+1 < len(p) {
			i++
			lo = p[i]
		}
		hi := lo
		if i+2 < len(p) && p[i+1] == '-' && p[i+2] != ']' {
			hi = p[i+2]
			i += 2
		}
		if lo <= c && c <= hi {
			matched = true
		}
		i++
	}
	return false, 0
}

// ignorePatterns are files and folders never added to a library: samples,
// metadata and system folders, trickplay data, recycle bins, snapshots and
// hidden files.
var ignorePatterns = func() []glob {
	patterns := []string{
		"**/small.jpg", "**/albumart.jpg",
		// Samples, also Hungarian ("minta"), with extensions of 1 to 5
		// characters.
		"**/sample/*", "**/minta/*",
		"**/*.trickplay", "**/*.trickplay/**",
		"**/.*", "**/thumbs.db", "**/*.bts", "**/*.sync",
	}
	for _, word := range []string{"sample", "minta"} {
		for n := 1; n <= 5; n++ {
			ext := strings.Repeat("?", n)
			patterns = append(patterns, "**/"+word+"."+ext, "**/*."+word+"."+ext)
		}
	}
	for _, dir := range []string{
		"metadata", "ps3_update", "ps3_vprm", "extrafanart", "extrathumbs", ".actors", ".wd_tv",
		"lost+found", "subs", ".snapshots", ".snapshot",
		// Windows Media Center recordings in progress.
		"TempRec", "TempSBE",
		// Synology, QNAP and Windows.
		"eaDir", "@eaDir", "#recycle", "@Recycle", ".@__thumb", "$RECYCLE.BIN", "System Volume Information",
		".grab", ".zfs",
	} {
		patterns = append(patterns, "**/"+dir, "**/"+dir+"/**")
	}
	globs := make([]glob, len(patterns))
	for i, p := range patterns {
		globs[i] = parseGlob(p)
	}
	return globs
}()

// IgnoredPath reports whether a path matches the built-in ignore patterns.
func IgnoredPath(path string) bool {
	for _, g := range ignorePatterns {
		if g.match(path) {
			return true
		}
	}
	return false
}
