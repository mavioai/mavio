import MavioAdmin
import MavioKit
import MavioUI
import SwiftUI

extension EnvironmentValues {
    /// How a `RouteLink` opens its page, when the stack opens it itself.
    @Entry public var openRoute: ((Route) -> Void)? = nil
}

/// A link to a page of the stack: a `NavigationLink`, or on the iPad a
/// button the stack opens the page for.
public struct RouteLink<Label: View>: View {
    let value: Route
    @ViewBuilder let label: Label
    @Environment(\.openRoute) private var open

    /// A link to the page of value, labeled by label.
    public init(value: Route, @ViewBuilder label: () -> Label) {
        self.value = value
        self.label = label()
    }

    /// The view's content.
    public var body: some View {
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
    /// A link of a title to the page of value.
    public init(_ title: String, value: Route) {
        self.init(value: value) { Text(title) }
    }
}

/// The page of a route, as every stack opens it.
public struct RouteDestination: View {
    let route: Route

    /// The page of route.
    public init(route: Route) { self.route = route }

    /// The view's content.
    public var body: some View {
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
    }
}
