#if os(macOS)
    import AppKit
    import MavioMac

    // Requests (docs/clients.md §6.3.2), on the Mac alone, as the App Store
    // apps leave requests out: the requests of the server that discovers, the
    // user's own or, for its administrators, everyone's, by status;
    // administrators approve, decline and retry them, and the seasons of a
    // pending series request are changed and pending requests deleted.

    /// The statuses the requests page filters by, as its tabs.
    private enum RequestsFilter: CaseIterable {
        case all, pending, approved, available, failed, declined

        /// The statuses it shows; none for all.
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

        /// The tab's name.
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

    /// The requests of a server, filtered by status, read a page at a time
    /// as the list scrolls to its end; for users not administrators, how
    /// many more they may request.
    final class RequestsPage: Page {
        private let serverId: String
        private var filter = RequestsFilter.all
        private var admin: Bool?
        private var names: [String: String] = [:]
        private var requests: [MediaRequest] = []
        private var next: Int32? = 0
        private var loading = false
        private var failure: String?
        private var task: Task<Void, Never>?
        private var userTask: Task<Void, Never>?
        private let page = ScrollPage(
            [], spacing: MavioSpacing.s12,
            insets: NSEdgeInsets(top: MavioSpacing.s8, left: 0, bottom: MavioSpacing.s40, right: 0))
        private let quotas: NSTextField
        private let list = NSView()
        private let tabs: PillTabs

        private static let pageSize: Int32 = 30

        /// The requests of serverId.
        init(context: MacContext, serverId: String) {
            self.serverId = serverId
            tabs = PillTabs(RequestsFilter.allCases.map(\.name), theme: context.theme)
            quotas = .label("", context.theme.fonts.callout, color: context.theme.colors.textSecondary, lines: 0)
            super.init(context: context)
        }

        isolated deinit {
            task?.cancel()
            userTask?.cancel()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.requestsTitle
            list.translatesAutoresizingMaskIntoConstraints = false
            quotas.isHidden = true
            tabs.onChange = { [weak self] i in
                guard let self else { return }
                self.filter = RequestsFilter.allCases[i]
                self.reload()
            }
            var views: [NSView] = []
            if theme.isTV {
                views.append(
                    margined(
                        NSTextField.label(
                            MavioStrings.requestsTitle, theme.fonts.title1, color: theme.colors.textPrimary),
                        top: MavioSpacing.s16))
            }
            views.append(margined(NSStackView.horizontal([tabs, Spacer()], spacing: 0)))
            views.append(margined(quotas))
            views.append(margined(list))
            page.setViews(views)
            NotificationCenter.default.addObserver(
                self, selector: #selector(scrolled), name: NSView.boundsDidChangeNotification,
                object: page.scroll.contentView)
            show(page)
            let (model, serverId) = (model, serverId)
            userTask = Task { [weak self] in
                guard let user = await RequestsAPI.currentUser(model, serverId), !Task.isCancelled else { return }
                self?.admin = user.admin
                if user.admin {
                    let names = await RequestsAPI.userNames(model, serverId)
                    self?.names = names
                    self?.render()
                } else {
                    await self?.loadQuotas()
                }
            }
            reload()
        }

        /// Reads more as the list nears its end.
        @objc private func scrolled() {
            let clip = page.scroll.contentView
            guard let document = page.scroll.documentView else { return }
            if clip.bounds.maxY > document.frame.height - 400 { load() }
        }

        /// Reads the requests again from the first page, the user's quotas
        /// too.
        private func reload() {
            task?.cancel()
            requests = []
            next = 0
            failure = nil
            loading = false
            render()
            load()
            if admin == false { Task { [weak self] in await self?.loadQuotas() } }
        }

        private func load() {
            guard let offset = next, !loading else { return }
            let shown = filter
            loading = true
            var r = Mavio_Acquisition_V1_ListRequestsRequest()
            r.statuses = shown.statuses
            r.limit = Self.pageSize
            r.offset = offset
            let (model, serverId) = (model, serverId)
            task = Task { [weak self] in
                let result: Result<Mavio_Acquisition_V1_ListRequestsResponse, any Error>
                do {
                    result = .success(
                        try await model.client.call(
                            serverId, Procedure.requests("ListRequests"), r,
                            as: Mavio_Acquisition_V1_ListRequestsResponse.self))
                } catch {
                    result = .failure(error)
                }
                guard let self, shown == self.filter, !Task.isCancelled else { return }
                switch result {
                case let .success(response):
                    var seen = Set(self.requests.map(\.id))
                    self.requests += response.requests.filter { seen.insert($0.id).inserted }
                    let read = offset + Int32(response.requests.count)
                    self.next = !response.requests.isEmpty && read < response.total ? read : nil
                    self.failure = nil
                case let .failure(error):
                    if error is CancellationError { return }
                    self.failure = error.reason
                }
                self.loading = false
                self.render()
            }
        }

        /// How many more movies and seasons the user may request, when
        /// limited.
        private func loadQuotas() async {
            guard
                let q = try? await model.client.call(
                    serverId, Procedure.requests("GetQuota"), Mavio_Acquisition_V1_GetQuotaRequest(),
                    as: Mavio_Acquisition_V1_GetQuotaResponse.self)
            else { return }
            let shown = [
                q.hasMovie ? Self.quotaText(q.movie, movies: true) : nil,
                q.hasSeries ? Self.quotaText(q.series, movies: false) : nil,
            ].compactMap(\.self)
            quotas.stringValue = shown.joined(separator: " · ")
            quotas.isHidden = shown.isEmpty
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

        private func render() {
            let view: NSView
            if let failure, requests.isEmpty {
                view = StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { [weak self] in self?.reload() }), theme: theme)
            } else if requests.isEmpty, next != nil || loading {
                view = spinner()
            } else if requests.isEmpty {
                view = StatusView(
                    icon: "discover", title: MavioStrings.requestsNone, message: MavioStrings.requestsNoneMessage,
                    theme: theme)
            } else {
                let admin = admin == true
                let rows = requests.map { r in
                    RequestRow(
                        page: self, presenter: self, serverId: serverId, request: r, admin: admin,
                        userName: admin ? names[r.requestedBy] ?? "" : ""
                    ) { [weak self] in self?.reload() }
                }
                var views: [NSView] = [group(nil, rows: rows, theme: theme)]
                if loading { views.append(spinner()) }
                let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
                for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                view = stack
            }
            list.subviews.forEach { $0.removeFromSuperview() }
            list.addFilling(view)
            FocusEngine.active(for: self.view)?.schedule()
            // A first page shorter than the window reads the next at once.
            DispatchQueue.main.async { [weak self] in self?.scrolled() }
        }

        private func spinner() -> NSView {
            let spinner = NSProgressIndicator()
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.startAnimation(nil)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(spinner)
            spinner.translatesAutoresizingMaskIntoConstraints = false
            spinner.center(in: holder)
            holder.heightAnchor.constraint(equalToConstant: 64).isActive = true
            return holder
        }
    }

    /// A request with what the user may do to it: administrators approve,
    /// decline and retry it, and a pending one's seasons are changed and it
    /// is removed once confirmed.
    @MainActor
    final class RequestRow: NSView {
        private weak var page: Page?
        private weak var presenter: NSViewController?
        private let serverId: String
        private let request: MediaRequest
        private let admin: Bool
        private let userName: String
        private let linked: Bool
        private let changed: @MainActor () -> Void
        private var title: TitleInfo?
        private var removing = false
        private var busy = false
        private var failure: String?
        private var task: Task<Void, Never>?
        private let theme: Theme
        private let poster: ArtworkView
        private let nameLabel: NSTextField
        private let actions = NSView()
        private let failureLabel: NSTextField

        /// A request on page, its dialogs presented over presenter; linked,
        /// its poster and name open the title. changed is called once it
        /// changed.
        init(
            page: Page, presenter: NSViewController, serverId: String, request: MediaRequest, admin: Bool,
            userName: String, linked: Bool = true, changed: @escaping @MainActor () -> Void
        ) {
            self.page = page
            self.presenter = presenter
            self.serverId = serverId
            self.request = request
            self.admin = admin
            self.userName = userName
            self.linked = linked
            self.changed = changed
            theme = page.theme
            let series = request.mediaKind == .series
            poster = ArtworkView(nil, icon: series ? "shows" : "movies", theme: theme)
            poster.radius = MavioRadius.md
            poster.size(width: 64, height: 96)
            nameLabel = label(
                request.title.isEmpty ? String(request.tmdbID) : request.title, theme.fonts.headline,
                color: theme.colors.textPrimary, lines: 2)
            failureLabel = .label("", theme.fonts.footnote, color: theme.colors.danger, lines: 0)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            actions.translatesAutoresizingMaskIntoConstraints = false
            failureLabel.isHidden = true
            build()
            let model = page.model
            let (kind, tmdbId) = (request.mediaKind, request.tmdbID)
            task = Task { [weak self] in
                guard let title = try? await model.title(serverId, kind, tmdbId), let self, !Task.isCancelled else {
                    return
                }
                self.title = title
                self.poster.set(sized(title.posterURL, 154), title: title.name)
                if request.title.isEmpty { self.nameLabel.stringValue = title.name }
                self.renderActions()
            }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { task?.cancel() }

        private var series: Bool { request.mediaKind == .series }
        private var pending: Bool { request.status == .pending }

        private var detail: String {
            let seasons = request.seasons.map(\.seasonNumber).sorted().map(seasonName).joined(separator: ", ")
            return [
                series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
                seasons.isEmpty ? nil : seasons,
                userName.isEmpty ? nil : MavioStrings.requestsBy(name: userName),
                request.hasCreateTime ? request.createTime.time.map(fromNow) : nil,
            ]
            .compactMap(\.self).joined(separator: " · ")
        }

        private func build() {
            let r = request
            var head: [NSView] = [link(nameLabel)]
            if r.year > 0 {
                head.append(NSTextField.label(String(r.year), theme.fonts.callout, color: theme.colors.textSecondary))
            }
            head.append(statusPill(r.status))
            var column: [NSView] = [
                WrapStack(head, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4),
                NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
            ]
            if r.status == .failed, !r.message.isEmpty {
                column.append(NSTextField.label(r.message, theme.fonts.footnote, color: theme.colors.danger, lines: 0))
            }
            column.append(inset(actions, NSEdgeInsets(top: MavioSpacing.s4, left: 0, bottom: 0, right: 0)))
            column.append(failureLabel)
            let stack = NSStackView.vertical(column, spacing: MavioSpacing.s4)
            for v in column { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let row = NSStackView.horizontal([link(poster), stack], spacing: MavioSpacing.s12, alignment: .top)
            stack.setContentHuggingPriority(.init(1), for: .horizontal)
            addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
            renderActions()
        }

        /// A view opening the title, when linked.
        private func link(_ content: NSView) -> NSView {
            guard linked else { return content }
            let (serverId, r) = (serverId, request)
            let link = Pressable(content, look: .highlight, radius: MavioRadius.md, theme: theme) { [weak self] in
                self?.page?.open(.title(serverId: serverId, kind: r.mediaKind, tmdbId: r.tmdbID))
            }
            link.setAccessibilityLabel(nameLabel.stringValue)
            return link
        }

        /// A request's status as a pill, colored as the web UI's.
        private func statusPill(_ status: Mavio_Acquisition_V1_RequestStatus) -> NSView {
            let c = theme.colors
            let (fill, text): (NSColor, NSColor) =
                switch status {
                case .approved: (c.button, c.onButton)
                case .completed: (NSColor(srgbRed: 0x34 / 255, green: 0xC7 / 255, blue: 0x59 / 255, alpha: 1), .white)
                case .failed: (c.danger, .white)
                case .declined: (c.fill, c.textSecondary)
                default: (c.fill, c.textPrimary)
                }
            return pill(requestStatusName(status), theme.fonts.caption, fill: fill, color: text)
        }

        private func renderActions() {
            var views: [NSView] = []
            if removing {
                views.append(
                    NSTextField.label(
                        MavioStrings.requestsRemoveTitle, theme.fonts.callout, color: theme.colors.textPrimary))
                views.append(
                    MavioButton(MavioStrings.actionCancel, kind: .plain, theme: theme) { [weak self] in
                        self?.removing = false
                        self?.renderActions()
                    })
                views.append(
                    MavioButton(MavioStrings.requestsRemove, kind: .destructive, theme: theme) { [weak self] in
                        self?.run { model, serverId, id in try await RequestsAPI.remove(model, serverId, id) }
                    })
            } else {
                if admin, pending {
                    views.append(
                        MavioButton(MavioStrings.requestsApprove, kind: .primary, theme: theme) { [weak self] in
                            self?.run { model, serverId, id in try await RequestsAPI.approve(model, serverId, id) }
                        })
                    views.append(
                        MavioButton(MavioStrings.requestsDecline, kind: .secondary, theme: theme) { [weak self] in
                            self?.run { model, serverId, id in try await RequestsAPI.decline(model, serverId, id) }
                        })
                }
                if admin, request.status == .failed {
                    views.append(
                        MavioButton(MavioStrings.requestsRetry, kind: .primary, theme: theme) { [weak self] in
                            self?.run { model, serverId, id in try await RequestsAPI.retry(model, serverId, id) }
                        })
                }
                if pending, series, title != nil {
                    views.append(
                        MavioButton(MavioStrings.requestsEditSeasons, kind: .secondary, theme: theme) { [weak self] in
                            self?.editSeasons()
                        })
                }
                if admin || pending {
                    views.append(
                        MavioButton(MavioStrings.requestsRemove, kind: .destructive, theme: theme) { [weak self] in
                            self?.removing = true
                            self?.renderActions()
                        })
                }
            }
            for case let b as MavioButton in views { b.isEnabled = !busy }
            actions.subviews.forEach { $0.removeFromSuperview() }
            if !views.isEmpty {
                actions.addFilling(WrapStack(views, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8))
            }
            actions.isHidden = views.isEmpty
            failureLabel.stringValue = failure ?? ""
            failureLabel.isHidden = failure == nil
            if let page { FocusEngine.active(for: page.view)?.schedule() }
        }

        /// Changes the seasons of the pending request.
        private func editSeasons() {
            guard let title, let presenter else { return }
            let dialog = SeasonsDialog(title: title, requested: request.seasons.map(\.seasonNumber), theme: theme)
            dialog.confirm = { [weak self, weak dialog] seasons in
                dialog?.busy = true
                self?.run(done: { dialog?.close() }) { model, serverId, id in
                    try await RequestsAPI.update(model, serverId, id, seasons: seasons ?? [])
                }
            }
            dialog.present(over: presenter)
        }

        /// Runs a change, showing why it failed, and reads the requests
        /// again.
        private func run(
            done: @escaping @MainActor () -> Void = {},
            _ work: @escaping @MainActor (AppModel, String, String) async throws -> Void
        ) {
            guard let page else { return }
            let (model, serverId, id) = (page.model, serverId, request.id)
            busy = true
            renderActions()
            Task { [weak self] in
                var failure: String?
                do {
                    try await work(model, serverId, id)
                } catch {
                    failure = error.reason
                }
                done()
                guard let self else { return }
                self.busy = false
                self.failure = failure
                if failure == nil {
                    self.removing = false
                    self.changed()
                }
                self.renderActions()
            }
        }
    }

    /// The dialog choosing the seasons of a series to request: all those not
    /// requested yet, or some; or, given a pending request's seasons,
    /// changing them.
    @MainActor
    final class SeasonsDialog {
        private let dialog: Dialog
        private let title: TitleInfo
        private let requested: [Int32]?
        private var chosen: Set<Int32>
        private var boxes: [Int32: CheckBox] = [:]
        private var allBox: CheckBox?
        private let confirmButton: MavioButton
        private var retained: SeasonsDialog?
        /// Called with the seasons chosen, nil for every season not
        /// requested yet.
        var confirm: (@MainActor ([Int32]?) -> Void)?
        /// Whether the request is on its way, which disables confirming.
        var busy = false { didSet { restyle() } }

        /// The seasons of title, or of a pending request of it.
        init(title: TitleInfo, requested: [Int32]? = nil, theme: Theme) {
            self.title = title
            self.requested = requested
            let open = title.seasons.filter { $0.requestable || requested?.contains($0.seasonNumber) == true }
            chosen = Set(requested ?? open.map(\.seasonNumber))
            dialog = Dialog(
                title: requested != nil
                    ? MavioStrings.requestsEditTitle(name: title.name)
                    : MavioStrings.discoverRequestTitle(name: title.name),
                width: 460, theme: theme)
            confirmButton = MavioButton("", kind: .primary, theme: theme)
            confirmButton.onPress = { [weak self] in
                guard let self else { return }
                self.confirm?(self.all && self.requested == nil ? nil : self.chosen.sorted())
            }
            let theme = dialog.theme
            var rows: [NSView] = []
            let (allRow, allBox) = checkRow(MavioStrings.discoverAllSeasons, on: all, theme: theme) { [weak self] on in
                guard let self else { return }
                self.chosen = on ? Set(self.open) : []
                self.restyle()
            }
            self.allBox = allBox
            rows.append(allRow)
            for s in title.seasons {
                let details = [
                    s.episodeCount > 0 ? MavioStrings.itemEpisodeCount(count: Int(s.episodeCount)) : nil,
                    statusLabel(s.status),
                ].compactMap(\.self)
                let number = s.seasonNumber
                let (row, box) = checkRow(
                    seasonName(number), details: details, on: chosen.contains(number), enabled: choosable(s),
                    theme: theme
                ) { [weak self] on in
                    guard let self else { return }
                    if on { self.chosen.insert(number) } else { self.chosen.remove(number) }
                    self.restyle()
                }
                boxes[number] = box
                rows.append(row)
            }
            dialog.setContent([group(nil, rows: rows, theme: theme)])
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                self?.close()
            }
            dialog.setButtons(trailing: [cancel, confirmButton])
            restyle()
        }

        private func choosable(_ s: Mavio_Acquisition_V1_SeasonStatus) -> Bool {
            s.requestable || requested?.contains(s.seasonNumber) == true
        }

        private var open: [Int32] { title.seasons.filter(choosable).map(\.seasonNumber) }
        private var all: Bool { chosen.count == open.count }

        private func restyle() {
            allBox?.state = all ? .on : .off
            for (number, box) in boxes { box.state = chosen.contains(number) ? .on : .off }
            confirmButton.title =
                requested != nil ? MavioStrings.actionSave : MavioStrings.discoverRequestSeasons(count: chosen.count)
            confirmButton.isEnabled = !busy && !chosen.isEmpty
        }

        /// Shows the dialog over presenter.
        func present(over presenter: NSViewController) {
            retained = self
            dialog.onClose = { [weak self] in self?.retained = nil }
            dialog.present(over: presenter)
        }

        /// Closes the dialog.
        func close() { dialog.close() }
    }
#endif
