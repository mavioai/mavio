package app

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// Focus regions of the TV mode: the tabs, the page, and the user menu,
// within which focus keeps while it is open. A page may name regions of
// its own, as a part beside another.
const (
	regionTabs = "tabs"
	regionPage = "page"
	regionMenu = "menu"
)

// focusFirst is the item a page opens with focus on: its first.
const focusFirst = -2

// focusItem is an element focus moves to in the TV mode, as the frame
// before laid it out.
type focusItem struct {
	rect   ui.Rect
	region string
	// view is the viewport of the row that scrolls it sideways, and scroll
	// that row's scroll, both zero where nothing does.
	view   ui.Rect
	scroll *ui.ScrollState
	// current tells the current tab, or the chosen element of a choice.
	current bool
	focused bool
}

// focusEngine moves focus with the arrow keys by rows in the TV mode, as
// a remote moves it on a television (docs/tv.md §4), as the web UI's does
// (apps/web/src/shell/spatial.ts). The elements focus moves to tell it
// where they are as each frame builds them, in the same order from frame
// to frame; a move names the one to focus by its place in that order, and
// the next frame focuses it.
type focusEngine struct {
	items  []focusItem
	region string
	view   ui.Rect
	scroll *ui.ScrollState
	// want is the item the frame focuses: its place in the order, or
	// focusFirst for the page's first; -1 for none.
	want int
	// at is the item last focused, location the page it was on, and
	// saved where focus was on the pages left, by their locations, so that
	// it returns there.
	at       int
	location string
	saved    map[string]int
	// rowFocused tells that the row being built has focus.
	rowFocused bool
	// dots is the user menu's button, to which focus returns as the menu
	// closes.
	dots int
}

// beginFocus starts the frame's items.
func (a *App) beginFocus() {
	f := &a.focus
	f.items, f.region, f.view, f.scroll = f.items[:0], regionPage, ui.Rect{}, nil
}

// inRegion builds what fn builds in the focus region name.
func (a *App) inRegion(name string, fn func()) {
	f := &a.focus
	was := f.region
	f.region = name
	fn()
	f.region = was
}

// inScrollRow builds what fn builds in a row that scroll scrolls sideways
// within view.
func (a *App) inScrollRow(view ui.Rect, scroll *ui.ScrollState, fn func()) {
	f := &a.focus
	wasView, wasScroll := f.view, f.scroll
	f.view, f.scroll = view, scroll
	fn()
	f.view, f.scroll = wasView, wasScroll
}

// tvFocus makes el an element focus moves to in the TV mode, current when
// it is the current tab or the chosen element of a choice, and reports
// whether it has focus; outside the TV mode it does nothing and reports
// false.
func (a *App) tvFocus(c *ui.Context, el ui.Element, current bool) bool {
	if !style.TV(c) {
		return false
	}
	f := &a.focus
	i := len(f.items)
	el.Focusable().FocusRing(false)
	if f.want == i || f.want == focusFirst && f.region == regionPage {
		el.Focus()
		f.want = -1
	}
	// While the user menu is open, focus is in it.
	focused := el.Focused() && (!a.userMenuOpen || f.region == regionMenu)
	if focused {
		f.at = i
	}
	f.items = append(f.items, focusItem{rect: el.Bounds(), region: f.region, view: f.view, scroll: f.scroll, current: current, focused: focused})
	return focused
}

// focusPage starts focus on the page shown when it changes: where it was
// when the page was left, else on its first item.
func (a *App) focusPage() {
	f := &a.focus
	loc := a.router.Location()
	if loc == f.location {
		return
	}
	if f.saved == nil {
		f.saved = map[string]int{}
	}
	if f.location != "" {
		f.saved[f.location] = f.at
	}
	f.location, f.want = loc, focusFirst
	if i, ok := f.saved[loc]; ok {
		f.want = i
	}
	// Only the pages the router keeps are returned to.
	if len(f.saved) > maxHistory {
		clear(f.saved)
	}
}

// focused returns the item with focus, -1 for none.
func (f *focusEngine) focused() int {
	return slices.IndexFunc(f.items, func(it focusItem) bool { return it.focused })
}

// focusKeys moves focus with the arrow keys, and gives the page's first
// item focus where nothing has it.
func (a *App) focusKeys(c *ui.Context, menuOpen bool) {
	f := &a.focus
	dir := ui.Key(0)
	for _, k := range []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyLeft, ui.KeyRight} {
		if c.Shortcut(0, k) {
			dir = k
		}
	}
	at := f.focused()
	if at < 0 {
		if f.want == -1 && !menuOpen && len(f.items) > 0 {
			f.want = focusFirst
			if !slices.ContainsFunc(f.items, func(it focusItem) bool { return it.region == regionPage }) {
				f.want = 0
			}
			c.After(0)
		}
		return
	}
	if dir == 0 {
		return
	}
	to := f.step(at, dir)
	if to < 0 {
		return
	}
	f.want = to
	a.revealItem(c, f.items[to])
	c.After(0)
}

// revealItem scrolls the page and the item's row so that the item shows,
// the page stopping 200 points below the window's top, under the tabs,
// and 120 above its bottom (docs/tv.md §4); an item within the page's
// first screen returns the page to its top.
func (a *App) revealItem(c *ui.Context, it focusItem) {
	if s := it.scroll; s != nil && it.view.W > 0 {
		const room = 24
		r, v := it.rect, it.view
		switch {
		case r.X < v.X+room:
			s.ScrollTo(max(s.X-(v.X+room-r.X), 0), 0, ui.Smooth)
		case r.X+r.W > v.X+v.W-room:
			s.ScrollTo(min(s.X+(r.X+r.W-(v.X+v.W-room)), s.MaxX), 0, ui.Smooth)
		}
	}
	if it.region != regionPage {
		return
	}
	page, ok := a.scrolls["page:"+a.pagePath]
	if !ok {
		return
	}
	_, h := c.Size()
	top, bottom := float32(200), h-120
	switch r := it.rect; {
	case r.Y+r.H+page.Y <= h:
		page.ScrollTo(0, 0, ui.Smooth)
	case r.Y < top:
		page.ScrollTo(0, max(page.Y-(top-r.Y), 0), ui.Smooth)
	case r.Y+r.H > bottom:
		page.ScrollTo(0, min(page.Y+(r.Y+r.H-bottom), page.MaxY), ui.Smooth)
	}
}

// focusTo gives focus to the item at i in the next frame.
func (a *App) focusTo(c *ui.Context, i int) {
	if i < 0 || i >= len(a.focus.items) {
		return
	}
	a.focus.want = i
	a.revealItem(c, a.focus.items[i])
	c.After(0)
}

// currentTab returns the current tab's item, -1 for none.
func (f *focusEngine) currentTab() int {
	return slices.IndexFunc(f.items, func(it focusItem) bool { return it.region == regionTabs && it.current })
}

// sideBySide reports whether two items lie in one row: overlapping
// vertically by at least half the smaller one's height.
func sideBySide(a, b ui.Rect) bool {
	return min(a.Y+a.H, b.Y+b.H)-max(a.Y, b.Y) >= min(a.H, b.H)/2
}

// rowsOf returns the rows the items at idx form, from the top, each from
// left to right, those its row scrolls out of view included.
func (f *focusEngine) rowsOf(idx []int) [][]int {
	idx = slices.Clone(idx)
	slices.SortStableFunc(idx, func(i, j int) int {
		a, b := f.items[i].rect, f.items[j].rect
		if a.Y != b.Y {
			return cmpFloat(a.Y, b.Y)
		}
		return cmpFloat(a.X, b.X)
	})
	var rows [][]int
	for _, i := range idx {
		k := slices.IndexFunc(rows, func(row []int) bool { return sideBySide(f.items[row[0]].rect, f.items[i].rect) })
		if k < 0 {
			rows = append(rows, []int{i})
			continue
		}
		rows[k] = append(rows[k], i)
	}
	for _, row := range rows {
		slices.SortStableFunc(row, func(i, j int) int { return cmpFloat(f.items[i].rect.X, f.items[j].rect.X) })
	}
	return rows
}

func cmpFloat(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// shown reports whether an item is in view across its row.
func (f *focusEngine) shown(i int) bool {
	it := f.items[i]
	if it.view.W == 0 {
		return true
	}
	x := it.rect.X + it.rect.W/2
	return x >= it.view.X && x <= it.view.X+it.view.W
}

// inView returns the items of row in view, else all of them.
func (f *focusEngine) inView(row []int) []int {
	if pool := slices.DeleteFunc(slices.Clone(row), func(i int) bool { return !f.shown(i) }); len(pool) > 0 {
		return pool
	}
	return row
}

// closestTo returns the item of row in view nearest x across.
func (f *focusEngine) closestTo(row []int, x float32) int {
	best, d := -1, float32(math.MaxFloat32)
	for _, i := range f.inView(row) {
		r := f.items[i].rect
		if e := abs(r.X + r.W/2 - x); e < d {
			best, d = i, e
		}
	}
	return best
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// endOf returns a row's first or last item in view.
func (f *focusEngine) endOf(row []int, last bool) int {
	pool := f.inView(row)
	if last {
		return pool[len(pool)-1]
	}
	return pool[0]
}

// nearest returns, of the items at idx wholly beyond from's edge in the
// direction dir, the one with the least distance along it plus twice its
// offset across it, -1 for none.
func (f *focusEngine) nearest(from ui.Rect, dir ui.Key, idx []int) int {
	cx, cy := from.X+from.W/2, from.Y+from.H/2
	best, score := -1, float32(math.MaxFloat32)
	for _, i := range idx {
		r := f.items[i].rect
		var along, across float32
		switch dir {
		case ui.KeyRight:
			along, across = r.X-(from.X+from.W), abs(r.Y+r.H/2-cy)
		case ui.KeyLeft:
			along, across = from.X-(r.X+r.W), abs(r.Y+r.H/2-cy)
		case ui.KeyDown:
			along, across = r.Y-(from.Y+from.H), abs(r.X+r.W/2-cx)
		case ui.KeyUp:
			along, across = from.Y-(r.Y+r.H), abs(r.X+r.W/2-cx)
		}
		if along < -1 {
			continue
		}
		if s := along + across*2; s < score {
			best, score = i, s
		}
	}
	return best
}

// step returns the item focus moves to from at in the direction dir, -1
// for none (docs/tv.md §4): up and down to the row above or below, to its
// item nearest across; left and right along the row, and from its ends
// into a region beside, else to the last item in view of the row above or
// the first of the row below. Above the page's first row are the tabs, at
// the current one; the menu keeps focus within itself.
func (f *focusEngine) step(at int, dir ui.Key) int {
	from := f.items[at]
	region := from.region
	var mine, page, others []int
	for i, it := range f.items {
		if it.rect.W == 0 || it.rect.H == 0 {
			continue
		}
		switch {
		case it.region == region:
			mine = append(mine, i)
		case region != regionMenu && it.region != regionMenu && it.region != regionTabs:
			others = append(others, i)
		}
		if it.region == regionPage {
			page = append(page, i)
		}
	}
	rows := f.rowsOf(mine)
	ri := slices.IndexFunc(rows, func(row []int) bool { return slices.Contains(row, at) })
	if ri < 0 {
		return -1
	}
	row := rows[ri]
	i := slices.Index(row, at)
	x := from.rect.X + from.rect.W/2
	var previous, below []int
	if ri > 0 {
		previous = rows[ri-1]
	}
	if ri+1 < len(rows) {
		below = rows[ri+1]
	}
	inTabs := region == regionTabs
	if inTabs {
		below = nil
		if r := f.rowsOf(page); len(r) > 0 {
			below = r[0]
		}
	}
	above := func() int {
		if region == regionMenu || inTabs {
			return -1
		}
		return f.currentTab()
	}
	switch dir {
	case ui.KeyUp:
		if previous != nil {
			return f.closestTo(previous, x)
		}
		return above()
	case ui.KeyDown:
		if below != nil {
			return f.closestTo(below, x)
		}
		return -1
	}
	right := dir == ui.KeyRight
	if j := i + 1; right && j < len(row) {
		return row[j]
	}
	if j := i - 1; !right && j >= 0 {
		return row[j]
	}
	if inTabs || region == regionMenu {
		return -1
	}
	if beside := f.nearest(from.rect, dir, slices.DeleteFunc(others, func(k int) bool { return f.items[k].region == region })); beside >= 0 {
		return beside
	}
	if right {
		if below != nil {
			return f.endOf(below, false)
		}
		return -1
	}
	if previous != nil {
		return f.endOf(previous, true)
	}
	return above()
}
