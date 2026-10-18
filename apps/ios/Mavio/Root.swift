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
    case home, libraries, discover, search, downloads, settings
}

/// The sign-in screen until a server is added, then the destinations in a
/// tab bar on iPhone and a sidebar on iPad.
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
            // A phone has search and the settings in its top bar, and the
            // downloads in the settings; an iPad's sidebar has them all.
            if sizeClass == .regular {
                SwiftUI.Tab(value: Tab.search, role: .search) {
                    Stack { SearchView() }
                } label: {
                    tabLabel(MavioStrings.navSearch, "search")
                }
                SwiftUI.Tab(value: Tab.downloads) {
                    Stack { DownloadsView() }
                } label: {
                    tabLabel(MavioStrings.navDownloads, "downloads")
                }
                SwiftUI.Tab(value: Tab.settings) {
                    Stack { SettingsView() }
                } label: {
                    tabLabel(MavioStrings.navSettings, "settings")
                }
            }
        }
        .tabViewStyle(.sidebarAdaptable)
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

    var body: some View {
        NavigationStack {
            content
                .background(theme.colors.background)
                .modifier(TopBar())
                .navigationDestination(for: Route.self) { route in
                    Group {
                        switch route {
                        case let .item(serverId, itemId): DetailsView(serverId: serverId, itemId: itemId)
                        case let .library(library): LibraryView(library: library)
                        case let .hub(serverId, hubId, title): HubView(serverId: serverId, hubId: hubId, title: title)
                        case let .credits(serverId, itemId, crew):
                            CreditsView(serverId: serverId, itemId: itemId, crew: crew)
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
        // Toolbar buttons and rows take the label's color, as in the web UI;
        // the tab bar keeps the accent.
        .tint(theme.colors.textPrimary)
    }
}

/// The top of a tab's first screen on a phone, as the web UI's: the
/// wordmark, and at the right search and the user's avatar opening the
/// settings.
private struct TopBar: ViewModifier {
    @Environment(AppModel.self) private var model
    @Environment(\.horizontalSizeClass) private var sizeClass

    func body(content: Content) -> some View {
        if sizeClass == .regular {
            content
        } else {
            bar(content)
        }
    }

    private func bar(_ content: Content) -> some View {
        let name = model.account?.name ?? model.signedInServers.first?.userName ?? ""
        return content.toolbar {
            ToolbarItem(placement: .topBarLeading) {
                Wordmark(height: 20).accessibilityLabel("Mavio")
            }
            .hiddenSharedBackground()
            ToolbarItem(placement: .topBarTrailing) {
                NavigationLink(value: Route.search) {
                    Icon("search", size: 22)
                }
                .accessibilityLabel(MavioStrings.navSearch)
            }
            .hiddenSharedBackground()
            ToolbarItem(placement: .topBarTrailing) {
                NavigationLink(value: Route.settings) {
                    Avatar(name: name, size: 32)
                }
                .accessibilityLabel(MavioStrings.navSettings)
            }
            .hiddenSharedBackground()
        }
    }
}

/// A phone's tab bar shows on the tabs' first screens alone, as the web UI's,
/// and hides on a page opened from one; the iPad's sidebar stays.
private struct HiddenTabBar: ViewModifier {
    @Environment(\.horizontalSizeClass) private var sizeClass

    func body(content: Content) -> some View {
        content.toolbar(sizeClass == .regular ? .automatic : .hidden, for: .tabBar)
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
