package plugindist

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Homepage    string    `json:"homepage"`
	Category    string    `json:"category"`
	Versions    []Version `json:"versions"`
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

// source is a plugin's folder and manifest.
type source struct {
	dir      string
	manifest *pluginv1.Manifest
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
		m, err := manifest.Load(filepath.Dir(p))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if m.GetCategory() == pluginv1.Category_CATEGORY_UNSPECIFIED {
			return nil, fmt.Errorf("%s: no category", p)
		}
		srcs = append(srcs, source{filepath.Dir(p), m})
	}
	slices.SortFunc(srcs, func(a, b source) int { return cmp.Compare(a.manifest.GetId(), b.manifest.GetId()) })
	return srcs, nil
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
	}
	for i, j := range jobs {
		p := &catalog.Plugins[j.plugin]
		p.Versions = append(p.Versions, versions[i])
	}
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
	exe := "plugin"
	if goos == "windows" {
		exe = "plugin.exe"
	}
	switch {
	case m.GetRuntime() == pluginv1.Runtime_RUNTIME_WASM:
		exe = "plugin.wasm"
		err = run(ctx, s.dir, []string{"GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0"},
			"go", "build", "-trimpath", "-ldflags=-s -w", "-buildmode=c-shared", "-o", filepath.Join(folder, exe), ".")
	case exists(filepath.Join(s.dir, "go.mod")):
		err = run(ctx, s.dir, []string{"GOOS=" + goos, "GOARCH=" + arch, "CGO_ENABLED=0"},
			"go", "build", "-trimpath", "-ldflags=-s -w -X main.Version="+m.GetVersion(), "-o", filepath.Join(folder, exe), ".")
	case exists(filepath.Join(s.dir, "package.json")):
		// A TypeScript plugin compiled by Bun into the folder.
		err = run(ctx, s.dir, nil, "bun", "scripts/build.ts", goos+"/"+arch, folder)
	default:
		err = errors.New("neither a WASM, Go nor Bun plugin")
	}
	if err != nil {
		return Version{}, err
	}

	name := filepath.Base(s.dir) + "-" + m.GetVersion()
	if goos != "" {
		name += "-" + goos + "-" + arch
	}
	name += ".zip"
	dir := cmp.Or(o.Out, filepath.Join(s.dir, "dist"))
	data, err := zipFolder(s.dir, folder, exe, o.Now)
	if err != nil {
		return Version{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Version{}, err
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return Version{}, err
	}
	sum := sha256.Sum256(data)
	v := Version{
		Version: m.GetVersion(), APIVersion: m.GetApiVersion(), Runtime: m.GetRuntime().String(),
		OS: goos, Arch: arch, URL: name, SHA256: hex.EncodeToString(sum[:]), ReleaseTime: o.Now,
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

// zipFolder zips the plugin's manifest and the executable exe of folder.
func zipFolder(src, folder, exe string, modified time.Time) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct {
		path, name string
		mode       fs.FileMode
	}{{filepath.Join(src, manifest.File), manifest.File, 0o644}, {filepath.Join(folder, exe), exe, 0o755}} {
		data, err := os.ReadFile(f.path)
		if err != nil {
			return nil, err
		}
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: modified}
		h.SetMode(f.mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
