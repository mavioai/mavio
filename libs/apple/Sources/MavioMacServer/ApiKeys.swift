#if os(macOS)
    import AppKit
    import MavioMac

    /// API keys (docs/clients.md §6.3.15): the keys scripts call the server
    /// with, each acting as the administrator who made it; a key's token is
    /// shown once, when it is made. A key is revoked from its menu, or by
    /// choosing it in television mode.
    final class ApiKeysPage: ServerPage {
        private var keys: [Mavio_Auth_V1_ApiKey]?
        private var users: [String: String] = [:]
        private var failure: String?
        private var name = ""
        private var creating = false
        private var createFailure: String?
        private var made: (id: String, name: String, token: String)?
        private var copied = false
        private lazy var form = FormPage(
            title: MavioStrings.settingsApiKeys, message: MavioStrings.keysMessage, theme: theme)
        private lazy var createSection = section(nil, footer: MavioStrings.keysCreateFooter)
        private let madeHolder = NSStackView.vertical([])
        private lazy var listSection = section(MavioStrings.keysList)
        private lazy var input: (FocusFrame, TextInput) = field("", placeholder: MavioStrings.keysName, width: nil) {
            [weak self] text in
            self?.name = text
            self?.refreshCreate()
        }
        private lazy var createButton = button(MavioStrings.keysCreate, kind: .primary) { [weak self] in self?.create()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsApiKeys
            input.1.onSubmit = { [weak self] in self?.create() }
            form.add(createSection)
            form.add(madeHolder)
            form.add(listSection)
            renderCreate()
            renderMade()
            renderList()
            show(form)
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func refreshCreate() {
            createButton.isEnabled = !name.trimmingCharacters(in: .whitespaces).isEmpty && !creating
            createButton.title = creating ? "…" : MavioStrings.keysCreate
        }

        private func renderCreate() {
            var rows: [NSView] = [ControlRow(input.0, controls: [createButton], theme: theme)]
            if let createFailure { rows.append(note(createFailure, failed: true)) }
            createSection.setRows(rows)
            refreshCreate()
        }

        private func renderMade() {
            guard let made, keys?.contains(where: { $0.id == made.id }) == true else {
                madeHolder.setViews([])
                madeHolder.isHidden = true
                return
            }
            madeHolder.isHidden = false
            let shown = FormSection(
                MavioStrings.keysMadeTitle(name: made.name), footer: MavioStrings.keysMadeFooter, theme: theme)
            let token = NSTextField.label(
                made.token, theme.fonts.callout, color: theme.colors.textPrimary, lines: 0, selectable: true)
            token.font = .monospacedSystemFont(ofSize: theme.fonts.callout.size, weight: .regular)
            let copy = button(copied ? MavioStrings.usersCopied : MavioStrings.keysCopy) { [weak self] in
                copyToClipboard(made.token)
                self?.copied = true
                self?.renderMade()
            }
            let done = button(MavioStrings.keysDone, kind: .link) { [weak self] in
                self?.made = nil
                self?.renderMade()
                self?.refocus()
            }
            shown.setRows([
                padded(token),
                ControlRow(NSStackView.horizontal([copy, Spacer()]), controls: [done], theme: theme),
            ])
            madeHolder.setViews([shown])
            shown.widthAnchor.constraint(equalTo: madeHolder.widthAnchor).isActive = true
        }

        private func renderList() {
            var rows: [NSView] = (keys ?? []).map(row)
            if let failure {
                rows.append(note(failure, failed: true))
            } else if keys?.isEmpty == true {
                rows.append(note(MavioStrings.keysNone))
            }
            listSection.setRows(rows)
            refocus()
        }

        private func row(_ key: Mavio_Auth_V1_ApiKey) -> NSView {
            let content = text(key.name, detail(key))
            let row = ContentRow(content, chevron: false, theme: theme)
            if theme.isTV { row.onPress = { [weak self] in self?.confirm(key) } }
            row.contextMenu = { [weak self] in
                let menu = NSMenu()
                menu.addItem(MenuAction(MavioStrings.keysRevoke) { self?.confirm(key) })
                return menu
            }
            return row
        }

        private func detail(_ key: Mavio_Auth_V1_ApiKey) -> String {
            [
                users[key.userID].map { MavioStrings.keysActsAs(name: $0) },
                key.hasCreateTime ? MavioStrings.keysCreated(when: fromNow(key.createTime.date)) : nil,
                key.hasLastUseTime
                    ? MavioStrings.keysUsed(when: fromNow(key.lastUseTime.date)) : MavioStrings.keysNeverUsed,
            ]
            .compactMap { $0 }
            .joined(separator: " · ")
        }

        private func load() async {
            do {
                keys = try await model.call(
                    server, Procedure.auth("ListApiKeys"), Mavio_Auth_V1_ListApiKeysRequest(),
                    as: Mavio_Auth_V1_ListApiKeysResponse.self
                ).apiKeys
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            if users.isEmpty,
                let r = try? await model.call(
                    server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                    as: Mavio_User_V1_ListUsersResponse.self)
            {
                users = Dictionary(r.users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
            }
            renderMade()
            renderList()
        }

        private func create() {
            let trimmed = name.trimmingCharacters(in: .whitespaces)
            guard !trimmed.isEmpty, !creating else { return }
            endEditing()
            creating = true
            createFailure = nil
            refreshCreate()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Auth_V1_CreateApiKeyRequest()
                request.name = trimmed
                do {
                    let r = try await model.call(
                        server, Procedure.auth("CreateApiKey"), request, as: Mavio_Auth_V1_CreateApiKeyResponse.self)
                    made = (r.apiKey.id, r.apiKey.name.isEmpty ? trimmed : r.apiKey.name, r.token)
                    copied = false
                    name = ""
                    input.1.text = ""
                    creating = false
                    renderCreate()
                    await load()
                } catch {
                    createFailure = error.reason
                    creating = false
                    renderCreate()
                }
            }
        }

        private func confirm(_ key: Mavio_Auth_V1_ApiKey) {
            confirm(
                MavioStrings.keysRevokeTitle(name: key.name), message: MavioStrings.keysRevokeMessage,
                button: MavioStrings.keysRevoke
            ) { [weak self] in self?.revoke(key) }
        }

        private func revoke(_ key: Mavio_Auth_V1_ApiKey) {
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Auth_V1_RevokeApiKeyRequest()
                request.id = key.id
                do {
                    _ = try await model.call(
                        server, Procedure.auth("RevokeApiKey"), request, as: Mavio_Auth_V1_RevokeApiKeyResponse.self)
                    await load()
                } catch {
                    failure = error.reason
                    renderList()
                }
            }
        }
    }
#endif
