#if os(macOS)
    import AppKit
    import MavioMac

    /// A page of the subtitle settings (docs/clients.md §6.3.12), as the web
    /// UI's: the settings as edited, saved whole, or the changes cancelled.
    final class SubtitleSettingsPage: FormController {
        private typealias Settings = Mavio_Acquisition_V1_SubtitleSettings
        private typealias Spec = Mavio_Acquisition_V1_SubtitleLanguageSpec
        private typealias Equivalence = Mavio_Acquisition_V1_SubtitleLanguageEquivalence

        private let section: SubtitleSettingsSection
        private var saved: Settings?
        private var draft = Settings()
        private var failure: String?
        private var saveFailure: String?
        private var notice: String?
        private var saving = false
        private var known: [Choice] = []
        private var providers: [Mavio_Acquisition_V1_SubtitleProviderState] = []
        private var translators: [Mavio_Acquisition_V1_SubtitleTranslatorState] = []
        private var models: [Mavio_Plugin_V1_AiModel] = []
        private var optionsFailure: String?
        private var saveBar: SaveBar?
        private var cancelButton: MavioButton?

        /// The largest a number without its own limit can be.
        private static let most = Double(Int32.max)

        /// The page of section of the settings of server.
        init(context: MacContext, server: ServerRecord, section: SubtitleSettingsSection) {
            self.section = section
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            title = section.title
            message = section.message
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var views: [NSView] = []
            if saved != nil {
                switch section {
                case .searching: views += searching()
                case .sync: views += sync()
                case .translation: views += translation()
                }
                let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                    guard let self, let saved = self.saved else { return }
                    self.draft = saved
                    self.saveFailure = nil
                    self.render()
                }
                cancelButton = cancel
                let bar = SaveBar(MavioStrings.actionSave, theme: theme, extra: [cancel]) { [weak self] in self?.save()
                }
                saveBar = bar
                refresh()
                views.append(bar)
            } else if failure == nil {
                views.append(card([spinnerRow()]))
            }
            if let failure { views.append(card([note(failure, failed: true)])) }
            return views
        }

        private var changed: Bool { saved.map { $0 != draft } ?? false }

        private func refresh() {
            let changed = changed
            cancelButton?.isHidden = !changed || saving
            saveBar?.update(enabled: changed, busy: saving, notice: changed ? nil : notice, failure: saveFailure)
        }

        // MARK: Rows

        private func number(
            _ title: String, _ field: WritableKeyPath<Settings, Int32>, _ range: ClosedRange<Double> = 0...most,
            subtitle: String? = nil
        ) -> FormRow {
            numberRow(title, subtitle: subtitle, value: Double(draft[keyPath: field]), range: range) { [weak self] v in
                self?.draft[keyPath: field] = Int32(v.rounded())
                self?.refresh()
            }
        }

        private func toggle(_ title: String, _ field: WritableKeyPath<Settings, Bool>, subtitle: String? = nil)
            -> FormRow
        {
            toggleRow(title, subtitle: subtitle, isOn: draft[keyPath: field]) { [weak self] on in
                self?.draft[keyPath: field] = on
                self?.render()
            }
        }

        /// A row of a list of words, edited as text separated by commas or spaces.
        private func words(_ title: String, subtitle: String? = nil, _ field: WritableKeyPath<Settings, [String]>)
            -> FormRow
        {
            fieldRow(title, subtitle: subtitle, text: draft[keyPath: field].joined(separator: ", "), width: 288) {
                [weak self] text in
                self?.draft[keyPath: field] = text.split { $0.isWhitespace || $0 == "," }.map(String.init)
                self?.refresh()
            }
        }

        /// A row of a number that has decimals.
        private func decimal(_ title: String, _ field: WritableKeyPath<Settings, Double>) -> FormRow {
            numberRow(
                title, value: draft[keyPath: field], range: -Self.most...Self.most, stepped: false, decimals: 3
            ) { [weak self] v in
                self?.draft[keyPath: field] = v
                self?.refresh()
            }
        }

        private func languageChoice(
            _ title: String, _ field: WritableKeyPath<Settings, String>, _ languages: [Choice], subtitle: String? = nil
        ) -> FormRow {
            let options = [(MavioStrings.subtitlesLanguageUnset, "")] + languages.map { ($0.name, $0.code) }
            return choiceRow(title, subtitle: subtitle, options: options, selected: draft[keyPath: field]) {
                [weak self] code in
                self?.draft[keyPath: field] = code
                self?.refresh()
            }
        }

        /// A block of text, such as a prompt, with its placeholders.
        private func textBlock(
            _ title: String, _ field: WritableKeyPath<Settings, String>, detail: String? = nil, lines: Int = 4
        ) -> NSView {
            let block = TextBlock(draft[keyPath: field], label: title, lines: lines, theme: theme) { [weak self] text in
                self?.draft[keyPath: field] = text
                self?.refresh()
            }
            var views: [NSView] = [NSTextField.label(title, theme.fonts.body, color: theme.colors.textPrimary)]
            if let detail {
                views.append(
                    NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0))
            }
            views.append(block)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s4)
            block.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return row(stack)
        }

        /// A row of what a list holds, removed by its button.
        private func listedRow(_ title: String, subtitle: String, remove: @escaping @MainActor () -> Void) -> FormRow {
            FormRow(title, subtitle: subtitle, accessory: removeButton(remove), theme: theme)
        }

        /// A menu adding one of options to a list.
        private func addChoice(_ title: String, _ options: [Choice], add: @escaping @MainActor (String) -> Void)
            -> FormRow
        {
            choiceRow(
                title, options: [(MavioStrings.subtitlesChoose, "")] + options.map { ($0.name, $0.code) }, selected: ""
            ) {
                code in if !code.isEmpty { add(code) }
            }
        }

        // MARK: Searching and saving

        private func searching() -> [NSView] {
            let unset = SubtitlesNaming.languages(known, [draft.defaultAudioLanguage, draft.defaultEmbeddedLanguage])
            var searches: [NSView] = [
                number(
                    MavioStrings.subtitlesWantedInterval, \.wantedSearchIntervalHours,
                    subtitle: MavioStrings.subtitlesHoursNever),
                number(
                    MavioStrings.subtitlesWantedIntervalMovie, \.wantedSearchIntervalMovieHours,
                    subtitle: MavioStrings.subtitlesHoursNever),
                toggle(
                    MavioStrings.subtitlesAdaptive, \.adaptiveSearching, subtitle: MavioStrings.subtitlesAdaptiveDetail),
            ]
            if draft.adaptiveSearching {
                searches.append(number(MavioStrings.subtitlesAdaptiveDelay, \.adaptiveDelayDays))
                searches.append(number(MavioStrings.subtitlesAdaptiveDelta, \.adaptiveDeltaDays))
            }
            searches.append(
                toggle(MavioStrings.subtitlesSceneName, \.useSceneName, subtitle: MavioStrings.subtitlesSceneNameDetail)
            )
            searches.append(
                toggle(
                    MavioStrings.subtitlesSkipHashing, \.skipHashing, subtitle: MavioStrings.subtitlesSkipHashingDetail)
            )
            var embedded: [NSView] = [
                toggle(
                    MavioStrings.subtitlesUseEmbedded, \.useEmbedded, subtitle: MavioStrings.subtitlesUseEmbeddedDetail)
            ]
            if draft.useEmbedded {
                embedded.append(toggle(MavioStrings.subtitlesIgnorePgs, \.ignorePgs))
                embedded.append(toggle(MavioStrings.subtitlesIgnoreVobsub, \.ignoreVobsub))
                embedded.append(toggle(MavioStrings.subtitlesIgnoreAss, \.ignoreAss))
            }
            embedded.append(
                languageChoice(
                    MavioStrings.subtitlesDefaultAudio, \.defaultAudioLanguage, unset,
                    subtitle: MavioStrings.subtitlesDefaultAudioDetail))
            embedded.append(languageChoice(MavioStrings.subtitlesDefaultEmbedded, \.defaultEmbeddedLanguage, unset))
            var upgrades: [NSView] = [
                toggle(MavioStrings.subtitlesUpgrade, \.upgrade, subtitle: MavioStrings.subtitlesUpgradeDetail)
            ]
            if draft.upgrade {
                upgrades.append(number(MavioStrings.subtitlesUpgradeInterval, \.upgradeIntervalHours, 1...Self.most))
                upgrades.append(number(MavioStrings.subtitlesUpgradeDays, \.upgradeDays, 0...30))
                upgrades.append(toggle(MavioStrings.subtitlesUpgradeManual, \.upgradeManual))
            }
            var saving: [NSView] = [
                choiceRow(
                    MavioStrings.subtitlesFolder,
                    options: [
                        (MavioStrings.subtitlesFolderVideo, Mavio_Acquisition_V1_SubtitleFolder.unspecified),
                        (MavioStrings.subtitlesFolderRelative, .relative),
                        (MavioStrings.subtitlesFolderMetadata, .metadata),
                    ], selected: draft.folder
                ) { [weak self] folder in
                    guard let self else { return }
                    self.draft.folder = folder
                    self.draft.customFolder =
                        folder == .relative ? (self.draft.customFolder.isEmpty ? "Subs" : self.draft.customFolder) : ""
                    self.render()
                }
            ]
            if draft.folder == .relative {
                saving.append(
                    fieldRow(MavioStrings.subtitlesCustomFolder, text: draft.customFolder) { [weak self] text in
                        self?.draft.customFolder = text.trimmingCharacters(in: .whitespaces)
                        self?.refresh()
                    })
            }
            saving.append(
                choiceRow(
                    MavioStrings.subtitlesHiExtension, subtitle: MavioStrings.subtitlesHiExtensionDetail,
                    options: ["hi", "cc", "sdh"].map { ($0, $0) },
                    selected: draft.hiExtension.isEmpty ? "hi" : draft.hiExtension
                ) { [weak self] ext in
                    self?.draft.hiExtension = ext
                    self?.refresh()
                })
            saving.append(
                toggle(
                    MavioStrings.subtitlesSingleLanguage, \.singleLanguage,
                    subtitle: MavioStrings.subtitlesSingleLanguageDetail))
            saving.append(toggle(MavioStrings.subtitlesUtf, \.utf8))
            let mods: [NSView] = Self.mods.map { mod in
                toggleRow(mod.label, isOn: draft.mods.contains(mod.id)) { [weak self] on in
                    guard let self else { return }
                    if on { self.draft.mods.append(mod.id) } else { self.draft.mods.removeAll { $0 == mod.id } }
                    self.refresh()
                }
            }
            return [
                card(
                    MavioStrings.subtitlesScores, footer: MavioStrings.subtitlesScoresFooter,
                    [
                        number(MavioStrings.subtitlesMinimumScore, \.minimumScore, 0...100),
                        number(MavioStrings.subtitlesMinimumScoreMovie, \.minimumScoreMovie, 0...100),
                    ]),
                card(MavioStrings.subtitlesSearches, searches),
                card(MavioStrings.subtitlesEmbedded, embedded),
                card(MavioStrings.subtitlesUpgrades, upgrades),
                card(MavioStrings.subtitlesSaving, saving),
                card(MavioStrings.subtitlesMods, footer: MavioStrings.subtitlesModsFooter, mods),
                equivalences(),
                card(
                    MavioStrings.subtitlesTags, footer: MavioStrings.subtitlesTagsFooter,
                    [
                        toggle(MavioStrings.subtitlesMovieTags, \.movieProfileTags),
                        toggle(MavioStrings.subtitlesSeriesTags, \.seriesProfileTags),
                        words(
                            MavioStrings.subtitlesNoProfileTags, subtitle: MavioStrings.subtitlesNoProfileTagsDetail,
                            \.noProfileTags),
                    ]),
            ]
        }

        /// The mods a download can have applied, as Bazarr names them.
        @MainActor private static var mods: [(id: String, label: String)] {
            [
                ("OCR_fixes", MavioStrings.subtitlesModOcr),
                ("remove_HI", MavioStrings.subtitlesModRemoveHi),
                ("common", MavioStrings.subtitlesModCommon),
                ("remove_tags", MavioStrings.subtitlesModRemoveTags),
                ("fix_uppercase", MavioStrings.subtitlesModUppercase),
                ("reverse_rtl", MavioStrings.subtitlesModRtl),
                ("emoji", MavioStrings.subtitlesModEmoji),
            ]
        }

        /// The languages whose subtitles count as another's.
        private func equivalences() -> NSView {
            var rows: [NSView] = draft.languageEquals.indices.map { i in
                let arrow = NSTextField.label("→", theme.fonts.body, color: theme.colors.textSecondary)
                let remove = removeButton { [weak self] in
                    self?.draft.languageEquals.remove(at: i)
                    self?.render()
                }
                return row(
                    NSStackView.horizontal(
                        side(i, \.from) + [arrow] + side(i, \.to) + [Spacer(), remove], spacing: MavioSpacing.s8))
            }
            if draft.languageEquals.isEmpty { rows.append(note(MavioStrings.subtitlesNoEquivalences)) }
            return card(
                MavioStrings.subtitlesEquivalences,
                button: (
                    MavioStrings.rulesAdd,
                    { [weak self] in
                        var e = Equivalence()
                        e.from.language = "pt-BR"
                        e.to.language = "pt"
                        self?.draft.languageEquals.append(e)
                        self?.render()
                    }
                ), footer: MavioStrings.subtitlesEquivalencesFooter, rows)
        }

        private func side(_ i: Int, _ which: WritableKeyPath<Equivalence, Spec>) -> [NSView] {
            let spec = draft.languageEquals[i][keyPath: which]
            let languages = SubtitlesNaming.languages(
                known, draft.languageEquals.flatMap { [$0.from.language, $0.to.language] })
            let language = PopUp(
                languages.map(\.name), selected: languages.firstIndex { $0.code == spec.language } ?? 0,
                label: MavioStrings.subtitlesLanguage
            ) { [weak self] k in
                guard let self, i < self.draft.languageEquals.count else { return }
                self.draft.languageEquals[i][keyPath: which].language = languages[k].code
                self.refresh()
            }
            language.widthAnchor.constraint(equalToConstant: 160).isActive = true
            let kinds = [
                (MavioStrings.subtitlesKindNormal, ""), (MavioStrings.subtitlesForced, "forced"),
                (MavioStrings.subtitlesHiShort, "hi"),
            ]
            let current = spec.forced ? "forced" : spec.hearingImpaired ? "hi" : ""
            let kind = PopUp(
                kinds.map(\.0), selected: kinds.firstIndex { $0.1 == current } ?? 0, label: MavioStrings.subtitlesKind
            ) { [weak self] k in
                guard let self, i < self.draft.languageEquals.count else { return }
                self.draft.languageEquals[i][keyPath: which].forced = kinds[k].1 == "forced"
                self.draft.languageEquals[i][keyPath: which].hearingImpaired = kinds[k].1 == "hi"
                self.refresh()
            }
            kind.widthAnchor.constraint(equalToConstant: 112).isActive = true
            return [language, kind]
        }

        // MARK: Synchronization

        private func sync() -> [NSView] {
            let names = providers.map(\.name)
            var rows: [NSView] = [toggle(MavioStrings.subtitlesSyncEnabled, \.sync.enabled)]
            guard draft.sync.enabled else { return [card(footer: MavioStrings.subtitlesSyncFooter, rows)] }
            rows.append(
                toggle(
                    MavioStrings.subtitlesSyncThreshold, \.sync.useThreshold,
                    subtitle: MavioStrings.subtitlesSyncThresholdDetail))
            if draft.sync.useThreshold {
                rows.append(number(MavioStrings.subtitlesThreshold, \.sync.threshold, 0...100))
            }
            rows.append(toggle(MavioStrings.subtitlesSyncMovieThreshold, \.sync.useMovieThreshold))
            if draft.sync.useMovieThreshold {
                rows.append(number(MavioStrings.subtitlesThreshold, \.sync.movieThreshold, 0...100))
            }
            rows.append(
                toggle(
                    MavioStrings.subtitlesForceAudio, \.sync.forceAudio,
                    subtitle: MavioStrings.subtitlesForceAudioDetail))
            rows.append(
                toggle(
                    MavioStrings.subtitlesOriginalLanguage, \.sync.originalLanguage,
                    subtitle: MavioStrings.subtitlesOriginalLanguageDetail))
            rows.append(
                words(
                    MavioStrings.subtitlesExcludedProviders,
                    subtitle: names.isEmpty
                        ? nil : MavioStrings.subtitlesExcludedProvidersDetail(names: names.joined(separator: ", ")),
                    \.sync.excludedProviders))
            rows.append(
                words(
                    MavioStrings.subtitlesExcludedLanguages, subtitle: MavioStrings.subtitlesExcludedLanguagesDetail,
                    \.sync.excludedLanguages))
            return [
                card(footer: MavioStrings.subtitlesSyncFooter, rows),
                card(
                    MavioStrings.subtitlesAlignment,
                    [
                        number(MavioStrings.subtitlesMaxOffset, \.sync.maxOffsetSeconds, 1...Self.most),
                        toggle(
                            MavioStrings.subtitlesFixFramerate, \.sync.fixFramerate,
                            subtitle: MavioStrings.subtitlesFixFramerateDetail),
                        toggle(
                            MavioStrings.subtitlesGoldenSection, \.sync.goldenSection,
                            subtitle: MavioStrings.subtitlesGoldenSectionDetail),
                        decimal(MavioStrings.subtitlesMinScore, \.sync.minScore),
                        number(MavioStrings.subtitlesMaxQualityOffset, \.sync.maxQualityOffsetSeconds),
                        decimal(MavioStrings.subtitlesMaxScaleDeviation, \.sync.maxScaleDeviation),
                    ]),
            ]
        }

        // MARK: Translation

        /// The translators and the AI models a translation can use, as
        /// values and their names.
        private var translatorOptions: [Choice] {
            translators.map { (code: $0.id, name: $0.displayName.isEmpty ? $0.name : $0.displayName) }
                + models.map {
                    (code: "ai/\($0.provider)/\($0.id)", name: "\($0.name.isEmpty ? $0.id : $0.name) (\($0.provider))")
                }
        }

        private func translation() -> [NSView] {
            let options = translatorOptions
            let named = { (id: String) in options.first { $0.code == id }?.name ?? id }
            let t = draft.translation
            let unused = options.filter { $0.code != t.translator && !t.fallbackTranslators.contains($0.code) }
            let placeholders = "{sourceLanguage}, {targetLanguage}, {lineToTranslate}, {contextBefore}, {contextAfter}"
            var translators: [NSView] = [
                choiceRow(
                    MavioStrings.subtitlesTranslator,
                    options: [(MavioStrings.subtitlesTranslatorFirst, "")]
                        + choices(options, t.translator).map { ($0.name, $0.code) },
                    selected: t.translator
                ) { [weak self] id in
                    self?.draft.translation.translator = id
                    self?.draft.translation.fallbackTranslators.removeAll { $0 == id }
                    self?.render()
                }
            ]
            for (i, id) in t.fallbackTranslators.enumerated() {
                translators.append(
                    listedRow(named(id), subtitle: MavioStrings.subtitlesFallbackOf(position: "\(i + 1)")) {
                        [weak self] in
                        self?.draft.translation.fallbackTranslators.removeAll { $0 == id }
                        self?.render()
                    })
            }
            if !unused.isEmpty {
                translators.append(
                    addChoice(MavioStrings.subtitlesAddFallback, unused) { [weak self] id in
                        self?.draft.translation.fallbackTranslators.append(id)
                        self?.render()
                    })
            }
            if let optionsFailure { translators.append(note(optionsFailure, failed: true)) }
            var modelRows: [NSView] = [
                number(MavioStrings.subtitlesContextBefore, \.translation.contextBefore, 0...50),
                number(MavioStrings.subtitlesContextAfter, \.translation.contextAfter, 0...50),
                toggle(
                    MavioStrings.subtitlesTranslatedContext, \.translation.translatedContext,
                    subtitle: MavioStrings.subtitlesTranslatedContextDetail),
                toggle(MavioStrings.subtitlesBatch, \.translation.batch, subtitle: MavioStrings.subtitlesBatchDetail),
            ]
            if t.batch {
                modelRows.append(
                    number(
                        MavioStrings.subtitlesMaxBatchSize, \.translation.maxBatchSize,
                        subtitle: MavioStrings.subtitlesMaxBatchSizeDetail))
            }
            modelRows += [
                toggle(MavioStrings.subtitlesLanguageCodes, \.translation.languageCodes),
                textBlock(MavioStrings.subtitlesPrompt, \.translation.prompt, detail: placeholders, lines: 6),
                textBlock(MavioStrings.subtitlesUserPrompt, \.translation.userPrompt),
                textBlock(
                    MavioStrings.subtitlesProofreadPrompt, \.translation.proofreadPrompt,
                    detail: "{sourceLanguage}, {targetLanguage}, {sourceLine}, {translatedLine}", lines: 6),
                textBlock(MavioStrings.subtitlesProofreadUserPrompt, \.translation.proofreadUserPrompt),
            ]
            return [
                card(MavioStrings.subtitlesTranslators, footer: MavioStrings.subtitlesTranslatorFooter, translators),
                card(
                    MavioStrings.subtitlesOutput,
                    [
                        number(
                            MavioStrings.subtitlesDefaultScore, \.translation.defaultScore, 0...100,
                            subtitle: MavioStrings.subtitlesDefaultScoreDetail),
                        toggle(
                            MavioStrings.subtitlesCredit, \.translation.credit,
                            subtitle: MavioStrings.subtitlesCreditDetail),
                        toggle(
                            MavioStrings.subtitlesStripFormatting, \.translation.stripFormatting,
                            subtitle: MavioStrings.subtitlesStripFormattingDetail),
                        toggle(MavioStrings.subtitlesPreserveLineBreaks, \.translation.preserveLineBreaks),
                        toggle(MavioStrings.subtitlesFixOverlaps, \.translation.fixOverlaps),
                    ]),
                card(MavioStrings.subtitlesModels, footer: MavioStrings.subtitlesModelsFooter, modelRows),
                card(
                    MavioStrings.subtitlesRetries,
                    [
                        number(MavioStrings.subtitlesMaxRetries, \.translation.maxRetries, 1...20),
                        number(MavioStrings.subtitlesRetryDelay, \.translation.retryDelaySeconds, 0...600),
                        number(MavioStrings.subtitlesRetryMultiplier, \.translation.retryMultiplier, 1...10),
                        number(MavioStrings.subtitlesRequestTimeout, \.translation.requestTimeoutSeconds, 1...3600),
                    ]),
                validation(),
                automation(),
            ]
        }

        private func validation() -> NSView {
            var rows: [NSView] = [toggle(MavioStrings.subtitlesValidationEnabled, \.translation.validation.enabled)]
            if draft.translation.validation.enabled {
                rows.append(
                    numberRow(
                        MavioStrings.subtitlesMaxFileSize,
                        value: (Double(draft.translation.validation.maxFileSize) / 1024).rounded(), range: 1...Self.most
                    ) { [weak self] v in
                        self?.draft.translation.validation.maxFileSize = Int64(max(1, v.rounded())) * 1024
                        self?.refresh()
                    })
                rows.append(
                    number(MavioStrings.subtitlesMaxCueLength, \.translation.validation.maxCueLength, 1...Self.most))
                rows.append(number(MavioStrings.subtitlesMinCueLength, \.translation.validation.minCueLength))
                rows.append(number(MavioStrings.subtitlesMinDuration, \.translation.validation.minDurationMs))
                rows.append(
                    number(
                        MavioStrings.subtitlesMaxDuration, \.translation.validation.maxDurationSeconds, 1...Self.most))
            }
            return card(MavioStrings.subtitlesValidation, footer: MavioStrings.subtitlesValidationFooter, rows)
        }

        private func automation() -> NSView {
            let sources = draft.translation.automation.sourceLanguages
            let languages = SubtitlesNaming.languages(known, sources).filter { !sources.contains($0.code) }
            var rows: [NSView] = [toggle(MavioStrings.subtitlesAutomationEnabled, \.translation.automation.enabled)]
            if draft.translation.automation.enabled {
                rows.append(
                    number(MavioStrings.subtitlesAutomationInterval, \.translation.automation.intervalHours, 1...8760))
                for l in sources {
                    rows.append(
                        listedRow(SubtitlesNaming.languageName(l), subtitle: MavioStrings.subtitlesSourceLanguage) {
                            [weak self] in
                            self?.draft.translation.automation.sourceLanguages.removeAll { $0 == l }
                            self?.render()
                        })
                }
                rows.append(
                    addChoice(MavioStrings.subtitlesAddSourceLanguage, languages) { [weak self] code in
                        self?.draft.translation.automation.sourceLanguages.append(code)
                        self?.render()
                    })
                rows.append(number(MavioStrings.subtitlesMaxPerRun, \.translation.automation.maxPerRun, 1...1000))
                rows.append(number(MavioStrings.subtitlesMovieAge, \.translation.automation.movieAgeHours))
                rows.append(number(MavioStrings.subtitlesEpisodeAge, \.translation.automation.episodeAgeHours))
            }
            return card(MavioStrings.subtitlesAutomation, footer: MavioStrings.subtitlesAutomationFooter, rows)
        }

        // MARK: Loading and saving

        private func load() async {
            let model = model
            let server = server
            do {
                let r = try await model.call(
                    server, Procedure.subtitles("GetSubtitleSettings"),
                    Mavio_Acquisition_V1_GetSubtitleSettingsRequest(),
                    as: Mavio_Acquisition_V1_GetSubtitleSettingsResponse.self)
                saved = r.settings
                draft = r.settings
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
            switch section {
            case .searching:
                known = await model.languages(server)
            case .sync:
                providers =
                    (try? await model.call(
                        server, Procedure.subtitles("ListSubtitleProviders"),
                        Mavio_Acquisition_V1_ListSubtitleProvidersRequest(),
                        as: Mavio_Acquisition_V1_ListSubtitleProvidersResponse.self))?.providers ?? []
            case .translation:
                async let languages = model.languages(server)
                await loadTranslators()
                known = await languages
            }
            guard !Task.isCancelled else { return }
            render()
        }

        private func loadTranslators() async {
            optionsFailure = nil
            do {
                translators = try await model.call(
                    server, Procedure.subtitles("ListSubtitleTranslators"),
                    Mavio_Acquisition_V1_ListSubtitleTranslatorsRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleTranslatorsResponse.self
                ).translators
            } catch is CancellationError {
            } catch {
                optionsFailure = error.reason
            }
            do {
                var request = Mavio_System_V1_ListAiModelsRequest()
                request.available = true
                models = try await model.call(
                    server, Procedure.aiProviders("ListAiModels"), request,
                    as: Mavio_System_V1_ListAiModelsResponse.self
                ).models
            } catch CoreError.FailedPrecondition {
                // No AI gateway plugin runs: there are no models.
                models = []
            } catch is CancellationError {
            } catch {
                optionsFailure = optionsFailure ?? error.reason
            }
        }

        private func save() {
            endEditing()
            var request = Mavio_Acquisition_V1_UpdateSubtitleSettingsRequest()
            request.settings = draft
            saving = true
            saveFailure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let r = try await model.call(
                        server, Procedure.subtitles("UpdateSubtitleSettings"), request,
                        as: Mavio_Acquisition_V1_UpdateSubtitleSettingsResponse.self)
                    guard let self else { return }
                    self.saved = r.settings
                    self.draft = r.settings
                    self.notice = MavioStrings.subtitlesSaved
                    self.saving = false
                    self.render()
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }
    }
#endif
