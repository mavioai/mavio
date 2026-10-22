#if os(macOS)
    import AppKit
    import MavioMac

    // A discovered title's actions (docs/clients.md §6.3.2): Watch and the
    // trailer, requesting it, administrators approving or declining its
    // pending requests, reporting an issue with what a library has of it,
    // and what administrators manage of it, as Seerr's manage panel.

    extension TitlePage {
        private var admin: Bool { user?.admin == true }

        /// An issue is reported of what a library has, as on Seerr once
        /// available.
        private func mayReport(_ info: TitleInfo) -> Bool {
            guard let user, user.admin || user.policy.reportIssues, !info.itemID.isEmpty else { return false }
            return info.status == .available || info.status == .partiallyAvailable
        }

        /// The header's actions: Watch, when a library has it, and the
        /// trailer, which television mode leaves out as it opens no web
        /// pages; Request, or Request More of a series with seasons
        /// requested or available; for administrators Approve and Decline
        /// of its pending requests; Report an Issue once available; Manage
        /// for administrators; and why a request or a decision failed.
        func actions(_ info: TitleInfo) -> (actions: [NSView], failures: [String]) {
            var views: [NSView] = []
            let serverId = serverId
            if !info.itemID.isEmpty {
                let watch = MavioButton(
                    MavioStrings.discoverWatch, icon: "play", kind: .artworkPrimary, theme: theme
                ) { [weak self] in
                    self?.open(.item(serverId: serverId, itemId: info.itemID))
                }
                views.append(watch)
            }
            if !theme.isTV, let url = info.trailerUrls.first.flatMap(URL.init(string:)) {
                views.append(
                    MavioButton(MavioStrings.itemTrailer, icon: "movies", kind: .artworkSecondary, theme: theme) {
                        NSWorkspace.shared.open(url)
                    })
            }
            let series = info.mediaKind == .series
            if info.requestable {
                let some = info.seasons.contains { $0.status != .unspecified && $0.status != .deleted }
                let button = MavioButton(
                    series && some ? MavioStrings.discoverRequestMore : MavioStrings.discoverRequest, icon: "add",
                    kind: info.itemID.isEmpty ? .artworkPrimary : .artworkSecondary, theme: theme
                ) { [weak self] in
                    guard let self, let info = self.info else { return }
                    if info.mediaKind == .series { self.askSeasons(info) } else { self.request(nil) }
                }
                button.isEnabled = !busy
                views.append(button)
            }
            let pending = info.requests.filter { $0.status == .pending }
            if admin, !pending.isEmpty {
                for approve in [true, false] {
                    let button = MavioButton(
                        approve ? MavioStrings.requestsApprove : MavioStrings.requestsDecline,
                        icon: approve ? "check" : "close", kind: .artworkSecondary, theme: theme
                    ) { [weak self] in self?.decide(approve: approve) }
                    button.isEnabled = !busy
                    views.append(button)
                }
            }
            if mayReport(info) {
                views.append(
                    MavioButton(MavioStrings.discoverReportIssue, icon: "error", kind: .artworkSecondary, theme: theme)
                    {
                        [weak self] in
                        guard let self, let info = self.info else { return }
                        ReportIssueDialog(page: self, info: info).present()
                    })
            }
            if admin {
                views.append(
                    MavioButton(MavioStrings.discoverManage, icon: "settings", kind: .artworkSecondary, theme: theme) {
                        [weak self] in
                        guard let self, let info = self.info else { return }
                        ManageDialog(page: self, info: info).present()
                    })
            }
            let failures = [requestFailure.map { MavioStrings.discoverRequestFailed(reason: $0) }, decideFailure]
                .compactMap(\.self)
            return (views, failures)
        }

        /// Asks which seasons of a series to request.
        private func askSeasons(_ info: TitleInfo) {
            let dialog = SeasonsDialog(title: info, theme: theme)
            dialog.confirm = { [weak self, weak dialog] seasons in
                dialog?.busy = true
                self?.request(seasons) { dialog?.close() }
            }
            dialog.present(over: self)
        }

        /// Requests the title: a movie, or the seasons of a series chosen,
        /// nil for all not requested yet; then reads it again.
        private func request(_ seasons: [Int32]?, done: @escaping @MainActor () -> Void = {}) {
            guard let info else { return }
            let (model, serverId) = (model, serverId)
            busy = true
            renderActions()
            Task { [weak self] in
                var failure: String?
                do {
                    try await RequestsAPI.create(model, serverId, info.mediaKind, info.tmdbID, seasons: seasons)
                } catch {
                    failure = error.reason
                }
                done()
                guard let self else { return }
                self.requestFailure = failure
                self.busy = false
                self.renderActions()
                self.load()
            }
        }

        /// Approves or declines every pending request of the title.
        private func decide(approve: Bool) {
            guard let info else { return }
            let ids = info.requests.filter { $0.status == .pending }.map(\.id)
            let (model, serverId) = (model, serverId)
            busy = true
            decideFailure = nil
            renderActions()
            Task { [weak self] in
                var failure: String?
                for id in ids {
                    do {
                        if approve {
                            try await RequestsAPI.approve(model, serverId, id)
                        } else {
                            try await RequestsAPI.decline(model, serverId, id)
                        }
                    } catch {
                        failure = error.reason
                    }
                }
                guard let self else { return }
                self.decideFailure = failure
                self.busy = false
                self.renderActions()
                self.load()
            }
        }
    }

    // MARK: Seasons

    /// A series' seasons, each opening its episodes, read as it first opens.
    @MainActor
    final class SeasonsView: NSView {
        /// The seasons of info.
        init(page: TitlePage, info: TitleInfo) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let rows: [NSView] =
                [page.heading(MavioStrings.itemSeasons)]
                + seasonOrder(info.seasons).map { SeasonBox(page: page, tmdbId: info.tmdbID, season: $0) }
            let stack = NSStackView.vertical(rows, spacing: MavioSpacing.s8)
            for r in rows { r.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            addFilling(page.margined(RegionView(stack), top: MavioSpacing.s12, bottom: MavioSpacing.s12))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// A season: its name, episodes and status, opening its episodes, the
    /// latest first.
    @MainActor
    private final class SeasonBox: Box {
        private weak var page: TitlePage?
        private let tmdbId: Int32
        private let season: Mavio_Acquisition_V1_SeasonStatus
        private let list = NSStackView.vertical([], spacing: 0)
        private let expand: IconView
        private let header: Pressable
        private var open = false
        private var episodes: [Mavio_Acquisition_V1_SeasonEpisode]?
        private var failed = false
        private var task: Task<Void, Never>?
        private let theme: Theme

        /// A season of the series of TMDB ID on page.
        init(page: TitlePage, tmdbId: Int32, season: Mavio_Acquisition_V1_SeasonStatus) {
            self.page = page
            self.tmdbId = tmdbId
            self.season = season
            theme = page.theme
            expand = IconView("expand", size: 16, color: theme.colors.textSecondary)
            var views: [NSView] = [
                label(seasonName(season.seasonNumber), theme.fonts.headline, color: theme.colors.textPrimary), Spacer(),
            ]
            if season.episodeCount > 0 {
                views.append(
                    NSTextField.label(
                        MavioStrings.itemEpisodeCount(count: Int(season.episodeCount)), theme.fonts.callout,
                        color: theme.colors.textSecondary))
            }
            if let status = statusLabel(season.status) {
                views.append(
                    pill(
                        status, theme.fonts.footnote, fill: theme.colors.fill, color: theme.colors.textPrimary,
                        insets: .symmetric(horizontal: 10, vertical: 2)))
            }
            views.append(expand)
            let row = NSStackView.horizontal(views, spacing: MavioSpacing.s12)
            row.heightAnchor.constraint(greaterThanOrEqualToConstant: 48).isActive = true
            header = Pressable(
                row, look: .highlight, radius: 10, insets: .symmetric(horizontal: MavioSpacing.s16), theme: theme)
            super.init(fill: theme.isTV ? theme.colors.fill : theme.colors.surfaceRaised, radius: 10)
            border = theme.colors.border
            header.onPress = { [weak self] in self?.toggle() }
            header.setAccessibilityLabel(seasonName(season.seasonNumber))
            list.isHidden = true
            let stack = NSStackView.vertical([header, list], spacing: 0)
            header.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            list.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            addFilling(stack)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { task?.cancel() }

        private func toggle() {
            open.toggle()
            expand.upsideDown = open
            header.setAccessibilitySelected(open)
            list.isHidden = !open
            if open, episodes == nil, task == nil { load() }
            renderList()
        }

        private func load() {
            guard let page else { return }
            let (model, serverId, tmdbId, number) = (page.model, page.serverId, tmdbId, season.seasonNumber)
            failed = false
            task = Task { [weak self] in
                do {
                    let episodes = try await model.season(serverId, tmdbId, number)
                    guard let self, !Task.isCancelled else { return }
                    self.episodes = episodes
                } catch {
                    guard let self, !Task.isCancelled else { return }
                    self.failed = !(error is CancellationError)
                }
                self?.task = nil
                self?.renderList()
            }
        }

        private func renderList() {
            guard open else { return }
            var rows: [NSView] = []
            if let episodes, !episodes.isEmpty {
                // Only the episodes near the page's view are made, a season
                // having up to hundreds, each with its still.
                let latest = Array(episodes.reversed())
                let lazy = LazyRows(estimate: 64)
                lazy.show(keys: latest.map { String($0.episodeNumber) }) { [weak self] i in
                    guard let self else { return NSView() }
                    let rows = [divider(self.theme), self.episode(latest[i])]
                    let stack = NSStackView.vertical(rows, spacing: 0)
                    for r in rows { r.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                    return stack
                }
                rows.append(lazy)
            } else if episodes != nil || failed {
                let text = NSTextField.label(
                    failed ? MavioStrings.stateError : MavioStrings.discoverNoEpisodes, theme.fonts.callout,
                    color: theme.colors.textSecondary)
                rows.append(
                    inset(
                        text,
                        NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: MavioSpacing.s16, right: MavioSpacing.s16))
                )
            } else {
                let spinner = NSProgressIndicator()
                spinner.style = .spinning
                spinner.controlSize = .small
                spinner.startAnimation(nil)
                // Beside a space taking the rest of the row: pinned to both
                // sides, the spinner's own width would narrow the window.
                rows.append(
                    inset(
                        NSStackView.horizontal([spinner, Spacer()], spacing: 0),
                        NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: MavioSpacing.s16, right: 0)))
            }
            list.setViews([])
            for r in rows {
                list.addArrangedSubview(r)
                r.widthAnchor.constraint(equalTo: list.widthAnchor).isActive = true
            }
            if let page { FocusEngine.active(for: page.view)?.schedule() }
        }

        private func episode(_ e: Mavio_Acquisition_V1_SeasonEpisode) -> NSView {
            var head: [NSView] = [
                NSTextField.label(
                    MavioStrings.discoverEpisodeTitle(number: String(e.episodeNumber), name: e.name),
                    theme.fonts.headline,
                    color: theme.colors.textPrimary, lines: 0)
            ]
            if let day = dayText(e.airDate) {
                head.append(
                    pill(
                        day, theme.fonts.footnote, fill: theme.colors.fill, color: theme.colors.textPrimary,
                        insets: .symmetric(horizontal: 10, vertical: 2)))
            }
            var views: [NSView] = [WrapStack(head, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4)]
            if !e.overview.isEmpty {
                views.append(
                    NSTextField.label(e.overview, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0))
            }
            if let still = sized(e.stillURL, 300) {
                let art = ArtworkView(still, title: e.name, icon: "shows", theme: theme)
                art.radius = MavioRadius.lg
                art.aspect(16 / 9)
                let holder = NSStackView.horizontal([art, Spacer()], spacing: 0)
                art.widthAnchor.constraint(lessThanOrEqualToConstant: 480).isActive = true
                let wide = art.widthAnchor.constraint(equalTo: holder.widthAnchor)
                wide.priority = .defaultLow
                wide.isActive = true
                views.append(holder)
            }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            // In television mode an episode takes focus, so that its list scrolls.
            let content: NSView =
                theme.isTV
                ? Pressable(stack, look: .highlight, radius: MavioRadius.md, theme: theme, action: {}) : stack
            return inset(content, .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s16))
        }
    }
#endif
