package library

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// rootFS gives access to one library folder through os.Root, by absolute
// slash-separated paths below it.
type rootFS struct {
	root *os.Root
	base string // slash-separated
}

// rel returns the root-relative OS path of an absolute slash path.
func (f rootFS) rel(p string) string {
	if p == f.base {
		return "."
	}
	return filepath.FromSlash(strings.TrimPrefix(p, f.base+"/"))
}

func (f rootFS) stat(p string) (fs.FileInfo, error) { return f.root.Stat(f.rel(p)) }

// List implements Lister.
func (f rootFS) List(dir string) ([]Entry, error) {
	des, err := fs.ReadDir(f.root.FS(), filepath.ToSlash(f.rel(dir)))
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		isDir := de.IsDir()
		if de.Type()&fs.ModeSymlink != 0 {
			// Follow links that stay within the root; os.Root refuses others.
			info, err := f.stat(path.Join(dir, de.Name()))
			if err != nil {
				continue
			}
			isDir = info.IsDir()
		}
		out = append(out, Entry{Path: path.Join(dir, de.Name()), IsDir: isDir})
	}
	return out, nil
}
