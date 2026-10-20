#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    // Requests (docs/clients.md §6.3.2), on the Mac alone, as the App Store
    // apps leave requests out: the requests of the server that discovers, the
    // user's own or, for its administrators, everyone's, by status;
    // administrators approve, decline and retry them, and the seasons of a
    // pending series request are changed and pending requests deleted.

    /// A request of a movie or of seasons of a series.
    typealias MediaRequest = Mavio_Acquisition_V1_Request

    /// The requests' calls, as the web UI's useRequestTitle and useRequestActions.
    enum RequestsAPI {
        /// Requests a title; a series' seasons, or all of them when nil.
        static func create(_ model: AppModel, _ serverId: String, _ kind: MediaKind, _ tmdbId: Int32, seasons: [Int32]?)
            async throws
        {
            var r = Mavio_Acquisition_V1_CreateRequestRequest()
            r.mediaKind = kind
            r.tmdbID = tmdbId
            r.seasons = seasons ?? []
            r.allSeasons = kind == .series && seasons == nil
            _ = try await model.client.call(
                serverId, Procedure.requests("CreateRequest"), r, as: Mavio_Acquisition_V1_CreateRequestResponse.self)
        }

        /// Approves a pending request.
        static func approve(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_ApproveRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("ApproveRequest"), r, as: Mavio_Acquisition_V1_ApproveRequestResponse.self)
        }

        /// Declines a pending request.
        static func decline(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_DeclineRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("DeclineRequest"), r, as: Mavio_Acquisition_V1_DeclineRequestResponse.self)
        }

        /// Tries a failed request again.
        static func retry(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_RetryRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("RetryRequest"), r, as: Mavio_Acquisition_V1_RetryRequestResponse.self)
        }

        /// Changes the seasons of a pending series request.
        static func update(_ model: AppModel, _ serverId: String, _ id: String, seasons: [Int32]) async throws {
            var r = Mavio_Acquisition_V1_UpdateRequestRequest()
            r.id = id
            r.seasons = seasons
            _ = try await model.client.call(
                serverId, Procedure.requests("UpdateRequest"), r, as: Mavio_Acquisition_V1_UpdateRequestResponse.self)
        }

        /// Deletes a request.
        static func remove(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_DeleteRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("DeleteRequest"), r, as: Mavio_Acquisition_V1_DeleteRequestResponse.self)
        }

        /// Blocklists a title, or takes it off the blocklist.
        static func block(_ model: AppModel, _ serverId: String, _ kind: MediaKind, _ tmdbId: Int32, _ on: Bool)
            async throws
        {
            if on {
                var r = Mavio_Acquisition_V1_BlockTitleRequest()
                r.mediaKind = kind
                r.tmdbID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("BlockTitle"), r, as: Mavio_Acquisition_V1_BlockTitleResponse.self)
            } else {
                var r = Mavio_Acquisition_V1_UnblockTitleRequest()
                r.mediaKind = kind
                r.tmdbID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("UnblockTitle"), r, as: Mavio_Acquisition_V1_UnblockTitleResponse.self)
            }
        }

        /// The user signed in to a server.
        static func currentUser(_ model: AppModel, _ serverId: String) async -> Mavio_User_V1_User? {
            try? await model.client.call(
                serverId, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
                as: Mavio_User_V1_GetCurrentUserResponse.self
            ).user
        }

        /// The names of a server's users by their IDs, for its administrators.
        static func userNames(_ model: AppModel, _ serverId: String) async -> [String: String] {
            let users =
                (try? await model.client.call(
                    serverId, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                    as: Mavio_User_V1_ListUsersResponse.self))?.users ?? []
            return Dictionary(users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
        }
    }

    /// The statuses the requests page filters by, as its tabs.
    private enum RequestsFilter: String, CaseIterable, Hashable {
        case all, pending, approved, available, failed, declined

        var statuses: [Mavio_Acquisition_V1_RequestStatus] {
            switch self {
            case .all: []
            case .pending: [.pending]
            case .approved: [.approved]
            case .available: [.completed]
            case .failed: [.failed]
            case .declined: [.declined]
            }
        }

        var name: String {
            switch self {
            case .all: MavioStrings.requestsAll
            case .pending: MavioStrings.requestsPending
            case .approved: MavioStrings.requestsApproved
            case .available: MavioStrings.requestsAvailable
            case .failed: MavioStrings.requestsFailed
            case .declined: MavioStrings.requestsDeclined
            }
        }
    }

    /// The requests of a server, filtered by status.
    struct RequestsView: View {
        let serverId: String
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var filter = RequestsFilter.all
        @State private var admin: Bool?
        @State private var quotas: [String] = []
        @State private var names: [String: String] = [:]
        @State private var requests: [MediaRequest] = []
        @State private var next: Int32? = 0
        @State private var loading = false
        @State private var failure: String?

        private static let pageSize: Int32 = 30

        var body: some View {
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Picker(MavioStrings.requestsTitle, selection: $filter) {
                        ForEach(RequestsFilter.allCases, id: \.self) { Text($0.name).tag($0) }
                    }
                    .pickerStyle(.segmented)
                    .labelsHidden()
                    .fixedSize()
                    if admin == false, !quotas.isEmpty {
                        Text(quotas.joined(separator: " · "))
                            .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    }
                    list
                }
                .padding(.horizontal, theme.layout.margin)
                .padding(.top, MavioSpacing.s8)
                .padding(.bottom, MavioSpacing.s40)
            }
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.requestsTitle)
            .task {
                guard admin == nil, let user = await RequestsAPI.currentUser(model, serverId) else { return }
                admin = user.admin
                if user.admin { names = await RequestsAPI.userNames(model, serverId) } else { await loadQuotas() }
            }
            .task(id: filter) { await reload() }
        }

        @ViewBuilder private var list: some View {
            if let failure, requests.isEmpty {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await reload() } }))
            } else if requests.isEmpty, next != nil || loading {
                ProgressView().frame(maxWidth: .infinity).padding(MavioSpacing.s24)
            } else if requests.isEmpty {
                StatusView(
                    icon: "discover", title: MavioStrings.requestsNone, message: MavioStrings.requestsNoneMessage)
            } else {
                VStack(spacing: 0) {
                    ForEach(requests, id: \.id) { r in
                        if r.id != requests.first?.id { Divider().overlay(theme.colors.border) }
                        RequestsRow(
                            serverId: serverId, request: r, admin: admin == true,
                            userName: admin == true ? names[r.requestedBy] ?? "" : ""
                        ) { await reload() }
                        .onAppear { if r.id == requests.last?.id { Task { await load() } } }
                    }
                }
                .background(theme.colors.surface, in: .rect(cornerRadius: 14))
                if loading { ProgressView().frame(maxWidth: .infinity).padding(MavioSpacing.s16) }
            }
        }

        /// Reads the requests again from the first page, the user's quotas too.
        private func reload() async {
            requests = []
            next = 0
            failure = nil
            loading = false
            await load()
            if admin == false { await loadQuotas() }
        }

        private func load() async {
            guard let offset = next, !loading else { return }
            let shown = filter
            loading = true
            defer { if shown == filter { loading = false } }
            var r = Mavio_Acquisition_V1_ListRequestsRequest()
            r.statuses = shown.statuses
            r.limit = Self.pageSize
            r.offset = offset
            do {
                let response = try await model.client.call(
                    serverId, Procedure.requests("ListRequests"), r, as: Mavio_Acquisition_V1_ListRequestsResponse.self)
                guard shown == filter else { return }
                var seen = Set(requests.map(\.id))
                requests += response.requests.filter { seen.insert($0.id).inserted }
                let read = offset + Int32(response.requests.count)
                next = !response.requests.isEmpty && read < response.total ? read : nil
                failure = nil
            } catch is CancellationError {
            } catch {
                if shown == filter { failure = error.reason }
            }
        }

        /// How many more movies and seasons the user may request, when limited.
        private func loadQuotas() async {
            guard
                let q = try? await model.client.call(
                    serverId, Procedure.requests("GetQuota"), Mavio_Acquisition_V1_GetQuotaRequest(),
                    as: Mavio_Acquisition_V1_GetQuotaResponse.self)
            else { return }
            quotas = [
                q.hasMovie ? Self.quotaText(q.movie, movies: true) : nil,
                q.hasSeries ? Self.quotaText(q.series, movies: false) : nil,
            ].compactMap(\.self)
        }

        private static func quotaText(_ q: Mavio_Acquisition_V1_Quota, movies: Bool) -> String? {
            guard q.limit != 0 else { return nil }
            let (remaining, limit, days) = (String(q.remaining), String(q.limit), String(q.days))
            if movies {
                return q.days > 0
                    ? MavioStrings.requestsMovieQuotaDays(remaining: remaining, limit: limit, days: days)
                    : MavioStrings.requestsMovieQuota(remaining: remaining, limit: limit)
            }
            return q.days > 0
                ? MavioStrings.requestsSeasonQuotaDays(remaining: remaining, limit: limit, days: days)
                : MavioStrings.requestsSeasonQuota(remaining: remaining, limit: limit)
        }
    }

    /// The name of a request's status.
    private func statusName(_ status: Mavio_Acquisition_V1_RequestStatus) -> String {
        switch status {
        case .pending: MavioStrings.requestsPending
        case .approved: MavioStrings.requestsApproved
        case .completed: MavioStrings.requestsAvailable
        case .failed: MavioStrings.requestsFailed
        case .declined: MavioStrings.requestsDeclined
        default: ""
        }
    }

    /// A season's name, Specials for season 0.
    func requestsSeasonName(_ number: Int32) -> String {
        number == 0 ? MavioStrings.discoverSpecials : MavioStrings.discoverSeason(number: String(number))
    }

    /// When something happened, relative to now, as "5 minutes ago".
    private func fromNow(_ date: Date) -> String {
        date.formatted(.relative(presentation: .named))
    }

    /// A request with what the user may do to it: administrators approve,
    /// decline and retry it, and a pending one's seasons are changed and it
    /// is removed once confirmed.
    struct RequestsRow: View {
        let serverId: String
        let request: MediaRequest
        let admin: Bool
        let userName: String
        /// Whether the poster and name open the title; not in a sheet.
        var linked = true
        /// Called once the request changed, to read it again.
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var title: TitleInfo?
        @State private var editing = false
        @State private var removing = false
        @State private var busy = false
        @State private var failure: String?

        private var r: MediaRequest { request }
        private var series: Bool { r.mediaKind == .series }
        private var pending: Bool { r.status == .pending }

        private var detail: String {
            let seasons = r.seasons.map(\.seasonNumber).sorted().map(requestsSeasonName).joined(separator: ", ")
            return [
                series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
                seasons.isEmpty ? nil : seasons,
                userName.isEmpty ? nil : MavioStrings.requestsBy(name: userName),
                r.hasCreateTime ? r.createTime.time.map(fromNow) : nil,
            ]
            .compactMap(\.self).joined(separator: " · ")
        }

        var body: some View {
            HStack(alignment: .top, spacing: MavioSpacing.s12) {
                link {
                    ArtworkView(sized(title?.posterURL ?? "", 154), icon: series ? "shows" : "movies")
                        .aspectRatio(2 / 3, contentMode: .fit)
                        .frame(width: 64)
                        .clipShape(.rect(cornerRadius: MavioRadius.md))
                }
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4) {
                        link {
                            Text(r.title.isEmpty ? (title?.name ?? String(r.tmdbID)) : r.title)
                                .type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                        }
                        if r.year > 0 {
                            Text(String(r.year)).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                        }
                        StatusPill(status: r.status)
                    }
                    Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    if r.status == .failed, !r.message.isEmpty {
                        Text(r.message).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                    }
                    actions.padding(.top, MavioSpacing.s4)
                    if let failure {
                        Text(failure).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, MavioSpacing.s12)
            .task(id: r.tmdbID) { title = try? await model.title(serverId, r.mediaKind, r.tmdbID) }
            .sheet(isPresented: $editing) {
                if let title {
                    RequestsSeasonsSheet(title: title, requested: r.seasons.map(\.seasonNumber), busy: busy) {
                        seasons in
                        run {
                            try await RequestsAPI.update(model, serverId, r.id, seasons: seasons ?? [])
                        } done: {
                            editing = false
                        }
                    }
                }
            }
        }

        @ViewBuilder private func link(@ViewBuilder _ content: () -> some View) -> some View {
            if linked {
                NavigationLink(value: Route.title(serverId: serverId, kind: r.mediaKind, tmdbId: r.tmdbID)) {
                    content()
                }
                .buttonStyle(.plain)
            } else {
                content()
            }
        }

        @ViewBuilder private var actions: some View {
            if removing {
                HStack(spacing: MavioSpacing.s8) {
                    Text(MavioStrings.requestsRemoveTitle).type(theme.type.callout)
                    Button(MavioStrings.actionCancel) { removing = false }.buttonStyle(.borderless)
                    Button(MavioStrings.requestsRemove) {
                        run { try await RequestsAPI.remove(model, serverId, r.id) }
                    }
                    .buttonStyle(MavioButtonStyle(.secondary))
                    .foregroundStyle(theme.colors.danger)
                    .disabled(busy)
                }
            } else {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                    if admin, pending {
                        Button(MavioStrings.requestsApprove) {
                            run { try await RequestsAPI.approve(model, serverId, r.id) }
                        }
                        .buttonStyle(MavioButtonStyle(.primary))
                        Button(MavioStrings.requestsDecline) {
                            run { try await RequestsAPI.decline(model, serverId, r.id) }
                        }
                        .buttonStyle(MavioButtonStyle(.secondary))
                    }
                    if admin, r.status == .failed {
                        Button(MavioStrings.requestsRetry) {
                            run { try await RequestsAPI.retry(model, serverId, r.id) }
                        }
                        .buttonStyle(MavioButtonStyle(.primary))
                    }
                    if pending, series, title != nil {
                        Button(MavioStrings.requestsEditSeasons) { editing = true }
                            .buttonStyle(MavioButtonStyle(.secondary))
                    }
                    if admin || pending {
                        Button(MavioStrings.requestsRemove) { removing = true }
                            .buttonStyle(.borderless)
                            .foregroundStyle(theme.colors.danger)
                            .frame(minHeight: 36)
                    }
                }
                .disabled(busy)
            }
        }

        /// Runs a change, showing why it failed, and reads the requests again.
        private func run(_ work: @escaping () async throws -> Void, done: @escaping () -> Void = {}) {
            busy = true
            Task {
                do {
                    try await work()
                    failure = nil
                    busy = false
                    done()
                    await changed()
                } catch {
                    failure = error.reason
                    busy = false
                    done()
                }
            }
        }
    }

    /// A request's status as a pill, colored as the web UI's.
    private struct StatusPill: View {
        let status: Mavio_Acquisition_V1_RequestStatus
        @Environment(\.theme) private var theme

        var body: some View {
            let (fill, text): (Color, Color) =
                switch status {
                case .approved: (theme.colors.button, theme.colors.onButton)
                case .completed: (Color(red: 0x34 / 255, green: 0xC7 / 255, blue: 0x59 / 255), .white)
                case .failed: (theme.colors.danger, .white)
                case .declined: (theme.colors.fill, theme.colors.textSecondary)
                default: (theme.colors.fill, theme.colors.textPrimary)
                }
            Text(statusName(status))
                .type(theme.type.caption).fontWeight(.semibold)
                .foregroundStyle(text)
                .padding(.horizontal, MavioSpacing.s8)
                .padding(.vertical, 2)
                .background(fill, in: Capsule())
        }
    }

    /// The sheet choosing the seasons of a series to request: all those not
    /// requested yet, or some; or, given a pending request's seasons,
    /// changing them.
    struct RequestsSeasonsSheet: View {
        let title: TitleInfo
        /// The seasons of the pending request changed, if one is.
        var requested: [Int32]?
        let busy: Bool
        /// Called with the seasons chosen, nil for every season not requested yet.
        let request: ([Int32]?) -> Void
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var chosen: Set<Int32>

        init(title: TitleInfo, requested: [Int32]? = nil, busy: Bool, request: @escaping ([Int32]?) -> Void) {
            (self.title, self.requested, self.busy, self.request) = (title, requested, busy, request)
            let open = title.seasons.filter { $0.requestable || requested?.contains($0.seasonNumber) == true }
            _chosen = State(initialValue: Set(requested ?? open.map(\.seasonNumber)))
        }

        private func choosable(_ s: Mavio_Acquisition_V1_SeasonStatus) -> Bool {
            s.requestable || requested?.contains(s.seasonNumber) == true
        }

        private var open: [Int32] { title.seasons.filter(choosable).map(\.seasonNumber) }
        private var all: Bool { chosen.count == open.count }

        var body: some View {
            NavigationStack {
                List {
                    Toggle(
                        isOn: Binding(get: { all }, set: { chosen = $0 ? Set(open) : [] })
                    ) {
                        Text(MavioStrings.discoverAllSeasons).type(theme.type.headline)
                    }
                    ForEach(title.seasons, id: \.seasonNumber) { s in
                        Toggle(
                            isOn: Binding(
                                get: { chosen.contains(s.seasonNumber) },
                                set: { if $0 { chosen.insert(s.seasonNumber) } else { chosen.remove(s.seasonNumber) } })
                        ) {
                            HStack(spacing: MavioSpacing.s12) {
                                Text(requestsSeasonName(s.seasonNumber)).lineLimit(1)
                                Spacer(minLength: 0)
                                if s.episodeCount > 0 {
                                    Text(MavioStrings.itemEpisodeCount(count: Int(s.episodeCount)))
                                        .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                                }
                                if let status = statusLabel(s.status) {
                                    Text(status).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                                }
                            }
                        }
                        .disabled(!choosable(s))
                    }
                }
                .toggleStyle(.checkbox)
                .navigationTitle(
                    requested != nil
                        ? MavioStrings.requestsEditTitle(name: title.name)
                        : MavioStrings.discoverRequestTitle(name: title.name)
                )
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(MavioStrings.actionCancel) { dismiss() }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button(
                            requested != nil
                                ? MavioStrings.actionSave : MavioStrings.discoverRequestSeasons(count: chosen.count)
                        ) {
                            request(all && requested == nil ? nil : chosen.sorted())
                        }
                        .disabled(busy || chosen.isEmpty)
                    }
                }
            }
            .frame(minWidth: 420, minHeight: 360)
        }
    }

    /// A group of rows on a card, its title above and footer below, as the
    /// web UI's Group.
    struct RequestsGroup<Content: View>: View {
        var title: String?
        var footer: String?
        @ViewBuilder let content: Content
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                if let title { Text(title).type(theme.type.headline).padding(.horizontal, MavioSpacing.s4) }
                VStack(alignment: .leading, spacing: 0) {
                    Group(subviews: content) { subviews in
                        ForEach(Array(subviews.enumerated()), id: \.offset) { i, view in
                            if i > 0 { Divider().overlay(theme.colors.border) }
                            view
                        }
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(theme.colors.surface, in: .rect(cornerRadius: 14))
                if let footer {
                    Text(footer).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                        .padding(.horizontal, MavioSpacing.s4)
                }
            }
        }
    }
#endif
