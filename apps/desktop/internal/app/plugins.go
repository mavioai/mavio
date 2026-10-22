package app

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// pluginsState is the state of the settings' Plugins.
type pluginsState struct {
	installed task[[]*systemv1.Plugin]
	catalog   task[[]*systemv1.CatalogPlugin]
	logs      task[[]*systemv1.LogRecord]
	// tab is Available or Installed; category the category shown, -1 for
	// all.
	tab      int
	category int32
	// busy is the action being taken, such as "install:1.2.0"; err why
	// the last failed.
	busy string
	err  error
	// confirming asks before uninstalling; restarting holds the proxy a
	// process plugin restarts to take, once confirmed.
	confirming bool
	restarting *string
	proxyErr   error
	catalogURL string
	config     pluginConfigState
}

// pluginCategories are how the plugins of each category show.
var pluginCategories = map[pluginv1.Category]struct {
	title, icon string
	color       ui.Color
}{
	pluginv1.Category_CATEGORY_UNSPECIFIED:    {"plugins.categoryOther", "plugins", ui.RGB(0x8E, 0x8E, 0x93)},
	pluginv1.Category_CATEGORY_METADATA:       {"plugins.categoryMetadata", "info", ui.RGB(0x00, 0x7A, 0xFF)},
	pluginv1.Category_CATEGORY_SUBTITLES:      {"plugins.categorySubtitles", "subtitles", ui.RGB(0x34, 0xC7, 0x59)},
	pluginv1.Category_CATEGORY_DOWNLOADS:      {"plugins.categoryDownloads", "downloads", ui.RGB(0xFF, 0x95, 0x00)},
	pluginv1.Category_CATEGORY_AUTHENTICATION: {"plugins.categoryAuthentication", "key", ui.RGB(0xFF, 0x2D, 0x55)},
	pluginv1.Category_CATEGORY_NOTIFICATIONS:  {"plugins.categoryNotifications", "pending", ui.RGB(0xFF, 0x3B, 0x30)},
	pluginv1.Category_CATEGORY_DEVICES:        {"plugins.categoryDevices", "television", ui.RGB(0x32, 0xAD, 0xE6)},
	pluginv1.Category_CATEGORY_AI:             {"plugins.categoryAi", "plugins", ui.RGB(0xAF, 0x52, 0xDE)},
	pluginv1.Category_CATEGORY_SPEECH:         {"plugins.categorySpeech", "plugins", ui.RGB(0x58, 0x56, 0xD6)},
}

func pluginCategory(c pluginv1.Category) (title, icon string, color ui.Color) {
	k, ok := pluginCategories[c]
	if !ok {
		k = pluginCategories[pluginv1.Category_CATEGORY_UNSPECIFIED]
	}
	return k.title, k.icon, k.color
}

var pluginStates = map[systemv1.PluginState]string{
	systemv1.PluginState_PLUGIN_STATE_READY:        "plugins.stateReady",
	systemv1.PluginState_PLUGIN_STATE_UNCONFIGURED: "plugins.stateUnconfigured",
	systemv1.PluginState_PLUGIN_STATE_FAILED:       "plugins.stateFailed",
	systemv1.PluginState_PLUGIN_STATE_DISABLED:     "plugins.stateDisabled",
}

func pluginStateName(c *ui.Context, s systemv1.PluginState) string {
	if k, ok := pluginStates[s]; ok {
		return style.T(c, k)
	}
	return ""
}

// pluginEntry is a plugin as shown: what its catalog offers of it and
// what is installed, either or both.
type pluginEntry struct {
	id, name, description, author string
	category                      pluginv1.Category
	offered                       *systemv1.CatalogPlugin
	installed                     *systemv1.Plugin
}

func newPluginEntry(id string, offered *systemv1.CatalogPlugin, installed *systemv1.Plugin) pluginEntry {
	m := installed.GetManifest()
	return pluginEntry{
		id:          id,
		name:        cmpOr(m.GetName(), offered.GetName(), id),
		description: cmpOr(m.GetDescription(), offered.GetDescription()),
		author:      cmpOr(m.GetAuthor(), offered.GetAuthor()),
		category:    cmp.Or(m.GetCategory(), offered.GetCategory()),
		offered:     offered,
		installed:   installed,
	}
}

// update returns the newest version the catalog offers when newer than
// the one installed.
func (e pluginEntry) update() string {
	if e.installed == nil || len(e.offered.GetVersions()) == 0 {
		return ""
	}
	newest, current := e.offered.GetVersions()[0].GetVersion(), e.installed.GetManifest().GetVersion()
	if newest == "" || current == "" || newest == current {
		return ""
	}
	return newest
}

// pluginEntries are the plugins of the server chosen: those its catalogs
// offer and those installed. The installed load again every 2 seconds
// while any is under development, which the server restarts as it is
// built.
type pluginEntries struct {
	installed, catalog bool // loaded
	catalogErr         error
	available, mine    []pluginEntry
	byID               map[string]pluginEntry
}

func (a *App) pluginEntries(c *ui.Context) pluginEntries {
	st := &a.settingsState.admin.plugins
	dev := slices.ContainsFunc(st.installed.value, func(p *systemv1.Plugin) bool { return p.GetDevFolder() != "" })
	var tick int64
	if dev {
		tick = every(c, 2*time.Second)
	}
	installed := adminTask(a, &st.installed, "plugins", func(ctx context.Context, s string) ([]*systemv1.Plugin, error) {
		res, err := a.pluginService(s).ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		return res.GetPlugins(), err
	}, tick)
	catalog := adminTask(a, &st.catalog, "catalog-plugins", func(ctx context.Context, s string) ([]*systemv1.CatalogPlugin, error) {
		res, err := a.pluginService(s).ListCatalogPlugins(ctx, &systemv1.ListCatalogPluginsRequest{})
		return res.GetPlugins(), err
	})
	offered := map[string]*systemv1.CatalogPlugin{}
	for _, p := range catalog.value {
		offered[p.GetId()] = p
	}
	mine := map[string]*systemv1.Plugin{}
	for _, p := range installed.value {
		mine[p.GetManifest().GetId()] = p
	}
	out := pluginEntries{installed: installed.loaded, catalog: catalog.loaded, catalogErr: catalog.err, byID: map[string]pluginEntry{}}
	for _, p := range catalog.value {
		e := newPluginEntry(p.GetId(), p, mine[p.GetId()])
		out.available = append(out.available, e)
		out.byID[e.id] = e
	}
	for _, p := range installed.value {
		id := p.GetManifest().GetId()
		e := newPluginEntry(id, offered[id], p)
		out.mine = append(out.mine, e)
		out.byID[id] = e
	}
	return out
}

// pluginsPath returns the path of the Plugins: their list, a plugin's
// page, or with catalogs the catalogs' page.
func pluginsPath(plugin string, catalogs bool) string {
	switch {
	case catalogs:
		return "/settings/plugins?catalogs=1"
	case plugin != "":
		return "/settings/plugins?" + url.Values{"plugin": {plugin}}.Encode()
	}
	return "/settings/plugins"
}

// pluginsSection builds the settings' Plugins, as the web UI's: their
// list, a plugin's page or the catalogs'.
func (a *App) pluginsSection(c *ui.Context, r *ui.Route) {
	switch {
	case r.Query("catalogs") != "":
		a.pluginCatalogs(c)
	case r.Query("plugin") != "":
		a.pluginPage(c, r.Query("plugin"))
	default:
		a.pluginList(c)
	}
}

func (a *App) pluginList(c *ui.Context) {
	st := &a.settingsState.admin.plugins
	colors := style.Colors(c)
	sectionHeader(c, style.T(c, "settings.plugins"), style.T(c, "settings.pluginsMessage"))
	es := a.pluginEntries(c)
	shown := es.available
	if st.tab == 1 {
		shown = es.mine
	}
	var present []pluginv1.Category
	for _, e := range shown {
		if !slices.Contains(present, e.category) {
			present = append(present, e.category)
		}
	}
	slices.Sort(present)
	if st.category >= 0 && !slices.Contains(present, pluginv1.Category(st.category)) {
		st.category = -1
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		segmented(c, &st.tab, style.T(c, "plugins.available"), style.T(c, "plugins.installed")).Label(style.T(c, "settings.plugins"))
		ui.Box(c).Grow(1)
		labels := []string{style.T(c, "plugins.allCategories")}
		for _, k := range present {
			title, _, _ := pluginCategory(k)
			labels = append(labels, style.T(c, title))
		}
		i := 0
		if st.category >= 0 {
			i = slices.Index(present, pluginv1.Category(st.category)) + 1
		}
		if choose(c.Key("category"), style.T(c, "plugins.category"), &i, labels) {
			st.category = -1
			if i > 0 {
				st.category = int32(present[i-1])
			}
		}
		if smallButton(c, style.T(c, "plugins.manageCatalogs"), false) {
			a.router.Push(pluginsPath("", true))
		}
	})
	if st.tab == 0 && es.catalogErr != nil {
		footnote(c, style.T(c, "plugins.catalogsUnreachable", "reason", errorMessage(c, es.catalogErr)), true)
	}
	var list []pluginEntry
	for _, e := range shown {
		if st.category < 0 || e.category == pluginv1.Category(st.category) {
			list = append(list, e)
		}
	}
	slices.SortFunc(list, func(x, y pluginEntry) int { return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name)) })
	loaded := es.catalog
	if st.tab == 1 {
		loaded = es.installed
	}
	if loaded && len(list) == 0 {
		msg := style.T(c, "plugins.noneAvailable")
		switch {
		case st.category >= 0:
			msg = style.T(c, "plugins.noneInCategory")
		case st.tab == 1:
			msg = style.T(c, "plugins.noneInstalled")
		}
		style.Text(ui.Text(c, msg), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).AlignSelf(ui.Center).Padding(40, 0)
	}
	ui.Grid(c).Columns(2).Gap(16).Children(func() {
		for _, e := range list {
			a.pluginCard(c, e)
		}
	})
}

// pluginTile builds a category's icon on its colored tile.
func pluginTile(c *ui.Context, category pluginv1.Category, size float32) {
	_, icon, color := pluginCategory(category)
	ui.Box(c).Size(size, size).Shrink(0).Radius(size * 0.22).Background(color).Center().Children(func() {
		s := float32(int(size*0.55 + 0.5))
		ui.Icon(c, style.Icon(icon, true)).Size(s, s).TextColor(ui.RGB(255, 255, 255))
	})
}

func (a *App) pluginCard(c *ui.Context, e pluginEntry) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(e.id)).Column().AlignItems(ui.Start).Justify(ui.Start).MinHeight(160).MinWidth(0).Gap(12).Padding(16).Radius(14).Label(e.name).
		Background(style.Color(colors.Surface))
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			pluginTile(c, e.category, 40)
			ui.Column(c).MinWidth(0).Shrink(1).Children(func() {
				style.Text(ui.Text(c, e.name), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
				title, _, _ := pluginCategory(e.category)
				style.Text(ui.Text(c, joinDetail(style.T(c, title), e.installed.GetManifest().GetVersion())), style.Type(c).Footnote).
					TextColor(style.Color(colors.TextSecondary)).SingleLine()
			})
		})
		style.Text(ui.Text(c, e.description), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(3).Grow(1)
		pluginBadge(c, e)
	})
	if b.Clicked() {
		a.router.Push(pluginsPath(e.id, false))
	}
}

// pluginBadge builds a plugin's state as short marks.
func pluginBadge(c *ui.Context, e pluginEntry) {
	colors := style.Colors(c)
	mark := func(text string, color ui.Color) {
		style.Text(ui.Text(c, text), style.Type(c).Caption).FontWeight(600).TextColor(color)
	}
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if e.installed == nil {
			if v := e.offered.GetVersions(); len(v) > 0 {
				mark(v[0].GetVersion(), style.Color(colors.TextSecondary))
			} else {
				mark(style.T(c, "plugins.noVersion"), style.Color(colors.TextTertiary))
			}
			return
		}
		if e.installed.GetDevFolder() != "" {
			mark(style.T(c, "plugins.development"), style.Color(colors.AccentText))
		} else {
			mark(style.T(c, "plugins.installed"), style.Color(colors.TextSecondary))
		}
		color := style.Color(colors.TextSecondary)
		switch e.installed.GetState() {
		case systemv1.PluginState_PLUGIN_STATE_FAILED:
			color = style.Color(colors.Danger)
		case systemv1.PluginState_PLUGIN_STATE_READY:
			color = ui.RGB(0x34, 0xC7, 0x59)
		}
		mark(pluginStateName(c, e.installed.GetState()), color)
		if e.update() != "" {
			mark(style.T(c, "plugins.updateAvailable"), style.Color(colors.TextPrimary))
		}
	})
}

// pluginAction takes an action on the plugin, keyed busy; the plugins
// load again once it ends.
func (a *App) pluginAction(busy string, fn func(ctx context.Context, s string) error) {
	st := &a.settingsState.admin.plugins
	server := a.chosenServer().GetId()
	st.busy, st.err = busy, nil
	go func() {
		err := fn(a.ctx, server)
		a.opts.Update(func() {
			st.busy, st.err = "", err
			a.settingsState.admin.reload++
		})
	}()
}

func (a *App) pluginPage(c *ui.Context, id string) {
	st := &a.settingsState.admin.plugins
	colors := style.Colors(c)
	a.backTo(c, style.T(c, "settings.plugins"), pluginsPath("", false))
	es := a.pluginEntries(c)
	e, ok := es.byID[id]
	if !ok {
		if es.installed && es.catalog {
			style.Text(ui.Text(c, style.T(c, "plugins.notFound")), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
		}
		return
	}
	m := e.installed.GetManifest()
	var newest string
	if v := e.offered.GetVersions(); len(v) > 0 {
		newest = v[0].GetVersion()
	}
	dev := e.installed.GetDevFolder()
	// A plugin under development takes no catalog version.
	update := ""
	if dev == "" {
		update = e.update()
	}
	enabled := e.installed.GetState() != systemv1.PluginState_PLUGIN_STATE_DISABLED
	install := func(version string) {
		a.pluginAction("install:"+version, func(ctx context.Context, s string) error {
			req := systemv1.InstallPluginRequest_builder{PluginId: &id}
			if version != "" {
				req.Version = &version
			}
			_, err := a.pluginService(s).InstallPlugin(ctx, req.Build())
			return err
		})
	}
	installLabel := func(version, label string) string {
		if st.busy == "install:"+version {
			return "…"
		}
		return label
	}
	ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
		pluginTile(c, e.category, 64)
		ui.Column(c).MinWidth(192).Grow(1).Shrink(1).Children(func() {
			style.Text(ui.Text(c, e.name), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary))
			if e.author != "" {
				style.Text(ui.Text(c, style.T(c, "plugins.by", "author", e.author)), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
			}
		})
		ui.Row(c).Gap(12).Shrink(0).AlignItems(ui.Center).Children(func() {
			if e.installed == nil && newest != "" {
				if ui.PrimaryButton(c, installLabel("", style.T(c, "plugins.install"))).Disabled(st.busy != "").Clicked() {
					install("")
				}
			}
			if update != "" {
				if ui.PrimaryButton(c, installLabel(update, style.T(c, "plugins.update", "version", update))).Disabled(st.busy != "").Clicked() {
					install(update)
				}
			}
			if e.installed == nil {
				return
			}
			// Starting or stopping a process plugin takes a while; the switch waits for it.
			on := enabled
			toggling := strings.HasPrefix(st.busy, "enable:")
			if toggling {
				on = !enabled
			}
			sw := ui.Switch(c, &on).Label(style.T(c, "plugins.enabled")).Disabled(toggling)
			if toggling {
				sw.Tooltip(style.T(c, map[bool]string{true: "plugins.disabling", false: "plugins.enabling"}[enabled]))
			}
			if sw.Changed() {
				want := on
				a.pluginAction("enable:"+id, func(ctx context.Context, s string) error {
					_, err := a.pluginService(s).SetPluginEnabled(ctx, systemv1.SetPluginEnabledRequest_builder{PluginId: &id, Enabled: &want}.Build())
					return err
				})
			}
			if enabled {
				label := style.T(c, "plugins.restart")
				if st.busy == "restart" {
					label = "…"
				}
				if smallButton(c, label, st.busy != "") {
					a.pluginAction("restart", func(ctx context.Context, s string) error {
						_, err := a.pluginService(s).RestartPlugin(ctx, systemv1.RestartPluginRequest_builder{PluginId: &id}.Build())
						return err
					})
				}
			}
			if dev == "" && dangerButton(c.Key("uninstall"), style.T(c, "plugins.uninstall"), st.busy != "") {
				st.confirming = true
			}
		})
	})
	if ui.AlertDialog(c, &st.confirming, style.T(c, "plugins.uninstallTitle", "name", e.name), style.T(c, "plugins.uninstallMessage"),
		style.T(c, "action.cancel"), style.T(c, "plugins.uninstall")) == 1 {
		a.pluginAction("uninstall", func(ctx context.Context, s string) error {
			_, err := a.pluginService(s).UninstallPlugin(ctx, systemv1.UninstallPluginRequest_builder{PluginId: &id}.Build())
			return err
		})
	}
	if st.err != nil {
		footnote(c, errorMessage(c, st.err), true)
	}
	if e.description != "" {
		style.Text(ui.Text(c, e.description), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
	}
	if e.installed != nil && (m.GetConfigSchema() != "" || m.GetConfigPage() != "") {
		a.pluginConfiguration(c, id, m.GetConfigSchema(), m.GetConfigPage())
	}
	hosts := m.GetPermissions().GetHttpHosts()
	if e.installed != nil && (m.GetRuntime() == pluginv1.Runtime_RUNTIME_WASM && len(hosts) > 0 || m.GetRuntime() == pluginv1.Runtime_RUNTIME_PROCESS) {
		if m.GetRuntime() == pluginv1.Runtime_RUNTIME_PROCESS {
			hosts = nil
		}
		a.pluginProxy(c, id, e.name, hosts, e.installed.GetProxy())
	}
	if e.installed != nil && len(m.GetContainers()) > 0 {
		a.pluginContainersGroup(c, id)
	}
	a.pluginInformation(c, e, newest)
	if e.installed != nil {
		a.pluginLogs(c, id, dev)
	}
	if dev == "" && len(e.offered.GetVersions()) > 0 {
		group(c, "versions", style.T(c, "plugins.versions"), "", func(g *settingsGroup) {
			for _, v := range e.offered.GetVersions() {
				var detail []string
				if v.HasReleaseTime() {
					detail = append(detail, mediumDate(v.GetReleaseTime().AsTime()))
				}
				if v.GetChangelog() != "" {
					detail = append(detail, v.GetChangelog())
				}
				g.row(v.GetVersion(), strings.Join(detail, " · "), nil, func() {
					switch {
					case v.GetVersion() == m.GetVersion():
						style.Text(ui.Text(c, style.T(c, "plugins.installed")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
					case e.installed != nil:
						if smallButton(c.Key(v.GetVersion()), installLabel(v.GetVersion(), style.T(c, "plugins.installThis")), st.busy != "") {
							install(v.GetVersion())
						}
					}
				})
			}
		})
	}
}

// pluginInformation builds what is known of a plugin: its state, version
// and where it runs from, its category, author, license, website and
// catalog, and the hosts it reaches.
func (a *App) pluginInformation(c *ui.Context, e pluginEntry, newest string) {
	colors := style.Colors(c)
	m := e.installed.GetManifest()
	type fact struct{ label, value, link string }
	var info []fact
	if e.installed != nil {
		state := pluginStateName(c, e.installed.GetState())
		if e.installed.GetState() == systemv1.PluginState_PLUGIN_STATE_FAILED && e.installed.GetError() != "" {
			state += ": " + e.installed.GetError()
		}
		info = append(info, fact{label: style.T(c, "plugins.state"), value: state})
		if v := m.GetVersion(); v != "" {
			info = append(info, fact{label: style.T(c, "plugins.version"), value: v})
		}
		if d := e.installed.GetDevFolder(); d != "" {
			info = append(info, fact{label: style.T(c, "plugins.devFolder"), value: d})
		}
	}
	if newest != "" {
		info = append(info, fact{label: style.T(c, "plugins.latestVersion"), value: newest})
	}
	title, _, _ := pluginCategory(e.category)
	info = append(info, fact{label: style.T(c, "plugins.category"), value: style.T(c, title)})
	if e.author != "" {
		info = append(info, fact{label: style.T(c, "plugins.author"), value: e.author})
	}
	if l := m.GetLicense(); l != "" {
		info = append(info, fact{label: style.T(c, "plugins.license"), value: l})
	}
	if h := cmpOr(m.GetHomepage(), e.offered.GetHomepage()); h != "" {
		info = append(info, fact{label: style.T(c, "plugins.website"), value: h, link: h})
	}
	if e.offered != nil {
		info = append(info, fact{label: style.T(c, "plugins.catalog"), value: e.offered.GetCatalogUrl()})
	}
	if hosts := m.GetPermissions().GetHttpHosts(); len(hosts) > 0 {
		info = append(info, fact{label: style.T(c, "plugins.networkAccess"), value: strings.Join(hosts, ", ")})
	}
	group(c, "information", style.T(c, "plugins.information"), "", func(g *settingsGroup) {
		for i, f := range info {
			g.divider()
			ui.Row(c.Key(i)).MinHeight(44).Padding(10, 16).Gap(24).Children(func() {
				style.Text(ui.Text(c, f.label), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).Width(160).Shrink(0)
				if f.link != "" && a.opts.OpenURL != nil {
					b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Shrink(1).Label(f.value)
					b.Children(func() {
						t := style.Text(ui.Text(c, f.value), style.Type(c).Body).TextColor(style.Color(colors.AccentText))
						if b.Hovered() {
							t.Underline()
						}
						ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(style.Color(colors.AccentText))
					})
					if b.Clicked() {
						_ = a.opts.OpenURL(f.link)
					}
					return
				}
				style.Text(ui.Text(c, f.value), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Grow(1).Shrink(1).MinWidth(0).Selectable()
			})
		}
	})
}

// pluginLogs builds a plugin's recent log records, newest first, loaded
// again every 3 seconds; under development, with where it runs from.
func (a *App) pluginLogs(c *ui.Context, id, dev string) {
	st := &a.settingsState.admin.plugins
	t := adminTask(a, &st.logs, "plugin-logs", func(ctx context.Context, s string) ([]*systemv1.LogRecord, error) {
		res, err := a.systemService(s).ListLogs(ctx, systemv1.ListLogsRequest_builder{PluginId: &id, Limit: new(int32(200))}.Build())
		return res.GetRecords(), err
	}, id, every(c, 3*time.Second))
	footer := style.T(c, "plugins.logsFooter")
	if dev != "" {
		footer += " " + style.T(c, "plugins.devFolderDetail")
	}
	group(c, "logs", style.T(c, "plugins.logs"), footer, func(g *settingsGroup) {
		switch {
		case t.err != nil && len(t.value) == 0:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(t.value) == 0:
			g.note(style.T(c, "logs.none"), false)
		}
		ui.Scroll(c).MaxHeight(384).Children(func() {
			ui.Column(c).Children(func() {
				for i, r := range t.value {
					logRow(g, i, r)
				}
			})
		})
	})
}

// pluginProxy builds the proxy a plugin reaches sites through, saved as it
// is chosen: a WASM plugin's requests to hosts, which the server performs,
// or a process plugin's, without hosts; a process plugin going from direct
// to proxied or back restarts, once confirmed.
func (a *App) pluginProxy(c *ui.Context, id, name string, hosts []string, current string) {
	st := &a.settingsState.admin.plugins
	list := a.proxies().value
	saving := st.busy == "proxy"
	ids, labels := proxyChoices(c, list, current)
	save := func(p string) {
		st.busy, st.proxyErr = "proxy", nil
		server := a.chosenServer().GetId()
		go func() {
			_, err := a.pluginService(server).SetPluginProxy(a.ctx, systemv1.SetPluginProxyRequest_builder{PluginId: &id, Proxy: &p}.Build())
			a.opts.Update(func() {
				st.busy, st.proxyErr, st.restarting = "", err, nil
				a.settingsState.admin.reload++
			})
		}()
	}
	detail := style.T(c, "plugins.proxyProcessDetail")
	if hosts != nil {
		detail = style.T(c, "plugins.proxyDetail", "hosts", strings.Join(hosts, ", "))
	}
	group(c, "network", style.T(c, "plugins.network"), "", func(g *settingsGroup) {
		g.row(style.T(c, "plugins.proxy"), detail, nil, func() {
			chosen := labels[max(0, slices.Index(ids, current))]
			if selectOf(c, &chosen, labels).Width(192).Label(style.T(c, "plugins.proxy")).Disabled(saving).Changed() {
				p := ids[slices.Index(labels, chosen)]
				was, wasKnown := proxyDirect(list, current)
				next, nextKnown := proxyDirect(list, p)
				if hosts == nil && wasKnown && nextKnown && was != next {
					st.restarting = &p
				} else {
					save(p)
				}
			}
		})
		if st.proxyErr != nil {
			g.note(errorMessage(c, st.proxyErr), true)
		}
	})
	if st.restarting == nil {
		return
	}
	open := true
	if ui.AlertDialog(c, &open, style.T(c, "plugins.proxyRestartTitle", "name", name), style.T(c, "plugins.proxyRestartMessage"),
		style.T(c, "action.cancel"), style.T(c, "plugins.proxyRestart")) == 1 {
		save(*st.restarting)
	}
	if !open && !saving {
		st.restarting = nil
	}
}

// proxyDirect reports whether a proxy choice reaches sites directly: none,
// or the system proxy while the system sets none; known is false before
// the proxies are read.
func proxyDirect(list []*systemv1.Proxy, id string) (direct, known bool) {
	switch id {
	case "":
		return true, true
	case systemProxyID:
		for _, p := range list {
			if p.GetSystem() {
				return p.GetKind() == systemv1.ProxyKind_PROXY_KIND_UNSPECIFIED, true
			}
		}
		return false, false
	}
	return false, true
}

// pluginCatalogs builds the catalogs' page: their addresses, to remove,
// and adding one.
func (a *App) pluginCatalogs(c *ui.Context) {
	st := &a.settingsState.admin.plugins
	ss := &a.settingsState.admin.server
	a.backTo(c, style.T(c, "settings.plugins"), pluginsPath("", false))
	sectionHeader(c, style.T(c, "plugins.catalogs"), style.T(c, "plugins.catalogsMessage"))
	t := adminTask(a, &ss.catalogs, "plugin-catalogs", func(ctx context.Context, s string) ([]string, error) {
		res, err := a.pluginService(s).GetPluginCatalogs(ctx, &systemv1.GetPluginCatalogsRequest{})
		return res.GetUrls(), err
	})
	list := t.value
	busy := ss.busy != ""
	set := func(urls []string, done func()) {
		st.err = nil
		a.saveSettings(func(ctx context.Context, server string) error {
			_, err := a.pluginService(server).UpdatePluginCatalogs(ctx, systemv1.UpdatePluginCatalogsRequest_builder{Urls: urls}.Build())
			return err
		}, func(err error) {
			st.err = err
			if err == nil && done != nil {
				done()
			}
		})
	}
	group(c, "catalogs", "", "", func(g *settingsGroup) {
		for _, u := range list {
			g.row(u, "", nil, func() {
				if dangerButton(c.Key(u), style.T(c, "plugins.removeCatalog"), busy) {
					set(slices.DeleteFunc(slices.Clone(list), func(o string) bool { return o == u }), nil)
				}
			})
		}
		g.block(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &st.catalogURL).Grow(1).Label(style.T(c, "plugins.catalogAddress")).Placeholder(style.T(c, "plugins.catalogAddress"))
				u := strings.TrimSpace(st.catalogURL)
				ok := u != "" && !slices.Contains(list, u) && !busy
				if (smallButton(c, style.T(c, "plugins.addCatalog"), !ok) || in.Submitted()) && ok {
					set(append(slices.Clone(list), u), func() { st.catalogURL = "" })
				}
			})
		})
	})
	if st.err != nil {
		footnote(c, errorMessage(c, st.err), true)
	}
}

// pluginConfigState is the state of a plugin's configuration and its
// container dependencies.
type pluginConfigState struct {
	config task[string]
	// plugin is whose configuration value holds, as edited; saved is the
	// JSON stored, to tell a change by.
	plugin  string
	value   any
	saved   string
	saving  bool
	done    bool
	err     error
	options map[string]*task[[]*systemv1.PluginConfigOption]

	containers task[[]*systemv1.PluginContainer]
	// chosen is how each dependency is to run, by name, and address the
	// one it is given; busy the dependency being set.
	chosen       map[string]pluginv1.ContainerMode
	address      map[string]string
	busy         string
	containerErr map[string]error
	// recreating holds the proxy a managed container is recreated to take,
	// once confirmed, of the dependency named recreatingName.
	recreating     *string
	recreatingName string
	// places are the server's folders each managed dependency mounts and
	// the port it publishes for peers, as edited, by name.
	places map[string]*containerPlaces
}

// containerPlaces are a managed container dependency's folders and peer
// port as edited, and the container they were drafted from.
type containerPlaces struct {
	from     *systemv1.PluginContainer
	folders  map[string]string
	peerPort string
}

func containerPlacesOf(p *systemv1.PluginContainer) *containerPlaces {
	return &containerPlaces{from: p, folders: maps.Clone(p.GetFolders()), peerPort: strconv.FormatUint(uint64(p.GetPeerPort()), 10)}
}

// changed tells whether the places differ from those saved.
func (x *containerPlaces) changed() bool {
	d := x.from.GetDependency()
	for _, f := range d.GetFolders() {
		if strings.TrimSpace(x.folders[f.GetName()]) != x.from.GetFolders()[f.GetName()] {
			return true
		}
	}
	return d.GetPeerPort() > 0 && x.peerPort != strconv.FormatUint(uint64(x.from.GetPeerPort()), 10)
}

// absoluteFolder is the pattern of a folder of the server a container
// mounts.
var absoluteFolder = regexp.MustCompile(`^(/|[A-Za-z]:[\\/])`)

// jsonText returns a configuration's JSON as compared: keys sorted, an
// empty configuration as {}.
func jsonText(v any) string {
	if v == nil {
		v = map[string]any{}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func parseConfig(text string) any {
	var v any
	if text == "" || json.Unmarshal([]byte(text), &v) != nil {
		return map[string]any{}
	}
	return v
}

// pluginConfiguration builds an installed plugin's configuration: the
// form of its schema, saved by Save, and a link to its own page.
func (a *App) pluginConfiguration(c *ui.Context, id, schemaText, page string) {
	st := &a.settingsState.admin.plugins.config
	colors := style.Colors(c)
	s := parseSchema(schemaText)
	if s != nil && len(s.properties) == 0 {
		s = nil
	}
	ui.Column(c.Key("configuration")).Gap(8).Children(func() {
		ui.Row(c).Gap(12).Padding(0, 4).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, style.T(c, "plugins.configuration")), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Grow(1)
			if page != "" && a.opts.OpenURL != nil {
				own := strings.TrimSuffix(a.chosenServer().GetAddress(), "/") + "/plugins/" + url.PathEscape(id) + "/" + page
				b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Label(style.T(c, "plugins.openSettingsPage"))
				b.Children(func() {
					style.Text(ui.Text(c, style.T(c, "plugins.openSettingsPage")), style.Type(c).Callout).TextColor(style.Color(colors.AccentText))
					ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(style.Color(colors.AccentText))
				})
				if b.Clicked() {
					_ = a.opts.OpenURL(own)
				}
			}
		})
		if s == nil {
			return
		}
		t := adminTask(a, &st.config, "plugin-config", func(ctx context.Context, server string) (string, error) {
			res, err := a.pluginService(server).GetPluginConfig(ctx, systemv1.GetPluginConfigRequest_builder{PluginId: &id}.Build())
			return res.GetConfigJson(), err
		}, id)
		if t.err != nil {
			footnote(c, errorMessage(c, t.err), true)
			return
		}
		if !t.loaded {
			return
		}
		if st.plugin != id {
			st.plugin, st.value, st.saved, st.done, st.err = id, parseConfig(t.value), jsonText(parseConfig(t.value)), false, nil
		}
		form := &schemaForm{c: c, options: a.pluginConfigOptions(id, optionSources(s)), unset: a.containerUnset(id)}
		form.draw(s, st.value, func(v any) { st.value, st.done = v, false })
		changed := jsonText(st.value) != st.saved
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			label := style.T(c, "action.save")
			if st.saving {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(!changed || st.saving).Clicked() {
				a.savePluginConfig(id, jsonText(st.value))
			}
			if st.done && !changed {
				style.Text(ui.Text(c, style.T(c, "plugins.configurationSaved")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			}
			if st.err != nil {
				style.Text(ui.Text(c, errorMessage(c, st.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger)).Selectable()
			}
		})
	})
}

// savePluginConfig stores a plugin's configuration; the form then holds
// what is stored, with the IDs the server generated.
func (a *App) savePluginConfig(id, text string) {
	st := &a.settingsState.admin.plugins.config
	server := a.chosenServer().GetId()
	st.saving, st.done, st.err = true, false, nil
	go func() {
		res, err := a.pluginService(server).SetPluginConfig(a.ctx, systemv1.SetPluginConfigRequest_builder{PluginId: &id, ConfigJson: &text}.Build())
		a.opts.Update(func() {
			st.saving, st.err = false, err
			if err != nil || st.plugin != id {
				return
			}
			v := parseConfig(res.GetConfigJson())
			st.value, st.saved, st.done = v, jsonText(v), true
			a.settingsState.admin.reload++
		})
	}()
}

// pluginConfigOptions loads the options of the sources a plugin's schema
// names, and returns them by source: nil while they load or when they
// cannot be had.
func (a *App) pluginConfigOptions(id string, sources []string) func(string) []*systemv1.PluginConfigOption {
	st := &a.settingsState.admin.plugins.config
	if st.options == nil {
		st.options = map[string]*task[[]*systemv1.PluginConfigOption]{}
	}
	for _, source := range sources {
		t := st.options[source]
		if t == nil {
			t = &task[[]*systemv1.PluginConfigOption]{}
			st.options[source] = t
		}
		adminTask(a, t, "plugin-options", func(ctx context.Context, server string) ([]*systemv1.PluginConfigOption, error) {
			res, err := a.pluginService(server).ListPluginConfigOptions(ctx, systemv1.ListPluginConfigOptionsRequest_builder{PluginId: &id, Source: &source}.Build())
			return res.GetOptions(), err
		}, id, source)
	}
	return func(source string) []*systemv1.PluginConfigOption {
		t := st.options[source]
		if t == nil || !t.loaded || t.err != nil {
			return nil
		}
		if t.value == nil {
			return []*systemv1.PluginConfigOption{}
		}
		return t.value
	}
}

// pluginContainers loads a plugin's container dependencies, again every 5
// seconds, as managed ones pull and start in the background.
func (a *App) pluginContainers(c *ui.Context, id string) *task[[]*systemv1.PluginContainer] {
	st := &a.settingsState.admin.plugins.config
	return adminTask(a, &st.containers, "plugin-containers", func(ctx context.Context, server string) ([]*systemv1.PluginContainer, error) {
		res, err := a.pluginService(server).ListPluginContainers(ctx, systemv1.ListPluginContainersRequest_builder{PluginId: &id}.Build())
		return res.GetContainers(), err
	}, id, every(c, containersInterval))
}

// containerUnset returns whether a container dependency of the plugin is
// not set up, by its name.
func (a *App) containerUnset(id string) func(string) bool {
	st := &a.settingsState.admin.plugins.config
	return func(name string) bool {
		for _, p := range st.containers.value {
			if p.GetDependency().GetName() == name {
				return p.GetStatus().GetMode() == pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED
			}
		}
		return false
	}
}

// pluginContainersGroup builds an installed plugin's container
// dependencies: how each runs, its state, and the commands running it
// elsewhere.
func (a *App) pluginContainersGroup(c *ui.Context, id string) {
	colors := style.Colors(c)
	t := a.pluginContainers(c, id)
	ui.Column(c.Key("containers")).Gap(8).Children(func() {
		style.Text(ui.Text(c, style.T(c, "plugins.containers")), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Padding(0, 4)
		for _, p := range t.value {
			a.containerDependency(c, id, p)
		}
		if t.err != nil {
			footnote(c, errorMessage(c, t.err), true)
		}
	})
}

func (a *App) containerDependency(c *ui.Context, id string, p *systemv1.PluginContainer) {
	st := &a.settingsState.admin.plugins.config
	colors := style.Colors(c)
	d, s := p.GetDependency(), p.GetStatus()
	name := d.GetName()
	gpu := a.settingsState.admin.overview.runtime.value.GetGpu()
	mode := s.GetMode()
	if st.chosen == nil {
		st.chosen, st.address, st.containerErr = map[string]pluginv1.ContainerMode{}, map[string]string{}, map[string]error{}
	}
	chosen, ok := st.chosen[name]
	if !ok {
		chosen = mode
		st.chosen[name] = mode
		if mode == pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL {
			st.address[name] = s.GetAddress()
		}
	}
	busy := st.busy == name
	if st.places == nil {
		st.places = map[string]*containerPlaces{}
	}
	places := st.places[name]
	if places == nil || places.from != p && !places.changed() {
		places = containerPlacesOf(p)
		st.places[name] = places
	}
	set := func(m pluginv1.ContainerMode, address string, useGPU bool, proxy *string, with ...func(*systemv1.SetPluginContainerRequest_builder)) {
		server := a.chosenServer().GetId()
		st.busy, st.containerErr[name] = name, nil
		req := systemv1.SetPluginContainerRequest_builder{PluginId: &id, Name: &name, Mode: &m, UseGpu: &useGPU, Proxy: proxy}
		if address != "" {
			req.Address = &address
		}
		for _, f := range with {
			f(&req)
		}
		go func() {
			_, err := a.pluginService(server).SetPluginContainer(a.ctx, req.Build())
			a.opts.Update(func() {
				st.busy, st.containerErr[name] = "", err
				if proxy != nil {
					st.recreating = nil
				}
				a.settingsState.admin.reload++
			})
		}()
	}
	useGPU := gpu && p.GetUseGpu()
	state := ""
	if s != nil {
		state = containerState(c, s)
	}
	var detail []string
	for _, x := range []string{d.GetDescription(), state} {
		if x != "" {
			detail = append(detail, x)
		}
	}
	group(c, "container-"+name, "", "", func(g *settingsGroup) {
		g.divider()
		ui.Row(c.Key("head")).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(224).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					style.Text(ui.Text(c, name), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
					need := style.T(c, "plugins.containerOptional")
					if d.GetRequired() {
						need = style.T(c, "plugins.containerRequired")
					}
					style.Text(ui.Text(c, need), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).
						Padding(2, 8).Radius(999).Background(style.Color(colors.Fill))
				})
				if len(detail) > 0 {
					style.Text(ui.Text(c, strings.Join(detail, " · ")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
				}
			})
			modes := []pluginv1.ContainerMode{pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED, pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL}
			labels := []string{style.T(c, "plugins.containerUnset"), style.T(c, "plugins.containerManaged"), style.T(c, "plugins.containerExternal")}
			i := max(0, slices.Index(modes, chosen))
			l := labels[i]
			if selectOf(c, &l, labels).Label(style.T(c, "plugins.containerMode")).Disabled(busy).Changed() {
				m := modes[slices.Index(labels, l)]
				st.chosen[name] = m
				// One whose folders are not chosen yet runs once they are saved.
				foldersSet := !slices.ContainsFunc(d.GetFolders(), func(f *pluginv1.ContainerFolder) bool { return p.GetFolders()[f.GetName()] == "" })
				if m != pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL && (m != pluginv1.ContainerMode_CONTAINER_MODE_MANAGED || foldersSet) {
					gpuOn := m == pluginv1.ContainerMode_CONTAINER_MODE_MANAGED && (d.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_REQUIRED || useGPU)
					set(m, "", gpuOn, nil)
				}
			}
		})
		managed := chosen == pluginv1.ContainerMode_CONTAINER_MODE_MANAGED
		if managed && d.GetGpu() != pluginv1.GpuSupport_GPU_SUPPORT_UNSPECIFIED && gpu {
			g.row(style.T(c, "plugins.containerUseGpu"), "", nil, func() {
				on := p.GetUseGpu()
				if ui.Switch(c, &on).Label(style.T(c, "plugins.containerUseGpu")).Disabled(busy || d.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_REQUIRED).Changed() {
					set(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, "", on, nil)
				}
			})
		}
		if managed && (len(d.GetFolders()) > 0 || d.GetPeerPort() > 0) {
			a.containerPlaces(g, d, places, mode == pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, busy, func() {
				folders := map[string]string{}
				for _, f := range d.GetFolders() {
					folders[f.GetName()] = strings.TrimSpace(places.folders[f.GetName()])
				}
				gpuOn := d.GetGpu() == pluginv1.GpuSupport_GPU_SUPPORT_REQUIRED || useGPU
				set(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, "", gpuOn, nil, func(r *systemv1.SetPluginContainerRequest_builder) {
					r.Folders = folders
					if d.GetPeerPort() > 0 {
						port, _ := strconv.ParseUint(places.peerPort, 10, 16)
						r.PeerPort = new(uint32(port))
					}
				})
			})
		}
		if managed && mode == pluginv1.ContainerMode_CONTAINER_MODE_MANAGED {
			// A container given the proxy of each request reaches sites directly.
			if !d.GetDirect() {
				list := a.proxies().value
				current := p.GetProxy()
				ids, labels := proxyChoices(c, list, current)
				g.row(style.T(c, "plugins.proxy"), style.T(c, "plugins.containerProxyDetail"), nil, func() {
					l := labels[max(0, slices.Index(ids, current))]
					if selectOf(c, &l, labels).Width(192).Label(style.T(c, "plugins.proxy")).Disabled(busy).Changed() {
						next := ids[slices.Index(labels, l)]
						was, wasKnown := proxyDirect(list, current)
						now, nowKnown := proxyDirect(list, next)
						if wasKnown && nowKnown && was != now {
							st.recreating, st.recreatingName = &next, name
						} else {
							set(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, "", useGPU, &next)
						}
					}
				})
			}
			if p.GetCommand() != "" {
				g.divider()
				ui.Box(c.Key("command")).Padding(10, 0, 0).Children(func() {
					a.containerDisclosure(c, id+"/"+name+"/command", style.T(c, "plugins.command"), p.GetCommand(), false)
				})
			}
		}
		if s.GetError() != "" {
			g.divider()
			ui.Box(c.Key("error")).Padding(10, 0, 0).Children(func() {
				a.containerDisclosure(c, id+"/"+name+"/error", style.T(c, "plugins.containerError"), s.GetError(), true)
			})
		}
		if chosen == pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL {
			hint := cmpOr(p.GetCommands().GetAddress(), p.GetGpuCommands().GetAddress())
			g.block(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					address := st.address[name]
					in := ui.TextInput(c, &address).Grow(1).Label(style.T(c, "plugins.containerAddress")).Placeholder(cmpOr(hint, style.T(c, "plugins.containerAddress")))
					st.address[name] = address
					v := strings.TrimSpace(address)
					ok := v != "" && !busy && (mode != pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL || strings.TrimSuffix(v, "/") != s.GetAddress())
					if (smallButton(c, style.T(c, "action.save"), !ok) || in.Submitted()) && ok {
						set(pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL, v, false, nil)
					}
				})
			})
			g.note(style.T(c, "plugins.containerCommandsHint", "address", hint), false)
			containerCommands(g, style.T(c, "plugins.containerCommands"), p.GetCommands())
			containerCommands(g, style.T(c, "plugins.containerGpuCommands"), p.GetGpuCommands())
		}
		if err := st.containerErr[name]; err != nil {
			g.note(errorMessage(c, err), true)
		}
	})
	if st.recreating == nil || st.recreatingName != name {
		return
	}
	open := true
	if ui.AlertDialog(c, &open, style.T(c, "plugins.containerProxyTitle", "name", name), style.T(c, "plugins.containerProxyMessage"),
		style.T(c, "action.cancel"), style.T(c, "plugins.containerRecreate")) == 1 {
		set(pluginv1.ContainerMode_CONTAINER_MODE_MANAGED, "", useGPU, st.recreating)
	}
	if !open && !busy {
		st.recreating = nil
	}
}

// containerCommands adds the commands running a container dependency
// elsewhere: docker run and Compose's service.
func containerCommands(g *settingsGroup, title string, cmds *systemv1.ContainerCommands) {
	if cmds == nil {
		return
	}
	c := g.c
	colors := style.Colors(c)
	code := func(text string) {
		ui.Box(c).FillWidth().Padding(10).Radius(6).Background(style.Color(colors.Fill)).Children(func() {
			style.Text(ui.Text(c, text), style.Type(c).Footnote).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextPrimary)).Selectable()
		})
	}
	note := func(text string) {
		style.Text(ui.Text(c, text), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
	}
	// A server in a container is joined on its network, which the
	// administrator names.
	network := strings.Contains(cmds.GetDockerRun(), "<network>")
	g.divider()
	ui.Column(c.Key(title)).Padding(10, 16).Gap(6).Children(func() {
		style.Text(ui.Text(c, title), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
		if network {
			note(style.T(c, "plugins.containerDockerRunNetwork", "network", "<network>"))
		} else {
			note(style.T(c, "plugins.containerDockerRun"))
		}
		code(cmds.GetDockerRun())
		if network {
			note(style.T(c, "plugins.containerComposeServer"))
		} else {
			note(style.T(c, "plugins.containerCompose"))
		}
		code(cmds.GetCompose())
	})
}

// containerPlaces adds the server's folders a managed dependency mounts
// and the port it publishes for peers, saved together; running tells
// whether it runs managed already, when Save takes only changes.
func (a *App) containerPlaces(g *settingsGroup, d *pluginv1.ContainerDependency, x *containerPlaces, running, busy bool, save func()) {
	c := g.c
	colors := style.Colors(c)
	if len(d.GetFolders()) > 0 {
		g.note(style.T(c, "plugins.containerFoldersHint"), false)
	}
	for _, f := range d.GetFolders() {
		g.divider()
		ui.Column(c.Key("folder-"+f.GetName())).Padding(10, 16).Gap(6).Children(func() {
			ui.Row(c).Gap(4).Wrap().Children(func() {
				style.Text(ui.Text(c, f.GetPath()), style.Type(c).Callout).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextPrimary))
				if f.GetDescription() != "" {
					style.Text(ui.Text(c, "· "+f.GetDescription()), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
				}
			})
			v := x.folders[f.GetName()]
			ui.TextInput(c, &v).FillWidth().Label(f.GetPath()).Placeholder(style.T(c, "plugins.containerFolderPath"))
			if x.folders == nil {
				x.folders = map[string]string{}
			}
			x.folders[f.GetName()] = v
		})
	}
	portValid := true
	if d.GetPeerPort() > 0 {
		n, err := strconv.ParseUint(x.peerPort, 10, 32)
		portValid = err == nil && n <= 65535
		g.row(style.T(c, "plugins.containerPeerPort"), style.T(c, "plugins.containerPeerPortDetail"), nil, func() {
			in := ui.TextInput(c, &x.peerPort).Width(96).Label(style.T(c, "plugins.containerPeerPort"))
			if !portValid {
				in.Border(1, style.Color(colors.Danger))
			}
		})
	}
	valid := portValid && !slices.ContainsFunc(d.GetFolders(), func(f *pluginv1.ContainerFolder) bool {
		return !absoluteFolder.MatchString(strings.TrimSpace(x.folders[f.GetName()]))
	})
	g.block(func() {
		if smallButton(c, style.T(c, "action.save"), !valid || (running && !x.changed()) || busy) {
			save()
		}
	})
}
