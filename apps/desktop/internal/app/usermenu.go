package app

import (
	goruntime "runtime"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// userButton builds the foot of the sidebar: the user's avatar, name and
// server or account, which open the user menu; on the rail, the avatar
// alone.
func (a *App) userButton(c *ui.Context, withName bool) {
	if len(a.servers) == 0 {
		return
	}
	colors := style.Colors(c)
	name, line := a.accountName(c)
	label := style.T(c, "nav.userMenu", "name", name)
	b := ui.ButtonBase(c).Margin(8, 12, 12).Padding(8).Gap(10).Radius(10).AlignItems(ui.Center).Label(label)
	if !withName {
		b.Margin(8).Size(36, 36).Padding(0).Center().Tooltip(name)
	}
	if b.Hovered() || a.userMenuOpen {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		if !withName {
			avatar(c, name, 28)
			return
		}
		avatar(c, name, 32)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, name), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			style.Text(ui.Text(c, line), style.Type.Caption).TextColor(style.Color(colors.TextSecondary)).SingleLine()
		})
		ui.Icon(c, style.Icon("selector", false)).Size(16, 16).Shrink(0).TextColor(style.Color(colors.TextTertiary))
	})
	if b.Clicked() {
		a.userMenuOpen = !a.userMenuOpen
	}
	ui.PopoverBase(c, b, &a.userMenuOpen, func(panel ui.Element) {
		a.userMenu(c, panel)
	})
}

// userMenu fills the user menu, as the web UI's: who is signed in beside
// the appearance, the settings, the TV mode, managing the account or the
// servers, and signing out. It is a popup of glass, its tint on a column
// of its own over the blur, as an element's background takes the place of
// its material.
func (a *App) userMenu(c *ui.Context, panel ui.Element) {
	colors := style.Colors(c)
	shadow := design.ElevationLevel3
	panel.Margin(8, 0, 0, 0).Width(300).Radius(14).Material(glass.Blur{Radius: 24}).AlignItems(ui.Stretch).
		Shadow(shadow.X, shadow.Y, shadow.Blur, shadow.Spread, style.Color(shadow.Color))
	ui.Column(c).Radius(14).Background(style.Color(colors.SurfaceOverlay).Alpha(0.9)).Border(1, style.Color(colors.Border)).
		Padding(6).AlignItems(ui.Stretch).Children(func() {
		name, line := a.accountName(c)
		kind := style.T(c, "auth.standaloneUser", "server", line)
		if a.account != nil {
			kind = style.T(c, "auth.mavioAccount")
		}
		ui.Row(c).Padding(8, 10).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, name, 36)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, name), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
				style.Text(ui.Text(c, kind), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
			})
			a.themeSwitch(c)
		})
		menuSeparator(c)
		settings, tv := "⌘,", "⌃⌘T"
		if goruntime.GOOS != "darwin" {
			settings, tv = "Ctrl+,", "Shift+F11"
		}
		if menuItem(c, "settings", style.T(c, "nav.settings"), settings) {
			a.userMenuOpen = false
			a.router.Push("/settings/appearance")
		}
		if menuItem(c, "television", style.T(c, "menu.tvMode"), tv) {
			a.userMenuOpen = false
			a.setTV(true)
		}
		if a.account != nil {
			if a.opts.OpenURL != nil && menuItem(c, "external", style.T(c, "menu.manageAccount"), "") {
				a.userMenuOpen = false
				_ = a.opts.OpenURL(a.mavioDevSite() + "/account")
			}
		} else if menuItem(c, "server", style.T(c, "menu.manageServers"), "") {
			a.userMenuOpen = false
			a.router.Push("/settings/account")
		}
		menuSeparator(c)
		if menuItem(c, "signOut", style.T(c, "auth.signOut"), "") {
			a.userMenuOpen = false
			a.signOut()
		}
	})
}

// themeSwitch builds the appearance's choice beside the account's name:
// the system's, light or dark, each a round icon.
func (a *App) themeSwitch(c *ui.Context) {
	colors := style.Colors(c)
	ui.Row(c).Shrink(0).Gap(2).Padding(2).Radius(design.RadiusFull).Background(style.Color(colors.Fill)).
		Label(style.T(c, "settings.appearance")).Children(func() {
		for _, it := range []struct {
			appearance Appearance
			icon, key  string
		}{
			{AppearanceSystem, "appearance", "settings.system"},
			{AppearanceLight, "lightMode", "settings.light"},
			{AppearanceDark, "darkMode", "settings.dark"},
		} {
			chosen := a.Appearance() == it.appearance
			label := style.T(c, it.key)
			b := ui.ButtonBase(c).Size(28, 28).Radius(design.RadiusFull).Center().Label(label).Tooltip(label).Selected(chosen)
			color := colors.TextSecondary
			if chosen {
				l1 := design.ElevationLevel1
				b.Background(style.Color(colors.SurfaceRaised)).Shadow(l1.X, l1.Y, l1.Blur, l1.Spread, style.Color(l1.Color))
				color = colors.TextPrimary
			} else if b.Hovered() {
				color = colors.TextPrimary
			}
			b.Children(func() {
				ui.Icon(c, style.Icon(it.icon, false)).Size(16, 16).TextColor(style.Color(color))
			})
			if b.Clicked() {
				a.setAppearance(it.appearance)
			}
		}
	})
}

// menuItem builds an item of a menu of the app's own, as the web UI's:
// its icon, its text and the keys of its shortcut, highlighted as it is
// hovered. It reports whether it was chosen.
func menuItem(c *ui.Context, icon, text, shortcut string) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).MinHeight(28).Padding(4, 8).Gap(8).Radius(6).AlignItems(ui.Center).Label(text)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(18, 18).Shrink(0).TextColor(style.Color(colors.TextSecondary))
		style.Text(ui.Text(c, text), style.Type.Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
		if shortcut != "" {
			style.Text(ui.Text(c, shortcut), style.Type.Footnote).TextColor(style.Color(colors.TextTertiary))
		}
	})
	return b.Clicked()
}

// menuSeparator builds the line between a menu's groups.
func menuSeparator(c *ui.Context) {
	ui.Box(c).Height(1).Margin(4, 10).Background(style.Color(style.Colors(c).Border))
}
