package plugins_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
)

func writePlugin(t *testing.T, dir, id, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := `{"id":"` + id + `","name":"Seeded","version":"` + version + `","runtime":"RUNTIME_PROCESS","apiVersion":"1.0","capabilities":["CAPABILITY_HTTP_HANDLER"]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(m), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestSeed installs the plugins of a seed folder once: a plugin
// uninstalled stays so, and one installed already is not copied.
func TestSeed(t *testing.T) {
	ctx, log := t.Context(), slog.New(slog.DiscardHandler)
	seed, dir := t.TempDir(), t.TempDir()
	writePlugin(t, filepath.Join(seed, "dlna"), "org.example.dlna", "0.1.0")
	writePlugin(t, filepath.Join(seed, "other"), "org.example.other", "0.1.0")
	writePlugin(t, filepath.Join(dir, "mine"), "org.example.other", "0.2.0")
	if err := os.MkdirAll(filepath.Join(seed, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := plugins.Seed(ctx, seed, dir, log); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "dlna", "plugin"))
	if err != nil || info.Mode()&0o100 == 0 {
		t.Errorf("seeded executable = %v, %v; want an executable file", info, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other")); !os.IsNotExist(err) {
		t.Errorf("a plugin installed already was seeded again: %v", err)
	}
	for _, id := range []string{"org.example.dlna", "org.example.other"} {
		if _, err := os.Stat(filepath.Join(dir, ".seeded-"+id)); err != nil {
			t.Errorf("marker of %s: %v", id, err)
		}
	}

	// Uninstalled, the plugin is not seeded again.
	if err := os.RemoveAll(filepath.Join(dir, "dlna")); err != nil {
		t.Fatal(err)
	}
	if err := plugins.Seed(ctx, seed, dir, log); err != nil {
		t.Fatalf("Seed again: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dlna")); !os.IsNotExist(err) {
		t.Errorf("an uninstalled plugin was seeded again: %v", err)
	}

	// A plugin folder taken by another plugin leaves the seeded one a free
	// name.
	writePlugin(t, filepath.Join(seed, "taken"), "org.example.new", "1.0.0")
	writePlugin(t, filepath.Join(dir, "taken"), "org.example.mine", "1.0.0")
	if err := plugins.Seed(ctx, seed, dir, log); err != nil {
		t.Fatalf("Seed with a taken name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "taken-2", "manifest.json")); err != nil {
		t.Errorf("seeded plugin under a free name: %v", err)
	}
}
