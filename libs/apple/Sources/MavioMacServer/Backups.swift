#if os(macOS)
    import AppKit
    import MavioMac

    /// Backups: making one of the server's database and settings, and those
    /// it keeps, to download or remove.
    final class BackupsPage: ServerPage {
        private var backups: [Mavio_System_V1_Backup]?
        private var failure: String?
        private var creating = false
        private var createFailure: String?
        private var downloading: String?
        private lazy var form = FormPage(title: MavioStrings.settingsBackups, theme: theme)
        private lazy var createSection = section(nil, footer: MavioStrings.backupsRestoreFooter)
        private lazy var listSection = section(MavioStrings.backupsList)

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsBackups
            form.add(createSection)
            form.add(listSection)
            renderCreate()
            renderList()
            show(form)
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func renderCreate() {
            let make = button(creating ? "…" : MavioStrings.backupsCreateAction, kind: .primary) { [weak self] in
                self?.create()
            }
            make.isEnabled = !creating
            var rows: [NSView] = [
                ControlRow(
                    text(
                        MavioStrings.backupsCreate,
                        creating ? MavioStrings.backupsCreating : MavioStrings.backupsCreateDetail),
                    controls: [make], theme: theme)
            ]
            if let createFailure { rows.append(note(createFailure, failed: true)) }
            createSection.setRows(rows)
            refocus()
        }

        private func renderList() {
            var rows: [NSView] = (backups ?? []).map(row)
            if let failure {
                rows.append(note(failure, failed: true))
            } else if let backups, backups.isEmpty {
                rows.append(note(MavioStrings.backupsNone))
            } else if backups == nil {
                rows.append(padded(LoadingView()))
            }
            listSection.setRows(rows)
            refocus()
        }

        private func row(_ b: Mavio_System_V1_Backup) -> NSView {
            let content = text(
                b.name,
                [
                    b.createTime.time.map(fromNow),
                    b.sizeBytes.formatted(.byteCount(style: .file)),
                    b.version.isEmpty ? nil : MavioStrings.backupsVersion(version: b.version),
                ].compactMap(\.self).joined(separator: " · "))
            // Saving a file where the user chooses is for the desktop; in
            // television mode a backup's row removes it.
            var accessory: NSView?
            if !theme.isTV {
                if downloading == b.name {
                    let spinner = NSProgressIndicator()
                    spinner.style = .spinning
                    spinner.controlSize = .small
                    spinner.startAnimation(nil)
                    accessory = spinner
                } else {
                    let download = MavioButton(icon: "downloads", kind: .plain, theme: theme) { [weak self] in
                        self?.download(b)
                    }
                    download.isEnabled = downloading == nil
                    download.setAccessibilityLabel(MavioStrings.backupsDownload)
                    download.toolTip = MavioStrings.backupsDownload
                    accessory = download
                }
            }
            let row = ContentRow(content, accessory: accessory, chevron: false, theme: theme)
            if theme.isTV { row.onPress = { [weak self] in self?.confirm(b) } }
            row.contextMenu = { [weak self] in
                guard let self else { return nil }
                let menu = NSMenu()
                if !theme.isTV, downloading == nil {
                    menu.addItem(MenuAction(MavioStrings.backupsDownload) { [weak self] in self?.download(b) })
                }
                menu.addItem(MenuAction(MavioStrings.backupsRemove) { [weak self] in self?.confirm(b) })
                return menu
            }
            return row
        }

        private func load() async {
            do {
                backups = try await model.call(
                    server, Procedure.backups("ListBackups"), Mavio_System_V1_ListBackupsRequest(),
                    as: Mavio_System_V1_ListBackupsResponse.self
                ).backups
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            renderList()
        }

        private func create() {
            creating = true
            renderCreate()
            run { [weak self] in
                guard let self else { return }
                do {
                    _ = try await model.call(
                        server, Procedure.backups("CreateBackup"), Mavio_System_V1_CreateBackupRequest(),
                        as: Mavio_System_V1_CreateBackupResponse.self)
                    createFailure = nil
                } catch {
                    createFailure = error.reason
                }
                creating = false
                renderCreate()
                await load()
            }
        }

        private func confirm(_ b: Mavio_System_V1_Backup) {
            confirm(
                MavioStrings.backupsRemoveTitle(name: b.name), message: MavioStrings.backupsRemoveMessage,
                button: MavioStrings.backupsRemove
            ) { [weak self] in self?.remove(b) }
        }

        private func remove(_ b: Mavio_System_V1_Backup) {
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_DeleteBackupRequest()
                request.name = b.name
                do {
                    _ = try await model.call(
                        server, Procedure.backups("DeleteBackup"), request,
                        as: Mavio_System_V1_DeleteBackupResponse.self)
                    await load()
                } catch {
                    createFailure = error.reason
                    renderCreate()
                }
            }
        }

        /// Downloads a backup's file with the session's access token, then
        /// asks where to save it.
        private func download(_ b: Mavio_System_V1_Backup) {
            downloading = b.name
            renderList()
            run { [weak self] in
                guard let self else { return }
                defer {
                    downloading = nil
                    renderList()
                }
                do {
                    let target = try await model.client.serverRequest(
                        serverId: server.id, path: "system/backups/\(b.name)")
                    guard let url = URL(string: target.url) else { return }
                    var request = URLRequest(url: url)
                    request.setValue("Bearer \(target.token)", forHTTPHeaderField: "Authorization")
                    let (file, response) = try await URLSession.shared.download(for: request)
                    if let http = response as? HTTPURLResponse, !(200..<300).contains(http.statusCode) {
                        throw URLError(.badServerResponse)
                    }
                    let named = FileManager.default.temporaryDirectory.appending(path: b.name)
                    try? FileManager.default.removeItem(at: named)
                    try FileManager.default.moveItem(at: file, to: named)
                    save(named)
                } catch is CancellationError {
                } catch {
                    failure = error.reason
                }
            }
        }

        /// Asks where to save a file downloaded, and moves it there.
        private func save(_ file: URL) {
            let panel = NSSavePanel()
            panel.nameFieldStringValue = file.lastPathComponent
            let finish = { (response: NSApplication.ModalResponse) in
                guard response == .OK, let target = panel.url else {
                    try? FileManager.default.removeItem(at: file)
                    return
                }
                do {
                    try? FileManager.default.removeItem(at: target)
                    try FileManager.default.moveItem(at: file, to: target)
                } catch {
                    self.failure = error.reason
                    self.renderList()
                }
            }
            if let window = view.window {
                panel.beginSheetModal(for: window) { response in MainActor.assumeIsolated { finish(response) } }
            } else {
                finish(panel.runModal())
            }
        }
    }
#endif
