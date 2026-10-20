import MavioDesign
import MavioKit

extension CardKind {
    /// The icon on the placeholder of this kind's artwork.
    public var icon: String {
        switch self {
        case .series, .season, .episode: "shows"
        case .artist, .album, .track, .musicVideo: "music"
        case .audiobook, .book: "books"
        case .photoAlbum, .photo: "photos"
        case .video: "homeVideos"
        case .folder: "folder"
        case .collection: "collections"
        case .playlist: "playlists"
        default: "movies"
        }
    }
}

/// The minutes left of a card begun, rounded up.
public func minutesLeft(_ card: Card) -> Int? {
    guard let runtime = card.runtime, let progress = card.progress else { return nil }
    return max(1, Int((runtime * Double(1 - progress) / 60).rounded(.up)))
}

/// The subtitle line of a card: its own subtitle, else its year.
public func subtitle(_ card: Card) -> String? {
    card.subtitle ?? card.year.map(String.init)
}

/// Whether a card shows as a 16:9 thumbnail rather than a poster.
public func isWide(_ card: Card) -> Bool {
    switch card.kind {
    case .episode, .video, .musicVideo, .photo: true
    default: false
    }
}

/// The title of a row of Home or of a library's Recommended tab, named by
/// its kind, library and seed.
public func rowTitle(_ row: Row) -> String {
    let library = row.libraryName ?? ""
    let seed = row.seedName ?? ""
    switch row.kind {
    case .continueWatching: return MavioStrings.homeContinueWatching
    case .availableRequests: return MavioStrings.homeAvailableRequests
    case .recentlyAdded: return MavioStrings.homeLatest(library: library)
    case .recentlyReleased: return MavioStrings.homeRecentlyReleased(library: library)
    case .recentlyAired: return MavioStrings.homeRecentlyAired(library: library)
    case .topRated: return MavioStrings.homeTopRated(library: library)
    case .topUnplayed: return MavioStrings.homeTopUnplayed(library: library)
    case .startWatching: return MavioStrings.homeStartWatching(library: library)
    case .rediscover: return MavioStrings.homeRediscover(library: library)
    case .becauseYouWatched: return MavioStrings.homeBecauseYouWatched(title: seed)
    case .becauseYouLike: return MavioStrings.homeBecauseYouLike(title: seed)
    case .moreInGenre: return MavioStrings.homeMoreInGenre(genre: seed)
    case .starring: return MavioStrings.homeStarring(name: seed)
    case .directedBy: return MavioStrings.homeDirectedBy(name: seed)
    case .trending: return MavioStrings.homeTrending
    case .server: return row.serverName ?? ""
    }
}
