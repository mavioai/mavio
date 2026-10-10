package library

import (
	"context"
	"errors"
	"maps"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// claimer leaves out files named "skip*" and claims folders holding
// "claim.me", each other file of which is a home video; with fail set it
// fails, and with bad set it claims files of other folders.
type claimer struct{ fail, bad bool }

func (*claimer) Name() string { return "claimer" }

func (c *claimer) Ignore(_ context.Context, _ Scope, _ string, entries []Entry) ([]string, error) {
	if c.fail {
		return nil, errors.New("broken")
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "skip") {
			out = append(out, e.Path)
		}
	}
	return out, nil
}

func (c *claimer) Resolve(_ context.Context, _ Scope, dir string, entries []Entry) (*Result, error) {
	claimed := false
	res := &Result{}
	for _, e := range entries {
		switch {
		case e.Name() == "claim.me":
			claimed = true
		case !e.IsDir:
			res.Items = append(res.Items, Node{Kind: core.KindVideo, Path: e.Path, Name: strings.TrimSuffix(e.Name(), path.Ext(e.Name()))})
		}
	}
	if c.bad {
		res.Items = append(res.Items, Node{Kind: core.KindVideo, Path: path.Join(path.Dir(dir), "other.mkv"), Name: "Other"})
	}
	if !claimed {
		return nil, nil
	}
	return res, nil
}

func TestFolderResolvers(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	c := &claimer{}
	f.sc.Resolvers = func() []FolderResolver { return []FolderResolver{c} }
	tree(t, f.root, "Heat (1995)/Heat (1995).mkv", "Heat (1995)/skip-me.mkv", "Odd/claim.me", "Odd/a.mkv", "Odd/b.mkv")
	f.scan()
	want := map[string]core.ItemKind{
		"Heat (1995)/Heat (1995).mkv": core.KindMovie,
		"Odd/a.mkv":                   core.KindVideo,
		"Odd/b.mkv":                   core.KindVideo,
	}
	if got := f.items(); !maps.Equal(got, want) {
		t.Errorf("items = %v, want = %v", got, want)
	}

	// A resolver failing, or claiming what is not the folder's, keeps the
	// folders' items.
	for _, broken := range []*claimer{{fail: true}, {bad: true}} {
		*c = *broken
		tree(t, f.root, "Odd/c.mkv")
		f.clock = f.clock.Add(time.Hour)
		if st := f.scan(); st.Unreadable == 0 {
			t.Errorf("%+v: scan = %+v, want unreadable folders", broken, st)
		}
		if got := f.items(); !maps.Equal(got, want) {
			t.Errorf("%+v: items = %v, want = %v", broken, got, want)
		}
	}
}
