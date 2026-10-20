#if os(macOS)
    import AppKit
    import MavioMac

    extension Mavio_Acquisition_V1_IssueType {
        /// The kind's name.
        fileprivate var title: String {
            switch self {
            case .video: MavioStrings.issuesVideo
            case .audio: MavioStrings.issuesAudio
            case .subtitles: MavioStrings.issuesSubtitles
            case .other: MavioStrings.issuesOther
            default: ""
            }
        }
    }

    extension Mavio_Acquisition_V1_Issue {
        /// The season or episode of a series it is about.
        fileprivate var numbers: String? {
            guard seasonNumber > 0 else { return nil }
            return episodeNumber > 0
                ? String(format: "S%02dE%02d", seasonNumber, episodeNumber)
                : MavioStrings.discoverSeason(number: "\(seasonNumber)")
        }
    }

    /// An item's open issues the user may see, on its details: each says
    /// its kind, numbers, first comment, who reported it and when it last
    /// changed, and opens the server's issues.
    final class ItemIssuesView: NSView {
        private weak var page: DetailsPage?
        private let server: ServerRecord
        private let itemId: String
        private var users: [String: String] = [:]
        private var task: Task<Void, Never>?

        /// The issues of an item of server.
        init(page: DetailsPage, server: ServerRecord, itemId: String) {
            self.page = page
            self.server = server
            self.itemId = itemId
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            load()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func load() {
            guard let model = page?.model else { return }
            var request = Mavio_Acquisition_V1_ListIssuesRequest()
            request.itemID = itemId
            request.statuses = [.open]
            request.sort = .modified
            request.limit = 50
            let server = server
            task = Task { [weak self] in
                guard
                    let r = try? await model.call(
                        server, Procedure.issues("ListIssues"), request,
                        as: Mavio_Acquisition_V1_ListIssuesResponse.self),
                    !Task.isCancelled
                else { return }
                let users = try? await model.call(
                    server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                    as: Mavio_User_V1_ListUsersResponse.self)
                guard !Task.isCancelled, let self else { return }
                self.users = Dictionary(
                    (users?.users ?? []).map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
                self.show(r.issues)
            }
        }

        /// A user's name: the user's own, everyone's for an administrator,
        /// else "Administrator".
        private func user(_ id: String) -> String {
            id == server.userId ? server.userName : users[id] ?? MavioStrings.issuesAdministrator
        }

        private func show(_ issues: [Mavio_Acquisition_V1_Issue]) {
            subviews.forEach { $0.removeFromSuperview() }
            guard let page, !issues.isEmpty else { return }
            let theme = page.theme
            var rows: [NSView] = []
            for issue in issues {
                if !rows.isEmpty {
                    let divider = Box(fill: theme.colors.border)
                    divider.size(height: 1)
                    rows.append(inset(divider, NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: 0, right: 0)))
                }
                rows.append(row(issue, theme: theme))
            }
            let list = NSStackView.vertical(rows, spacing: 0)
            for v in rows { v.widthAnchor.constraint(equalTo: list.widthAnchor).isActive = true }
            let box = Box(fill: theme.colors.surface, radius: MavioRadius.lg)
            box.addFilling(list)
            let stack = NSStackView.vertical(
                [page.heading(MavioStrings.issuesItemTitle), box], spacing: MavioSpacing.s12)
            box.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            addFilling(
                stack,
                insets: NSEdgeInsets(
                    top: MavioSpacing.s24, left: theme.layout.margin, bottom: 0, right: theme.layout.margin))
            FocusEngine.active(for: self)?.schedule()
        }

        private func row(_ issue: Mavio_Acquisition_V1_Issue, theme: Theme) -> NSView {
            let top = [issue.type.title, issue.numbers].compactMap(\.self).filter { !$0.isEmpty }.joined(
                separator: " · ")
            let bottom = [issue.comments.first?.message, user(issue.createdBy), issue.updateTime.time.map(fromNow)]
                .compactMap(\.self).filter { !$0.isEmpty }.joined(separator: " · ")
            let text = NSStackView.vertical(
                [
                    NSTextField.label(top, theme.fonts.body, color: theme.colors.textPrimary),
                    NSTextField.label(bottom, theme.fonts.footnote, color: theme.colors.textSecondary),
                ], spacing: 2)
            let open = issue.status == .open
            let badge = Box(
                fill: open
                    ? theme.colors.button : NSColor(red: 0x34 / 255, green: 0xC7 / 255, blue: 0x59 / 255, alpha: 1),
                radius: 999)
            badge.addFilling(
                label(
                    open ? MavioStrings.issuesOpen : MavioStrings.issuesResolved, theme.fonts.caption,
                    weight: .semibold,
                    color: open ? theme.colors.onButton : .white),
                insets: .symmetric(horizontal: MavioSpacing.s8, vertical: 2))
            let row = NSStackView.horizontal([text, Spacer(), badge], spacing: MavioSpacing.s12)
            let server = server
            let pressable = Pressable(
                row, look: .highlight, radius: MavioRadius.lg,
                insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12), theme: theme
            ) { [weak page] in
                guard let page else { return }
                page.push(page.context.sectionPage(.issues, server, page.context))
            }
            pressable.setAccessibilityLabel("\(top), \(bottom)")
            return pressable
        }

        isolated deinit { task?.cancel() }
    }

    /// Reports a problem with a movie, series, season or episode: its kind,
    /// for a series the season and episode, 0 for all, and what is wrong.
    final class ReportIssueSheet: NSViewController {
        private let server: ServerRecord
        private let itemId: String
        private let name: String
        private let series: Bool
        private let context: MacContext
        private let reported: @MainActor () -> Void
        private let types: [Mavio_Acquisition_V1_IssueType] = [.video, .audio, .subtitles, .other]
        private let kind = NSSegmentedControl()
        private let season = NSTextField()
        private let seasonStepper = NSStepper()
        private let episode = NSTextField()
        private let episodeStepper = NSStepper()
        private let message = NSTextView()
        private let failure = NSTextField(wrappingLabelWithString: "")
        private var send: MavioButton?
        private var task: Task<Void, Never>?

        /// Reports an issue with an item of server named name, a series'
        /// with its season and episode; reported is called once it is.
        init(
            server: ServerRecord, itemId: String, name: String, series: Bool, context: MacContext,
            reported: @escaping @MainActor () -> Void
        ) {
            self.server = server
            self.itemId = itemId
            self.name = name
            self.series = series
            self.context = context
            self.reported = reported
            super.init(nibName: nil, bundle: nil)
            title = MavioStrings.issuesReportTitle(name: name)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private var theme: Theme { context.theme }

        override func loadView() {
            let theme = theme
            var views: [NSView] = [
                NSTextField.label(
                    MavioStrings.issuesReportTitle(name: name), theme.fonts.title3, color: theme.colors.textPrimary,
                    lines: 0),
                caption(MavioStrings.issuesType),
            ]
            kind.segmentCount = types.count
            for (i, t) in types.enumerated() {
                kind.setLabel(t.title, forSegment: i)
            }
            kind.selectedSegment = 0
            kind.segmentDistribution = .fillEqually
            kind.translatesAutoresizingMaskIntoConstraints = false
            views.append(kind)
            if series {
                views.append(number(MavioStrings.issuesSeason, field: season, stepper: seasonStepper, max: 999))
                views.append(number(MavioStrings.issuesEpisode, field: episode, stepper: episodeStepper, max: 9999))
                let note = NSTextField.label(
                    MavioStrings.issuesAllNumbers, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0)
                note.preferredMaxLayoutWidth = 432
                views.append(note)
                updateEpisode()
            }
            views.append(caption(MavioStrings.issuesMessage))
            message.isRichText = false
            message.font = theme.fonts.body.font
            message.textContainerInset = NSSize(width: 4, height: 6)
            message.delegate = self
            let scroll = NSScrollView()
            scroll.documentView = message
            scroll.hasVerticalScroller = true
            scroll.borderType = .bezelBorder
            scroll.translatesAutoresizingMaskIntoConstraints = false
            message.autoresizingMask = [.width]
            message.isVerticallyResizable = true
            message.minSize = NSSize(width: 0, height: 0)
            message.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: .greatestFiniteMagnitude)
            message.textContainer?.widthTracksTextView = true
            scroll.size(height: 120)
            views.append(scroll)
            failure.font = theme.fonts.footnote.font
            failure.textColor = theme.colors.danger
            failure.isHidden = true
            failure.preferredMaxLayoutWidth = 432
            failure.translatesAutoresizingMaskIntoConstraints = false
            views.append(failure)
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                guard let self else { return }
                self.presentingViewController?.dismiss(self)
            }
            let send = MavioButton(MavioStrings.issuesSend, kind: .primary, theme: theme) { [weak self] in
                self?.submit()
            }
            send.isEnabled = false
            self.send = send
            let buttons = NSStackView.horizontal([Spacer(), cancel, send], spacing: MavioSpacing.s12)
            views.append(buttons)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            for v in views where v !== views[0] { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let root = NSView()
            root.addFilling(stack, insets: .all(MavioSpacing.s24))
            root.widthAnchor.constraint(equalToConstant: 480).isActive = true
            view = root
        }

        private func caption(_ text: String) -> NSTextField {
            label(text, theme.fonts.footnote, weight: .semibold, color: theme.colors.textSecondary)
        }

        private func number(_ title: String, field: NSTextField, stepper: NSStepper, max: Double) -> NSView {
            stepper.minValue = 0
            stepper.maxValue = max
            stepper.increment = 1
            stepper.valueWraps = false
            stepper.integerValue = 0
            stepper.target = self
            stepper.action = #selector(stepped(_:))
            field.integerValue = 0
            field.alignment = .right
            field.formatter = {
                let f = NumberFormatter()
                f.minimum = 0
                f.maximum = NSNumber(value: max)
                f.allowsFloats = false
                return f
            }()
            field.target = self
            field.action = #selector(typed(_:))
            field.translatesAutoresizingMaskIntoConstraints = false
            field.size(width: 64)
            stepper.translatesAutoresizingMaskIntoConstraints = false
            return NSStackView.horizontal(
                [
                    NSTextField.label(title, theme.fonts.body, color: theme.colors.textPrimary), Spacer(), field,
                    stepper,
                ],
                spacing: MavioSpacing.s8)
        }

        @objc private func stepped(_ sender: NSStepper) {
            (sender === seasonStepper ? season : episode).integerValue = sender.integerValue
            updateEpisode()
        }

        @objc private func typed(_ sender: NSTextField) {
            (sender === season ? seasonStepper : episodeStepper).integerValue = sender.integerValue
            updateEpisode()
        }

        /// An episode is chosen only within a season: with all seasons it
        /// shows 0 and cannot change.
        private func updateEpisode() {
            let all = seasonStepper.integerValue == 0
            episode.isEnabled = !all
            episodeStepper.isEnabled = !all
            if all { episode.integerValue = 0 }
        }

        private var text: String { message.string.trimmingCharacters(in: .whitespacesAndNewlines) }

        private func submit() {
            guard !text.isEmpty, task == nil else { return }
            var request = Mavio_Acquisition_V1_CreateIssueRequest()
            request.itemID = itemId
            request.type = types[max(0, kind.selectedSegment)]
            let seasonNumber = seasonStepper.integerValue
            request.seasonNumber = series ? Int32(seasonNumber) : 0
            request.episodeNumber = series && seasonNumber > 0 ? Int32(episodeStepper.integerValue) : 0
            request.message = text
            send?.isEnabled = false
            let model = context.model
            let server = server
            task = Task { [weak self] in
                do {
                    _ = try await model.call(
                        server, Procedure.issues("CreateIssue"), request,
                        as: Mavio_Acquisition_V1_CreateIssueResponse.self)
                    guard let self else { return }
                    self.reported()
                    self.presentingViewController?.dismiss(self)
                } catch {
                    guard let self else { return }
                    self.failure.stringValue = error.reason
                    self.failure.isHidden = false
                    self.task = nil
                    self.send?.isEnabled = !self.text.isEmpty
                }
            }
        }

        isolated deinit { task?.cancel() }
    }

    extension ReportIssueSheet: NSTextViewDelegate {
        func textDidChange(_ notification: Notification) {
            send?.isEnabled = !text.isEmpty && task == nil
        }
    }
#endif
