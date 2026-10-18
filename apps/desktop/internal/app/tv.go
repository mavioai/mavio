package app

import (
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// tvBarHeight is the height of the TV mode's navigation bar, below which
// the pages that show it start.
const tvBarHeight = 128

// tvTab is a tab of the TV mode's navigation bar.
type tvTab struct{ path, label string }

// tvTabs returns the tabs: Home, Libraries, Discover where a server
// discovers, and Search (docs/tv.md §3).
func (a *App) tvTabs(c *ui.Context) []tvTab {
	tabs := []tvTab{{"/home", style.T(c, "nav.home")}, {"/libraries", style.T(c, "nav.libraries")}}
	if server, _, _ := a.discovery(c); server != "" {
		tabs = append(tabs, tvTab{"/discover", style.T(c, "nav.discover")})
	}
	return append(tabs, tvTab{"/search", style.T(c, "nav.search")})
}

// tvShowsBar reports whether the page at path shows the navigation bar:
// a title's details, a discovered title, a library, a catalog and Search
// hide it (docs/tv.md §3).
func tvShowsBar(path string) bool {
	if path == "/search" {
		return false
	}
	for _, p := range []string{"/item/", "/title/", "/library/", "/catalog/", "/discover/"} {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	return true
}

// tvShell builds the TV mode's layout (docs/tv.md): the page on the
// screen's whole width, under the navigation bar where it shows it, with
// focus moved by the arrow keys and back going up a level.
func (a *App) tvShell(c *ui.Context) {
	colors := style.Colors(c)
	a.contentWidth, _ = c.Size()
	a.sidebarOpen = false
	a.visitPlaces()
	a.focusPage()
	a.beginFocus()
	path := a.router.Path()
	bar := tvShowsBar(path)
	page := ui.Column(c).Fill().Background(style.Color(colors.Background))
	// Home's hero reaches the top of the screen, behind the bar.
	if bar && path != "/home" {
		page.Padding(tvBarHeight, 0, 0)
	}
	page.Children(func() {
		a.router.View(c, func(r *ui.Route) { a.page(c, r) })
	})
	menuWasOpen := a.userMenuOpen
	if bar {
		a.tvBar(c)
	} else {
		a.userMenuOpen = false
	}
	a.tvBack(c)
	a.focusKeys(c, a.userMenuOpen)
	// Focus returns to the menu's button as the menu closes.
	if menuWasOpen && !a.userMenuOpen {
		a.focus.want = a.focus.dots
		c.After(0)
	}
}

// tvBar builds the navigation bar: the tabs in a pill centered at the
// top, and the user menu's button at the screen's right, over a band of
// the background once the page scrolls.
func (a *App) tvBar(c *ui.Context) {
	colors := style.Colors(c)
	path := a.router.Path()
	tabs := a.tvTabs(c)
	onTab := false
	for _, t := range tabs {
		onTab = onTab || t.path == path
	}
	band := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(tvBarHeight).Padding(32, style.Layout(c).Margin, 24).
		AlignItems(ui.Center).Justify(ui.Center).DragWindow()
	if s, ok := a.scrolls["page:"+a.pagePath]; ok && s.Y > 0 && path != "/home" || path == "/home" && a.scrolledHome() {
		band.Background(style.Color(colors.Background))
	}
	band.Children(func() {
		a.inRegion(regionTabs, func() {
			pill := ui.Row(c).Gap(4).Radius(design.RadiusFull).Material(glass.Blur{Radius: 30}).Label(style.T(c, "nav.navigation"))
			pill.Children(func() {
				ui.Row(c).Gap(4).Padding(6).Radius(design.RadiusFull).Background(style.Color(colors.SurfaceRaised).Alpha(0.6)).Children(func() {
					for _, t := range tabs {
						a.tvTabButton(c, t, path, onTab)
					}
				})
			})
			ui.Row(c).Absolute().Top(42).Right(style.Layout(c).Margin).Gap(8).AlignItems(ui.Center).Children(func() {
				a.tvDots(c)
			})
		})
	})
}

// scrolledHome reports whether Home has scrolled from its top.
func (a *App) scrolledHome() bool {
	s, ok := a.scrolls["page:/home"]
	return ok && s.Y > 0
}

// tvTabButton builds a tab: filled while current, white with focus. On a
// tab's own page, focus on another tab opens it in its place; deeper pages
// open a tab only when it is selected, and Search opens only so.
func (a *App) tvTabButton(c *ui.Context, t tvTab, path string, onTab bool) {
	colors := style.Colors(c)
	current := a.places.current == t.path
	b := ui.ButtonBase(c.Key(t.path)).Padding(10, 28).Radius(design.RadiusFull).Label(t.label).Selected(current)
	focused := a.tvFocus(c, b, current)
	fg := colors.TextSecondary
	switch {
	case focused:
		b.Background(ui.RGB(255, 255, 255))
	case current:
		b.Background(style.Color(colors.FillSelected))
		fg = colors.TextPrimary
	}
	b.Children(func() {
		text := style.Text(ui.Text(c, t.label), style.Type(c).Headline).TextColor(style.Color(fg)).SingleLine()
		if focused {
			text.TextColor(ui.RGB(0, 0, 0))
		}
	})
	switch {
	case b.Clicked():
		a.openPlace(t.path)
	case focused && onTab && t.path != path && t.path != "/search":
		a.replace(t.path)
		a.places.current = t.path
		// Focus stays on the tab rather than going to the page.
		a.focus.location = a.router.Location()
	}
}

// tvDots builds the user menu's button: three vertical dots, bare, and in
// the tabs' glass with focus or while its menu is open; white with focus
// (docs/tv.md §3, §5).
func (a *App) tvDots(c *ui.Context) {
	if len(a.servers) == 0 {
		return
	}
	colors := style.Colors(c)
	name, _ := a.accountName(c)
	label := style.T(c, "nav.userMenu", "name", name)
	b := ui.ButtonBase(c.Key("tv-dots")).Size(48, 48).Radius(design.RadiusFull).Label(label)
	a.focus.dots = len(a.focus.items)
	focused := a.tvFocus(c, b, false) && !a.userMenuOpen
	fg := style.Color(colors.TextSecondary)
	switch {
	case focused:
		b.Background(ui.RGB(255, 255, 255))
		fg = ui.RGB(0, 0, 0)
	case a.userMenuOpen:
		b.Material(glass.Blur{Radius: 30})
	}
	b.Children(func() {
		inner := ui.Box(c).Size(48, 48).Radius(design.RadiusFull).Center()
		if !focused && a.userMenuOpen {
			inner.Background(style.Color(colors.SurfaceRaised).Alpha(0.6))
		}
		inner.Children(func() {
			ui.Icon(c, style.Icon("moreVertical", false)).Size(26, 26).TextColor(fg)
		})
	})
	if b.Clicked() {
		a.userMenuOpen = !a.userMenuOpen
	}
	ui.PopoverBase(c, b, &a.userMenuOpen, func(panel ui.Element) {
		a.inRegion(regionMenu, func() { a.userMenu(c, panel) })
	})
}

// tvBack goes up a level with Escape and Backspace (docs/tv.md §4): on a
// tab's page from the page to its tab, the page returning to its top;
// from a tab to Home's; from any deeper page to where it was opened from.
// An open menu closes by itself.
func (a *App) tvBack(c *ui.Context) {
	back := c.Shortcut(0, ui.KeyEscape) || c.Shortcut(0, ui.KeyBackspace)
	if a.userMenuOpen || !back {
		return
	}
	f := &a.focus
	path := a.router.Path()
	if !a.places.isPlace(path) {
		a.goBack()
		return
	}
	at := f.focused()
	if at >= 0 && f.items[at].region == regionTabs {
		if path != "/home" {
			a.openPlace("/home")
		}
		return
	}
	if s, ok := a.scrolls["page:"+a.pagePath]; ok {
		s.ScrollTo(0, 0, ui.Smooth)
	}
	for k, s := range a.scrolls {
		if !strings.HasPrefix(k, "page:") {
			s.ScrollTo(0, 0, ui.Smooth)
		}
	}
	if tab := f.currentTab(); tab >= 0 {
		a.focusTo(c, tab)
	}
}
