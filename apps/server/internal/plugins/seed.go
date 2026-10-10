package plugins

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/plugin/manifest"
)

// Seed installs the plugins of a seed folder, such as those a container
// image ships, into the plugin folder dir: each once, so that a plugin
// uninstalled or upgraded stays so. A marker file in dir, which Open
// passes over, records each plugin seeded; a plugin dir already holds is
// not copied. A seed plugin that cannot be read or copied is logged and
// left out.
func Seed(ctx context.Context, seed, dir string, log *slog.Logger) error {
	entries, err := os.ReadDir(seed)
	if err != nil {
		return fmt.Errorf("plugin seed folder: %w", err)
	}
	held, err := heldIDs(dir)
	if err != nil {
		return err
	}
	for _, d := range entries {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		src := filepath.Join(seed, d.Name())
		m, err := manifest.Load(src)
		if err != nil {
			log.WarnContext(ctx, "seed plugin unreadable", "folder", src, "err", err)
			continue
		}
		marker := filepath.Join(dir, ".seeded-"+m.GetId())
		if _, err := os.Stat(marker); err == nil {
			continue
		}
		if !held[m.GetId()] {
			if err := copyPlugin(src, dir, d.Name()); err != nil {
				log.WarnContext(ctx, "seed plugin not installed", "plugin", m.GetId(), "err", err)
				continue
			}
			log.InfoContext(ctx, "seed plugin installed", "plugin", m.GetId(), "version", m.GetVersion())
		}
		if err := os.WriteFile(marker, []byte(m.GetVersion()+"\n"), 0o640); err != nil {
			return fmt.Errorf("mark plugin %s seeded: %w", m.GetId(), err)
		}
	}
	return nil
}

// heldIDs returns the IDs of the plugins in the plugin folder.
func heldIDs(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("plugin folder: %w", err)
	}
	held := map[string]bool{}
	for _, d := range entries {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if m, err := manifest.Load(filepath.Join(dir, d.Name())); err == nil {
			held[m.GetId()] = true
		}
	}
	return held, nil
}

// copyPlugin copies a plugin's folder into the plugin folder, under its
// name or, when that is taken, a free one; it appears whole or not at all.
func copyPlugin(src, dir, name string) error {
	tmp := filepath.Join(dir, ".seed-"+rand.Text())
	if err := os.CopyFS(tmp, os.DirFS(src)); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	target := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			break
		}
		target = filepath.Join(dir, fmt.Sprintf("%s-%d", name, i))
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return nil
}
