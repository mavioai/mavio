package app

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// pageSize is how many items a grid loads at a time.
const pageSize = 100

// grid is the state of a grid of cards loaded page by page.
type grid struct {
	key     string
	cards   []*capiv1.Card
	total   int // -1 when unknown
	more    bool
	loading bool
	err     error
	state   ui.GridState
	started time.Time // when it started loading for key
}

// libraryState is a library page's state.
type libraryState struct {
	tab         int
	sort        capiv1.Sort
	filter      int // 0 all, 1 unplayed, 2 played, 3 favorites
	recommended task[[]*capiv1.Row]
}

// library builds a library: its Recommended tab and the grid of all its
// items.
func (a *App) library(c *ui.Context, r *ui.Route) {
	server, id := r.Param("server"), r.Param("id")
	name := ""
	for _, l := range a.serverLibraries(server).value.GetLibraries() {
		if l.GetId() == id {
			name = l.GetName()
		}
	}
	r.Title(name)
	st := a.libraryStates[key(server, id)]
	if st == nil {
		st = &libraryState{}
		a.libraryStates[key(server, id)] = st
	}
	tabs := func() {
		if style.TV(c) {
			a.tvPill(c, []string{style.T(c, "library.recommended"), style.T(c, "library.all")}, &st.tab)
			return
		}
		ui.Segmented(c, &st.tab, style.T(c, "library.recommended"), style.T(c, "library.all")).Width(240)
	}
	if st.tab == 0 {
		t := load(a, &st.recommended, key("recommended", server, id, a.generation), func(ctx context.Context) ([]*capiv1.Row, error) {
			m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{GetRecommended: capiv1.GetRecommended_builder{ServerId: server, LibraryId: id}.Build()}.Build())
			return m.GetRows().GetRows(), err
		})
		ui.Column(c).Fill().Children(func() {
			a.pageHeader(c, name, tabs)
			if taskStatus(c, t, func() { st.recommended.key = "" }, nil) {
				return
			}
			if len(t.value) == 0 {
				statusView(c, "", style.T(c, "state.empty"), "", nil)
				return
			}
			ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Grow(1).Children(func() {
				ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
					for _, row := range t.value {
						a.row(c, row)
					}
				})
			})
		})
		return
	}
	filters := []string{"library.filterAll", "library.filterUnplayed", "library.filterPlayed", "library.filterFavorites"}
	sorts := []struct {
		sort capiv1.Sort
		key  string
	}{
		{capiv1.Sort_SORT_UNSPECIFIED, "library.sortName"},
		{capiv1.Sort_SORT_DATE_ADDED, "library.sortDateAdded"},
		{capiv1.Sort_SORT_RELEASED, "library.sortReleased"},
		{capiv1.Sort_SORT_RATING, "library.sortRating"},
		{capiv1.Sort_SORT_LAST_PLAYED, "library.sortLastPlayed"},
		{capiv1.Sort_SORT_RANDOM, "library.sortRandom"},
	}
	sortName := ""
	for _, s := range sorts {
		if s.sort == st.sort {
			sortName = style.T(c, s.key)
		}
	}
	controls := func() {
		tabs()
		// On the television, the filters are a pill alike, which focus
		// applies (docs/tv.md §6.3).
		if style.TV(c) {
			labels := make([]string, len(filters))
			for i, f := range filters {
				labels[i] = style.T(c, f)
			}
			a.tvPill(c, labels, &st.filter)
		}
		ui.Spacer(c)
		ui.MenuButton(c, style.T(c, "library.sortBy", "sort", sortName), func(m *ui.Menu) {
			for _, s := range sorts {
				if m.Item(style.T(c, s.key)).Checked(s.sort == st.sort).Chosen() {
					st.sort = s.sort
				}
			}
		})
		if style.TV(c) {
			return
		}
		ui.MenuButton(c, style.T(c, filters[st.filter]), func(m *ui.Menu) {
			for i, f := range filters {
				if m.Item(style.T(c, f)).Checked(i == st.filter).Chosen() {
					st.filter = i
				}
			}
		})
	}
	filter := capiv1.Filter_builder{}
	switch st.filter {
	case 1:
		filter.Played = new(false)
	case 2:
		filter.Played = new(true)
	case 3:
		filter.Favorite = true
	}
	f := filter.Build()
	a.cardGrid(c, key("library", server, id, st.sort, st.filter), name, controls, func(ctx context.Context, offset, limit uint32) (*capiv1.Page, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{ListLibraryItems: capiv1.ListLibraryItems_builder{
			ServerId: server, LibraryId: id, Sort: st.sort, Filter: f, Offset: offset, Limit: limit,
		}.Build()}.Build())
		return m.GetPage(), err
	})
}

// pageHeader builds the top of a page with a title and controls beside it.
func (a *App) pageHeader(c *ui.Context, title string, controls func()) {
	colors := style.Colors(c)
	// On the television, the name heads the page at the top left, beside
	// the controls (docs/tv.md §6.3).
	if style.TV(c) {
		ui.Row(c).Padding(48, style.Layout(c).Margin, 24).Gap(32).AlignItems(ui.Center).DragWindow().Children(func() {
			style.Text(ui.Text(c, title), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary)).SingleLine().Shrink(1)
			if controls != nil {
				ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Center).Children(controls)
			}
		})
		return
	}
	ui.Column(c).Padding(a.pageTop(c), style.Layout(c).Margin, 16).Gap(12).DragWindow().Children(func() {
		style.Text(ui.Text(c, title), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary)).SingleLine()
		if controls != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(controls)
		}
	})
}

// cardGrid builds a page of a grid of cards that fetch loads page by page,
// starting anew when id changes.
func (a *App) cardGrid(c *ui.Context, id, title string, controls func(), fetch func(ctx context.Context, offset, limit uint32) (*capiv1.Page, error)) {
	g := a.grids[id]
	if g == nil {
		g = &grid{}
		a.grids[id] = g
	}
	k := key(id, a.generation)
	if g.key != k {
		*g = grid{key: k, total: -1, more: true, state: g.state, started: time.Now()}
	}
	next := func() {
		if g.loading || !g.more || g.err != nil {
			return
		}
		g.loading = true
		offset := uint32(len(g.cards))
		go func() {
			p, err := fetch(a.ctx, offset, pageSize)
			a.opts.Update(func() {
				if g.key != k {
					return
				}
				g.loading, g.err = false, err
				if err != nil {
					return
				}
				g.cards = append(g.cards, p.GetCards()...)
				g.more = p.GetMore() && len(p.GetCards()) > 0
				if p.HasTotal() {
					g.total = int(p.GetTotal())
				}
			})
		}()
	}
	if len(g.cards) == 0 {
		next()
	}
	colors := style.Colors(c)
	ui.Column(c).Fill().Children(func() {
		a.pageHeader(c, title, func() {
			if controls != nil {
				controls()
			}
			if g.total >= 0 {
				style.Text(ui.Text(c, style.T(c, "library.itemCount", "count", g.total)), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			}
		})
		switch {
		case len(g.cards) == 0 && g.err != nil:
			statusView(c, "error", style.T(c, "state.error"), errorMessage(c, g.err), func() { g.err = nil; next() })
			return
		case len(g.cards) == 0 && !g.more:
			statusView(c, "", style.T(c, "state.empty"), "", nil)
			return
		case len(g.cards) == 0:
			loading(c, g.started)
			return
		}
		width, _ := a.gridColumns(c)
		height := width*1.5 + 58
		ui.GridView(c, &g.state, len(g.cards), width-1, height, func(i int) {
			if i >= len(g.cards)-pageSize/2 {
				next()
			}
			a.posterCard(c, g.cards[i], width)
		}).Grow(1).Padding(0, style.Layout(c).Margin, 40).Gap(style.Layout(c).Gutter)
	})
}

// gridColumns returns the width of the cards of a grid filling the page,
// as many as fit at least the poster's width, and how many there are.
func (a *App) gridColumns(c *ui.Context) (float32, int) {
	avail := a.contentWidth - 2*style.Layout(c).Margin
	gap := style.Layout(c).Gutter
	cols := max(int((avail+gap)/(style.Layout(c).Poster+gap)), 1)
	return (avail - gap*float32(cols-1)) / float32(cols), cols
}

// librariesPage builds the TV mode's Libraries: each server's libraries,
// under the server's name when there are several, as a grid of their
// artwork at least a thumbnail wide (docs/tv.md §3).
func (a *App) librariesPage(c *ui.Context) {
	colors := style.Colors(c)
	layout := style.Layout(c)
	avail := a.contentWidth - 2*layout.Margin
	cols := max(int((avail+layout.Gutter)/(layout.Thumbnail*0.75+layout.Gutter)), 1)
	width := (avail - layout.Gutter*float32(cols-1)) / float32(cols)
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		ui.Column(c).Padding(8, layout.Margin, 40).Gap(32).Children(func() {
			for _, s := range a.servers {
				libs := a.serverLibraries(s.GetId()).value.GetLibraries()
				if len(libs) == 0 {
					continue
				}
				ui.Column(c.Key(s.GetId())).Gap(16).Children(func() {
					if len(a.servers) > 1 {
						style.Text(ui.Text(c, s.GetName()), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
					}
					ui.Row(c).Wrap().Gap(layout.Gutter).Children(func() {
						for _, l := range libs {
							a.libraryCard(c, s.GetId(), l, width)
						}
					})
				})
			}
		})
	})
}

// libraryCard builds a library's card on Libraries: its collage and name.
func (a *App) libraryCard(c *ui.Context, server string, l *capiv1.Library, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(l.GetId())).Cursor(ui.CursorPointer).Column().Width(width).Gap(10).AlignItems(ui.Start).Label(l.GetName())
	focused := a.cardFocus(c, b)
	b.Children(func() {
		a.artwork(c, capiv1.Artwork_builder{Url: l.GetImageUrl()}.Build(), libraryIcon(l.GetKind()), nil, width, width*9/16, b.Hovered() || focused)
		style.Text(ui.Text(c, l.GetName()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
	})
	if b.Clicked() {
		a.router.Push("/library/" + server + "/" + l.GetId())
	}
}
