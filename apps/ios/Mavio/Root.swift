import MavioAdmin
import MavioKit
import MavioScreens
import MavioUI
import SwiftUI

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
    @Environment(Assistant.self) private var assistant
    @State private var navigation = Navigation(router: Router())
    @State private var discovery = Discovery()
    /// Whether the launch's mark still shows, and whether it may leave.
    @State private var launching = true
    @State private var ready = false

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
        .overlay {
            if navigation.router.playing == nil {
                AssistantOverlay(assistant: assistant, lift: navigation.tabBarShows ? 56 : 0)
            }
        }
        .overlay(alignment: .bottomTrailing) {
            if navigation.router.playing == nil {
                GroupIndicator(lift: navigation.tabBarShows ? 56 : 0)
            }
        }
        .overlay {
            if launching {
                LaunchView(ready: ready) { launching = false }
            }
        }
        .task {
            await LaunchView.prepare(model)
            ready = true
        }
        // The player opens on what the Watch Together group plays.
        .task { await navigation.router.follow(model.syncPlay) }
        .environment(navigation.router)
        .environment(navigation)
        .environment(discovery)
        .environment(\.theme, theme)
        // Devices opens the remote control of another device.
        .environment(\.remoteControl) { server, sessionId in
            AnyView(RemoteView(server: server, sessionId: sessionId).presentationDetents([.medium, .large]))
        }
        // The accent marks only progress, selection, links and switches;
        // buttons take the label color or `color.button`.
        .tint(MavioColors.dark.accent)
        .fullScreenCover(item: Binding(get: { navigation.router.playing }, set: { navigation.router.playing = $0 })) {
            request in
            PlayerScreen(request: request)
                .overlay { AssistantOverlay(assistant: assistant) }
                .environment(model)
                .environment(assistant)
                .environment(navigation)
                .environment(\.theme, Theme(colors: .dark, layout: theme.layout))
        }
        .sheet(
            isPresented: Binding(get: { navigation.router.addingServer }, set: { navigation.router.addingServer = $0 })
        ) {
            // A sheet takes the environment of where it is presented, outside
            // the theme's.
            SignInView(adding: true).environment(\.theme, theme)
        }
        .task(id: Discovery.key(model)) { await discovery.load(model) }
        .onAppear {
            navigation.model = model
            assistant.host = navigation
        }
        .onChange(of: sizeClass, initial: true) { _, size in navigation.regular = size == .regular }
        // The assistant asks the server of what shows, and tells the voice
        // session what shows as it changes.
        .onChange(of: navigation.assistantScreen, initial: true) {
            assistant.refresh()
            assistant.contextChanged()
        }
        .onChange(of: model.signedInServers.map(\.id)) { assistant.refresh() }
        .onChange(of: phase, initial: true) { _, phase in
            switch phase {
            case .active: model.start()
            case .background: model.stop()
            default: break
            }
        }
    }

    private var tabs: some View {
        @Bindable var navigation = navigation
        return TabView(selection: $navigation.tab) {
            SwiftUI.Tab(value: Tab.home) {
                Stack(tab: .home) { HomeView() }
            } label: {
                tabLabel(MavioStrings.navHome, "home")
            }
            SwiftUI.Tab(value: Tab.libraries) {
                Stack(tab: .libraries) { LibrariesView() }
            } label: {
                tabLabel(MavioStrings.navLibraries, "movies")
            }
            // Discover is a destination only where a server discovers.
            if discovery.server != nil {
                SwiftUI.Tab(value: Tab.discover) {
                    Stack(tab: .discover) { DiscoverView() }
                } label: {
                    tabLabel(MavioStrings.navDiscover, "discover")
                }
            }
            // A phone has search in its top bar, an iPad a search tab at
            // the tab bar's end; both open the settings from the avatar at
            // the top bar's right, and the downloads from the settings.
            if sizeClass == .regular {
                SwiftUI.Tab(value: Tab.search, role: .search) {
                    Stack(tab: .search) { SearchView() }
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
    let tab: Tab
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(Navigation.self) private var navigation

    var body: some View {
        let tab = tab
        let navigation = navigation
        let tabBar: Visibility = navigation.tabBarHidden.contains(tab) ? .hidden : .visible
        return NavigationStack(
            path: Binding(get: { navigation.paths[tab, default: []] }, set: { navigation.paths[tab] = $0 })
        ) {
            content
                .background(theme.colors.background)
                .modifier(TopBar())
                .onAppear { navigation.tabBarHidden.remove(tab) }
                .toolbar(sizeClass == .regular ? tabBar : .automatic, for: .tabBar)
                .navigationDestination(for: Route.self) { route in
                    RouteDestination(route: route)
                        .background(theme.colors.background)
                        .modifier(HiddenTabBar())
                }
        }
        .environment(\.openRoute, opener)
        // Toolbar buttons and rows take the label's color, as in the web UI;
        // the tab bar keeps the accent.
        .tint(theme.colors.textPrimary)
    }

    /// On the iPad the stack opens pages itself, hiding the tab bar first.
    private var opener: ((Route) -> Void)? {
        guard sizeClass == .regular else { return nil }
        let navigation = navigation
        let tab = tab
        return { navigation.open($0, in: tab) }
    }
}

/// The top of a tab's first screen, as the web UI's: on a phone the
/// wordmark and search, and at the right the user's avatar opening the
/// settings.
private struct TopBar: ViewModifier {
    @Environment(AppModel.self) private var model
    @Environment(Assistant.self) private var assistant
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
                // Search bare beside the avatar, the two as close as the
                // web UI's.
                ToolbarItem(placement: .topBarTrailing) {
                    HStack(spacing: MavioSpacing.s12) {
                        AssistantButton(assistant: assistant)
                        RouteLink(value: Route.search) {
                            Icon("search", size: 22)
                        }
                        .accessibilityLabel(MavioStrings.navSearch)
                        settings(name, circle: true)
                    }
                    .buttonStyle(.plain)
                }
                .hiddenSharedBackground()
            } else {
                // On the glass the tab bar beside it has, its letter taking
                // the glass's light or dark as the tabs' do.
                if assistant.server != nil {
                    ToolbarItem(placement: .topBarTrailing) { AssistantButton(assistant: assistant) }
                }
                ToolbarItem(placement: .topBarTrailing) { settings(name) }
            }
        }
    }

    /// The avatar opening the settings: the user's initial on the bar's
    /// glass, or on the phone, beside search, on glass of its own of the
    /// iPad's kind.
    @ViewBuilder private func settings(_ name: String, circle: Bool = false) -> some View {
        let initial = Text(name.first.map { String($0).uppercased() } ?? "")
        RouteLink(value: Route.settings) {
            if circle {
                initial.font(.system(size: 17, weight: .semibold))
                    .frame(width: 44, height: 44)
                    .contentShape(.circle)
                    .modifier(AvatarGlass())
            } else {
                initial.font(.system(size: 17, weight: .semibold))
            }
        }
        .accessibilityLabel(MavioStrings.navSettings)
    }
}

/// The regular Liquid Glass the iPad's avatar has, the phone's on a circle
/// of its own, its letter taking the glass's light or dark.
private struct AvatarGlass: ViewModifier {
    func body(content: Content) -> some View {
        if #available(iOS 26, *) {
            content.glassEffect(.regular.interactive(), in: .circle)
        } else {
            content.background(.ultraThinMaterial, in: .circle)
        }
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
