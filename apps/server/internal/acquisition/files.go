package acquisition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/library/storage"
)

// transferMode is how an imported file reaches the library, as Radarr's
// TransferMode.
type transferMode int

const (
	transferCopy transferMode = iota
	transferMove
	// transferHardLinkOrCopy links the file, copying it where links are
	// not possible, as across volumes.
	transferHardLinkOrCopy
)

func (m transferMode) String() string {
	switch m {
	case transferMove:
		return "move"
	case transferHardLinkOrCopy:
		return "hardlink"
	}
	return "copy"
}

// errDestinationExists is returned when an import would overwrite a file.
var errDestinationExists = errors.New("destination already exists")

// transfer moves, links or copies src to rel in root, whose folder must
// exist. The destination is confined to root: copies are written through
// it, and moves and links go to the path it resolves.
func transfer(root *os.Root, src, rel string, mode transferMode) error {
	if _, err := root.Lstat(rel); err == nil {
		return fmt.Errorf("%s: %w", rel, errDestinationExists)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// Stat resolves the folder within root, failing for one that escapes.
	if _, err := root.Stat(filepath.Dir(rel)); err != nil {
		return err
	}
	dst := filepath.Join(root.Name(), rel)
	switch mode {
	case transferMove:
		if err := os.Rename(src, dst); err == nil {
			return nil
		}
		// Across volumes: copy, then delete the source.
		if err := copyFile(root, src, rel); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil {
			return fmt.Errorf("remove moved file: %w", err)
		}
		return nil
	case transferHardLinkOrCopy:
		if err := os.Link(src, dst); err == nil {
			return nil
		}
	}
	return copyFile(root, src, rel)
}

// copyFile copies src to rel in root, keeping its modification time; a
// partial copy is removed.
func copyFile(root *os.Root, src, rel string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = root.Remove(rel)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	if err = out.Sync(); err != nil {
		return err
	}
	return root.Chtimes(rel, time.Time{}, fi.ModTime())
}

// recycle sends a file to the recycle bin, into a folder named sub, or
// deletes it when no bin is set, as Radarr's
// RecycleBinProvider.DeleteFile: a name taken in the bin takes "_2",
// "_3"…, and the file's modification time becomes now, for the cleanup.
// A file already gone is no error.
func (r *Runtime) recycle(ctx context.Context, file, sub string) error {
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	bin := r.settings().RecycleBin
	if strings.TrimSpace(bin) == "" {
		r.logger().InfoContext(ctx, "recycle bin is not set; deleting permanently", "path", file)
		return os.Remove(file)
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return fmt.Errorf("recycle bin: %w", err)
	}
	root, err := os.OpenRoot(bin)
	if err != nil {
		return fmt.Errorf("recycle bin: %w", err)
	}
	defer func() { _ = root.Close() }()
	if sub = filepath.Base(sub); sub == "." || sub == string(filepath.Separator) || !filepath.IsLocal(sub) {
		sub = ""
	}
	if err := root.MkdirAll(filepath.Join(".", sub), 0o755); err != nil {
		return fmt.Errorf("recycle bin folder %q: %w", sub, err)
	}
	name := filepath.Base(file)
	ext := filepath.Ext(name)
	rel := filepath.Join(sub, name)
	for i := 2; ; i++ {
		if _, err := root.Lstat(rel); errors.Is(err, fs.ErrNotExist) {
			break
		}
		rel = filepath.Join(sub, strings.TrimSuffix(name, ext)+"_"+strconv.Itoa(i)+ext)
	}
	if err := transfer(root, file, rel, transferMove); err != nil {
		return fmt.Errorf("move %s to the recycle bin: %w", file, err)
	}
	now := r.now()
	if err := root.Chtimes(rel, now, now); err != nil {
		r.logger().WarnContext(ctx, "couldn't set the time of a recycled file", "path", rel, "err", err)
	}
	r.logger().DebugContext(ctx, "moved file to the recycle bin", "path", file, "to", rel)
	return nil
}

// CleanRecycleBin deletes the files that have been in the recycle bin
// longer than RecycleBinCleanupDays, then the folders left empty, as
// Radarr's RecycleBinProvider.Cleanup. No bin or 0 days does nothing.
func (r *Runtime) CleanRecycleBin(ctx context.Context) error {
	s := r.settings()
	if strings.TrimSpace(s.RecycleBin) == "" || s.RecycleBinCleanupDays == 0 {
		return nil
	}
	root, err := os.OpenRoot(s.RecycleBin)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("recycle bin: %w", err)
	}
	defer func() { _ = root.Close() }()
	cutoff := r.now().AddDate(0, 0, -s.RecycleBinCleanupDays)
	var dirs []string
	deleted := 0
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." {
				dirs = append(dirs, p)
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if fi.ModTime().After(cutoff) {
			return nil
		}
		if err := root.Remove(filepath.FromSlash(p)); err != nil {
			r.logger().ErrorContext(ctx, "couldn't delete file from the recycle bin", "path", p, "err", err)
			return nil
		}
		deleted++
		return nil
	})
	if err != nil {
		return fmt.Errorf("clean recycle bin: %w", err)
	}
	// Deepest first, so that emptied parents go too.
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := fs.ReadDir(root.FS(), dirs[i])
		if err == nil && len(entries) == 0 {
			_ = root.Remove(filepath.FromSlash(dirs[i]))
		}
	}
	r.logger().InfoContext(ctx, "cleaned the recycle bin", "deleted", deleted, "days", s.RecycleBinCleanupDays)
	return nil
}

// pace waits for playback to quiet down and takes the slot of the volume
// holding path when its reads are serialized, as scans do; the returned
// function gives the slot back.
func (r *Runtime) pace(ctx context.Context, path string) (func(), error) {
	if r.QuietGate != nil {
		if err := r.QuietGate.PauseOrCancel(ctx); err != nil {
			return nil, err
		}
	}
	if r.VolumeLedger == nil {
		return func() {}, nil
	}
	devices := r.Devices
	if devices == nil {
		r.mu.Lock()
		if r.devices == nil {
			r.devices = &storage.Detector{}
		}
		devices = r.devices
		r.mu.Unlock()
	}
	if dev := devices.Device(path); dev.Serialized() && dev.ID != "" {
		return r.VolumeLedger.Acquire(ctx, dev.ID)
	}
	return func() {}, nil
}
