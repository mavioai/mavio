package library

import (
	"path"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
)

var bookExtensions = []string{".azw", ".azw3", ".cb7", ".cbr", ".cbt", ".cbz", ".epub", ".mobi", ".pdf"}

// resolveBooks resolves a folder of a books library: a folder holding one
// single-file audiobook is that audiobook; otherwise every single-file
// audiobook and every book file is an item.
func (r *Resolver) resolveBooks(scope Scope, dir string, entries []Entry) (Result, error) {
	files, dirs := split(entries)
	var res Result
	top := dir == scope.Root
	if !top {
		if book, ok := r.AudioBookFolder(dir, entries); ok {
			res.Item = &book
			return res, nil
		}
	}
	res.Items = r.audioBooks(files, true, top)
	for _, f := range files {
		if !slices.Contains(bookExtensions, strings.ToLower(path.Ext(f.Path))) {
			continue
		}
		b := naming.ParseBookFileName(fileNameWithoutExt(f.Path))
		n := Node{Kind: core.KindBook, Path: f.Path, Name: b.Name, Year: b.Year, Index: b.Index, ParentIndex: b.ParentIndex}
		if n.Name == "" {
			n.Name = fileNameWithoutExt(f.Path)
		}
		res.Items = append(res.Items, n)
	}
	for _, d := range dirs {
		res.Subfolders = append(res.Subfolders, Subfolder{d.Path, Scope{Kind: scope.Kind, Root: scope.Root, ParentPath: dir}})
	}
	return res, nil
}

// AudioBookFolder ports AudioResolver.FindAudioBook: a folder is an
// audiobook, named after the folder, when its audio files make exactly one
// single-file book. Books of several chapters or parts are not resolved
// yet.
func (r *Resolver) AudioBookFolder(dir string, entries []Entry) (Node, bool) {
	files, _ := split(entries)
	books := r.audioBooks(files, false, false)
	if len(books) != 1 {
		return Node{}, false
	}
	b := books[0]
	b.InMixedFolder = false
	b.Name = path.Base(dir)
	return b, true
}

// audioBooks ports AudioResolver.ResolveMultipleAudio: books made of more
// than one file, or with extras or alternate versions, are left out until
// they can be browsed as such.
func (r *Resolver) audioBooks(files []Entry, parseName, top bool) []Node {
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	books := r.Parser.ResolveAudioBooks(paths)
	mixed := len(books) > 1 || top
	var nodes []Node
	for _, b := range books {
		if len(b.Files) != 1 || len(b.Extras) > 0 || len(b.AlternateVersions) > 0 {
			continue
		}
		n := Node{Kind: core.KindAudioBook, Path: b.Files[0].Path, Year: b.Year, InMixedFolder: mixed, Name: fileNameWithoutExt(b.Files[0].Path)}
		if parseName {
			n.Name = b.Name
		}
		nodes = append(nodes, n)
	}
	return nodes
}
