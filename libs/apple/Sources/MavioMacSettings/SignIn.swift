#if os(macOS)
    import AppKit
    import AuthenticationServices
    import MavioMac

    /// The steps of signing in, as the web UI's and the Android app's.
    private enum Step: Equatable {
        /// The way in: a Mavio account, or a server's address.
        case welcome
        /// A link code approved on another device.
        case link
        /// A server's address, typed or found on the network.
        case address
        /// The name and password of a server's user.
        case credentials
        /// A Quick Connect code entered on a device signed in.
        case quickConnect
        /// A server not set up yet, which its setup in a browser does.
        case setUp
    }

    /// Sign-in (docs/accounts.md §6), as the web UI's in a wide window: a
    /// card on the wall of posters with a Mavio account, in the browser or
    /// with a link code, or a server's address, then its users' name and
    /// password or Quick Connect. A server in mavio.dev mode goes back to
    /// the Mavio account, at its mavio.dev. Television mode signs a Mavio
    /// account in with a link code alone and offers Quick Connect first
    /// (docs/tv.md §6.5).
    final class SignInPage: Page, NSTextFieldDelegate, ASWebAuthenticationPresentationContextProviding {
        private let adding: Bool
        private let done: @MainActor () -> Void
        private var step = Step.welcome
        private var history: [Step] = []
        /// The server connected to, whose mavio.dev a Mavio account signs in at.
        private var info: ServerInfo?
        private var address = ""
        private var found: [FoundServer] = []
        private var name = ""
        private var password = ""
        private var quickConnect: QuickConnectRequest?
        private var link: LinkCode?
        private var error: String?
        private var busy = false

        private let steps = NSStackView.vertical([], spacing: 22)
        private var continueButton: StepButton?
        private var signInButton: StepButton?
        private var discovering: Task<Void, Never>?
        private var polling: Task<Void, Never>?
        private var linking: Task<Void, Never>?
        private var running: Task<Void, Never>?

        /// The client ID of this app at mavio.dev.
        private static let clientId = "dev.mavio.macos"

        /// Signing in, or adding a server when adding; done is called once
        /// signed in.
        init(context: MacContext, adding: Bool, done: @escaping @MainActor () -> Void) {
            self.adding = adding
            self.done = done
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = adding ? MavioStrings.authAddServer : MavioStrings.authSignIn
            if adding { step = .address }
            let wall = PosterWallView()
            let page = NSView()
            page.translatesAutoresizingMaskIntoConstraints = false
            page.addFilling(wall)
            let scroll = NSScrollView()
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.translatesAutoresizingMaskIntoConstraints = false
            let document = FlippedView()
            document.translatesAutoresizingMaskIntoConstraints = false
            scroll.documentView = document
            page.addFilling(scroll)
            let tv = theme.isTV
            let card = SignInCard(
                steps,
                insets: NSEdgeInsets(
                    top: tv ? 56 : 40, left: tv ? 56 : 40, bottom: tv ? 48 : 34, right: tv ? 56 : 40))
            document.addSubview(card)
            let margin = theme.layout.margin
            NSLayoutConstraint.activate([
                document.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
                document.trailingAnchor.constraint(equalTo: scroll.contentView.trailingAnchor),
                document.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
                document.heightAnchor.constraint(greaterThanOrEqualTo: scroll.contentView.heightAnchor),
                card.centerXAnchor.constraint(equalTo: document.centerXAnchor),
                card.centerYAnchor.constraint(equalTo: document.centerYAnchor),
                card.topAnchor.constraint(greaterThanOrEqualTo: document.topAnchor, constant: margin),
                card.leadingAnchor.constraint(greaterThanOrEqualTo: document.leadingAnchor, constant: margin),
                card.widthAnchor.constraint(lessThanOrEqualToConstant: tv ? 720 : 460),
            ])
            let wide = card.widthAnchor.constraint(equalToConstant: tv ? 720 : 460)
            wide.priority = .defaultHigh
            wide.isActive = true
            show(page)
            render()
            discovering = Task { [weak self] in
                guard let model = self?.model else { return }
                let servers = (try? await model.client.discover(wait: 2)) ?? []
                guard let self, !Task.isCancelled else { return }
                self.found = servers
                if self.step == .address { self.render() }
            }
        }

        isolated deinit {
            discovering?.cancel()
            polling?.cancel()
            linking?.cancel()
            running?.cancel()
        }

        // MARK: Steps

        /// Shows the step, its back link, and the Cancel of a sheet.
        private func render() {
            var views: [NSView] = []
            if adding && history.isEmpty {
                views.append(
                    MavioButton(MavioStrings.actionCancel, kind: .link, theme: theme) { [weak self] in self?.done() })
            } else if !history.isEmpty {
                views.append(
                    MavioButton(MavioStrings.actionBack, icon: "back", kind: .link, theme: theme) { [weak self] in
                        self?.back()
                    })
            }
            continueButton = nil
            signInButton = nil
            switch step {
            case .welcome: views += welcomeStep()
            case .link: views += linkStep()
            case .address: views += addressStep()
            case .credentials: views += credentialsStep()
            case .quickConnect: views += quickConnectStep()
            case .setUp: views += setUpStep()
            }
            steps.setViews([])
            for view in views {
                steps.addArrangedSubview(view)
                if !(view is MavioButton) { view.widthAnchor.constraint(equalTo: steps.widthAnchor).isActive = true }
            }
            steps.alphaValue = busy ? 0.6 : 1
            for case let button as StepButton in allViews(steps) where busy { button.isEnabled = false }
            FocusEngine.active(for: view)?.schedule()
        }

        private func allViews(_ root: NSView) -> [NSView] {
            root.subviews + root.subviews.flatMap(allViews)
        }

        private func errorViews() -> [NSView] {
            error.map { [SignInStyle.error($0, theme)] } ?? []
        }

        private func welcomeStep() -> [NSView] {
            var views: [NSView] = []
            views.append(SignInStyle.centered(WordmarkView(height: theme.isTV ? 56 : 40)))
            let title = NSTextField.label(
                info.map { MavioStrings.authSignInTo(server: $0.name) } ?? MavioStrings.authWelcome,
                SignInStyle.title(theme), color: theme.colors.textPrimary, lines: 0)
            title.alignment = .center
            views.append(title)
            if info != nil {
                let text = SignInStyle.text(MavioStrings.authModeMavioDevMessage, theme)
                text.alignment = .center
                views.append(text)
            }
            views += errorViews()
            if theme.isTV {
                // A television signs the account in with a link code.
                views.append(
                    StepButton(MavioStrings.authSignInWithMavio, theme: theme) { [weak self] in self?.go(.link) })
            } else {
                views.append(
                    StepButton(MavioStrings.authSignInWithMavio, theme: theme) { [weak self] in self?.browserSignIn() })
                views.append(
                    StepButton(MavioStrings.authUseLinkCode, icon: "devices", primary: false, theme: theme) {
                        [weak self] in self?.go(.link)
                    })
            }
            if info == nil {
                views.append(SignInStyle.or(theme))
                views.append(
                    StepButton(MavioStrings.authConnectToServer, icon: "server", primary: false, theme: theme) {
                        [weak self] in self?.go(.address)
                    })
            }
            return views
        }

        private func linkStep() -> [NSView] {
            var views: [NSView] = [
                SignInStyle.heading(MavioStrings.authSignInWithMavio, subtitle: info?.name, theme: theme)
            ]
            if let link {
                let url = link.verificationUri().replacing(/^https?:\/\//, with: "")
                views.append(SignInStyle.text(MavioStrings.authLinkMessage(url: url), theme))
                views.append(CodeBoxes(link.userCode(), theme: theme))
            }
            if let error {
                views.append(SignInStyle.error(error, theme))
                views.append(
                    StepButton(MavioStrings.authTryAgain, primary: false, theme: theme) { [weak self] in
                        self?.startLink()
                    })
            } else {
                views.append(SignInStyle.waiting(theme))
            }
            return views
        }

        private func addressStep() -> [NSView] {
            var views: [NSView] = [
                SignInStyle.heading(MavioStrings.authConnectToServer, theme: theme),
                SignInStyle.text(MavioStrings.authServerAddressMessage, theme),
            ]
            let field = NSTextField(string: address)
            field.placeholderString = "http://192.168.1.5:8686"
            field.delegate = self
            field.identifier = NSUserInterfaceItemIdentifier("address")
            field.contentType = .URL
            field.target = self
            field.action = #selector(submitAddress)
            views.append(
                FieldGroup([FieldGroup.row(MavioStrings.authServer, field: field, theme: theme)], theme: theme))
            views += errorViews()
            let go = StepButton(MavioStrings.actionContinue, theme: theme) { [weak self] in self?.submitAddress() }
            go.isEnabled = !address.trimmingCharacters(in: .whitespaces).isEmpty
            continueButton = go
            views.append(go)
            let known = Set(model.servers.map(\.id))
            let nearby = found.filter { !known.contains($0.id) }
            if !nearby.isEmpty {
                let header = NSTextField.label(
                    MavioStrings.authNearby.uppercased(), SignInStyle.small(theme, weight: .semibold),
                    color: theme.colors.textTertiary)
                let rows = nearby.map { server in
                    NearbyRow(name: server.name, address: server.address, theme: theme) { [weak self] in
                        self?.address = server.address
                        self?.probe(server.address)
                    } as NSView
                }
                let headerHolder = NSView()
                headerHolder.translatesAutoresizingMaskIntoConstraints = false
                headerHolder.addFilling(header, insets: .symmetric(horizontal: MavioSpacing.s16))
                let group = NSStackView.vertical(
                    [headerHolder, FieldGroup(rows, theme: theme)], spacing: MavioSpacing.s8)
                for view in group.arrangedSubviews {
                    view.widthAnchor.constraint(equalTo: group.widthAnchor).isActive = true
                }
                views.append(group)
            }
            if !theme.isTV {
                DispatchQueue.main.async { [weak field] in field?.window?.makeFirstResponder(field) }
            }
            return views
        }

        private func credentialsStep() -> [NSView] {
            guard let info else { return [] }
            var views: [NSView] = [
                SignInStyle.heading(
                    MavioStrings.authSignInTo(server: info.name),
                    subtitle: "\(MavioStrings.authStandaloneServer) · \(info.address)", theme: theme)
            ]
            let quick = StepButton(MavioStrings.authQuickConnect, icon: "devices", primary: theme.isTV, theme: theme) {
                [weak self] in self?.startQuickConnect()
            }
            if theme.isTV {
                // A television offers Quick Connect first, focused.
                views.append(quick)
                views.append(SignInStyle.or(theme))
            }
            let user = NSTextField(string: name)
            user.contentType = .username
            user.delegate = self
            user.identifier = NSUserInterfaceItemIdentifier("name")
            let secret = NSSecureTextField(string: password)
            secret.contentType = .password
            secret.delegate = self
            secret.identifier = NSUserInterfaceItemIdentifier("password")
            secret.target = self
            secret.action = #selector(submitCredentials)
            user.nextKeyView = secret
            views.append(
                FieldGroup(
                    [
                        FieldGroup.row(MavioStrings.authUsername, field: user, theme: theme),
                        FieldGroup.row(MavioStrings.authPassword, field: secret, theme: theme),
                    ], theme: theme))
            views += errorViews()
            let sign = StepButton(MavioStrings.authSignIn, primary: !theme.isTV, theme: theme) { [weak self] in
                self?.submitCredentials()
            }
            sign.isEnabled = !name.trimmingCharacters(in: .whitespaces).isEmpty
            signInButton = sign
            views.append(sign)
            if theme.isTV {
                DispatchQueue.main.async { [weak self, weak quick] in
                    guard let self, let quick else { return }
                    FocusEngine.active(for: self.view)?.prefer(quick)
                }
            } else {
                views.append(SignInStyle.or(theme))
                views.append(quick)
                DispatchQueue.main.async { [weak user] in user?.window?.makeFirstResponder(user) }
            }
            return views
        }

        private func quickConnectStep() -> [NSView] {
            var views: [NSView] = [SignInStyle.heading(MavioStrings.authQuickConnectTitle, theme: theme)]
            if let quickConnect {
                views.append(SignInStyle.text(MavioStrings.authQuickConnectCode(code: quickConnect.code()), theme))
                views.append(CodeBoxes(quickConnect.code(), theme: theme))
            }
            if let error {
                views.append(SignInStyle.error(error, theme))
            } else {
                views.append(SignInStyle.waiting(theme))
            }
            return views
        }

        private func setUpStep() -> [NSView] {
            guard let info else { return [] }
            let address = NSTextField.label(
                info.address.replacing(/^https?:\/\//, with: ""), SignInStyle.title(theme),
                color: theme.colors.textPrimary,
                lines: 0, selectable: true)
            address.font = .monospacedSystemFont(ofSize: theme.isTV ? 44 : 30, weight: .semibold)
            var views: [NSView] = [
                SignInStyle.heading(
                    MavioStrings.authSetUpElsewhere(server: info.name), eyebrow: MavioStrings.authNewServerSetup,
                    theme: theme),
                SignInStyle.text(MavioStrings.authSetUpElsewhereMessage, theme),
                address,
            ]
            // Television opens no web page: the address is for a computer
            // or phone (docs/tv.md §1).
            if !theme.isTV {
                views.append(
                    StepButton(MavioStrings.authOpenInBrowser, icon: "external", theme: theme) {
                        if let url = URL(string: info.address) { NSWorkspace.shared.open(url) }
                    })
            }
            return views
        }

        // MARK: Fields

        func controlTextDidChange(_ obj: Notification) {
            guard let field = obj.object as? NSTextField else { return }
            switch field.identifier?.rawValue {
            case "address":
                address = field.stringValue
                continueButton?.isEnabled = !address.trimmingCharacters(in: .whitespaces).isEmpty
            case "name":
                name = field.stringValue
                signInButton?.isEnabled = !name.trimmingCharacters(in: .whitespaces).isEmpty
            case "password":
                password = field.stringValue
            default:
                break
            }
        }

        @objc private func submitAddress() {
            probe(address)
        }

        @objc private func submitCredentials() {
            guard !name.trimmingCharacters(in: .whitespaces).isEmpty, let info else { return }
            run { [weak self] in
                guard let self else { return }
                try await self.model.signIn(
                    address: info.address, name: self.name.trimmingCharacters(in: .whitespaces),
                    password: self.password)
                self.finish()
            }
        }

        // MARK: Navigation

        private func go(_ next: Step) {
            history.append(step)
            step = next
            error = nil
            if next == .link { startLink() } else { render() }
        }

        private func back() {
            guard let previous = history.popLast() else { return }
            step = previous
            quickConnect = nil
            polling?.cancel()
            link = nil
            linking?.cancel()
            if step == .welcome || step == .address { info = nil }
            error = nil
            render()
        }

        // MARK: Actions

        /// Runs body, busy meanwhile, showing what went wrong.
        private func run(_ body: @escaping @MainActor () async throws -> Void) {
            running?.cancel()
            busy = true
            error = nil
            render()
            running = Task { [weak self] in
                do {
                    try await body()
                } catch let e as CoreError where e.isUnauthenticated {
                    self?.error = MavioStrings.authWrongPassword
                } catch is CancellationError {
                } catch {
                    self?.error = error.reason
                }
                guard let self else { return }
                self.busy = false
                self.render()
            }
        }

        private func probe(_ address: String) {
            let address = address.trimmingCharacters(in: .whitespaces)
            guard !address.isEmpty else { return }
            run { [weak self] in
                guard let self else { return }
                do {
                    let found = try await self.model.client.probe(address: address)
                    self.info = found
                    self.history.append(self.step)
                    self.step = found.mode == .mavioDev ? .welcome : found.setupRequired ? .setUp : .credentials
                } catch let e as CoreError where !e.isUnauthenticated {
                    if "\(e)".localizedCaseInsensitiveContains("not a Mavio") { throw NotMavio() }
                    throw e
                }
            }
        }

        private func startQuickConnect() {
            guard let info else { return }
            run { [weak self] in
                guard let self else { return }
                self.quickConnect = try await self.model.client.startQuickConnect(address: info.address)
                self.history.append(self.step)
                self.step = .quickConnect
                self.pollQuickConnect()
            }
        }

        /// Asks every three seconds whether the code was entered, until it
        /// expires.
        private func pollQuickConnect() {
            polling?.cancel()
            guard let request = quickConnect else { return }
            polling = Task { [weak self] in
                while !Task.isCancelled, Date() < request.expires() {
                    try? await Task.sleep(for: .seconds(3))
                    guard !Task.isCancelled, let self else { return }
                    do {
                        if try await self.model.pollQuickConnect(request) != nil {
                            self.finish()
                            return
                        }
                    } catch {
                        self.error = error.reason
                        self.render()
                        return
                    }
                }
                guard !Task.isCancelled, let self, self.step == .quickConnect else { return }
                self.back()
            }
        }

        /// The API of the mavio.dev the server belongs to, or the one this
        /// build signs in at (`MAVIO_DEV_API`).
        private var api: String {
            if let url = info?.mavioDevUrl, !url.isEmpty { return url }
            let built = Bundle.main.object(forInfoDictionaryKey: "MavioDevAPI") as? String ?? ""
            return built.isEmpty ? "https://mavio.dev/api" : built
        }

        /// Shows a link code and asks every interval whether it was
        /// approved, with a new code as each expires.
        private func startLink() {
            linking?.cancel()
            error = nil
            link = nil
            render()
            let api = api
            linking = Task { [weak self] in
                guard let model = self?.model else { return }
                do {
                    while !Task.isCancelled {
                        let code = try await model.client.startLink(api: api, clientId: Self.clientId)
                        guard let self, !Task.isCancelled else { return }
                        self.link = code
                        self.render()
                        while !Task.isCancelled, Date() < code.expires() {
                            try await Task.sleep(for: .seconds(max(code.interval(), 1)))
                            if try await model.pollLink(code) != nil {
                                self.finish()
                                return
                            }
                        }
                    }
                } catch is CancellationError {
                } catch {
                    guard let self, self.step == .link else { return }
                    self.link = nil
                    self.error = error.reason
                    self.render()
                }
            }
        }

        /// Signs the account in in a web authentication session. mavio.dev
        /// sends the browser back to the core's loopback address (RFC 8252),
        /// which the session cannot catch, so the session is closed once the
        /// core has the code; closing it first cancels the sign-in.
        private func browserSignIn() {
            let api = api
            run { [weak self] in
                guard let self else { return }
                let signIn = try await self.model.client.startBrowserSignIn(api: api, clientId: Self.clientId)
                guard let url = URL(string: signIn.url()) else { return }
                let session = ASWebAuthenticationSession(url: url, callbackURLScheme: "mavio") { @Sendable _, _ in
                    // The browser closed before the core had the code: the
                    // person cancelled. AuthenticationServices calls this off
                    // the main thread.
                    signIn.cancel()
                }
                session.presentationContextProvider = self
                session.start()
                defer { session.cancel() }
                do {
                    try await self.model.finishBrowserSignIn(signIn)
                } catch CoreError.NotFound {
                    return
                }
                self.finish()
            }
        }

        func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
            view.window ?? NSApp.mainWindow ?? NSWindow()
        }

        private func finish() {
            polling?.cancel()
            linking?.cancel()
            done()
        }
    }

    /// The address answers but is not a Mavio server.
    private struct NotMavio: LocalizedError {
        var errorDescription: String? { MavioStrings.authNotMavio }
    }
#endif
