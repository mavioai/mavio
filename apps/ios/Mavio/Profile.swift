import MavioKit
import MavioUI
import SwiftUI

/// The settings, as the web UI's on a phone: the account's card, opening
/// the account with its servers; this device's sections; the version; and
/// signing out.
struct ProfileView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var serverId = ""

    /// The server whose settings the Server group shows.
    private var chosen: ServerRecord? {
        model.signedInServers.first { $0.id == serverId } ?? model.signedInServers.first
    }

    var body: some View {
        List {
            Section {
                NavigationLink {
                    AccountView()
                } label: {
                    AccountCard()
                }
            }
            if model.account == nil, let server = model.signedInServers.first {
                SwitchUserSection(server: server)
            }
            Section(MavioStrings.settingsThisDevice) {
                NavigationLink {
                    AppearanceView()
                } label: {
                    EntryLabel(MavioStrings.settingsAppearance, icon: "appearance", color: Color(hex: 0x5E5CE6))
                }
                NavigationLink {
                    PlaybackSettingsView()
                } label: {
                    EntryLabel(MavioStrings.settingsPlayback, icon: "play", color: Color(hex: 0xFF3B30))
                }
                if !model.signedInServers.isEmpty {
                    NavigationLink {
                        SubtitlesAudioView()
                    } label: {
                        EntryLabel(MavioStrings.settingsSubtitlesAudio, icon: "subtitles", color: Color(hex: 0x34C759))
                    }
                }
            }
            if let server = chosen {
                Section {
                    if model.signedInServers.count > 1 {
                        Picker(MavioStrings.authServer, selection: Binding(get: { server.id }, set: { serverId = $0 }))
                        {
                            ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
                        }
                        .tint(theme.colors.textSecondary)
                    }
                    NavigationLink {
                        DevicesView(server: server)
                    } label: {
                        EntryLabel(MavioStrings.settingsDevices, icon: "devices", color: Color(hex: 0x32ADE6))
                    }
                } header: {
                    Text(MavioStrings.settingsServerGroup)
                }
            }
            Section {
                LabeledContent(
                    "Mavio",
                    value: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "")
                LabeledContent("Core", value: MavioKit.coreVersion)
            }
            Section {
                // Signing the account out forgets its servers too.
                Button(MavioStrings.authSignOut, role: .destructive) {
                    Task { try? await model.signOut() }
                }
                .frame(maxWidth: .infinity)
            }
        }
        .scrollContentBackground(.hidden)
        .navigationTitle(MavioStrings.navProfile)
    }
}

/// The user's card: the Mavio account, or the user of the server signed in
/// to.
private struct AccountCard: View {
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
private struct AccountView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme

    var body: some View {
        List {
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
                SwitchUserSection(server: server)
                SecuritySections(server: server)
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
                            if !model.signedIn.contains(server.id) {
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
                Button(MavioStrings.authAddServer) { router.addingServer = true }
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
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsAccount)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar { EditButton() }
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
                    .keyboardType(.numberPad)
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

/// A section's row: its icon on a colored tile, and its title.
struct EntryLabel: View {
    let title: String
    let icon: String
    let color: Color
    @Environment(\.theme) private var theme

    init(_ title: String, icon: String, color: Color) {
        (self.title, self.icon, self.color) = (title, icon, color)
    }

    var body: some View {
        Label {
            Text(title)
        } icon: {
            Icon(icon, fill: true, size: 17)
                .foregroundStyle(.white)
                .frame(width: 28, height: 28)
                .background(color, in: .rect(cornerRadius: 7))
        }
    }
}

/// A person's initial in a circle.
struct Avatar: View {
    let name: String
    let size: CGFloat
    @Environment(\.theme) private var theme

    var body: some View {
        Text(name.first.map { String($0).uppercased() } ?? "")
            .font(.system(size: size * 0.42, weight: .semibold))
            .foregroundStyle(theme.colors.textPrimary)
            .frame(width: size, height: size)
            .background(theme.colors.fill, in: .circle)
    }
}

extension Color {
    /// A color of its sRGB hex value, as the web UI writes the tiles'.
    init(hex: UInt32) {
        self.init(
            .sRGB, red: Double(hex >> 16 & 0xFF) / 255, green: Double(hex >> 8 & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255)
    }
}
