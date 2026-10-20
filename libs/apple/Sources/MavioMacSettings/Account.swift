#if os(macOS)
    import AppKit
    import MavioMac

    /// The account, as the web UI's: the Mavio account with its page at
    /// mavio.dev, or the user of the server with switching users and their
    /// sign-in and security; the servers in the user's order, which orders
    /// Home; and authorizing a device with Quick Connect.
    final class AccountPage: Page {
        private var form: FormPage?
        private var observer: Observer?
        private var structure = ""
        private var loading: Task<Void, Never>?
        private var servers: FormSection?
        private var switchUsers: SwitchUsers?
        private var security: SecurityEditor?
        private var authorize: AuthorizeDevice?

        /// The server a device without a Mavio account signs in to.
        private var standalone: ServerRecord? {
            model.account == nil ? model.signedInServers.first : nil
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsAccount
            observer = Observer { [weak self] in self?.observe() }
        }

        isolated deinit {
            observer?.cancel()
            loading?.cancel()
        }

        /// Builds the page anew when who is signed in where changes, and
        /// the servers' rows when their states do.
        private func observe() {
            let key = [
                model.account?.id ?? "",
                standalone.map { "\($0.id)/\($0.userId)" } ?? "",
                model.signedInServers.map(\.id).joined(separator: ","),
            ].joined(separator: "|")
            if key != structure {
                structure = key
                build()
            } else {
                renderServers()
            }
        }

        private func build() {
            let form = FormPage(title: MavioStrings.settingsAccount, theme: theme)
            self.form = form
            var sections: [NSView] = []
            switchUsers = nil
            security = nil
            if let account = model.account {
                var rows: [NSView] = [header(account.name)]
                // A web page opens in the browser, which television mode does not.
                if !theme.isTV, let manage = Self.manage(account) {
                    rows.append(
                        FormRow(
                            MavioStrings.settingsManageAtMavio, icon: "external", tint: theme.colors.accent,
                            theme: theme
                        ) { NSWorkspace.shared.open(manage) })
                }
                sections.append(
                    FormSection(footer: MavioStrings.settingsMavioAccountMessage, rows: rows, theme: theme))
            } else if let server = model.signedInServers.first {
                sections.append(
                    FormSection(
                        footer: MavioStrings.settingsStandaloneAccountMessage(
                            name: server.userName, server: server.name),
                        rows: [header(server.userName)], theme: theme))
                let switchUsers = SwitchUsers(page: self, server: server)
                self.switchUsers = switchUsers
                sections.append(switchUsers.section)
                let security = SecurityEditor(page: self, server: server)
                self.security = security
                sections.append(contentsOf: security.sections)
            }
            let servers = FormSection(
                MavioStrings.authServers,
                footer: model.account != nil
                    ? MavioStrings.settingsServersMavioMessage : MavioStrings.settingsServersMessage,
                theme: theme)
            self.servers = servers
            sections.append(servers)
            if !model.signedInServers.isEmpty {
                let authorize = AuthorizeDevice(page: self)
                self.authorize = authorize
                sections.append(authorize.section)
            } else {
                authorize = nil
            }
            form.setSections(sections)
            renderServers()
            show(form)
            FocusEngine.active(for: view)?.schedule()
            load()
        }

        /// Reads the users to switch to and the user's security, on a
        /// standalone server.
        private func load() {
            loading?.cancel()
            guard let server = standalone else { return }
            loading = Task { [weak self] in
                guard let model = self?.model else { return }
                async let readUsers = model.switchableUsers(server)
                async let readSecurity = model.accountSecurity(server)
                let (u, s) = await (readUsers, readSecurity)
                guard !Task.isCancelled, let self else { return }
                switchUsers?.set(u, server: server)
                security?.set(s)
            }
        }

        /// A name beside its avatar, as the account's section begins.
        private func header(_ name: String) -> NSView {
            let row = NSStackView.horizontal(
                [
                    AvatarView(name: name, size: theme.isTV ? 64 : 44, theme: theme),
                    NSTextField.label(name, theme.fonts.headline, color: theme.colors.textPrimary),
                ], spacing: MavioSpacing.s12)
            return SettingsParts.padded(row, theme: theme)
        }

        // MARK: Servers

        private func renderServers() {
            guard let servers else { return }
            var rows: [NSView] = model.servers.map(serverRow)
            // A Mavio account follows its own servers, removing any other
            // the device signs in to.
            if model.account == nil {
                rows.append(
                    FormRow.action(MavioStrings.authAddServer, theme: theme) { [weak self] in
                        self?.context.addServer()
                    })
            }
            servers.setRows(rows)
            FocusEngine.active(for: view)?.schedule()
        }

        private func serverRow(_ server: ServerRecord) -> NSView {
            let signedIn = model.signedIn.contains(server.id)
            let status: NSView? =
                switch model.states[server.id] {
                case .online?:
                    Box(fill: theme.colors.success, radius: 4).size(width: 8, height: 8)
                case .offline?:
                    IconView("offline", size: 18, color: theme.colors.textTertiary)
                default: nil
                }
            let menu = serverMenu(server)
            let row = FormRow(
                server.name,
                subtitle: [
                    signedIn ? MavioStrings.authSignedInAs(name: server.userName) : MavioStrings.authSignedOut,
                    server.address,
                ].joined(separator: "\n"),
                accessory: status, theme: theme)
            row.menu = menu
            // In television mode, select opens what a right-click would.
            if theme.isTV {
                row.onPress = { [weak row] in
                    guard let row else { return }
                    menu.popUp(positioning: nil, at: NSPoint(x: row.bounds.midX, y: row.bounds.midY), in: row)
                }
            }
            return row
        }

        /// Signing in again, moving in the order and signing out.
        private func serverMenu(_ server: ServerRecord) -> NSMenu {
            let menu = NSMenu()
            let model = model
            if model.account == nil, !model.signedIn.contains(server.id) {
                menu.addItem(
                    MenuAction(MavioStrings.authSignIn, icon: "profile") { [weak self] in self?.context.addServer() })
            }
            let ids = model.servers.map(\.id)
            if let index = ids.firstIndex(of: server.id) {
                if index > 0 {
                    menu.addItem(
                        MenuAction(MavioStrings.authMoveUp) {
                            var order = ids
                            order.swapAt(index, index - 1)
                            model.order(order)
                        })
                }
                if index < ids.count - 1 {
                    menu.addItem(
                        MenuAction(MavioStrings.authMoveDown) {
                            var order = ids
                            order.swapAt(index, index + 1)
                            model.order(order)
                        })
                }
            }
            menu.addItem(.separator())
            menu.addItem(
                MenuAction(MavioStrings.authSignOut, icon: "signOut") {
                    Task { try? await model.remove(server.id) }
                })
            return menu
        }

        /// The account's page at the mavio.dev whose API it signed in with.
        private static func manage(_ account: Account) -> URL? {
            guard var site = URL(string: account.api) else { return nil }
            if site.lastPathComponent == "api" { site.deleteLastPathComponent() }
            return site.appending(path: "account")
        }
    }

    /// Approves the Quick Connect code another device shows, on the server
    /// chosen when there are several.
    @MainActor
    final class AuthorizeDevice {
        /// The section, which the page adds.
        let section: FormSection
        private weak var page: Page?
        private var serverId: String
        private let field = SettingsParts.codeField(placeholder: "000000")
        private var watcher: FieldWatcher?
        private let button: MavioButton
        private let message: NSTextField

        /// Authorizing devices on the servers page's user is signed in to.
        init(page: Page) {
            self.page = page
            let theme = page.theme
            let servers = page.model.signedInServers
            serverId = servers.first?.id ?? ""
            section = FormSection(MavioStrings.authAuthorizeDevice, theme: theme)
            button = MavioButton(MavioStrings.authAuthorize, kind: .primary, theme: theme)
            message = .label("", theme.fonts.callout, color: theme.colors.textSecondary, lines: 0)
            message.isHidden = true
            button.onPress = { [weak self] in self?.authorize() }
            watcher = FieldWatcher(
                field,
                onChange: { [weak self] text in
                    guard let self else { return }
                    let digits = SettingsParts.digits(text, 6)
                    if digits != text { field.stringValue = digits }
                    update()
                },
                onSubmit: { [weak self] in self?.authorize() })
            var rows: [NSView] = []
            if servers.count > 1 {
                rows.append(
                    FormRow.choice(
                        MavioStrings.authServer, options: servers.map { ($0.name, $0.id) }, selected: serverId,
                        theme: theme
                    ) { [weak self] in
                        self?.serverId = $0
                        self?.update()
                    })
            }
            let box = FocusField(field, theme: theme)
            box.widthAnchor.constraint(equalToConstant: theme.isTV ? 240 : 140).isActive = true
            rows.append(
                SettingsParts.padded(
                    NSStackView.horizontal([box, button, message], spacing: MavioSpacing.s12), theme: theme))
            section.setRows(rows)
            update()
        }

        private func update() {
            button.isEnabled = field.stringValue.count == 6 && !serverId.isEmpty
        }

        private func authorize() {
            let code = field.stringValue
            guard code.count == 6, !serverId.isEmpty, let model = page?.model, let theme = page?.theme else { return }
            button.isEnabled = false
            let serverId = serverId
            Task { [weak self] in
                do {
                    try await model.client.authorizeQuickConnect(serverId: serverId, code: code)
                    guard let self else { return }
                    message.stringValue = MavioStrings.authAuthorized
                    message.textColor = theme.colors.textSecondary
                    field.stringValue = ""
                } catch {
                    guard let self else { return }
                    message.stringValue = error.reason
                    message.textColor = theme.colors.danger
                }
                self?.message.isHidden = false
                self?.update()
            }
        }
    }
#endif
