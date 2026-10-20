package plugindist

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/plugin/manifest"
)

// firstPartyLocales are the languages every first-party plugin is
// translated into, as the clients' strings are.
var firstPartyLocales = []string{"zh-Hans"}

// TestFirstPartyPlugins checks that every plugin of plugins/ has a manifest
// the catalog can be made of, titles for the properties of its
// configuration, and translations of all its strings.
func TestFirstPartyPlugins(t *testing.T) {
	srcs, err := sources(filepath.Join("..", "..", "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range srcs {
		if !strings.HasPrefix(s.manifest.GetId(), "dev.mavio.") {
			t.Errorf("%s: id = %s, want dev.mavio.*", s.dir, s.manifest.GetId())
		}
		if schema := s.manifest.GetConfigSchema(); schema != "" {
			var v any
			if err := json.Unmarshal([]byte(schema), &v); err != nil {
				t.Fatalf("%s: config schema: %v", s.dir, err)
			}
			for _, p := range untitled(v, "") {
				t.Errorf("%s: config property %s has no title", s.dir, p)
			}
		}
		for _, tag := range firstPartyLocales {
			strs, ok := s.locales[tag]
			if !ok {
				t.Errorf("%s: no locales/%s.json", s.dir, tag)
				continue
			}
			for _, str := range manifest.Strings(s.manifest) {
				if _, ok := strs[str]; !ok {
					t.Errorf("%s: locales/%s.json does not translate %q", s.dir, tag, str)
				}
			}
		}
	}
}

// untitled returns the paths of the properties of a schema without a
// title.
func untitled(schema any, at string) []string {
	var out []string
	switch s := schema.(type) {
	case map[string]any:
		for k, v := range s {
			if k == "properties" {
				props, _ := v.(map[string]any)
				for name, p := range props {
					if ps, ok := p.(map[string]any); ok && ps["title"] == nil {
						out = append(out, at+"/"+name)
					}
					out = append(out, untitled(p, at+"/"+name)...)
				}
				continue
			}
			if k != "default" && k != "const" && k != "enum" && k != "examples" {
				out = append(out, untitled(v, at)...)
			}
		}
	case []any:
		for _, v := range s {
			out = append(out, untitled(v, at)...)
		}
	}
	return out
}

// plugin writes a Go plugin with an empty main and its manifest into
// dir/name.
func plugin(t *testing.T, dir, name, runtime, category string) {
	t.Helper()
	p := filepath.Join(dir, name)
	files := map[string]string{
		"go.mod":  "module example.com/" + name + "\n\ngo 1.27.1\n",
		"main.go": "package main\n\nvar Version = \"\"\n\nfunc main() {}\n",
		"manifest.json": `{"id": "org.example.` + name + `", "name": "` + name + `", "version": "1.2.0", "description": "A test plugin.",
			"author": "Tests", "homepage": "https://example.com", "runtime": "` + runtime + `", "category": "` + category + `",
			"capabilities": ["CAPABILITY_METADATA_PROVIDER"], "apiVersion": "1.0"}`,
	}
	if runtime == "RUNTIME_WASM" {
		files["locales/zh-Hans.json"] = `{"A test plugin.": "一个测试插件。"}`
	}
	if err := os.MkdirAll(filepath.Join(p, "locales"), 0o755); err != nil {
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
			if got := c.Plugins[1].Locales; len(got) != 1 || got["zh-Hans"] != (Strings{Description: "一个测试插件。"}) {
				t.Errorf("wasm locales = %+v, want its description in Chinese", got)
			}
			if got := c.Plugins[0].Locales; got != nil {
				t.Errorf("proc locales = %+v, want none", got)
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
					check(t, path, v, p.Locales != nil)
				}
			}
		})
	}
}

// check checks the package of a version, holding translations or not.
func check(t *testing.T, path string, v Version, translated bool) {
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
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	want := []string{"manifest.json", exe}
	if translated {
		want = []string{"manifest.json", "locales/zh-Hans.json", exe}
	}
	if !slices.Equal(names, want) || zr.File[len(zr.File)-1].Mode()&0o111 == 0 {
		t.Errorf("%s holds %v, want %v, the executable executable", path, names, want)
	}
}

func TestDistWithoutCategory(t *testing.T) {
	plugins := t.TempDir()
	plugin(t, plugins, "none", "RUNTIME_WASM", "CATEGORY_UNSPECIFIED")
	if _, err := Dist(t.Context(), Options{Plugins: plugins}); err == nil || !strings.Contains(err.Error(), "no category") {
		t.Errorf("Dist of a plugin without a category: %v, want an error", err)
	}
}

func TestZipFolderHoldsWhatWasBuilt(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	src, folder := filepath.Join(dir, "src"), filepath.Join(dir, "folder")
	files := map[string]string{
		filepath.Join(src, "manifest.json"):              "{}",
		filepath.Join(folder, "manifest.json"):           "copied",
		filepath.Join(folder, "plugin"):                  "#!/bin/sh",
		filepath.Join(folder, "python/bin/python3.14"):   "elf",
		filepath.Join(folder, "python/lib/site/a.py"):    "a",
		filepath.Join(folder, "locales", "zh-Hans.json"): "{}",
	}
	for f, data := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.Contains(f, "plugin") || strings.Contains(f, "bin") {
			mode = 0o755
		}
		if err := os.WriteFile(f, []byte(data), mode); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := zipFolder(&buf, src, folder, time.Now()); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range zr.File {
		got = append(got, fmt.Sprintf("%s %v", f.Name, f.Mode()&0o111 != 0))
	}
	want := []string{"manifest.json false", "plugin true", "python/bin/python3.14 true", "python/lib/site/a.py false"}
	if !slices.Equal(got, want) {
		t.Errorf("files = %q, want = %q", got, want)
	}

	if err := os.Symlink("python3.14", filepath.Join(folder, "python/bin/python3")); err != nil {
		t.Skip("no symbolic links:", err)
	}
	if err := zipFolder(io.Discard, src, folder, time.Now()); err == nil {
		t.Error("zipFolder of a symbolic link succeeded, want an error")
	}
	if err := removeLinks(folder); err != nil {
		t.Fatal(err)
	}
	if err := zipFolder(io.Discard, src, folder, time.Now()); err != nil {
		t.Errorf("zipFolder after removeLinks = %v", err)
	}
}
