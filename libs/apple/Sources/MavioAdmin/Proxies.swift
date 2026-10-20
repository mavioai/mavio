#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftProtobuf
    import SwiftUI

    /// Proxies (docs/clients.md §6.3.30), as the web UI's: the server's
    /// proxies, which the server and its plugins reach sites through, added,
    /// edited, tested and deleted here; the system proxy first, read only; and
    /// the proxies of the server's own requests.
    struct ProxiesView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var proxies: [Mavio_System_V1_Proxy]?
        @State private var failure: String?
        /// The proxy edited, by its ID; "" for a new one.
        @State private var editing: String?
        @State private var settings: Mavio_System_V1_ServerSettings?
        @State private var mavioDev = false

        private var system: Mavio_System_V1_Proxy? { proxies?.first { $0.system } }
        private var own: [Mavio_System_V1_Proxy] { proxies?.filter { !$0.system } ?? [] }

        var body: some View {
            SettingsList(message: MavioStrings.proxiesMessage) {
                if let system {
                    Section {
                        ProxiesLine(
                            server: server, id: system.id,
                            tile: ColorTile(
                                icon: "network",
                                color: Color(hex: system.kind != .unspecified ? 0x007AFF : 0x8E8E93)),
                            name: MavioStrings.proxiesSystem, detail: systemDetail(system))
                    } footer: {
                        Text(MavioStrings.proxiesSystemFooter)
                    }
                }
                Section {
                    if editing == "" {
                        ProxiesForm(server: server, proxy: nil, close: { editing = nil }, changed: load)
                    }
                    ForEach(own, id: \.id) { proxy in
                        if editing == proxy.id {
                            ProxiesForm(server: server, proxy: proxy, close: { editing = nil }, changed: load)
                        } else {
                            ProxiesLine(
                                server: server, id: proxy.id,
                                tile: ColorTile(icon: "network", color: Color(hex: 0x30B0C7)),
                                name: proxy.name,
                                detail: [proxiesAddress(proxy), proxy.username].filter { !$0.isEmpty }
                                    .joined(separator: " · "),
                                disabled: editing != nil, edit: { editing = proxy.id })
                        }
                    }
                    if proxies != nil, own.isEmpty, editing != "" {
                        Text(MavioStrings.proxiesEmpty)
                            .type(theme.type.callout)
                            .foregroundStyle(theme.colors.textSecondary)
                    }
                    if let failure {
                        Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                    if editing == nil, proxies != nil {
                        Button {
                            editing = ""
                        } label: {
                            Label {
                                Text(MavioStrings.proxiesAdd)
                            } icon: {
                                Icon("add", size: 20)
                            }
                            .foregroundStyle(theme.colors.accentText)
                        }
                    }
                } header: {
                    Text(MavioStrings.proxiesList)
                }
                if settings != nil {
                    ProxiesServerRequests(
                        server: server, proxies: proxies, mavioDev: mavioDev, settings: $settings)
                }
            }
            .overlay { if proxies == nil && failure == nil { ProgressView() } }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsProxies)
            .inlineTitle()
            .task { await loadAll() }
        }

        private func systemDetail(_ proxy: Mavio_System_V1_Proxy) -> String {
            guard proxy.kind != .unspecified else { return MavioStrings.proxiesSystemUnset }
            var parts = [proxiesAddress(proxy)]
            if !proxy.bypass.isEmpty {
                parts.append(MavioStrings.proxiesBypass(hosts: proxy.bypass.joined(separator: ", ")))
            }
            return parts.joined(separator: " · ")
        }

        private func loadAll() async {
            async let info = try? model.call(
                server, Procedure.auth("GetAuthInfo"), Mavio_Auth_V1_GetAuthInfoRequest(),
                as: Mavio_Auth_V1_GetAuthInfoResponse.self)
            async let saved = try? model.call(
                server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                as: Mavio_System_V1_GetServerSettingsResponse.self)
            await load()
            mavioDev = await info?.mode == .mavioDev
            if let s = await saved, s.hasSettings { settings = s.settings }
        }

        private func load() async {
            do {
                proxies = try await model.call(
                    server, Procedure.proxies("ListProxies"), Mavio_System_V1_ListProxiesRequest(),
                    as: Mavio_System_V1_ListProxiesResponse.self
                ).proxies
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }

    /// The kinds of proxies, by the scheme of their addresses.
    private let kinds: [(kind: Mavio_System_V1_ProxyKind, scheme: String, label: String)] = [
        (.http, "http", "HTTP"), (.socks4, "socks4", "SOCKS4"), (.socks5, "socks5", "SOCKS5"),
    ]

    /// A proxy's address, kind://host:port.
    private func proxiesAddress(_ p: Mavio_System_V1_Proxy) -> String {
        let scheme = kinds.first { $0.kind == p.kind }?.scheme ?? ""
        let host = p.host.contains(":") ? "[\(p.host)]" : p.host
        return "\(scheme)://\(host):\(p.port)"
    }

    /// The ID of the system proxy, a choice of the server's own requests.
    private let systemProxyId = "system"

    /// What testing a proxy told: its result, or why it could not be run.
    private struct TestOutcome {
        var result: Mavio_System_V1_TestProxyResponse?
        var error: String?
    }

    extension AppModel {
        /// Tests a proxy: a stored one by its ID, or one being edited, which
        /// keeps the stored password when it gives none.
        fileprivate func testProxy(
            _ server: ServerRecord, _ target: Mavio_System_V1_TestProxyRequest.OneOf_Target
        ) async -> TestOutcome {
            var request = Mavio_System_V1_TestProxyRequest()
            request.target = target
            do {
                let result = try await call(
                    server, Procedure.proxies("TestProxy"), request, as: Mavio_System_V1_TestProxyResponse.self)
                return TestOutcome(result: result)
            } catch {
                return TestOutcome(error: error.reason)
            }
        }
    }

    /// A proxy's line: its tile, name and address, editing it when pressed
    /// where it may be, and testing it, with the result under it.
    private struct ProxiesLine<Tile: View>: View {
        let server: ServerRecord
        let id: String
        let tile: Tile
        let name: String
        let detail: String
        var disabled = false
        var edit: (() -> Void)?
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var testing = false
        @State private var outcome: TestOutcome?

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                HStack(spacing: MavioSpacing.s12) {
                    if let edit {
                        Button(action: edit) { label.contentShape(.rect) }
                            .buttonStyle(.plain)
                            .disabled(disabled)
                    } else {
                        label
                    }
                    Button(testing ? "…" : MavioStrings.proxiesTest) {
                        testing = true
                        Task {
                            outcome = await model.testProxy(server, .id(id))
                            testing = false
                        }
                    }
                    .buttonStyle(.bordered)
                    .disabled(testing)
                }
                if let outcome { ProxiesTestResult(outcome: outcome) }
            }
        }

        private var label: some View {
            HStack(spacing: MavioSpacing.s12) {
                tile
                VStack(alignment: .leading, spacing: 2) {
                    Text(name).lineLimit(1)
                    Text(detail)
                        .type(theme.type.footnote)
                        .foregroundStyle(theme.colors.textSecondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer(minLength: 0)
            }
        }
    }

    /// A test's result: how long the site took through the proxy, the exit
    /// address and its country, or why it failed.
    private struct ProxiesTestResult: View {
        let outcome: TestOutcome
        @Environment(\.theme) private var theme

        var body: some View {
            if let error = outcome.error ?? outcome.result.flatMap({ $0.ok ? nil : $0.error }) {
                Text(MavioStrings.proxiesTestFailed(error: error))
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.danger)
            } else if let result = outcome.result {
                Text(success(result))
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.success)
            }
        }

        private func success(_ r: Mavio_System_V1_TestProxyResponse) -> String {
            let ms =
                r.hasLatency ? Int((Double(r.latency.seconds) * 1000 + Double(r.latency.nanos) / 1e6).rounded()) : 0
            var country = r.country
            if country.count == 2, country.allSatisfy({ $0.isASCII && $0.isUppercase }) {
                country = Locale.current.localizedString(forRegionCode: country) ?? country
            }
            return [
                MavioStrings.proxiesTestOk(ms: String(ms)),
                r.exitIp.isEmpty ? "" : MavioStrings.proxiesExit(address: r.exitIp),
                country,
            ]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
        }
    }

    /// What a proxy's form holds.
    private struct Draft {
        var name = ""
        var kind = Mavio_System_V1_ProxyKind.http
        var host = ""
        var port: Int32 = 8080
        var username = ""
        var password = ""
        var clearPassword = false

        var valid: Bool {
            !name.trimmingCharacters(in: .whitespaces).isEmpty
                && !host.trimmingCharacters(in: .whitespaces).isEmpty && (1...65535).contains(port)
        }
    }

    /// The form adding a proxy, or editing one and deleting it: its rows in
    /// the list of proxies, in the place of the proxy's line.
    private struct ProxiesForm: View {
        let server: ServerRecord
        let proxy: Mavio_System_V1_Proxy?
        let close: () -> Void
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var draft = Draft()
        @State private var saving = false
        @State private var removing = false
        @State private var confirming = false
        @State private var failure: String?
        @State private var testing = false
        @State private var outcome: TestOutcome?

        init(
            server: ServerRecord, proxy: Mavio_System_V1_Proxy?, close: @escaping () -> Void,
            changed: @escaping () async -> Void
        ) {
            (self.server, self.proxy, self.close, self.changed) = (server, proxy, close, changed)
            if let proxy {
                _draft = State(
                    initialValue: Draft(
                        name: proxy.name, kind: kinds.contains(where: { $0.kind == proxy.kind }) ? proxy.kind : .http,
                        host: proxy.host, port: proxy.port, username: proxy.username))
            }
        }

        var body: some View {
            TextRow(label: MavioStrings.proxiesName, text: $draft.name)
            Picker(MavioStrings.proxiesKind, selection: $draft.kind) {
                ForEach(kinds, id: \.scheme) { Text($0.label).tag($0.kind) }
            }
            TextRow(label: MavioStrings.proxiesHost, text: $draft.host, prompt: "proxy.lan")
            NumberRow(label: MavioStrings.proxiesPort, value: $draft.port.double, range: 1...65535)
            TextRow(label: MavioStrings.proxiesUsername, text: $draft.username)
            FormRow(label: MavioStrings.proxiesPassword) {
                SecureField(
                    MavioStrings.proxiesPassword, text: $draft.password,
                    prompt: proxy?.hasPassword_p == true && !draft.clearPassword
                        ? Text(MavioStrings.proxiesPasswordKept) : nil
                )
                .fieldLabelHidden()
                .textContentType(.newPassword)
                .multilineTextAlignment(.trailing)
                .frame(maxWidth: 170)
            }
            if proxy?.hasPassword_p == true, draft.password.isEmpty {
                Toggle(MavioStrings.proxiesClearPassword, isOn: $draft.clearPassword)
            }
            HStack(spacing: MavioSpacing.s12) {
                Button(saving ? "…" : MavioStrings.actionSave, action: save)
                    .buttonStyle(.borderedProminent)
                    .disabled(!draft.valid || saving || removing)
                Button(testing ? "…" : MavioStrings.proxiesTest, action: test)
                    .buttonStyle(.bordered)
                    .disabled(!draft.valid || testing)
                Button(MavioStrings.actionCancel, action: close)
                if let proxy {
                    Button(MavioStrings.proxiesRemove, role: .destructive) {
                        failure = nil
                        confirming = true
                    }
                    .confirmationDialog(
                        MavioStrings.proxiesRemoveTitle(name: proxy.name), isPresented: $confirming,
                        titleVisibility: .visible
                    ) {
                        Button(MavioStrings.proxiesRemove, role: .destructive) { remove(proxy) }
                    } message: {
                        Text(MavioStrings.proxiesRemoveMessage)
                    }
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
            }
            if let outcome { ProxiesTestResult(outcome: outcome) }
        }

        /// The proxy as the form holds it.
        private var message: Mavio_System_V1_Proxy {
            var p = Mavio_System_V1_Proxy()
            p.id = proxy?.id ?? ""
            p.name = draft.name.trimmingCharacters(in: .whitespaces)
            p.kind = draft.kind
            p.host = draft.host.trimmingCharacters(in: .whitespaces)
            p.port = draft.port
            p.username = draft.username.trimmingCharacters(in: .whitespaces)
            p.password = draft.password
            return p
        }

        private func save() {
            saving = true
            failure = nil
            Task {
                defer { saving = false }
                do {
                    if proxy != nil {
                        var request = Mavio_System_V1_UpdateProxyRequest()
                        request.proxy = message
                        request.clearPassword_p = draft.clearPassword && draft.password.isEmpty
                        _ = try await model.call(
                            server, Procedure.proxies("UpdateProxy"), request,
                            as: Mavio_System_V1_UpdateProxyResponse.self)
                    } else {
                        var request = Mavio_System_V1_CreateProxyRequest()
                        request.proxy = message
                        _ = try await model.call(
                            server, Procedure.proxies("CreateProxy"), request,
                            as: Mavio_System_V1_CreateProxyResponse.self)
                    }
                    await changed()
                    close()
                } catch {
                    failure = error.reason
                }
            }
        }

        private func test() {
            testing = true
            Task {
                outcome = await model.testProxy(server, .proxy(message))
                testing = false
            }
        }

        private func remove(_ proxy: Mavio_System_V1_Proxy) {
            removing = true
            Task {
                defer { removing = false }
                do {
                    var request = Mavio_System_V1_DeleteProxyRequest()
                    request.id = proxy.id
                    _ = try await model.call(
                        server, Procedure.proxies("DeleteProxy"), request,
                        as: Mavio_System_V1_DeleteProxyResponse.self)
                    await changed()
                    close()
                } catch {
                    failure = error.reason
                }
            }
        }
    }

    /// The proxies of the server's own requests, each saved as it is chosen;
    /// mavio.dev and its certificates only in mavio.dev mode.
    private struct ProxiesServerRequests: View {
        let server: ServerRecord
        let proxies: [Mavio_System_V1_Proxy]?
        let mavioDev: Bool
        @Binding var settings: Mavio_System_V1_ServerSettings?
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        /// The choice being saved, shown until the settings come back.
        @State private var choosing: (key: Request, id: String)?
        @State private var saving = false
        @State private var failure: String?

        /// What the server's own requests reach, as OutboundProxies names it.
        enum Request: CaseIterable {
            case pluginCatalogs, images, mavioDev, acme

            var path: WritableKeyPath<Mavio_System_V1_OutboundProxies, String> {
                switch self {
                case .pluginCatalogs: \.pluginCatalogs
                case .images: \.images
                case .mavioDev: \.mavioDev
                case .acme: \.acme
                }
            }

            var label: String {
                switch self {
                case .pluginCatalogs: MavioStrings.proxiesForCatalogs
                case .images: MavioStrings.proxiesForImages
                case .mavioDev: MavioStrings.proxiesForMavioDev
                case .acme: MavioStrings.proxiesForAcme
                }
            }

            var detail: String {
                switch self {
                case .pluginCatalogs: MavioStrings.proxiesForCatalogsDetail
                case .images: MavioStrings.proxiesForImagesDetail
                case .mavioDev: MavioStrings.proxiesForMavioDevDetail
                case .acme: MavioStrings.proxiesForAcmeDetail
                }
            }

            var onlyMavioDev: Bool { self == .mavioDev || self == .acme }
        }

        var body: some View {
            Section {
                ForEach(Request.allCases.filter { !$0.onlyMavioDev || mavioDev }, id: \.self) { request in
                    let value = value(request)
                    FormRow(label: request.label, detail: request.detail) {
                        Picker(
                            request.label,
                            selection: Binding(get: { value }, set: { choose(request, $0) })
                        ) {
                            ForEach(choices(value), id: \.value) { Text($0.label).tag($0.value) }
                        }
                        .labelsHidden()
                        .frame(width: 192)
                        .disabled(saving)
                    }
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
            } header: {
                Text(MavioStrings.proxiesServerRequests)
            } footer: {
                Text(MavioStrings.proxiesServerRequestsFooter)
            }
        }

        private func value(_ request: Request) -> String {
            if let choosing, choosing.key == request { return choosing.id }
            return settings?.network.proxies[keyPath: request.path] ?? ""
        }

        /// The choices of a proxy: none, the system proxy, then the server's
        /// by name; one chosen but gone by its ID.
        private func choices(_ current: String) -> [(value: String, label: String)] {
            let own = (proxies ?? []).filter { !$0.system }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            var out: [(value: String, label: String)] =
                [("", MavioStrings.proxiesNone), (systemProxyId, MavioStrings.proxiesSystem)]
                + own.map { ($0.id, $0.name) }
            if proxies != nil, !out.contains(where: { $0.value == current }) {
                out.append((current, MavioStrings.proxiesUnknown))
            }
            return out
        }

        private func choose(_ request: Request, _ id: String) {
            choosing = (request, id)
            saving = true
            failure = nil
            Task {
                defer {
                    saving = false
                    choosing = nil
                }
                do {
                    var current = try await model.call(
                        server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                        as: Mavio_System_V1_GetServerSettingsResponse.self
                    ).settings
                    current.network.proxies[keyPath: request.path] = id
                    var update = Mavio_System_V1_UpdateServerSettingsRequest()
                    update.settings = current
                    settings = try await model.call(
                        server, Procedure.system("UpdateServerSettings"), update,
                        as: Mavio_System_V1_UpdateServerSettingsResponse.self
                    ).settings
                } catch {
                    failure = error.reason
                    if let saved = try? await model.call(
                        server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                        as: Mavio_System_V1_GetServerSettingsResponse.self)
                    {
                        settings = saved.settings
                    }
                }
            }
        }
    }
#endif
