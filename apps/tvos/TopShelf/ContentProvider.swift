import TVServices

/// The Top Shelf of the app: the sections the app wrote, their items shown
/// wide with how far they were watched, selected to show their details and
/// played with the remote's Play button (docs/tv.md §8).
final class ContentProvider: TVTopShelfContentProvider {
    override func loadTopShelfContent(completionHandler: @escaping ((any TVTopShelfContent)?) -> Void) {
        let sections = TopShelfShelf.read().sections.compactMap {
            section -> TVTopShelfItemCollection<TVTopShelfSectionedItem>? in
            let items = section.items.map { entry in
                let item = TVTopShelfSectionedItem(identifier: entry.id)
                item.title = entry.title
                item.imageShape = .hdtv
                item.setImageURL(entry.image, for: [.screenScale1x, .screenScale2x])
                if let progress = entry.progress { item.playbackProgress = progress }
                item.displayAction = TVTopShelfAction(url: entry.display)
                item.playAction = TVTopShelfAction(url: entry.play)
                return item
            }
            guard !items.isEmpty else { return nil }
            let collection = TVTopShelfItemCollection(items: items)
            collection.title = section.title
            return collection
        }
        completionHandler(sections.isEmpty ? nil : TVTopShelfSectionedContent(sections: sections))
    }
}
