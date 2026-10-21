import MavioCommon
import MavioKit
import MavioUI
import SwiftUI

/// A screen below the top-level destinations.
enum Route: Hashable {
    /// An item's details.
    case item(serverId: String, itemId: String)
    /// A library, on its Recommended tab.
    case library(LibraryEntry)
    /// A person's page.
    case person(serverId: String, personId: String)
    /// An item's whole cast and crew, or its crew alone.
    case credits(serverId: String, itemId: String, crew: Bool)
    /// All the titles of a catalog of Discover.
    case catalog(CatalogQuery)
    /// A discovered title.
    case title(serverId: String, kind: MediaKind, tmdbId: Int32)
    /// A discovered title's whole cast and crew, or its crew alone.
    case titleCredits(serverId: String, kind: MediaKind, tmdbId: Int32, crew: Bool)
    /// A discovered person.
    case discoveredPerson(serverId: String, tmdbId: Int32)
    /// A discovered collection.
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
    /// Where to start, in seconds; unset starts where the server says.
    var start: TimeInterval?
}

/// The top-level destinations, in the navigation bar's pill.
enum Tab: Hashable, CaseIterable {
    case home, libraries, discover, search

    var title: String {
        switch self {
        case .home: MavioStrings.navHome
        case .libraries: MavioStrings.navLibraries
        case .discover: MavioStrings.navDiscover
        case .search: MavioStrings.navSearch
        }
    }
}

/// What the screens present over the tabs, and the pages each tab pushed.
@MainActor
@Observable
final class Router {
    /// The current tab.
    var tab: Tab = .home
    /// The pages each tab pushed.
    var paths: [Tab: [Route]] = [:]
    /// Whether the current tab's page scrolled, which puts a band under the bar.
    var scrolled = false
    /// Whether the settings show over the tabs.
    var settings = false
    /// The playback the player shows.
    var playing: PlayRequest?
    /// The player shown, as the assistant controls it.
    @ObservationIgnored var player: PlayerControl?
    /// Whether the system's dictation is open for the assistant.
    var dictating = false
    /// A query the assistant searches for, which Search takes.
    var searchQuery = ""
    /// The app's model, for what the assistant opens.
    @ObservationIgnored weak var model: AppModel?

    /// Plays an item.
    func play(_ request: PlayRequest) {
        playing = request
    }

    /// Plays an item of what the shared screens ask, as Watch Together's.
    func play(_ request: MavioCommon.PlayRequest) {
        playing = PlayRequest(
            serverId: request.serverId, itemId: request.itemId, title: request.title,
            mediaSourceId: request.mediaSourceId, start: request.start)
    }

    /// Opens a link of the app, as the Top Shelf's: `mavio://item/<server>/<item>`
    /// shows an item's details on Home, `mavio://person/<server>/<person>` a
    /// person's page, and `mavio://play/<server>/<item>` plays it.
    func open(_ url: URL) {
        let parts = url.pathComponents.filter { $0 != "/" }
        guard url.scheme == "mavio", parts.count == 2 else { return }
        let (serverId, itemId) = (parts[0], parts[1])
        switch url.host() {
        case "item":
            (settings, playing, tab) = (false, nil, .home)
            paths[.home] = [.item(serverId: serverId, itemId: itemId)]
        case "person":
            (settings, playing, tab) = (false, nil, .home)
            paths[.home] = [.person(serverId: serverId, personId: itemId)]
        case "play":
            playing = PlayRequest(serverId: serverId, itemId: itemId, title: "")
        #if DEBUG
            // A discovered title, `mavio://title/<server>/movie-155`, for
            // the debug build's `-open`.
            case "title":
                let pair = itemId.split(separator: "-")
                guard pair.count == 2, let tmdbId = Int32(pair[1]) else { return }
                (settings, playing, tab) = (false, nil, .home)
                paths[.home] = [
                    .title(serverId: serverId, kind: pair[0] == "series" ? .series : .movie, tmdbId: tmdbId)
                ]
        #endif
        default:
            break
        }
    }

    /// The pages of a tab, bound for its navigation stack.
    func path(_ tab: Tab) -> Binding<[Route]> {
        Binding(get: { self.paths[tab] ?? [] }, set: { self.paths[tab] = $0 })
    }
}

/// The sign-in screen until a server is added, then the destinations under
/// the navigation bar, on the dark background the television starts on
/// (docs/tv.md §2, §3).
struct RootView: View {
    @Environment(AppModel.self) private var model
    @Environment(Assistant.self) private var assistant
    @Environment(\.scenePhase) private var phase
    @State private var router = Router()
    @State private var discovery = Discovery()

    private let theme = Theme(colors: .dark, layout: .tv, type: .tv)

    var body: some View {
        Group {
            if model.signInState == .signedOut && model.signedIn.isEmpty {
                SignInView()
            } else {
                Tabs()
            }
        }
        // The assistant over the page, and the dictation it opens without
        // a microphone.
        .overlay { AssistantOverlay() }
        .background {
            Dictation(active: Bindable(router).dictating) { assistant.ask(dictated: $0) }
                .frame(width: 1, height: 1)
        }
        .environment(\.theme, theme)
        .environment(router)
        .environment(discovery)
        .task(id: Discovery.key(model)) { await discovery.load(model) }
        .background(theme.colors.tvBackground.ignoresSafeArea())
        .preferredColorScheme(.dark)
        .fullScreenCover(item: Bindable(router).playing) { request in
            PlayerScreen(request: request)
                .environment(\.theme, theme)
                .environment(model)
                .environment(assistant)
                .environment(router)
        }
        .onOpenURL { router.open($0) }
        // The player opens on what the Watch Together group plays.
        .task {
            for await u in model.syncPlay.updates() {
                let playing = router.playing.map { ($0.serverId, $0.itemId) }
                if let request = groupPlayback(u, syncPlay: model.syncPlay, playing: playing) {
                    router.play(request)
                }
            }
        }
        .task {
            router.model = model
            assistant.host = router
            assistant.refresh()
        }
        .onChange(of: model.signedInServers.map(\.id)) { assistant.refresh() }
        .onChange(of: model.home, initial: true) {
            TopShelf.write(model.home, signedIn: !model.signedInServers.isEmpty)
        }
        // The assistant asks the server of what shows, and tells a session
        // what it is.
        .onChange(of: router.assistantScreen) {
            assistant.refresh()
            assistant.contextChanged()
        }
        #if DEBUG
            // A debug build opens a link given at launch (`-open <url>`),
            // with no remote to confirm it, or the settings (`-settings YES`).
            .task {
                if let link = UserDefaults.standard.string(forKey: "open"), let url = URL(string: link) {
                    router.open(url)
                }
                if UserDefaults.standard.bool(forKey: "settings") { router.settings = true }
            }
            // Or a tab (`-tab discover`), once the bar shows it.
            .onChange(of: discovery.server?.id, initial: true) {
                if let name = UserDefaults.standard.string(forKey: "tab"),
                    let tab = Tab.allCases.first(where: { "\($0)" == name }),
                    tab != .discover || discovery.server != nil
                {
                    Task {
                        try? await Task.sleep(for: .seconds(1))
                        router.tab = tab
                    }
                }
            }
        #endif
        .onChange(of: phase, initial: true) { _, phase in
            switch phase {
            case .active: model.start()
            case .background: model.stop()
            default: break
            }
        }
    }
}

/// The tabs' pages under the navigation bar, which deeper pages hide.
private struct Tabs: View {
    @Environment(Router.self) private var router
    @Environment(Assistant.self) private var assistant
    @Environment(\.theme) private var theme
    @FocusState private var focus: BarFocus?

    var body: some View {
        // Deeper pages and Search hide the bar (§3, §6.4).
        let deeper = !(router.paths[router.tab] ?? []).isEmpty || router.settings || router.tab == .search
        ZStack(alignment: .top) {
            page
                .focusSection()
                // Back on a tab's page moves focus up to its tab (§4).
                .onExitCommand(perform: exit(deeper: deeper))
            if !deeper {
                NavBar(focus: $focus)
                    .onExitCommand(
                        perform: assistant.open
                            ? { assistant.close() } : router.tab == .home ? nil : { focus = .tab(.home) })
            }
        }
        .ignoresSafeArea()
    }

    /// What back does on the page: the assistant goes away first; on a
    /// tab's page, focus moves up to its tab; Search and the settings return
    /// to Home; deeper pages pop.
    private func exit(deeper: Bool) -> (() -> Void)? {
        if assistant.open { return { assistant.close() } }
        if router.settings { return { router.settings = false } }
        if router.tab == .search && (router.paths[.search] ?? []).isEmpty {
            return { router.tab = .home }
        }
        return deeper ? nil : { focus = .tab(router.tab) }
    }

    @ViewBuilder private var page: some View {
        if router.settings {
            NavigationStack { SettingsView() }
        } else {
            switch router.tab {
            case .home: stack(.home) { HomeView() }
            case .libraries: stack(.libraries) { LibrariesView() }
            case .discover: stack(.discover) { DiscoverView() }
            case .search: stack(.search) { SearchView() }
            }
        }
    }

    private func stack(_ tab: Tab, @ViewBuilder root: () -> some View) -> some View {
        NavigationStack(path: router.path(tab)) {
            root().destinations()
        }
        .id(tab)
    }
}

/// What has focus in the navigation bar.
enum BarFocus: Hashable {
    case tab(Tab)
    case group
    case assistant
    case menu
}

/// The tabs in a pill centered at the top, and the assistant and menu
/// buttons at the right; focus on a tab opens it, as on tvOS (§3).
private struct NavBar: View {
    @Environment(Router.self) private var router
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    var focus: FocusState<BarFocus?>.Binding

    /// The tabs shown: Discover only once a server discovers.
    private var tabs: [Tab] { Tab.allCases.filter { $0 != .discover || discovery.server != nil } }

    var body: some View {
        ZStack {
            HStack(spacing: MavioSpacing.s4) {
                ForEach(tabs, id: \.self) { tab in
                    Button {
                        router.tab = tab
                    } label: {
                        Text(tab.title)
                    }
                    .buttonStyle(TabButtonStyle(selected: router.tab == tab))
                    .focused(focus, equals: .tab(tab))
                }
            }
            .padding(MavioSpacing.s4)
            .background(theme.colors.surfaceRaised.opacity(0.6), in: .capsule)
            .focusSection()
            .defaultFocus(focus, .tab(router.tab))
            HStack(spacing: MavioSpacing.s24) {
                Spacer()
                GroupIndicator()
                    .focused(focus, equals: .group)
                AssistantButton()
                    .focused(focus, equals: .assistant)
                Menu {
                    Button(MavioStrings.navSettings) { router.settings = true }
                    Button(MavioStrings.authSignOut, role: .destructive) {
                        Task { try? await model.signOut() }
                    }
                } label: {
                    Icon("moreVertical", size: 32).frame(width: 72, height: 72)
                }
                .buttonStyle(RoundButtonStyle())
                .focused(focus, equals: .menu)
            }
        }
        .padding(.horizontal, theme.layout.margin)
        .padding(.top, 48)
        .padding(.bottom, MavioSpacing.s24)
        .background {
            if router.scrolled { theme.colors.tvBackground.ignoresSafeArea() }
        }
        .onChange(of: focus.wrappedValue) { _, focused in
            if case let .tab(tab) = focused, tab != .search { router.tab = tab }
        }
    }
}

/// The device's Watch Together group, before the assistant's button while
/// it is in one: its name and size, opening its dialog (§3).
private struct GroupIndicator: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var open = false

    var body: some View {
        if let joined = model.syncPlay.joined {
            Button {
                open = true
            } label: {
                Label {
                    Text(groupChip(joined.group)).lineLimit(1)
                } icon: {
                    Icon("users", fill: true, size: 28)
                }
                .frame(maxWidth: 320)
            }
            .buttonStyle(TVButtonStyle())
            .fullScreenCover(isPresented: $open) {
                if let server = model.server(joined.serverId) {
                    SyncPlayView(server: server) { router.play($0) }
                }
            }
        }
    }
}

/// The height the navigation bar takes, below which pages other than Home start.
let navBarHeight: CGFloat = 150

extension View {
    /// The screens a stack of the tabs pushes.
    func destinations() -> some View {
        navigationDestination(for: Route.self) { route in
            switch route {
            case let .item(serverId, itemId): DetailsView(serverId: serverId, itemId: itemId)
            case let .library(library): LibraryView(library: library)
            case let .person(serverId, personId): PersonView(serverId: serverId, personId: personId)
            case let .credits(serverId, itemId, crew): CreditsView(serverId: serverId, itemId: itemId, crew: crew)
            case let .catalog(query): CatalogView(query: query)
            case let .title(serverId, kind, tmdbId): TitleView(serverId: serverId, kind: kind, tmdbId: tmdbId)
            case let .titleCredits(serverId, kind, tmdbId, crew):
                TitleCreditsView(serverId: serverId, kind: kind, tmdbId: tmdbId, crew: crew)
            case let .discoveredPerson(serverId, tmdbId): DiscoveredPersonView(serverId: serverId, tmdbId: tmdbId)
            case let .discoveredCollection(serverId, tmdbId):
                DiscoveredCollectionView(serverId: serverId, tmdbId: tmdbId)
            }
        }
    }

    /// Tells the bar whether the page scrolled, for the band under it.
    func reportsScroll() -> some View {
        modifier(ReportsScroll())
    }
}

private struct ReportsScroll: ViewModifier {
    @Environment(Router.self) private var router

    func body(content: Content) -> some View {
        content.onScrollGeometryChange(for: Bool.self) {
            $0.contentOffset.y + $0.contentInsets.top > 1
        } action: {
            _, scrolled in
            router.scrolled = scrolled
        }
    }
}
