package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"rsc.io/qr"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// Signing in (docs/accounts.md §6, docs/clients.md §4.1), as the web UI
// does: a Mavio account at mavio.dev, in the browser or with a link code
// and its QR code, or a server by its address, which a server in the
// Mavio account mode sends back to the Mavio account's sign-in. A card on
// a wall of posters; in TV mode, the link code alone.

// ClientID is the desktop app's client ID at mavio.dev.
const ClientID = "dev.mavio.desktop"

// discoverWait is how long discovery listens for servers' answers.
const discoverWait = 1500 * time.Millisecond

// quickConnectPoll is how often a Quick Connect request is polled.
const quickConnectPoll = 3 * time.Second

// signInStep is a step of signing in.
type signInStep int

const (
	// stepMavio offers the Mavio account's sign-ins and connecting to a
	// server.
	stepMavio signInStep = iota
	// stepBrowser waits for the browser to come back from mavio.dev.
	stepBrowser
	// stepLink shows a link code and its QR code.
	stepLink
	// stepAddress asks for a server's address.
	stepAddress
	// stepCredentials asks for a standalone server's user and password.
	stepCredentials
	// stepQuickConnect shows a Quick Connect code.
	stepQuickConnect
	// stepSetUp sends a new server's setup to the browser.
	stepSetUp
)

// signIn is the sign-in's state: its step and the steps before, the
// server connected to, and what waits for another device.
type signIn struct {
	step                    signInStep
	history                 []signInStep
	address, name, password string
	// server is the server connected to; in the Mavio account mode, the
	// Mavio account signs in at its mavio.dev.
	server  *capiv1.ServerInfo
	busy    bool
	err     error
	found   task[[]*capiv1.FoundServer]
	browser *capiv1.AccountSignIn
	link    *capiv1.LinkCode
	linkQR  *ui.SVG
	quick   *capiv1.QuickConnect
	// stop stops waiting for the browser, the link code or Quick Connect.
	stop context.CancelFunc
}

// look is the sizes of the sign-in in a layout, after the web UI's.
type look struct {
	tv                                     bool
	gap, title, subtitle, body, label      float32
	button, buttonRadius, field, fieldText float32
	digitW, digitH, digit, icon, qr        float32
}

var (
	cardLook = look{
		gap: 22, title: 28, subtitle: 14, body: 15, label: 13,
		button: 48, buttonRadius: 12, field: 44, fieldText: 15,
		digitW: 40, digitH: 54, digit: 28, icon: 18, qr: 168,
	}
	tvLook = look{
		tv: true, gap: 32, title: 54, subtitle: 26, body: 26, label: 22,
		button: 84, buttonRadius: 22, field: 84, fieldText: 28,
		digitW: 88, digitH: 110, digit: 56, icon: 30, qr: 260,
	}
)

// goTo goes on to step, which Back leaves.
func (a *App) goTo(step signInStep) {
	s := &a.signIn
	s.history = append(s.history, s.step)
	s.step, s.err = step, nil
}

// back goes back a step, giving up what waits on this one.
func (a *App) back() {
	s := &a.signIn
	a.endPending()
	if len(s.history) == 0 {
		return
	}
	s.step, s.history = s.history[len(s.history)-1], s.history[:len(s.history)-1]
	s.err, s.busy = nil, false
	if s.step == stepAddress || len(s.history) == 0 {
		s.server = nil
	}
}

// endPending stops waiting for the browser, a link code or Quick Connect,
// and tells the core to give them up.
func (a *App) endPending() {
	s := &a.signIn
	if s.stop != nil {
		s.stop()
	}
	var req *capiv1.Request
	switch {
	case s.browser != nil:
		req = capiv1.Request_builder{CancelAccountSignIn: capiv1.CancelAccountSignIn_builder{Request: s.browser.GetRequest()}.Build()}.Build()
	case s.link != nil:
		req = capiv1.Request_builder{CancelLink: capiv1.CancelLink_builder{Request: s.link.GetRequest()}.Build()}.Build()
	case s.quick != nil:
		req = capiv1.Request_builder{CancelQuickConnect: capiv1.CancelQuickConnect_builder{Request: s.quick.GetRequest()}.Build()}.Build()
	}
	if req != nil {
		go func() {
			if _, err := a.opts.Core.Call(a.ctx, req); err != nil {
				slog.DebugContext(a.ctx, "give up signing in", "error", err)
			}
		}()
	}
	s.browser, s.link, s.linkQR, s.quick, s.stop = nil, nil, nil, nil, nil
}

// mavioAPI is the API of the mavio.dev to sign the Mavio account in at:
// that of the server connected to, else the app's.
func (a *App) mavioAPI() string {
	if u := a.signIn.server.GetMavioDevUrl(); u != "" {
		return u
	}
	return a.opts.MavioDev
}

// signedIn shows the servers once a sign-in completed: the Mavio
// account's, or the standalone server's.
func (a *App) signedIn(account *capiv1.Account) {
	a.endPending()
	if account.GetId() != "" {
		a.account = account
	}
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build())
		a.opts.Update(func() {
			if err != nil {
				slog.ErrorContext(a.ctx, "list the servers", "error", err)
			}
			a.signIn = signIn{}
			a.setServers(m.GetServers().GetServers())
			a.router.Replace("/home")
		})
	}()
}

// signInView builds the sign-in, over the whole window.
func (a *App) signInView(c *ui.Context) {
	s := &a.signIn
	l := cardLook
	if a.prefs.TV {
		l = tvLook
		if s.step != stepLink {
			a.endPending()
			*s = signIn{step: stepLink}
		}
	}
	if c.Shortcut(0, ui.KeyEscape) && len(s.history) > 0 {
		a.back()
	}
	colors := style.Colors(c)
	width, height := c.Size()
	ui.Box(c).Fill().Background(style.Color(colors.Background)).Clip().DragWindow().Children(func() {
		posterWall(c, l.tv, width, height)
		if l.tv {
			ui.Image(c, style.Wordmark).Height(60).AspectRatio(244.0 / 64).Absolute().Top(90).Left(120)
		}
		bar := max(c.TitleBar().Height, 28) + 8
		ui.Column(c).Absolute().Top(bar).Left(0).Right(0).Bottom(0).Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				pad := float32(24)
				if l.tv {
					pad = 90
				}
				ui.Column(c).MinHeight(height - bar).Padding(pad).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					maxWidth, padX, padTop, padBottom, radius := float32(460), float32(40), float32(40), float32(34), float32(26)
					if l.tv {
						maxWidth, padX, padTop, padBottom, radius = 1040, 90, 80, 80, 44
					}
					card := ui.Column(c).Width(min(maxWidth, width-2*pad)).Radius(radius).Material(glass.Blur{Radius: 40})
					if c.Theme().Dark {
						card.Shadow(0, 30, 80, 0, ui.RGBA(0, 0, 0, 0.5))
					} else {
						card.Shadow(0, 30, 80, 0, ui.RGBA(0, 0, 0, 0.14))
					}
					card.Children(func() {
						ui.Column(c).Radius(radius).Background(signInCard(c)).Border(1, signInSeparator(c)).
							Padding(padTop, padX, padBottom).Gap(l.gap).AlignItems(ui.Stretch).Children(func() {
							switch s.step {
							case stepBrowser:
								a.browserView(c, l)
							case stepLink:
								a.linkView(c, l)
							case stepAddress:
								a.addressView(c, l)
							case stepCredentials:
								a.credentialsView(c, l)
							case stepQuickConnect:
								a.quickConnectView(c, l)
							case stepSetUp:
								a.setUpView(c, l)
							default:
								a.mavioView(c, l)
							}
						})
					})
				})
			})
		})
	})
}

func signInCard(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.RGBA(28, 28, 30, 0.82)
	}
	return ui.RGBA(255, 255, 255, 0.84)
}

func signInSeparator(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.RGBA(84, 84, 88, 0.5)
	}
	return ui.RGBA(60, 60, 67, 0.14)
}

func signInField(c *ui.Context) ui.Color {
	if c.Theme().Dark {
		return ui.RGBA(118, 118, 128, 0.12)
	}
	return ui.RGBA(255, 255, 255, 0.9)
}

// tones are the colors of the posters, top to bottom: warm, cool and
// dusk, as films are.
var tones = [][2]string{
	{"#FF9F0A", "#6B2400"},
	{"#2A6F97", "#0A1A2E"},
	{"#8E5CF7", "#24104D"},
	{"#F0412F", "#3A0A0A"},
	{"#30B07A", "#0A2E20"},
	{"#FFD34E", "#7A3E00"},
	{"#5AC8FA", "#10325A"},
	{"#FF6482", "#420D26"},
	{"#C7A17A", "#33241A"},
	{"#64D2FF", "#1A2A66"},
	{"#E07A5F", "#2E1A3A"},
}

// posterWall builds the screen behind sign-in: a wall of posters drawn
// from gradients, tilted and blurred under a veil, darker or lighter with
// the theme.
func posterWall(c *ui.Context, tv bool, width, height float32) {
	columns, blur, rotate := 8, float32(2), float32(-8)
	if tv {
		columns, blur, rotate = 10, 30, -6
	}
	dark := c.Theme().Dark
	opacity, veil := float32(0.6), ui.RGBA(255, 255, 255, 0.6)
	if dark {
		opacity, veil = 0.7, ui.RGBA(0, 0, 0, 0.6)
	}
	w, h := width*1.5, height*1.5
	gap := w * 0.01
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Clip().PassThrough().Children(func() {
		ui.Row(c).Absolute().Left(-width*0.25).Top(-height*0.25).Width(w).Height(h).Gap(gap).
			Rotate(rotate).Scale(1.15, 1.15).Blur(blur).Opacity(opacity).Children(func() {
			for col := range columns {
				column := ui.Column(c).Grow(1).Gap(gap)
				if col%2 == 1 {
					column.Translate(0, -w*0.06)
				}
				column.Children(func() {
					for r := range 9 {
						t := tones[(col*4+r*7)%len(tones)]
						ui.Box(c).AspectRatio(2.0 / 3).Shrink(0).Radius(8).
							LinearGradient(ui.LinearGradient{From: ui.Hex(t[0]), To: ui.Hex(t[1]), Angle: 165}).Children(func() {
							ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(8).
								LinearGradient(ui.LinearGradient{From: ui.RGBA(255, 255, 255, 0.28), To: ui.RGBA(255, 255, 255, 0), Angle: 160, End: 0.55})
						})
					}
				})
			}
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(veil)
	})
}

// steps builds a step's back link, eyebrow, title and the line under it.
func (a *App) steps(c *ui.Context, l look, eyebrow, title, subtitle string) {
	colors := style.Colors(c)
	if len(a.signIn.history) > 0 && !l.tv {
		b := ui.ButtonBase(c).Gap(2).AlignItems(ui.Center).AlignSelf(ui.Start).Label(style.T(c, "action.back"))
		if b.Hovered() {
			b.Opacity(0.8)
		}
		b.Children(func() {
			ui.Icon(c, style.Icon("back", false)).Size(18, 18).TextColor(style.Color(colors.AccentText))
			ui.Text(c, style.T(c, "action.back")).FontSize(15).TextColor(style.Color(colors.AccentText))
		})
		if b.Clicked() {
			a.back()
		}
	}
	if title == "" {
		return
	}
	ui.Column(c).Gap(6).Children(func() {
		if eyebrow != "" {
			ui.Text(c, eyebrow).FontSize(l.label).FontWeight(600).TextColor(style.Color(colors.TextTertiary))
		}
		ui.Text(c, title).FontSize(l.title).FontWeight(700).TextColor(style.Color(colors.TextPrimary))
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(l.subtitle).TextColor(style.Color(colors.TextTertiary)).SingleLine()
		}
	})
}

func body(c *ui.Context, l look, text string) ui.Element {
	return ui.Text(c, text).FontSize(l.body).TextColor(style.Color(style.Colors(c).TextSecondary))
}

// action builds a button of sign-in, as wide as its step.
func action(c *ui.Context, l look, primary bool, icon, label string) ui.Element {
	colors := style.Colors(c)
	fill, text := colors.Fill, colors.TextPrimary
	if primary {
		fill, text = colors.Button, colors.OnButton
	}
	b := ui.ButtonBase(c.Key(label)).MinHeight(l.button).Padding(0, 20).Gap(8).Radius(l.buttonRadius).
		AlignItems(ui.Center).Justify(ui.Center).Background(style.Color(fill)).Label(label)
	switch {
	case b.IsDisabled():
		b.Opacity(0.5)
	case b.Pressed():
		b.Opacity(0.8)
	case b.Hovered() && !primary:
		b.Background(style.Color(colors.FillSelected))
	case b.Hovered():
		b.Opacity(0.9)
	}
	b.Children(func() {
		if icon != "" {
			ui.Icon(c, style.Icon(icon, false)).Size(l.icon, l.icon).TextColor(style.Color(text))
		}
		ui.Text(c, label).FontSize(l.body + 1).FontWeight(600).TextColor(style.Color(text)).SingleLine()
	})
	return b
}

// or builds a line between ways of going on, broken by "or".
func or(c *ui.Context) {
	colors := style.Colors(c)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(signInSeparator(c))
		ui.Text(c, style.T(c, "auth.or")).FontSize(13).TextColor(style.Color(colors.TextTertiary))
		ui.Box(c).Height(1).Grow(1).Background(signInSeparator(c))
	})
}

func field(c *ui.Context, l look, label string, value *string, password, focus bool) ui.Element {
	colors := style.Colors(c)
	var in ui.Element
	k := c.Key(label)
	ui.Column(k).Gap(8).Children(func() {
		ui.Text(c, label).FontSize(l.label).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
		in = ui.TextInput(c, value).Height(l.field).FontSize(l.fieldText).Padding(0, 14).Radius(10).
			Background(signInField(c)).Border(1, signInSeparator(c)).Label(label)
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
		ui.Text(c, errorMessage(c, err)).FontSize(15).TextColor(style.Color(style.Colors(c).Danger))
	}
}

func waiting(c *ui.Context, l look) {
	ui.Row(c).Gap(8).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		activityIndicator(c, 28, time.Time{})
		body(c, l, style.T(c, "auth.waiting"))
	})
}

// codeBoxes shows a code to enter on another device, a character to a
// box.
func codeBoxes(c *ui.Context, l look, code string) {
	colors := style.Colors(c)
	ui.Row(c).Gap(l.gap / 3.5).Justify(ui.Center).AlignItems(ui.Center).Label(code).Children(func() {
		for i, r := range code {
			if r == '-' {
				ui.Text(c.Key(i), "–").Width(12).TextAlign(ui.Center).TextColor(style.Color(colors.TextTertiary))
				continue
			}
			ui.Box(c.Key(i)).Width(l.digitW).Height(l.digitH).Radius(l.buttonRadius).Background(style.Color(colors.Fill)).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, string(r)).Font("monospace").FontSize(l.digit).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
			})
		}
	})
}

// mavioView offers signing a Mavio account in, in the browser or with a
// link code, and connecting to a server by its address.
func (a *App) mavioView(c *ui.Context, l look) {
	s := &a.signIn
	colors := style.Colors(c)
	if len(s.history) > 0 {
		a.steps(c, l, "", "", "")
	}
	ui.Column(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Image(c, style.Wordmark).Height(36).Margin(0, 0, 12)
		ui.Text(c, style.T(c, "auth.welcome")).FontSize(l.title).FontWeight(700).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center)
		if s.server != nil {
			ui.Text(c, s.server.GetName()).FontSize(l.subtitle).TextColor(style.Color(colors.TextTertiary)).SingleLine()
		}
	})
	errorLine(c, s.err)
	if action(c, l, true, "", style.T(c, "auth.signInWithMavio")).Disabled(s.busy).Clicked() && !s.busy {
		a.startBrowserSignIn()
	}
	if action(c, l, false, "devices", style.T(c, "auth.useLinkCode")).Disabled(s.busy).Clicked() && !s.busy {
		a.goTo(stepLink)
	}
	if s.server == nil {
		or(c)
		if action(c, l, false, "server", style.T(c, "auth.connectToServer")).Clicked() {
			a.goTo(stepAddress)
		}
	}
}

// startBrowserSignIn opens mavio.dev's sign-in in the browser and waits
// for it to come back.
func (a *App) startBrowserSignIn() {
	s := &a.signIn
	s.busy, s.err = true, nil
	api := a.mavioAPI()
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{
			StartAccountSignIn: capiv1.StartAccountSignIn_builder{Api: api, ClientId: ClientID}.Build(),
		}.Build())
		a.opts.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			b := m.GetAccountSignIn()
			ctx, cancel := context.WithCancel(a.ctx)
			a.goTo(stepBrowser)
			s.browser, s.stop = b, cancel
			a.openURL(b.GetUrl())
			go a.waitForBrowser(ctx, b.GetRequest())
		})
	}()
}

func (a *App) waitForBrowser(ctx context.Context, request uint64) {
	m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{
		FinishAccountSignIn: capiv1.FinishAccountSignIn_builder{Request: request}.Build(),
	}.Build())
	if ctx.Err() != nil {
		return
	}
	a.opts.Update(func() {
		s := &a.signIn
		if s.browser.GetRequest() != request {
			return
		}
		s.browser, s.stop = nil, nil
		if err != nil {
			a.back()
			s.err = err
			return
		}
		a.signedIn(m.GetAccount())
	})
}

func (a *App) openURL(u string) {
	if a.opts.OpenURL == nil {
		return
	}
	if err := a.opts.OpenURL(u); err != nil {
		slog.ErrorContext(a.ctx, "open the browser", "url", u, "error", err)
		a.signIn.err = err
	}
}

// browserView waits for the browser to come back from mavio.dev.
func (a *App) browserView(c *ui.Context, l look) {
	s := &a.signIn
	host := hostOf(s.browser.GetUrl())
	a.steps(c, l, "", style.T(c, "auth.continueInBrowser"), "")
	body(c, l, style.T(c, "auth.continueInBrowserMessage", "host", host))
	errorLine(c, s.err)
	waiting(c, l)
	if action(c, l, false, "external", style.T(c, "auth.openBrowserAgain")).Clicked() && s.browser != nil {
		a.openURL(s.browser.GetUrl())
	}
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return p.Host
}

// startLink asks mavio.dev for a link code and polls it until it is
// approved, asking for another as it expires.
func (a *App) startLink() {
	s := &a.signIn
	s.busy, s.err = true, nil
	api := a.mavioAPI()
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{
			StartLink: capiv1.StartLink_builder{Api: api, ClientId: ClientID}.Build(),
		}.Build())
		a.opts.Update(func() {
			s.busy = false
			if s.step != stepLink {
				return
			}
			if err != nil {
				s.err = err
				return
			}
			link := m.GetLinkCode()
			ctx, cancel := context.WithCancel(a.ctx)
			s.link, s.linkQR, s.stop = link, qrCode(link.GetVerificationUriComplete()), cancel
			go a.pollLink(ctx, link)
		})
	}()
}

func (a *App) pollLink(ctx context.Context, link *capiv1.LinkCode) {
	t := time.NewTicker(max(time.Duration(link.GetIntervalMs())*time.Millisecond, time.Second))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if time.Now().UnixMilli() > link.GetExpiresMs() {
			a.opts.Update(func() {
				if a.signIn.link == link {
					a.endPending()
					a.startLink()
				}
			})
			return
		}
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{PollLink: capiv1.PollLink_builder{Request: link.GetRequest()}.Build()}.Build())
		if ctx.Err() != nil {
			return
		}
		switch {
		case err != nil:
			a.opts.Update(func() {
				if a.signIn.link == link {
					a.signIn.link, a.signIn.stop, a.signIn.err = nil, nil, err
				}
			})
			return
		case m.HasAccount():
			a.opts.Update(func() {
				if a.signIn.link == link {
					a.signIn.link, a.signIn.stop = nil, nil
					a.signedIn(m.GetAccount())
				}
			})
			return
		}
	}
}

// qrCode is the QR code of text, black on a transparent ground.
func qrCode(text string) *ui.SVG {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><path fill="#000" d="`, code.Size, code.Size)
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	svg, err := ui.ParseSVG([]byte(b.String()))
	if err != nil {
		return nil
	}
	return svg
}

// linkView shows the link code to approve at mavio.dev/link, and its QR
// code, which opens the page with the code filled in.
func (a *App) linkView(c *ui.Context, l look) {
	s := &a.signIn
	colors := style.Colors(c)
	if s.link == nil && !s.busy && s.err == nil {
		a.startLink()
	}
	a.steps(c, l, "", style.T(c, "auth.signInWithMavio"), s.server.GetName())
	if link := s.link; link != nil {
		where := strings.TrimPrefix(strings.TrimPrefix(link.GetVerificationUri(), "https://"), "http://")
		body(c, l, style.T(c, "auth.linkMessage", "url", where))
		codeBoxes(c, l, link.GetUserCode())
		if s.linkQR != nil {
			ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Padding(l.qr / 14).Radius(l.buttonRadius).Background(ui.RGB(255, 255, 255)).Children(func() {
					ui.Image(c, s.linkQR).Size(l.qr, l.qr).Label(link.GetVerificationUriComplete())
				})
				ui.Text(c, style.T(c, "auth.scanLinkCode")).FontSize(l.subtitle).TextColor(style.Color(colors.TextTertiary)).TextAlign(ui.Center)
			})
		}
	}
	if s.err != nil {
		errorLine(c, s.err)
		if action(c, l, false, "", style.T(c, "auth.tryAgain")).Clicked() {
			s.err = nil
		}
		return
	}
	waiting(c, l)
}

// addressView asks for a server's address, offering those on the
// network.
func (a *App) addressView(c *ui.Context, l look) {
	s := &a.signIn
	colors := style.Colors(c)
	a.steps(c, l, "", style.T(c, "auth.connectToServer"), "")
	body(c, l, style.T(c, "auth.serverAddressMessage"))
	in := field(c, l, style.T(c, "auth.server"), &s.address, false, true).Placeholder("http://192.168.1.5:8686")
	errorLine(c, s.err)
	ok := !s.busy && strings.TrimSpace(s.address) != ""
	next := action(c, l, true, "", style.T(c, "action.continue")).Disabled(!ok)
	if (in.Submitted() || next.Clicked()) && ok {
		a.probe(strings.TrimSpace(s.address))
	}
	found := load(a, &s.found, "discover", func(ctx context.Context) ([]*capiv1.FoundServer, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{Discover: capiv1.Discover_builder{WaitMs: uint32(discoverWait.Milliseconds())}.Build()}.Build())
		return m.GetFoundServers().GetServers(), err
	})
	if len(found.value) == 0 {
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, style.T(c, "auth.nearby")).FontSize(13).FontWeight(600).TextColor(style.Color(colors.TextTertiary))
		ui.Column(c).Radius(12).Clip().Background(signInField(c)).Children(func() {
			for i, f := range found.value {
				if i > 0 {
					ui.Box(c.Key(i)).Height(1).Margin(0, 0, 0, 48).Background(signInSeparator(c))
				}
				row := ui.ButtonBase(c.Key(f.GetId())).Padding(10, 14).Gap(12).AlignItems(ui.Center).Label(f.GetName())
				if row.Hovered() {
					row.Background(style.Color(colors.FillSelected))
				}
				row.Children(func() {
					ui.Icon(c, style.Icon("server", false)).Size(22, 22).TextColor(style.Color(colors.Accent))
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, f.GetName()).FontSize(15).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).SingleLine()
						ui.Text(c, f.GetAddress()).FontSize(13).TextColor(style.Color(colors.TextSecondary)).SingleLine()
					})
				})
				if row.Clicked() && !s.busy {
					s.address = f.GetAddress()
					a.probe(f.GetAddress())
				}
			}
		})
	})
}

// probe asks the server at address who it is, and goes on to the way it
// signs people in: the Mavio account's sign-in at its mavio.dev, its
// setup in the browser, or its users' credentials.
func (a *App) probe(address string) {
	s := &a.signIn
	s.busy, s.err = true, nil
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Probe: capiv1.Probe_builder{Address: address}.Build()}.Build())
		a.opts.Update(func() {
			s.busy = false
			if s.step != stepAddress {
				return
			}
			if err != nil {
				s.err = err
				return
			}
			info := m.GetServerInfo()
			s.server, s.address = info, info.GetAddress()
			switch {
			case info.GetMode() == capiv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV:
				a.goTo(stepMavio)
			case info.GetSetupRequired():
				a.goTo(stepSetUp)
			default:
				a.goTo(stepCredentials)
			}
		})
	}()
}

// setUpView sends a server not set up yet to the browser, where its setup
// chooses its mode.
func (a *App) setUpView(c *ui.Context, l look) {
	s := &a.signIn
	colors := style.Colors(c)
	a.steps(c, l, style.T(c, "auth.newServerSetup"), style.T(c, "auth.setUpElsewhere", "server", s.server.GetName()), "")
	body(c, l, style.T(c, "auth.setUpElsewhereMessage"))
	ui.Text(c, s.server.GetAddress()).Font("monospace").FontSize(l.title - 6).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).Selectable()
	if action(c, l, true, "external", style.T(c, "auth.openInBrowser")).Clicked() {
		a.openURL(s.server.GetAddress())
	}
}

// credentialsView asks for the name and password of a user of a
// standalone server, or offers Quick Connect.
func (a *App) credentialsView(c *ui.Context, l look) {
	s := &a.signIn
	subtitle := style.T(c, "auth.standaloneServer") + " · " + s.server.GetAddress()
	a.steps(c, l, "", style.T(c, "auth.signInTo", "server", s.server.GetName()), subtitle)
	field(c, l, style.T(c, "auth.username"), &s.name, false, true)
	pw := field(c, l, style.T(c, "auth.password"), &s.password, true, false)
	errorLine(c, s.err)
	ok := strings.TrimSpace(s.name) != "" && !s.busy
	submit := action(c, l, true, "", style.T(c, "auth.signIn")).Disabled(!ok)
	if (submit.Clicked() || pw.Submitted()) && ok {
		a.submitCredentials()
	}
	or(c)
	if action(c, l, false, "devices", style.T(c, "auth.quickConnect")).Disabled(s.busy).Clicked() && !s.busy {
		a.startQuickConnect()
	}
}

func (a *App) submitCredentials() {
	s := &a.signIn
	s.busy, s.err = true, nil
	address, name, password := s.address, strings.TrimSpace(s.name), s.password
	go func() {
		_, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{
			SignIn: capiv1.SignIn_builder{Address: address, Name: name, Password: password}.Build(),
		}.Build())
		a.opts.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			a.signedIn(nil)
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
			a.goTo(stepQuickConnect)
			s.quick, s.stop = q, cancel
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
			a.opts.Update(func() {
				if a.signIn.quick.GetRequest() == request {
					a.signIn.quick, a.signIn.stop, a.signIn.err = nil, nil, err
				}
			})
			return
		case m.HasServer():
			a.opts.Update(func() {
				if a.signIn.quick.GetRequest() == request {
					a.signIn.quick, a.signIn.stop = nil, nil
					a.signedIn(nil)
				}
			})
			return
		}
	}
}

// quickConnectView shows the Quick Connect code while waiting for it to be
// entered.
func (a *App) quickConnectView(c *ui.Context, l look) {
	s := &a.signIn
	a.steps(c, l, "", style.T(c, "auth.quickConnectTitle"), s.server.GetName())
	if q := s.quick; q != nil {
		body(c, l, style.T(c, "auth.quickConnectCode", "code", q.GetCode()))
		codeBoxes(c, l, q.GetCode())
	}
	if s.err != nil {
		errorLine(c, s.err)
		if action(c, l, false, "", style.T(c, "auth.tryAgain")).Clicked() {
			a.back()
			a.startQuickConnect()
		}
		return
	}
	waiting(c, l)
}
