#if os(macOS)
    import AppKit
    import MavioAdmin
    import MavioKit
    import MavioUI
    import SwiftUI

    // A discovered title's requests on the Mac alone (docs/clients.md
    // §6.3.2): requesting it, administrators approving or declining its
    // pending requests, reporting an issue with what a library has of it,
    // and what administrators manage of it, as Seerr's manage panel.

    /// The actions of a title's header after Watch and the trailer: Request,
    /// or Request More of a series with seasons requested or available;
    /// for administrators Approve and Decline of its pending requests;
    /// Report an Issue once available; Manage for administrators; and below
    /// them why a request failed.
    struct RequestsTitleActions<Leading: View>: View {
        let serverId: String
        let title: TitleInfo
        /// The user signed in, once known.
        let user: Mavio_User_V1_User?
        /// Reads the title again.
        let reload: () async -> Void
        @ViewBuilder let leading: Leading
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var asking = false
        @State private var requesting = false
        @State private var requestFailure: String?
        @State private var deciding = false
        @State private var decideFailure: String?
        @State private var managing = false
        @State private var reporting = false
        @State private var openItem = false

        private var admin: Bool { user?.admin == true }
        private var series: Bool { title.mediaKind == .series }
        private var pending: [MediaRequest] { title.requests.filter { $0.status == .pending } }
        /// An issue is reported of what a library has, as on Seerr once available.
        private var mayReport: Bool {
            guard let user, user.admin || user.policy.reportIssues, !title.itemID.isEmpty else { return false }
            return title.status == .available || title.status == .partiallyAvailable
        }

        /// A series with seasons requested or available already requests more.
        private var some: Bool {
            title.seasons.contains { $0.status != .unspecified && $0.status != .deleted }
        }

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                    leading
                    if title.requestable {
                        button(series && some ? MavioStrings.discoverRequestMore : MavioStrings.discoverRequest, "add")
                        {
                            if series { asking = true } else { request(nil) }
                        }
                        .buttonStyle(MavioButtonStyle(title.itemID.isEmpty ? .artworkPrimary : .artworkSecondary))
                        .disabled(requesting)
                    }
                    if admin, !pending.isEmpty {
                        button(MavioStrings.requestsApprove, "check") { decide(approve: true) }
                            .buttonStyle(MavioButtonStyle(.artworkSecondary))
                            .disabled(deciding)
                        button(MavioStrings.requestsDecline, "close") { decide(approve: false) }
                            .buttonStyle(MavioButtonStyle(.artworkSecondary))
                            .disabled(deciding)
                    }
                    if mayReport {
                        button(MavioStrings.discoverReportIssue, "error") { reporting = true }
                            .buttonStyle(MavioButtonStyle(.artworkSecondary))
                    }
                    if admin {
                        button(MavioStrings.discoverManage, "settings") { managing = true }
                            .buttonStyle(MavioButtonStyle(.artworkSecondary))
                    }
                }
                if let requestFailure {
                    Text(MavioStrings.discoverRequestFailed(reason: requestFailure))
                        .type(theme.type.callout).foregroundStyle(.white)
                }
                if let decideFailure {
                    Text(decideFailure).type(theme.type.callout).foregroundStyle(.white)
                }
            }
            .sheet(isPresented: $asking) {
                RequestsSeasonsSheet(title: title, busy: requesting) { seasons in request(seasons) }
                    .environment(\.colorScheme, appScheme)
            }
            .sheet(isPresented: $managing) {
                RequestsManageSheet(serverId: serverId, title: title, changed: reload) {
                    managing = false
                    openItem = true
                }
                .environment(\.colorScheme, appScheme)
            }
            .sheet(isPresented: $reporting) {
                if let server = model.server(serverId) {
                    ReportIssueView(server: server, itemId: title.itemID, name: title.name, series: series) {}
                        .frame(minWidth: 480, minHeight: 420)
                        .environment(\.colorScheme, appScheme)
                }
            }
            .navigationDestination(isPresented: $openItem) {
                DetailsView(serverId: serverId, itemId: title.itemID)
            }
        }

        private func button(_ label: String, _ icon: String, action: @escaping () -> Void) -> some View {
            Button(action: action) {
                Label {
                    Text(label).lineLimit(1)
                } icon: {
                    Icon(icon, size: 20)
                }
            }
        }

        /// Requests the title: a movie, or the seasons of a series chosen,
        /// nil for all not requested yet; the sheet closes once answered.
        private func request(_ seasons: [Int32]?) {
            requesting = true
            Task {
                do {
                    try await RequestsAPI.create(model, serverId, title.mediaKind, title.tmdbID, seasons: seasons)
                    requestFailure = nil
                } catch {
                    requestFailure = error.reason
                }
                requesting = false
                asking = false
                await reload()
            }
        }

        /// Approves or declines every pending request of the title.
        private func decide(approve: Bool) {
            deciding = true
            let ids = pending.map(\.id)
            Task {
                decideFailure = nil
                for id in ids {
                    do {
                        if approve {
                            try await RequestsAPI.approve(model, serverId, id)
                        } else {
                            try await RequestsAPI.decline(model, serverId, id)
                        }
                    } catch {
                        decideFailure = error.reason
                    }
                }
                deciding = false
                await reload()
            }
        }
    }

    /// The app's color scheme, which sheets opened from a header take
    /// rather than the header's dark one.
    @MainActor private var appScheme: ColorScheme {
        NSApp.effectiveAppearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? .dark : .light
    }

    /// What an administrator manages of a title, as Seerr's manage panel: its
    /// requests, downloads and open issues, opening its item, and the
    /// blocklist.
    struct RequestsManageSheet: View {
        let serverId: String
        let title: TitleInfo
        /// Reads the title again once something changed.
        let changed: () async -> Void
        /// Opens the title's item, closing the sheet.
        let openItem: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var names: [String: String] = [:]
        @State private var blocking = false
        @State private var blockFailure: String?

        private var blocked: Bool { title.status == .blocklisted }
        private var blockable: Bool { title.status == .unspecified && title.requests.isEmpty }

        var body: some View {
            NavigationStack {
                ScrollView {
                    VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                        RequestsGroup(title: MavioStrings.requestsTitle) {
                            if title.requests.isEmpty {
                                Text(MavioStrings.discoverNoRequests)
                                    .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                                    .padding(.horizontal, MavioSpacing.s16)
                                    .padding(.vertical, MavioSpacing.s12)
                            } else {
                                ForEach(title.requests, id: \.id) { r in
                                    RequestsRow(
                                        serverId: serverId, request: r, admin: true,
                                        userName: names[r.requestedBy] ?? "", linked: false, changed: changed)
                                }
                            }
                        }
                        if !title.itemID.isEmpty {
                            ManageDownloads(serverId: serverId, itemId: title.itemID)
                        }
                    }
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.top, MavioSpacing.s16)
                    if !title.itemID.isEmpty, let server = model.server(serverId) {
                        // Its own heading and card, at the page's margins.
                        ItemIssues(server: server, itemId: title.itemID, version: 0)
                    }
                    VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                        if !title.itemID.isEmpty {
                            Button(MavioStrings.discoverOpenItem, action: openItem)
                                .buttonStyle(.borderless)
                                .type(theme.type.callout).fontWeight(.semibold)
                                .foregroundStyle(theme.colors.accent)
                        }
                        if blocked || blockable { block }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.vertical, MavioSpacing.s24)
                }
                .background(theme.colors.background)
                .navigationTitle(MavioStrings.discoverManageTitle(name: title.name))
                .toolbar {
                    ToolbarItem(placement: .confirmationAction) {
                        Button(MavioStrings.actionDone) { dismiss() }
                    }
                }
            }
            .frame(minWidth: 600, minHeight: 520)
            .task { names = await RequestsAPI.userNames(model, serverId) }
        }

        private var block: some View {
            RequestsGroup(footer: blocked ? nil : MavioStrings.discoverBlockFooter) {
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    Button(blocked ? MavioStrings.collectionUnblock : MavioStrings.collectionBlock) {
                        blocking = true
                        Task {
                            do {
                                try await RequestsAPI.block(model, serverId, title.mediaKind, title.tmdbID, !blocked)
                                blockFailure = nil
                            } catch {
                                blockFailure = error.reason
                            }
                            blocking = false
                            await changed()
                        }
                    }
                    .buttonStyle(MavioButtonStyle(.secondary))
                    .foregroundStyle(blocked ? theme.colors.textPrimary : theme.colors.danger)
                    .disabled(blocking)
                    if let blockFailure {
                        Text(blockFailure).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                    }
                }
                .padding(.horizontal, MavioSpacing.s16)
                .padding(.vertical, MavioSpacing.s12)
            }
        }
    }

    /// A title's downloads, in the queue of the download clients.
    private struct ManageDownloads: View {
        let serverId: String
        let itemId: String
        @Environment(AppModel.self) private var model
        @State private var downloads: [Mavio_Acquisition_V1_QueueItem] = []

        var body: some View {
            VStack(alignment: .leading, spacing: 0) {
                if !downloads.isEmpty {
                    RequestsGroup(title: MavioStrings.downloadsQueue) {
                        ForEach(downloads, id: \.id) { d in
                            QueueRow(serverId: serverId, item: d) { await load() }
                        }
                    }
                }
            }
            .task { await load() }
        }

        private func load() async {
            if let items = try? await queue(model, serverId, itemId) { downloads = items }
        }
    }

    /// The downloads of an item in the queue.
    private func queue(_ model: AppModel, _ serverId: String, _ itemId: String) async throws
        -> [Mavio_Acquisition_V1_QueueItem]
    {
        var r = Mavio_Acquisition_V1_ListQueueRequest()
        r.itemID = itemId
        r.limit = 50
        return try await model.client.call(
            serverId, Procedure.downloads("ListQueue"), r, as: Mavio_Acquisition_V1_ListQueueResponse.self
        ).items
    }

    /// A download in the queue: its state, release, details and progress, and
    /// removing it while unfinished.
    private struct QueueRow: View {
        let serverId: String
        let item: Mavio_Acquisition_V1_QueueItem
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var removing = false
        @State private var fromClient = true
        @State private var blocklist = false
        @State private var search = false
        @State private var busy = false
        @State private var failure: String?

        private var i: Mavio_Acquisition_V1_QueueItem { item }

        private var progress: Double? {
            guard i.size > 0, i.state == .downloading else { return nil }
            return max(0, min(1, 1 - Double(i.remainingSize) / Double(i.size)))
        }

        private var finished: Bool { [.imported, .failed, .ignored].contains(i.state) }

        private var detail: String {
            [
                i.quality.isEmpty ? nil : i.quality,
                i.languages.isEmpty ? nil : i.languages.joined(separator: ", "),
                i.size > 0 ? sizeOf(i.size) : nil,
                i.indexerName.isEmpty ? nil : i.indexerName,
                i.hasRemainingTime && i.state == .downloading
                    ? MavioStrings.downloadsLeft(time: lengthOf(i.remainingTime.timeInterval)) : nil,
                i.hasGrabTime ? i.grabTime.time.map { $0.formatted(.relative(presentation: .named)) } : nil,
            ]
            .compactMap(\.self).joined(separator: " · ")
        }

        var body: some View {
            VStack(alignment: .leading, spacing: 6) {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4) {
                    Text(i.itemName).type(theme.type.headline)
                    let failed = [.importBlocked, .failed, .failedPending].contains(i.state)
                    Text(stateName(i.state))
                        .type(theme.type.caption).fontWeight(.semibold)
                        .foregroundStyle(failed ? .white : theme.colors.textPrimary)
                        .padding(.horizontal, MavioSpacing.s8)
                        .padding(.vertical, 2)
                        .background(failed ? theme.colors.danger : theme.colors.fill, in: Capsule())
                    if let status = statusName(i.status) {
                        Text(status).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                Text(i.title).type(theme.type.footnote)
                Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                if let progress {
                    ProgressView(value: progress).tint(theme.colors.progress).accessibilityLabel(i.title)
                }
                ForEach(Array(i.messages.enumerated()), id: \.offset) { _, m in
                    Text(m).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                }
                if !finished {
                    Button(MavioStrings.downloadsRemove) { removing.toggle() }
                        .buttonStyle(.borderless)
                        .foregroundStyle(theme.colors.danger)
                        .padding(.top, MavioSpacing.s4)
                }
                if removing { removePanel }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, MavioSpacing.s12)
        }

        private var removePanel: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Toggle(MavioStrings.downloadsRemoveFromClient, isOn: $fromClient)
                Toggle(MavioStrings.downloadsBlocklistRelease, isOn: $blocklist)
                    .onChange(of: blocklist) { if !blocklist { search = false } }
                if blocklist { Toggle(MavioStrings.downloadsSearchAgain, isOn: $search) }
                HStack(spacing: MavioSpacing.s8) {
                    Button(MavioStrings.actionCancel) { removing = false }.buttonStyle(.borderless)
                    Button(MavioStrings.downloadsRemove, action: remove)
                        .buttonStyle(MavioButtonStyle(.secondary))
                        .foregroundStyle(theme.colors.danger)
                        .disabled(busy)
                    if let failure {
                        Text(failure).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                    }
                }
            }
            .toggleStyle(.switch)
            .type(theme.type.callout)
            .padding(.horizontal, MavioSpacing.s12)
            .padding(.vertical, MavioSpacing.s8)
            .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: MavioRadius.lg))
        }

        private func remove() {
            var r = Mavio_Acquisition_V1_RemoveFromQueueRequest()
            r.id = i.id
            r.removeFromClient = fromClient
            r.blocklist = blocklist
            r.search = search
            busy = true
            Task {
                do {
                    _ = try await model.client.call(
                        serverId, Procedure.downloads("RemoveFromQueue"), r,
                        as: Mavio_Acquisition_V1_RemoveFromQueueResponse.self)
                    failure = nil
                    removing = false
                    await changed()
                } catch {
                    failure = error.reason
                }
                busy = false
            }
        }
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

    /// A size in bytes, in gigabytes from one, else in megabytes, as the web UI's.
    private func sizeOf(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let unit: UnitInformationStorage = n >= 1e9 ? .gigabytes : .megabytes
        return Measurement(value: n / (n >= 1e9 ? 1e9 : 1e6), unit: unit)
            .formatted(
                .measurement(
                    width: .abbreviated, usage: .asProvided,
                    numberFormatStyle: .number.precision(.fractionLength(0...1))))
    }

    /// A length of time, as "1 hr, 30 min".
    private func lengthOf(_ seconds: TimeInterval) -> String {
        Duration.seconds(seconds).formatted(
            .units(allowed: [.days, .hours, .minutes, .seconds], width: .abbreviated, maximumUnitCount: 2))
    }

    /// How far a title's downloads have come, for administrators, as Seerr's
    /// status badge; nothing while none downloads.
    struct RequestsDownloadBadge: View {
        let serverId: String
        let itemId: String
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var percent: Int?

        var body: some View {
            Group {
                if let percent {
                    Text(MavioStrings.discoverDownloading(percent: String(percent)))
                        .type(theme.type.caption).fontWeight(.semibold)
                        .foregroundStyle(.white)
                        .padding(.horizontal, 6)
                        .background(theme.colors.scrim, in: .rect(cornerRadius: 4))
                        .padding(.leading, MavioSpacing.s12)
                }
            }
            .task(id: itemId) {
                guard let items = try? await queue(model, serverId, itemId) else { return }
                let downloading = items.filter { $0.state == .downloading && $0.size > 0 }
                guard !downloading.isEmpty else {
                    percent = nil
                    return
                }
                let size = Double(downloading.reduce(0) { $0 + $1.size })
                let left = Double(downloading.reduce(0) { $0 + $1.remainingSize })
                percent = Int((max(0, min(1, 1 - left / size)) * 100).rounded())
            }
        }
    }
#endif
