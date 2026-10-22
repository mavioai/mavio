#if os(macOS)
    import AppKit
    import MavioMac

    /// Acquisition's Requests (docs/clients.md §6.3.11), as the web UI's:
    /// where approved requests go — the library and quality profile of
    /// movies, of series and of anime — what new users may request and
    /// their quotas, and the keywords whose titles are blocklisted, as
    /// Seerr's Radarr and Sonarr servers, user defaults and blocklisted
    /// tags; saved whole, or the changes cancelled.
    final class RequestSettingsPage: FormController {
        private typealias Settings = Mavio_Acquisition_V1_RequestSettings

        private var saved: Settings?
        private var draft = Settings()
        private var failure: String?
        private var saveFailure: String?
        private var notice: String?
        private var saving = false
        private var libraries: [Mavio_Library_V1_Library] = []
        private var profiles: [Mavio_Acquisition_V1_QualityProfile] = []
        /// The first catalog filtering by keyword, whose provider's keywords are blocklisted.
        private var catalog: Mavio_Acquisition_V1_Catalog?
        /// The names of keywords, by their tags.
        private var names: [String: String] = [:]
        /// The keyword search: the text typed, the search running and what it found.
        private var query = ""
        private var searchTask: Task<Void, Never>?
        private var found: [Mavio_Plugin_V1_FilterValue]?
        private weak var results: FlowGrid?
        private weak var resultsNote: NSTextField?
        private var saveBar: SaveBar?
        private var cancelButton: MavioButton?

        override func viewDidLoad() {
            title = MavioStrings.requestsettingsTitle
            message = MavioStrings.requestsettingsMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        isolated deinit { searchTask?.cancel() }

        override func sections() -> [NSView] {
            guard saved != nil else {
                if let failure { return [card([note(failure, failed: true)])] }
                return [card([spinnerRow()])]
            }
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                guard let self, let saved = self.saved else { return }
                self.draft = saved
                self.saveFailure = nil
                self.render()
            }
            cancelButton = cancel
            let bar = SaveBar(MavioStrings.actionSave, theme: theme, extra: [cancel]) { [weak self] in self?.save() }
            saveBar = bar
            refresh()
            return [movies(), series(), anime(), defaults(), tags(), bar]
        }

        private var changed: Bool { saved.map { !acquisitionSame($0, draft) } ?? false }

        /// Shows whether saving and cancelling may be pressed, without laying the form out again.
        private func refresh() {
            let changed = changed
            cancelButton?.isHidden = !changed || saving
            saveBar?.update(enabled: changed, busy: saving, notice: changed ? nil : notice, failure: saveFailure)
        }

        // MARK: Rows

        /// A choice of a library or a profile, none first; one no longer
        /// offered, such as a deleted library, shown by its ID until changed.
        private func choose(
            _ title: String, _ field: WritableKeyPath<Settings, String>, _ options: [Choice], none: String
        ) -> FormRow {
            let all = choices(options, draft[keyPath: field])
            return choiceRow(
                title, options: [(none, "")] + all.map { ($0.name, $0.code) }, selected: draft[keyPath: field]
            ) { [weak self] id in
                self?.draft[keyPath: field] = id
                self?.refresh()
            }
        }

        private func librariesOf(_ kind: Mavio_Library_V1_LibraryKind) -> [Choice] {
            libraries.filter { $0.kind == kind }.map { (code: $0.id, name: $0.name) }
        }

        private func profilesOf(_ kind: Mavio_Acquisition_V1_MediaKind) -> [Choice] {
            profiles.filter { $0.kind == kind }.map { (code: $0.id, name: $0.name) }
        }

        private func toggle(_ title: String, _ field: WritableKeyPath<Settings, Bool>, subtitle: String? = nil)
            -> FormRow
        {
            toggleRow(title, subtitle: subtitle, isOn: draft[keyPath: field]) { [weak self] on in
                self?.draft[keyPath: field] = on
                self?.refresh()
            }
        }

        /// A quota: as many requests every so many days.
        private func quota(
            _ title: String, _ limit: WritableKeyPath<Settings, Int32>, _ days: WritableKeyPath<Settings, Int32>
        ) -> FormRow {
            let count = NumberField(
                MavioStrings.usersQuotaLimit, value: Double(draft[keyPath: limit]), width: 72, theme: theme
            ) { [weak self] v in
                self?.draft[keyPath: limit] = Int32(v.rounded())
                self?.refresh()
            }
            let period = NumberField(
                MavioStrings.usersQuotaDays, value: Double(draft[keyPath: days]), width: 72, theme: theme
            ) { [weak self] v in
                self?.draft[keyPath: days] = Int32(v.rounded())
                self?.refresh()
            }
            let secondary = theme.colors.textSecondary
            let stack = NSStackView.horizontal(
                [
                    count, NSTextField.label(MavioStrings.usersQuotaEvery, theme.fonts.callout, color: secondary),
                    period, NSTextField.label(MavioStrings.usersQuotaDaysUnit, theme.fonts.callout, color: secondary),
                ], spacing: MavioSpacing.s8)
            let row = FormRow(title, subtitle: MavioStrings.requestsettingsQuotaDetail, accessory: stack, theme: theme)
            if theme.isTV { row.onPress = { [weak count] in count?.activate() } }
            return row
        }

        // MARK: Groups

        private func movies() -> NSView {
            let availabilities: [(String, Mavio_Acquisition_V1_MinimumAvailability)] = [
                (MavioStrings.requestsettingsAnnounced, .announced),
                (MavioStrings.requestsettingsInCinemas, .inCinemas),
                (MavioStrings.requestsettingsReleased, .released),
            ]
            let none = MavioStrings.requestsettingsNone
            return card(
                MavioStrings.requestsettingsMovies, footer: MavioStrings.requestsettingsMoviesFooter,
                [
                    choose(MavioStrings.requestsettingsLibrary, \.movieLibraryID, librariesOf(.movies), none: none),
                    choose(
                        MavioStrings.requestsettingsProfile, \.movieQualityProfileID, profilesOf(.movie), none: none),
                    choiceRow(
                        MavioStrings.requestsettingsMinimumAvailability,
                        subtitle: MavioStrings.requestsettingsMinimumAvailabilityDetail, options: availabilities,
                        selected: draft.minimumAvailability == .unspecified ? .released : draft.minimumAvailability
                    ) { [weak self] in
                        self?.draft.minimumAvailability = $0
                        self?.refresh()
                    },
                ])
        }

        private func series() -> NSView {
            let monitors: [(String, Mavio_Acquisition_V1_NewItemMonitor)] = [
                (MavioStrings.requestsettingsMonitorAll, .all), (MavioStrings.requestsettingsMonitorNone, .none),
            ]
            let none = MavioStrings.requestsettingsNone
            return card(
                MavioStrings.requestsettingsSeries, footer: MavioStrings.requestsettingsSeriesFooter,
                [
                    choose(MavioStrings.requestsettingsLibrary, \.seriesLibraryID, librariesOf(.shows), none: none),
                    choose(
                        MavioStrings.requestsettingsProfile, \.seriesQualityProfileID, profilesOf(.series), none: none),
                    toggle(
                        MavioStrings.requestsettingsSeasonFolder, \.seasonFolder,
                        subtitle: MavioStrings.requestsettingsSeasonFolderDetail),
                    choiceRow(
                        MavioStrings.requestsettingsMonitorNew, subtitle: MavioStrings.requestsettingsMonitorNewDetail,
                        options: monitors,
                        selected: draft.monitorNewItems == .unspecified ? .all : draft.monitorNewItems
                    ) { [weak self] in
                        self?.draft.monitorNewItems = $0
                        self?.refresh()
                    },
                    toggle(
                        MavioStrings.requestsettingsSpecials, \.specials,
                        subtitle: MavioStrings.requestsettingsSpecialsDetail),
                ])
        }

        private func anime() -> NSView {
            let same = MavioStrings.requestsettingsSameAsSeries
            return card(
                MavioStrings.requestsettingsAnime, footer: MavioStrings.requestsettingsAnimeFooter,
                [
                    choose(MavioStrings.requestsettingsLibrary, \.animeLibraryID, librariesOf(.shows), none: same),
                    choose(
                        MavioStrings.requestsettingsProfile, \.animeQualityProfileID, profilesOf(.series), none: same),
                ])
        }

        private func defaults() -> NSView {
            card(
                MavioStrings.requestsettingsDefaults, footer: MavioStrings.requestsettingsDefaultsFooter,
                [
                    toggle(MavioStrings.usersRequestMovies, \.defaultRequestMovies),
                    toggle(MavioStrings.usersAutoApproveMovies, \.defaultAutoApproveMovies),
                    toggle(MavioStrings.usersRequestSeries, \.defaultRequestSeries),
                    toggle(MavioStrings.usersAutoApproveSeries, \.defaultAutoApproveSeries),
                    toggle(MavioStrings.usersReportIssues, \.defaultReportIssues),
                    quota(MavioStrings.usersMovieQuota, \.movieQuotaLimit, \.movieQuotaDays),
                    quota(MavioStrings.usersSeriesQuota, \.seriesQuotaLimit, \.seriesQuotaDays),
                ])
        }

        // MARK: Blocklisted tags

        /// The tag prefix of the catalog's provider, "<plugin>/".
        private var prefix: String { "\(catalog?.pluginID ?? "")/" }

        /// A tag's keyword's name, or the tag.
        private func nameOf(_ tag: String) -> String { names[tag] ?? tag }

        /// The blocklisted tags: keywords of a discovery provider, as
        /// "<plugin>/<keyword>", found by name in the first catalog
        /// filtering by keyword, and shown by their names.
        private func tags() -> NSView {
            var rows: [NSView] = []
            if !draft.blocklistedTags.isEmpty {
                let grid = FlowGrid(minimum: 160, gutter: MavioSpacing.s8, rowSpacing: MavioSpacing.s8)
                grid.setItems(
                    draft.blocklistedTags.map { tag in
                        let chip = MavioButton(nameOf(tag), icon: "close", kind: .secondary, theme: theme) {
                            [weak self] in
                            self?.draft.blocklistedTags.removeAll { $0 == tag }
                            self?.render()
                        }
                        chip.setAccessibilityLabel("\(MavioStrings.rulesRemove) \(nameOf(tag))")
                        return chip
                    })
                rows.append(row(grid))
            }
            if catalog != nil {
                let input = TextInput(query, placeholder: MavioStrings.requestsettingsSearchKeywords) {
                    [weak self] text in self?.typed(text)
                }
                let grid = FlowGrid(minimum: 160, gutter: MavioSpacing.s8, rowSpacing: MavioSpacing.s8)
                let empty = NSTextField.label(
                    MavioStrings.requestsettingsNoKeywords, theme.fonts.callout, color: theme.colors.textSecondary)
                results = grid
                resultsNote = empty
                let stack = NSStackView.vertical([input, grid, empty], spacing: MavioSpacing.s8)
                for view in [input, grid] { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                let holder = row(stack)
                showResults()
                rows.append(holder)
            } else {
                rows.append(note(MavioStrings.requestsettingsNoProvider))
            }
            rows.append(
                numberRow(
                    MavioStrings.requestsettingsTagsLimit, subtitle: MavioStrings.requestsettingsTagsLimitDetail,
                    value: Double(draft.blocklistedTagsLimit == 0 ? 50 : draft.blocklistedTagsLimit), range: 1...250
                ) { [weak self] v in
                    self?.draft.blocklistedTagsLimit = Int32(v.rounded())
                    self?.refresh()
                })
            return card(MavioStrings.requestsettingsTags, footer: MavioStrings.requestsettingsTagsFooter, rows)
        }

        /// Searches the keywords typed once typing pauses.
        private func typed(_ text: String) {
            query = text
            searchTask?.cancel()
            let q = text.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty, let catalog else {
                found = nil
                showResults()
                return
            }
            searchTask = Task { [weak self, model, server] in
                try? await Task.sleep(for: .milliseconds(300))
                guard !Task.isCancelled else { return }
                let values = try? await model.filterValues(
                    serverId: server.id, catalog: catalog.id, filter: .keyword,
                    mediaKind: catalog.mediaKinds.first ?? .movie, query: q)
                guard !Task.isCancelled, let self else { return }
                self.found = values ?? []
                self.showResults()
            }
        }

        /// Shows the keywords found, in place, so that the search field keeps the caret.
        private func showResults() {
            let shown = (found ?? []).filter { !draft.blocklistedTags.contains(prefix + $0.id) }.prefix(20)
            results?.setItems(
                shown.map { value in
                    MavioButton(value.name, icon: "add", kind: .secondary, theme: theme) { [weak self] in
                        self?.add(value)
                    }
                })
            results?.isHidden = shown.isEmpty
            resultsNote?.isHidden = !(found?.isEmpty ?? false)
        }

        /// Blocklists a keyword found, clearing the search.
        private func add(_ value: Mavio_Plugin_V1_FilterValue) {
            let tag = prefix + value.id
            names[tag] = value.name
            if !draft.blocklistedTags.contains(tag) { draft.blocklistedTags.append(tag) }
            searchTask?.cancel()
            query = ""
            found = nil
            render()
        }

        /// Names the tags of the catalog's provider.
        private func loadNames() async {
            guard let catalog else { return }
            let ids = draft.blocklistedTags.filter { $0.hasPrefix(prefix) && names[$0] == nil }.map {
                String($0.dropFirst(prefix.count))
            }
            guard !ids.isEmpty else { return }
            var request = Mavio_Acquisition_V1_ListFilterValuesRequest()
            request.catalog = catalog.id
            request.filter = .keyword
            request.mediaKind = catalog.mediaKinds.first ?? .movie
            request.ids = ids
            request.language = discoverLanguage
            guard
                let r = try? await model.call(
                    server, Procedure.discover("ListFilterValues"), request,
                    as: Mavio_Acquisition_V1_ListFilterValuesResponse.self)
            else { return }
            for value in r.values { names[prefix + value.id] = value.name }
        }

        // MARK: Loading and saving

        private func load() async {
            let model = model
            let server = server
            async let libs = try? model.call(
                server, Procedure.libraries("ListLibraries"), Mavio_Library_V1_ListLibrariesRequest(),
                as: Mavio_Library_V1_ListLibrariesResponse.self)
            async let profs = try? model.call(
                server, Procedure.profiles("ListQualityProfiles"), Mavio_Acquisition_V1_ListQualityProfilesRequest(),
                as: Mavio_Acquisition_V1_ListQualityProfilesResponse.self)
            var catalogs = Mavio_Acquisition_V1_ListCatalogsRequest()
            catalogs.language = discoverLanguage
            async let cats = try? model.call(
                server, Procedure.discover("ListCatalogs"), catalogs, as: Mavio_Acquisition_V1_ListCatalogsResponse.self
            )
            do {
                let r = try await model.call(
                    server, Procedure.requests("GetRequestSettings"), Mavio_Acquisition_V1_GetRequestSettingsRequest(),
                    as: Mavio_Acquisition_V1_GetRequestSettingsResponse.self)
                saved = r.settings
                draft = r.settings
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            libraries = await libs?.libraries ?? []
            profiles = await profs?.profiles ?? []
            catalog = await cats?.catalogs.first { $0.filters.contains(.keyword) }
            guard !Task.isCancelled else { return }
            render()
            await loadNames()
            guard !Task.isCancelled else { return }
            render()
        }

        private func save() {
            endEditing()
            var request = Mavio_Acquisition_V1_UpdateRequestSettingsRequest()
            request.settings = draft
            saving = true
            saveFailure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let r = try await model.call(
                        server, Procedure.requests("UpdateRequestSettings"), request,
                        as: Mavio_Acquisition_V1_UpdateRequestSettingsResponse.self)
                    guard let self else { return }
                    self.saved = r.settings
                    self.draft = r.settings
                    self.notice = MavioStrings.acquisitionSaved
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
