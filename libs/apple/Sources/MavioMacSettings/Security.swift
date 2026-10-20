#if os(macOS)
    import AppKit
    import MavioMac

    /// Sign-In and Security on a standalone server, as the web UI's: the
    /// password, the passkeys, the authenticator app and the PIN; a managed
    /// user has none of them, and the sections hide.
    @MainActor
    final class SecurityEditor {
        /// The sections, which the page adds: the security's and the
        /// passkeys'.
        let sections: [FormSection]
        private let main: FormSection
        private let passkeys: FormSection
        private weak var page: Page?
        private let server: ServerRecord
        private let theme: Theme
        private var security: AccountSecurity?
        private var busy = false
        private var failure: String?
        private var editing: Editing?
        private var totp: TotpSetup?
        private var passwordChanged = false
        private let current = NSSecureTextField()
        private let new = NSSecureTextField()
        private let pin = NSSecureTextField()
        private let code = SettingsParts.codeField(placeholder: "000000")
        private var watchers: [FieldWatcher] = []
        private var saveButton: MavioButton?

        private enum Editing { case password, pin }

        /// The sign-in and security of page's user on server.
        init(page: Page, server: ServerRecord) {
            self.page = page
            self.server = server
            theme = page.theme
            main = FormSection(MavioStrings.authSecurity, theme: page.theme)
            passkeys = FormSection(MavioStrings.authPasskeys, theme: page.theme)
            sections = [main, passkeys]
            for section in sections { section.isHidden = true }
            current.placeholderString = MavioStrings.authCurrentPassword
            new.placeholderString = MavioStrings.authNewPassword
            pin.placeholderString = MavioStrings.authPin
            for field in [current, new, pin] {
                field.bezelStyle = .roundedBezel
                field.translatesAutoresizingMaskIntoConstraints = false
            }
            watchers = [
                FieldWatcher(current) { [weak self] _ in self?.updateSave() },
                FieldWatcher(new) { [weak self] _ in self?.updateSave() },
                FieldWatcher(pin) { [weak self] text in
                    guard let self else { return }
                    let digits = SettingsParts.digits(text, 4)
                    if digits != text { pin.stringValue = digits }
                    updateSave()
                },
                FieldWatcher(code) { [weak self] text in
                    guard let self else { return }
                    let digits = SettingsParts.digits(text, 6)
                    if digits != text { code.stringValue = digits }
                    updateSave()
                },
            ]
        }

        /// Shows security, once read.
        func set(_ security: AccountSecurity?) {
            self.security = security
            render()
        }

        private func render() {
            guard let security, !security.managed else {
                for section in sections { section.isHidden = true }
                return
            }
            for section in sections { section.isHidden = false }
            saveButton = nil
            var rows: [NSView] = []
            if security.hasPassword { rows.append(contentsOf: passwordRows()) }
            rows.append(contentsOf: totpRows(on: security.hasTotp))
            rows.append(contentsOf: pinRows(on: security.hasPin))
            main.setRows(rows)
            main.footer = failure
            if security.passkeys.isEmpty {
                passkeys.setRows([
                    SettingsParts.text(MavioStrings.authNoPasskeys, color: theme.colors.textSecondary, theme: theme)
                ])
            } else {
                passkeys.setRows(security.passkeys.map(passkeyRow))
            }
            updateSave()
            if let view = page?.view { FocusEngine.active(for: view)?.schedule() }
        }

        // MARK: Rows

        private func passwordRows() -> [NSView] {
            var buttons: [MavioButton] = []
            if editing != .password {
                buttons.append(
                    button(MavioStrings.authChangePassword) { [weak self] in
                        guard let self else { return }
                        passwordChanged = false
                        editing = .password
                        render()
                        focus(current)
                    })
            }
            var rows = [
                row(
                    MavioStrings.authPassword,
                    detail: passwordChanged ? MavioStrings.authPasswordChanged : MavioStrings.authPasswordMessage,
                    buttons: buttons)
            ]
            if editing == .password {
                rows.append(fieldRow(current))
                rows.append(fieldRow(new))
                rows.append(
                    actions(enabled: { [weak self] in
                        guard let self else { return false }
                        return !current.stringValue.isEmpty && !new.stringValue.isEmpty
                    }) { [weak self] client, serverId in
                        guard let self else { return }
                        try await client.setPassword(
                            serverId: serverId, current: current.stringValue, new: new.stringValue)
                        passwordChanged = true
                    })
            }
            return rows
        }

        private func totpRows(on: Bool) -> [NSView] {
            var buttons: [MavioButton] = []
            if on {
                buttons.append(
                    button(MavioStrings.authTurnOff, destructive: true) { [weak self] in
                        self?.change { client, serverId in try await client.deleteTotp(serverId: serverId) }
                    })
            } else if totp == nil {
                buttons.append(
                    button(MavioStrings.authSetUpTotp) { [weak self] in
                        self?.change { [weak self] client, serverId in
                            let setup = try await client.startTotp(serverId: serverId)
                            self?.totp = setup
                        }
                    })
            }
            var rows = [
                row(
                    MavioStrings.authTotp, detail: on ? MavioStrings.authTotpOn : MavioStrings.authTotpOff,
                    buttons: buttons)
            ]
            if let totp, !on {
                rows.append(
                    SettingsParts.padded(
                        NSTextField.label(
                            MavioStrings.authTotpSecret(secret: totp.secret), theme.fonts.callout,
                            color: theme.colors.textSecondary, lines: 0, selectable: true),
                        theme: theme))
                // The authenticator opens from its link, which television
                // mode has no app for.
                if !theme.isTV, let uri = URL(string: totp.uri) {
                    rows.append(
                        SettingsParts.padded(
                            NSStackView.horizontal(
                                [button(MavioStrings.authOpenAuthenticator) { NSWorkspace.shared.open(uri) }]),
                            theme: theme))
                }
                code.placeholderString = "000000"
                rows.append(fieldRow(code, width: theme.isTV ? 240 : 140))
                let save = button(MavioStrings.actionSave) { [weak self] in
                    guard let self else { return }
                    let digits = code.stringValue
                    change { [weak self] client, serverId in
                        try await client.confirmTotp(serverId: serverId, code: digits)
                        self?.totp = nil
                        self?.code.stringValue = ""
                    }
                }
                let cancel = button(MavioStrings.actionCancel, plain: true) { [weak self] in
                    guard let self else { return }
                    self.totp = nil
                    code.stringValue = ""
                    render()
                }
                saveButton = save
                saveEnabled = { [weak self] in self?.code.stringValue.count == 6 }
                rows.append(
                    SettingsParts.padded(
                        NSStackView.horizontal([save, cancel], spacing: MavioSpacing.s20), theme: theme))
            }
            return rows
        }

        private func pinRows(on: Bool) -> [NSView] {
            var buttons: [MavioButton] = []
            if on {
                buttons.append(
                    button(MavioStrings.authRemovePin, destructive: true) { [weak self] in
                        self?.change { client, serverId in try await client.setPin(serverId: serverId, pin: "") }
                    })
            }
            if editing != .pin {
                buttons.append(
                    button(MavioStrings.authSetPin) { [weak self] in
                        guard let self else { return }
                        editing = .pin
                        render()
                        focus(pin)
                    })
            }
            var rows = [
                row(
                    MavioStrings.authPin,
                    detail:
                        "\(on ? MavioStrings.authPinOn : MavioStrings.authTotpOff) · \(MavioStrings.authPinMessage)",
                    buttons: buttons)
            ]
            if editing == .pin {
                rows.append(fieldRow(pin, width: theme.isTV ? 240 : 140))
                rows.append(
                    actions(enabled: { [weak self] in self?.pin.stringValue.count == 4 }) {
                        [weak self] client, serverId in
                        guard let self else { return }
                        try await client.setPin(serverId: serverId, pin: pin.stringValue)
                    })
            }
            return rows
        }

        private func passkeyRow(_ passkey: Passkey) -> NSView {
            var detail = [MavioStrings.authPasskeyHost(host: passkey.host)]
            if let created = passkey.created { detail.append(created.formatted(date: .abbreviated, time: .omitted)) }
            let id = passkey.id
            return row(
                passkey.name, detail: detail.joined(separator: " · "),
                buttons: [
                    button(MavioStrings.actionDelete, destructive: true) { [weak self] in
                        self?.change { client, serverId in
                            try await client.deletePasskey(serverId: serverId, passkeyId: id)
                        }
                    }
                ])
        }

        /// A setting's title and state, with its buttons at its end: each a
        /// button of its own, which television mode's focus lands on.
        private func row(_ title: String, detail: String, buttons: [MavioButton]) -> NSView {
            let labels = NSStackView.vertical(
                [
                    NSTextField.label(
                        title, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary),
                    NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                ], spacing: 2)
            labels.setContentHuggingPriority(.init(1), for: .horizontal)
            labels.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            for button in buttons { button.isEnabled = !busy }
            let line = NSStackView.horizontal([labels] + buttons, spacing: MavioSpacing.s12)
            return SettingsParts.padded(line, theme: theme)
        }

        private func fieldRow(_ field: NSTextField, width: CGFloat? = nil) -> NSView {
            let box = FocusField(field, theme: theme)
            if let width {
                box.widthAnchor.constraint(equalToConstant: width).isActive = true
                return SettingsParts.padded(NSStackView.horizontal([box]), theme: theme)
            }
            box.widthAnchor.constraint(lessThanOrEqualToConstant: 420).isActive = true
            let holder = SettingsParts.padded(box, theme: theme)
            return holder
        }

        private func button(
            _ title: String, destructive: Bool = false, plain: Bool = false, action: @escaping @MainActor () -> Void
        ) -> MavioButton {
            MavioButton(title, kind: destructive ? .destructive : plain ? .plain : .link, theme: theme, action: action)
        }

        /// Save, which runs f and closes the form when it succeeds, and Cancel.
        private func actions(
            enabled: @escaping @MainActor () -> Bool,
            _ f: @escaping @MainActor (Client, String) async throws -> Void
        ) -> NSView {
            let save = button(MavioStrings.actionSave) { [weak self] in
                self?.change { [weak self] client, serverId in
                    try await f(client, serverId)
                    self?.close()
                }
            }
            let cancel = button(MavioStrings.actionCancel, plain: true) { [weak self] in
                self?.close()
                self?.render()
            }
            saveButton = save
            saveEnabled = enabled
            return SettingsParts.padded(
                NSStackView.horizontal([save, cancel], spacing: MavioSpacing.s20), theme: theme)
        }

        private var saveEnabled: @MainActor () -> Bool = { false }

        private func updateSave() {
            saveButton?.isEnabled = !busy && saveEnabled()
        }

        private func close() {
            editing = nil
            for field in [current, new, pin] { field.stringValue = "" }
        }

        private func focus(_ field: NSTextField) {
            // Typing starts at once with a pointer; television mode's focus
            // moves to it, and select starts typing.
            guard !theme.isTV else { return }
            DispatchQueue.main.async { [weak field] in field?.window?.makeFirstResponder(field) }
        }

        /// Runs a change, then reads the user's security again.
        private func change(_ f: @escaping @MainActor (Client, String) async throws -> Void) {
            guard let model = page?.model else { return }
            busy = true
            failure = nil
            render()
            let server = server
            Task { [weak self] in
                do {
                    try await f(model.client, server.id)
                } catch {
                    self?.failure = error.reason
                }
                let security = await model.accountSecurity(server)
                guard let self else { return }
                busy = false
                if let security { self.security = security }
                render()
            }
        }
    }
#endif
