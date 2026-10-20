package app

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// overviewState is the state of the Overview's load and containers.
type overviewState struct {
	load       task[[]*systemv1.LoadSample]
	runtime    task[*systemv1.ContainerRuntime]
	containers task[[]*systemv1.PluginContainer]
	plugins    task[[]*systemv1.Plugin]
	// shown are the containers' commands and errors shown, by key.
	shown map[string]bool
}

// The load refreshes every 2 seconds, the containers every 5, as the web
// UI's.
const (
	loadInterval       = 2 * time.Second
	containersInterval = 5 * time.Second
)

// loadColumns are the samples a graph has room for: two minutes', as the
// server keeps; loadRows the dots of a column.
const (
	loadColumns = 60
	loadRows    = 6
)

// every returns the period of length d now is in, to key what refreshes
// that often, and asks for a frame as the next begins.
func every(c *ui.Context, d time.Duration) int64 {
	now := c.Now()
	n := now.UnixNano() / int64(d)
	c.After(time.Unix(0, (n+1)*int64(d)).Sub(now))
	return n
}

// loadGroup builds the load of the server's computer, as the web UI's: a
// card each of its CPU, memory, disks and network, with what it is now,
// three figures more and a graph of the last two minutes.
func (a *App) loadGroup(c *ui.Context) {
	st := &a.settingsState.admin.overview
	t := adminTask(a, &st.load, "load", func(ctx context.Context, s string) ([]*systemv1.LoadSample, error) {
		res, err := a.systemService(s).GetSystemLoad(ctx, &systemv1.GetSystemLoadRequest{})
		return res.GetSamples(), err
	}, every(c, loadInterval))
	samples := t.value
	var now *systemv1.LoadSample
	if len(samples) > 0 {
		now = samples[len(samples)-1]
	}
	colors := style.Colors(c)
	group(c, "load", style.T(c, "overview.load"), style.T(c, "overview.loadFooter"), func(g *settingsGroup) {
		if t.err != nil && len(samples) == 0 {
			g.note(errorMessage(c, t.err), true)
			return
		}
		g.divider()
		ui.Grid(c).Columns(2).Gap(1).Background(style.Color(colors.Border)).Children(func() {
			cpuCard(c, now, samples)
			memoryCard(c, now, samples)
			diskCard(c, now, samples)
			networkCard(c, now, samples)
		})
	})
}

// loadCard is a card of the load: its tile and title, what it is now,
// three figures more and the graph of the last samples. told is false
// where the server's system does not tell it; while no sample came, the
// figures wait.
type loadCard struct {
	icon, title string
	color       ui.Color
	badge       func()
	told        bool
	value       func()
	caption     string
	stats       [3][2]string
	graph       []float64 // below 0 where a sample has no value
}

func (k loadCard) build(c *ui.Context) {
	colors := style.Colors(c)
	ui.Column(c.Key(k.title)).MinWidth(0).Gap(12).Padding(16).Background(style.Color(colors.Surface)).Label(k.title).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Shrink(0).Radius(6).Background(k.color).Center().Children(func() {
				ui.Icon(c, style.Icon(k.icon, true)).Size(15, 15).TextColor(ui.RGB(255, 255, 255))
			})
			style.Text(ui.Text(c, k.title), style.Type(c).Headline).TextColor(k.color).Grow(1)
			if k.badge != nil {
				k.badge()
			}
		})
		if !k.told {
			style.Text(ui.Text(c, style.T(c, "overview.loadUnknown")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			return
		}
		ui.Row(c).MinHeight(36).Gap(8).AlignItems(ui.End).Children(func() {
			if k.value == nil {
				style.Text(ui.Text(c, "…"), style.Type(c).Title2).TextColor(style.Color(colors.TextTertiary))
				return
			}
			k.value()
			if k.caption != "" {
				style.Text(ui.Text(c, k.caption), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Padding(0, 0, 3)
			}
		})
		ui.Grid(c).Columns(3).Gap(8).Children(func() {
			for _, s := range k.stats {
				ui.Column(c.Key(s[0])).MinWidth(0).Children(func() {
					style.Text(ui.Text(c, s[0]), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary)).SingleLine()
					style.Text(ui.Text(c, cmpOr(s[1], "…")), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary)).FontFeatures("tnum").SingleLine()
				})
			}
		})
		loadDots(c, k.graph, k.color)
	})
}

// loadDots builds a graph of dots: a column per sample, the latest at the
// right, lit from the bottom as high as its value, from 0 to 1; columns
// without a sample stay unlit.
func loadDots(c *ui.Context, values []float64, color ui.Color) {
	if len(values) > loadColumns {
		values = values[len(values)-loadColumns:]
	}
	offset := loadColumns - len(values)
	off := style.Color(style.Colors(c).TextSecondary).Alpha(0.12)
	ui.Box(c).FillWidth().AspectRatio(loadColumns / loadRows).Draw(func(p *ui.Painter, r ui.Rect) {
		cell := r.W / loadColumns
		var lit, unlit ui.Path
		for x := range loadColumns {
			n := 0
			if x >= offset {
				// A value above nothing lights a dot, so that activity shows.
				if v := values[x-offset]; v > 0 {
					n = max(1, int(math.Round(min(1, v)*loadRows)))
				}
			}
			for y := range loadRows {
				path := &unlit
				if loadRows-y <= n {
					path = &lit
				}
				path.Circle(r.X+float32(x)*cell+cell/2, r.Y+float32(y)*cell+cell/2, cell*1.7/6)
			}
		}
		p.FillPath(&unlit, off)
		p.FillPath(&lit, color)
	})
}

// bigFigure builds a figure large, its unit small, and what follows it.
func bigFigure(c *ui.Context, text, after string) {
	colors := style.Colors(c)
	digits, unit := text, ""
	if m := figureParts.FindStringSubmatch(text); m != nil {
		digits, unit = m[1]+strings.TrimSpace(m[2]), strings.TrimSpace(m[3])
	}
	ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
		style.Text(ui.Text(c, digits), style.Type(c).Title2).FontWeight(600).FontFeatures("tnum").TextColor(style.Color(colors.TextPrimary))
		if unit != "" {
			style.Text(ui.Text(c, unit), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(0, 0, 2)
		}
		if after != "" {
			style.Text(ui.Text(c, after), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Padding(0, 0, 3, 4)
		}
	})
}

var figureParts = regexp.MustCompile(`^([^\d]*)([\d.,\s]+)(.*)$`)

func cpuCard(c *ui.Context, now *systemv1.LoadSample, samples []*systemv1.LoadSample) {
	k := loadCard{icon: "cpu", color: ui.RGB(0x00, 0x7A, 0xFF), title: style.T(c, "overview.cpu"), told: now == nil || now.HasCpu()}
	k.stats = [3][2]string{{style.T(c, "overview.cpuUser")}, {style.T(c, "overview.cpuSystem")}, {style.T(c, "overview.byServer")}}
	if l := now.GetCpu(); now.HasCpu() {
		k.value = func() { bigFigure(c, percentOf(l.GetUser()+l.GetSystem()), "") }
		k.stats[0][1], k.stats[1][1], k.stats[2][1] = percentOf(l.GetUser()), percentOf(l.GetSystem()), percentOf(l.GetServer())
	}
	for _, s := range samples {
		v := -1.0
		if s.HasCpu() {
			v = s.GetCpu().GetUser() + s.GetCpu().GetSystem()
		}
		k.graph = append(k.graph, v)
	}
	k.build(c)
}

func memoryCard(c *ui.Context, now *systemv1.LoadSample, samples []*systemv1.LoadSample) {
	k := loadCard{icon: "memory", color: ui.RGB(0x34, 0xC7, 0x59), title: style.T(c, "overview.memory"), told: now == nil || now.HasMemory()}
	k.stats = [3][2]string{{style.T(c, "overview.memoryAvailable")}, {style.T(c, "overview.swap")}, {style.T(c, "overview.byServer")}}
	if m := now.GetMemory(); now.HasMemory() {
		k.value = func() {
			bigFigure(c, capacityOf(m.GetUsedBytes()), style.T(c, "overview.memoryOf", "total", capacityOf(m.GetTotalBytes())))
		}
		swap := "–"
		if m.GetSwapTotalBytes() > 0 {
			swap = capacityOf(m.GetSwapUsedBytes())
		}
		k.stats[0][1], k.stats[1][1], k.stats[2][1] = capacityOf(m.GetTotalBytes()-m.GetUsedBytes()), swap, capacityOf(m.GetServerBytes())
		if m.GetTotalBytes() > 0 {
			high := float64(m.GetUsedBytes())/float64(m.GetTotalBytes()) >= 0.9
			label, fg, bg := style.T(c, "overview.memoryNormal"), ui.RGB(0x24, 0x8A, 0x3D), ui.RGB(0x34, 0xC7, 0x59).Alpha(0.15)
			if high {
				label, fg, bg = style.T(c, "overview.memoryHigh"), ui.RGB(0xC9, 0x34, 0x00), ui.RGB(0xFF, 0x95, 0x00).Alpha(0.15)
			}
			k.badge = func() {
				ui.Box(c).Padding(2, 8).Radius(999).Background(bg).Children(func() {
					style.Text(ui.Text(c, label), style.Type(c).Caption).TextColor(fg)
				})
			}
		}
	}
	for _, s := range samples {
		v := -1.0
		if m := s.GetMemory(); s.HasMemory() && m.GetTotalBytes() > 0 {
			v = float64(m.GetUsedBytes()) / float64(m.GetTotalBytes())
		}
		k.graph = append(k.graph, v)
	}
	k.build(c)
}

func diskCard(c *ui.Context, now *systemv1.LoadSample, samples []*systemv1.LoadSample) {
	colors := style.Colors(c)
	k := loadCard{icon: "disk", color: ui.RGB(0xFF, 0x95, 0x00), title: style.T(c, "overview.disk"), told: now == nil || now.HasDisk()}
	k.stats = [3][2]string{{style.T(c, "overview.read")}, {style.T(c, "overview.write")}, {style.T(c, "overview.written")}}
	if d := now.GetDisk(); now.HasDisk() {
		k.value = func() {
			if !d.HasTotalBytes() {
				style.Text(ui.Text(c, "–"), style.Type(c).Title2).TextColor(style.Color(colors.TextTertiary))
				return
			}
			bigFigure(c, capacityOf(d.GetAvailableBytes()), style.T(c, "overview.diskAvailable", "total", capacityOf(d.GetTotalBytes())))
		}
		k.stats[0][1], k.stats[1][1], k.stats[2][1] = "–", "–", "–"
		if d.HasReadRate() {
			k.stats[0][1] = rateOf(d.GetReadRate())
		}
		if d.HasWriteRate() {
			k.stats[1][1] = rateOf(d.GetWriteRate())
		}
		if d.HasWrittenBytes() {
			k.stats[2][1] = capacityOf(d.GetWrittenBytes())
		}
	}
	for _, s := range samples {
		v := -1.0
		if s.HasDisk() {
			v = s.GetDisk().GetReadRate() + s.GetDisk().GetWriteRate()
		}
		k.graph = append(k.graph, v)
	}
	k.graph = scaledLoad(k.graph, 1e6)
	k.build(c)
}

func networkCard(c *ui.Context, now *systemv1.LoadSample, samples []*systemv1.LoadSample) {
	k := loadCard{icon: "network", color: ui.RGB(0x30, 0xB0, 0xC7), title: style.T(c, "overview.network"), told: now == nil || now.HasNetwork()}
	k.caption = style.T(c, "overview.download")
	k.stats = [3][2]string{{style.T(c, "overview.upload")}, {style.T(c, "overview.received")}, {style.T(c, "overview.sent")}}
	if n := now.GetNetwork(); now.HasNetwork() {
		k.value = func() { bigFigure(c, rateOf(n.GetReceiveRate()), "") }
		k.stats[0][1], k.stats[1][1], k.stats[2][1] = rateOf(n.GetSendRate()), capacityOf(n.GetReceivedBytes()), capacityOf(n.GetSentBytes())
	}
	for _, s := range samples {
		v := -1.0
		if s.HasNetwork() {
			v = s.GetNetwork().GetReceiveRate() + s.GetNetwork().GetSendRate()
		}
		k.graph = append(k.graph, v)
	}
	k.graph = scaledLoad(k.graph, 1e5)
	k.build(c)
}

// scaledLoad returns rates as shares of the highest of them, or of floor
// when all are lower; those without a value stay without.
func scaledLoad(values []float64, floor float64) []float64 {
	top := floor
	for _, v := range values {
		top = max(top, v)
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v
		if v > 0 {
			out[i] = v / top
		}
	}
	return out
}

// percentOf returns a share from 0 to 1 as a whole percentage.
func percentOf(v float64) string { return fmt.Sprintf("%d%%", int(math.Round(v*100))) }

// rateOf returns bytes per second in kilobytes, megabytes or gigabytes as
// it is large, a decimal only below ten, as the web UI's.
func rateOf(b float64) string {
	n, unit := b/1e3, "kB/s"
	switch {
	case b >= 1e9:
		n, unit = b/1e9, "GB/s"
	case b >= 1e6:
		n, unit = b/1e6, "MB/s"
	}
	digits := 0
	if n < 10 {
		digits = 1
	}
	s := fmt.Sprintf("%.*f", digits, n)
	if digits == 1 {
		s = strings.TrimSuffix(s, ".0")
	}
	return s + " " + unit
}

// containersGroup builds the plugins' containers, as the web UI's: which
// plugin each is for, its state, the command it runs as, and why it
// failed.
func (a *App) containersGroup(c *ui.Context) {
	st := &a.settingsState.admin.overview
	t := adminTask(a, &st.containers, "containers", func(ctx context.Context, s string) ([]*systemv1.PluginContainer, error) {
		res, err := a.pluginService(s).ListContainers(ctx, &systemv1.ListContainersRequest{})
		return res.GetContainers(), err
	}, every(c, containersInterval))
	plugins := adminTask(a, &st.plugins, "overview-plugins", func(ctx context.Context, s string) ([]*systemv1.Plugin, error) {
		res, err := a.pluginService(s).ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		return res.GetPlugins(), err
	})
	pluginName := func(id string) string {
		for _, p := range plugins.value {
			if p.GetManifest().GetId() == id {
				return cmpOr(p.GetManifest().GetName(), id)
			}
		}
		return id
	}
	group(c, "containers", style.T(c, "overview.containers"), "", func(g *settingsGroup) {
		if t.loaded && t.err == nil && len(t.value) == 0 {
			g.note(style.T(c, "overview.containersNone"), false)
		}
		for _, k := range t.value {
			s := k.GetStatus()
			external := s.GetMode() == pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL
			parts := []string{style.T(c, "overview.containerOf", "plugin", pluginName(k.GetPluginId()))}
			command := k.GetCommand()
			if k.HasStatus() {
				if external {
					parts = append(parts, style.T(c, "overview.containerExternal", "address", s.GetAddress()))
					command = k.GetCommands().GetDockerRun()
					if k.GetUseGpu() {
						command = k.GetGpuCommands().GetDockerRun()
					}
				} else {
					parts = append(parts, containerState(c, s))
				}
			}
			id := k.GetPluginId() + "/" + k.GetDependency().GetName()
			g.row(k.GetDependency().GetName(), strings.Join(parts, " · "), nil, nil)
			if command != "" {
				a.containerDisclosure(c, id+"/command", style.T(c, "plugins.command"), command, false)
			}
			if e := s.GetError(); e != "" {
				a.containerDisclosure(c, id+"/error", style.T(c, "plugins.containerError"), e, true)
			}
		}
		if t.err != nil {
			g.note(errorMessage(c, t.err), true)
		}
	})
}

// containerState tells a container's state, with its download's progress.
func containerState(c *ui.Context, s *pluginv1.ContainerStatus) string {
	switch s.GetState() {
	case pluginv1.ContainerState_CONTAINER_STATE_PULLING:
		if s.GetQueued() {
			return style.T(c, "plugins.containerQueued")
		}
		if p := s.GetProgress(); p > 0 {
			return fmt.Sprintf("%s %d%%", style.T(c, "plugins.containerPulling"), int(p*100))
		}
		return style.T(c, "plugins.containerPulling")
	case pluginv1.ContainerState_CONTAINER_STATE_STARTING:
		return style.T(c, "plugins.containerStarting")
	case pluginv1.ContainerState_CONTAINER_STATE_READY:
		return style.T(c, "plugins.containerReady")
	case pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY:
		return style.T(c, "plugins.containerUnhealthy")
	}
	return style.T(c, "plugins.containerUnavailable")
}

// containerDisclosure builds text of a container's, such as its command or
// error, shown when asked for.
func (a *App) containerDisclosure(c *ui.Context, id, label, text string, danger bool) {
	st := &a.settingsState.admin.overview
	colors := style.Colors(c)
	shown := st.shown[id]
	ui.Column(c.Key(id)).Padding(0, 16, 10).Gap(6).AlignItems(ui.Start).Children(func() {
		b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Label(label)
		fg := style.Color(colors.TextSecondary)
		if b.Hovered() {
			fg = style.Color(colors.TextPrimary)
		}
		b.Children(func() {
			icon := ui.Icon(c, style.Icon("expand", false)).Size(14, 14).TextColor(fg)
			if !shown {
				icon.Rotate(-90)
			}
			style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(fg)
		})
		if b.Clicked() {
			if st.shown == nil {
				st.shown = map[string]bool{}
			}
			st.shown[id] = !shown
		}
		if shown {
			fg := style.Color(colors.TextPrimary)
			if danger {
				fg = style.Color(colors.Danger)
			}
			ui.Box(c).FillWidth().Padding(10).Radius(6).Background(style.Color(colors.Fill)).Children(func() {
				style.Text(ui.Text(c, text), style.Type(c).Footnote).Font("SF Mono, Menlo, monospace").TextColor(fg).Selectable()
			})
		}
	})
}

// containerEngine tells the container engine the server found, or why it
// found none, and its endpoint; empty while it loads.
func (a *App) containerEngine(c *ui.Context) (engine, endpoint string) {
	st := &a.settingsState.admin.overview
	t := adminTask(a, &st.runtime, "runtime", func(ctx context.Context, s string) (*systemv1.ContainerRuntime, error) {
		res, err := a.pluginService(s).GetContainerRuntime(ctx, &systemv1.GetContainerRuntimeRequest{})
		return res.GetRuntime(), err
	})
	rt := t.value
	if rt == nil {
		return "", ""
	}
	if !rt.GetAvailable() {
		return style.T(c, "overview.containerEngineNone", "reason", rt.GetReason()), ""
	}
	name := map[string]string{"docker": "Docker", "podman": "Podman"}[rt.GetEngine()]
	parts := []string{cmpOr(name, rt.GetEngine()) + " " + rt.GetVersion(), "API " + rt.GetApiVersion()}
	if rt.GetRootless() {
		parts = append(parts, style.T(c, "overview.containerEngineRootless"))
	}
	return strings.Join(parts, " · "), rt.GetEndpoint()
}
