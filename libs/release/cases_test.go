package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test cases ported from Sonarr (testdata/cases/series) and Radarr
// (testdata/cases/movie) by tools/testport; see docs/testing.md §2. Each case
// file is run by portedCases: every theory is run or skipped with a reason,
// and every fact is ported by hand (named in facts) or skipped.

type caseFile struct {
	Theories []struct {
		Method string `json:"method"`
		Skip   string `json:"skip"`
		Cases  []struct {
			ID   string                     `json:"id"`
			Skip string                     `json:"skip"`
			Args map[string]json.RawMessage `json:"args"`
		} `json:"cases"`
	} `json:"theories"`
	Facts []struct {
		Method string `json:"method"`
		Skip   string `json:"skip"`
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
	f, err := readCaseFile(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range f.Theories {
		check, ok := p.run[th.Method]
		methodSkip, skipped := p.skip[th.Method]
		if !ok && !skipped && th.Skip == "" {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, c := range th.Cases {
			t.Run(c.ID, func(t *testing.T) {
				switch {
				case th.Skip != "":
					t.Skip("skipped upstream: " + th.Skip)
				case c.Skip != "":
					t.Skip("skipped upstream: " + c.Skip)
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
		if fact.Skip == "" && p.facts[fact.Method] == "" && p.skip[fact.Method] == "" {
			t.Errorf("%s: fact %s is neither ported nor skipped", name, fact.Method)
		}
	}
}

func readCaseFile(name string) (caseFile, error) {
	var f caseFile
	data, err := os.ReadFile(filepath.Join("testdata", "cases", filepath.FromSlash(name)))
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("%s: %w", name, err)
	}
	return f, nil
}

func (a args) has(name string) bool {
	_, ok := a[name]
	return ok
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

// str returns a string argument; null and an omitted argument are "".
func (a args) str(t *testing.T, name string) string {
	t.Helper()
	var s *string
	if !a.has(name) {
		return ""
	}
	a.decode(t, name, &s)
	if s == nil {
		return ""
	}
	return *s
}

func (a args) integer(t *testing.T, name string) int {
	t.Helper()
	var n int
	a.decode(t, name, &n)
	return n
}

func (a args) number(t *testing.T, name string) float64 {
	t.Helper()
	var n float64
	a.decode(t, name, &n)
	return n
}

func (a args) ints(t *testing.T, name string) []int {
	t.Helper()
	var n []int
	a.decode(t, name, &n)
	return n
}

func (a args) strs(t *testing.T, name string) []string {
	t.Helper()
	var s []string
	a.decode(t, name, &s)
	return s
}

func (a args) boolean(t *testing.T, name string) bool {
	t.Helper()
	var b bool
	if !a.has(name) {
		return false
	}
	a.decode(t, name, &b)
	return b
}

// symbol returns the member name of an enum argument ("SDTV" for
// Quality.SDTV), or "" for null.
func (a args) symbol(t *testing.T, name string) string {
	t.Helper()
	var s *struct {
		Symbol string `json:"$symbol"`
	}
	a.decode(t, name, &s)
	if s == nil {
		return ""
	}
	i := strings.LastIndex(s.Symbol, ".")
	return s.Symbol[i+1:]
}

// expr returns the source text of a non-constant argument such as
// "@\"C:\\Test\\file.mkv\".AsOsAgnostic()".
func (a args) expr(t *testing.T, name string) string {
	t.Helper()
	var e struct {
		Expr string `json:"$expr"`
	}
	a.decode(t, name, &e)
	return e.Expr
}
