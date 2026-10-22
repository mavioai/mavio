#if os(macOS)
    import AppKit
    import MavioMac

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

    /// What a proxy's form holds.
    private struct Draft {
        var name = ""
        var kind = Mavio_System_V1_ProxyKind.http
        var host = ""
        var port: Int32 = 8080
        var username = ""
        var password = ""
        var clearPassword = false

        /// Whether the proxy can be saved and tested: named, with a host
        /// and a port.
        var valid: Bool {
            !name.trimmingCharacters(in: .whitespaces).isEmpty
                && !host.trimmingCharacters(in: .whitespaces).isEmpty && (1...65535).contains(port)
        }
    }

    /// What the server's own requests reach, as OutboundProxies names it.
    private enum Request: CaseIterable {
        case pluginCatalogs, images, mavioDev, acme

        /// The field of OutboundProxies holding the request's proxy.
        var path: WritableKeyPath<Mavio_System_V1_OutboundProxies, String> {
            switch self {
            case .pluginCatalogs: \.pluginCatalogs
            case .images: \.images
            case .mavioDev: \.mavioDev
            case .acme: \.acme
            }
        }

        /// The request's name.
        @MainActor var label: String {
            switch self {
            case .pluginCatalogs: MavioStrings.proxiesForCatalogs
            case .images: MavioStrings.proxiesForImages
            case .mavioDev: MavioStrings.proxiesForMavioDev
            case .acme: MavioStrings.proxiesForAcme
            }
        }

        /// What the request reaches.
        @MainActor var detail: String {
            switch self {
            case .pluginCatalogs: MavioStrings.proxiesForCatalogsDetail
            case .images: MavioStrings.proxiesForImagesDetail
            case .mavioDev: MavioStrings.proxiesForMavioDevDetail
            case .acme: MavioStrings.proxiesForAcmeDetail
            }
        }

        /// Whether the request is made only in mavio.dev mode.
        var onlyMavioDev: Bool { self == .mavioDev || self == .acme }
    }

    /// Proxies (docs/clients.md §6.3.30), as the web UI's: the server's
    /// proxies, which the server and its plugins reach sites through, added,
    /// edited, tested and deleted here; the system proxy first, read only;
    /// and the proxies of the server's own requests, each saved as it is
    /// chosen.
    final class ProxiesPage: FormController {
        private var proxies: [Mavio_System_V1_Proxy]?
        private var failure: String?
        /// The proxy edited, by its ID; "" for a new one.
        private var editing: String?
        private var draft = Draft()
        private var saving = false
        private var removing = false
        private var formFailure: String?
        private var formTesting = false
        private var formOutcome: TestOutcome?
        /// The lines' tests, by their proxies' IDs.
        private var testing: Set<String> = []
        private var outcomes: [String: TestOutcome] = [:]
        private var settings: Mavio_System_V1_OutboundProxies?
        private var mavioDev = false
        /// The choice being saved, shown until the settings come back.
        private var choosing: (key: Request, id: String)?
        private var choosingFailure: String?
        private weak var saveButton: MavioButton?
        private weak var testButton: MavioButton?

        override func viewDidLoad() {
            title = MavioStrings.settingsProxies
            message = MavioStrings.proxiesMessage
            super.viewDidLoad()
            run { [weak self] in await self?.loadAll() }
        }

        private var system: Mavio_System_V1_Proxy? { proxies?.first { $0.system } }
        private var own: [Mavio_System_V1_Proxy] { proxies?.filter { !$0.system } ?? [] }

        override func sections() -> [NSView] {
            if proxies == nil && failure == nil { return [card([spinnerRow()])] }
            var views: [NSView] = []
            if let system {
                let tint: UInt32 = system.kind != .unspecified ? 0x007AFF : 0x8E8E93
                views.append(
                    card(
                        nil, footer: MavioStrings.proxiesSystemFooter,
                        line(id: system.id, tint: tint, name: MavioStrings.proxiesSystem, detail: systemDetail(system)))
                )
            }
            var rows: [NSView] = []
            if editing == "" { rows += form(nil) }
            for proxy in own {
                if editing == proxy.id {
                    rows += form(proxy)
                } else {
                    let detail = [proxiesAddress(proxy), proxy.username].filter { !$0.isEmpty }.joined(separator: " · ")
                    var edit: (@MainActor () -> Void)?
                    if editing == nil {
                        let id = proxy.id
                        edit = { [weak self] in self?.edit(id) }
                    }
                    rows += line(id: proxy.id, tint: 0x30B0C7, name: proxy.name, detail: detail, edit: edit)
                }
            }
            if proxies != nil, own.isEmpty, editing != "" { rows.append(note(MavioStrings.proxiesEmpty)) }
            if let failure { rows.append(note(failure, failed: true)) }
            if editing == nil, proxies != nil {
                rows.append(addRow(MavioStrings.proxiesAdd) { [weak self] in self?.edit("") })
            }
            views.append(card(MavioStrings.proxiesList, rows))
            if settings != nil { views.append(serverRequests()) }
            return views
        }

        /// The system proxy's address and the hosts it bypasses, or that it
        /// is not set.
        private func systemDetail(_ proxy: Mavio_System_V1_Proxy) -> String {
            guard proxy.kind != .unspecified else { return MavioStrings.proxiesSystemUnset }
            var parts = [proxiesAddress(proxy)]
            if !proxy.bypass.isEmpty {
                parts.append(MavioStrings.proxiesBypass(hosts: proxy.bypass.joined(separator: ", ")))
            }
            return parts.joined(separator: " · ")
        }

        // MARK: Loading

        /// Loads the proxies, the server's mode and the proxies of its own
        /// requests.
        private func loadAll() async {
            let model = model
            let server = server
            async let info = try? model.call(
                server, Procedure.auth("GetAuthInfo"), Mavio_Auth_V1_GetAuthInfoRequest(),
                as: Mavio_Auth_V1_GetAuthInfoResponse.self)
            async let saved = try? model.call(
                server, Procedure.proxies("GetOutboundProxies"), Mavio_System_V1_GetOutboundProxiesRequest(),
                as: Mavio_System_V1_GetOutboundProxiesResponse.self)
            await load()
            mavioDev = await info?.mode == .mavioDev
            if let s = await saved { settings = s.proxies }
            guard !Task.isCancelled else { return }
            render()
        }

        /// Loads the proxies again.
        private func load() async {
            do {
                proxies = try await model.call(
                    server, Procedure.proxies("ListProxies"), Mavio_System_V1_ListProxiesRequest(),
                    as: Mavio_System_V1_ListProxiesResponse.self
                ).proxies
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        // MARK: A proxy's line

        /// A proxy's line: its tile, name and address, editing it when
        /// selected where it may be, and testing it, with the result under
        /// it.
        private func line(
            id: String, tint: UInt32, name: String, detail: String, edit: (@MainActor () -> Void)? = nil
        ) -> [NSView] {
            let busy = testing.contains(id)
            let test = MavioButton(busy ? "…" : MavioStrings.proxiesTest, kind: .secondary, theme: theme) {
                [weak self] in self?.test(id)
            }
            test.isEnabled = !busy
            let row = FormRow(
                name, subtitle: detail, icon: "network", tint: NSColor(hex: tint), accessory: test, theme: theme)
            row.onPress = edit
            var rows: [NSView] = [row]
            if let outcome = outcomes[id] { rows.append(self.row(result(outcome))) }
            return rows
        }

        /// Tests a stored proxy, by its ID.
        private func test(_ id: String) {
            testing.insert(id)
            render()
            run { [weak self, model, server] in
                let outcome = await testProxy(model, server, .id(id))
                self?.outcomes[id] = outcome
                self?.testing.remove(id)
                self?.render()
            }
        }

        /// A test's result: how long the site took through the proxy, the
        /// exit address and its country, or why it failed.
        private func result(_ outcome: TestOutcome) -> NSView {
            if let error = outcome.error ?? outcome.result.flatMap({ $0.ok ? nil : $0.error }) {
                return label(
                    MavioStrings.proxiesTestFailed(error: error), theme.colors.danger, style: theme.fonts.footnote)
            }
            guard let r = outcome.result else { return NSView() }
            let ms =
                r.hasLatency ? Int((Double(r.latency.seconds) * 1000 + Double(r.latency.nanos) / 1e6).rounded()) : 0
            var country = r.country
            if country.count == 2, country.allSatisfy({ $0.isASCII && $0.isUppercase }) {
                country = Locale.current.localizedString(forRegionCode: country) ?? country
            }
            let text = [
                MavioStrings.proxiesTestOk(ms: String(ms)),
                r.exitIp.isEmpty ? "" : MavioStrings.proxiesExit(address: r.exitIp),
                country,
            ]
            .filter { !$0.isEmpty }
            .joined(separator: " · ")
            return label(text, theme.colors.success, style: theme.fonts.footnote)
        }

        // MARK: The form

        /// Opens the form of a proxy, by its ID; "" for a new one.
        private func edit(_ id: String) {
            let proxy = own.first { $0.id == id }
            draft = Draft()
            if let proxy {
                draft = Draft(
                    name: proxy.name, kind: kinds.contains { $0.kind == proxy.kind } ? proxy.kind : .http,
                    host: proxy.host, port: proxy.port, username: proxy.username)
            }
            editing = id
            formFailure = nil
            formOutcome = nil
            formTesting = false
            render()
        }

        /// Closes the form, keeping the proxies as they were.
        private func close() {
            endEditing()
            editing = nil
            render()
        }

        /// The form adding a proxy, or editing one and deleting it: its rows
        /// in the list of proxies, in the place of the proxy's line.
        private func form(_ proxy: Mavio_System_V1_Proxy?) -> [NSView] {
            var rows: [NSView] = [
                fieldRow(MavioStrings.proxiesName, text: draft.name) { [weak self] in
                    self?.draft.name = $0
                    self?.revalidate()
                },
                choiceRow(
                    MavioStrings.proxiesKind, options: kinds.map { ($0.label, $0.kind) }, selected: draft.kind
                ) { [weak self] in self?.draft.kind = $0 },
                fieldRow(MavioStrings.proxiesHost, text: draft.host, placeholder: "proxy.lan") { [weak self] in
                    self?.draft.host = $0
                    self?.revalidate()
                },
                numberRow(MavioStrings.proxiesPort, value: Double(draft.port), range: 1...65535) { [weak self] in
                    self?.draft.port = Int32($0)
                    self?.revalidate()
                },
                fieldRow(MavioStrings.proxiesUsername, text: draft.username) { [weak self] in
                    self?.draft.username = $0
                },
            ]
            let kept = proxy?.hasPassword_p == true && !draft.clearPassword
            rows.append(
                fieldRow(
                    MavioStrings.proxiesPassword, text: draft.password,
                    placeholder: kept ? MavioStrings.proxiesPasswordKept : "", secure: true, width: 170
                ) { [weak self] in self?.draft.password = $0 })
            // Clearing applies only while no new password is typed.
            if proxy?.hasPassword_p == true {
                rows.append(
                    toggleRow(MavioStrings.proxiesClearPassword, isOn: draft.clearPassword) { [weak self] in
                        self?.draft.clearPassword = $0
                        self?.render()
                    })
            }
            rows.append(row(buttons(proxy)))
            if let formOutcome { rows.append(row(result(formOutcome))) }
            return rows
        }

        /// Save, Test, Cancel and Remove, with why saving failed.
        private func buttons(_ proxy: Mavio_System_V1_Proxy?) -> NSView {
            let save = MavioButton(saving ? "…" : MavioStrings.actionSave, kind: .primary, theme: theme) {
                [weak self] in self?.save(proxy)
            }
            let test = MavioButton(formTesting ? "…" : MavioStrings.proxiesTest, kind: .secondary, theme: theme) {
                [weak self] in self?.testDraft(proxy)
            }
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                self?.close()
            }
            saveButton = save
            testButton = test
            var views: [NSView] = [save, test, cancel]
            if let proxy {
                views.append(
                    MavioButton(MavioStrings.proxiesRemove, kind: .destructive, theme: theme) { [weak self] in
                        self?.formFailure = nil
                        self?.confirm(
                            MavioStrings.proxiesRemoveTitle(name: proxy.name),
                            message: MavioStrings.proxiesRemoveMessage,
                            action: MavioStrings.proxiesRemove
                        ) { [weak self] in self?.remove(proxy) }
                    })
            }
            if let formFailure { views.append(label(formFailure, theme.colors.danger)) }
            views.append(Spacer())
            revalidate()
            return NSStackView.horizontal(views, spacing: MavioSpacing.s12)
        }

        /// Enables Save and Test as the draft allows, without laying the
        /// form out again while it is typed in.
        private func revalidate() {
            saveButton?.isEnabled = draft.valid && !saving && !removing
            testButton?.isEnabled = draft.valid && !formTesting
        }

        /// The proxy as the form holds it.
        private func message(_ proxy: Mavio_System_V1_Proxy?) -> Mavio_System_V1_Proxy {
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

        /// Saves the form: a new proxy, or the one edited.
        private func save(_ proxy: Mavio_System_V1_Proxy?) {
            endEditing()
            guard draft.valid else { return }
            saving = true
            formFailure = nil
            render()
            let message = message(proxy)
            let clear = draft.clearPassword && draft.password.isEmpty
            run { [weak self, model, server] in
                do {
                    if proxy != nil {
                        var request = Mavio_System_V1_UpdateProxyRequest()
                        request.proxy = message
                        request.clearPassword_p = clear
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
                    self?.saving = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.formFailure = error.reason
                    self?.saving = false
                    self?.render()
                }
            }
        }

        /// Tests the proxy as the form holds it.
        private func testDraft(_ proxy: Mavio_System_V1_Proxy?) {
            endEditing()
            guard draft.valid else { return }
            formTesting = true
            render()
            let message = message(proxy)
            run { [weak self, model, server] in
                let outcome = await testProxy(model, server, .proxy(message))
                self?.formOutcome = outcome
                self?.formTesting = false
                self?.render()
            }
        }

        /// Deletes the proxy edited.
        private func remove(_ proxy: Mavio_System_V1_Proxy) {
            removing = true
            render()
            var request = Mavio_System_V1_DeleteProxyRequest()
            request.id = proxy.id
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.proxies("DeleteProxy"), request,
                        as: Mavio_System_V1_DeleteProxyResponse.self)
                    self?.removing = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.formFailure = error.reason
                    self?.removing = false
                    self?.render()
                }
            }
        }

        // MARK: The server's requests

        /// The proxies of the server's own requests; mavio.dev and its
        /// certificates only in mavio.dev mode.
        private func serverRequests() -> NSView {
            var rows: [NSView] = Request.allCases.filter { !$0.onlyMavioDev || mavioDev }.map { request in
                let value = value(request)
                let options = choices(value)
                let menu = PopUp(
                    options.map(\.label), selected: options.firstIndex { $0.value == value } ?? 0, label: request.label
                ) { [weak self] i in self?.choose(request, options[i].value) }
                menu.widthAnchor.constraint(equalToConstant: 192).isActive = true
                menu.isEnabled = choosing == nil
                let row = FormRow(request.label, subtitle: request.detail, accessory: menu, theme: theme)
                if theme.isTV { row.onPress = { [weak menu] in menu?.activate() } }
                return row
            }
            if let choosingFailure { rows.append(note(choosingFailure, failed: true)) }
            return card(MavioStrings.proxiesServerRequests, footer: MavioStrings.proxiesServerRequestsFooter, rows)
        }

        /// The proxy chosen for a request: the one being saved, else the
        /// settings'.
        private func value(_ request: Request) -> String {
            if let choosing, choosing.key == request { return choosing.id }
            return settings?[keyPath: request.path] ?? ""
        }

        /// The choices of a proxy: none, the system proxy, then the server's
        /// by name; one chosen but gone by its ID.
        private func choices(_ current: String) -> [(value: String, label: String)] {
            let own = own.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            var out: [(value: String, label: String)] =
                [("", MavioStrings.proxiesNone), (systemProxyId, MavioStrings.proxiesSystem)]
                + own.map { ($0.id, $0.name) }
            if proxies != nil, !out.contains(where: { $0.value == current }) {
                out.append((current, MavioStrings.proxiesUnknown))
            }
            return out
        }

        /// Saves the proxy of a request with the others as shown, keeping
        /// what the server returns.
        private func choose(_ request: Request, _ id: String) {
            guard var proxies = settings else { return }
            choosing = (request, id)
            choosingFailure = nil
            render()
            proxies[keyPath: request.path] = id
            var update = Mavio_System_V1_UpdateOutboundProxiesRequest()
            update.proxies = proxies
            run { [weak self, model, server] in
                do {
                    self?.settings = try await model.call(
                        server, Procedure.proxies("UpdateOutboundProxies"), update,
                        as: Mavio_System_V1_UpdateOutboundProxiesResponse.self
                    ).proxies
                } catch {
                    self?.choosingFailure = error.reason
                }
                self?.choosing = nil
                self?.render()
            }
        }
    }

    /// Tests a proxy: a stored one by its ID, or one being edited, which
    /// keeps the stored password when it gives none.
    @MainActor private func testProxy(
        _ model: AppModel, _ server: ServerRecord, _ target: Mavio_System_V1_TestProxyRequest.OneOf_Target
    ) async -> TestOutcome {
        var request = Mavio_System_V1_TestProxyRequest()
        request.target = target
        do {
            let result = try await model.call(
                server, Procedure.proxies("TestProxy"), request, as: Mavio_System_V1_TestProxyResponse.self)
            return TestOutcome(result: result)
        } catch {
            return TestOutcome(error: error.reason)
        }
    }
#endif
