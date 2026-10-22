#if os(macOS)
    import AppKit
    import MavioMac

    /// The provider of remotes' offline downloads, built into the server.
    let offlineProvider = "dev.mavio.rclone-offline"

    /// Whether the server may read a folder there: empty, an absolute path
    /// or a remote's folder.
    private func validLocal(_ s: String) -> Bool {
        let v = s.trimmingCharacters(in: .whitespaces)
        return v.isEmpty || v.firstMatch(of: /^(\/|[A-Za-z]:[\\\/]|\\\\)/) != nil
            || v.firstMatch(of: /^remote:[^\/\\:]+:(?!\/)/) != nil
    }

    /// Acquisition's page of a download client (docs/clients.md §6.3.11),
    /// as the web UI's: a link to the plugin that set it up, and the folders
    /// the client puts downloads in, with where the server reads each,
    /// whether it can, and the libraries imports from it hard link to. A
    /// container's client takes no folders of its own: they are its
    /// container's. A remote's offline downloads link to the remote's
    /// storage, where their folder is set, and show the libraries imports
    /// move files to within the drive.
    final class DownloadClientPage: FormController {
        /// A folder being edited: added ones name the client's folder too.
        private struct Draft: Equatable {
            var remote: String
            var local: String
            var added = false
            /// Mounted in the client's managed container, which is not edited here.
            var container = false
        }

        private let client: Mavio_Acquisition_V1_DownloadClient
        private let pluginName: String
        private var saved: [Mavio_Acquisition_V1_DownloadClientPath]?
        private var unreported = ""
        private var drafts: [Draft] = []
        private var failure: String?
        private var saveFailure: String?
        private var saving = false
        private var saveBar: SaveBar?

        /// The page of client of server, set up by the plugin named pluginName.
        init(
            context: MacContext, server: ServerRecord, client: Mavio_Acquisition_V1_DownloadClient,
            pluginName: String
        ) {
            self.client = client
            self.pluginName = pluginName
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            title = client.name.isEmpty ? client.id : client.name
            message = MavioStrings.clientMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        private var offline: Bool { client.pluginID == offlineProvider }
        /// Whether the folders are set elsewhere: with the container, or the remote.
        private var readOnly: Bool { !client.container.isEmpty || offline }

        override func sections() -> [NSView] {
            var views: [NSView] = [card([settingsLink()])]
            let footer =
                offline
                ? MavioStrings.clientOfflineFooter
                : client.container.isEmpty ? MavioStrings.clientFoldersFooter : MavioStrings.clientContainerFooter
            var rows: [NSView] = []
            for i in drafts.indices { rows.append(folder(i)) }
            if let saved, drafts.isEmpty, saved.isEmpty, unreported.isEmpty {
                rows.append(note(MavioStrings.clientNoFolders))
            }
            if !unreported.isEmpty {
                rows.append(note(MavioStrings.clientUnreported(failure: unreported), failed: true))
            }
            if let failure { rows.append(note(failure, failed: true)) }
            if saved == nil && failure == nil { rows.append(spinnerRow()) }
            if saved != nil && !readOnly {
                rows.append(
                    addRow(MavioStrings.clientAddFolder) { [weak self] in
                        self?.drafts.append(Draft(remote: "", local: "", added: true))
                        self?.render()
                    })
            }
            views.append(card(MavioStrings.clientFolders, footer: footer, rows))
            if saved != nil && !readOnly {
                let bar = SaveBar(MavioStrings.actionSave, theme: theme) { [weak self] in self?.save() }
                saveBar = bar
                refresh()
                views.append(bar)
            }
            return views
        }

        /// The link to where the client is set up: its remote's page, or its plugin's.
        private func settingsLink() -> NSView {
            let context = context
            let server = server
            let remote = client.id
            let plugin = client.pluginID
            if offline {
                return tileRow(MavioStrings.clientRemote, subtitle: client.id, icon: "cloud", tint: 0x30B0C7) {
                    [weak self] in self?.push(context.remotePage(remote, server, context))
                }
            }
            return tileRow(MavioStrings.clientSettings, subtitle: pluginName, icon: "plugins", tint: 0x5856D6) {
                [weak self] in self?.push(context.pluginPage(plugin, server, context))
            }
        }

        // MARK: A folder

        /// A folder's row: the client's folder, where the server reads it,
        /// and whether it can, with the folder it suggests when it cannot.
        private func folder(_ i: Int) -> NSView {
            let x = drafts[i]
            let p = x.added ? nil : saved?.first { $0.remotePath == x.remote }
            let fixed = x.container || readOnly
            var lines: [NSView] = []
            if x.added {
                let input = TextInput(x.remote, placeholder: MavioStrings.clientClientFolder, monospaced: true) {
                    [weak self] text in
                    self?.drafts[i].remote = text
                    self?.refresh()
                }
                lines.append(input)
            } else {
                lines.append(path(x.remote))
            }
            if !(fixed && x.local.isEmpty) {
                var line: [NSView] = [
                    NSTextField.label("→", theme.fonts.body, color: theme.colors.textSecondary)
                ]
                if fixed {
                    line.append(path(x.local))
                } else {
                    let input = TextInput(x.local, placeholder: MavioStrings.clientReadAt, monospaced: true)
                    input.field.setAccessibilityLabel(MavioStrings.clientServerFolder)
                    input.onChange = { [weak self, weak input] text in
                        guard let self else { return }
                        self.drafts[i].local = text
                        input?.field.textColor =
                            validLocal(text) ? self.theme.colors.textPrimary : self.theme.colors.danger
                        self.refresh()
                    }
                    if !validLocal(x.local) { input.field.textColor = theme.colors.danger }
                    line.append(input)
                }
                if !fixed && (x.added || p.map { !$0.rootFolder } == true) {
                    line.append(
                        MavioButton(MavioStrings.rulesRemove, kind: .destructive, theme: theme) { [weak self] in
                            self?.drafts.remove(at: i)
                            self?.render()
                        })
                }
                let stack = NSStackView.horizontal(line, spacing: MavioSpacing.s8)
                lines.append(stack)
            }
            if let p, x.local == p.localPath {
                lines.append(status(p, container: x.container))
                if !fixed && !p.readable && !p.suggestedPath.isEmpty && x.local.isEmpty {
                    let suggested = p.suggestedPath
                    lines.append(
                        MavioButton(MavioStrings.clientUse(path: suggested), kind: .secondary, theme: theme) {
                            [weak self] in
                            self?.drafts[i].local = suggested
                            self?.render()
                        })
                }
            }
            let stack = NSStackView.vertical(lines, spacing: MavioSpacing.s8)
            for view in lines where view is TextInput || view is NSStackView {
                view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            return row(stack)
        }

        /// A folder's path, in a monospaced font, selectable.
        private func path(_ text: String) -> NSTextField {
            let label = NSTextField.label(
                text, theme.fonts.body, color: theme.colors.textPrimary, lines: 0, selectable: true)
            label.font = .monospacedSystemFont(ofSize: theme.fonts.body.size, weight: .regular)
            return label
        }

        /// Whether the server reads a saved folder, and how imports reach the libraries, or why it cannot.
        private func status(_ p: Mavio_Acquisition_V1_DownloadClientPath, container: Bool) -> NSView {
            let text = NSMutableAttributedString()
            let font = theme.fonts.footnote.font
            func add(_ s: String, _ color: NSColor) {
                text.append(NSAttributedString(string: s, attributes: [.font: font, .foregroundColor: color]))
            }
            let secondary = theme.colors.textSecondary
            if container { add(MavioStrings.clientContainerFolder + " · ", secondary) }
            if p.readable {
                add(MavioStrings.clientReadable, theme.colors.success)
                let paths = p.linkLibraryPaths.joined(separator: ", ")
                let how =
                    p.linkLibraryPaths.isEmpty
                    ? (p.moves ? MavioStrings.clientTransfers : MavioStrings.clientCopies)
                    : (p.moves ? MavioStrings.clientMoves(paths: paths) : MavioStrings.clientLinks(paths: paths))
                add(" · " + how, secondary)
            } else {
                add(p.error, theme.colors.danger)
            }
            let label = NSTextField.label("", theme.fonts.footnote, color: secondary, lines: 0, selectable: true)
            label.attributedStringValue = text
            return label
        }

        // MARK: Saving

        private func draftsOf(_ paths: [Mavio_Acquisition_V1_DownloadClientPath]) -> [Draft] {
            paths.map { Draft(remote: $0.remotePath, local: $0.localPath, container: $0.container) }
        }

        private var changed: Bool { saved.map { draftsOf($0) != drafts } ?? false }

        private var valid: Bool {
            drafts.allSatisfy { !$0.remote.trimmingCharacters(in: .whitespaces).isEmpty && validLocal($0.local) }
        }

        /// Shows whether saving may be pressed, without laying the form out again.
        private func refresh() {
            saveBar?.update(enabled: changed && valid, busy: saving, notice: nil, failure: saveFailure)
        }

        /// Takes the folders the server tells.
        private func show(_ paths: [Mavio_Acquisition_V1_DownloadClientPath], failure: String) {
            saved = paths
            drafts = draftsOf(paths)
            unreported = failure
        }

        private func load() async {
            var request = Mavio_Acquisition_V1_GetDownloadClientPathsRequest()
            request.pluginID = client.pluginID
            request.clientID = client.id
            do {
                let r = try await model.call(
                    server, Procedure.downloads("GetDownloadClientPaths"), request,
                    as: Mavio_Acquisition_V1_GetDownloadClientPathsResponse.self)
                show(r.paths, failure: r.failure)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func save() {
            endEditing()
            guard valid else { return }
            var request = Mavio_Acquisition_V1_UpdateDownloadClientPathsRequest()
            request.pluginID = client.pluginID
            request.clientID = client.id
            request.paths = drafts.filter { !$0.container && !$0.local.trimmingCharacters(in: .whitespaces).isEmpty }
                .map { d in
                    var p = Mavio_Acquisition_V1_DownloadClientPath()
                    p.remotePath = d.remote.trimmingCharacters(in: .whitespaces)
                    p.localPath = d.local.trimmingCharacters(in: .whitespaces)
                    return p
                }
            saving = true
            saveFailure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let r = try await model.call(
                        server, Procedure.downloads("UpdateDownloadClientPaths"), request,
                        as: Mavio_Acquisition_V1_UpdateDownloadClientPathsResponse.self)
                    guard let self else { return }
                    self.show(r.paths, failure: r.failure)
                    self.saving = false
                    self.render()
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }
    }
#endif
