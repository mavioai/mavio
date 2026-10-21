package app

import (
	"net/url"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// tvSettingsView builds the settings in the TV mode (docs/tv.md §6.6):
// the list of their sections, a menu's list or a section in its place,
// with no sidebar, focus moved by the arrows and back going up a level.
func (a *App) tvSettingsView(c *ui.Context) {
	colors := style.Colors(c)
	a.visitPlaces()
	a.focusPage()
	a.beginFocus()
	ui.Column(c).Fill().Background(style.Color(colors.Background)).Children(func() {
		a.router.View(c, func(r *ui.Route) { a.settings(c, r) })
	})
	a.tvKeyboardKeys(c)
	if c.Shortcut(0, ui.KeyEscape) || c.Shortcut(0, ui.KeyBackspace) {
		a.settingsBack()
	}
	a.focusKeys(c, false)
}

// tvSettings builds the page of the settings at id in the TV mode: their
// list, a menu's sections, or a section alone.
func (a *App) tvSettings(c *ui.Context, r *ui.Route, id string) {
	r.Title(style.T(c, "nav.settings"))
	r.Page().Background(style.Color(style.Colors(c).Background))
	admin, _ := a.admin()
	key := subpageKey(id, r)
	ui.Scroll(c.Key(key)).Fill().Children(func() {
		ui.Column(c).Padding(64, style.Layout(c).Margin, 48).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(768).Gap(28).Children(func() {
				if m, ok := menuOf(id); ok {
					ids := shownSections(m.sections, admin)
					if len(ids) > 1 {
						m.sections = ids
						a.tvMenuList(c, m)
						return
					}
					id = m.sections[0]
					if len(ids) == 1 {
						id = ids[0]
					}
				}
				if id == "" {
					a.tvSettingsList(c)
					return
				}
				if sec, ok := sectionOf(id); !ok || sec.admin && !admin {
					id = "devices"
				}
				a.settingsSection(c, r, id)
				// A server's section names the server chosen beside its
				// title, as the sidebar would.
				if !slices.Contains(settingsGroups[0].entries, id) && id != "account" && a.account != nil && len(a.servers) > 1 {
					ui.Box(c).Absolute().Top(0).Right(0).Width(280).Children(func() { a.serverChoice(c) })
				}
			})
		})
	})
}

// tvSettingsList builds the list of the settings: the account, then the
// sections and menus by group, and signing out.
func (a *App) tvSettingsList(c *ui.Context) {
	colors := style.Colors(c)
	style.Text(ui.Text(c, style.T(c, "nav.settings")), style.Type(c).Display).TextColor(style.Color(colors.TextPrimary))
	a.tvAccountRow(c)
	a.tvMembers(c)
	admin, _ := a.admin()
	server := a.chosenServer()
	for _, g := range settingsGroups {
		if g.server && server == nil {
			continue
		}
		group(c, g.title, style.T(c, g.title), "", func(gr *settingsGroup) {
			if g.server && a.account != nil && len(a.servers) > 1 {
				gr.block(func() { a.serverChoice(c) })
			}
			for _, id := range g.entries {
				if m, ok := menuOf(id); ok {
					ids := shownSections(m.sections, admin)
					if len(ids) > 1 {
						a.tvEntryRow(gr, m.id, m.icon, m.color, m.title)
						continue
					}
					if len(ids) == 0 {
						continue
					}
					id = ids[0]
				}
				s, _ := sectionOf(id)
				if s.admin && !admin {
					continue
				}
				a.tvEntryRow(gr, s.id, s.icon, s.color, s.title)
			}
		})
	}
	if len(a.servers) > 0 {
		b := ui.ButtonBase(c.Key("sign-out")).Height(52).Radius(14).Center().Label(style.T(c, "auth.signOut"))
		lit := a.tvFocusLit(c, b, false)
		if !lit {
			b.Background(style.Color(colors.Surface))
		}
		fg := style.Color(colors.Danger)
		b.Children(func() {
			style.Text(ui.Text(c, style.T(c, "auth.signOut")), style.Type(c).Headline).TextColor(fg)
		})
		if b.Clicked() {
			a.signOut()
		}
	}
}

// tvMenuList builds a menu's list of its sections, under its title and
// the choice of the server, whose they are.
func (a *App) tvMenuList(c *ui.Context, m settingsMenu) {
	colors := style.Colors(c)
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		style.Text(ui.Text(c, style.T(c, m.title)), style.Type(c).Display).TextColor(style.Color(colors.TextPrimary)).Grow(1).MinWidth(0)
		if a.account != nil && len(a.servers) > 1 {
			ui.Box(c).Width(280).Children(func() { a.serverChoice(c) })
		}
	})
	group(c, m.id, "", "", func(g *settingsGroup) {
		for _, id := range m.sections {
			s, _ := sectionOf(id)
			a.tvEntryRow(g, s.id, s.icon, s.color, s.title)
		}
	})
}

// tvAccountRow builds the user's card, opening the account's section.
func (a *App) tvAccountRow(c *ui.Context) {
	colors := style.Colors(c)
	name, line := a.accountName(c)
	if a.account != nil {
		line = style.T(c, "auth.mavioAccount")
	}
	b := ui.ButtonBase(c.Key("account")).Padding(16).Gap(16).Radius(14).AlignItems(ui.Center).Label(name)
	lit := a.tvFocusLit(c, b, false)
	fg, sub := style.Color(colors.TextPrimary), style.Color(colors.TextSecondary)
	if lit {
		fg, sub = ui.RGB(0, 0, 0), style.Color(design.LightColors.TextSecondary)
	} else {
		b.Background(style.Color(colors.Surface))
	}
	b.Children(func() {
		avatar(c, name, 64)
		ui.Column(c).MinWidth(0).Grow(1).Children(func() {
			style.Text(ui.Text(c, name), style.Type(c).Title3).TextColor(fg).SingleLine()
			style.Text(ui.Text(c, line), style.Type(c).Callout).TextColor(sub).SingleLine()
		})
		ui.Icon(c, style.Icon("forward", false)).Size(20, 20).Shrink(0).TextColor(sub)
	})
	if b.Clicked() {
		a.router.Push("/settings/account")
	}
}

// tvEntryRow adds the row of a section or a menu, opening it, white with
// focus and filling the group's row, its corners rounded as the group's
// (docs/tv.md §4).
func (a *App) tvEntryRow(g *settingsGroup, id, icon string, color ui.Color, title string) {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	label := style.T(c, title)
	b := ui.ButtonBase(c.Key(id)).MinHeight(64).Padding(12, 20).Gap(16).Radius(14).AlignItems(ui.Center).Label(label)
	lit := a.tvFocusLit(c, b, false)
	fg, sub := style.Color(colors.TextPrimary), style.Color(colors.TextTertiary)
	if lit {
		fg, sub = ui.RGB(0, 0, 0), style.Color(design.LightColors.TextSecondary)
	}
	b.Children(func() {
		sectionTile(c, icon, color)
		style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(fg).SingleLine().Grow(1)
		ui.Icon(c, style.Icon("forward", false)).Size(18, 18).Shrink(0).TextColor(sub)
	})
	if b.Clicked() {
		a.router.Push("/settings/" + id)
	}
}

// settingsSubpages are the parameters naming a page within a section of
// the settings, as a user or a library.
var settingsSubpages = []string{"user", "library", "remote", "acquisition", "subtitles", "id", "plugin", "catalogs", "conversation"}

// subpageKey returns the key of a section's page, which scrolls on its
// own: the section's ID and its subpage's parameters.
func subpageKey(id string, r *ui.Route) string {
	var b strings.Builder
	b.WriteString(id)
	for _, k := range settingsSubpages {
		b.WriteString("/" + r.Query(k))
	}
	return b.String()
}

// settingsNested are the pages within a page within a section, by the
// page they lie in.
var settingsNested = map[string]map[string]string{"acquisition": {"format": "formats"}}

// settingsDepth returns how deep the location loc lies in the settings:
// their list 0, a menu 1, a section 2, a page within one 3 and a page
// within that 4; any other page -1.
func settingsDepth(loc string) int {
	path, query, _ := strings.Cut(loc, "?")
	if path == "/settings" {
		return 0
	}
	id, ok := strings.CutPrefix(path, "/settings/")
	if !ok {
		return -1
	}
	if _, ok := menuOf(id); ok {
		return 1
	}
	q, _ := url.ParseQuery(query)
	for k, pages := range settingsNested {
		if _, ok := pages[q.Get(k)]; ok {
			return 4
		}
	}
	for _, k := range settingsSubpages {
		if q.Has(k) {
			return 3
		}
	}
	if id == "" {
		return 0
	}
	return 2
}

// settingsParent returns the page the page at loc lies in: a nested page's
// page, a page within a section's section, a section's menu or their list,
// a menu's list. A menu left with one section of those shown to the
// user, an administrator or not, is that section.
func settingsParent(loc string, admin bool) string {
	path, query, _ := strings.Cut(loc, "?")
	q, _ := url.ParseQuery(query)
	switch settingsDepth(loc) {
	case 4:
		for k, pages := range settingsNested {
			if up, ok := pages[q.Get(k)]; ok {
				return path + "?" + url.Values{k: {up}}.Encode()
			}
		}
	case 3:
		for _, k := range settingsSubpages {
			q.Del(k)
		}
		if len(q) > 0 {
			return path + "?" + q.Encode()
		}
		return path
	case 2:
		id := strings.TrimPrefix(path, "/settings/")
		for _, m := range settingsMenus {
			if len(shownSections(m.sections, admin)) > 1 && slices.Contains(m.sections, id) {
				return "/settings/" + m.id
			}
		}
	}
	return "/settings"
}

// settingsBack goes up a level in the settings (docs/tv.md §6.6): to the
// last page before in the history lying above this one, past those beside
// it, as a user's or a format's opened from a list; failing one, to the
// page it lies in, and from their list to the page they were opened from,
// else Home.
func (a *App) settingsBack() {
	p := &a.places
	loc := a.router.Location()
	depth := settingsDepth(loc)
	for k := p.at - 1; k >= 0; k-- {
		if settingsDepth(p.history[k]) < depth {
			a.router.Go(k - p.at)
			return
		}
	}
	if depth == 0 {
		a.replace("/home")
		return
	}
	admin, _ := a.admin()
	a.replace(settingsParent(loc, admin))
}
