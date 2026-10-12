package acquisition

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// downloadPath returns the path of a download's files the server reads:
// an absolute local path, cleaned, or a path on an rclone remote, which
// downloads of cloud drives' offline downloads are at; false for others.
func downloadPath(p string) (string, bool) {
	if core.IsRemotePath(p) {
		_, rest, ok := core.SplitRemotePath(p)
		return strings.TrimRight(p, "/"), ok && strings.Trim(rest, "/") != ""
	}
	if !filepath.IsAbs(p) {
		return p, false
	}
	return filepath.Clean(p), true
}

// statDownload stats a download's file or folder.
func (r *Runtime) statDownload(ctx context.Context, p string) (fs.FileInfo, error) {
	if !core.IsRemotePath(p) {
		return os.Stat(p)
	}
	st, name, err := r.openFile(ctx, p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()
	return st.Stat(ctx, name)
}

// walkDownload walks a download's folder as filepath.WalkDir does, passing
// fn the paths of its entries as root's are written.
func (r *Runtime) walkDownload(ctx context.Context, root string, fn fs.WalkDirFunc) error {
	if !core.IsRemotePath(root) {
		return filepath.WalkDir(root, fn)
	}
	st, err := r.Storages.Open(ctx, root)
	if err != nil {
		return fn(root, nil, err)
	}
	defer func() { _ = st.Close() }()
	return fs.WalkDir(core.StorageFS(ctx, st), ".", func(name string, d fs.DirEntry, err error) error {
		return fn(core.StoragePath(root, name), d, err)
	})
}

// removeDownload deletes a download's folder and everything in it.
func (r *Runtime) removeDownload(ctx context.Context, root string) error {
	if !core.IsRemotePath(root) {
		return os.RemoveAll(root)
	}
	st, name, err := r.openFile(ctx, root)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if ra, ok := st.(interface {
		RemoveAll(ctx context.Context, name string) error
	}); ok {
		return ra.RemoveAll(ctx, name)
	}
	var dirs []string
	err = fs.WalkDir(core.StorageFS(ctx, st), name, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			dirs = append(dirs, p)
			return nil
		}
		return st.Remove(ctx, p)
	})
	if err != nil {
		return err
	}
	for _, d := range slices.Backward(dirs) {
		if err := st.Remove(ctx, d); err != nil {
			return err
		}
	}
	return nil
}
