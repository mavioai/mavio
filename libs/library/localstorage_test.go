package library

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestLocalStorage(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	outside := t.TempDir()
	tree(t, dir, "Movie/movie.mkv")
	tree(t, outside, "secret.txt")
	st, err := OpenLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.WriteFile(ctx, "Movie/extras/movie.nfo", []byte("<movie/>")); err != nil {
		t.Fatal(err)
	}
	if data, err := fs.ReadFile(core.StorageFS(ctx, st), "Movie/extras/movie.nfo"); err != nil || string(data) != "<movie/>" {
		t.Errorf("ReadFile() = %q, %v, want = <movie/>", data, err)
	}
	entries, err := st.ReadDir(ctx, "Movie/extras")
	if err != nil || len(entries) != 1 {
		t.Errorf("ReadDir() = %v, %v, want = movie.nfo alone, no file left aside", entries, err)
	}

	w, err := st.Create(ctx, "Movie/big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "0123456789"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(ctx, "Movie/big.bin"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("Create(existing) error = %v, want = %v", err, fs.ErrExist)
	}
	f, err := st.Open(ctx, "Movie/big.bin")
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 3)
	if _, err := f.ReadAt(b, 4); err != nil || string(b) != "456" {
		t.Errorf("ReadAt(4) = %q, %v, want = 456", b, err)
	}
	_ = f.Close()

	mtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := st.Chtimes(ctx, "Movie/big.bin", mtime); err != nil {
		t.Fatal(err)
	}
	if info, err := st.Stat(ctx, "Movie/big.bin"); err != nil || !info.ModTime().Equal(mtime) {
		t.Errorf("Stat().ModTime() = %v, %v, want = %v", info, err, mtime)
	}
	if err := st.Rename(ctx, "Movie/big.bin", "Movie/renamed.bin"); err != nil {
		t.Fatal(err)
	}
	if err := st.Link(ctx, "Movie/renamed.bin", "Movie/linked.bin"); err != nil {
		t.Fatal(err)
	}
	if err := st.Copy(ctx, "Movie/renamed.bin", "Movie/copy.bin"); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("Copy() error = %v, want = %v", err, core.ErrUnsupported)
	}
	if err := st.Remove(ctx, "Movie/renamed.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "Movie/renamed.bin"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(removed) error = %v, want = %v", err, fs.ErrNotExist)
	}

	if u, err := st.MediaURL(ctx, "Movie/movie.mkv"); err != nil || u != "file:"+filepath.Join(dir, "Movie", "movie.mkv") {
		t.Errorf("MediaURL() = %q, %v", u, err)
	}
	if p, ok := core.LocalPath(st, "Movie/movie.mkv"); !ok || p != filepath.Join(dir, "Movie", "movie.mkv") {
		t.Errorf("LocalPath() = %q, %v", p, ok)
	}
	if !st.Capabilities().Local {
		t.Error("Capabilities().Local = false")
	}

	// Nothing outside the folder is reached.
	if _, err := st.Stat(ctx, "../"+filepath.Base(outside)+"/secret.txt"); err == nil {
		t.Error("Stat() reached a file outside the folder")
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, filepath.Join(dir, "out")); err != nil {
			t.Fatal(err)
		}
		// os.Root follows relative links only.
		if err := os.Symlink("Movie", filepath.Join(dir, "in")); err != nil {
			t.Fatal(err)
		}
		entries, err := st.ReadDir(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
			if e.Name() == "in" && !e.IsDir() {
				t.Error("a link to a folder is not listed as one")
			}
		}
		if want := []string{"Movie", "in"}; !slices.Equal(names, want) {
			t.Errorf("ReadDir(.) = %v, want = %v", names, want)
		}
	}
}

func TestStorages(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	tree(t, dir, "a/b.mkv")
	var none *Storages
	if _, err := none.Open(ctx, "rclone:gdrive:Movies"); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("Open(remote) without rclone error = %v, want = %v", err, core.ErrUnsupported)
	}
	var gotRemote, gotDir string
	s := &Storages{Remote: func(_ context.Context, remote, dir string) (core.Storage, error) {
		gotRemote, gotDir = remote, dir
		return OpenLocalStorage(dir)
	}}
	if st, err := s.Open(ctx, "rclone:gdrive:"+filepath.ToSlash(dir)[1:]); err == nil {
		_ = st.Close()
	}
	if gotRemote != "gdrive" || gotDir != filepath.ToSlash(dir)[1:] {
		t.Errorf("Remote(%q, %q), want = gdrive, %q", gotRemote, gotDir, filepath.ToSlash(dir)[1:])
	}

	root := filepath.ToSlash(dir)
	if r, name, ok := RootOf([]string{"/elsewhere", dir}, root+"/a/b.mkv"); !ok || r != root || name != "a/b.mkv" {
		t.Errorf("RootOf() = %q, %q, %v, want = %q, a/b.mkv, true", r, name, ok, root)
	}
	if _, _, ok := RootOf([]string{dir}, root); ok {
		t.Error("RootOf(root itself) is a file of it")
	}
	f, err := none.OpenFile(ctx, root, "a/b.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	m, err := none.Media(ctx, root+"/a/b.mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Remote() || m.Local != filepath.Join(dir, "a", "b.mkv") || m.URL != "file:"+m.Local {
		t.Errorf("Media() = %+v", m)
	}
}
