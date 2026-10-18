package app

import (
	"context"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// shell builds the desktop layout: the sidebar, or its rail, beside the
// page shown.
func (a *App) shell(c *ui.Context) {
	w, _ := c.Size()
	medium := w < design.BreakpointExpanded
	expanded := !medium && !a.prefs.SidebarCollapsed
	if !medium {
		a.sidebarOpen = false
	}
	a.contentWidth = w - style.Layout.Rail
	if expanded {
		a.contentWidth = w - a.sidebarWidth()
	}
	colors := style.Colors(c)
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	a.visitPlaces()
	path := a.router.Path()
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		bar := ui.Column(c).Shrink(0)
		if expanded {
			bar.Width(a.sidebarWidth())
		}
		if !c.Vibrancy() {
			bar.Background(style.Color(colors.Sidebar))
		}
		bar.Children(func() {
			if expanded {
				a.sidebar(c, medium)
				a.sidebarResizer(c)
			} else {
				a.rail(c, medium)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Background(style.Color(colors.Background)).Children(func() {
			a.router.View(c, func(r *ui.Route) { a.page(c, r) })
			a.backButton(c)
		})
	})
	if a.sidebarOpen {
		scrim := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(style.Color(colors.Scrim).Alpha(0.3))
		if scrim.Clicked() || c.Shortcut(0, ui.KeyEscape) {
			a.sidebarOpen = false
		}
		ui.Column(c).Absolute().Top(0).Left(0).Bottom(0).Background(style.Color(colors.Sidebar)).
			Shadow(0, 12, 32, 0, style.Color(design.ElevationLevel3.Color)).
			Children(func() { a.sidebar(c, medium) })
	}
	if a.router.Path() != path {
		a.sidebarOpen = false
	}
}

// sidebarWidths are the widths the sidebar's edge is dragged between.
var sidebarWidths = struct{ min, max float32 }{200, 400}

// sidebarWidth returns the width of the expanded sidebar: the one chosen
// by dragging its edge, else the layout's.
func (a *App) sidebarWidth() float32 {
	if w := a.prefs.SidebarWidth; w > 0 {
		return min(max(w, sidebarWidths.min), sidebarWidths.max)
	}
	return style.Layout.Sidebar
}

// sidebarResizer builds the right edge of an expanded sidebar, dragged or
// moved with the arrow keys to widen or narrow it within sidebarWidths,
// and double-clicked to return it to the layout's width. The width is
// kept, and shared by the app's sidebar and the settings'.
func (a *App) sidebarResizer(c *ui.Context) {
	colors := style.Colors(c)
	edge := ui.Box(c.Key("sidebar-resizer")).Absolute().Top(0).Bottom(0).Right(0).Width(6).
		Cursor(ui.CursorResizeEW).Focusable().FocusRing(false).Role(ui.RoleSplitter).
		Label(style.T(c, "nav.resizeSidebar"))
	resize := func(w float32) {
		a.prefs.SidebarWidth = float32(math.Round(float64(min(max(w, sidebarWidths.min), sidebarWidths.max))))
	}
	if dx, _, ok := edge.Dragged(); ok {
		a.sidebarResizing = true
		if dx != 0 {
			resize(a.sidebarWidth() + dx)
		}
	} else if a.sidebarResizing && !edge.Dragging() {
		a.sidebarResizing = false
		a.savePreferences()
	}
	if edge.DoubleClicked() {
		a.prefs.SidebarWidth = 0
		a.savePreferences()
	}
	for key, step := range map[ui.Key]float32{ui.KeyLeft: -10, ui.KeyRight: 10} {
		if edge.Shortcut(0, key) {
			resize(a.sidebarWidth() + step)
			a.savePreferences()
		}
	}
	if edge.Hovered() || edge.Dragging() || edge.Focused() {
		edge.Children(func() {
			ui.Box(c).Absolute().Top(0).Bottom(0).Right(0).Width(2).Background(style.Color(colors.Accent))
		})
	}
}

// page builds the page at the router's path.
func (a *App) page(c *ui.Context, r *ui.Route) {
	a.pagePath = r.Path()
	switch {
	case r.Match("/home"):
		r.Title(style.T(c, "nav.home"))
		a.home(c)
	case r.Match("/search"):
		r.Title(style.T(c, "nav.search"))
		a.search(c)
	case r.Match("/library/{server}/{id}"):
		a.library(c, r)
	case r.Match("/hub/{server}/{hub}"):
		a.hubPage(c, r)
	case r.Match("/item/{server}/{id}"):
		a.details(c, r, r.Param("server"), r.Param("id"))
	case r.Match("/credits/{server}/{id}"):
		a.creditsPage(c, r, r.Param("server"), r.Param("id"), false)
	case r.Match("/crew/{server}/{id}"):
		a.creditsPage(c, r, r.Param("server"), r.Param("id"), true)
	case r.Match("/person/{server}/{id}"):
		a.person(c, r, r.Param("server"), r.Param("id"))
	case r.Match("/discover"):
		a.discoverPage(c, r)
	case r.Match("/catalog/{server}/{catalog}"):
		a.catalogPage(c, r)
	case r.Match("/title/{server}/{kind}/{tmdb}/crew"):
		a.titleCrewPage(c, r)
	case r.Match("/title/{server}/{kind}/{tmdb}"):
		a.titlePage(c, r)
	case r.Match("/discover/person/{server}/{tmdb}"):
		a.discoveredPersonPage(c, r)
	case r.Match("/discover/collection/{server}/{tmdb}"):
		a.collectionPage(c, r)
	case r.Match("/requests/{server}"):
		a.requestsPage(c, r)
	default:
		a.home(c)
	}
}

// navItem is an entry of the sidebar.
type navItem struct {
	path, icon, label string
}

// navItems returns the top-level destinations, then each server's
// libraries, under the server's name when there are several.
func (a *App) navItems(c *ui.Context) (top []navItem, sections []navSection) {
	top = []navItem{
		{"/search", "search", style.T(c, "nav.search")},
		{"/home", "home", style.T(c, "nav.home")},
	}
	if server, _, _ := a.discovery(c); server != "" {
		top = append(top, navItem{"/discover", "discover", style.T(c, "nav.discover")})
	}
	for _, s := range a.servers {
		libs := a.serverLibraries(s.GetId())
		if len(libs.value.GetLibraries()) == 0 {
			continue
		}
		title := style.T(c, "nav.libraries")
		if len(a.servers) > 1 {
			title = s.GetName()
		}
		sec := navSection{title: title}
		for _, l := range libs.value.GetLibraries() {
			sec.items = append(sec.items, navItem{"/library/" + s.GetId() + "/" + l.GetId(), libraryIcon(l.GetKind()), l.GetName()})
		}
		sections = append(sections, sec)
	}
	return top, sections
}

type navSection struct {
	title string
	items []navItem
}

// serverLibraries loads the libraries of the server with id.
func (a *App) serverLibraries(id string) *task[*capiv1.Libraries] {
	t := a.libraries[id]
	if t == nil {
		t = &task[*capiv1.Libraries]{}
		a.libraries[id] = t
	}
	return load(a, t, key("libraries", id, a.generation), func(ctx context.Context) (*capiv1.Libraries, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{ListLibraries: capiv1.ListLibraries_builder{ServerId: id}.Build()}.Build())
		return m.GetLibraries(), err
	})
}

func libraryIcon(k capiv1.LibraryType) string {
	switch k {
	case capiv1.LibraryType_LIBRARY_TYPE_MOVIES:
		return "movies"
	case capiv1.LibraryType_LIBRARY_TYPE_SHOWS:
		return "shows"
	case capiv1.LibraryType_LIBRARY_TYPE_MUSIC, capiv1.LibraryType_LIBRARY_TYPE_MUSIC_VIDEOS:
		return "music"
	case capiv1.LibraryType_LIBRARY_TYPE_HOME_VIDEOS:
		return "homeVideos"
	case capiv1.LibraryType_LIBRARY_TYPE_BOOKS:
		return "books"
	case capiv1.LibraryType_LIBRARY_TYPE_PHOTOS:
		return "photos"
	case capiv1.LibraryType_LIBRARY_TYPE_COLLECTIONS:
		return "collections"
	case capiv1.LibraryType_LIBRARY_TYPE_PLAYLISTS:
		return "playlists"
	}
	return "folder"
}

// titleBar builds the band under the window controls, which drags the
// window, with the button that collapses or expands the sidebar.
func (a *App) titleBar(c *ui.Context, medium, expanded bool) {
	bar := c.TitleBar()
	h := max(bar.Height, 28)
	ui.Row(c).Height(h + style.Layout.Gutter/2).PaddingStart(bar.Left).PaddingEnd(8).AlignItems(ui.Center).DragWindow().Children(func() {
		if !expanded {
			return
		}
		ui.Spacer(c)
		label := style.T(c, "nav.collapseSidebar")
		if iconButton(c, "sidebar", label).Clicked() {
			if medium {
				a.sidebarOpen = false
			} else {
				a.prefs.SidebarCollapsed = true
				a.savePreferences()
			}
		}
	})
}

// sidebar builds the expanded sidebar.
func (a *App) sidebar(c *ui.Context, medium bool) {
	top, sections := a.navItems(c)
	ui.Column(c).Width(a.sidebarWidth()).Fill().Label(style.T(c, "nav.navigation")).Children(func() {
		a.titleBar(c, medium, true)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 12, 12).Gap(2).Children(func() {
				for _, it := range top {
					a.sidebarRow(c, it, a.places.current == it.path)
				}
				for _, sec := range sections {
					style.Text(ui.Text(c, sec.title), style.Type.Footnote).FontWeight(600).
						TextColor(style.Color(style.Colors(c).TextTertiary)).SingleLine().Padding(16, 8, 6)
					for _, it := range sec.items {
						a.sidebarRow(c.Key(it.path), it, a.places.current == it.path)
					}
				}
			})
		})
		a.userButton(c, true)
	})
}

func (a *App) sidebarRow(c *ui.Context, it navItem, selected bool) {
	colors := style.Colors(c)
	row := ui.ButtonBase(c).Height(32).Padding(0, 8).Gap(10).Radius(design.RadiusMd).AlignItems(ui.Center).Label(it.label)
	switch {
	case selected:
		row.Background(style.Color(colors.FillSelected))
	case row.Hovered():
		row.Background(style.Color(colors.Fill))
	}
	if row.Pressed() {
		row.Opacity(0.8)
	}
	row.Children(func() {
		ui.Icon(c, style.Icon(it.icon, selected)).Size(18, 18).TextColor(style.Color(colors.Accent))
		style.Text(ui.Text(c, it.label), style.Type.Callout).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
	})
	if row.Clicked() {
		a.openPlace(it.path)
	}
}

// rail builds the sidebar collapsed to a rail of icons.
func (a *App) rail(c *ui.Context, medium bool) {
	top, sections := a.navItems(c)
	colors := style.Colors(c)
	ui.Column(c).Width(style.Layout.Rail).Fill().Label(style.T(c, "nav.navigation")).Children(func() {
		a.titleBar(c, medium, false)
		ui.Row(c).Justify(ui.Center).Padding(0, 0, 8).Children(func() {
			if iconButton(c, "sidebar", style.T(c, "nav.expandSidebar")).Clicked() {
				if medium {
					a.sidebarOpen = true
				} else {
					a.prefs.SidebarCollapsed = false
					a.savePreferences()
				}
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 8, 8).Gap(4).AlignItems(ui.Stretch).Children(func() {
				items := top
				for _, s := range sections {
					items = append(items, s.items...)
				}
				for i, it := range items {
					if i == len(top) {
						ui.Box(c).Height(1).Margin(4, 12).Background(style.Color(colors.Border))
					}
					selected := a.places.current == it.path
					cell := ui.ButtonBase(c.Key(it.path)).Padding(8, 4).Gap(4).Radius(design.RadiusMd).Column().AlignItems(ui.Center).Label(it.label).Tooltip(it.label)
					switch {
					case selected:
						cell.Background(style.Color(colors.FillSelected))
					case cell.Hovered():
						cell.Background(style.Color(colors.Fill))
					}
					cell.Children(func() {
						ui.Icon(c, style.Icon(it.icon, selected)).Size(22, 22).TextColor(style.Color(colors.Accent))
						style.Text(ui.Text(c, it.label), style.Type.Caption).TextColor(style.Color(colors.TextPrimary)).SingleLine().MaxWidth(style.Layout.Rail - 24)
					})
					if cell.Clicked() {
						a.openPlace(it.path)
					}
				}
			})
		})
		a.userButton(c, false)
	})
}

// iconButton builds a borderless button of an icon.
func iconButton(c *ui.Context, icon, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Size(28, 28).Radius(design.RadiusMd).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(18, 18).TextColor(style.Color(colors.TextSecondary))
	})
	return b
}

// avatar builds a user's avatar, size DIPs wide: the name's initial, quiet
// and the same for everyone.
func avatar(c *ui.Context, name string, size float32) {
	r, _ := utf8.DecodeRuneInString(name)
	initial := ""
	if r != utf8.RuneError {
		initial = string(unicode.ToUpper(r))
	}
	colors := style.Colors(c)
	ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Background(style.Color(colors.FillSelected)).Center().Children(func() {
		ui.Text(c, initial).FontSize(size * 0.42).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
	})
}
