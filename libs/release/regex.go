package release

import (
	"time"

	"github.com/dlclark/regexp2"
)

// matchTimeout bounds a single match, so that a pathological release name
// cannot stall a search on a backtracking expression.
const matchTimeout = time.Second

// regex is a compiled .NET-syntax regular expression. Indexes are in runes,
// as reported by regexp2.
type regex struct{ re *regexp2.Regexp }

// Options of a regex.
const (
	ignoreCase = regexp2.IgnoreCase
	patternWS  = regexp2.IgnorePatternWhitespace
)

// re compiles a built-in expression.
func re(pattern string, opts ...regexp2.RegexOptions) *regex {
	var o regexp2.RegexOptions
	for _, opt := range opts {
		o |= opt
	}
	r := regexp2.MustCompile(pattern, o)
	r.MatchTimeout = matchTimeout
	return &regex{r}
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

// matches returns every match in s.
func (r *regex) matches(s string) []*match {
	var out []*match
	m, err := r.re.FindStringMatch(s)
	for err == nil && m != nil {
		out = append(out, &match{m})
		m, err = r.re.FindNextMatch(m)
	}
	return out
}

func (r *regex) isMatch(s string) bool {
	ok, err := r.re.MatchString(s)
	return err == nil && ok
}

// replace replaces every match in s with the .NET substitution pattern
// replacement; on timeout s is returned unchanged.
func (r *regex) replace(s, replacement string) string {
	out, err := r.re.Replace(s, replacement, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// replaceFunc replaces every match in s with the result of f.
func (r *regex) replaceFunc(s string, f func(m *match) string) string {
	out, err := r.re.ReplaceFunc(s, func(m regexp2.Match) string { return f(&match{&m}) }, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// regexReplace is an expression with its replacement, like Sonarr's
// RegexReplace.
type regexReplace struct {
	*regex
	replacement string
}

func rr(pattern, replacement string, opts ...regexp2.RegexOptions) regexReplace {
	return regexReplace{re(pattern, opts...), replacement}
}

func (r regexReplace) apply(s string) string { return r.replace(s, r.replacement) }

// tryReplace replaces the matches in *s and reports whether there were any.
func (r regexReplace) tryReplace(s *string) bool {
	if !r.isMatch(*s) {
		return false
	}
	*s = r.replace(*s, r.replacement)
	return true
}

// match is a successful match.
type match struct{ m *regexp2.Match }

// String returns the matched text.
func (m *match) String() string { return m.m.String() }

// group returns the value of a named group and whether it participated in
// the match. For a group captured several times it is the last capture.
func (m *match) group(name string) (string, bool) {
	g := m.m.GroupByName(name)
	if g == nil || len(g.Captures) == 0 {
		return "", false
	}
	return g.String(), true
}

// value returns the value of a named group, or "".
func (m *match) value(name string) string {
	v, _ := m.group(name)
	return v
}

// has reports whether a named group participated in the match.
func (m *match) has(name string) bool {
	_, ok := m.group(name)
	return ok
}

// capture is one capture of a group.
type capture struct {
	Value         string
	Index, Length int
}

func (c capture) end() int { return c.Index + c.Length }

// captures returns every capture of a named group, in order.
func (m *match) captures(name string) []capture {
	g := m.m.GroupByName(name)
	if g == nil {
		return nil
	}
	out := make([]capture, 0, len(g.Captures))
	for _, c := range g.Captures {
		out = append(out, capture{c.String(), c.Index, c.Length})
	}
	return out
}

// groupEnd returns the rune offset just past the last capture of a named
// group, or 0 when it did not participate.
func (m *match) groupEnd(name string) int {
	g := m.m.GroupByName(name)
	if g == nil || len(g.Captures) == 0 {
		return 0
	}
	return g.Index + g.Length
}

// split splits s around the matches, like .NET's Regex.Split for an
// expression without capturing groups.
func (r *regex) split(s string) []string {
	runes := []rune(s)
	var out []string
	start := 0
	for _, m := range r.matches(s) {
		out = append(out, string(runes[start:m.m.Index]))
		start = m.m.Index + m.m.Length
	}
	return append(out, string(runes[start:]))
}
