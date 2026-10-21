package app

import (
	"context"
	goruntime "runtime"

	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// aboutState is the About section: the core's version and what the system
// tells, each asked once.
type aboutState struct {
	core   task[string]
	system task[System]
}

// aboutSection builds About, the settings' last section, as the other
// apps': the app, its name as the servers know it, its versions and
// license; and this device, the name and ID the servers know it by, its
// maker and model, its system and architecture.
func (a *App) aboutSection(c *ui.Context) {
	st := &a.settingsState.about
	sectionHeader(c, style.T(c, "settings.about"), "")
	core := load(a, &st.core, "core", func(ctx context.Context) (string, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{GetVersion: &capiv1.GetVersion{}}.Build())
		return m.GetVersion(), err
	})
	system := load(a, &st.system, "system", func(ctx context.Context) (System, error) {
		if a.opts.System == nil {
			return System{}, nil
		}
		return a.opts.System(ctx), nil
	})
	d := a.opts.Device
	group(c, "about.app", style.T(c, "about.app"), "", func(g *settingsGroup) {
		aboutFacts(g, [][2]string{
			{style.T(c, "about.appName"), d.GetClient()},
			{style.T(c, "about.clientVersion"), d.GetClientVersion()},
			{style.T(c, "about.coreVersion"), core.value},
			{style.T(c, "about.license"), "GPL-3.0"},
		})
	})
	s := system.value
	group(c, "about.device", style.T(c, "about.device"), style.T(c, "about.deviceFooter"), func(g *settingsGroup) {
		aboutFacts(g, [][2]string{
			{style.T(c, "about.deviceName"), d.GetName()},
			{style.T(c, "about.deviceId"), d.GetId()},
			{style.T(c, "about.manufacturer"), s.Manufacturer},
			{style.T(c, "about.model"), s.Model},
			{style.T(c, "about.system"), s.Version},
			{style.T(c, "about.architecture"), goruntime.GOARCH},
		})
	})
}

// aboutFacts adds a row of each fact known, its label and its value,
// which takes the rest of the row and wraps when it is long, as a
// device's ID.
func aboutFacts(g *settingsGroup, facts [][2]string) {
	c := g.c
	colors := style.Colors(c)
	for _, f := range facts {
		if f[1] == "" {
			continue
		}
		g.divider()
		ui.Row(c.Key(g.rows)).MinHeight(52).Padding(10, 16).Gap(16).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, f[0]), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Shrink(0)
			style.Text(ui.Text(c, f[1]), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).
				Grow(1).MinWidth(0).TextAlign(ui.End).Selectable()
		})
	}
}
