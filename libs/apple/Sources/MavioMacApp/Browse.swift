#if os(macOS)
    import AppKit
    import MavioMac

    /// The desktop layout: the sidebar with the search field, Home,
    /// Discover, Downloads and the libraries, the user's menu at its foot,
    /// and the place chosen beside it. Collapsed, the sidebar is a rail of
    /// icons, as the desktop app's (docs/clients.md §6.3, Desktop).
    @MainActor
    final class BrowseController: NSSplitViewController {
        private unowned let root: RootController
        private let sidebar: SidebarController
        private let places: PlacesController
        private var side: NSSplitViewItem?
        private static let railKey = "sidebarCollapsed"
        private static let widthKey = "sidebarWidth"

        init(root: RootController) {
            self.root = root
            places = PlacesController(context: root.context)
            sidebar = SidebarController(root: root)
            super.init(nibName: nil, bundle: nil)
            sidebar.onSelect = { [weak self] in self?.select($0) }
            sidebar.onSearch = { [weak self] in self?.search($0) }
            sidebar.onToggle = { [weak self] in self?.toggleSidebar(nil) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func viewDidLoad() {
            super.viewDidLoad()
            let side = NSSplitViewItem(sidebarWithViewController: sidebar)
            side.canCollapse = false
            side.holdingPriority = .init(260)
            addSplitViewItem(side)
            self.side = side
            let content = NSSplitViewItem(viewController: places)
            content.titlebarSeparatorStyle = .none
            addSplitViewItem(content)
            splitView.dividerStyle = .thin
            setRail(UserDefaults.standard.bool(forKey: Self.railKey))
            select(.home)
        }

        /// Collapses the sidebar to a rail of icons, or expands it.
        override func toggleSidebar(_ sender: Any?) {
            setRail(!sidebar.rail)
            UserDefaults.standard.set(sidebar.rail, forKey: Self.railKey)
        }

        override func splitViewDidResizeSubviews(_ notification: Notification) {
            super.splitViewDidResizeSubviews(notification)
            guard !sidebar.rail, sidebar.isViewLoaded, sidebar.view.frame.width >= 200 else { return }
            UserDefaults.standard.set(Double(sidebar.view.frame.width), forKey: Self.widthKey)
        }

        private func setRail(_ rail: Bool) {
            guard let side else { return }
            sidebar.rail = rail
            root.context.sidebarSearches = !rail
            let rail = root.context.theme.layout.rail
            if sidebar.rail {
                side.minimumThickness = rail
                side.maximumThickness = rail
                splitView.setPosition(rail, ofDividerAt: 0)
            } else {
                side.minimumThickness = 200
                side.maximumThickness = 400
                let saved = UserDefaults.standard.double(forKey: Self.widthKey)
                splitView.setPosition(saved >= 200 ? saved : 240, ofDividerAt: 0)
            }
        }

        /// Shows place; choosing the place shown returns to its top.
        func select(_ place: Place) {
            places.select(place)
            sidebar.show(place)
        }

        /// Opens route on place.
        func open(_ route: Route, on place: Place) {
            select(place)
            places.open(route)
        }

        /// Searches the query confirmed in the sidebar's field on Search.
        func search(_ query: String) {
            select(.search)
            places.search(query)
        }

        /// Puts the cursor in the sidebar's field, expanding the sidebar;
        /// with Search shown, in the page's own.
        func focusSearch() {
            if sidebar.rail { toggleSidebar(nil) }
            sidebar.focusSearch()
        }
    }

    /// The places' stacks, each kept as the user moves among them: in the
    /// window, hidden while another shows, so that coming back to a place
    /// does not lay its pages out anew.
    @MainActor
    final class PlacesController: NSViewController {
        private let context: MacContext
        private var stacks: [Place: NavigationController] = [:]
        private(set) var place: Place?

        init(context: MacContext) {
            self.context = context
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let box = Box(fill: context.theme.colors.background)
            box.frame = NSRect(x: 0, y: 0, width: 900, height: 700)
            view = box
        }

        func select(_ next: Place) {
            if next == place {
                stacks[next]?.popToRoot()
                return
            }
            if let place { stacks[place]?.view.isHidden = true }
            place = next
            if let stack = stacks[next] {
                stack.view.isHidden = false
                (stack.top as? Reappearing)?.reappeared()
            } else {
                let stack = NavigationController(root: rootPage(next))
                stack.keepsBelowTitleBar = true
                stacks[next] = stack
                addChild(stack)
                view.addFilling(stack.view)
            }
            NotificationCenter.default.post(name: NavigationController.didChange, object: stacks[next])
        }

        func open(_ route: Route) {
            guard let place, let stack = stacks[place] else { return }
            stack.push(context.page(route, context))
        }

        /// Searches query on Search's root page.
        func search(_ query: String) {
            guard let stack = stacks[.search] else { return }
            stack.popToRoot(animated: false)
            (stack.top as? QuerySearching)?.search(query: query)
        }

        private func rootPage(_ place: Place) -> NSViewController {
            switch place {
            case .search: context.page(.search, context)
            case .home: Routes.home(context)
            case .discover: Routes.discover(context)
            case .downloads: context.page(.downloads, context)
            case .library(let library): context.page(.library(library), context)
            }
        }
    }

    /// The icon of a library of a kind, as the web UI's.
    func libraryIcon(_ kind: LibraryType) -> String {
        switch kind {
        case .movies: "movies"
        case .shows: "shows"
        case .music, .musicVideos: "music"
        case .homeVideos: "homeVideos"
        case .books: "books"
        case .photos: "photos"
        case .collections: "collections"
        case .playlists: "playlists"
        case .mixed: "folder"
        }
    }

    /// The sidebar, as the desktop app's: a band under the window's
    /// controls with the button collapsing it at its end, the search field,
    /// Home, Discover where a server discovers, Downloads, and the libraries
    /// of every server, by server with several; the user's button at its
    /// foot. As a rail it is icons alone, Search one of them, the button
    /// expanding it under the window's controls.
    @MainActor
    final class SidebarController: NSViewController, NSTableViewDataSource, NSTableViewDelegate,
        NSSearchFieldDelegate
    {
        private enum Entry {
            case place(Place, String, String)
            case header(String)
        }

        private unowned let root: RootController
        private var context: MacContext { root.context }
        private let table = NSTableView()
        private let field = NSSearchField()
        private var toggle: Pressable?
        private var user: UserButton?
        private var expandedConstraints: [NSLayoutConstraint] = []
        private var railConstraints: [NSLayoutConstraint] = []
        private var entries: [Entry] = []
        private var libraries: [String: [LibraryEntry]] = [:]
        private var observer: Observer?
        private var librariesKey = ""
        private var loading: Task<Void, Never>?
        private var shown: Place = .home
        var onSelect: (@MainActor (Place) -> Void)?
        /// Called with the query confirmed in the search field.
        var onSearch: (@MainActor (String) -> Void)?
        /// Called by the button collapsing or expanding the sidebar.
        var onToggle: (@MainActor () -> Void)?

        /// Whether the sidebar is a rail of icons.
        var rail = false {
            didSet {
                guard rail != oldValue, isViewLoaded else { return }
                applyRail()
                reload()
            }
        }

        init(root: RootController) {
            self.root = root
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let theme = context.theme
            let view = NSView(frame: NSRect(x: 0, y: 0, width: 240, height: 700))
            let column = NSTableColumn(identifier: .init("place"))
            table.addTableColumn(column)
            table.headerView = nil
            table.style = .sourceList
            table.rowSizeStyle = .medium
            table.floatsGroupRows = false
            table.dataSource = self
            table.delegate = self
            table.backgroundColor = .clear
            let scroll = NSScrollView()
            scroll.documentView = table
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.automaticallyAdjustsContentInsets = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            field.placeholderString = MavioStrings.navSearch
            field.setAccessibilityLabel(MavioStrings.navSearch)
            field.sendsWholeSearchString = true
            field.sendsSearchStringImmediately = false
            field.target = self
            field.action = #selector(submitted(_:))
            field.controlSize = .large
            field.translatesAutoresizingMaskIntoConstraints = false
            let icon = IconView("sidebar", size: 18, color: theme.colors.textSecondary)
            let toggle = Pressable(icon, radius: MavioRadius.md, theme: theme) { [weak self] in self?.onToggle?() }
            toggle.size(width: 28, height: 28)
            self.toggle = toggle
            let line = Box(fill: theme.colors.border)
            line.size(height: 1)
            let user = UserButton(context: context) { [weak self] button in self?.root.toggleUserMenu(from: button) }
            self.user = user
            for v in [toggle, field, scroll, line, user] as [NSView] { view.addSubview(v) }
            // The band under the window's controls is as tall as the title bar.
            let band = view.safeAreaLayoutGuide.topAnchor
            NSLayoutConstraint.activate([
                scroll.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                scroll.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                scroll.bottomAnchor.constraint(equalTo: line.topAnchor),
                line.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                line.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                user.topAnchor.constraint(equalTo: line.bottomAnchor, constant: MavioSpacing.s8),
                user.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: MavioSpacing.s8),
                user.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s8),
                user.bottomAnchor.constraint(equalTo: view.bottomAnchor, constant: -MavioSpacing.s8),
            ])
            expandedConstraints = [
                toggle.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s8),
                toggle.centerYAnchor.constraint(equalTo: view.topAnchor, constant: 24.5),
                field.topAnchor.constraint(equalTo: band, constant: MavioSpacing.s4),
                field.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: MavioSpacing.s12),
                field.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s12),
                scroll.topAnchor.constraint(equalTo: field.bottomAnchor, constant: MavioSpacing.s8),
            ]
            railConstraints = [
                toggle.centerXAnchor.constraint(equalTo: view.centerXAnchor),
                toggle.topAnchor.constraint(equalTo: band),
                scroll.topAnchor.constraint(equalTo: toggle.bottomAnchor, constant: MavioSpacing.s8),
            ]
            self.view = view
            applyRail()
        }

        private func applyRail() {
            NSLayoutConstraint.deactivate(rail ? expandedConstraints : railConstraints)
            NSLayoutConstraint.activate(rail ? railConstraints : expandedConstraints)
            field.isHidden = rail
            let label = rail ? MavioStrings.navExpandSidebar : MavioStrings.navCollapseSidebar
            toggle?.setAccessibilityLabel(label)
            toggle?.toolTip = label
            user?.compact = rail
            table.style = rail ? .plain : .sourceList
            table.selectionHighlightStyle = rail ? .none : .regular
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            observer = Observer { [weak self] in
                guard let self else { return }
                let model = self.context.model
                let key = model.signedInServers.map { "\($0.id):\(model.revision($0.id))" }.joined(separator: ",")
                _ = self.context.discovery.server
                self.reload()
                if key != self.librariesKey {
                    self.librariesKey = key
                    self.loadLibraries()
                }
            }
        }

        isolated deinit {
            observer?.cancel()
            loading?.cancel()
        }

        @objc private func submitted(_ sender: NSSearchField) {
            let query = sender.stringValue.trimmingCharacters(in: .whitespaces)
            if !query.isEmpty { onSearch?(query) }
        }

        /// Puts the cursor in the search field.
        func focusSearch() {
            view.window?.makeFirstResponder(field)
        }

        private func loadLibraries() {
            loading?.cancel()
            let model = context.model
            let servers = model.signedInServers
            loading = Task { [weak self] in
                await withTaskGroup(of: (String, [LibraryEntry]?).self) { group in
                    for server in servers {
                        let client = model.client
                        group.addTask { (server.id, try? await client.libraries(serverId: server.id).libraries) }
                    }
                    for await (id, list) in group {
                        guard let self, !Task.isCancelled else { return }
                        if let list { self.libraries[id] = list }
                        self.reload()
                    }
                }
            }
        }

        private func reload() {
            var next: [Entry] = rail ? [.place(.search, MavioStrings.navSearch, "search")] : []
            next.append(.place(.home, MavioStrings.navHome, "home"))
            if context.discovery.server != nil { next.append(.place(.discover, MavioStrings.navDiscover, "discover")) }
            next.append(.place(.downloads, MavioStrings.navDownloads, "downloads"))
            next.append(.header(MavioStrings.navLibraries))
            let servers = context.model.signedInServers
            for server in servers {
                let list = libraries[server.id] ?? []
                if servers.count > 1 && !list.isEmpty && !rail { next.append(.header(server.name)) }
                for library in list {
                    let title = servers.count > 1 ? "\(library.name) · \(server.name)" : library.name
                    next.append(.place(.library(library), rail ? title : library.name, libraryIcon(library.kind)))
                }
            }
            entries = next
            table.reloadData()
            show(shown)
        }

        /// Marks place as the one shown; leaving Search empties the field.
        func show(_ place: Place) {
            if place != .search { field.stringValue = "" }
            shown = place
            let row = entries.firstIndex {
                if case .place(let p, _, _) = $0 { p == place } else { false }
            }
            if let row {
                table.selectRowIndexes([row], byExtendingSelection: false)
            } else {
                table.deselectAll(nil)
            }
            for i in 0..<entries.count {
                (table.view(atColumn: 0, row: i, makeIfNecessary: false) as? PlaceCell)?.refresh()
            }
        }

        func numberOfRows(in tableView: NSTableView) -> Int { entries.count }

        func tableView(_ tableView: NSTableView, isGroupRow row: Int) -> Bool {
            if case .header = entries[row] { !rail } else { false }
        }

        func tableView(_ tableView: NSTableView, shouldSelectRow row: Int) -> Bool {
            if case .place = entries[row] { true } else { false }
        }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            switch entries[row] {
            case .header(let title):
                let cell = NSTableCellView()
                if rail {
                    // A line between the places and the libraries.
                    let line = Box(fill: context.theme.colors.border)
                    line.translatesAutoresizingMaskIntoConstraints = false
                    cell.addSubview(line)
                    NSLayoutConstraint.activate([
                        line.heightAnchor.constraint(equalToConstant: 1),
                        line.widthAnchor.constraint(equalToConstant: 32),
                        line.centerXAnchor.constraint(equalTo: cell.centerXAnchor),
                        line.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
                    ])
                    return cell
                }
                let label = NSTextField.label(title, context.theme.fonts.caption, color: .secondaryLabelColor)
                cell.addSubview(label)
                NSLayoutConstraint.activate([
                    label.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: MavioSpacing.s4),
                    label.trailingAnchor.constraint(lessThanOrEqualTo: cell.trailingAnchor),
                    label.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
                ])
                return cell
            case .place(let place, let title, let icon):
                let cell = PlaceCell(title: title, icon: icon, rail: rail, theme: context.theme)
                cell.isShown = { [weak self] in self?.shown == place }
                return cell
            }
        }

        func tableViewSelectionDidChange(_ notification: Notification) {
            let row = table.selectedRow
            guard entries.indices.contains(row), case .place(let place, _, _) = entries[row] else { return }
            if place != shown { onSelect?(place) }
            if rail { show(place) }
        }

        func tableView(_ tableView: NSTableView, heightOfRow row: Int) -> CGFloat {
            if case .header = entries[row] { rail ? 17 : 28 } else { rail ? 44 : 30 }
        }
    }

    /// A place of the sidebar: its icon, filled while it shows, and its
    /// title; in the rail its icon alone, centered in a tile filled while
    /// it shows, its title a tooltip.
    final class PlaceCell: NSTableCellView {
        private let icon: IconView
        private let tile: Box?
        private let theme: Theme
        var isShown: (@MainActor () -> Bool)?

        init(title: String, icon name: String, rail: Bool, theme: Theme) {
            self.theme = theme
            icon = IconView(name, size: rail ? 20 : 18)
            tile = rail ? Box(radius: MavioRadius.md) : nil
            super.init(frame: .zero)
            if let tile {
                tile.translatesAutoresizingMaskIntoConstraints = false
                addSubview(tile)
                tile.addSubview(icon)
                icon.center(in: tile)
                NSLayoutConstraint.activate([
                    tile.widthAnchor.constraint(equalToConstant: 36),
                    tile.heightAnchor.constraint(equalToConstant: 36),
                    tile.centerXAnchor.constraint(equalTo: centerXAnchor),
                    tile.centerYAnchor.constraint(equalTo: centerYAnchor),
                ])
                toolTip = title
                setAccessibilityLabel(title)
                return
            }
            let label = NSTextField.label(title, .init(size: 13, lineHeight: 16, weight: .regular))
            label.textColor = .labelColor
            let stack = NSStackView.horizontal([icon, label], spacing: MavioSpacing.s8)
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.leadingAnchor.constraint(equalTo: leadingAnchor, constant: MavioSpacing.s4),
                stack.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor),
                stack.centerYAnchor.constraint(equalTo: centerYAnchor),
            ])
            textField = label
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        func refresh() {
            let shown = isShown?() ?? false
            icon.fillIcon = shown
            tile?.fill = shown ? theme.colors.fillSelected : nil
        }

        override func viewDidMoveToSuperview() {
            super.viewDidMoveToSuperview()
            refresh()
        }
    }

    /// The user's name and what they are signed in to, as the web UI's.
    @MainActor
    func signedIn(_ model: AppModel) -> (name: String, line: String) {
        let name = model.account?.name ?? model.signedInServers.first?.userName ?? ""
        let line =
            model.account != nil
            ? MavioStrings.menuServerCount(count: model.signedInServers.count)
            : model.signedInServers.first?.name ?? ""
        return (name, line)
    }

    /// The user's avatar and name at the sidebar's foot, opening their menu.
    final class UserButton: Box {
        private let context: MacContext
        private let press: @MainActor (NSView) -> Void
        private var observer: Observer?
        private let stack = NSStackView.horizontal([], spacing: 10)
        private var tracking: NSTrackingArea?

        init(context: MacContext, press: @escaping @MainActor (NSView) -> Void) {
            self.context = context
            self.press = press
            super.init(radius: 10)
            addFilling(stack, insets: .all(MavioSpacing.s8))
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            observer = Observer { [weak self] in self?.update() }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { observer?.cancel() }

        private func update() {
            let theme = context.theme
            let (name, line) = signedIn(context.model)
            let title = NSTextField.label(name, theme.fonts.callout, color: theme.colors.textPrimary)
            title.font = .systemFont(ofSize: theme.fonts.callout.size, weight: .semibold)
            let lines = NSStackView.vertical(
                [title, NSTextField.label(line, theme.fonts.caption, color: theme.colors.textSecondary)], spacing: 0)
            lines.setContentHuggingPriority(.init(1), for: .horizontal)
            let avatar = AvatarView(name: name, size: 32, theme: theme)
            // In the rail the avatar is centered between two empty views.
            stack.distribution = compact ? .equalCentering : .fill
            stack.setViews(
                compact
                    ? [NSView(), avatar, NSView()]
                    : [avatar, lines, IconView("selector", size: 16, color: theme.colors.textTertiary)])
            toolTip = compact ? name : nil
            setAccessibilityLabel(MavioStrings.navUserMenu(name: name))
        }

        /// Whether it is the avatar alone, in the rail.
        var compact = false {
            didSet {
                guard compact != oldValue else { return }
                update()
            }
        }

        /// Whether the menu it opens is open, which keeps it highlighted.
        var open = false { didSet { fill = open ? context.theme.colors.fill : nil } }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { press(self) }
        }

        override func accessibilityPerformPress() -> Bool {
            press(self)
            return true
        }

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseEntered(with event: NSEvent) { fill = context.theme.colors.fill }

        override func mouseExited(with event: NSEvent) { if !open { fill = nil } }
    }
#endif
