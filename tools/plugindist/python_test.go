package plugindist

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLauncherRunsTheConsoleScript builds the Windows launcher for this
// platform and runs it on a fake Python: the console script's code, the
// arguments, the standard streams and the exit code go through.
func TestLauncherRunsTheConsoleScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Python is a shell script")
	}
	folder := t.TempDir()
	exe := filepath.Join(folder, "plugin.exe")
	if err := buildLauncher(t.Context(), t.TempDir(), runtime.GOOS, runtime.GOARCH, "mavio_pipecat.plugin", "main", exe); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\necho \"$@\"\ncat\nexit 3\n"
	if err := os.MkdirAll(filepath.Join(folder, "python"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "python", "python.exe"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "manifest.json")
	cmd.Stdin = strings.NewReader("lifeline\n")
	out, err := cmd.Output()
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != 3 {
		t.Fatalf("launcher = %v, want exit code 3", err)
	}
	want := "-I -c import sys; from mavio_pipecat.plugin import main; sys.exit(main()) manifest.json\nlifeline\n"
	if string(out) != want {
		t.Errorf("output = %q, want = %q", out, want)
	}
}
