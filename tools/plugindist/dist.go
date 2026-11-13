package plugindist

import (
	"archive/zip"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Platforms are the os/arch the server is built for.
var Platforms = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

// Options are what Dist packages, and where.
type Options struct {
	// Plugins is the folder of the plugins, one per subfolder holding a
	// manifest.json.
	Plugins string
	// Out is the folder the packages and the catalog go into; empty puts
	// each plugin's packages into its dist folder and the catalog into
	// Plugins.
	Out string
	// Platforms are the os/arch process plugins are built for.
	Platforms []string
	// Now is the versions' release time.
	Now time.Time
}

// Catalog is a catalog of plugins, as the server reads it.
type Catalog struct {
	Plugins []Plugin `json:"plugins"`
}

// Plugin is a plugin a catalog offers.
type Plugin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Homepage    string `json:"homepage"`
	Category    string `json:"category"`
	// Locales are the name and description in other languages, by BCP 47
	// tag; either is left out when not translated.
	Locales  map[string]Strings `json:"locales,omitempty"`
	Versions []Version          `json:"versions"`
}

// Strings are a plugin's name and description in a language.
type Strings struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Version is a package of a plugin's version.
type Version struct {
	Version     string    `json:"version"`
	APIVersion  string    `json:"api_version"`
	Runtime     string    `json:"runtime"`
	OS          string    `json:"os,omitempty"`
	Arch        string    `json:"arch,omitempty"`
	URL         string    `json:"url"`
	SHA256      string    `json:"sha256"`
	ReleaseTime time.Time `json:"release_time"`
}

// source is a plugin's folder, manifest and translations.
type source struct {
	dir      string
	manifest *pluginv1.Manifest
	locales  manifest.Locales
}

// sources reads the manifests of the plugins in dir, ordered by id.
func sources(dir string) ([]source, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", manifest.File))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no plugins in %s", dir)
	}
	var srcs []source
	for _, p := range paths {
		s, err := load(filepath.Dir(p))
		if err != nil {
			return nil, err
		}
		srcs = append(srcs, s)
	}
	slices.SortFunc(srcs, func(a, b source) int { return cmp.Compare(a.manifest.GetId(), b.manifest.GetId()) })
	return srcs, nil
}

// load reads a plugin's manifest and translations.
func load(dir string) (source, error) {
	m, err := manifest.Load(dir)
	if err != nil {
		return source{}, fmt.Errorf("%s: %w", dir, err)
	}
	if m.GetCategory() == pluginv1.Category_CATEGORY_UNSPECIFIED {
		return source{}, fmt.Errorf("%s: no category", dir)
	}
	locales, err := manifest.LoadLocales(dir)
	if err != nil {
		return source{}, fmt.Errorf("%s: %w", dir, err)
	}
	return source{dir, m, locales}, nil
}

// Dist packages the plugins and writes their catalog, returning its path.
func Dist(ctx context.Context, o Options) (string, error) {
	srcs, err := sources(o.Plugins)
	if err != nil {
		return "", err
	}
	type job struct {
		plugin   int
		os, arch string
	}
	var jobs []job
	for i, s := range srcs {
		if o.Out == "" {
			// Packages of earlier versions go.
			old, _ := filepath.Glob(filepath.Join(s.dir, "dist", filepath.Base(s.dir)+"-*.zip"))
			for _, f := range old {
				if err := os.Remove(f); err != nil {
					return "", err
				}
			}
		}
		if s.manifest.GetRuntime() == pluginv1.Runtime_RUNTIME_WASM {
			jobs = append(jobs, job{plugin: i})
			continue
		}
		if python(s) {
			// A development catalog runs Python plugins from the uv workspace,
			// on this platform; a release packages them for the platforms
			// their wheels cover.
			for _, p := range o.Platforms {
				goos, arch, _ := strings.Cut(p, "/")
				_, packaged := pythonTargets[p]
				if o.Out == "" && goos != "windows" && p == runtime.GOOS+"/"+runtime.GOARCH || o.Out != "" && packaged {
					jobs = append(jobs, job{i, goos, arch})
				}
			}
			continue
		}
		for _, p := range o.Platforms {
			goos, arch, ok := strings.Cut(p, "/")
			if !ok || !slices.Contains(Platforms, p) {
				return "", fmt.Errorf("platform %q is not one of %s", p, strings.Join(Platforms, ", "))
			}
			jobs = append(jobs, job{i, goos, arch})
		}
	}
	if o.Out != "" {
		if err := os.MkdirAll(o.Out, 0o755); err != nil {
			return "", err
		}
	}

	versions := make([]Version, len(jobs))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())
	for i, j := range jobs {
		g.Go(func() error {
			v, err := pack(ctx, o, srcs[j.plugin], j.os, j.arch)
			if err != nil {
				return fmt.Errorf("%s %s/%s: %w", srcs[j.plugin].manifest.GetId(), j.os, j.arch, err)
			}
			versions[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return "", err
	}

	catalog := Catalog{Plugins: make([]Plugin, len(srcs))}
	for i, s := range srcs {
		m := s.manifest
		catalog.Plugins[i] = Plugin{
			ID: m.GetId(), Name: m.GetName(), Description: m.GetDescription(), Author: m.GetAuthor(),
			Homepage: m.GetHomepage(), Category: m.GetCategory().String(), Versions: []Version{},
		}
		for tag, strs := range s.locales {
			t := Strings{Name: strs[m.GetName()], Description: strs[m.GetDescription()]}
			if t != (Strings{}) {
				if catalog.Plugins[i].Locales == nil {
					catalog.Plugins[i].Locales = map[string]Strings{}
				}
				catalog.Plugins[i].Locales[tag] = t
			}
		}
	}
	for i, j := range jobs {
		p := &catalog.Plugins[j.plugin]
		p.Versions = append(p.Versions, versions[i])
	}
	// Plugins packaged for none of the platforms are not offered.
	catalog.Plugins = slices.DeleteFunc(catalog.Plugins, func(p Plugin) bool { return len(p.Versions) == 0 })
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return "", err
	}
	file := filepath.Join(cmp.Or(o.Out, o.Plugins), "catalog.json")
	return file, os.WriteFile(file, append(data, '\n'), 0o644)
}

// pack builds a plugin, for goos and arch unless it is a WASM plugin, and
// zips its folder.
func pack(ctx context.Context, o Options, s source, goos, arch string) (Version, error) {
	m := s.manifest
	tmp, err := os.MkdirTemp("", "plugindist-")
	if err != nil {
		return Version{}, err
	}
	defer os.RemoveAll(tmp)
	folder := filepath.Join(tmp, "plugin")
	if err := build(ctx, s, goos, arch, folder, o.Out == ""); err != nil {
		return Version{}, err
	}

	name := filepath.Base(s.dir) + "-" + m.GetVersion()
	if goos != "" {
		name += "-" + goos + "-" + arch
	}
	name += ".zip"
	dir := cmp.Or(o.Out, filepath.Join(s.dir, "dist"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Version{}, err
	}
	file := filepath.Join(dir, name)
	f, err := os.Create(file)
	if err != nil {
		return Version{}, err
	}
	sum := sha256.New()
	err = zipFolder(io.MultiWriter(f, sum), s.dir, folder, o.Now)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Version{}, err
	}
	v := Version{
		Version: m.GetVersion(), APIVersion: m.GetApiVersion(), Runtime: m.GetRuntime().String(),
		OS: goos, Arch: arch, URL: name, SHA256: hex.EncodeToString(sum.Sum(nil)), ReleaseTime: o.Now,
	}
	if o.Out == "" {
		abs, err := filepath.Abs(file)
		if err != nil {
			return Version{}, err
		}
		v.URL = fileURL(abs)
	}
	return v, nil
}

// python reports whether a plugin is a Python process plugin, a member of
// the uv workspace.
func python(s source) bool {
	return s.manifest.GetRuntime() == pluginv1.Runtime_RUNTIME_PROCESS && exists(filepath.Join(s.dir, "pyproject.toml"))
}

// projectName matches the name of a pyproject.toml's project.
var projectName = regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)

// devLauncher writes a launcher running a Python plugin from the uv
// workspace's environment, by the script its project names after itself:
// a development server's, which runs only on this machine.
func devLauncher(dir, exe string) error {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return err
	}
	m := projectName.FindSubmatch(data)
	if m == nil {
		return errors.New("pyproject.toml names no project")
	}
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	script := filepath.Join(root, ".venv", "bin", string(m[1]))
	if !exists(script) {
		return fmt.Errorf("%s is missing; run uv sync --all-packages", script)
	}
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		return err
	}
	launcher := "#!/bin/sh\n# Runs the plugin from the uv workspace, for a development server.\nexec '" + script + "' \"$@\"\n"
	return os.WriteFile(exe, []byte(launcher), 0o755)
}

// executable reports whether the file name of a package is executable:
// the plugin's executable, the files of a bin directory (the interpreter
// of python-build-standalone), and what the host marks executable. Windows
// marks nothing, so the first two keep the executables of packages for
// Unix built there executable.
func executable(name string, mode fs.FileMode) bool {
	return mode&0o111 != 0 || slices.Contains([]string{"plugin", "plugin.exe", "plugin.wasm"}, name) ||
		path.Base(path.Dir(name)) == "bin"
}

// zipFolder zips the plugin's manifest and translations, and what was
// built into folder: the plugin's executable, and the files beside it,
// whose executables stay executable.
func zipFolder(w io.Writer, src, folder string, modified time.Time) error {
	type file struct {
		path, name string
		mode       fs.FileMode
	}
	files := []file{{filepath.Join(src, manifest.File), manifest.File, 0o644}}
	locales, err := filepath.Glob(filepath.Join(src, manifest.LocalesDir, "*.json"))
	if err != nil {
		return err
	}
	slices.Sort(locales)
	for _, l := range locales {
		files = append(files, file{l, manifest.LocalesDir + "/" + filepath.Base(l), 0o644})
	}
	// WalkDir walks in lexical order, so packages are reproducible.
	err = filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name, err := filepath.Rel(folder, p)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if name == manifest.File || strings.HasPrefix(name, manifest.LocalesDir+"/") {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", name)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if executable(name, info.Mode()) {
			mode = 0o755
		}
		files = append(files, file{p, name, mode})
		return nil
	})
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: modified}
		h.SetMode(f.mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		r, err := os.Open(f.path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

// run runs a command in dir with env added to this process's.
func run(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fileURL is the file URL of an absolute path, such as file:///C:/mavio on
// Windows.
func fileURL(abs string) string {
	return (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(abs), "/")}).String()
}

// build builds a plugin into folder, for goos and arch unless it is a WASM
// plugin; dev runs a Python plugin from the uv workspace instead of
// packaging it.
func build(ctx context.Context, s source, goos, arch, folder string, dev bool) error {
	m := s.manifest
	exe := "plugin"
	if goos == "windows" {
		exe = "plugin.exe"
	}
	switch {
	case m.GetRuntime() == pluginv1.Runtime_RUNTIME_WASM:
		exe = "plugin.wasm"
		return run(ctx, s.dir, []string{"GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0"},
			"go", "build", "-trimpath", "-ldflags=-s -w", "-buildmode=c-shared", "-o", filepath.Join(folder, exe), ".")
	case exists(filepath.Join(s.dir, "go.mod")):
		return run(ctx, s.dir, []string{"GOOS=" + goos, "GOARCH=" + arch, "CGO_ENABLED=0"},
			"go", "build", "-trimpath", "-ldflags=-s -w -X main.Version="+m.GetVersion(), "-o", filepath.Join(folder, exe), ".")
	case exists(filepath.Join(s.dir, "package.json")):
		// A TypeScript plugin compiled by Bun into the folder.
		return run(ctx, s.dir, nil, "bun", "scripts/build.ts", goos+"/"+arch, folder)
	case python(s) && dev:
		if goos == "windows" {
			return errors.New("python plugins run from the uv workspace only on macOS and Linux")
		}
		return devLauncher(s.dir, filepath.Join(folder, exe))
	case python(s):
		return BuildPython(ctx, s.dir, goos, arch, folder)
	default:
		return errors.New("neither a WASM, Go, Bun nor Python plugin")
	}
}
