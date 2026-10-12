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
    case home, libraries, search, downloads, profile
}

/// The sign-in screen until a server is added, then the destinations in a
/// tab bar on iPhone and a sidebar on iPad.
struct RootView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.scenePhase) private var phase
    @Environment(\.colorScheme) private var scheme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var router = Router()
    @State private var tab: Tab = .home

    var body: some View {
        Group {
            if model.servers.isEmpty {
                SignInView()
            } else {
                tabs
            }
        }
        .environment(router)
        .environment(
            \.theme,
            Theme(colors: scheme == .light ? .light : .dark, layout: sizeClass == .regular ? .tablet : .phone)
        )
        .tint(MavioColors.dark.accent)
        .fullScreenCover(item: $router.playing) { request in
            PlayerScreen(request: request)
                .environment(model)
        }
        .sheet(isPresented: $router.addingServer) {
            SignInView(adding: true)
        }
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
                tabLabel(MavioStrings.navLibraries, "books")
            }
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
            SwiftUI.Tab(value: Tab.profile) {
                Stack { ProfileView() }
            } label: {
                tabLabel(MavioStrings.navProfile, "profile")
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
                .navigationDestination(for: Route.self) { route in
                    Group {
                        switch route {
                        case let .item(serverId, itemId): DetailsView(serverId: serverId, itemId: itemId)
                        case let .library(library): LibraryView(library: library)
                        case let .hub(serverId, hubId, title): HubView(serverId: serverId, hubId: hubId, title: title)
                        }
                    }
                    .background(theme.colors.background)
                }
        }
    }
}
