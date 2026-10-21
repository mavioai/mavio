import MavioAdmin
import MavioKit
import MavioUI
import SwiftUI

/// The settings (docs/tv.md §6.6): the list of their sections — the
/// account, the users to switch to, this device's playback, the user's
/// subtitles and audio and the assistant's settings, the Server group — each shown in its place when
/// selected, then About and signing out.
struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var switchable: [SwitchableUser] = []
    @State private var serverId = ""

    /// The server a device without a Mavio account signs in to.
    private var standalone: ServerRecord? {
        model.account == nil ? model.signedInServers.first : nil
    }

    /// The server whose settings the Server group shows.
    private var chosen: ServerRecord? {
        model.signedInServers.first { $0.id == serverId } ?? model.signedInServers.first
    }

    var body: some View {
        SettingsPage(MavioStrings.navSettings) {
            NavigationLink {
                AccountView()
            } label: {
                AccountCard()
            }
            .buttonStyle(RowButtonStyle())
            if let server = standalone {
                SwitchUsers(server: server, users: switchable)
            }
            Heading(MavioStrings.settingsThisDevice)
            NavigationLink {
                PlaybackView()
            } label: {
                EntryLabel(MavioStrings.settingsPlayback, icon: "play")
            }
            .buttonStyle(RowButtonStyle())
            if !model.signedInServers.isEmpty {
                NavigationLink {
                    SubtitlesAudioView()
                } label: {
                    EntryLabel(MavioStrings.settingsSubtitlesAudio, icon: "subtitles")
                }
                .buttonStyle(RowButtonStyle())
            }
            if let server = chosen {
                NavigationLink {
                    AssistantSettingsView(server: server)
                } label: {
                    EntryLabel(MavioStrings.assistantTitle, icon: "microphone")
                }
                .buttonStyle(RowButtonStyle())
            }
            if let server = chosen {
                ServerGroup(server: server, serverId: $serverId)
                    .buttonStyle(RowButtonStyle())
            }
            NavigationLink {
                AboutView()
            } label: {
                EntryLabel(MavioStrings.settingsAbout, icon: "info")
            }
            .buttonStyle(RowButtonStyle())
            .padding(.top, MavioSpacing.s32)
            // Signing the account out forgets its servers too.
            Button {
                Task { try? await model.signOut() }
            } label: {
                EntryLabel(MavioStrings.authSignOut, icon: "signOut")
            }
            .buttonStyle(RowButtonStyle())
        }
        .task(id: standalone?.userId) {
            if let server = standalone {
                switchable = (try? await model.client.switchableUsers(serverId: server.id)) ?? []
            }
        }
    }
}

/// A page of the settings: its title over its rows, in a column at the
/// screen's middle.
struct SettingsPage<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme

    init(_ title: String, @ViewBuilder content: () -> Content) {
        self.title = title
        self.content = content()
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(title).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
                    .padding(.bottom, MavioSpacing.s24)
                content
            }
            .frame(maxWidth: 1100, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(.vertical, MavioSpacing.s64)
        }
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// A section's row: its icon and title, and an arrow.
private struct EntryLabel: View {
    let title: String
    let icon: String
    @Environment(\.theme) private var theme

    init(_ title: String, icon: String) { (self.title, self.icon) = (title, icon) }

    var body: some View {
        HStack(spacing: MavioSpacing.s16) {
            Icon(icon, size: 32)
            Text(title).type(theme.type.headline)
            Spacer()
        }
        .padding(.vertical, MavioSpacing.s12)
    }
}

/// The user's card: the Mavio account, or the user of the server signed in to.
private struct AccountCard: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        let server = model.signedInServers.first
        let name = model.account?.name ?? server?.userName ?? ""
        HStack(spacing: MavioSpacing.s24) {
            Avatar(name: name, size: 80)
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                Text(name).type(theme.type.headline)
                Text(model.account != nil ? MavioStrings.authMavioAccount : server?.name ?? "")
                    .type(theme.type.footnote).opacity(0.7)
            }
            .lineLimit(1)
            Spacer()
        }
        .padding(.vertical, MavioSpacing.s16)
    }
}

/// The account: the Mavio account or the server's user, the servers in the
/// user's order, which orders Home, and authorizing a device with Quick
/// Connect. Nothing opens mavio.dev, a web page (§1).
private struct AccountView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var adding = false

    var body: some View {
        SettingsPage(MavioStrings.settingsAccount) {
            if let account = model.account {
                HStack(spacing: MavioSpacing.s24) {
                    Avatar(name: account.name, size: 80)
                    Text(account.name).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                }
                Note(MavioStrings.settingsMavioAccountMessage)
            } else if let server = model.signedInServers.first {
                HStack(spacing: MavioSpacing.s24) {
                    Avatar(name: server.userName, size: 80)
                    Text(server.userName).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                }
                Note(MavioStrings.settingsStandaloneAccountMessage(name: server.userName, server: server.name))
            }
            Heading(MavioStrings.authServers)
            ForEach(Array(model.servers.enumerated()), id: \.element.id) { index, server in
                ServerRow(server: server, index: index, adding: $adding)
            }
            // A Mavio account follows its own servers, removing any other
            // the device signs in to.
            if model.account == nil {
                Button {
                    adding = true
                } label: {
                    EntryLabel(MavioStrings.authAddServer, icon: "add")
                }
                .buttonStyle(RowButtonStyle())
            }
            Note(model.account != nil ? MavioStrings.settingsServersMavioMessage : MavioStrings.settingsServersMessage)
            if !model.signedInServers.isEmpty {
                AuthorizeDevice()
            }
        }
        .fullScreenCover(isPresented: $adding) {
            SignInView(adding: true)
                .environment(\.theme, theme)
                .background(theme.colors.tvBackground.ignoresSafeArea())
        }
    }
}

/// A server with the user signed in to it and the state of its connection;
/// selecting it offers signing in again, moving it in the order, and
/// signing out of it.
private struct ServerRow: View {
    let server: ServerRecord
    let index: Int
    @Binding var adding: Bool
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        Menu {
            if model.account == nil, !model.signedIn.contains(server.id) {
                Button(MavioStrings.authSignIn) { adding = true }
            }
            if index > 0 {
                Button(MavioStrings.authMoveUp) { move(by: -1) }
            }
            if index < model.servers.count - 1 {
                Button(MavioStrings.authMoveDown) { move(by: 1) }
            }
            Button(MavioStrings.authSignOut, role: .destructive) {
                Task { try? await model.remove(server.id) }
            }
        } label: {
            HStack(spacing: MavioSpacing.s16) {
                Icon("server", size: 32)
                VStack(alignment: .leading, spacing: 2) {
                    Text(server.name).type(theme.type.headline)
                    Text(
                        model.signedIn.contains(server.id)
                            ? MavioStrings.authSignedInAs(name: server.userName) : MavioStrings.authSignedOut
                    )
                    .type(theme.type.footnote).opacity(0.7)
                    Text(server.address).type(theme.type.caption).opacity(0.5)
                }
                .lineLimit(1)
                Spacer()
                switch model.states[server.id] {
                case .online?: Circle().fill(theme.colors.success).frame(width: 12, height: 12)
                case .offline?: Icon("offline", size: 28).opacity(0.6)
                default: EmptyView()
                }
            }
            .padding(.vertical, MavioSpacing.s12)
        }
        .buttonStyle(RowButtonStyle())
    }

    private func move(by offset: Int) {
        var ids = model.servers.map(\.id)
        ids.swapAt(index, index + offset)
        model.order(ids)
    }
}

/// Approves the Quick Connect code another device shows, on the server
/// chosen when there are several.
private struct AuthorizeDevice: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var serverId = ""
    @State private var code = ""
    @State private var message: String?
    @State private var done = false

    var body: some View {
        Heading(MavioStrings.authAuthorizeDevice)
        if model.signedInServers.count > 1 {
            Picker(selection: $serverId) {
                ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
            } label: {
                Text(MavioStrings.authServer)
            }
        }
        TextField(text: $code, prompt: Text(verbatim: "000000")) { Text(MavioStrings.authQuickConnect) }
            .keyboardType(.numberPad)
            .onChange(of: code) { _, new in
                let digits = String(new.filter(\.isNumber).prefix(6))
                if digits != new { code = digits }
            }
        Button(MavioStrings.authAuthorize) {
            Task {
                do {
                    try await model.client.authorizeQuickConnect(serverId: serverId, code: code)
                    done = true
                    message = MavioStrings.authAuthorized
                    code = ""
                } catch {
                    done = false
                    message = error.reason
                }
            }
        }
        .buttonStyle(TVButtonStyle())
        .disabled(code.count < 6 || serverId.isEmpty)
        .onAppear { if serverId.isEmpty { serverId = model.signedInServers.first?.id ?? "" } }
        if let message {
            Text(message).type(theme.type.callout)
                .foregroundStyle(done ? theme.colors.textSecondary : theme.colors.danger)
        }
    }
}

/// The users of a standalone server as avatars, to switch to, with their
/// PIN when they have one, entered on a keypad.
private struct SwitchUsers: View {
    let server: ServerRecord
    let users: [SwitchableUser]
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var asking: SwitchableUser?
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        // Switching needs someone else to switch to.
        if users.count > 1 {
            Heading(MavioStrings.authSwitchUser)
            ScrollView(.horizontal) {
                HStack(alignment: .top, spacing: theme.layout.gutter) {
                    ForEach(users, id: \.id) { user in
                        Button {
                            if user.hasPin { asking = user } else { Task { await switchTo(user, pin: nil) } }
                        } label: {
                            MemberCard(user: user)
                        }
                        .buttonStyle(PersonButtonStyle())
                        .disabled(user.current || busy)
                    }
                }
                .padding(.vertical, MavioSpacing.s24)
            }
            .scrollClipDisabled()
            .focusSection()
            if let failure, asking == nil {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            }
            EmptyView()
                .fullScreenCover(item: $asking) { user in
                    PinPad(name: user.name, busy: busy, error: failure) { pin in
                        Task { await switchTo(user, pin: pin) }
                    }
                    .environment(\.theme, theme)
                    .onDisappear { failure = nil }
                }
        }
    }

    private func switchTo(_ user: SwitchableUser, pin: String?) async {
        busy = true
        failure = nil
        defer { busy = false }
        do {
            try await model.switchUser(server.id, to: user.id, pin: pin)
            asking = nil
        } catch {
            failure = error.reason
        }
    }
}

/// A user to switch to: their initial, ringed when current, with a lock
/// when they have a PIN, over their name.
private struct MemberCard: View {
    let user: SwitchableUser
    @Environment(\.cardFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(spacing: MavioSpacing.s12) {
            Avatar(name: user.name, size: 120)
                .padding(4)
                .overlay {
                    if user.current { Circle().strokeBorder(theme.colors.accent, lineWidth: 4) }
                }
                .overlay(alignment: .bottomTrailing) {
                    if user.hasPin {
                        Icon("lock", fill: true, size: 24)
                            .foregroundStyle(theme.colors.textPrimary)
                            .padding(MavioSpacing.s8)
                            .background(theme.colors.surfaceRaised, in: .circle)
                    }
                }
            Text(user.name).type(theme.type.callout).fontWeight(.semibold).lineLimit(1)
                .foregroundStyle(focused ? .black : theme.colors.textPrimary)
        }
        .frame(width: 168)
    }
}

extension SwitchableUser: @retroactive Identifiable {}

/// Asks for a user's four-digit PIN on a keypad the remote moves over;
/// the fourth digit sends it, a wrong one clears the dots, and back cancels.
private struct PinPad: View {
    let name: String
    let busy: Bool
    let error: String?
    let enter: (String) -> Void
    @Environment(\.theme) private var theme
    @State private var pin = ""

    var body: some View {
        VStack(spacing: MavioSpacing.s32) {
            Avatar(name: name, size: 120)
            Text(MavioStrings.authEnterPin(name: name)).type(theme.type.title2)
                .foregroundStyle(theme.colors.textPrimary)
            HStack(spacing: MavioSpacing.s24) {
                ForEach(0..<4, id: \.self) { i in
                    Circle()
                        .strokeBorder(theme.colors.textPrimary, lineWidth: 3)
                        .background(Circle().fill(i < pin.count ? theme.colors.textPrimary : .clear))
                        .frame(width: 24, height: 24)
                }
            }
            .accessibilityElement()
            .accessibilityLabel(MavioStrings.authPin)
            .accessibilityValue("\(pin.count)/4")
            Text(error ?? " ").type(theme.type.callout).foregroundStyle(theme.colors.danger)
            Grid(horizontalSpacing: MavioSpacing.s24, verticalSpacing: MavioSpacing.s24) {
                ForEach([["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"], ["", "0", "⌫"]], id: \.self) { row in
                    GridRow {
                        ForEach(row, id: \.self) { key($0) }
                    }
                }
            }
            .focusSection()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(theme.colors.tvBackground.ignoresSafeArea())
        .onChange(of: error) { _, new in if new != nil { pin = "" } }
    }

    @ViewBuilder private func key(_ k: String) -> some View {
        if k.isEmpty {
            Color.clear.frame(width: 96, height: 96)
        } else {
            Button {
                press(k)
            } label: {
                Group {
                    if k == "⌫" {
                        Icon("backspace", size: 36).accessibilityLabel(MavioStrings.searchBackspace)
                    } else {
                        Text(k).font(.system(size: 40, weight: .medium, design: .rounded))
                    }
                }
                .frame(width: 96, height: 96)
            }
            .buttonStyle(RoundButtonStyle())
            .disabled(busy)
        }
    }

    private func press(_ k: String) {
        if k == "⌫" {
            pin = String(pin.dropLast())
        } else if pin.count < 4 {
            pin += k
            if pin.count == 4 { enter(pin) }
        }
    }
}

/// A person's initial in a circle, dark on a focused row's white.
private struct Avatar: View {
    let name: String
    let size: CGFloat
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        Text(name.first.map { String($0).uppercased() } ?? "")
            .font(.system(size: size * 0.42, weight: .semibold))
            .foregroundStyle(focused ? .black : theme.colors.textPrimary)
            .frame(width: size, height: size)
            .background(focused ? .black.opacity(0.08) : theme.colors.fill, in: .circle)
    }
}

/// This device's playback: the quality asked for and playing the next
/// episode on.
private struct PlaybackView: View {
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true

    var body: some View {
        SettingsPage(MavioStrings.settingsPlayback) {
            Picker(selection: $quality) {
                ForEach(PlaybackQuality.allCases, id: \.self) { Text($0.label).tag($0) }
            } label: {
                Text(MavioStrings.settingsMaxQuality)
            }
            Toggle(MavioStrings.settingsAutoplayNext, isOn: $autoplayNext)
        }
    }
}

/// Subtitles and Audio, as the web UI's: the user's languages and when
/// subtitles show, read from the first server signed in to and saved to
/// each of them.
private struct SubtitlesAudioView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var saved: PlaybackPreferences?
    @State private var audio = ""
    @State private var subtitles = ""
    @State private var mode = SubtitleMode.default
    @State private var failure: String?
    @State private var saving = false

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
        SettingsPage(MavioStrings.settingsSubtitlesAudio) {
            if saved != nil {
                Note(MavioStrings.settingsSubtitlesAudioMessage)
                field(MavioStrings.settingsAudioLanguages, text: $audio)
                field(MavioStrings.settingsSubtitleLanguages, text: $subtitles)
                Note(MavioStrings.settingsLanguagesHint)
                Picker(selection: $mode) {
                    ForEach(Self.modes, id: \.0) { Text($0.1).tag($0.0) }
                } label: {
                    Text(MavioStrings.settingsSubtitleMode)
                }
                Button(MavioStrings.actionSave, action: save)
                    .buttonStyle(TVButtonStyle())
                    .disabled(
                        saving || !changed || languageCodes(audio) == nil || languageCodes(subtitles) == nil)
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity)
            }
            if let failure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            }
        }
        .task { if saved == nil { await load() } }
    }

    private func field(_ label: String, text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            Text(label).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
            TextField(text: text, prompt: Text(verbatim: "chi, eng")) { Text(label) }
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
                .foregroundStyle(languageCodes(text.wrappedValue) == nil ? theme.colors.danger : .primary)
        }
    }

    private var changed: Bool {
        guard let saved else { return false }
        return languageCodes(audio) != saved.audioLanguages
            || languageCodes(subtitles) != saved.subtitleLanguages || mode != saved.subtitleMode
    }

    private func load() async {
        do {
            show(try await model.client.playbackPreferences())
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func show(_ prefs: PlaybackPreferences) {
        saved = prefs
        audio = prefs.audioLanguages.joined(separator: ", ")
        subtitles = prefs.subtitleLanguages.joined(separator: ", ")
        mode = prefs.subtitleMode
    }

    private func save() {
        guard var prefs = saved, let a = languageCodes(audio), let s = languageCodes(subtitles) else { return }
        prefs.audioLanguages = a
        prefs.subtitleLanguages = s
        prefs.subtitleMode = mode
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            do {
                try await model.client.setPlaybackPreferences(preferences: prefs)
                show(prefs)
            } catch {
                failure = error.reason
            }
        }
    }
}

/// About, as the Android apps' and the Mac's: the app — its name as the
/// servers know it, its version and build, the core's version and the
/// license — and the Apple TV — its name and ID as the servers know them,
/// its maker, model, system and architecture. Its rows take focus, so that
/// the remote scrolls through them.
private struct AboutView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        let device = model.device
        SettingsPage(MavioStrings.settingsAbout) {
            Heading(MavioStrings.aboutApp)
            row(MavioStrings.aboutAppName, device.client)
            row(MavioStrings.aboutClientVersion, Bundle.main.mavioVersionAndBuild)
            row(MavioStrings.aboutCoreVersion, coreVersion)
            row(MavioStrings.aboutLicense, "GPL-3.0")
            Heading(MavioStrings.aboutDevice)
            row(MavioStrings.aboutDeviceName, device.name)
            row(MavioStrings.aboutDeviceId, device.id)
            row(MavioStrings.aboutManufacturer, "Apple")
            row(MavioStrings.aboutModel, Self.model)
            row(MavioStrings.aboutSystem, "tvOS \(UIDevice.current.systemVersion)")
            row(MavioStrings.aboutArchitecture, Self.architecture)
            Note(MavioStrings.aboutDeviceFooter)
        }
    }

    private func row(_ title: String, _ value: String) -> some View {
        Button {
        } label: {
            HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s24) {
                Text(title).fixedSize()
                // The value wraps rather than shortening.
                Text(value).opacity(0.7).multilineTextAlignment(.trailing)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .trailing)
            }
            .padding(.vertical, MavioSpacing.s8)
        }
        .buttonStyle(RowButtonStyle())
    }

    /// The Apple TV's model identifier, such as AppleTV14,1.
    private static var model: String {
        var system = utsname()
        uname(&system)
        return withUnsafeBytes(of: &system.machine) { bytes in
            String(decoding: bytes.prefix { $0 != 0 }, as: UTF8.self)
        }
    }

    /// The architecture the app runs on.
    private static var architecture: String {
        #if arch(arm64)
            "arm64"
        #else
            "x86_64"
        #endif
    }
}

/// A note under a group of rows.
struct Note: View {
    let text: String
    @Environment(\.theme) private var theme

    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text).type(theme.type.callout).foregroundStyle(theme.colors.textTertiary)
            .fixedSize(horizontal: false, vertical: true)
    }
}

/// A group's heading.
struct Heading: View {
    let text: String
    @Environment(\.theme) private var theme

    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text).type(theme.type.title3).foregroundStyle(theme.colors.textSecondary)
            .padding(.top, MavioSpacing.s32)
    }
}
