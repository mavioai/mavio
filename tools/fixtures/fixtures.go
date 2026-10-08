// Package fixtures generates deterministic test media with ffmpeg and lets
// tests locate it.
//
// Fixtures are written to the git-ignored .fixtures directory at the
// repository root (override with MAVIO_FIXTURES). Tests call Require, which
// skips the test when a fixture has not been generated.
package fixtures

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// EnvDir overrides the fixtures directory.
const EnvDir = "MAVIO_FIXTURES"

// GenerateCommand is the command that generates all fixtures.
const GenerateCommand = "pnpm nx run fixtures:media"

// Dir returns the fixtures directory: $MAVIO_FIXTURES if set, otherwise
// .fixtures in the repository root, found by walking up from the working
// directory to the directory containing go.work.
func Dir() (string, error) {
	if dir := os.Getenv(EnvDir); dir != "" {
		return filepath.Abs(dir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return filepath.Join(dir, ".fixtures"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.work found above %s; set %s", wd, EnvDir)
		}
		dir = parent
	}
}

// Require returns the path of the named fixture, skipping the test when the
// fixture has not been generated.
func Require(tb testing.TB, name string) string {
	tb.Helper()
	dir, err := Dir()
	if err != nil {
		tb.Skipf("fixtures directory unavailable: %v", err)
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tb.Skipf("fixture %s not generated; run %q", name, GenerateCommand)
		}
		tb.Fatalf("stat fixture %s: %v", name, err)
	}
	return path
}
