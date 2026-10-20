#if os(macOS)
    import AppKit
    import MavioMac

    extension AppModel {
        /// The users of a server, its administrators first, then by name.
        fileprivate func users(_ server: ServerRecord) async throws -> [Mavio_User_V1_User] {
            try await call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self
            ).users.sorted {
                $0.admin != $1.admin ? $0.admin : $0.name.localizedStandardCompare($1.name) == .orderedAscending
            }
        }

        /// The libraries access is given to: not those of collections and
        /// playlists.
        fileprivate func accessibleLibraries(_ server: ServerRecord) async -> [Mavio_Library_V1_Library] {
            let r = try? await call(
                server, Procedure.libraries("ListLibraries"), Mavio_Library_V1_ListLibrariesRequest(),
                as: Mavio_Library_V1_ListLibrariesResponse.self)
            return (r?.libraries ?? []).filter { $0.kind != .collections && $0.kind != .playlists }
        }

        /// The link of an invitation's token, which opens the server's page
        /// accepting it.
        fileprivate func invitationLink(_ server: ServerRecord, _ token: String) -> String {
            var base = server.address
            while base.hasSuffix("/") { base.removeLast() }
            let encoded = token.addingPercentEncoding(
                withAllowedCharacters: .alphanumerics.union(.init(charactersIn: "-_.!~*'()")))
            return "\(base)/invite#\(encoded ?? token)"
        }
    }

    /// The restriction profiles a managed user may have.
    @MainActor private var restrictions: [(String, Mavio_User_V1_RestrictionProfile)] {
        [
            (MavioStrings.usersRestrictionNone, .unspecified),
            (MavioStrings.usersRestrictionYoungerChild, .youngerChild),
            (MavioStrings.usersRestrictionOlderChild, .olderChild),
            (MavioStrings.usersRestrictionTeen, .teen),
        ]
    }

    /// The badges of a user: administrator, managed, disabled.
    @MainActor private func badges(_ user: Mavio_User_V1_User, theme: Theme) -> [NSView] {
        [
            user.admin ? MavioStrings.usersAdmin : nil, user.managed ? MavioStrings.usersManaged : nil,
            user.disabled ? MavioStrings.usersDisabled : nil,
        ]
        .compactMap { $0 }
        .map { badge($0, theme.fonts.caption, color: theme.colors.textSecondary, fill: theme.colors.fill) }
    }

    /// A row of a library that is chosen or not, a check at its end when it
    /// is; choosing it flips it, unless it cannot be changed.
    @MainActor private func checkRow(
        _ title: String, on: Bool, enabled: Bool, theme: Theme, flip: @escaping @MainActor () -> Void
    ) -> NSView {
        let check = IconView("check", size: 16, color: theme.colors.accentText)
        check.isHidden = !on
        let row = ContentRow(
            NSTextField.label(
                title, theme.isTV ? theme.fonts.headline : theme.fonts.body,
                color: enabled ? theme.colors.textPrimary : theme.colors.textDisabled),
            accessory: check, chevron: false, theme: theme, action: enabled ? flip : nil)
        row.setAccessibilityLabel(title)
        row.setAccessibilityValue(on ? MavioStrings.usersOn : MavioStrings.usersOff)
        return row
    }

    /// A field of a four-digit PIN.
    @MainActor private func pinField(
        _ pin: String, theme: Theme, onChange: @escaping @MainActor (String) -> Void
    ) -> (FocusFrame, TextInput) {
        let input = TextInput(pin, placeholder: "0000")
        input.field.alignment = .right
        input.size(width: 80)
        input.onChange = { [weak input] text in
            let digits = String(text.filter(\.isNumber).prefix(4))
            if digits != text { input?.text = digits }
            onChange(digits)
        }
        return (FocusFrame(input, theme: theme), input)
    }

    /// Buttons copying a link, telling once it is copied, and sharing it.
    @MainActor private func copyLink(_ link: String, theme: Theme) -> NSView {
        let copy = MavioButton(MavioStrings.usersCopyLink, theme: theme)
        copy.onPress = { [weak copy] in
            copyToClipboard(link)
            copy?.title = MavioStrings.usersCopied
        }
        var views: [NSView] = [copy]
        if !theme.isTV, let url = URL(string: link) {
            let share = MavioButton(icon: "external", theme: theme)
            share.onPress = { [weak share] in
                guard let share else { return }
                NSSharingServicePicker(items: [url]).show(relativeTo: share.bounds, of: share, preferredEdge: .minY)
            }
            views.append(share)
        }
        return NSStackView.horizontal(views + [Spacer()], spacing: MavioSpacing.s8)
    }

    // MARK: Users

    /// Users (docs/clients.md §6.3.5): the users of the server and its
    /// invitation links, and a user's page: its name, whether it is
    /// disabled, its restriction profile, the libraries and ratings it may
    /// see, how it may play, and its sign-ins. In mavio.dev mode a user's
    /// name, access and state are mavio.dev's, and the server's own
    /// credentials and invitations are not used.
    final class UsersPage: ServerPage {
        private var users: [Mavio_User_V1_User]?
        private var failure: String?
        private var standalone = false
        private lazy var form = FormPage(title: MavioStrings.settingsUsers, theme: theme)
        private lazy var message = NSTextField.label(
            "", theme.fonts.callout, color: theme.colors.textSecondary, lines: 0)
        private lazy var list = section()
        private lazy var invitations = Invitations(page: self)

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsUsers
            form.add(message)
            form.add(list)
            form.add(invitations.section)
            form.add(invitations.conversions)
            message.isHidden = true
            invitations.section.isHidden = true
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func reload() {
            run { [weak self] in await self?.load() }
        }

        private func load() async {
            do {
                async let info = try? model.call(
                    server, Procedure.auth("GetAuthInfo"), Mavio_Auth_V1_GetAuthInfoRequest(),
                    as: Mavio_Auth_V1_GetAuthInfoResponse.self)
                users = try await model.users(server)
                standalone = await info?.mode != .mavioDev
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
            if standalone { await invitations.load() }
        }

        private func render() {
            guard let users else {
                if let failure {
                    showError(failure) { [weak self] in
                        self?.show(LoadingView())
                        self?.reload()
                    }
                }
                return
            }
            message.stringValue = standalone ? MavioStrings.usersMessage : MavioStrings.usersMessageMavioDev
            message.isHidden = false
            var rows: [NSView] = users.map(row)
            if standalone {
                rows.append(
                    FormRow.link(MavioStrings.usersAdd, icon: "add", tint: theme.colors.accent, theme: theme) {
                        [weak self] in
                        guard let self else { return }
                        push(NewUserPage(context: context, server: server) { [weak self] in self?.reload() })
                    })
            }
            if let failure { rows.append(note(failure, failed: true)) }
            list.setRows(rows)
            invitations.users = users
            invitations.section.isHidden = !standalone
            invitations.conversions.isHidden = !standalone
            show(form)
            refocus()
        }

        private func row(_ u: Mavio_User_V1_User) -> NSView {
            let name = NSTextField.label(
                u.name, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary)
            name.lineBreakMode = .byTruncatingTail
            let detail = NSTextField.label(
                u.hasLastLoginTime
                    ? MavioStrings.usersLastSignIn(date: day(u.lastLoginTime)) : MavioStrings.usersNeverSignedIn,
                theme.fonts.footnote, color: theme.colors.textSecondary)
            let lines = NSStackView.vertical(
                [NSStackView.horizontal([name] + badges(u, theme: theme), spacing: MavioSpacing.s8), detail], spacing: 2
            )
            let content = NSStackView.horizontal(
                [AvatarView(name: u.name, size: 32, theme: theme), lines], spacing: MavioSpacing.s12)
            let row = ContentRow(content, theme: theme) { [weak self] in
                guard let self else { return }
                push(
                    UserPage(
                        context: context, server: server, user: u, standalone: standalone,
                        changed: { [weak self] in self?.reload() }))
            }
            row.setAccessibilityLabel(u.name)
            return row
        }
    }

    // MARK: New user

    /// Adds a member with a password, or a managed user with a PIN and a
    /// restriction profile; then shows its page.
    final class NewUserPage: ServerPage {
        private let changed: @MainActor () -> Void
        private var managed = false
        private var name = ""
        private var password = ""
        private var pin = ""
        private var restriction = Mavio_User_V1_RestrictionProfile.unspecified
        private var saving = false
        private var failure: String?
        private lazy var form = FormPage(theme: theme)
        private lazy var createButton = saveButton(MavioStrings.usersCreate) { [weak self] in self?.create() }

        /// The page adding a user to server; changed is called once it is
        /// added.
        init(context: MacContext, server: ServerRecord, changed: @escaping @MainActor () -> Void) {
            self.changed = changed
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.usersAdd
            rebuild()
            show(form)
        }

        private var valid: Bool {
            !name.trimmingCharacters(in: .whitespaces).isEmpty
                && (managed ? pin.isEmpty || pin.wholeMatch(of: /\d{4}/) != nil : !password.isEmpty)
        }

        private func refresh() {
            createButton.isEnabled = valid && !saving
            createButton.title = saving ? "…" : MavioStrings.usersCreate
        }

        private func rebuild() {
            var rows: [NSView] = [
                choiceRow(
                    MavioStrings.usersKind, nil,
                    [(MavioStrings.usersMember, false), (MavioStrings.usersManaged, true)],
                    managed
                ) { [weak self] v in
                    self?.endEditing()
                    self?.managed = v
                    self?.rebuild()
                },
                fieldRow(MavioStrings.usersName, text: name) { [weak self] v in
                    self?.name = v
                    self?.refresh()
                },
            ]
            if managed {
                let (frame, _) = pinField(pin, theme: theme) { [weak self] v in
                    self?.pin = v
                    self?.refresh()
                }
                rows.append(
                    ControlRow(
                        text(MavioStrings.usersPin, MavioStrings.usersPinDetail), controls: [frame], theme: theme))
                rows.append(
                    choiceRow(
                        MavioStrings.usersRestriction, MavioStrings.usersRestrictionDetail, restrictions, restriction
                    ) { [weak self] v in self?.restriction = v })
            } else {
                rows.append(
                    fieldRow(MavioStrings.authPassword, text: password, secure: true) { [weak self] v in
                        self?.password = v
                        self?.refresh()
                    })
            }
            var views: [NSView] = [
                heading(MavioStrings.usersAdd, actions: [createButton]),
                section(nil, footer: managed ? MavioStrings.usersManagedFooter : MavioStrings.usersMemberFooter, rows),
            ]
            if let failure { views.append(section(nil, [note(failure, failed: true)])) }
            form.setSections(views)
            refresh()
            refocus()
        }

        private func create() {
            endEditing()
            saving = true
            failure = nil
            refresh()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_User_V1_CreateUserRequest()
                request.name = name.trimmingCharacters(in: .whitespaces)
                if managed {
                    request.managed = true
                    request.restriction = restriction
                    if !pin.isEmpty { request.pin = pin }
                } else {
                    request.password = password
                }
                do {
                    let user = try await model.call(
                        server, Procedure.users("CreateUser"), request, as: Mavio_User_V1_CreateUserResponse.self
                    ).user
                    changed()
                    // The user's page takes this one's place.
                    let page = UserPage(
                        context: context, server: server, user: user, standalone: true, changed: changed)
                    if let navigation {
                        navigation.pop(animated: false)
                        navigation.push(page, animated: false)
                    }
                } catch {
                    failure = error.reason
                    saving = false
                    rebuild()
                }
            }
        }
    }

    // MARK: User

    /// A user as its page edits it.
    private struct UserDraft: Equatable {
        var name: String
        var disabled: Bool
        var restriction: Mavio_User_V1_RestrictionProfile
        var allLibraries: Bool
        var libraryIds: [String]
        /// The highest rating score allowed; nil when unrestricted.
        var maxRating: Int32?
        var blockUnrated: Bool
        var allowedLabels: String
        var blockedLabels: String
        var allowDownload: Bool
        var allowTranscoding: Bool
        var maxBitrate: Int64
        var maxSessions: Int32
        var reportIssues: Bool

        init(_ u: Mavio_User_V1_User) {
            let p = u.policy
            name = u.name
            disabled = u.disabled
            restriction = u.restriction
            allLibraries = p.allLibraries
            libraryIds = p.libraryIds
            maxRating = p.hasMaxParentalRating ? p.maxParentalRating : nil
            blockUnrated = p.blockUnrated
            allowedLabels = p.allowedLabels.joined(separator: ", ")
            blockedLabels = p.blockedLabels.joined(separator: ", ")
            allowDownload = p.allowDownload
            allowTranscoding = p.allowTranscoding
            maxBitrate = p.maxStreamingBitrate
            maxSessions = p.maxSessions
            reportIssues = p.reportIssues
        }

        private static func labels(_ s: String) -> [String] {
            var seen = Set<String>()
            return s.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter {
                !$0.isEmpty && seen.insert($0).inserted
            }
        }

        /// The policy of the draft, keeping what the page does not show of
        /// the user's.
        func policy(keeping p: Mavio_User_V1_UserPolicy) -> Mavio_User_V1_UserPolicy {
            var q = p
            q.allLibraries = allLibraries
            q.libraryIds = allLibraries ? [] : libraryIds
            if let maxRating { q.maxParentalRating = maxRating } else { q.clearMaxParentalRating() }
            q.blockUnrated = blockUnrated
            q.allowedLabels = Self.labels(allowedLabels)
            q.blockedLabels = Self.labels(blockedLabels)
            q.allowDownload = allowDownload
            q.allowTranscoding = allowTranscoding
            q.maxStreamingBitrate = maxBitrate
            q.maxSessions = maxSessions
            q.reportIssues = reportIssues
            return q
        }
    }

    /// A user's page.
    final class UserPage: ServerPage {
        private var user: Mavio_User_V1_User
        /// Whether the server keeps its own accounts rather than mavio.dev's.
        private let standalone: Bool
        private let changed: @MainActor () -> Void
        private var draft: UserDraft
        private var libraries: [Mavio_Library_V1_Library] = []
        private var ratings: [Mavio_System_V1_Rating] = []
        private var saving = false
        private var notice: String?
        private var failure: String?
        private var started = false
        private var noticeSection: FormSection?
        private lazy var signIns = SignIns(page: self)
        private lazy var form = FormPage(theme: theme)
        private lazy var save = saveButton { [weak self] in self?.saveTapped() }

        private static let mbps: Int64 = 1_000_000
        private static let bitrates: [Int64] = [0, 120, 80, 40, 20, 10, 8, 4, 2, 1].map { $0 * mbps }
        /// The rating score of adult content.
        private static let adult: Int32 = 1000

        /// The page of user of server; changed is called once it is changed
        /// or deleted.
        init(
            context: MacContext, server: ServerRecord, user: Mavio_User_V1_User, standalone: Bool,
            changed: @escaping @MainActor () -> Void
        ) {
            self.user = user
            self.standalone = standalone
            self.changed = changed
            draft = UserDraft(user)
            super.init(context: context, server: server)
        }

        private var saved: UserDraft { UserDraft(user) }
        private var valid: Bool { !draft.name.trimmingCharacters(in: .whitespaces).isEmpty }
        private var deletable: Bool { !user.admin && (standalone || user.disabled) }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = user.name
            rebuild()
            show(form)
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in
                guard let self else { return }
                async let l = model.accessibleLibraries(server)
                let r = try? await model.call(
                    server, Procedure.localization("ListRatings"), Mavio_System_V1_ListRatingsRequest(),
                    as: Mavio_System_V1_ListRatingsResponse.self)
                ratings = r?.ratings ?? []
                libraries = await l
                endEditing()
                rebuild()
            }
        }

        /// The user, as the sign-ins changed it.
        func userChanged(_ user: Mavio_User_V1_User) {
            self.user = user
            changed()
            rebuild()
        }

        /// The user whose page this is.
        var shownUser: Mavio_User_V1_User { user }

        /// Calls back that the user changed.
        func tellChanged() { changed() }

        private func edit(_ change: (inout UserDraft) -> Void) {
            change(&draft)
            refresh()
        }

        private func choose(_ change: (inout UserDraft) -> Void) {
            change(&draft)
            rebuild()
        }

        private func refresh() {
            save.isEnabled = draft != saved && valid && !saving
            save.title = saving ? "…" : MavioStrings.actionSave
            var rows: [NSView] = []
            if let notice, draft == saved { rows.append(note(notice)) }
            if let failure { rows.append(note(failure, failed: true)) }
            noticeSection?.setRows(rows)
            noticeSection?.isHidden = rows.isEmpty
        }

        private func rebuild() {
            let title = NSTextField.label(user.name, theme.fonts.title3, color: theme.colors.textPrimary)
            let head = NSStackView.horizontal(
                [AvatarView(name: user.name, size: 40, theme: theme), title] + badges(user, theme: theme) + [
                    Spacer(), save,
                ], spacing: MavioSpacing.s12)
            var views: [NSView] = [head]
            var main: [NSView] = []
            // The account's name, state and access are mavio.dev's in its mode.
            if standalone {
                main.append(
                    fieldRow(MavioStrings.usersName, text: draft.name) { [weak self] v in self?.edit { $0.name = v } })
            } else {
                main.append(valueRow(MavioStrings.usersName, user.name))
            }
            if !user.admin {
                main.append(
                    toggle(MavioStrings.usersDisable, MavioStrings.usersDisableDetail, draft.disabled, own: standalone)
                    {
                        $0.disabled = $1
                    })
            }
            if user.managed {
                main.append(
                    choiceRow(
                        MavioStrings.usersRestriction, MavioStrings.usersRestrictionDetail, restrictions,
                        draft.restriction
                    ) { [weak self] v in self?.edit { $0.restriction = v } })
            }
            views.append(section(nil, footer: standalone ? nil : MavioStrings.usersMavioDevNote, main))
            if !user.admin { views.append(access()) }
            views.append(
                section(
                    MavioStrings.usersPlayback,
                    [
                        toggle(
                            MavioStrings.usersAllowTranscoding, MavioStrings.usersAllowTranscodingDetail,
                            draft.allowTranscoding
                        ) { $0.allowTranscoding = $1 },
                        choiceRow(
                            MavioStrings.usersMaxBitrate, nil, bitrates.map { (Self.bitrateTitle($0), $0) },
                            draft.maxBitrate
                        ) { [weak self] v in self?.choose { $0.maxBitrate = v } },
                        numberRow(
                            MavioStrings.usersMaxSessions, MavioStrings.usersMaxSessionsDetail,
                            value: Double(draft.maxSessions), range: 0...100
                        ) { [weak self] v in
                            self?.edit { $0.maxSessions = v.map { Int32(clamping: Int64($0.rounded())) } ?? -1 }
                        },
                    ]))
            if !user.admin {
                views.append(
                    section(
                        nil, [toggle(MavioStrings.usersReportIssues, nil, draft.reportIssues) { $0.reportIssues = $1 }])
                )
            }
            if standalone {
                signIns.render()
                views.append(signIns.section)
            }
            if deletable {
                views.append(
                    section(
                        nil,
                        [
                            FormRow.action(MavioStrings.actionDelete, destructive: true, theme: theme) {
                                [weak self] in self?.confirmDelete()
                            }
                        ]))
            }
            let noticeSection = section()
            self.noticeSection = noticeSection
            views.append(noticeSection)
            form.setSections(views)
            refresh()
            refocus()
        }

        private func access() -> NSView {
            var rows: [NSView] = [
                toggle(MavioStrings.usersAllLibraries, nil, draft.allLibraries, own: standalone, rebuilds: true) {
                    $0.allLibraries = $1
                }
            ]
            if !draft.allLibraries {
                for l in libraries {
                    let on = draft.libraryIds.contains(l.id)
                    rows.append(
                        checkRow(l.name, on: on, enabled: standalone, theme: theme) { [weak self] in
                            self?.choose {
                                if on { $0.libraryIds.removeAll { $0 == l.id } } else { $0.libraryIds.append(l.id) }
                            }
                        })
                }
            }
            let ratingOptions: [(String, Int32?)] =
                [(MavioStrings.usersNoLimit, nil)] + ratingScores.map { (ratingTitle($0), $0) }
            if standalone {
                rows.append(
                    choiceRow(
                        MavioStrings.usersMaxRating, MavioStrings.usersMaxRatingDetail, ratingOptions, draft.maxRating
                    ) { [weak self] v in self?.edit { $0.maxRating = v } })
            } else {
                rows.append(
                    valueRow(
                        MavioStrings.usersMaxRating, draft.maxRating.map(ratingTitle) ?? MavioStrings.usersNoLimit,
                        detail: MavioStrings.usersMaxRatingDetail))
            }
            rows.append(
                toggle(MavioStrings.usersBlockUnrated, nil, draft.blockUnrated, own: standalone) {
                    $0.blockUnrated = $1
                })
            rows.append(
                labelsRow(MavioStrings.usersAllowedLabels, MavioStrings.usersAllowedLabelsDetail, draft.allowedLabels) {
                    $0.allowedLabels = $1
                })
            rows.append(
                labelsRow(MavioStrings.usersBlockedLabels, MavioStrings.usersBlockedLabelsDetail, draft.blockedLabels) {
                    $0.blockedLabels = $1
                })
            rows.append(
                toggle(MavioStrings.usersAllowDownload, nil, draft.allowDownload, own: standalone) {
                    $0.allowDownload = $1
                })
            return section(MavioStrings.usersAccess, footer: standalone ? nil : MavioStrings.usersAccessMavioDev, rows)
        }

        private func labelsRow(
            _ label: String, _ detail: String, _ text: String, _ set: @escaping (inout UserDraft, String) -> Void
        ) -> NSView {
            let (frame, input) = field(text, width: nil) { [weak self] v in self?.edit { set(&$0, v) } }
            input.field.isEditable = standalone
            input.field.isEnabled = standalone
            let stack = NSStackView.vertical([self.text(label, detail), frame], spacing: MavioSpacing.s8)
            frame.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return ControlRow(stack, theme: theme)
        }

        /// Ratings by the age they stand for; those above adult content are
        /// left to no limit.
        private var ratingScores: [Int32] {
            var scores = Set(ratings.map(\.score).filter { $0 <= Self.adult })
            if let r = draft.maxRating { scores.insert(r) }
            return scores.sorted()
        }

        /// A rating's age, a few of its names beside it.
        private func ratingTitle(_ s: Int32) -> String {
            let names = ratings.filter { $0.score == s }.map(\.name)
            let age =
                s == 0
                ? MavioStrings.usersRatingAll
                : s >= Self.adult ? MavioStrings.usersRatingAdult : MavioStrings.usersRatingAge(age: String(s))
            return !names.isEmpty && names.count <= 3 ? "\(age) (\(names.joined(separator: ", ")))" : age
        }

        private var bitrates: [Int64] {
            Self.bitrates.contains(draft.maxBitrate) ? Self.bitrates : Self.bitrates + [draft.maxBitrate]
        }

        private static func bitrateTitle(_ b: Int64) -> String {
            b == 0 ? MavioStrings.usersUnlimited : MavioStrings.usersMbps(mbps: String(b / mbps))
        }

        /// A switch, or its state when the user's setting is not the
        /// server's own.
        private func toggle(
            _ label: String, _ detail: String?, _ on: Bool, own: Bool = true, rebuilds: Bool = false,
            _ set: @escaping (inout UserDraft, Bool) -> Void
        ) -> FormRow {
            guard own else {
                return valueRow(label, on ? MavioStrings.usersOn : MavioStrings.usersOff, detail: detail)
            }
            return toggleRow(label, detail, on) { [weak self] v in
                if rebuilds { self?.choose { set(&$0, v) } } else { self?.edit { set(&$0, v) } }
            }
        }

        private func saveTapped() {
            endEditing()
            saving = true
            failure = nil
            refresh()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_User_V1_UpdateUserRequest()
                request.id = user.id
                request.name = draft.name.trimmingCharacters(in: .whitespaces)
                request.disabled = draft.disabled
                request.restriction = draft.restriction
                request.policy = draft.policy(keeping: user.policy)
                do {
                    user = try await model.call(
                        server, Procedure.users("UpdateUser"), request, as: Mavio_User_V1_UpdateUserResponse.self
                    ).user
                    draft = saved
                    notice = MavioStrings.usersSaved
                    title = user.name
                    changed()
                } catch {
                    failure = error.reason
                }
                saving = false
                rebuild()
            }
        }

        private func confirmDelete() {
            confirm(
                MavioStrings.usersDeleteTitle(name: user.name), message: MavioStrings.usersDeleteMessage,
                button: MavioStrings.actionDelete
            ) { [weak self] in self?.delete() }
        }

        private func delete() {
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_User_V1_DeleteUserRequest()
                request.id = user.id
                do {
                    _ = try await model.call(
                        server, Procedure.users("DeleteUser"), request, as: Mavio_User_V1_DeleteUserResponse.self)
                    changed()
                    dismiss()
                } catch {
                    failure = error.reason
                    refresh()
                }
            }
        }
    }

    // MARK: Sign-ins

    /// A user's sign-ins: its password, PIN, authenticator and passkeys, and
    /// an invitation that converts a managed user into a member; a section
    /// of the user's page, rebuilt alone as they change.
    @MainActor
    private final class SignIns {
        private weak var page: UserPage?
        private var password = ""
        private var pin = ""
        private var link: String?
        private var busy = false
        private var notice: String?
        private var failure: String?
        private var work: Task<Void, Never>?
        /// The section they show in.
        let section: FormSection

        init(page: UserPage) {
            self.page = page
            section = FormSection(MavioStrings.usersSignIns, theme: page.theme)
        }

        /// Builds the section from the user.
        func render() {
            guard let page else { return }
            let theme = page.theme
            let user = page.shownUser
            var rows: [NSView] = []
            // Users change their own password, with the current one, in
            // their account's settings.
            if user.authProvider.isEmpty && user.id != page.server.userId {
                let input = TextInput(password, placeholder: MavioStrings.authPassword, secure: true)
                let set = MavioButton(MavioStrings.usersSet, theme: theme) { [weak self] in self?.setPassword() }
                set.isEnabled = !password.isEmpty && !busy
                input.onChange = { [weak self, weak set] v in
                    self?.password = v
                    set?.isEnabled = !v.isEmpty && self?.busy == false
                }
                input.onSubmit = { [weak self] in
                    guard let self, !password.isEmpty, !busy else { return }
                    setPassword()
                }
                let line = NSStackView.horizontal(
                    [FocusFrame(input, theme: theme), set], spacing: MavioSpacing.s8)
                let stack = NSStackView.vertical(
                    [
                        page.text(
                            user.managed ? MavioStrings.usersGivePassword : MavioStrings.usersSetPassword,
                            user.managed ? MavioStrings.usersGivePasswordDetail : MavioStrings.usersSetPasswordDetail),
                        line,
                    ], spacing: MavioSpacing.s8)
                line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                rows.append(ControlRow(stack, theme: theme))
            }
            if user.managed {
                let set = MavioButton(MavioStrings.usersSet, theme: theme) { [weak self] in
                    guard let self else { return }
                    setPin(pin)
                }
                set.isEnabled = pin.count == 4 && !busy
                let (frame, _) = pinField(pin, theme: theme) { [weak self, weak set] v in
                    self?.pin = v
                    set?.isEnabled = v.count == 4 && self?.busy == false
                }
                rows.append(
                    ControlRow(
                        page.text(
                            MavioStrings.usersPin, user.hasPin_p ? MavioStrings.usersPinOn : MavioStrings.usersPinOff),
                        controls: [frame, set], theme: theme))
                if user.hasPin_p {
                    rows.append(
                        FormRow.action(MavioStrings.usersClear, theme: theme) { [weak self] in
                            guard let self, !busy else { return }
                            setPin("")
                        })
                }
                let detail = NSTextField.label(
                    link ?? MavioStrings.usersInviteManagedDetail, theme.fonts.footnote,
                    color: theme.colors.textSecondary, lines: 0, selectable: link != nil)
                var invite: [NSView] = [
                    NSTextField.label(
                        MavioStrings.usersInviteManaged, theme.isTV ? theme.fonts.headline : theme.fonts.body,
                        color: theme.colors.textPrimary, lines: 0),
                    detail,
                ]
                if let link {
                    invite.append(copyLink(link, theme: theme))
                } else {
                    let make = MavioButton(MavioStrings.usersCreateLink, theme: theme) { [weak self] in self?.invite() }
                    make.isEnabled = !busy
                    invite.append(make)
                }
                rows.append(ControlRow(NSStackView.vertical(invite, spacing: MavioSpacing.s8), theme: theme))
            }
            if user.hasTotp_p {
                let off = MavioButton(MavioStrings.usersTurnOff, kind: .destructive, theme: theme) { [weak self] in
                    self?.deleteTotp()
                }
                off.isEnabled = !busy
                rows.append(
                    ControlRow(
                        page.text(MavioStrings.usersTotp, MavioStrings.usersTotpDetail), controls: [off], theme: theme))
            }
            for k in user.passkeys {
                let delete = MavioButton(MavioStrings.actionDelete, kind: .destructive, theme: theme) { [weak self] in
                    self?.deletePasskey(k.id)
                }
                delete.isEnabled = !busy
                rows.append(
                    ControlRow(
                        page.text(
                            k.name.isEmpty ? MavioStrings.usersPasskey : k.name,
                            [k.rpID, k.hasLastUseTime ? MavioStrings.usersLastUsed(date: day(k.lastUseTime)) : ""]
                                .filter { !$0.isEmpty }.joined(separator: " · ")),
                        controls: [delete], theme: theme))
            }
            section.setRows(rows)
            section.footer = failure ?? notice
            section.isHidden = rows.isEmpty && section.footer == nil
            page.refocus()
        }

        /// Runs a change of the user's sign-ins, then reloads the user.
        private func perform(_ done: String?, _ action: @escaping @MainActor () async throws -> Void) {
            guard let page else { return }
            page.endEditing()
            busy = true
            failure = nil
            render()
            let model = page.model
            let server = page.server
            let id = page.shownUser.id
            work = Task { [weak self] in
                do {
                    try await action()
                    self?.notice = done
                    var request = Mavio_User_V1_GetUserRequest()
                    request.id = id
                    let r = try? await model.call(
                        server, Procedure.users("GetUser"), request, as: Mavio_User_V1_GetUserResponse.self)
                    self?.busy = false
                    if let r { self?.page?.userChanged(r.user) } else { self?.page?.tellChanged() }
                } catch {
                    self?.failure = error.reason
                }
                self?.busy = false
                self?.render()
            }
        }

        private func setPassword() {
            guard let page else { return }
            let user = page.shownUser
            let password = password
            let model = page.model
            let server = page.server
            perform(user.managed ? MavioStrings.usersConverted : MavioStrings.usersPasswordSet) { [weak self] in
                var request = Mavio_User_V1_SetPasswordRequest()
                request.userID = user.id
                request.newPassword = password
                _ = try await model.call(
                    server, Procedure.users("SetPassword"), request, as: Mavio_User_V1_SetPasswordResponse.self)
                self?.password = ""
            }
        }

        private func setPin(_ value: String) {
            guard let page else { return }
            let id = page.shownUser.id
            let model = page.model
            let server = page.server
            perform(value.isEmpty ? MavioStrings.usersPinCleared : MavioStrings.usersPinSet) { [weak self] in
                var request = Mavio_User_V1_SetPinRequest()
                request.userID = id
                request.pin = value
                _ = try await model.call(
                    server, Procedure.users("SetPin"), request, as: Mavio_User_V1_SetPinResponse.self)
                self?.pin = ""
            }
        }

        private func deleteTotp() {
            guard let page else { return }
            let id = page.shownUser.id
            let model = page.model
            let server = page.server
            perform(nil) {
                var request = Mavio_User_V1_DeleteTotpRequest()
                request.userID = id
                _ = try await model.call(
                    server, Procedure.users("DeleteTotp"), request, as: Mavio_User_V1_DeleteTotpResponse.self)
            }
        }

        private func deletePasskey(_ key: String) {
            guard let page else { return }
            let id = page.shownUser.id
            let model = page.model
            let server = page.server
            perform(nil) {
                var request = Mavio_User_V1_DeletePasskeyRequest()
                request.userID = id
                request.id = key
                _ = try await model.call(
                    server, Procedure.users("DeletePasskey"), request, as: Mavio_User_V1_DeletePasskeyResponse.self)
            }
        }

        private func invite() {
            guard let page else { return }
            busy = true
            failure = nil
            render()
            let model = page.model
            let server = page.server
            var request = Mavio_User_V1_CreateInvitationRequest()
            request.userID = page.shownUser.id
            request.maxUses = 1
            var d = Google_Protobuf_Duration()
            d.seconds = 7 * 86400
            request.validFor = d
            work = Task { [weak self] in
                do {
                    let r = try await model.call(
                        server, Procedure.users("CreateInvitation"), request,
                        as: Mavio_User_V1_CreateInvitationResponse.self)
                    self?.link = model.invitationLink(server, r.token)
                } catch {
                    self?.failure = error.reason
                }
                self?.busy = false
                self?.render()
            }
        }

        isolated deinit {
            work?.cancel()
        }
    }

    // MARK: Invitations

    /// The server's invitation links — making one, which shows its link
    /// once, and revoking them — and the links made for managed users, each
    /// converting one into a member: two sections of the users' page.
    @MainActor
    private final class Invitations {
        private weak var page: ServerPage?
        private var invitations: [Mavio_User_V1_Invitation] = []
        private var loaded = false
        private var libraries: [Mavio_Library_V1_Library] = []
        private var making = false
        private var uses: Int32 = 1
        private var days: Int32 = 7
        private var all = true
        private var ids: [String] = []
        private var download = true
        private var made: (id: String, link: String)?
        private var busy = false
        private var failure: String?
        private var work: Task<Void, Never>?
        /// The users, to name those of conversion links.
        var users: [Mavio_User_V1_User] = []
        /// The section of the invitation links.
        let section: FormSection
        /// The section of the links converting managed users.
        let conversions: FormSection

        init(page: ServerPage) {
            self.page = page
            section = FormSection(
                MavioStrings.usersInvitations, footer: MavioStrings.usersInvitationsFooter, theme: page.theme)
            conversions = FormSection(MavioStrings.usersConversionLinks, theme: page.theme)
        }

        /// Links used up can no longer be accepted.
        private var open: [Mavio_User_V1_Invitation] { invitations.filter { $0.uses < $0.maxUses } }

        /// Reads the invitations and the libraries.
        func load() async {
            guard let page else { return }
            do {
                invitations = try await page.model.call(
                    page.server, Procedure.users("ListInvitations"), Mavio_User_V1_ListInvitationsRequest(),
                    as: Mavio_User_V1_ListInvitationsResponse.self
                ).invitations
                loaded = true
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            if libraries.isEmpty { libraries = await page.model.accessibleLibraries(page.server) }
            render()
        }

        private func render() {
            guard let page else { return }
            let theme = page.theme
            let list = open.filter(\.userID.isEmpty)
            // The link made is shown until its invitation is revoked or used up.
            let link = made.flatMap { m in list.contains { $0.id == m.id } ? m.link : nil }
            var rows: [NSView] = []
            if let link {
                let stack = NSStackView.vertical(
                    [
                        page.text(MavioStrings.usersLinkMade),
                        NSTextField.label(
                            link, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0, selectable: true),
                        copyLink(link, theme: theme),
                    ], spacing: MavioSpacing.s8)
                rows.append(ControlRow(stack, theme: theme))
            }
            if making {
                rows.append(
                    page.numberRow(MavioStrings.usersMaxUses, value: Double(uses), range: 1...100) { [weak self] v in
                        self?.uses = v.map { Int32(clamping: Int64($0.rounded())) } ?? 0
                    })
                rows.append(
                    page.numberRow(MavioStrings.usersValidDays, value: Double(days), range: 1...30) { [weak self] v in
                        self?.days = v.map { Int32(clamping: Int64($0.rounded())) } ?? 0
                    })
                rows.append(
                    page.toggleRow(MavioStrings.usersAllLibraries, nil, all) { [weak self] v in
                        self?.all = v
                        self?.render()
                    })
                if !all {
                    for l in libraries {
                        let on = ids.contains(l.id)
                        rows.append(
                            checkRow(l.name, on: on, enabled: true, theme: theme) { [weak self] in
                                guard let self else { return }
                                if on { ids.removeAll { $0 == l.id } } else { ids.append(l.id) }
                                render()
                            })
                    }
                }
                rows.append(
                    page.toggleRow(MavioStrings.usersAllowDownload, nil, download) { [weak self] v in
                        self?.download = v
                    })
                rows.append(
                    FormRow.action(MavioStrings.usersCreateLink, theme: theme) { [weak self] in
                        guard let self, !busy, (1...100).contains(uses), (1...30).contains(days) else { return }
                        make()
                    })
                rows.append(
                    FormRow.action(MavioStrings.actionCancel, theme: theme) { [weak self] in
                        self?.making = false
                        self?.render()
                    })
            } else {
                rows.append(
                    FormRow.link(MavioStrings.usersInvite, icon: "add", tint: theme.colors.accent, theme: theme) {
                        [weak self] in
                        self?.making = true
                        self?.made = nil
                        self?.render()
                    })
            }
            for i in list {
                rows.append(row(i, MavioStrings.usersInvitationUses(uses: String(i.uses), max: String(i.maxUses))))
            }
            if loaded && list.isEmpty && !making && link == nil {
                rows.append(page.note(MavioStrings.usersNoInvitations))
            }
            section.setRows(rows)
            section.footer = failure ?? MavioStrings.usersInvitationsFooter
            let converting = open.filter { !$0.userID.isEmpty }
            conversions.setRows(converting.map { i in row(i, users.first { $0.id == i.userID }?.name ?? i.userID) })
            conversions.isHidden = converting.isEmpty || section.isHidden
            page.refocus()
        }

        private func row(_ i: Mavio_User_V1_Invitation, _ label: String) -> NSView {
            guard let page else { return NSView() }
            let revoke = MavioButton(MavioStrings.usersRevoke, kind: .destructive, theme: page.theme) {
                [weak self] in self?.revoke(i.id)
            }
            revoke.isEnabled = !busy
            return ControlRow(
                page.text(
                    label,
                    [
                        MavioStrings.usersExpires(date: day(i.expireTime)),
                        i.hasCreateTime ? MavioStrings.usersCreated(date: day(i.createTime)) : "",
                    ].filter { !$0.isEmpty }.joined(separator: " · ")),
                controls: [revoke], theme: page.theme)
        }

        private func make() {
            guard let page else { return }
            page.endEditing()
            busy = true
            failure = nil
            var request = Mavio_User_V1_CreateInvitationRequest()
            request.maxUses = uses
            var d = Google_Protobuf_Duration()
            d.seconds = Int64(days) * 86400
            request.validFor = d
            var access = Mavio_User_V1_UserPolicy()
            access.allLibraries = all
            access.libraryIds = all ? [] : ids
            access.allowDownload = download
            request.access = access
            let model = page.model
            let server = page.server
            work = Task { [weak self] in
                do {
                    let r = try await model.call(
                        server, Procedure.users("CreateInvitation"), request,
                        as: Mavio_User_V1_CreateInvitationResponse.self)
                    self?.made = (r.invitation.id, model.invitationLink(server, r.token))
                    self?.making = false
                    self?.busy = false
                    await self?.load()
                } catch {
                    self?.failure = error.reason
                    self?.busy = false
                    self?.render()
                }
            }
        }

        private func revoke(_ id: String) {
            guard let page else { return }
            busy = true
            failure = nil
            render()
            let model = page.model
            let server = page.server
            var request = Mavio_User_V1_DeleteInvitationRequest()
            request.id = id
            work = Task { [weak self] in
                do {
                    _ = try await model.call(
                        server, Procedure.users("DeleteInvitation"), request,
                        as: Mavio_User_V1_DeleteInvitationResponse.self)
                    self?.busy = false
                    await self?.load()
                } catch {
                    self?.failure = error.reason
                    self?.busy = false
                    self?.render()
                }
            }
        }

        isolated deinit {
            work?.cancel()
        }
    }
#endif
