#if os(macOS)
    import AppKit
    import MavioMac

    /// The desktop layout: the sidebar with the search field, Home,
    /// Discover, Downloads and the libraries, the user's menu at its foot,
    /// and the place chosen beside it, as the desktop app's
    /// (docs/clients.md §6.3, Desktop), without its rail: the window's
    /// controls would straddle it.
    @MainActor
    final class BrowseController: SidebarSplitController {
        private unowned let root: RootController
        private let sidebar: SidebarController
        private let places: PlacesController

        init(root: RootController) {
            self.root = root
            places = PlacesController(context: root.context)
            sidebar = SidebarController(root: root)
            super.init(theme: root.context.theme)
            sidebar.onSelect = { [weak self] in self?.select($0) }
            sidebar.onSearch = { [weak self] in self?.search($0) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func viewDidLoad() {
            super.viewDidLoad()
            addSidebar(sidebar)
            let content = NSSplitViewItem(viewController: places)
            content.titlebarSeparatorStyle = .none
            addSplitViewItem(content)
            root.context.sidebarSearches = true
            select(.home)
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

        /// Puts the cursor in the sidebar's field; with Search shown, in the
        /// page's own.
        func focusSearch() {
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
        /// The Watch Together group, at the bottom right over the pages.
        private lazy var group = GroupIndicator(context: context)

        init(context: MacContext) {
            self.context = context
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let box = Box(fill: context.theme.colors.background)
            box.frame = NSRect(x: 0, y: 0, width: 900, height: 700)
            box.addSubview(group)
            NSLayoutConstraint.activate([
                group.trailingAnchor.constraint(equalTo: box.trailingAnchor, constant: -MavioSpacing.s16),
                group.bottomAnchor.constraint(equalTo: box.bottomAnchor, constant: -MavioSpacing.s16),
            ])
            group.page = self
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
                view.addSubview(stack.view, positioned: .below, relativeTo: group)
                stack.view.pin(to: view)
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
    /// controls with the assistant's button at its end, the search field,
    /// Home, Discover where a server discovers, Downloads, and the libraries
    /// of every server, by server with several; the user's button at its
    /// foot.
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
        private var entries: [Entry] = []
        private var libraries: [String: [LibraryEntry]] = [:]
        private var observer: Observer?
        private var librariesKey = ""
        private var loading: Task<Void, Never>?
        private var shown: Place = .home
        var onSelect: (@MainActor (Place) -> Void)?
        /// Called with the query confirmed in the search field.
        var onSearch: (@MainActor (String) -> Void)?

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
            // The assistant's button at the end of the band.
            let header = NSStackView.horizontal([], spacing: MavioSpacing.s4)
            if let assistant = context.assistant {
                header.addArrangedSubview(AssistantButton(assistant: assistant, theme: theme))
            }
            let line = Box(fill: theme.colors.border)
            line.size(height: 1)
            let user = UserButton(context: context) { [weak self] button in self?.root.toggleUserMenu(from: button) }
            for v in [header, field, scroll, line, user] as [NSView] { view.addSubview(v) }
            // The band under the window's controls is as tall as the title bar.
            let band = view.safeAreaLayoutGuide.topAnchor
            NSLayoutConstraint.activate([
                scroll.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                scroll.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                scroll.bottomAnchor.constraint(equalTo: line.topAnchor),
                line.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                line.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                user.topAnchor.constraint(equalTo: line.bottomAnchor, constant: MavioSpacing.s8),
                user.bottomAnchor.constraint(equalTo: view.bottomAnchor, constant: -MavioSpacing.s8),
                user.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: MavioSpacing.s8),
                user.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s8),
                header.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s8),
                header.centerYAnchor.constraint(equalTo: view.topAnchor, constant: 24.5),
                field.topAnchor.constraint(equalTo: band, constant: MavioSpacing.s4),
                field.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: MavioSpacing.s12),
                field.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -MavioSpacing.s12),
                scroll.topAnchor.constraint(equalTo: field.bottomAnchor, constant: MavioSpacing.s8),
            ])
            self.view = view
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

        override func viewDidAppear() {
            super.viewDidAppear()
            // The window gives its first key view, the search field, the
            // cursor when it opens; the app opens with nothing focused, the
            // field taking the cursor when clicked or by ⌘F.
            if field.currentEditor() != nil { view.window?.makeFirstResponder(nil) }
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
            var next: [Entry] = [.place(.home, MavioStrings.navHome, "home")]
            if context.discovery.server != nil { next.append(.place(.discover, MavioStrings.navDiscover, "discover")) }
            next.append(.place(.downloads, MavioStrings.navDownloads, "downloads"))
            next.append(.header(MavioStrings.navLibraries))
            let servers = context.model.signedInServers
            for server in servers {
                let list = libraries[server.id] ?? []
                if servers.count > 1 && !list.isEmpty { next.append(.header(server.name)) }
                for library in list {
                    next.append(.place(.library(library), library.name, libraryIcon(library.kind)))
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
            if case .header = entries[row] { true } else { false }
        }

        func tableView(_ tableView: NSTableView, shouldSelectRow row: Int) -> Bool {
            if case .place = entries[row] { true } else { false }
        }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            switch entries[row] {
            case .header(let title):
                let cell = NSTableCellView()
                let label = NSTextField.label(title, context.theme.fonts.caption, color: .secondaryLabelColor)
                cell.addSubview(label)
                NSLayoutConstraint.activate([
                    label.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: MavioSpacing.s4),
                    label.trailingAnchor.constraint(lessThanOrEqualTo: cell.trailingAnchor),
                    label.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
                ])
                return cell
            case .place(let place, let title, let icon):
                let cell = PlaceCell(title: title, icon: icon)
                cell.isShown = { [weak self] in self?.shown == place }
                return cell
            }
        }

        func tableViewSelectionDidChange(_ notification: Notification) {
            let row = table.selectedRow
            guard entries.indices.contains(row), case .place(let place, _, _) = entries[row] else { return }
            if place != shown { onSelect?(place) }
        }

        func tableView(_ tableView: NSTableView, heightOfRow row: Int) -> CGFloat {
            if case .header = entries[row] { 28 } else { 30 }
        }

        func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
            SidebarRowView()
        }
    }

    /// A place of the sidebar: its icon, filled while it shows, and its
    /// title.
    final class PlaceCell: NSTableCellView {
        private let icon: IconView
        var isShown: (@MainActor () -> Bool)?

        init(title: String, icon name: String) {
            icon = IconView(name, size: 18)
            super.init(frame: .zero)
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
            stack.setViews([
                AvatarView(name: name, size: 32, theme: theme), lines,
                IconView("selector", size: 16, color: theme.colors.textTertiary),
            ])
            setAccessibilityLabel(MavioStrings.navUserMenu(name: name))
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
