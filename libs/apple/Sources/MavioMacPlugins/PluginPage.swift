#if os(macOS)
    import AppKit
    import MavioMac

    /// A plugin's page, as the web UI's: its details and what can be done to
    /// it, its configuration, containers, network, information, logs and
    /// versions. The configuration and the containers keep their fields as
    /// the plugin is read again.
    final class PluginPage: Page {
        private let store: PluginsStore
        private let id: String
        /// The version being installed, the newest when it holds nil.
        private var installing: String??
        /// Whether the plugin is being enabled or disabled, the state wanted.
        private var toggling: Bool?
        private var restarting = false
        private var uninstalling = false
        private var failure: String?
        private var containers: [Mavio_System_V1_PluginContainer]?
        private var containersFailure: String?
        private var gpu = false
        private var proxies: [Mavio_System_V1_Proxy]?
        private var logs: [Mavio_System_V1_LogRecord]?
        private var logsFailure: String?

        private var observer: Observer?
        private var tasks: [Task<Void, Never>] = []
        private var polling: [Task<Void, Never>] = []
        private var polled = false

        private lazy var form = FormPage(theme: theme)
        private let headerSlot = NSStackView.vertical([], spacing: 0)
        private let configSlot = NSStackView.vertical([], spacing: 0)
        private let containersSlot = NSStackView.vertical([], spacing: 0)
        private let proxySlot = NSStackView.vertical([], spacing: 0)
        private let infoSlot = NSStackView.vertical([], spacing: 0)
        private let logsSlot = NSStackView.vertical([], spacing: 0)
        private let versionsSlot = NSStackView.vertical([], spacing: 0)
        private var configuration: PluginConfiguration?
        private var containerGroups: PluginContainers?
        private lazy var logsSection = LiveSection(MavioStrings.pluginsLogs, theme: theme)
        private lazy var logsStack = NSStackView.vertical([], spacing: MavioSpacing.s12)
        private lazy var logsRow = ContentRow(CappedScroll(logsStack, maxHeight: 384), theme: theme)
        private var logsShown: [Mavio_System_V1_LogRecord]?

        /// The page of the plugin of id among store's.
        init(context: MacContext, store: PluginsStore, id: String) {
            self.store = store
            self.id = id
            super.init(context: context)
        }

        isolated deinit {
            observer?.cancel()
            tasks.forEach { $0.cancel() }
            polling.forEach { $0.cancel() }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsPlugins
            for slot in [headerSlot, configSlot, containersSlot, proxySlot, infoSlot, logsSlot, versionsSlot] {
                form.add(slot)
            }
            show(form)
            observer = Observer { [weak self] in self?.render() }
            run { [weak self] in await self?.loadOnce() }
            if store.installed == nil || store.catalog == nil {
                let store = store
                run { await store.refresh() }
            }
        }

        private var server: ServerRecord { store.server }

        private func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        /// Puts views in a slot of the page's column, as wide as it, and
        /// hides it without any.
        private func fill(_ slot: NSStackView, _ views: [NSView]) {
            if slot.arrangedSubviews.map(ObjectIdentifier.init) != views.map(ObjectIdentifier.init) {
                slot.setViews(views)
                for v in views { v.widthAnchor.constraint(equalTo: slot.widthAnchor).isActive = true }
            }
            slot.isHidden = views.isEmpty
        }

        // MARK: Drawing

        private func render() {
            guard let e = store.entry(id) else {
                title = MavioStrings.settingsPlugins
                let found = store.installed != nil && (store.catalog != nil || store.catalogFailure != nil)
                fill(
                    headerSlot,
                    found ? [plainNote(MavioStrings.pluginsNotFound, theme.colors.textSecondary, theme: theme)] : [])
                for slot in [configSlot, containersSlot, proxySlot, infoSlot, logsSlot, versionsSlot] { fill(slot, []) }
                return
            }
            title = e.name
            let m = e.installed?.manifest
            let dev = e.installed?.devFolder ?? ""
            let hosts = m?.permissions.httpHosts ?? []
            poll(e.installed != nil)
            fill(headerSlot, [header(e, dev: dev)])
            if let m, e.installed != nil, !m.configSchema.isEmpty || !m.configPage.isEmpty {
                let config =
                    configuration
                    ?? PluginConfiguration(page: self, server: server, id: id) { [weak self] in
                        await self?.store.refresh()
                    }
                configuration = config
                config.update(schema: m.configSchema, settingsPage: m.configPage, containers: containers)
                fill(configSlot, [config.view])
            } else {
                configuration = nil
                fill(configSlot, [])
            }
            if let m, e.installed != nil, !m.containers.isEmpty {
                let groups =
                    containerGroups
                    ?? PluginContainers(page: self, server: server, id: id) { [weak self] in
                        await self?.loadContainers()
                        await self?.store.refresh()
                    }
                containerGroups = groups
                groups.update(containers: containers, failure: containersFailure, gpu: gpu, proxies: proxies)
                fill(containersSlot, [groups.view])
            } else {
                containerGroups = nil
                fill(containersSlot, [])
            }
            if let m, e.installed != nil, (m.runtime == .wasm && !hosts.isEmpty) || m.runtime == .process {
                fill(
                    proxySlot,
                    [
                        proxySection(
                            name: m.name, hosts: m.runtime == .wasm ? hosts : nil, proxy: e.installed?.proxy ?? "")
                    ])
            } else {
                fill(proxySlot, [])
            }
            fill(infoSlot, [FormSection(MavioStrings.pluginsInformation, rows: information(e, dev: dev), theme: theme)])
            if e.installed != nil {
                renderLogs(dev: dev)
                fill(logsSlot, [logsSection])
            } else {
                fill(logsSlot, [])
            }
            if dev.isEmpty, let offered = e.offered, !offered.versions.isEmpty {
                fill(versionsSlot, [versions(e, offered)])
            } else {
                fill(versionsSlot, [])
            }
            FocusEngine.active(for: view)?.schedule()
        }

        // MARK: Header

        private func header(_ e: PluginsEntry, dev: String) -> NSView {
            let newest = e.offered?.versions.first?.version ?? ""
            // A plugin under development takes no catalog version.
            let update = dev.isEmpty ? e.update : nil
            let enabled = e.installed?.state != .disabled
            var titles: [NSView] = [NSTextField.label(e.name, theme.fonts.title1, color: theme.colors.textPrimary)]
            if !e.author.isEmpty {
                titles.append(
                    NSTextField.label(
                        MavioStrings.pluginsBy(author: e.author), theme.fonts.body, color: theme.colors.textSecondary))
            }
            let names = NSStackView.vertical(titles, spacing: 2)
            names.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            names.widthAnchor.constraint(greaterThanOrEqualToConstant: 192).isActive = true
            var actions: [NSView] = []
            if e.installed == nil && !newest.isEmpty {
                let button = MavioButton(
                    installing == .some(nil) ? "…" : MavioStrings.pluginsInstall, kind: .primary, theme: theme
                ) { [weak self] in self?.install(nil) }
                button.isEnabled = installing == nil
                actions.append(button)
            }
            if let update {
                let button = MavioButton(
                    installing == .some(update) ? "…" : MavioStrings.pluginsUpdate(version: update), kind: .primary,
                    theme: theme
                ) { [weak self] in self?.install(update) }
                button.isEnabled = installing == nil
                actions.append(button)
            }
            if e.installed != nil {
                // Starting or stopping a process plugin takes a while; the
                // switch waits for it.
                let toggle = ActionSwitch(isOn: toggling ?? enabled) { [weak self] on in self?.setEnabled(on) }
                toggle.isEnabled = toggling == nil
                toggle.setAccessibilityLabel(MavioStrings.pluginsEnabled)
                if toggling != nil, !theme.isTV {
                    toggle.toolTip = enabled ? MavioStrings.pluginsDisabling : MavioStrings.pluginsEnabling
                }
                let label = NSTextField.label(
                    MavioStrings.pluginsEnabled, theme.fonts.body, color: theme.colors.textPrimary)
                actions.append(
                    NSStackView.horizontal([label, focusable(toggle, theme: theme)], spacing: MavioSpacing.s8))
                if enabled {
                    let button = MavioButton(
                        restarting ? "…" : MavioStrings.pluginsRestart, kind: .secondary, theme: theme
                    ) { [weak self] in self?.restart() }
                    button.isEnabled = !restarting
                    actions.append(button)
                }
                if dev.isEmpty {
                    let button = MavioButton(MavioStrings.pluginsUninstall, kind: .destructive, theme: theme) {
                        [weak self] in
                        guard let self else { return }
                        confirm(
                            MavioStrings.pluginsUninstallTitle(name: e.name),
                            message: MavioStrings.pluginsUninstallMessage, action: MavioStrings.pluginsUninstall,
                            in: self.view
                        ) { [weak self] in self?.uninstall() }
                    }
                    button.isEnabled = !uninstalling
                    actions.append(button)
                }
            }
            let actionBar = NSStackView.horizontal(actions, spacing: MavioSpacing.s12)
            actionBar.setContentCompressionResistancePriority(.required, for: .horizontal)
            let top = NSStackView.horizontal(
                [categoryTile(e.category, size: 64), names, actionBar], spacing: MavioSpacing.s16)
            var views: [NSView] = [top]
            if let failure { views.append(note(failure, theme.fonts.callout, theme.colors.danger, selectable: true)) }
            if !e.description.isEmpty {
                views.append(note(e.description, theme.fonts.body, theme.colors.textPrimary, selectable: true))
            }
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s24)
            for v in views { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            return column
        }

        // MARK: Information

        private func information(_ e: PluginsEntry, dev: String) -> [NSView] {
            let m = e.installed?.manifest
            var rows: [NSView] = []
            if let installed = e.installed {
                let state = pluginsStateName(installed.state)
                rows.append(
                    fact(
                        MavioStrings.pluginsState,
                        installed.state == .failed && !installed.error.isEmpty ? "\(state): \(installed.error)" : state)
                )
                if let v = m?.version, !v.isEmpty { rows.append(fact(MavioStrings.pluginsVersion, v)) }
                if !dev.isEmpty { rows.append(fact(MavioStrings.pluginsDevFolder, dev, monospaced: true)) }
            }
            if let newest = e.offered?.versions.first?.version, !newest.isEmpty {
                rows.append(fact(MavioStrings.pluginsLatestVersion, newest))
            }
            rows.append(fact(MavioStrings.pluginsCategory, PluginsCategory(e.category).title))
            if !e.author.isEmpty { rows.append(fact(MavioStrings.pluginsAuthor, e.author)) }
            if let license = m?.license, !license.isEmpty { rows.append(fact(MavioStrings.pluginsLicense, license)) }
            let homepage = [m?.homepage ?? "", e.offered?.homepage ?? ""].first { !$0.isEmpty }
            if let homepage, let url = URL(string: homepage) {
                if theme.isTV {
                    // Television mode opens no web pages.
                    rows.append(fact(MavioStrings.pluginsWebsite, homepage))
                } else {
                    let link = MavioButton(homepage, icon: "external", kind: .link, theme: theme) {
                        NSWorkspace.shared.open(url)
                    }
                    rows.append(FormRow(MavioStrings.pluginsWebsite, accessory: link, theme: theme))
                }
            }
            if let offered = e.offered { rows.append(fact(MavioStrings.pluginsCatalog, offered.catalogURL)) }
            let hosts = m?.permissions.httpHosts ?? []
            if !hosts.isEmpty { rows.append(fact(MavioStrings.pluginsNetworkAccess, hosts.joined(separator: ", "))) }
            return rows
        }

        private func fact(_ label: String, _ value: String, monospaced: Bool = false) -> NSView {
            let text = NSTextField.label(
                value, theme.fonts.body, color: theme.colors.textSecondary, lines: 0, selectable: !theme.isTV)
            text.alignment = .right
            if monospaced { text.font = .monospacedSystemFont(ofSize: theme.fonts.body.size, weight: .regular) }
            text.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            let title = NSTextField.label(label, theme.fonts.body, color: theme.colors.textPrimary)
            title.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
            return ContentRow(NSStackView.horizontal([title, Spacer(), text], spacing: MavioSpacing.s16), theme: theme)
        }

        // MARK: Logs

        /// The plugin's recent log records, newest first; under development,
        /// with where it runs from.
        private func renderLogs(dev: String) {
            var rows: [NSView] = []
            if let logsFailure {
                rows.append(noteRow(logsFailure, theme.colors.danger, theme: theme))
            } else if let logs {
                if logs.isEmpty {
                    rows.append(noteRow(MavioStrings.logsNone, theme.colors.textSecondary, theme: theme))
                } else {
                    if logs != logsShown {
                        logsShown = logs
                        let views = logs.map(logRow)
                        logsStack.setViews(views)
                        for v in views { v.widthAnchor.constraint(equalTo: logsStack.widthAnchor).isActive = true }
                    }
                    rows.append(logsRow)
                }
            }
            logsSection.setRows(rows)
            logsSection.footer =
                dev.isEmpty
                ? MavioStrings.pluginsLogsFooter
                : "\(MavioStrings.pluginsLogsFooter) \(MavioStrings.pluginsDevFolderDetail)"
        }

        /// A log record: its time and level, message and attributes.
        private func logRow(_ record: Mavio_System_V1_LogRecord) -> NSView {
            var head: [NSView] = []
            if let time = record.time.time {
                let label = NSTextField.label(
                    time.formatted(date: .omitted, time: .standard), theme.fonts.footnote,
                    color: theme.colors.textSecondary, selectable: true)
                label.font = .monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
                head.append(label)
            }
            let color: NSColor =
                switch record.level {
                case .debug: theme.colors.textTertiary
                case .warn: NSColor(hex: 0xFF9F0A)
                case .error: theme.colors.danger
                default: theme.colors.textSecondary
                }
            let level = NSTextField.label(logLevelName(record.level), theme.fonts.footnote, color: color)
            level.font = .systemFont(ofSize: theme.fonts.footnote.size, weight: .semibold)
            head.append(level)
            var views: [NSView] = [
                NSStackView.horizontal(head, spacing: MavioSpacing.s8),
                note(record.message, theme.fonts.callout, theme.colors.textPrimary, selectable: true),
            ]
            let attrs = record.attributes.sorted { $0.key < $1.key }
            if !attrs.isEmpty {
                let text = note(
                    attrs.map { "\($0.key)=\($0.value)" }.joined(separator: "   "), theme.fonts.caption,
                    theme.colors.textSecondary, selectable: true)
                text.font = .monospacedSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
                views.append(text)
            }
            return NSStackView.vertical(views, spacing: 2)
        }

        // MARK: Versions

        private func versions(_ e: PluginsEntry, _ offered: Mavio_System_V1_CatalogPlugin) -> NSView {
            let rows: [NSView] = offered.versions.map { v in
                let detail = [v.releaseTime.time?.formatted(date: .abbreviated, time: .omitted) ?? "", v.changelog]
                    .filter { !$0.isEmpty }.joined(separator: " · ")
                let subtitle = detail.isEmpty ? nil : detail
                if v.version == e.installed?.manifest.version {
                    let label = NSTextField.label(
                        MavioStrings.pluginsInstalled, theme.fonts.callout, color: theme.colors.textPrimary)
                    return FormRow(v.version, subtitle: subtitle, accessory: label, theme: theme)
                } else if e.installed != nil {
                    let button = MavioButton(
                        installing == .some(v.version) ? "…" : MavioStrings.pluginsInstallThis, kind: .secondary,
                        theme: theme
                    ) { [weak self] in self?.install(v.version) }
                    button.isEnabled = installing == nil
                    return controlRow(v.version, subtitle: subtitle, control: button, theme: theme)
                }
                return FormRow(v.version, subtitle: subtitle, theme: theme)
            }
            return FormSection(MavioStrings.pluginsVersions, rows: rows, theme: theme)
        }

        // MARK: Network

        private var proxySaving: String?
        private var proxyFailure: String?

        /// The proxy a plugin reaches sites through, saved as it is chosen: a
        /// WASM plugin's requests to hosts, which the server performs, or a
        /// process plugin's, which go to the server's proxy gateway; a
        /// process plugin going from direct to proxied or back restarts,
        /// once confirmed.
        private func proxySection(name: String, hosts: [String]?, proxy: String) -> NSView {
            let value = proxySaving ?? proxy
            let choices = pluginsProxyChoices(proxies, value)
            let popup = ActionPopUp(
                options: choices.map(\.name), selected: choices.firstIndex { $0.id == value } ?? 0
            ) { [weak self] i in
                guard let self else { return }
                let p = choices[i].id
                let was = pluginsProxyDirect(self.proxies, proxy)
                let next = pluginsProxyDirect(self.proxies, p)
                if hosts == nil, let was, let next, was != next {
                    confirm(
                        MavioStrings.pluginsProxyRestartTitle(name: name),
                        message: MavioStrings.pluginsProxyRestartMessage, action: MavioStrings.pluginsProxyRestart,
                        destructive: false, in: self.view
                    ) { [weak self] in self?.saveProxy(p) }
                    // Until confirmed, the choice stays as it was.
                    self.render()
                } else {
                    self.saveProxy(p)
                }
            }
            popup.isEnabled = proxySaving == nil
            popup.setAccessibilityLabel(MavioStrings.pluginsProxy)
            popup.widthAnchor.constraint(equalToConstant: 192).isActive = true
            var rows: [NSView] = [
                controlRow(
                    MavioStrings.pluginsProxy,
                    subtitle: hosts.map { MavioStrings.pluginsProxyDetail(hosts: $0.joined(separator: ", ")) }
                        ?? MavioStrings.pluginsProxyProcessDetail, control: popup, enabled: proxySaving == nil,
                    theme: theme)
            ]
            if let proxyFailure { rows.append(noteRow(proxyFailure, theme.colors.danger, theme: theme)) }
            return FormSection(MavioStrings.pluginsNetwork, rows: rows, theme: theme)
        }

        private func saveProxy(_ p: String) {
            proxySaving = p
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_SetPluginProxyRequest()
                request.pluginID = self.id
                request.proxy = p
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.plugins("SetPluginProxy"), request,
                        as: Mavio_System_V1_SetPluginProxyResponse.self)
                    self.proxyFailure = nil
                } catch {
                    self.proxyFailure = error.reason
                }
                await self.store.refresh()
                self.proxySaving = nil
                self.render()
            }
        }

        // MARK: Actions

        private func install(_ version: String?) {
            installing = .some(version)
            perform { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_InstallPluginRequest()
                request.pluginID = self.id
                if let version { request.version = version }
                _ = try await self.model.call(
                    self.server, Procedure.plugins("InstallPlugin"), request,
                    as: Mavio_System_V1_InstallPluginResponse.self)
            } done: { [weak self] in
                self?.installing = nil
            }
        }

        private func setEnabled(_ on: Bool) {
            toggling = on
            perform { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_SetPluginEnabledRequest()
                request.pluginID = self.id
                request.enabled = on
                _ = try await self.model.call(
                    self.server, Procedure.plugins("SetPluginEnabled"), request,
                    as: Mavio_System_V1_SetPluginEnabledResponse.self)
            } done: { [weak self] in
                self?.toggling = nil
            }
        }

        /// Stops the plugin and starts it again, as after an upgrade; one
        /// under development with what was built.
        private func restart() {
            restarting = true
            perform { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_RestartPluginRequest()
                request.pluginID = self.id
                _ = try await self.model.call(
                    self.server, Procedure.plugins("RestartPlugin"), request,
                    as: Mavio_System_V1_RestartPluginResponse.self)
            } done: { [weak self] in
                self?.restarting = false
            }
        }

        private func uninstall() {
            uninstalling = true
            perform { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_UninstallPluginRequest()
                request.pluginID = self.id
                _ = try await self.model.call(
                    self.server, Procedure.plugins("UninstallPlugin"), request,
                    as: Mavio_System_V1_UninstallPluginResponse.self)
            } done: { [weak self] in
                self?.uninstalling = false
            }
        }

        /// Does work, then reads the plugins again and calls done.
        private func perform(
            _ work: @escaping @MainActor () async throws -> Void, done: @escaping @MainActor () -> Void
        ) {
            render()
            run { [weak self] in
                do {
                    try await work()
                    self?.failure = nil
                } catch {
                    self?.failure = error.reason
                }
                await self?.store.refresh()
                done()
                self?.render()
            }
        }

        // MARK: Loading

        private func loadOnce() async {
            let model = model
            let server = server
            async let engine = try? model.call(
                server, Procedure.plugins("GetContainerRuntime"), Mavio_System_V1_GetContainerRuntimeRequest(),
                as: Mavio_System_V1_GetContainerRuntimeResponse.self)
            async let list = try? model.call(
                server, Procedure.proxies("ListProxies"), Mavio_System_V1_ListProxiesRequest(),
                as: Mavio_System_V1_ListProxiesResponse.self)
            // A GPU is offered only where the engine gives containers one.
            gpu = await engine?.runtime.gpu ?? false
            proxies = await list?.proxies
            render()
        }

        /// Reads the logs every few seconds, and the containers, whose
        /// managed dependencies pull and start in the background, while the
        /// plugin is installed.
        private func poll(_ installed: Bool) {
            guard installed != polled else { return }
            polled = installed
            polling.forEach { $0.cancel() }
            polling = []
            guard installed else { return }
            polling.append(
                Task { [weak self] in
                    while !Task.isCancelled {
                        await self?.loadLogs()
                        try? await Task.sleep(for: .seconds(3))
                    }
                })
            polling.append(
                Task { [weak self] in
                    while !Task.isCancelled {
                        await self?.loadContainers()
                        try? await Task.sleep(for: .seconds(5))
                    }
                })
        }

        private func loadLogs() async {
            var request = Mavio_System_V1_ListLogsRequest()
            request.pluginID = id
            request.limit = 200
            do {
                let records = try await model.call(
                    server, Procedure.system("ListLogs"), request, as: Mavio_System_V1_ListLogsResponse.self
                ).records
                guard records != logs || logsFailure != nil else { return }
                logs = records
                logsFailure = nil
            } catch is CancellationError {
                return
            } catch {
                logsFailure = error.reason
            }
            render()
        }

        func loadContainers() async {
            var request = Mavio_System_V1_ListPluginContainersRequest()
            request.pluginID = id
            do {
                let list = try await model.call(
                    server, Procedure.plugins("ListPluginContainers"), request,
                    as: Mavio_System_V1_ListPluginContainersResponse.self
                ).containers
                guard list != containers || containersFailure != nil else { return }
                containers = list
                containersFailure = nil
            } catch is CancellationError {
                return
            } catch {
                containersFailure = error.reason
            }
            render()
        }
    }

    /// A log level's word.
    @MainActor func logLevelName(_ level: Mavio_System_V1_LogLevel) -> String {
        switch level {
        case .debug: MavioStrings.logsDebug
        case .warn: MavioStrings.logsWarn
        case .error: MavioStrings.logsError
        default: MavioStrings.logsInfo
        }
    }

    // MARK: Network

    /// The proxies a plugin or container can reach sites through: none, the
    /// system proxy, then the server's by name, and the one set though the
    /// list lacks it.
    @MainActor func pluginsProxyChoices(_ list: [Mavio_System_V1_Proxy]?, _ current: String) -> [(
        id: String, name: String
    )] {
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

    // MARK: Configuration

    /// An installed plugin's configuration: the form of its schema, or a
    /// link to its own page; the form stays as the plugin is read again.
    @MainActor final class PluginConfiguration {
        /// The section, its heading, card and Save bar.
        let view = NSStackView.vertical([], spacing: MavioSpacing.s8)
        private weak var page: Page?
        private let server: ServerRecord
        private let id: String
        private let changed: @MainActor () async -> Void
        private let theme: Theme
        private let model: AppModel
        private var schema: String?
        private var settingsPage = ""
        private var containers: [Mavio_System_V1_PluginContainer]?
        private var parsed: PluginsSchema?
        private var saved: PluginsJSON?
        private var value: PluginsJSON?
        private var options: [String: [PluginsOption]] = [:]
        private var loadFailure: String?
        private var saving = false
        private var done = false
        private var failure: String?
        private var form: PluginsSchemaForm?
        private var tasks: [Task<Void, Never>] = []
        private lazy var heading = NSStackView.horizontal([], spacing: MavioSpacing.s12)
        private lazy var card = FormSection(theme: theme)
        private lazy var saveButton = MavioButton(MavioStrings.actionSave, kind: .primary, theme: theme) {
            [weak self] in self?.save()
        }
        private lazy var doneLabel = NSTextField.label(
            MavioStrings.pluginsConfigurationSaved, theme.fonts.callout, color: theme.colors.textPrimary)
        private lazy var failureLabel = note("", theme.fonts.callout, theme.colors.danger, selectable: true)
        private lazy var saveBar: NSView = {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(
                bar([saveButton, doneLabel, failureLabel]),
                insets: NSEdgeInsets(top: MavioSpacing.s4, left: MavioSpacing.s4, bottom: 0, right: MavioSpacing.s4))
            return holder
        }()
        private lazy var loadNote = plainNote("", theme.colors.danger, theme: theme)

        /// The configuration of the plugin of id on page, calling changed
        /// once saved.
        init(page: Page, server: ServerRecord, id: String, changed: @escaping @MainActor () async -> Void) {
            self.page = page
            self.server = server
            self.id = id
            self.changed = changed
            theme = page.theme
            model = page.model
            let views: [NSView] = [heading, card, saveBar, loadNote]
            view.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        /// Shows the configuration of schema, reading it again as the schema
        /// changes, and the plugin's own settingsPage; the switches needing
        /// containers wait for them.
        func update(schema: String, settingsPage: String, containers: [Mavio_System_V1_PluginContainer]?) {
            let before = self.containers.map { $0.map(\.status.mode) }
            self.containers = containers
            if settingsPage != self.settingsPage || heading.arrangedSubviews.isEmpty {
                self.settingsPage = settingsPage
                drawHeading()
            }
            if schema != self.schema {
                self.schema = schema
                tasks.forEach { $0.cancel() }
                tasks = [Task { [weak self] in await self?.load(schema) }]
            } else if before != containers.map({ $0.map(\.status.mode) }) {
                form?.refresh()
            }
            render()
        }

        /// The plugin's own settings page on the server.
        private var own: URL? {
            guard !settingsPage.isEmpty else { return nil }
            let base = server.address.hasSuffix("/") ? String(server.address.dropLast()) : server.address
            let pid = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(["/"])) ?? id
            return URL(string: "\(base)/plugins/\(pid)/\(settingsPage)")
        }

        private func drawHeading() {
            var views: [NSView] = [
                NSTextField.label(
                    MavioStrings.pluginsConfiguration, theme.fonts.headline, color: theme.colors.textPrimary),
                Spacer(),
            ]
            // Television mode opens no web pages.
            if let own, !theme.isTV {
                views.append(
                    MavioButton(MavioStrings.pluginsOpenSettingsPage, icon: "external", kind: .link, theme: theme) {
                        NSWorkspace.shared.open(own)
                    })
            }
            heading.setViews(views)
            heading.edgeInsets = NSEdgeInsets(top: 0, left: MavioSpacing.s4, bottom: 0, right: MavioSpacing.s4)
        }

        // Saving refreshes saved, which the form then matches.
        private var isChanged: Bool { (value ?? .null).text != (saved ?? .null).text }

        private func render() {
            let ready = parsed != nil && saved != nil
            card.isHidden = !ready
            if let loadFailure {
                (loadNote.subviews.first as? NSTextField)?.stringValue = loadFailure
                loadNote.isHidden = false
                saveBar.isHidden = true
            } else {
                loadNote.isHidden = true
                saveBar.isHidden = !ready
            }
            saveButton.title = saving ? "…" : MavioStrings.actionSave
            saveButton.isEnabled = isChanged && !saving
            doneLabel.isHidden = !(done && !isChanged)
            failureLabel.stringValue = failure ?? ""
            failureLabel.isHidden = failure == nil
        }

        /// Whether a container dependency of the plugin is not set up, by
        /// its name.
        private func unset(_ name: String) -> Bool {
            guard let c = containers?.first(where: { $0.dependency.name == name }) else { return false }
            return c.status.mode == .unspecified
        }

        private func load(_ schema: String) async {
            guard let s = PluginsSchema(parsing: schema), s.hasProperties else {
                parsed = nil
                form = nil
                card.setRows([])
                render()
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
            if saved != nil {
                let made = PluginsSchemaForm(
                    schema: s, value: value, options: options, theme: theme,
                    unset: { [weak self] in self?.unset($0) ?? false }
                ) { [weak self] v in
                    self?.value = v
                    self?.done = false
                    self?.render()
                }
                form = made
                card.setRows([made])
            }
            render()
            for source in s.optionSources {
                var r = Mavio_System_V1_ListPluginConfigOptionsRequest()
                r.pluginID = id
                r.source = source
                if let list = try? await model.call(
                    server, Procedure.plugins("ListPluginConfigOptions"), r,
                    as: Mavio_System_V1_ListPluginConfigOptionsResponse.self
                ).options {
                    guard !Task.isCancelled else { return }
                    options[source] = list
                    form?.options = options
                }
            }
        }

        private func save() {
            guard !saving else { return }
            saving = true
            done = false
            render()
            var request = Mavio_System_V1_SetPluginConfigRequest()
            request.pluginID = id
            request.configJson = (value ?? .object(PluginsJSONObject())).text
            tasks.append(
                Task { [weak self] in
                    guard let self else { return }
                    do {
                        // The configuration stored holds the IDs the server generated.
                        let stored = try await self.model.call(
                            self.server, Procedure.plugins("SetPluginConfig"), request,
                            as: Mavio_System_V1_SetPluginConfigResponse.self
                        ).configJson
                        let v = PluginsJSON(parsing: stored) ?? self.value
                        self.saved = v
                        self.value = v
                        self.form?.value = v
                        self.done = true
                        self.failure = nil
                    } catch {
                        self.failure = error.reason
                    }
                    self.saving = false
                    self.render()
                    await self.changed()
                })
        }
    }
#endif
