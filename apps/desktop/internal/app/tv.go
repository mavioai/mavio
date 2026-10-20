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
	// A menu of the page closes as the page builds, as a choice is made.
	menuWasOpen := a.menuOpen()
	page := ui.Column(c).Fill().Background(style.Color(colors.Background))
	// Home's hero reaches the top of the screen, behind the bar.
	if bar && path != "/home" {
		page.Padding(tvBarHeight, 0, 0)
	}
	page.Children(func() {
		a.router.View(c, func(r *ui.Route) { a.page(c, r) })
	})
	if bar {
		a.tvBar(c)
	} else {
		a.userMenuOpen = false
	}
	a.tvKeyboardKeys(c)
	a.tvBack(c)
	a.focusKeys(c, a.menuOpen())
	// Focus returns to the menu's button as the menu closes.
	if menuWasOpen && !a.menuOpen() {
		a.focus.want = a.focus.menuButton
		c.After(0)
	}
}

// menuOpen tells whether a menu is open in the TV mode, where focus stays
// within it (docs/tv.md §5).
func (a *App) menuOpen() bool {
	return a.userMenuOpen || a.tvMenuOpen != ""
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
				a.tvAssistantButton(c)
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
	current := a.places.current == t.path
	b := ui.ButtonBase(c.Key(t.path)).Label(t.label).Selected(current)
	focused := a.tvFocus(c, b, current)
	pillLook(c, b, t.label, focused, current)
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

// pillLook dresses b as an element of a pill like the tabs: filled while
// current, white with black text with focus.
func pillLook(c *ui.Context, b ui.Element, label string, focused, current bool) {
	colors := style.Colors(c)
	b.Padding(10, 28).Radius(design.RadiusFull)
	fg := style.Color(colors.TextSecondary)
	switch {
	case focused:
		b.Background(ui.RGB(255, 255, 255))
		fg = ui.RGB(0, 0, 0)
	case current:
		b.Background(style.Color(colors.FillSelected))
		fg = style.Color(colors.TextPrimary)
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(fg).SingleLine()
	})
}

// tvPill builds a choice in a pill that looks like the tabs, as a
// library's Recommended and All: focus chooses without selecting, and
// focus entering it lands on the current one (docs/tv.md §6.3).
func (a *App) tvPill(c *ui.Context, labels []string, current *int) {
	colors := style.Colors(c)
	a.inChoice(func() {
		ui.Row(c).Shrink(0).Gap(4).Padding(6).Radius(design.RadiusFull).Background(style.Color(colors.SurfaceRaised).Alpha(0.6)).
			Role(ui.RoleTabList).Children(func() {
			for i, l := range labels {
				b := ui.ButtonBase(c.Key(i)).Label(l).Selected(i == *current)
				focused := a.tvFocus(c, b, i == *current)
				pillLook(c, b, l, focused, i == *current)
				if b.Clicked() || focused && i != *current {
					*current = i
				}
			}
		})
	})
}

// tvDots builds the user menu's button: three vertical dots in the tabs'
// glass, as the assistant's button; white with focus, which it shows not
// while its menu is open (docs/tv.md §3, §5).
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
	if focused {
		b.Background(ui.RGB(255, 255, 255))
		fg = ui.RGB(0, 0, 0)
	} else {
		b.Material(glass.Blur{Radius: 30})
	}
	b.Children(func() {
		inner := ui.Box(c).Size(48, 48).Radius(design.RadiusFull).Center()
		if !focused {
			inner.Background(style.Color(colors.SurfaceRaised).Alpha(0.6))
		}
		inner.Children(func() {
			ui.Icon(c, style.Icon("moreVertical", false)).Size(26, 26).TextColor(fg)
		})
	})
	if b.Clicked() {
		a.userMenuOpen = !a.userMenuOpen
	}
	if a.userMenuOpen {
		a.focus.menuButton = a.focus.dots
	}
	ui.PopoverBase(c, b, &a.userMenuOpen, func(panel ui.Element) {
		a.inRegion(regionMenu, func() { a.userMenu(c, panel) })
	})
}

// tvChoiceMenu builds a button opening a menu of choices of the app's own
// in the TV mode, as the web UI's (docs/tv.md §5): with icon and label,
// white with focus and filled with color.fill while its menu is open; the
// menu in frosted glass, 320 points wide, its items marking the current
// choice and opening with focus on it. Back closes it, focus returning to
// the button. It returns the choice made, -1 for none.
func (a *App) tvChoiceMenu(c *ui.Context, key, icon, label string, choices []string, current int) int {
	colors := style.Colors(c)
	open := a.tvMenuOpen == key
	b := ui.ButtonBase(c.Key("menu-"+key)).Height(48).Padding(0, 20).Gap(8).Radius(design.RadiusFull).AlignItems(ui.Center).Label(label)
	at := len(a.focus.items)
	fg := style.Color(colors.TextSecondary)
	switch {
	case a.tvFocus(c, b, false):
		b.Background(ui.RGB(255, 255, 255))
		fg = ui.RGB(0, 0, 0)
	case open:
		b.Background(style.Color(colors.Fill))
		fg = style.Color(colors.TextPrimary)
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(22, 22).TextColor(fg)
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		open = !open
	}
	if open {
		a.focus.menuButton = at
	}
	chosen := -1
	ui.PopoverBase(c, b, &open, func(panel ui.Element) {
		a.inRegion(regionMenu, func() {
			menuPanel(c, panel, 320, 0.45, 48, func() {
				for i, choice := range choices {
					it := ui.ButtonBase(c.Key(i)).MinHeight(48).Padding(6, 10).Gap(10).Radius(8).AlignItems(ui.Center).
						Label(choice).Role(ui.RoleMenuItemRadio).Checked(i == current)
					if i == current {
						it.AutoFocus()
					}
					if a.tvFocus(c, it, i == current) {
						it.Background(style.Color(colors.Fill))
					}
					it.Children(func() {
						ui.Box(c).Width(16).Shrink(0).Children(func() {
							if i == current {
								ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(colors.AccentText))
							}
						})
						style.Text(ui.Text(c, choice), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
					})
					if it.Clicked() {
						chosen, open = i, false
					}
				}
			})
		})
	})
	switch {
	case open:
		a.tvMenuOpen = key
	case a.tvMenuOpen == key:
		a.tvMenuOpen = ""
	}
	return chosen
}

// tvBack goes up a level with Escape and Backspace (docs/tv.md §4): on a
// tab's page from the page to its tab, the page returning to its top;
// from a tab to Home's; from any deeper page to where it was opened from.
// An open menu closes by itself.
func (a *App) tvBack(c *ui.Context) {
	back := c.Shortcut(0, ui.KeyEscape) || c.Shortcut(0, ui.KeyBackspace)
	if !back {
		return
	}
	if a.menuOpen() {
		// Back closes the menu, focus returning to its button.
		a.userMenuOpen, a.tvMenuOpen = false, ""
		return
	}
	f := &a.focus
	path := a.router.Path()
	if !a.places.isPlace(path) {
		a.goBack()
		return
	}
	if path == "/search" && a.searchState.query != "" {
		// Search's back deletes a character, and leaves only once there
		// is none (docs/tv.md §6.4).
		a.searchState.query = dropLast(a.searchState.query)
		return
	}
	if path == "/search" {
		// Search, which hides the tabs, returns to where it was opened
		// from (docs/tv.md §4).
		if a.places.at > 0 {
			a.router.Go(-1)
		} else {
			a.replace("/home")
		}
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
	tab := f.currentTab()
	if tab < 0 && path != "/home" {
		// A tab's page without its tab, as Discover with no server
		// discovering, leaves for Home.
		a.replace("/home")
		return
	}
	a.focusTo(c, tab)
}
