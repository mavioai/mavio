#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// A plugin's page, as the web UI's: its details and what can be done to
    /// it, its configuration, containers, network, information, logs and
    /// versions.
    struct PluginsPage: View {
        let store: PluginsStore
        let id: String
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        /// The version being installed, the newest when it holds nil.
        @State private var installing: String??
        /// Whether the plugin is being enabled or disabled, the state wanted.
        @State private var toggling: Bool?
        @State private var restarting = false
        @State private var uninstalling = false
        @State private var confirming = false
        @State private var failure: String?
        @State private var containers: [Mavio_System_V1_PluginContainer]?
        @State private var containersFailure: String?
        @State private var gpu = false
        @State private var proxies: [Mavio_System_V1_Proxy]?
        @State private var logs: [Mavio_System_V1_LogRecord]?
        @State private var logsFailure: String?

        var body: some View {
            Group {
                if let e = store.entry(id) {
                    page(e)
                } else {
                    SettingsList {
                        if store.installed != nil && (store.catalog != nil || store.catalogFailure != nil) {
                            Section {
                            } header: {
                                Text(MavioStrings.pluginsNotFound).type(theme.type.body)
                                    .foregroundStyle(theme.colors.textSecondary)
                            }
                        }
                    }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(store.entry(id)?.name ?? MavioStrings.settingsPlugins)
            .inlineTitle()
            .task { await loadOnce() }
            .task(id: store.entry(id)?.installed != nil) {
                guard store.entry(id)?.installed != nil else { return }
                while !Task.isCancelled {
                    await loadLogs()
                    try? await Task.sleep(for: .seconds(3))
                }
            }
            .task(id: store.entry(id)?.installed != nil) {
                // Managed dependencies pull and start in the background.
                guard store.entry(id)?.installed != nil else { return }
                while !Task.isCancelled {
                    await loadContainers()
                    try? await Task.sleep(for: .seconds(5))
                }
            }
        }

        private func page(_ e: PluginsEntry) -> some View {
            let m = e.installed?.manifest
            let dev = e.installed?.devFolder ?? ""
            let hosts = m?.permissions.httpHosts ?? []
            return SettingsList {
                Section {
                } header: {
                    header(e, dev: dev)
                }
                if let m, e.installed != nil {
                    if !m.configSchema.isEmpty || !m.configPage.isEmpty {
                        PluginsConfiguration(
                            server: store.server, id: id, schema: m.configSchema, page: m.configPage,
                            containers: containers
                        ) { await store.refresh() }
                    }
                    if !m.containers.isEmpty {
                        PluginsContainers(
                            server: store.server, id: id, containers: containers, failure: containersFailure, gpu: gpu,
                            proxies: proxies
                        ) {
                            await loadContainers()
                            await store.refresh()
                        }
                    }
                    if (m.runtime == .wasm && !hosts.isEmpty) || m.runtime == .process {
                        PluginsProxySection(
                            server: store.server, id: id, name: m.name, hosts: m.runtime == .wasm ? hosts : nil,
                            proxy: e.installed?.proxy ?? "", proxies: proxies
                        ) { await store.refresh() }
                    }
                }
                Section(MavioStrings.pluginsInformation) {
                    information(e, dev: dev)
                }
                if e.installed != nil {
                    logsSection(dev: dev)
                }
                if dev.isEmpty, let offered = e.offered, !offered.versions.isEmpty {
                    versions(e, offered)
                }
            }
            .confirmationDialog(
                MavioStrings.pluginsUninstallTitle(name: e.name), isPresented: $confirming, titleVisibility: .visible
            ) {
                Button(MavioStrings.pluginsUninstall, role: .destructive) {
                    uninstalling = true
                    perform {
                        var request = Mavio_System_V1_UninstallPluginRequest()
                        request.pluginID = id
                        _ = try await model.call(
                            store.server, Procedure.plugins("UninstallPlugin"), request,
                            as: Mavio_System_V1_UninstallPluginResponse.self)
                    } done: {
                        uninstalling = false
                    }
                }
            } message: {
                Text(MavioStrings.pluginsUninstallMessage)
            }
        }

        // MARK: Header

        private func header(_ e: PluginsEntry, dev: String) -> some View {
            let newest = e.offered?.versions.first?.version ?? ""
            // A plugin under development takes no catalog version.
            let update = dev.isEmpty ? e.update : nil
            let enabled = e.installed?.state != .disabled
            return VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                HStack(spacing: MavioSpacing.s16) {
                    PluginsCategoryTile(category: e.category, size: 64)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(e.name).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
                        if !e.author.isEmpty {
                            Text(MavioStrings.pluginsBy(author: e.author)).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                    .frame(minWidth: 192, maxWidth: .infinity, alignment: .leading)
                    HStack(spacing: MavioSpacing.s12) {
                        if e.installed == nil && !newest.isEmpty {
                            Button(installing == .some(nil) ? "…" : MavioStrings.pluginsInstall) { install(nil) }
                                .buttonStyle(.borderedProminent)
                                .disabled(installing != nil)
                        }
                        if let update {
                            Button(installing == .some(update) ? "…" : MavioStrings.pluginsUpdate(version: update)) {
                                install(update)
                            }
                            .buttonStyle(.borderedProminent)
                            .disabled(installing != nil)
                        }
                        if e.installed != nil {
                            // Starting or stopping a process plugin takes a
                            // while; the switch waits for it.
                            Toggle(
                                MavioStrings.pluginsEnabled,
                                isOn: Binding(get: { toggling ?? enabled }, set: { setEnabled($0) })
                            )
                            .toggleStyle(.switch)
                            .disabled(toggling != nil)
                            .help(
                                toggling == nil
                                    ? "" : enabled ? MavioStrings.pluginsDisabling : MavioStrings.pluginsEnabling)
                            if enabled {
                                Button(restarting ? "…" : MavioStrings.pluginsRestart) { restart() }
                                    .buttonStyle(.bordered)
                                    .disabled(restarting)
                            }
                            if dev.isEmpty {
                                Button(MavioStrings.pluginsUninstall) { confirming = true }
                                    .buttonStyle(.bordered)
                                    .foregroundStyle(theme.colors.danger)
                                    .disabled(uninstalling)
                            }
                        }
                    }
                    .fixedSize()
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
                if !e.description.isEmpty {
                    Text(e.description).type(theme.type.body).foregroundStyle(theme.colors.textPrimary)
                }
            }
            .textCase(nil)
        }

        // MARK: Information

        @ViewBuilder private func information(_ e: PluginsEntry, dev: String) -> some View {
            let m = e.installed?.manifest
            if let installed = e.installed {
                let state = pluginsStateName(installed.state)
                fact(
                    MavioStrings.pluginsState,
                    installed.state == .failed && !installed.error.isEmpty ? "\(state): \(installed.error)" : state)
                if let v = m?.version, !v.isEmpty { fact(MavioStrings.pluginsVersion, v) }
                if !dev.isEmpty {
                    LabeledContent(MavioStrings.pluginsDevFolder) { Text(dev).monospaced().selectable() }
                }
            }
            if let newest = e.offered?.versions.first?.version, !newest.isEmpty {
                fact(MavioStrings.pluginsLatestVersion, newest)
            }
            fact(MavioStrings.pluginsCategory, PluginsCategory(e.category).title)
            if !e.author.isEmpty { fact(MavioStrings.pluginsAuthor, e.author) }
            if let license = m?.license, !license.isEmpty { fact(MavioStrings.pluginsLicense, license) }
            let homepage = [m?.homepage ?? "", e.offered?.homepage ?? ""].first { !$0.isEmpty }
            if let homepage, let url = URL(string: homepage) {
                LabeledContent(MavioStrings.pluginsWebsite) {
                    Link(destination: url) {
                        HStack(spacing: MavioSpacing.s4) {
                            Text(homepage)
                            Icon("external", size: 14)
                        }
                    }
                    .foregroundStyle(theme.colors.accentText)
                }
            }
            if let offered = e.offered { fact(MavioStrings.pluginsCatalog, offered.catalogURL) }
            let hosts = m?.permissions.httpHosts ?? []
            if !hosts.isEmpty { fact(MavioStrings.pluginsNetworkAccess, hosts.joined(separator: ", ")) }
        }

        private func fact(_ label: String, _ value: String) -> some View {
            LabeledContent(label) { Text(value).selectable() }
        }

        // MARK: Logs

        /// The plugin's recent log records, newest first; under development,
        /// with where it runs from.
        private func logsSection(dev: String) -> some View {
            Section {
                if let logsFailure {
                    Text(logsFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                } else if let logs {
                    if logs.isEmpty {
                        Text(MavioStrings.logsNone).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    } else {
                        let rows = VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                            ForEach(Array(logs.enumerated()), id: \.offset) { PluginsLogRow(record: $0.element) }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        ViewThatFits(in: .vertical) {
                            rows
                            ScrollView { rows }
                        }
                        .frame(maxHeight: 384)
                    }
                }
            } header: {
                Text(MavioStrings.pluginsLogs)
            } footer: {
                Text(
                    dev.isEmpty
                        ? MavioStrings.pluginsLogsFooter
                        : "\(MavioStrings.pluginsLogsFooter) \(MavioStrings.pluginsDevFolderDetail)")
            }
        }

        // MARK: Versions

        private func versions(_ e: PluginsEntry, _ offered: Mavio_System_V1_CatalogPlugin) -> some View {
            Section(MavioStrings.pluginsVersions) {
                ForEach(offered.versions, id: \.version) { v in
                    let detail = [
                        v.releaseTime.time?.formatted(date: .abbreviated, time: .omitted) ?? "", v.changelog,
                    ]
                    .filter { !$0.isEmpty }.joined(separator: " · ")
                    LabeledContent {
                        if v.version == e.installed?.manifest.version {
                            Text(MavioStrings.pluginsInstalled).type(theme.type.callout)
                        } else if e.installed != nil {
                            Button(installing == .some(v.version) ? "…" : MavioStrings.pluginsInstallThis) {
                                install(v.version)
                            }
                            .buttonStyle(.bordered)
                            .foregroundStyle(theme.colors.textPrimary)
                            .disabled(installing != nil)
                        }
                    } label: {
                        Text(v.version)
                        if !detail.isEmpty { Text(detail) }
                    }
                }
            }
        }

        // MARK: Actions

        private func install(_ version: String?) {
            installing = .some(version)
            perform {
                var request = Mavio_System_V1_InstallPluginRequest()
                request.pluginID = id
                if let version { request.version = version }
                _ = try await model.call(
                    store.server, Procedure.plugins("InstallPlugin"), request,
                    as: Mavio_System_V1_InstallPluginResponse.self)
            } done: {
                installing = nil
            }
        }

        private func setEnabled(_ on: Bool) {
            toggling = on
            perform {
                var request = Mavio_System_V1_SetPluginEnabledRequest()
                request.pluginID = id
                request.enabled = on
                _ = try await model.call(
                    store.server, Procedure.plugins("SetPluginEnabled"), request,
                    as: Mavio_System_V1_SetPluginEnabledResponse.self)
            } done: {
                toggling = nil
            }
        }

        /// Stops the plugin and starts it again, as after an upgrade; one
        /// under development with what was built.
        private func restart() {
            restarting = true
            perform {
                var request = Mavio_System_V1_RestartPluginRequest()
                request.pluginID = id
                _ = try await model.call(
                    store.server, Procedure.plugins("RestartPlugin"), request,
                    as: Mavio_System_V1_RestartPluginResponse.self)
            } done: {
                restarting = false
            }
        }

        /// Does work, then reads the plugins again and calls done.
        private func perform(work: @escaping @MainActor () async throws -> Void, done: @escaping @MainActor () -> Void)
        {
            Task {
                do {
                    try await work()
                    failure = nil
                } catch {
                    failure = error.reason
                }
                await store.refresh()
                done()
            }
        }

        // MARK: Loading

        private func loadOnce() async {
            async let engine = try? model.call(
                store.server, Procedure.plugins("GetContainerRuntime"), Mavio_System_V1_GetContainerRuntimeRequest(),
                as: Mavio_System_V1_GetContainerRuntimeResponse.self)
            async let list = try? model.call(
                store.server, Procedure.proxies("ListProxies"), Mavio_System_V1_ListProxiesRequest(),
                as: Mavio_System_V1_ListProxiesResponse.self)
            // A GPU is offered only where the engine gives containers one.
            gpu = await engine?.runtime.gpu ?? false
            proxies = await list?.proxies
        }

        private func loadLogs() async {
            var request = Mavio_System_V1_ListLogsRequest()
            request.pluginID = id
            request.limit = 200
            do {
                logs = try await model.call(
                    store.server, Procedure.system("ListLogs"), request, as: Mavio_System_V1_ListLogsResponse.self
                ).records
                logsFailure = nil
            } catch is CancellationError {
            } catch {
                logsFailure = error.reason
            }
        }

        private func loadContainers() async {
            var request = Mavio_System_V1_ListPluginContainersRequest()
            request.pluginID = id
            do {
                containers = try await model.call(
                    store.server, Procedure.plugins("ListPluginContainers"), request,
                    as: Mavio_System_V1_ListPluginContainersResponse.self
                ).containers
                containersFailure = nil
            } catch is CancellationError {
            } catch {
                containersFailure = error.reason
            }
        }
    }

    /// A log record of a plugin's: its time and level, message and attributes.
    private struct PluginsLogRow: View {
        let record: Mavio_System_V1_LogRecord
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: MavioSpacing.s8) {
                    if let time = record.time.time {
                        Text(time.formatted(date: .omitted, time: .standard)).monospacedDigit()
                            .foregroundStyle(theme.colors.textSecondary)
                    }
                    Text(LogsView.name(record.level)).fontWeight(.semibold).foregroundStyle(color)
                }
                .type(theme.type.footnote)
                Text(record.message).type(theme.type.callout)
                let attrs = record.attributes.sorted { $0.key < $1.key }
                if !attrs.isEmpty {
                    Text(attrs.map { "\($0.key)=\($0.value)" }.joined(separator: "   "))
                        .type(theme.type.caption).monospaced()
                        .foregroundStyle(theme.colors.textSecondary)
                }
            }
            .selectable()
        }

        private var color: Color {
            switch record.level {
            case .debug: theme.colors.textTertiary
            case .warn: Color(hex: 0xFF9F0A)
            case .error: theme.colors.danger
            default: theme.colors.textSecondary
            }
        }
    }

    // MARK: Network

    /// The proxies a plugin or container can reach sites through: none, the
    /// system proxy, then the server's by name, and the one set though the
    /// list lacks it.
    func pluginsProxyChoices(_ list: [Mavio_System_V1_Proxy]?, _ current: String) -> [(id: String, name: String)] {
        let own = (list ?? []).filter { !$0.system }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        var out: [(id: String, name: String)] =
            [("", MavioStrings.proxiesNone), ("system", MavioStrings.proxiesSystem)] + own.map { ($0.id, $0.name) }
        if list != nil && !out.contains(where: { $0.id == current }) {
            out.append((current, MavioStrings.proxiesUnknown))
        }
        return out
    }

    /// Whether a proxy choice reaches sites directly: none, or the system
    /// proxy while the system sets none; nil before the list is read.
    func pluginsProxyDirect(_ list: [Mavio_System_V1_Proxy]?, _ id: String) -> Bool? {
        if id.isEmpty { return true }
        if id != "system" { return false }
        return list?.first { $0.system }.map { $0.kind == .unspecified }
    }

    /// The proxy a plugin reaches sites through, saved as it is chosen: a
    /// WASM plugin's requests to hosts, which the server performs, or a
    /// process plugin's, which go to the server's proxy gateway; a process
    /// plugin going from direct to proxied or back restarts, once confirmed.
    private struct PluginsProxySection: View {
        let server: ServerRecord
        let id: String
        let name: String
        let hosts: [String]?
        let proxy: String
        let proxies: [Mavio_System_V1_Proxy]?
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var saving: String?
        @State private var restarting: String?
        @State private var failure: String?

        var body: some View {
            let value = saving ?? proxy
            Section {
                HStack(spacing: MavioSpacing.s12) {
                    RowLabel(
                        label: MavioStrings.pluginsProxy,
                        detail: hosts.map { MavioStrings.pluginsProxyDetail(hosts: $0.joined(separator: ", ")) }
                            ?? MavioStrings.pluginsProxyProcessDetail)
                    Spacer(minLength: 0)
                    Picker(
                        MavioStrings.pluginsProxy,
                        selection: Binding(
                            get: { value },
                            set: { p in
                                let was = pluginsProxyDirect(proxies, proxy)
                                let next = pluginsProxyDirect(proxies, p)
                                if hosts == nil, let was, let next, was != next { restarting = p } else { save(p) }
                            })
                    ) {
                        ForEach(pluginsProxyChoices(proxies, value), id: \.id) { Text($0.name).tag($0.id) }
                    }
                    .labelsHidden()
                    .pickerStyle(.menu)
                    .frame(width: 192)
                    .disabled(saving != nil)
                }
                .confirmationDialog(
                    MavioStrings.pluginsProxyRestartTitle(name: name),
                    isPresented: Binding(get: { restarting != nil }, set: { if !$0 { restarting = nil } }),
                    titleVisibility: .visible
                ) {
                    Button(MavioStrings.pluginsProxyRestart) {
                        if let restarting { save(restarting) }
                    }
                } message: {
                    Text(MavioStrings.pluginsProxyRestartMessage)
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
            } header: {
                Text(MavioStrings.pluginsNetwork)
            }
        }

        private func save(_ p: String) {
            saving = p
            Task {
                var request = Mavio_System_V1_SetPluginProxyRequest()
                request.pluginID = id
                request.proxy = p
                do {
                    _ = try await model.call(
                        server, Procedure.plugins("SetPluginProxy"), request,
                        as: Mavio_System_V1_SetPluginProxyResponse.self)
                    failure = nil
                } catch {
                    failure = error.reason
                }
                await changed()
                saving = nil
                restarting = nil
            }
        }
    }

    // MARK: Configuration

    /// An installed plugin's configuration: the form of its schema, or a
    /// link to its own page.
    private struct PluginsConfiguration: View {
        let server: ServerRecord
        let id: String
        let schema: String
        let page: String
        let containers: [Mavio_System_V1_PluginContainer]?
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var parsed: PluginsSchema?
        @State private var saved: PluginsJSON?
        @State private var value: PluginsJSON?
        @State private var options: [String: [PluginsOption]] = [:]
        @State private var loadFailure: String?
        @State private var saving = false
        @State private var done = false
        @State private var failure: String?

        /// The plugin's own settings page on the server.
        private var own: URL? {
            guard !page.isEmpty else { return nil }
            let base = server.address.hasSuffix("/") ? String(server.address.dropLast()) : server.address
            let pid = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(["/"])) ?? id
            return URL(string: "\(base)/plugins/\(pid)/\(page)")
        }

        // Saving refreshes saved, which the form then matches.
        private var isChanged: Bool { (value ?? .null).text != (saved ?? .null).text }

        var body: some View {
            Section {
                if let parsed, saved != nil {
                    PluginsSchemaForm(schema: parsed, value: value, options: options, unset: unset) {
                        value = $0
                        done = false
                    }
                }
            } header: {
                HStack(spacing: MavioSpacing.s12) {
                    Text(MavioStrings.pluginsConfiguration)
                    Spacer(minLength: 0)
                    if let own {
                        Link(destination: own) {
                            HStack(spacing: MavioSpacing.s4) {
                                Text(MavioStrings.pluginsOpenSettingsPage)
                                Icon("external", size: 14)
                            }
                        }
                        .type(theme.type.callout)
                        .foregroundStyle(theme.colors.accentText)
                    }
                }
                .task(id: schema) { await load() }
            } footer: {
                if let loadFailure {
                    Text(loadFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                } else if parsed != nil && saved != nil {
                    HStack(spacing: MavioSpacing.s12) {
                        Button(saving ? "…" : MavioStrings.actionSave, action: save)
                            .buttonStyle(.borderedProminent)
                            .type(theme.type.body)
                            .disabled(!isChanged || saving)
                        if done && !isChanged {
                            Text(MavioStrings.pluginsConfigurationSaved).type(theme.type.callout)
                        }
                        if let failure {
                            Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                        }
                    }
                    .padding(.top, MavioSpacing.s4)
                }
            }
        }

        /// Whether a container dependency of the plugin is not set up, by its name.
        private func unset(_ name: String) -> Bool {
            guard let c = containers?.first(where: { $0.dependency.name == name }) else { return false }
            return c.status.mode == .unspecified
        }

        private func load() async {
            guard let s = PluginsSchema(parsing: schema), s.hasProperties else {
                parsed = nil
                return
            }
            parsed = s
            var request = Mavio_System_V1_GetPluginConfigRequest()
            request.pluginID = id
            do {
                let text = try await model.call(
                    server, Procedure.plugins("GetPluginConfig"), request,
                    as: Mavio_System_V1_GetPluginConfigResponse.self
                ).configJson
                let v = text.isEmpty ? .object(PluginsJSONObject()) : PluginsJSON(parsing: text) ?? .object(.init())
                saved = v
                value = v
                loadFailure = nil
            } catch is CancellationError {
                return
            } catch {
                loadFailure = error.reason
            }
            for source in s.optionSources {
                var r = Mavio_System_V1_ListPluginConfigOptionsRequest()
                r.pluginID = id
                r.source = source
                if let list = try? await model.call(
                    server, Procedure.plugins("ListPluginConfigOptions"), r,
                    as: Mavio_System_V1_ListPluginConfigOptionsResponse.self
                ).options {
                    options[source] = list
                }
            }
        }

        private func save() {
            saving = true
            done = false
            Task {
                var request = Mavio_System_V1_SetPluginConfigRequest()
                request.pluginID = id
                request.configJson = (value ?? .object(PluginsJSONObject())).text
                do {
                    // The configuration stored holds the IDs the server generated.
                    let stored = try await model.call(
                        server, Procedure.plugins("SetPluginConfig"), request,
                        as: Mavio_System_V1_SetPluginConfigResponse.self
                    ).configJson
                    let v = PluginsJSON(parsing: stored) ?? value
                    saved = v
                    value = v
                    done = true
                    failure = nil
                } catch {
                    failure = error.reason
                }
                saving = false
                await changed()
            }
        }
    }
#endif
