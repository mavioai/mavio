import MavioAdmin
import MavioKit
import MavioUI
import SwiftUI

/// The settings, as the web UI's on a phone, opened from the avatar at the
/// top right: the account's card, opening the account with its servers;
/// this device's sections, with the downloads; About; and signing
/// out.
public struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var serverId = ""
    @State private var switchable: [SwitchableUser] = []

    /// The server whose settings the Server group shows.
    private var chosen: ServerRecord? {
        model.signedInServers.first { $0.id == serverId } ?? model.signedInServers.first
    }

    /// The settings.
    public init() {}

    /// The view's content.
    public var body: some View {
        SettingsList {
            Section {
                NavigationLink {
                    AccountView()
                } label: {
                    AccountCard()
                }
                .rowLink()
            }
            if let server = standalone {
                SwitchUserSection(server: server, users: switchable)
            }
            Section(MavioStrings.settingsThisDevice) {
                NavigationLink {
                    AppearanceView()
                } label: {
                    EntryLabel(MavioStrings.settingsAppearance, icon: "appearance", color: Color(hex: 0x5E5CE6))
                }
                .rowLink()
                NavigationLink {
                    PlaybackSettingsView()
                } label: {
                    EntryLabel(MavioStrings.settingsPlayback, icon: "play", color: Color(hex: 0xFF3B30))
                }
                .rowLink()
                if !model.signedInServers.isEmpty {
                    NavigationLink {
                        SubtitlesAudioView()
                    } label: {
                        EntryLabel(MavioStrings.settingsSubtitlesAudio, icon: "subtitles", color: Color(hex: 0x34C759))
                    }
                    .rowLink()
                }
                if let server = chosen {
                    NavigationLink {
                        AssistantSettingsView(server: server)
                    } label: {
                        EntryLabel(MavioStrings.assistantTitle, icon: "microphone", color: Color(hex: 0xFF2D55))
                    }
                    .rowLink()
                }
                NavigationLink(value: Route.downloads) {
                    EntryLabel(MavioStrings.navDownloads, icon: "downloads", color: Color(hex: 0x0A84FF))
                }
                .rowLink()
            }
            if let server = chosen {
                ServerGroup(server: server, serverId: $serverId)
            }
            Section {
                NavigationLink {
                    AboutSettingsView()
                } label: {
                    EntryLabel(MavioStrings.settingsAbout, icon: "info", color: Color(hex: 0x8E8E93))
                }
                .rowLink()
            }
            Section {
                // Signing the account out forgets its servers too.
                Button(MavioStrings.authSignOut, role: .destructive) {
                    Task { try? await model.signOut() }
                }
                .frame(maxWidth: .infinity)
            }
        }
        .hiddenListBackground()
        .navigationTitle(MavioStrings.navSettings)
        // Large at the top left wherever it is opened from, centered small
        // in the bar once scrolled.
        .largeTitle()
        .task(id: standalone?.userId) {
            if let server = standalone { switchable = await SwitchUserSection.load(model, server) }
        }
    }

    /// The server a device without a Mavio account signs in to.
    private var standalone: ServerRecord? {
        model.account == nil ? model.signedInServers.first : nil
    }
}

/// The user's card: the Mavio account, or the user of the server signed in
/// to.
struct AccountCard: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        let server = model.signedInServers.first
        let name = model.account?.name ?? server?.userName ?? ""
        HStack(spacing: MavioSpacing.s12) {
            Avatar(name: name, size: 44)
            VStack(alignment: .leading, spacing: 2) {
                Text(name).type(theme.type.headline).lineLimit(1)
                Text(model.account != nil ? MavioStrings.authMavioAccount : server?.name ?? "")
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
                    .lineLimit(1)
            }
        }
        .padding(.vertical, 2)
    }
}

/// The account: the Mavio account with its page at mavio.dev, or the user
/// of the server; the servers in the user's order, which orders Home;
/// and authorizing a device with Quick Connect.
struct AccountView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @State private var switchable: [SwitchableUser] = []
    @State private var security: AccountSecurity?

    /// The server a device without a Mavio account signs in to.
    private var standalone: ServerRecord? {
        model.account == nil ? model.signedInServers.first : nil
    }

    var body: some View {
        SettingsList {
            if let account = model.account {
                Section {
                    HStack(spacing: MavioSpacing.s12) {
                        Avatar(name: account.name, size: 44)
                        Text(account.name).type(theme.type.headline)
                    }
                    if let manage = Self.manage(account) {
                        Link(destination: manage) {
                            Label {
                                Text(MavioStrings.settingsManageAtMavio)
                            } icon: {
                                Icon("external", size: 18)
                            }
                        }
                    }
                } footer: {
                    Text(MavioStrings.settingsMavioAccountMessage)
                }
            } else if let server = model.signedInServers.first {
                Section {
                    HStack(spacing: MavioSpacing.s12) {
                        Avatar(name: server.userName, size: 44)
                        Text(server.userName).type(theme.type.headline)
                    }
                } footer: {
                    Text(MavioStrings.settingsStandaloneAccountMessage(name: server.userName, server: server.name))
                }
                SwitchUserSection(server: server, users: switchable)
                SecuritySections(server: server, security: $security)
            }
            Section {
                ForEach(model.servers, id: \.id) { server in
                    ServerRow(server: server)
                        .swipeActions {
                            Button(MavioStrings.authSignOut, role: .destructive) {
                                Task { try? await model.remove(server.id) }
                            }
                        }
                        .contextMenu {
                            if model.account == nil, !model.signedIn.contains(server.id) {
                                Button(MavioStrings.authSignIn) { router.addingServer = true }
                            }
                            Button(MavioStrings.authSignOut, role: .destructive) {
                                Task { try? await model.remove(server.id) }
                            }
                        }
                }
                .onMove { from, to in
                    var ids = model.servers.map(\.id)
                    ids.move(fromOffsets: from, toOffset: to)
                    model.order(ids)
                }
                // A Mavio account follows its own servers, removing any other
                // the device signs in to.
                if model.account == nil {
                    Button(MavioStrings.authAddServer) { router.addingServer = true }
                }
            } header: {
                Text(MavioStrings.authServers)
            } footer: {
                Text(
                    model.account != nil
                        ? MavioStrings.settingsServersMavioMessage : MavioStrings.settingsServersMessage)
            }
            if !model.signedInServers.isEmpty {
                AuthorizeDevice()
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsAccount)
        .inlineTitle()
        #if os(iOS)
            .toolbar { EditButton() }
        #endif
        .task(id: standalone?.userId) {
            guard let server = standalone else { return }
            async let users = SwitchUserSection.load(model, server)
            async let security = SecuritySections.load(model, server)
            (switchable, self.security) = await (users, security)
        }
    }

    /// The account's page at the mavio.dev whose API it signed in with.
    private static func manage(_ account: Account) -> URL? {
        guard var site = URL(string: account.api) else { return nil }
        if site.lastPathComponent == "api" { site.deleteLastPathComponent() }
        return site.appending(path: "account")
    }
}

/// A server with the user signed in to it and the state of its connection.
private struct ServerRow: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(server.name).type(theme.type.headline)
                Text(
                    model.signedIn.contains(server.id)
                        ? MavioStrings.authSignedInAs(name: server.userName) : MavioStrings.authSignedOut
                )
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.textSecondary)
                Text(server.address).type(theme.type.caption).foregroundStyle(theme.colors.textTertiary)
            }
            Spacer()
            switch model.states[server.id] {
            case .online?: Circle().fill(theme.colors.success).frame(width: 8, height: 8)
            case .offline?: Icon("offline", size: 18).foregroundStyle(theme.colors.textTertiary)
            default: EmptyView()
            }
        }
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
        Section(MavioStrings.authAuthorizeDevice) {
            if model.signedInServers.count > 1 {
                Picker(MavioStrings.authServer, selection: $serverId) {
                    ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
                }
            }
            HStack {
                TextField(MavioStrings.authQuickConnect, text: $code, prompt: Text("000000"))
                    .fieldLabelHidden()
                    .numberKeyboard()
                    .font(.system(.body, design: .monospaced))
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
                            message = error.localizedDescription
                        }
                    }
                }
                .disabled(code.count < 6 || serverId.isEmpty)
            }
            if let message {
                Text(message)
                    .type(theme.type.callout)
                    .foregroundStyle(done ? theme.colors.textSecondary : theme.colors.danger)
            }
        }
        .onAppear { if serverId.isEmpty { serverId = model.signedInServers.first?.id ?? "" } }
    }
}
