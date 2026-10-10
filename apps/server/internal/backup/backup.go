// Package backup makes and restores backups of a server: a zip file with
// its database rows (store.Dump), the artwork chosen for items in the
// metadata folder, and its plugins.
package backup

import (
	"archive/zip"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

// manifest describes a backup.
type manifest struct {
	Version string         `json:"version"`
	Created time.Time      `json:"created"`
	Dump    store.DumpInfo `json:"dump"`
}

// Backup is a backup file.
type Backup struct {
	Name    string
	Size    int64
	Created time.Time
	// Version is the server version that made it.
	Version string
}

// Config configures a Manager.
type Config struct {
	Store *store.Store
	// Dir holds the backups.
	Dir string
	Folders
	Version string
	Now     func() time.Time
}

// Folders are the folders a backup holds besides the database; empty ones
// are skipped.
type Folders struct {
	Metadata, Plugins, PluginData string
}

func (f Folders) byPrefix() map[string]string {
	return map[string]string{"metadata/": f.Metadata, "plugins/": f.Plugins, "plugin-data/": f.PluginData}
}

// Manager makes and lists backups.
type Manager struct{ cfg Config }

// New returns a manager.
func New(cfg Config) *Manager {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Manager{cfg: cfg}
}

// validName reports whether name is a backup's file name.
func validName(name string) bool {
	return strings.HasSuffix(name, ".zip") && filepath.Base(name) == name && !strings.HasPrefix(name, ".")
}

// Create writes a new backup.
func (m *Manager) Create(ctx context.Context) (Backup, error) {
	if err := os.MkdirAll(m.cfg.Dir, 0o750); err != nil {
		return Backup{}, err
	}
	now := m.cfg.Now().UTC()
	name := "mavio-" + now.Format("20060102-150405") + ".zip"
	tmp, err := os.CreateTemp(m.cfg.Dir, ".backup-*")
	if err != nil {
		return Backup{}, err
	}
	defer os.Remove(tmp.Name())
	if err := m.write(ctx, tmp, now); err != nil {
		_ = tmp.Close()
		return Backup{}, err
	}
	if err := tmp.Close(); err != nil {
		return Backup{}, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(m.cfg.Dir, name)); err != nil {
		return Backup{}, err
	}
	return m.describe(name)
}

func (m *Manager) write(ctx context.Context, w io.Writer, now time.Time) error {
	zw := zip.NewWriter(w)
	info, err := m.cfg.Store.Dump(ctx, func(name string) (io.Writer, error) { return zw.Create("db/" + name) })
	if err != nil {
		return fmt.Errorf("back up the database: %w", err)
	}
	for prefix, dir := range m.cfg.byPrefix() {
		if err := addFolder(ctx, zw, prefix, dir); err != nil {
			return err
		}
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(mw).Encode(manifest{Version: m.cfg.Version, Created: now, Dump: info}); err != nil {
		return err
	}
	return zw.Close()
}

// addFolder adds a folder's files under prefix, leaving out hidden ones,
// such as plugin installations in progress.
func addFolder(ctx context.Context, zw *zip.Writer, prefix, dir string) error {
	if dir == "" {
		return nil
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name, h.Method = prefix+p, zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		f, err := root.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

func readManifest(zr *zip.Reader) (manifest, error) {
	f, err := zr.Open("manifest.json")
	if err != nil {
		return manifest{}, fmt.Errorf("%w: not a backup: %w", core.ErrInvalid, err)
	}
	defer f.Close()
	var mf manifest
	if err := json.NewDecoder(f).Decode(&mf); err != nil {
		return manifest{}, fmt.Errorf("%w: backup manifest: %w", core.ErrInvalid, err)
	}
	return mf, nil
}

func (m *Manager) describe(name string) (Backup, error) {
	zr, err := zip.OpenReader(filepath.Join(m.cfg.Dir, name))
	if err != nil {
		return Backup{}, err
	}
	defer zr.Close()
	mf, err := readManifest(&zr.Reader)
	if err != nil {
		return Backup{}, err
	}
	st, err := os.Stat(filepath.Join(m.cfg.Dir, name))
	if err != nil {
		return Backup{}, err
	}
	return Backup{Name: name, Size: st.Size(), Created: mf.Created, Version: mf.Version}, nil
}

// List lists the backups, newest first; unreadable files are left out.
func (m *Manager) List() ([]Backup, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var out []Backup
	for _, e := range entries {
		if e.IsDir() || !validName(e.Name()) {
			continue
		}
		if b, err := m.describe(e.Name()); err == nil {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b Backup) int { return cmp.Or(b.Created.Compare(a.Created), cmp.Compare(a.Name, b.Name)) })
	return out, nil
}

// Open opens a backup for reading.
func (m *Manager) Open(name string) (*os.File, error) {
	if !validName(name) {
		return nil, fmt.Errorf("backup %q: %w", name, core.ErrNotFound)
	}
	f, err := os.Open(filepath.Join(m.cfg.Dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("backup %q: %w", name, core.ErrNotFound)
	}
	return f, err
}

// Delete deletes a backup.
func (m *Manager) Delete(name string) error {
	if !validName(name) {
		return fmt.Errorf("backup %q: %w", name, core.ErrNotFound)
	}
	err := os.Remove(filepath.Join(m.cfg.Dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("backup %q: %w", name, core.ErrNotFound)
	}
	return err
}

// Restore restores a backup file into an empty database and puts its
// artwork, plugins and plugin data into the folders given, over files of
// the same names.
func Restore(ctx context.Context, file string, st *store.Store, folders Folders) error {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer zr.Close()
	mf, err := readManifest(&zr.Reader)
	if err != nil {
		return err
	}
	err = st.Restore(ctx, mf.Dump, func(name string) (io.ReadCloser, error) {
		f, err := zr.Open("db/" + name)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, core.ErrNotFound
		}
		return f, err
	})
	if err != nil {
		return fmt.Errorf("restore the database: %w", err)
	}
	for prefix, dir := range folders.byPrefix() {
		if err := extract(&zr.Reader, prefix, dir); err != nil {
			return err
		}
	}
	return nil
}

func extract(zr *zip.Reader, prefix, dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range zr.File {
		name, ok := strings.CutPrefix(f.Name, prefix)
		if !ok || name == "" || f.FileInfo().IsDir() {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("%w: backup names %q outside its folder", core.ErrInvalid, f.Name)
		}
		if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return err
		}
		if err := copyFile(root, f, name); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(root *os.Root, f *zip.File, name string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}
