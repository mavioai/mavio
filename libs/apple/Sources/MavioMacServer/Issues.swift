#if os(macOS)
    import AppKit
    import MavioMac

    extension Mavio_Acquisition_V1_IssueType {
        @MainActor fileprivate var title: String {
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
        @MainActor fileprivate var numbers: String? {
            guard seasonNumber > 0 else { return nil }
            return episodeNumber > 0
                ? String(format: "S%02dE%02d", seasonNumber, episodeNumber)
                : MavioStrings.discoverSeason(number: "\(seasonNumber)")
        }
    }

    /// The names of items and users issues name, read once, telling the
    /// pages showing them as they come.
    @MainActor final class IssueNames {
        private(set) var items: [String: String] = [:]
        private(set) var users: [String: String] = [:]
        private var asked: Set<String> = []
        /// Called as names come.
        var onChange: (@MainActor () -> Void)?

        /// Reads an item's name, once.
        func item(_ id: String, _ model: AppModel, _ server: ServerRecord) async {
            guard !id.isEmpty, asked.insert(id).inserted else { return }
            var request = Mavio_Library_V1_GetItemRequest()
            request.id = id
            if let r = try? await model.call(
                server, Procedure.items("GetItem"), request, as: Mavio_Library_V1_GetItemResponse.self)
            {
                items[id] = r.item.name
                onChange?()
            }
        }

        /// Everyone's names for an administrator.
        func loadUsers(_ model: AppModel, _ server: ServerRecord) async {
            guard asked.insert("users").inserted else { return }
            if let r = try? await model.call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self)
            {
                users = Dictionary(r.users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
                onChange?()
            }
        }

        /// A user's name: the user's own, everyone's for an administrator,
        /// else "Administrator".
        func user(_ id: String, _ server: ServerRecord) -> String {
            id == server.userId ? server.userName : users[id] ?? MavioStrings.issuesAdministrator
        }
    }

    /// An issue's status, as a badge.
    @MainActor private func statusBadge(_ status: Mavio_Acquisition_V1_IssueStatus, theme: Theme) -> NSView {
        let open = status == .open
        let label = badge(
            open ? MavioStrings.issuesOpen : MavioStrings.issuesResolved, theme.fonts.caption,
            color: open ? theme.colors.onButton : .white, fill: open ? theme.colors.button : NSColor(hex: 0x34C759),
            radius: 999, horizontal: MavioSpacing.s8, vertical: 2)
        if let text = label.subviews.first as? NSTextField {
            text.font = .systemFont(ofSize: theme.fonts.caption.size, weight: .semibold)
        }
        return label
    }

    /// Issues: the problems reported with the server's items — the user's
    /// own, or everyone's for its administrators — open ones at first.
    final class IssuesPage: ServerPage {
        private var issues: [Mavio_Acquisition_V1_Issue]?
        private var total: Int32 = 0
        private var failure: String?
        private var filter = Filter.open
        private var loading: Task<Void, Never>?
        private var started = false
        private let names = IssueNames()
        private lazy var form = FormPage(title: MavioStrings.settingsIssues, theme: theme)
        private lazy var filterSection = section(nil, footer: MavioStrings.issuesIntro)
        private lazy var listSection = section()

        private enum Filter: CaseIterable {
            case open, resolved, all

            var statuses: [Mavio_Acquisition_V1_IssueStatus] {
                switch self {
                case .open: [.open]
                case .resolved: [.resolved]
                case .all: []
                }
            }

            @MainActor var title: String {
                switch self {
                case .open: MavioStrings.issuesOpen
                case .resolved: MavioStrings.issuesResolved
                case .all: MavioStrings.issuesAll
                }
            }
        }

        private static let page: Int32 = 50

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsIssues
            let filters = Filter.allCases
            let tabs = PillTabs(filters.map(\.title), theme: theme)
            tabs.setAccessibilityLabel(MavioStrings.issuesFilter)
            tabs.onChange = { [weak self] i in
                guard let self else { return }
                filter = filters[i]
                issues = nil
                render()
                reload(offset: 0)
            }
            // The section's card would frame the tabs, which stand on their
            // own as the SwiftUI screen's segmented picker does.
            let holder = NSStackView.vertical([tabs, filterSection], spacing: MavioSpacing.s8)
            filterSection.widthAnchor.constraint(equalTo: holder.widthAnchor).isActive = true
            form.add(holder)
            form.add(listSection)
            names.onChange = { [weak self] in self?.render() }
            render()
            show(form)
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in
                guard let self else { return }
                await names.loadUsers(model, server)
            }
            reload(offset: 0)
        }

        private func reload(offset: Int32) {
            loading?.cancel()
            loading = Task { [weak self] in await self?.load(offset: offset) }
        }

        private func render() {
            var rows: [NSView] = (issues ?? []).map(row)
            if let failure {
                rows.append(note(failure, failed: true))
            } else if let issues, issues.isEmpty {
                rows.append(note(MavioStrings.issuesNone))
            } else if issues == nil {
                rows.append(padded(LoadingView()))
            }
            // More as the list's end shows, which a row to choose stands for.
            if let issues, issues.count < total {
                rows.append(
                    FormRow.action(MavioStrings.activityShowMore, theme: theme) { [weak self] in
                        self?.reload(offset: Int32(issues.count))
                    })
            }
            listSection.setRows(rows)
            listSection.footer =
                issues.map { $0.isEmpty ? nil : MavioStrings.issuesCount(count: Int(total)) } ?? nil
            refocus()
        }

        /// An issue in a list: its item, kind, numbers and status, what it
        /// says, who reported it and when it last changed.
        private func row(_ issue: Mavio_Acquisition_V1_Issue) -> NSView {
            run { [weak self] in
                guard let self else { return }
                await names.item(issue.itemID, model, server)
            }
            let title = NSTextField.label(
                [names.items[issue.itemID], issue.type.title, issue.numbers].compactMap(\.self)
                    .filter { !$0.isEmpty }.joined(separator: " · "),
                theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary)
            title.lineBreakMode = .byTruncatingTail
            let detail = NSTextField.label(
                [
                    issue.comments.first?.message, names.user(issue.createdBy, server),
                    issue.updateTime.time.map(fromNow),
                ]
                .compactMap(\.self).filter { !$0.isEmpty }.joined(separator: " · "),
                theme.fonts.footnote, color: theme.colors.textSecondary)
            detail.lineBreakMode = .byTruncatingTail
            return ContentRow(
                NSStackView.vertical([title, detail], spacing: 2), accessory: statusBadge(issue.status, theme: theme),
                theme: theme
            ) { [weak self] in
                guard let self else { return }
                push(
                    IssuePage(context: context, server: server, id: issue.id, names: names) { [weak self] changed in
                        self?.replace(changed, id: issue.id)
                    })
            }
        }

        /// An issue changed on its page, or deleted when nil.
        private func replace(_ changed: Mavio_Acquisition_V1_Issue?, id: String) {
            guard let i = issues?.firstIndex(where: { $0.id == id }) else { return }
            if let changed { issues?[i] = changed } else { issues?.remove(at: i) }
            render()
        }

        private func load(offset: Int32) async {
            var request = Mavio_Acquisition_V1_ListIssuesRequest()
            request.statuses = filter.statuses
            request.sort = .modified
            request.limit = Self.page
            request.offset = offset
            do {
                let r = try await model.call(
                    server, Procedure.issues("ListIssues"), request, as: Mavio_Acquisition_V1_ListIssuesResponse.self)
                issues = offset == 0 ? r.issues : (issues ?? []) + r.issues
                total = r.total
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        isolated deinit {
            loading?.cancel()
        }
    }

    /// An issue: its item, kind and status, its comments, a comment to add,
    /// and resolving, reopening or deleting it.
    final class IssuePage: ServerPage {
        private let id: String
        private let names: IssueNames
        /// Told the issue as it changed, or nil once deleted.
        private let changed: @MainActor (Mavio_Acquisition_V1_Issue?) -> Void
        private var issue: Mavio_Acquisition_V1_Issue?
        private var failure: String?
        private var comment = ""
        private var busy = false
        private var started = false
        private lazy var form = FormPage(theme: theme)
        private lazy var input: TextInput = {
            let input = TextInput(placeholder: MavioStrings.issuesComment, lines: 6)
            input.onChange = { [weak self] text in
                self?.comment = text
                self?.sendButton.isEnabled = self?.canSend == true
            }
            return input
        }()
        private lazy var sendButton = button(MavioStrings.issuesCommentSend, kind: .primary) { [weak self] in
            self?.send()
        }

        /// The page of issue id, naming items and users with names, telling
        /// changed as it changes.
        init(
            context: MacContext, server: ServerRecord, id: String, names: IssueNames,
            changed: @escaping @MainActor (Mavio_Acquisition_V1_Issue?) -> Void
        ) {
            self.id = id
            self.names = names
            self.changed = changed
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.issuesTitle
            render()
            show(form)
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in await self?.load() }
        }

        private var canSend: Bool {
            !comment.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !busy
        }

        private func render() {
            let heading =
                issue.map { i in [i.type.title, names.items[i.itemID]].compactMap(\.self).joined(separator: " · ") }
                ?? MavioStrings.issuesTitle
            title = heading
            var views: [NSView] = [self.heading(heading)]
            if let i = issue {
                let by = NSTextField.label(
                    [i.numbers, MavioStrings.issuesReportedBy(name: names.user(i.createdBy, server))]
                        .compactMap(\.self).joined(separator: " · "), theme.fonts.footnote,
                    color: theme.colors.textSecondary, lines: 0)
                views.append(
                    section(
                        nil, footer: nil,
                        [
                            padded(
                                NSStackView.horizontal(
                                    [statusBadge(i.status, theme: theme), by, Spacer()], spacing: MavioSpacing.s8))
                        ]))
                if !i.comments.isEmpty {
                    views.append(section(nil, footer: nil, i.comments.map(commentRow)))
                }
                sendButton.isEnabled = canSend
                let frame = FocusFrame(input, theme: theme)
                views.append(
                    section(
                        nil, footer: nil,
                        [ControlRow(frame, controls: [sendButton], theme: theme)]))
                views.append(
                    section(
                        nil, footer: nil,
                        [
                            FormRow.action(
                                i.status == .open ? MavioStrings.issuesResolve : MavioStrings.issuesReopen,
                                theme: theme
                            ) { [weak self] in
                                guard let self, !busy else { return }
                                act(i.status == .open ? "ResolveIssue" : "ReopenIssue")
                            },
                            FormRow.action(MavioStrings.issuesDelete, destructive: true, theme: theme) {
                                [weak self] in
                                guard let self, !busy else { return }
                                confirm(MavioStrings.issuesConfirmDelete, button: MavioStrings.issuesDelete) {
                                    [weak self] in self?.remove()
                                }
                            },
                        ]))
            } else if failure == nil {
                views.append(padded(LoadingView()))
            }
            if let failure {
                views.append(section(nil, footer: nil, [note(failure, failed: true)]))
            }
            form.setSections(views)
            refocus()
        }

        private func commentRow(_ c: Mavio_Acquisition_V1_IssueComment) -> NSView {
            padded(
                NSStackView.vertical(
                    [
                        NSTextField.label(
                            [names.user(c.userID, server), c.createTime.time.map(fromNow)].compactMap(\.self)
                                .joined(separator: " · "), theme.fonts.footnote, color: theme.colors.textSecondary),
                        NSTextField.label(
                            c.message, theme.fonts.body, color: theme.colors.textPrimary, lines: 0, selectable: true),
                    ], spacing: 2))
        }

        private func load() async {
            var request = Mavio_Acquisition_V1_GetIssueRequest()
            request.id = id
            do {
                issue = try await model.call(
                    server, Procedure.issues("GetIssue"), request, as: Mavio_Acquisition_V1_GetIssueResponse.self
                ).issue
                failure = nil
                render()
                if let issue { await names.item(issue.itemID, model, server) }
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func send() {
            guard canSend else { return }
            let message = comment.trimmingCharacters(in: .whitespacesAndNewlines)
            var request = Mavio_Acquisition_V1_AddCommentRequest()
            request.id = id
            request.message = message
            perform { [weak self] in
                guard let self else { throw CancellationError() }
                let r = try await model.call(
                    server, Procedure.issues("AddComment"), request, as: Mavio_Acquisition_V1_AddCommentResponse.self)
                comment = ""
                input.text = ""
                return r.issue
            }
        }

        /// Resolves or reopens the issue.
        private func act(_ method: String) {
            let id = id
            perform { [weak self] in
                guard let self else { throw CancellationError() }
                if method == "ResolveIssue" {
                    var request = Mavio_Acquisition_V1_ResolveIssueRequest()
                    request.id = id
                    return try await model.call(
                        server, Procedure.issues(method), request, as: Mavio_Acquisition_V1_ResolveIssueResponse.self
                    ).issue
                }
                var request = Mavio_Acquisition_V1_ReopenIssueRequest()
                request.id = id
                return try await model.call(
                    server, Procedure.issues(method), request, as: Mavio_Acquisition_V1_ReopenIssueResponse.self
                ).issue
            }
        }

        private func remove() {
            busy = true
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Acquisition_V1_DeleteIssueRequest()
                request.id = id
                do {
                    _ = try await model.call(
                        server, Procedure.issues("DeleteIssue"), request,
                        as: Mavio_Acquisition_V1_DeleteIssueResponse.self)
                    busy = false
                    changed(nil)
                    dismiss()
                    return
                } catch {
                    failure = error.reason
                }
                busy = false
                render()
            }
        }

        private func perform(_ work: @escaping @MainActor () async throws -> Mavio_Acquisition_V1_Issue) {
            busy = true
            sendButton.isEnabled = false
            run { [weak self] in
                do {
                    let i = try await work()
                    guard let self else { return }
                    issue = i
                    changed(i)
                    failure = nil
                } catch {
                    self?.failure = error.reason
                }
                self?.busy = false
                self?.render()
            }
        }
    }
#endif
