#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    // The settings' Plugins (docs/clients.md §6.3.3), as the web UI's: the
    // plugins of the server chosen, as cards in two tabs, Available, what its
    // catalogs offer, installed or not, and Installed, filtered by category;
    // a plugin's page, to install, update, enable, disable and uninstall it,
    // with its configuration, containers, network, information, logs and
    // versions; and the catalogs, to add and remove.

    /// A plugin as shown: what its catalog offers of it and what is
    /// installed, either or both.
    struct PluginsEntry: Identifiable {
        let id: String
        let offered: Mavio_System_V1_CatalogPlugin?
        let installed: Mavio_System_V1_Plugin?

        private var manifest: Mavio_Plugin_V1_Manifest? { installed?.manifest }

        private static func first(_ values: String?...) -> String {
            values.compactMap(\.self).first { !$0.isEmpty } ?? ""
        }

        var name: String { Self.first(manifest?.name, offered?.name, id) }
        var description: String { Self.first(manifest?.description_p, offered?.description_p) }
        var author: String { Self.first(manifest?.author, offered?.author) }
        var category: Mavio_Plugin_V1_Category {
            if let c = manifest?.category, c != .unspecified { return c }
            return offered?.category ?? .unspecified
        }

        /// The newest version the catalog offers when newer than the one installed.
        var update: String? {
            guard let installed, let newest = offered?.versions.first?.version, !newest.isEmpty else { return nil }
            let current = installed.manifest.version
            return !current.isEmpty && newest != current ? newest : nil
        }
    }

    /// The plugins of a server, those its catalogs offer and those
    /// installed, and its catalogs, which the section's pages share and
    /// refresh after what is done to them.
    @MainActor @Observable final class PluginsStore {
        let server: ServerRecord
        let model: AppModel
        var installed: [Mavio_System_V1_Plugin]?
        var installedFailure: String?
        var catalog: [Mavio_System_V1_CatalogPlugin]?
        var catalogFailure: String?
        var catalogs: [String]?
        var catalogsFailure: String?

        init(server: ServerRecord, model: AppModel) {
            (self.server, self.model) = (server, model)
        }

        /// The plugins its catalogs offer, installed or not.
        var available: [PluginsEntry] {
            let byId = Dictionary((installed ?? []).map { ($0.manifest.id, $0) }, uniquingKeysWith: { a, _ in a })
            return (catalog ?? []).map { PluginsEntry(id: $0.id, offered: $0, installed: byId[$0.id]) }
        }

        /// The plugins installed.
        var installedEntries: [PluginsEntry] {
            let offered = Dictionary((catalog ?? []).map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
            return (installed ?? []).map {
                PluginsEntry(id: $0.manifest.id, offered: offered[$0.manifest.id], installed: $0)
            }
        }

        /// A plugin by its ID, nil when neither offered nor installed.
        func entry(_ id: String) -> PluginsEntry? {
            let offered = catalog?.first { $0.id == id }
            let installed = installed?.first { $0.manifest.id == id }
            return offered == nil && installed == nil
                ? nil : PluginsEntry(id: id, offered: offered, installed: installed)
        }

        /// Reads everything again.
        func refresh() async {
            async let a: Void = loadInstalled()
            async let b: Void = loadCatalog()
            async let c: Void = loadCatalogs()
            _ = await (a, b, c)
        }

        func loadInstalled() async {
            do {
                installed = try await model.call(
                    server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
                    as: Mavio_System_V1_ListPluginsResponse.self
                ).plugins
                installedFailure = nil
            } catch is CancellationError {
            } catch {
                installedFailure = error.reason
            }
        }

        func loadCatalog() async {
            do {
                catalog = try await model.call(
                    server, Procedure.plugins("ListCatalogPlugins"), Mavio_System_V1_ListCatalogPluginsRequest(),
                    as: Mavio_System_V1_ListCatalogPluginsResponse.self
                ).plugins
                catalogFailure = nil
            } catch is CancellationError {
            } catch {
                catalogFailure = error.reason
            }
        }

        func loadCatalogs() async {
            do {
                catalogs = try await model.call(
                    server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                    as: Mavio_System_V1_GetServerSettingsResponse.self
                ).settings.pluginCatalogs
                catalogsFailure = nil
            } catch is CancellationError {
            } catch {
                catalogsFailure = error.reason
            }
        }

        /// Replaces the catalogs' URLs, keeping the other settings.
        func setCatalogs(_ urls: [String]) async throws {
            var settings = try await model.call(
                server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                as: Mavio_System_V1_GetServerSettingsResponse.self
            ).settings
            settings.pluginCatalogs = urls
            var request = Mavio_System_V1_UpdateServerSettingsRequest()
            request.settings = settings
            _ = try await model.call(
                server, Procedure.system("UpdateServerSettings"), request,
                as: Mavio_System_V1_UpdateServerSettingsResponse.self)
        }
    }

    /// A plugin category's title, icon and color.
    struct PluginsCategory {
        let title: String
        let icon: String
        let color: Color

        init(_ c: Mavio_Plugin_V1_Category) {
            switch c {
            case .metadata: (title, icon, color) = (MavioStrings.pluginsCategoryMetadata, "info", Color(hex: 0x007AFF))
            case .subtitles:
                (title, icon, color) = (MavioStrings.pluginsCategorySubtitles, "subtitles", Color(hex: 0x34C759))
            case .downloads:
                (title, icon, color) = (MavioStrings.pluginsCategoryDownloads, "downloads", Color(hex: 0xFF9500))
            case .authentication:
                (title, icon, color) = (MavioStrings.pluginsCategoryAuthentication, "key", Color(hex: 0xFF2D55))
            case .notifications:
                (title, icon, color) = (MavioStrings.pluginsCategoryNotifications, "pending", Color(hex: 0xFF3B30))
            case .devices:
                (title, icon, color) = (MavioStrings.pluginsCategoryDevices, "television", Color(hex: 0x32ADE6))
            case .ai: (title, icon, color) = (MavioStrings.pluginsCategoryAi, "plugins", Color(hex: 0xAF52DE))
            case .speech: (title, icon, color) = (MavioStrings.pluginsCategorySpeech, "plugins", Color(hex: 0x5856D6))
            default: (title, icon, color) = (MavioStrings.pluginsCategoryOther, "plugins", Color(hex: 0x8E8E93))
            }
        }
    }

    /// A plugin state's word.
    func pluginsStateName(_ s: Mavio_System_V1_PluginState) -> String {
        switch s {
        case .ready: MavioStrings.pluginsStateReady
        case .unconfigured: MavioStrings.pluginsStateUnconfigured
        case .failed: MavioStrings.pluginsStateFailed
        case .disabled: MavioStrings.pluginsStateDisabled
        default: ""
        }
    }

    /// A category's icon on a tile of its color.
    struct PluginsCategoryTile: View {
        let category: Mavio_Plugin_V1_Category
        var size: CGFloat = 40

        var body: some View {
            let c = PluginsCategory(category)
            Icon(c.icon, fill: true, size: (size * 0.55).rounded())
                .foregroundStyle(.white)
                .frame(width: size, height: size)
                .background(c.color, in: .rect(cornerRadius: size * 0.22))
        }
    }

    /// The section of the server chosen in the settings: the plugins as
    /// cards in two tabs, filtered by category.
    struct PluginsView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var store: PluginsStore?
        @State private var installedTab = false
        @State private var category: Mavio_Plugin_V1_Category?

        var body: some View {
            Group {
                if let store {
                    list(store)
                } else {
                    ProgressView()
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsPlugins)
            .inlineTitle()
            .task {
                // The catalogs are fetched once; the plugins under
                // development again every few seconds, as the server
                // restarts them when they are built.
                let s = store ?? PluginsStore(server: server, model: model)
                store = s
                async let catalog: Void = s.loadCatalog()
                await s.loadInstalled()
                while !Task.isCancelled {
                    try? await Task.sleep(for: .seconds(2))
                    if s.installed?.contains(where: { !$0.devFolder.isEmpty }) == true { await s.loadInstalled() }
                }
                await catalog
            }
        }

        private func list(_ store: PluginsStore) -> some View {
            let shown = installedTab ? store.installedEntries : store.available
            var present = Array(Set(shown.map(\.category))).sorted { $0.rawValue < $1.rawValue }
            if let category, !present.contains(category) { present.append(category) }
            let list = shown.filter { category == nil || $0.category == category }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            let loaded = installedTab ? store.installed != nil : store.catalog != nil
            return SettingsList(message: MavioStrings.settingsPluginsMessage) {
                Section {
                } header: {
                    controls(store, present: present)
                }
                if !installedTab, let failure = store.catalogFailure {
                    Section {
                    } header: {
                        Text(MavioStrings.pluginsCatalogsUnreachable(reason: failure))
                            .type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                }
                Section {
                } header: {
                    if loaded && list.isEmpty {
                        Text(
                            category != nil
                                ? MavioStrings.pluginsNoneInCategory
                                : installedTab ? MavioStrings.pluginsNoneInstalled : MavioStrings.pluginsNoneAvailable
                        )
                        .type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 40)
                    } else if !loaded && (installedTab ? store.installedFailure : store.catalogFailure) == nil {
                        ProgressView().frame(maxWidth: .infinity)
                    } else {
                        LazyVGrid(
                            columns: [GridItem(.adaptive(minimum: 260), spacing: MavioSpacing.s16)],
                            spacing: MavioSpacing.s16
                        ) {
                            ForEach(list) { e in
                                NavigationLink {
                                    PluginsPage(store: store, id: e.id)
                                } label: {
                                    PluginCard(entry: e)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                }
            }
        }

        /// The tabs, the choice of the category and the link to the catalogs.
        private func controls(_ store: PluginsStore, present: [Mavio_Plugin_V1_Category]) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                Picker(MavioStrings.settingsPlugins, selection: $installedTab) {
                    Text(MavioStrings.pluginsAvailable).tag(false)
                    Text(MavioStrings.pluginsInstalled).tag(true)
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                .fixedSize()
                Spacer(minLength: 0)
                Picker(MavioStrings.pluginsCategory, selection: $category) {
                    Text(MavioStrings.pluginsAllCategories).tag(Mavio_Plugin_V1_Category?.none)
                    ForEach(present, id: \.self) { c in
                        Text(PluginsCategory(c).title).tag(Optional(c))
                    }
                }
                .pickerStyle(.menu)
                .labelsHidden()
                .fixedSize()
                .accessibilityLabel(MavioStrings.pluginsCategory)
                NavigationLink {
                    PluginsCatalogsPage(store: store)
                } label: {
                    Text(MavioStrings.pluginsManageCatalogs)
                }
                .buttonStyle(.bordered)
            }
            .type(theme.type.body)
            .foregroundStyle(theme.colors.textPrimary)
        }
    }

    /// A plugin's card: its tile, name, category and version, description
    /// and state.
    private struct PluginCard: View {
        let entry: PluginsEntry
        @Environment(\.theme) private var theme

        var body: some View {
            let e = entry
            let version = e.installed?.manifest.version ?? ""
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                HStack(spacing: MavioSpacing.s12) {
                    PluginsCategoryTile(category: e.category)
                    VStack(alignment: .leading, spacing: 0) {
                        Text(e.name).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                        Text(
                            [PluginsCategory(e.category).title, version].filter { !$0.isEmpty }.joined(separator: " · ")
                        )
                        .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                    .lineLimit(1)
                }
                Text(e.description).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    .lineLimit(3)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                badge
            }
            .padding(MavioSpacing.s16)
            .frame(maxWidth: .infinity, minHeight: 160, alignment: .topLeading)
            .background(theme.colors.surface, in: .rect(cornerRadius: 14))
            .contentShape(.rect(cornerRadius: 14))
        }

        /// The plugin's state as a short mark.
        @ViewBuilder private var badge: some View {
            let e = entry
            if let installed = e.installed {
                let state = installed.state
                HStack(spacing: MavioSpacing.s8) {
                    if installed.devFolder.isEmpty {
                        mark(MavioStrings.pluginsInstalled, theme.colors.textSecondary)
                    } else {
                        mark(MavioStrings.pluginsDevelopment, theme.colors.accentText)
                    }
                    mark(
                        pluginsStateName(state),
                        state == .failed
                            ? theme.colors.danger : state == .ready ? Color(hex: 0x34C759) : theme.colors.textSecondary)
                    if e.update != nil { mark(MavioStrings.pluginsUpdateAvailable, theme.colors.textPrimary) }
                }
            } else if let v = e.offered?.versions.first?.version, !v.isEmpty {
                mark(v, theme.colors.textSecondary)
            } else {
                mark(MavioStrings.pluginsNoVersion, theme.colors.textTertiary)
            }
        }

        private func mark(_ text: String, _ color: Color) -> some View {
            Text(text).type(theme.type.caption).fontWeight(.semibold).foregroundStyle(color)
        }
    }

    // MARK: Catalogs

    /// The catalogs' URLs, to add to and remove from.
    struct PluginsCatalogsPage: View {
        let store: PluginsStore
        @Environment(\.theme) private var theme
        @State private var url = ""
        @State private var saving = false
        @State private var failure: String?

        var body: some View {
            let list = store.catalogs ?? []
            SettingsList(message: MavioStrings.pluginsCatalogsMessage) {
                Section {
                    ForEach(list, id: \.self) { c in
                        HStack(spacing: MavioSpacing.s12) {
                            Text(c).textSelection(.enabled)
                                .frame(maxWidth: .infinity, alignment: .leading)
                            Button(MavioStrings.pluginsRemoveCatalog, role: .destructive) {
                                save(list.filter { $0 != c })
                            }
                            .foregroundStyle(theme.colors.danger)
                            .disabled(saving)
                        }
                    }
                    HStack(spacing: MavioSpacing.s12) {
                        TextField(
                            MavioStrings.pluginsCatalogAddress, text: $url,
                            prompt: Text(MavioStrings.pluginsCatalogAddress)
                        )
                        .fieldLabelHidden()
                        .textFieldStyle(.plain)
                        .autocorrectionDisabled()
                        .onSubmit { add(list) }
                        Button(MavioStrings.pluginsAddCatalog) { add(list) }
                            .buttonStyle(.bordered)
                            .disabled(url.trimmingCharacters(in: .whitespaces).isEmpty || saving)
                    }
                } footer: {
                    if let failure = failure ?? store.catalogsFailure {
                        Text(failure).foregroundStyle(theme.colors.danger)
                    }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.pluginsCatalogs)
            .inlineTitle()
            .task { await store.loadCatalogs() }
        }

        private func add(_ list: [String]) {
            let u = url.trimmingCharacters(in: .whitespaces)
            guard !u.isEmpty, !list.contains(u) else { return }
            save(list + [u]) { url = "" }
        }

        private func save(_ urls: [String], then done: @escaping () -> Void = {}) {
            saving = true
            Task {
                do {
                    try await store.setCatalogs(urls)
                    failure = nil
                    done()
                } catch {
                    failure = error.reason
                }
                await store.refresh()
                saving = false
            }
        }
    }
#endif
