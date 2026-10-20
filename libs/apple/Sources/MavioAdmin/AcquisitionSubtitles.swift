#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// Subtitles (docs/clients.md §6.3.12), as the web UI's: the providers and translators the server's plugins
    /// have configured, the subtitle profiles and the libraries they are assigned to, and the pages of the
    /// settings — searching and saving, synchronization and translation.
    struct SubtitlesAdminView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var providers: Mavio_Acquisition_V1_ListSubtitleProvidersResponse?
        @State private var providersFailure: String?
        @State private var translators: Mavio_Acquisition_V1_ListSubtitleTranslatorsResponse?
        @State private var translatorsFailure: String?
        @State private var profiles: [Mavio_Acquisition_V1_SubtitleProfile]?
        @State private var profilesFailure: String?
        @State private var plugins: [Mavio_System_V1_Plugin] = []
        @State private var libraries: [Mavio_Library_V1_Library]?
        @State private var assignments: [String: String] = [:]
        @State private var assignFailure: String?
        @State private var loaded = false

        var body: some View {
            SettingsList(message: MavioStrings.subtitlesMessage) {
                providersSection
                translatorsSection
                profilesSection
                librariesSection
                Section {
                    ForEach(SubtitlesAdminSettingsPage.allCases, id: \.self) { page in
                        NavigationLink {
                            SubtitlesAdminSettingsView(server: server, page: page)
                        } label: {
                            SubtitlesAdminEntry(
                                icon: page.icon, color: page.color, label: page.title, detail: page.message)
                        }
                        .rowLink()
                    }
                } header: {
                    Text(MavioStrings.subtitlesSettings)
                }
            }
            .overlay { if !loaded { ProgressView() } }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsSubtitles)
            .inlineTitle()
            .task { await load() }
        }

        private var throttledColor: Color { Color(hex: 0xFF9500) }

        @ViewBuilder private var providersSection: some View {
            Section {
                ForEach(providers?.providers ?? [], id: \.id) { p in
                    let until = p.hasThrottledUntil ? p.throttledUntil.time : nil
                    let throttled = until.map { $0 > .now } ?? false
                    SubtitlesAdminEntry(
                        icon: "subtitles",
                        color: !p.enabled ? Color(hex: 0x8E8E93) : throttled ? throttledColor : Color(hex: 0x34C759),
                        label: p.displayName.isEmpty ? p.name : p.displayName,
                        detail: providerDetail(p, throttledUntil: throttled ? until : nil))
                }
                failed(providers?.failedPlugins ?? [])
                if let providers, providers.providers.isEmpty, providers.failedPlugins.isEmpty {
                    note(MavioStrings.subtitlesNoProviders)
                }
                if let providersFailure { danger(providersFailure) }
            } header: {
                Text(MavioStrings.subtitlesProviders)
            } footer: {
                Text(MavioStrings.subtitlesProvidersFooter)
            }
        }

        private func providerDetail(
            _ p: Mavio_Acquisition_V1_SubtitleProviderState, throttledUntil until: Date?
        ) -> String {
            let kinds = [
                p.movies ? MavioStrings.subtitlesMovies : "",
                p.episodes ? MavioStrings.subtitlesEpisodes : "",
            ]
            let throttled = until.map {
                MavioStrings.subtitlesThrottled(when: fromNow($0), reason: p.throttleReason)
            }
            return [
                pluginName(p.pluginID),
                kinds.filter { !$0.isEmpty }.joined(separator: " + "),
                p.enabled ? "" : MavioStrings.acquisitionOff,
                throttled ?? "",
            ].filter { !$0.isEmpty }.joined(separator: " · ")
        }

        @ViewBuilder private var translatorsSection: some View {
            Section {
                ForEach(translators?.translators ?? [], id: \.id) { x in
                    SubtitlesAdminEntry(
                        icon: "subtitles",
                        color: x.enabled ? Color(hex: 0x007AFF) : Color(hex: 0x8E8E93),
                        label: x.displayName.isEmpty ? x.name : x.displayName,
                        detail: [pluginName(x.pluginID), x.enabled ? "" : MavioStrings.acquisitionOff]
                            .filter { !$0.isEmpty }.joined(separator: " · "))
                }
                failed(translators?.failedPlugins ?? [])
                if let translators, translators.translators.isEmpty, translators.failedPlugins.isEmpty {
                    note(MavioStrings.subtitlesNoTranslators)
                }
                if let translatorsFailure { danger(translatorsFailure) }
            } header: {
                Text(MavioStrings.subtitlesTranslators)
            } footer: {
                Text(MavioStrings.subtitlesTranslatorsFooter)
            }
        }

        private var sorted: [Mavio_Acquisition_V1_SubtitleProfile] {
            (profiles ?? []).sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }

        @ViewBuilder private var profilesSection: some View {
            Section {
                ForEach(sorted, id: \.id) { p in
                    NavigationLink {
                        SubtitlesAdminProfileView(server: server, profile: p) { await load() }
                    } label: {
                        RowLabel(
                            label: p.name, detail: p.items.map(SubtitlesAdmin.itemName).joined(separator: ", "))
                    }
                    .rowLink()
                }
                NavigationLink {
                    SubtitlesAdminProfileView(server: server, profile: nil) { await load() }
                } label: {
                    Text(MavioStrings.subtitlesAddProfile).foregroundStyle(theme.colors.accentText)
                }
                .rowLink()
                if let profilesFailure { danger(profilesFailure) }
            } header: {
                Text(MavioStrings.subtitlesProfiles)
            } footer: {
                Text(MavioStrings.subtitlesProfilesFooter)
            }
        }

        @ViewBuilder private var librariesSection: some View {
            Section {
                ForEach(libraries ?? [], id: \.id) { l in
                    Picker(
                        l.name,
                        selection: Binding(
                            get: { assignments[l.id] ?? "" },
                            set: { assign(library: l.id, profile: $0) })
                    ) {
                        Text(MavioStrings.subtitlesNoProfile).tag("")
                        ForEach(sorted, id: \.id) { Text($0.name).tag($0.id) }
                    }
                    .listPicker()
                }
                if let libraries, libraries.isEmpty { note(MavioStrings.subtitlesNoLibraries) }
                if let assignFailure { danger(assignFailure) }
            } header: {
                Text(MavioStrings.subtitlesLibraries)
            } footer: {
                Text(MavioStrings.subtitlesLibrariesFooter)
            }
        }

        @ViewBuilder private func failed(_ ids: [String]) -> some View {
            ForEach(ids, id: \.self) { id in
                danger(MavioStrings.subtitlesPluginFailed(plugin: pluginName(id)))
            }
        }

        private func note(_ text: String) -> some View {
            Text(text).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
        }

        private func danger(_ text: String) -> some View {
            Text(text).type(theme.type.callout).foregroundStyle(theme.colors.danger)
        }

        private func pluginName(_ id: String) -> String {
            let name = plugins.first { $0.manifest.id == id }?.manifest.name ?? ""
            return name.isEmpty ? id : name
        }

        private func load() async {
            async let providers = attempt {
                try await model.call(
                    server, Procedure.subtitles("ListSubtitleProviders"),
                    Mavio_Acquisition_V1_ListSubtitleProvidersRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleProvidersResponse.self)
            }
            async let translators = attempt {
                try await model.call(
                    server, Procedure.subtitles("ListSubtitleTranslators"),
                    Mavio_Acquisition_V1_ListSubtitleTranslatorsRequest(),
                    as: Mavio_Acquisition_V1_ListSubtitleTranslatorsResponse.self)
            }
            async let profiles = attempt {
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
            async let assigned = loadAssignments()
            (self.providers, providersFailure) = await providers
            (self.translators, translatorsFailure) = await translators
            (self.profiles, profilesFailure) = await profiles
            self.plugins = (await plugins)?.plugins ?? []
            self.libraries = (await libraries)?.libraries
            await assigned
            loaded = true
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
            Task {
                do {
                    var request = Mavio_Acquisition_V1_AssignSubtitleProfileRequest()
                    request.targetID = library
                    request.profileID = profile
                    _ = try await model.call(
                        server, Procedure.subtitles("AssignSubtitleProfile"), request,
                        as: Mavio_Acquisition_V1_AssignSubtitleProfileResponse.self)
                    await loadAssignments()
                } catch {
                    await loadAssignments()
                    assignFailure = error.reason
                }
            }
        }

        /// What a call returned, or what failed, as a pair the page keeps.
        private func attempt<T>(_ call: () async throws -> T) async -> (T?, String?) {
            do {
                return (try await call(), nil)
            } catch {
                return (nil, error is CancellationError ? nil : error.reason)
            }
        }
    }

    /// A row of the section, as the web UI's links: an icon on a colored tile, its label and what it tells.
    struct SubtitlesAdminEntry: View {
        let icon: String
        let color: Color
        let label: String
        let detail: String
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                ColorTile(icon: icon, color: color)
                VStack(alignment: .leading, spacing: 2) {
                    Text(label).lineLimit(1)
                    if !detail.isEmpty {
                        Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
            }
        }
    }

    /// What the pages of Subtitles share: the languages subtitles can have, and how they are named.
    enum SubtitlesAdmin {
        /// Languages a subtitle can have beyond the server's, which name only ISO 639-1 codes.
        private static let extraLanguages = ["pt-BR", "zh-Hans", "zh-Hant"]

        /// The cutoff that ends the search once any language has subtitles.
        static let cutoffAny: Int32 = 65535

        /// The choices of subtitle languages: the server's, named in the user's language, with those it lacks
        /// and any of values, by name.
        static func languages(_ known: [Choice], _ values: [String]) -> [Choice] {
            var seen = Set(known.map(\.code))
            var all = known
            for code in extraLanguages + values where !code.isEmpty && seen.insert(code).inserted {
                all.append((code: code, name: languageName(code)))
            }
            return all.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }

        /// A language's name in the user's language, or its tag.
        static func languageName(_ tag: String) -> String {
            Locale.current.localizedString(forIdentifier: tag) ?? tag
        }

        /// A profile's language, as "French (forced)".
        static func itemName(_ item: Mavio_Acquisition_V1_SubtitleProfileItem) -> String {
            let kinds = [
                item.forced ? MavioStrings.subtitlesForced : "",
                item.hearingImpaired == .required ? MavioStrings.subtitlesHiShort : "",
                item.hearingImpaired == .excluded ? MavioStrings.subtitlesNoHiShort : "",
            ].filter { !$0.isEmpty }
            let name = languageName(item.language)
            return kinds.isEmpty ? name : "\(name) (\(kinds.joined(separator: ", ")))"
        }
    }
#endif
