import Foundation
import MavioKit
import Observation

/// A screen below the top-level destinations.
public enum Route: Hashable {
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
public struct PlayRequest: Identifiable, Hashable {
    /// The playback's identity, new for each request.
    public let id = UUID()
    /// The server.
    public var serverId: String
    /// The item.
    public var itemId: String
    /// The title, shown while loading.
    public var title: String
    /// The version to play; unset lets the server choose.
    public var mediaSourceId: String?
    /// Where to start, in seconds; unset starts at the beginning.
    public var start: TimeInterval?
    /// The downloaded file, played without the server.
    public var file: URL?

    /// A request to play an item of a server.
    public init(
        serverId: String, itemId: String, title: String, mediaSourceId: String? = nil, start: TimeInterval? = nil,
        file: URL? = nil
    ) {
        (self.serverId, self.itemId, self.title) = (serverId, itemId, title)
        (self.mediaSourceId, self.start, self.file) = (mediaSourceId, start, file)
    }
}

/// What the screens present over the tabs.
@MainActor
@Observable
public final class Router {
    /// The playback the player shows.
    public var playing: PlayRequest?
    /// Whether the sheet adding a server shows.
    public var addingServer = false
    /// A query for search to run as it shows, which the assistant asks
    /// for; search takes it.
    public var searchQuery: String?

    /// A router showing nothing over the screens.
    public init() {}

    /// Plays an item.
    public func play(_ request: PlayRequest) {
        playing = request
    }
}
