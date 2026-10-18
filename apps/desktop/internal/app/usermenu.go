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
			style.Text(ui.Text(c, name), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			style.Text(ui.Text(c, line), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary)).SingleLine()
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
	// On the television, wider for the larger type and in frosted glass,
	// so that the artwork behind shows through (docs/tv.md §5).
	tv := style.TV(c)
	width, blur, tint := float32(300), float32(24), float32(0.9)
	if tv {
		width, blur, tint = 400, 48, 0.45
	}
	panel.Margin(8, 0, 0, 0).Width(width).Radius(14).Material(glass.Blur{Radius: blur}).AlignItems(ui.Stretch).
		Shadow(shadow.X, shadow.Y, shadow.Blur, shadow.Spread, style.Color(shadow.Color))
	ui.Column(c).Radius(14).Background(style.Color(colors.SurfaceOverlay).Alpha(tint)).Border(1, style.Color(colors.Border)).
		Padding(6).AlignItems(ui.Stretch).Children(func() {
		name, line := a.accountName(c)
		kind := style.T(c, "auth.standaloneUser", "server", line)
		if a.account != nil {
			kind = style.T(c, "auth.mavioAccount")
		}
		ui.Row(c).Padding(8, 10).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, name, 36)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, name), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
				style.Text(ui.Text(c, kind), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
			})
			a.themeSwitch(c)
		})
		menuSeparator(c)
		settings, tvKeys := "⌘,", "⌃⌘T"
		if goruntime.GOOS != "darwin" {
			settings, tvKeys = "Ctrl+,", "Shift+F11"
		}
		// A remote cannot press a shortcut.
		if tv {
			settings, tvKeys = "", ""
		}
		if a.menuItem(c, "settings", style.T(c, "nav.settings"), settings, tv) {
			a.userMenuOpen = false
			a.router.Push("/settings/appearance")
		}
		icon, text := "television", style.T(c, "menu.tvMode")
		if tv {
			icon, text = "sidebar", style.T(c, "menu.standardMode")
		}
		if a.menuItem(c, icon, text, tvKeys, false) {
			a.userMenuOpen = false
			a.setTV(!tv)
		}
		if tv && a.opts.ToggleFullScreen != nil {
			icon, text := "fullScreen", style.T(c, "menu.fullScreen")
			if a.opts.IsFullScreen != nil && a.opts.IsFullScreen() {
				icon, text = "exitFullScreen", style.T(c, "menu.exitFullScreen")
			}
			if a.menuItem(c, icon, text, "", false) {
				a.userMenuOpen = false
				a.opts.ToggleFullScreen()
			}
		}
		if a.account != nil {
			if a.opts.OpenURL != nil && !tv && a.menuItem(c, "external", style.T(c, "menu.manageAccount"), "", false) {
				a.userMenuOpen = false
				_ = a.opts.OpenURL(a.mavioDevSite() + "/account")
			}
		} else if a.menuItem(c, "server", style.T(c, "menu.manageServers"), "", false) {
			a.userMenuOpen = false
			a.router.Push("/settings/account")
		}
		menuSeparator(c)
		if a.menuItem(c, "signOut", style.T(c, "auth.signOut"), "", false) {
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
			if a.tvFocus(c, b, chosen) {
				b.Background(ui.RGB(255, 255, 255))
				b.Children(func() {
					ui.Icon(c, style.Icon(it.icon, false)).Size(16, 16).TextColor(ui.RGB(0, 0, 0))
				})
				if b.Clicked() {
					a.setAppearance(it.appearance)
				}
				continue
			}
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
// hovered, or on the television as it has focus, which the menu opens
// with on its first item. It reports whether it was chosen.
func (a *App) menuItem(c *ui.Context, icon, text, shortcut string, first bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(icon)).MinHeight(28).Padding(4, 8).Gap(8).Radius(6).AlignItems(ui.Center).Label(text)
	if first {
		b.AutoFocus()
	}
	if a.tvFocus(c, b, false) || b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(18, 18).Shrink(0).TextColor(style.Color(colors.TextSecondary))
		style.Text(ui.Text(c, text), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
		if shortcut != "" {
			style.Text(ui.Text(c, shortcut), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary))
		}
	})
	return b.Clicked()
}

// menuSeparator builds the line between a menu's groups.
func menuSeparator(c *ui.Context) {
	ui.Box(c).Height(1).Margin(4, 10).Background(style.Color(style.Colors(c).Border))
}
