#if os(macOS)
    import AppKit
    import MavioMac

    /// AI (docs/clients.md §6.3.13), as the web UI's: the assistant's
    /// settings of the server; the providers of the AI gateway plugin added,
    /// each with where its credential comes from, and the others to add; a
    /// provider's page with its API key, its OAuth login followed step by
    /// step, signing out, and its models; and at their end the
    /// administrators' conversations with the models (§6.3.26).
    final class AIPage: Page {
        private let server: ServerRecord
        private lazy var form = FormPage(title: MavioStrings.settingsAi, message: MavioStrings.aiMessage, theme: theme)
        private lazy var assistant = AssistantSettings(server: server, page: self)
        private lazy var conversations = AIConversations(server: server, page: self)
        private let providersHolder = NSStackView.vertical([], spacing: MavioSpacing.s32)
        private var providers: [Mavio_Plugin_V1_AiProvider]?
        private var models: [Mavio_Plugin_V1_AiModel] = []
        private var failure: String?
        private var noGateway = false
        private var loading: Task<Void, Never>?

        /// The AI settings of server.
        init(context: MacContext, server: ServerRecord) {
            self.server = server
            super.init(context: context)
        }

        isolated deinit { loading?.cancel() }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsAi
            form.add(assistant.view)
            form.add(providersHolder)
            form.add(conversations.view)
            show(form)
            render()
            assistant.load()
            conversations.load()
            reload()
        }

        private func reload() {
            loading?.cancel()
            loading = Task { [weak self] in await self?.load() }
        }

        /// Reads the providers and the models of those usable.
        func load() async {
            var request = Mavio_System_V1_ListAiModelsRequest()
            request.available = true
            async let list = try? model.call(
                server, Procedure.aiProviders("ListAiModels"), request, as: Mavio_System_V1_ListAiModelsResponse.self)
            do {
                providers = try await model.call(
                    server, Procedure.aiProviders("ListAiProviders"), Mavio_System_V1_ListAiProvidersRequest(),
                    as: Mavio_System_V1_ListAiProvidersResponse.self
                ).providers
                failure = nil
                noGateway = false
            } catch is CancellationError {
                return
            } catch {
                noGateway = isNoAiGateway(error)
                failure = error.reason
            }
            models = (await list)?.models ?? []
            render()
        }

        private func render() {
            if noGateway {
                let row = pageLinkRow(
                    icon: "plugins", color: NSColor(hex: 0xAF52DE), label: MavioStrings.aiNoGateway,
                    detail: MavioStrings.aiNoGatewayDetail, theme: theme
                ) { [weak self] in
                    guard let self else { return }
                    self.push(PluginsPage(context: self.context, server: self.server))
                }
                providersHolder.setViews([
                    FormSection(footer: MavioStrings.aiNoGatewayFooter, rows: [row], theme: theme)
                ])
            } else {
                providersHolder.setViews([providersSection()])
            }
            for v in providersHolder.arrangedSubviews {
                v.widthAnchor.constraint(equalTo: providersHolder.widthAnchor).isActive = true
            }
            FocusEngine.active(for: view)?.schedule()
        }

        private func providersSection() -> FormSection {
            let all = (providers ?? []).sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            let added = all.filter(\.available)
            let others = all.filter { !$0.available }
            var rows: [NSView] = added.map { p in
                pageLinkRow(
                    icon: "key", color: NSColor(hex: 0x34C759), label: p.name, detail: detail(p), theme: theme
                ) { [weak self] in self?.openProvider(p.id) }
            }
            if providers != nil, added.isEmpty {
                rows.append(noteRow(MavioStrings.aiNoProviders, theme.colors.textSecondary, theme: theme))
            }
            if providers == nil, failure == nil { rows.append(spinnerRow(theme: theme)) }
            if let failure { rows.append(noteRow(failure, theme.colors.danger, theme: theme)) }
            if !others.isEmpty {
                let items = others.map { p in
                    PickerItem(
                        value: p.id, label: p.name,
                        detail: [p.apiKey ? MavioStrings.aiApiKey : nil, p.oauth ? MavioStrings.aiOauth : nil]
                            .compactMap { $0 }.joined(separator: " · "),
                        badges: p.custom ? [(MavioStrings.aiCustom, .neutral)] : [])
                }
                let picker = SearchPicker(
                    items: items, value: nil, placeholder: MavioStrings.aiChooseProvider, width: 256, theme: theme)
                picker.setAccessibilityLabel(MavioStrings.aiAddProvider)
                picker.onChange = { [weak self, weak picker] id in
                    picker?.value = nil
                    self?.openProvider(id)
                }
                rows.append(controlRow(MavioStrings.aiAddProvider, control: picker, theme: theme))
            }
            return FormSection(
                MavioStrings.aiProviders, footer: MavioStrings.aiProvidersFooter, rows: rows, theme: theme)
        }

        private func openProvider(_ id: String) {
            push(
                AIProviderPage(context: context, server: server, id: id) { [weak self] in await self?.load() })
        }

        private func detail(_ p: Mavio_Plugin_V1_AiProvider) -> String {
            let count = models.filter { $0.provider == p.id }.count
            return [aiCredentialOf(p), MavioStrings.aiModelCount(count: count), p.custom ? MavioStrings.aiCustom : nil]
                .compactMap { $0 }.joined(separator: " · ")
        }
    }

    /// Where a provider's credential comes from, or that it has none.
    func aiCredentialOf(_ p: Mavio_Plugin_V1_AiProvider) -> String {
        if !p.available { return MavioStrings.aiNotSignedIn }
        switch p.credential {
        case .oauth: return MavioStrings.aiSignedIn(label: p.oauthLabel.isEmpty ? p.name : p.oauthLabel)
        case .apiKey: return MavioStrings.aiApiKeyStored
        default: return MavioStrings.aiFromSource(source: p.source)
        }
    }

    // MARK: A provider

    /// A provider's page: its OAuth login, its API key, signing out and its
    /// models. Its sections keep their rows as they change, so that the
    /// fields being typed in stay as the login is followed.
    final class AIProviderPage: Page {
        private let server: ServerRecord
        private let id: String
        /// Refreshes the providers' list, once a credential changed.
        private let changed: @MainActor () async -> Void
        private lazy var form = FormPage(theme: theme)
        private lazy var heading = NSTextField.label(id, theme.fonts.title1, color: theme.colors.textPrimary)
        private lazy var message = note("", theme.fonts.body, theme.colors.textSecondary)
        private lazy var oauthSection = LiveSection(MavioStrings.aiOauth, theme: theme)
        private lazy var keySection = LiveSection(MavioStrings.aiApiKey, theme: theme)
        private lazy var signOutSection = LiveSection(theme: theme)
        private lazy var modelsSection = LiveSection(MavioStrings.aiModels, theme: theme)
        private lazy var status = NSStackView.vertical([], spacing: 0)

        private var providers: [Mavio_Plugin_V1_AiProvider]?
        private var failure: String?
        private var models: [Mavio_Plugin_V1_AiModel]?
        private var modelsFailure: String?
        private var loading: Task<Void, Never>?

        // The API key.
        private lazy var keyField = InputField(secure: true, width: 288)
        private lazy var keyRow = controlRow(MavioStrings.aiApiKey, control: keyField, theme: theme)
        private lazy var keySave = MavioButton(MavioStrings.actionSave, kind: .primary, theme: theme)
        private lazy var keyNote = note("", theme.fonts.callout, theme.colors.textSecondary, selectable: true)
        private lazy var keyBar = ContentRow(bar([keySave, keyNote]), theme: theme)
        private var keySaved = false
        private var keySaving = false
        private var keyFailure: String?

        // Signing out.
        private var signingOut = false
        private var signOutFailure: String?

        // The OAuth login.
        private var loginId: String?
        private var login: Mavio_System_V1_GetAiLoginResponse?
        private var loginFailure: String?
        private var starting = false
        private var startFailure: String?
        private var answering = false
        private var answerFailure: String?
        private var cancelFailure: String?
        private var following: Task<Void, Never>?
        private var tasks: [Task<Void, Never>] = []
        /// The rows of the login's events, by their place, kept as more come.
        private var eventRows: [NSView] = []
        private var prompt: (id: String, row: LoginPromptRow)?

        /// The page of a provider by its ID, calling changed once its
        /// credential changed.
        init(
            context: MacContext, server: ServerRecord, id: String, changed: @escaping @MainActor () async -> Void
        ) {
            self.server = server
            self.id = id
            self.changed = changed
            super.init(context: context)
        }

        isolated deinit {
            loading?.cancel()
            following?.cancel()
            tasks.forEach { $0.cancel() }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = id
            let head = NSStackView.vertical([heading, message], spacing: MavioSpacing.s4)
            message.widthAnchor.constraint(equalTo: head.widthAnchor).isActive = true
            form.add(head)
            for section in [oauthSection, keySection, signOutSection, modelsSection, status] as [NSView] {
                form.add(section)
            }
            keyField.onChange = { [weak self] _ in
                self?.keySaved = false
                self?.render()
            }
            keyField.onSubmit = { [weak self] in self?.saveKey() }
            keySave.onPress = { [weak self] in self?.saveKey() }
            show(form)
            render()
            loading = Task { [weak self] in await self?.load() }
        }

        private var provider: Mavio_Plugin_V1_AiProvider? { providers?.first { $0.id == id } }

        private func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        // MARK: Drawing

        private func render() {
            let p = provider
            heading.stringValue = p?.name ?? id
            title = p?.name ?? id
            message.stringValue = p.map(aiCredentialOf) ?? ""
            message.isHidden = p == nil
            oauthSection.isHidden = p?.oauth != true
            keySection.isHidden = p?.apiKey != true
            signOutSection.isHidden = p == nil || p?.credential == .unspecified
            modelsSection.isHidden = p == nil
            if let p {
                if p.oauth { renderOauth(p) }
                if p.apiKey { renderKey(p) }
                if p.credential != .unspecified { renderSignOut(p) }
                renderModels(p)
                status.setViews([])
            } else if let failure {
                status.setViews([plainNote(failure, theme.colors.danger, theme: theme)])
            } else if providers != nil {
                status.setViews([plainNote(MavioStrings.aiProviderGone, theme.colors.textSecondary, theme: theme)])
            } else {
                status.setViews([LoadingView()])
            }
            FocusEngine.active(for: view)?.schedule()
        }

        // MARK: OAuth

        private var running: Bool {
            loginId != nil && (login == nil || login?.state == .running) && loginFailure == nil
        }

        private func renderOauth(_ p: Mavio_Plugin_V1_AiProvider) {
            var rows: [NSView] = []
            if !running {
                let button = MavioButton(
                    starting ? "…" : MavioStrings.aiSignIn, kind: .primary, theme: theme
                ) { [weak self] in self?.start(p) }
                button.isEnabled = !starting
                rows.append(
                    controlRow(
                        MavioStrings.aiSignInWith(label: p.oauthLabel.isEmpty ? p.name : p.oauthLabel),
                        subtitle: p.credential == .oauth ? MavioStrings.aiSignInAgain : nil, control: button,
                        theme: theme))
            }
            if loginId != nil, let login {
                if login.events.count < eventRows.count { eventRows = [] }
                for event in login.events.dropFirst(eventRows.count) {
                    eventRows.append(ContentRow(eventView(event), theme: theme))
                }
                rows += eventRows
            }
            if running, let login, login.hasPrompt {
                let row: LoginPromptRow
                if let prompt, prompt.id == login.prompt.id {
                    row = prompt.row
                } else {
                    row = LoginPromptRow(prompt: login.prompt, theme: theme) { [weak self] in self?.answer($0) }
                    prompt = (login.prompt.id, row)
                }
                row.busy = answering
                rows.append(row)
            } else {
                prompt = nil
            }
            if running {
                let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                    self?.cancel()
                }
                rows.append(
                    ContentRow(
                        bar([note(MavioStrings.aiWaiting, theme.fonts.callout, theme.colors.textSecondary), cancel]),
                        theme: theme))
            }
            switch login?.state {
            case .succeeded:
                rows.append(noteRow(MavioStrings.aiLoginSucceeded, theme.colors.textSecondary, theme: theme))
            case .failed:
                let error = login?.error ?? ""
                rows.append(
                    noteRow(error.isEmpty ? MavioStrings.aiLoginFailed : error, theme.colors.danger, theme: theme))
            case .canceled:
                rows.append(noteRow(MavioStrings.aiLoginCanceled, theme.colors.textSecondary, theme: theme))
            default:
                break
            }
            for failure in [loginFailure, startFailure, answerFailure, cancelFailure].compactMap(\.self) {
                rows.append(noteRow(failure, theme.colors.danger, theme: theme))
            }
            oauthSection.setRows(rows)
            oauthSection.footer = p.subscription ? MavioStrings.aiSubscriptionFooter : nil
        }

        /// What a login showed: a message with links, a page to open, a code
        /// to enter, or its progress. Television mode shows addresses as
        /// text, to open on another device.
        private func eventView(_ event: Mavio_Plugin_V1_AiLoginEvent) -> NSView {
            let style = theme.fonts.callout
            switch event.event {
            case .info(let info):
                var views: [NSView] = [note(info.message, style, theme.colors.textPrimary, selectable: true)]
                for l in info.links {
                    views.append(link(l.label.isEmpty ? l.url : l.label, url: l.url))
                }
                return NSStackView.vertical(views, spacing: MavioSpacing.s4)
            case .url(let page):
                var views: [NSView] = []
                if !page.instructions.isEmpty {
                    views.append(note(page.instructions, style, theme.colors.textPrimary, selectable: true))
                }
                if !theme.isTV {
                    views.append(
                        MavioButton(MavioStrings.aiOpenPage, kind: .secondary, theme: theme) {
                            if let url = URL(string: page.url) { NSWorkspace.shared.open(url) }
                        })
                }
                views.append(copyableUrl(page.url))
                return NSStackView.vertical(views, spacing: 6)
            case .deviceCode(let code):
                let codeLabel = NSTextField.label(
                    code.userCode, theme.fonts.title3, color: theme.colors.textPrimary, selectable: true)
                codeLabel.attributedStringValue = NSAttributedString(
                    string: code.userCode,
                    attributes: [
                        .font: NSFont.monospacedSystemFont(ofSize: theme.fonts.title3.size, weight: .regular),
                        .kern: 4, .foregroundColor: theme.colors.textPrimary,
                    ])
                let box = Box(fill: theme.colors.surfaceRaised, radius: 6)
                box.addFilling(codeLabel, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: 6))
                return NSStackView.vertical(
                    [
                        note(MavioStrings.aiEnterCode, style, theme.colors.textPrimary), box,
                        link(code.verificationUri, url: code.verificationUri),
                    ], spacing: 6)
            case .progress(let text):
                return note(text, style, theme.colors.textSecondary)
            case nil:
                return NSView()
            }
        }

        /// A link opening url in the browser; in television mode its text.
        private func link(_ label: String, url: String) -> NSView {
            guard !theme.isTV, let target = URL(string: url) else {
                return note(label, theme.fonts.callout, theme.colors.accentText, selectable: !theme.isTV)
            }
            let button = MavioButton(label, kind: .link, theme: theme) { NSWorkspace.shared.open(target) }
            button.toolTip = url
            return button
        }

        /// A long address on one line, cut in its middle to fit, with a
        /// button copying it.
        private func copyableUrl(_ url: String) -> NSView {
            let text = NSTextField.label(url, theme.fonts.footnote, color: theme.colors.textSecondary)
            text.lineBreakMode = .byTruncatingMiddle
            text.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            guard !theme.isTV else { return text }
            text.toolTip = url
            return bar(
                [
                    text,
                    copyButton(url, label: MavioStrings.aiCopyLink, copied: MavioStrings.usersCopied, theme: theme),
                ],
                spacing: MavioSpacing.s4)
        }

        private func start(_ p: Mavio_Plugin_V1_AiProvider) {
            answerFailure = nil
            starting = true
            startFailure = nil
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_StartAiLoginRequest()
                request.provider = p.id
                do {
                    let response = try await self.model.call(
                        self.server, Procedure.aiProviders("StartAiLogin"), request,
                        as: Mavio_System_V1_StartAiLoginResponse.self)
                    self.starting = false
                    self.follow(response.loginID)
                } catch {
                    self.starting = false
                    self.startFailure = error.reason
                    self.render()
                }
            }
        }

        /// Follows a login as it runs, until it ends; the provider and its
        /// models are read again then.
        private func follow(_ id: String?) {
            following?.cancel()
            loginId = id
            login = nil
            loginFailure = nil
            eventRows = []
            prompt = nil
            render()
            guard let id else { return }
            following = Task { [weak self] in
                var request = Mavio_System_V1_GetAiLoginRequest()
                request.loginID = id
                while !Task.isCancelled {
                    guard let self else { return }
                    do {
                        let state = try await self.model.call(
                            self.server, Procedure.aiProviders("GetAiLogin"), request,
                            as: Mavio_System_V1_GetAiLoginResponse.self)
                        guard !Task.isCancelled else { return }
                        self.login = state
                        self.render()
                        if state.state != .running && state.state != .unspecified { break }
                    } catch is CancellationError {
                        return
                    } catch {
                        self.loginFailure = error.reason
                        self.render()
                        break
                    }
                    try? await Task.sleep(for: .seconds(1))
                }
                guard !Task.isCancelled, let self else { return }
                await self.load()
                await self.changed()
            }
        }

        private func answer(_ value: String) {
            guard let loginId, let login, login.hasPrompt else { return }
            answering = true
            answerFailure = nil
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_AnswerAiLoginRequest()
                request.loginID = loginId
                request.promptID = login.prompt.id
                request.value = value
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.aiProviders("AnswerAiLogin"), request,
                        as: Mavio_System_V1_AnswerAiLoginResponse.self)
                } catch {
                    self.answerFailure = error.reason
                }
                self.answering = false
                self.render()
            }
        }

        private func cancel() {
            guard let id = loginId else { return }
            cancelFailure = nil
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_CancelAiLoginRequest()
                request.loginID = id
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.aiProviders("CancelAiLogin"), request,
                        as: Mavio_System_V1_CancelAiLoginResponse.self)
                    self.follow(nil)
                } catch {
                    self.cancelFailure = error.reason
                    self.render()
                }
            }
        }

        // MARK: API key

        private func renderKey(_ p: Mavio_Plugin_V1_AiProvider) {
            keyField.field.placeholderString = p.credential == .apiKey ? "••••••••" : nil
            keyField.isEnabled = !keySaving
            keySave.title = keySaving ? "…" : MavioStrings.actionSave
            keySave.isEnabled = !keyField.text.trimmingCharacters(in: .whitespaces).isEmpty && !keySaving
            if let keyFailure {
                keyNote.stringValue = keyFailure
                keyNote.textColor = theme.colors.danger
            } else {
                keyNote.stringValue = keySaved ? MavioStrings.aiApiKeySaved : ""
                keyNote.textColor = theme.colors.textSecondary
            }
            keyNote.isHidden = keyNote.stringValue.isEmpty
            keySection.setRows([keyRow, keyBar])
            keySection.footer =
                p.credential == .apiKey ? MavioStrings.aiApiKeyReplaceFooter : MavioStrings.aiApiKeyFooter
        }

        private func saveKey() {
            let key = keyField.text.trimmingCharacters(in: .whitespaces)
            guard let p = provider, !key.isEmpty, !keySaving else { return }
            keySaving = true
            keyFailure = nil
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_SetAiApiKeyRequest()
                request.provider = p.id
                request.key = key
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.aiProviders("SetAiApiKey"), request,
                        as: Mavio_System_V1_SetAiApiKeyResponse.self)
                    self.keyField.text = ""
                    self.keySaved = true
                    self.keySaving = false
                    await self.load()
                    await self.changed()
                } catch {
                    self.keyFailure = error.reason
                    self.keySaving = false
                    self.render()
                }
            }
        }

        // MARK: Signing out

        private func renderSignOut(_ p: Mavio_Plugin_V1_AiProvider) {
            let action = p.credential == .oauth ? MavioStrings.aiSignOut : MavioStrings.aiRemoveKey
            let button = MavioButton(action, kind: .destructive, theme: theme) { [weak self] in
                guard let self else { return }
                confirm(action, message: MavioStrings.aiSignOutFooter, action: action, in: self.view) {
                    [weak self] in self?.signOut(p)
                }
            }
            button.isEnabled = !signingOut
            var rows: [NSView] = [ContentRow(bar([button]), theme: theme)]
            if let signOutFailure { rows.append(noteRow(signOutFailure, theme.colors.danger, theme: theme)) }
            signOutSection.setRows(rows)
        }

        private func signOut(_ p: Mavio_Plugin_V1_AiProvider) {
            signingOut = true
            signOutFailure = nil
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_LogoutAiRequest()
                request.provider = p.id
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.aiProviders("LogoutAi"), request,
                        as: Mavio_System_V1_LogoutAiResponse.self)
                    self.signingOut = false
                    await self.load()
                    await self.changed()
                } catch {
                    self.signingOut = false
                    self.signOutFailure = error.reason
                    self.render()
                }
            }
        }

        // MARK: Models

        private lazy var modelsTable: ListTable = ListTable(maxHeight: 448, theme: theme)
        private lazy var modelsRow = ContentRow(modelsTable, theme: theme)
        private var modelsShown: [Mavio_Plugin_V1_AiModel] = []

        private func renderModels(_ p: Mavio_Plugin_V1_AiProvider) {
            var rows: [NSView] = []
            if let models, !models.isEmpty {
                // Providers list up to hundreds of models; the list scrolls within the group.
                if models != modelsShown {
                    modelsShown = models
                    let theme = theme
                    modelsTable.show(count: models.count, rowHeight: { _ in theme.isTV ? 64 : 44 }) { i in
                        let m = models[i]
                        let text = NSStackView.vertical(
                            [
                                NSTextField.label(m.name.isEmpty ? m.id : m.name, theme.fonts.body),
                                NSTextField.label(
                                    aiModelDetail(m), theme.fonts.footnote, color: theme.colors.textSecondary,
                                    selectable: true),
                            ], spacing: 2)
                        let cell = NSView()
                        cell.addFilling(text, insets: .symmetric(vertical: 4))
                        return cell
                    }
                }
                rows.append(modelsRow)
            } else if models != nil {
                rows.append(noteRow(MavioStrings.aiNoModels, theme.colors.textSecondary, theme: theme))
            } else if let modelsFailure {
                rows.append(noteRow(modelsFailure, theme.colors.danger, theme: theme))
            } else {
                rows.append(spinnerRow(theme: theme))
            }
            modelsSection.setRows(rows)
            modelsSection.footer = p.available ? nil : MavioStrings.aiModelsFooter
        }

        // MARK: Loading

        private func load() async {
            var request = Mavio_System_V1_ListAiModelsRequest()
            request.provider = id
            let server = server
            let model = model
            async let list = model.call(
                server, Procedure.aiProviders("ListAiModels"), request, as: Mavio_System_V1_ListAiModelsResponse.self)
            do {
                providers = try await model.call(
                    server, Procedure.aiProviders("ListAiProviders"), Mavio_System_V1_ListAiProvidersRequest(),
                    as: Mavio_System_V1_ListAiProvidersResponse.self
                ).providers
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
            do {
                models = try await list.models
                modelsFailure = nil
            } catch is CancellationError {
            } catch {
                models = isNoAiGateway(error) ? [] : nil
                modelsFailure = isNoAiGateway(error) ? nil : error.reason
            }
            render()
        }
    }

    // MARK: The login's prompt

    /// What a login asks: text, a secret, a choice or a code pasted back,
    /// and the button answering it.
    final class LoginPromptRow: ContentRow {
        private let submit: MavioButton
        private let answer: Answer

        /// What is answered, which the controls set.
        private final class Answer {
            var value = ""
            var changed: (@MainActor () -> Void)?
        }

        /// Whether the answer is being sent.
        var busy = false { didSet { restyle() } }

        /// The row of prompt, sending what is answered to onAnswer.
        init(
            prompt: Mavio_Plugin_V1_AiLoginPrompt, theme: Theme,
            onAnswer: @escaping @MainActor @Sendable (String) -> Void
        ) {
            let answer = Answer()
            self.answer = answer
            submit = MavioButton(MavioStrings.aiSubmit, kind: .primary, theme: theme)
            let label = prompt.message.isEmpty ? MavioStrings.aiAnswer : prompt.message
            var views: [NSView] = [note(label, theme.fonts.body, theme.colors.textPrimary, selectable: true)]
            let control: NSView
            var field: InputField?
            switch prompt.kind {
            case .select:
                let ids = prompt.options.map(\.id)
                answer.value = ids.first ?? ""
                let popup = ActionPopUp(
                    options: prompt.options.map {
                        $0.description_p.isEmpty ? $0.label : "\($0.label) — \($0.description_p)"
                    },
                    selected: 0
                ) { i in
                    answer.value = ids[i]
                    answer.changed?()
                }
                popup.setAccessibilityLabel(label)
                control = popup
            default:
                let f = InputField(placeholder: prompt.placeholder, secure: prompt.kind == .secret)
                f.setAccessibilityLabel(label)
                f.onChange = { text in
                    answer.value = text
                    answer.changed?()
                }
                field = f
                control = f
            }
            let shown = focusable(control, theme: theme)
            views.append(shown)
            if prompt.kind == .manualCode {
                views.append(note(MavioStrings.aiManualCode, theme.fonts.footnote, theme.colors.textSecondary))
            }
            views.append(submit)
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            if field != nil { shown.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            super.init(column, theme: theme)
            answer.changed = { [weak self] in self?.restyle() }
            let send: @MainActor @Sendable () -> Void = { [weak self] in
                guard let self, !self.trimmed.isEmpty, !self.busy else { return }
                onAnswer(self.trimmed)
            }
            field?.onSubmit = send
            submit.onPress = send
            restyle()
        }

        private var trimmed: String { answer.value.trimmingCharacters(in: .whitespaces) }

        private func restyle() {
            submit.title = busy ? "…" : MavioStrings.aiSubmit
            submit.isEnabled = !trimmed.isEmpty && !busy
        }
    }
#endif
