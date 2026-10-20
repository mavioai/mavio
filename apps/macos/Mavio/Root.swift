import MavioAdmin
import MavioKit
import MavioScreens
import MavioUI
import SwiftUI

/// A place of the sidebar (docs/clients.md §6.3, Desktop).
enum Place: Hashable {
    case search, home, discover, downloads, settings
    case library(LibraryEntry)
}

/// The main window's state: the place shown, and the pages each place
/// opened, kept as the user moves among them.
@MainActor
@Observable
final class Shell {
    var place: Place = .home
    var paths: [Place: [Route]] = [:]
    /// Whether the user's menu is open over the window.
    var userMenu = false
    /// Whether the settings open on the account's page.
    var settingsAccount = false

    /// Opens a link of the app: `mavio://item/<server>/<item>` shows an
    /// item's details on Home, `mavio://person/<server>/<person>` a person.
    func open(_ url: URL) {
        let parts = url.pathComponents.filter { $0 != "/" }
        guard url.scheme == "mavio", parts.count == 2 else { return }
        let (serverId, id) = (parts[0], parts[1])
        let route: Route? =
            switch url.host() {
            case "item": .item(serverId: serverId, itemId: id)
            case "person": .person(serverId: serverId, personId: id)
            default: nil
            }
        guard let route else { return }
        place = .home
        paths[.home, default: []].append(route)
    }
}

/// Signing in until a server is added, then the desktop layout: the
/// sidebar with Search, Home, Discover, Downloads and the libraries, the
/// user's menu at its foot, and the place chosen beside it.
struct RootView: View {
    /// Opens a playback in a window of its own.
    let play: (PlayRequest) -> Void
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(Shell.self) private var shell
    @Environment(\.colorScheme) private var scheme
    @State private var discovery = Discovery()
    @AppStorage(DeviceSetting.appearance) private var appearance = Appearance.system

    private var theme: Theme {
        Theme(colors: scheme == .light ? .light : .dark, layout: .desktop, type: .desktop)
    }

    var body: some View {
        @Bindable var router = router
        Group {
            if case .signedOut = model.signInState {
                SignInView()
            } else if shell.place == .settings {
                // The settings beside their own sidebar, which returns Home.
                DesktopSettingsView(account: shell.settingsAccount) {
                    shell.settingsAccount = false
                    shell.place = .home
                }
            } else {
                split
            }
        }
        .environment(discovery)
        .environment(\.theme, theme)
        .environment(\.remoteControl) { server, sessionId in
            AnyView(RemoteView(server: server, sessionId: sessionId).frame(minWidth: 420, minHeight: 520))
        }
        .tint(theme.colors.accent)
        .sheet(isPresented: $router.addingServer) {
            SignInView(adding: true).environment(\.theme, theme).frame(minWidth: 480, minHeight: 600)
        }
        .onChange(of: router.playing) { _, request in
            guard let request else { return }
            router.playing = nil
            play(request)
        }
        .onChange(of: appearance) { Appearance.apply() }
        .task(id: Discovery.key(model)) { await discovery.load(model) }
    }

    private var split: some View {
        NavigationSplitView {
            Sidebar()
                .navigationSplitViewColumnWidth(min: 200, ideal: 240, max: 400)
        } detail: {
            PlaceStack(place: shell.place)
                .id(shell.place)
        }
        // The user's menu over the window, as the web UI's popup: above
        // its button, past the sidebar's edge where it must; a click
        // elsewhere or Escape closes it.
        .overlay(alignment: .bottomLeading) {
            if shell.userMenu {
                ZStack(alignment: .bottomLeading) {
                    Color.black.opacity(0.001)
                        .onTapGesture { shell.userMenu = false }
                    UserMenu()
                        .padding(.leading, MavioSpacing.s8)
                        .padding(.bottom, 64)
                        .transition(.scale(scale: 0.95, anchor: .bottomLeading).combined(with: .opacity))
                }
                .ignoresSafeArea()
            }
        }
        .animation(.easeOut(duration: 0.1), value: shell.userMenu)
    }
}

/// A place's navigation stack, which opens the pages below it.
private struct PlaceStack: View {
    let place: Place
    @Environment(Shell.self) private var shell
    @Environment(\.theme) private var theme

    var body: some View {
        NavigationStack(path: path) {
            root
                .background(theme.colors.background)
                .navigationDestination(for: Route.self) { route in
                    RouteDestination(route: route).background(theme.colors.background)
                }
        }
        .tint(theme.colors.textPrimary)
    }

    private var path: Binding<[Route]> {
        Binding(get: { shell.paths[place] ?? [] }, set: { shell.paths[place] = $0 })
    }

    @ViewBuilder private var root: some View {
        switch place {
        case .search: SearchView()
        case .home: HomeView()
        case .discover: DiscoverView()
        case .downloads: RouteDestination(route: .downloads)
        case .settings: EmptyView()
        case let .library(library): RouteDestination(route: .library(library))
        }
    }
}

/// The sidebar: Search, Home, Discover where a server discovers, Downloads,
/// and the libraries of every server, by server with several; the user's
/// menu at its foot.
private struct Sidebar: View {
    @Environment(AppModel.self) private var model
    @Environment(Shell.self) private var shell
    @Environment(Discovery.self) private var discovery
    @State private var libraries: [String: [LibraryEntry]] = [:]

    var body: some View {
        @Bindable var shell = shell
        List(selection: selection) {
            row(.search, MavioStrings.navSearch, "search")
            row(.home, MavioStrings.navHome, "home")
            if discovery.server != nil { row(.discover, MavioStrings.navDiscover, "discover") }
            row(.downloads, MavioStrings.navDownloads, "downloads")
            Section(MavioStrings.navLibraries) {
                ForEach(model.signedInServers, id: \.id) { server in
                    let list = libraries[server.id] ?? []
                    if model.signedInServers.count > 1 && !list.isEmpty {
                        Text(server.name).font(.caption).foregroundStyle(.secondary)
                            .selectionDisabled()
                    }
                    ForEach(list, id: \.id) { library in
                        row(.library(library), library.name, library.kind.icon)
                    }
                }
            }
        }
        .listStyle(.sidebar)
        .safeAreaInset(edge: .bottom, spacing: 0) {
            VStack(spacing: 0) {
                Divider()
                UserButton().padding(MavioSpacing.s8)
            }
        }
        .task(id: model.signedInServers.map { "\($0.id):\(model.revision($0.id))" }) { await load() }
    }

    /// The place chosen; choosing the place shown returns to its top.
    private var selection: Binding<Place?> {
        Binding(
            get: { shell.place },
            set: { place in
                guard let place else { return }
                if place == shell.place { shell.paths[place] = [] }
                shell.place = place
            })
    }

    private func row(_ place: Place, _ title: String, _ icon: String) -> some View {
        Label {
            Text(title)
        } icon: {
            Icon(icon, fill: shell.place == place, size: 18)
        }
        .tag(place)
    }

    private func load() async {
        await withTaskGroup(of: (String, [LibraryEntry]?).self) { group in
            for server in model.signedInServers {
                let client = model.client
                group.addTask { (server.id, try? await client.libraries(serverId: server.id).libraries) }
            }
            for await (id, list) in group {
                if let list { libraries[id] = list }
            }
        }
    }
}

/// The user's name and what they are signed in to, as the web UI's.
@MainActor
private func signedIn(_ model: AppModel) -> (name: String, line: String) {
    let name = model.account?.name ?? model.signedInServers.first?.userName ?? ""
    let line =
        model.account != nil
        ? MavioStrings.menuServerCount(count: model.signedInServers.count)
        : model.signedInServers.first?.name ?? ""
    return (name, line)
}

/// The user's avatar and name at the sidebar's foot, opening their menu.
private struct UserButton: View {
    @Environment(AppModel.self) private var model
    @Environment(Shell.self) private var shell
    @Environment(\.theme) private var theme
    @State private var hovered = false

    var body: some View {
        let (name, line) = signedIn(model)
        Button {
            shell.userMenu.toggle()
        } label: {
            HStack(spacing: 10) {
                Avatar(name: name, size: 32)
                VStack(alignment: .leading, spacing: 0) {
                    Text(name).type(theme.type.callout).fontWeight(.semibold).lineLimit(1)
                    Text(line).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                }
                Spacer(minLength: 0)
                Icon("selector", size: 16).foregroundStyle(theme.colors.textTertiary)
            }
            .padding(MavioSpacing.s8)
            .background(
                hovered || shell.userMenu ? theme.colors.fill : .clear, in: .rect(cornerRadius: 10)
            )
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .onHover { hovered = $0 }
        .accessibilityLabel(MavioStrings.navUserMenu(name: name))
    }
}

/// The user's menu, as the web UI's: the user with the appearance beside
/// them, the users of a standalone server to switch to, the settings, the
/// account at mavio.dev or the servers, and signing out.
private struct UserMenu: View {
    @Environment(AppModel.self) private var model
    @Environment(Shell.self) private var shell
    @Environment(\.theme) private var theme
    @Environment(\.openURL) private var openURL
    @AppStorage(DeviceSetting.appearance) private var appearance = Appearance.system
    @State private var users: [SwitchableUser] = []
    @State private var asking: SwitchableUser?
    @State private var busy = false
    @State private var failure: String?

    /// The server of a standalone sign-in, whose users the menu switches among.
    private var standalone: ServerRecord? {
        model.account == nil ? model.signedInServers.first : nil
    }

    var body: some View {
        let (name, line) = signedIn(model)
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: MavioSpacing.s12) {
                Avatar(name: name, size: 36)
                VStack(alignment: .leading, spacing: 0) {
                    Text(name).type(theme.type.headline).lineLimit(1)
                    Text(
                        model.account != nil
                            ? MavioStrings.authMavioAccount : MavioStrings.authStandaloneUser(server: line)
                    )
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
                    .lineLimit(1)
                }
                Spacer(minLength: 0)
                themes
            }
            .padding(.horizontal, 10)
            .padding(.vertical, MavioSpacing.s8)
            if users.count > 1 {
                separator
                Text(MavioStrings.authSwitchUser)
                    .type(theme.type.caption)
                    .foregroundStyle(theme.colors.textSecondary)
                    .padding(.horizontal, 10)
                    .padding(.top, 6)
                    .padding(.bottom, MavioSpacing.s4)
                ForEach(users, id: \.id) { user in
                    MenuRow(disabled: user.current || busy) {
                        if user.hasPin {
                            shell.userMenu = false
                            asking = user
                        } else {
                            Task { await switchTo(user, pin: nil) }
                        }
                    } label: {
                        Avatar(name: user.name, size: 24)
                        Text(user.name).lineLimit(1)
                        Spacer(minLength: 0)
                        if user.current {
                            Icon("check", size: 16).foregroundStyle(theme.colors.accentText)
                        } else if user.hasPin {
                            Icon("lock", size: 14).foregroundStyle(theme.colors.textTertiary)
                        }
                    }
                }
            }
            separator
            item("settings", MavioStrings.navSettings, shortcut: "⌘,") { shell.place = .settings }
            if let account = model.account {
                item("external", MavioStrings.menuManageAccount) {
                    if let url = URL(string: site(account.api) + "/account") { openURL(url) }
                }
            } else {
                item("server", MavioStrings.menuManageServers) {
                    shell.settingsAccount = true
                    shell.place = .settings
                }
            }
            separator
            item("signOut", MavioStrings.authSignOut) { Task { try? await model.signOut() } }
        }
        .padding(6)
        .frame(width: 300)
        .background(.regularMaterial, in: .rect(cornerRadius: 14))
        .background(theme.colors.surfaceOverlay.opacity(0.9), in: .rect(cornerRadius: 14))
        .overlay { RoundedRectangle(cornerRadius: 14).strokeBorder(theme.colors.border) }
        .shadow(color: .black.opacity(0.18), radius: 16, y: 8)
        .background {
            // Escape closes it, as the web UI's.
            Button("") { shell.userMenu = false }.keyboardShortcut(.escape, modifiers: []).hidden()
        }
        .task(id: standalone?.id) {
            if let server = standalone { users = await SwitchUserSection.load(model, server) }
        }
        .sheet(isPresented: Binding(get: { asking != nil }, set: { if !$0 { asking = nil } })) {
            if let asking {
                PinPad(name: asking.name, busy: busy, error: failure) { pin in
                    Task { await switchTo(asking, pin: pin) }
                } cancel: {
                    self.asking = nil
                    failure = nil
                }
                .padding(MavioSpacing.s24)
                .frame(width: 340)
                .presentationBackground(theme.colors.background)
            }
        }
    }

    /// The appearance, chosen beside the user's name: the system's, light or dark.
    private var themes: some View {
        HStack(spacing: 2) {
            ForEach(Appearance.allCases, id: \.self) { choice in
                let chosen = appearance == choice
                Button {
                    appearance = choice
                } label: {
                    Icon(choice.icon, size: 16)
                        .foregroundStyle(chosen ? theme.colors.textPrimary : theme.colors.textSecondary)
                        .frame(width: 28, height: 28)
                        .background {
                            if chosen {
                                Circle().fill(theme.colors.surfaceRaised).shadow(
                                    color: .black.opacity(0.12), radius: 1, y: 1)
                            }
                        }
                        .contentShape(.circle)
                }
                .buttonStyle(.plain)
                .help(choice.label)
                .accessibilityLabel(choice.label)
                .accessibilityAddTraits(chosen ? .isSelected : [])
            }
        }
        .padding(2)
        .background(theme.colors.fill, in: .capsule)
    }

    private var separator: some View {
        Rectangle().fill(theme.colors.border).frame(height: 1).padding(.horizontal, 10).padding(
            .vertical, MavioSpacing.s4)
    }

    /// An item with its icon and, where it has one, its shortcut; choosing
    /// it closes the menu.
    private func item(_ icon: String, _ title: String, shortcut: String? = nil, action: @escaping () -> Void)
        -> some View
    {
        MenuRow {
            shell.userMenu = false
            action()
        } label: {
            Icon(icon, size: 18).foregroundStyle(theme.colors.textSecondary)
            Text(title).lineLimit(1)
            Spacer(minLength: 0)
            if let shortcut {
                Text(shortcut).type(theme.type.footnote).foregroundStyle(theme.colors.textTertiary)
            }
        }
    }

    /// The site of mavio.dev whose API is api.
    private func site(_ api: String) -> String {
        var site = api
        while site.hasSuffix("/") { site.removeLast() }
        if site.hasSuffix("/api") { site.removeLast(4) }
        return site
    }

    private func switchTo(_ user: SwitchableUser, pin: String?) async {
        guard let server = standalone else { return }
        busy = true
        failure = nil
        defer { busy = false }
        do {
            try await model.switchUser(server.id, to: user.id, pin: pin)
            asking = nil
            shell.userMenu = false
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }
}

/// A row of the user's menu, highlighted under the pointer, as the web UI's.
private struct MenuRow<Label: View>: View {
    var disabled = false
    let action: () -> Void
    @ViewBuilder let label: Label
    @Environment(\.theme) private var theme
    @State private var hovered = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: MavioSpacing.s8) { label }
                .type(theme.type.body)
                .foregroundStyle(theme.colors.textPrimary)
                .padding(.horizontal, MavioSpacing.s8)
                .frame(maxWidth: .infinity, minHeight: 28, alignment: .leading)
                .background(hovered && !disabled ? theme.colors.fill : .clear, in: .rect(cornerRadius: 6))
                .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? 0.5 : 1)
        .onHover { hovered = $0 }
    }
}

extension Appearance {
    /// Its icon, as the web UI's.
    fileprivate var icon: String {
        switch self {
        case .system: "appearance"
        case .light: "lightMode"
        case .dark: "darkMode"
        }
    }
}

extension LibraryType {
    /// The icon of a library of this kind, as the web UI's.
    var icon: String {
        switch self {
        case .movies: "movies"
        case .shows: "shows"
        case .music, .musicVideos: "music"
        case .homeVideos: "homeVideos"
        case .books: "books"
        case .photos: "photos"
        case .collections: "collections"
        case .playlists: "playlists"
        case .mixed: "folder"
        }
    }
}
