package app

import (
	"context"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// subtitleMods are the mods a download can have applied, as Bazarr names
// them, with their labels' keys; those taking arguments are kept as they
// are.
var subtitleMods = [][2]string{
	{"OCR_fixes", "subtitles.modOcr"},
	{"remove_HI", "subtitles.modRemoveHi"},
	{"common", "subtitles.modCommon"},
	{"remove_tags", "subtitles.modRemoveTags"},
	{"fix_uppercase", "subtitles.modUppercase"},
	{"reverse_rtl", "subtitles.modRtl"},
	{"emoji", "subtitles.modEmoji"},
}

var wordSeparators = regexp.MustCompile(`[\s,]+`)

// words returns the words of a text, separated by spaces or commas.
func words(s string) []string {
	return slices.DeleteFunc(wordSeparators.Split(s, -1), func(w string) bool { return w == "" })
}

// withParts returns settings whose nested messages are all set, so that
// a draft's are edited in place and compare equal to the saved ones.
func withParts(s *acquisitionv1.SubtitleSettings) *acquisitionv1.SubtitleSettings {
	s = proto.CloneOf(s)
	if !s.HasSync() {
		s.SetSync(&acquisitionv1.SubtitleSyncSettings{})
	}
	if !s.HasTranslation() {
		s.SetTranslation(&acquisitionv1.SubtitleTranslationSettings{})
	}
	t := s.GetTranslation()
	if !t.HasValidation() {
		t.SetValidation(&acquisitionv1.SubtitleTranslationValidation{})
	}
	if !t.HasAutomation() {
		t.SetAutomation(&acquisitionv1.SubtitleTranslationAutomation{})
	}
	return s
}

// subtitleSettingsPage builds a page of the subtitle settings: searching
// and saving, synchronization or translation.
func (a *App) subtitleSettingsPage(c *ui.Context, page string) {
	st := &a.settingsState.admin.subtitles
	a.backTo(c, style.T(c, "settings.subtitles"), subtitlesPath("", ""))
	title := map[string]string{"settings": "subtitles.searching", "sync": "subtitles.sync", "translation": "subtitles.translation"}[page]
	sectionHeader(c, style.T(c, title), style.T(c, title+"Detail"))
	t := adminTask(a, &st.settings, "subtitle-settings", func(ctx context.Context, s string) (*acquisitionv1.SubtitleSettings, error) {
		res, err := a.subtitleService(s).GetSubtitleSettings(ctx, &acquisitionv1.GetSubtitleSettingsRequest{})
		return res.GetSettings(), err
	})
	if t.err != nil {
		dangerText(c, errorMessage(c, t.err))
	}
	if st.draft == nil && t.loaded && t.err == nil {
		st.draft0 = withParts(t.value)
		st.draft = proto.CloneOf(st.draft0)
	}
	if st.draft == nil {
		return
	}
	switch page {
	case "settings":
		a.searchingForm(c)
	case "sync":
		a.syncForm(c)
	default:
		a.translationForm(c)
	}
	changed := !proto.Equal(st.draft, st.draft0)
	if changed {
		st.notice = ""
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		label := style.T(c, "action.save")
		if st.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || st.busy != "").Clicked() {
			draft := proto.CloneOf(st.draft)
			var saved *acquisitionv1.SubtitleSettings
			a.subtitleAction("save", func(ctx context.Context, s acquisitionv1connect.SubtitleServiceClient) error {
				res, err := s.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: draft}.Build())
				saved = res.GetSettings()
				return err
			}, func() {
				st.draft0 = withParts(cmpOrMessage(saved, draft))
				st.draft, st.words, st.notice = proto.CloneOf(st.draft0), nil, "subtitles.saved"
			})
		}
		if changed && linkButton(c, style.T(c, "action.cancel")).Clicked() {
			st.draft, st.words = proto.CloneOf(st.draft0), nil
		}
		if st.notice != "" && !changed {
			secondaryText(c, style.T(c, st.notice))
		}
		if st.err != nil {
			dangerText(c, errorMessage(c, st.err))
		}
	})
}

// cmpOrMessage returns m, or else if m is nil.
func cmpOrMessage[M proto.Message](m, otherwise M) M {
	var zero M
	if any(m) == any(zero) {
		return otherwise
	}
	return m
}

// numberRow adds a row of a whole number between lo and hi.
func numberRow(g *settingsGroup, label, detail string, value int32, lo, hi float64, set func(int32)) {
	g.row(label, detail, nil, func() {
		numberField(g.c.Key(label), label, float64(value), lo, hi, 1, func(v float64) { set(int32(math.Trunc(v))) })
	})
}

// fractionRow adds a row of a number of steps of step.
func fractionRow(g *settingsGroup, label, detail string, value, lo, hi, step float64, set func(float64)) {
	g.row(label, detail, nil, func() {
		numberField(g.c.Key(label), label, value, lo, hi, step, set)
	})
}

// toggle adds a row of a switch.
func toggle(g *settingsGroup, label, detail string, on bool, set func(bool)) {
	if g.switchRow(label, detail, &on) {
		set(on)
	}
}

// wordsRow adds a row of a list of words, edited as text.
func (a *App) wordsRow(g *settingsGroup, label, detail string, value []string, set func([]string)) {
	st := &a.settingsState.admin.subtitles
	if st.words == nil {
		st.words = map[string]string{}
	}
	text, ok := st.words[label]
	if !ok {
		text = strings.Join(value, ", ")
	}
	g.row(label, detail, nil, func() {
		if ui.TextInput(g.c.Key(label), &text).Width(288).Label(label).Changed() {
			set(words(text))
		}
		st.words[label] = text
	})
}

// textBlock adds a row of a block of text, such as a prompt, with its
// placeholders.
func textBlock(g *settingsGroup, label, detail string, value string, height float32, set func(string)) {
	c := g.c
	colors := style.Colors(c)
	g.divider()
	ui.Column(c.Key(label)).Padding(10, 16).Gap(6).Children(func() {
		style.Text(ui.Text(c, label), style.Type.Body).TextColor(style.Color(colors.TextPrimary))
		if detail != "" {
			style.Text(ui.Text(c, detail), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
		}
		if ui.TextArea(c, &value).FillWidth().Height(height).Font("SF Mono, Menlo, monospace").Label(label).Changed() {
			set(value)
		}
	})
}

// languageRow adds a row choosing a language, or none labeled unset.
func (a *App) languageRow(g *settingsGroup, label, detail, value, unset string, set func(string)) {
	c := g.c
	codes, names := a.subtitleLanguages(c, value)
	codes, names = append([]string{""}, codes...), append([]string{unset}, names...)
	g.row(label, detail, nil, func() {
		v := value
		chooseString(c.Key(label), label, &v, codes, names)
		if v != value {
			set(v)
		}
	})
}

func (a *App) searchingForm(c *ui.Context) {
	d := a.settingsState.admin.subtitles.draft
	t := func(key string) string { return style.T(c, key) }
	group(c, "scores", t("subtitles.scores"), t("subtitles.scoresFooter"), func(g *settingsGroup) {
		numberRow(g, t("subtitles.minimumScore"), "", d.GetMinimumScore(), 0, 100, d.SetMinimumScore)
		numberRow(g, t("subtitles.minimumScoreMovie"), "", d.GetMinimumScoreMovie(), 0, 100, d.SetMinimumScoreMovie)
	})
	const most = 1_000_000
	group(c, "searches", t("subtitles.searches"), "", func(g *settingsGroup) {
		numberRow(g, t("subtitles.wantedInterval"), t("subtitles.hoursNever"), d.GetWantedSearchIntervalHours(), 0, most, d.SetWantedSearchIntervalHours)
		numberRow(g, t("subtitles.wantedIntervalMovie"), t("subtitles.hoursNever"), d.GetWantedSearchIntervalMovieHours(), 0, most, d.SetWantedSearchIntervalMovieHours)
		toggle(g, t("subtitles.adaptive"), t("subtitles.adaptiveDetail"), d.GetAdaptiveSearching(), d.SetAdaptiveSearching)
		if d.GetAdaptiveSearching() {
			numberRow(g, t("subtitles.adaptiveDelay"), "", d.GetAdaptiveDelayDays(), 0, most, d.SetAdaptiveDelayDays)
			numberRow(g, t("subtitles.adaptiveDelta"), "", d.GetAdaptiveDeltaDays(), 0, most, d.SetAdaptiveDeltaDays)
		}
		toggle(g, t("subtitles.sceneName"), t("subtitles.sceneNameDetail"), d.GetUseSceneName(), d.SetUseSceneName)
		toggle(g, t("subtitles.skipHashing"), t("subtitles.skipHashingDetail"), d.GetSkipHashing(), d.SetSkipHashing)
	})
	group(c, "embedded", t("subtitles.embedded"), "", func(g *settingsGroup) {
		toggle(g, t("subtitles.useEmbedded"), t("subtitles.useEmbeddedDetail"), d.GetUseEmbedded(), d.SetUseEmbedded)
		if d.GetUseEmbedded() {
			toggle(g, t("subtitles.ignorePgs"), "", d.GetIgnorePgs(), d.SetIgnorePgs)
			toggle(g, t("subtitles.ignoreVobsub"), "", d.GetIgnoreVobsub(), d.SetIgnoreVobsub)
			toggle(g, t("subtitles.ignoreAss"), "", d.GetIgnoreAss(), d.SetIgnoreAss)
		}
		a.languageRow(g, t("subtitles.defaultAudio"), t("subtitles.defaultAudioDetail"), d.GetDefaultAudioLanguage(), t("subtitles.languageUnset"), d.SetDefaultAudioLanguage)
		a.languageRow(g, t("subtitles.defaultEmbedded"), "", d.GetDefaultEmbeddedLanguage(), t("subtitles.languageUnset"), d.SetDefaultEmbeddedLanguage)
	})
	group(c, "upgrades", t("subtitles.upgrades"), "", func(g *settingsGroup) {
		toggle(g, t("subtitles.upgrade"), t("subtitles.upgradeDetail"), d.GetUpgrade(), d.SetUpgrade)
		if d.GetUpgrade() {
			numberRow(g, t("subtitles.upgradeInterval"), "", d.GetUpgradeIntervalHours(), 1, most, d.SetUpgradeIntervalHours)
			numberRow(g, t("subtitles.upgradeDays"), "", d.GetUpgradeDays(), 0, 30, d.SetUpgradeDays)
			toggle(g, t("subtitles.upgradeManual"), "", d.GetUpgradeManual(), d.SetUpgradeManual)
		}
	})
	group(c, "saving", t("subtitles.saving"), "", func(g *settingsGroup) {
		g.row(t("subtitles.folder"), "", nil, func() {
			folder := int(d.GetFolder())
			if choose(c, t("subtitles.folder"), &folder, []string{t("subtitles.folderVideo"), t("subtitles.folderRelative"), t("subtitles.folderMetadata")}) {
				f := acquisitionv1.SubtitleFolder(folder)
				d.SetFolder(f)
				custom := ""
				if f == acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_RELATIVE {
					custom = cmpOr(d.GetCustomFolder(), "Subs")
				}
				d.SetCustomFolder(custom)
			}
		})
		if d.GetFolder() == acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_RELATIVE {
			g.row(t("subtitles.customFolder"), "", nil, func() {
				folder := d.GetCustomFolder()
				if ui.TextInput(c, &folder).Width(192).Label(t("subtitles.customFolder")).Changed() {
					d.SetCustomFolder(strings.TrimSpace(folder))
				}
			})
		}
		g.row(t("subtitles.hiExtension"), t("subtitles.hiExtensionDetail"), nil, func() {
			ext := cmpOr(d.GetHiExtension(), "hi")
			was := ext
			chooseString(c, t("subtitles.hiExtension"), &ext, []string{"hi", "cc", "sdh"}, nil)
			if ext != was {
				d.SetHiExtension(ext)
			}
		})
		toggle(g, t("subtitles.singleLanguage"), t("subtitles.singleLanguageDetail"), d.GetSingleLanguage(), d.SetSingleLanguage)
		toggle(g, t("subtitles.utf"), "", d.GetUtf8(), d.SetUtf8)
	})
	group(c, "mods", t("subtitles.mods"), t("subtitles.modsFooter"), func(g *settingsGroup) {
		for _, m := range subtitleMods {
			toggle(g, t(m[1]), "", slices.Contains(d.GetMods(), m[0]), func(on bool) {
				mods := slices.DeleteFunc(slices.Clone(d.GetMods()), func(x string) bool { return x == m[0] })
				if on {
					mods = append(mods, m[0])
				}
				d.SetMods(mods)
			})
		}
	})
	a.equivalences(c, d)
	group(c, "tags", t("subtitles.tags"), t("subtitles.tagsFooter"), func(g *settingsGroup) {
		toggle(g, t("subtitles.movieTags"), "", d.GetMovieProfileTags(), d.SetMovieProfileTags)
		toggle(g, t("subtitles.seriesTags"), "", d.GetSeriesProfileTags(), d.SetSeriesProfileTags)
		a.wordsRow(g, t("subtitles.noProfileTags"), t("subtitles.noProfileTagsDetail"), d.GetNoProfileTags(), d.SetNoProfileTags)
	})
}

// specKind returns a language spec's kind, an index of the kinds' labels:
// normal, forced or hearing impaired.
func specKind(s *acquisitionv1.SubtitleLanguageSpec) int {
	switch {
	case s.GetForced():
		return 1
	case s.GetHearingImpaired():
		return 2
	}
	return 0
}

// equivalences builds the languages whose subtitles count as another's.
func (a *App) equivalences(c *ui.Context, d *acquisitionv1.SubtitleSettings) {
	value := d.GetLanguageEquals()
	var used []string
	for _, e := range value {
		used = append(used, e.GetFrom().GetLanguage(), e.GetTo().GetLanguage())
	}
	codes, names := a.subtitleLanguages(c, used...)
	kinds := []string{style.T(c, "subtitles.kindNormal"), style.T(c, "subtitles.forced"), style.T(c, "subtitles.hiShort")}
	spec := func(language string, kind int) *acquisitionv1.SubtitleLanguageSpec {
		return acquisitionv1.SubtitleLanguageSpec_builder{Language: &language, Forced: new(kind == 1), HearingImpaired: new(kind == 2)}.Build()
	}
	add := func() {
		if linkButton(c, style.T(c, "rules.add")).Clicked() {
			d.SetLanguageEquals(append(slices.Clone(value), acquisitionv1.SubtitleLanguageEquivalence_builder{From: spec("pt-BR", 0), To: spec("pt", 0)}.Build()))
		}
	}
	groupWith(c, "equivalences", style.T(c, "subtitles.equivalences"), add, style.T(c, "subtitles.equivalencesFooter"), func(g *settingsGroup) {
		for i, e := range value {
			set := func(from bool, s *acquisitionv1.SubtitleLanguageSpec) {
				next := slices.Clone(value)
				n := proto.CloneOf(e)
				if from {
					n.SetFrom(s)
				} else {
					n.SetTo(s)
				}
				next[i] = n
				d.SetLanguageEquals(next)
			}
			side := func(key string, s *acquisitionv1.SubtitleLanguageSpec, from bool) {
				language, kind := cmpOr(s.GetLanguage(), "en"), specKind(s)
				chooseString(c.Key(key+"language"), style.T(c, "subtitles.language"), &language, codes, names)
				if choose(c.Key(key+"kind"), style.T(c, "subtitles.kind"), &kind, kinds) || language != cmpOr(s.GetLanguage(), "en") {
					set(from, spec(language, kind))
				}
			}
			g.divider()
			ui.Row(c.Key(i)).Padding(10, 16).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				side("from", e.GetFrom(), true)
				secondaryText(c, "→")
				side("to", e.GetTo(), false)
				ui.Box(c).Grow(1)
				if iconButton(c, "close", style.T(c, "rules.remove")).Clicked() {
					d.SetLanguageEquals(slices.Delete(slices.Clone(value), i, i+1))
				}
			})
		}
		if len(value) == 0 {
			g.note(style.T(c, "subtitles.noEquivalences"), false)
		}
	})
}

func (a *App) syncForm(c *ui.Context) {
	y := a.settingsState.admin.subtitles.draft.GetSync()
	t := func(key string) string { return style.T(c, key) }
	providers := a.subtitleProviders()
	group(c, "sync", "", t("subtitles.syncFooter"), func(g *settingsGroup) {
		toggle(g, t("subtitles.syncEnabled"), "", y.GetEnabled(), y.SetEnabled)
		if !y.GetEnabled() {
			return
		}
		toggle(g, t("subtitles.syncThreshold"), t("subtitles.syncThresholdDetail"), y.GetUseThreshold(), y.SetUseThreshold)
		if y.GetUseThreshold() {
			numberRow(g, t("subtitles.threshold"), "", y.GetThreshold(), 0, 100, y.SetThreshold)
		}
		toggle(g, t("subtitles.syncMovieThreshold"), "", y.GetUseMovieThreshold(), y.SetUseMovieThreshold)
		if y.GetUseMovieThreshold() {
			numberRow(g, t("subtitles.threshold"), "", y.GetMovieThreshold(), 0, 100, y.SetMovieThreshold)
		}
		toggle(g, t("subtitles.forceAudio"), t("subtitles.forceAudioDetail"), y.GetForceAudio(), y.SetForceAudio)
		toggle(g, t("subtitles.originalLanguage"), t("subtitles.originalLanguageDetail"), y.GetOriginalLanguage(), y.SetOriginalLanguage)
		var names []string
		for _, p := range providers.value.GetProviders() {
			names = append(names, p.GetName())
		}
		detail := ""
		if len(names) > 0 {
			detail = style.T(c, "subtitles.excludedProvidersDetail", "names", strings.Join(names, ", "))
		}
		a.wordsRow(g, t("subtitles.excludedProviders"), detail, y.GetExcludedProviders(), y.SetExcludedProviders)
		a.wordsRow(g, t("subtitles.excludedLanguages"), t("subtitles.excludedLanguagesDetail"), y.GetExcludedLanguages(), y.SetExcludedLanguages)
	})
	if !y.GetEnabled() {
		return
	}
	const most = 1_000_000
	group(c, "alignment", t("subtitles.alignment"), "", func(g *settingsGroup) {
		numberRow(g, t("subtitles.maxOffset"), "", y.GetMaxOffsetSeconds(), 1, most, y.SetMaxOffsetSeconds)
		toggle(g, t("subtitles.fixFramerate"), t("subtitles.fixFramerateDetail"), y.GetFixFramerate(), y.SetFixFramerate)
		toggle(g, t("subtitles.goldenSection"), t("subtitles.goldenSectionDetail"), y.GetGoldenSection(), y.SetGoldenSection)
		fractionRow(g, t("subtitles.minScore"), "", y.GetMinScore(), 0, most, 0.01, y.SetMinScore)
		numberRow(g, t("subtitles.maxQualityOffset"), "", y.GetMaxQualityOffsetSeconds(), 0, most, y.SetMaxQualityOffsetSeconds)
		fractionRow(g, t("subtitles.maxScaleDeviation"), "", y.GetMaxScaleDeviation(), 0, most, 0.001, y.SetMaxScaleDeviation)
	})
}

func (a *App) translationForm(c *ui.Context) {
	tr := a.settingsState.admin.subtitles.draft.GetTranslation()
	v, au := tr.GetValidation(), tr.GetAutomation()
	t := func(key string) string { return style.T(c, key) }
	translators, models := a.subtitleTranslators(), a.aiModels()
	var ids, labels []string
	for _, x := range translators.value.GetTranslators() {
		ids, labels = append(ids, x.GetId()), append(labels, cmpOr(x.GetDisplayName(), x.GetName()))
	}
	for _, m := range models.value {
		ids, labels = append(ids, "ai/"+m.GetProvider()+"/"+m.GetId()), append(labels, cmpOr(m.GetName(), m.GetId())+" ("+m.GetProvider()+")")
	}
	named := func(id string) string {
		if i := slices.Index(ids, id); i >= 0 {
			return labels[i]
		}
		return id
	}
	group(c, "translators", t("subtitles.translators"), t("subtitles.translatorFooter"), func(g *settingsGroup) {
		g.row(t("subtitles.translator"), "", nil, func() {
			translator := tr.GetTranslator()
			chooseString(c, t("subtitles.translator"), &translator, append([]string{""}, ids...), append([]string{t("subtitles.translatorFirst")}, labels...))
			if translator != tr.GetTranslator() {
				tr.SetTranslator(translator)
				tr.SetFallbackTranslators(slices.DeleteFunc(slices.Clone(tr.GetFallbackTranslators()), func(x string) bool { return x == translator }))
			}
		})
		fallbacks := tr.GetFallbackTranslators()
		for i, id := range fallbacks {
			g.row(named(id), style.T(c, "subtitles.fallbackOf", "position", i+1), nil, func() {
				if iconButton(c.Key(id), "close", t("rules.remove")).Clicked() {
					tr.SetFallbackTranslators(slices.Delete(slices.Clone(fallbacks), i, i+1))
				}
			})
		}
		unusedIDs, unusedLabels := []string{""}, []string{t("subtitles.choose")}
		for i, id := range ids {
			if id != tr.GetTranslator() && !slices.Contains(fallbacks, id) {
				unusedIDs, unusedLabels = append(unusedIDs, id), append(unusedLabels, labels[i])
			}
		}
		if len(unusedIDs) > 1 {
			g.row(t("subtitles.addFallback"), "", nil, func() {
				id := ""
				chooseString(c, t("subtitles.addFallback"), &id, unusedIDs, unusedLabels)
				if id != "" {
					tr.SetFallbackTranslators(append(slices.Clone(fallbacks), id))
				}
			})
		}
		for _, err := range []error{translators.err, models.err} {
			if err != nil {
				g.note(errorMessage(c, err), true)
			}
		}
	})
	group(c, "output", t("subtitles.output"), "", func(g *settingsGroup) {
		numberRow(g, t("subtitles.defaultScore"), t("subtitles.defaultScoreDetail"), tr.GetDefaultScore(), 0, 100, tr.SetDefaultScore)
		toggle(g, t("subtitles.credit"), t("subtitles.creditDetail"), tr.GetCredit(), tr.SetCredit)
		toggle(g, t("subtitles.stripFormatting"), t("subtitles.stripFormattingDetail"), tr.GetStripFormatting(), tr.SetStripFormatting)
		toggle(g, t("subtitles.preserveLineBreaks"), "", tr.GetPreserveLineBreaks(), tr.SetPreserveLineBreaks)
		toggle(g, t("subtitles.fixOverlaps"), "", tr.GetFixOverlaps(), tr.SetFixOverlaps)
	})
	const most = 1_000_000
	group(c, "models", t("subtitles.models"), t("subtitles.modelsFooter"), func(g *settingsGroup) {
		numberRow(g, t("subtitles.contextBefore"), "", tr.GetContextBefore(), 0, 50, tr.SetContextBefore)
		numberRow(g, t("subtitles.contextAfter"), "", tr.GetContextAfter(), 0, 50, tr.SetContextAfter)
		toggle(g, t("subtitles.translatedContext"), t("subtitles.translatedContextDetail"), tr.GetTranslatedContext(), tr.SetTranslatedContext)
		toggle(g, t("subtitles.batch"), t("subtitles.batchDetail"), tr.GetBatch(), tr.SetBatch)
		if tr.GetBatch() {
			numberRow(g, t("subtitles.maxBatchSize"), t("subtitles.maxBatchSizeDetail"), tr.GetMaxBatchSize(), 0, most, tr.SetMaxBatchSize)
		}
		toggle(g, t("subtitles.languageCodes"), "", tr.GetLanguageCodes(), tr.SetLanguageCodes)
		textBlock(g, t("subtitles.prompt"), "{sourceLanguage}, {targetLanguage}, {lineToTranslate}, {contextBefore}, {contextAfter}", tr.GetPrompt(), 120, tr.SetPrompt)
		textBlock(g, t("subtitles.userPrompt"), "", tr.GetUserPrompt(), 84, tr.SetUserPrompt)
		textBlock(g, t("subtitles.proofreadPrompt"), "{sourceLanguage}, {targetLanguage}, {sourceLine}, {translatedLine}", tr.GetProofreadPrompt(), 120, tr.SetProofreadPrompt)
		textBlock(g, t("subtitles.proofreadUserPrompt"), "", tr.GetProofreadUserPrompt(), 84, tr.SetProofreadUserPrompt)
	})
	group(c, "retries", t("subtitles.retries"), "", func(g *settingsGroup) {
		numberRow(g, t("subtitles.maxRetries"), "", tr.GetMaxRetries(), 1, 20, tr.SetMaxRetries)
		numberRow(g, t("subtitles.retryDelay"), "", tr.GetRetryDelaySeconds(), 0, 600, tr.SetRetryDelaySeconds)
		numberRow(g, t("subtitles.retryMultiplier"), "", tr.GetRetryMultiplier(), 1, 10, tr.SetRetryMultiplier)
		numberRow(g, t("subtitles.requestTimeout"), "", tr.GetRequestTimeoutSeconds(), 1, 3600, tr.SetRequestTimeoutSeconds)
	})
	group(c, "validation", t("subtitles.validation"), t("subtitles.validationFooter"), func(g *settingsGroup) {
		toggle(g, t("subtitles.validationEnabled"), "", v.GetEnabled(), v.SetEnabled)
		if !v.GetEnabled() {
			return
		}
		numberRow(g, t("subtitles.maxFileSize"), "", int32(math.Round(float64(v.GetMaxFileSize())/1024)), 1, most, func(kb int32) {
			v.SetMaxFileSize(int64(max(1, kb)) * 1024)
		})
		numberRow(g, t("subtitles.maxCueLength"), "", v.GetMaxCueLength(), 1, most, v.SetMaxCueLength)
		numberRow(g, t("subtitles.minCueLength"), "", v.GetMinCueLength(), 0, most, v.SetMinCueLength)
		numberRow(g, t("subtitles.minDuration"), "", v.GetMinDurationMs(), 0, most, v.SetMinDurationMs)
		numberRow(g, t("subtitles.maxDuration"), "", v.GetMaxDurationSeconds(), 1, most, v.SetMaxDurationSeconds)
	})
	group(c, "automation", t("subtitles.automation"), t("subtitles.automationFooter"), func(g *settingsGroup) {
		toggle(g, t("subtitles.automationEnabled"), "", au.GetEnabled(), au.SetEnabled)
		if !au.GetEnabled() {
			return
		}
		numberRow(g, t("subtitles.automationInterval"), "", au.GetIntervalHours(), 1, 8760, au.SetIntervalHours)
		sources := au.GetSourceLanguages()
		for i, l := range sources {
			g.row(cmpOr(languageName(c, l), l), t("subtitles.sourceLanguage"), nil, func() {
				if iconButton(c.Key(l), "close", t("rules.remove")).Clicked() {
					au.SetSourceLanguages(slices.Delete(slices.Clone(sources), i, i+1))
				}
			})
		}
		codes, names := a.subtitleLanguages(c, sources...)
		addCodes, addNames := []string{""}, []string{t("subtitles.choose")}
		for i, code := range codes {
			if !slices.Contains(sources, code) {
				addCodes, addNames = append(addCodes, code), append(addNames, names[i])
			}
		}
		g.row(t("subtitles.addSourceLanguage"), "", nil, func() {
			l := ""
			chooseString(c, t("subtitles.addSourceLanguage"), &l, addCodes, addNames)
			if l != "" {
				au.SetSourceLanguages(append(slices.Clone(sources), l))
			}
		})
		numberRow(g, t("subtitles.maxPerRun"), "", au.GetMaxPerRun(), 1, 1000, au.SetMaxPerRun)
		numberRow(g, t("subtitles.movieAge"), "", au.GetMovieAgeHours(), 0, most, au.SetMovieAgeHours)
		numberRow(g, t("subtitles.episodeAge"), "", au.GetEpisodeAgeHours(), 0, most, au.SetEpisodeAgeHours)
	})
}
