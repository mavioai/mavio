import MavioKit
import MavioUI
import SwiftUI

/// A screen below the top-level destinations.
enum Route: Hashable {
    /// An item's details.
    case item(serverId: String, itemId: String)
    /// A library, on its Recommended tab.
    case library(LibraryEntry)
    /// Every item of a hub.
    case hub(serverId: String, hubId: String, title: String)
    /// An item's cast and crew, or its crew alone.
    case credits(serverId: String, itemId: String, crew: Bool)
    /// A person of a server's library.
    case person(serverId: String, personId: String)
    /// A library's grid narrowed to a genre, keyword or studio.
    case filteredLibrary(serverId: String, libraryId: String, filter: Filter)
    /// Search across the servers.
    case search
    /// The settings.
    case settings
    /// The items downloaded.
    case downloads
    /// A catalog of Discover, or the popular titles alike a title.
    case catalog(CatalogQuery)
    /// A discovered movie or series.
    case title(serverId: String, kind: MediaKind, tmdbId: Int32)
    /// A discovered title's cast and crew, or its crew alone.
    case titleCredits(serverId: String, kind: MediaKind, tmdbId: Int32, crew: Bool)
    /// A person of a discovered title.
    case discoveredPerson(serverId: String, tmdbId: Int32)
    /// A discovered collection of movies.
    case discoveredCollection(serverId: String, tmdbId: Int32)
}

/// An item to play, which the root presents the player for.
struct PlayRequest: Identifiable, Hashable {
    let id = UUID()
    /// The server and item.
    var serverId: String
    var itemId: String
    /// The title, shown while loading.
    var title: String
    /// The version to play; unset lets the server choose.
    var mediaSourceId: String?
    /// Where to start, in seconds; unset starts at the beginning.
    var start: TimeInterval?
    /// The downloaded file, played without the server.
    var file: URL?
}

/// What the screens present over the tabs.
@MainActor
@Observable
final class Router {
    /// The playback the player shows.
    var playing: PlayRequest?
    /// Whether the sheet adding a server shows.
    var addingServer = false

    /// Plays an item.
    func play(_ request: PlayRequest) {
        playing = request
    }
}

/// The top-level destinations.
enum Tab: Hashable {
    case home, libraries, discover, search
}

/// The sign-in screen until a server is added, then the destinations in a
/// tab bar, which an iPad shows at the top.
struct RootView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.scenePhase) private var phase
    @Environment(\.colorScheme) private var scheme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var router = Router()
    @State private var discovery = Discovery()
    @State private var tab: Tab = .home

    private var theme: Theme {
        Theme(colors: scheme == .light ? .light : .dark, layout: sizeClass == .regular ? .tablet : .phone)
    }

    var body: some View {
        Group {
            if case .signedOut = model.signInState {
                SignInView()
            } else {
                tabs
            }
        }
        .environment(router)
        .environment(discovery)
        .environment(\.theme, theme)
        // The accent marks only progress, selection, links and switches;
        // buttons take the label color or `color.button`.
        .tint(MavioColors.dark.accent)
        .fullScreenCover(item: $router.playing) { request in
            PlayerScreen(request: request)
                .environment(model)
        }
        .sheet(isPresented: $router.addingServer) {
            SignInView(adding: true)
        }
        .task(id: model.signedIn) { await discovery.load(model) }
        .onChange(of: phase, initial: true) { _, phase in
            switch phase {
            case .active: model.start()
            case .background: model.stop()
            default: break
            }
        }
    }

    private var tabs: some View {
        TabView(selection: $tab) {
            SwiftUI.Tab(value: Tab.home) {
                Stack { HomeView() }
            } label: {
                tabLabel(MavioStrings.navHome, "home")
            }
            SwiftUI.Tab(value: Tab.libraries) {
                Stack { LibrariesView() }
            } label: {
                tabLabel(MavioStrings.navLibraries, "movies")
            }
            // Discover is a destination only where a server discovers.
            if discovery.server != nil {
                SwiftUI.Tab(value: Tab.discover) {
                    Stack { DiscoverView() }
                } label: {
                    tabLabel(MavioStrings.navDiscover, "discover")
                }
            }
            // A phone has search in its top bar, an iPad a search tab at
            // the tab bar's end; both open the settings from the avatar at
            // the top bar's right, and the downloads from the settings.
            if sizeClass == .regular {
                SwiftUI.Tab(value: Tab.search, role: .search) {
                    Stack { SearchView() }
                } label: {
                    tabLabel(MavioStrings.navSearch, "search")
                }
            }
        }
        .tabViewStyle(.tabBarOnly)
    }

    private func tabLabel(_ title: String, _ icon: String) -> some View {
        Label {
            Text(title)
        } icon: {
            Image(icon, bundle: MavioStrings.bundle).renderingMode(.template)
        }
    }
}

/// A destination's navigation stack, which opens the screens below it.
struct Stack<Content: View>: View {
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var path = NavigationPath()
    @State private var tabBar = Visibility.visible

    var body: some View {
        NavigationStack(path: $path) {
            content
                .background(theme.colors.background)
                .modifier(TopBar())
                .onAppear { tabBar = .visible }
                .toolbar(sizeClass == .regular ? tabBar : .automatic, for: .tabBar)
                .navigationDestination(for: Route.self) { route in
                    Group {
                        switch route {
                        case let .item(serverId, itemId): DetailsView(serverId: serverId, itemId: itemId)
                        case let .library(library): LibraryView(library: library)
                        case let .hub(serverId, hubId, title): HubView(serverId: serverId, hubId: hubId, title: title)
                        case let .credits(serverId, itemId, crew):
                            CreditsView(serverId: serverId, itemId: itemId, crew: crew)
                        case let .person(serverId, personId): PersonView(serverId: serverId, personId: personId)
                        case let .filteredLibrary(serverId, libraryId, filter):
                            FilteredLibraryView(serverId: serverId, libraryId: libraryId, filter: filter)
                        case .search: SearchView()
                        case .settings: SettingsView()
                        case .downloads: DownloadsView()
                        case let .catalog(query): CatalogView(query: query)
                        case let .title(serverId, kind, tmdbId):
                            TitleView(serverId: serverId, kind: kind, tmdbId: tmdbId)
                        case let .titleCredits(serverId, kind, tmdbId, crew):
                            TitleCreditsView(serverId: serverId, kind: kind, tmdbId: tmdbId, crew: crew)
                        case let .discoveredPerson(serverId, tmdbId):
                            DiscoveredPersonView(serverId: serverId, tmdbId: tmdbId)
                        case let .discoveredCollection(serverId, tmdbId):
                            DiscoveredCollectionView(serverId: serverId, tmdbId: tmdbId)
                        }
                    }
                    .background(theme.colors.background)
                    .modifier(HiddenTabBar())
                }
        }
        .environment(\.openRoute, opener)
        // Toolbar buttons and rows take the label's color, as in the web UI;
        // the tab bar keeps the accent.
        .tint(theme.colors.textPrimary)
    }

    private var opener: ((Route) -> Void)? {
        guard sizeClass == .regular else { return nil }
        return { open($0) }
    }

    /// Opens a page on the iPad, from the first screen only once the tab bar
    /// at the top is hidden: hidden with the push, it would give its space
    /// back after it, the page jumping up.
    private func open(_ route: Route) {
        guard path.isEmpty else {
            path.append(route)
            return
        }
        tabBar = .hidden
        Task { @MainActor in path.append(route) }
    }
}

extension EnvironmentValues {
    /// How a `RouteLink` opens its page, when the stack opens it itself.
    @Entry var openRoute: ((Route) -> Void)? = nil
}

/// A link to a page of the stack: a `NavigationLink`, or on the iPad a
/// button the stack opens the page for.
struct RouteLink<Label: View>: View {
    let value: Route
    @ViewBuilder let label: Label
    @Environment(\.openRoute) private var open

    init(value: Route, @ViewBuilder label: () -> Label) {
        self.value = value
        self.label = label()
    }

    var body: some View {
        if let open {
            Button {
                open(value)
            } label: {
                label
            }
        } else {
            NavigationLink(value: value) { label }
        }
    }
}

extension RouteLink where Label == Text {
    init(_ title: String, value: Route) {
        self.init(value: value) { Text(title) }
    }
}

/// The top of a tab's first screen, as the web UI's: on a phone the
/// wordmark and search, and at the right the user's avatar opening the
/// settings.
private struct TopBar: ViewModifier {
    @Environment(AppModel.self) private var model
    @Environment(\.horizontalSizeClass) private var sizeClass

    func body(content: Content) -> some View {
        let name = model.account?.name ?? model.signedInServers.first?.userName ?? ""
        let phone = sizeClass != .regular
        return content.toolbar {
            if phone {
                ToolbarItem(placement: .topBarLeading) {
                    Wordmark(height: 20).accessibilityLabel("Mavio")
                }
                .hiddenSharedBackground()
                // Search and the avatar on one glass, as the bar buttons of
                // the system's apps, their marks taking its light or dark.
                ToolbarItemGroup(placement: .topBarTrailing) {
                    RouteLink(value: Route.search) {
                        Icon("search", size: 22)
                    }
                    .accessibilityLabel(MavioStrings.navSearch)
                    settings(name)
                }
            } else {
                // On the glass the tab bar beside it has.
                ToolbarItem(placement: .topBarTrailing) { settings(name) }
            }
        }
    }

    /// The avatar opening the settings: the user's initial on the bar's
    /// glass.
    private func settings(_ name: String) -> some View {
        RouteLink(value: Route.settings) {
            Text(name.first.map { String($0).uppercased() } ?? "")
                .font(.system(size: 17, weight: .semibold))
        }
        .accessibilityLabel(MavioStrings.navSettings)
    }
}

/// The tab bar shows on the tabs' first screens alone, as the web UI's,
/// and hides on a page opened from one.
private struct HiddenTabBar: ViewModifier {
    func body(content: Content) -> some View {
        content.toolbar(.hidden, for: .tabBar)
    }
}

extension ToolbarContent {
    /// Without the glass iOS draws behind toolbar items, for what has its own
    /// shape, as the wordmark and the avatar.
    @ToolbarContentBuilder func hiddenSharedBackground() -> some ToolbarContent {
        if #available(iOS 26, *) {
            sharedBackgroundVisibility(.hidden)
        } else {
            self
        }
    }
}
