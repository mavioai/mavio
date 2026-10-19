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
	// choice is the choice the item is an element of, 0 for none.
	choice  int
	focused bool
	// id is the element's (ui.Element.ID); other tells an element that
	// focus moves to as well, of those MyGo reports, which the frame did
	// not make an item.
	id    uint64
	other bool
	// flow is the region in the page's flow the item is in, 0 for none,
	// and column its column there; marks are its marks (focusMarks).
	flow, column int
	marks        focusMarks
}

// focusMarks mark elements of a region in the page's flow, as the web
// UI's data attributes do (apps/web/src/shell/spatial.ts): beside, those
// left and right cross between its columns from and to; end, one at the
// end of what is above it, which up and down reach only from the last
// element of a row of its block; aside, one aside above a row, as
// Discover's Requests over its first row, which down passes, from the tabs
// as from above, and up reaches only from the row below's elements past
// its first, down from the tabs landing on that row's first; and block,
// the block of a column it is in, into which up and down land on the first
// element of a row.
type focusMarks struct {
	block              string
	beside, end, aside bool
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
	// choice is the choice being built, choices how many the frame has.
	choice, choices int
	// flow, column and marks are those of what is being built, flows and
	// columns how many the frame has.
	flow, column, flows, columns int
	marks                        focusMarks
	// reveal tells the item focused next to scroll itself into view, as
	// one of a grid does, which the page's scroll does not hold.
	reveal bool
	// want is the item the frame focuses: its place in the order, or
	// focusFirst for the page's first; -1 for none.
	want int
	// at is the item last focused, another element's place after the
	// items included, location the page it was on, and saved where focus
	// was on the pages left, by their locations, so that it returns there.
	at       int
	location string
	saved    map[string]int
	// last is where the item last focused was, while it is on the page
	// shown, so that focus whose element goes goes to the item nearest it.
	last    ui.Rect
	hasLast bool
	// leaving tells that the page being built goes away, whose elements
	// focus moves to none of.
	leaving bool
	// waited is how many frames focus that went waited for the items to
	// be laid out.
	waited int
	// frames is how many frames the page has been shown, the elements MyGo
	// reports being those of the frame before.
	frames int
	// initial is the item a page opens with focus on in place of its
	// first, -1 for none, and first the first that took focus as the page
	// opened in the frame being built; initialMark tells that what is
	// built is the initial item.
	initial, first int
	initialMark    bool
	// dots is the user menu's button, to which focus returns as the menu
	// closes.
	dots int
}

// beginFocus starts the frame's items.
func (a *App) beginFocus() {
	f := &a.focus
	f.items, f.region, f.view, f.scroll = f.items[:0], regionPage, ui.Rect{}, nil
	f.choice, f.choices = 0, 0
	f.flow, f.column, f.flows, f.columns, f.marks = 0, 0, 0, 0, focusMarks{}
	f.initial, f.first = -1, -1
}

// initialFocus builds what fn builds as the item the page opens with focus
// on in place of its first, as the details' Play (the web UI's
// data-tv-initial).
func (a *App) initialFocus(fn func()) {
	f := &a.focus
	f.initialMark = true
	fn()
	f.initialMark = false
}

// inFlow builds what fn builds as a region in the page's flow, as an
// item's details at the foot of its page (docs/tv.md §6.2): up and down
// leave it only from its first and last rows, to the row before or after
// it, landing on its first element as they come into it, and left and
// right keep within it.
func (a *App) inFlow(fn func()) {
	f := &a.focus
	f.flows++
	was := f.flow
	f.flow = f.flows
	fn()
	f.flow = was
}

// inColumn builds what fn builds as a column of a region in the page's
// flow, which has rows of its own while beside another.
func (a *App) inColumn(fn func()) {
	f := &a.focus
	f.columns++
	was := f.column
	f.column = f.columns
	fn()
	f.column = was
}

// marked builds what fn builds with the marks m besides those around it.
func (a *App) marked(m focusMarks, fn func()) {
	f := &a.focus
	was := f.marks
	if m.block != "" {
		f.marks.block = m.block
	}
	f.marks.beside = was.beside || m.beside
	f.marks.end = was.end || m.end
	f.marks.aside = was.aside || m.aside
	fn()
	f.marks = was
}

// inChoice builds what fn builds as one choice that focus makes, as a
// library's Recommended and All: focus entering it lands on its current
// element (docs/tv.md §4).
func (a *App) inChoice(fn func()) {
	f := &a.focus
	was := f.choice
	f.choices++
	f.choice = f.choices
	fn()
	f.choice = was
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
	f := &a.focus
	if !style.TV(c) || f.leaving {
		return false
	}
	i := len(f.items)
	el.Focusable().FocusRing(false)
	if f.initialMark && f.initial < 0 {
		f.initial = i
	}
	// A page opens on its first item, in a region of its own as well, as
	// Search's keyboard, past those aside.
	first := f.want == focusFirst && f.region != regionTabs && f.region != regionMenu && !f.marks.aside
	if first {
		f.first = i
	}
	if f.want == i || first {
		el.Focus()
		if f.reveal {
			el.ScrollIntoView()
		}
		f.want, f.reveal = -1, false
	}
	// While the user menu is open, focus is in it.
	focused := el.Focused() && (!a.userMenuOpen || f.region == regionMenu)
	if focused {
		f.at = i
	}
	f.items = append(f.items, focusItem{
		rect: el.Bounds(), region: f.region, view: f.view, scroll: f.scroll, current: current, choice: f.choice, focused: focused, id: el.ID(),
		flow: f.flow, column: f.column, marks: f.marks,
	})
	return focused
}

// tvFocusLit makes el an element focus moves to that turns white with
// focus, its content to be black, its corners rounded over a shadow, as a
// row of the settings or a person of the cast (docs/tv.md §4); it reports
// whether it has focus.
func (a *App) tvFocusLit(c *ui.Context, el ui.Element, current bool) bool {
	if !a.tvFocus(c, el, current) {
		return false
	}
	shadow := style.Color(style.Colors(c).TvFocusShadow)
	el.Background(ui.RGB(255, 255, 255)).Shadow(0, 8, 24, 0, shadow)
	return true
}

// tvLink makes el a link focus moves to in the TV mode, as the web UI's
// links: with focus it turns white, its content to be black, ringed in
// white and over a shadow, its corners rounded by radius, and its layout
// unchanged, so that nothing around it moves (docs/tv.md §4); it reports
// whether it has focus.
func (a *App) tvLink(c *ui.Context, el ui.Element, radius float32) bool {
	if !a.tvFocus(c, el, false) {
		return false
	}
	white := ui.RGB(255, 255, 255)
	el.Background(white).Radius(radius).
		Shadow(0, 8, 24, 0, style.Color(style.Colors(c).TvFocusShadow)).Shadow(0, 0, 0, 6, white)
	return true
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
	f.location, f.want, f.hasLast, f.frames = loc, focusFirst, false, 0
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
// element focus where nothing has it. Focus moves among the items and the
// other elements that take it alike, as a field of the settings, those the
// items do not name lying in the page; an arrow a field keeps, as its
// caret's, is its own, and in a dialog MyGo moves focus by rows itself
// (ui.Context.SpatialFocus).
func (a *App) focusKeys(c *ui.Context, menuOpen bool) {
	f := &a.focus
	c.SpatialFocus()
	if c.InDialog() {
		return
	}
	f.frames++
	if f.first >= 0 && f.initial > f.first {
		// The page opens with focus on its initial item in place of its
		// first, as the details' on Play.
		a.focusItem(c, f.initial)
		c.After(0)
		return
	}
	if f.want != -1 && f.frames < 2 {
		// The elements MyGo reports are still the page's before.
		c.After(0)
		return
	}
	a.addOthers(c)
	if f.want >= 0 && f.want < len(f.items) && f.items[f.want].other {
		// Back on a page, focus returns to the element it was on.
		a.focusItem(c, f.want)
		f.want, f.waited = -1, 0
		c.After(0)
		return
	}
	if f.want == focusFirst && f.frames >= 2 && !slices.ContainsFunc(f.items, func(it focusItem) bool { return it.region == regionPage && !it.other }) {
		// A page of no items, as a section of the settings, opens at its
		// first element (docs/tv.md §4).
		if i := slices.IndexFunc(f.items, func(it focusItem) bool { return it.other }); i >= 0 {
			a.focusItem(c, i)
			f.want = -1
			c.After(0)
			return
		}
	}
	if f.want >= len(f.items) {
		// The element it returns to is not laid out yet; failing it, the
		// page's first.
		if f.waited < 3 {
			f.waited++
		} else {
			f.want, f.waited = focusFirst, 0
		}
		c.After(0)
		return
	}
	at := f.focused()
	if at >= 0 {
		f.at = at
	}
	dir := ui.Key(0)
	if at >= 0 {
		// The menu keeps focus within itself.
		inMenu := f.items[at].region == regionMenu
		for _, k := range []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyLeft, ui.KeyRight} {
			if (inMenu || !c.KeepsArrow(k)) && c.Shortcut(0, k) {
				dir = k
			}
		}
	}
	if at >= 0 && f.items[at].rect.W > 0 {
		f.last, f.hasLast = f.items[at].rect, true
	}
	if at < 0 && c.HasFocus() {
		// On an element in no row, as one a popup of MyGo's holds.
		return
	}
	if at < 0 && f.want == -1 && !menuOpen && f.hasLast {
		// The element with focus went, as a page's part replaced: focus
		// goes to the item nearest where it was, once the items are laid
		// out, the frame after they appear.
		unlaid := slices.ContainsFunc(f.items, func(it focusItem) bool { return it.rect.W == 0 })
		if f.waited < 2 && unlaid {
			f.waited++
		} else if i := f.nearestTo(f.last); i >= 0 {
			a.focusItem(c, i)
			f.waited = 0
		}
		c.After(0)
		return
	}
	if at < 0 {
		if f.want == -1 && !menuOpen && len(f.items) > 0 {
			switch {
			case slices.ContainsFunc(f.items, func(it focusItem) bool { return it.region == regionPage && !it.other }):
				f.want = focusFirst
			case slices.ContainsFunc(f.items, func(it focusItem) bool { return it.other }):
				// A page of no items, as a section of the settings, at its
				// first element.
				a.focusItem(c, slices.IndexFunc(f.items, func(it focusItem) bool { return it.other }))
			default:
				f.want = 0
			}
			c.After(0)
		}
		return
	}
	if dir == 0 {
		return
	}
	to := f.enter(at, f.step(at, dir))
	if to < 0 {
		// Where focus cannot move, the element shakes (docs/tv.md §4).
		c.ShakeFocus()
		return
	}
	a.focusItem(c, to)
	c.After(0)
}

// addOthers adds the elements that take focus which the frame made no
// items, as fields and buttons of the settings, as items of the page.
func (a *App) addOthers(c *ui.Context) {
	f := &a.focus
	items := make(map[uint64]bool, len(f.items))
	for _, it := range f.items {
		items[it.id] = true
	}
	for _, e := range c.FocusableElements() {
		if items[e.ID] {
			continue
		}
		it := focusItem{rect: e.Bounds, region: regionPage, id: e.ID, other: true, focused: e.Focused, view: e.Shown}
		if e.Shown.W == 0 || e.Shown.H == 0 {
			// Wholly out of view, as a card a row scrolled away.
			it.view = ui.Rect{X: e.Bounds.X - 2, W: 1, H: 1}
		}
		f.items = append(f.items, it)
	}
}

// focusItem gives focus to the item at i, scrolled into view: an item in
// the next frame, another element at once.
func (a *App) focusItem(c *ui.Context, i int) {
	f := &a.focus
	if i < 0 || i >= len(f.items) {
		return
	}
	it := f.items[i]
	if !it.other {
		f.want = i
		a.revealItem(c, it)
		return
	}
	c.FocusID(it.id)
	// The page stops 200 points below the window's top, and 120 above its
	// bottom, as for an item; elsewhere, as in the settings' scroll, it
	// scrolls only to show it.
	if _, ok := a.scrolls["page:"+a.pagePath]; ok && it.region == regionPage {
		a.revealItem(c, it)
	} else {
		c.RevealID(it.id)
	}
}

// revealItem scrolls the page and the item's row so that the item shows,
// the page stopping 200 points below the window's top, under the tabs,
// and 120 above its bottom (docs/tv.md §4); an item within the page's
// first screen returns the page to its top.
func (a *App) revealItem(c *ui.Context, it focusItem) {
	if s := it.scroll; s != nil && it.view.W > 0 {
		// The row keeps the page's margin beside the item, as the web UI's
		// scroll-padding.
		room := style.Layout(c).Margin
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
		a.focus.reveal = true
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

// enter returns where focus entering the item to from at lands: on its
// choice's current element, unless it comes from within the choice.
func (f *focusEngine) enter(at, to int) int {
	if to < 0 || f.items[to].choice == 0 || f.items[to].choice == f.items[at].choice {
		return to
	}
	if i := slices.IndexFunc(f.items, func(it focusItem) bool { return it.choice == f.items[to].choice && it.current }); i >= 0 {
		return i
	}
	return to
}

// nearestTo returns the item laid out nearest r, -1 for none.
func (f *focusEngine) nearestTo(r ui.Rect) int {
	best, d := -1, float32(math.MaxFloat32)
	for i, it := range f.items {
		if it.rect.W == 0 || it.region == regionMenu {
			continue
		}
		dx, dy := it.rect.X+it.rect.W/2-(r.X+r.W/2), it.rect.Y+it.rect.H/2-(r.Y+r.H/2)
		if e := dx*dx + dy*dy; e < d {
			best, d = i, e
		}
	}
	return best
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

// sameRegion reports whether two items lie in one region: of the same
// name and in the same region of the page's flow, or in none.
func sameRegion(a, b focusItem) bool {
	return a.region == b.region && a.flow == b.flow
}

// columnsOf returns the column of a region of the page's flow each item
// is in while that column lies beside another of the region, 0 where not
// (columnOf in the web UI's spatial.ts).
func (f *focusEngine) columnsOf() []int {
	type key struct{ flow, column int }
	bounds := map[key]ui.Rect{}
	for _, it := range f.items {
		if it.flow == 0 || it.column == 0 || it.rect.W == 0 {
			continue
		}
		k := key{it.flow, it.column}
		r, ok := bounds[k]
		if !ok {
			bounds[k] = it.rect
			continue
		}
		x, right := min(r.X, it.rect.X), max(r.X+r.W, it.rect.X+it.rect.W)
		y, bottom := min(r.Y, it.rect.Y), max(r.Y+r.H, it.rect.Y+it.rect.H)
		bounds[k] = ui.Rect{X: x, Y: y, W: right - x, H: bottom - y}
	}
	cols := make([]int, len(f.items))
	for i, it := range f.items {
		r, ok := bounds[key{it.flow, it.column}]
		if !ok {
			continue
		}
		for k, o := range bounds {
			if k.flow == it.flow && k.column != it.column && (o.X >= r.X+r.W || o.X+o.W <= r.X) {
				cols[i] = it.column
				break
			}
		}
	}
	return cols
}

// acrossFlow returns where up or down goes from row across a region of
// the page's flow (acrossFlow in the web UI's spatial.ts): the first item
// in view of what follows the row in the page, or of the last row of what
// precedes it, in its first column where a region of the flow; the
// nearest across x of that row where not; -1 where neither side is such a
// region. A region of the flow is left whole, past its other columns.
func (f *focusEngine) acrossFlow(row []int, down bool, x float32, cols []int) int {
	from := f.items[row[0]]
	leaving := from.flow != 0
	lo, hi := slices.Min(row), slices.Max(row)
	var page []int
	for i, it := range f.items {
		if it.rect.W == 0 || it.rect.H == 0 || it.region == regionTabs || it.region == regionMenu || leaving && sameRegion(it, from) {
			continue
		}
		if down && i > hi || !down && i < lo {
			page = append(page, i)
		}
	}
	if len(page) == 0 {
		return -1
	}
	next := page[0]
	if !down {
		next = page[len(page)-1]
	}
	to := f.items[next]
	if sameRegion(to, from) || to.flow == 0 && !leaving {
		return -1
	}
	if down {
		in := slices.DeleteFunc(page, func(i int) bool { return !sameRegion(f.items[i], to) || cols[i] != cols[next] })
		return f.endOf(f.rowsOf(in)[0], false)
	}
	into := slices.DeleteFunc(page, func(i int) bool { return !sameRegion(f.items[i], to) })
	if to.flow == 0 {
		rows := f.rowsOf(into)
		return f.closestTo(rows[len(rows)-1], x)
	}
	first := cols[into[0]]
	rows := f.rowsOf(slices.DeleteFunc(into, func(i int) bool { return cols[i] != first }))
	return f.endOf(rows[len(rows)-1], false)
}

// step returns the item focus moves to from at in the direction dir, -1
// for none (docs/tv.md §4), as the web UI's (apps/web/src/shell/spatial.ts):
// up and down to the row above or below, to its item nearest across, or
// its first where in another block; left and right along the row, and
// from its ends into a region beside, else to the last item in view of
// the row above or the first of the row below. Above the page's first row
// are the tabs, at the current one; the menu keeps focus within itself. A
// region of the page's flow is left by up and down only from its first
// and last rows, its columns crossed only from and to what is marked
// beside, and what is marked at an end reached only from a row's last.
func (f *focusEngine) step(at int, dir ui.Key) int {
	from := f.items[at]
	region := from.region
	cols := f.columnsOf()
	var mine, page, others []int
	for i, it := range f.items {
		if it.rect.W == 0 || it.rect.H == 0 {
			continue
		}
		switch {
		case sameRegion(it, from):
			if cols[i] == cols[at] {
				mine = append(mine, i)
			}
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
	block := from.marks.block
	// The row above or below, past what only the last item of a row of its
	// block reaches.
	rowAt := func(d int) []int {
		for j := ri + d; j >= 0 && j < len(rows); j += d {
			r := slices.DeleteFunc(slices.Clone(rows[j]), func(k int) bool {
				m := f.items[k].marks
				return m.end && (i != len(row)-1 || m.block != block) || m.aside && (d != -1 || i == 0 || j != ri-1)
			})
			if len(r) > 0 {
				return r
			}
		}
		return nil
	}
	// Into another block, focus lands on the row's first item.
	to := func(r []int) int {
		if f.items[r[0]].marks.block != block {
			return f.endOf(r, false)
		}
		return f.closestTo(r, x)
	}
	previous, below := rowAt(-1), rowAt(1)
	inTabs := region == regionTabs
	aside := false
	if inTabs {
		below = nil
		page = slices.DeleteFunc(page, func(k int) bool {
			aside = aside || f.items[k].marks.aside
			return f.items[k].marks.aside
		})
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
	inFlow := !inTabs && from.flow != 0
	across := func(down bool, next []int) int {
		if inTabs || region == regionMenu || inFlow && next != nil {
			return -1
		}
		return f.acrossFlow(row, down, x, cols)
	}
	switch dir {
	case ui.KeyUp:
		// A row of the page goes first to a region of the flow between it
		// and the row above.
		if k := across(false, previous); k >= 0 {
			return k
		}
		if previous != nil {
			return to(previous)
		}
		return above()
	case ui.KeyDown:
		if k := across(true, below); k >= 0 {
			return k
		}
		switch {
		case below == nil:
			return -1
		case inTabs && aside:
			return f.endOf(below, false)
		case inTabs:
			return f.closestTo(below, x)
		}
		return to(below)
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
	if inFlow {
		if from.marks.beside {
			var other []int
			for k, it := range f.items {
				if it.rect.W != 0 && sameRegion(it, from) && cols[k] != cols[at] && it.marks.beside {
					other = append(other, k)
				}
			}
			if k := f.nearest(from.rect, dir, other); k >= 0 {
				return k
			}
		}
		switch {
		case right && below != nil:
			return f.endOf(below, false)
		case !right && previous != nil:
			return f.endOf(previous, true)
		}
		return -1
	}
	if beside := f.nearest(from.rect, dir, others); beside >= 0 {
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
