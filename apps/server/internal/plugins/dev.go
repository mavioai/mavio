package plugins

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/plugin/manifest"
)

// DevBuild is the folder of a plugin's project that its dev target builds
// the plugin into, for a server to run in place (Config.Dev).
const DevBuild = "dist/dev"

// devFolder is the folder a plugin under development runs from: the
// development folder of its project, a folder holding a manifest but no
// executable, or else the folder itself.
func devFolder(path string) string {
	if _, err := os.Stat(filepath.Join(path, manifest.File)); err != nil {
		return path
	}
	for _, exe := range []string{"plugin", "plugin.exe", "plugin.wasm"} {
		if _, err := os.Stat(filepath.Join(path, exe)); err == nil {
			return path
		}
	}
	return filepath.Join(path, filepath.FromSlash(DevBuild))
}

// dir is the folder a plugin runs from.
func (m *Manager) dir(e *entry) string {
	if e.dev {
		return devFolder(e.folder)
	}
	return filepath.Join(m.cfg.Dir, e.folder)
}

// isDev reports whether a plugin under development has the ID.
func (m *Manager) isDev(id string) bool {
	return slices.ContainsFunc(m.plugins, func(e *entry) bool { return e.dev && e.manifest.GetId() == id })
}

// refuseDev refuses to install or uninstall a plugin under development.
func (m *Manager) refuseDev(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isDev(id) {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plugin %s runs from its development folder", id))
	}
	return nil
}

// startWatching watches the folders of the plugins under development from
// what they hold now.
func (m *Manager) startWatching() {
	if len(m.cfg.Dev) == 0 {
		return
	}
	last := map[string]string{}
	for _, p := range m.cfg.Dev {
		last[p] = fingerprint(devFolder(p))
	}
	go m.watchDev(last)
}

// watchDev restarts the plugins under development when the files of their
// folders change, once they have stayed the same for a poll: a build is
// seen once it is done. A folder made or emptied counts as a change.
func (m *Manager) watchDev(last map[string]string) {
	poll := m.cfg.DevPoll
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	pending := map[string]bool{}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
		}
		for _, p := range m.cfg.Dev {
			f := fingerprint(devFolder(p))
			if f != last[p] {
				last[p], pending[p] = f, true
				continue
			}
			if !pending[p] {
				continue
			}
			pending[p] = false
			m.reloadDev(m.ctx, p)
		}
	}
}

// reloadDev restarts the plugin under development of a project or folder.
func (m *Manager) reloadDev(ctx context.Context, path string) {
	m.mu.Lock()
	i := slices.IndexFunc(m.plugins, func(e *entry) bool { return e.dev && e.folder == path })
	var id string
	if i >= 0 {
		id = m.plugins[i].manifest.GetId()
	}
	m.mu.Unlock()
	if id == "" {
		return
	}
	m.log.InfoContext(ctx, "development plugin changed", "plugin", id, "folder", devFolder(path))
	if err := m.Restart(ctx, id); err != nil {
		m.log.WarnContext(ctx, "restart development plugin", "plugin", id, "err", err)
	}
}

// fingerprint sums the names, sizes, modes and modification times of the
// files in a folder, the same for every missing or empty folder.
func fingerprint(dir string) string {
	h := sha256.New()
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil || d.IsDir() {
			return nil
		}
		fmt.Fprintf(h, "%s\x00%d\x00%v\x00%d\n", p, info.Size(), info.Mode(), info.ModTime().UnixNano())
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}
