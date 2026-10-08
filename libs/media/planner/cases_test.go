package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test cases ported from Jellyfin by tools/testport live in
// testdata/cases, constants they name in testdata/constants;
// see docs/testing.md §2.

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

func (a args) str(t *testing.T, name string) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(a[name], &s); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
	return s
}

type ported struct {
	run   map[string]func(t *testing.T, a args)
	skip  map[string]string
	facts map[string]string // fact → Go test porting it
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
		reason, skipped := p.skip[th.Method]
		if !ok && !skipped {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, c := range th.Cases {
			t.Run(c.ID, func(t *testing.T) {
				switch {
				case c.Skip != "":
					t.Skip("skipped in Jellyfin: " + c.Skip)
				case skipped:
					t.Skip(reason)
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

func (a args) null(name string) bool {
	v, ok := a[name]
	return !ok || string(v) == "null"
}

func (a args) boolean(t *testing.T, name string) bool {
	t.Helper()
	var b bool
	if err := json.Unmarshal(a[name], &b); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
	return b
}

func (a args) integer(t *testing.T, name string) int {
	t.Helper()
	var n int
	if err := json.Unmarshal(a[name], &n); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
	return n
}

// symbol returns the member of an enum argument ("DirectPlay" for
// {"$symbol": "PlayMethod.DirectPlay"}), or "" for null.
func (a args) symbol(t *testing.T, name string) string {
	t.Helper()
	if a.null(name) {
		return ""
	}
	var sym struct {
		Symbol string `json:"$symbol"`
	}
	if err := json.Unmarshal(a[name], &sym); err != nil || sym.Symbol == "" {
		t.Fatalf("argument %s: not a symbol: %s", name, a[name])
	}
	_, member, _ := strings.Cut(sym.Symbol, ".")
	return member
}

// constant resolves an argument naming a constant of the test class,
// against the constants testport extracted into testdata/constants.
func (a args) constant(t *testing.T, name, file, class string) json.RawMessage {
	t.Helper()
	var sym struct {
		Symbol string `json:"$symbol"`
	}
	if err := json.Unmarshal(a[name], &sym); err != nil || sym.Symbol == "" {
		return a[name]
	}
	data, err := os.ReadFile(filepath.Join("testdata", "constants", file))
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Constants map[string]json.RawMessage }
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	key := sym.Symbol
	if !strings.Contains(key, ".") {
		key = class + "." + key
	}
	v, ok := f.Constants[key]
	if !ok {
		t.Fatalf("constant %s not found", key)
	}
	return v
}
