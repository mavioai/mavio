package library

import (
	"context"
	"fmt"
	"path"
	"slices"

	"github.com/mavioai/mavio/libs/core"
)

// FolderResolver takes part in scans before the built-in resolvers, such
// as a resolver plugin.
type FolderResolver interface {
	Name() string
	// Ignore returns the paths of a folder's entries to leave out.
	Ignore(ctx context.Context, scope Scope, dir string, entries []Entry) ([]string, error)
	// Resolve returns what a folder holds when the resolver claims it,
	// else nil.
	Resolve(ctx context.Context, scope Scope, dir string, entries []Entry) (*Result, error)
}

// resolvedKinds are the kinds of items folder resolvers may find.
var resolvedKinds = []core.ItemKind{
	core.KindMovie, core.KindSeries, core.KindSeason, core.KindEpisode, core.KindVideo, core.KindMusicArtist,
	core.KindMusicAlbum, core.KindTrack, core.KindMusicVideo, core.KindAudioBook, core.KindBook, core.KindPhoto,
}

func (sc *scan) resolvers() []FolderResolver {
	if sc.Resolvers == nil {
		return nil
	}
	return sc.Resolvers()
}

// resolve resolves a folder: the folder resolvers leave entries out, then
// the first that claims the folder resolves it, else the built-in
// resolvers do.
func (sc *scan) resolve(ctx context.Context, t task, entries []Entry, l Lister) (Result, error) {
	resolvers := sc.resolvers()
	for _, r := range resolvers {
		ignored, err := r.Ignore(ctx, t.scope, t.dir, entries)
		if err != nil {
			return Result{}, &resolverError{r.Name(), err}
		}
		if len(ignored) > 0 {
			entries = slices.DeleteFunc(slices.Clone(entries), func(e Entry) bool { return slices.Contains(ignored, e.Path) })
		}
	}
	for _, r := range resolvers {
		res, err := r.Resolve(ctx, t.scope, t.dir, entries)
		if err == nil && res != nil {
			err = checkResolved(res, t.dir, entries)
		}
		if err != nil {
			return Result{}, &resolverError{r.Name(), err}
		}
		if res != nil {
			return *res, nil
		}
	}
	return sc.Resolver.Resolve(t.scope, t.dir, entries, l)
}

// resolverError is a folder resolver failing; the folder keeps its items.
type resolverError struct {
	resolver string
	err      error
}

func (e *resolverError) Error() string { return fmt.Sprintf("resolver %s: %v", e.resolver, e.err) }

func (e *resolverError) Unwrap() error { return e.err }

// checkResolved checks what a folder resolver found in dir: items of the
// kinds scans find, made of the folder or of its files, and subfolders
// among its folders.
func checkResolved(res *Result, dir string, entries []Entry) error {
	isEntry := func(p string, isDir bool) bool {
		return slices.ContainsFunc(entries, func(e Entry) bool { return e.Path == p && e.IsDir == isDir })
	}
	check := func(n Node) error {
		switch {
		case !slices.Contains(resolvedKinds, n.Kind):
			return fmt.Errorf("item %q: kind %q", n.Path, n.Kind)
		case n.Name == "":
			return fmt.Errorf("item %q has no name", n.Path)
		case len(n.Extras) > 0 || len(n.Versions) > 0 || n.Extra != "":
			return fmt.Errorf("item %q: extras and versions are the built-in resolvers'", n.Path)
		}
		for _, p := range n.Parts {
			if !isEntry(p, false) {
				return fmt.Errorf("item %q: part %q is not a file of %s", n.Path, p, dir)
			}
		}
		return nil
	}
	if res.Item != nil {
		if res.Item.Path != dir {
			return fmt.Errorf("the folder's item has the path %q", res.Item.Path)
		}
		if err := check(*res.Item); err != nil {
			return err
		}
	}
	for _, n := range res.Items {
		if !isEntry(n.Path, false) {
			return fmt.Errorf("item %q is not a file of %s", n.Path, dir)
		}
		if err := check(n); err != nil {
			return err
		}
	}
	for _, s := range res.Subfolders {
		if !isEntry(s.Path, true) || path.Dir(s.Path) != dir {
			return fmt.Errorf("subfolder %q is not a folder of %s", s.Path, dir)
		}
	}
	return nil
}
