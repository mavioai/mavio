import MavioKit
import MavioUI
import SwiftUI

/// Subtitles and Audio, as the web UI's: the user's languages and when
/// subtitles show, read from the first server signed in to and saved to
/// each of them.
struct SubtitlesAudioView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var saved: PlaybackPreferences?
    @State private var audio = ""
    @State private var subtitles = ""
    @State private var mode = SubtitleMode.default
    @State private var failure: String?
    @State private var saving = false
    @FocusState private var editing: Int?

    /// The modes in the web UI's order.
    private static let modes: [(SubtitleMode, String)] = [
        (.default, MavioStrings.settingsSubtitleModeDefault),
        (.always, MavioStrings.settingsSubtitleModeAlways),
        (.foreign, MavioStrings.settingsSubtitleModeForeign),
        (.smart, MavioStrings.settingsSubtitleModeSmart),
        (.forced, MavioStrings.settingsSubtitleModeForced),
        (.never, MavioStrings.settingsSubtitleModeNone),
    ]

    var body: some View {
        List {
            if saved != nil {
                Section {
                    field(MavioStrings.settingsAudioLanguages, text: $audio).focused($editing, equals: 0)
                    field(MavioStrings.settingsSubtitleLanguages, text: $subtitles).focused($editing, equals: 1)
                } header: {
                    Text(MavioStrings.settingsSubtitlesAudioMessage)
                } footer: {
                    Text(MavioStrings.settingsLanguagesHint)
                }
                Section {
                    Picker(MavioStrings.settingsSubtitleMode, selection: $mode) {
                        ForEach(Self.modes, id: \.0) { Text($0.1).tag($0.0) }
                    }
                    .pickerStyle(.navigationLink)
                }
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity)
                    .listRowBackground(Color.clear)
            }
            if let failure {
                Text(failure)
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.danger)
            }
        }
        .scrollContentBackground(.hidden)
        .scrollDismissesKeyboard(.interactively)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsSubtitlesAudio)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if saving {
                ProgressView()
            } else if saved != nil {
                Button(MavioStrings.actionSave, action: save)
                    .disabled(!changed || Self.languages(audio) == nil || Self.languages(subtitles) == nil)
            }
        }
        // Coming back from the mode's list keeps what was typed.
        .task { if saved == nil { await load() } }
    }

    private func field(_ label: String, text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            Text(label)
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.textSecondary)
            TextField(label, text: text, prompt: Text("chi, eng"))
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .foregroundStyle(
                    Self.languages(text.wrappedValue) == nil ? theme.colors.danger : theme.colors.textPrimary)
        }
        .padding(.vertical, 2)
    }

    private var changed: Bool {
        guard let saved else { return false }
        return Self.languages(audio) != saved.audioLanguages
            || Self.languages(subtitles) != saved.subtitleLanguages || mode != saved.subtitleMode
    }

    private func load() async {
        do {
            let prefs = try await model.client.playbackPreferences()
            show(prefs)
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    private func show(_ prefs: PlaybackPreferences) {
        saved = prefs
        audio = prefs.audioLanguages.joined(separator: ", ")
        subtitles = prefs.subtitleLanguages.joined(separator: ", ")
        mode = prefs.subtitleMode
    }

    private func save() {
        guard var prefs = saved, let a = Self.languages(audio), let s = Self.languages(subtitles) else { return }
        prefs.audioLanguages = a
        prefs.subtitleLanguages = s
        prefs.subtitleMode = mode
        editing = nil
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            do {
                try await model.client.setPlaybackPreferences(preferences: prefs)
                show(prefs)
            } catch {
                failure = (error as? CoreError)?.reason ?? error.localizedDescription
            }
        }
    }

    /// Three-letter codes from a field: lowercase, separated by commas or
    /// spaces; nil when one is not three letters.
    static func languages(_ s: String) -> [String]? {
        let list = s.lowercased()
            .split(whereSeparator: { $0.isWhitespace || ",，、".contains($0) })
            .map(String.init)
        return list.allSatisfy { $0.count == 3 && $0.allSatisfy { ("a"..."z").contains($0) } } ? list : nil
    }
}

/// The users of a standalone server as avatars, to switch to with their
/// PIN, as the web UI's account settings show them.
struct SwitchUserSection: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var users: [SwitchableUser] = []
    @State private var asking: SwitchableUser?
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        Group {
            // Switching needs someone else to switch to.
            if users.count > 1 {
                Section(MavioStrings.authSwitchUser) {
                    ScrollView(.horizontal) {
                        HStack(alignment: .top, spacing: MavioSpacing.s20) {
                            ForEach(users, id: \.id) { user in
                                Button {
                                    if user.hasPin { asking = user } else { Task { await switchTo(user, pin: nil) } }
                                } label: {
                                    member(user)
                                }
                                .buttonStyle(.plain)
                                .disabled(user.current || busy)
                            }
                        }
                        .padding(.vertical, MavioSpacing.s8)
                    }
                    .scrollIndicators(.hidden)
                    if let failure, asking == nil {
                        Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                }
            }
        }
        .task(id: server.userId) {
            users = (try? await model.client.switchableUsers(serverId: server.id)) ?? []
        }
        .sheet(
            isPresented: Binding(get: { asking != nil }, set: { if !$0 { asking = nil } })
        ) {
            if let asking {
                PinPad(name: asking.name, busy: busy, error: failure) { pin in
                    Task { await switchTo(asking, pin: pin) }
                } cancel: {
                    self.asking = nil
                    failure = nil
                }
                .presentationDetents([.height(560)])
                .presentationBackground(theme.colors.background)
            }
        }
    }

    private func member(_ user: SwitchableUser) -> some View {
        VStack(spacing: 6) {
            Avatar(name: user.name, size: 56)
                .padding(3)
                .overlay {
                    if user.current { Circle().strokeBorder(theme.colors.accent, lineWidth: 2) }
                }
                .overlay(alignment: .bottomTrailing) {
                    if user.hasPin {
                        Icon("lock", fill: true, size: 12)
                            .padding(4)
                            .background(theme.colors.surfaceRaised, in: .circle)
                    }
                }
            Text(user.name)
                .type(theme.type.footnote)
                .lineLimit(1)
        }
        .frame(width: 68)
        .contentShape(.rect)
    }

    private func switchTo(_ user: SwitchableUser, pin: String?) async {
        busy = true
        failure = nil
        defer { busy = false }
        do {
            try await model.switchUser(server.id, to: user.id, pin: pin)
            asking = nil
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }
}

/// Asks for a user's four-digit PIN on a phone's keypad, as the web UI's;
/// the fourth digit sends it, and a wrong one clears the dots.
private struct PinPad: View {
    let name: String
    let busy: Bool
    let error: String?
    let enter: (String) -> Void
    let cancel: () -> Void
    @Environment(\.theme) private var theme
    @State private var pin = ""

    var body: some View {
        VStack(spacing: MavioSpacing.s20) {
            Avatar(name: name, size: 56)
            Text(MavioStrings.authEnterPin(name: name))
                .type(theme.type.title3)
                .multilineTextAlignment(.center)
            HStack(spacing: MavioSpacing.s16) {
                ForEach(0..<4, id: \.self) { i in
                    Circle()
                        .strokeBorder(theme.colors.textPrimary, lineWidth: 2)
                        .background(Circle().fill(i < pin.count ? theme.colors.textPrimary : .clear))
                        .frame(width: 12, height: 12)
                }
            }
            .accessibilityElement()
            .accessibilityLabel(MavioStrings.authPin)
            .accessibilityValue("\(pin.count)/4")
            Text(error ?? " ")
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.danger)
                .opacity(error == nil ? 0 : 1)
            Grid(horizontalSpacing: MavioSpacing.s20, verticalSpacing: MavioSpacing.s12) {
                ForEach([["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"], ["", "0", "⌫"]], id: \.self) { row in
                    GridRow {
                        ForEach(row, id: \.self) { key($0) }
                    }
                }
            }
        }
        .padding(MavioSpacing.s24)
        .sensoryFeedback(.error, trigger: error) { _, new in new != nil }
        .onChange(of: error) { _, new in if new != nil { pin = "" } }
    }

    @ViewBuilder private func key(_ k: String) -> some View {
        let label: AnyView =
            switch k {
            // Cancel takes the keypad's empty corner, as on a phone's.
            case "": AnyView(Text(MavioStrings.actionCancel).type(theme.type.callout))
            case "⌫": AnyView(Icon("backspace", size: 24).accessibilityLabel(MavioStrings.searchBackspace))
            default: AnyView(Text(k).font(.system(size: 28, weight: .regular, design: .rounded)))
            }
        Button {
            press(k)
        } label: {
            label
                .frame(width: 72, height: 72)
                .background(k == "⌫" ? .clear : theme.colors.fill, in: .circle)
                .contentShape(.circle)
        }
        .buttonStyle(.plain)
        .disabled(busy && k != "")
    }

    private func press(_ k: String) {
        switch k {
        case "": cancel()
        case "⌫": pin = String(pin.dropLast())
        default:
            guard pin.count < 4 else { return }
            pin += k
            if pin.count == 4 { enter(pin) }
        }
    }
}

/// Sign-In and Security on a standalone server, as the web UI's: the
/// password, the passkeys, the authenticator app and the PIN; a managed
/// user has none of them.
struct SecuritySections: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var security: AccountSecurity?
    @State private var busy = false
    @State private var failure: String?
    @State private var editing: Editing?
    @State private var current = ""
    @State private var new = ""
    @State private var pin = ""
    @State private var code = ""
    @State private var totp: TotpSetup?
    @State private var passwordChanged = false

    private enum Editing { case password, pin }

    var body: some View {
        Group {
            if let security, !security.managed {
                Section {
                    if security.hasPassword { password }
                    totpRows(on: security.hasTotp)
                    pinRows(on: security.hasPin)
                } header: {
                    Text(MavioStrings.authSecurity)
                } footer: {
                    if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
                }
                Section(MavioStrings.authPasskeys) {
                    if security.passkeys.isEmpty {
                        Text(MavioStrings.authNoPasskeys).foregroundStyle(theme.colors.textSecondary)
                    }
                    ForEach(security.passkeys, id: \.id) { passkey in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(passkey.name)
                            Text(
                                [MavioStrings.authPasskeyHost(host: passkey.host)]
                                    .appending(passkey.created.map { $0.formatted(date: .abbreviated, time: .omitted) })
                                    .joined(separator: " · ")
                            )
                            .type(theme.type.footnote)
                            .foregroundStyle(theme.colors.textSecondary)
                        }
                        .swipeActions {
                            Button(MavioStrings.actionDelete, role: .destructive) {
                                change {
                                    try await model.client.deletePasskey(serverId: server.id, passkeyId: passkey.id)
                                }
                            }
                        }
                    }
                }
            }
        }
        .task(id: server.userId) { await load() }
    }

    private var password: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            row(
                MavioStrings.authPassword,
                detail: passwordChanged ? MavioStrings.authPasswordChanged : MavioStrings.authPasswordMessage
            ) {
                if editing != .password {
                    Button(MavioStrings.authChangePassword) {
                        passwordChanged = false
                        editing = .password
                    }
                }
            }
            if editing == .password {
                SecureField(MavioStrings.authCurrentPassword, text: $current)
                    .textContentType(.password)
                SecureField(MavioStrings.authNewPassword, text: $new)
                    .textContentType(.newPassword)
                actions(save: current.isEmpty || new.isEmpty) {
                    try await model.client.setPassword(serverId: server.id, current: current, new: new)
                    passwordChanged = true
                }
            }
        }
    }

    @ViewBuilder private func totpRows(on: Bool) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            row(MavioStrings.authTotp, detail: on ? MavioStrings.authTotpOn : MavioStrings.authTotpOff) {
                if on {
                    Button(MavioStrings.authTurnOff, role: .destructive) {
                        change { try await model.client.deleteTotp(serverId: server.id) }
                    }
                } else if totp == nil {
                    Button(MavioStrings.authSetUpTotp) {
                        change { totp = try await model.client.startTotp(serverId: server.id) }
                    }
                }
            }
            if let totp, !on {
                Text(MavioStrings.authTotpSecret(secret: totp.secret))
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                    .textSelection(.enabled)
                if let uri = URL(string: totp.uri) {
                    Link(MavioStrings.authOpenAuthenticator, destination: uri)
                }
                TextField(MavioStrings.authTotpCode, text: $code, prompt: Text("000000"))
                    .keyboardType(.numberPad)
                    .textContentType(.oneTimeCode)
                    .font(.system(.body, design: .monospaced))
                    .onChange(of: code) { _, new in
                        let digits = String(new.filter(\.isNumber).prefix(6))
                        if digits != new { code = digits }
                    }
                HStack {
                    Button(MavioStrings.actionSave) {
                        change {
                            try await model.client.confirmTotp(serverId: server.id, code: code)
                            self.totp = nil
                            code = ""
                        }
                    }
                    .disabled(busy || code.count != 6)
                    Button(MavioStrings.actionCancel) {
                        self.totp = nil
                        code = ""
                    }
                    .tint(theme.colors.textSecondary)
                }
                .buttonStyle(.borderless)
            }
        }
    }

    @ViewBuilder private func pinRows(on: Bool) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            row(
                MavioStrings.authPin,
                detail: "\(on ? MavioStrings.authPinOn : MavioStrings.authTotpOff) · \(MavioStrings.authPinMessage)"
            ) {
                if on {
                    Button(MavioStrings.authRemovePin, role: .destructive) {
                        change { try await model.client.setPin(serverId: server.id, pin: "") }
                    }
                }
                if editing != .pin {
                    Button(MavioStrings.authSetPin) { editing = .pin }
                }
            }
            if editing == .pin {
                SecureField(MavioStrings.authPin, text: $pin)
                    .keyboardType(.numberPad)
                    .textContentType(.newPassword)
                    .onChange(of: pin) { _, new in
                        let digits = String(new.filter(\.isNumber).prefix(4))
                        if digits != new { pin = digits }
                    }
                actions(save: pin.count != 4) {
                    try await model.client.setPin(serverId: server.id, pin: pin)
                }
            }
        }
    }

    /// A setting's title and state, with its buttons.
    private func row(_ title: String, detail: String, @ViewBuilder buttons: () -> some View) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            }
            Spacer(minLength: 0)
            buttons().buttonStyle(.borderless).disabled(busy)
        }
    }

    /// Save, which runs f and closes the form when it succeeds, and Cancel.
    private func actions(save disabled: Bool, _ f: @escaping () async throws -> Void) -> some View {
        HStack(spacing: MavioSpacing.s20) {
            Button(MavioStrings.actionSave) {
                change {
                    try await f()
                    close()
                }
            }
            .disabled(busy || disabled)
            Button(MavioStrings.actionCancel, action: close)
                .tint(theme.colors.textSecondary)
        }
        .buttonStyle(.borderless)
    }

    private func close() {
        editing = nil
        (current, new, pin) = ("", "", "")
    }

    private func load() async {
        do {
            security = try await model.client.security(serverId: server.id)
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    /// Runs a change, then reads the user's security again.
    private func change(_ f: @escaping () async throws -> Void) {
        busy = true
        failure = nil
        Task {
            defer { busy = false }
            do {
                try await f()
            } catch {
                failure = (error as? CoreError)?.reason ?? error.localizedDescription
            }
            await load()
        }
    }
}

/// Devices, as the web UI's: the user's devices signed in to the server,
/// to sign the others out.
struct DevicesView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var devices: [DeviceSession]?
    @State private var failure: String?
    @State private var busy = false
    @State private var controlling: String?

    var body: some View {
        List {
            Section {
                ForEach(devices ?? [], id: \.id) { device in
                    HStack(spacing: MavioSpacing.s12) {
                        Icon("devices", size: 20)
                            .frame(width: 36, height: 36)
                            .background(theme.colors.fill, in: .rect(cornerRadius: 9))
                        VStack(alignment: .leading, spacing: 2) {
                            Text(RemoteView.name(device)).lineLimit(1)
                            Text(detail(device))
                                .type(theme.type.footnote)
                                .foregroundStyle(theme.colors.textSecondary)
                                .lineLimit(1)
                        }
                        Spacer(minLength: 0)
                        if device.current {
                            Text(MavioStrings.settingsThisDevice)
                                .type(theme.type.callout)
                                .foregroundStyle(theme.colors.textSecondary)
                        } else {
                            if device.online {
                                Button(MavioStrings.remoteControl) { controlling = device.id }
                                    .buttonStyle(.borderless)
                            }
                            Button(MavioStrings.authSignOut, role: .destructive) { signOut(device) }
                                .buttonStyle(.borderless)
                                .disabled(busy)
                        }
                    }
                }
            } header: {
                Text(MavioStrings.settingsDevicesMessage)
            } footer: {
                if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
            }
        }
        .overlay { if devices == nil && failure == nil { ProgressView() } }
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsDevices)
        .navigationBarTitleDisplayMode(.inline)
        .refreshable { await load() }
        .sheet(
            isPresented: Binding(get: { controlling != nil }, set: { if !$0 { controlling = nil } }),
            onDismiss: { Task { await load() } }
        ) {
            if let controlling {
                RemoteView(server: server, sessionId: controlling)
                    .presentationDetents([.medium, .large])
            }
        }
        .task { await load() }
    }

    private func detail(_ d: DeviceSession) -> String {
        let state: String? =
            if let playing = d.playing {
                MavioStrings.remotePlaying(name: playing.card.title)
            } else if d.online {
                MavioStrings.remoteOnline
            } else {
                d.lastSeen.map { $0.formatted(date: .abbreviated, time: .shortened) }
            }
        return [d.client].appending(state).filter { !$0.isEmpty }.joined(separator: " · ")
    }

    private func load() async {
        do {
            devices = try await model.client.devices(serverId: server.id)
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    private func signOut(_ device: DeviceSession) {
        busy = true
        Task {
            defer { busy = false }
            do {
                try await model.client.signOutDevice(serverId: server.id, sessionId: device.id)
            } catch {
                failure = (error as? CoreError)?.reason ?? error.localizedDescription
            }
            await load()
        }
    }
}

extension Array {
    /// The array with element after it, when there is one.
    fileprivate func appending(_ element: Element?) -> [Element] {
        guard let element else { return self }
        return self + [element]
    }
}
