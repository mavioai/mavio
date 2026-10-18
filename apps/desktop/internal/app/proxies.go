package app

import (
	"context"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// systemProxyID is the ID that chooses the system proxy.
const systemProxyID = "system"

// proxiesState is the state of the settings' Proxies: the proxy edited,
// and the tests of each proxy, by its ID ("" for the one edited).
type proxiesState struct {
	list    task[[]*systemv1.Proxy]
	editing *string // the ID of the proxy edited, "" for a new one
	draft   proxyDraft
	tests   map[string]proxyTest
	busy    string
	err     error
	// confirming asks before deleting the proxy edited.
	confirming bool
	choosing   map[string]string // the server's requests' proxies being saved
	chooseErr  error             // of saving one
}

// proxyTest is a test of a proxy, running or done.
type proxyTest struct {
	running bool
	res     *systemv1.TestProxyResponse
	err     error
}

// proxyDraft is what a proxy's form holds.
type proxyDraft struct {
	name, kind, host, username, password string
	port                                 float64
	clearPassword                        bool
}

// proxyKinds are the kinds of proxies, by the scheme of their addresses.
var proxyKinds = []struct {
	value string
	kind  systemv1.ProxyKind
	label string
}{
	{"http", systemv1.ProxyKind_PROXY_KIND_HTTP, "HTTP"},
	{"socks4", systemv1.ProxyKind_PROXY_KIND_SOCKS4, "SOCKS4"},
	{"socks5", systemv1.ProxyKind_PROXY_KIND_SOCKS5, "SOCKS5"},
}

// proxyAddress returns a proxy's address, kind://host:port.
func proxyAddress(p *systemv1.Proxy) string {
	scheme := ""
	for _, k := range proxyKinds {
		if k.kind == p.GetKind() {
			scheme = k.value
		}
	}
	return scheme + "://" + net.JoinHostPort(p.GetHost(), strconv.Itoa(int(p.GetPort())))
}

func (a *App) proxyService(id string) systemv1connect.ProxyServiceClient {
	return systemv1connect.NewProxyServiceClient(a.connectHTTP(), serverURL(id))
}

// proxies loads the proxies of the server chosen: the system proxy, then
// the others by name.
func (a *App) proxies() *task[[]*systemv1.Proxy] {
	ps := &a.settingsState.admin.proxies
	return adminTask(a, &ps.list, "proxies", func(ctx context.Context, s string) ([]*systemv1.Proxy, error) {
		res, err := a.proxyService(s).ListProxies(ctx, &systemv1.ListProxiesRequest{})
		return res.GetProxies(), err
	})
}

// proxyChoices returns the choices of a proxy: none, the system proxy,
// then the server's by name; one chosen but gone by its ID.
func proxyChoices(c *ui.Context, list []*systemv1.Proxy, current string) (ids, labels []string) {
	ids, labels = []string{"", systemProxyID}, []string{style.T(c, "proxies.none"), style.T(c, "proxies.system")}
	own := slices.DeleteFunc(slices.Clone(list), func(p *systemv1.Proxy) bool { return p.GetSystem() })
	slices.SortFunc(own, func(x, y *systemv1.Proxy) int {
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	})
	for _, p := range own {
		ids, labels = append(ids, p.GetId()), append(labels, p.GetName())
	}
	if !slices.Contains(ids, current) {
		ids, labels = append(ids, current), append(labels, style.T(c, "proxies.unknown"))
	}
	return ids, labels
}

func (a *App) proxiesSection(c *ui.Context) {
	ps := &a.settingsState.admin.proxies
	sectionHeader(c, style.T(c, "settings.proxies"), style.T(c, "proxies.message"))
	t := a.proxies()
	if taskStatus(c, t, func() { ps.list.key = "" }, nil) {
		return
	}
	var system *systemv1.Proxy
	var own []*systemv1.Proxy
	for _, p := range t.value {
		if p.GetSystem() {
			system = p
		} else {
			own = append(own, p)
		}
	}
	if system != nil {
		group(c, "system", "", style.T(c, "proxies.systemFooter"), func(g *settingsGroup) {
			color, detail := ui.RGB(0x8E, 0x8E, 0x93), style.T(c, "proxies.systemUnset")
			if system.GetKind() != systemv1.ProxyKind_PROXY_KIND_UNSPECIFIED {
				color, detail = ui.RGB(0x00, 0x7A, 0xFF), proxyAddress(system)
				if len(system.GetBypass()) > 0 {
					detail += " · " + style.T(c, "proxies.bypass", "hosts", strings.Join(system.GetBypass(), ", "))
				}
			}
			a.proxyLine(g, system, color, style.T(c, "proxies.system"), detail, false)
		})
	}
	group(c, "proxies", style.T(c, "proxies.list"), "", func(g *settingsGroup) {
		if ps.editing != nil && *ps.editing == "" {
			a.proxyForm(g, nil)
		}
		for _, p := range own {
			if ps.editing != nil && *ps.editing == p.GetId() {
				a.proxyForm(g, p)
				continue
			}
			detail := proxyAddress(p)
			if p.GetUsername() != "" {
				detail += " · " + p.GetUsername()
			}
			if a.proxyLine(g, p, ui.RGB(0x30, 0xB0, 0xC7), p.GetName(), detail, ps.editing == nil) {
				a.editProxy(p)
			}
		}
		if len(own) == 0 && (ps.editing == nil || *ps.editing != "") {
			g.note(style.T(c, "proxies.empty"), false)
		}
		if ps.editing == nil && g.addRow(style.T(c, "proxies.add")) {
			a.editProxy(nil)
		}
	})
	a.serverRequests(c, t.value)
}

// editProxy opens the form of a proxy, or of a new one when p is nil.
func (a *App) editProxy(p *systemv1.Proxy) {
	ps := &a.settingsState.admin.proxies
	id := p.GetId()
	ps.editing, ps.err = &id, nil
	delete(ps.tests, "")
	ps.draft = proxyDraft{name: p.GetName(), kind: "http", host: p.GetHost(), port: 8080, username: p.GetUsername()}
	if p != nil {
		ps.draft.port = float64(p.GetPort())
		for _, k := range proxyKinds {
			if k.kind == p.GetKind() {
				ps.draft.kind = k.value
			}
		}
	}
}

// proxyLine adds a proxy's line to g: its tile, name and address, which a
// click edits when editable, and its test with the result under it. It
// reports a click.
func (a *App) proxyLine(g *settingsGroup, p *systemv1.Proxy, color ui.Color, name, detail string, editable bool) bool {
	c := g.c
	ps := &a.settingsState.admin.proxies
	colors := style.Colors(c)
	g.divider()
	clicked := false
	ui.Column(c.Key(g.rows)).Children(func() {
		ui.Row(c).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			b := ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(name).Disabled(!editable)
			b.Children(func() {
				sectionTile(c, "network", color)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					t := style.Text(ui.Text(c, name), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
					if editable && b.Hovered() {
						t.Underline()
					}
					style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
				})
			})
			clicked = editable && b.Clicked()
			id := p.GetId()
			a.proxyTestButton(c, id, func() *systemv1.TestProxyRequest {
				return systemv1.TestProxyRequest_builder{Id: &id}.Build()
			})
		})
		proxyTestResult(c, ps.tests[p.GetId()])
	})
	return clicked
}

// proxyTestButton builds the button testing a proxy, whose test is keyed
// key, of the request req makes.
func (a *App) proxyTestButton(c *ui.Context, key string, req func() *systemv1.TestProxyRequest) {
	ps := &a.settingsState.admin.proxies
	test := ps.tests[key]
	label := style.T(c, "proxies.test")
	if test.running {
		label = "…"
	}
	if !smallButton(c.Key("test"+key), label, test.running) {
		return
	}
	if ps.tests == nil {
		ps.tests = map[string]proxyTest{}
	}
	ps.tests[key] = proxyTest{running: true}
	r, client := req(), a.proxyService(a.chosenServer().GetId())
	go func() {
		res, err := client.TestProxy(a.ctx, r)
		a.opts.Update(func() { ps.tests[key] = proxyTest{res: res, err: err} })
	}()
}

var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// proxyTestResult builds a test's result: how long the site took through
// the proxy, the exit address and its country, or why it failed.
func proxyTestResult(c *ui.Context, t proxyTest) {
	colors := style.Colors(c)
	failed := func(err string) {
		style.Text(ui.Text(c, style.T(c, "proxies.testFailed", "error", err)), style.Type(c).Footnote).
			TextColor(style.Color(colors.Danger)).Padding(0, 16, 10).Selectable()
	}
	switch {
	case t.err != nil:
		failed(errorMessage(c, t.err))
	case t.res == nil:
	case !t.res.GetOk():
		failed(t.res.GetError())
	default:
		parts := []string{style.T(c, "proxies.testOk", "ms", t.res.GetLatency().AsDuration().Milliseconds())}
		if ip := t.res.GetExitIp(); ip != "" {
			parts = append(parts, style.T(c, "proxies.exit", "address", ip))
		}
		if country := t.res.GetCountry(); countryCode.MatchString(country) {
			parts = append(parts, regionName(c, country))
		} else if country != "" {
			parts = append(parts, country)
		}
		style.Text(ui.Text(c, strings.Join(parts, " · ")), style.Type(c).Footnote).TextColor(style.Color(colors.Success)).Padding(0, 16, 10)
	}
}

// proxyForm adds the form adding a proxy, or editing p and deleting it.
func (a *App) proxyForm(g *settingsGroup, p *systemv1.Proxy) {
	c := g.c
	ps := &a.settingsState.admin.proxies
	d := &ps.draft
	server := a.chosenServer().GetId()
	text := func(value *string, label, placeholder string) {
		g.row(label, "", nil, func() {
			ui.TextInput(c, value).Width(288).Label(label).Placeholder(placeholder)
		})
	}
	text(&d.name, style.T(c, "proxies.name"), "")
	g.row(style.T(c, "proxies.kind"), "", nil, func() {
		values, labels := make([]string, len(proxyKinds)), make([]string, len(proxyKinds))
		for i, k := range proxyKinds {
			values[i], labels[i] = k.value, k.label
		}
		chooseString(c, style.T(c, "proxies.kind"), &d.kind, values, labels)
	})
	text(&d.host, style.T(c, "proxies.host"), "proxy.lan")
	g.row(style.T(c, "proxies.port"), "", nil, func() {
		ui.NumberInput(c, &d.port, 1, 65535, 1).Label(style.T(c, "proxies.port"))
	})
	text(&d.username, style.T(c, "proxies.username"), "")
	g.row(style.T(c, "proxies.password"), "", nil, func() {
		in := ui.TextInput(c, &d.password).Width(288).Password().Label(style.T(c, "proxies.password"))
		if p.GetHasPassword() && !d.clearPassword {
			in.Placeholder(style.T(c, "proxies.passwordKept"))
		}
	})
	if p.GetHasPassword() && d.password == "" {
		g.switchRow(style.T(c, "proxies.clearPassword"), "", &d.clearPassword)
	}
	message := func() *systemv1.Proxy {
		kind := systemv1.ProxyKind_PROXY_KIND_HTTP
		for _, k := range proxyKinds {
			if k.value == d.kind {
				kind = k.kind
			}
		}
		return systemv1.Proxy_builder{
			Id: new(p.GetId()), Name: new(strings.TrimSpace(d.name)), Kind: &kind, Host: new(strings.TrimSpace(d.host)),
			Port: new(int32(d.port)), Username: new(strings.TrimSpace(d.username)), Password: new(d.password),
		}.Build()
	}
	valid := strings.TrimSpace(d.name) != "" && strings.TrimSpace(d.host) != "" && d.port >= 1 && d.port <= 65535
	action := func(what string, fn func(ctx context.Context, s systemv1connect.ProxyServiceClient) error) {
		ps.busy, ps.err = what, nil
		client := a.proxyService(server)
		go func() {
			err := fn(a.ctx, client)
			a.opts.Update(func() {
				ps.busy, ps.err = "", err
				if err == nil {
					ps.editing = nil
					a.settingsState.admin.reload++
				}
			})
		}()
	}
	g.block(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			label := style.T(c, "action.save")
			if ps.busy == "save" {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(!valid || ps.busy != "").Clicked() {
				m := message()
				action("save", func(ctx context.Context, s systemv1connect.ProxyServiceClient) error {
					if p == nil {
						_, err := s.CreateProxy(ctx, systemv1.CreateProxyRequest_builder{Proxy: m}.Build())
						return err
					}
					clear := d.clearPassword && d.password == ""
					_, err := s.UpdateProxy(ctx, systemv1.UpdateProxyRequest_builder{Proxy: m, ClearPassword: &clear}.Build())
					return err
				})
			}
			if valid {
				a.proxyTestButton(c, "", func() *systemv1.TestProxyRequest {
					return systemv1.TestProxyRequest_builder{Proxy: message()}.Build()
				})
			}
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				ps.editing = nil
			}
			if p != nil && dangerButton(c, style.T(c, "proxies.remove"), ps.busy != "") {
				ps.confirming = true
			}
			if ps.err != nil {
				dangerText(c, errorMessage(c, ps.err))
			}
		})
		proxyTestResult(c, ps.tests[""])
	})
	if p == nil {
		return
	}
	title := style.T(c, "proxies.removeTitle", "name", p.GetName())
	if ui.AlertDialog(c, &ps.confirming, title, style.T(c, "proxies.removeMessage"), style.T(c, "action.cancel"), style.T(c, "proxies.remove")) == 1 {
		id := p.GetId()
		action("delete", func(ctx context.Context, s systemv1connect.ProxyServiceClient) error {
			_, err := s.DeleteProxy(ctx, systemv1.DeleteProxyRequest_builder{Id: &id}.Build())
			return err
		})
	}
}

// outboundRequests are what the server's own requests reach, each with
// its proxy; mavio.dev and its certificates only in mavio.dev mode.
var outboundRequests = []struct {
	key, label, detail string
	mavioDev           bool
	get                func(*systemv1.OutboundProxies) string
	set                func(*systemv1.OutboundProxies, string)
}{
	{
		"catalogs", "proxies.forCatalogs", "proxies.forCatalogsDetail", false,
		(*systemv1.OutboundProxies).GetPluginCatalogs, (*systemv1.OutboundProxies).SetPluginCatalogs,
	},
	{
		"images", "proxies.forImages", "proxies.forImagesDetail", false,
		(*systemv1.OutboundProxies).GetImages, (*systemv1.OutboundProxies).SetImages,
	},
	{
		"mavio-dev", "proxies.forMavioDev", "proxies.forMavioDevDetail", true,
		(*systemv1.OutboundProxies).GetMavioDev, (*systemv1.OutboundProxies).SetMavioDev,
	},
	{
		"acme", "proxies.forAcme", "proxies.forAcmeDetail", true,
		(*systemv1.OutboundProxies).GetAcme, (*systemv1.OutboundProxies).SetAcme,
	},
}

// serverRequests builds the proxies of the server's own requests, each
// saved as it is chosen.
func (a *App) serverRequests(c *ui.Context, list []*systemv1.Proxy) {
	ps := &a.settingsState.admin.proxies
	ss := &a.settingsState.admin.server
	t := a.serverSettings()
	if !t.loaded || t.err != nil {
		return
	}
	current := t.value.GetSettings().GetNetwork().GetProxies()
	footer := style.T(c, "proxies.serverRequestsFooter")
	if ps.chooseErr != nil {
		footer = errorMessage(c, ps.chooseErr)
	}
	group(c, "requests", style.T(c, "proxies.serverRequests"), footer, func(g *settingsGroup) {
		for _, r := range outboundRequests {
			if r.mavioDev && a.account == nil {
				continue
			}
			value, ok := ps.choosing[r.key]
			if !ok {
				value = r.get(current)
			}
			g.row(style.T(c, r.label), style.T(c, r.detail), nil, func() {
				ids, labels := proxyChoices(c, list, value)
				i := slices.Index(ids, value)
				if !choose(c, style.T(c, r.label), &i, labels) || ss.busy != "" {
					return
				}
				id := ids[i]
				if ps.choosing == nil {
					ps.choosing = map[string]string{}
				}
				ps.choosing[r.key] = id
				a.updateServerSettings(func(s *systemv1.ServerSettings) {
					if !s.HasNetwork() {
						s.SetNetwork(&systemv1.NetworkSettings{})
					}
					n := s.GetNetwork()
					if !n.HasProxies() {
						n.SetProxies(&systemv1.OutboundProxies{})
					}
					r.set(n.GetProxies(), id)
				}, func(err error) {
					delete(ps.choosing, r.key)
					ps.chooseErr = err
				})
			})
		}
	})
}
