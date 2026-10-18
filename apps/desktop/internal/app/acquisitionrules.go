package app

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// specKinds are the kinds of a custom format's conditions, named.
var specKinds = []struct {
	kind  acquisitionv1.CustomFormatSpecKind
	title string
}{
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE, "formats.releaseTitle"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_EDITION, "formats.edition"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_LANGUAGE, "formats.language"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_INDEXER_FLAG, "formats.indexerFlag"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SOURCE, "formats.source"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RESOLUTION, "formats.resolution"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_QUALITY_MODIFIER, "formats.modifier"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SIZE, "formats.size"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_GROUP, "formats.releaseGroup"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_YEAR, "formats.year"},
	{acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TYPE, "formats.releaseType"},
}

// specChoices are the values of the conditions that take one of a list,
// by the names libs/acquire gives them, and their labels.
var specChoices = map[acquisitionv1.CustomFormatSpecKind][2][]string{
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_INDEXER_FLAG: {
		{"freeleech", "halfleech", "doubleupload", "internal", "scene", "exclusive", "nuked", "neutralleech"},
	},
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SOURCE: {
		{"cam", "telesync", "telecine", "workprint", "dvd", "tv", "webdl", "webrip", "bluray", "unknown"},
		{"CAM", "Telesync", "Telecine", "Workprint", "DVD", "TV", "WEB-DL", "WEBRip", "Blu-ray", "Unknown"},
	},
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RESOLUTION: {
		{"360", "480", "540", "576", "720", "1080", "2160"},
		{"360p", "480p", "540p", "576p", "720p", "1080p", "2160p"},
	},
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_QUALITY_MODIFIER: {
		{"none", "regional", "screener", "rawhd", "brdisk", "remux"},
		{"None", "Regional", "Screener", "Raw-HD", "BR-DISK", "Remux"},
	},
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TYPE: {
		{"single_episode", "multi_episode", "season_pack", "unknown"},
		{"Single episode", "Multi-episode", "Season pack", "Unknown"},
	},
	acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_LANGUAGE: {
		append([]string{"Original", "Unknown"}, releaseLanguages...),
	},
}

func patterned(k acquisitionv1.CustomFormatSpecKind) bool {
	return k == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE ||
		k == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_EDITION ||
		k == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_GROUP
}

func ranged(k acquisitionv1.CustomFormatSpecKind) bool {
	return k == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SIZE ||
		k == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_YEAR
}

func specKindTitle(c *ui.Context, k acquisitionv1.CustomFormatSpecKind) string {
	for _, s := range specKinds {
		if s.kind == k {
			return style.T(c, s.title)
		}
	}
	return ""
}

func (a *App) customFormatsPage(c *ui.Context) {
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "acquisition.formats"), style.T(c, "formats.message"))
	t := a.customFormats()
	group(c, "formats", "", "", func(g *settingsGroup) {
		list := slices.Clone(t.value)
		slices.SortFunc(list, func(x, y *acquisitionv1.CustomFormat) int {
			return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
		})
		for _, f := range list {
			var kinds []string
			for _, s := range f.GetSpecifications() {
				if k := specKindTitle(c, s.GetKind()); !slices.Contains(kinds, k) {
					kinds = append(kinds, k)
				}
			}
			if g.link(f.GetId(), "", ui.Color{}, f.GetName(), strings.Join(kinds, ", "), true) {
				a.router.Push(acquisitionPath("format", f.GetId()))
			}
		}
		if g.addRow(style.T(c, "formats.add")) {
			a.router.Push(acquisitionPath("format", "new"))
		}
		if t.err != nil {
			g.note(errorMessage(c, t.err), true)
		}
	})
}

// customFormatPage builds a custom format's page; id is "new" for one to
// add.
func (a *App) customFormatPage(c *ui.Context, id string) {
	st := &a.settingsState.admin.acquisition
	back := acquisitionPath("formats", "")
	a.backTo(c, style.T(c, "acquisition.formats"), back)
	t := a.customFormats()
	isNew := id == "new"
	var saved *acquisitionv1.CustomFormat
	if i := slices.IndexFunc(t.value, func(f *acquisitionv1.CustomFormat) bool { return f.GetId() == id }); i >= 0 {
		saved = t.value[i]
	}
	if !isNew && t.loaded && t.err == nil && saved == nil {
		sectionHeader(c, style.T(c, "acquisition.formats"), "")
		secondaryText(c, style.T(c, "formats.none"))
		return
	}
	title := saved.GetName()
	if title == "" {
		title = style.T(c, "formats.new")
	}
	sectionHeader(c, title, "")
	if t.err != nil {
		dangerText(c, errorMessage(c, t.err))
	}
	if st.format == nil {
		switch {
		case isNew:
			st.format0 = &acquisitionv1.CustomFormat{}
		case saved != nil:
			st.format0 = saved
		}
		if st.format0 != nil {
			st.format = proto.CloneOf(st.format0)
		}
	}
	d := st.format
	if d == nil {
		return
	}
	edited := func() { st.notice = "" }
	group(c, "format", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "formats.name"), "", nil, func() {
			name := d.GetName()
			if ui.TextInput(c, &name).Width(288).Label(style.T(c, "formats.name")).Changed() {
				d.SetName(name)
				edited()
			}
		})
		rename := d.GetIncludeWhenRenaming()
		if g.switchRow(style.T(c, "formats.rename"), style.T(c, "formats.renameDetail"), &rename) {
			d.SetIncludeWhenRenaming(rename)
			edited()
		}
	})
	add := func() {
		if linkButton(c, style.T(c, "formats.addCondition")).Clicked() {
			kind := acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE
			d.SetSpecifications(append(d.GetSpecifications(), acquisitionv1.CustomFormatSpec_builder{Kind: &kind}.Build()))
			edited()
		}
	}
	groupWith(c, "conditions", style.T(c, "formats.conditions"), add, style.T(c, "formats.conditionsFooter"), func(g *settingsGroup) {
		specs := d.GetSpecifications()
		for i, s := range specs {
			g.block(func() {
				if specForm(c, s) {
					d.SetSpecifications(slices.Delete(slices.Clone(specs), i, i+1))
				}
			})
		}
		if len(specs) == 0 {
			g.note(style.T(c, "formats.noConditions"), false)
		}
	})
	// The edits of conditions change them in place: the notice of saving
	// stays until something differs from what is saved.
	changed := !proto.Equal(d, st.format0)
	if changed {
		st.notice = ""
	}
	complete := !slices.ContainsFunc(d.GetSpecifications(), func(s *acquisitionv1.CustomFormatSpec) bool {
		if strings.TrimSpace(s.GetName()) == "" {
			return true
		}
		if ranged(s.GetKind()) {
			return s.GetMax() <= s.GetMin()
		}
		return strings.TrimSpace(s.GetValue()) == ""
	})
	label := style.T(c, "action.save")
	if isNew {
		label = style.T(c, "acquisition.create")
	}
	a.acquisitionSave(c, label, changed && strings.TrimSpace(d.GetName()) != "" && complete, changed, func() {
		f := proto.CloneOf(d)
		f.SetName(strings.TrimSpace(f.GetName()))
		a.acquisitionAction("save", func(ctx context.Context, s string) error {
			if isNew {
				_, err := a.profileService(s).CreateCustomFormat(ctx, acquisitionv1.CreateCustomFormatRequest_builder{Format: f}.Build())
				return err
			}
			_, err := a.profileService(s).UpdateCustomFormat(ctx, acquisitionv1.UpdateCustomFormatRequest_builder{Format: f}.Build())
			return err
		}, func() {
			if isNew {
				a.replace(back)
				return
			}
			st.format0, st.format, st.notice = f, proto.CloneOf(f), "acquisition.saved"
		})
	})
	if !isNew {
		a.removeButton(c, "formats.remove", "formats.removeConfirm", "formats.removeDetail", st.format0.GetName(),
			func(ctx context.Context, s string) error {
				_, err := a.profileService(s).DeleteCustomFormat(ctx, acquisitionv1.DeleteCustomFormatRequest_builder{Id: new(st.format0.GetId())}.Build())
				return err
			}, back)
	}
}

// specForm builds the form of a custom format's condition, changing it in
// place, and reports a click of its Remove.
func specForm(c *ui.Context, s *acquisitionv1.CustomFormatSpec) (removed bool) {
	colors := style.Colors(c)
	kind := s.GetKind()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		labels := make([]string, len(specKinds))
		i := 0
		for j, k := range specKinds {
			labels[j] = style.T(c, k.title)
			if k.kind == kind {
				i = j
			}
		}
		if choose(c, style.T(c, "formats.kind"), &i, labels) {
			s.SetKind(specKinds[i].kind)
			s.SetValue("")
			s.SetMin(0)
			s.SetMax(0)
			s.SetExceptLanguage(false)
		}
		name := s.GetName()
		if ui.TextInput(c, &name).Grow(1).MinWidth(160).Label(style.T(c, "formats.conditionName")).
			Placeholder(style.T(c, "formats.conditionName")).Changed() {
			s.SetName(name)
		}
		removed = dangerButton(c, style.T(c, "formats.removeCondition"), false)
	})
	ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
		if patterned(kind) {
			value := s.GetValue()
			if ui.TextInput(c.Key("pattern"), &value).Grow(1).MinWidth(240).Label(style.T(c, "formats.pattern")).
				Placeholder(style.T(c, "formats.pattern")).Changed() {
				s.SetValue(value)
			}
		}
		if list, ok := specChoices[kind]; ok {
			values, labels := list[0], list[1]
			if labels == nil {
				labels = values
			}
			if !slices.Contains(values, s.GetValue()) {
				label := s.GetValue()
				if label == "" {
					label = style.T(c, "formats.choose")
				}
				values, labels = append([]string{s.GetValue()}, values...), append([]string{label}, labels...)
			}
			value := s.GetValue()
			chooseString(c.Key("value"), style.T(c, "formats.value"), &value, values, labels)
			s.SetValue(value)
		}
		if ranged(kind) {
			step := 1.0
			if kind == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_SIZE {
				step = 0.1
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				numberField(c.Key("min"), style.T(c, "formats.min"), s.GetMin(), 0, 1_000_000, step, s.SetMin)
				style.Text(ui.Text(c, "–"), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
				numberField(c.Key("max"), style.T(c, "formats.max"), s.GetMax(), 0, 1_000_000, step, s.SetMax)
				if step < 1 {
					secondaryText(c, style.T(c, "formats.gigabytes"))
				}
			})
		}
		check := func(label string, on bool, set func(bool)) {
			if ui.Checkbox(c.Key(label), &on, label).Changed() {
				set(on)
			}
		}
		if kind == acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_LANGUAGE {
			check(style.T(c, "formats.exceptLanguage"), s.GetExceptLanguage(), s.SetExceptLanguage)
		}
		check(style.T(c, "formats.negate"), s.GetNegate(), s.SetNegate)
		check(style.T(c, "formats.required"), s.GetRequired(), s.SetRequired)
	})
	return removed
}

// splitList returns the entries of a list typed with sep between them.
func splitList(s, sep string) []string {
	var list []string
	for x := range strings.SplitSeq(s, sep) {
		if x = strings.TrimSpace(x); x != "" {
			list = append(list, x)
		}
	}
	return list
}

// editEntry opens the form of a delay or release profile, by its ID,
// "new" for a new one.
func (a *App) editEntry(id string) {
	st := &a.settingsState.admin.acquisition
	st.editing, st.err, st.removing = &id, nil, false
}

// entryRow adds a row of a list that opens into its form, its trailing
// controls after it; it reports a click.
func entryRow(g *settingsGroup, key, label, detail string, disabled bool, trailing func()) bool {
	c := g.c
	colors := style.Colors(c)
	g.divider()
	clicked := false
	ui.Row(c.Key(key)).MinHeight(52).Padding(0, 8, 0, 0).Gap(4).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).Grow(1).MinWidth(0).Padding(10, 16).AlignItems(ui.Center).Justify(ui.Start).Label(label).Disabled(disabled)
		if b.Hovered() && !disabled {
			b.Background(style.Color(colors.Fill))
		}
		b.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1)
				if detail != "" {
					style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				}
			})
		})
		clicked = b.Clicked() && !disabled
		if trailing != nil {
			trailing()
		}
	})
	return clicked
}

// entryButtons adds the buttons under a form of an entry: Save, Cancel,
// and Remove, which asks again, when remove is set.
func (a *App) entryButtons(g *settingsGroup, valid bool, save, remove func()) {
	c := g.c
	st := &a.settingsState.admin.acquisition
	g.block(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			label := style.T(c, "action.save")
			if st.busy != "" {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(!valid || st.busy != "").Clicked() {
				save()
			}
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				st.editing, st.adding = nil, false
			}
			if remove != nil {
				if st.removing {
					if dangerButton(c, style.T(c, "rules.removeConfirm"), st.busy != "") {
						remove()
					}
				} else if dangerButton(c, style.T(c, "rules.remove"), false) {
					st.removing = true
				}
			}
			if st.err != nil {
				dangerText(c, errorMessage(c, st.err))
			}
		})
	})
}

// delayDraft is a delay profile as edited, its delays in minutes and its
// tags as typed.
type delayDraft struct {
	usenet, torrent, bypassQuality, bypassScore bool
	torrentPreferred                            bool
	usenetDelay, torrentDelay, minScore         float64
	tags                                        string
}

func minutesOf(d *durationpb.Duration) float64 {
	return float64((d.AsDuration() + 30*time.Second) / time.Minute)
}

func delayDraftOf(p *acquisitionv1.DelayProfile) delayDraft {
	return delayDraft{
		usenet: p.GetEnableUsenet(), torrent: p.GetEnableTorrent(),
		torrentPreferred: p.GetPreferredProtocol() == acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT,
		usenetDelay:      minutesOf(p.GetUsenetDelay()), torrentDelay: minutesOf(p.GetTorrentDelay()),
		bypassQuality: p.GetBypassIfHighestQuality(), bypassScore: p.GetBypassIfAboveCustomFormatScore(),
		minScore: float64(p.GetMinimumCustomFormatScore()), tags: strings.Join(p.GetTags(), ", "),
	}
}

// profile returns the delay profile p as edited.
func (d delayDraft) profile(p *acquisitionv1.DelayProfile) *acquisitionv1.DelayProfile {
	n := proto.CloneOf(p)
	preferred := acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET
	if d.torrentPreferred && d.usenet || !d.usenet {
		preferred = acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT
	}
	n.SetEnableUsenet(d.usenet)
	n.SetEnableTorrent(d.torrent)
	n.SetPreferredProtocol(preferred)
	n.SetUsenetDelay(durationpb.New(time.Duration(d.usenetDelay) * time.Minute))
	n.SetTorrentDelay(durationpb.New(time.Duration(d.torrentDelay) * time.Minute))
	n.SetBypassIfHighestQuality(d.bypassQuality)
	n.SetBypassIfAboveCustomFormatScore(d.bypassScore)
	n.SetMinimumCustomFormatScore(int32(d.minScore))
	n.SetTags(nil)
	if !p.GetDefault() {
		n.SetTags(splitList(d.tags, ","))
	}
	return n
}

func (a *App) delayProfilesPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "acquisition.delays"), style.T(c, "rules.delaysMessage"))
	t := adminTask(a, &st.delays, "delay-profiles", func(ctx context.Context, s string) ([]*acquisitionv1.DelayProfile, error) {
		res, err := a.profileService(s).ListDelayProfiles(ctx, &acquisitionv1.ListDelayProfilesRequest{})
		return res.GetProfiles(), err
	})
	list := t.value
	var ordered []*acquisitionv1.DelayProfile
	for _, p := range list {
		if !p.GetDefault() {
			ordered = append(ordered, p)
		}
	}
	move := func(i, by int) {
		ids := make([]string, len(ordered))
		for j, p := range ordered {
			ids[j] = p.GetId()
		}
		ids[i], ids[i+by] = ids[i+by], ids[i]
		a.acquisitionAction("reorder", func(ctx context.Context, s string) error {
			_, err := a.profileService(s).ReorderDelayProfiles(ctx, acquisitionv1.ReorderDelayProfilesRequest_builder{Ids: ids}.Build())
			return err
		}, nil)
	}
	summary := func(p *acquisitionv1.DelayProfile) string {
		var protocols []string
		if p.GetEnableUsenet() {
			protocols = append(protocols, style.T(c, "acquisition.usenet"))
		}
		if p.GetEnableTorrent() {
			protocols = append(protocols, style.T(c, "acquisition.torrent"))
		}
		preferred := style.T(c, "acquisition.usenet")
		if p.GetPreferredProtocol() == acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT {
			preferred = style.T(c, "acquisition.torrent")
		}
		parts := []string{strings.Join(protocols, " + "), style.T(c, "rules.prefers", "protocol", preferred)}
		if p.GetEnableUsenet() {
			parts = append(parts, style.T(c, "rules.usenetDelayOf", "minutes", int(minutesOf(p.GetUsenetDelay()))))
		}
		if p.GetEnableTorrent() {
			parts = append(parts, style.T(c, "rules.torrentDelayOf", "minutes", int(minutesOf(p.GetTorrentDelay()))))
		}
		return joinDetail(parts...)
	}
	add := func() {
		if st.editing == nil && linkButton(c, style.T(c, "rules.add")).Clicked() {
			a.editEntry("new")
			st.delay = delayDraft{usenet: true, torrent: true}
		}
	}
	groupWith(c, "delays", style.T(c, "rules.delaysInOrder"), add, style.T(c, "rules.delaysFooter"), func(g *settingsGroup) {
		for _, p := range list {
			if st.editing != nil && *st.editing == p.GetId() {
				a.delayForm(g, p)
				continue
			}
			label := strings.Join(p.GetTags(), ", ")
			if p.GetDefault() {
				label = style.T(c, "rules.defaultDelay")
			}
			i := slices.Index(ordered, p)
			trailing := func() {
				if p.GetDefault() {
					return
				}
				if moveButton(c.Key("up"), true, i == 0 || st.busy != "") {
					move(i, -1)
				}
				if moveButton(c.Key("down"), false, i == len(ordered)-1 || st.busy != "") {
					move(i, 1)
				}
			}
			if entryRow(g, p.GetId(), label, summary(p), st.editing != nil, trailing) {
				a.editEntry(p.GetId())
				st.delay = delayDraftOf(p)
			}
		}
		if st.editing != nil && *st.editing == "new" {
			a.delayForm(g, acquisitionv1.DelayProfile_builder{}.Build())
		}
		if t.err != nil {
			g.note(errorMessage(c, t.err), true)
		}
		if st.err != nil && st.editing == nil {
			g.note(errorMessage(c, st.err), true)
		}
	})
}

func (a *App) delayForm(g *settingsGroup, p *acquisitionv1.DelayProfile) {
	c := g.c
	st := &a.settingsState.admin.acquisition
	d := &st.delay
	isNew := p.GetId() == ""
	if !p.GetDefault() {
		g.row(style.T(c, "rules.tags"), style.T(c, "rules.delayTagsDetail"), nil, func() {
			ui.TextInput(c, &d.tags).Width(288).Label(style.T(c, "rules.tags"))
		})
	}
	g.switchRow(style.T(c, "acquisition.usenet"), "", &d.usenet)
	g.switchRow(style.T(c, "acquisition.torrent"), "", &d.torrent)
	if d.usenet && d.torrent {
		g.row(style.T(c, "rules.preferred"), "", nil, func() {
			i := 0
			if d.torrentPreferred {
				i = 1
			}
			if choose(c, style.T(c, "rules.preferred"), &i, []string{style.T(c, "acquisition.usenet"), style.T(c, "acquisition.torrent")}) {
				d.torrentPreferred = i == 1
			}
		})
	}
	minutes := func(label string, value *float64) {
		g.row(label, style.T(c, "rules.minutes"), nil, func() {
			ui.NumberInput(c, value, 0, 525600, 1).Width(128).Label(label)
		})
	}
	if d.usenet {
		minutes(style.T(c, "rules.usenetDelay"), &d.usenetDelay)
	}
	if d.torrent {
		minutes(style.T(c, "rules.torrentDelay"), &d.torrentDelay)
	}
	g.switchRow(style.T(c, "rules.bypassQuality"), style.T(c, "rules.bypassQualityDetail"), &d.bypassQuality)
	g.switchRow(style.T(c, "rules.bypassScore"), "", &d.bypassScore)
	if d.bypassScore {
		g.row(style.T(c, "rules.minScore"), "", nil, func() {
			ui.NumberInput(c, &d.minScore, -100000, 100000, 1).Width(128).Label(style.T(c, "rules.minScore"))
		})
	}
	valid := (d.usenet || d.torrent) && (p.GetDefault() || len(splitList(d.tags, ",")) > 0)
	done := func() { st.editing = nil }
	save := func() {
		n := d.profile(p)
		a.acquisitionAction("save", func(ctx context.Context, s string) error {
			if isNew {
				_, err := a.profileService(s).CreateDelayProfile(ctx, acquisitionv1.CreateDelayProfileRequest_builder{Profile: n}.Build())
				return err
			}
			_, err := a.profileService(s).UpdateDelayProfile(ctx, acquisitionv1.UpdateDelayProfileRequest_builder{Profile: n}.Build())
			return err
		}, done)
	}
	var remove func()
	if !isNew && !p.GetDefault() {
		remove = func() {
			a.acquisitionAction("delete", func(ctx context.Context, s string) error {
				_, err := a.profileService(s).DeleteDelayProfile(ctx, acquisitionv1.DeleteDelayProfileRequest_builder{Id: new(p.GetId())}.Build())
				return err
			}, done)
		}
	}
	a.entryButtons(g, valid, save, remove)
}

// releaseDraft is a release profile as edited: its terms one a line, its
// indexer as plugin/indexer, and its tags as typed.
type releaseDraft struct {
	name, required, ignored, indexer, tags string
	enabled                                bool
}

func releaseDraftOf(p *acquisitionv1.ReleaseProfile) releaseDraft {
	d := releaseDraft{
		name: p.GetName(), enabled: p.GetEnabled(), tags: strings.Join(p.GetTags(), ", "),
		required: strings.Join(p.GetRequired(), "\n"), ignored: strings.Join(p.GetIgnored(), "\n"),
	}
	if i := p.GetIndexer(); i != nil {
		d.indexer = i.GetPluginId() + "/" + i.GetIndexerId()
	}
	return d
}

func (a *App) releaseProfilesPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "acquisition.releases"), style.T(c, "rules.releasesMessage"))
	t := adminTask(a, &st.releases, "release-profiles", func(ctx context.Context, s string) ([]*acquisitionv1.ReleaseProfile, error) {
		res, err := a.profileService(s).ListReleaseProfiles(ctx, &acquisitionv1.ListReleaseProfilesRequest{})
		return res.GetProfiles(), err
	})
	indexers := a.acquisitionIndexers().value.GetIndexers()
	indexerName := func(p *acquisitionv1.ReleaseProfile) string {
		ref := p.GetIndexer()
		if ref == nil {
			return ""
		}
		for _, x := range indexers {
			if x.GetPluginId() == ref.GetPluginId() && x.GetId() == ref.GetIndexerId() {
				return x.GetName()
			}
		}
		return ref.GetIndexerId()
	}
	add := func() {
		if st.editing == nil && linkButton(c, style.T(c, "rules.add")).Clicked() {
			a.editEntry("new")
			st.release = releaseDraft{enabled: true}
		}
	}
	groupWith(c, "releases", style.T(c, "acquisition.releases"), add, style.T(c, "rules.releasesFooter"), func(g *settingsGroup) {
		for _, p := range t.value {
			if st.editing != nil && *st.editing == p.GetId() {
				a.releaseForm(g, p.GetId(), indexers)
				continue
			}
			var parts []string
			if !p.GetEnabled() {
				parts = append(parts, style.T(c, "acquisition.off"))
			}
			if n := len(p.GetRequired()); n > 0 {
				parts = append(parts, style.T(c, "rules.requiredCount", "count", n))
			}
			if n := len(p.GetIgnored()); n > 0 {
				parts = append(parts, style.T(c, "rules.ignoredCount", "count", n))
			}
			parts = append(parts, indexerName(p), strings.Join(p.GetTags(), ", "))
			label := cmpOr(p.GetName(), style.T(c, "rules.unnamed"))
			if entryRow(g, p.GetId(), label, joinDetail(parts...), st.editing != nil, nil) {
				a.editEntry(p.GetId())
				st.release = releaseDraftOf(p)
			}
		}
		if st.editing != nil && *st.editing == "new" {
			a.releaseForm(g, "", indexers)
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(t.value) == 0 && st.editing == nil:
			g.note(style.T(c, "rules.noReleases"), false)
		}
	})
}

func (a *App) releaseForm(g *settingsGroup, id string, indexers []*acquisitionv1.Indexer) {
	c := g.c
	st := &a.settingsState.admin.acquisition
	d := &st.release
	g.row(style.T(c, "rules.name"), "", nil, func() {
		ui.TextInput(c, &d.name).Width(288).Label(style.T(c, "rules.name"))
	})
	g.switchRow(style.T(c, "rules.enabled"), "", &d.enabled)
	terms := func(value *string, label, detail string) {
		g.block(func() {
			colors := style.Colors(c)
			ui.Column(c).Gap(6).Children(func() {
				style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
				style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				ui.TextArea(c, value).FillWidth().Height(76).Label(label)
			})
		})
	}
	terms(&d.required, style.T(c, "rules.required"), style.T(c, "rules.requiredDetail"))
	terms(&d.ignored, style.T(c, "rules.ignored"), style.T(c, "rules.ignoredDetail"))
	g.row(style.T(c, "rules.indexer"), "", nil, func() {
		values, labels := []string{""}, []string{style.T(c, "rules.allIndexers")}
		for _, x := range indexers {
			values, labels = append(values, x.GetPluginId()+"/"+x.GetId()), append(labels, x.GetName())
		}
		if d.indexer != "" && !slices.Contains(values, d.indexer) {
			_, rest, _ := strings.Cut(d.indexer, "/")
			values, labels = append(values, d.indexer), append(labels, rest)
		}
		chooseString(c, style.T(c, "rules.indexer"), &d.indexer, values, labels)
	})
	g.row(style.T(c, "rules.tags"), style.T(c, "rules.releaseTagsDetail"), nil, func() {
		ui.TextInput(c, &d.tags).Width(288).Label(style.T(c, "rules.tags"))
	})
	required, ignored := splitList(d.required, "\n"), splitList(d.ignored, "\n")
	done := func() { st.editing = nil }
	save := func() {
		b := acquisitionv1.ReleaseProfile_builder{
			Id: &id, Name: new(strings.TrimSpace(d.name)), Enabled: &d.enabled,
			Required: required, Ignored: ignored, Tags: splitList(d.tags, ","),
		}
		if plugin, indexer, ok := strings.Cut(d.indexer, "/"); ok {
			b.Indexer = acquisitionv1.IndexerRef_builder{PluginId: &plugin, IndexerId: &indexer}.Build()
		}
		p := b.Build()
		a.acquisitionAction("save", func(ctx context.Context, s string) error {
			if id == "" {
				_, err := a.profileService(s).CreateReleaseProfile(ctx, acquisitionv1.CreateReleaseProfileRequest_builder{Profile: p}.Build())
				return err
			}
			_, err := a.profileService(s).UpdateReleaseProfile(ctx, acquisitionv1.UpdateReleaseProfileRequest_builder{Profile: p}.Build())
			return err
		}, done)
	}
	var remove func()
	if id != "" {
		remove = func() {
			a.acquisitionAction("delete", func(ctx context.Context, s string) error {
				_, err := a.profileService(s).DeleteReleaseProfile(ctx, acquisitionv1.DeleteReleaseProfileRequest_builder{Id: &id}.Build())
				return err
			}, done)
		}
	}
	a.entryButtons(g, len(required)+len(ignored) > 0, save, remove)
}

// mappingDraft is a remote path mapping being added.
type mappingDraft struct{ host, remote, local string }

// localPath is the pattern of an absolute path of the server's.
var localPath = regexp.MustCompile(`^(/|[A-Za-z]:[\\/])`)

func (a *App) remotePathMappingsPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "acquisition.mappings"), style.T(c, "rules.mappingsMessage"))
	t := adminTask(a, &st.mappings, "remote-path-mappings", func(ctx context.Context, s string) ([]*acquisitionv1.RemotePathMapping, error) {
		res, err := a.profileService(s).ListRemotePathMappings(ctx, &acquisitionv1.ListRemotePathMappingsRequest{})
		return res.GetMappings(), err
	})
	add := func() {
		if !st.adding && linkButton(c, style.T(c, "rules.add")).Clicked() {
			st.adding, st.err = true, nil
		}
	}
	groupWith(c, "mappings", style.T(c, "acquisition.mappings"), add, "", func(g *settingsGroup) {
		for _, m := range t.value {
			id := m.GetId()
			g.row(m.GetHost(), m.GetRemotePath()+" → "+m.GetLocalPath(), nil, func() {
				if dangerButton(c, style.T(c, "rules.remove"), st.busy == "remove-"+id) {
					a.acquisitionAction("remove-"+id, func(ctx context.Context, s string) error {
						_, err := a.profileService(s).DeleteRemotePathMapping(ctx, acquisitionv1.DeleteRemotePathMappingRequest_builder{Id: &id}.Build())
						return err
					}, nil)
				}
			})
		}
		if st.adding {
			d := &st.mapping
			text := func(value *string, label, detail, placeholder string) {
				g.row(label, detail, nil, func() {
					ui.TextInput(c, value).Width(288).Label(label).Placeholder(placeholder)
				})
			}
			text(&d.host, style.T(c, "rules.host"), style.T(c, "rules.hostDetail"), "192.168.1.10")
			text(&d.remote, style.T(c, "rules.remotePath"), style.T(c, "rules.remotePathDetail"), "/downloads/")
			text(&d.local, style.T(c, "rules.localPath"), style.T(c, "rules.localPathDetail"), "/mnt/downloads/")
			host, remote, local := strings.TrimSpace(d.host), strings.TrimSpace(d.remote), strings.TrimSpace(d.local)
			a.entryButtons(g, host != "" && remote != "" && localPath.MatchString(local), func() {
				m := acquisitionv1.RemotePathMapping_builder{Host: &host, RemotePath: &remote, LocalPath: &local}.Build()
				a.acquisitionAction("save", func(ctx context.Context, s string) error {
					_, err := a.profileService(s).CreateRemotePathMapping(ctx, acquisitionv1.CreateRemotePathMappingRequest_builder{Mapping: m}.Build())
					return err
				}, func() { st.adding, st.mapping = false, mappingDraft{} })
			}, nil)
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(t.value) == 0 && !st.adding:
			g.note(style.T(c, "rules.noMappings"), false)
		}
		if st.err != nil && !st.adding {
			g.note(errorMessage(c, st.err), true)
		}
	})
}
