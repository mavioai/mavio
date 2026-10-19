package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// subtitleFormats are the formats of a subtitle file uploaded.
var subtitleFormats = []string{"srt", "ass", "ssa", "vtt", "sub"}

// itemSubsState is the state of an item's subtitles on its details.
type itemSubsState struct {
	subs   task[*acquisitionv1.GetItemSubtitlesResponse]
	reload int
	// panels is the panel open under a subtitle, by its key: "sync",
	// "mods", "translate" or "delete".
	panels map[string]string
	busy   string           // what runs, by its action and subtitle's key
	errs   map[string]error // of what was done, by its action and subtitle's key
	// sync, mods and translate are a subtitle's panels' choices, by its key.
	references map[string]string
	synced     map[string]*acquisitionv1.SyncSubtitleResponse
	mods       map[string]map[string]bool
	into       map[string]string
	translator map[string]string
	queued     map[string]bool
	uploading  bool
	upload     subtitleUpload
	searching  bool
	language   string // the language searched in, empty for the profile's
	found      task[*acquisitionv1.SearchItemSubtitlesResponse]
	downloads  map[string]*grab
}

// subtitleUpload is a subtitle file chosen to upload.
type subtitleUpload struct {
	path, language string
	forced, hi     bool
}

// subtitleKey returns the key of an item's subtitle: its file, or its
// embedded stream's index.
func subtitleKey(s *acquisitionv1.ItemSubtitle) string {
	if s.GetPath() != "" {
		return s.GetPath()
	}
	return fmt.Sprint("stream-", s.GetStreamIndex())
}

// specName names a subtitle's language with its kind, as "English
// (Forced)".
func specName(c *ui.Context, s *acquisitionv1.SubtitleLanguageSpec) string {
	if s.GetLanguage() == "" {
		return style.T(c, "itemsubs.unknown")
	}
	name := cmpOr(languageName(c, s.GetLanguage()), s.GetLanguage())
	switch {
	case s.GetForced():
		return style.T(c, "itemsubs.forcedLanguage", "language", name)
	case s.GetHearingImpaired():
		return style.T(c, "itemsubs.hiLanguage", "language", name)
	}
	return name
}

// itemSubtitles builds a movie's or episode's subtitles on its details,
// as the web UI's, for administrators: its subtitle profile and the
// languages it lacks; the subtitles it has, each synchronized, modified,
// translated, extracted or deleted; the providers' subtitles searched and
// downloaded; and a subtitle file uploaded.
func (a *App) itemSubtitles(c *ui.Context, st *itemSubsState, server string, it *libraryv1.Item) {
	k := it.GetKind()
	if !a.isAdmin(server) || k != libraryv1.ItemKind_ITEM_KIND_MOVIE && k != libraryv1.ItemKind_ITEM_KIND_EPISODE {
		return
	}
	colors := style.Colors(c)
	id := it.GetId()
	t := load(a, &st.subs, key("item-subtitles", server, id, a.generation, st.reload), func(ctx context.Context) (*acquisitionv1.GetItemSubtitlesResponse, error) {
		return a.subtitleService(server).GetItemSubtitles(ctx, acquisitionv1.GetItemSubtitlesRequest_builder{ItemId: &id}.Build())
	})
	data := t.value
	ui.Column(c).Padding(4, style.Layout(c).Margin, 0).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "itemsubs.title")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		ui.Column(c).MaxWidth(768).Gap(24).Children(func() {
			if t.err != nil {
				footnote(c, errorMessage(c, t.err), true)
			}
			if data == nil {
				return
			}
			group(c, "profile", "", style.T(c, "itemsubs.profileFooter"), func(g *settingsGroup) {
				g.row(style.T(c, "itemsubs.profile"), "", nil, func() {
					secondaryText(c, cmpOr(data.GetProfile().GetName(), style.T(c, "itemsubs.noProfile")))
				})
				if missing := data.GetMissing(); len(missing) > 0 {
					names := make([]string, len(missing))
					for i, m := range missing {
						names[i] = specName(c, m)
					}
					g.row(style.T(c, "itemsubs.missing"), "", nil, func() {
						style.Text(ui.Text(c, strings.Join(names, ", ")), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
					})
				}
				g.divider()
				ui.Row(c.Key("actions")).Padding(12, 16).Gap(8).Wrap().Children(func() {
					if smallButton(c, style.T(c, "itemsubs.search"), false) {
						st.searching, st.downloads = true, nil
						st.language = ""
						if len(data.GetMissing()) == 0 {
							st.language = discoverLanguage(c)
						}
					}
					if smallButton(c, style.T(c, "itemsubs.upload"), false) {
						st.uploading = !st.uploading
					}
				})
				if st.uploading {
					g.divider()
					ui.Column(c.Key("upload")).Padding(12, 16).Gap(12).Children(func() { a.uploadSubtitle(c, st, server, id) })
				}
			})
			external := slices.DeleteFunc(slices.Clone(data.GetSubtitles()), func(s *acquisitionv1.ItemSubtitle) bool { return s.GetPath() == "" })
			group(c, "present", style.T(c, "itemsubs.present"), "", func(g *settingsGroup) {
				for _, s := range data.GetSubtitles() {
					g.divider()
					ui.Column(c.Key(subtitleKey(s))).Padding(12, 16).Gap(8).Children(func() { a.subtitleRow(c, st, server, id, s, external) })
				}
				if len(data.GetSubtitles()) == 0 {
					g.note(style.T(c, "itemsubs.none"), false)
				}
			})
		})
	})
	if data != nil {
		a.subtitleSearchDialog(c, st, server, it, data.GetMissing())
	}
}

// subtitleAct runs a change of an item's subtitles in the background,
// keyed what, which shows busy until it ends and keeps its error; done
// runs on the main thread if it succeeded.
func (a *App) subtitleAct(st *itemSubsState, server, what string, fn func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error, done func()) {
	if st.errs == nil {
		st.errs = map[string]error{}
	}
	st.busy = what
	delete(st.errs, what)
	s := a.subtitleService(server)
	go func() {
		err := fn(a.ctx, s)
		a.opts.Update(func() {
			st.busy = ""
			if err != nil {
				st.errs[what] = err
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}

// actionError builds the error of what was done, if it failed.
func (st *itemSubsState) actionError(c *ui.Context, what string) {
	if err := st.errs[what]; err != nil {
		footnote(c, errorMessage(c, err), true)
	}
}

// subtitleRow builds a subtitle the item has: its language, file or
// stream and codec, and its actions with their panels.
func (a *App) subtitleRow(c *ui.Context, st *itemSubsState, server, id string, s *acquisitionv1.ItemSubtitle, external []*acquisitionv1.ItemSubtitle) {
	colors := style.Colors(c)
	k := subtitleKey(s)
	if st.panels == nil {
		st.panels = map[string]string{}
	}
	toggle := func(p string) {
		if st.panels[k] == p {
			delete(st.panels, k)
		} else {
			st.panels[k] = p
		}
	}
	detail := []string{style.T(c, "itemsubs.embedded", "index", s.GetStreamIndex())}
	if s.GetPath() != "" {
		detail[0] = path.Base(s.GetPath())
	}
	detail = append(detail, strings.ToUpper(s.GetCodec()))
	if !s.GetCounted() {
		detail = append(detail, style.T(c, "itemsubs.notCounted"))
	}
	if st.panels[k] == "delete" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			style.Text(ui.Text(c, style.T(c, "itemsubs.deleteTitle", "name", path.Base(s.GetPath()))), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Grow(1)
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				delete(st.panels, k)
			}
			if dangerButton(c, style.T(c, "itemsubs.delete"), st.busy == "delete"+k) {
				p := s.GetPath()
				a.subtitleAct(st, server, "delete"+k, func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
					_, err := svc.DeleteSubtitle(ctx, acquisitionv1.DeleteSubtitleRequest_builder{ItemId: &id, Path: &p}.Build())
					return err
				}, func() {
					delete(st.panels, k)
					st.reload++
				})
			}
		})
		st.actionError(c, "delete"+k)
		return
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Column(c).Grow(1).Basis(224).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, specName(c, s.GetLanguage())), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
			footnote(c, joinDetail(detail...), false)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			plain := func(label string) bool { return linkButton(c.Key(label), label).Clicked() }
			if s.GetPath() != "" {
				if plain(style.T(c, "itemsubs.sync")) {
					toggle("sync")
				}
				if plain(style.T(c, "itemsubs.modify")) {
					toggle("mods")
				}
			} else {
				label := style.T(c, "itemsubs.extract")
				if st.busy == "extract"+k {
					label = "…"
				}
				if plain(label) && st.busy == "" {
					index, language := s.GetStreamIndex(), s.GetLanguage()
					a.subtitleAct(st, server, "extract"+k, func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
						_, err := svc.ExtractSubtitle(ctx, acquisitionv1.ExtractSubtitleRequest_builder{ItemId: &id, StreamIndex: &index, Language: language}.Build())
						return err
					}, func() { st.reload++ })
				}
			}
			if plain(style.T(c, "itemsubs.translate")) {
				toggle("translate")
			}
			if s.GetPath() != "" {
				b := ui.ButtonBase(c.Key("delete")).Height(28).Padding(0, 8).AlignItems(ui.Center).Label(style.T(c, "itemsubs.delete"))
				b.Children(func() {
					style.Text(ui.Text(c, style.T(c, "itemsubs.delete")), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
				})
				if b.Clicked() {
					st.panels[k] = "delete"
				}
			}
		})
	})
	st.actionError(c, "extract"+k)
	switch st.panels[k] {
	case "sync":
		a.syncPanel(c, st, server, id, s, external)
	case "mods":
		a.modsPanel(c, st, server, id, s)
	case "translate":
		a.translatePanel(c, st, server, id, s)
	}
}

// inset builds a row of controls under a subtitle, on the inset fill.
func inset(c *ui.Context, fn func()) {
	ui.Column(c).Padding(12).Gap(12).Radius(10).Background(style.Color(style.Colors(c).Fill)).Children(fn)
}

// syncPanel builds synchronizing a subtitle with the audio or another
// subtitle file.
func (a *App) syncPanel(c *ui.Context, st *itemSubsState, server, id string, s *acquisitionv1.ItemSubtitle, external []*acquisitionv1.ItemSubtitle) {
	k := subtitleKey(s)
	if st.references == nil {
		st.references = map[string]string{}
		st.synced = map[string]*acquisitionv1.SyncSubtitleResponse{}
	}
	paths := []string{""}
	labels := []string{style.T(c, "itemsubs.audio")}
	for _, o := range external {
		if o.GetPath() != s.GetPath() {
			paths = append(paths, o.GetPath())
			labels = append(labels, specName(c, o.GetLanguage())+" · "+path.Base(o.GetPath()))
		}
	}
	inset(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			i := max(0, slices.Index(paths, st.references[k]))
			if choose(c, style.T(c, "itemsubs.reference"), &i, labels) {
				st.references[k] = paths[i]
			}
			label := style.T(c, "itemsubs.sync")
			if st.busy == "sync"+k {
				label = style.T(c, "itemsubs.syncing")
			}
			// Named with the subtitle, as the row's Sync opening it shows the same.
			if ui.PrimaryButton(c, label).Label(label + ", " + specName(c, s.GetLanguage())).Disabled(st.busy != "").Clicked() {
				p, ref := s.GetPath(), st.references[k]
				var res *acquisitionv1.SyncSubtitleResponse
				a.subtitleAct(st, server, "sync"+k, func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
					req := acquisitionv1.SyncSubtitleRequest_builder{ItemId: &id, Path: &p}
					if ref != "" {
						req.ReferencePath = &ref
					}
					r, err := svc.SyncSubtitle(ctx, req.Build())
					res = r
					return err
				}, func() { st.synced[k] = res })
			}
		})
		if r := st.synced[k]; r != nil {
			footnote(c, style.T(c, "itemsubs.synced", "offset", r.GetOffsetMs(), "ratio", fmt.Sprintf("%.4f", r.GetRatio())), false)
		}
		st.actionError(c, "sync"+k)
	})
}

// modsPanel builds applying modifications to a subtitle file.
func (a *App) modsPanel(c *ui.Context, st *itemSubsState, server, id string, s *acquisitionv1.ItemSubtitle) {
	k := subtitleKey(s)
	if st.mods == nil {
		st.mods = map[string]map[string]bool{}
	}
	if st.mods[k] == nil {
		st.mods[k] = map[string]bool{}
	}
	chosen := st.mods[k]
	inset(c, func() {
		for _, m := range subtitleMods {
			on := chosen[m[0]]
			ui.Row(c.Key(m[0])).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, style.T(c, m[1])), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).TextPrimary)).Grow(1)
				if ui.Switch(c, &on).Label(style.T(c, m[1])).Changed() {
					chosen[m[0]] = on
				}
			})
		}
		var mods []string
		for _, m := range subtitleMods {
			if chosen[m[0]] {
				mods = append(mods, m[0])
			}
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			label := style.T(c, "itemsubs.apply")
			if st.busy == "mods"+k {
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(len(mods) == 0 || st.busy != "").Clicked() {
				p := s.GetPath()
				a.subtitleAct(st, server, "mods"+k, func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
					_, err := svc.ModifySubtitle(ctx, acquisitionv1.ModifySubtitleRequest_builder{ItemId: &id, Path: &p, Mods: mods}.Build())
					return err
				}, func() {
					delete(st.panels, k)
					delete(st.mods, k)
				})
			}
			st.actionError(c, "mods"+k)
		})
	})
}

// languageChoice builds a drop-down of the subtitle languages choosing
// *code, first naming the empty choice.
func (a *App) languageChoice(c *ui.Context, server, label, first string, code *string) bool {
	var codes, labels []string
	a.asItemServer(server, func() { codes, labels = a.subtitleLanguages(c, *code) })
	codes, labels = append([]string{""}, codes...), append([]string{first}, labels...)
	i := max(0, slices.Index(codes, *code))
	if choose(c, label, &i, labels) {
		*code = codes[i]
		return true
	}
	return false
}

// translatePanel builds translating a subtitle into another language, by
// the default translator, another or an AI model.
func (a *App) translatePanel(c *ui.Context, st *itemSubsState, server, id string, s *acquisitionv1.ItemSubtitle) {
	k := subtitleKey(s)
	if st.into == nil {
		st.into, st.translator, st.queued = map[string]string{}, map[string]string{}, map[string]bool{}
	}
	values := []string{""}
	labels := []string{style.T(c, "itemsubs.settingsTranslator")}
	a.asItemServer(server, func() {
		for _, x := range a.subtitleTranslators().value.GetTranslators() {
			values = append(values, x.GetId())
			labels = append(labels, cmpOr(x.GetDisplayName(), x.GetName()))
		}
		for _, m := range a.aiModels().value {
			values = append(values, "ai/"+m.GetProvider()+"/"+m.GetId())
			labels = append(labels, cmpOr(m.GetName(), m.GetId())+" ("+m.GetProvider()+")")
		}
	})
	inset(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			into := st.into[k]
			if a.languageChoice(c, server, style.T(c, "itemsubs.into"), style.T(c, "itemsubs.into"), &into) {
				st.into[k] = into
			}
			i := max(0, slices.Index(values, st.translator[k]))
			if choose(c, style.T(c, "subtitles.translator"), &i, labels) {
				st.translator[k] = values[i]
			}
			label := style.T(c, "itemsubs.translate")
			switch {
			case st.queued[k]:
				label = style.T(c, "itemsubs.translationQueued")
			case st.busy == "translate"+k:
				label = "…"
			}
			if ui.PrimaryButton(c, label).Disabled(into == "" || st.busy != "" || st.queued[k]).Clicked() {
				req := acquisitionv1.TranslateSubtitleRequest_builder{
					ItemId: &id, Language: acquisitionv1.SubtitleLanguageSpec_builder{Language: &into}.Build(),
				}
				if p := s.GetPath(); p != "" {
					req.Path = &p
				} else {
					req.StreamIndex = new(s.GetStreamIndex())
				}
				if tr := st.translator[k]; tr != "" {
					req.Translator = &tr
				}
				a.subtitleAct(st, server, "translate"+k, func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
					_, err := svc.TranslateSubtitle(ctx, req.Build())
					return err
				}, func() { st.queued[k] = true })
			}
		})
		if st.queued[k] {
			footnote(c, style.T(c, "itemsubs.translationFooter"), false)
		}
		st.actionError(c, "translate"+k)
	})
}

// uploadSubtitle builds uploading a subtitle file: the file chosen, its
// language and whether it is forced or for the hearing impaired.
func (a *App) uploadSubtitle(c *ui.Context, st *itemSubsState, server, id string) {
	u := &st.upload
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(u.path)), ".")
	known := slices.Contains(subtitleFormats, format)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		if smallButton(c, style.T(c, "itemsubs.chooseFile"), a.opts.ChooseFile == nil) {
			if p, err := a.opts.ChooseFile(style.T(c, "itemsubs.file"), subtitleFormats); err == nil && p != "" {
				u.path = p
			}
		}
		if u.path != "" {
			secondaryText(c, filepath.Base(u.path))
		}
	})
	if u.path != "" && !known {
		footnote(c, style.T(c, "itemsubs.badFormat"), true)
	}
	a.languageChoice(c, server, style.T(c, "itemsubs.language"), style.T(c, "itemsubs.language"), &u.language)
	for _, x := range []struct {
		label     string
		on, other *bool
	}{{style.T(c, "subtitles.forced"), &u.forced, &u.hi}, {style.T(c, "subtitles.hearingImpaired"), &u.hi, &u.forced}} {
		ui.Row(c.Key(x.label)).Gap(12).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, x.label), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).TextPrimary)).Grow(1)
			if ui.Switch(c, x.on).Label(x.label).Changed() && *x.on {
				*x.other = false
			}
		})
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "itemsubs.uploadAction")
		if st.busy == "upload" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(u.path == "" || u.language == "" || !known || st.busy != "").Clicked() {
			file := *u
			a.subtitleAct(st, server, "upload", func(ctx context.Context, svc acquisitionv1connect.SubtitleServiceClient) error {
				data, err := os.ReadFile(file.path)
				if err != nil {
					return err
				}
				_, err = svc.UploadSubtitle(ctx, acquisitionv1.UploadSubtitleRequest_builder{
					ItemId: &id, Data: data, Format: &format,
					Language: acquisitionv1.SubtitleLanguageSpec_builder{Language: &file.language, Forced: &file.forced, HearingImpaired: &file.hi}.Build(),
				}.Build())
				return err
			}, func() {
				st.uploading, st.upload = false, subtitleUpload{}
				st.reload++
			})
		}
		st.actionError(c, "upload")
	})
}

// subtitleSearchDialog builds the providers' subtitles of the item in a
// language, or those its profile wants, each downloaded in place.
func (a *App) subtitleSearchDialog(c *ui.Context, st *itemSubsState, server string, it *libraryv1.Item, missing []*acquisitionv1.SubtitleLanguageSpec) {
	if !st.searching {
		return
	}
	colors := style.Colors(c)
	id := it.GetId()
	specs := missing
	if st.language != "" {
		specs = []*acquisitionv1.SubtitleLanguageSpec{acquisitionv1.SubtitleLanguageSpec_builder{Language: new(st.language)}.Build()}
	}
	found := load(a, &st.found, key("subtitle-search", server, id, st.language), func(ctx context.Context) (*acquisitionv1.SearchItemSubtitlesResponse, error) {
		return a.subtitleService(server).SearchItemSubtitles(ctx, acquisitionv1.SearchItemSubtitlesRequest_builder{ItemId: &id, Languages: specs}.Build())
	})
	title := style.T(c, "itemsubs.searchTitle", "name", it.GetName())
	ui.Modal(c, &st.searching, func() {
		ui.Column(c).Width(min(960, a.contentWidth-64)).MaxHeight(720).Gap(12).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
				style.Text(ui.Text(c, title), style.Type(c).Title2).TextColor(style.Color(colors.TextPrimary)).Grow(1).Shrink(1)
				first := style.T(c, "itemsubs.wantedLanguages")
				if len(missing) == 0 {
					first = style.T(c, "itemsubs.language")
				}
				if a.languageChoice(c, server, style.T(c, "itemsubs.language"), first, &st.language) {
					st.downloads = nil
				}
				if linkButton(c, style.T(c, "keys.done")).Clicked() {
					st.searching = false
				}
			})
			for _, p := range found.value.GetFailedProviders() {
				footnote(c.Key(p), style.T(c, "itemsubs.providerFailed", "provider", p), true)
			}
			ui.Box(c).Height(1).Background(style.Color(colors.Border))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Children(func() {
					list := found.value.GetSubtitles()
					switch {
					case !found.loaded:
						secondaryText(c, style.T(c, "itemsubs.searching"))
					case found.err != nil:
						footnote(c, errorMessage(c, found.err), true)
					case len(list) == 0:
						secondaryText(c, style.T(c, "itemsubs.noResults"))
					}
					for i, r := range list {
						if i > 0 {
							ui.Box(c.Key(key("divider", i))).Height(1).Background(style.Color(colors.Border))
						}
						a.subtitleResult(c.Key(r.GetProvider()+"/"+r.GetSubtitleId()), st, server, id, r)
					}
				})
			})
		})
	})
}

// subtitleResult builds a subtitle a provider has: its releases, language,
// score, provider, uploader and format, what it does not match, and
// downloading it.
func (a *App) subtitleResult(c *ui.Context, st *itemSubsState, server, id string, r *acquisitionv1.ScoredSubtitle) {
	colors := style.Colors(c)
	k := r.GetProvider() + "/" + r.GetSubtitleId()
	if st.downloads == nil {
		st.downloads = map[string]*grab{}
	}
	g := st.downloads[k]
	if g == nil {
		g = &grab{}
		st.downloads[k] = g
	}
	detail := []string{specName(c, r.GetLanguage()), style.T(c, "itemsubs.score", "percent", r.GetPercent()), r.GetProvider(), r.GetUploader(), strings.ToUpper(r.GetFormat())}
	names := r.GetReleaseNames()
	ui.Row(c).Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			name := r.GetSubtitleId()
			if len(names) > 0 {
				name = names[0]
			}
			style.Text(ui.Text(c, name), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
			if len(names) > 1 {
				footnote(c, strings.Join(names[1:], ", "), false)
			}
			footnote(c, joinDetail(detail...), false)
			if len(r.GetNotMatched()) > 0 {
				style.Text(ui.Text(c, style.T(c, "itemsubs.notMatched", "fields", strings.Join(r.GetNotMatched(), ", "))), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary))
			}
			if r.GetBlocklisted() {
				footnote(c, style.T(c, "itemsubs.blocklisted"), true)
			}
			if g.err != nil {
				footnote(c, errorMessage(c, g.err), true)
			}
		})
		label := style.T(c, "itemsubs.download")
		switch {
		case g.done:
			label = style.T(c, "itemsubs.downloaded")
		case g.busy:
			label = "…"
		}
		disabled := g.busy || g.done || !r.HasLanguage()
		var clicked bool
		if r.GetBlocklisted() {
			clicked = smallButton(c, label, disabled)
		} else {
			clicked = ui.PrimaryButton(c, label).Disabled(disabled).Clicked()
		}
		if clicked && !disabled {
			g.busy, g.err = true, nil
			provider, sub := r.GetProvider(), r.GetSubtitleId()
			go func() {
				_, err := a.subtitleService(server).DownloadItemSubtitle(a.ctx, acquisitionv1.DownloadItemSubtitleRequest_builder{
					ItemId: &id, Provider: &provider, SubtitleId: &sub, Language: r.GetLanguage(),
				}.Build())
				a.opts.Update(func() {
					g.busy, g.err, g.done = false, err, err == nil
					if err == nil {
						st.reload++
					}
				})
			}()
		}
	})
}
