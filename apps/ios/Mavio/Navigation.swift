import MavioKit
import MavioScreens
import MavioUI
import SwiftUI

/// Where the app is: the tab shown and the pages opened in each, and the
/// player over them. It opens the screens the assistant asks for and tells
/// it what shows (docs/assistant.md §5.2).
@MainActor
@Observable
final class Navigation {
    /// The tab shown.
    var tab: Tab = .home
    /// The pages opened in each tab, below its first screen.
    var paths: [Tab: [Route]] = [:]
    /// The tabs whose tab bar an iPad hides while their first page opens.
    var tabBarHidden: Set<Tab> = []
    /// Whether the app has an iPad's regular width.
    var regular = false
    /// The player shown.
    @ObservationIgnored weak var player: PlayerControl?

    let router: Router
    @ObservationIgnored var model: AppModel?

    init(router: Router) {
        self.router = router
    }

    /// Opens a page in tab: on the iPad from the first screen only once the
    /// tab bar at the top is hidden, as hidden with the push it would give
    /// its space back after it, the page jumping up.
    func open(_ route: Route, in tab: Tab) {
        guard regular, paths[tab, default: []].isEmpty else {
            paths[tab, default: []].append(route)
            return
        }
        tabBarHidden.insert(tab)
        Task { @MainActor in self.paths[tab, default: []].append(route) }
    }

    /// Whether the phone's tab bar shows: on the tabs' first screens, under
    /// no player.
    var tabBarShows: Bool { !regular && paths[tab, default: []].isEmpty && router.playing == nil }

    /// Closes the player, if one shows.
    private func closePlayer() {
        if let player { player.close() } else { router.playing = nil }
    }
}

extension Navigation: AssistantHost {
    var assistantScreen: AssistantScreen {
        if let p = router.playing { return AssistantScreen("player", serverId: p.serverId, itemId: p.itemId) }
        guard let route = paths[tab]?.last else {
            switch tab {
            case .home: return AssistantScreen("home")
            case .libraries: return AssistantScreen("libraries")
            case .discover: return AssistantScreen("discover")
            case .search: return AssistantScreen("search")
            }
        }
        switch route {
        case .item(let serverId, let itemId), .credits(let serverId, let itemId, _):
            return AssistantScreen("item", serverId: serverId, itemId: itemId)
        case .library(let library):
            return AssistantScreen("library", serverId: library.serverId, libraryId: library.id)
        case .filteredLibrary(let serverId, let libraryId, _):
            return AssistantScreen("library", serverId: serverId, libraryId: libraryId)
        case .person(let serverId, _): return AssistantScreen("person", serverId: serverId)
        case .hub(let serverId, _, _): return AssistantScreen("home", serverId: serverId)
        case .search: return AssistantScreen("search")
        case .settings: return AssistantScreen("settings")
        case .downloads: return AssistantScreen("downloads")
        case .catalog, .title, .titleCredits, .discoveredPerson, .discoveredCollection:
            return AssistantScreen("discover")
        }
    }

    var assistantPlayer: (any AssistantPlayer)? { player }

    func assistantPlay(_ action: Mavio_Assistant_V1_PlayAction, serverId: String) -> Bool {
        let ids = action.itemIds
        guard !ids.isEmpty else { return false }
        let i = Int(action.startIndex)
        let start: TimeInterval? =
            action.hasStartPosition
            ? TimeInterval(action.startPosition.seconds) + TimeInterval(action.startPosition.nanos) / 1e9 : nil
        router.play(
            PlayRequest(serverId: serverId, itemId: ids[ids.indices.contains(i) ? i : 0], title: "", start: start))
        return true
    }

    func assistantNavigate(_ action: Mavio_Assistant_V1_NavigateAction, serverId: String) -> Bool {
        switch action.target {
        case .home:
            closePlayer()
            tab = .home
            paths[.home] = []
        case .back:
            if router.playing != nil {
                closePlayer()
            } else {
                guard paths[tab]?.isEmpty == false else { return false }
                paths[tab]?.removeLast()
            }
        case .item:
            guard !action.id.isEmpty else { return false }
            closePlayer()
            open(.item(serverId: serverId, itemId: action.id), in: tab)
        case .library:
            guard let model else { return false }
            closePlayer()
            let id = action.id
            Task { [weak self] in
                guard let list = try? await model.client.libraries(serverId: serverId).libraries,
                    let library = list.first(where: { $0.id == id }), let self
                else { return }
                open(.library(library), in: tab)
            }
        case .search:
            closePlayer()
            if !action.query.isEmpty { router.searchQuery = action.query }
            if regular {
                tab = .search
                paths[.search] = []
            } else if paths[tab]?.last != .search {
                open(.search, in: tab)
            }
        case .settings:
            closePlayer()
            if paths[tab]?.last != .settings { open(.settings, in: tab) }
        default:
            return false
        }
        return true
    }
}

/// The player shown, as the assistant controls it: the player screen sets
/// what it does.
@MainActor
final class PlayerControl: AssistantPlayer {
    var assistantServerId = ""
    var playing: () -> Mavio_Assistant_V1_AskPlaying = { .init() }
    var control: (Mavio_Assistant_V1_ControlCommand, Double) -> Bool = { _, _ in false }
    var streams: (Int32?, Int32?) -> Bool = { _, _ in false }
    var close: () -> Void = {}

    var assistantPlaying: Mavio_Assistant_V1_AskPlaying { playing() }

    func assistantControl(_ command: Mavio_Assistant_V1_ControlCommand, value: Double) -> Bool {
        control(command, value)
    }

    func assistantStreams(audio: Int32?, subtitle: Int32?) -> Bool { streams(audio, subtitle) }
}
