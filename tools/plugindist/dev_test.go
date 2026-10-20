package plugindist

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command to build plugins with")
	}
	dir := t.TempDir()
	plugin(t, dir, "wasm", "RUNTIME_WASM", "CATEGORY_METADATA")
	project := filepath.Join(dir, "wasm")
	out := filepath.Join(project, filepath.FromSlash(DevBuild))
	ctx, cancel := context.WithCancel(t.Context())
	var log syncBuffer
	done := make(chan error, 1)
	go func() { done <- Watch(ctx, project, 20*time.Millisecond, &log) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Watch = %v", err)
		}
	})

	built := func() string {
		data, _ := os.ReadFile(filepath.Join(out, ".built"))
		return string(data)
	}
	waitFor(t, func() bool { return built() != "" })
	for _, f := range []string{"manifest.json", "plugin.wasm", "locales/zh-Hans.json"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("development folder: %v", err)
		}
	}

	// A broken source keeps the folder and reports the failure; fixing it
	// builds again.
	first := built()
	main := filepath.Join(project, "main.go")
	if err := os.WriteFile(main, []byte("package main\n\nfunc main() {"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return strings.Contains(log.String(), "build failed") })
	if built() != first {
		t.Error("a failed build replaced the development folder")
	}
	if err := os.WriteFile(main, []byte("package main\n\nvar Version = \"\"\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return built() != first })
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(60 * time.Second); !done(); {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// syncBuffer is a buffer written and read concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
