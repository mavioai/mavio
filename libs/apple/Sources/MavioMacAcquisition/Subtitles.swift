#if os(macOS)
    import AppKit
    import MavioMac

    /// A page of the subtitle settings, as the web UI's.
    enum SubtitleSettingsSection: CaseIterable {
        /// Searching and saving: scores, intervals, embedded streams,
        /// upgrades, where subtitles are saved, the mods applied, equivalent
        /// languages and tags.
        case searching
        /// Synchronization.
        case sync
        /// Translation, with its validation and automation.
        case translation

        /// The page's title.
        @MainActor var title: String {
            switch self {
            case .searching: MavioStrings.subtitlesSearching
            case .sync: MavioStrings.subtitlesSync
            case .translation: MavioStrings.subtitlesTranslation
            }
        }

        /// What the page is for.
        @MainActor var message: String {
            switch self {
            case .searching: MavioStrings.subtitlesSearchingDetail
            case .sync: MavioStrings.subtitlesSyncDetail
            case .translation: MavioStrings.subtitlesTranslationDetail
            }
        }

        /// The icon of its row.
        var icon: String {
            switch self {
            case .searching: "search"
            case .sync: "pending"
            case .translation: "subtitles"
            }
        }

        /// The color of its row's tile.
        var tint: UInt32 {
            switch self {
            case .searching: 0x5856D6
            case .sync: 0xFF9500
            case .translation: 0x007AFF
            }
        }
    }

    /// Subtitles (docs/clients.md §6.3.12), as the web UI's: the providers
    /// and translators the server's plugins have configured, the subtitle
    /// profiles and the libraries they are assigned to, and the pages of
    /// the settings — searching and saving, synchronization and translation.
    final class SubtitlesPage: FormController {
        private var providers: Mavio_Acquisition_V1_ListSubtitleProvidersResponse?
        private var providersFailure: String?
        private var translators: Mavio_Acquisition_V1_ListSubtitleTranslatorsResponse?
        private var translatorsFailure: String?
        private var profiles: [Mavio_Acquisition_V1_SubtitleProfile]?
        private var profilesFailure: String?
        private var plugins: [Mavio_System_V1_Plugin] = []
        private var libraries: [Mavio_Library_V1_Library]?
        private var assignments: [String: String] = [:]
        private var assignFailure: String?
        private var loaded = false

        override func viewDidLoad() {
            title = MavioStrings.settingsSubtitles
            message = MavioStrings.subtitlesMessage
            super.viewDidLoad()
        }

        override func viewWillAppear() {
            super.viewWillAppear()
            // Coming back from a profile, its changes show.
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var views: [NSView] = [providersSection(), translatorsSection(), profilesSection(), librariesSection()]
            let server = server
            let context = context
            views.append(
                card(
                    MavioStrings.subtitlesSettings,
                    SubtitleSettingsSection.allCases.map { section in
                        tileRow(section.title, subtitle: section.message, icon: section.icon, tint: section.tint) {
                            [weak self] in
                            self?.push(SubtitleSettingsPage(context: context, server: server, section: section))
                        }
                    }))
            return views
        }

        private func providersSection() -> NSView {
            var rows: [NSView] = (providers?.providers ?? []).map { p in
                let until = p.hasThrottledUntil ? p.throttledUntil.time : nil
                let throttled = until.map { $0 > .now } ?? false
                return tileRow(
                    p.displayName.isEmpty ? p.name : p.displayName,
                    subtitle: providerDetail(p, throttledUntil: throttled ? until : nil), icon: "subtitles",
                    tint: !p.enabled ? 0x8E8E93 : throttled ? 0xFF9500 : 0x34C759)
            }
            rows += failed(providers?.failedPlugins ?? [])
            if let providers, providers.providers.isEmpty, providers.failedPlugins.isEmpty {
                rows.append(note(MavioStrings.subtitlesNoProviders))
            }
            if let providersFailure { rows.append(note(providersFailure, failed: true)) }
            if !loaded { rows.append(spinnerRow()) }
            return card(MavioStrings.subtitlesProviders, footer: MavioStrings.subtitlesProvidersFooter, rows)
        }

        private func providerDetail(
            _ p: Mavio_Acquisition_V1_SubtitleProviderState, throttledUntil until: Date?
        ) -> String {
            let kinds = [
                p.movies ? MavioStrings.subtitlesMovies : "", p.episodes ? MavioStrings.subtitlesEpisodes : "",
            ]
            return dotted([
                pluginName(p.pluginID),
                kinds.filter { !$0.isEmpty }.joined(separator: " + "),
                p.enabled ? nil : MavioStrings.acquisitionOff,
                until.map { MavioStrings.subtitlesThrottled(when: fromNow($0), reason: p.throttleReason) },
            ])
        }

        private func translatorsSection() -> NSView {
            var rows: [NSView] = (translators?.translators ?? []).map { x in
                tileRow(
                    x.displayName.isEmpty ? x.name : x.displayName,
                    subtitle: dotted([pluginName(x.pluginID), x.enabled ? nil : MavioStrings.acquisitionOff]),
                    icon: "subtitles", tint: x.enabled ? 0x007AFF : 0x8E8E93)
            }
            rows += failed(translators?.failedPlugins ?? [])
            if let translators, translators.translators.isEmpty, translators.failedPlugins.isEmpty {
                rows.append(note(MavioStrings.subtitlesNoTranslators))
            }
            if let translatorsFailure { rows.append(note(translatorsFailure, failed: true)) }
            return card(MavioStrings.subtitlesTranslators, footer: MavioStrings.subtitlesTranslatorsFooter, rows)
        }

        private var sorted: [Mavio_Acquisition_V1_SubtitleProfile] {
            (profiles ?? []).sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }

        private func profilesSection() -> NSView {
            var rows: [NSView] = sorted.map { p in
                FormRow.link(
                    p.name, subtitle: p.items.map(SubtitlesNaming.itemName).joined(separator: ", "), theme: theme
                ) { [weak self] in self?.openProfile(p) }
            }
            rows.append(addRow(MavioStrings.subtitlesAddProfile) { [weak self] in self?.openProfile(nil) })
            if let profilesFailure { rows.append(note(profilesFailure, failed: true)) }
            return card(MavioStrings.subtitlesProfiles, footer: MavioStrings.subtitlesProfilesFooter, rows)
        }

        private func openProfile(_ p: Mavio_Acquisition_V1_SubtitleProfile?) {
            push(SubtitleProfilePage(context: context, server: server, profile: p))
        }

        private func librariesSection() -> NSView {
            let options = [(MavioStrings.subtitlesNoProfile, "")] + sorted.map { ($0.name, $0.id) }
            var rows: [NSView] = (libraries ?? []).map { l in
                let id = l.id
                var choices = options
                let chosen = assignments[id] ?? ""
                if !choices.contains(where: { $0.1 == chosen }) { choices.append((chosen, chosen)) }
                return choiceRow(l.name, options: choices, selected: chosen) { [weak self] profile in
                    self?.assign(library: id, profile: profile)
                }
            }
            if let libraries, libraries.isEmpty { rows.append(note(MavioStrings.subtitlesNoLibraries)) }
            if let assignFailure { rows.append(note(assignFailure, failed: true)) }
            return card(MavioStrings.subtitlesLibraries, footer: MavioStrings.subtitlesLibrariesFooter, rows)
        }

        private func failed(_ ids: [String]) -> [NSView] {
            ids.map { note(MavioStrings.subtitlesPluginFailed(plugin: pluginName($0)), failed: true) }
        }

        private func pluginName(_ id: String) -> String {
            let name = plugins.first { $0.manifest.id == id }?.manifest.name ?? ""
            return name.isEmpty ? id : name
        }

        private func load() async {
            let model = model
            let server = server
            async let providers = Result {
                try await model.call(
                    server, Procedure.subtitles("ListSubtitleProviders"),
                    Mavio_Acquisition_V1_ListSubtitleProvidersRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleProvidersResponse.self)
            }
            async let translators = Result {
                try await model.call(
                    server, Procedure.subtitles("ListSubtitleTranslators"),
                    Mavio_Acquisition_V1_ListSubtitleTranslatorsRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleTranslatorsResponse.self)
            }
            async let profiles = Result {
                try await model.call(
                    server, Procedure.subtitles("ListSubtitleProfiles"),
                    Mavio_Acquisition_V1_ListSubtitleProfilesRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleProfilesResponse.self
                ).profiles
            }
            async let plugins = try? model.call(
                server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
                as: Mavio_System_V1_ListPluginsResponse.self)
            async let libraries = try? model.call(
                server, Procedure.libraries("ListLibraries"), Mavio_Library_V1_ListLibrariesRequest(),
                as: Mavio_Library_V1_ListLibrariesResponse.self)
            let (rp, rt, rf, pl, li) = await (providers, translators, profiles, plugins, libraries)
            guard !Task.isCancelled else { return }
            (self.providers, providersFailure) = split(rp, self.providers)
            (self.translators, translatorsFailure) = split(rt, self.translators)
            (self.profiles, profilesFailure) = split(rf, self.profiles)
            self.plugins = pl?.plugins ?? []
            self.libraries = li?.libraries
            await loadAssignments()
            loaded = true
            render()
        }

        /// What a call returned, or the value kept and what failed.
        private func split<T>(_ r: Result<T, any Error>, _ kept: T?) -> (T?, String?) {
            switch r {
            case .success(let v): (v, nil)
            case .failure(let e): (kept, e is CancellationError ? nil : e.reason)
            }
        }

        private func loadAssignments() async {
            do {
                let r = try await model.call(
                    server, Procedure.subtitles("ListSubtitleProfileAssignments"),
                    Mavio_Acquisition_V1_ListSubtitleProfileAssignmentsRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleProfileAssignmentsResponse.self)
                assignments = Dictionary(r.assignments.map { ($0.targetID, $0.profileID) }) { _, last in last }
                assignFailure = nil
            } catch is CancellationError {
            } catch {
                assignFailure = error.reason
            }
        }

        private func assign(library: String, profile: String) {
            assignments[library] = profile
            var request = Mavio_Acquisition_V1_AssignSubtitleProfileRequest()
            request.targetID = library
            request.profileID = profile
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.subtitles("AssignSubtitleProfile"), request,
                        as: Mavio_Acquisition_V1_AssignSubtitleProfileResponse.self)
                    await self?.loadAssignments()
                } catch {
                    await self?.loadAssignments()
                    self?.assignFailure = error.reason
                }
                self?.render()
            }
        }
    }

    // MARK: Profile

    /// A subtitle profile's page, as the web UI's, or a new one's: its name,
    /// its languages, the cutoff, the original format and the tag, and what
    /// release names must and must not contain; saved, or removed.
    final class SubtitleProfilePage: FormController {
        private typealias Profile = Mavio_Acquisition_V1_SubtitleProfile
        private typealias Item = Mavio_Acquisition_V1_SubtitleProfileItem
        private typealias HearingImpaired = Mavio_Acquisition_V1_SubtitleHearingImpaired

        private var profile: Profile
        private var draft: Profile
        private var mustContain: String
        private var mustNotContain: String
        private var known: [Choice] = []
        private var saving = false
        private var failure: String?
        private var notice: String?
        private var saveBar: SaveBar?

        /// The page of profile, or of a new one when it is nil.
        init(context: MacContext, server: ServerRecord, profile: Mavio_Acquisition_V1_SubtitleProfile?) {
            let p =
                profile
                ?? {
                    var p = Profile()
                    var item = Item()
                    item.id = 1
                    item.language = "en"
                    p.items = [item]
                    return p
                }()
            self.profile = p
            draft = p
            mustContain = p.mustContain.joined(separator: "\n")
            mustNotContain = p.mustNotContain.joined(separator: "\n")
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            retitle()
            super.viewDidLoad()
            run { [weak self, model, server] in
                let known = await model.languages(server)
                self?.known = known
                self?.render()
            }
        }

        private func retitle() {
            title = profile.name.isEmpty ? MavioStrings.subtitlesNewProfile : profile.name
        }

        private var isNew: Bool { profile.id.isEmpty }

        private var result: Profile {
            var r = draft
            r.name = draft.name.trimmingCharacters(in: .whitespaces)
            r.tag = draft.tag.trimmingCharacters(in: .whitespaces)
            r.mustContain = Self.lines(mustContain)
            r.mustNotContain = Self.lines(mustNotContain)
            return r
        }

        private var languages: [Choice] { SubtitlesNaming.languages(known, draft.items.map(\.language)) }

        override func sections() -> [NSView] {
            let items: [NSView] = draft.items.map { itemRow($0) }
            var cutoffs: [(String, Int32)] = [
                (MavioStrings.subtitlesCutoffNone, 0), (MavioStrings.subtitlesCutoffAny, SubtitlesNaming.cutoffAny),
            ]
            cutoffs += draft.items.map { (SubtitlesNaming.itemName($0), $0.id) }
            if !cutoffs.contains(where: { $0.1 == draft.cutoff }) {
                cutoffs.append((String(draft.cutoff), draft.cutoff))
            }
            var extra: [NSView] = []
            if !isNew {
                let remove = MavioButton(MavioStrings.subtitlesRemove, kind: .destructive, theme: theme) {
                    [weak self] in
                    guard let self else { return }
                    self.confirm(
                        MavioStrings.subtitlesRemoveTitle(name: self.profile.name),
                        message: MavioStrings.subtitlesRemoveMessage, action: MavioStrings.subtitlesRemove
                    ) { [weak self] in self?.remove() }
                }
                remove.isEnabled = !saving
                extra.append(remove)
            }
            let bar = SaveBar(MavioStrings.actionSave, theme: theme, extra: extra) { [weak self] in self?.save() }
            saveBar = bar
            refresh()
            return [
                card([
                    fieldRow(MavioStrings.rulesName, text: draft.name) { [weak self] text in
                        self?.draft.name = text
                        self?.edited()
                    }
                ]),
                card(
                    MavioStrings.subtitlesLanguages,
                    button: (MavioStrings.rulesAdd, { [weak self] in self?.addItem() }),
                    footer: MavioStrings.subtitlesLanguagesFooter, items),
                card([
                    choiceRow(
                        MavioStrings.subtitlesCutoff, subtitle: MavioStrings.subtitlesCutoffDetail, options: cutoffs,
                        selected: draft.cutoff
                    ) { [weak self] value in
                        self?.draft.cutoff = value
                        self?.edited()
                    },
                    toggleRow(
                        MavioStrings.subtitlesOriginalFormat, subtitle: MavioStrings.subtitlesOriginalFormatDetail,
                        isOn: draft.originalFormat
                    ) { [weak self] on in
                        self?.draft.originalFormat = on
                        self?.edited()
                    },
                    fieldRow(MavioStrings.subtitlesTag, subtitle: MavioStrings.subtitlesTagDetail, text: draft.tag) {
                        [weak self] text in
                        self?.draft.tag = text
                        self?.edited()
                    },
                ]),
                card(
                    MavioStrings.subtitlesReleaseNames, footer: MavioStrings.subtitlesReleaseNamesFooter,
                    [
                        textBlock(MavioStrings.subtitlesMustContain, mustContain) { [weak self] text in
                            self?.mustContain = text
                            self?.edited()
                        },
                        textBlock(MavioStrings.subtitlesMustNotContain, mustNotContain) { [weak self] text in
                            self?.mustNotContain = text
                            self?.edited()
                        },
                    ]),
                bar,
            ]
        }

        /// A language of the profile: the language and hearing impaired
        /// menus, removing it, and how it counts.
        private func itemRow(_ item: Item) -> NSView {
            let id = item.id
            let languages = languages
            let language = PopUp(
                languages.map(\.name), selected: languages.firstIndex { $0.code == item.language } ?? 0,
                label: MavioStrings.subtitlesLanguage
            ) { [weak self] i in self?.change(id, render: true) { $0.language = languages[i].code } }
            language.widthAnchor.constraint(equalToConstant: 192).isActive = true
            let hiOptions: [(String, HearingImpaired)] = [
                (MavioStrings.subtitlesHiAllowed, .unspecified), (MavioStrings.subtitlesHiRequired, .required),
                (MavioStrings.subtitlesHiExcluded, .excluded),
            ]
            let hi = PopUp(
                hiOptions.map(\.0), selected: hiOptions.firstIndex { $0.1 == item.hearingImpaired } ?? 0,
                label: MavioStrings.subtitlesHearingImpaired
            ) { [weak self] i in self?.change(id, render: true) { $0.hearingImpaired = hiOptions[i].1 } }
            hi.widthAnchor.constraint(equalToConstant: 192).isActive = true
            let remove = removeButton(nil)
            if draft.items.count > 1 { remove.onPress = { [weak self] in self?.removeItem(id) } }
            let checks: [NSView] = [
                Check(MavioStrings.subtitlesForcedOnly, isOn: item.forced, theme: theme) { [weak self] on in
                    self?.change(id, render: true) { $0.forced = on }
                },
                Check(MavioStrings.subtitlesAudioExclude, isOn: item.audioExclude, theme: theme) { [weak self] on in
                    self?.change(id, render: true) {
                        $0.audioExclude = on
                        if on { $0.audioOnlyInclude = false }
                    }
                },
                Check(MavioStrings.subtitlesAudioOnlyInclude, isOn: item.audioOnlyInclude, theme: theme) {
                    [weak self] on in
                    self?.change(id, render: true) {
                        $0.audioOnlyInclude = on
                        if on { $0.audioExclude = false }
                    }
                },
                Spacer(),
            ]
            let top = NSStackView.horizontal([language, hi, Spacer(), remove], spacing: MavioSpacing.s8)
            let bottom = NSStackView.horizontal(checks, spacing: MavioSpacing.s16)
            let stack = NSStackView.vertical([top, bottom], spacing: MavioSpacing.s8)
            for view in [top, bottom] { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return row(stack)
        }

        /// A block of text, one term a line.
        private func textBlock(_ title: String, _ text: String, onChange: @escaping @MainActor (String) -> Void)
            -> NSView
        {
            let block = TextBlock(text, label: title, lines: 2, theme: theme, onChange: onChange)
            let stack = NSStackView.vertical(
                [NSTextField.label(title, theme.fonts.body, color: theme.colors.textPrimary), block],
                spacing: MavioSpacing.s8)
            block.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return row(stack)
        }

        private func change(_ id: Int32, render again: Bool, _ body: (inout Item) -> Void) {
            guard let i = draft.items.firstIndex(where: { $0.id == id }) else { return }
            body(&draft.items[i])
            if again { render() } else { edited() }
        }

        private func edited() {
            notice = nil
            refresh()
        }

        private func refresh() {
            let r = result
            let changed = isNew || r != profile
            let valid = !r.name.isEmpty && !r.items.isEmpty && !r.tag.contains { $0.isWhitespace }
            saveBar?.update(
                enabled: changed && valid, busy: saving, notice: changed ? nil : notice, failure: failure)
        }

        private func addItem() {
            let next = (draft.items.map(\.id).max() ?? 0) + 1
            let used = Set(draft.items.map(\.language))
            let own = Locale.current.language.languageCode?.identifier ?? "en"
            var item = Item()
            item.id = next
            item.language = ([own, "en"] + languages.map(\.code)).first { !used.contains($0) } ?? "en"
            draft.items.append(item)
            render()
        }

        private func removeItem(_ id: Int32) {
            draft.items.removeAll { $0.id == id }
            if draft.cutoff == id { draft.cutoff = 0 }
            render()
        }

        private func save() {
            endEditing()
            let result = result
            let isNew = isNew
            saving = true
            failure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let saved: Profile
                    if isNew {
                        var request = Mavio_Acquisition_V1_CreateSubtitleProfileRequest()
                        request.profile = result
                        saved = try await model.call(
                            server, Procedure.subtitles("CreateSubtitleProfile"), request,
                            as: Mavio_Acquisition_V1_CreateSubtitleProfileResponse.self
                        ).profile
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateSubtitleProfileRequest()
                        request.profile = result
                        saved = try await model.call(
                            server, Procedure.subtitles("UpdateSubtitleProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateSubtitleProfileResponse.self
                        ).profile
                    }
                    guard let self else { return }
                    self.profile = saved
                    self.draft = saved
                    self.mustContain = saved.mustContain.joined(separator: "\n")
                    self.mustNotContain = saved.mustNotContain.joined(separator: "\n")
                    self.notice = MavioStrings.acquisitionSaved
                    self.saving = false
                    self.retitle()
                    self.render()
                } catch {
                    self?.saving = false
                    self?.failure = error.reason
                    self?.refresh()
                }
            }
        }

        private func remove() {
            saving = true
            failure = nil
            render()
            var request = Mavio_Acquisition_V1_DeleteSubtitleProfileRequest()
            request.id = profile.id
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.subtitles("DeleteSubtitleProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteSubtitleProfileResponse.self)
                    self?.saving = false
                    self?.dismiss()
                } catch {
                    self?.saving = false
                    self?.failure = error.reason
                    self?.render()
                }
            }
        }

        private static func lines(_ s: String) -> [String] {
            s.split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        }
    }
#endif
