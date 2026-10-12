package acquisition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
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

// folder is the storage of a library path or recycle bin that imports
// write to.
type folder struct {
	core.Storage
	// root is the folder's path, as items record it.
	root string
}

// path returns the path, as items record it, of rel in the folder.
func (f folder) path(rel string) string { return core.StoragePath(f.root, filepath.ToSlash(rel)) }

// openFolder opens the storage of a library path or recycle bin; the
// caller closes it.
func (r *Runtime) openFolder(ctx context.Context, root string) (folder, error) {
	if !core.IsRemotePath(root) {
		root = filepath.ToSlash(filepath.Clean(root))
	}
	st, err := r.Storages.Open(ctx, root)
	if err != nil {
		return folder{}, err
	}
	return folder{st, root}, nil
}

// openFile opens the storage of the folder holding p, a local path or a
// path on a remote, and returns p's name in it; the caller closes it.
func (r *Runtime) openFile(ctx context.Context, p string) (core.Storage, string, error) {
	p = filepath.ToSlash(p)
	st, err := r.Storages.Open(ctx, path.Dir(p))
	if err != nil {
		return nil, "", err
	}
	return st, path.Base(p), nil
}

// transfer moves, links or copies src, a local path or a path on a
// remote, to rel in dst, whose folder must exist. Local files move and
// link to the path dst resolves; a remote that holds both moves and
// copies the file itself; otherwise the file is copied through the
// server, and a move then deletes it.
func (r *Runtime) transfer(ctx context.Context, dst folder, src, rel string, mode transferMode) error {
	name := filepath.ToSlash(rel)
	if _, err := dst.Stat(ctx, name); err == nil {
		return fmt.Errorf("%s: %w", rel, errDestinationExists)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// Stat resolves the folder within dst, failing for one that escapes.
	if _, err := dst.Stat(ctx, path.Dir(name)); err != nil {
		return err
	}
	switch to, local := core.LocalPath(dst.Storage, name); {
	case local && !core.IsRemotePath(src):
		switch mode {
		case transferMove:
			if err := os.Rename(src, to); err == nil {
				return nil
			}
		case transferHardLinkOrCopy:
			if err := os.Link(src, to); err == nil {
				return nil
			}
		}
	case core.IsRemotePath(src):
		if t, ok := dst.Storage.(core.StorageTransfers); ok {
			var err error
			if mode == transferMove {
				err = t.MoveFrom(ctx, src, name)
			} else {
				err = t.CopyFrom(ctx, src, name)
			}
			if !errors.Is(err, core.ErrUnsupported) {
				return err
			}
		}
	}
	if err := r.copyFile(ctx, dst, src, name); err != nil {
		return err
	}
	if mode == transferMove {
		if err := r.removeFile(ctx, src); err != nil {
			return fmt.Errorf("remove moved file: %w", err)
		}
	}
	return nil
}

// copyFile copies src to name in dst, keeping its modification time; a
// partial copy is removed.
func (r *Runtime) copyFile(ctx context.Context, dst folder, src, name string) (err error) {
	st, base, err := r.openFile(ctx, src)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	in, err := st.Open(ctx, base)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := dst.Create(ctx, name)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = dst.Remove(ctx, name)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	if f, ok := out.(interface{ Sync() error }); ok {
		if err = f.Sync(); err != nil {
			return err
		}
	}
	if err := dst.Chtimes(ctx, name, fi.ModTime()); err != nil && !errors.Is(err, core.ErrUnsupported) {
		return err
	}
	return nil
}

// removeFile deletes a local file or a file on a remote.
func (r *Runtime) removeFile(ctx context.Context, p string) error {
	if !core.IsRemotePath(p) {
		return os.Remove(p)
	}
	st, name, err := r.openFile(ctx, p)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	return st.Remove(ctx, name)
}

// recycle sends a file to the recycle bin, into a folder named sub, or
// deletes it when no bin is set, as Radarr's
// RecycleBinProvider.DeleteFile: a name taken in the bin takes "_2",
// "_3"…, and the file's modification time becomes now, for the cleanup.
// A file already gone is no error.
func (r *Runtime) recycle(ctx context.Context, file, sub string) error {
	st, name, err := r.openFile(ctx, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	_, err = st.Stat(ctx, name)
	_ = st.Close()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	bin := r.settings().RecycleBin
	if strings.TrimSpace(bin) == "" {
		r.logger().InfoContext(ctx, "recycle bin is not set; deleting permanently", "path", file)
		return r.removeFile(ctx, file)
	}
	if !core.IsRemotePath(bin) {
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return fmt.Errorf("recycle bin: %w", err)
		}
	}
	dst, err := r.openFolder(ctx, bin)
	if err != nil {
		return fmt.Errorf("recycle bin: %w", err)
	}
	defer func() { _ = dst.Close() }()
	if sub = path.Base(filepath.ToSlash(sub)); !fs.ValidPath(sub) || sub == "." || sub == "/" {
		sub = ""
	}
	if err := dst.MkdirAll(ctx, path.Join(".", sub)); err != nil {
		return fmt.Errorf("recycle bin folder %q: %w", sub, err)
	}
	ext := path.Ext(name)
	rel := path.Join(sub, name)
	for i := 2; ; i++ {
		if _, err := dst.Stat(ctx, rel); errors.Is(err, fs.ErrNotExist) {
			break
		}
		rel = path.Join(sub, strings.TrimSuffix(name, ext)+"_"+strconv.Itoa(i)+ext)
	}
	if err := r.transfer(ctx, dst, file, rel, transferMove); err != nil {
		return fmt.Errorf("move %s to the recycle bin: %w", file, err)
	}
	if err := dst.Chtimes(ctx, rel, r.now()); err != nil {
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
	root, err := r.openFolder(ctx, s.RecycleBin)
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
	err = fs.WalkDir(core.StorageFS(ctx, root), ".", func(p string, d fs.DirEntry, err error) error {
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
		if err := root.Remove(ctx, p); err != nil {
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
		entries, err := root.ReadDir(ctx, dirs[i])
		if err == nil && len(entries) == 0 {
			_ = root.Remove(ctx, dirs[i])
		}
	}
	r.logger().InfoContext(ctx, "cleaned the recycle bin", "deleted", deleted, "days", s.RecycleBinCleanupDays)
	return nil
}

// freeSpace returns the bytes free on the volume of path, or of its
// nearest existing parent; unknown reports false.
func (r *Runtime) freeSpace(path string) (int64, bool) {
	if r.FreeSpace != nil {
		return r.FreeSpace(path)
	}
	return freeSpace(path)
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
