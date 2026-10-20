#if os(macOS)
    import AppKit
    import MavioMac

    /// All the titles of a catalog, page by page, with Filters at the top
    /// right when it takes filters or sorts; in television mode under its
    /// name, focus starting on its first title (docs/tv.md §6.3).
    final class CatalogPage: Page {
        private let query: CatalogQuery
        /// The query with the filters applied, once some are.
        private var filtered: CatalogQuery?
        private var titles: [DiscoveredTitle] = []
        private var next: Int32? = 1
        private var loading = false
        private var failed = false
        private var task: Task<Void, Never>?
        private let grid: CardGrid
        private let body = NSView()
        private var filters: MavioButton?
        private var cardHeights: [CGFloat: CGFloat] = [:]
        private var focusedFirst = false

        /// The catalog of query.
        init(context: MacContext, query: CatalogQuery) {
            self.query = query
            grid = CardGrid(theme: context.theme)
            super.init(context: context)
        }

        isolated deinit { task?.cancel() }

        private var current: CatalogQuery { filtered ?? query }

        /// The catalog, when it takes filters or sorts; recommendations and
        /// similar titles take none.
        private var filterable: Catalog? {
            guard query.tmdbId == 0,
                let catalog = context.discovery.catalogs.first(where: { $0.id == query.catalog }),
                !catalog.filters.isEmpty || !catalog.sorts.isEmpty
            else { return nil }
            return catalog
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = query.title
            body.translatesAutoresizingMaskIntoConstraints = false
            var bar: [NSView] = []
            if theme.isTV {
                bar.append(NSTextField.label(query.title, theme.fonts.title1, color: theme.colors.textPrimary))
            }
            bar.append(Spacer())
            if filterable != nil {
                let button = MavioButton(icon: "filter", kind: .secondary, theme: theme) { [weak self] in
                    self?.openFilters()
                }
                filters = button
                bar.append(button)
            }
            let stack = NSStackView.vertical([], spacing: 0)
            if bar.count > 1 || theme.isTV {
                let row = NSStackView.horizontal(bar, spacing: MavioSpacing.s8)
                let holder = margined(row, top: theme.isTV ? MavioSpacing.s24 : MavioSpacing.s8)
                stack.addArrangedSubview(holder)
                holder.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            stack.addArrangedSubview(body)
            body.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            body.setContentHuggingPriority(.init(1), for: .vertical)
            grid.onReachEnd = { [weak self] in self?.load() }
            show(stack)
            restyleFilters()
            render()
            load()
        }

        private func restyleFilters() {
            guard let filters else { return }
            let n = current.filterCount
            filters.icon = n > 0 || current.sort != .unspecified ? "filterFill" : "filter"
            filters.title = n > 0 ? String(n) : ""
            filters.setAccessibilityLabel(
                n > 0 ? "\(MavioStrings.discoverFilters) (\(n))" : MavioStrings.discoverFilters)
        }

        private func setBody(_ view: NSView) {
            guard view.superview !== body else { return }
            body.subviews.forEach { $0.removeFromSuperview() }
            body.addFilling(view)
            FocusEngine.active(for: self.view)?.schedule()
        }

        /// Shows the titles, or the state of the catalog without them.
        private func render() {
            if !titles.isEmpty {
                let serverId = query.serverId
                let titles = titles
                grid.show(count: titles.count, height: { [weak self] in self?.cardHeight($0) ?? $0 * 1.5 }) {
                    [weak self] i in
                    guard let self else { return NSView() }
                    let card = self.discoveredCard(titles[i], serverId: serverId, width: nil)
                    if i == 0 { self.focusFirst(card) }
                    return card
                }
                setBody(grid)
            } else if failed {
                setBody(
                    StatusView(
                        icon: "error", title: MavioStrings.stateError,
                        action: (MavioStrings.actionRetry, { [weak self] in self?.load() }), theme: theme))
            } else if loading || next != nil {
                setBody(LoadingView())
            } else if current.filterCount > 0 || current.sort != .unspecified {
                setBody(
                    StatusView(
                        icon: "search", title: MavioStrings.discoverNoMatches,
                        action: (
                            MavioStrings.discoverReset,
                            { [weak self] in
                                guard let self else { return }
                                self.apply(self.query.unfiltered)
                            }
                        ), theme: theme))
            } else {
                setBody(StatusView(icon: "search", title: MavioStrings.discoverNoMatches, theme: theme))
            }
        }

        /// Puts television mode's focus on the first title as the catalog
        /// opens, rather than on Filters above it.
        private func focusFirst(_ card: NSView) {
            guard !focusedFirst else { return }
            DispatchQueue.main.async { [weak self, weak card] in
                guard let self, let card, card.window != nil, let engine = FocusEngine.active(for: self.view) else {
                    return
                }
                self.focusedFirst = true
                if engine.focused == nil || engine.focused === self.filters { engine.focus(card) }
            }
        }

        /// A card's height at a width: its poster and two lines.
        private func cardHeight(_ width: CGFloat) -> CGFloat {
            if let h = cardHeights[width] { return h }
            let sample = CardView(width: nil, aspect: 2 / 3, theme: theme)
            sample.setText("A", "A")
            let h = height(of: sample, width: width)
            cardHeights[width] = h
            return h
        }

        /// Reads the next page, adding its titles not shown yet.
        private func load() {
            guard let page = next, !loading else { return }
            let query = current
            loading = true
            failed = false
            let model = model
            task = Task { [weak self] in
                do {
                    let response = try await model.discover(query, page: page)
                    guard let self, query == self.current else { return }
                    var seen = Set(self.titles.map(\.key))
                    self.titles += response.titles.filter { seen.insert($0.key).inserted }
                    self.next = response.page < response.totalPages ? response.page + 1 : nil
                    self.loading = false
                    self.render()
                } catch {
                    guard let self, query == self.current else { return }
                    self.loading = false
                    self.failed = !(error is CancellationError)
                    if self.titles.isEmpty { self.render() }
                }
            }
        }

        /// Shows the titles of other filters, from their first page.
        private func apply(_ value: CatalogQuery) {
            filtered = value
            task?.cancel()
            titles = []
            next = 1
            loading = false
            failed = false
            restyleFilters()
            grid.scrollToTop()
            render()
            load()
        }

        private func openFilters() {
            guard let catalog = filterable else { return }
            let dialog = FiltersDialog(catalog: catalog, value: current, page: self) { [weak self] in
                self?.apply($0)
            }
            dialog.present()
        }
    }
#endif
