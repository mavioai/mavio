package library

import (
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"
)

// IgnoreRules is a parsed .ignore file: gitignore patterns, matched without
// regard to case. Every rule is tested against the whole path and the last
// matching rule decides, so "!name" can bring back a file in an ignored
// folder. A file with no usable rule ignores everything.
type IgnoreRules struct {
	rules []ignoreRule
}

type ignoreRule struct {
	segs     []string // glob segments; "**" spans segments
	negate   bool
	dirOnly  bool
	anchored bool
}

// ParseIgnoreRules parses the lines of an .ignore file. Blank lines and
// comments are skipped, and so are invalid patterns: a backslash escapes
// only punctuation and spaces.
func ParseIgnoreRules(lines []string) IgnoreRules {
	var r IgnoreRules
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var rule ignoreRule
		if p, ok := strings.CutPrefix(line, "!"); ok {
			rule.negate, line = true, p
		}
		if p, ok := strings.CutSuffix(line, "/"); ok {
			rule.dirOnly, line = true, p
		}
		if p, ok := strings.CutPrefix(line, "/"); ok {
			rule.anchored, line = true, p
		} else if strings.Contains(line, "/") {
			rule.anchored = true
		}
		if line == "" || !validEscapes(line) {
			continue
		}
		rule.segs = strings.Split(line, "/")
		r.rules = append(r.rules, rule)
	}
	return r
}

func validEscapes(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != '\\' {
			continue
		}
		if i+1 == len(p) {
			return false
		}
		c := p[i+1]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' {
			return false
		}
		i++
	}
	return true
}

// Empty reports whether the rules ignore everything.
func (r IgnoreRules) Empty() bool { return len(r.rules) == 0 }

// Ignored reports whether a slash-separated path is ignored; isDir marks
// folders, which "name/" patterns require.
func (r IgnoreRules) Ignored(p string, isDir bool) bool {
	if r.Empty() {
		return true
	}
	segs := strings.FieldsFunc(p, func(c rune) bool { return c == '/' })
	ignored := false
	for _, rule := range r.rules {
		if rule.matches(segs, isDir) {
			ignored = !rule.negate
		}
	}
	return ignored
}

// matches reports whether the rule matches the path or one of its folders.
func (rule ignoreRule) matches(segs []string, isDir bool) bool {
	// A match ending before the last segment matched a folder.
	ok := func(end int) bool { return end < len(segs) || !rule.dirOnly || isDir }
	if !rule.anchored {
		for i, s := range segs {
			if matchSegment(rule.segs[0], s) && ok(i+1) {
				return true
			}
		}
		return false
	}
	for end := 1; end <= len(segs); end++ {
		if matchSegments(rule.segs, segs[:end]) && ok(end) {
			return true
		}
	}
	return false
}

// CheckIgnoreRules applies rules to a path; normalize turns backslashes
// into slashes first, for Windows paths.
func CheckIgnoreRules(p string, rules []string, isDir, normalize bool) bool {
	if normalize {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	return ParseIgnoreRules(rules).Ignored(p, isDir)
}

// IgnoreFiles finds the .ignore file governing a path within a file
// system: the nearest one in the path's folder or above. Parsed files are
// cached until their size or modification time changes.
type IgnoreFiles struct {
	FS    fs.FS
	mu    sync.Mutex
	cache map[string]cachedIgnore
}

type cachedIgnore struct {
	size  int64
	mtime time.Time
	rules IgnoreRules
}

// Ignored reports whether the slash-separated path p, relative to the file
// system root, is ignored by an .ignore file.
func (f *IgnoreFiles) Ignored(p string, isDir bool) (bool, error) {
	dir := path.Dir(p)
	if isDir {
		dir = p
	}
	for {
		name := path.Join(dir, ".ignore")
		rules, ok, err := f.load(name)
		if err != nil {
			return false, err
		}
		if ok {
			rel := strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
			if rel == "" {
				rel = path.Base(p)
			}
			return rules.Ignored(rel, isDir), nil
		}
		if dir == "." || dir == "/" {
			return false, nil
		}
		dir = path.Dir(dir)
	}
}

func (f *IgnoreFiles) load(name string) (IgnoreRules, bool, error) {
	info, err := fs.Stat(f.FS, name)
	if err != nil {
		if isNotExist(err) {
			f.mu.Lock()
			delete(f.cache, name)
			f.mu.Unlock()
			return IgnoreRules{}, false, nil
		}
		return IgnoreRules{}, false, err
	}
	f.mu.Lock()
	c, hit := f.cache[name]
	f.mu.Unlock()
	if hit && c.size == info.Size() && c.mtime.Equal(info.ModTime()) {
		return c.rules, true, nil
	}
	data, err := fs.ReadFile(f.FS, name)
	if err != nil {
		if isNotExist(err) {
			return IgnoreRules{}, false, nil
		}
		return IgnoreRules{}, false, err
	}
	rules := ParseIgnoreRules(strings.Split(string(data), "\n"))
	f.mu.Lock()
	if f.cache == nil {
		f.cache = map[string]cachedIgnore{}
	}
	f.cache[name] = cachedIgnore{size: info.Size(), mtime: info.ModTime(), rules: rules}
	f.mu.Unlock()
	return rules, true, nil
}
