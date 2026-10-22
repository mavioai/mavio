package app

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// acquisitionState is the state of the settings' Acquisition: what the
// server chosen grabs releases with and by, and the form of the page
// shown.
type acquisitionState struct {
	indexers    task[*acquisitionv1.ListIndexersResponse]
	clients     task[*acquisitionv1.ListDownloadClientsResponse]
	plugins     task[[]*systemv1.Plugin]
	profiles    task[[]*acquisitionv1.QualityProfile]
	definitions map[string]*task[[]*acquisitionv1.QualityDefinition] // by kind
	formats     task[[]*acquisitionv1.CustomFormat]
	delays      task[[]*acquisitionv1.DelayProfile]
	releases    task[[]*acquisitionv1.ReleaseProfile]
	settings    task[*acquisitionv1.AcquisitionSettings]
	paths       task[*acquisitionv1.GetDownloadClientPathsResponse]

	page  string                                     // the page the form is of, its query
	kind  int                                        // the index of the kind whose qualities are shown
	sizes map[int32]*acquisitionv1.QualityDefinition // the qualities' sizes changed

	// The quality profile or custom format edited, and as saved.
	profile, profile0 *acquisitionv1.QualityProfile
	format, format0   *acquisitionv1.CustomFormat

	editing  *string // the ID of the delay or release profile edited, "new" for a new one
	delay    delayDraft
	release  releaseDraft
	options  optionsForm
	requests requestsForm
	folders  *folderDrafts // the download client's folders edited

	busy       string
	notice     string
	err        error
	confirming bool // asks before deleting the profile or format
	removing   bool // asks again before removing the entry edited
}

// mediaKinds are the kinds of quality profiles and definitions, named.
var mediaKinds = []struct {
	kind  acquisitionv1.MediaKind
	title string
}{
	{acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, "acquisition.movies"},
	{acquisitionv1.MediaKind_MEDIA_KIND_SERIES, "acquisition.series"},
}

func mediaKindTitle(c *ui.Context, k acquisitionv1.MediaKind) string {
	for _, m := range mediaKinds {
		if m.kind == k {
			return style.T(c, m.title)
		}
	}
	return ""
}

// releaseLanguages are the languages releases are parsed into, by their
// English names, as libs/release names them.
var releaseLanguages = []string{
	"English", "French", "Spanish", "German", "Italian", "Danish", "Dutch", "Japanese", "Icelandic", "Chinese", "Russian",
	"Polish", "Vietnamese", "Swedish", "Norwegian", "Finnish", "Turkish", "Portuguese", "Flemish", "Greek", "Korean",
	"Hungarian", "Hebrew", "Lithuanian", "Czech", "Arabic", "Hindi", "Bulgarian", "Malayalam", "Ukrainian", "Slovak", "Thai",
	"Portuguese (Brazil)", "Spanish (Latino)", "Romanian", "Latvian", "Persian", "Catalan", "Croatian", "Serbian", "Bosnian",
	"Estonian", "Tamil", "Indonesian", "Macedonian", "Slovenian", "Bengali", "Telugu", "Kannada", "Albanian", "Afrikaans",
	"Marathi", "Tagalog", "Urdu", "Romansh", "Mongolian", "Georgian",
}

func protocolTitle(c *ui.Context, p acquisitionv1.DownloadProtocol) string {
	switch p {
	case acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT:
		return style.T(c, "acquisition.torrent")
	case acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET:
		return style.T(c, "acquisition.usenet")
	}
	return ""
}

func privacyTitle(c *ui.Context, p acquisitionv1.IndexerPrivacy) string {
	switch p {
	case acquisitionv1.IndexerPrivacy_INDEXER_PRIVACY_PUBLIC:
		return style.T(c, "acquisition.public")
	case acquisitionv1.IndexerPrivacy_INDEXER_PRIVACY_SEMI_PRIVATE:
		return style.T(c, "acquisition.semiPrivate")
	case acquisitionv1.IndexerPrivacy_INDEXER_PRIVACY_PRIVATE:
		return style.T(c, "acquisition.private")
	}
	return ""
}

// joinDetail joins the parts of a row's line that are set.
func joinDetail(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), " · ")
}

// leftOut tells why an indexer or download client is left out: it is
// off, or failed, until when.
func leftOut(c *ui.Context, enabled bool, failure string, until *timestamppb.Timestamp) string {
	if !enabled {
		return style.T(c, "acquisition.off")
	}
	if until == nil || !until.AsTime().After(time.Now()) {
		return ""
	}
	return style.T(c, "acquisition.leftOut", "when", fromNow(c, until.AsTime()), "failure", failure)
}

func (a *App) profileService(id string) acquisitionv1connect.ProfileServiceClient {
	return acquisitionv1connect.NewProfileServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) releaseService(id string) acquisitionv1connect.ReleaseServiceClient {
	return acquisitionv1connect.NewReleaseServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) downloadService(id string) acquisitionv1connect.DownloadServiceClient {
	return acquisitionv1connect.NewDownloadServiceClient(a.connectHTTP(), serverURL(id))
}

// acquisitionPath returns the path of a page of the settings' Acquisition,
// the list's when page is empty.
func acquisitionPath(page, id string) string {
	q := url.Values{}
	if page != "" {
		q.Set("acquisition", page)
	}
	if id != "" {
		q.Set("id", id)
	}
	if len(q) == 0 {
		return "/settings/acquisition"
	}
	return "/settings/acquisition?" + q.Encode()
}

func (a *App) qualityProfiles() *task[[]*acquisitionv1.QualityProfile] {
	st := &a.settingsState.admin.acquisition
	return adminTask(a, &st.profiles, "quality-profiles", func(ctx context.Context, s string) ([]*acquisitionv1.QualityProfile, error) {
		res, err := a.profileService(s).ListQualityProfiles(ctx, &acquisitionv1.ListQualityProfilesRequest{})
		return res.GetProfiles(), err
	})
}

func (a *App) qualityDefinitions(kind acquisitionv1.MediaKind) *task[[]*acquisitionv1.QualityDefinition] {
	st := &a.settingsState.admin.acquisition
	k := kind.String()
	return adminTask(a, taskIn(&st.definitions, k), "quality-definitions", func(ctx context.Context, s string) ([]*acquisitionv1.QualityDefinition, error) {
		res, err := a.profileService(s).ListQualityDefinitions(ctx, acquisitionv1.ListQualityDefinitionsRequest_builder{Kind: &kind}.Build())
		return res.GetDefinitions(), err
	}, k)
}

func (a *App) customFormats() *task[[]*acquisitionv1.CustomFormat] {
	st := &a.settingsState.admin.acquisition
	return adminTask(a, &st.formats, "custom-formats", func(ctx context.Context, s string) ([]*acquisitionv1.CustomFormat, error) {
		res, err := a.profileService(s).ListCustomFormats(ctx, &acquisitionv1.ListCustomFormatsRequest{})
		return res.GetFormats(), err
	})
}

func (a *App) acquisitionIndexers() *task[*acquisitionv1.ListIndexersResponse] {
	st := &a.settingsState.admin.acquisition
	return adminTask(a, &st.indexers, "indexers", func(ctx context.Context, s string) (*acquisitionv1.ListIndexersResponse, error) {
		return a.releaseService(s).ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
	})
}

// acquisitionAction runs a change of the page shown in the background,
// what showing busy until it ends; done runs on the main thread if it
// succeeded.
func (a *App) acquisitionAction(what string, fn func(ctx context.Context, server string) error, done func()) {
	st := &a.settingsState.admin.acquisition
	st.busy, st.err, st.notice = what, nil, ""
	server := a.chosenServer().GetId()
	go func() {
		err := fn(a.ctx, server)
		a.opts.Update(func() {
			st.busy, st.err = "", err
			if err != nil {
				return
			}
			a.settingsState.admin.reload++
			if done != nil {
				done()
			}
		})
	}()
}

func (a *App) acquisitionSection(c *ui.Context, r *ui.Route) {
	st := &a.settingsState.admin.acquisition
	page, id := r.Query("acquisition"), r.Query("id")
	if k := page + "/" + id; st.page != k {
		st.page, st.sizes, st.profile, st.profile0, st.format, st.format0 = k, nil, nil, nil, nil, nil
		st.editing, st.folders, st.options, st.requests = nil, nil, optionsForm{}, requestsForm{}
		st.notice, st.err, st.confirming, st.removing = "", nil, false, false
	}
	switch page {
	case "profile":
		a.qualityProfilePage(c, id)
	case "qualities":
		a.qualityDefinitionsPage(c)
	case "formats":
		a.customFormatsPage(c)
	case "format":
		a.customFormatPage(c, id)
	case "delays":
		a.delayProfilesPage(c)
	case "releases":
		a.releaseProfilesPage(c)
	case "client":
		a.downloadClientPage(c, id)
	case "options":
		a.acquisitionOptionsPage(c)
	case "requests":
		a.requestSettingsPage(c)
	default:
		a.acquisitionList(c)
	}
}

func (a *App) acquisitionList(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	sectionHeader(c, style.T(c, "settings.acquisition"), style.T(c, "acquisition.message"))
	indexers := a.acquisitionIndexers()
	clients := adminTask(a, &st.clients, "download-clients", func(ctx context.Context, s string) (*acquisitionv1.ListDownloadClientsResponse, error) {
		return a.downloadService(s).ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
	})
	plugins := adminTask(a, &st.plugins, "plugins", func(ctx context.Context, s string) ([]*systemv1.Plugin, error) {
		res, err := a.pluginService(s).ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		return res.GetPlugins(), err
	})
	profiles := a.qualityProfiles()
	pluginName := func(id string) string {
		for _, p := range plugins.value {
			if m := p.GetManifest(); m.GetId() == id && m.GetName() != "" {
				return m.GetName()
			}
		}
		return id
	}
	// leftOutColor is the color of an indexer's or client's tile.
	tileColor := func(enabled bool, failure string, color ui.Color) ui.Color {
		switch {
		case !enabled:
			return ui.RGB(0x8E, 0x8E, 0x93)
		case failure != "":
			return ui.RGB(0xFF, 0x95, 0x00)
		}
		return color
	}
	group(c, "indexers", style.T(c, "acquisition.indexers"), style.T(c, "acquisition.indexersFooter"), func(g *settingsGroup) {
		for _, x := range indexers.value.GetIndexers() {
			failure := leftOut(c, x.GetEnabled(), x.GetFailure(), x.GetDisabledUntil())
			g.link(x.GetPluginId()+"/"+x.GetId(), "search", tileColor(x.GetEnabled(), failure, ui.RGB(0x58, 0x56, 0xD6)), x.GetName(),
				joinDetail(pluginName(x.GetPluginId()), protocolTitle(c, x.GetProtocol()), privacyTitle(c, x.GetPrivacy()),
					style.T(c, "acquisition.priority", "priority", x.GetPriority()), failure), false)
		}
		failures := indexers.value.GetFailures()
		for _, f := range failures {
			g.note(pluginName(f.GetPluginId())+": "+f.GetMessage(), true)
		}
		switch {
		case indexers.err != nil:
			g.note(errorMessage(c, indexers.err), true)
		case indexers.loaded && len(indexers.value.GetIndexers()) == 0 && len(failures) == 0:
			g.note(style.T(c, "acquisition.noIndexers"), false)
		}
	})
	group(c, "clients", style.T(c, "acquisition.clients"), style.T(c, "acquisition.clientsFooter"), func(g *settingsGroup) {
		for _, x := range clients.value.GetClients() {
			failure := leftOut(c, x.GetEnabled(), x.GetFailure(), x.GetDisabledUntil())
			offline := x.GetPluginId() == offlineProvider
			provider := pluginName(x.GetPluginId())
			if offline {
				provider = style.T(c, "storage.offline")
			}
			if g.link(x.GetPluginId()+"/"+x.GetId(), "downloads", tileColor(x.GetEnabled(), failure, ui.RGB(0x34, 0xC7, 0x59)), x.GetName(),
				joinDetail(provider, protocolTitle(c, x.GetProtocol()),
					style.T(c, "acquisition.priority", "priority", x.GetPriority()), failure), true) {
				// A remote's offline downloads are set on the remote's page.
				if offline {
					a.router.Push(storagePath(x.GetId()))
				} else {
					a.router.Push(acquisitionPath("client", x.GetPluginId()+"/"+x.GetId()))
				}
			}
		}
		failures := clients.value.GetFailures()
		for _, plugin := range slices.Sorted(func(yield func(string) bool) {
			for k := range failures {
				if !yield(k) {
					return
				}
			}
		}) {
			g.note(pluginName(plugin)+": "+failures[plugin], true)
		}
		switch {
		case clients.err != nil:
			g.note(errorMessage(c, clients.err), true)
		case clients.loaded && len(clients.value.GetClients()) == 0 && len(failures) == 0:
			g.note(style.T(c, "acquisition.noClients"), false)
		}
	})
	for _, k := range mediaKinds {
		group(c, k.kind.String(), style.T(c, "acquisition.profilesOf", "kind", style.T(c, k.title)), "", func(g *settingsGroup) {
			var list []*acquisitionv1.QualityProfile
			for _, p := range profiles.value {
				if p.GetKind() == k.kind {
					list = append(list, p)
				}
			}
			slices.SortFunc(list, func(x, y *acquisitionv1.QualityProfile) int {
				return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
			})
			for _, p := range list {
				detail := ""
				if p.GetUpgradeAllowed() {
					detail = style.T(c, "acquisition.upgrades")
				}
				if g.link(p.GetId(), "", ui.Color{}, p.GetName(), detail, true) {
					a.router.Push(acquisitionPath("profile", p.GetId()))
				}
			}
			if g.addRow(style.T(c, "acquisition.addProfile")) {
				id := "new-movie"
				if k.kind == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
					id = "new-series"
				}
				a.router.Push(acquisitionPath("profile", id))
			}
			if profiles.err != nil {
				g.note(errorMessage(c, profiles.err), true)
			}
		})
	}
	group(c, "rules", style.T(c, "acquisition.rules"), "", func(g *settingsGroup) {
		for _, l := range []struct {
			page, icon string
			color      ui.Color
			title      string
		}{
			{"qualities", "quality", ui.RGB(0xFF, 0x2D, 0x55), "qualities"},
			{"formats", "filter", ui.RGB(0xAF, 0x52, 0xDE), "formats"},
			{"delays", "pending", ui.RGB(0xFF, 0x95, 0x00), "delays"},
			{"releases", "sort", ui.RGB(0x00, 0x7A, 0xFF), "releases"},
			{"requests", "discover", ui.RGB(0x5A, 0xC8, 0xFA), "requestsettings.title"},
			{"options", "settings", ui.RGB(0x8E, 0x8E, 0x93), "options.options"},
		} {
			title := l.title
			if !strings.Contains(title, ".") {
				title = "acquisition." + title
			}
			detail := title + "Detail"
			if t, ok := strings.CutSuffix(title, ".title"); ok {
				detail = t + ".detail"
			}
			if g.link(l.page, l.icon, l.color, style.T(c, title), style.T(c, detail), true) {
				a.router.Push(acquisitionPath(l.page, ""))
			}
		}
	})
}

// numberField builds a number input of a value, which set takes when it
// changes.
func numberField(c *ui.Context, label string, value, lo, hi, step float64, set func(v float64)) {
	v := value
	if ui.NumberInput(c, &v, lo, hi, step).Width(128).Label(label).Changed() {
		set(v)
	}
}

// moveButton builds a button moving a row up, or down, and reports a
// click.
func moveButton(c *ui.Context, up, disabled bool) bool {
	colors := style.Colors(c)
	label := style.T(c, "acquisition.moveDown")
	if up {
		label = style.T(c, "acquisition.moveUp")
	}
	b := ui.ButtonBase(c).Size(28, 28).Shrink(0).Radius(6).Center().Label(label).Tooltip(label).Disabled(disabled)
	switch {
	case disabled:
		b.Opacity(0.3)
	case b.Hovered():
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		icon := ui.Icon(c, style.Icon("expand", false)).Size(16, 16).TextColor(style.Color(colors.TextSecondary))
		if up {
			icon.Rotate(180)
		}
	})
	return b.Clicked() && !disabled
}

// acquisitionSave builds a page's Save, labeled label, its notice once
// saved and the error of the last change.
func (a *App) acquisitionSave(c *ui.Context, label string, enabled, changed bool, save func()) {
	st := &a.settingsState.admin.acquisition
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		if st.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!enabled || st.busy != "").Clicked() {
			save()
		}
		if st.notice != "" && !changed {
			secondaryText(c, style.T(c, st.notice))
		}
		if st.err != nil {
			dangerText(c, errorMessage(c, st.err))
		}
	})
}

func (a *App) qualityDefinitionsPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	colors := style.Colors(c)
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "acquisition.qualities"), style.T(c, "acquisition.qualitiesMessage"))
	labels := make([]string, len(mediaKinds))
	for i, k := range mediaKinds {
		labels[i] = style.T(c, k.title)
	}
	ui.Row(c).Children(func() {
		if choose(c, style.T(c, "acquisition.kind"), &st.kind, labels) {
			st.sizes, st.notice, st.err = nil, "", nil
		}
	})
	kind := mediaKinds[st.kind].kind
	t := a.qualityDefinitions(kind)
	var changed []*acquisitionv1.QualityDefinition
	for _, saved := range t.value {
		if d := st.sizes[saved.GetQuality()]; d != nil && !proto.Equal(d, saved) {
			changed = append(changed, d)
		}
	}
	const most = 1_000_000
	group(c, "definitions", "", style.T(c, "acquisition.qualitiesFooter"), func(g *settingsGroup) {
		g.divider()
		ui.Row(c.Key("head")).MinHeight(44).Padding(0, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			head := func(key string, width float32) {
				t := style.Text(ui.Text(c, style.T(c, key)), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				if width > 0 {
					t.Width(width)
				} else {
					t.Grow(1)
				}
			}
			head("acquisition.quality", 0)
			head("acquisition.minSize", 128)
			head("acquisition.preferredSize", 128)
			head("acquisition.maxSize", 128)
		})
		for _, saved := range t.value {
			q := saved.GetQuality()
			d := saved
			if e := st.sizes[q]; e != nil {
				d = e
			}
			set := func(f func(d *acquisitionv1.QualityDefinition)) {
				n := proto.CloneOf(d)
				f(n)
				if st.sizes == nil {
					st.sizes = map[int32]*acquisitionv1.QualityDefinition{}
				}
				st.sizes[q], st.notice = n, ""
			}
			title := d.GetTitle()
			g.divider()
			ui.Row(c.Key(q)).MinHeight(52).Padding(8, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, title), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Grow(1).MinWidth(0)
				numberField(c.Key("min"), title+" "+style.T(c, "acquisition.minSize"), d.GetMinSize(), 0, most, 0.1,
					func(v float64) { set(func(d *acquisitionv1.QualityDefinition) { d.SetMinSize(v) }) })
				numberField(c.Key("preferred"), title+" "+style.T(c, "acquisition.preferredSize"), d.GetPreferredSize(), 0, most, 0.1,
					func(v float64) { set(func(d *acquisitionv1.QualityDefinition) { d.SetPreferredSize(v) }) })
				numberField(c.Key("max"), title+" "+style.T(c, "acquisition.maxSize"), d.GetMaxSize(), 0, most, 0.1,
					func(v float64) { set(func(d *acquisitionv1.QualityDefinition) { d.SetMaxSize(v) }) })
			})
		}
		if t.err != nil {
			g.note(errorMessage(c, t.err), true)
		}
	})
	a.acquisitionSave(c, style.T(c, "action.save"), len(changed) > 0, len(changed) > 0, func() {
		a.acquisitionAction("save", func(ctx context.Context, s string) error {
			_, err := a.profileService(s).UpdateQualityDefinitions(ctx, acquisitionv1.UpdateQualityDefinitionsRequest_builder{
				Kind: &kind, Definitions: changed,
			}.Build())
			return err
		}, func() { st.sizes, st.notice = nil, "acquisition.saved" })
	})
}

// itemID returns the ID an item of a quality profile is ranked by: a
// group's, else its quality's.
func itemID(it *acquisitionv1.QualityProfileItem) int32 {
	if it.GetId() != 0 {
		return it.GetId()
	}
	return it.GetQuality()
}

// qualityProfilePage builds a quality profile's page; id is "new-movie"
// or "new-series" for one to add.
func (a *App) qualityProfilePage(c *ui.Context, id string) {
	st := &a.settingsState.admin.acquisition
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	profiles := a.qualityProfiles()
	isNew := strings.HasPrefix(id, "new-")
	kind := acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	var saved *acquisitionv1.QualityProfile
	if isNew {
		if id == "new-series" {
			kind = acquisitionv1.MediaKind_MEDIA_KIND_SERIES
		}
	} else if i := slices.IndexFunc(profiles.value, func(p *acquisitionv1.QualityProfile) bool { return p.GetId() == id }); i >= 0 {
		saved = profiles.value[i]
		kind = saved.GetKind()
	}
	defs := a.qualityDefinitions(kind)
	if !isNew && profiles.loaded && profiles.err == nil && saved == nil {
		sectionHeader(c, style.T(c, "acquisition.profile"), "")
		secondaryText(c, style.T(c, "acquisition.noProfile"))
		return
	}
	title := saved.GetName()
	if title == "" {
		title = style.T(c, "acquisition.newProfile")
	}
	sectionHeader(c, title, mediaKindTitle(c, kind))
	for _, err := range []error{profiles.err, defs.err} {
		if err != nil {
			dangerText(c, errorMessage(c, err))
		}
	}
	if st.profile == nil {
		switch {
		case saved != nil:
			st.profile0 = saved
		case isNew && defs.loaded && defs.err == nil:
			items := make([]*acquisitionv1.QualityProfileItem, len(defs.value))
			for i, d := range defs.value {
				items[i] = acquisitionv1.QualityProfileItem_builder{Quality: new(d.GetQuality()), Allowed: new(false)}.Build()
			}
			st.profile0 = acquisitionv1.QualityProfile_builder{Kind: &kind, Items: items, UpgradeAllowed: new(true)}.Build()
		}
		if st.profile0 != nil {
			st.profile = proto.CloneOf(st.profile0)
		}
	}
	if st.profile == nil || !defs.loaded {
		return
	}
	a.profileForm(c, defs.value)
}

func (a *App) profileForm(c *ui.Context, defs []*acquisitionv1.QualityDefinition) {
	st := &a.settingsState.admin.acquisition
	colors := style.Colors(c)
	d := st.profile
	isNew := st.profile0.GetId() == ""
	titles := map[int32]string{}
	for _, def := range defs {
		titles[def.GetQuality()] = def.GetTitle()
	}
	nameOf := func(it *acquisitionv1.QualityProfileItem) string {
		if it.GetId() != 0 {
			return it.GetName()
		}
		if t, ok := titles[it.GetQuality()]; ok {
			return t
		}
		return strconv.Itoa(int(it.GetQuality()))
	}
	edited := func() { st.notice = "" }
	items := d.GetItems()
	var allowed []*acquisitionv1.QualityProfileItem
	for _, it := range items {
		if it.GetAllowed() {
			allowed = append(allowed, it)
		}
	}
	// setItems changes the items, moving the cutoff to the best allowed
	// when it is no longer allowed.
	setItems := func(items []*acquisitionv1.QualityProfileItem) {
		cutoffAllowed := slices.ContainsFunc(items, func(it *acquisitionv1.QualityProfileItem) bool {
			return it.GetAllowed() && itemID(it) == d.GetCutoff()
		})
		d.SetItems(items)
		if !cutoffAllowed {
			for _, it := range slices.Backward(items) {
				if it.GetAllowed() {
					d.SetCutoff(itemID(it))
					break
				}
			}
		}
		edited()
	}
	group(c, "profile", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "acquisition.profileName"), "", nil, func() {
			name := d.GetName()
			if ui.TextInput(c, &name).Width(288).Label(style.T(c, "acquisition.profileName")).Changed() {
				d.SetName(name)
				edited()
			}
		})
		upgrade := d.GetUpgradeAllowed()
		if g.switchRow(style.T(c, "acquisition.upgradeAllowed"), style.T(c, "acquisition.upgradeAllowedDetail"), &upgrade) {
			d.SetUpgradeAllowed(upgrade)
			edited()
		}
		if upgrade {
			g.row(style.T(c, "acquisition.cutoff"), style.T(c, "acquisition.cutoffDetail"), nil, func() {
				var values, labels []string
				if !slices.ContainsFunc(allowed, func(it *acquisitionv1.QualityProfileItem) bool { return itemID(it) == d.GetCutoff() }) {
					values, labels = append(values, strconv.Itoa(int(d.GetCutoff()))), append(labels, "—")
				}
				for _, it := range slices.Backward(allowed) {
					values, labels = append(values, strconv.Itoa(int(itemID(it)))), append(labels, nameOf(it))
				}
				cutoff := strconv.Itoa(int(d.GetCutoff()))
				chooseString(c, style.T(c, "acquisition.cutoff"), &cutoff, values, labels)
				if n, err := strconv.Atoi(cutoff); err == nil && int32(n) != d.GetCutoff() {
					d.SetCutoff(int32(n))
					edited()
				}
			})
		}
		if d.GetKind() == acquisitionv1.MediaKind_MEDIA_KIND_MOVIE {
			g.row(style.T(c, "acquisition.language"), style.T(c, "acquisition.languageDetail"), nil, func() {
				values := []string{"", "Original", "Unknown"}
				labels := []string{style.T(c, "acquisition.languageAny"), style.T(c, "acquisition.languageOriginal"), style.T(c, "acquisition.languageUnknown")}
				sorted := slices.Sorted(slices.Values(releaseLanguages))
				values, labels = append(values, sorted...), append(labels, sorted...)
				language := d.GetLanguage()
				if language == "Any" {
					language = ""
				}
				was := language
				chooseString(c, style.T(c, "acquisition.language"), &language, values, labels)
				if language != was {
					d.SetLanguage(language)
					edited()
				}
			})
		}
	})
	group(c, "qualities", style.T(c, "acquisition.profileQualities"), style.T(c, "acquisition.profileQualitiesFooter"), func(g *settingsGroup) {
		// Best first, as Radarr shows them; the profile keeps them lowest
		// first.
		for i := len(items) - 1; i >= 0; i-- {
			it := items[i]
			name := nameOf(it)
			g.divider()
			ui.Row(c.Key(itemID(it))).MinHeight(44).Padding(6, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				on := it.GetAllowed()
				if ui.Switch(c, &on).Label(name).Changed() {
					next := slices.Clone(items)
					n := proto.CloneOf(it)
					n.SetAllowed(on)
					for _, q := range n.GetItems() {
						q.SetAllowed(on)
					}
					next[i] = n
					setItems(next)
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					color := colors.TextPrimary
					if !on {
						color = colors.TextSecondary
					}
					style.Text(ui.Text(c, name), style.Type(c).Body).TextColor(style.Color(color))
					if it.GetId() > 0 {
						var names []string
						for _, q := range it.GetItems() {
							names = append(names, nameOf(q))
						}
						style.Text(ui.Text(c, strings.Join(names, ", ")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
					}
				})
				move := func(by int) {
					next := slices.Clone(items)
					next[i], next[i+by] = next[i+by], next[i]
					setItems(next)
				}
				if moveButton(c.Key("up"), true, i == len(items)-1) {
					move(1)
				}
				if moveButton(c.Key("down"), false, i == 0) {
					move(-1)
				}
			})
		}
	})
	formats := a.customFormats()
	group(c, "scores", style.T(c, "acquisition.formatScores"), style.T(c, "acquisition.formatScoresFooter"), func(g *settingsGroup) {
		const most = 100_000
		for _, f := range formats.value {
			fid := f.GetId()
			score := 0.0
			for _, s := range d.GetFormatScores() {
				if s.GetFormatId() == fid {
					score = float64(s.GetScore())
				}
			}
			g.row(f.GetName(), "", nil, func() {
				numberField(c, f.GetName(), score, -most, most, 1, func(v float64) {
					scores := slices.DeleteFunc(slices.Clone(d.GetFormatScores()), func(s *acquisitionv1.FormatScore) bool { return s.GetFormatId() == fid })
					if v != 0 {
						scores = append(scores, acquisitionv1.FormatScore_builder{FormatId: &fid, Score: new(int32(v))}.Build())
					}
					d.SetFormatScores(scores)
					edited()
				})
			})
		}
		switch {
		case formats.err != nil:
			g.note(errorMessage(c, formats.err), true)
		case formats.loaded && len(formats.value) == 0:
			g.note(style.T(c, "acquisition.noFormats"), false)
		}
		score := func(key string, value int32, lo float64, set func(int32)) {
			g.row(style.T(c, key), style.T(c, key+"Detail"), nil, func() {
				numberField(c, style.T(c, key), float64(value), lo, most, 1, func(v float64) {
					set(int32(v))
					edited()
				})
			})
		}
		score("acquisition.minFormatScore", d.GetMinFormatScore(), -most, d.SetMinFormatScore)
		if d.GetUpgradeAllowed() {
			score("acquisition.cutoffFormatScore", d.GetCutoffFormatScore(), -most, d.SetCutoffFormatScore)
			score("acquisition.minUpgradeFormatScore", d.GetMinUpgradeFormatScore(), 1, d.SetMinUpgradeFormatScore)
		}
	})
	changed := !proto.Equal(d, st.profile0)
	label := style.T(c, "action.save")
	if isNew {
		label = style.T(c, "acquisition.create")
	}
	a.acquisitionSave(c, label, changed && strings.TrimSpace(d.GetName()) != "" && len(allowed) > 0, changed, func() {
		p := proto.CloneOf(d)
		p.SetName(strings.TrimSpace(p.GetName()))
		var created string
		a.acquisitionAction("save", func(ctx context.Context, s string) error {
			if isNew {
				res, err := a.profileService(s).CreateQualityProfile(ctx, acquisitionv1.CreateQualityProfileRequest_builder{Profile: p}.Build())
				created = res.GetProfile().GetId()
				return err
			}
			_, err := a.profileService(s).UpdateQualityProfile(ctx, acquisitionv1.UpdateQualityProfileRequest_builder{Profile: p}.Build())
			return err
		}, func() {
			if isNew {
				a.replace(acquisitionPath("profile", created))
				return
			}
			st.profile0, st.notice = p, "acquisition.saved"
			st.profile = proto.CloneOf(p)
		})
	})
	if isNew {
		return
	}
	a.removeButton(c, "acquisition.removeProfile", "acquisition.removeProfileConfirm", "acquisition.removeProfileDetail", st.profile0.GetName(),
		func(ctx context.Context, s string) error {
			_, err := a.profileService(s).DeleteQualityProfile(ctx, acquisitionv1.DeleteQualityProfileRequest_builder{Id: new(st.profile0.GetId())}.Build())
			return err
		}, acquisitionPath("", ""))
}

// removeButton builds the button deleting what a page shows, which asks
// first, then goes back to back.
func (a *App) removeButton(c *ui.Context, label, confirm, detail, name string, remove func(ctx context.Context, s string) error, back string) {
	st := &a.settingsState.admin.acquisition
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		if dangerButton(c, style.T(c, label), st.busy != "") {
			st.confirming = true
		}
		if st.busy == "delete" {
			secondaryText(c, "…")
		}
	})
	if ui.AlertDialog(c, &st.confirming, style.T(c, confirm, "name", name), style.T(c, detail),
		style.T(c, "action.cancel"), style.T(c, label)) == 1 {
		a.acquisitionAction("delete", remove, func() { a.replace(back) })
	}
}
