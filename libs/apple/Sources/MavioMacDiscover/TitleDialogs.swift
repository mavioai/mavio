#if os(macOS)
    import AppKit
    import MavioMac

    /// What an administrator manages of a title, as Seerr's manage panel: its
    /// requests, downloads and open issues, opening its item, and the
    /// blocklist.
    @MainActor
    final class ManageDialog {
        private let dialog: Dialog
        private weak var page: TitlePage?
        private var info: TitleInfo
        private var names: [String: String] = [:]
        private var downloads: [Mavio_Acquisition_V1_QueueItem] = []
        private var issues: [Mavio_Acquisition_V1_Issue] = []
        private var blocking = false
        private var blockFailure: String?
        private var tasks: [Task<Void, Never>] = []
        private var retained: ManageDialog?

        /// The dialog managing info on page.
        init(page: TitlePage, info: TitleInfo) {
            self.page = page
            self.info = info
            dialog = Dialog(title: MavioStrings.discoverManageTitle(name: info.name), width: 600, theme: page.theme)
            let done = MavioButton(MavioStrings.actionDone, kind: .primary, theme: page.theme) { [weak self] in
                self?.dialog.close()
            }
            dialog.setButtons(trailing: [done])
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        private var theme: Theme { dialog.theme }
        private var blocked: Bool { info.status == .blocklisted }
        private var blockable: Bool { info.status == .unspecified && info.requests.isEmpty }

        /// Shows the dialog over its page, following the title as it is
        /// read again.
        func present() {
            guard let page else { return }
            retained = self
            page.onInfo = { [weak self] info in
                guard let self else { return }
                self.info = info
                self.render()
                self.loadDownloads()
            }
            dialog.onClose = { [weak self, weak page] in
                page?.onInfo = nil
                self?.retained = nil
            }
            render()
            dialog.present(over: page)
            let (model, serverId) = (page.model, page.serverId)
            tasks.append(
                Task { [weak self] in
                    let names = await RequestsAPI.userNames(model, serverId)
                    guard let self, !Task.isCancelled else { return }
                    self.names = names
                    self.render()
                })
            loadDownloads()
            loadIssues()
        }

        private func loadDownloads() {
            guard let page, !info.itemID.isEmpty else { return }
            let (model, serverId, itemId) = (page.model, page.serverId, info.itemID)
            tasks.append(
                Task { [weak self] in
                    guard let items = try? await RequestsAPI.queue(model, serverId, itemId), let self, !Task.isCancelled
                    else { return }
                    self.downloads = items
                    self.render()
                })
        }

        private func loadIssues() {
            guard let page, !info.itemID.isEmpty, let server = page.model.server(page.serverId) else { return }
            var r = Mavio_Acquisition_V1_ListIssuesRequest()
            r.itemID = info.itemID
            r.statuses = [.open]
            r.sort = .modified
            r.limit = 50
            let model = page.model
            tasks.append(
                Task { [weak self] in
                    guard
                        let response = try? await model.call(
                            server, Procedure.issues("ListIssues"), r, as: Mavio_Acquisition_V1_ListIssuesResponse.self),
                        let self, !Task.isCancelled
                    else { return }
                    self.issues = response.issues
                    self.render()
                })
        }

        private func changed() {
            page?.load()
        }

        private func render() {
            guard let page, let presenter = dialog.presenter else { return }
            let serverId = page.serverId
            var views: [NSView] = []
            var requestRows: [NSView] = info.requests.map { r in
                RequestRow(
                    page: page, presenter: presenter, serverId: serverId, request: r, admin: true,
                    userName: names[r.requestedBy] ?? "", linked: false
                ) { [weak self] in self?.changed() }
            }
            if requestRows.isEmpty {
                requestRows = [
                    inset(
                        NSTextField.label(
                            MavioStrings.discoverNoRequests, theme.fonts.callout, color: theme.colors.textSecondary),
                        .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
                ]
            }
            views.append(group(MavioStrings.requestsTitle, rows: requestRows, theme: theme))
            if !downloads.isEmpty {
                views.append(
                    group(
                        MavioStrings.downloadsQueue,
                        rows: downloads.map { d in
                            QueueRow(page: page, serverId: serverId, item: d) { [weak self] in self?.loadDownloads() }
                        }, theme: theme))
            }
            if !issues.isEmpty { views.append(issuesGroup(page)) }
            if !info.itemID.isEmpty {
                let itemId = info.itemID
                let open = MavioButton(MavioStrings.discoverOpenItem, kind: .link, theme: theme) {
                    [weak self, weak page] in
                    self?.dialog.close()
                    page?.open(.item(serverId: serverId, itemId: itemId))
                }
                views.append(NSStackView.horizontal([open, Spacer()], spacing: 0))
            }
            if blocked || blockable { views.append(blockGroup(page)) }
            dialog.setContent(views)
        }

        /// The title's open issues, each opening the server's issues, where
        /// its thread is.
        private func issuesGroup(_ page: TitlePage) -> NSView {
            let rows = issues.map { issue -> NSView in
                let first = [issueTypeName(issue.type), issueNumbers(issue)].compactMap(\.self).filter { !$0.isEmpty }
                let second = [
                    issue.comments.first?.message, names[issue.createdBy],
                    issue.hasUpdateTime ? issue.updateTime.time.map(fromNow) : nil,
                ].compactMap(\.self).filter { !$0.isEmpty }
                let text = NSStackView.vertical(
                    [
                        NSTextField.label(
                            first.joined(separator: " · "), theme.fonts.body, color: theme.colors.textPrimary),
                        NSTextField.label(
                            second.joined(separator: " · "), theme.fonts.footnote, color: theme.colors.textSecondary),
                    ], spacing: 2)
                let open = issue.status == .open
                let badge = pill(
                    open ? MavioStrings.issuesOpen : MavioStrings.issuesResolved, theme.fonts.caption,
                    fill: open
                        ? theme.colors.button
                        : NSColor(srgbRed: 0x34 / 255, green: 0xC7 / 255, blue: 0x59 / 255, alpha: 1),
                    color: open ? theme.colors.onButton : .white)
                let row = NSStackView.horizontal([text, Spacer(), badge], spacing: MavioSpacing.s12)
                text.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
                let pressable = Pressable(
                    row, look: .highlight, radius: 0,
                    insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12), theme: theme
                ) { [weak self, weak page] in
                    guard let self, let page, let server = page.model.server(page.serverId) else { return }
                    self.dialog.close()
                    page.push(page.context.sectionPage(.issues, server, page.context))
                }
                pressable.setAccessibilityLabel(first.joined(separator: " · "))
                return pressable
            }
            return group(MavioStrings.issuesItemTitle, rows: rows, theme: theme)
        }

        /// Blocklisting the title, or taking it off the blocklist.
        private func blockGroup(_ page: TitlePage) -> NSView {
            let button = MavioButton(
                blocked ? MavioStrings.collectionUnblock : MavioStrings.collectionBlock,
                kind: blocked ? .secondary : .destructive, theme: theme
            ) { [weak self] in self?.block() }
            button.isEnabled = !blocking
            var views: [NSView] = [NSStackView.horizontal([button, Spacer()], spacing: 0)]
            if let blockFailure {
                views.append(
                    NSTextField.label(blockFailure, theme.fonts.footnote, color: theme.colors.danger, lines: 0))
            }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return group(
                nil, footer: blocked ? nil : MavioStrings.discoverBlockFooter,
                rows: [inset(stack, .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))], theme: theme
            )
        }

        private func block() {
            guard let page else { return }
            let (model, serverId, kind, tmdbId, on) = (page.model, page.serverId, info.mediaKind, info.tmdbID, !blocked)
            blocking = true
            render()
            tasks.append(
                Task { [weak self] in
                    var failure: String?
                    do {
                        try await RequestsAPI.block(model, serverId, kind, tmdbId, on)
                    } catch {
                        failure = error.reason
                    }
                    guard let self else { return }
                    self.blocking = false
                    self.blockFailure = failure
                    self.render()
                    self.changed()
                })
        }
    }

    /// The name of an issue's kind.
    func issueTypeName(_ type: Mavio_Acquisition_V1_IssueType) -> String {
        switch type {
        case .video: MavioStrings.issuesVideo
        case .audio: MavioStrings.issuesAudio
        case .subtitles: MavioStrings.issuesSubtitles
        case .other: MavioStrings.issuesOther
        default: ""
        }
    }

    /// The season or episode of a series an issue is about.
    func issueNumbers(_ issue: Mavio_Acquisition_V1_Issue) -> String? {
        guard issue.seasonNumber > 0 else { return nil }
        return issue.episodeNumber > 0
            ? String(format: "S%02dE%02d", issue.seasonNumber, issue.episodeNumber)
            : MavioStrings.discoverSeason(number: "\(issue.seasonNumber)")
    }

    /// A download in the queue: its state, release, details and progress, and
    /// removing it while unfinished.
    @MainActor
    final class QueueRow: NSView {
        private weak var page: Page?
        private let serverId: String
        private let item: Mavio_Acquisition_V1_QueueItem
        private let changed: @MainActor () -> Void
        private let theme: Theme
        private var removing = false
        private var fromClient = true
        private var blocklist = false
        private var search = false
        private var busy = false
        private var failure: String?
        private let panel = NSView()
        private let removeButton: MavioButton

        /// A download of serverId on page; changed is called once it is
        /// removed.
        init(
            page: Page, serverId: String, item: Mavio_Acquisition_V1_QueueItem, changed: @escaping @MainActor () -> Void
        ) {
            self.page = page
            self.serverId = serverId
            self.item = item
            self.changed = changed
            theme = page.theme
            removeButton = MavioButton(MavioStrings.downloadsRemove, kind: .destructive, theme: page.theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            panel.translatesAutoresizingMaskIntoConstraints = false
            panel.isHidden = true
            removeButton.onPress = { [weak self] in
                guard let self else { return }
                self.removing.toggle()
                self.renderPanel()
            }
            build()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private var finished: Bool { [.imported, .failed, .ignored].contains(item.state) }

        private var detail: String {
            let i = item
            return [
                i.quality.isEmpty ? nil : i.quality,
                i.languages.isEmpty ? nil : i.languages.joined(separator: ", "),
                i.size > 0 ? sizeOf(i.size) : nil,
                i.indexerName.isEmpty ? nil : i.indexerName,
                i.hasRemainingTime && i.state == .downloading
                    ? MavioStrings.downloadsLeft(time: lengthOf(i.remainingTime.timeInterval)) : nil,
                i.hasGrabTime ? i.grabTime.time.map(fromNow) : nil,
            ]
            .compactMap(\.self).joined(separator: " · ")
        }

        private func build() {
            let i = item
            let failed = [.importBlocked, .failed, .failedPending].contains(i.state)
            var head: [NSView] = [
                NSTextField.label(i.itemName, theme.fonts.headline, color: theme.colors.textPrimary),
                pill(
                    downloadStateName(i.state), theme.fonts.caption,
                    fill: failed ? theme.colors.danger : theme.colors.fill,
                    color: failed ? .white : theme.colors.textPrimary),
            ]
            if let status = downloadStatusName(i.status) {
                head.append(NSTextField.label(status, theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            var views: [NSView] = [
                WrapStack(head, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4),
                NSTextField.label(i.title, theme.fonts.footnote, color: theme.colors.textPrimary, lines: 0),
                NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
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
            for m in i.messages {
                views.append(NSTextField.label(m, theme.fonts.footnote, color: theme.colors.danger, lines: 0))
            }
            if !finished {
                views.append(
                    inset(
                        NSStackView.horizontal([removeButton, Spacer()], spacing: 0),
                        NSEdgeInsets(top: MavioSpacing.s4, left: 0, bottom: 0, right: 0)))
                views.append(panel)
            }
            let stack = NSStackView.vertical(views, spacing: 6)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            addFilling(stack, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
        }

        /// The choices of removing it: from the download client, blocklisting
        /// the release and searching again.
        private func renderPanel() {
            panel.subviews.forEach { $0.removeFromSuperview() }
            panel.isHidden = !removing
            defer { if let page { FocusEngine.active(for: page.view)?.schedule() } }
            guard removing else { return }
            var views: [NSView] = [
                LabeledSwitch(MavioStrings.downloadsRemoveFromClient, isOn: fromClient, theme: theme) { [weak self] in
                    self?.fromClient = $0
                },
                LabeledSwitch(MavioStrings.downloadsBlocklistRelease, isOn: blocklist, theme: theme) { [weak self] in
                    guard let self else { return }
                    self.blocklist = $0
                    if !$0 { self.search = false }
                    self.renderPanel()
                },
            ]
            if blocklist {
                views.append(
                    LabeledSwitch(MavioStrings.downloadsSearchAgain, isOn: search, theme: theme) { [weak self] in
                        self?.search = $0
                    })
            }
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .plain, theme: theme) { [weak self] in
                guard let self else { return }
                self.removing = false
                self.renderPanel()
            }
            let remove = MavioButton(MavioStrings.downloadsRemove, kind: .destructive, theme: theme) { [weak self] in
                self?.remove()
            }
            remove.isEnabled = !busy
            var buttons: [NSView] = [cancel, remove]
            if let failure {
                buttons.append(NSTextField.label(failure, theme.fonts.footnote, color: theme.colors.danger, lines: 0))
            }
            buttons.append(Spacer())
            views.append(NSStackView.horizontal(buttons, spacing: MavioSpacing.s8))
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let box = Box(fill: theme.colors.surfaceRaised, radius: MavioRadius.lg)
            box.addFilling(stack, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s8))
            panel.addFilling(box)
        }

        private func remove() {
            guard let page else { return }
            var r = Mavio_Acquisition_V1_RemoveFromQueueRequest()
            r.id = item.id
            r.removeFromClient = fromClient
            r.blocklist = blocklist
            r.search = search
            let (model, serverId) = (page.model, serverId)
            busy = true
            renderPanel()
            Task { [weak self] in
                var failure: String?
                do {
                    _ = try await model.client.call(
                        serverId, Procedure.downloads("RemoveFromQueue"), r,
                        as: Mavio_Acquisition_V1_RemoveFromQueueResponse.self)
                } catch {
                    failure = error.reason
                }
                guard let self else { return }
                self.busy = false
                self.failure = failure
                if failure == nil {
                    self.removing = false
                    self.changed()
                }
                self.renderPanel()
            }
        }
    }

    /// Reports a problem with a movie or series: its kind, for a series the
    /// season and episode, 0 for all, and what is wrong.
    @MainActor
    final class ReportIssueDialog {
        private let dialog: Dialog
        private weak var page: TitlePage?
        private let info: TitleInfo
        private var type = Mavio_Acquisition_V1_IssueType.video
        private var season = 0
        private var episode = 0
        private var message = ""
        private var busy = false
        private let failureLabel: NSTextField
        private let send: MavioButton
        private let episodeField: WholeNumberField
        private var retained: ReportIssueDialog?

        private static let types: [Mavio_Acquisition_V1_IssueType] = [.video, .audio, .subtitles, .other]

        /// The dialog reporting an issue with info on page.
        init(page: TitlePage, info: TitleInfo) {
            self.page = page
            self.info = info
            let theme = page.theme
            dialog = Dialog(title: MavioStrings.issuesReportTitle(name: info.name), width: 520, theme: theme)
            failureLabel = .label("", theme.fonts.footnote, color: theme.colors.danger, lines: 0)
            failureLabel.isHidden = true
            send = MavioButton(MavioStrings.issuesSend, kind: .primary, theme: theme)
            episodeField = WholeNumberField(MavioStrings.issuesEpisode, value: 0, max: 9999, theme: theme)
            send.onPress = { [weak self] in self?.report() }
            let tabs = PillTabs(Self.types.map(issueTypeName), theme: theme)
            tabs.onChange = { [weak self] in self?.type = Self.types[$0] }
            var views: [NSView] = [
                NSTextField.label(MavioStrings.issuesType, theme.fonts.headline, color: theme.colors.textPrimary),
                NSStackView.horizontal([tabs, Spacer()], spacing: 0),
            ]
            if info.mediaKind == .series {
                let seasonField = WholeNumberField(MavioStrings.issuesSeason, value: 0, max: 999, theme: theme)
                seasonField.onChange = { [weak self] value in
                    guard let self else { return }
                    self.season = value
                    self.episodeField.isHidden = value == 0
                }
                episodeField.onChange = { [weak self] in self?.episode = $0 }
                episodeField.isHidden = true
                views.append(
                    NSStackView.horizontal(
                        [
                            NSTextField.label(
                                MavioStrings.issuesSeason, theme.fonts.body, color: theme.colors.textPrimary),
                            seasonField,
                            NSTextField.label(
                                MavioStrings.issuesEpisode, theme.fonts.body, color: theme.colors.textPrimary),
                            episodeField, Spacer(),
                        ], spacing: MavioSpacing.s8))
                views.append(
                    NSTextField.label(
                        MavioStrings.issuesAllNumbers, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0
                    ))
            }
            let field = InputField("", placeholder: MavioStrings.issuesMessage, lines: 6, theme: theme)
            field.onChange = { [weak self] text in
                guard let self else { return }
                self.message = text
                self.restyle()
            }
            views.append(field)
            views.append(failureLabel)
            dialog.setContent(views)
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                self?.dialog.close()
            }
            dialog.setButtons(trailing: [cancel, send])
            restyle()
        }

        private var trimmed: String { message.trimmingCharacters(in: .whitespacesAndNewlines) }

        private func restyle() {
            send.isEnabled = !trimmed.isEmpty && !busy
        }

        /// Shows the dialog over its page.
        func present() {
            guard let page else { return }
            retained = self
            dialog.onClose = { [weak self] in self?.retained = nil }
            dialog.present(over: page)
        }

        private func report() {
            guard let page, let server = page.model.server(page.serverId) else { return }
            let series = info.mediaKind == .series
            var r = Mavio_Acquisition_V1_CreateIssueRequest()
            r.itemID = info.itemID
            r.type = type
            r.seasonNumber = series ? Int32(season) : 0
            r.episodeNumber = series && season > 0 ? Int32(episode) : 0
            r.message = trimmed
            busy = true
            restyle()
            let model = page.model
            Task { [weak self] in
                do {
                    _ = try await model.call(
                        server, Procedure.issues("CreateIssue"), r, as: Mavio_Acquisition_V1_CreateIssueResponse.self)
                    self?.dialog.close()
                } catch {
                    guard let self else { return }
                    self.failureLabel.stringValue = error.reason
                    self.failureLabel.isHidden = false
                    self.busy = false
                    self.restyle()
                }
            }
        }
    }
#endif
