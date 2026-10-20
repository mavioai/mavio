#if os(macOS)
    import AppKit
    import MavioMac

    /// Discover: a row of each catalog of the first server that discovers,
    /// with Requests at the right above the first row, where the SwiftUI
    /// app had it in the toolbar (docs/tv.md §6.3).
    final class DiscoverPage: Page {
        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: MavioSpacing.s8, left: 0, bottom: MavioSpacing.s24, right: 0))
        private var observer: Observer?
        /// The rows shown, which rebuild the page only when they change.
        private var shown: [CatalogQuery]?

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navDiscover
            observer = Observer { [weak self] in self?.update() }
        }

        isolated deinit { observer?.cancel() }

        private func update() {
            let discovery = context.discovery
            guard let server = discovery.server else {
                shown = nil
                if discovery.loaded {
                    show(
                        StatusView(
                            icon: "discover", title: MavioStrings.discoverNotSetUp,
                            message: MavioStrings.discoverNotSetUpMessage, theme: theme))
                } else {
                    show(LoadingView())
                }
                return
            }
            let queries = discovery.rows.map {
                CatalogQuery(serverId: server.id, catalog: $0.catalog.id, kind: $0.kind, title: $0.title)
            }
            guard queries != shown else { return }
            shown = queries
            let requests = MavioButton(MavioStrings.requestsTitle, icon: "pending", kind: .secondary, theme: theme) {
                [weak self] in
                guard let self else { return }
                self.push(RequestsPage(context: self.context, serverId: server.id))
            }
            let bar = NSStackView.horizontal([Spacer(), requests], spacing: MavioSpacing.s8)
            var views: [NSView] = [margined(bar, bottom: theme.isTV ? 0 : MavioSpacing.s4)]
            var first: CatalogShelf?
            for query in queries {
                let shelf = CatalogShelf(query: query, page: self)
                if first == nil { first = shelf }
                views.append(shelf)
            }
            page.setViews(views)
            if page.superview == nil { show(page) }
            // Focus opens on the first row's first title rather than on
            // Requests, which is reached up from the row (docs/tv.md §6.3).
            first?.onFirstCard = { [weak self, weak requests] card in
                guard let self, let engine = FocusEngine.active(for: self.view) else { return }
                if engine.focused == nil || engine.focused === requests { engine.focus(card) }
            }
            FocusEngine.active(for: view)?.schedule()
        }
    }

    /// A row of a catalog's first page, with More opening the catalog
    /// beside its title, or in television mode a See All card at its end;
    /// nothing when it has no titles.
    final class CatalogShelf: NSView {
        private let query: CatalogQuery
        private weak var page: Page?
        private var loading: Task<Void, Never>?
        private var titles: [DiscoveredTitle]?
        /// Called as the first title's card shows.
        var onFirstCard: (@MainActor (NSView) -> Void)?

        /// The row of query, on page, loading as it is made.
        init(query: CatalogQuery, page: Page) {
            self.query = query
            self.page = page
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let theme = page.theme
            let shelf = Shelf(query.title, theme: theme)
            shelf.row.show(count: 8) { _ in
                CardView.placeholder(wide: false, width: theme.layout.poster, theme: theme)
            }
            addFilling(shelf)
            reload()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { loading?.cancel() }

        /// Reads the first page again, the titles shown staying when it
        /// cannot be.
        func reload() {
            guard let model = page?.model else { return }
            let query = query
            loading?.cancel()
            loading = Task { [weak self] in
                let page = try? await model.discover(query, page: 1)
                guard let self, !Task.isCancelled else { return }
                if let page {
                    self.show(page.titles, more: page.page < page.totalPages)
                } else if self.titles == nil {
                    self.show([], more: false)
                }
            }
        }

        private func show(_ titles: [DiscoveredTitle], more: Bool) {
            self.titles = titles
            guard let page else { return }
            subviews.forEach { $0.removeFromSuperview() }
            isHidden = titles.isEmpty
            guard !titles.isEmpty else { return }
            let theme = page.theme
            let query = query
            var accessory: NSView?
            if more, !theme.isTV {
                accessory = NSStackView.horizontal(
                    [
                        Spacer(),
                        MavioButton(MavioStrings.homeMore, kind: .link, theme: theme) { [weak page] in
                            page?.open(.catalog(query))
                        },
                    ], spacing: 0)
            }
            let shelf = Shelf(query.title, accessory: accessory, theme: theme)
            let seeAll = more && theme.isTV
            let serverId = query.serverId
            shelf.row.show(count: titles.count + (seeAll ? 1 : 0)) { [weak self, weak page] i in
                guard let page else { return NSView() }
                if i == titles.count {
                    return page.seeAllCard(height: theme.layout.poster * 1.5) { [weak page] in
                        page?.open(.catalog(query))
                    }
                }
                let card = page.discoveredCard(titles[i], serverId: serverId, width: theme.layout.poster)
                if i == 0 {
                    // Once in the window: the first card made is the row's
                    // sample.
                    DispatchQueue.main.async { [weak self, weak card] in
                        if let card, card.window != nil { self?.onFirstCard?(card) }
                    }
                }
                return card
            }
            addFilling(shelf)
            FocusEngine.active(for: self)?.schedule()
        }
    }
#endif
