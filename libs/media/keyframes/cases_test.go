package keyframes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test cases ported from Jellyfin by tools/testport live in
// testdata/cases, ffprobe outputs in testdata/ffprobe; see docs/testing.md §2.

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
