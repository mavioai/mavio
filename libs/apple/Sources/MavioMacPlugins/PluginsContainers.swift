#if os(macOS)
    import AppKit
    import MavioMac

    /// An installed plugin's container dependencies, each in its own section:
    /// how it runs, its state, and the commands running it elsewhere. Each
    /// dependency's section, with its chosen mode and typed address, stays
    /// as the containers are read again.
    @MainActor final class PluginContainers {
        /// The sections, one under the other.
        let view = NSStackView.vertical([], spacing: MavioSpacing.s24)
        private weak var page: Page?
        private let server: ServerRecord
        private let id: String
        private let changed: @MainActor () async -> Void
        private var groups: [String: ContainerGroup] = [:]
        private let empty: LiveSection

        /// The container dependencies of the plugin of id on page, calling
        /// changed once one is set.
        init(page: Page, server: ServerRecord, id: String, changed: @escaping @MainActor () async -> Void) {
            self.page = page
            self.server = server
            self.id = id
            self.changed = changed
            empty = LiveSection(MavioStrings.pluginsContainers, theme: page.theme)
        }

        /// Shows containers, or why they could not be read again under the
        /// last; gpu tells whether the container engine gives containers a
        /// GPU, proxies the server's proxies.
        func update(
            containers: [Mavio_System_V1_PluginContainer]?, failure: String?, gpu: Bool,
            proxies: [Mavio_System_V1_Proxy]?
        ) {
            guard let page else { return }
            let list = containers ?? []
            var views: [NSView] = []
            if list.isEmpty {
                empty.footer = failure
                views = [empty]
            }
            var kept: [String: ContainerGroup] = [:]
            for (i, c) in list.enumerated() {
                let name = c.dependency.name
                let first = i == 0
                let group =
                    groups[name].flatMap { $0.first == first ? $0 : nil }
                    ?? ContainerGroup(page: page, server: server, id: id, c: c, first: first, changed: changed)
                group.update(c, gpu: gpu, proxies: proxies, failure: i == list.count - 1 ? failure : nil)
                kept[name] = group
                views.append(group.view)
            }
            groups = kept
            if view.arrangedSubviews.map(ObjectIdentifier.init) != views.map(ObjectIdentifier.init) {
                view.setViews(views)
                for v in views { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
            }
        }
    }

    /// A container dependency's state, with how much of its image is downloaded.
    @MainActor func pluginsContainerState(_ st: Mavio_Plugin_V1_ContainerStatus) -> String {
        if st.state == .pulling && st.queued { return MavioStrings.pluginsContainerQueued }
        let name =
            switch st.state {
            case .pulling: MavioStrings.pluginsContainerPulling
            case .starting: MavioStrings.pluginsContainerStarting
            case .ready: MavioStrings.pluginsContainerReady
            case .unhealthy: MavioStrings.pluginsContainerUnhealthy
            default: MavioStrings.pluginsContainerUnavailable
            }
        if st.state == .pulling && st.progress > 0 { return "\(name) \(Int(st.progress * 100))%" }
        return name
    }

    /// A container dependency's section: its mode, GPU, folders, peer port
    /// and proxy, its command and error, and, run elsewhere, its address and
    /// the commands running it.
    @MainActor private final class ContainerGroup {
        let view: LiveSection
        /// Whether it is the first, under the Containers heading.
        let first: Bool
        private weak var page: Page?
        private let server: ServerRecord
        private let id: String
        private let changed: @MainActor () async -> Void
        private let theme: Theme
        private var c: Mavio_System_V1_PluginContainer
        private var gpu = false
        private var proxies: [Mavio_System_V1_Proxy]?
        private var chosen: Mavio_Plugin_V1_ContainerMode
        private var saving = false
        /// The proxy being saved.
        private var savingProxy: String?
        private var setFailure: String?
        private var tasks: [Task<Void, Never>] = []

        private let addressField: InputField
        private lazy var saveButton = MavioButton(MavioStrings.actionSave, kind: .secondary, theme: theme) {
            [weak self] in self?.saveAddress()
        }
        private lazy var addressRow = ContentRow(
            bar([addressField, saveButton]), theme: theme)
        private var commandView: (String, DisclosureView)?
        private var errorView: (String, DisclosureView)?
        private var hintRow: (String, NSView)?
        private var commandsRows: [String: (Mavio_System_V1_ContainerCommands, NSView)] = [:]
        /// The fields of the server's folders it mounts, by the folder's name,
        /// and of the port it publishes for peers, saved together.
        private var folderFields: [String: InputField] = [:]
        private var folderRows: [String: ContentRow] = [:]
        private let peerPortField: InputField
        private lazy var peerPortRow: FormRow = {
            peerPortField.widthAnchor.constraint(equalToConstant: 96).isActive = true
            return controlRow(
                MavioStrings.pluginsContainerPeerPort, subtitle: MavioStrings.pluginsContainerPeerPortDetail,
                control: peerPortField, theme: theme)
        }()
        private lazy var foldersHintRow = ContentRow(
            note(MavioStrings.pluginsContainerFoldersHint, theme.fonts.footnote, theme.colors.textSecondary),
            theme: theme)
        private lazy var placesButton = MavioButton(MavioStrings.actionSave, kind: .secondary, theme: theme) {
            [weak self] in self?.savePlaces()
        }
        private lazy var placesRow = ContentRow(bar([placesButton]), theme: theme)

        init(
            page: Page, server: ServerRecord, id: String, c: Mavio_System_V1_PluginContainer, first: Bool,
            changed: @escaping @MainActor () async -> Void
        ) {
            self.page = page
            self.server = server
            self.id = id
            self.c = c
            self.first = first
            self.changed = changed
            theme = page.theme
            view = LiveSection(first ? MavioStrings.pluginsContainers : nil, theme: page.theme)
            let mode = c.hasStatus ? c.status.mode : .unspecified
            chosen = mode
            addressField = InputField(text: mode == .external ? c.status.address : "")
            peerPortField = InputField(text: String(c.peerPort))
            addressField.setAccessibilityLabel(MavioStrings.pluginsContainerAddress)
            addressField.onChange = { [weak self] _ in self?.render() }
            addressField.onSubmit = { [weak self] in
                guard let self, self.canSave else { return }
                self.saveAddress()
            }
            peerPortField.setAccessibilityLabel(MavioStrings.pluginsContainerPeerPort)
            peerPortField.onChange = { [weak self] _ in self?.render() }
            peerPortField.onSubmit = { [weak self] in
                guard let self, self.canSavePlaces else { return }
                self.savePlaces()
            }
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        private var d: Mavio_Plugin_V1_ContainerDependency { c.dependency }
        private var mode: Mavio_Plugin_V1_ContainerMode { c.hasStatus ? c.status.mode : .unspecified }
        private var trimmed: String { addressField.text.trimmingCharacters(in: .whitespaces) }
        private var canSave: Bool {
            let same =
                mode == .external && (trimmed.hasSuffix("/") ? String(trimmed.dropLast()) : trimmed) == c.status.address
            return !trimmed.isEmpty && !same && !saving
        }

        /// Whether every folder of the dependency has a folder of the server
        /// saved, which a managed container needs to run.
        private var foldersSet: Bool { d.folders.allSatisfy { !(c.folders[$0.name] ?? "").isEmpty } }
        /// The server's folder typed for the dependency's folder of name.
        private func folderText(_ name: String) -> String {
            (folderFields[name]?.text ?? c.folders[name] ?? "").trimmingCharacters(in: .whitespaces)
        }
        private var peerPortText: String { peerPortField.text.trimmingCharacters(in: .whitespaces) }
        private var peerPortValid: Bool {
            !peerPortText.isEmpty && peerPortText.allSatisfy(\.isASCII) && peerPortText.allSatisfy(\.isNumber)
                && (Int(peerPortText) ?? .max) <= 65535
        }
        /// Whether the folders typed are absolute paths, of Unix or Windows,
        /// and the peer port a port.
        private var placesValid: Bool {
            d.folders.allSatisfy { f in
                folderText(f.name).wholeMatch(of: /(\/|[A-Za-z]:[\\\/]).*/) != nil
            } && (d.peerPort == 0 || peerPortValid)
        }
        /// Whether the folders or the peer port typed differ from those saved.
        private var placesChanged: Bool {
            d.folders.contains { folderText($0.name) != (c.folders[$0.name] ?? "") }
                || (d.peerPort > 0 && peerPortText != String(c.peerPort))
        }
        private var canSavePlaces: Bool {
            placesValid && (mode != .managed || placesChanged) && !saving
        }

        /// Shows the container as read, gpu telling whether the engine gives
        /// containers a GPU, and failure, why the list could not be read
        /// again.
        func update(
            _ c: Mavio_System_V1_PluginContainer, gpu: Bool, proxies: [Mavio_System_V1_Proxy]?, failure: String?
        ) {
            self.c = c
            self.gpu = gpu
            self.proxies = proxies
            view.footer = failure
            render()
        }

        private func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        // MARK: Drawing

        private func render() {
            var rows: [NSView] = [modeRow()]
            if chosen == .managed && d.gpu != .unspecified && gpu {
                rows.append(
                    toggleRow(
                        MavioStrings.pluginsContainerUseGpu, isOn: c.useGpu,
                        enabled: !saving && d.gpu != .required, theme: theme
                    ) { [weak self] on in self?.set(.managed, useGpu: on) })
            }
            if chosen == .managed && (!d.folders.isEmpty || d.peerPort > 0) { rows += places() }
            if chosen == .managed && mode == .managed {
                // A container given the proxy of each request reaches sites directly.
                if !d.direct { rows.append(proxyRow()) }
                if !c.command.isEmpty {
                    if commandView?.0 != c.command {
                        commandView = (
                            c.command, DisclosureView(MavioStrings.pluginsCommand, text: c.command, theme: theme)
                        )
                    }
                    if let commandView { rows.append(row(commandView.1)) }
                }
            }
            if c.hasStatus && !c.status.error.isEmpty {
                if errorView?.0 != c.status.error {
                    errorView = (
                        c.status.error,
                        DisclosureView(
                            MavioStrings.pluginsContainerError, text: c.status.error, danger: true, theme: theme)
                    )
                }
                if let errorView { rows.append(row(errorView.1)) }
            }
            if chosen == .external { rows += external() }
            if let setFailure { rows.append(noteRow(setFailure, theme.colors.danger, theme: theme)) }
            view.setRows(rows)
        }

        /// A view in a row of its own, kept as long as the view is.
        private var holders: [ObjectIdentifier: ContentRow] = [:]
        private func row(_ v: NSView) -> ContentRow {
            if let r = holders[ObjectIdentifier(v)] { return r }
            let r = ContentRow(v, theme: theme)
            holders[ObjectIdentifier(v)] = r
            return r
        }

        /// The dependency's name, whether it is required, its description
        /// and state, and how it runs: not set up, managed by the server, or
        /// run elsewhere.
        private func modeRow() -> NSView {
            let state = c.hasStatus ? pluginsContainerState(c.status) : ""
            let detail = [d.description_p, state].filter { !$0.isEmpty }.joined(separator: " · ")
            let name = NSTextField.label(d.name, theme.fonts.body, color: theme.colors.textPrimary)
            let kind = capsule(
                d.required ? MavioStrings.pluginsContainerRequired : MavioStrings.pluginsContainerOptional,
                theme.fonts.footnote, color: theme.colors.textSecondary, fill: theme.colors.fill)
            var labels: [NSView] = [NSStackView.horizontal([name, kind], spacing: MavioSpacing.s8)]
            if !detail.isEmpty { labels.append(note(detail, theme.fonts.footnote, theme.colors.textSecondary)) }
            let column = NSStackView.vertical(labels, spacing: 2)
            column.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            let modes: [(String, Mavio_Plugin_V1_ContainerMode)] = [
                (MavioStrings.pluginsContainerUnset, .unspecified),
                (MavioStrings.pluginsContainerManaged, .managed),
                (MavioStrings.pluginsContainerExternal, .external),
            ]
            let popup = ActionPopUp(options: modes.map(\.0), selected: modes.firstIndex { $0.1 == chosen } ?? 0) {
                [weak self] i in
                guard let self else { return }
                let m = modes[i].1
                self.chosen = m
                // One whose folders are not chosen yet runs once they are saved.
                if m != .external && (m != .managed || self.foldersSet) {
                    self.set(m, useGpu: m == .managed && (self.d.gpu == .required || (self.gpu && self.c.useGpu)))
                } else {
                    self.render()
                }
            }
            popup.isEnabled = !saving
            popup.setAccessibilityLabel(MavioStrings.pluginsContainerMode)
            popup.setContentCompressionResistancePriority(.required, for: .horizontal)
            return ContentRow(
                NSStackView.horizontal([column, Spacer(), focusable(popup, theme: theme)], spacing: MavioSpacing.s12),
                theme: theme)
        }

        /// The folders of the server the managed container mounts, each under
        /// its path in the container, the port it publishes for peers, and
        /// the button saving them.
        private func places() -> [NSView] {
            var rows: [NSView] = []
            if !d.folders.isEmpty { rows.append(foldersHintRow) }
            for f in d.folders { rows.append(folderRow(f)) }
            if d.peerPort > 0 {
                peerPortField.isEnabled = !saving
                rows.append(peerPortRow)
            }
            placesButton.isEnabled = canSavePlaces
            rows.append(placesRow)
            return rows
        }

        /// A folder's path in the container, what it is for, and the field
        /// of the server's folder mounted there.
        private func folderRow(_ f: Mavio_Plugin_V1_ContainerFolder) -> ContentRow {
            let field: InputField
            if let kept = folderFields[f.name] {
                field = kept
            } else {
                field = InputField(text: c.folders[f.name] ?? "", placeholder: MavioStrings.pluginsContainerFolderPath)
                field.setAccessibilityLabel(f.path)
                field.onChange = { [weak self] _ in self?.render() }
                field.onSubmit = { [weak self] in
                    guard let self, self.canSavePlaces else { return }
                    self.savePlaces()
                }
                folderFields[f.name] = field
            }
            field.isEnabled = !saving
            if let kept = folderRows[f.name] { return kept }
            let path = NSTextField.label(f.path, theme.fonts.callout, color: theme.colors.textPrimary)
            path.font = .monospacedSystemFont(ofSize: theme.fonts.callout.size, weight: .regular)
            path.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            var labels: [NSView] = [path]
            if !f.description_p.isEmpty {
                labels.append(note("· \(f.description_p)", theme.fonts.callout, theme.colors.textSecondary))
            }
            let title = NSStackView.horizontal(labels, spacing: MavioSpacing.s4)
            title.alignment = .firstBaseline
            let stack = NSStackView.vertical([title, field], spacing: 6)
            for v in [title, field] as [NSView] { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let r = ContentRow(stack, theme: theme)
            folderRows[f.name] = r
            return r
        }

        /// The proxy the container reaches sites through; going from direct
        /// to proxied or back recreates it, once confirmed.
        private func proxyRow() -> NSView {
            let proxy = savingProxy ?? c.proxy
            let choices = pluginsProxyChoices(proxies, proxy)
            let popup = ActionPopUp(
                options: choices.map(\.name), selected: choices.firstIndex { $0.id == proxy } ?? 0
            ) { [weak self] i in
                guard let self, let page = self.page else { return }
                let p = choices[i].id
                let was = pluginsProxyDirect(self.proxies, self.c.proxy)
                let next = pluginsProxyDirect(self.proxies, p)
                if let was, let next, was != next {
                    confirm(
                        MavioStrings.pluginsContainerProxyTitle(name: self.d.name),
                        message: MavioStrings.pluginsContainerProxyMessage,
                        action: MavioStrings.pluginsContainerRecreate, destructive: false, in: page.view
                    ) { [weak self] in self?.setProxy(p) }
                    // Until confirmed, the choice stays as it was.
                    self.render()
                } else {
                    self.setProxy(p)
                }
            }
            popup.widthAnchor.constraint(equalToConstant: 192).isActive = true
            popup.setAccessibilityLabel(MavioStrings.pluginsProxy)
            return controlRow(
                MavioStrings.pluginsProxy, subtitle: MavioStrings.pluginsContainerProxyDetail, control: popup,
                enabled: !saving, theme: theme)
        }

        /// The address of the container run elsewhere, and the commands that
        /// run it.
        private func external() -> [NSView] {
            let suggested = c.hasCommands ? c.commands.address : c.hasGpuCommands ? c.gpuCommands.address : ""
            addressField.field.placeholderString = suggested.isEmpty ? MavioStrings.pluginsContainerAddress : suggested
            addressField.isEnabled = !saving
            saveButton.isEnabled = canSave
            var rows: [NSView] = [addressRow]
            if hintRow?.0 != suggested {
                hintRow = (
                    suggested,
                    ContentRow(
                        note(
                            MavioStrings.pluginsContainerCommandsHint(address: suggested), theme.fonts.footnote,
                            theme.colors.textSecondary, selectable: true), theme: theme)
                )
            }
            if let hintRow { rows.append(hintRow.1) }
            if c.hasCommands { rows.append(commands(MavioStrings.pluginsContainerCommands, c.commands)) }
            if c.hasGpuCommands { rows.append(commands(MavioStrings.pluginsContainerGpuCommands, c.gpuCommands)) }
            return rows
        }

        private func commands(_ title: String, _ cmds: Mavio_System_V1_ContainerCommands) -> NSView {
            if let kept = commandsRows[title], kept.0 == cmds { return kept.1 }
            // A server in a container is joined on its network, which the
            // administrator names.
            let network = cmds.dockerRun.contains("<network>")
            let views: [NSView] = [
                note(title, theme.fonts.callout, theme.colors.textSecondary),
                note(
                    network
                        ? MavioStrings.pluginsContainerDockerRunNetwork(network: "<network>")
                        : MavioStrings.pluginsContainerDockerRun, theme.fonts.footnote, theme.colors.textSecondary),
                codeBox(cmds.dockerRun, theme: theme),
                note(
                    network ? MavioStrings.pluginsContainerComposeServer : MavioStrings.pluginsContainerCompose,
                    theme.fonts.footnote, theme.colors.textSecondary),
                codeBox(cmds.compose, theme: theme),
            ]
            let stack = NSStackView.vertical(views, spacing: 6)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let r = ContentRow(stack, theme: theme)
            commandsRows[title] = (cmds, r)
            return r
        }

        // MARK: Actions

        private func saveAddress() {
            set(.external, address: trimmed, useGpu: false)
        }

        /// Saves the folders and the peer port, running the container once
        /// its folders are all chosen.
        private func savePlaces() {
            set(
                .managed, useGpu: d.gpu == .required || (gpu && c.useGpu),
                folders: Dictionary(uniqueKeysWithValues: d.folders.map { ($0.name, folderText($0.name)) }),
                peerPort: d.peerPort > 0 ? UInt32(peerPortText) : nil)
        }

        private func set(
            _ m: Mavio_Plugin_V1_ContainerMode, address: String? = nil, useGpu: Bool? = nil, proxy: String? = nil,
            folders: [String: String]? = nil, peerPort: UInt32? = nil,
            then done: @escaping @MainActor () -> Void = {}
        ) {
            guard let page else { return }
            saving = true
            render()
            var request = Mavio_System_V1_SetPluginContainerRequest()
            request.pluginID = id
            request.name = d.name
            request.mode = m
            request.useGpu = useGpu ?? (gpu && c.useGpu)
            if let address { request.address = address }
            if let proxy { request.proxy = proxy }
            if let folders { request.folders = folders }
            if let peerPort { request.peerPort = peerPort }
            let model = page.model
            let server = server
            run { [weak self] in
                do {
                    _ = try await model.call(
                        server, Procedure.plugins("SetPluginContainer"), request,
                        as: Mavio_System_V1_SetPluginContainerResponse.self)
                    self?.setFailure = nil
                } catch {
                    self?.setFailure = error.reason
                }
                await self?.changed()
                self?.saving = false
                done()
                self?.render()
            }
        }

        private func setProxy(_ p: String) {
            savingProxy = p
            set(.managed, proxy: p) { [weak self] in self?.savingProxy = nil }
        }
    }
#endif
