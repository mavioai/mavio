#if os(macOS)
    import AppKit
    import MavioMac

    private let page: Int32 = 50

    @MainActor private func stateName(_ s: Mavio_Acquisition_V1_DownloadState) -> String {
        switch s {
        case .downloading: MavioStrings.downloadsDownloading
        case .importBlocked: MavioStrings.downloadsImportBlocked
        case .importPending: MavioStrings.downloadsImportPending
        case .importing: MavioStrings.downloadsImporting
        case .imported: MavioStrings.downloadsImported
        case .failedPending, .failed: MavioStrings.downloadsFailed
        case .ignored: MavioStrings.downloadsIgnored
        default: ""
        }
    }

    @MainActor private func statusName(_ s: Mavio_Acquisition_V1_DownloadStatus) -> String? {
        switch s {
        case .queued: MavioStrings.downloadsQueued
        case .paused: MavioStrings.downloadsPaused
        case .warning: MavioStrings.downloadsWarning
        default: nil
        }
    }

    @MainActor private func historyName(_ type: Mavio_Acquisition_V1_HistoryEventType) -> String {
        switch type {
        case .grabbed: MavioStrings.downloadsGrabbed
        case .imported: MavioStrings.downloadsImported
        case .downloadFailed: MavioStrings.downloadsFailed
        case .fileDeleted: MavioStrings.downloadsFileDeleted
        case .fileRenamed: MavioStrings.downloadsFileRenamed
        case .ignored: MavioStrings.downloadsIgnored
        default: ""
        }
    }

    /// The settings' Downloads (docs/clients.md §6.3.17), as the web UI's:
    /// the tracked downloads with their progress, removed with their
    /// release blocklisted or not; the history, where a grab is marked
    /// failed; the blocklist; and the manual import of a download's files
    /// or of a folder, as Radarr's Activity — each on its tab.
    final class DownloadsPage: FormController {
        /// A tab of the page.
        private enum Tab: Int, CaseIterable {
            case queue, history, blocklist, folderImport

            @MainActor var title: String {
                switch self {
                case .queue: MavioStrings.downloadsQueue
                case .history: MavioStrings.downloadsHistory
                case .blocklist: MavioStrings.downloadsBlocklist
                case .folderImport: MavioStrings.downloadsImport
                }
            }
        }

        /// A list loaded page by page.
        private struct Paged<Element> {
            var items: [Element] = []
            var total: Int32 = 0
            var loaded = false
            var more = false
            var failure: String?
        }

        /// How a download is removed, while its panel is open.
        private struct Removal {
            var fromClient = true
            var blocklist = false
            var search = false
            var removing = false
            var failure: String?
        }

        /// The files a manual import would take from a download or a folder.
        private struct Import {
            var files: [Mavio_Acquisition_V1_ManualImportFile]?
            var failure: String?
            var mode = Mavio_Acquisition_V1_ImportMode.unspecified
            /// The files' state, by their paths.
            var importing: Set<String> = []
            var done: Set<String> = []
            var failures: [String: String] = [:]
        }

        private var tab = Tab.queue
        private lazy var tabs: PillTabs = {
            let tabs = PillTabs(Tab.allCases.map(\.title), selected: 0, theme: theme)
            tabs.setAccessibilityLabel(MavioStrings.settingsDownloads)
            tabs.onChange = { [weak self] i in self?.choose(Tab(rawValue: i) ?? .queue) }
            return tabs
        }()
        private var loading: Task<Void, Never>?

        private var finished = false
        private var queue = Paged<Mavio_Acquisition_V1_QueueItem>()
        private var removals: [String: Removal] = [:]
        /// The downloads whose files are shown to import, by their IDs.
        private var importing: Set<String> = []
        private var imports: [String: Import] = [:]

        private var type = Mavio_Acquisition_V1_HistoryEventType.unspecified
        private var history = Paged<Mavio_Acquisition_V1_HistoryEvent>()
        private var confirming: Set<String> = []
        private var busy: Set<String> = []
        private var rowFailures: [String: String] = [:]

        private var blocklist = Paged<Mavio_Acquisition_V1_BlocklistEntry>()

        private var path = ""
        private var folder = ""

        override func viewDidLoad() {
            title = MavioStrings.settingsDownloads
            message = MavioStrings.downloadsMessage
            super.viewDidLoad()
            onNearEnd = { [weak self] in self?.more() }
            choose(.queue)
        }

        isolated deinit { loading?.cancel() }

        private func choose(_ tab: Tab) {
            self.tab = tab
            loading?.cancel()
            loading = nil
            switch tab {
            case .queue: startQueue()
            case .history: reloadHistory()
            case .blocklist: loadBlocklist(offset: 0)
            case .folderImport: break
            }
            render()
        }

        override func sections() -> [NSView] {
            let head = plain(NSStackView.horizontal([tabs, Spacer()]))
            switch tab {
            case .queue: return [head] + queueSections()
            case .history: return [head] + historySections()
            case .blocklist: return [head] + blocklistSections()
            case .folderImport: return [head] + folderSections()
            }
        }

        /// The rows closing a paged list: empty, failed or loading.
        private func ending<E>(_ list: Paged<E>, empty: String) -> [NSView] {
            var rows: [NSView] = []
            if list.loaded && list.items.isEmpty { rows.append(note(empty)) }
            if let failure = list.failure { rows.append(note(failure, failed: true)) }
            if !list.loaded && list.failure == nil || list.more { rows.append(spinnerRow()) }
            return rows
        }

        /// A footnote of text in a color, wrapping.
        private func line(_ text: String, _ color: NSColor? = nil, selectable: Bool = false) -> NSTextField {
            NSTextField.label(
                text, theme.fonts.footnote, color: color ?? theme.colors.textPrimary, lines: 0, selectable: selectable)
        }

        /// A panel raised inside a row.
        private func panel(_ views: [NSView]) -> NSView {
            let box = Box(fill: theme.colors.surfaceRaised, radius: 8)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            for view in views { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            box.addFilling(stack, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s8))
            return box
        }

        /// A stack of a row's lines, each as wide as it.
        private func lines(_ views: [NSView], spacing: CGFloat = 6) -> NSStackView {
            let stack = NSStackView.vertical(views, spacing: spacing)
            for view in views where !(view is MavioButton) {
                view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            return stack
        }

        // MARK: Queue

        private func queueSections() -> [NSView] {
            var rows: [NSView] = [
                toggleRow(MavioStrings.downloadsShowFinished, isOn: finished) { [weak self] on in
                    self?.finished = on
                    self?.startQueue()
                }
            ]
            rows += queue.items.map(queueRow)
            rows += ending(queue, empty: MavioStrings.downloadsQueueEmpty)
            return [card(footer: queue.loaded ? MavioStrings.downloadsCount(count: Int(queue.total)) : nil, rows)]
        }

        /// A tracked download: its item, state, release, progress and
        /// messages, and removing or importing it.
        private func queueRow(_ i: Mavio_Acquisition_V1_QueueItem) -> NSView {
            var top: [NSView] = []
            let name = i.itemID.isEmpty || !i.itemName.isEmpty ? i.itemName : MavioStrings.downloadsItem
            if i.itemID.isEmpty {
                top.append(NSTextField.label(name, theme.fonts.headline, color: theme.colors.textPrimary))
            } else {
                top.append(itemLink(name, itemId: i.itemID))
            }
            top.append(
                Badge(
                    stateName(i.state), failed: [.importBlocked, .failed, .failedPending].contains(i.state),
                    theme: theme))
            if let status = statusName(i.status) { top.append(line(status, theme.colors.textSecondary)) }
            top.append(Spacer())
            var views: [NSView] = [
                NSStackView.horizontal(top, spacing: MavioSpacing.s8),
                line(i.title, selectable: true),
                line(detail(i), theme.colors.textSecondary),
            ]
            if i.size > 0, i.state == .downloading {
                let bar = NSProgressIndicator()
                bar.style = .bar
                bar.isIndeterminate = false
                bar.minValue = 0
                bar.maxValue = 1
                bar.doubleValue = max(0, min(1, 1 - Double(i.remainingSize) / Double(i.size)))
                bar.setAccessibilityLabel(i.title)
                views.append(bar)
            }
            views += i.messages.map { line($0, theme.colors.danger) }
            let id = i.id
            if ![.imported, .failed, .ignored].contains(i.state) {
                var buttons: [NSView] = []
                if [.importBlocked, .importPending].contains(i.state) {
                    buttons.append(
                        MavioButton(MavioStrings.downloadsManualImport, kind: .secondary, theme: theme) { [weak self] in
                            self?.toggleImport(id)
                        })
                }
                buttons.append(
                    MavioButton(MavioStrings.downloadsRemove, kind: .destructive, theme: theme) { [weak self] in
                        guard let self else { return }
                        if self.removals[id] == nil {
                            self.importing.remove(id)
                            self.removals[id] = Removal()
                        } else {
                            self.removals[id] = nil
                        }
                        self.render()
                    })
                buttons.append(Spacer())
                views.append(NSStackView.horizontal(buttons, spacing: MavioSpacing.s8))
            }
            if let removal = removals[id] {
                views.append(removePanel(id, removal))
            } else if importing.contains(id) {
                views.append(importPanel(key: "q:\(id)", id: id, folder: nil))
            }
            return row(lines(views))
        }

        private func detail(_ i: Mavio_Acquisition_V1_QueueItem) -> String {
            let left: String? =
                i.hasRemainingTime && i.state == .downloading
                ? MavioStrings.downloadsLeft(
                    time: lengthOf(Double(i.remainingTime.seconds) + Double(i.remainingTime.nanos) / 1e9))
                : nil
            return dotted([
                i.quality,
                i.languages.joined(separator: ", "),
                i.size > 0 ? acquisitionSize(i.size) : nil,
                i.indexerName,
                left,
                i.hasGrabTime ? i.grabTime.time.map(fromNow) : nil,
            ])
        }

        /// How a download is removed: from its client or not, its release
        /// blocklisted and searched again or not.
        private func removePanel(_ id: String, _ r: Removal) -> NSView {
            var views: [NSView] = [
                Check(MavioStrings.downloadsRemoveFromClient, isOn: r.fromClient, theme: theme) { [weak self] on in
                    self?.removals[id]?.fromClient = on
                },
                Check(MavioStrings.downloadsBlocklistRelease, isOn: r.blocklist, theme: theme) { [weak self] on in
                    self?.removals[id]?.blocklist = on
                    if !on { self?.removals[id]?.search = false }
                    self?.render()
                },
            ]
            if r.blocklist {
                views.append(
                    Check(MavioStrings.downloadsSearchAgain, isOn: r.search, theme: theme) { [weak self] on in
                        self?.removals[id]?.search = on
                    })
            }
            let remove = MavioButton(MavioStrings.downloadsRemove, kind: .destructive, theme: theme) { [weak self] in
                self?.remove(id)
            }
            remove.isEnabled = !r.removing
            var buttons: [NSView] = [
                MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                    self?.removals[id] = nil
                    self?.render()
                },
                remove,
            ]
            if let failure = r.failure { buttons.append(line(failure, theme.colors.danger)) }
            buttons.append(Spacer())
            views.append(NSStackView.horizontal(buttons, spacing: MavioSpacing.s8))
            return panel(views)
        }

        private func toggleImport(_ id: String) {
            removals[id] = nil
            if importing.contains(id) {
                importing.remove(id)
            } else {
                importing.insert(id)
                loadImport(key: "q:\(id)", id: id, folder: nil)
            }
            render()
        }

        private func startQueue() {
            loading?.cancel()
            queue = Paged()
            render()
            loading = Task { [weak self] in
                while !Task.isCancelled {
                    await self?.loadQueue(offset: 0)
                    try? await Task.sleep(for: .seconds(10))
                }
            }
        }

        /// Loads a page, or from the first the pages shown.
        private func loadQueue(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListQueueRequest()
            request.includeFinished = finished
            request.limit = offset == 0 ? min(1000, max(page, Int32(queue.items.count))) : page
            request.offset = offset
            do {
                let response = try await model.client.call(
                    server.id, Procedure.downloads("ListQueue"), request,
                    as: Mavio_Acquisition_V1_ListQueueResponse.self)
                guard request.includeFinished == finished, tab == .queue else { return }
                queue.items = offset == 0 ? response.items : queue.items + response.items
                queue.total = response.total
                queue.loaded = true
                queue.failure = nil
            } catch is CancellationError {
                return
            } catch {
                queue.failure = error.reason
            }
            queue.more = false
            render()
        }

        private func remove(_ id: String) {
            guard let r = removals[id] else { return }
            removals[id]?.removing = true
            removals[id]?.failure = nil
            render()
            var request = Mavio_Acquisition_V1_RemoveFromQueueRequest()
            request.id = id
            request.removeFromClient = r.fromClient
            request.blocklist = r.blocklist
            request.search = r.search
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("RemoveFromQueue"), request,
                        as: Mavio_Acquisition_V1_RemoveFromQueueResponse.self)
                    self?.removals[id] = nil
                    await self?.loadQueue(offset: 0)
                } catch {
                    self?.removals[id]?.removing = false
                    self?.removals[id]?.failure = error.reason
                    self?.render()
                }
            }
        }

        // MARK: Manual import

        /// The files a manual import would take from a download or a folder,
        /// each imported on its own.
        private func importPanel(key: String, id: String?, folder: String?) -> NSView {
            let state = imports[key] ?? Import()
            let modes: [(String, Mavio_Acquisition_V1_ImportMode)] = [
                (MavioStrings.downloadsModeAuto, .unspecified), (MavioStrings.downloadsModeMove, .move),
                (MavioStrings.downloadsModeCopy, .copy),
            ]
            let heading = NSTextField.label(
                MavioStrings.downloadsFiles, theme.fonts.callout, color: theme.colors.textPrimary)
            heading.font = NSFont.systemFont(ofSize: theme.fonts.callout.size, weight: .semibold)
            let mode = PopUp(
                modes.map(\.0), selected: modes.firstIndex { $0.1 == state.mode } ?? 0,
                label: MavioStrings.downloadsMode
            ) { [weak self] i in self?.imports[key, default: Import()].mode = modes[i].1 }
            var views: [NSView] = [NSStackView.horizontal([heading, Spacer(), mode], spacing: MavioSpacing.s8)]
            if state.files == nil && state.failure == nil { views.append(spinnerRow()) }
            if let failure = state.failure { views.append(line(failure, theme.colors.danger)) }
            if let files = state.files, files.isEmpty {
                views.append(label(MavioStrings.downloadsNoFiles, theme.colors.textSecondary))
            }
            for (index, f) in (state.files ?? []).enumerated() {
                if index > 0 {
                    let divider = NSBox()
                    divider.boxType = .separator
                    views.append(divider)
                }
                var text: [NSView] = [
                    line(f.relativePath.isEmpty ? f.path : f.relativePath, selectable: true),
                    line(
                        dotted([
                            acquisitionSize(f.size), f.quality, f.languages.joined(separator: ", "), f.releaseGroup,
                        ]),
                        theme.colors.textSecondary),
                ]
                text += f.rejections.map { line($0, theme.colors.danger) }
                if let failure = state.failures[f.path] { text.append(line(failure, theme.colors.danger)) }
                let stack = lines(text, spacing: 2)
                stack.setContentHuggingPriority(.init(1), for: .horizontal)
                let done = state.done.contains(f.path)
                let path = f.path
                let button = MavioButton(
                    done ? MavioStrings.downloadsImportedFile : MavioStrings.downloadsImportFile, kind: .secondary,
                    theme: theme
                ) { [weak self] in self?.manualImport(key: key, id: id, folder: folder, path: path) }
                button.isEnabled = !f.movieID.isEmpty && !state.importing.contains(f.path) && !done
                views.append(NSStackView.horizontal([stack, button], spacing: MavioSpacing.s12))
            }
            return panel(views)
        }

        private func loadImport(key: String, id: String?, folder: String?) {
            imports[key] = Import()
            var request = Mavio_Acquisition_V1_ListManualImportRequest()
            if let id { request.id = id }
            if let folder { request.folder = folder }
            run { [weak self, model, server] in
                do {
                    let files = try await model.client.call(
                        server.id, Procedure.downloads("ListManualImport"), request,
                        as: Mavio_Acquisition_V1_ListManualImportResponse.self
                    ).files
                    self?.imports[key]?.files = files
                    self?.imports[key]?.failure = nil
                } catch {
                    self?.imports[key]?.failure = error.reason
                }
                self?.render()
            }
        }

        private func manualImport(key: String, id: String?, folder: String?, path: String) {
            guard imports[key] != nil else { return }
            imports[key]?.importing.insert(path)
            imports[key]?.failures[path] = nil
            render()
            var request = Mavio_Acquisition_V1_ManualImportRequest()
            if let id { request.id = id }
            if let folder { request.folder = folder }
            request.path = path
            request.mode = imports[key]?.mode ?? .unspecified
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("ManualImport"), request,
                        as: Mavio_Acquisition_V1_ManualImportResponse.self)
                    self?.imports[key]?.done.insert(path)
                    if id != nil { await self?.loadQueue(offset: 0) }
                } catch {
                    self?.imports[key]?.failures[path] = error.reason
                }
                self?.imports[key]?.importing.remove(path)
                self?.render()
            }
        }

        // MARK: Folder import

        /// A folder's files, imported for the wanted movies their names match.
        private func folderSections() -> [NSView] {
            let input = TextInput(path, placeholder: MavioStrings.downloadsFolder, monospaced: true)
            input.setContentHuggingPriority(.init(1), for: .horizontal)
            let scan = MavioButton(MavioStrings.downloadsScan, kind: .primary, theme: theme) { [weak self] in
                self?.scan()
            }
            scan.isEnabled = !path.trimmingCharacters(in: .whitespaces).isEmpty
            input.onChange = { [weak self, weak scan] text in
                self?.path = text
                scan?.isEnabled = !text.trimmingCharacters(in: .whitespaces).isEmpty
            }
            input.onSubmit = { [weak self] in self?.scan() }
            var rows: [NSView] = [row(NSStackView.horizontal([input, scan], spacing: MavioSpacing.s12))]
            if !folder.isEmpty { rows.append(row(importPanel(key: "f:\(folder)", id: nil, folder: folder))) }
            return [card(footer: MavioStrings.downloadsImportFooter, rows)]
        }

        private func scan() {
            let trimmed = path.trimmingCharacters(in: .whitespaces)
            guard !trimmed.isEmpty else { return }
            endEditing()
            folder = trimmed
            loadImport(key: "f:\(trimmed)", id: nil, folder: trimmed)
            render()
        }

        // MARK: History

        private static let types: [Mavio_Acquisition_V1_HistoryEventType] = [
            .grabbed, .imported, .downloadFailed, .fileDeleted, .fileRenamed, .ignored,
        ]

        /// What happened in acquiring, newest first, of one type or all.
        private func historySections() -> [NSView] {
            let types: [Mavio_Acquisition_V1_HistoryEventType] = [.unspecified] + Self.types
            let picker = PopUp(
                types.map { $0 == .unspecified ? MavioStrings.downloadsAllEvents : historyName($0) },
                selected: types.firstIndex(of: type) ?? 0, label: MavioStrings.downloadsEventType
            ) { [weak self] i in
                self?.type = types[i]
                self?.reloadHistory()
            }
            var rows: [NSView] = history.items.map(historyRow)
            rows += ending(history, empty: MavioStrings.downloadsHistoryEmpty)
            return [
                plain(NSStackView.horizontal([picker, Spacer()])),
                card(footer: history.loaded ? MavioStrings.downloadsEventCount(count: Int(history.total)) : nil, rows),
            ]
        }

        /// An event of the history: its type, release and data, and marking
        /// a grab failed.
        private func historyRow(_ e: Mavio_Acquisition_V1_HistoryEvent) -> NSView {
            var views: [NSView] = [
                NSStackView.horizontal(
                    [
                        Badge(historyName(e.type), failed: e.type == .downloadFailed, theme: theme),
                        line(
                            dotted([
                                e.quality, e.languages.joined(separator: ", "),
                                e.hasTime ? e.time.time.map(fromNow) : nil,
                            ]), theme.colors.textSecondary),
                        Spacer(),
                    ], spacing: MavioSpacing.s8),
                line(e.sourceTitle, selectable: true),
            ]
            if !e.itemID.isEmpty { views.append(itemLink(MavioStrings.downloadsItem, itemId: e.itemID)) }
            if !e.data.isEmpty {
                let data = NSTextField.label(
                    e.data.sorted { $0.key.localizedStandardCompare($1.key) == .orderedAscending }
                        .map { "\($0.key)=\($0.value)" }
                        .joined(separator: "   "),
                    theme.fonts.caption, color: theme.colors.textSecondary, lines: 0, selectable: true)
                data.font = .monospacedSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
                views.append(data)
            }
            if e.type == .grabbed {
                let id = e.id
                var buttons: [NSView] = []
                if confirming.contains(id) {
                    let mark = MavioButton(MavioStrings.downloadsMarkFailed, kind: .destructive, theme: theme) {
                        [weak self] in self?.markFailed(id)
                    }
                    mark.isEnabled = !busy.contains(id)
                    buttons += [
                        NSTextField.label(
                            MavioStrings.downloadsMarkFailedTitle, theme.fonts.callout, color: theme.colors.textPrimary),
                        MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                            self?.confirming.remove(id)
                            self?.render()
                        },
                        mark,
                    ]
                } else {
                    buttons.append(
                        MavioButton(MavioStrings.downloadsMarkFailed, kind: .destructive, theme: theme) { [weak self] in
                            self?.confirming.insert(id)
                            self?.render()
                        })
                }
                if let failure = rowFailures[id] { buttons.append(line(failure, theme.colors.danger)) }
                buttons.append(Spacer())
                views.append(NSStackView.horizontal(buttons, spacing: MavioSpacing.s8))
            }
            return row(lines(views, spacing: MavioSpacing.s4))
        }

        private func reloadHistory() {
            loading?.cancel()
            history = Paged()
            render()
            loading = Task { [weak self] in await self?.loadHistory(offset: 0) }
        }

        private func loadHistory(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListHistoryRequest()
            request.types = type == .unspecified ? [] : [type]
            request.limit = offset == 0 ? min(1000, max(page, Int32(history.items.count))) : page
            request.offset = offset
            let shown = type
            do {
                let response = try await model.client.call(
                    server.id, Procedure.downloads("ListHistory"), request,
                    as: Mavio_Acquisition_V1_ListHistoryResponse.self)
                guard shown == type, tab == .history else { return }
                history.items = offset == 0 ? response.events : history.items + response.events
                history.total = response.total
                history.loaded = true
                history.failure = nil
            } catch is CancellationError {
                return
            } catch {
                history.failure = error.reason
            }
            history.more = false
            render()
        }

        private func markFailed(_ id: String) {
            busy.insert(id)
            rowFailures[id] = nil
            render()
            var request = Mavio_Acquisition_V1_MarkFailedRequest()
            request.historyEventID = id
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("MarkFailed"), request,
                        as: Mavio_Acquisition_V1_MarkFailedResponse.self)
                    self?.confirming.remove(id)
                    self?.busy.remove(id)
                    await self?.loadHistory(offset: 0)
                } catch {
                    self?.busy.remove(id)
                    self?.rowFailures[id] = error.reason
                    self?.render()
                }
            }
        }

        // MARK: Blocklist

        /// The releases not to grab again, newest first.
        private func blocklistSections() -> [NSView] {
            var rows: [NSView] = blocklist.items.map(blocklistRow)
            rows += ending(blocklist, empty: MavioStrings.downloadsBlocklistEmpty)
            return [card(footer: MavioStrings.downloadsBlocklistFooter, rows)]
        }

        /// A blocklisted release: what it is, why it was blocklisted, and
        /// unblocking it.
        private func blocklistRow(_ e: Mavio_Acquisition_V1_BlocklistEntry) -> NSView {
            var text: [NSView] = [
                line(e.sourceTitle, selectable: true),
                line(
                    dotted([
                        e.quality, e.languages.joined(separator: ", "), e.size > 0 ? acquisitionSize(e.size) : nil,
                        e.indexerName, e.hasTime ? e.time.time.map(fromNow) : nil,
                    ]), theme.colors.textSecondary),
            ]
            if !e.message.isEmpty { text.append(line(e.message, theme.colors.textSecondary)) }
            if !e.itemID.isEmpty { text.append(itemLink(MavioStrings.downloadsItem, itemId: e.itemID)) }
            if let failure = rowFailures[e.id] { text.append(line(failure, theme.colors.danger)) }
            let stack = lines(text, spacing: 2)
            stack.setContentHuggingPriority(.init(1), for: .horizontal)
            let id = e.id
            let unblock = MavioButton(MavioStrings.downloadsUnblock, kind: .destructive, theme: theme) { [weak self] in
                self?.unblock(id)
            }
            unblock.isEnabled = !busy.contains(id)
            return row(NSStackView.horizontal([stack, unblock], spacing: MavioSpacing.s12))
        }

        private func loadBlocklist(offset: Int32) {
            if offset == 0 && !blocklist.loaded { blocklist = Paged() }
            var request = Mavio_Acquisition_V1_ListBlocklistRequest()
            request.limit = offset == 0 ? min(1000, max(page, Int32(blocklist.items.count))) : page
            request.offset = offset
            loading?.cancel()
            loading = Task { [weak self, model, server] in
                do {
                    let response = try await model.client.call(
                        server.id, Procedure.downloads("ListBlocklist"), request,
                        as: Mavio_Acquisition_V1_ListBlocklistResponse.self)
                    guard let self, !Task.isCancelled, self.tab == .blocklist else { return }
                    self.blocklist.items = offset == 0 ? response.entries : self.blocklist.items + response.entries
                    self.blocklist.total = response.total
                    self.blocklist.loaded = true
                    self.blocklist.failure = nil
                } catch is CancellationError {
                    return
                } catch {
                    self?.blocklist.failure = error.reason
                }
                self?.blocklist.more = false
                self?.render()
            }
        }

        private func unblock(_ id: String) {
            busy.insert(id)
            rowFailures[id] = nil
            render()
            var request = Mavio_Acquisition_V1_DeleteBlocklistRequest()
            request.ids = [id]
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("DeleteBlocklist"), request,
                        as: Mavio_Acquisition_V1_DeleteBlocklistResponse.self)
                    self?.busy.remove(id)
                    self?.loadBlocklist(offset: 0)
                } catch {
                    self?.busy.remove(id)
                    self?.rowFailures[id] = error.reason
                    self?.render()
                }
            }
        }

        // MARK: Paging

        /// Loads the next page of the tab's list when its end shows.
        private func more() {
            switch tab {
            case .queue:
                guard queue.loaded, !queue.more, queue.items.count < queue.total else { return }
                queue.more = true
                let offset = Int32(queue.items.count)
                run { [weak self] in await self?.loadQueue(offset: offset) }
            case .history:
                guard history.loaded, !history.more, history.items.count < history.total else { return }
                history.more = true
                let offset = Int32(history.items.count)
                run { [weak self] in await self?.loadHistory(offset: offset) }
            case .blocklist:
                guard blocklist.loaded, !blocklist.more, blocklist.items.count < blocklist.total else { return }
                blocklist.more = true
                loadBlocklist(offset: Int32(blocklist.items.count))
            case .folderImport:
                return
            }
            render()
        }
    }
#endif
