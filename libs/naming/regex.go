package naming

import (
	"fmt"
	"time"

	"github.com/dlclark/regexp2"
)

// matchTimeout bounds a single match, so that a pathological file name
// cannot stall a scan on a backtracking expression.
const matchTimeout = time.Second

// regex is a compiled .NET-syntax regular expression.
type regex struct{ re *regexp2.Regexp }

func compile(pattern string, ignoreCase bool) (*regex, error) {
	var opts regexp2.RegexOptions
	if ignoreCase {
		opts |= regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(pattern, opts)
	if err != nil {
		return nil, fmt.Errorf("compile %q: %w", pattern, err)
	}
	re.MatchTimeout = matchTimeout
	return &regex{re}, nil
}

// mustCompile compiles a built-in expression.
func mustCompile(pattern string, ignoreCase bool) *regex {
	r, err := compile(pattern, ignoreCase)
	if err != nil {
		panic(err)
	}
	return r
}

// match returns the first match in s, or nil. A timed-out match counts as
// no match.
func (r *regex) match(s string) *match {
	m, err := r.re.FindStringMatch(s)
	if err != nil || m == nil {
		return nil
	}
	return &match{m}
}

func (r *regex) isMatch(s string) bool {
	ok, err := r.re.MatchString(s)
	return err == nil && ok
}

// replaceAll replaces every match in s; on timeout s is returned unchanged.
func (r *regex) replaceAll(s, replacement string) string {
	out, err := r.re.Replace(s, replacement, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// match is a successful match.
type match struct{ m *regexp2.Match }

// group returns the value of a named group and whether it participated in
// the match.
func (m *match) group(name string) (string, bool) {
	return groupValue(m.m.GroupByName(name))
}

// groupN returns the value of a numbered group.
func (m *match) groupN(n int) (string, bool) {
	return groupValue(m.m.GroupByNumber(n))
}

// groupEnd returns the rune offset just past a named group.
func (m *match) groupEnd(name string) int {
	g := m.m.GroupByName(name)
	if g == nil {
		return 0
	}
	return g.Index + g.Length
}

// groupCount returns the number of groups, including group 0.
func (m *match) groupCount() int { return m.m.GroupCount() }

func groupValue(g *regexp2.Group) (string, bool) {
	if g == nil || len(g.Captures) == 0 {
		return "", false
	}
	return g.String(), true
}
