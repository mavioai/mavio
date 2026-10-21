import Foundation
import MavioCommon
import MavioKit
import TVServices

/// Writes Home's Continue Watching for the Top Shelf, as it changes, and
/// tells the system to load the shelf again (docs/tv.md §8).
@MainActor
enum TopShelf {
    /// The widest the shelf draws an item, in pixels.
    private static let width = 808

    /// Writes the shelf of rows, unless Home has not come or a row of the
    /// shelf still loads or its server is offline, when the last shelf
    /// stays; signed out of every server, the shelf is empty.
    static func write(_ rows: [Row], signedIn: Bool) {
        guard let file = TopShelfShelf.file, !signedIn || !rows.isEmpty else { return }
        var sections: [TopShelfShelf.Section] = []
        if signedIn {
            for row in rows where row.kind == .continueWatching {
                guard case let .items(cards) = row.content, !row.offline else { return }
                let items = cards.compactMap(item)
                if !items.isEmpty { sections.append(.init(title: rowTitle(row), items: items)) }
            }
        }
        let shelf = TopShelfShelf(sections: sections)
        guard shelf != TopShelfShelf.read(), let data = try? JSONEncoder().encode(shelf) else { return }
        // A shelf not written is written again as Home next changes.
        guard (try? data.write(to: file, options: .atomic)) != nil else { return }
        TVTopShelfContentProvider.topShelfContentDidChange()
    }

    private static func item(_ card: Card) -> TopShelfShelf.Item? {
        let path = "\(card.serverId)/\(card.itemId)"
        guard let display = URL(string: "mavio://item/\(path)"), let play = URL(string: "mavio://play/\(path)")
        else { return nil }
        let image = (card.thumb ?? card.backdrop).flatMap { URL(string: $0.url + "?maxWidth=\(width)") }
        // An episode by its series and numbers.
        let title = card.kind == .episode ? card.subtitle ?? card.title : card.title
        return TopShelfShelf.Item(
            id: path, title: title, image: image, progress: card.progress.map(Double.init), display: display,
            play: play)
    }
}
