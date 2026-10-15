package plugindist

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFirstPartyPlugins checks that every plugin of plugins/ has a manifest
// the catalog can be made of.
func TestFirstPartyPlugins(t *testing.T) {
	srcs, err := sources(filepath.Join("..", "..", "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range srcs {
		if !strings.HasPrefix(s.manifest.GetId(), "dev.mavio.") {
			t.Errorf("%s: id = %s, want dev.mavio.*", s.dir, s.manifest.GetId())
		}
	}
}

// plugin writes a Go plugin with an empty main and its manifest into
// dir/name.
func plugin(t *testing.T, dir, name, runtime, category string) {
	t.Helper()
	p := filepath.Join(dir, name)
	files := map[string]string{
		"go.mod":  "module example.com/" + name + "\n\ngo 1.27.0\n",
		"main.go": "package main\n\nvar Version = \"\"\n\nfunc main() {}\n",
		"manifest.json": `{"id": "org.example.` + name + `", "name": "` + name + `", "version": "1.2.0", "description": "A test plugin.",
			"author": "Tests", "homepage": "https://example.com", "runtime": "` + runtime + `", "category": "` + category + `",
			"capabilities": ["CAPABILITY_METADATA_PROVIDER"], "apiVersion": "1.0"}`,
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, data := range files {
		if err := os.WriteFile(filepath.Join(p, f), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDist(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command to build plugins with")
	}
	host := runtime.GOOS + "/" + runtime.GOARCH
	for _, tt := range []struct {
		name string
		out  bool
	}{{"release", true}, {"development", false}} {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.ToSlash(t.TempDir())
			plugins := filepath.Join(dir, "plugins")
			plugin(t, plugins, "wasm", "RUNTIME_WASM", "CATEGORY_METADATA")
			plugin(t, plugins, "proc", "RUNTIME_PROCESS", "CATEGORY_DEVICES")
			o := Options{Plugins: plugins, Platforms: []string{host, "windows/arm64"}, Now: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}
			if tt.out {
				o.Out = filepath.Join(dir, "out")
			}
			file, err := Dist(t.Context(), o)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var c Catalog
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatal(err)
			}
			if len(c.Plugins) != 2 || c.Plugins[0].ID != "org.example.proc" || c.Plugins[1].ID != "org.example.wasm" {
				t.Fatalf("plugins = %+v, want proc and wasm", c.Plugins)
			}
			if got := c.Plugins[0]; got.Category != "CATEGORY_DEVICES" || got.Description != "A test plugin." || len(got.Versions) != 2 {
				t.Errorf("proc = %+v, want its manifest's details and 2 platforms", got)
			}
			if got := c.Plugins[1].Versions; len(got) != 1 || got[0].OS != "" || got[0].Runtime != "RUNTIME_WASM" {
				t.Errorf("wasm versions = %+v, want one for every platform", got)
			}
			for _, p := range c.Plugins {
				for _, v := range p.Versions {
					path := filepath.Join(o.Out, v.URL)
					if !tt.out {
						if !strings.HasPrefix(v.URL, "file:///") {
							t.Errorf("url = %s, want a file URL", v.URL)
						}
						path = filepath.Join(plugins, strings.TrimPrefix(p.ID, "org.example."), "dist", filepath.Base(v.URL))
					}
					check(t, path, v)
				}
			}
		})
	}
}

// check checks the package of a version.
func check(t *testing.T, path string, v Version) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != v.SHA256 {
		t.Errorf("%s: sha256 = %x, want %s", path, sum, v.SHA256)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	exe := map[string]string{"": "plugin.wasm", "windows": "plugin.exe"}[v.OS]
	if exe == "" {
		exe = "plugin"
	}
	if len(zr.File) != 2 || zr.File[0].Name != "manifest.json" || zr.File[1].Name != exe || zr.File[1].Mode()&0o111 == 0 {
		t.Errorf("%s holds %v, want manifest.json and the executable %s", path, zr.File, exe)
	}
}

func TestDistWithoutCategory(t *testing.T) {
	plugins := t.TempDir()
	plugin(t, plugins, "none", "RUNTIME_WASM", "CATEGORY_UNSPECIFIED")
	if _, err := Dist(t.Context(), Options{Plugins: plugins}); err == nil || !strings.Contains(err.Error(), "no category") {
		t.Errorf("Dist of a plugin without a category: %v, want an error", err)
	}
}
