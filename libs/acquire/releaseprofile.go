package acquire

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dlclark/regexp2"
)

// ReleaseProfile requires or forbids terms in release titles, as Sonarr's
// ReleaseProfile.
type ReleaseProfile struct {
	ID      int
	Name    string
	Enabled bool
	// Required are terms of which a release title must contain one.
	Required []string
	// Ignored are terms a release title must contain none of.
	Ignored []string
	// IndexerID limits the profile to an indexer's releases; 0 for all.
	IndexerID int
	// Tags are the tags of the movies and series the profile applies to,
	// all when none.
	Tags []int
}

// perlRegex is Sonarr's form of a term that is a regular expression.
var perlRegex = regexp.MustCompile(`/(?P<pattern>.*)/(?P<modifiers>[a-z]*)`)

// TermMatcher matches release profile terms, Sonarr's TermMatcherService:
// a term /pattern/modifiers is a .NET regular expression, any other term
// a case-insensitive substring. Its zero value is ready to use.
type TermMatcher struct {
	cache map[string]*regexp2.Regexp
}

// compileTerm returns the expression of a term, or nil for a substring.
func compileTerm(term string) (*regexp2.Regexp, error) {
	m := perlRegex.FindStringSubmatch(term)
	if m == nil {
		return nil, nil
	}
	var opts regexp2.RegexOptions
	for _, c := range m[2] {
		switch c {
		case 'm':
			opts |= regexp2.Multiline
		case 's':
			opts |= regexp2.Singleline
		case 'i':
			opts |= regexp2.IgnoreCase
		case 'x':
			opts |= regexp2.IgnorePatternWhitespace
		case 'n':
			opts |= regexp2.ExplicitCapture
		default:
			return nil, fmt.Errorf("unknown or unsupported perl regex modifier: %c", c)
		}
	}
	re, err := regexp2.Compile(m[1], opts)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = regexTimeout
	return re, nil
}

// Validate checks the profile's terms.
func (p *ReleaseProfile) Validate() error {
	if len(p.Required) == 0 && len(p.Ignored) == 0 {
		return fmt.Errorf("release profile %q has no terms", p.Name)
	}
	for _, t := range slices.Concat(p.Required, p.Ignored) {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("release profile %q has an empty term", p.Name)
		}
		if _, err := compileTerm(t); err != nil {
			return fmt.Errorf("term %q: %w", t, err)
		}
	}
	return nil
}

// Match reports whether a term is in a title; an invalid expression is
// in none.
func (m *TermMatcher) Match(term, title string) bool {
	re, ok := m.cache[term]
	if !ok {
		var err error
		if re, err = compileTerm(term); err != nil {
			return false
		}
		if m.cache == nil {
			m.cache = map[string]*regexp2.Regexp{}
		}
		m.cache[term] = re
	}
	if re == nil {
		return strings.Contains(strings.ToLower(title), strings.ToLower(term))
	}
	ok, err := re.MatchString(title)
	return err == nil && ok
}

// AppliesTo reports whether the enabled profile applies to a release of an
// indexer for an item of tags, as Sonarr's EnabledForTags.
func (p *ReleaseProfile) AppliesTo(tags []int, indexerID int) bool {
	return p.Enabled &&
		(p.IndexerID == 0 || p.IndexerID == indexerID) &&
		(len(p.Tags) == 0 || slices.ContainsFunc(p.Tags, func(t int) bool { return slices.Contains(tags, t) }))
}

// CheckTerms returns why a title fails the profiles, Sonarr's
// ReleaseRestrictionsSpecification, or "" when it passes: a profile's
// required terms all missing, or ignored terms present.
func (m *TermMatcher) CheckTerms(profiles []ReleaseProfile, title string) string {
	for _, p := range profiles {
		if len(p.Required) > 0 && !slices.ContainsFunc(p.Required, func(t string) bool { return m.Match(t, title) }) {
			return "Does not contain one of the required terms: " + strings.Join(p.Required, ", ")
		}
	}
	for _, p := range profiles {
		var found []string
		for _, t := range p.Ignored {
			if m.Match(t, title) {
				found = append(found, t)
			}
		}
		if len(found) > 0 {
			return "Contains these ignored terms: " + strings.Join(found, ", ")
		}
	}
	return ""
}
