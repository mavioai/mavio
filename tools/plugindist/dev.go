package plugindist

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// DevBuild is the folder of a plugin's project that Dev builds the plugin
// into, which a server run with --plugin-dev runs in place.
const DevBuild = "dist/dev"

// Dev builds a plugin for this platform into its project's development
// folder, which it replaces whole once the build is done: a Python plugin
// runs from the uv workspace, and a file there with the build's time makes
// a change of its sources one of the folder's. A failed build leaves the
// folder as it was.
func Dev(ctx context.Context, dir string) error {
	s, err := load(dir)
	if err != nil {
		return err
	}
	out := filepath.Join(dir, filepath.FromSlash(DevBuild))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(out), ".dev-"+rand.Text())
	defer os.RemoveAll(tmp)
	if err := build(ctx, s, runtime.GOOS, runtime.GOARCH, tmp, true); err != nil {
		return err
	}
	if err := copyManifest(dir, tmp); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".built"), []byte(time.Now().UTC().Format(time.RFC3339Nano)+"\n"), 0o644); err != nil {
		return err
	}
	old := filepath.Join(filepath.Dir(out), ".dev-old-"+rand.Text())
	if err := os.Rename(out, old); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer os.RemoveAll(old)
	return os.Rename(tmp, out)
}

// Watch builds a plugin into its development folder, then again whenever
// its sources change, until ctx ends; poll is how often it looks. The
// sources are the project's, and those of the in-repo modules and packages
// it depends on: a Go module's replaced modules, a Bun plugin's workspace
// packages and the uv workspace's members. A failed build is reported to
// log, and the next change builds again.
func Watch(ctx context.Context, dir string, poll time.Duration, log io.Writer) error {
	watched, err := watchedDirs(dir)
	if err != nil {
		return err
	}
	report := func() {
		if err := Dev(ctx, dir); err != nil {
			if ctx.Err() == nil {
				fmt.Fprintln(log, "plugindist: build failed:", err)
			}
			return
		}
		fmt.Fprintln(log, "plugindist: built", filepath.Join(dir, filepath.FromSlash(DevBuild)))
	}
	last := sourcesPrint(watched)
	report()
	t := time.NewTicker(poll)
	defer t.Stop()
	pending := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		f := sourcesPrint(watched)
		if f != last {
			last, pending = f, true
			continue
		}
		if pending {
			pending = false
			report()
		}
	}
}

// watchedDirs are the folders of a plugin's sources and of what it depends
// on in the repository.
func watchedDirs(dir string) ([]string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dirs := []string{abs}
	if data, err := os.ReadFile(filepath.Join(abs, "go.mod")); err == nil {
		f, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return nil, err
		}
		for _, r := range f.Replace {
			if modfile.IsDirectoryPath(r.New.Path) {
				dirs = append(dirs, filepath.Join(abs, filepath.FromSlash(r.New.Path)))
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(abs, "package.json")); err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return nil, fmt.Errorf("package.json: %w", err)
		}
		deps := maps.Clone(pkg.Dependencies)
		maps.Copy(deps, pkg.DevDependencies)
		for name, v := range deps {
			if !strings.HasPrefix(v, "workspace:") {
				continue
			}
			// pnpm links the workspace's packages into node_modules.
			if p, err := filepath.EvalSymlinks(filepath.Join(abs, "node_modules", filepath.FromSlash(name))); err == nil {
				dirs = append(dirs, p)
			}
		}
	}
	if exists(filepath.Join(abs, "pyproject.toml")) {
		root, err := workspaceRoot(abs)
		if err != nil {
			return nil, err
		}
		members, err := workspaceMembers(root)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, members...)
	}
	slices.Sort(dirs)
	return slices.Compact(dirs), nil
}

// membersList matches the members of a uv workspace's root pyproject.toml.
var membersList = regexp.MustCompile(`(?m)^members\s*=\s*\[([^\]]*)\]`)

// workspaceMembers returns the folders of the uv workspace's members.
func workspaceMembers(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return nil, err
	}
	m := membersList.FindSubmatch(data)
	if m == nil {
		return nil, nil
	}
	var dirs []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(string(q[1]))))
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, matches...)
	}
	return dirs, nil
}

// sourceExts are the extensions of the files whose changes rebuild a
// plugin.
var sourceExts = []string{".go", ".mod", ".sum", ".ts", ".tsx", ".js", ".mjs", ".json", ".py", ".pyi", ".toml", ".lock"}

// skippedDirs hold what is built, installed or cached, never sources.
var skippedDirs = []string{"dist", "node_modules", "__pycache__", "testdata", "target", "build"}

// sourcesPrint sums the names, sizes and modification times of the
// sources in dirs.
func sourcesPrint(dirs []string) string {
	h := sha256.New()
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != dir && (strings.HasPrefix(d.Name(), ".") || slices.Contains(skippedDirs, d.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if !slices.Contains(sourceExts, filepath.Ext(p)) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano())
			return nil
		})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
