package naming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// Test cases ported from Jellyfin by tools/testport live in
// testdata/cases; see docs/testing.md §2. Each case file is run by
// portedCases: every theory is run or skipped with a reason, and every fact
// is ported by hand (named in facts) or skipped.

type caseFile struct {
	Theories []struct {
		Method string `json:"method"`
		Cases  []struct {
			ID   string                     `json:"id"`
			Skip string                     `json:"skip"`
			Args map[string]json.RawMessage `json:"args"`
		} `json:"cases"`
	} `json:"theories"`
	Facts []struct {
		Method string `json:"method"`
	} `json:"facts"`
}

type args map[string]json.RawMessage

type ported struct {
	// run checks one case of a theory.
	run map[string]func(t *testing.T, a args)
	// skip gives the reason a theory, a case ID or a fact is not run.
	skip map[string]string
	// facts maps a fact to the Go test that ports it.
	facts map[string]string
}

func portedCases(t *testing.T, name string, p ported) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cases", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	var f caseFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, th := range f.Theories {
		check, ok := p.run[th.Method]
		methodSkip, skipped := p.skip[th.Method]
		if !ok && !skipped {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, c := range th.Cases {
			t.Run(c.ID, func(t *testing.T) {
				switch {
				case c.Skip != "":
					t.Skip("skipped in Jellyfin: " + c.Skip)
				case p.skip[c.ID] != "":
					t.Skip(p.skip[c.ID])
				case skipped:
					t.Skip(methodSkip)
				}
				check(t, args(c.Args))
			})
		}
	}
	for _, fact := range f.Facts {
		if p.facts[fact.Method] == "" && p.skip[fact.Method] == "" {
			t.Errorf("%s: fact %s is neither ported nor skipped", name, fact.Method)
		}
	}
}

func (a args) decode(t *testing.T, name string, v any) {
	t.Helper()
	raw, ok := a[name]
	if !ok {
		t.Fatalf("missing argument %s", name)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
}

// str returns a string argument; null and an omitted optional argument
// are "".
func (a args) str(t *testing.T, name string) string {
	t.Helper()
	var s *string
	if _, ok := a[name]; !ok {
		return ""
	}
	a.decode(t, name, &s)
	if s == nil {
		return ""
	}
	return *s
}

// optInt returns an int? argument.
func (a args) optInt(t *testing.T, name string) *int {
	t.Helper()
	var n *int
	if _, ok := a[name]; !ok {
		return nil
	}
	a.decode(t, name, &n)
	return n
}

func (a args) integer(t *testing.T, name string) int {
	t.Helper()
	var n int
	a.decode(t, name, &n)
	return n
}

func (a args) boolean(t *testing.T, name string) bool {
	t.Helper()
	var b bool
	if _, ok := a[name]; !ok {
		return false
	}
	a.decode(t, name, &b)
	return b
}

func (a args) optBool(t *testing.T, name string) *bool {
	t.Helper()
	var b *bool
	if _, ok := a[name]; !ok {
		return nil
	}
	a.decode(t, name, &b)
	return b
}

// symbol returns the member name of an enum argument ("Trailer" for
// ExtraType.Trailer), or "" for null.
func (a args) symbol(t *testing.T, name string) string {
	t.Helper()
	var s *struct {
		Symbol string `json:"$symbol"`
	}
	a.decode(t, name, &s)
	if s == nil {
		return ""
	}
	_, member, _ := strings.Cut(s.Symbol, ".")
	return member
}

// object decodes a "$new" argument into its named arguments.
func (a args) object(t *testing.T, name string) (named args, positional []json.RawMessage) {
	t.Helper()
	var o struct {
		Named args              `json:"named"`
		Args  []json.RawMessage `json:"args"`
	}
	a.decode(t, name, &o)
	return o.Named, o.Args
}

// extraKinds maps Jellyfin's ExtraType members.
var extraKinds = map[string]core.ExtraKind{
	"":                "",
	"Unknown":         core.ExtraOther,
	"Clip":            core.ExtraClip,
	"Trailer":         core.ExtraTrailer,
	"BehindTheScenes": core.ExtraBehindTheScene,
	"DeletedScene":    core.ExtraDeletedScene,
	"Interview":       core.ExtraInterview,
	"Scene":           core.ExtraScene,
	"Sample":          core.ExtraSample,
	"ThemeSong":       core.ExtraThemeSong,
	"ThemeVideo":      core.ExtraThemeVideo,
	"Featurette":      core.ExtraFeaturette,
	"Short":           core.ExtraShort,
}

func eqInt(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func fmtInt(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}

var parser = Default()
