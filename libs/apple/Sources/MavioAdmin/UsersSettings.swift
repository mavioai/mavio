import MavioKit
import MavioUI
import SwiftUI

/// Users (docs/clients.md §6.3.5): the users of the server and its
/// invitation links, and a user's page: its name, whether it is disabled,
/// its restriction profile, the libraries and ratings it may see, how it may
/// play, and its sign-ins. What a user may request in Discover is left out
/// (docs/clients.md §4.2). In mavio.dev mode a user's name, access and state
/// are mavio.dev's, and the server's own credentials and invitations are not
/// used.
struct UsersView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var users: [Mavio_User_V1_User]?
    @State private var failure: String?
    @State private var standalone = false

    var body: some View {
        List {
            if let users {
                Section {
                    Text(standalone ? MavioStrings.usersMessage : MavioStrings.usersMessageMavioDev)
                        .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                        .listRowBackground(Color.clear)
                }
                Section {
                    ForEach(users, id: \.id) { u in
                        NavigationLink {
                            UserView(server: server, user: u, standalone: standalone, changed: reload)
                        } label: {
                            HStack(spacing: MavioSpacing.s12) {
                                Avatar(name: u.name, size: 32)
                                VStack(alignment: .leading, spacing: 2) {
                                    HStack(spacing: MavioSpacing.s8) {
                                        Text(u.name).lineLimit(1)
                                        Badges(user: u)
                                    }
                                    Text(
                                        u.hasLastLoginTime
                                            ? MavioStrings.usersLastSignIn(date: day(u.lastLoginTime))
                                            : MavioStrings.usersNeverSignedIn
                                    )
                                    .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                                }
                            }
                        }
                    }
                    if standalone {
                        NavigationLink {
                            NewUserView(server: server, changed: reload)
                        } label: {
                            Label(MavioStrings.usersAdd, systemImage: "plus")
                        }
                    }
                }
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
            }
            if let failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            }
            if standalone {
                InvitationsSections(server: server, users: users ?? [])
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsUsers)
        .inlineTitle()
        .task { await load() }
        .pullToRefresh { await load() }
    }

    private func reload() {
        Task { await load() }
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
        } catch {
            failure = error.reason
        }
    }
}

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

    /// The libraries access is given to: not those of collections and playlists.
    fileprivate func accessibleLibraries(_ server: ServerRecord) async -> [Mavio_Library_V1_Library] {
        let r = try? await call(
            server, Procedure.libraries("ListLibraries"), Mavio_Library_V1_ListLibrariesRequest(),
            as: Mavio_Library_V1_ListLibrariesResponse.self)
        return (r?.libraries ?? []).filter { $0.kind != .collections && $0.kind != .playlists }
    }

    /// The link of an invitation's token, which opens the server's page accepting it.
    fileprivate func invitationLink(_ server: ServerRecord, _ token: String) -> String {
        var base = server.address
        while base.hasSuffix("/") { base.removeLast() }
        let encoded = token.addingPercentEncoding(
            withAllowedCharacters: .alphanumerics.union(.init(charactersIn: "-_.!~*'()")))
        return "\(base)/invite#\(encoded ?? token)"
    }
}

/// A date as a day, in the user's language.
private func day(_ ts: Google_Protobuf_Timestamp) -> String {
    ts.date.formatted(date: .abbreviated, time: .omitted)
}

/// The badges of a user: administrator, managed, disabled.
private struct Badges: View {
    let user: Mavio_User_V1_User
    @Environment(\.theme) private var theme

    var body: some View {
        let badges = [
            user.admin ? MavioStrings.usersAdmin : nil, user.managed ? MavioStrings.usersManaged : nil,
            user.disabled ? MavioStrings.usersDisabled : nil,
        ].compactMap { $0 }
        ForEach(badges, id: \.self) { b in
            Text(b).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                .padding(.horizontal, 6)
                .background(theme.colors.fill, in: .rect(cornerRadius: 4))
        }
    }
}

/// The restriction profiles a managed user may have.
private let restrictions: [(value: Mavio_User_V1_RestrictionProfile, title: String)] = [
    (.unspecified, MavioStrings.usersRestrictionNone),
    (.youngerChild, MavioStrings.usersRestrictionYoungerChild),
    (.olderChild, MavioStrings.usersRestrictionOlderChild),
    (.teen, MavioStrings.usersRestrictionTeen),
]

/// Adds a member with a password, or a managed user with a PIN and a
/// restriction profile; then shows its page.
private struct NewUserView: View {
    let server: ServerRecord
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var managed = false
    @State private var name = ""
    @State private var password = ""
    @State private var pin = ""
    @State private var restriction = Mavio_User_V1_RestrictionProfile.unspecified
    @State private var created: Mavio_User_V1_User?
    @State private var saving = false
    @State private var failure: String?

    private var valid: Bool {
        !name.trimmingCharacters(in: .whitespaces).isEmpty
            && (managed ? pin.isEmpty || pin.wholeMatch(of: /\d{4}/) != nil : !password.isEmpty)
    }

    var body: some View {
        if let created {
            UserView(server: server, user: created, standalone: true, changed: changed)
        } else {
            form
        }
    }

    private var form: some View {
        List {
            Section {
                Picker(MavioStrings.usersKind, selection: $managed) {
                    Text(MavioStrings.usersMember).tag(false)
                    Text(MavioStrings.usersManaged).tag(true)
                }
                TextRow(label: MavioStrings.usersName, text: $name)
                if managed {
                    FormRow(label: MavioStrings.usersPin, detail: MavioStrings.usersPinDetail) {
                        PinField(pin: $pin)
                    }
                    Picker(selection: $restriction) {
                        ForEach(restrictions, id: \.value) { Text($0.title).tag($0.value) }
                    } label: {
                        RowLabel(label: MavioStrings.usersRestriction, detail: MavioStrings.usersRestrictionDetail)
                    }
                } else {
                    FormRow(label: MavioStrings.authPassword) {
                        SecureField(MavioStrings.authPassword, text: $password)
                            .textContentType(.newPassword)
                            .multilineTextAlignment(.trailing)
                            .frame(maxWidth: 170)
                    }
                }
            } footer: {
                Text(managed ? MavioStrings.usersManagedFooter : MavioStrings.usersMemberFooter)
            }
            if let failure {
                Section { Text(failure).foregroundStyle(theme.colors.danger) }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.usersAdd)
        .inlineTitle()
        .toolbar {
            if saving {
                ProgressView()
            } else {
                Button(MavioStrings.usersCreate, action: create).disabled(!valid)
            }
        }
    }

    private func create() {
        endEditing()
        saving = true
        failure = nil
        Task {
            defer { saving = false }
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
                created = try await model.call(
                    server, Procedure.users("CreateUser"), request, as: Mavio_User_V1_CreateUserResponse.self
                ).user
                changed()
            } catch {
                failure = error.reason
            }
        }
    }
}

/// A field of a four-digit PIN.
private struct PinField: View {
    @Binding var pin: String

    var body: some View {
        TextField("0000", text: $pin)
            .numberKeyboard()
            .multilineTextAlignment(.trailing)
            .frame(maxWidth: 80)
            .onChange(of: pin) {
                let digits = String(pin.filter(\.isNumber).prefix(4))
                if digits != pin { pin = digits }
            }
    }
}

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

    /// The policy of the draft, keeping what the page does not show of the user's.
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
private struct UserView: View {
    let server: ServerRecord
    @State var user: Mavio_User_V1_User
    /// Whether the server keeps its own accounts rather than mavio.dev's.
    let standalone: Bool
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var draft: UserDraft
    @State private var libraries: [Mavio_Library_V1_Library] = []
    @State private var ratings: [Mavio_System_V1_Rating] = []
    @State private var saving = false
    @State private var notice: String?
    @State private var failure: String?
    @State private var confirming = false

    init(server: ServerRecord, user: Mavio_User_V1_User, standalone: Bool, changed: @escaping () -> Void) {
        self.server = server
        self.standalone = standalone
        self._user = State(initialValue: user)
        self._draft = State(initialValue: UserDraft(user))
        self.changed = changed
    }

    private var saved: UserDraft { UserDraft(user) }
    private var valid: Bool { !draft.name.trimmingCharacters(in: .whitespaces).isEmpty }
    private var deletable: Bool { !user.admin && (standalone || user.disabled) }

    private static let mbps: Int64 = 1_000_000
    private static let bitrates: [Int64] = [0, 120, 80, 40, 20, 10, 8, 4, 2, 1].map { $0 * mbps }
    /// The rating score of adult content.
    private static let adult: Int32 = 1000

    var body: some View {
        List {
            Section {
                // The account's name, state and access are mavio.dev's in its mode.
                if standalone {
                    TextRow(label: MavioStrings.usersName, text: $draft.name)
                } else {
                    LabeledContent(MavioStrings.usersName, value: user.name)
                }
                if !user.admin {
                    toggle($draft.disabled, MavioStrings.usersDisable, MavioStrings.usersDisableDetail, own: standalone)
                }
                if user.managed {
                    Picker(selection: $draft.restriction) {
                        ForEach(restrictions, id: \.value) { Text($0.title).tag($0.value) }
                    } label: {
                        RowLabel(label: MavioStrings.usersRestriction, detail: MavioStrings.usersRestrictionDetail)
                    }
                }
            } header: {
                HStack(spacing: MavioSpacing.s12) {
                    Avatar(name: user.name, size: 40)
                    Text(user.name).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                    Badges(user: user)
                }
                .textCase(nil)
                .padding(.bottom, MavioSpacing.s8)
            } footer: {
                if !standalone { Text(MavioStrings.usersMavioDevNote) }
            }
            if !user.admin { access }
            Section(MavioStrings.usersPlayback) {
                toggle(
                    $draft.allowTranscoding, MavioStrings.usersAllowTranscoding,
                    MavioStrings.usersAllowTranscodingDetail)
                Picker(MavioStrings.usersMaxBitrate, selection: $draft.maxBitrate) {
                    ForEach(bitrates, id: \.self) { Text(Self.bitrateTitle($0)).tag($0) }
                }
                NumberRow(
                    label: MavioStrings.usersMaxSessions, detail: MavioStrings.usersMaxSessionsDetail,
                    value: $draft.maxSessions.double, range: 0...100)
            }
            if !user.admin {
                Section {
                    toggle($draft.reportIssues, MavioStrings.usersReportIssues)
                }
            }
            if standalone {
                SignInsSection(server: server, user: $user, changed: changed)
            }
            if deletable {
                Section {
                    Button(MavioStrings.actionDelete, role: .destructive) { confirming = true }
                        .confirmationDialog(
                            MavioStrings.usersDeleteTitle(name: user.name), isPresented: $confirming,
                            titleVisibility: .visible
                        ) {
                            Button(MavioStrings.actionDelete, role: .destructive, action: delete)
                        } message: {
                            Text(MavioStrings.usersDeleteMessage)
                        }
                }
            }
            if notice != nil && draft == saved || failure != nil {
                Section {
                    if let notice, draft == saved { Text(notice).foregroundStyle(theme.colors.textSecondary) }
                    if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
                }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(user.name)
        .inlineTitle()
        .toolbar {
            if saving {
                ProgressView()
            } else {
                Button(MavioStrings.actionSave, action: save).disabled(draft == saved || !valid)
            }
        }
        .task {
            async let l = model.accessibleLibraries(server)
            let r = try? await model.call(
                server, Procedure.localization("ListRatings"), Mavio_System_V1_ListRatingsRequest(),
                as: Mavio_System_V1_ListRatingsResponse.self)
            ratings = r?.ratings ?? []
            libraries = await l
        }
    }

    @ViewBuilder private var access: some View {
        Section {
            toggle($draft.allLibraries, MavioStrings.usersAllLibraries, own: standalone)
            if !draft.allLibraries {
                ForEach(libraries, id: \.id) { l in
                    let on = draft.libraryIds.contains(l.id)
                    Button {
                        if on { draft.libraryIds.removeAll { $0 == l.id } } else { draft.libraryIds.append(l.id) }
                    } label: {
                        HStack {
                            Text(l.name).foregroundStyle(theme.colors.textPrimary)
                            Spacer()
                            if on { Image(systemName: "checkmark").foregroundStyle(theme.colors.accentText) }
                        }
                    }
                    .disabled(!standalone)
                }
            }
            Picker(selection: $draft.maxRating) {
                Text(MavioStrings.usersNoLimit).tag(Int32?.none)
                ForEach(ratingScores, id: \.self) { Text(ratingTitle($0)).tag(Int32?.some($0)) }
            } label: {
                RowLabel(label: MavioStrings.usersMaxRating, detail: MavioStrings.usersMaxRatingDetail)
            }
            .disabled(!standalone)
            toggle($draft.blockUnrated, MavioStrings.usersBlockUnrated, own: standalone)
            labelsRow(MavioStrings.usersAllowedLabels, MavioStrings.usersAllowedLabelsDetail, $draft.allowedLabels)
            labelsRow(MavioStrings.usersBlockedLabels, MavioStrings.usersBlockedLabelsDetail, $draft.blockedLabels)
            toggle($draft.allowDownload, MavioStrings.usersAllowDownload, own: standalone)
        } header: {
            Text(MavioStrings.usersAccess)
        } footer: {
            if !standalone { Text(MavioStrings.usersAccessMavioDev) }
        }
    }

    private func labelsRow(_ label: String, _ detail: String, _ text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            RowLabel(label: label, detail: detail)
            TextField(label, text: text)
                .noCapitals()
                .autocorrectionDisabled()
                .borderedField()
                .disabled(!standalone)
        }
    }

    /// Ratings by the age they stand for; those above adult content are left to no limit.
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

    @ViewBuilder
    private func toggle(_ on: Binding<Bool>, _ label: String, _ detail: String? = nil, own: Bool = true) -> some View {
        if own {
            Toggle(isOn: on) { RowLabel(label: label, detail: detail) }
        } else {
            LabeledContent {
                Text(on.wrappedValue ? MavioStrings.usersOn : MavioStrings.usersOff)
            } label: {
                RowLabel(label: label, detail: detail)
            }
        }
    }

    private func save() {
        endEditing()
        saving = true
        failure = nil
        Task {
            defer { saving = false }
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
                changed()
            } catch {
                failure = error.reason
            }
        }
    }

    private func delete() {
        Task {
            var request = Mavio_User_V1_DeleteUserRequest()
            request.id = user.id
            do {
                _ = try await model.call(
                    server, Procedure.users("DeleteUser"), request, as: Mavio_User_V1_DeleteUserResponse.self)
                changed()
                dismiss()
            } catch {
                failure = error.reason
            }
        }
    }
}

/// A user's sign-ins: its password, PIN, authenticator and passkeys, and an
/// invitation that converts a managed user into a member.
private struct SignInsSection: View {
    let server: ServerRecord
    @Binding var user: Mavio_User_V1_User
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var password = ""
    @State private var pin = ""
    @State private var link: String?
    @State private var busy = false
    @State private var notice: String?
    @State private var failure: String?

    var body: some View {
        Section {
            // Users change their own password, with the current one, in their account's settings.
            if user.authProvider.isEmpty && user.id != server.userId {
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    RowLabel(
                        label: user.managed ? MavioStrings.usersGivePassword : MavioStrings.usersSetPassword,
                        detail: user.managed
                            ? MavioStrings.usersGivePasswordDetail : MavioStrings.usersSetPasswordDetail)
                    HStack {
                        SecureField(MavioStrings.authPassword, text: $password)
                            .textContentType(.newPassword)
                            .borderedField()
                        Button(MavioStrings.usersSet) { setPassword() }
                            .buttonStyle(.bordered)
                            .disabled(password.isEmpty || busy)
                    }
                }
            }
            if user.managed {
                HStack {
                    RowLabel(
                        label: MavioStrings.usersPin,
                        detail: user.hasPin_p ? MavioStrings.usersPinOn : MavioStrings.usersPinOff)
                    Spacer()
                    PinField(pin: $pin)
                    Button(MavioStrings.usersSet) { setPin(pin) }
                        .buttonStyle(.bordered)
                        .disabled(pin.count != 4 || busy)
                }
                if user.hasPin_p {
                    Button(MavioStrings.usersClear) { setPin("") }.disabled(busy)
                }
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    RowLabel(
                        label: MavioStrings.usersInviteManaged, detail: link ?? MavioStrings.usersInviteManagedDetail
                    )
                    .selectable()
                    if let link {
                        CopyLinkButton(link: link)
                    } else {
                        Button(MavioStrings.usersCreateLink) { invite() }
                            .buttonStyle(.bordered)
                            .disabled(busy)
                    }
                }
            }
            if user.hasTotp_p {
                HStack {
                    RowLabel(label: MavioStrings.usersTotp, detail: MavioStrings.usersTotpDetail)
                    Spacer()
                    Button(MavioStrings.usersTurnOff, role: .destructive) { deleteTotp() }
                        .buttonStyle(.bordered)
                        .disabled(busy)
                }
            }
            ForEach(user.passkeys, id: \.id) { k in
                HStack {
                    RowLabel(
                        label: k.name.isEmpty ? MavioStrings.usersPasskey : k.name,
                        detail: [k.rpID, k.hasLastUseTime ? MavioStrings.usersLastUsed(date: day(k.lastUseTime)) : ""]
                            .filter { !$0.isEmpty }.joined(separator: " · "))
                    Spacer()
                    Button(MavioStrings.actionDelete, role: .destructive) { deletePasskey(k.id) }
                        .buttonStyle(.bordered)
                        .disabled(busy)
                }
            }
        } header: {
            Text(MavioStrings.usersSignIns)
        } footer: {
            if let failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            } else if let notice {
                Text(notice)
            }
        }
    }

    /// Runs a change of the user's sign-ins, then reloads the user.
    private func run(_ done: String?, _ action: @escaping () async throws -> Void) {
        endEditing()
        busy = true
        failure = nil
        Task {
            defer { busy = false }
            do {
                try await action()
                notice = done
                var request = Mavio_User_V1_GetUserRequest()
                request.id = user.id
                if let r = try? await model.call(
                    server, Procedure.users("GetUser"), request, as: Mavio_User_V1_GetUserResponse.self)
                {
                    user = r.user
                }
                changed()
            } catch {
                failure = error.reason
            }
        }
    }

    private func setPassword() {
        let converted = user.managed
        run(converted ? MavioStrings.usersConverted : MavioStrings.usersPasswordSet) {
            var request = Mavio_User_V1_SetPasswordRequest()
            request.userID = user.id
            request.newPassword = password
            _ = try await model.call(
                server, Procedure.users("SetPassword"), request, as: Mavio_User_V1_SetPasswordResponse.self)
            password = ""
        }
    }

    private func setPin(_ value: String) {
        run(value.isEmpty ? MavioStrings.usersPinCleared : MavioStrings.usersPinSet) {
            var request = Mavio_User_V1_SetPinRequest()
            request.userID = user.id
            request.pin = value
            _ = try await model.call(server, Procedure.users("SetPin"), request, as: Mavio_User_V1_SetPinResponse.self)
            pin = ""
        }
    }

    private func deleteTotp() {
        run(nil) {
            var request = Mavio_User_V1_DeleteTotpRequest()
            request.userID = user.id
            _ = try await model.call(
                server, Procedure.users("DeleteTotp"), request, as: Mavio_User_V1_DeleteTotpResponse.self)
        }
    }

    private func deletePasskey(_ id: String) {
        run(nil) {
            var request = Mavio_User_V1_DeletePasskeyRequest()
            request.userID = user.id
            request.id = id
            _ = try await model.call(
                server, Procedure.users("DeletePasskey"), request, as: Mavio_User_V1_DeletePasskeyResponse.self)
        }
    }

    private func invite() {
        busy = true
        failure = nil
        Task {
            defer { busy = false }
            var request = Mavio_User_V1_CreateInvitationRequest()
            request.userID = user.id
            request.maxUses = 1
            var d = Google_Protobuf_Duration()
            d.seconds = 7 * 86400
            request.validFor = d
            do {
                let r = try await model.call(
                    server, Procedure.users("CreateInvitation"), request,
                    as: Mavio_User_V1_CreateInvitationResponse.self)
                link = model.invitationLink(server, r.token)
            } catch {
                failure = error.reason
            }
        }
    }
}

/// A button copying a link, telling once it is copied.
private struct CopyLinkButton: View {
    let link: String
    @State private var copied = false

    var body: some View {
        // Apple TV has neither a clipboard nor sharing: the link shows.
        #if !os(tvOS)
            HStack {
                Button(copied ? MavioStrings.usersCopied : MavioStrings.usersCopyLink) {
                    copyToClipboard(link)
                    copied = true
                }
                .buttonStyle(.bordered)
                if let url = URL(string: link) {
                    ShareLink(item: url) { Image(systemName: "square.and.arrow.up") }
                        .buttonStyle(.bordered)
                }
            }
        #endif
    }
}

/// The server's invitation links — making one, which shows its link once,
/// and revoking them — and the links made for managed users, each
/// converting one into a member.
private struct InvitationsSections: View {
    let server: ServerRecord
    let users: [Mavio_User_V1_User]
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var invitations: [Mavio_User_V1_Invitation] = []
    @State private var loaded = false
    @State private var libraries: [Mavio_Library_V1_Library] = []
    @State private var making = false
    @State private var uses: Int32 = 1
    @State private var days: Int32 = 7
    @State private var all = true
    @State private var ids: [String] = []
    @State private var download = true
    @State private var made: (id: String, link: String)?
    @State private var busy = false
    @State private var failure: String?

    /// Links used up can no longer be accepted.
    private var open: [Mavio_User_V1_Invitation] { invitations.filter { $0.uses < $0.maxUses } }

    var body: some View {
        let list = open.filter(\.userID.isEmpty)
        // The link made is shown until its invitation is revoked or used up.
        let link = made.flatMap { m in list.contains { $0.id == m.id } ? m.link : nil }
        Section {
            if let link {
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    RowLabel(label: MavioStrings.usersLinkMade, detail: link).selectable()
                    CopyLinkButton(link: link)
                }
            }
            if making {
                NumberRow(label: MavioStrings.usersMaxUses, value: $uses.double, range: 1...100)
                NumberRow(label: MavioStrings.usersValidDays, value: $days.double, range: 1...30)
                Toggle(MavioStrings.usersAllLibraries, isOn: $all)
                if !all {
                    ForEach(libraries, id: \.id) { l in
                        let on = ids.contains(l.id)
                        Button {
                            if on { ids.removeAll { $0 == l.id } } else { ids.append(l.id) }
                        } label: {
                            HStack {
                                Text(l.name).foregroundStyle(theme.colors.textPrimary)
                                Spacer()
                                if on { Image(systemName: "checkmark").foregroundStyle(theme.colors.accentText) }
                            }
                        }
                    }
                }
                Toggle(MavioStrings.usersAllowDownload, isOn: $download)
                Button(MavioStrings.usersCreateLink, action: make).disabled(busy)
                Button(MavioStrings.actionCancel) { making = false }
            } else {
                Button {
                    making = true
                    made = nil
                } label: {
                    Label(MavioStrings.usersInvite, systemImage: "plus")
                }
            }
            ForEach(list, id: \.id) {
                row($0, MavioStrings.usersInvitationUses(uses: String($0.uses), max: String($0.maxUses)))
            }
            if loaded && list.isEmpty && !making && link == nil {
                Text(MavioStrings.usersNoInvitations).foregroundStyle(theme.colors.textSecondary)
            }
        } header: {
            Text(MavioStrings.usersInvitations)
        } footer: {
            if let failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            } else {
                Text(MavioStrings.usersInvitationsFooter)
            }
        }
        .task {
            await load()
            libraries = await model.accessibleLibraries(server)
        }
        let conversions = open.filter { !$0.userID.isEmpty }
        if !conversions.isEmpty {
            Section(MavioStrings.usersConversionLinks) {
                ForEach(conversions, id: \.id) { i in
                    row(i, users.first { $0.id == i.userID }?.name ?? i.userID)
                }
            }
        }
    }

    private func row(_ i: Mavio_User_V1_Invitation, _ label: String) -> some View {
        HStack {
            RowLabel(
                label: label,
                detail: [
                    MavioStrings.usersExpires(date: day(i.expireTime)),
                    i.hasCreateTime ? MavioStrings.usersCreated(date: day(i.createTime)) : "",
                ].filter { !$0.isEmpty }.joined(separator: " · "))
            Spacer()
            Button(MavioStrings.usersRevoke, role: .destructive) { revoke(i.id) }
                .buttonStyle(.bordered)
                .disabled(busy)
        }
    }

    private func load() async {
        do {
            invitations = try await model.call(
                server, Procedure.users("ListInvitations"), Mavio_User_V1_ListInvitationsRequest(),
                as: Mavio_User_V1_ListInvitationsResponse.self
            ).invitations
            loaded = true
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func make() {
        busy = true
        failure = nil
        Task {
            defer { busy = false }
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
            do {
                let r = try await model.call(
                    server, Procedure.users("CreateInvitation"), request,
                    as: Mavio_User_V1_CreateInvitationResponse.self)
                made = (r.invitation.id, model.invitationLink(server, r.token))
                making = false
                await load()
            } catch {
                failure = error.reason
            }
        }
    }

    private func revoke(_ id: String) {
        busy = true
        failure = nil
        Task {
            defer { busy = false }
            var request = Mavio_User_V1_DeleteInvitationRequest()
            request.id = id
            do {
                _ = try await model.call(
                    server, Procedure.users("DeleteInvitation"), request,
                    as: Mavio_User_V1_DeleteInvitationResponse.self)
                await load()
            } catch {
                failure = error.reason
            }
        }
    }
}
