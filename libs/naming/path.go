package naming

import (
	"strconv"
	"strings"
	"unicode"
)

// Path helpers with .NET semantics, which the rules were written against.
// Both '/' and '\' separate path components.

func lastSeparator(p string) int { return strings.LastIndexAny(p, `/\`) }

// fileName returns the last path component.
func fileName(p string) string { return p[lastSeparator(p)+1:] }

// dirName returns the path without its last component, or "" when there is
// none; the root stays the root.
func dirName(p string) string {
	i := lastSeparator(p)
	switch {
	case i < 0:
		return ""
	case i == 0:
		return p[:1]
	}
	return p[:i]
}

// extension returns the extension of the last component including the dot,
// or "" when there is none or the name ends with a dot.
func extension(p string) string {
	name := fileName(p)
	i := strings.LastIndexByte(name, '.')
	if i < 0 || i == len(name)-1 {
		return ""
	}
	return name[i:]
}

// fileNameWithoutExtension returns the last component without its
// extension.
func fileNameWithoutExtension(p string) string {
	name := fileName(p)
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[:i]
	}
	return name
}

// containsFold reports whether list contains s, ignoring case.
func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// hasPrefixFold reports whether s starts with prefix, ignoring case.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// hasSuffixFold reports whether s ends with suffix, ignoring case.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// containsStringFold reports whether s contains sub, ignoring case.
func containsStringFold(s, sub string) bool {
	return indexFold(s, sub) >= 0
}

// indexFold returns the byte index of the first case-insensitive occurrence
// of sub in s, or -1.
func indexFold(s, sub string) int {
	if sub == "" {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// replaceFold replaces every case-insensitive occurrence of old in s.
func replaceFold(s, old, repl string) string {
	if old == "" {
		return s
	}
	var b strings.Builder
	for {
		i := indexFold(s, old)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(repl)
		s = s[i+len(old):]
	}
}

// parseInt parses a decimal 32-bit integer like .NET's int.TryParse with
// NumberStyles.Integer: surrounding white space and a sign are allowed.
func parseInt(s string) (int, bool) {
	s = strings.TrimFunc(s, unicode.IsSpace)
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }
