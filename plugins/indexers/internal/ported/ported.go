// Package ported runs the test cases tools/testport ported from Prowlarr
// into the plugin's testdata/cases (see docs/testing.md §2): every theory
// of a case file is run or skipped with a reason, and every fact is ported
// by hand, named in Cases.Facts, or skipped. Only tests use it.
package ported

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Args are the arguments of a case, by parameter name.
type Args map[string]json.RawMessage

// Cases says how a case file is run.
type Cases struct {
	// Run checks one case of a theory.
	Run map[string]func(t *testing.T, a Args)
	// Skip gives the reason a theory, a case ID or a fact is not run.
	Skip map[string]string
	// Facts maps a fact to the Go test that ports it.
	Facts map[string]string
}

type caseFile struct {
	Theories []struct {
		Method string `json:"method"`
		Skip   string `json:"skip"`
		Cases  []struct {
			ID   string `json:"id"`
			Skip string `json:"skip"`
			Args Args   `json:"args"`
		} `json:"cases"`
	} `json:"theories"`
	Facts []struct {
		Method string `json:"method"`
		Skip   string `json:"skip"`
	} `json:"facts"`
}

// Run runs the case file name, a path under testdata/cases.
func Run(t *testing.T, name string, c Cases) {
	t.Helper()
	var f caseFile
	data, err := os.ReadFile(Path(t, "cases", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, th := range f.Theories {
		check, ok := c.Run[th.Method]
		methodSkip, skipped := c.Skip[th.Method]
		if !ok && !skipped && th.Skip == "" {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, cs := range th.Cases {
			t.Run(cs.ID, func(t *testing.T) {
				switch {
				case th.Skip != "":
					t.Skip("skipped upstream: " + th.Skip)
				case cs.Skip != "":
					t.Skip("skipped upstream: " + cs.Skip)
				case c.Skip[cs.ID] != "":
					t.Skip(c.Skip[cs.ID])
				case skipped:
					t.Skip(methodSkip)
				}
				check(t, cs.Args)
			})
		}
	}
	for _, fact := range f.Facts {
		if fact.Skip == "" && c.Facts[fact.Method] == "" && c.Skip[fact.Method] == "" {
			t.Errorf("%s: fact %s is neither ported nor skipped", name, fact.Method)
		}
	}
}

// Path returns the path of a file under the plugin's testdata directory.
func Path(t *testing.T, elem ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's directory")
		}
		dir = parent
	}
	return filepath.Join(append([]string{dir, "testdata"}, elem...)...)
}

// Response returns a recorded response under testdata/responses.
func Response(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(Path(t, "responses", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Has reports whether the case has the argument.
func (a Args) Has(name string) bool {
	_, ok := a[name]
	return ok
}

// Decode decodes an argument into v.
func (a Args) Decode(t *testing.T, name string, v any) {
	t.Helper()
	raw, ok := a[name]
	if !ok {
		t.Fatalf("missing argument %s", name)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
}

// Str returns a string argument; null is "" and ok false.
func (a Args) Str(t *testing.T, name string) (s string, ok bool) {
	t.Helper()
	var p *string
	a.Decode(t, name, &p)
	if p == nil {
		return "", false
	}
	return *p, true
}

// Int returns an integer argument; null is 0 and ok false.
func (a Args) Int(t *testing.T, name string) (n int64, ok bool) {
	t.Helper()
	var p *int64
	a.Decode(t, name, &p)
	if p == nil {
		return 0, false
	}
	return *p, true
}

// Float returns a number argument.
func (a Args) Float(t *testing.T, name string) float64 {
	t.Helper()
	var n float64
	a.Decode(t, name, &n)
	return n
}

// Bool returns a boolean argument.
func (a Args) Bool(t *testing.T, name string) bool {
	t.Helper()
	var b bool
	a.Decode(t, name, &b)
	return b
}

// Symbol returns the member name of an enum argument ("Movies" for
// NewznabStandardCategory.Movies), or "" for null.
func (a Args) Symbol(t *testing.T, name string) string {
	t.Helper()
	var s *struct {
		Symbol string `json:"$symbol"`
	}
	a.Decode(t, name, &s)
	if s == nil {
		return ""
	}
	return s.Symbol[strings.LastIndex(s.Symbol, ".")+1:]
}

// Expr returns the source text of a non-constant argument.
func (a Args) Expr(t *testing.T, name string) string {
	t.Helper()
	var e struct {
		Expr string `json:"$expr"`
	}
	a.Decode(t, name, &e)
	if e.Expr == "" {
		t.Fatalf("argument %s is not an expression", name)
	}
	return e.Expr
}
