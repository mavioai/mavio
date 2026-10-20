#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    // The settings' Downloads (docs/clients.md §6.3.17), as the web UI's: the
    // tracked downloads with their progress, removed with their release
    // blocklisted or not; the history, where a grab is marked failed; the
    // blocklist; and the manual import of a download's files or of a folder,
    // as Radarr's Activity.

    /// Downloads: the queue, the history, the blocklist and the import of a folder, each on its tab.
    struct DownloadsAdminView: View {
        let server: ServerRecord
        @Environment(\.theme) private var theme
        @State private var tab = Tab.queue

        /// A tab of the page.
        enum Tab: CaseIterable {
            case queue, history, blocklist, folderImport

            var title: String {
                switch self {
                case .queue: MavioStrings.downloadsQueue
                case .history: MavioStrings.downloadsHistory
                case .blocklist: MavioStrings.downloadsBlocklist
                case .folderImport: MavioStrings.downloadsImport
                }
            }
        }

        var body: some View {
            SettingsList(message: MavioStrings.downloadsMessage) {
                Section {
                } header: {
                    Picker(MavioStrings.settingsDownloads, selection: $tab) {
                        ForEach(Tab.allCases, id: \.self) { Text($0.title).tag($0) }
                    }
                    .pickerStyle(.segmented)
                    .labelsHidden()
                    .fixedSize()
                    .type(theme.type.body)
                }
                switch tab {
                case .queue: AcquisitionDownloadsQueue(server: server)
                case .history: AcquisitionDownloadsHistory(server: server)
                case .blocklist: AcquisitionDownloadsBlocklist(server: server)
                case .folderImport: AcquisitionDownloadsFolderImport(server: server)
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsDownloads)
            .inlineTitle()
        }
    }

    private let page: Int32 = 50

    /// A file's size in megabytes, or gigabytes from a thousand megabytes.
    private func sizeOf(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let measurement =
            n >= 1e9
            ? Measurement(value: n / 1e9, unit: UnitInformationStorage.gigabytes)
            : Measurement(value: n / 1e6, unit: UnitInformationStorage.megabytes)
        return measurement.formatted(
            .measurement(
                width: .abbreviated, usage: .asProvided,
                numberFormatStyle: .number.precision(.fractionLength(0...(n >= 1e9 ? 2 : 1)))))
    }

    /// Texts joined by dots, the empty ones left out.
    private func dotted(_ parts: [String?]) -> String {
        parts.compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
    }

    private func stateName(_ s: Mavio_Acquisition_V1_DownloadState) -> String {
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

    private func statusName(_ s: Mavio_Acquisition_V1_DownloadStatus) -> String? {
        switch s {
        case .queued: MavioStrings.downloadsQueued
        case .paused: MavioStrings.downloadsPaused
        case .warning: MavioStrings.downloadsWarning
        default: nil
        }
    }

    private func historyName(_ type: Mavio_Acquisition_V1_HistoryEventType) -> String {
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

    /// A state's or an event's badge: red when it failed.
    private struct Badge: View {
        let text: String
        let failed: Bool
        @Environment(\.theme) private var theme

        var body: some View {
            Text(text)
                .type(theme.type.caption)
                .fontWeight(.semibold)
                .foregroundStyle(failed ? Color.white : theme.colors.textPrimary)
                .padding(.horizontal, MavioSpacing.s8)
                .padding(.vertical, 2)
                .background(failed ? theme.colors.danger : theme.colors.fill, in: .capsule)
        }
    }

    /// A row saying a list is empty, or why it could not be read.
    private struct Note: View {
        let text: String
        var failed = false
        @Environment(\.theme) private var theme

        var body: some View {
            Text(text)
                .type(theme.type.callout)
                .foregroundStyle(failed ? theme.colors.danger : theme.colors.textSecondary)
                .textSelection(.enabled)
        }
    }

    /// A footnote, in a color.
    private struct Line: View {
        let text: String
        var color: Color?
        @Environment(\.theme) private var theme

        var body: some View {
            Text(text)
                .type(theme.type.footnote)
                .foregroundStyle(color ?? theme.colors.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    // MARK: Queue

    /// The tracked downloads, most recently grabbed first, read again every ten seconds.
    struct AcquisitionDownloadsQueue: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @State private var finished = false
        @State private var items: [Mavio_Acquisition_V1_QueueItem] = []
        @State private var total: Int32 = 0
        @State private var loaded = false
        @State private var failure: String?

        var body: some View {
            Section {
                // The task is on the switch, a row always there: a section's modifiers are its rows'.
                Toggle(isOn: $finished) { Text(MavioStrings.downloadsShowFinished) }
                    .task(id: finished) {
                        loaded = false
                        items = []
                        while !Task.isCancelled {
                            await load(offset: 0)
                            try? await Task.sleep(for: .seconds(10))
                        }
                    }
                ForEach(items, id: \.id) { item in
                    QueueRow(server: server, item: item, changed: { Task { await load(offset: 0) } })
                        .onAppear {
                            if item.id == items.last?.id, items.count < total {
                                Task { await load(offset: Int32(items.count)) }
                            }
                        }
                }
                if loaded && items.isEmpty { Note(text: MavioStrings.downloadsQueueEmpty) }
                if let failure { Note(text: failure, failed: true) }
            } footer: {
                if loaded { Text(MavioStrings.downloadsCount(count: Int(total))) }
            }
        }

        /// Loads a page, or from the first the pages shown.
        private func load(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListQueueRequest()
            request.includeFinished = finished
            request.limit = offset == 0 ? min(1000, max(page, Int32(items.count))) : page
            request.offset = offset
            do {
                let response = try await model.client.call(
                    server.id, Procedure.downloads("ListQueue"), request,
                    as: Mavio_Acquisition_V1_ListQueueResponse.self)
                guard request.includeFinished == finished else { return }
                items = offset == 0 ? response.items : items + response.items
                total = response.total
                loaded = true
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }

    /// A tracked download: its item, state, release, progress and messages, and removing or importing it.
    private struct QueueRow: View {
        let server: ServerRecord
        let item: Mavio_Acquisition_V1_QueueItem
        let changed: () -> Void
        @Environment(\.theme) private var theme
        @State private var panel: Panel?

        private enum Panel { case remove, manualImport }

        var body: some View {
            let i = item
            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                    ItemLink(server: server, itemId: i.itemID) {
                        Text(i.itemID.isEmpty || !i.itemName.isEmpty ? i.itemName : MavioStrings.downloadsItem)
                            .type(theme.type.headline)
                    }
                    Badge(
                        text: stateName(i.state), failed: [.importBlocked, .failed, .failedPending].contains(i.state))
                    if let status = statusName(i.status) {
                        Line(text: status, color: theme.colors.textSecondary)
                    }
                }
                Line(text: i.title).textSelection(.enabled)
                Line(text: detail, color: theme.colors.textSecondary)
                if let progress {
                    ProgressView(value: progress).accessibilityLabel(i.title)
                }
                ForEach(Array(i.messages.enumerated()), id: \.offset) {
                    Line(text: $0.element, color: theme.colors.danger)
                }
                if !finished {
                    HStack(spacing: MavioSpacing.s8) {
                        if importable {
                            Button(MavioStrings.downloadsManualImport) {
                                panel = panel == .manualImport ? nil : .manualImport
                            }
                            .buttonStyle(.bordered)
                        }
                        Button(MavioStrings.downloadsRemove, role: .destructive) {
                            panel = panel == .remove ? nil : .remove
                        }
                    }
                    .padding(.top, MavioSpacing.s4)
                }
                switch panel {
                case .remove:
                    RemovePanel(server: server, item: i) { removed in
                        panel = nil
                        if removed { changed() }
                    }
                case .manualImport:
                    ImportFiles(server: server, id: i.id, folder: nil, changed: changed)
                case nil: EmptyView()
                }
            }
            .padding(.vertical, MavioSpacing.s4)
        }

        private var size: Int64 { item.size }

        private var progress: Double? {
            guard size > 0, item.state == .downloading else { return nil }
            return max(0, min(1, 1 - Double(item.remainingSize) / Double(size)))
        }

        private var finished: Bool { [.imported, .failed, .ignored].contains(item.state) }

        private var importable: Bool { [.importBlocked, .importPending].contains(item.state) }

        private var detail: String {
            let i = item
            let left: String? =
                i.hasRemainingTime && i.state == .downloading
                ? MavioStrings.downloadsLeft(
                    time: lengthOf(Double(i.remainingTime.seconds) + Double(i.remainingTime.nanos) / 1e9))
                : nil
            return dotted([
                i.quality,
                i.languages.joined(separator: ", "),
                size > 0 ? sizeOf(size) : nil,
                i.indexerName,
                left,
                i.hasGrabTime ? i.grabTime.time.map(fromNow) : nil,
            ])
        }
    }

    /// How a download is removed: from its client or not, its release blocklisted and searched again or not.
    private struct RemovePanel: View {
        let server: ServerRecord
        let item: Mavio_Acquisition_V1_QueueItem
        let done: (Bool) -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var fromClient = true
        @State private var blocklist = false
        @State private var search = false
        @State private var removing = false
        @State private var failure: String?

        var body: some View {
            VStack(alignment: .leading, spacing: 0) {
                Toggle(isOn: $fromClient) { Text(MavioStrings.downloadsRemoveFromClient) }
                    .frame(minHeight: 36)
                Toggle(isOn: $blocklist) { Text(MavioStrings.downloadsBlocklistRelease) }
                    .frame(minHeight: 36)
                    .onChange(of: blocklist) { if !blocklist { search = false } }
                if blocklist {
                    Toggle(isOn: $search) { Text(MavioStrings.downloadsSearchAgain) }
                        .frame(minHeight: 36)
                }
                HStack(spacing: MavioSpacing.s8) {
                    Button(MavioStrings.actionCancel) { done(false) }
                    Button(MavioStrings.downloadsRemove, role: .destructive) { remove() }
                        .buttonStyle(.bordered)
                        .disabled(removing)
                    if let failure { Line(text: failure, color: theme.colors.danger) }
                }
                .padding(.top, MavioSpacing.s4)
            }
            .type(theme.type.callout)
            .padding(.horizontal, MavioSpacing.s12)
            .padding(.vertical, MavioSpacing.s8)
            .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: 8))
        }

        private func remove() {
            removing = true
            failure = nil
            var request = Mavio_Acquisition_V1_RemoveFromQueueRequest()
            request.id = item.id
            request.removeFromClient = fromClient
            request.blocklist = blocklist
            request.search = search
            Task {
                defer { removing = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("RemoveFromQueue"), request,
                        as: Mavio_Acquisition_V1_RemoveFromQueueResponse.self)
                    done(true)
                } catch {
                    failure = error.reason
                }
            }
        }
    }

    // MARK: Manual import

    /// The files a manual import would take from a download or a folder, each imported on its own.
    private struct ImportFiles: View {
        let server: ServerRecord
        let id: String?
        let folder: String?
        let changed: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var files: [Mavio_Acquisition_V1_ManualImportFile]?
        @State private var failure: String?
        @State private var mode = Mavio_Acquisition_V1_ImportMode.unspecified

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                HStack(spacing: MavioSpacing.s8) {
                    Text(MavioStrings.downloadsFiles).type(theme.type.callout).fontWeight(.semibold)
                    Spacer(minLength: 0)
                    Picker(MavioStrings.downloadsMode, selection: $mode) {
                        Text(MavioStrings.downloadsModeAuto).tag(Mavio_Acquisition_V1_ImportMode.unspecified)
                        Text(MavioStrings.downloadsModeMove).tag(Mavio_Acquisition_V1_ImportMode.move)
                        Text(MavioStrings.downloadsModeCopy).tag(Mavio_Acquisition_V1_ImportMode.copy)
                    }
                    .pickerStyle(.menu)
                    .labelsHidden()
                    .fixedSize()
                    .accessibilityLabel(MavioStrings.downloadsMode)
                }
                if files == nil && failure == nil { ProgressView().controlSize(.small) }
                if let failure { Line(text: failure, color: theme.colors.danger) }
                if let files, files.isEmpty { Note(text: MavioStrings.downloadsNoFiles) }
                ForEach(Array((files ?? []).enumerated()), id: \.element.path) { index, file in
                    if index > 0 { Divider() }
                    ImportFile(server: server, id: id, folder: folder, file: file, mode: mode, changed: changed)
                }
            }
            .padding(.horizontal, MavioSpacing.s12)
            .padding(.vertical, MavioSpacing.s8)
            .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: 8))
            .task { await load() }
        }

        private func load() async {
            var request = Mavio_Acquisition_V1_ListManualImportRequest()
            if let id { request.id = id }
            if let folder { request.folder = folder }
            do {
                files = try await model.client.call(
                    server.id, Procedure.downloads("ListManualImport"), request,
                    as: Mavio_Acquisition_V1_ListManualImportResponse.self
                ).files
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }

    /// A file a manual import would take: its path, what it is, why it would be rejected, and importing it.
    private struct ImportFile: View {
        let server: ServerRecord
        let id: String?
        let folder: String?
        let file: Mavio_Acquisition_V1_ManualImportFile
        let mode: Mavio_Acquisition_V1_ImportMode
        let changed: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var importing = false
        @State private var done = false
        @State private var failure: String?

        var body: some View {
            let f = file
            HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    Line(text: f.relativePath.isEmpty ? f.path : f.relativePath).textSelection(.enabled)
                    Line(
                        text: dotted([sizeOf(f.size), f.quality, f.languages.joined(separator: ", "), f.releaseGroup]),
                        color: theme.colors.textSecondary)
                    ForEach(Array(f.rejections.enumerated()), id: \.offset) {
                        Line(text: $0.element, color: theme.colors.danger)
                    }
                    if let failure { Line(text: failure, color: theme.colors.danger) }
                }
                Spacer(minLength: 0)
                Button(done ? MavioStrings.downloadsImportedFile : MavioStrings.downloadsImportFile) { manualImport() }
                    .buttonStyle(.bordered)
                    .disabled(f.movieID.isEmpty || importing || done)
            }
        }

        private func manualImport() {
            importing = true
            failure = nil
            var request = Mavio_Acquisition_V1_ManualImportRequest()
            if let id { request.id = id }
            if let folder { request.folder = folder }
            request.path = file.path
            request.mode = mode
            Task {
                defer { importing = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("ManualImport"), request,
                        as: Mavio_Acquisition_V1_ManualImportResponse.self)
                    done = true
                    changed()
                } catch {
                    failure = error.reason
                }
            }
        }
    }

    /// A folder's files, imported for the wanted movies their names match.
    struct AcquisitionDownloadsFolderImport: View {
        let server: ServerRecord
        @Environment(\.theme) private var theme
        @State private var path = ""
        @State private var folder = ""

        var body: some View {
            Section {
                HStack(spacing: MavioSpacing.s12) {
                    TextField(MavioStrings.downloadsFolder, text: $path, prompt: Text(MavioStrings.downloadsFolder))
                        .fieldLabelHidden()
                        .font(.body.monospaced())
                        .onSubmit(scan)
                    Button(MavioStrings.downloadsScan, action: scan)
                        .buttonStyle(.borderedProminent)
                        .disabled(path.trimmingCharacters(in: .whitespaces).isEmpty)
                }
                if !folder.isEmpty {
                    ImportFiles(server: server, id: nil, folder: folder, changed: {})
                        .id(folder)
                }
            } footer: {
                Text(MavioStrings.downloadsImportFooter)
            }
        }

        private func scan() {
            let trimmed = path.trimmingCharacters(in: .whitespaces)
            if !trimmed.isEmpty { folder = trimmed }
        }
    }

    // MARK: History

    /// What happened in acquiring, newest first, of one type or all.
    struct AcquisitionDownloadsHistory: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var type = Mavio_Acquisition_V1_HistoryEventType.unspecified
        @State private var events: [Mavio_Acquisition_V1_HistoryEvent] = []
        @State private var total: Int32 = 0
        @State private var loaded = false
        @State private var failure: String?

        private static let types: [Mavio_Acquisition_V1_HistoryEventType] = [
            .grabbed, .imported, .downloadFailed, .fileDeleted, .fileRenamed, .ignored,
        ]

        var body: some View {
            Section {
            } header: {
                Picker(MavioStrings.downloadsEventType, selection: $type) {
                    Text(MavioStrings.downloadsAllEvents).tag(Mavio_Acquisition_V1_HistoryEventType.unspecified)
                    ForEach(Self.types, id: \.self) { Text(historyName($0)).tag($0) }
                }
                .pickerStyle(.menu)
                .labelsHidden()
                .fixedSize()
                .accessibilityLabel(MavioStrings.downloadsEventType)
                .type(theme.type.body)
                .task(id: type) {
                    loaded = false
                    events = []
                    await load(offset: 0)
                }
            }
            Section {
                ForEach(events, id: \.id) { event in
                    HistoryRow(server: server, event: event) { Task { await load(offset: 0) } }
                        .onAppear {
                            if event.id == events.last?.id, events.count < total {
                                Task { await load(offset: Int32(events.count)) }
                            }
                        }
                }
                if loaded && events.isEmpty { Note(text: MavioStrings.downloadsHistoryEmpty) }
                if let failure { Note(text: failure, failed: true) }
            } footer: {
                if loaded { Text(MavioStrings.downloadsEventCount(count: Int(total))) }
            }
        }

        private func load(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListHistoryRequest()
            request.types = type == .unspecified ? [] : [type]
            request.limit = offset == 0 ? min(1000, max(page, Int32(events.count))) : page
            request.offset = offset
            let shown = type
            do {
                let response = try await model.client.call(
                    server.id, Procedure.downloads("ListHistory"), request,
                    as: Mavio_Acquisition_V1_ListHistoryResponse.self)
                guard shown == type else { return }
                events = offset == 0 ? response.events : events + response.events
                total = response.total
                loaded = true
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }

    /// An event of the history: its type, release and data, and marking a grab failed.
    private struct HistoryRow: View {
        let server: ServerRecord
        let event: Mavio_Acquisition_V1_HistoryEvent
        let changed: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var confirming = false
        @State private var marking = false
        @State private var failure: String?

        var body: some View {
            let e = event
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                    Badge(text: historyName(e.type), failed: e.type == .downloadFailed)
                    Line(
                        text: dotted([
                            e.quality, e.languages.joined(separator: ", "),
                            e.hasTime ? e.time.time.map(fromNow) : nil,
                        ]),
                        color: theme.colors.textSecondary)
                }
                Line(text: e.sourceTitle).textSelection(.enabled)
                if !e.itemID.isEmpty {
                    ItemLink(server: server, itemId: e.itemID) { Line(text: MavioStrings.downloadsItem) }
                }
                if !e.data.isEmpty {
                    Text(
                        e.data.sorted { $0.key.localizedStandardCompare($1.key) == .orderedAscending }
                            .map { "\($0.key)=\($0.value)" }
                            .joined(separator: "   ")
                    )
                    .type(theme.type.caption)
                    .font(.caption.monospaced())
                    .foregroundStyle(theme.colors.textSecondary)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: false, vertical: true)
                }
                if e.type == .grabbed {
                    HStack(spacing: MavioSpacing.s8) {
                        if confirming {
                            Text(MavioStrings.downloadsMarkFailedTitle).type(theme.type.callout)
                            Button(MavioStrings.actionCancel) { confirming = false }
                            Button(MavioStrings.downloadsMarkFailed, role: .destructive) { markFailed() }
                                .buttonStyle(.bordered)
                                .disabled(marking)
                        } else {
                            Button(MavioStrings.downloadsMarkFailed, role: .destructive) { confirming = true }
                        }
                        if let failure { Line(text: failure, color: theme.colors.danger) }
                    }
                    .padding(.top, MavioSpacing.s4)
                }
            }
            .padding(.vertical, MavioSpacing.s4)
        }

        private func markFailed() {
            marking = true
            failure = nil
            var request = Mavio_Acquisition_V1_MarkFailedRequest()
            request.historyEventID = event.id
            Task {
                defer { marking = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("MarkFailed"), request,
                        as: Mavio_Acquisition_V1_MarkFailedResponse.self)
                    confirming = false
                    changed()
                } catch {
                    failure = error.reason
                }
            }
        }
    }

    // MARK: Blocklist

    /// The releases not to grab again, newest first.
    struct AcquisitionDownloadsBlocklist: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @State private var entries: [Mavio_Acquisition_V1_BlocklistEntry] = []
        @State private var total: Int32 = 0
        @State private var loaded = false
        @State private var failure: String?

        var body: some View {
            Section {
                ForEach(entries, id: \.id) { entry in
                    BlocklistRow(server: server, entry: entry) { Task { await load(offset: 0) } }
                        .onAppear {
                            if entry.id == entries.last?.id, entries.count < total {
                                Task { await load(offset: Int32(entries.count)) }
                            }
                        }
                }
                if loaded && entries.isEmpty { Note(text: MavioStrings.downloadsBlocklistEmpty) }
                if let failure { Note(text: failure, failed: true) }
            } footer: {
                Text(MavioStrings.downloadsBlocklistFooter)
                    .task { await load(offset: 0) }
            }
        }

        private func load(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListBlocklistRequest()
            request.limit = offset == 0 ? min(1000, max(page, Int32(entries.count))) : page
            request.offset = offset
            do {
                let response = try await model.client.call(
                    server.id, Procedure.downloads("ListBlocklist"), request,
                    as: Mavio_Acquisition_V1_ListBlocklistResponse.self)
                entries = offset == 0 ? response.entries : entries + response.entries
                total = response.total
                loaded = true
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }

    /// A blocklisted release: what it is, why it was blocklisted, and unblocking it.
    private struct BlocklistRow: View {
        let server: ServerRecord
        let entry: Mavio_Acquisition_V1_BlocklistEntry
        let changed: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var unblocking = false
        @State private var failure: String?

        var body: some View {
            let e = entry
            HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    Line(text: e.sourceTitle).textSelection(.enabled)
                    Line(
                        text: dotted([
                            e.quality, e.languages.joined(separator: ", "), e.size > 0 ? sizeOf(e.size) : nil,
                            e.indexerName, e.hasTime ? e.time.time.map(fromNow) : nil,
                        ]),
                        color: theme.colors.textSecondary)
                    if !e.message.isEmpty { Line(text: e.message, color: theme.colors.textSecondary) }
                    if !e.itemID.isEmpty {
                        ItemLink(server: server, itemId: e.itemID) { Line(text: MavioStrings.downloadsItem) }
                    }
                    if let failure { Line(text: failure, color: theme.colors.danger) }
                }
                Spacer(minLength: 0)
                Button(MavioStrings.downloadsUnblock, role: .destructive) { unblock() }
                    .disabled(unblocking)
            }
            .padding(.vertical, MavioSpacing.s4)
        }

        private func unblock() {
            unblocking = true
            failure = nil
            var request = Mavio_Acquisition_V1_DeleteBlocklistRequest()
            request.ids = [entry.id]
            Task {
                defer { unblocking = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.downloads("DeleteBlocklist"), request,
                        as: Mavio_Acquisition_V1_DeleteBlocklistResponse.self)
                    changed()
                } catch {
                    failure = error.reason
                }
            }
        }
    }
#endif
