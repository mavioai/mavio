package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test cases ported from Jellyfin by tools/testport live in
// testdata/cases; see docs/testing.md §2.

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
	// Unsupported theories have data testport cannot extract; they are
	// ported by hand like facts.
	Unsupported []struct {
		Method string `json:"method"`
	} `json:"unsupported"`
}

type args map[string]json.RawMessage

func (a args) decode(t *testing.T, name string, v any) {
	t.Helper()
	if err := json.Unmarshal(a[name], v); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
}

// str returns a string argument; null is "".
func (a args) str(t *testing.T, name string) string {
	t.Helper()
	var s *string
	a.decode(t, name, &s)
	if s == nil {
		return ""
	}
	return *s
}

func (a args) boolean(t *testing.T, name string) bool {
	t.Helper()
	var b bool
	a.decode(t, name, &b)
	return b
}

func (a args) integer(t *testing.T, name string) int {
	t.Helper()
	var i int
	a.decode(t, name, &i)
	return i
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
	for _, fact := range append(f.Facts, f.Unsupported...) {
		if p.facts[fact.Method] == "" && p.skip[fact.Method] == "" {
			t.Errorf("%s: fact %s is neither ported nor skipped", name, fact.Method)
		}
	}
}
