#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// A page of the subtitle settings, as the web UI's.
    enum SubtitlesAdminSettingsPage: CaseIterable {
        /// Searching and saving: scores, intervals, embedded streams, upgrades, where subtitles are saved, the
        /// mods applied, equivalent languages and tags.
        case searching
        /// Synchronization.
        case sync
        /// Translation, with its validation and automation.
        case translation

        var title: String {
            switch self {
            case .searching: MavioStrings.subtitlesSearching
            case .sync: MavioStrings.subtitlesSync
            case .translation: MavioStrings.subtitlesTranslation
            }
        }

        var message: String {
            switch self {
            case .searching: MavioStrings.subtitlesSearchingDetail
            case .sync: MavioStrings.subtitlesSyncDetail
            case .translation: MavioStrings.subtitlesTranslationDetail
            }
        }

        var icon: String {
            switch self {
            case .searching: "search"
            case .sync: "pending"
            case .translation: "subtitles"
            }
        }

        var color: Color {
            switch self {
            case .searching: Color(hex: 0x5856D6)
            case .sync: Color(hex: 0xFF9500)
            case .translation: Color(hex: 0x007AFF)
            }
        }
    }

    /// A page of the subtitle settings (docs/clients.md §6.3.12), as the web UI's: the settings as edited,
    /// saved whole, or the changes cancelled.
    struct SubtitlesAdminSettingsView: View {
        typealias Settings = Mavio_Acquisition_V1_SubtitleSettings

        let server: ServerRecord
        let page: SubtitlesAdminSettingsPage
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var saved: Settings?
        @State private var draft = Settings()
        @State private var failure: String?
        @State private var saveFailure: String?
        @State private var notice: String?
        @State private var saving = false
        @State private var known: [Choice] = []
        @State private var providers: [Mavio_Acquisition_V1_SubtitleProviderState] = []
        @State private var translators: [Mavio_Acquisition_V1_SubtitleTranslatorState] = []
        @State private var models: [Mavio_Plugin_V1_AiModel] = []
        @State private var optionsFailure: String?

        private var changed: Bool { saved.map { $0 != draft } ?? false }

        var body: some View {
            SettingsList(message: page.message) {
                if saved != nil {
                    switch page {
                    case .searching: searching
                    case .sync: sync
                    case .translation: translation
                    }
                } else if failure == nil {
                    ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
                }
                if let failure {
                    Section { Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger) }
                }
                if let saveFailure {
                    Section { Text(saveFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger) }
                } else if let notice, !changed {
                    Section { Text(notice).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary) }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(page.title)
            .inlineTitle()
            .toolbar {
                if saving {
                    ProgressView()
                } else if let saved {
                    if changed {
                        Button(MavioStrings.actionCancel) {
                            draft = saved
                            saveFailure = nil
                        }
                    }
                    Button(MavioStrings.actionSave, action: save).disabled(!changed)
                }
            }
            .task { await load() }
        }

        // MARK: Searching and saving

        @ViewBuilder private var searching: some View {
            let unset = SubtitlesAdmin.languages(known, [draft.defaultAudioLanguage, draft.defaultEmbeddedLanguage])
            Section {
                number(MavioStrings.subtitlesMinimumScore, $draft.minimumScore, 0...100)
                number(MavioStrings.subtitlesMinimumScoreMovie, $draft.minimumScoreMovie, 0...100)
            } header: {
                Text(MavioStrings.subtitlesScores)
            } footer: {
                Text(MavioStrings.subtitlesScoresFooter)
            }
            Section(MavioStrings.subtitlesSearches) {
                number(
                    MavioStrings.subtitlesWantedInterval, $draft.wantedSearchIntervalHours,
                    detail: MavioStrings.subtitlesHoursNever)
                number(
                    MavioStrings.subtitlesWantedIntervalMovie, $draft.wantedSearchIntervalMovieHours,
                    detail: MavioStrings.subtitlesHoursNever)
                toggle(
                    MavioStrings.subtitlesAdaptive, $draft.adaptiveSearching,
                    detail: MavioStrings.subtitlesAdaptiveDetail)
                if draft.adaptiveSearching {
                    number(MavioStrings.subtitlesAdaptiveDelay, $draft.adaptiveDelayDays)
                    number(MavioStrings.subtitlesAdaptiveDelta, $draft.adaptiveDeltaDays)
                }
                toggle(
                    MavioStrings.subtitlesSceneName, $draft.useSceneName, detail: MavioStrings.subtitlesSceneNameDetail)
                toggle(
                    MavioStrings.subtitlesSkipHashing, $draft.skipHashing,
                    detail: MavioStrings.subtitlesSkipHashingDetail)
            }
            Section(MavioStrings.subtitlesEmbedded) {
                toggle(
                    MavioStrings.subtitlesUseEmbedded, $draft.useEmbedded,
                    detail: MavioStrings.subtitlesUseEmbeddedDetail)
                if draft.useEmbedded {
                    toggle(MavioStrings.subtitlesIgnorePgs, $draft.ignorePgs)
                    toggle(MavioStrings.subtitlesIgnoreVobsub, $draft.ignoreVobsub)
                    toggle(MavioStrings.subtitlesIgnoreAss, $draft.ignoreAss)
                }
                languagePicker(
                    MavioStrings.subtitlesDefaultAudio, $draft.defaultAudioLanguage, unset,
                    detail: MavioStrings.subtitlesDefaultAudioDetail)
                languagePicker(MavioStrings.subtitlesDefaultEmbedded, $draft.defaultEmbeddedLanguage, unset)
            }
            Section(MavioStrings.subtitlesUpgrades) {
                toggle(MavioStrings.subtitlesUpgrade, $draft.upgrade, detail: MavioStrings.subtitlesUpgradeDetail)
                if draft.upgrade {
                    number(MavioStrings.subtitlesUpgradeInterval, $draft.upgradeIntervalHours, 1...Self.most)
                    number(MavioStrings.subtitlesUpgradeDays, $draft.upgradeDays, 0...30)
                    toggle(MavioStrings.subtitlesUpgradeManual, $draft.upgradeManual)
                }
            }
            Section(MavioStrings.subtitlesSaving) {
                Picker(
                    MavioStrings.subtitlesFolder,
                    selection: Binding(
                        get: { draft.folder },
                        set: { folder in
                            draft.folder = folder
                            draft.customFolder =
                                folder == .relative ? (draft.customFolder.isEmpty ? "Subs" : draft.customFolder) : ""
                        })
                ) {
                    Text(MavioStrings.subtitlesFolderVideo).tag(Mavio_Acquisition_V1_SubtitleFolder.unspecified)
                    Text(MavioStrings.subtitlesFolderRelative).tag(Mavio_Acquisition_V1_SubtitleFolder.relative)
                    Text(MavioStrings.subtitlesFolderMetadata).tag(Mavio_Acquisition_V1_SubtitleFolder.metadata)
                }
                .listPicker()
                if draft.folder == .relative {
                    TextRow(
                        label: MavioStrings.subtitlesCustomFolder,
                        text: Binding(
                            get: { draft.customFolder },
                            set: { draft.customFolder = $0.trimmingCharacters(in: .whitespaces) }))
                }
                Picker(
                    selection: Binding(
                        get: { draft.hiExtension.isEmpty ? "hi" : draft.hiExtension },
                        set: { draft.hiExtension = $0 })
                ) {
                    ForEach(["hi", "cc", "sdh"], id: \.self) { Text($0).tag($0) }
                } label: {
                    RowLabel(label: MavioStrings.subtitlesHiExtension, detail: MavioStrings.subtitlesHiExtensionDetail)
                }
                .listPicker()
                toggle(
                    MavioStrings.subtitlesSingleLanguage, $draft.singleLanguage,
                    detail: MavioStrings.subtitlesSingleLanguageDetail)
                toggle(MavioStrings.subtitlesUtf, $draft.utf8)
            }
            Section {
                ForEach(Self.mods, id: \.id) { mod in
                    toggle(
                        mod.label,
                        Binding(
                            get: { draft.mods.contains(mod.id) },
                            set: { on in
                                if on { draft.mods.append(mod.id) } else { draft.mods.removeAll { $0 == mod.id } }
                            }))
                }
            } header: {
                Text(MavioStrings.subtitlesMods)
            } footer: {
                Text(MavioStrings.subtitlesModsFooter)
            }
            equivalences
            Section {
                toggle(MavioStrings.subtitlesMovieTags, $draft.movieProfileTags)
                toggle(MavioStrings.subtitlesSeriesTags, $draft.seriesProfileTags)
                SubtitlesAdminWordsRow(
                    label: MavioStrings.subtitlesNoProfileTags, detail: MavioStrings.subtitlesNoProfileTagsDetail,
                    value: $draft.noProfileTags)
            } header: {
                Text(MavioStrings.subtitlesTags)
            } footer: {
                Text(MavioStrings.subtitlesTagsFooter)
            }
        }

        /// The mods a download can have applied, as Bazarr names them.
        private static var mods: [(id: String, label: String)] {
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

        private typealias Spec = Mavio_Acquisition_V1_SubtitleLanguageSpec
        private typealias Equivalence = Mavio_Acquisition_V1_SubtitleLanguageEquivalence

        /// The languages whose subtitles count as another's.
        @ViewBuilder private var equivalences: some View {
            Section {
                ForEach(draft.languageEquals.indices, id: \.self) { i in
                    HStack(spacing: MavioSpacing.s8) {
                        side(i, \.from)
                        Text("→").foregroundStyle(theme.colors.textSecondary)
                        side(i, \.to)
                        Spacer(minLength: 0)
                        removeButton { draft.languageEquals.remove(at: i) }
                    }
                }
                if draft.languageEquals.isEmpty {
                    Text(MavioStrings.subtitlesNoEquivalences).type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                }
            } header: {
                HStack {
                    Text(MavioStrings.subtitlesEquivalences)
                    Spacer()
                    Button(MavioStrings.rulesAdd) {
                        var e = Equivalence()
                        e.from.language = "pt-BR"
                        e.to.language = "pt"
                        draft.languageEquals.append(e)
                    }
                    .buttonStyle(.borderless)
                }
            } footer: {
                Text(MavioStrings.subtitlesEquivalencesFooter)
            }
        }

        private func side(_ i: Int, _ which: WritableKeyPath<Equivalence, Spec>) -> some View {
            let spec = Binding<Spec>(
                get: { i < draft.languageEquals.count ? draft.languageEquals[i][keyPath: which] : Spec() },
                set: { if i < draft.languageEquals.count { draft.languageEquals[i][keyPath: which] = $0 } })
            let languages = SubtitlesAdmin.languages(
                known, draft.languageEquals.flatMap { [$0.from.language, $0.to.language] })
            return HStack(spacing: MavioSpacing.s8) {
                Picker(MavioStrings.subtitlesLanguage, selection: spec.language) {
                    ForEach(languages, id: \.code) { Text($0.name).tag($0.code) }
                }
                .labelsHidden()
                .frame(width: 160)
                Picker(
                    MavioStrings.subtitlesKind,
                    selection: Binding(
                        get: { spec.wrappedValue.forced ? "forced" : spec.wrappedValue.hearingImpaired ? "hi" : "" },
                        set: { kind in
                            spec.wrappedValue.forced = kind == "forced"
                            spec.wrappedValue.hearingImpaired = kind == "hi"
                        })
                ) {
                    Text(MavioStrings.subtitlesKindNormal).tag("")
                    Text(MavioStrings.subtitlesForced).tag("forced")
                    Text(MavioStrings.subtitlesHiShort).tag("hi")
                }
                .labelsHidden()
                .frame(width: 112)
            }
        }

        // MARK: Synchronization

        @ViewBuilder private var sync: some View {
            let names = providers.map(\.name)
            Section {
                toggle(MavioStrings.subtitlesSyncEnabled, $draft.sync.enabled)
                if draft.sync.enabled {
                    toggle(
                        MavioStrings.subtitlesSyncThreshold, $draft.sync.useThreshold,
                        detail: MavioStrings.subtitlesSyncThresholdDetail)
                    if draft.sync.useThreshold {
                        number(MavioStrings.subtitlesThreshold, $draft.sync.threshold, 0...100)
                    }
                    toggle(MavioStrings.subtitlesSyncMovieThreshold, $draft.sync.useMovieThreshold)
                    if draft.sync.useMovieThreshold {
                        number(MavioStrings.subtitlesThreshold, $draft.sync.movieThreshold, 0...100)
                    }
                    toggle(
                        MavioStrings.subtitlesForceAudio, $draft.sync.forceAudio,
                        detail: MavioStrings.subtitlesForceAudioDetail)
                    toggle(
                        MavioStrings.subtitlesOriginalLanguage, $draft.sync.originalLanguage,
                        detail: MavioStrings.subtitlesOriginalLanguageDetail)
                    SubtitlesAdminWordsRow(
                        label: MavioStrings.subtitlesExcludedProviders,
                        detail: names.isEmpty
                            ? nil : MavioStrings.subtitlesExcludedProvidersDetail(names: names.joined(separator: ", ")),
                        value: $draft.sync.excludedProviders)
                    SubtitlesAdminWordsRow(
                        label: MavioStrings.subtitlesExcludedLanguages,
                        detail: MavioStrings.subtitlesExcludedLanguagesDetail, value: $draft.sync.excludedLanguages)
                }
            } footer: {
                Text(MavioStrings.subtitlesSyncFooter)
            }
            if draft.sync.enabled {
                Section(MavioStrings.subtitlesAlignment) {
                    number(MavioStrings.subtitlesMaxOffset, $draft.sync.maxOffsetSeconds, 1...Self.most)
                    toggle(
                        MavioStrings.subtitlesFixFramerate, $draft.sync.fixFramerate,
                        detail: MavioStrings.subtitlesFixFramerateDetail)
                    toggle(
                        MavioStrings.subtitlesGoldenSection, $draft.sync.goldenSection,
                        detail: MavioStrings.subtitlesGoldenSectionDetail)
                    SubtitlesAdminDecimalRow(label: MavioStrings.subtitlesMinScore, value: $draft.sync.minScore)
                    number(MavioStrings.subtitlesMaxQualityOffset, $draft.sync.maxQualityOffsetSeconds)
                    SubtitlesAdminDecimalRow(
                        label: MavioStrings.subtitlesMaxScaleDeviation, value: $draft.sync.maxScaleDeviation)
                }
            }
        }

        // MARK: Translation

        /// The translators and the AI models a translation can use, as values and their names.
        private var translatorOptions: [Choice] {
            translators.map { (code: $0.id, name: $0.displayName.isEmpty ? $0.name : $0.displayName) }
                + models.map {
                    (code: "ai/\($0.provider)/\($0.id)", name: "\($0.name.isEmpty ? $0.id : $0.name) (\($0.provider))")
                }
        }

        @ViewBuilder private var translation: some View {
            let options = translatorOptions
            let named = { (id: String) in options.first { $0.code == id }?.name ?? id }
            let unused = options.filter {
                $0.code != draft.translation.translator && !draft.translation.fallbackTranslators.contains($0.code)
            }
            let placeholders = "{sourceLanguage}, {targetLanguage}, {lineToTranslate}, {contextBefore}, {contextAfter}"
            Section {
                Picker(
                    MavioStrings.subtitlesTranslator,
                    selection: Binding(
                        get: { draft.translation.translator },
                        set: { id in
                            draft.translation.translator = id
                            draft.translation.fallbackTranslators.removeAll { $0 == id }
                        })
                ) {
                    Text(MavioStrings.subtitlesTranslatorFirst).tag("")
                    ForEach(choices(options, draft.translation.translator), id: \.code) { Text($0.name).tag($0.code) }
                }
                .listPicker()
                ForEach(Array(draft.translation.fallbackTranslators.enumerated()), id: \.element) { i, id in
                    FormRow(label: named(id), detail: MavioStrings.subtitlesFallbackOf(position: "\(i + 1)")) {
                        removeButton { draft.translation.fallbackTranslators.removeAll { $0 == id } }
                    }
                }
                if !unused.isEmpty {
                    Picker(
                        MavioStrings.subtitlesAddFallback,
                        selection: Binding(
                            get: { "" },
                            set: { if !$0.isEmpty { draft.translation.fallbackTranslators.append($0) } })
                    ) {
                        Text(MavioStrings.subtitlesChoose).tag("")
                        ForEach(unused, id: \.code) { Text($0.name).tag($0.code) }
                    }
                    .listPicker()
                }
                if let optionsFailure {
                    Text(optionsFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
            } header: {
                Text(MavioStrings.subtitlesTranslators)
            } footer: {
                Text(MavioStrings.subtitlesTranslatorFooter)
            }
            Section(MavioStrings.subtitlesOutput) {
                number(
                    MavioStrings.subtitlesDefaultScore, $draft.translation.defaultScore, 0...100,
                    detail: MavioStrings.subtitlesDefaultScoreDetail)
                toggle(
                    MavioStrings.subtitlesCredit, $draft.translation.credit, detail: MavioStrings.subtitlesCreditDetail)
                toggle(
                    MavioStrings.subtitlesStripFormatting, $draft.translation.stripFormatting,
                    detail: MavioStrings.subtitlesStripFormattingDetail)
                toggle(MavioStrings.subtitlesPreserveLineBreaks, $draft.translation.preserveLineBreaks)
                toggle(MavioStrings.subtitlesFixOverlaps, $draft.translation.fixOverlaps)
            }
            Section {
                number(MavioStrings.subtitlesContextBefore, $draft.translation.contextBefore, 0...50)
                number(MavioStrings.subtitlesContextAfter, $draft.translation.contextAfter, 0...50)
                toggle(
                    MavioStrings.subtitlesTranslatedContext, $draft.translation.translatedContext,
                    detail: MavioStrings.subtitlesTranslatedContextDetail)
                toggle(MavioStrings.subtitlesBatch, $draft.translation.batch, detail: MavioStrings.subtitlesBatchDetail)
                if draft.translation.batch {
                    number(
                        MavioStrings.subtitlesMaxBatchSize, $draft.translation.maxBatchSize,
                        detail: MavioStrings.subtitlesMaxBatchSizeDetail)
                }
                toggle(MavioStrings.subtitlesLanguageCodes, $draft.translation.languageCodes)
                textBlock(MavioStrings.subtitlesPrompt, $draft.translation.prompt, detail: placeholders, lines: 6)
                textBlock(MavioStrings.subtitlesUserPrompt, $draft.translation.userPrompt)
                textBlock(
                    MavioStrings.subtitlesProofreadPrompt, $draft.translation.proofreadPrompt,
                    detail: "{sourceLanguage}, {targetLanguage}, {sourceLine}, {translatedLine}", lines: 6)
                textBlock(MavioStrings.subtitlesProofreadUserPrompt, $draft.translation.proofreadUserPrompt)
            } header: {
                Text(MavioStrings.subtitlesModels)
            } footer: {
                Text(MavioStrings.subtitlesModelsFooter)
            }
            Section(MavioStrings.subtitlesRetries) {
                number(MavioStrings.subtitlesMaxRetries, $draft.translation.maxRetries, 1...20)
                number(MavioStrings.subtitlesRetryDelay, $draft.translation.retryDelaySeconds, 0...600)
                number(MavioStrings.subtitlesRetryMultiplier, $draft.translation.retryMultiplier, 1...10)
                number(MavioStrings.subtitlesRequestTimeout, $draft.translation.requestTimeoutSeconds, 1...3600)
            }
            validation
            automation
        }

        @ViewBuilder private var validation: some View {
            Section {
                toggle(MavioStrings.subtitlesValidationEnabled, $draft.translation.validation.enabled)
                if draft.translation.validation.enabled {
                    NumberRow(
                        label: MavioStrings.subtitlesMaxFileSize,
                        value: Binding(
                            get: { (Double(draft.translation.validation.maxFileSize) / 1024).rounded() },
                            set: { draft.translation.validation.maxFileSize = Int64(max(1, $0.rounded())) * 1024 }),
                        range: 1...Self.most)
                    number(
                        MavioStrings.subtitlesMaxCueLength, $draft.translation.validation.maxCueLength, 1...Self.most)
                    number(MavioStrings.subtitlesMinCueLength, $draft.translation.validation.minCueLength)
                    number(MavioStrings.subtitlesMinDuration, $draft.translation.validation.minDurationMs)
                    number(
                        MavioStrings.subtitlesMaxDuration, $draft.translation.validation.maxDurationSeconds,
                        1...Self.most)
                }
            } header: {
                Text(MavioStrings.subtitlesValidation)
            } footer: {
                Text(MavioStrings.subtitlesValidationFooter)
            }
        }

        @ViewBuilder private var automation: some View {
            let sources = draft.translation.automation.sourceLanguages
            let languages = SubtitlesAdmin.languages(known, sources).filter { !sources.contains($0.code) }
            Section {
                toggle(MavioStrings.subtitlesAutomationEnabled, $draft.translation.automation.enabled)
                if draft.translation.automation.enabled {
                    number(
                        MavioStrings.subtitlesAutomationInterval, $draft.translation.automation.intervalHours, 1...8760)
                    ForEach(sources, id: \.self) { l in
                        FormRow(label: SubtitlesAdmin.languageName(l), detail: MavioStrings.subtitlesSourceLanguage) {
                            removeButton { draft.translation.automation.sourceLanguages.removeAll { $0 == l } }
                        }
                    }
                    Picker(
                        MavioStrings.subtitlesAddSourceLanguage,
                        selection: Binding(
                            get: { "" },
                            set: { if !$0.isEmpty { draft.translation.automation.sourceLanguages.append($0) } })
                    ) {
                        Text(MavioStrings.subtitlesChoose).tag("")
                        ForEach(languages, id: \.code) { Text($0.name).tag($0.code) }
                    }
                    .listPicker()
                    number(MavioStrings.subtitlesMaxPerRun, $draft.translation.automation.maxPerRun, 1...1000)
                    number(MavioStrings.subtitlesMovieAge, $draft.translation.automation.movieAgeHours)
                    number(MavioStrings.subtitlesEpisodeAge, $draft.translation.automation.episodeAgeHours)
                }
            } header: {
                Text(MavioStrings.subtitlesAutomation)
            } footer: {
                Text(MavioStrings.subtitlesAutomationFooter)
            }
        }

        // MARK: Rows

        /// The largest a number without its own limit can be.
        private static let most = Double(Int32.max)

        private func number(
            _ label: String, _ value: Binding<Int32>, _ range: ClosedRange<Double> = 0...most, detail: String? = nil
        ) -> some View {
            NumberRow(label: label, detail: detail, value: value.double, range: range)
        }

        private func toggle(_ label: String, _ on: Binding<Bool>, detail: String? = nil) -> some View {
            Toggle(isOn: on) { RowLabel(label: label, detail: detail) }
        }

        private func languagePicker(
            _ label: String, _ value: Binding<String>, _ languages: [Choice], detail: String? = nil
        ) -> some View {
            Picker(selection: value) {
                Text(MavioStrings.subtitlesLanguageUnset).tag("")
                ForEach(languages, id: \.code) { Text($0.name).tag($0.code) }
            } label: {
                RowLabel(label: label, detail: detail)
            }
            .listPicker()
        }

        /// A block of text, such as a prompt, with its placeholders.
        private func textBlock(_ label: String, _ text: Binding<String>, detail: String? = nil, lines: Int = 4)
            -> some View
        {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                RowLabel(label: label, detail: detail)
                TextEditor(text: text)
                    .font(.system(.body, design: .monospaced))
                    .autocorrectionDisabled()
                    .frame(height: CGFloat(lines) * 18 + 8)
                    .accessibilityLabel(label)
            }
        }

        private func removeButton(_ action: @escaping () -> Void) -> some View {
            Button(action: action) {
                Icon("close", size: 16).foregroundStyle(theme.colors.textSecondary)
            }
            .buttonStyle(.borderless)
            .accessibilityLabel(MavioStrings.rulesRemove)
        }

        // MARK: Loading and saving

        private func load() async {
            if saved == nil {
                do {
                    let r = try await model.call(
                        server, Procedure.subtitles("GetSubtitleSettings"),
                        Mavio_Acquisition_V1_GetSubtitleSettingsRequest(),
                        as: Mavio_Acquisition_V1_GetSubtitleSettingsResponse.self)
                    saved = r.settings
                    draft = r.settings
                    failure = nil
                } catch is CancellationError {
                } catch {
                    failure = error.reason
                }
            }
            switch page {
            case .searching:
                known = await model.localization(server).languages
            case .sync:
                providers =
                    (try? await model.call(
                        server, Procedure.subtitles("ListSubtitleProviders"),
                        Mavio_Acquisition_V1_ListSubtitleProvidersRequest(),
                        as: Mavio_Acquisition_V1_ListSubtitleProvidersResponse.self))?.providers ?? []
            case .translation:
                async let languages = model.localization(server).languages
                await loadTranslators()
                known = await languages
            }
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
            let draft = draft
            saving = true
            saveFailure = nil
            Task {
                defer { saving = false }
                do {
                    var request = Mavio_Acquisition_V1_UpdateSubtitleSettingsRequest()
                    request.settings = draft
                    let r = try await model.call(
                        server, Procedure.subtitles("UpdateSubtitleSettings"), request,
                        as: Mavio_Acquisition_V1_UpdateSubtitleSettingsResponse.self)
                    saved = r.settings
                    self.draft = r.settings
                    notice = MavioStrings.subtitlesSaved
                } catch {
                    saveFailure = error.reason
                }
            }
        }
    }

    /// A row with a list of words, edited as text separated by commas or spaces.
    struct SubtitlesAdminWordsRow: View {
        let label: String
        var detail: String?
        @Binding var value: [String]
        @State private var text = ""

        var body: some View {
            FormRow(label: label, detail: detail) {
                TextField(label, text: $text)
                    .fieldLabelHidden()
                    .autocorrectionDisabled()
                    .multilineTextAlignment(.trailing)
                    .frame(maxWidth: 288)
            }
            .onAppear { text = value.joined(separator: ", ") }
            .onChange(of: text) { value = Self.words(text) }
            .onChange(of: value) { if Self.words(text) != value { text = value.joined(separator: ", ") } }
        }

        private static func words(_ s: String) -> [String] {
            s.split { $0.isWhitespace || $0 == "," }.map(String.init)
        }
    }

    /// A row with a number that has decimals.
    struct SubtitlesAdminDecimalRow: View {
        let label: String
        @Binding var value: Double

        var body: some View {
            FormRow(label: label) {
                TextField(label, value: $value, format: .number.precision(.fractionLength(0...3)).grouping(.never))
                    .fieldLabelHidden()
                    .multilineTextAlignment(.trailing)
                    .frame(width: 80)
            }
        }
    }
#endif
