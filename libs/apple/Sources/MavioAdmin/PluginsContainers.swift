#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// An installed plugin's container dependencies, each in its own group:
    /// how it runs, its state, and the commands running it elsewhere.
    struct PluginsContainers: View {
        let server: ServerRecord
        let id: String
        let containers: [Mavio_System_V1_PluginContainer]?
        let failure: String?
        /// Whether the container engine gives containers a GPU.
        let gpu: Bool
        let proxies: [Mavio_System_V1_Proxy]?
        let changed: () async -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            let list = containers ?? []
            if list.isEmpty {
                Section {
                } header: {
                    Text(MavioStrings.pluginsContainers)
                } footer: {
                    if let failure { Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger) }
                }
            }
            ForEach(Array(list.enumerated()), id: \.element.dependency.name) { i, c in
                ContainerDependency(
                    server: server, id: id, c: c, gpu: gpu, proxies: proxies, first: i == 0,
                    failure: i == list.count - 1 ? failure : nil, changed: changed)
            }
        }
    }

    /// A container dependency's state, with how much of its image is downloaded.
    func pluginsContainerState(_ st: Mavio_Plugin_V1_ContainerStatus) -> String {
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

    /// A container dependency's group.
    private struct ContainerDependency: View {
        let server: ServerRecord
        let id: String
        let c: Mavio_System_V1_PluginContainer
        let gpu: Bool
        let proxies: [Mavio_System_V1_Proxy]?
        /// Whether it is the first, under the Containers heading.
        let first: Bool
        /// Why the list could not be read again, under the last.
        let failure: String?
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var chosen: Mavio_Plugin_V1_ContainerMode
        @State private var address: String
        @State private var saving = false
        /// The proxy being saved.
        @State private var savingProxy: String?
        @State private var recreating: String?
        @State private var setFailure: String?

        init(
            server: ServerRecord, id: String, c: Mavio_System_V1_PluginContainer, gpu: Bool,
            proxies: [Mavio_System_V1_Proxy]?, first: Bool, failure: String?, changed: @escaping () async -> Void
        ) {
            (self.server, self.id, self.c, self.gpu, self.proxies) = (server, id, c, gpu, proxies)
            (self.first, self.failure, self.changed) = (first, failure, changed)
            let mode = c.hasStatus ? c.status.mode : .unspecified
            _chosen = State(initialValue: mode)
            _address = State(initialValue: mode == .external ? c.status.address : "")
        }

        private var d: Mavio_Plugin_V1_ContainerDependency { c.dependency }
        private var mode: Mavio_Plugin_V1_ContainerMode { c.hasStatus ? c.status.mode : .unspecified }

        var body: some View {
            Section {
                modeRow
                    .confirmationDialog(
                        MavioStrings.pluginsContainerProxyTitle(name: d.name),
                        isPresented: Binding(get: { recreating != nil }, set: { if !$0 { recreating = nil } }),
                        titleVisibility: .visible
                    ) {
                        Button(MavioStrings.pluginsContainerRecreate) {
                            if let recreating { setProxy(recreating) }
                        }
                    } message: {
                        Text(MavioStrings.pluginsContainerProxyMessage)
                    }
                if chosen == .managed && d.gpu != .unspecified && gpu {
                    Toggle(
                        MavioStrings.pluginsContainerUseGpu,
                        isOn: Binding(get: { c.useGpu }, set: { set(.managed, useGpu: $0) })
                    )
                    .disabled(saving || d.gpu == .required)
                }
                if chosen == .managed && mode == .managed {
                    // A container given the proxy of each request reaches sites directly.
                    if !d.direct { proxyRow }
                    if !c.command.isEmpty {
                        Disclosure(label: MavioStrings.pluginsCommand, text: c.command)
                    }
                }
                if c.hasStatus && !c.status.error.isEmpty {
                    Disclosure(label: MavioStrings.pluginsContainerError, text: c.status.error, danger: true)
                }
                if chosen == .external {
                    external
                }
                if let setFailure {
                    Text(setFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
            } header: {
                if first { Text(MavioStrings.pluginsContainers) }
            } footer: {
                if let failure { Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger) }
            }
        }

        private var modeRow: some View {
            let state = c.hasStatus ? pluginsContainerState(c.status) : ""
            let detail = [d.description_p, state].filter { !$0.isEmpty }.joined(separator: " · ")
            return HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: MavioSpacing.s8) {
                        Text(d.name).type(theme.type.body)
                        Text(d.required ? MavioStrings.pluginsContainerRequired : MavioStrings.pluginsContainerOptional)
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                            .padding(.horizontal, MavioSpacing.s8)
                            .padding(.vertical, 2)
                            .background(theme.colors.fill, in: .capsule)
                    }
                    if !detail.isEmpty {
                        Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                Spacer(minLength: 0)
                Picker(
                    MavioStrings.pluginsContainerMode,
                    selection: Binding(
                        get: { chosen },
                        set: { m in
                            chosen = m
                            if m != .external {
                                set(m, useGpu: m == .managed && (d.gpu == .required || (gpu && c.useGpu)))
                            }
                        })
                ) {
                    Text(MavioStrings.pluginsContainerUnset).tag(Mavio_Plugin_V1_ContainerMode.unspecified)
                    Text(MavioStrings.pluginsContainerManaged).tag(Mavio_Plugin_V1_ContainerMode.managed)
                    Text(MavioStrings.pluginsContainerExternal).tag(Mavio_Plugin_V1_ContainerMode.external)
                }
                .labelsHidden()
                .pickerStyle(.menu)
                .fixedSize()
                .disabled(saving)
            }
        }

        private var proxyRow: some View {
            let proxy = savingProxy ?? c.proxy
            return HStack(spacing: MavioSpacing.s12) {
                RowLabel(label: MavioStrings.pluginsProxy, detail: MavioStrings.pluginsContainerProxyDetail)
                Spacer(minLength: 0)
                Picker(
                    MavioStrings.pluginsProxy,
                    selection: Binding(
                        get: { proxy },
                        set: { p in
                            let was = pluginsProxyDirect(proxies, c.proxy)
                            let next = pluginsProxyDirect(proxies, p)
                            if let was, let next, was != next { recreating = p } else { setProxy(p) }
                        })
                ) {
                    ForEach(pluginsProxyChoices(proxies, proxy), id: \.id) { Text($0.name).tag($0.id) }
                }
                .labelsHidden()
                .pickerStyle(.menu)
                .frame(width: 192)
                .disabled(saving)
            }
        }

        @ViewBuilder private var external: some View {
            let suggested = c.hasCommands ? c.commands.address : c.hasGpuCommands ? c.gpuCommands.address : ""
            let trimmed = address.trimmingCharacters(in: .whitespaces)
            let same =
                mode == .external && (trimmed.hasSuffix("/") ? String(trimmed.dropLast()) : trimmed) == c.status.address
            HStack(spacing: MavioSpacing.s12) {
                TextField(
                    MavioStrings.pluginsContainerAddress, text: $address,
                    prompt: Text(suggested.isEmpty ? MavioStrings.pluginsContainerAddress : suggested)
                )
                .fieldLabelHidden()
                .textFieldStyle(.plain)
                .autocorrectionDisabled()
                .onSubmit {
                    if !trimmed.isEmpty && !same && !saving { set(.external, address: trimmed, useGpu: false) }
                }
                Button(MavioStrings.actionSave) { set(.external, address: trimmed, useGpu: false) }
                    .buttonStyle(.bordered)
                    .disabled(trimmed.isEmpty || same || saving)
            }
            Text(MavioStrings.pluginsContainerCommandsHint(address: suggested))
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            if c.hasCommands { commands(MavioStrings.pluginsContainerCommands, c.commands) }
            if c.hasGpuCommands { commands(MavioStrings.pluginsContainerGpuCommands, c.gpuCommands) }
        }

        private func commands(_ title: String, _ cmds: Mavio_System_V1_ContainerCommands) -> some View {
            // A server in a container is joined on its network, which the
            // administrator names.
            let network = cmds.dockerRun.contains("<network>")
            return VStack(alignment: .leading, spacing: 6) {
                Text(title).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                Text(
                    network
                        ? MavioStrings.pluginsContainerDockerRunNetwork(network: "<network>")
                        : MavioStrings.pluginsContainerDockerRun
                )
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                Code(text: cmds.dockerRun)
                Text(network ? MavioStrings.pluginsContainerComposeServer : MavioStrings.pluginsContainerCompose)
                    .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                Code(text: cmds.compose)
            }
        }

        // MARK: Actions

        private func set(
            _ m: Mavio_Plugin_V1_ContainerMode, address: String? = nil, useGpu: Bool? = nil, proxy: String? = nil,
            then done: @escaping @MainActor () -> Void = {}
        ) {
            saving = true
            Task {
                var request = Mavio_System_V1_SetPluginContainerRequest()
                request.pluginID = id
                request.name = d.name
                request.mode = m
                request.useGpu = useGpu ?? (gpu && c.useGpu)
                if let address { request.address = address }
                if let proxy { request.proxy = proxy }
                do {
                    _ = try await model.call(
                        server, Procedure.plugins("SetPluginContainer"), request,
                        as: Mavio_System_V1_SetPluginContainerResponse.self)
                    setFailure = nil
                } catch {
                    setFailure = error.reason
                }
                await changed()
                saving = false
                done()
            }
        }

        private func setProxy(_ p: String) {
            savingProxy = p
            set(.managed, proxy: p) {
                savingProxy = nil
                recreating = nil
            }
        }
    }

    /// Text in a box, such as a command, selectable whole.
    private struct Code: View {
        let text: String
        var danger = false
        @Environment(\.theme) private var theme

        var body: some View {
            Text(text)
                .type(theme.type.footnote).monospaced()
                .foregroundStyle(danger ? theme.colors.danger : theme.colors.textPrimary)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(theme.colors.fill, in: .rect(cornerRadius: 6))
        }
    }

    /// Text of a container's, such as its command or error, shown when asked for.
    private struct Disclosure: View {
        let label: String
        let text: String
        var danger = false
        @State private var shown = false
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: 6) {
                Button {
                    shown.toggle()
                } label: {
                    HStack(spacing: MavioSpacing.s4) {
                        Icon("expand", size: 14).rotationEffect(.degrees(shown ? 0 : -90))
                        Text(label).type(theme.type.callout)
                    }
                    .foregroundStyle(theme.colors.textSecondary)
                    .contentShape(.rect)
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(shown ? .isSelected : [])
                if shown { Code(text: text, danger: danger) }
            }
        }
    }
#endif
