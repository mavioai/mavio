package library

import (
	"path"
	"slices"

	"github.com/mavioai/mavio/libs/core"
)

// resolveMovies resolves a folder of a movies or music videos library. A
// folder holding one movie, and no other folders but extras, is that
// movie; a folder holding a disc rip is a disc movie; folders of disc rips
// make one multi-disc movie; anything else is a folder of movies, each
// file its own item.
func (r *Resolver) resolveMovies(scope Scope, dir string, entries []Entry, l Lister, kind core.ItemKind, parseName bool) (Result, error) {
	files, dirs := split(entries)
	top := dir == scope.Root
	var res Result
	if !top {
		d, err := disc(entries, l)
		if err != nil {
			return res, err
		}
		if d != "" {
			n := r.folderVideo(dir, kind, parseName)
			n.Disc = d
			setMovieIDs(&n, dir)
			res.Item = &n
			return res, nil
		}
	}
	var others []Entry
	for _, d := range dirs {
		if _, extras := r.extrasFolderKind(d.Name()); !extras {
			others = append(others, d)
		}
	}
	movies := r.videos(dir, files, kind, parseName, top)
	if !top && len(movies) == 1 && len(others) == 0 {
		n := movies[0]
		n.InMixedFolder = false
		if kind == core.KindMovie {
			// A movie in its own folder is named after the folder.
			folder := r.folderVideo(dir, kind, true)
			n.Name, n.Year = folder.Name, cmpYear(folder.Year, n.Year)
		}
		setMovieIDs(&n, dir)
		extras, err := r.Extras(n, false, entries, l)
		if err != nil {
			return res, err
		}
		n.Extras = extras
		res.Item = &n
		return res, nil
	}
	if !top && len(movies) == 0 && len(others) > 0 {
		n, ok, err := r.multiDisc(others, kind, l)
		if err != nil {
			return res, err
		}
		if ok {
			setMovieIDs(&n, dir)
			res.Item = &n
			return res, nil
		}
	}
	for i := range movies {
		n := &movies[i]
		setMovieIDs(n, "")
		extras, err := r.Extras(*n, false, files, l)
		if err != nil {
			return res, err
		}
		n.Extras = extras
	}
	res.Items = movies
	for _, d := range others {
		res.Subfolders = append(res.Subfolders, Subfolder{Path: d.Path, Scope: Scope{Kind: scope.Kind, Root: scope.Root}})
	}
	return res, nil
}

func cmpYear(a, b *int) *int {
	if a != nil {
		return a
	}
	return b
}

// folderVideo names a video made of a folder.
func (r *Resolver) folderVideo(dir string, kind core.ItemKind, parseName bool) Node {
	n := Node{Kind: kind, Path: dir, Name: path.Base(dir)}
	if v, ok := r.Parser.ResolveVideo(dir, true, parseName, ""); ok {
		n.Name, n.Year = v.Name, v.Year
		n.Format3D = format3D(v.Is3D, v.Format3D)
	}
	return n
}

// multiDisc ports MovieResolver.GetMultiDiscMovie: subfolders that each
// hold a disc rip of the same type and stack by name, such as "Disc 1" and
// "Disc 2", make one movie.
func (r *Resolver) multiDisc(dirs []Entry, kind core.ItemKind, l Lister) (Node, bool, error) {
	if l == nil {
		return Node{}, false, nil
	}
	var paths []string
	discType := ""
	for _, d := range dirs {
		sub, err := l.List(d.Path)
		if err != nil {
			return Node{}, false, err
		}
		t, err := disc(sub, l)
		if err != nil {
			return Node{}, false, err
		}
		if t == "" {
			continue
		}
		if discType != "" && discType != t {
			return Node{}, false, nil
		}
		discType = t
		paths = append(paths, d.Path)
	}
	if len(paths) == 0 {
		return Node{}, false, nil
	}
	slices.Sort(paths)
	stacks := r.Parser.ResolveDirectoryStacks(paths)
	if len(stacks) != 1 {
		return Node{}, false, nil
	}
	return Node{Kind: kind, Path: paths[0], Parts: paths[1:], Disc: discType, Name: stacks[0].Name}, true, nil
}
