// Package ported runs the test cases tools/testport ported from Bazarr
// into testdata/cases; see docs/testing.md §2.
package ported

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
	Unsupported []struct {
		Method string `json:"method"`
	} `json:"unsupported"`
}

// Cases is how a test runs a file of cases.
type Cases struct {
	// Run checks the cases of a theory.
	Run map[string]func(t *testing.T, args map[string]json.RawMessage)
	// Skip gives why a theory, fact or unsupported test is not run.
	Skip map[string]string
	// Facts names the Go test porting a fact or an unsupported test.
	Facts map[string]string
}

// Dir returns the plugin's testdata folder.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// Response returns a recorded response of testdata/responses.
func Response(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(), "responses", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Run runs a file of testdata/cases: every theory must be run or skipped,
// and every fact and unsupported test ported or skipped.
func Run(t *testing.T, name string, c Cases) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(), "cases", name))
	if err != nil {
		t.Fatal(err)
	}
	var f caseFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, th := range f.Theories {
		check, ok := c.Run[th.Method]
		reason, skipped := c.Skip[th.Method]
		if !ok && !skipped {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, cs := range th.Cases {
			t.Run(cs.ID, func(t *testing.T) {
				switch {
				case cs.Skip != "":
					t.Skip(cs.Skip)
				case skipped:
					t.Skip(reason)
				}
				check(t, cs.Args)
			})
		}
	}
	for _, m := range append(f.Facts, f.Unsupported...) {
		if c.Facts[m.Method] == "" && c.Skip[m.Method] == "" {
			t.Errorf("%s: %s is neither ported nor skipped", name, m.Method)
		}
	}
}

// Arg decodes an argument of a case.
func Arg[T any](t *testing.T, args map[string]json.RawMessage, name string) T {
	t.Helper()
	var v T
	if raw, ok := args[name]; ok {
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("argument %s: %v", name, err)
		}
	}
	return v
}
