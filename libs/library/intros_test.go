package library

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

type introPicker struct {
	ids  []core.ID
	fail bool
}

func (*introPicker) Name() string { return "picker" }

func (p *introPicker) Intros(context.Context, core.Item, core.ID) ([]core.ID, error) {
	if p.fail {
		return nil, errors.New("down")
	}
	return p.ids, nil
}

func TestIntros(t *testing.T) {
	ctx := t.Context()
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Heat (1995)/Heat (1995).mkv", "Alien (1979)/Alien (1979).mkv", "Up (2009)/Up (2009).mkv")
	f.scan()
	heat, alien, up := f.item("Heat (1995)/Heat (1995).mkv"), f.item("Alien (1979)/Alien (1979).mkv"), f.item("Up (2009)/Up (2009).mkv")
	picker := &introPicker{ids: []core.ID{heat.ID, alien.ID, core.NewID(), alien.ID, up.ID}}
	in := &Intros{Store: f.store, Source: func() []IntroProvider { return []IntroProvider{&introPicker{fail: true}, picker} }}

	user := &core.User{ID: core.NewID()}
	got, err := in.List(ctx, user, heat.ID)
	ids := make([]core.ID, len(got))
	for i := range got {
		ids[i] = got[i].ID
	}
	// The item itself, unknown items and repeats are left out.
	if err != nil || !slices.Equal(ids, []core.ID{alien.ID, up.ID}) {
		t.Errorf("List() = %v, %v, want [%s %s]", ids, err, alien.ID, up.ID)
	}

	other := &core.User{ID: core.NewID(), Policy: core.UserPolicy{Libraries: []core.ID{core.NewID()}}}
	if _, err := in.List(ctx, other, heat.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("List() for a user without the library = %v, want ErrNotFound", err)
	}
}
