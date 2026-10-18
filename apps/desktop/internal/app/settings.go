package app

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// userService returns the UserService client of the server with id.
func (a *App) userService(id string) userv1connect.UserServiceClient {
	return userv1connect.NewUserServiceClient(a.connectHTTP(), serverURL(id))
}

// sessionService returns the SessionService client of the server with id.
func (a *App) sessionService(id string) sessionv1connect.SessionServiceClient {
	return sessionv1connect.NewSessionServiceClient(a.connectHTTP(), serverURL(id))
}

// settingsSection is a section of the settings, as the web UI's.
type settingsSection struct {
	id, icon string
	color    ui.Color
	title    string // its string's key
	// admin marks a section of the server's administrators.
	admin bool
}

var settingsSections = []settingsSection{
	{id: "overview", icon: "server", color: ui.RGB(0x8E, 0x8E, 0x93), title: "settings.overview", admin: true},
	{id: "general", icon: "settings", color: ui.RGB(0x8E, 0x8E, 0x93), title: "settings.general", admin: true},
	{id: "users", icon: "users", color: ui.RGB(0xFF, 0x95, 0x00), title: "settings.users", admin: true},
	{id: "api-keys", icon: "lock", color: ui.RGB(0x63, 0x63, 0x66), title: "settings.apiKeys", admin: true},
	{id: "proxies", icon: "network", color: ui.RGB(0x30, 0xB0, 0xC7), title: "settings.proxies", admin: true},
	{id: "libraries", icon: "folder", color: ui.RGB(0x00, 0x7A, 0xFF), title: "settings.libraries", admin: true},
	{id: "storage", icon: "server", color: ui.RGB(0x30, 0xB0, 0xC7), title: "settings.storage", admin: true},
	{id: "acquisition", icon: "downloads", color: ui.RGB(0x34, 0xC7, 0x59), title: "settings.acquisition", admin: true},
	{id: "wanted", icon: "pending", color: ui.RGB(0xFF, 0x3B, 0x30), title: "settings.wanted", admin: true},
	{id: "downloads", icon: "downloads", color: ui.RGB(0x34, 0xC7, 0x59), title: "settings.downloads", admin: true},
	{id: "subtitles", icon: "subtitles", color: ui.RGB(0xFF, 0x95, 0x00), title: "settings.subtitles", admin: true},
	{id: "transcoding", icon: "quality", color: ui.RGB(0xFF, 0x2D, 0x55), title: "settings.transcoding", admin: true},
	{id: "activity", icon: "activity", color: ui.RGB(0xFF, 0x2D, 0x55), title: "settings.activity", admin: true},
	{id: "tasks", icon: "pending", color: ui.RGB(0x58, 0x56, 0xD6), title: "settings.tasks", admin: true},
	{id: "logs", icon: "info", color: ui.RGB(0x8E, 0x8E, 0x93), title: "settings.logs", admin: true},
	{id: "appearance", icon: "appearance", color: ui.RGB(0x5E, 0x5C, 0xE6), title: "settings.appearance"},
	{id: "playback", icon: "play", color: ui.RGB(0xFF, 0x3B, 0x30), title: "settings.playback"},
	{id: "subtitles-audio", icon: "subtitles", color: ui.RGB(0x34, 0xC7, 0x59), title: "settings.subtitlesAudio"},
	{id: "account", icon: "profile", color: ui.RGB(0xFF, 0x95, 0x00), title: "settings.account"},
	{id: "devices", icon: "devices", color: ui.RGB(0x32, 0xAD, 0xE6), title: "settings.devices"},
}

func sectionOf(id string) (settingsSection, bool) {
	i := slices.IndexFunc(settingsSections, func(s settingsSection) bool { return s.id == id })
	if i < 0 {
		return settingsSection{}, false
	}
	return settingsSections[i], true
}

// settingsMenu is a menu of the server's settings: a second level holding
// the sections of one concern.
type settingsMenu struct {
	id, icon string
	color    ui.Color
	title    string
	sections []string
}

var settingsMenus = []settingsMenu{
	{id: "media", icon: "movies", color: ui.RGB(0xFF, 0x9F, 0x0A), title: "settings.mediaMenu", sections: []string{"libraries", "storage", "transcoding"}},
	{id: "acquiring", icon: "cloud", color: ui.RGB(0x30, 0xD1, 0x58), title: "settings.acquisitionMenu", sections: []string{"acquisition", "wanted", "downloads", "subtitles"}},
	{id: "maintenance", icon: "admin", color: ui.RGB(0x8E, 0x8E, 0x93), title: "settings.maintenanceMenu", sections: []string{"activity", "tasks", "logs"}},
}

// settingsGroups are the sidebar's groups: this device's sections, then
// the server's, in the order a server is set up and kept, with its menus.
var settingsGroups = []struct {
	title   string
	entries []string
	server  bool
}{
	{"settings.thisDevice", []string{"appearance", "playback", "subtitles-audio"}, false},
	{"settings.serverGroup", []string{"overview", "general", "users", "devices", "api-keys", "proxies", "media", "acquiring", "maintenance"}, true},
}

func menuOf(id string) (settingsMenu, bool) {
	i := slices.IndexFunc(settingsMenus, func(m settingsMenu) bool { return m.id == id })
	if i < 0 {
		return settingsMenu{}, false
	}
	return settingsMenus[i], true
}

// admin reports whether the user administers the server chosen; known
// tells whether that is known yet.
func (a *App) admin() (admin, known bool) {
	s := a.chosenServer()
	if s == nil {
		return false, true
	}
	t := a.currentUser(s.GetId())
	return t.value.GetAdmin(), t.loaded
}

// settingsState is the state of the settings.
type settingsState struct {
	// server is the ID of the server whose settings are shown.
	server string
	// users are the user signed in to each server.
	users map[string]*task[*userv1.User]
	// open tells which menus of the sidebar are open, when not the one
	// holding the section shown.
	open  map[string]bool
	admin adminState

	languages languagesForm
	sessions  task[[]*sessionv1.Session]
	reload    int // counts the sessions' changes
	revoking  string
	err       error // of the last change
	copied    string
	code      string // the Quick Connect code typed
	authorize struct {
		busy, done bool
		err        error
	}
}

// languagesForm is the form of the user's languages and when subtitles
// show, as loaded and as edited.
type languagesForm struct {
	key                   string // the user it was loaded for
	audio, subtitles      string
	mode                  userv1.SubtitleMode
	audio0, subtitles0    string
	mode0                 userv1.SubtitleMode
	saving                bool
	err                   error
	playDefaultAudioTrack bool
}

// chosenServer returns the server whose settings are shown: the one chosen,
// else the first.
func (a *App) chosenServer() *capiv1.ServerRecord {
	for _, s := range a.servers {
		if s.GetId() == a.settingsState.server {
			return s
		}
	}
	if len(a.servers) > 0 {
		return a.servers[0]
	}
	return nil
}

// settings builds the settings, in a window of their own layout: the
// sidebar of the sections beside the section shown.
func (a *App) settings(c *ui.Context, r *ui.Route) {
	// The settings are a layout: the sidebar stays as the section beside
	// it changes, which alone fades in.
	r.Match("/settings/{section...}")
	id := strings.SplitN(r.Param("section"), "/", 2)[0]
	if id == "" {
		// A section, so that going to another fades it in rather than
		// sliding in as a page deeper.
		a.replace("/settings/appearance")
		id = "appearance"
	}
	if m, ok := menuOf(id); ok {
		id = m.sections[0]
	}
	sec, ok := sectionOf(id)
	if !ok {
		id = "appearance"
	}
	// A server the user does not administer has no administration.
	if admin, known := a.admin(); sec.admin && known && !admin {
		id = "devices"
	}
	r.Title(style.T(c, "nav.settings"))
	colors := style.Colors(c)
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		bar := ui.Column(c).Shrink(0).Width(a.sidebarWidth())
		if !c.Vibrancy() {
			bar.Background(style.Color(colors.Sidebar))
		}
		bar.Children(func() {
			a.settingsSidebar(c, id)
			a.sidebarResizer(c)
		})
		ui.Column(c).Grow(1).MinWidth(0).Background(style.Color(colors.Background)).Children(func() {
			r.View(c, func(r *ui.Route) {
				r.Page().Background(style.Color(colors.Background))
				ui.Scroll(c.Key(id + r.Query("user") + r.Query("library") + r.Query("remote") + r.Query("acquisition") + r.Query("subtitles") + r.Query("id"))).Fill().Children(func() {
					top := max(c.TitleBar().Height, 12)
					ui.Column(c).Padding(top+28, 56, 40).AlignItems(ui.Center).Children(func() {
						ui.Column(c).FillWidth().MaxWidth(900).Gap(28).Children(func() { a.settingsSection(c, r, id) })
					})
				})
			})
		})
	})
	if c.Shortcut(0, ui.KeyEscape) {
		a.router.Push("/home")
	}
}

func (a *App) settingsSidebar(c *ui.Context, current string) {
	colors := style.Colors(c)
	top := max(c.TitleBar().Height, 12)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(top+8, 16, 20).Gap(16).Children(func() {
			ui.Box(c).Height(4).FillWidth().DragWindow()
			home := ui.ButtonBase(c).AlignSelf(ui.Start).Gap(4).AlignItems(ui.Center).Label(style.T(c, "nav.home"))
			home.Children(func() {
				ui.Icon(c, style.Icon("back", false)).Size(18, 18).TextColor(style.Color(colors.AccentText))
				t := style.Text(ui.Text(c, style.T(c, "nav.home")), style.Type(c).Body).TextColor(style.Color(colors.AccentText))
				if home.Hovered() {
					t.Underline()
				}
			})
			if home.Clicked() {
				a.router.Push("/home")
			}
			style.Text(ui.Text(c, style.T(c, "nav.settings")), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary)).Padding(0, 4)
			a.accountCard(c, current == "account")
			server := a.chosenServer()
			for _, g := range settingsGroups {
				if g.server && server == nil {
					continue
				}
				ui.Column(c.Key(g.title)).Gap(2).Children(func() {
					style.Text(ui.Text(c, style.T(c, g.title)), style.Type(c).Caption).FontWeight(600).TextColor(style.Color(colors.TextTertiary)).Padding(0, 8, 4)
					if g.server && a.account != nil && len(a.servers) > 1 {
						a.serverChoice(c)
					}
					admin, _ := a.admin()
					for _, id := range g.entries {
						if m, ok := menuOf(id); ok {
							if !admin {
								continue
							}
							if len(m.sections) == 1 {
								id = m.sections[0]
							} else {
								a.sidebarMenu(c, m, current)
								continue
							}
						}
						s, _ := sectionOf(id)
						if s.admin && !admin {
							continue
						}
						a.sidebarLink(c, s, s.id == current)
					}
				})
			}
		})
	})
}

// serverChoice builds the choice of the server whose settings are shown.
func (a *App) serverChoice(c *ui.Context) {
	names := make([]string, len(a.servers))
	for i, s := range a.servers {
		names[i] = s.GetName()
	}
	chosen := a.chosenServer().GetName()
	sel := ui.Select(c, &chosen, names).FillWidth().Margin(0, 0, 6).Label(style.T(c, "settings.serverChoice"))
	if sel.Changed() {
		if i := slices.Index(names, chosen); i >= 0 {
			a.settingsState.server = a.servers[i].GetId()
		}
	}
}

// sidebarLink builds a section's link in the sidebar.
func (a *App) sidebarLink(c *ui.Context, s settingsSection, selected bool) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(s.id)).Height(40).Padding(0, 8).Gap(12).Radius(10).AlignItems(ui.Center).Label(style.T(c, s.title)).Selected(selected)
	switch {
	case selected:
		b.Background(style.Color(colors.FillSelected))
	case b.Hovered():
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		sectionTile(c, s.icon, s.color)
		style.Text(ui.Text(c, style.T(c, s.title)), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
	})
	if b.Clicked() && !selected {
		a.replace("/settings/" + s.id)
	}
}

// sidebarMenu builds a menu in the sidebar, opening in place to its
// sections: open while it holds the section shown, until closed, and
// marked while closed over it.
func (a *App) sidebarMenu(c *ui.Context, m settingsMenu, current string) {
	colors := style.Colors(c)
	st := &a.settingsState
	within := slices.Contains(m.sections, current)
	open, set := st.open[m.id]
	if !set {
		open = within
	}
	ui.Column(c.Key(m.id)).Gap(2).Children(func() {
		b := ui.ButtonBase(c).Height(40).Padding(0, 8).Gap(12).Radius(10).AlignItems(ui.Center).Label(style.T(c, m.title))
		switch {
		case within && !open:
			b.Background(style.Color(colors.FillSelected))
		case b.Hovered():
			b.Background(style.Color(colors.Fill))
		}
		b.Children(func() {
			sectionTile(c, m.icon, m.color)
			style.Text(ui.Text(c, style.T(c, m.title)), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
			chevron := ui.Icon(c, style.Icon("expand", false)).Size(16, 16).TextColor(style.Color(colors.TextTertiary))
			if !open {
				chevron.Rotate(-90)
			}
		})
		if b.Clicked() {
			if st.open == nil {
				st.open = map[string]bool{}
			}
			st.open[m.id] = !open
		}
		if open {
			ui.Column(c).Gap(2).Padding(0, 0, 0, 24).Children(func() {
				for _, id := range m.sections {
					s, _ := sectionOf(id)
					a.sidebarLink(c, s, s.id == current)
				}
			})
		}
	})
}

// sectionTile builds a section's icon on its colored tile.
func sectionTile(c *ui.Context, icon string, color ui.Color) {
	ui.Box(c).Size(28, 28).Shrink(0).Radius(7).Background(color).Center().Children(func() {
		ui.Icon(c, style.Icon(icon, true)).Size(17, 17).TextColor(ui.RGB(255, 255, 255))
	})
}

// accountName returns the name of the account signed in, and the line
// under it: how many servers it has, or the server of a standalone user.
func (a *App) accountName(c *ui.Context) (name, line string) {
	if a.account != nil {
		return a.account.GetName(), style.T(c, "menu.serverCount", "count", len(a.servers))
	}
	if len(a.servers) > 0 {
		return a.servers[0].GetUserName(), a.servers[0].GetName()
	}
	return "", ""
}

// accountCard builds the user's card, opening the account's section.
func (a *App) accountCard(c *ui.Context, active bool) {
	colors := style.Colors(c)
	name, line := a.accountName(c)
	if a.account != nil {
		line = style.T(c, "auth.mavioAccount")
	}
	b := ui.ButtonBase(c).Padding(12).Gap(12).Radius(14).AlignItems(ui.Center).Label(name).Selected(active)
	// At rest the card is the surface, as the web UI's, which differs
	// from the sidebar by a shade; over the sidebar of the desktop's own
	// material, where an opaque surface would stand apart, it is clear.
	// Hovered, the fill tints it.
	// Over keeps the alpha of what is under it, so over the clear card
	// the fill shows by itself.
	rest, hovered := style.Color(colors.Surface), style.Color(colors.Fill).Over(style.Color(colors.Surface))
	if c.Vibrancy() {
		rest, hovered = ui.Transparent, style.Color(colors.Fill)
	}
	switch {
	case active:
		b.Background(style.Color(colors.FillSelected))
	case b.Hovered():
		b.Background(hovered)
	default:
		b.Background(rest)
	}
	b.Children(func() {
		avatar(c, name, 44)
		ui.Column(c).MinWidth(0).Grow(1).Children(func() {
			style.Text(ui.Text(c, name), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			style.Text(ui.Text(c, line), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
		})
	})
	if b.Clicked() && !active {
		a.replace("/settings/account")
	}
}

// settingsSection builds the section with id.
func (a *App) settingsSection(c *ui.Context, r *ui.Route, id string) {
	switch id {
	case "appearance":
		a.appearanceSection(c)
	case "playback":
		a.playbackSection(c)
	case "subtitles-audio":
		a.languagesSection(c)
	case "account":
		a.accountSection(c)
	case "devices":
		a.devicesSection(c)
	case "overview":
		a.overviewSection(c)
	case "general":
		a.generalSection(c)
	case "users":
		a.usersSection(c, r)
	case "api-keys":
		a.apiKeysSection(c)
	case "transcoding":
		a.transcodingSection(c)
	case "proxies":
		a.proxiesSection(c)
	case "libraries":
		a.librariesSection(c, r)
	case "storage":
		a.storageSection(c, r)
	case "acquisition":
		a.acquisitionSection(c, r)
	case "wanted":
		a.wantedSection(c)
	case "downloads":
		a.downloadsSection(c)
	case "subtitles":
		a.subtitlesSection(c, r)
	case "activity":
		a.activitySection(c)
	case "tasks":
		a.tasksSection(c)
	case "logs":
		a.logsSection(c)
	}
}

// sectionHeader builds a section's title and the line under it.
func sectionHeader(c *ui.Context, title, message string) {
	colors := style.Colors(c)
	ui.Column(c).Gap(4).Children(func() {
		style.Text(ui.Text(c, title), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary))
		if message != "" {
			style.Text(ui.Text(c, message), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
		}
	})
}

// settingsGroup is a group of rows on a surface, as the web UI's Group.
type settingsGroup struct {
	c    *ui.Context
	rows int
}

// group builds a group, with a title over it and a footer under it, of
// the rows fn adds.
func group(c *ui.Context, key, title, footer string, fn func(g *settingsGroup)) {
	groupWith(c, key, title, nil, footer, fn)
}

// groupWith is group with a control at the end of its title, such as a
// filter of its rows.
func groupWith(c *ui.Context, key, title string, control func(), footer string, fn func(g *settingsGroup)) {
	colors := style.Colors(c)
	ui.Column(c.Key(key)).Gap(8).Children(func() {
		if title != "" || control != nil {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(0, 4).Children(func() {
				style.Text(ui.Text(c, title), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Grow(1)
				if control != nil {
					control()
				}
			})
		}
		ui.Column(c).Radius(14).Clip().Background(style.Color(colors.Surface)).Children(func() {
			fn(&settingsGroup{c: c})
		})
		if footer != "" {
			style.Text(ui.Text(c, footer), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Padding(0, 4)
		}
	})
}

// divider separates a row from the one before it.
func (g *settingsGroup) divider() {
	if g.rows > 0 {
		ui.Box(g.c).Height(1).Margin(0, 0, 0, 16).Background(style.Color(style.Colors(g.c).Border))
	}
	g.rows++
}

// row adds a row of a label, the line under it, what leads it and its
// control.
func (g *settingsGroup) row(label, detail string, leading, control func()) {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	ui.Row(c.Key(g.rows)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		if leading != nil {
			leading()
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
			if detail != "" {
				style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
			}
		})
		if control != nil {
			control()
		}
	})
}

// switchRow adds a row of a switch, which a click anywhere on the row
// toggles, and reports a change of *on.
func (g *settingsGroup) switchRow(label, detail string, on *bool) bool {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	changed := false
	b := ui.ButtonBase(c.Key(g.rows)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Label(label)
	b.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
			if detail != "" {
				style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			}
		})
		changed = ui.Switch(c, on).Label(label).Changed()
	})
	if b.Clicked() && !changed {
		*on, changed = !*on, true
	}
	return changed
}

// link adds a row of a page's link, or of what is listed when it opens
// nothing: its tile when icon is set, its label and the line under it. It
// reports a click.
func (g *settingsGroup) link(key, icon string, color ui.Color, label, detail string, opens bool) bool {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(label).Disabled(!opens)
	if opens && b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		if icon != "" {
			sectionTile(c, icon, color)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1)
			if detail != "" {
				style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			}
		})
		if opens {
			ui.Icon(c, style.Icon("forward", false)).Size(16, 16).Shrink(0).TextColor(style.Color(colors.TextTertiary))
		}
	})
	return opens && b.Clicked()
}

// block adds a row of its own content.
func (g *settingsGroup) block(fn func()) {
	g.divider()
	ui.Column(g.c.Key(g.rows)).Padding(16).Gap(16).Children(fn)
}

// choose builds a drop-down of labels choosing *value, an index of them,
// and reports a new choice.
func choose(c *ui.Context, label string, value *int, labels []string) bool {
	chosen := labels[max(0, min(*value, len(labels)-1))]
	if ui.Select(c, &chosen, labels).MinWidth(200).Label(label).Changed() {
		if i := slices.Index(labels, chosen); i >= 0 && i != *value {
			*value = i
			return true
		}
	}
	return false
}

func (a *App) appearanceSection(c *ui.Context) {
	colors := style.Colors(c)
	sectionHeader(c, style.T(c, "settings.appearance"), style.T(c, "settings.appearanceMessage"))
	themes := []struct {
		appearance Appearance
		key        string
	}{
		{AppearanceSystem, "settings.system"},
		{AppearanceLight, "settings.light"},
		{AppearanceDark, "settings.dark"},
	}
	ui.Row(c).Gap(16).Children(func() {
		for _, th := range themes {
			on := a.Appearance() == th.appearance
			b := ui.ButtonBase(c.Key(th.key)).Basis(0).Grow(1).Column().Gap(8).AlignItems(ui.Center).Label(style.T(c, th.key))
			b.Children(func() {
				themePreview(c, th.appearance, on || b.Hovered())
				t := style.Text(ui.Text(c, style.T(c, th.key)), style.Type(c).Callout)
				if on {
					t.FontWeight(600).TextColor(style.Color(colors.TextPrimary))
				} else {
					t.TextColor(style.Color(colors.TextSecondary))
				}
			})
			if b.Clicked() {
				a.setAppearance(th.appearance)
			}
		}
	})
	locales := append([]string{""}, design.Locales...)
	labels := make([]string, len(locales))
	for i, l := range locales {
		if l == "" {
			labels[i] = style.T(c, "settings.systemLanguage")
			continue
		}
		name := display.Self.Name(language.Make(l))
		if r := []rune(name); len(r) > 0 {
			name = string(unicode.ToUpper(r[0])) + string(r[1:])
		}
		labels[i] = name
	}
	group(c, "language", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "settings.language"), "", nil, func() {
			i := max(slices.Index(locales, a.prefs.Language), 0)
			if choose(c, style.T(c, "settings.language"), &i, labels) {
				a.setLanguage(locales[i])
			}
		})
	})
	group(c, "tv", "", style.T(c, "settings.tvModeNote"), func(g *settingsGroup) {
		on := a.prefs.TV
		if g.switchRow(style.T(c, "settings.tvMode"), style.T(c, "settings.tvModeMessage"), &on) {
			a.setTV(on)
		}
	})
}

// themePreview builds a small window in a theme; System shows both
// halves.
func themePreview(c *ui.Context, theme Appearance, on bool) {
	colors := style.Colors(c)
	pane := func(dark bool) {
		bg, side, line := ui.RGB(0xF2, 0xF2, 0xF7), ui.RGB(255, 255, 255), ui.RGB(0xD1, 0xD1, 0xD6)
		if dark {
			bg, side, line = ui.RGB(0x1C, 0x1C, 0x1E), ui.RGB(0x2C, 0x2C, 0x2E), ui.RGB(0x3A, 0x3A, 0x3C)
		}
		ui.Row(c).Basis(0).Grow(1).AlignItems(ui.Stretch).Padding(8).Gap(6).Background(bg).Children(func() {
			ui.Box(c).Basis(0).Grow(1).Radius(4).Background(side)
			ui.Column(c).Basis(0).Grow(3).Gap(6).Children(func() {
				ui.Box(c).Basis(0).Grow(1).Radius(4).LinearGradient(ui.LinearGradient{
					Angle: 135,
					Stops: []ui.GradientStop{{At: 0, Color: ui.RGB(0xFF, 0xD3, 0x4E)}, {At: 0.5, Color: ui.RGB(0xFF, 0x9A, 0x2E)}, {At: 1, Color: ui.RGB(0xF0, 0x41, 0x2F)}},
				})
				ui.Box(c).Height(6).Width(60).Radius(3).Background(line)
				ui.Box(c).Basis(0).Grow(1)
			})
		})
	}
	box := ui.Row(c).FillWidth().AspectRatio(1.6).Radius(12).Clip().AlignItems(ui.Stretch).Border(1, style.Color(colors.Border))
	if on {
		box.Border(2, style.Color(colors.Accent))
	}
	box.Children(func() {
		if theme == AppearanceSystem {
			pane(false)
			pane(true)
		} else {
			pane(theme == AppearanceDark)
		}
	})
}

// qualityName returns how a quality is called.
func qualityName(c *ui.Context, q quality) string {
	if q.key == "original" || q.key == "auto" {
		return style.T(c, "player.quality"+strings.ToUpper(q.key[:1])+q.key[1:])
	}
	return q.key
}

func (a *App) playbackSection(c *ui.Context) {
	sectionHeader(c, style.T(c, "settings.playback"), style.T(c, "settings.appearanceMessage"))
	group(c, "playback", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "settings.maxQuality"), "", nil, func() {
			labels := make([]string, len(qualities))
			for i, q := range qualities {
				labels[i] = qualityName(c, q)
			}
			i := slices.IndexFunc(qualities, func(q quality) bool { return q.key == a.quality().key })
			if choose(c, style.T(c, "settings.maxQuality"), &i, labels) {
				a.prefs.Quality = qualities[i].key
				a.savePreferences()
			}
		})
		on := a.autoplayNext()
		if g.switchRow(style.T(c, "settings.autoplayNext"), "", &on) {
			a.prefs.NoAutoplayNext = !on
			a.savePreferences()
		}
		windows := a.prefs.PlayerWindows
		if g.switchRow(style.T(c, "settings.playerWindows"), style.T(c, "settings.playerWindowsMessage"), &windows) {
			a.prefs.PlayerWindows = windows
			a.savePreferences()
		}
	})
}

// subtitleModes are the choices of when subtitles show, in their order.
var subtitleModes = []struct {
	mode userv1.SubtitleMode
	key  string
}{
	{userv1.SubtitleMode_SUBTITLE_MODE_UNSPECIFIED, "settings.subtitleModeDefault"},
	{userv1.SubtitleMode_SUBTITLE_MODE_ALWAYS, "settings.subtitleModeAlways"},
	{userv1.SubtitleMode_SUBTITLE_MODE_FOREIGN, "settings.subtitleModeForeign"},
	{userv1.SubtitleMode_SUBTITLE_MODE_SMART, "settings.subtitleModeSmart"},
	{userv1.SubtitleMode_SUBTITLE_MODE_FORCED, "settings.subtitleModeForced"},
	{userv1.SubtitleMode_SUBTITLE_MODE_NONE, "settings.subtitleModeNone"},
}

// languageCodes returns the three-letter codes of a field, lowercase and
// separated by commas or spaces, and whether it holds only such codes.
func languageCodes(s string) ([]string, bool) {
	list := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == '，' || r == '、'
	})
	for _, l := range list {
		if len(l) != 3 || strings.IndexFunc(l, func(r rune) bool { return r < 'a' || r > 'z' }) >= 0 {
			return nil, false
		}
	}
	return list, true
}

// currentUser loads the user signed in to a server: whose preferences
// the section of languages shows, and whether they administer it.
func (a *App) currentUser(server string) *task[*userv1.User] {
	st := &a.settingsState
	if st.users == nil {
		st.users = map[string]*task[*userv1.User]{}
	}
	t := st.users[server]
	if t == nil {
		t = &task[*userv1.User]{}
		st.users[server] = t
	}
	return load(a, t, key("me", server, a.generation), func(ctx context.Context) (*userv1.User, error) {
		res, err := a.userService(server).GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
		return res.GetUser(), err
	})
}

// languagesSection builds the user's languages and when subtitles show,
// read from the first server and saved to each.
func (a *App) languagesSection(c *ui.Context) {
	sectionHeader(c, style.T(c, "settings.subtitlesAudio"), style.T(c, "settings.subtitlesAudioMessage"))
	if len(a.servers) == 0 {
		return
	}
	t := a.currentUser(a.servers[0].GetId())
	if taskStatus(c, t, func() { t.key = "" }, nil) {
		return
	}
	f := &a.settingsState.languages
	if f.key != t.key {
		p := t.value.GetPlaybackPreferences()
		*f = languagesForm{
			key: t.key, audio: strings.Join(p.GetAudioLanguages(), ", "), subtitles: strings.Join(p.GetSubtitleLanguages(), ", "),
			mode: p.GetSubtitleMode(), playDefaultAudioTrack: !p.HasPlayDefaultAudioTrack() || p.GetPlayDefaultAudioTrack(),
		}
		f.audio0, f.subtitles0, f.mode0 = f.audio, f.subtitles, f.mode
	}
	colors := style.Colors(c)
	field := func(label string, value *string) {
		ui.Column(c.Key(label)).Gap(6).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			ui.TextInput(c, value).Placeholder("chi, eng").Label(label)
		})
	}
	group(c, "languages", "", style.T(c, "settings.languagesHint"), func(g *settingsGroup) {
		g.block(func() {
			field(style.T(c, "settings.audioLanguages"), &f.audio)
			field(style.T(c, "settings.subtitleLanguages"), &f.subtitles)
		})
	})
	group(c, "mode", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "settings.subtitleMode"), "", nil, func() {
			labels := make([]string, len(subtitleModes))
			for i, m := range subtitleModes {
				labels[i] = style.T(c, m.key)
			}
			i := slices.IndexFunc(subtitleModes, func(m struct {
				mode userv1.SubtitleMode
				key  string
			},
			) bool {
				return m.mode == f.mode
			})
			if choose(c, style.T(c, "settings.subtitleMode"), &i, labels) {
				f.mode = subtitleModes[i].mode
			}
		})
	})
	audio, okAudio := languageCodes(f.audio)
	subtitles, okSubtitles := languageCodes(f.subtitles)
	changed := f.audio != f.audio0 || f.subtitles != f.subtitles0 || f.mode != f.mode0
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		save := ui.PrimaryButton(c, style.T(c, "action.save")).Disabled(!changed || !okAudio || !okSubtitles || f.saving)
		if save.Clicked() {
			f.saving, f.err = true, nil
			prefs := userv1.PlaybackPreferences_builder{
				AudioLanguages: audio, SubtitleLanguages: subtitles, SubtitleMode: &f.mode, PlayDefaultAudioTrack: &f.playDefaultAudioTrack,
			}.Build()
			servers := slices.Clone(a.servers)
			go func() {
				var err error
				for _, s := range servers {
					_, e := a.userService(s.GetId()).UpdatePlaybackPreferences(a.ctx, userv1.UpdatePlaybackPreferencesRequest_builder{PlaybackPreferences: prefs}.Build())
					if e != nil && err == nil {
						err = e
					}
				}
				a.opts.Update(func() {
					f.saving, f.err = false, err
					if err == nil {
						f.audio0, f.subtitles0, f.mode0 = f.audio, f.subtitles, f.mode
					}
				})
			}()
		}
		if f.err != nil {
			style.Text(ui.Text(c, errorMessage(c, f.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
		}
	})
}

// mavioDevSite returns mavio.dev's site of the account's API.
func (a *App) mavioDevSite() string {
	if a.account == nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(a.account.GetApi(), "/"), "/api")
}

func (a *App) accountSection(c *ui.Context) {
	colors := style.Colors(c)
	if a.account != nil {
		sectionHeader(c, style.T(c, "settings.account"), style.T(c, "settings.mavioAccountMessage"))
		name, line := a.accountName(c)
		group(c, "account", "", "", func(g *settingsGroup) {
			g.row(name, line, func() { avatar(c, name, 44) }, func() {
				if a.opts.OpenURL == nil {
					return
				}
				b := ui.ButtonBase(c).Gap(6).AlignItems(ui.Center).Label(style.T(c, "settings.manageAtMavio"))
				b.Children(func() {
					t := style.Text(ui.Text(c, style.T(c, "settings.manageAtMavio")), style.Type(c).Body).TextColor(style.Color(colors.AccentText))
					if b.Hovered() {
						t.Underline()
					}
					ui.Icon(c, style.Icon("external", false)).Size(16, 16).TextColor(style.Color(colors.AccentText))
				})
				if b.Clicked() {
					_ = a.opts.OpenURL(a.mavioDevSite() + "/account")
				}
			})
		})
	} else if len(a.servers) > 0 {
		s := a.servers[0]
		sectionHeader(c, style.T(c, "settings.account"), style.T(c, "settings.standaloneAccountMessage", "name", s.GetUserName(), "server", s.GetName()))
		a.authorizeDevice(c, s.GetId())
	}
	a.serversGroup(c)
}

// authorizeDevice builds the signing in of another device showing a Quick
// Connect code, as the user of a standalone server.
func (a *App) authorizeDevice(c *ui.Context, server string) {
	colors := style.Colors(c)
	st := &a.settingsState
	group(c, "authorize", style.T(c, "auth.authorizeDevice"), "", func(g *settingsGroup) {
		g.block(func() {
			ui.Row(c).Gap(12).AlignItems(ui.End).Wrap().Children(func() {
				ui.Column(c).Width(160).Gap(6).Children(func() {
					style.Text(ui.Text(c, style.T(c, "auth.quickConnect")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
					in := ui.TextInput(c, &st.code).Placeholder("000000").Label(style.T(c, "auth.quickConnect"))
					if in.Changed() {
						digits := strings.Map(func(r rune) rune {
							if r >= '0' && r <= '9' {
								return r
							}
							return -1
						}, st.code)
						st.code = digits[:min(len(digits), 6)]
						st.authorize.done, st.authorize.err = false, nil
					}
				})
				b := ui.PrimaryButton(c, style.T(c, "auth.authorize")).Disabled(len(st.code) < 6 || st.authorize.busy)
				if b.Clicked() || (len(st.code) == 6 && c.Shortcut(0, ui.KeyEnter)) {
					st.authorize.busy, st.authorize.err = true, nil
					code := st.code
					go func() {
						_, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{
							AuthorizeQuickConnect: capiv1.AuthorizeQuickConnect_builder{ServerId: server, Code: code}.Build(),
						}.Build())
						a.opts.Update(func() {
							st.authorize.busy, st.authorize.err, st.authorize.done = false, err, err == nil
							if err == nil {
								st.code = ""
							}
						})
					}()
				}
			})
			switch {
			case st.authorize.done:
				style.Text(ui.Text(c, style.T(c, "auth.authorized")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			case st.authorize.err != nil:
				style.Text(ui.Text(c, errorMessage(c, st.authorize.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
			}
		})
	})
}

// serversGroup builds the servers in the user's order, each with its
// address, moved up and down.
func (a *App) serversGroup(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState
	footer := style.T(c, "settings.serversMessage")
	if a.account != nil {
		footer = style.T(c, "settings.serversMavioMessage")
	}
	move := func(i, by int) {
		ids := make([]string, len(a.servers))
		for j, s := range a.servers {
			ids[j] = s.GetId()
		}
		ids[i], ids[i+by] = ids[i+by], ids[i]
		servers := slices.Clone(a.servers)
		servers[i], servers[i+by] = servers[i+by], servers[i]
		a.servers = servers
		go func() {
			if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{OrderServers: capiv1.OrderServers_builder{ServerIds: ids}.Build()}.Build()); err != nil {
				a.opts.Update(func() { st.err = err })
			}
			a.listServers()
		}()
	}
	group(c, "servers", style.T(c, "auth.servers"), footer, func(g *settingsGroup) {
		for i, s := range a.servers {
			detail := style.T(c, "auth.signedInAs", "name", s.GetUserName()) + " · " + s.GetAddress()
			leading := func() {
				ui.Box(c).Size(36, 36).Shrink(0).Radius(9).Background(style.Color(colors.Fill)).Center().Children(func() {
					style.Text(ui.Text(c, initials(s.GetName())[:min(len(initials(s.GetName())), 1)]), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
				})
			}
			g.row(s.GetName(), detail, leading, func() {
				ui.Row(c.Key(s.GetId())).Gap(2).AlignItems(ui.Center).Children(func() {
					if len(a.servers) > 1 {
						up := iconButton(c.Key("up"), "expand", style.T(c, "auth.moveUp")).Size(36, 36).Rotate(180).Disabled(i == 0)
						if up.Clicked() && i > 0 {
							move(i, -1)
						}
						down := iconButton(c.Key("down"), "expand", style.T(c, "auth.moveDown")).Size(36, 36).Disabled(i == len(a.servers)-1)
						if down.Clicked() && i < len(a.servers)-1 {
							move(i, 1)
						}
					}
					icon := "copy"
					if st.copied == s.GetId() {
						icon = "check"
					}
					if iconButton(c.Key("copy"), icon, style.T(c, "settings.copyAddress")).Size(36, 36).Clicked() {
						c.WriteClipboard(s.GetAddress())
						st.copied = s.GetId()
					}
				})
			})
		}
	})
	if st.err != nil {
		style.Text(ui.Text(c, errorMessage(c, st.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
	}
}

// devicesSection builds the devices signed in to the server chosen as the
// user, to sign them out.
func (a *App) devicesSection(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState
	sectionHeader(c, style.T(c, "settings.devices"), style.T(c, "settings.devicesMessage"))
	server := a.chosenServer()
	if server == nil {
		return
	}
	id := server.GetId()
	t := load(a, &st.sessions, key("sessions", id, a.generation, st.reload), func(ctx context.Context) ([]*sessionv1.Session, error) {
		res, err := a.sessionService(id).ListSessions(ctx, &sessionv1.ListSessionsRequest{})
		return res.GetSessions(), err
	})
	if taskStatus(c, t, func() { st.sessions.key = "" }, nil) {
		return
	}
	group(c, "sessions", "", "", func(g *settingsGroup) {
		for _, s := range t.value {
			name := cmp.Or(s.GetDeviceName(), s.GetClient(), "—")
			var detail []string
			if s.GetClient() != "" {
				detail = append(detail, s.GetClient())
			}
			switch {
			case s.GetNowPlaying().GetItem() != nil:
				detail = append(detail, style.T(c, "remote.playing", "name", s.GetNowPlaying().GetItem().GetName()))
			case s.GetOnline():
				detail = append(detail, style.T(c, "remote.online"))
			case s.HasLastSeenTime():
				detail = append(detail, mediumDateTime(s.GetLastSeenTime().AsTime()))
			}
			leading := func() {
				ui.Box(c).Size(36, 36).Shrink(0).Radius(9).Background(style.Color(colors.Fill)).Center().Children(func() {
					ui.Icon(c, style.Icon("devices", false)).Size(20, 20).TextColor(style.Color(colors.TextPrimary))
				})
			}
			g.row(name, strings.Join(detail, " · "), leading, func() {
				if s.GetCurrent() {
					style.Text(ui.Text(c, style.T(c, "settings.thisDevice")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
					return
				}
				if s.GetPluginId() != "" {
					return
				}
				b := ui.ButtonBase(c.Key(s.GetId())).Padding(6, 10).Radius(8).Label(style.T(c, "auth.signOut")).Disabled(st.revoking != "")
				if b.Hovered() {
					b.Background(style.Color(colors.Fill))
				}
				b.Children(func() {
					style.Text(ui.Text(c, style.T(c, "auth.signOut")), style.Type(c).Body).TextColor(style.Color(colors.Danger))
				})
				if b.Clicked() && st.revoking == "" {
					st.revoking, st.err = s.GetId(), nil
					sid := s.GetId()
					go func() {
						_, err := a.sessionService(id).RevokeSession(a.ctx, sessionv1.RevokeSessionRequest_builder{Id: &sid}.Build())
						a.opts.Update(func() {
							st.revoking, st.err = "", err
							st.reload++
						})
					}()
				}
			})
		}
	})
	if st.err != nil {
		style.Text(ui.Text(c, errorMessage(c, st.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
	}
}

// mediumDateTime formats a time as the web UI's medium date and short time.
func mediumDateTime(t time.Time) string { return t.Local().Format("Jan 2, 2006, 3:04 PM") }
