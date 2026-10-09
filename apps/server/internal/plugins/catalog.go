package plugins

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Limits of downloads.
const (
	maxCatalog = 10 << 20
	maxPackage = 200 << 20
)

// A catalog is a JSON document listing plugins and their versions:
//
//	{"plugins": [{"id": "org.mavio.scraper-tmdb", "name": "TMDB", …,
//	  "versions": [{"version": "0.2.0", "api_version": "1.0",
//	    "runtime": "RUNTIME_WASM", "url": "tmdb-0.2.0.zip",
//	    "sha256": "…", "changelog": "…", "release_time": "…"}]}]}
//
// A version's url, relative to the catalog's, names a zip package holding
// the plugin's folder: its manifest.json and executable. Process plugins
// name the os and arch they are built for.
type catalogFile struct {
	Plugins []CatalogPlugin `json:"plugins"`
}

// CatalogPlugin is a plugin a catalog offers.
type CatalogPlugin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Homepage    string `json:"homepage"`
	// Versions are newest first, once listed by Catalog.
	Versions []CatalogVersion `json:"versions"`
	// CatalogURL is the catalog listing it.
	CatalogURL string `json:"-"`
	// Installed is the version installed, empty when none is.
	Installed string `json:"-"`
}

// CatalogVersion is a version of a catalog plugin.
type CatalogVersion struct {
	Version     string    `json:"version"`
	APIVersion  string    `json:"api_version"`
	Runtime     string    `json:"runtime"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	URL         string    `json:"url"`
	SHA256      string    `json:"sha256"`
	Changelog   string    `json:"changelog"`
	ReleaseTime time.Time `json:"release_time"`
}

// runnable reports whether this server can run a version.
func (v *CatalogVersion) runnable() bool {
	major, _, _ := strings.Cut(v.APIVersion, ".")
	hostMajor, _, _ := strings.Cut(manifest.APIVersion, ".")
	switch {
	case major != hostMajor:
		return false
	case v.Runtime == pluginv1.Runtime_RUNTIME_WASM.String():
		return true
	case v.Runtime == pluginv1.Runtime_RUNTIME_PROCESS.String():
		return v.OS == runtime.GOOS && v.Arch == runtime.GOARCH
	}
	return false
}

// compareVersions orders dotted version numbers, numerically where the
// parts are numbers.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := range max(len(pa), len(pb)) {
		var x, y string
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		if c := cmp.Compare(nx, ny); ex == nil && ey == nil && c != 0 {
			return c
		}
		if c := cmp.Compare(x, y); (ex != nil || ey != nil) && c != 0 {
			return c
		}
	}
	return 0
}

// Catalog lists the plugins of the catalogs, with the versions this server
// can run, newest first. The first catalog listing a plugin wins. A
// catalog that cannot be read is logged and left out, unless none can be.
func (m *Manager) Catalog(ctx context.Context) ([]CatalogPlugin, error) {
	var catalogs []string
	if m.cfg.Catalogs != nil {
		catalogs = m.cfg.Catalogs()
	}
	installed := map[string]string{}
	for _, i := range m.Plugins() {
		installed[i.Manifest.GetId()] = i.Manifest.GetVersion()
	}
	var out []CatalogPlugin
	var errs []error
	for _, c := range catalogs {
		plugins, err := m.readCatalog(ctx, c)
		if err != nil {
			m.log.WarnContext(ctx, "plugin catalog unavailable", "catalog", c, "err", err)
			errs = append(errs, err)
			continue
		}
		for _, p := range plugins {
			if slices.ContainsFunc(out, func(o CatalogPlugin) bool { return o.ID == p.ID }) {
				continue
			}
			p.Versions = slices.DeleteFunc(p.Versions, func(v CatalogVersion) bool { return !v.runnable() })
			if len(p.Versions) == 0 {
				continue
			}
			slices.SortStableFunc(p.Versions, func(a, b CatalogVersion) int { return compareVersions(b.Version, a.Version) })
			p.CatalogURL, p.Installed = c, installed[p.ID]
			out = append(out, p)
		}
	}
	if len(errs) > 0 && len(errs) == len(catalogs) {
		return nil, fmt.Errorf("plugin catalogs: %w", errors.Join(errs...))
	}
	slices.SortFunc(out, func(a, b CatalogPlugin) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (m *Manager) readCatalog(ctx context.Context, catalog string) ([]CatalogPlugin, error) {
	data, err := m.download(ctx, catalog, maxCatalog)
	if err != nil {
		return nil, err
	}
	var f catalogFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", catalog, err)
	}
	return f.Plugins, nil
}

func (m *Manager) download(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s: larger than %d bytes", rawURL, limit)
	}
	return data, nil
}

// Install installs a catalog plugin's version, the newest when version is
// empty, or upgrades or downgrades the installed plugin to it; the plugin
// keeps its configuration. An upgrade that fails to start puts the
// previous version back.
func (m *Manager) Install(ctx context.Context, id, version string) (Info, error) {
	if m.cfg.Dir == "" {
		return Info{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("the server has no plugin folder"))
	}
	m.install.Lock()
	defer m.install.Unlock()
	catalog, err := m.Catalog(ctx)
	if err != nil {
		return Info{}, connect.NewError(connect.CodeUnavailable, err)
	}
	i := slices.IndexFunc(catalog, func(p CatalogPlugin) bool { return p.ID == id })
	if i < 0 {
		return Info{}, fmt.Errorf("plugin %s is in no catalog: %w", id, core.ErrNotFound)
	}
	p := catalog[i]
	v := p.Versions[0]
	if version != "" {
		j := slices.IndexFunc(p.Versions, func(v CatalogVersion) bool { return v.Version == version })
		if j < 0 {
			return Info{}, fmt.Errorf("plugin %s has no version %s this server runs: %w", id, version, core.ErrNotFound)
		}
		v = p.Versions[j]
	}
	pkg, err := m.fetchPackage(ctx, p.CatalogURL, v)
	if err != nil {
		return Info{}, connect.NewError(connect.CodeUnavailable, err)
	}
	staged, err := m.stage(id, v.Version, pkg)
	if err != nil {
		return Info{}, err
	}
	defer os.RemoveAll(staged)
	return m.swap(ctx, id, staged)
}

// fetchPackage downloads a version's package and checks its hash.
func (m *Manager) fetchPackage(ctx context.Context, catalog string, v CatalogVersion) ([]byte, error) {
	base, err := url.Parse(catalog)
	if err != nil {
		return nil, err
	}
	ref, err := url.Parse(v.URL)
	if err != nil {
		return nil, fmt.Errorf("package URL %q: %w", v.URL, err)
	}
	data, err := m.download(ctx, base.ResolveReference(ref).String(), maxPackage)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), v.SHA256) {
		return nil, fmt.Errorf("package of %s: SHA-256 does not match the catalog's", v.Version)
	}
	return data, nil
}

// stage unpacks a package into a new hidden folder of the plugin folder
// and checks that it holds the plugin and version expected.
func (m *Manager) stage(id, version string, pkg []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return "", fmt.Errorf("%w: plugin package: %w", core.ErrInvalid, err)
	}
	dir := filepath.Join(m.cfg.Dir, ".install-"+rand.Text())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := unzip(zr, dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	mf, err := manifest.Load(dir)
	switch {
	case err != nil:
		err = fmt.Errorf("%w: plugin package: %w", core.ErrInvalid, err)
	case mf.GetId() != id || mf.GetVersion() != version:
		err = fmt.Errorf("%w: package holds %s %s, not %s %s", core.ErrInvalid, mf.GetId(), mf.GetVersion(), id, version)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// unzip extracts an archive within dir; executables stay executable.
func unzip(zr *zip.Reader, dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("%w: plugin package names %q outside its folder", core.ErrInvalid, f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return err
		}
		perm := os.FileMode(0o644)
		if f.Mode()&0o111 != 0 {
			perm = 0o755
		}
		if err := extract(root, f, name, perm); err != nil {
			return err
		}
	}
	return nil
}

func extract(root *os.Root, f *zip.File, name string, perm os.FileMode) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, io.LimitReader(r, maxPackage)); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// swap replaces the plugin's folder with the staged one and starts it; if
// it fails to start, the previous folder comes back and starts again.
func (m *Manager) swap(ctx context.Context, id, staged string) (Info, error) {
	m.mu.Lock()
	old, _ := m.find(id)
	folder := id
	if old != nil {
		folder = old.folder
		if old.plugin != nil {
			if err := old.plugin.Close(ctx); err != nil {
				m.log.WarnContext(ctx, "stop plugin", "plugin", id, "err", err)
			}
			old.plugin = nil
		}
	}
	m.mu.Unlock()

	target := filepath.Join(m.cfg.Dir, folder)
	backup := ""
	if _, err := os.Stat(target); err == nil {
		backup = filepath.Join(m.cfg.Dir, ".previous-"+rand.Text())
		if err := os.Rename(target, backup); err != nil {
			return Info{}, m.restart(ctx, old, err)
		}
	}
	if err := os.Rename(staged, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return Info{}, m.restart(ctx, old, err)
	}
	seen := m.otherIDs(id)
	e := m.start(ctx, folder, seen)
	if e.plugin == nil && backup != "" {
		// The new version failed: bring the old one back.
		_ = os.RemoveAll(target)
		if err := os.Rename(backup, target); err != nil {
			return Info{}, fmt.Errorf("restore plugin %s: %w", id, err)
		}
		return Info{}, m.restart(ctx, old, fmt.Errorf("new version failed: %w", e.err))
	}
	if backup != "" {
		_ = os.RemoveAll(backup)
	}
	m.put(e)
	m.log.InfoContext(ctx, "plugin installed", "plugin", id, "version", e.manifest.GetVersion())
	return e.info(), nil
}

// restart starts a plugin again after a failed swap and returns cause.
func (m *Manager) restart(ctx context.Context, old *entry, cause error) error {
	if old != nil {
		m.put(m.start(ctx, old.folder, m.otherIDs(old.manifest.GetId())))
	}
	return fmt.Errorf("install plugin: %w", cause)
}

func (m *Manager) otherIDs(id string) map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	for _, e := range m.plugins {
		if e.manifest.GetId() != id {
			seen[e.manifest.GetId()] = true
		}
	}
	return seen
}

// put replaces or adds a plugin's entry.
func (m *Manager) put(e *entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = slices.DeleteFunc(m.plugins, func(o *entry) bool { return o.manifest.GetId() == e.manifest.GetId() })
	m.plugins = append(m.plugins, e)
	m.sort()
}

// Uninstall stops a plugin and removes its folder and configuration.
func (m *Manager) Uninstall(ctx context.Context, id string) error {
	m.install.Lock()
	defer m.install.Unlock()
	m.mu.Lock()
	e, err := m.find(id)
	if err == nil {
		m.plugins = slices.DeleteFunc(m.plugins, func(o *entry) bool { return o == e })
	}
	m.mu.Unlock()
	if err != nil {
		return err
	}
	var errs []error
	if e.plugin != nil {
		errs = append(errs, e.plugin.Close(ctx))
	}
	errs = append(errs, os.RemoveAll(filepath.Join(m.cfg.Dir, e.folder)), m.store.PluginConfigs().Delete(ctx, id))
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("uninstall plugin %s: %w", id, err)
	}
	m.log.InfoContext(ctx, "plugin uninstalled", "plugin", id)
	return nil
}
