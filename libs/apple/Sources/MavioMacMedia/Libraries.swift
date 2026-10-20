#if os(macOS)
    import AppKit
    import MavioMac

    // MARK: - Libraries

    /// The libraries of every server the user is signed in to, by server, as
    /// tiles of their images with their names: on the desktop as wide as a
    /// thumbnail at least, in television mode four to a row, as on Apple TV,
    /// with the server's name over its libraries when there are several.
    final class LibrariesPage: Page {
        private lazy var page = ScrollPage(
            [], spacing: theme.isTV ? MavioSpacing.s48 : MavioSpacing.s24,
            insets: NSEdgeInsets(top: MavioSpacing.s16, left: 0, bottom: theme.isTV ? 120 : MavioSpacing.s16, right: 0))
        private var libraries: [String: Libraries] = [:]
        private var failed: [String: String] = [:]
        private var observer: Observer?
        private var loadTask: Task<Void, Never>?
        private var key = ""

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navLibraries
            show(page)
            observer = Observer { [weak self] in self?.observe() }
        }

        private func observe() {
            let servers = model.signedInServers
            let now = servers.map { "\($0.id):\(model.revision($0.id))" }.joined(separator: ",")
            guard now != key else { return }
            key = now
            load(servers)
        }

        private func load(_ servers: [ServerRecord]) {
            loadTask?.cancel()
            render()
            let client = model.client
            loadTask = Task { [weak self] in
                await withTaskGroup(of: (String, Result<Libraries, any Error>).self) { group in
                    for server in servers {
                        group.addTask {
                            do {
                                return (server.id, .success(try await client.libraries(serverId: server.id)))
                            } catch {
                                return (server.id, .failure(error))
                            }
                        }
                    }
                    for await (id, result) in group {
                        guard let self, !Task.isCancelled else { return }
                        switch result {
                        case let .success(value):
                            self.libraries[id] = value
                            self.failed[id] = nil
                        case let .failure(error):
                            if self.libraries[id] == nil { self.failed[id] = error.reason }
                        }
                        self.render()
                    }
                }
            }
        }

        private func render() {
            let servers = model.signedInServers
            let several = model.signedIn.count > 1
            var views: [NSView] = []
            for server in servers {
                var section: [NSView] = []
                if several { section.append(margined(heading(server.name))) }
                if let entries = libraries[server.id]?.libraries {
                    let grid = TileGrid(columns: theme.isTV ? 4 : nil, minimum: theme.layout.thumbnail, theme: theme)
                    grid.setItems(entries.map(tile))
                    section.append(margined(grid))
                } else if let message = failed[server.id] {
                    section.append(unreachable(server.name, reason: message))
                } else {
                    let spinner = NSProgressIndicator()
                    spinner.style = .spinning
                    spinner.controlSize = .small
                    spinner.startAnimation(nil)
                    spinner.translatesAutoresizingMaskIntoConstraints = false
                    let holder = NSView()
                    holder.translatesAutoresizingMaskIntoConstraints = false
                    holder.addSubview(spinner)
                    spinner.center(in: holder)
                    holder.size(height: 48)
                    section.append(holder)
                }
                let stack = NSStackView.vertical(section, spacing: theme.isTV ? MavioSpacing.s16 : MavioSpacing.s8)
                for v in section { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                views.append(stack)
            }
            page.setViews(views)
            FocusEngine.active(for: view)?.schedule()
        }

        /// A library's tile: its image, and its name over a scrim at its
        /// bottom.
        private func tile(_ library: LibraryEntry) -> NSView {
            let tv = theme.isTV
            let holder = FlippedView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            let art = ArtworkView(library.imageUrl.isEmpty ? nil : library.imageUrl, icon: "folder", theme: theme)
            art.radius = MavioRadius.md
            art.setAccessibilityElement(false)
            holder.addFilling(art)
            holder.addFilling(
                GradientView(
                    colors: [.clear, theme.colors.scrim], locations: [0, 1], from: CGPoint(x: 0.5, y: 0.5),
                    to: CGPoint(x: 0.5, y: 1)))
            let name = NSTextField.label(library.name, tv ? theme.fonts.title3 : theme.fonts.headline, color: .white)
            holder.addSubview(name)
            let padding = tv ? MavioSpacing.s24 : MavioSpacing.s12
            NSLayoutConstraint.activate([
                name.leadingAnchor.constraint(equalTo: holder.leadingAnchor, constant: padding),
                name.trailingAnchor.constraint(lessThanOrEqualTo: holder.trailingAnchor, constant: -padding),
                name.bottomAnchor.constraint(equalTo: holder.bottomAnchor, constant: -padding),
            ])
            holder.wantsLayer = true
            holder.layer?.cornerRadius = MavioRadius.md
            holder.layer?.masksToBounds = true
            let tile = Pressable(holder, look: .lift, radius: MavioRadius.md, theme: theme) { [weak self] in
                self?.open(.library(library))
            }
            tile.setAccessibilityLabel(library.name)
            return tile
        }

        isolated deinit {
            observer?.cancel()
            loadTask?.cancel()
        }
    }

    /// Tiles of 16:9 in columns, gutter apart: a fixed number of them, else
    /// as many at least minimum wide as fit.
    private final class TileGrid: FlippedView {
        private let columns: Int?
        private let minimum: CGFloat
        private let gutter: CGFloat
        private var items: [NSView] = []
        private var height: CGFloat = 0

        init(columns: Int?, minimum: CGFloat, theme: Theme) {
            self.columns = columns
            self.minimum = minimum
            gutter = theme.layout.gutter
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        func setItems(_ views: [NSView]) {
            items.forEach { $0.removeFromSuperview() }
            items = views
            for v in views {
                v.translatesAutoresizingMaskIntoConstraints = true
                addSubview(v)
            }
            needsLayout = true
        }

        override var intrinsicContentSize: NSSize { NSSize(width: NSView.noIntrinsicMetric, height: height) }

        override func layout() {
            super.layout()
            let width = bounds.width
            guard width > 0, !items.isEmpty else { return setHeight(0) }
            let count = columns ?? max(1, Int((width + gutter) / (minimum + gutter)))
            let w = floor((width - gutter * CGFloat(count - 1)) / CGFloat(count))
            let h = round(w * 9 / 16)
            for (i, v) in items.enumerated() {
                v.frame = NSRect(
                    x: CGFloat(i % count) * (w + gutter), y: CGFloat(i / count) * (h + gutter), width: w, height: h)
            }
            let rows = (items.count + count - 1) / count
            setHeight(CGFloat(rows) * h + CGFloat(rows - 1) * gutter)
        }

        private func setHeight(_ h: CGFloat) {
            guard h != height else { return }
            height = h
            invalidateIntrinsicContentSize()
        }
    }

    // MARK: - Paging

    /// The pages of a grid of a server's items, loaded as it scrolls: the
    /// grid, of posters or, when every item is wide, of thumbnails; a header
    /// over it; and while it loads, fails or is empty, a line saying so
    /// under the header.
    @MainActor
    final class PagedGrid {
        /// What the grid fills.
        let view = NSView()
        /// Called as the count of every item becomes known.
        var onTotal: (@MainActor (UInt32?) -> Void)?
        private weak var page: Page?
        private let load: @MainActor (UInt32, UInt32) async throws -> MavioCore.Page
        private var grid: CardGrid?
        private var wide = false
        private let header = NSStackView.vertical([], spacing: 0)
        private var headerViews: [NSView] = []
        private var cards: [Card] = []
        private var total: UInt32?
        private var more = true
        private var loading = false
        private var failed = false
        private var generation = 0
        private var task: Task<Void, Never>?

        /// The cards of a page.
        static let limit: UInt32 = 60

        /// A grid on page whose pages load reads.
        init(page: Page, load: @escaping @MainActor (UInt32, UInt32) async throws -> MavioCore.Page) {
            self.page = page
            self.load = load
            view.translatesAutoresizingMaskIntoConstraints = false
            makeGrid(wide: false)
        }

        /// Shows views over the grid.
        func setHeader(_ views: [NSView]) {
            headerViews = views
            renderHeader()
        }

        /// Starts again from the first page, as a new order or filter does.
        func reset() {
            generation += 1
            task?.cancel()
            task = nil
            cards = []
            total = nil
            more = true
            loading = false
            failed = false
            onTotal?(nil)
            render()
            next()
        }

        /// Loads the next page, if there is one.
        func next() {
            guard more, !loading else { return }
            let generation = generation
            loading = true
            failed = false
            renderHeader()
            let offset = UInt32(cards.count)
            task = Task { [weak self] in
                guard let self else { return }
                do {
                    let result = try await self.load(offset, Self.limit)
                    guard generation == self.generation else { return }
                    self.cards += result.cards
                    self.total = result.total ?? self.total
                    self.more = result.more && !result.cards.isEmpty
                    self.onTotal?(self.total)
                } catch {
                    guard generation == self.generation else { return }
                    self.failed = !(error is CancellationError)
                }
                self.loading = false
                self.render()
            }
        }

        private func makeGrid(wide: Bool) {
            guard let page else { return }
            self.wide = wide
            grid?.removeFromSuperview()
            let grid = CardGrid(wide: wide, theme: page.theme)
            grid.onReachEnd = { [weak self] in self?.next() }
            self.grid = grid
            view.addFilling(grid)
        }

        private func render() {
            guard let page else { return }
            let wide = !cards.isEmpty && cards.allSatisfy(isWide)
            if wide != self.wide { makeGrid(wide: wide) }
            renderHeader()
            let cards = cards
            let theme = page.theme
            let title = NSTextField.label("Xg", theme.fonts.headline).fittingSize.height
            let subtitle = NSTextField.label("Xg", theme.fonts.footnote).fittingSize.height
            grid?.show(
                count: cards.count,
                height: { width in ceil(width / (wide ? 16 / 9 : 2 / 3) + MavioSpacing.s4 + title + subtitle) },
                make: { [unowned page] in page.itemCard(cards[$0], wide: wide) })
            FocusEngine.active(for: view)?.schedule()
        }

        private func renderHeader() {
            guard let page else { return }
            var views = headerViews
            if loading && cards.isEmpty {
                let spinner = NSProgressIndicator()
                spinner.style = .spinning
                spinner.controlSize = .small
                spinner.startAnimation(nil)
                spinner.translatesAutoresizingMaskIntoConstraints = false
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addSubview(spinner)
                spinner.center(in: holder)
                holder.size(height: 72)
                views.append(holder)
            } else if failed {
                views.append(
                    StatusView(
                        icon: "error", title: MavioStrings.stateError,
                        action: (MavioStrings.actionRetry, { [weak self] in self?.next() }), theme: page.theme))
            } else if cards.isEmpty && !more {
                views.append(StatusView(title: MavioStrings.stateEmpty, theme: page.theme))
            }
            header.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: header.widthAnchor).isActive = true }
            grid?.setHeader(views.isEmpty ? nil : header)
        }

        isolated deinit { task?.cancel() }
    }

    // MARK: - Library

    /// A library: its Recommended tab of hubs beside the grid of all its
    /// items, sorted and filtered, as the web UI's: the tabs under the name,
    /// and over the grid the genre, keyword or studio it is narrowed to, the
    /// filters, the order and the count. In television mode its name heads
    /// it beside Recommended and All in a pill, and focus on them or on the
    /// filters chooses them (docs/tv.md §6.3).
    final class LibraryPage: Page {
        private var library: LibraryEntry?
        private let serverId: String
        private let libraryId: String
        private var tab: Tab
        private var rows: [Row]?
        private var sort: Sort = .name
        private var filter: Filter
        private var revision: Int?
        private var observer: Observer?
        private var loadTask: Task<Void, Never>?
        private var rowsTask: Task<Void, Never>?
        private var paged: PagedGrid?
        private var countLabel: NSTextField?
        private var valueChip: NSView?
        private var choiceChips: [Pressable] = []
        private var choicePills: PillTabs?
        private var sortButton: NSView?

        private enum Tab { case recommended, all }

        /// The filters, as the pills name them.
        private enum Choice: CaseIterable {
            case all, unplayed, played, favorites

            var label: String {
                switch self {
                case .all: MavioStrings.libraryFilterAll
                case .unplayed: MavioStrings.libraryFilterUnplayed
                case .played: MavioStrings.libraryFilterPlayed
                case .favorites: MavioStrings.libraryFilterFavorites
                }
            }

            /// The filter with this choice, keeping the value it is narrowed to.
            func applied(to filter: Filter) -> Filter {
                var f = filter
                f.played = self == .unplayed ? false : self == .played ? true : nil
                f.favorite = self == .favorites
                return f
            }
        }

        private static let sorts: [(Sort, String)] = [
            (.name, MavioStrings.librarySortName), (.dateAdded, MavioStrings.librarySortDateAdded),
            (.released, MavioStrings.librarySortReleased), (.rating, MavioStrings.librarySortRating),
            (.lastPlayed, MavioStrings.librarySortLastPlayed), (.random, MavioStrings.librarySortRandom),
        ]

        /// A library on its Recommended tab, or on its grid narrowed by a
        /// filter, as an item's genres, keywords and studios open it.
        init(library: LibraryEntry, filter: Filter? = nil, context: MacContext) {
            self.library = library
            serverId = library.serverId
            libraryId = library.id
            self.filter = filter ?? Filter(played: nil, favorite: false, genre: nil)
            tab = filter == nil ? .recommended : .all
            super.init(context: context)
        }

        /// A library of a server by its ID, its grid narrowed by a filter.
        init(serverId: String, libraryId: String, filter: Filter, context: MacContext) {
            self.serverId = serverId
            self.libraryId = libraryId
            self.filter = filter
            tab = .all
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = library?.name
            if library == nil { find() } else { start() }
        }

        /// Looks the library up by its ID.
        private func find() {
            show(LoadingView())
            loadTask?.cancel()
            loadTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let all = try await self.model.client.libraries(serverId: self.serverId).libraries
                    guard !Task.isCancelled else { return }
                    if let found = all.first(where: { $0.id == self.libraryId }) {
                        self.library = found
                        self.title = found.name
                        self.start()
                    } else {
                        self.showError(MavioStrings.librariesNotFound) { [weak self] in self?.find() }
                    }
                } catch is CancellationError {
                } catch {
                    guard !Task.isCancelled else { return }
                    self.showError(error.reason) { [weak self] in self?.find() }
                }
            }
        }

        private func start() {
            guard let library else { return }
            let paged = PagedGrid(page: self) { [weak self] offset, limit in
                guard let self else { throw CancellationError() }
                return try await self.model.client.libraryItems(
                    serverId: library.serverId, libraryId: library.id, sort: self.sort, filter: self.filter,
                    offset: offset, limit: limit)
            }
            paged.onTotal = { [weak self] in self?.showTotal($0) }
            self.paged = paged
            build()
            observer = Observer { [weak self] in self?.observe() }
        }

        private func observe() {
            let now = model.revision(serverId)
            guard now != revision else { return }
            let first = revision == nil
            revision = now
            loadRows(switching: first)
            if !first, pagedStarted { paged?.reset() }
        }

        /// The Recommended tab's rows, kept when they cannot be read again;
        /// the first time, none switch to All.
        private func loadRows(switching: Bool) {
            rowsTask?.cancel()
            let client = model.client
            let serverId = serverId
            let libraryId = libraryId
            rowsTask = Task { [weak self] in
                let loaded = try? await client.recommended(serverId: serverId, libraryId: libraryId)
                guard let self, !Task.isCancelled else { return }
                if let loaded { self.rows = loaded }
                if switching, self.rows?.isEmpty == true, self.tab == .recommended {
                    self.choose(.all)
                } else if self.tab == .recommended {
                    self.build()
                }
            }
        }

        private func choose(_ tab: Tab) {
            guard tab != self.tab else { return }
            self.tab = tab
            build()
        }

        /// Whether the grid has loaded, which it does as All first shows.
        private var pagedStarted = false

        // MARK: Building

        private func build() {
            guard let library else { return }
            let tabs = tabsView(library)
            switch tab {
            case .recommended:
                var views: [NSView] = [tabs]
                if let rows {
                    views += rows.map { rowView($0, several: false) }
                    if rows.isEmpty { views.append(StatusView(title: MavioStrings.stateEmpty, theme: theme)) }
                } else {
                    let loading = LoadingView()
                    loading.size(height: 120)
                    views.append(loading)
                }
                let page = ScrollPage(
                    views, spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s8, right: 0))
                show(page)
            case .all:
                guard let paged else { return }
                if !pagedStarted {
                    pagedStarted = true
                    paged.reset()
                }
                let filters = filtersView()
                paged.setHeader([tabs, filters])
                show(paged.view)
            }
            if let engine = FocusEngine.active(for: view) {
                if theme.isTV,
                    let pills = tabs.subviews.compactMap({ $0 as? NSStackView }).first?.arrangedSubviews
                        .compactMap({ $0 as? PillTabs }).first, let current = pills.currentChoice as? Focusable
                {
                    engine.prefer(current)
                }
                engine.schedule()
            }
        }

        /// The library's name with Recommended and All beside it: a segmented
        /// control on the desktop, a pill in television mode.
        private func tabsView(_ library: LibraryEntry) -> NSView {
            let index = tab == .recommended ? 0 : 1
            if theme.isTV {
                let pills = PillTabs(
                    [MavioStrings.libraryRecommended, MavioStrings.libraryAll], selected: index, theme: theme)
                pills.choosesOnFocus = true
                pills.onChange = { [weak self] in self?.choose($0 == 0 ? .recommended : .all) }
                let name = NSTextField.label(library.name, theme.fonts.title2, color: theme.colors.textPrimary)
                let row = NSStackView.horizontal([name, pills], spacing: MavioSpacing.s32)
                return margined(row, top: MavioSpacing.s48, bottom: MavioSpacing.s24)
            }
            let control = NSSegmentedControl(
                labels: [MavioStrings.libraryRecommended, MavioStrings.libraryAll], trackingMode: .selectOne,
                target: self, action: #selector(tabChanged(_:)))
            control.selectedSegment = index
            control.translatesAutoresizingMaskIntoConstraints = false
            let name = NSTextField.label(library.name, theme.fonts.title1, color: theme.colors.textPrimary)
            let row = NSStackView.horizontal([name, control, Spacer()], spacing: MavioSpacing.s24)
            row.alignment = .lastBaseline
            return margined(row, top: MavioSpacing.s24, bottom: MavioSpacing.s12)
        }

        @objc private func tabChanged(_ sender: NSSegmentedControl) {
            choose(sender.selectedSegment == 0 ? .recommended : .all)
        }

        /// The genre, keyword or studio the grid is narrowed to, named.
        private var value: String? {
            if let genre = filter.genre { return MavioStrings.libraryGenreValue(value: genre) }
            if let tag = filter.tag { return MavioStrings.libraryTagValue(value: tag) }
            if let studio = filter.studio { return MavioStrings.libraryStudioValue(value: studio) }
            return nil
        }

        private var choice: Int {
            Choice.allCases.firstIndex { $0.applied(to: filter) == filter } ?? 0
        }

        /// Over the grid: the value it is narrowed to, which clears it, the
        /// filters as pills, the order's menu and the count, wrapping as the
        /// width ends.
        private func filtersView() -> NSView {
            var views: [NSView] = []
            if let value {
                let chip = chip(
                    value, icon: "close", style: theme.fonts.callout, fill: theme.colors.accent,
                    color: theme.colors.onAccent, height: theme.isTV ? 56 : 32,
                    insets: NSEdgeInsets(top: 0, left: 14, bottom: 0, right: 10)
                ) { [weak self] in self?.clearValue() }
                chip.setAccessibilityLabel("\(value), \(MavioStrings.libraryClearValue)")
                valueChip = chip
                views.append(chip)
            }
            if theme.isTV {
                let pills = PillTabs(Choice.allCases.map(\.label), selected: choice, theme: theme)
                pills.choosesOnFocus = true
                pills.onChange = { [weak self] in self?.apply(Choice.allCases[$0]) }
                choicePills = pills
                views.append(pills)
            } else {
                choiceChips = Choice.allCases.map { c in
                    chip(c.label, style: theme.fonts.callout, weight: .medium, height: 32) { [weak self] in
                        self?.apply(c)
                    }
                }
                styleChoices()
                views += choiceChips
            }
            let sortTitle = MavioStrings.librarySortBy(sort: Self.sorts.first { $0.0 == sort }?.1 ?? "")
            let sortButton: NSView
            if theme.isTV {
                let button = MavioButton(sortTitle, icon: "sort", kind: .secondary, theme: theme)
                button.onPress = { [weak self, weak button] in
                    guard let self, let button else { return }
                    popUp(self.sortMenu(), under: button)
                }
                sortButton = button
            } else {
                let button = chip(
                    sortTitle, leadingIcon: "sort", style: theme.fonts.callout, weight: .regular,
                    color: theme.colors.textSecondary, height: 32,
                    insets: NSEdgeInsets(top: 0, left: MavioSpacing.s4, bottom: 0, right: MavioSpacing.s12))
                button.onPress = { [weak self, weak button] in
                    guard let self, let button else { return }
                    popUp(self.sortMenu(), under: button)
                }
                sortButton = button
            }
            self.sortButton = sortButton
            views.append(sortButton)
            let count = NSTextField.label("", theme.fonts.callout, color: theme.colors.textTertiary)
            countLabel = count
            views.append(count)
            let wrap = WrapStack(views, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
            return margined(wrap, bottom: theme.isTV ? 0 : MavioSpacing.s16)
        }

        private func styleChoices() {
            let current = choice
            for (i, chip) in choiceChips.enumerated() {
                let on = i == current
                chip.restingFill = on ? theme.colors.button : theme.colors.fill
                if let label = chip.subviews.compactMap({ $0 as? NSStackView }).first?.arrangedSubviews.first
                    as? NSTextField
                {
                    label.textColor = on ? theme.colors.onButton : theme.colors.textPrimary
                }
                chip.setAccessibilitySelected(on)
            }
        }

        private func sortMenu() -> NSMenu {
            let menu = NSMenu()
            for (s, title) in Self.sorts {
                let item = MenuAction(title) { [weak self] in self?.setSort(s) }
                item.state = s == sort ? .on : .off
                menu.addItem(item)
            }
            return menu
        }

        private func showTotal(_ total: UInt32?) {
            countLabel?.stringValue = total.map { MavioStrings.libraryItemCount(count: Int($0)) } ?? ""
        }

        private func setSort(_ sort: Sort) {
            guard sort != self.sort else { return }
            self.sort = sort
            paged?.reset()
            build()
        }

        private func apply(_ choice: Choice) {
            let applied = choice.applied(to: filter)
            guard applied != filter else { return }
            filter = applied
            paged?.reset()
            if theme.isTV {
                choicePills?.choose(Choice.allCases.firstIndex(of: choice) ?? 0)
            } else {
                styleChoices()
            }
        }

        private func clearValue() {
            filter.genre = nil
            filter.tag = nil
            filter.studio = nil
            paged?.reset()
            build()
        }

        isolated deinit {
            observer?.cancel()
            loadTask?.cancel()
            rowsTask?.cancel()
        }
    }

    // MARK: - Hub

    /// Every item of a hub.
    final class HubPage: Page {
        private let serverId: String
        private let hubId: String
        private var paged: PagedGrid?
        private var observer: Observer?
        private var revision: Int?

        /// The items of a hub of a server, titled title.
        init(serverId: String, hubId: String, title: String, context: MacContext) {
            self.serverId = serverId
            self.hubId = hubId
            super.init(context: context)
            self.title = title
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            let serverId = serverId
            let hubId = hubId
            let client = model.client
            let paged = PagedGrid(page: self) { offset, limit in
                try await client.hub(serverId: serverId, hubId: hubId, offset: offset, limit: limit)
            }
            self.paged = paged
            if theme.isTV, let title {
                paged.setHeader([
                    margined(
                        NSTextField.label(title, theme.fonts.title2, color: theme.colors.textPrimary),
                        top: MavioSpacing.s48, bottom: MavioSpacing.s8)
                ])
            }
            show(paged.view)
            observer = Observer { [weak self] in self?.observe() }
        }

        private func observe() {
            let now = model.revision(serverId)
            guard now != revision else { return }
            revision = now
            paged?.reset()
        }

        isolated deinit { observer?.cancel() }
    }
#endif
