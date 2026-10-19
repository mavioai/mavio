package app

import (
	"context"
	"fmt"
	"maps"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// storageState is the state of the settings' Storage: the disks of the
// server's computer, its rclone, the remotes, the remote shown with its
// form, and the download clients of offline downloads.
type storageState struct {
	status    task[*storagev1.GetStatusResponse]
	disks     task[[]*storagev1.Disk]
	remotes   task[[]*storagev1.Remote]
	providers task[[]*storagev1.Provider]
	roots     map[string]*task[[]string] // each remote's root, listed to see that it is reached
	usage     map[string]*task[*storagev1.GetRemoteUsageResponse]
	folders   map[string]*task[[]string]
	qrs       map[string]*ui.SVG // the QR codes of questions' help, by the help
	form      remoteForm
	offline   offlineState
}

// remoteForm is the page of a remote, or of a new one: its settings as
// edited, and the questions of its configuration while they are answered.
type remoteForm struct {
	remote     string // the remote of the page, "new" for a new one
	name, kind string // a new one's
	values     map[string]string
	advanced   bool
	path       string // the folder shown

	question  *storagev1.ConfigQuestion
	answer    string
	scan      *helpQR // the QR code kept while rclone asks again without it
	scanState string
	waiting   bool // an answer of yes is to be sent again
	cancelled bool // cancelling, once the answer being sent is back

	busy       string
	notice     string
	err        error
	confirming bool // asks before deleting the remote
}

// offlineState is the state of the download clients of offline downloads:
// the client edited, by its index, the list's length for a new one.
type offlineState struct {
	list     task[[]*storagev1.OfflineClient]
	editing  *int
	draft    clientDraft
	busy     bool
	err      error
	removing bool
}

// clientDraft is a download client as edited; tags as typed.
type clientDraft struct {
	id, name, folder, tags        string
	enabled                       bool
	priority                      float64
	removeCompleted, removeFailed bool
}

func storagePath(remote string) string {
	if remote == "" {
		return "/settings/storage"
	}
	return "/settings/storage?remote=" + url.QueryEscape(remote)
}

func (a *App) storageStatus() *task[*storagev1.GetStatusResponse] {
	ss := &a.settingsState.admin.storage
	return adminTask(a, &ss.status, "storage-status", func(ctx context.Context, s string) (*storagev1.GetStatusResponse, error) {
		return a.storageService(s).GetStatus(ctx, &storagev1.GetStatusRequest{})
	})
}

func (a *App) storageRemotes() *task[[]*storagev1.Remote] {
	ss := &a.settingsState.admin.storage
	return adminTask(a, &ss.remotes, "storage-remotes", func(ctx context.Context, s string) ([]*storagev1.Remote, error) {
		res, err := a.storageService(s).ListRemotes(ctx, &storagev1.ListRemotesRequest{})
		return res.GetRemotes(), err
	})
}

func (a *App) storageProviders() *task[[]*storagev1.Provider] {
	ss := &a.settingsState.admin.storage
	return adminTask(a, &ss.providers, "storage-providers", func(ctx context.Context, s string) ([]*storagev1.Provider, error) {
		res, err := a.storageService(s).ListProviders(ctx, &storagev1.ListProvidersRequest{})
		return res.GetProviders(), err
	})
}

func (a *App) storageSection(c *ui.Context, r *ui.Route) {
	switch remote := r.Query("remote"); remote {
	case "":
		a.storageList(c)
	case "new":
		a.newRemote(c)
	default:
		a.remotePage(c, remote)
	}
}

func (a *App) storageList(c *ui.Context) {
	ss := &a.settingsState.admin.storage
	colors := style.Colors(c)
	sectionHeader(c, style.T(c, "settings.storage"), style.T(c, "storage.message"))
	status := a.storageStatus()
	disks := adminTask(a, &ss.disks, "disks", func(ctx context.Context, s string) ([]*storagev1.Disk, error) {
		res, err := a.storageService(s).ListDisks(ctx, &storagev1.ListDisksRequest{})
		return res.GetDisks(), err
	})
	group(c, "disks", style.T(c, "storage.disks"), style.T(c, "storage.disksFooter"), func(g *settingsGroup) {
		for _, d := range disks.value {
			diskRow(g, d)
		}
		switch {
		case disks.err != nil:
			g.note(errorMessage(c, disks.err), true)
		case disks.loaded && len(disks.value) == 0:
			g.note(style.T(c, "storage.noDisks"), false)
		}
	})
	available := status.value.GetAvailable()
	var remotes []*storagev1.Remote
	if available {
		rt, providers := a.storageRemotes(), a.storageProviders()
		remotes = rt.value
		group(c, "remotes", style.T(c, "storage.remotes"), style.T(c, "storage.remotesFooter"), func(g *settingsGroup) {
			sorted := slices.Clone(remotes)
			slices.SortFunc(sorted, func(x, y *storagev1.Remote) int {
				return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
			})
			for _, r := range sorted {
				a.remoteRow(g, r, kindOf(c, providers.value, r.GetType()))
			}
			switch {
			case rt.err != nil:
				g.note(errorMessage(c, rt.err), true)
			case rt.loaded && len(remotes) == 0:
				g.note(style.T(c, "storage.noRemotes"), false)
			}
			if g.addRow(style.T(c, "storage.add")) {
				a.router.Push(storagePath("new"))
			}
		})
	}
	group(c, "rclone", style.T(c, "storage.rclone"), "", func(g *settingsGroup) {
		s := status.value
		g.row(style.T(c, "storage.status"), "", nil, func() {
			color, text := style.Color(colors.Fill), "…"
			switch {
			case status.err != nil:
				color, text = style.Color(colors.Danger), errorMessage(c, status.err)
			case !status.loaded:
			case available:
				color, text = ui.RGB(0x34, 0xC7, 0x59), style.T(c, "storage.available")
			default:
				color, text = style.Color(colors.Danger), cmpOr(s.GetError(), style.T(c, "storage.unavailable"))
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
				dot(c, color)
				secondaryText(c, text)
			})
		})
		if s.GetVersion() != "" {
			g.row(style.T(c, "storage.version"), "", nil, func() { secondaryText(c, s.GetVersion()) })
		}
		// Whose rcd it is tells only of one reached.
		if available {
			detail, value := style.T(c, "storage.externalDetail"), style.T(c, "storage.external")
			if s.GetManaged() {
				detail, value = style.T(c, "storage.managedDetail"), style.T(c, "storage.managed")
			}
			g.row(style.T(c, "storage.rcd"), detail, nil, func() { secondaryText(c, value) })
		}
	})
	a.offlineClients(c, remotes)
}

// diskKinds are the tile and name of each kind of disk.
var diskKinds = map[storagev1.DiskKind]struct {
	icon  string
	color ui.Color
	label string
}{
	storagev1.DiskKind_DISK_KIND_UNSPECIFIED: {"disk", ui.RGB(0x8E, 0x8E, 0x93), "storage.diskUnknown"},
	storagev1.DiskKind_DISK_KIND_SSD:         {"disk", ui.RGB(0x5E, 0x5C, 0xE6), "storage.diskSsd"},
	storagev1.DiskKind_DISK_KIND_HDD:         {"disk", ui.RGB(0x5E, 0x5C, 0xE6), "storage.diskHdd"},
	storagev1.DiskKind_DISK_KIND_NETWORK:     {"network", ui.RGB(0x00, 0x7A, 0xFF), "storage.diskNetwork"},
	storagev1.DiskKind_DISK_KIND_CLOUD:       {"cloud", ui.RGB(0x30, 0xB0, 0xC7), "storage.diskCloud"},
}

// diskRow adds a disk of the server's computer: its name or path, its kind
// and file system, and how much of it is used.
func diskRow(g *settingsGroup, d *storagev1.Disk) {
	c := g.c
	colors := style.Colors(c)
	k := diskKinds[d.GetKind()]
	g.divider()
	ui.Row(c.Key(d.GetPath())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		sectionTile(c, k.icon, k.color)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, cmpOr(d.GetName(), d.GetPath())), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).
					MaxLines(1).Grow(1).MinWidth(0)
				facts := style.T(c, k.label)
				if d.GetFileSystem() != "" {
					facts += " · " + d.GetFileSystem()
				}
				style.Text(ui.Text(c, facts), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Shrink(0)
			})
			if d.GetName() != "" {
				style.Text(ui.Text(c, d.GetPath()), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(1).Selectable()
			}
			if d.HasTotalBytes() {
				usageBar(c, d.GetUsedBytes(), d.GetTotalBytes())
			} else {
				style.Text(ui.Text(c, style.T(c, "storage.noAnswer")), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary))
			}
		})
	})
}

// usageBar builds a bar of how much of a disk or a remote is used, orange
// when nearly full and red when full, and its sizes.
func usageBar(c *ui.Context, used, total int64) {
	colors := style.Colors(c)
	share := 0.0
	if total > 0 {
		share = min(1, float64(used)/float64(total))
	}
	color := ui.RGB(0x00, 0x7A, 0xFF)
	switch {
	case share >= 0.95:
		color = style.Color(colors.Danger)
	case share >= 0.85:
		color = ui.RGB(0xFF, 0x95, 0x00)
	}
	text := style.T(c, "storage.usage", "used", capacityOf(used), "total", capacityOf(total))
	ui.Column(c).Gap(4).Margin(6, 0, 0).Children(func() {
		ui.Row(c).Height(6).Radius(3).Clip().Background(style.Color(colors.Fill)).Label(text).Children(func() {
			if share > 0 {
				ui.Box(c).Height(6).Grow(float32(share)).Radius(3).Background(color)
			}
			if share < 1 {
				ui.Box(c).Height(6).Grow(float32(1 - share))
			}
		})
		style.Text(ui.Text(c, text), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
	})
}

// capacityOf returns a disk's or a remote's size in terabytes, gigabytes
// or megabytes as it is large, as file managers tell them.
func capacityOf(n int64) string {
	v, unit := float64(n)/1e6, "MB"
	switch {
	case n >= 1e12:
		v, unit = float64(n)/1e12, "TB"
	case n >= 1e9:
		v, unit = float64(n)/1e9, "GB"
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) + " " + unit
}

// remoteRow adds a remote, with a dot telling whether the server reaches
// it: listing its root, green when that succeeds and red, with the error,
// when it fails, as a remote whose login expired does; and how much of it
// is used, where its backend tells.
func (a *App) remoteRow(g *settingsGroup, r *storagev1.Remote, kind string) {
	c := g.c
	ss := &a.settingsState.admin.storage
	colors := style.Colors(c)
	name := r.GetName()
	root := adminTask(a, taskIn(&ss.roots, name), "root", func(ctx context.Context, s string) ([]string, error) {
		path := "rclone:" + name + ":"
		res, err := a.storageService(s).ListFolders(ctx, storagev1.ListFoldersRequest_builder{Path: &path}.Build())
		return res.GetFolders(), err
	}, name)
	usage := adminTask(a, taskIn(&ss.usage, name), "usage", func(ctx context.Context, s string) (*storagev1.GetRemoteUsageResponse, error) {
		return a.storageService(s).GetRemoteUsage(ctx, storagev1.GetRemoteUsageRequest_builder{Remote: &name}.Build())
	}, name)
	u := usage.value
	used, hasUsed := u.GetUsedBytes(), u.HasUsedBytes()
	if !hasUsed && u.HasTotalBytes() && u.HasFreeBytes() {
		used, hasUsed = u.GetTotalBytes()-u.GetFreeBytes(), true
	}
	state, mark := style.T(c, "storage.checking"), style.Color(colors.Fill)
	switch {
	case root.err != nil:
		state, mark = style.T(c, "storage.unreached"), style.Color(colors.Danger)
	case root.loaded:
		state, mark = style.T(c, "storage.reached"), ui.RGB(0x34, 0xC7, 0x59)
	}
	g.divider()
	b := ui.ButtonBase(c.Key(name)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(name)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		sectionTile(c, "cloud", ui.RGB(0x30, 0xB0, 0xC7))
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, name), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1).Shrink(1).MinWidth(0)
				ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(mark).Label(state)
			})
			style.Text(ui.Text(c, kind), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(1)
			switch {
			case root.err != nil:
				style.Text(ui.Text(c, errorMessage(c, root.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger)).MaxLines(2)
			case !hasUsed:
			case u.GetTotalBytes() > 0:
				usageBar(c, used, u.GetTotalBytes())
			default:
				style.Text(ui.Text(c, style.T(c, "storage.used", "used", capacityOf(used))), style.Type(c).Footnote).
					TextColor(style.Color(colors.TextSecondary))
			}
		})
		ui.Icon(c, style.Icon("forward", false)).Size(16, 16).Shrink(0).TextColor(style.Color(colors.TextTertiary))
	})
	if b.Clicked() {
		a.router.Push(storagePath(name))
	}
}

// kindKeys are the keys of the kinds of remotes whose names are not
// camelCase as they are.
var kindKeys = map[string]string{"b2": "backblaze", "cloud115": "cloudOneFifteen", "cloud123": "cloudOneTwentyThree", "s3": "amazon"}

var spacedWord = regexp.MustCompile(` (\w)`)

// kindOf returns a kind of remote's name in the UI's language, else as
// rclone describes it.
func kindOf(c *ui.Context, providers []*storagev1.Provider, typ string) string {
	k, ok := kindKeys[typ]
	if !ok {
		k = spacedWord.ReplaceAllStringFunc(typ, func(s string) string { return strings.ToUpper(s[1:]) })
	}
	key := "storage.provider." + k
	if s := style.T(c, key); s != key {
		return s
	}
	for _, p := range providers {
		if p.GetName() == typ && p.GetDescription() != "" {
			return p.GetDescription()
		}
	}
	return typ
}

// providerMatches reports whether an rclone provider filter, "AWS,Ceph"
// or "!AWS,Ceph", takes a provider.
func providerMatches(filter, provider string) bool {
	if filter == "" {
		return true
	}
	negated := strings.HasPrefix(filter, "!")
	list := strings.Split(strings.TrimPrefix(filter, "!"), ",")
	for i := range list {
		list[i] = strings.TrimSpace(list[i])
	}
	return slices.Contains(list, provider) != negated
}

// providerOf returns the provider values choose among a kind's.
func providerOf(options []*storagev1.Option, values map[string]string) string {
	if v, ok := values["provider"]; ok {
		return v
	}
	for _, o := range options {
		if o.GetName() == "provider" {
			return o.GetDefaultValue()
		}
	}
	return ""
}

// shownOptions returns the options of a kind shown for the values set:
// those its provider takes.
func shownOptions(options []*storagev1.Option, values map[string]string, advanced bool) []*storagev1.Option {
	provider := providerOf(options, values)
	var shown []*storagev1.Option
	for _, o := range options {
		if (advanced || !o.GetAdvanced() || o.GetRequired()) && providerMatches(o.GetProvider(), provider) {
			shown = append(shown, o)
		}
	}
	return shown
}

// changedValues returns the values to send: those set, differing from
// their defaults.
func changedValues(options []*storagev1.Option, values map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range values {
		i := slices.IndexFunc(options, func(o *storagev1.Option) bool { return o.GetName() == k })
		if i < 0 {
			if v != "" {
				out[k] = v
			}
			continue
		}
		def := options[i].GetDefaultValue()
		if v != def && (v != "" || def == "") && strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

// optionControl builds the control of an option's value, set or else
// its default, and returns a new value.
func optionControl(c *ui.Context, o *storagev1.Option, value string, set bool, provider, placeholder string) (string, bool) {
	var examples []*storagev1.Example
	for _, e := range o.GetExamples() {
		if providerMatches(e.GetProvider(), provider) {
			examples = append(examples, e)
		}
	}
	current := value
	if !set {
		current = o.GetDefaultValue()
	}
	if placeholder == "" {
		placeholder = o.GetDefaultValue()
	}
	if o.GetType() == "bool" {
		on := current == "true"
		if ui.Switch(c, &on).Label(o.GetName()).Changed() {
			return strconv.FormatBool(on), true
		}
		return "", false
	}
	if len(examples) > 0 && o.GetExclusive() {
		var values, labels []string
		for _, e := range examples {
			label := e.GetValue()
			if h, _, _ := strings.Cut(e.GetHelp(), "\n"); h != "" {
				label += " — " + h
			}
			values, labels = append(values, e.GetValue()), append(labels, label)
		}
		if !slices.Contains(values, current) {
			values, labels = append([]string{current}, values...), append([]string{cmpOr(current, "—")}, labels...)
		}
		v := current
		chooseString(c, o.GetName(), &v, values, labels)
		return v, v != current
	}
	v := value
	if !set {
		v = ""
	}
	var in ui.Element
	if len(examples) > 0 && !o.GetPassword() {
		suggestions := make([]string, len(examples))
		for i, e := range examples {
			suggestions[i] = e.GetValue()
		}
		in = ui.Autocomplete(c, &v, suggestions)
		// On the television up and down move focus on from it while its
		// suggestions are closed (docs/tv.md §4).
		if style.TV(c) {
			in.PassArrows()
		}
	} else {
		in = ui.TextInput(c, &v)
		if o.GetPassword() {
			in.Password()
		}
	}
	in.Width(288).Label(o.GetName()).Placeholder(placeholder)
	return v, in.Changed()
}

// optionsGroup builds a remote's settings: its kind's options, with
// those rarely changed behind a switch; placeholder names the
// placeholder of an option, if not its default.
func optionsGroup(c *ui.Context, options []*storagev1.Option, values map[string]string, advanced *bool, placeholder func(o *storagev1.Option) string, changed func()) {
	colors := style.Colors(c)
	provider := providerOf(options, values)
	group(c, "options", style.T(c, "storage.settings"), style.T(c, "storage.settingsFooter"), func(g *settingsGroup) {
		for _, o := range shownOptions(options, values, *advanced) {
			g.divider()
			ui.Row(c.Key(o.GetName())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Column(c).Grow(1).MinWidth(240).Gap(2).Children(func() {
					ui.Row(c).Children(func() {
						style.Text(ui.Text(c, o.GetName()), style.Type(c).Body).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextPrimary))
						if o.GetRequired() {
							style.Text(ui.Text(c, " *"), style.Type(c).Body).TextColor(style.Color(colors.Danger))
						}
					})
					if help := strings.TrimSpace(o.GetHelp()); help != "" {
						style.Text(ui.Text(c, help), style.Type(c).Caption).FontWeight(400).TextColor(style.Color(colors.TextSecondary)).Selectable()
					}
				})
				p := ""
				if placeholder != nil {
					p = placeholder(o)
				}
				value, set := values[o.GetName()]
				if v, ok := optionControl(c, o, value, set, provider, p); ok {
					values[o.GetName()] = v
					changed()
				}
			})
		}
		if slices.ContainsFunc(options, func(o *storagev1.Option) bool { return o.GetAdvanced() && !o.GetRequired() }) {
			g.switchRow(style.T(c, "storage.advanced"), "", advanced)
		}
	})
}

// remoteName is the pattern of remotes' names.
var remoteName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.+@-]*$`)

func (a *App) newRemote(c *ui.Context) {
	ss := &a.settingsState.admin.storage
	f := &ss.form
	if f.remote != "new" {
		*f = remoteForm{remote: "new", values: map[string]string{}}
	}
	a.backTo(c, style.T(c, "settings.storage"), storagePath(""))
	name := strings.TrimSpace(f.name)
	if f.question != nil {
		sectionHeader(c, name, style.T(c, "storage.questionMessage"))
		a.questionStep(c, name, true)
		return
	}
	sectionHeader(c, style.T(c, "storage.new"), "")
	providers, remotes := a.storageProviders(), a.storageRemotes()
	sorted := slices.Clone(providers.value)
	slices.SortFunc(sorted, func(x, y *storagev1.Provider) int {
		return strings.Compare(strings.ToLower(kindOf(c, nil, x.GetName())), strings.ToLower(kindOf(c, nil, y.GetName())))
	})
	taken := slices.ContainsFunc(remotes.value, func(r *storagev1.Remote) bool { return r.GetName() == name })
	group(c, "remote", "", "", func(g *settingsGroup) {
		detail := style.T(c, "storage.nameDetail")
		if taken {
			detail = style.T(c, "storage.nameTaken")
		}
		g.row(style.T(c, "storage.name"), detail, nil, func() {
			ui.TextInput(c, &f.name).Width(288).Label(style.T(c, "storage.name"))
		})
		g.row(style.T(c, "storage.kind"), "", nil, func() {
			values, labels := []string{""}, []string{style.T(c, "storage.chooseKind")}
			for _, p := range sorted {
				values, labels = append(values, p.GetName()), append(labels, fmt.Sprintf("%s (%s)", kindOf(c, providers.value, p.GetName()), p.GetName()))
			}
			kind := f.kind
			chooseString(c, style.T(c, "storage.kind"), &f.kind, values, labels)
			if f.kind != kind {
				f.values, f.advanced = map[string]string{}, false
			}
		})
		if providers.err != nil {
			g.note(errorMessage(c, providers.err), true)
		}
	})
	i := slices.IndexFunc(sorted, func(p *storagev1.Provider) bool { return p.GetName() == f.kind })
	var provider *storagev1.Provider
	if i >= 0 {
		provider = sorted[i]
		optionsGroup(c, provider.GetOptions(), f.values, &f.advanced, nil, func() {})
	}
	missing := provider == nil || slices.ContainsFunc(shownOptions(provider.GetOptions(), f.values, false), func(o *storagev1.Option) bool {
		v, ok := f.values[o.GetName()]
		if !ok {
			v = o.GetDefaultValue()
		}
		return o.GetRequired() && v == ""
	})
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "storage.create")
		if f.busy == "create" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!remoteName.MatchString(name) || taken || missing || f.busy != "").Clicked() {
			typ, params := f.kind, changedValues(provider.GetOptions(), f.values)
			a.remoteAction("create", func(ctx context.Context, s string) (*storagev1.ConfigQuestion, error) {
				res, err := a.storageService(s).CreateRemote(ctx, storagev1.CreateRemoteRequest_builder{Name: &name, Type: &typ, Parameters: params}.Build())
				return res.GetQuestion(), err
			}, func(q *storagev1.ConfigQuestion) {
				if q != nil {
					a.askQuestion(q)
					return
				}
				a.replace(storagePath(name))
			})
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
}

// remoteAction runs a change of the remote's form in the background,
// what showing busy until it ends; done takes the question the
// configuration asks next on the main thread, if it succeeded.
func (a *App) remoteAction(what string, fn func(ctx context.Context, server string) (*storagev1.ConfigQuestion, error), done func(q *storagev1.ConfigQuestion)) {
	ss := &a.settingsState.admin.storage
	f := &ss.form
	f.busy, f.err, f.notice = what, nil, ""
	server := a.chosenServer().GetId()
	go func() {
		q, err := fn(a.ctx, server)
		a.opts.Update(func() {
			f.busy = ""
			if err != nil {
				f.err = err
				return
			}
			a.settingsState.admin.reload++
			done(q)
		})
	}()
}

// askQuestion shows a question of the remote's configuration.
func (a *App) askQuestion(q *storagev1.ConfigQuestion) {
	f := &a.settingsState.admin.storage.form
	f.question, f.answer, f.cancelled, f.waiting = q, q.GetOption().GetDefaultValue(), false, false
}

func (a *App) remotePage(c *ui.Context, name string) {
	ss := &a.settingsState.admin.storage
	f := &ss.form
	if f.remote != name {
		*f = remoteForm{remote: name, path: "rclone:" + name + ":"}
	}
	a.backTo(c, style.T(c, "settings.storage"), storagePath(""))
	remotes, providers := a.storageRemotes(), a.storageProviders()
	if f.question != nil {
		sectionHeader(c, name, style.T(c, "storage.questionMessage"))
		a.questionStep(c, name, false)
		return
	}
	if taskStatus(c, remotes, func() { ss.remotes.key = "" }, nil) {
		return
	}
	i := slices.IndexFunc(remotes.value, func(r *storagev1.Remote) bool { return r.GetName() == name })
	if i < 0 {
		sectionHeader(c, name, "")
		secondaryText(c, style.T(c, "storage.noRemote"))
		return
	}
	remote := remotes.value[i]
	sectionHeader(c, name, kindOf(c, providers.value, remote.GetType()))
	if providers.err != nil {
		dangerText(c, errorMessage(c, providers.err))
	}
	if j := slices.IndexFunc(providers.value, func(p *storagev1.Provider) bool { return p.GetName() == remote.GetType() }); j >= 0 {
		a.remoteForm(c, remote, providers.value[j])
	}
	a.remoteFolders(c, name)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		if dangerButton(c, style.T(c, "storage.remove"), f.busy != "") {
			f.confirming = true
		}
		if f.busy == "delete" {
			secondaryText(c, "…")
		}
	})
	if ui.AlertDialog(c, &f.confirming, style.T(c, "storage.removeConfirm", "name", name), style.T(c, "storage.removeDetail"),
		style.T(c, "action.cancel"), style.T(c, "storage.remove")) == 1 {
		a.remoteAction("delete", func(ctx context.Context, s string) (*storagev1.ConfigQuestion, error) {
			_, err := a.storageService(s).DeleteRemote(ctx, storagev1.DeleteRemoteRequest_builder{Name: &name}.Build())
			return nil, err
		}, func(*storagev1.ConfigQuestion) { a.replace(storagePath("")) })
	}
}

// remoteForm builds a remote's settings, saved, or configured again.
// Secrets are never listed: an empty one keeps what is set.
func (a *App) remoteForm(c *ui.Context, remote *storagev1.Remote, provider *storagev1.Provider) {
	f := &a.settingsState.admin.storage.form
	if f.values == nil {
		f.values = maps.Clone(remote.GetParameters())
		if f.values == nil {
			f.values = map[string]string{}
		}
	}
	options := provider.GetOptions()
	secret := func(o *storagev1.Option) bool { return o.GetPassword() || o.GetSensitive() }
	changes := map[string]string{}
	for k, v := range f.values {
		i := slices.IndexFunc(options, func(o *storagev1.Option) bool { return o.GetName() == k })
		if i < 0 {
			continue
		}
		if secret(options[i]) && v != "" || !secret(options[i]) && v != remote.GetParameters()[k] {
			changes[k] = v
		}
	}
	placeholder := func(o *storagev1.Option) string {
		if secret(o) {
			return style.T(c, "storage.secretKept")
		}
		return ""
	}
	optionsGroup(c, options, f.values, &f.advanced, placeholder, func() { f.notice = "" })
	name := remote.GetName()
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "action.save")
		if f.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(len(changes) == 0 || f.busy != "").Clicked() {
			a.updateRemote(name, changes, "storage.saved")
		}
		if smallButton(c, style.T(c, "storage.reconfigure"), f.busy != "") {
			a.updateRemote(name, map[string]string{}, "storage.reconfigured")
		}
		if f.notice != "" && len(changes) == 0 {
			secondaryText(c, style.T(c, f.notice))
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
}

// updateRemote changes a remote's settings, which configures it again,
// asking what it needs; notice is said once it is done.
func (a *App) updateRemote(name string, params map[string]string, notice string) {
	f := &a.settingsState.admin.storage.form
	a.remoteAction("save", func(ctx context.Context, s string) (*storagev1.ConfigQuestion, error) {
		res, err := a.storageService(s).UpdateRemote(ctx, storagev1.UpdateRemoteRequest_builder{Name: &name, Parameters: params}.Build())
		if err == nil && res.GetQuestion() == nil {
			a.opts.Update(func() { f.values = maps.Clone(res.GetRemote().GetParameters()) })
		}
		return res.GetQuestion(), err
	}, func(q *storagev1.ConfigQuestion) {
		if q != nil {
			a.askQuestion(q)
			return
		}
		f.notice = notice
	})
}

// questionStep builds the questions of a remote's configuration — a yes
// or no, a choice, a value, or the token of authorizing a cloud drive —
// answered until none is left. A question drawing a QR code, as 115's
// login asks, is answered yes again and again until the phone confirms
// the login, its code kept on screen while rclone asks again without it.
// Cancelling stops asking once no answer is being sent, and deletes a
// new remote, leaving an existing one as its answers left it.
func (a *App) questionStep(c *ui.Context, name string, isNew bool) {
	ss := &a.settingsState.admin.storage
	f := &ss.form
	colors := style.Colors(c)
	q := f.question
	o := q.GetOption()
	drawn := qrOf(o.GetHelp())
	qr := drawn
	if qr == nil && f.scan != nil && f.scanState == q.GetState() {
		qr = f.scan
	}
	confirm := o.GetType() == "bool"
	waiting := qr != nil && confirm && f.err == nil && !f.cancelled
	pending := f.busy == "answer"
	send := func(answer string) {
		state := q.GetState()
		f.busy, f.err = "answer", nil
		server := a.chosenServer().GetId()
		go func() {
			res, err := a.storageService(server).ContinueRemote(a.ctx, storagev1.ContinueRemoteRequest_builder{Name: &name, State: &state, Answer: &answer}.Build())
			a.opts.Update(func() {
				f.busy = ""
				switch {
				case f.cancelled:
					a.cancelQuestion(name, isNew)
				case err != nil:
					f.err = err
				case res.GetQuestion() == nil:
					a.settingsState.admin.reload++
					if isNew {
						a.replace(storagePath(name))
						return
					}
					f.question, f.values, f.notice = nil, nil, "storage.saved"
				default:
					if drawn != nil {
						f.scan, f.scanState = drawn, state
					}
					a.askQuestion(res.GetQuestion())
				}
			})
		}()
	}
	// Each yes waits for the last; the one sent answers the question shown.
	if waiting && !pending && !f.waiting {
		f.waiting = true
		go func() {
			select {
			case <-time.After(2500 * time.Millisecond):
			case <-a.ctx.Done():
				return
			}
			a.opts.Update(func() {
				if f.question == q && f.waiting && !f.cancelled && f.busy == "" {
					f.waiting = false
					send("true")
				}
			})
		}()
	}
	footer := ""
	if o.GetName() == "config_token" {
		footer = style.T(c, "storage.questionFooter")
	}
	group(c, "question", "", footer, func(g *settingsGroup) {
		g.block(func() {
			help := strings.TrimSpace(o.GetHelp())
			switch {
			case o == nil:
				secondaryText(c, style.T(c, "storage.questionNone"))
			case qr != nil:
				if qr.before != "" {
					style.Text(ui.Text(c, qr.before), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
				}
				if svg := ss.qrSVG(qr); svg != nil {
					ui.Box(c).AlignSelf(ui.Start).Padding(8).Radius(8).Background(ui.RGB(255, 255, 255)).Children(func() {
						ui.Image(c, svg).Size(208, 208).Label("QR")
					})
				}
				if !f.cancelled {
					text := qr.after
					if waiting {
						text = style.T(c, "storage.waitingScan")
					}
					style.Text(ui.Text(c, text), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				}
			case help != "":
				style.Text(ui.Text(c, help), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
			}
			if o != nil && !confirm {
				if v, ok := optionControl(c, o, f.answer, true, "", ""); ok {
					f.answer = v
				}
			}
		})
		if q.GetError() != "" {
			g.note(q.GetError(), true)
		}
	})
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		switch {
		case waiting || f.cancelled:
		case confirm:
			yes, no := style.T(c, "storage.yes"), style.T(c, "storage.no")
			if o.GetDefaultValue() == "false" {
				if ui.PrimaryButton(c, no).Disabled(pending).Clicked() {
					send("false")
				}
				if smallButton(c, yes, pending) {
					send("true")
				}
			} else {
				if ui.PrimaryButton(c, yes).Disabled(pending).Clicked() {
					send("true")
				}
				if smallButton(c, no, pending) {
					send("false")
				}
			}
		default:
			label := style.T(c, "storage.continue")
			if pending {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(pending).Clicked() {
				send(f.answer)
			}
		}
		label := style.T(c, "action.cancel")
		if f.cancelled {
			label = style.T(c, "storage.cancelling")
		}
		if linkButton(c, label).Disabled(f.cancelled).Clicked() && !f.cancelled {
			f.cancelled, f.waiting = true, false
			if !pending {
				a.cancelQuestion(name, isNew)
			}
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
}

// cancelQuestion stops asking: a new remote is deleted, and an existing
// one is left as its answers left it.
func (a *App) cancelQuestion(name string, isNew bool) {
	f := &a.settingsState.admin.storage.form
	if !isNew {
		f.question, f.cancelled = nil, false
		return
	}
	server := a.chosenServer().GetId()
	go func() {
		_, _ = a.storageService(server).DeleteRemote(a.ctx, storagev1.DeleteRemoteRequest_builder{Name: &name}.Build())
		a.opts.Update(func() {
			a.settingsState.admin.reload++
			f.remote = ""
			a.replace(storagePath(""))
		})
	}()
}

// helpQR is a help split around a QR code drawn in it, as rclone's
// configuration questions draw one for a terminal: rows of half blocks,
// each character two modules high, a block where a module is light. dark
// holds its modules, true where dark.
type helpQR struct {
	before, after string
	dark          [][]bool
}

// qrOf returns the QR code an option's help draws, if it draws one.
func qrOf(help string) *helpQR {
	lines := strings.Split(help, "\n")
	art := func(l string) bool {
		return len([]rune(l)) >= 21 && strings.Trim(l, " █▀▄") == "" && strings.ContainsAny(l, "█▀▄")
	}
	start := slices.IndexFunc(lines, art)
	if start < 0 {
		return nil
	}
	end := start
	for end < len(lines) && art(lines[end]) {
		end++
	}
	var dark [][]bool
	for i, l := range lines[start:end] {
		chars := []rune(l)
		top, bottom := make([]bool, len(chars)), make([]bool, len(chars))
		for j, ch := range chars {
			top[j], bottom[j] = ch != '█' && ch != '▀', ch != '█' && ch != '▄'
		}
		dark = append(dark, top)
		// An odd last row is drawn as the top halves of its line alone.
		if i < end-start-1 || strings.ContainsAny(l, "█▄") {
			dark = append(dark, bottom)
		}
	}
	if len(dark) < 21 {
		return nil
	}
	return &helpQR{
		before: strings.TrimSpace(strings.Join(lines[:start], "\n")),
		dark:   dark,
		after:  strings.TrimSpace(strings.Join(lines[end:], "\n")),
	}
}

// qrSVG returns the image of a QR code, dark on light whatever the theme,
// for a phone to scan.
func (ss *storageState) qrSVG(qr *helpQR) *ui.SVG {
	var b strings.Builder
	width := 0
	for _, row := range qr.dark {
		width = max(width, len(row))
	}
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><path fill="#000" d="`, width, len(qr.dark))
	for y, row := range qr.dark {
		for x, on := range row {
			if on {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	k := b.String()
	if svg, ok := ss.qrs[k]; ok {
		return svg
	}
	svg, err := ui.ParseSVG([]byte(k))
	if err != nil {
		return nil
	}
	if ss.qrs == nil {
		ss.qrs = map[string]*ui.SVG{}
	}
	ss.qrs[k] = svg
	return svg
}

// remoteFolders builds a remote's folders, to walk and to see that it is
// reached.
func (a *App) remoteFolders(c *ui.Context, name string) {
	ss := &a.settingsState.admin.storage
	f := &ss.form
	colors := style.Colors(c)
	root := "rclone:" + name + ":"
	path := f.path
	t := adminTask(a, taskIn(&ss.folders, path), "folders", func(ctx context.Context, s string) ([]string, error) {
		res, err := a.storageService(s).ListFolders(ctx, storagev1.ListFoldersRequest_builder{Path: &path}.Build())
		return res.GetFolders(), err
	}, path)
	parent, hasParent := "", path != root
	if hasParent {
		parent = root
		if i := strings.LastIndex(path, "/"); i >= 0 {
			parent = path[:i]
		}
	}
	group(c, "folders", style.T(c, "storage.folders"), style.T(c, "storage.foldersFooter"), func(g *settingsGroup) {
		g.divider()
		style.Text(ui.Text(c, path), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(10, 16).Selectable()
		g.divider()
		ui.Scroll(c.Key(path)).MaxHeight(320).Children(func() {
			ui.Column(c).Children(func() {
				row := func(key, label, icon, to string) {
					b := ui.ButtonBase(c.Key(key)).MinHeight(44).Padding(0, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(label)
					if b.Hovered() {
						b.Background(style.Color(colors.Fill))
					}
					b.Children(func() {
						ui.Icon(c, style.Icon(icon, false)).Size(18, 18).Shrink(0).TextColor(style.Color(colors.TextSecondary))
						style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1).Grow(1).MinWidth(0)
					})
					if b.Clicked() {
						f.path = to
					}
				}
				if hasParent {
					row("..", style.T(c, "libraries.parentFolder"), "back", parent)
				}
				for _, p := range t.value {
					sep := ":"
					if strings.Contains(p, "/") {
						sep = "/"
					}
					row(p, p[strings.LastIndex(p, sep)+1:], "folder", p)
				}
				note := func(text string, color ui.Color) {
					style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(color).Padding(12, 16).Selectable()
				}
				switch {
				case t.err != nil:
					note(errorMessage(c, t.err), style.Color(colors.Danger))
				case !t.loaded:
					note("…", style.Color(colors.TextSecondary))
				case len(t.value) == 0:
					note(style.T(c, "libraries.noFolders"), style.Color(colors.TextSecondary))
				}
			})
		})
	})
}

func clientDraftOf(o *storagev1.OfflineClient) clientDraft {
	return clientDraft{
		id: o.GetId(), name: o.GetName(), folder: o.GetFolder(), tags: strings.Join(o.GetTags(), ", "),
		enabled: o.GetEnabled(), priority: float64(o.GetPriority()),
		removeCompleted: o.GetRemoveCompletedDownloads(), removeFailed: o.GetRemoveFailedDownloads(),
	}
}

func (d clientDraft) client() *storagev1.OfflineClient {
	var tags []string
	for t := range strings.SplitSeq(d.tags, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return storagev1.OfflineClient_builder{
		Id: new(strings.TrimSpace(d.id)), Name: new(strings.TrimSpace(d.name)), Enabled: new(d.enabled),
		Priority: new(int32(max(1, d.priority))), Folder: new(strings.TrimSpace(d.folder)), Tags: tags,
		RemoveCompletedDownloads: new(d.removeCompleted), RemoveFailedDownloads: new(d.removeFailed),
	}.Build()
}

var notID = regexp.MustCompile(`[^a-z0-9_-]+`)

// idFor returns an ID for a client named so, unique among the others.
func idFor(name string, taken []string) string {
	base := strings.Trim(notID.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "offline"
	}
	id := base
	for i := 2; slices.Contains(taken, id); i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return id
}

// offlineClients builds the download clients of offline downloads: cloud
// drives that download magnet links into a folder of their remote.
func (a *App) offlineClients(c *ui.Context, remotes []*storagev1.Remote) {
	off := &a.settingsState.admin.storage.offline
	colors := style.Colors(c)
	t := adminTask(a, &off.list, "offline", func(ctx context.Context, s string) ([]*storagev1.OfflineClient, error) {
		res, err := a.storageService(s).ListOfflineClients(ctx, &storagev1.ListOfflineClientsRequest{})
		return res.GetClients(), err
	})
	list := t.value
	edit := func(i int, d clientDraft) {
		off.editing, off.draft, off.err, off.removing = &i, d, nil, false
	}
	control := func() {
		if off.editing == nil && t.loaded && t.err == nil && linkButton(c, style.T(c, "storage.addOffline")).Clicked() {
			edit(len(list), clientDraft{enabled: true, priority: 1, removeCompleted: true})
		}
	}
	groupWith(c, "offline", style.T(c, "storage.offline"), control, style.T(c, "storage.offlineFooter"), func(g *settingsGroup) {
		for i, o := range list {
			if off.editing != nil && *off.editing == i {
				a.clientForm(g, list, i, remotes)
				continue
			}
			color := ui.RGB(0x8E, 0x8E, 0x93)
			if o.GetEnabled() {
				color = ui.RGB(0x34, 0xC7, 0x59)
			}
			parts := []string{o.GetFolder()}
			if !o.GetEnabled() {
				parts = append(parts, style.T(c, "storage.disabled"))
			}
			if len(o.GetTags()) > 0 {
				parts = append(parts, strings.Join(o.GetTags(), ", "))
			}
			g.divider()
			b := ui.ButtonBase(c.Key(o.GetId())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).
				Label(o.GetName()).Disabled(off.editing != nil)
			if b.Hovered() && off.editing == nil {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				sectionTile(c, "downloads", color)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					style.Text(ui.Text(c, o.GetName()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1)
					style.Text(ui.Text(c, strings.Join(parts, " · ")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(1)
				})
			})
			if b.Clicked() && off.editing == nil {
				edit(i, clientDraftOf(o))
			}
		}
		if off.editing != nil && *off.editing == len(list) {
			a.clientForm(g, list, len(list), remotes)
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(list) == 0 && off.editing == nil:
			g.note(style.T(c, "storage.noOffline"), false)
		}
	})
}

// offlineFolder is the pattern of offline downloads' folders.
var offlineFolder = regexp.MustCompile(`^rclone:[^:/\\]+:`)

// clientForm adds the form of the download client at i of list, a new
// one at its length.
func (a *App) clientForm(g *settingsGroup, list []*storagev1.OfflineClient, i int, remotes []*storagev1.Remote) {
	c := g.c
	off := &a.settingsState.admin.storage.offline
	d := &off.draft
	isNew := i == len(list)
	text := func(value *string, label, detail, placeholder string) {
		g.row(label, detail, nil, func() {
			ui.TextInput(c, value).Width(288).Label(label).Placeholder(placeholder)
		})
	}
	folder := "rclone:pikpak:downloads"
	if len(remotes) > 0 {
		folder = "rclone:" + remotes[0].GetName() + ":downloads"
	}
	text(&d.name, style.T(c, "storage.offlineName"), "", "PikPak")
	text(&d.folder, style.T(c, "storage.offlineFolder"), style.T(c, "storage.offlineFolderDetail"), folder)
	text(&d.tags, style.T(c, "storage.offlineTags"), style.T(c, "storage.offlineTagsDetail"), "")
	g.row(style.T(c, "storage.offlinePriority"), style.T(c, "storage.offlinePriorityDetail"), nil, func() {
		ui.NumberInput(c, &d.priority, 1, 50, 1).Label(style.T(c, "storage.offlinePriority"))
	})
	g.switchRow(style.T(c, "storage.offlineEnabled"), "", &d.enabled)
	g.switchRow(style.T(c, "storage.removeCompleted"), "", &d.removeCompleted)
	g.switchRow(style.T(c, "storage.removeFailed"), "", &d.removeFailed)
	save := func(next []*storagev1.OfflineClient) {
		off.busy, off.err = true, nil
		server := a.chosenServer().GetId()
		go func() {
			_, err := a.storageService(server).SetOfflineClients(a.ctx, storagev1.SetOfflineClientsRequest_builder{Clients: next}.Build())
			a.opts.Update(func() {
				off.busy, off.err = false, err
				if err == nil {
					off.editing = nil
					a.settingsState.admin.reload++
				}
			})
		}()
	}
	valid := strings.TrimSpace(d.name) != "" && offlineFolder.MatchString(strings.TrimSpace(d.folder))
	g.block(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			label := style.T(c, "action.save")
			if off.busy {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(!valid || off.busy).Clicked() {
				draft := *d
				next := slices.Clone(list)
				if isNew {
					var taken []string
					for _, o := range list {
						taken = append(taken, o.GetId())
					}
					draft.id = idFor(d.name, taken)
					next = append(next, draft.client())
				} else {
					next[i] = draft.client()
				}
				save(next)
			}
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				off.editing = nil
			}
			if !isNew {
				if off.removing {
					if dangerButton(c, style.T(c, "storage.removeOfflineConfirm"), off.busy) {
						save(slices.Delete(slices.Clone(list), i, i+1))
					}
				} else if dangerButton(c, style.T(c, "storage.removeOffline"), false) {
					off.removing = true
				}
			}
			if off.err != nil {
				dangerText(c, errorMessage(c, off.err))
			}
		})
	})
}
