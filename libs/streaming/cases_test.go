package streaming

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
}

type args map[string]json.RawMessage

func (a args) decode(t *testing.T, name string, v any) {
	t.Helper()
	if err := json.Unmarshal(a[name], v); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
}

// ticks reads a .NET tick count: a number, MsToTicks(ms) or
// TimeSpan.FromSeconds(s).Ticks.
func ticks(t *testing.T, raw json.RawMessage) time.Duration {
	t.Helper()
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return time.Duration(n) * 100
	}
	var e struct {
		Expr string `json:"$expr"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("ticks %s: %v", raw, err)
	}
	for _, f := range []struct {
		prefix, suffix string
		unit           float64
	}{{"MsToTicks(", ")", 1e6}, {"TimeSpan.FromSeconds(", ").Ticks", 1e9}} {
		if v, ok := strings.CutPrefix(e.Expr, f.prefix); ok {
			if v, ok = strings.CutSuffix(v, f.suffix); ok {
				if x, err := strconv.ParseFloat(v, 64); err == nil {
					return time.Duration(math.Round(x * f.unit))
				}
			}
		}
	}
	t.Fatalf("ticks: unknown expression %q", e.Expr)
	return 0
}

// seconds reads segment lengths in seconds.
func (a args) seconds(t *testing.T, name string) []time.Duration {
	t.Helper()
	var s []float64
	a.decode(t, name, &s)
	out := make([]time.Duration, len(s))
	for i, v := range s {
		out[i] = time.Duration(math.Round(v * 1e9))
	}
	return out
}

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
