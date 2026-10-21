import Foundation

/// What the Top Shelf shows of the app: Continue Watching on the servers
/// signed in, which the app writes into the app group's container as Home
/// changes and the extension reads, so the extension needs no tokens or
/// core of its own (docs/tv.md §8).
struct TopShelfShelf: Codable, Equatable {
    /// The app group the app and the extension share.
    static let group = "group.dev.mavio.app"

    /// A section of the shelf.
    struct Section: Codable, Equatable {
        /// Its title, in the app's language.
        var title: String
        /// Its items.
        var items: [Item]
    }

    /// An item of a section.
    struct Item: Codable, Equatable {
        /// The item's server and ID.
        var id: String
        /// Its title; an episode's series with its numbers.
        var title: String
        /// Its wide image, signed for the session.
        var image: URL?
        /// How far it was watched, from 0 to 1.
        var progress: Double?
        /// The link that shows its details.
        var display: URL
        /// The link that plays it.
        var play: URL
    }

    /// The sections.
    var sections: [Section]

    /// The file in the app group's container, nil when the app is not
    /// entitled to the group.
    static var file: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group)?
            .appending(path: "TopShelf.json")
    }

    /// The shelf the app last wrote, empty without one.
    static func read() -> TopShelfShelf {
        guard let file, let data = try? Data(contentsOf: file),
            let shelf = try? JSONDecoder().decode(TopShelfShelf.self, from: data)
        else { return TopShelfShelf(sections: []) }
        return shelf
    }
}
