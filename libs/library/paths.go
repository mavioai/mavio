package library

import (
	"errors"
	"strings"

	"github.com/mavioai/mavio/libs/metadata"
)

// ErrEmpty is returned for empty names where one is required.
var ErrEmpty = errors.New("empty string")

// AttributeValue returns the value of an attribute tagged in a file or
// folder name as "[name=value]", "(name-value)" or "{name=value}", e.g.
// "tmdbid" in "Movie (2020) [tmdbid=123]". "tmdb", "tvdb" and "imdb" may
// stand for "tmdbid", "tvdbid" and "imdbid", and an IMDb ID is also found
// untagged. It returns "" when there is none.
func AttributeValue(s, attribute string) (string, error) {
	if s == "" || attribute == "" {
		return "", ErrEmpty
	}
	var short string
	switch strings.ToLower(attribute) {
	case "tmdbid":
		short = "tmdb"
	case "tvdbid":
		short = "tvdb"
	case "imdbid":
		short = "imdb"
	}
	name := attribute
	if short != "" {
		name = short
	}
	for start := 0; ; {
		i := indexFoldASCII(s[start:], name)
		if i < 0 {
			break
		}
		sub := s[start:]
		end := i + len(name)
		start += end
		if i == 0 {
			continue
		}
		var closer byte
		switch sub[i-1] {
		case '[':
			closer = ']'
		case '(':
			closer = ')'
		case '{':
			closer = '}'
		default:
			continue
		}
		// An alias may be followed by "id".
		if short != "" && end+1 < len(sub) && (sub[end] == 'i' || sub[end] == 'I') && (sub[end+1] == 'd' || sub[end+1] == 'D') {
			end += 2
		}
		// A separator, at least one character and the closer must follow.
		if end+2 >= len(sub) || (sub[end] != '=' && sub[end] != '-') {
			continue
		}
		closing := strings.IndexByte(sub[end:], closer)
		if closing <= 1 {
			continue
		}
		if v := strings.TrimSpace(sub[end+1 : end+closing]); v != "" {
			return v, nil
		}
	}
	if strings.EqualFold(attribute, "imdbid") {
		if id, ok := metadata.FindIMDbID(s); ok {
			return id, nil
		}
	}
	return "", nil
}

// indexFoldASCII is strings.Index ignoring ASCII case.
func indexFoldASCII(s, sub string) int {
	lower := func(b byte) byte {
		if 'A' <= b && b <= 'Z' {
			return b + 'a' - 'A'
		}
		return b
	}
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				continue outer
			}
		}
		return i
	}
	return -1
}

// ReplaceSubPath replaces the directory prefix sub of path with repl, as
// when a library moves between machines; separators follow sub's style.
// It reports false when path is not under sub or an argument is empty.
func ReplaceSubPath(path, sub, repl string) (string, bool) {
	if path == "" || sub == "" || repl == "" || len(sub) > len(path) {
		return "", false
	}
	sub, sep := NormalizePathDetect(sub)
	path = NormalizePath(path, sep)
	subEndsWithSep := sub[len(sub)-1] == sep
	if !strings.HasPrefix(strings.ToLower(path), strings.ToLower(sub)) {
		return "", false
	}
	if len(path) > len(sub) && !subEndsWithSep && path[len(sub)] != sep {
		return "", false
	}
	// Keep the separator that starts the remainder.
	i := len(sub)
	if subEndsWithSep {
		i--
	}
	return strings.TrimRight(repl, string(sep)) + path[i:], true
}

// NormalizePath makes every separator of path sep, which must be '/' or
// '\\'.
func NormalizePath(path string, sep byte) string {
	switch sep {
	case '/':
		return strings.ReplaceAll(path, `\`, "/")
	case '\\':
		return strings.ReplaceAll(path, "/", `\`)
	}
	panic("library: separator must be '/' or '\\\\'")
}

// NormalizePathDetect normalizes path to '/' when it contains one, a likely
// Unix path, else to '\\'. It returns the separator used, 0 for "".
func NormalizePathDetect(path string) (string, byte) {
	if path == "" {
		return path, 0
	}
	sep := byte('\\')
	if strings.Contains(path, "/") {
		sep = '/'
	}
	return NormalizePath(path, sep), sep
}
