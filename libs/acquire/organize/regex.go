package organize

import (
	"time"

	"github.com/dlclark/regexp2"
)

// matchTimeout bounds a single match, so that a pathological title cannot
// stall naming on a backtracking expression.
const matchTimeout = time.Second

// regex is a compiled .NET-syntax regular expression, as Radarr writes
// them.
type regex struct{ re *regexp2.Regexp }

// re compiles a built-in expression.
func re(pattern string, opts regexp2.RegexOptions) *regex {
	r := regexp2.MustCompile(pattern, opts)
	r.MatchTimeout = matchTimeout
	return &regex{r}
}

// match returns the first match in s, or nil. A timed-out match counts as
// no match.
func (r *regex) match(s string) *regexp2.Match {
	m, err := r.re.FindStringMatch(s)
	if err != nil {
		return nil
	}
	return m
}

// count returns the number of matches in s.
func (r *regex) count(s string) int {
	n := 0
	m, err := r.re.FindStringMatch(s)
	for err == nil && m != nil {
		n++
		m, err = r.re.FindNextMatch(m)
	}
	return n
}

// all returns every match in s.
func (r *regex) all(s string) []*regexp2.Match {
	var out []*regexp2.Match
	m, err := r.re.FindStringMatch(s)
	for err == nil && m != nil {
		out = append(out, m)
		m, err = r.re.FindNextMatch(m)
	}
	return out
}

// matches returns the text of every match in s.
func (r *regex) matches(s string) []string {
	var out []string
	m, err := r.re.FindStringMatch(s)
	for err == nil && m != nil {
		out = append(out, m.String())
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

// replaceFunc replaces every match in s with the result of f; on timeout
// s is returned unchanged.
func (r *regex) replaceFunc(s string, f func(m regexp2.Match) string) string {
	out, err := r.re.ReplaceFunc(s, f, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// group returns the last capture of a named group, or "".
func group(m regexp2.Match, name string) string {
	g := m.GroupByName(name)
	if g == nil || len(g.Captures) == 0 {
		return ""
	}
	return g.String()
}
