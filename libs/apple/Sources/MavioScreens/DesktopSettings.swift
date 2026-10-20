#if os(macOS)
    import MavioAdmin
    import MavioKit
    import MavioUI
    import SwiftUI

    /// A page of the settings on the desktop.
    enum SettingsPage: Hashable {
        case account, appearance, playback, subtitlesAudio
        case server(ServerSection)
    }

    /// The settings as the web UI's in a wide window (docs/clients.md §6.3):
    /// beside their own sidebar, which returns Home and lists the account,
    /// this device's sections and the sections of the server chosen, its
    /// menus opening in place; the section chosen, centered and at most 900
    /// points wide.
    public struct DesktopSettingsView: View {
        let leave: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var page = SettingsPage.appearance
        @State private var serverId = ""
        /// Whether a server's user administers it, by the server and user.
        @State private var admin: [String: Bool] = [:]

        /// The settings, leave returning Home.
        public init(leave: @escaping () -> Void) {
            self.leave = leave
        }

        /// The server whose sections the Server group shows.
        private var chosen: ServerRecord? {
            model.signedInServers.first { $0.id == serverId } ?? model.signedInServers.first
        }

        private var key: String { chosen.map { "\($0.id)/\($0.userId)" } ?? "" }

        /// The view's content.
        public var body: some View {
            NavigationSplitView {
                sidebar.navigationSplitViewColumnWidth(min: 220, ideal: 260, max: 400)
            } detail: {
                NavigationStack {
                    detail
                        .environment(\.pageTitle, title)
                        .environment(\.itemPage) { server, itemId in
                            AnyView(
                                RouteDestination(route: .item(serverId: server.id, itemId: itemId))
                                    .background(theme.colors.background))
                        }
                        .toolbar(removing: .title)
                        .background(theme.colors.background)
                        .navigationDestination(for: Route.self) { route in
                            RouteDestination(route: route).background(theme.colors.background)
                        }
                }
                .id(DetailKey(page: page, server: key))
            }
            .task(id: key) {
                guard let server = chosen else { return }
                if let known = await ServerGroup.administers(model, server) { admin[key] = known }
            }
            .onChange(of: admin[key]) {
                // A section the user no longer sees gives way to the first.
                if case .server(let section) = page, !shown(section) { page = .appearance }
            }
        }

        private var sidebar: some View {
            List(selection: Binding(get: { page }, set: { if let new = $0 { page = new } })) {
                Section {
                    AccountCard().padding(.leading, MavioSpacing.s4).tag(SettingsPage.account)
                }
                // The advanced tools and the AI conversations, for the
                // server's administrators.
                if admin[key] == true {
                    Section(MavioStrings.settingsAdvancedGroup) {
                        row(.indexerSearch)
                        row(.conversations)
                    }
                }
                Section(MavioStrings.settingsThisDevice) {
                    row(.appearance, MavioStrings.settingsAppearance, "appearance", Color(hex: 0x5E5CE6))
                    row(.playback, MavioStrings.settingsPlayback, "play", Color(hex: 0xFF3B30))
                    if !model.signedInServers.isEmpty {
                        row(.subtitlesAudio, MavioStrings.settingsSubtitlesAudio, "subtitles", Color(hex: 0x34C759))
                    }
                }
                if let server = chosen {
                    Section(MavioStrings.settingsServerGroup) {
                        // A Mavio account chooses among its servers; a
                        // standalone user is signed in to one.
                        if model.account != nil, model.signedInServers.count > 1 {
                            Picker(
                                MavioStrings.authServer,
                                selection: Binding(get: { server.id }, set: { serverId = $0 })
                            ) {
                                ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
                            }
                            .labelsHidden()
                            .selectionDisabled()
                        }
                        // The menus' sections in their place, one level.
                        ForEach(serverSections, id: \.self) { row($0) }
                    }
                }
            }
            .listStyle(.sidebar)
            // Rows apart, as the web UI's.
            .environment(\.defaultMinListRowHeight, 32)
            .modifier(
                TopBar {
                    VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                        Button(action: leave) {
                            Label {
                                Text(MavioStrings.navHome)
                            } icon: {
                                Icon("back", size: 16)
                            }
                        }
                        .buttonStyle(.plain)
                        .foregroundStyle(theme.colors.accent)
                        .keyboardShortcut(.escape, modifiers: [])
                        Text(MavioStrings.navSettings).type(theme.type.title1)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, MavioSpacing.s24)
                    .padding(.bottom, MavioSpacing.s8)
                })
        }

        private func row(_ page: SettingsPage, _ title: String, _ icon: String, _ color: Color) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                Icon(icon, fill: true, size: 17)
                    .foregroundStyle(.white)
                    .frame(width: 28, height: 28)
                    .background(color, in: .rect(cornerRadius: 7))
                Text(title).type(theme.type.body)
            }
            .padding(.leading, MavioSpacing.s4)
            .padding(.vertical, 1)
            .tag(page)
        }

        private func row(_ section: ServerSection) -> some View {
            row(.server(section), section.title, section.icon, section.color)
        }

        /// The sections of the Server group the user sees, its menus' among them.
        private var serverSections: [ServerSection] {
            ServerEntry.shown(admin: admin[key] == true).flatMap { entry in
                switch entry {
                case .section(let section): [section]
                case .menu(let menu): menu.sections(admin: true)
                }
            }
        }

        private func shown(_ section: ServerSection) -> Bool {
            section.forAnyone || admin[key] == true
        }

        /// The title of the page chosen.
        private var title: String {
            switch page {
            case .account: MavioStrings.settingsAccount
            case .appearance: MavioStrings.settingsAppearance
            case .playback: MavioStrings.settingsPlayback
            case .subtitlesAudio: MavioStrings.settingsSubtitlesAudio
            case .server(let section): section.title
            }
        }

        @ViewBuilder private var detail: some View {
            switch page {
            case .account: AccountView()
            case .appearance: AppearanceView()
            case .playback: PlaybackSettingsView()
            case .subtitlesAudio: SubtitlesAudioView()
            case .server(let section):
                if let server = chosen { section.view(server) }
            }
        }
    }

    /// A bar over a scroll view's top that the rows scroll under, blurred as
    /// the system's bars where it can, else on the window's background.
    private struct TopBar<Bar: View>: ViewModifier {
        @ViewBuilder let bar: Bar

        func body(content: Content) -> some View {
            if #available(macOS 26, *) {
                content.safeAreaBar(edge: .top, spacing: 0) { bar }
            } else {
                content.safeAreaInset(edge: .top, spacing: 0) {
                    bar.background(.background)
                }
            }
        }
    }

    /// What the detail's stack shows: another page, or the same section of
    /// another server, starts it anew.
    private struct DetailKey: Hashable {
        let page: SettingsPage
        let server: String
    }

#endif
