package app

import (
	"context"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// switchState is the state of switching the standalone server's session
// to another of its users (docs/accounts.md §4.4).
type switchState struct {
	users task[[]*authv1.SwitchableUser]
	// asking is the user whose PIN the pad asks for; pin the digits
	// entered.
	asking *authv1.SwitchableUser
	pin    string
	busy   bool
	err    error
}

// switchServer returns the standalone server whose session switches
// users, or nil signed in with a Mavio account.
func (a *App) switchServer() *capiv1.ServerRecord {
	if a.account != nil || len(a.servers) == 0 {
		return nil
	}
	return a.servers[0]
}

// switchableUsers loads the users the standalone server's session may
// switch to, none when it is the only one.
func (a *App) switchableUsers() []*authv1.SwitchableUser {
	s := a.switchServer()
	if s == nil {
		return nil
	}
	id := s.GetId()
	st := &a.switching
	users := load(a, &st.users, key("switchable", id, s.GetUserId(), a.generation), func(ctx context.Context) ([]*authv1.SwitchableUser, error) {
		res, err := a.authService(id).ListSwitchableUsers(ctx, &authv1.ListSwitchableUsersRequest{})
		return res.GetUsers(), err
	}).value
	if len(users) < 2 {
		return nil
	}
	return users
}

// chooseUser switches to a user, or asks for its PIN first.
func (a *App) chooseUser(u *authv1.SwitchableUser) {
	st := &a.switching
	if u.GetCurrent() || st.busy {
		return
	}
	if u.GetHasPin() {
		st.asking, st.pin, st.err = u, "", nil
		return
	}
	a.switchUser(u, "")
}

// switchUser switches the session to a user, with its PIN when it has
// one; everything shown was the previous user's, so it all loads again.
func (a *App) switchUser(u *authv1.SwitchableUser, pin string) {
	s := a.switchServer()
	if s == nil {
		return
	}
	st := &a.switching
	st.busy, st.err = true, nil
	server := s.GetId()
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{SwitchUser: capiv1.SwitchUser_builder{
			ServerId: server, UserId: u.GetId(), Pin: pin,
		}.Build()}.Build())
		a.opts.Update(func() {
			st.busy, st.err = false, err
			if err != nil {
				// A wrong PIN clears the dots for another try.
				st.pin = ""
				return
			}
			st.asking = nil
			a.userMenuOpen = false
			for i, x := range a.servers {
				if x.GetId() == server && m.HasServer() {
					a.servers[i] = m.GetServer()
				}
			}
			a.generation++
		})
	}()
}

// switchMenu builds the user menu's users to switch to, as the web UI's:
// each with its avatar, the current one checked and those with a PIN
// locked.
func (a *App) switchMenu(c *ui.Context) {
	users := a.switchableUsers()
	if len(users) == 0 {
		return
	}
	colors := style.Colors(c)
	menuSeparator(c)
	style.Text(ui.Text(c, style.T(c, "auth.switchUser")), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary)).Padding(4, 8)
	for _, u := range users {
		b := ui.ButtonBase(c.Key("switch-"+u.GetId())).MinHeight(32).Padding(4, 8).Gap(10).Radius(6).AlignItems(ui.Center).Label(u.GetName()).
			Disabled(u.GetCurrent() || a.switching.busy)
		if !u.GetCurrent() && (a.tvFocus(c, b, false) || b.Hovered()) {
			b.Background(style.Color(colors.Fill))
		}
		b.Children(func() {
			avatar(c, u.GetName(), 24)
			style.Text(ui.Text(c, u.GetName()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
			switch {
			case u.GetCurrent():
				ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(colors.AccentText))
			case u.GetHasPin():
				ui.Icon(c, style.Icon("lock", false)).Size(14, 14).TextColor(style.Color(colors.TextTertiary))
			}
		})
		if b.Clicked() {
			if u.GetHasPin() {
				a.userMenuOpen = false
			}
			a.chooseUser(u)
		}
	}
}

// tvMembers builds the users to switch to on the television's settings,
// as avatars (docs/tv.md §6.6).
func (a *App) tvMembers(c *ui.Context) {
	users := a.switchableUsers()
	if len(users) == 0 {
		return
	}
	colors := style.Colors(c)
	group(c, "members", style.T(c, "auth.switchUser"), "", func(g *settingsGroup) {
		g.block(func() {
			ui.Row(c).Padding(16).Gap(24).Wrap().Children(func() {
				for _, u := range users {
					b := ui.ButtonBase(c.Key("member-" + u.GetId())).Column().Width(96).Padding(8).Gap(8).Radius(14).AlignItems(ui.Center).Label(u.GetName())
					lit := a.tvFocusLit(c, b, false)
					fg := style.Color(colors.TextPrimary)
					if lit {
						fg = ui.RGB(0, 0, 0)
					}
					b.Children(func() {
						ui.Box(c).Children(func() {
							// Every avatar has the ring's room, so that the names line up.
							ring := ui.Box(c).Padding(3).Radius(999).Border(2, ui.RGBA(0, 0, 0, 0))
							if u.GetCurrent() {
								ring.Border(2, style.Color(colors.Accent))
							}
							ring.Children(func() { avatar(c, u.GetName(), 64) })
							if u.GetHasPin() {
								ui.Box(c).Absolute().Right(0).Bottom(0).Padding(4).Radius(999).Background(style.Color(colors.SurfaceOverlay)).Children(func() {
									ui.Icon(c, style.Icon("lock", true)).Size(14, 14).TextColor(style.Color(colors.TextPrimary))
								})
							}
						})
						style.Text(ui.Text(c, u.GetName()), style.Type(c).Callout).TextColor(fg).SingleLine()
					})
					if b.Clicked() {
						a.chooseUser(u)
					}
				}
			})
		})
	})
}

// pinPad builds the dialog asking for the PIN of the user to switch to,
// as the web UI's: four dots and a keypad, the digits typed too; the PIN
// is sent once four are entered.
func (a *App) pinPad(c *ui.Context) {
	st := &a.switching
	if st.asking == nil {
		return
	}
	colors := style.Colors(c)
	u := st.asking
	press := func(k string) {
		if st.busy {
			return
		}
		before := len(st.pin)
		if k == "⌫" {
			if before > 0 {
				st.pin = st.pin[:before-1]
			}
			return
		}
		if before < 4 {
			st.pin += k
		}
		if len(st.pin) == 4 && before < 4 {
			a.switchUser(u, st.pin)
		}
	}
	cancel := func() { st.asking, st.pin, st.err = nil, "", nil }
	open := true
	title := style.T(c, "auth.enterPin", "name", u.GetName())
	ui.Modal(c, &open, func() {
		// The keys count only inside the dialog, which they are not behind.
		for d := range 10 {
			if c.Shortcut(0, ui.Key0+ui.Key(d)) {
				press(string(rune('0' + d)))
			}
		}
		if c.Shortcut(0, ui.KeyBackspace) {
			// On the television Back with no digit entered leaves, as Escape.
			if st.pin == "" && style.TV(c) {
				open = false
			} else {
				press("⌫")
			}
		}
		ui.Column(c).Width(288).Gap(0).AlignItems(ui.Center).Label(title).Children(func() {
			avatar(c, u.GetName(), 56)
			style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Margin(12, 0, 0)
			ui.Row(c).Gap(16).Margin(20, 0).Children(func() {
				for i := range 4 {
					dot := ui.Box(c.Key(key("dot", i))).Size(12, 12).Radius(999).Border(2, style.Color(colors.TextPrimary))
					if i < len(st.pin) {
						dot.Background(style.Color(colors.TextPrimary))
					}
				}
			})
			if st.err != nil {
				msg := errorMessage(c, st.err)
				if e := coreError(st.err); e != nil && e.Code == capiv1.Error_CODE_PERMISSION_DENIED {
					// The server refuses a wrong PIN.
					msg = style.T(c, "auth.wrongCode")
				}
				footnote(c, msg, true)
				ui.Box(c).Height(12)
			}
			ui.Column(c).Gap(12).Children(func() {
				for _, row := range [][]string{{"1", "2", "3"}, {"4", "5", "6"}, {"7", "8", "9"}, {"", "0", "⌫"}} {
					ui.Row(c.Key(row[1])).Gap(12).Children(func() {
						for _, k := range row {
							label := k
							switch k {
							case "":
								// Cancel takes the keypad's empty corner, as on a phone's.
								label = style.T(c, "action.cancel")
							case "⌫":
								label = style.T(c, "action.delete")
							}
							b := ui.ButtonBase(c.Key("key-"+label)).Size(64, 64).Radius(999).Center().Label(label)
							if a.tvFocusLit(c, b, false) || b.Hovered() {
								b.Background(style.Color(colors.FillSelected))
							} else {
								b.Background(style.Color(colors.Fill))
							}
							b.Children(func() {
								switch k {
								case "":
									style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary))
								case "⌫":
									ui.Icon(c, style.Icon("backspace", false)).Size(24, 24).TextColor(style.Color(colors.TextPrimary))
								default:
									style.Text(ui.Text(c, k), style.Type(c).Title2).TextColor(style.Color(colors.TextPrimary))
								}
							})
							if b.Clicked() {
								if k == "" {
									cancel()
								} else {
									press(k)
								}
							}
						}
					})
				}
			})
		})
	})
	if !open {
		cancel()
	}
}
