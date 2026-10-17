package app

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// discoverWait is how long discovery listens for servers' answers.
const discoverWait = 1500 * time.Millisecond

// quickConnectPoll is how often a Quick Connect request is polled.
const quickConnectPoll = 2 * time.Second

// signIn is the sign-in's state: the address, then the server found there
// with the user's credentials, or a Quick Connect request.
type signIn struct {
	address, name, password string
	server                  *capiv1.ServerInfo
	busy                    bool
	err                     error
	found                   task[[]*capiv1.FoundServer]
	quick                   *capiv1.QuickConnect
	stopQuick               context.CancelFunc
}

// addServer shows the sign-in to add a server.
func (a *App) addServer() {
	a.signIn = signIn{}
	a.phase = phaseSignIn
}

// signedIn shows the server just signed in to.
func (a *App) signedIn(s *capiv1.ServerRecord) {
	a.endQuickConnect()
	servers := slices.DeleteFunc(slices.Clone(a.servers), func(r *capiv1.ServerRecord) bool { return r.GetId() == s.GetId() })
	a.setServers(append(servers, s))
	a.router.Replace("/home")
}

func (a *App) endQuickConnect() {
	if a.signIn.stopQuick != nil {
		a.signIn.stopQuick()
	}
	if q := a.signIn.quick; q != nil {
		go func() {
			_, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{CancelQuickConnect: capiv1.CancelQuickConnect_builder{Request: q.GetRequest()}.Build()}.Build())
			if err != nil {
				slog.DebugContext(a.ctx, "cancel quick connect", "error", err)
			}
		}()
	}
	a.signIn.quick, a.signIn.stopQuick = nil, nil
}

// signInView builds the sign-in, over the whole window.
func (a *App) signInView(c *ui.Context) {
	s := &a.signIn
	colors := style.Colors(c)
	ui.Column(c).Fill().Background(style.Color(colors.Background)).DragWindow().Children(func() {
		ui.Row(c).Height(max(c.TitleBar().Height, 28)+8).Padding(0, 16).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			if len(a.servers) > 0 && ui.Button(c, style.T(c, "action.cancel")).Clicked() {
				a.endQuickConnect()
				a.phase = phaseReady
			}
		})
		_, height := c.Size()
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(max(24, height*0.14), 24, 48).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Width(360).Gap(16).AlignItems(ui.Stretch).Children(func() {
					switch {
					case s.quick != nil:
						a.quickConnectView(c)
					case s.server != nil:
						a.credentialsView(c)
					default:
						a.addressView(c)
					}
				})
			})
		})
	})
}

func title(c *ui.Context, text, message string) {
	colors := style.Colors(c)
	ui.Column(c).Gap(6).Padding(0, 0, 8).AlignItems(ui.Center).Children(func() {
		style.Text(ui.Text(c, text), style.Type.Title1).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center)
		if message != "" {
			style.Text(ui.Text(c, message), style.Type.Body).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.Center)
		}
	})
}

func field(c *ui.Context, label string, value *string, password bool, focus bool) ui.Element {
	var in ui.Element
	k := c.Key(label)
	ui.Field(k, label, func() {
		in = ui.TextInput(k, value).Height(36)
		if password {
			in.Password()
		}
		if focus {
			in.AutoFocus()
		}
	})
	return in
}

func errorLine(c *ui.Context, err error) {
	if err != nil {
		style.Text(ui.Text(c, errorMessage(c, err)), style.Type.Footnote).TextColor(style.Color(style.Colors(c).Danger)).TextAlign(ui.Center)
	}
}

// addressView asks for the server's address, offering those on the
// network.
func (a *App) addressView(c *ui.Context) {
	s := &a.signIn
	colors := style.Colors(c)
	title(c, style.T(c, "auth.welcome"), "")
	in := field(c, style.T(c, "auth.server"), &s.address, false, true).Placeholder("http://192.168.1.5:8686")
	errorLine(c, s.err)
	next := primaryButton(c, "", style.T(c, "action.continue")).Disabled(s.busy || strings.TrimSpace(s.address) == "")
	if (in.Submitted() || next.Clicked()) && !s.busy && strings.TrimSpace(s.address) != "" {
		a.probe(strings.TrimSpace(s.address))
	}
	found := load(a, &s.found, "discover", func(ctx context.Context) ([]*capiv1.FoundServer, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{Discover: capiv1.Discover_builder{WaitMs: uint32(discoverWait.Milliseconds())}.Build()}.Build())
		return m.GetFoundServers().GetServers(), err
	})
	if len(found.value) == 0 {
		return
	}
	style.Text(ui.Text(c, style.T(c, "auth.nearby")), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextTertiary)).Margin(16, 0, 0)
	ui.Column(c).Gap(4).Children(func() {
		for _, f := range found.value {
			row := ui.ButtonBase(c.Key(f.GetId())).Padding(10, 12).Gap(12).Radius(10).AlignItems(ui.Center).Background(style.Color(colors.Fill)).Label(f.GetName())
			if row.Hovered() {
				row.Background(style.Color(colors.FillSelected))
			}
			row.Children(func() {
				ui.Icon(c, style.Icon("server", false)).Size(20, 20).TextColor(style.Color(colors.Accent))
				ui.Column(c).Grow(1).Children(func() {
					style.Text(ui.Text(c, f.GetName()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
					style.Text(ui.Text(c, f.GetAddress()), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
				})
			})
			if row.Clicked() && !s.busy {
				s.address = f.GetAddress()
				a.probe(f.GetAddress())
			}
		}
	})
}

// probe asks the server at address who it is.
func (a *App) probe(address string) {
	s := &a.signIn
	s.busy, s.err = true, nil
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Probe: capiv1.Probe_builder{Address: address}.Build()}.Build())
		a.opts.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			s.server = m.GetServerInfo()
			s.address = s.server.GetAddress()
		})
	}()
}

// credentialsView asks for the user's name and password, or for those of
// the first user of a new server.
func (a *App) credentialsView(c *ui.Context) {
	s := &a.signIn
	setup := s.server.GetSetupRequired()
	if setup {
		title(c, style.T(c, "auth.setUp", "server", s.server.GetName()), style.T(c, "auth.setUpMessage"))
	} else {
		title(c, style.T(c, "auth.signInTo", "server", s.server.GetName()), s.server.GetAddress())
	}
	field(c, style.T(c, "auth.username"), &s.name, false, true)
	pw := field(c, style.T(c, "auth.password"), &s.password, true, false)
	errorLine(c, s.err)
	label := style.T(c, "auth.signIn")
	if setup {
		label = style.T(c, "auth.createAccount")
	}
	ok := strings.TrimSpace(s.name) != "" && !s.busy
	submit := primaryButton(c, "", label).Disabled(!ok)
	if (submit.Clicked() || pw.Submitted()) && ok {
		a.submitCredentials(setup)
	}
	if !setup {
		style.Text(ui.Text(c, style.T(c, "auth.or")), style.Type.Footnote).TextColor(style.Color(style.Colors(c).TextTertiary)).TextAlign(ui.Center)
		if secondaryButton(c, "devices", style.T(c, "auth.quickConnect")).Disabled(s.busy).Clicked() && !s.busy {
			a.startQuickConnect()
		}
	}
	if ui.Link(c, style.T(c, "action.back"), "").AlignSelf(ui.Center).Clicked() {
		*s = signIn{address: s.address, found: s.found}
	}
}

func (a *App) submitCredentials(setup bool) {
	s := &a.signIn
	s.busy, s.err = true, nil
	address, name, password := s.address, strings.TrimSpace(s.name), s.password
	go func() {
		req := capiv1.Request_builder{SignIn: capiv1.SignIn_builder{Address: address, Name: name, Password: password}.Build()}
		if setup {
			req = capiv1.Request_builder{CreateFirstUser: capiv1.CreateFirstUser_builder{Address: address, Name: name, Password: password}.Build()}
		}
		m, err := a.opts.Core.Call(a.ctx, req.Build())
		a.opts.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			a.signedIn(m.GetServer())
		})
	}()
}

// startQuickConnect asks the server for a Quick Connect code and polls
// the request until it is authorized.
func (a *App) startQuickConnect() {
	s := &a.signIn
	s.busy, s.err = true, nil
	address := s.address
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{StartQuickConnect: capiv1.StartQuickConnect_builder{Address: address}.Build()}.Build())
		a.opts.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			q := m.GetQuickConnect()
			ctx, cancel := context.WithCancel(a.ctx)
			s.quick, s.stopQuick = q, cancel
			go a.pollQuickConnect(ctx, q.GetRequest())
		})
	}()
}

func (a *App) pollQuickConnect(ctx context.Context, request uint64) {
	t := time.NewTicker(quickConnectPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{PollQuickConnect: capiv1.PollQuickConnect_builder{Request: request}.Build()}.Build())
		if ctx.Err() != nil {
			return
		}
		switch {
		case err != nil:
			slog.WarnContext(ctx, "poll Quick Connect", "error", err)
			a.opts.Update(func() {
				if a.signIn.quick.GetRequest() == request {
					a.signIn.quick, a.signIn.err = nil, err
				}
			})
			return
		case m.HasServer():
			a.opts.Update(func() {
				if a.signIn.quick.GetRequest() == request {
					a.signIn.quick = nil
					a.signedIn(m.GetServer())
				}
			})
			return
		}
	}
}

// quickConnectView shows the Quick Connect code while waiting for it to be
// entered.
func (a *App) quickConnectView(c *ui.Context) {
	colors := style.Colors(c)
	q := a.signIn.quick
	title(c, style.T(c, "auth.quickConnectTitle"), style.T(c, "auth.quickConnectCode", "code", q.GetCode()))
	style.Text(ui.Text(c, q.GetCode()), style.Type.Display).FontSize(48).LetterSpacing(8).
		TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center).Selectable()
	ui.Row(c).Gap(8).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		style.Text(ui.Text(c, style.T(c, "auth.waiting")), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
	})
	if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
		a.endQuickConnect()
	}
}
