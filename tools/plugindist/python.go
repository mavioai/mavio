package plugindist

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/plugin/manifest"
)

// pythonTargets are the platforms Python plugins are packaged for, by
// os/arch: the uv Python download of python-build-standalone and the
// platform their wheels are installed for. macOS on Intel and Windows on
// Arm are not: onnxruntime, soxr and others publish no wheels for them.
var pythonTargets = map[string]struct{ python, wheels string }{
	"linux/amd64":   {"linux-x86_64-gnu", "x86_64-unknown-linux-gnu"},
	"linux/arm64":   {"linux-aarch64-gnu", "aarch64-unknown-linux-gnu"},
	"darwin/arm64":  {"macos-aarch64-none", "aarch64-apple-darwin"},
	"windows/amd64": {"windows-x86_64-none", "x86_64-pc-windows-msvc"},
}

// PythonPackaged reports whether Python plugins are packaged for os/arch.
func PythonPackaged(platform string) bool {
	_, ok := pythonTargets[platform]
	return ok
}

// macOSTarget is the oldest macOS whose wheels Python plugins take.
const macOSTarget = "14.0"

// pythonLayout is where python-build-standalone keeps its interpreter (the
// Windows launcher's is python.exe) and
// site-packages on an operating system, and the parts of it no plugin
// runs: the scripts, headers and build files, Tcl/Tk, the standard
// library's tests and tools, and pip. Its symbolic links go too, as
// packages hold none.
type pythonLayout struct {
	exe, site string
	unused    []string
}

var (
	unixPython = pythonLayout{
		exe: "bin/python{v}", site: "lib/python{v}/site-packages",
		unused: []string{
			"bin/idle{v}", "bin/pip", "bin/pip3", "bin/pip{v}", "bin/pydoc{v}", "bin/python{v}-config",
			"include", "share", "lib/pkgconfig", "lib/itcl*", "lib/tcl*", "lib/tk*", "lib/thread*", "lib/libtcl*", "lib/libtk*",
			"lib/python{v}/config-*", "lib/python{v}/test", "lib/python{v}/idlelib", "lib/python{v}/tkinter",
			"lib/python{v}/turtledemo", "lib/python{v}/ensurepip", "lib/python{v}/lib2to3",
			"lib/python{v}/site-packages/pip", "lib/python{v}/site-packages/pip-*.dist-info",
		},
	}
	windowsPython = pythonLayout{
		exe: "python.exe", site: "Lib/site-packages",
		unused: []string{
			"BUILD", "Scripts", "include", "libs", "tcl", "pythonw.exe",
			"DLLs/_test*.pyd", "DLLs/_ctypes_test.pyd", "DLLs/_tkinter.pyd", "DLLs/tcl*.dll", "DLLs/libtommath.dll",
			"Lib/test", "Lib/idlelib", "Lib/tkinter", "Lib/turtledemo", "Lib/ensurepip",
			"Lib/site-packages/pip", "Lib/site-packages/pip-*.dist-info",
		},
	}
)

// launcherSource is the plugin.exe of Python plugins on Windows.
//
//go:embed launcher/main.go
var launcherSource []byte

var (
	// projectScript matches the console script a pyproject.toml names
	// after its project: "<name> = "module:function"".
	projectScript = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+)\s*=\s*"([A-Za-z0-9_.]+):([A-Za-z0-9_]+)"`)
	// projectSdists matches the pure-Python packages a pyproject.toml lets
	// install from source distributions: [tool.mavio] sdists = [...].
	projectSdists = regexp.MustCompile(`(?m)^sdists\s*=\s*\[([^\]]*)\]`)
	// exportedPackage matches a requirement of uv export.
	exportedPackage = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+)==`)
	// exportedMember matches a member of the uv workspace uv export lists.
	exportedMember = regexp.MustCompile(`(?m)^-e \./(.+)$`)
)

// workspaceRoot returns the folder of the uv workspace holding dir.
func workspaceRoot(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for !exists(filepath.Join(root, "uv.lock")) {
		parent := filepath.Dir(root)
		if parent == root {
			return "", errors.New("no uv workspace holds the plugin")
		}
		root = parent
	}
	return root, nil
}

// BuildPython builds the folder of a Python process plugin for os/arch:
// python-build-standalone's Python with the plugin's locked dependencies
// in its site-packages, and the executable plugin that runs the console
// script the plugin's project names after itself.
func BuildPython(ctx context.Context, dir, goos, arch, folder string) error {
	target, ok := pythonTargets[goos+"/"+arch]
	if !ok {
		return fmt.Errorf("no Python plugin packages for %s/%s", goos, arch)
	}
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	pyproject, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return err
	}
	name := projectName.FindSubmatch(pyproject)
	if name == nil {
		return errors.New("pyproject.toml names no project")
	}
	var module, function string
	for _, m := range projectScript.FindAllSubmatch(pyproject, -1) {
		if string(m[1]) == string(name[1]) {
			module, function = string(m[2]), string(m[3])
		}
	}
	if module == "" {
		return fmt.Errorf("pyproject.toml has no console script %s", name[1])
	}
	versionData, err := os.ReadFile(filepath.Join(root, ".python-version"))
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(versionData))
	minor := strings.Join(strings.SplitN(version, ".", 3)[:2], ".")

	tmp, err := os.MkdirTemp("", "plugindist-python-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	key := "cpython-" + version + "-" + target.python
	if err := run(ctx, root, nil, "uv", "python", "install", key, "--install-dir", filepath.Join(tmp, "python"), "--no-bin"); err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	python := filepath.Join(folder, "python")
	if err := os.Rename(filepath.Join(tmp, "python", key), python); err != nil {
		return err
	}
	layout := unixPython
	if goos == "windows" {
		layout = windowsPython
	}
	for _, p := range layout.unused {
		matches, err := filepath.Glob(filepath.Join(python, filepath.FromSlash(strings.ReplaceAll(p, "{v}", minor))))
		if err != nil {
			return err
		}
		for _, m := range matches {
			if err := os.RemoveAll(m); err != nil {
				return err
			}
		}
	}
	if err := removeLinks(python); err != nil {
		return err
	}

	// The locked dependencies, and the workspace's members as wheels.
	reqs := filepath.Join(tmp, "requirements.txt")
	if err := run(ctx, root, nil, "uv", "export", "--package", string(name[1]), "--no-dev", "--frozen", "--no-hashes", "-q", "-o", reqs); err != nil {
		return err
	}
	exported, err := os.ReadFile(reqs)
	if err != nil {
		return err
	}
	wheels := filepath.Join(tmp, "wheels")
	for _, m := range exportedMember.FindAllSubmatch(exported, -1) {
		if err := run(ctx, root, nil, "uv", "build", "--wheel", "-q", "-o", wheels, string(m[1])); err != nil {
			return err
		}
	}
	var sdists []string
	if m := projectSdists.FindSubmatch(pyproject); m != nil {
		for s := range strings.SplitSeq(string(m[1]), ",") {
			if s = strings.Trim(strings.TrimSpace(s), `"'`); s != "" {
				sdists = append(sdists, s)
			}
		}
	}
	var binary []string
	for _, m := range exportedPackage.FindAllSubmatch(exported, -1) {
		if !slices.Contains(sdists, string(m[1])) {
			binary = append(binary, string(m[1]))
		}
	}
	// Only the packages, not the members, which are built above.
	lines := slices.DeleteFunc(strings.Split(string(exported), "\n"), func(l string) bool { return strings.HasPrefix(l, "-e ") })
	if err := os.WriteFile(reqs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	built, err := filepath.Glob(filepath.Join(wheels, "*.whl"))
	if err != nil {
		return err
	}
	args := []string{
		"pip", "install", "-q", "--no-deps", "--no-compile", "--target", filepath.Join(python, filepath.FromSlash(strings.ReplaceAll(layout.site, "{v}", minor))),
		"--python-platform", target.wheels, "--python-version", minor, "--only-binary", strings.Join(binary, ","), "-r", reqs,
	}
	if err := run(ctx, root, []string{"MACOSX_DEPLOYMENT_TARGET=" + macOSTarget}, "uv", append(args, built...)...); err != nil {
		return err
	}

	// The launcher runs the console script on the folder's Python, isolated
	// from the environment's.
	if goos == "windows" {
		return buildLauncher(ctx, tmp, "windows", arch, module, function, filepath.Join(folder, "plugin.exe"))
	}
	launcher := fmt.Sprintf(`#!/bin/sh
# Runs the plugin on its Python.
dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$dir/python/%s" -I -c 'import sys; from %s import %s; sys.exit(%s())' "$@"
`, strings.ReplaceAll(layout.exe, "{v}", minor), module, function, function)
	return os.WriteFile(filepath.Join(folder, "plugin"), []byte(launcher), 0o755)
}

// buildLauncher builds the launcher of a console script for os/arch into
// exe, as a module of its own in tmp.
func buildLauncher(ctx context.Context, tmp, goos, arch, module, function, exe string) error {
	dir := filepath.Join(tmp, "launcher")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), launcherSource, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module launcher\n\ngo 1.27.1\n"), 0o644); err != nil {
		return err
	}
	exe, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	return run(ctx, dir, []string{"GOOS=" + goos, "GOARCH=" + arch, "CGO_ENABLED=0", "GOWORK=off"},
		"go", "build", "-trimpath", "-ldflags=-s -w -X main.module="+module+" -X main.function="+function, "-o", exe, ".")
}

// removeLinks removes the symbolic links under dir.
func removeLinks(dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink == 0 {
			return err
		}
		return os.Remove(p)
	})
}

// PythonFolder builds a Python process plugin into folder, which it
// replaces, as a server starts it: with its manifest and translations.
func PythonFolder(ctx context.Context, dir, goos, arch, folder string) error {
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	if err := BuildPython(ctx, dir, goos, arch, folder); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, manifest.LocalesDir, "*.json"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(folder, manifest.LocalesDir), 0o755); err != nil {
		return err
	}
	for _, f := range append(files, filepath.Join(dir, manifest.File)) {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(folder, rel), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
