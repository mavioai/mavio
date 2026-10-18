package app

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// subtitlesState is the state of the settings' Subtitles: the providers,
// translators, profiles and their libraries, the profile edited, and the
// settings edited on one of their pages.
type subtitlesState struct {
	page        string // the page and ID shown, whose change resets the drafts
	providers   task[*acquisitionv1.ListSubtitleProvidersResponse]
	translators task[*acquisitionv1.ListSubtitleTranslatorsResponse]
	profiles    task[[]*acquisitionv1.SubtitleProfile]
	assignments task[map[string]string]
	plugins     task[[]*systemv1.Plugin]
	settings    task[*acquisitionv1.SubtitleSettings]
	models      task[[]*pluginv1.AiModel]

	profile, profile0           *acquisitionv1.SubtitleProfile
	mustContain, mustNotContain string
	draft, draft0               *acquisitionv1.SubtitleSettings
	words                       map[string]string // the text of the rows of words, by label

	busy       string
	notice     string
	err        error
	confirming bool
}

// extraSubtitleLanguages are the languages a subtitle can have beyond the
// server's, which name only ISO 639-1 codes.
var extraSubtitleLanguages = []string{"pt-BR", "zh-Hans", "zh-Hant"}

// subtitleCutoffAny is the cutoff that ends the search once any language
// has subtitles.
const subtitleCutoffAny = 65535

func (a *App) subtitleService(id string) acquisitionv1connect.SubtitleServiceClient {
	return acquisitionv1connect.NewSubtitleServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) aiProviderService(id string) systemv1connect.AiProviderServiceClient {
	return systemv1connect.NewAiProviderServiceClient(a.connectHTTP(), serverURL(id))
}

// subtitlesPath returns the path of a page of the settings' Subtitles: the
// list when page is empty.
func subtitlesPath(page, id string) string {
	q := url.Values{}
	if page != "" {
		q.Set("subtitles", page)
	}
	if id != "" {
		q.Set("id", id)
	}
	if len(q) == 0 {
		return "/settings/subtitles"
	}
	return "/settings/subtitles?" + q.Encode()
}

func (a *App) subtitlesSection(c *ui.Context, r *ui.Route) {
	st := &a.settingsState.admin.subtitles
	page, id := r.Query("subtitles"), r.Query("id")
	if k := page + "/" + id; st.page != k {
		st.page, st.profile, st.profile0, st.draft, st.draft0, st.words = k, nil, nil, nil, nil, nil
		st.notice, st.err, st.confirming = "", nil, false
	}
	switch page {
	case "profile":
		a.subtitleProfilePage(c, id)
	case "settings", "sync", "translation":
		a.subtitleSettingsPage(c, page)
	default:
		a.subtitlesList(c)
	}
}

// subtitleAction runs a change of the subtitles in the background, what
// showing busy until it ends; done runs on the main thread if it
// succeeded.
func (a *App) subtitleAction(what string, fn func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error, done func()) {
	st := &a.settingsState.admin.subtitles
	st.busy, st.err, st.notice = what, nil, ""
	s := a.subtitleService(a.chosenServer().GetId())
	go func() {
		err := fn(a.ctx, s)
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

func (a *App) subtitleProfiles() *task[[]*acquisitionv1.SubtitleProfile] {
	st := &a.settingsState.admin.subtitles
	t := adminTask(a, &st.profiles, "subtitle-profiles", func(ctx context.Context, s string) ([]*acquisitionv1.SubtitleProfile, error) {
		res, err := a.subtitleService(s).ListSubtitleProfiles(ctx, &acquisitionv1.ListSubtitleProfilesRequest{})
		return res.GetProfiles(), err
	})
	return t
}

func (a *App) subtitleProviders() *task[*acquisitionv1.ListSubtitleProvidersResponse] {
	st := &a.settingsState.admin.subtitles
	return adminTask(a, &st.providers, "subtitle-providers", func(ctx context.Context, s string) (*acquisitionv1.ListSubtitleProvidersResponse, error) {
		return a.subtitleService(s).ListSubtitleProviders(ctx, &acquisitionv1.ListSubtitleProvidersRequest{})
	})
}

func (a *App) subtitleTranslators() *task[*acquisitionv1.ListSubtitleTranslatorsResponse] {
	st := &a.settingsState.admin.subtitles
	return adminTask(a, &st.translators, "subtitle-translators", func(ctx context.Context, s string) (*acquisitionv1.ListSubtitleTranslatorsResponse, error) {
		return a.subtitleService(s).ListSubtitleTranslators(ctx, &acquisitionv1.ListSubtitleTranslatorsRequest{})
	})
}

// subtitleLanguages returns the codes and names of the languages a
// subtitle can have, in the order of their names, with any of values the
// server's list lacks.
func (a *App) subtitleLanguages(c *ui.Context, values ...string) (codes, labels []string) {
	type named struct{ code, name string }
	var list []named
	add := func(code, name string) {
		if code != "" && !slices.ContainsFunc(list, func(n named) bool { return n.code == code }) {
			list = append(list, named{code, cmpOr(languageName(c, code), name, code)})
		}
	}
	for _, l := range a.languages(a.chosenServer().GetId()) {
		add(l.GetCode(), l.GetName())
	}
	for _, code := range extraSubtitleLanguages {
		add(code, code)
	}
	for _, code := range values {
		add(code, code)
	}
	slices.SortStableFunc(list, func(x, y named) int { return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name)) })
	for _, n := range list {
		codes, labels = append(codes, n.code), append(labels, n.name)
	}
	return codes, labels
}

// subtitleItemName names a profile's language, as "French (forced)".
func subtitleItemName(c *ui.Context, it *acquisitionv1.SubtitleProfileItem) string {
	var kinds []string
	if it.GetForced() {
		kinds = append(kinds, style.T(c, "subtitles.forced"))
	}
	switch it.GetHearingImpaired() {
	case acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_REQUIRED:
		kinds = append(kinds, style.T(c, "subtitles.hiShort"))
	case acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_EXCLUDED:
		kinds = append(kinds, style.T(c, "subtitles.noHiShort"))
	}
	name := cmpOr(languageName(c, it.GetLanguage()), it.GetLanguage())
	if len(kinds) > 0 {
		return name + " (" + strings.Join(kinds, ", ") + ")"
	}
	return name
}

func (a *App) subtitlesList(c *ui.Context) {
	st := &a.settingsState.admin.subtitles
	sectionHeader(c, style.T(c, "settings.subtitles"), style.T(c, "subtitles.message"))
	providers, translators, profiles := a.subtitleProviders(), a.subtitleTranslators(), a.subtitleProfiles()
	plugins := adminTask(a, &st.plugins, "plugins", func(ctx context.Context, s string) ([]*systemv1.Plugin, error) {
		res, err := a.pluginService(s).ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		return res.GetPlugins(), err
	})
	pluginName := func(id string) string {
		for _, p := range plugins.value {
			if m := p.GetManifest(); m.GetId() == id && m.GetName() != "" {
				return m.GetName()
			}
		}
		return id
	}
	gray := ui.RGB(0x8E, 0x8E, 0x93)
	group(c, "providers", style.T(c, "subtitles.providers"), style.T(c, "subtitles.providersFooter"), func(g *settingsGroup) {
		list := providers.value.GetProviders()
		for _, p := range list {
			color := ui.RGB(0x34, 0xC7, 0x59)
			var kinds []string
			if p.GetMovies() {
				kinds = append(kinds, style.T(c, "subtitles.movies"))
			}
			if p.GetEpisodes() {
				kinds = append(kinds, style.T(c, "subtitles.episodes"))
			}
			detail := []string{pluginName(p.GetPluginId()), strings.Join(kinds, " + ")}
			if !p.GetEnabled() {
				color = gray
				detail = append(detail, style.T(c, "acquisition.off"))
			}
			if until := p.GetThrottledUntil(); until != nil && until.AsTime().After(time.Now()) {
				if p.GetEnabled() {
					color = ui.RGB(0xFF, 0x95, 0x00)
				}
				detail = append(detail, style.T(c, "subtitles.throttled", "when", fromNow(c, until.AsTime()), "reason", p.GetThrottleReason()))
			}
			g.link(p.GetPluginId()+"/"+p.GetId(), "subtitles", color, cmpOr(p.GetDisplayName(), p.GetName()), joinDetail(detail...), false)
		}
		failed := providers.value.GetFailedPlugins()
		for _, id := range failed {
			g.note(style.T(c, "subtitles.pluginFailed", "plugin", pluginName(id)), true)
		}
		switch {
		case providers.err != nil:
			g.note(errorMessage(c, providers.err), true)
		case providers.loaded && len(list) == 0 && len(failed) == 0:
			g.note(style.T(c, "subtitles.noProviders"), false)
		}
	})
	group(c, "translators", style.T(c, "subtitles.translators"), style.T(c, "subtitles.translatorsFooter"), func(g *settingsGroup) {
		list := translators.value.GetTranslators()
		for _, x := range list {
			color := ui.RGB(0x00, 0x7A, 0xFF)
			detail := []string{pluginName(x.GetPluginId())}
			if !x.GetEnabled() {
				color = gray
				detail = append(detail, style.T(c, "acquisition.off"))
			}
			g.link(x.GetPluginId()+"/"+x.GetId(), "subtitles", color, cmpOr(x.GetDisplayName(), x.GetName()), joinDetail(detail...), false)
		}
		failed := translators.value.GetFailedPlugins()
		for _, id := range failed {
			g.note(style.T(c, "subtitles.pluginFailed", "plugin", pluginName(id)), true)
		}
		switch {
		case translators.err != nil:
			g.note(errorMessage(c, translators.err), true)
		case translators.loaded && len(list) == 0 && len(failed) == 0:
			g.note(style.T(c, "subtitles.noTranslators"), false)
		}
	})
	sorted := slices.Clone(profiles.value)
	slices.SortFunc(sorted, func(x, y *acquisitionv1.SubtitleProfile) int {
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	})
	group(c, "profiles", style.T(c, "subtitles.profiles"), style.T(c, "subtitles.profilesFooter"), func(g *settingsGroup) {
		for _, p := range sorted {
			var names []string
			for _, it := range p.GetItems() {
				names = append(names, subtitleItemName(c, it))
			}
			if g.link(p.GetId(), "", ui.Color{}, p.GetName(), strings.Join(names, ", "), true) {
				a.router.Push(subtitlesPath("profile", p.GetId()))
			}
		}
		if g.addRow(style.T(c, "subtitles.addProfile")) {
			a.router.Push(subtitlesPath("profile", "new"))
		}
		if profiles.err != nil {
			g.note(errorMessage(c, profiles.err), true)
		}
	})
	a.libraryProfiles(c, sorted)
	group(c, "settings", style.T(c, "subtitles.settings"), "", func(g *settingsGroup) {
		for _, p := range []struct {
			page, icon string
			color      ui.Color
			label      string
		}{
			{"settings", "search", ui.RGB(0x58, 0x56, 0xD6), "subtitles.searching"},
			{"sync", "pending", ui.RGB(0xFF, 0x95, 0x00), "subtitles.sync"},
			{"translation", "subtitles", ui.RGB(0x00, 0x7A, 0xFF), "subtitles.translation"},
		} {
			detail := strings.TrimPrefix(p.label, "subtitles.")
			if g.link(p.page, p.icon, p.color, style.T(c, p.label), style.T(c, "subtitles."+detail+"Detail"), true) {
				a.router.Push(subtitlesPath(p.page, ""))
			}
		}
	})
}

// libraryProfiles builds each library with the profile assigned to it.
func (a *App) libraryProfiles(c *ui.Context, profiles []*acquisitionv1.SubtitleProfile) {
	st := &a.settingsState.admin.subtitles
	libraries := a.adminLibraries()
	assignments := adminTask(a, &st.assignments, "subtitle-assignments", func(ctx context.Context, s string) (map[string]string, error) {
		res, err := a.subtitleService(s).ListSubtitleProfileAssignments(ctx, &acquisitionv1.ListSubtitleProfileAssignmentsRequest{})
		m := map[string]string{}
		for _, x := range res.GetAssignments() {
			m[x.GetTargetId()] = x.GetProfileId()
		}
		return m, err
	})
	values, labels := []string{""}, []string{style.T(c, "subtitles.noProfile")}
	for _, p := range profiles {
		values, labels = append(values, p.GetId()), append(labels, p.GetName())
	}
	group(c, "libraries", style.T(c, "subtitles.libraries"), style.T(c, "subtitles.librariesFooter"), func(g *settingsGroup) {
		for _, l := range libraries.value {
			id := l.GetId()
			g.row(l.GetName(), "", nil, func() {
				profile := assignments.value[id]
				was := profile
				chooseString(c.Key(id), l.GetName(), &profile, values, labels)
				if profile != was {
					a.subtitleAction("assign", func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error {
						_, err := s.AssignSubtitleProfile(ctx, acquisitionv1.AssignSubtitleProfileRequest_builder{TargetId: &id, ProfileId: &profile}.Build())
						return err
					}, nil)
				}
			})
		}
		switch {
		case libraries.loaded && libraries.err == nil && len(libraries.value) == 0:
			g.note(style.T(c, "subtitles.noLibraries"), false)
		case assignments.err != nil:
			g.note(errorMessage(c, assignments.err), true)
		case st.err != nil && st.page == "/":
			g.note(errorMessage(c, st.err), true)
		}
	})
}

// subtitleProfilePage builds a subtitle profile's page; id is "new" for
// one to add.
func (a *App) subtitleProfilePage(c *ui.Context, id string) {
	st := &a.settingsState.admin.subtitles
	a.backTo(c, style.T(c, "settings.subtitles"), subtitlesPath("", ""))
	profiles := a.subtitleProfiles()
	isNew := id == "new"
	if st.profile == nil {
		switch {
		case isNew:
			st.profile0 = acquisitionv1.SubtitleProfile_builder{Items: []*acquisitionv1.SubtitleProfileItem{
				acquisitionv1.SubtitleProfileItem_builder{Id: new(int32(1)), Language: new("en")}.Build(),
			}}.Build()
		default:
			if i := slices.IndexFunc(profiles.value, func(p *acquisitionv1.SubtitleProfile) bool { return p.GetId() == id }); i >= 0 {
				st.profile0 = profiles.value[i]
			}
		}
		if st.profile0 != nil {
			st.profile = proto.CloneOf(st.profile0)
			st.mustContain = strings.Join(st.profile0.GetMustContain(), "\n")
			st.mustNotContain = strings.Join(st.profile0.GetMustNotContain(), "\n")
		}
	}
	if st.profile == nil {
		sectionHeader(c, style.T(c, "subtitles.profile"), "")
		switch {
		case profiles.err != nil:
			dangerText(c, errorMessage(c, profiles.err))
		case profiles.loaded:
			secondaryText(c, style.T(c, "subtitles.profileGone"))
		}
		return
	}
	a.subtitleProfileForm(c)
}

// lines returns the lines of a text that are not blank, trimmed.
func lines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

var whitespace = regexp.MustCompile(`\s`)

func (a *App) subtitleProfileForm(c *ui.Context) {
	st := &a.settingsState.admin.subtitles
	d := st.profile
	isNew := st.profile0.GetId() == ""
	title := st.profile0.GetName()
	if title == "" {
		title = style.T(c, "subtitles.newProfile")
	}
	sectionHeader(c, title, "")
	edited := func() { st.notice = "" }
	var used []string
	for _, it := range d.GetItems() {
		used = append(used, it.GetLanguage())
	}
	codes, names := a.subtitleLanguages(c, used...)
	group(c, "name", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "rules.name"), "", nil, func() {
			name := d.GetName()
			if ui.TextInput(c, &name).Width(288).Label(style.T(c, "rules.name")).Changed() {
				d.SetName(name)
				edited()
			}
		})
	})
	items := d.GetItems()
	setItem := func(i int, f func(it *acquisitionv1.SubtitleProfileItem)) {
		next := slices.Clone(items)
		next[i] = proto.CloneOf(items[i])
		f(next[i])
		d.SetItems(next)
		edited()
	}
	add := func() {
		next := int32(0)
		for _, it := range items {
			next = max(next, it.GetId())
		}
		base, _, _ := strings.Cut(c.Locale(), "-")
		language := "en"
		for _, l := range append([]string{base, "en"}, codes...) {
			if l != "" && !slices.Contains(used, l) {
				language = l
				break
			}
		}
		d.SetItems(append(slices.Clone(items), acquisitionv1.SubtitleProfileItem_builder{Id: new(next + 1), Language: &language}.Build()))
		edited()
	}
	addButton := func() {
		if linkButton(c, style.T(c, "rules.add")).Clicked() {
			add()
		}
	}
	hiLabels := []string{style.T(c, "subtitles.hiAllowed"), style.T(c, "subtitles.hiRequired"), style.T(c, "subtitles.hiExcluded")}
	groupWith(c, "languages", style.T(c, "subtitles.languages"), addButton, style.T(c, "subtitles.languagesFooter"), func(g *settingsGroup) {
		for i, it := range items {
			g.divider()
			ui.Column(c.Key(it.GetId())).Padding(10, 16).Gap(8).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					language := it.GetLanguage()
					chooseString(c.Key("language"), style.T(c, "subtitles.language"), &language, codes, names)
					if language != it.GetLanguage() {
						setItem(i, func(it *acquisitionv1.SubtitleProfileItem) { it.SetLanguage(language) })
					}
					hi := int(it.GetHearingImpaired())
					if choose(c.Key("hi"), style.T(c, "subtitles.hearingImpaired"), &hi, hiLabels) {
						setItem(i, func(it *acquisitionv1.SubtitleProfileItem) {
							it.SetHearingImpaired(acquisitionv1.SubtitleHearingImpaired(hi))
						})
					}
					ui.Box(c).Grow(1)
					remove := iconButton(c, "close", style.T(c, "rules.remove")).Disabled(len(items) == 1)
					if len(items) == 1 {
						remove.Opacity(0.3)
					}
					if remove.Clicked() && len(items) > 1 {
						cutoff := d.GetCutoff()
						if cutoff == it.GetId() {
							cutoff = 0
						}
						d.SetItems(slices.Delete(slices.Clone(items), i, i+1))
						d.SetCutoff(cutoff)
						edited()
					}
				})
				ui.Row(c).Gap(16).Wrap().Children(func() {
					forced, exclude, only := it.GetForced(), it.GetAudioExclude(), it.GetAudioOnlyInclude()
					if ui.Checkbox(c, &forced, style.T(c, "subtitles.forcedOnly")).Changed() {
						setItem(i, func(it *acquisitionv1.SubtitleProfileItem) { it.SetForced(forced) })
					}
					if ui.Checkbox(c, &exclude, style.T(c, "subtitles.audioExclude")).Changed() {
						setItem(i, func(it *acquisitionv1.SubtitleProfileItem) {
							it.SetAudioExclude(exclude)
							if exclude {
								it.SetAudioOnlyInclude(false)
							}
						})
					}
					if ui.Checkbox(c, &only, style.T(c, "subtitles.audioOnlyInclude")).Changed() {
						setItem(i, func(it *acquisitionv1.SubtitleProfileItem) {
							it.SetAudioOnlyInclude(only)
							if only {
								it.SetAudioExclude(false)
							}
						})
					}
				})
			})
		}
	})
	group(c, "options", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "subtitles.cutoff"), style.T(c, "subtitles.cutoffDetail"), nil, func() {
			values := []string{"0", strconv.Itoa(subtitleCutoffAny)}
			labels := []string{style.T(c, "subtitles.cutoffNone"), style.T(c, "subtitles.cutoffAny")}
			for _, it := range items {
				values, labels = append(values, strconv.Itoa(int(it.GetId()))), append(labels, subtitleItemName(c, it))
			}
			cutoff := strconv.Itoa(int(d.GetCutoff()))
			chooseString(c, style.T(c, "subtitles.cutoff"), &cutoff, values, labels)
			if n, err := strconv.Atoi(cutoff); err == nil && int32(n) != d.GetCutoff() {
				d.SetCutoff(int32(n))
				edited()
			}
		})
		original := d.GetOriginalFormat()
		if g.switchRow(style.T(c, "subtitles.originalFormat"), style.T(c, "subtitles.originalFormatDetail"), &original) {
			d.SetOriginalFormat(original)
			edited()
		}
		g.row(style.T(c, "subtitles.tag"), style.T(c, "subtitles.tagDetail"), nil, func() {
			tag := d.GetTag()
			if ui.TextInput(c, &tag).Width(192).Label(style.T(c, "subtitles.tag")).Changed() {
				d.SetTag(tag)
				edited()
			}
		})
	})
	group(c, "names", style.T(c, "subtitles.releaseNames"), style.T(c, "subtitles.releaseNamesFooter"), func(g *settingsGroup) {
		for _, f := range []struct {
			label string
			value *string
		}{{"subtitles.mustContain", &st.mustContain}, {"subtitles.mustNotContain", &st.mustNotContain}} {
			g.divider()
			ui.Column(c.Key(f.label)).Padding(10, 16).Gap(6).Children(func() {
				label := style.T(c, f.label)
				style.Text(ui.Text(c, label), style.Type.Body).TextColor(style.Color(style.Colors(c).TextPrimary))
				if ui.TextArea(c, f.value).FillWidth().Height(56).Font("SF Mono, Menlo, monospace").Label(label).Changed() {
					edited()
				}
			})
		}
	})
	result := proto.CloneOf(d)
	result.SetName(strings.TrimSpace(d.GetName()))
	result.SetTag(strings.TrimSpace(d.GetTag()))
	result.SetMustContain(lines(st.mustContain))
	result.SetMustNotContain(lines(st.mustNotContain))
	changed := isNew || !proto.Equal(result, st.profile0)
	valid := result.GetName() != "" && len(result.GetItems()) > 0 && !whitespace.MatchString(result.GetTag())
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		label := style.T(c, "action.save")
		if st.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || !valid || st.busy != "").Clicked() {
			var created string
			a.subtitleAction("save", func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error {
				if isNew {
					res, err := s.CreateSubtitleProfile(ctx, acquisitionv1.CreateSubtitleProfileRequest_builder{Profile: result}.Build())
					created = res.GetProfile().GetId()
					return err
				}
				_, err := s.UpdateSubtitleProfile(ctx, acquisitionv1.UpdateSubtitleProfileRequest_builder{Profile: result}.Build())
				return err
			}, func() {
				if isNew {
					a.router.Replace(subtitlesPath("profile", created))
					return
				}
				st.profile0, st.profile = result, proto.CloneOf(result)
			})
		}
		if !isNew && dangerButton(c, style.T(c, "subtitles.remove"), st.busy != "") {
			st.confirming = true
		}
		if st.err != nil {
			dangerText(c, errorMessage(c, st.err))
		}
	})
	if ui.AlertDialog(c, &st.confirming, style.T(c, "subtitles.removeTitle", "name", st.profile0.GetName()), style.T(c, "subtitles.removeMessage"),
		style.T(c, "action.cancel"), style.T(c, "subtitles.remove")) == 1 {
		id := st.profile0.GetId()
		a.subtitleAction("delete", func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error {
			_, err := s.DeleteSubtitleProfile(ctx, acquisitionv1.DeleteSubtitleProfileRequest_builder{Id: &id}.Build())
			return err
		}, func() { a.router.Replace(subtitlesPath("", "")) })
	}
}

// aiModels returns the models of the AI providers that can be used, none
// without the AI gateway.
func (a *App) aiModels() *task[[]*pluginv1.AiModel] {
	st := &a.settingsState.admin.subtitles
	return adminTask(a, &st.models, "ai-models", func(ctx context.Context, s string) ([]*pluginv1.AiModel, error) {
		res, err := a.aiProviderService(s).ListAiModels(ctx, systemv1.ListAiModelsRequest_builder{Available: new(true)}.Build())
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return res.GetModels(), nil
	})
}
