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
                DesktopSettingsView { shell.place = .home }
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
                UserMenu().padding(MavioSpacing.s8)
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

/// The user's avatar and name, opening the menu: the appearance, the
/// settings and signing out.
private struct UserMenu: View {
    @Environment(AppModel.self) private var model
    @Environment(Shell.self) private var shell
    @AppStorage(DeviceSetting.appearance) private var appearance = Appearance.system

    var body: some View {
        let name = model.account?.name ?? model.signedInServers.first?.userName ?? ""
        let line =
            model.account != nil
            ? MavioStrings.menuServerCount(count: model.signedInServers.count)
            : model.signedInServers.first?.name ?? ""
        Menu {
            Picker(MavioStrings.settingsAppearance, selection: $appearance) {
                ForEach(Appearance.allCases, id: \.self) { Text($0.label).tag($0) }
            }
            Divider()
            Button(MavioStrings.navSettings) { shell.place = .settings }
                .keyboardShortcut(",", modifiers: .command)
            Divider()
            Button(MavioStrings.authSignOut, role: .destructive) {
                Task { try? await model.signOut() }
            }
        } label: {
            HStack(spacing: MavioSpacing.s8) {
                Avatar(name: name, size: 28)
                VStack(alignment: .leading, spacing: 0) {
                    Text(name).font(.callout.weight(.semibold)).lineLimit(1)
                    Text(line).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                }
                Spacer(minLength: 0)
            }
            .contentShape(.rect)
        }
        .menuStyle(.button)
        .buttonStyle(.plain)
        .menuIndicator(.hidden)
        .accessibilityLabel(MavioStrings.navUserMenu(name: name))
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
