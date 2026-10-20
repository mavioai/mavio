#if os(macOS)
    import AppKit
    import MavioKit
    import MavioUI
    import SwiftUI

    /// AI (docs/clients.md §6.3.13), as the web UI's: the assistant's settings of the server; the providers of the
    /// AI gateway plugin added, each with where its credential comes from, and the others to add; a provider's
    /// page with its API key, its OAuth login followed step by step, signing out, and its models.
    struct AiView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var providers: [Mavio_Plugin_V1_AiProvider]?
        @State private var models: [Mavio_Plugin_V1_AiModel] = []
        @State private var failure: String?
        @State private var noGateway = false
        @State private var adding = ""
        @State private var opened: String?
        @State private var assistant = AiAssistantStore()

        var body: some View {
            SettingsList(message: MavioStrings.aiMessage) {
                AiAssistantSettings(server: server, store: assistant)
                if noGateway {
                    Section {
                        NavigationLink {
                            PluginsView(server: server)
                        } label: {
                            AiPageLabel(
                                icon: "plugins", color: Color(hex: 0xAF52DE), label: MavioStrings.aiNoGateway,
                                detail: MavioStrings.aiNoGatewayDetail)
                        }
                        .rowLink()
                    } footer: {
                        Text(MavioStrings.aiNoGatewayFooter)
                    }
                } else {
                    providersSection
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsAi)
            .inlineTitle()
            .navigationDestination(item: $opened) { id in
                AiProviderView(server: server, id: id, changed: load)
            }
            .onChange(of: adding) { _, id in
                guard !id.isEmpty else { return }
                adding = ""
                opened = id
            }
            .task {
                async let settings: Void = assistant.load(model, server)
                await load()
                await settings
            }
        }

        private var providersSection: some View {
            let all = (providers ?? []).sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            let added = all.filter(\.available)
            let others = all.filter { !$0.available }
            return Section {
                ForEach(added, id: \.id) { p in
                    NavigationLink {
                        AiProviderView(server: server, id: p.id, changed: load)
                    } label: {
                        AiPageLabel(icon: "key", color: Color(hex: 0x34C759), label: p.name, detail: detail(p))
                    }
                    .rowLink()
                }
                if providers != nil, added.isEmpty {
                    Text(MavioStrings.aiNoProviders).foregroundStyle(theme.colors.textSecondary)
                }
                if providers == nil, failure == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                }
                if !others.isEmpty {
                    FormRow(label: MavioStrings.aiAddProvider) {
                        AiSearchPicker(
                            label: MavioStrings.aiAddProvider,
                            options: others.map { p in
                                AiPickerOption(
                                    value: p.id, label: p.name,
                                    detail: [
                                        p.apiKey ? MavioStrings.aiApiKey : nil, p.oauth ? MavioStrings.aiOauth : nil,
                                    ]
                                    .compactMap { $0 }.joined(separator: " · "),
                                    badge: p.custom ? MavioStrings.aiCustom : nil)
                            },
                            value: $adding, placeholder: MavioStrings.aiChooseProvider, width: 256)
                    }
                }
            } header: {
                Text(MavioStrings.aiProviders)
            } footer: {
                Text(MavioStrings.aiProvidersFooter)
            }
        }

        private func detail(_ p: Mavio_Plugin_V1_AiProvider) -> String {
            let count = models.filter { $0.provider == p.id }.count
            return [aiCredentialOf(p), MavioStrings.aiModelCount(count: count), p.custom ? MavioStrings.aiCustom : nil]
                .compactMap { $0 }.joined(separator: " · ")
        }

        private func load() async {
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
            } catch {
                noGateway = error.isNoAiGateway
                failure = error.reason
            }
            models = (await list)?.models ?? []
        }
    }

    /// Where a provider's credential comes from, or that it has none.
    private func aiCredentialOf(_ p: Mavio_Plugin_V1_AiProvider) -> String {
        if !p.available { return MavioStrings.aiNotSignedIn }
        switch p.credential {
        case .oauth: return MavioStrings.aiSignedIn(label: p.oauthLabel.isEmpty ? p.name : p.oauthLabel)
        case .apiKey: return MavioStrings.aiApiKeyStored
        default: return MavioStrings.aiFromSource(source: p.source)
        }
    }

    // MARK: A provider

    /// A provider's page: its OAuth login, its API key, signing out and its models.
    private struct AiProviderView: View {
        let server: ServerRecord
        let id: String
        /// Refreshes the providers' list, once a credential changed.
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var providers: [Mavio_Plugin_V1_AiProvider]?
        @State private var failure: String?
        @State private var models: [Mavio_Plugin_V1_AiModel]?
        @State private var modelsFailure: String?

        // The API key.
        @State private var key = ""
        @State private var keySaved = false
        @State private var keySaving = false
        @State private var keyFailure: String?

        // Signing out.
        @State private var confirming = false
        @State private var signingOut = false
        @State private var signOutFailure: String?

        // The OAuth login.
        @State private var loginId: String?
        @State private var login: Mavio_System_V1_GetAiLoginResponse?
        @State private var loginFailure: String?
        @State private var starting = false
        @State private var startFailure: String?
        @State private var answering = false
        @State private var answerFailure: String?
        @State private var cancelFailure: String?

        var body: some View {
            let p = providers?.first { $0.id == id }
            SettingsList(message: p.map(aiCredentialOf)) {
                if let p {
                    if p.oauth { oauthSection(p) }
                    if p.apiKey { apiKeySection(p) }
                    if p.credential != .unspecified { signOutSection(p) }
                    modelsSection(p)
                } else if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                } else if providers != nil {
                    Text(MavioStrings.aiProviderGone).foregroundStyle(theme.colors.textSecondary)
                } else {
                    ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(p?.name ?? id)
            .inlineTitle()
            .task { await load() }
            .task(id: loginId) { await follow() }
        }

        // MARK: OAuth

        private var running: Bool {
            loginId != nil && (login == nil || login?.state == .running) && loginFailure == nil
        }

        @ViewBuilder private func oauthSection(_ p: Mavio_Plugin_V1_AiProvider) -> some View {
            Section {
                if !running {
                    FormRow(
                        label: MavioStrings.aiSignInWith(label: p.oauthLabel.isEmpty ? p.name : p.oauthLabel),
                        detail: p.credential == .oauth ? MavioStrings.aiSignInAgain : nil
                    ) {
                        Button(starting ? "…" : MavioStrings.aiSignIn) { start(p) }
                            .buttonStyle(.borderedProminent)
                            .disabled(starting)
                    }
                }
                if loginId != nil, let login {
                    ForEach(Array(login.events.enumerated()), id: \.offset) { _, event in
                        AiLoginEventRow(event: event)
                    }
                }
                if running, let login, login.hasPrompt {
                    AiLoginPromptRow(prompt: login.prompt, busy: answering) { answer($0) }
                        .id(login.prompt.id)
                }
                if running {
                    HStack(spacing: MavioSpacing.s12) {
                        Text(MavioStrings.aiWaiting).foregroundStyle(theme.colors.textSecondary)
                        Button(MavioStrings.actionCancel, action: cancel)
                    }
                    .type(theme.type.callout)
                }
                switch login?.state {
                case .succeeded:
                    Text(MavioStrings.aiLoginSucceeded).foregroundStyle(theme.colors.textSecondary)
                case .failed:
                    Text(login?.error.isEmpty == false ? login?.error ?? "" : MavioStrings.aiLoginFailed)
                        .foregroundStyle(theme.colors.danger)
                        .textSelection(.enabled)
                case .canceled:
                    Text(MavioStrings.aiLoginCanceled).foregroundStyle(theme.colors.textSecondary)
                default:
                    EmptyView()
                }
                ForEach(
                    [loginFailure, startFailure, answerFailure, cancelFailure].compactMap { $0 }, id: \.self
                ) { failure in
                    Text(failure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                }
            } header: {
                Text(MavioStrings.aiOauth)
            } footer: {
                if p.subscription { Text(MavioStrings.aiSubscriptionFooter) }
            }
        }

        private func start(_ p: Mavio_Plugin_V1_AiProvider) {
            answerFailure = nil
            starting = true
            startFailure = nil
            Task {
                defer { starting = false }
                var request = Mavio_System_V1_StartAiLoginRequest()
                request.provider = p.id
                do {
                    let response = try await model.call(
                        server, Procedure.aiProviders("StartAiLogin"), request,
                        as: Mavio_System_V1_StartAiLoginResponse.self)
                    login = nil
                    loginFailure = nil
                    loginId = response.loginID
                } catch {
                    startFailure = error.reason
                }
            }
        }

        /// Follows the login as it runs, until it ends; the provider and its models are read again then.
        private func follow() async {
            login = nil
            loginFailure = nil
            guard let loginId else { return }
            var request = Mavio_System_V1_GetAiLoginRequest()
            request.loginID = loginId
            while !Task.isCancelled {
                do {
                    let state = try await model.call(
                        server, Procedure.aiProviders("GetAiLogin"), request,
                        as: Mavio_System_V1_GetAiLoginResponse.self)
                    login = state
                    if state.state != .running && state.state != .unspecified { break }
                } catch is CancellationError {
                    return
                } catch {
                    loginFailure = error.reason
                    break
                }
                try? await Task.sleep(for: .seconds(1))
            }
            guard !Task.isCancelled else { return }
            await load()
            await changed()
        }

        private func answer(_ value: String) {
            guard let loginId, let login, login.hasPrompt else { return }
            answering = true
            answerFailure = nil
            Task {
                defer { answering = false }
                var request = Mavio_System_V1_AnswerAiLoginRequest()
                request.loginID = loginId
                request.promptID = login.prompt.id
                request.value = value
                do {
                    _ = try await model.call(
                        server, Procedure.aiProviders("AnswerAiLogin"), request,
                        as: Mavio_System_V1_AnswerAiLoginResponse.self)
                } catch {
                    answerFailure = error.reason
                }
            }
        }

        private func cancel() {
            guard let id = loginId else { return }
            cancelFailure = nil
            Task {
                var request = Mavio_System_V1_CancelAiLoginRequest()
                request.loginID = id
                do {
                    _ = try await model.call(
                        server, Procedure.aiProviders("CancelAiLogin"), request,
                        as: Mavio_System_V1_CancelAiLoginResponse.self)
                    loginId = nil
                } catch {
                    cancelFailure = error.reason
                }
            }
        }

        // MARK: API key

        @ViewBuilder private func apiKeySection(_ p: Mavio_Plugin_V1_AiProvider) -> some View {
            Section {
                FormRow(label: MavioStrings.aiApiKey) {
                    SecureField(
                        MavioStrings.aiApiKey, text: $key, prompt: p.credential == .apiKey ? Text("••••••••") : nil
                    )
                    .fieldLabelHidden()
                    .frame(width: 288)
                    .onChange(of: key) { keySaved = false }
                }
                HStack(spacing: MavioSpacing.s12) {
                    Button(keySaving ? "…" : MavioStrings.actionSave) { saveKey(p) }
                        .buttonStyle(.borderedProminent)
                        .disabled(key.trimmingCharacters(in: .whitespaces).isEmpty || keySaving)
                    if keySaved {
                        Text(MavioStrings.aiApiKeySaved).foregroundStyle(theme.colors.textSecondary)
                    }
                    if let keyFailure {
                        Text(keyFailure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                    }
                }
                .type(theme.type.callout)
            } header: {
                Text(MavioStrings.aiApiKey)
            } footer: {
                Text(p.credential == .apiKey ? MavioStrings.aiApiKeyReplaceFooter : MavioStrings.aiApiKeyFooter)
            }
        }

        private func saveKey(_ p: Mavio_Plugin_V1_AiProvider) {
            keySaving = true
            keyFailure = nil
            Task {
                defer { keySaving = false }
                var request = Mavio_System_V1_SetAiApiKeyRequest()
                request.provider = p.id
                request.key = key.trimmingCharacters(in: .whitespaces)
                do {
                    _ = try await model.call(
                        server, Procedure.aiProviders("SetAiApiKey"), request,
                        as: Mavio_System_V1_SetAiApiKeyResponse.self)
                    key = ""
                    keySaved = true
                    await load()
                    await changed()
                } catch {
                    keyFailure = error.reason
                }
            }
        }

        // MARK: Signing out

        @ViewBuilder private func signOutSection(_ p: Mavio_Plugin_V1_AiProvider) -> some View {
            let action = p.credential == .oauth ? MavioStrings.aiSignOut : MavioStrings.aiRemoveKey
            Section {
                Button(action, role: .destructive) { confirming = true }
                    .disabled(signingOut)
                    .confirmationDialog(action, isPresented: $confirming, titleVisibility: .visible) {
                        Button(action, role: .destructive) { signOut(p) }
                    } message: {
                        Text(MavioStrings.aiSignOutFooter)
                    }
                if let signOutFailure {
                    Text(signOutFailure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                }
            }
        }

        private func signOut(_ p: Mavio_Plugin_V1_AiProvider) {
            signingOut = true
            signOutFailure = nil
            Task {
                defer { signingOut = false }
                var request = Mavio_System_V1_LogoutAiRequest()
                request.provider = p.id
                do {
                    _ = try await model.call(
                        server, Procedure.aiProviders("LogoutAi"), request, as: Mavio_System_V1_LogoutAiResponse.self)
                    await load()
                    await changed()
                } catch {
                    signOutFailure = error.reason
                }
            }
        }

        // MARK: Models

        @ViewBuilder private func modelsSection(_ p: Mavio_Plugin_V1_AiProvider) -> some View {
            Section {
                if let models, !models.isEmpty {
                    // Providers list up to hundreds of models; the list scrolls within the group.
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(Array(models.enumerated()), id: \.element.id) { i, m in
                                if i > 0 { Divider() }
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(m.name.isEmpty ? m.id : m.name)
                                    Text(AiPickerOption.detail(of: m))
                                        .type(theme.type.footnote)
                                        .foregroundStyle(theme.colors.textSecondary)
                                        .textSelection(.enabled)
                                }
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .padding(.vertical, 8)
                            }
                        }
                    }
                    .frame(maxHeight: 448)
                    .fixedSize(horizontal: false, vertical: models.count < 9)
                } else if models != nil {
                    Text(MavioStrings.aiNoModels).foregroundStyle(theme.colors.textSecondary)
                } else if let modelsFailure {
                    Text(modelsFailure).foregroundStyle(theme.colors.danger)
                } else {
                    ProgressView().frame(maxWidth: .infinity)
                }
            } header: {
                Text(MavioStrings.aiModels)
            } footer: {
                if !p.available { Text(MavioStrings.aiModelsFooter) }
            }
        }

        private func load() async {
            var request = Mavio_System_V1_ListAiModelsRequest()
            request.provider = id
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
                models = error.isNoAiGateway ? [] : nil
                modelsFailure = error.isNoAiGateway ? nil : error.reason
            }
        }
    }

    // MARK: The login's steps

    /// What a login showed: a message with links, a page to open, a code to enter, or its progress.
    private struct AiLoginEventRow: View {
        let event: Mavio_Plugin_V1_AiLoginEvent
        @Environment(\.theme) private var theme
        @Environment(\.openURL) private var openURL

        var body: some View {
            switch event.event {
            case .info(let info):
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    Text(info.message).textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                    ForEach(info.links, id: \.url) { l in
                        if let url = URL(string: l.url) {
                            Link(l.label.isEmpty ? l.url : l.label, destination: url)
                                .foregroundStyle(theme.colors.accentText)
                        }
                    }
                }
                .type(theme.type.callout)
            case .url(let page):
                VStack(alignment: .leading, spacing: 6) {
                    if !page.instructions.isEmpty {
                        Text(page.instructions).textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                    }
                    Button(MavioStrings.aiOpenPage) {
                        if let url = URL(string: page.url) { openURL(url) }
                    }
                    .buttonStyle(.bordered)
                    AiCopyableUrl(url: page.url)
                }
                .type(theme.type.callout)
            case .deviceCode(let code):
                VStack(alignment: .leading, spacing: 6) {
                    Text(MavioStrings.aiEnterCode)
                    Text(code.userCode)
                        .font(.system(.title3, design: .monospaced))
                        .tracking(4)
                        .textSelection(.enabled)
                        .padding(.horizontal, MavioSpacing.s12)
                        .padding(.vertical, 6)
                        .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: 6))
                    if let url = URL(string: code.verificationUri) {
                        Link(code.verificationUri, destination: url).foregroundStyle(theme.colors.accentText)
                    }
                }
                .type(theme.type.callout)
            case .progress(let text):
                Text(text).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
            case nil:
                EmptyView()
            }
        }
    }

    /// A long address on one line, cut to fit, with a button copying it.
    private struct AiCopyableUrl: View {
        let url: String
        @Environment(\.theme) private var theme
        @State private var copied = false

        var body: some View {
            HStack(spacing: MavioSpacing.s4) {
                Text(url)
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
                    .help(url)
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(url, forType: .string)
                    copied = true
                } label: {
                    Icon(copied ? "check" : "copy", size: 16).foregroundStyle(theme.colors.textSecondary)
                }
                .buttonStyle(.borderless)
                .help(copied ? MavioStrings.usersCopied : MavioStrings.aiCopyLink)
                .accessibilityLabel(MavioStrings.aiCopyLink)
            }
        }
    }

    /// What a login asks: text, a secret, a choice or a code pasted back, and the button answering it.
    private struct AiLoginPromptRow: View {
        let prompt: Mavio_Plugin_V1_AiLoginPrompt
        let busy: Bool
        let onAnswer: (String) -> Void
        @Environment(\.theme) private var theme
        @State private var value: String

        init(prompt: Mavio_Plugin_V1_AiLoginPrompt, busy: Bool, onAnswer: @escaping (String) -> Void) {
            (self.prompt, self.busy, self.onAnswer) = (prompt, busy, onAnswer)
            _value = State(initialValue: prompt.kind == .select ? prompt.options.first?.id ?? "" : "")
        }

        var body: some View {
            let label = prompt.message.isEmpty ? MavioStrings.aiAnswer : prompt.message
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(label).textSelection(.enabled).fixedSize(horizontal: false, vertical: true)
                switch prompt.kind {
                case .select:
                    Picker(label, selection: $value) {
                        ForEach(prompt.options, id: \.id) { o in
                            Text(o.description_p.isEmpty ? o.label : "\(o.label) — \(o.description_p)").tag(o.id)
                        }
                    }
                    .labelsHidden()
                    .fixedSize()
                case .secret:
                    SecureField(label, text: $value, prompt: Text(prompt.placeholder))
                        .labelsHidden()
                        .textFieldStyle(.roundedBorder)
                default:
                    TextField(label, text: $value, prompt: Text(prompt.placeholder))
                        .labelsHidden()
                        .textFieldStyle(.roundedBorder)
                        .autocorrectionDisabled()
                }
                if prompt.kind == .manualCode {
                    Text(MavioStrings.aiManualCode).type(theme.type.footnote).foregroundStyle(
                        theme.colors.textSecondary)
                }
                Button(busy ? "…" : MavioStrings.aiSubmit) {
                    onAnswer(value.trimmingCharacters(in: .whitespaces))
                }
                .buttonStyle(.borderedProminent)
                .disabled(value.trimmingCharacters(in: .whitespaces).isEmpty || busy)
            }
        }
    }
#endif
