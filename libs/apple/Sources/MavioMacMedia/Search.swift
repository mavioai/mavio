#if os(macOS)
    import AppKit
    import MavioMac

    /// Search across every server, as the web UI's (docs/clients.md §5.4,
    /// §6.3.28): each server's results as it answers, in the order of the
    /// servers, then the titles the server that discovers finds, to
    /// request. On the desktop a search runs when it is confirmed with
    /// Return in the sidebar's field, or in the page's own while the
    /// sidebar is a rail, and each server's results are a row. In television mode
    /// (docs/tv.md §6.4) the query is typed on a keyboard of its own at the
    /// left, the search runs as it is typed, after a pause of 300 ms, and
    /// the right shows Home's rows while nothing is typed, then the results
    /// in grids.
    final class SearchPage: Page, QuerySearching, Reappearing {
        /// The query searched, empty before the first search.
        private var searched = ""
        /// The servers asked, and what each has answered.
        private var servers: [ServerRecord] = []
        private var results: [String: SearchResults] = [:]
        /// The titles found to discover, nil while they are being searched.
        private var titles: [DiscoveredTitle]?
        private var searchTask: Task<Void, Never>?
        private var titlesTask: Task<Void, Never>?
        private var debounce: Task<Void, Never>?
        private var observer: Observer?
        /// Whether discovery had loaded, and its server, when titles were last searched.
        private var discoveryKey: String?
        private let page = ScrollPage([], spacing: 0, insets: .symmetric(vertical: MavioSpacing.s8))
        private let field = NSSearchField()
        private let fieldHolder = NSView()
        private var keyboard: SearchKeyboard?

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navSearch
            if theme.isTV { buildTV() } else { buildDesktop() }
            observer = Observer { [weak self] in self?.observe() }
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            reappeared()
        }

        /// Puts the cursor in the page's own field, while nothing is searched.
        func reappeared() {
            if !theme.isTV, searched.isEmpty, !fieldHolder.isHidden { view.window?.makeFirstResponder(field) }
        }

        /// Searches the query confirmed in the sidebar's field.
        func search(query: String) {
            field.stringValue = query
            search(query)
        }

        /// Draws again as discovery loads, and in television mode as Home's
        /// rows change while nothing is searched.
        private func observe() {
            let discovery = context.discovery
            let key = "\(discovery.loaded)/\(discovery.server?.id ?? "")"
            let home = model.home
            if key != discoveryKey {
                discoveryKey = key
                if !searched.isEmpty { searchTitles(searched) }
            }
            if theme.isTV, searched.isEmpty {
                _ = home
                render()
            }
        }

        // MARK: Desktop

        private func buildDesktop() {
            field.placeholderString = MavioStrings.searchPlaceholder
            field.sendsWholeSearchString = true
            field.sendsSearchStringImmediately = false
            field.target = self
            field.action = #selector(submitted(_:))
            field.controlSize = .large
            field.font = theme.fonts.body.font
            field.translatesAutoresizingMaskIntoConstraints = false
            let holder = fieldHolder
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(field)
            NSLayoutConstraint.activate([
                field.leadingAnchor.constraint(equalTo: holder.leadingAnchor, constant: theme.layout.margin),
                field.topAnchor.constraint(equalTo: holder.topAnchor, constant: MavioSpacing.s8),
                field.bottomAnchor.constraint(equalTo: holder.bottomAnchor, constant: -MavioSpacing.s8),
                field.trailingAnchor.constraint(
                    lessThanOrEqualTo: holder.trailingAnchor, constant: -theme.layout.margin),
            ])
            let width = field.widthAnchor.constraint(equalToConstant: 560)
            width.priority = .defaultHigh
            width.isActive = true
            let root = NSView()
            root.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(holder)
            root.addSubview(page)
            // The page's own field shows only while the sidebar has none.
            let shown = holder.bottomAnchor.constraint(equalTo: page.topAnchor)
            let hidden = page.topAnchor.constraint(equalTo: root.topAnchor)
            fieldConstraints = (shown, hidden)
            NSLayoutConstraint.activate([
                holder.topAnchor.constraint(equalTo: root.topAnchor),
                holder.leadingAnchor.constraint(equalTo: root.leadingAnchor),
                holder.trailingAnchor.constraint(equalTo: root.trailingAnchor),
                page.leadingAnchor.constraint(equalTo: root.leadingAnchor),
                page.trailingAnchor.constraint(equalTo: root.trailingAnchor),
                page.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            ])
            show(root)
            sidebarSearchesChanged()
            NotificationCenter.default.addObserver(
                self, selector: #selector(sidebarSearchesChanged), name: MacContext.sidebarSearchesDidChange,
                object: context)
        }

        private var fieldConstraints: (shown: NSLayoutConstraint, hidden: NSLayoutConstraint)?

        @objc private func sidebarSearchesChanged() {
            let sidebar = context.sidebarSearches
            fieldHolder.isHidden = sidebar
            fieldConstraints?.shown.isActive = !sidebar
            fieldConstraints?.hidden.isActive = sidebar
        }

        @objc private func submitted(_ sender: NSSearchField) {
            // Confirming the same query again searches again.
            search(sender.stringValue.trimmingCharacters(in: .whitespaces))
        }

        // MARK: Television

        private func buildTV() {
            let keyboard = SearchKeyboard(theme: theme)
            keyboard.onChange = { [weak self] in self?.typed($0) }
            self.keyboard = keyboard
            // The right keeps only room for a focused card to grow: its
            // content is drawn from 24 pt left of the margin, clipped at
            // the column's edge.
            let right = NSView()
            right.translatesAutoresizingMaskIntoConstraints = false
            right.wantsLayer = true
            right.layer?.masksToBounds = true
            right.addSubview(page)
            let shift = theme.layout.margin - MavioSpacing.s24
            NSLayoutConstraint.activate([
                page.leadingAnchor.constraint(equalTo: right.leadingAnchor, constant: -shift),
                page.trailingAnchor.constraint(equalTo: right.trailingAnchor),
                page.topAnchor.constraint(equalTo: right.topAnchor),
                page.bottomAnchor.constraint(equalTo: right.bottomAnchor),
            ])
            let root = NSView()
            root.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(keyboard)
            root.addSubview(right)
            let preferred = keyboard.widthAnchor.constraint(equalTo: root.widthAnchor, multiplier: 0.3)
            preferred.priority = .defaultHigh
            NSLayoutConstraint.activate([
                keyboard.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: theme.layout.margin),
                keyboard.topAnchor.constraint(equalTo: root.topAnchor, constant: MavioSpacing.s48),
                keyboard.bottomAnchor.constraint(lessThanOrEqualTo: root.bottomAnchor),
                preferred,
                keyboard.widthAnchor.constraint(greaterThanOrEqualToConstant: 300),
                keyboard.widthAnchor.constraint(lessThanOrEqualToConstant: 480),
                right.leadingAnchor.constraint(equalTo: keyboard.trailingAnchor, constant: theme.layout.gutter),
                right.trailingAnchor.constraint(equalTo: root.trailingAnchor),
                right.topAnchor.constraint(equalTo: root.topAnchor),
                right.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            ])
            show(root)
            render()
            if let engine = FocusEngine.active(for: view), let first = keyboard.firstKey {
                engine.prefer(first)
                engine.schedule()
            }
        }

        /// Searches what is typed after a pause of 300 ms.
        private func typed(_ text: String) {
            debounce?.cancel()
            let q = text.trimmingCharacters(in: .whitespaces)
            debounce = Task { [weak self] in
                try? await Task.sleep(for: .milliseconds(300))
                guard !Task.isCancelled, let self, q != self.searched else { return }
                self.search(q)
            }
        }

        // MARK: Searching

        /// Asks every signed-in server at once, drawing each as it answers,
        /// and the server that discovers for titles.
        private func search(_ q: String) {
            searched = q
            searchTask?.cancel()
            servers = q.isEmpty ? [] : model.signedInServers
            results = [:]
            render()
            searchTitles(q)
            guard !q.isEmpty else { return }
            let servers = servers
            let client = model.client
            searchTask = Task { [weak self] in
                await withTaskGroup(of: SearchResults?.self) { group in
                    for server in servers {
                        group.addTask { try? await client.searchServer(serverId: server.id, query: q) }
                    }
                    for await found in group {
                        guard let found, !Task.isCancelled, let self else { continue }
                        self.results[found.serverId] = found
                        self.render()
                    }
                }
            }
        }

        /// Asks the server that discovers for titles of the query.
        private func searchTitles(_ q: String) {
            titlesTask?.cancel()
            titles = nil
            let discovery = context.discovery
            guard !q.isEmpty, discovery.loaded else { return }
            guard let server = discovery.server else {
                titles = []
                render()
                return
            }
            var request = Mavio_Acquisition_V1_SearchTitlesRequest()
            request.query = q
            request.language = discoverLanguage
            let model = model
            titlesTask = Task { [weak self] in
                let response = try? await model.call(
                    server, Procedure.discover("SearchTitles"), request,
                    as: Mavio_Acquisition_V1_SearchTitlesResponse.self)
                guard !Task.isCancelled, let self else { return }
                var seen = Set<String>()
                self.titles = (response?.titles ?? []).filter { seen.insert($0.key).inserted }
                self.render()
            }
        }

        /// Whether something is still searching and nothing has been drawn yet.
        private var waiting: Bool {
            let discovery = context.discovery
            let answering =
                servers.contains { results[$0.id] == nil } || !discovery.loaded
                || (discovery.server != nil && titles == nil)
            let drawn =
                servers.contains { results[$0.id].map { $0.error != nil || !$0.cards.isEmpty } ?? false }
                || titles?.isEmpty == false
            return answering && !drawn
        }

        /// Whether everything has answered and found nothing.
        private var none: Bool {
            servers.allSatisfy { results[$0.id]?.cards.isEmpty ?? false } && context.discovery.loaded
                && titles?.isEmpty == true
        }

        // MARK: Drawing

        private func render() {
            var views: [NSView] = []
            if theme.isTV && searched.isEmpty {
                let several = model.signedIn.count > 1
                views += model.home.map { rowView($0, several: several) }
            } else if !searched.isEmpty {
                let several = servers.count > 1
                for server in servers {
                    guard let found = results[server.id] else { continue }
                    if let error = found.error {
                        views.append(unreachable(server.name, reason: error))
                    } else if !found.cards.isEmpty {
                        views.append(
                            serverResults(
                                several ? server.name : MavioStrings.searchServer, found.cards, named: several))
                    }
                }
                if let server = context.discovery.server, let titles, !titles.isEmpty {
                    views.append(discovered(titles, serverId: server.id))
                }
                if waiting {
                    let loading = LoadingView()
                    loading.size(height: 120)
                    views.append(inset(loading, NSEdgeInsets(top: MavioSpacing.s40, left: 0, bottom: 0, right: 0)))
                } else if none {
                    views.append(
                        inset(
                            StatusView(icon: "search", title: MavioStrings.searchNoResults, theme: theme),
                            NSEdgeInsets(top: MavioSpacing.s40, left: 0, bottom: 0, right: 0)))
                }
            }
            page.setViews(views)
            FocusEngine.active(for: view)?.schedule()
        }

        /// A server's results: a row on the desktop, a grid in television
        /// mode, named by the server there only when there are several.
        private func serverResults(_ title: String, _ cards: [Card], named: Bool) -> NSView {
            if theme.isTV {
                return grid(named ? title : nil, cards.map { itemCard($0, wide: false) })
            }
            let shelf = Shelf(title, theme: theme)
            let width = theme.layout.poster
            shelf.row.show(count: cards.count) { [unowned self] in self.itemCard(cards[$0], wide: false, width: width) }
            return shelf
        }

        private func discovered(_ titles: [DiscoveredTitle], serverId: String) -> NSView {
            if theme.isTV {
                return grid(
                    MavioStrings.searchDiscover, titles.map { discoveredCard($0, serverId: serverId, width: nil) })
            }
            let shelf = Shelf(MavioStrings.searchDiscover, theme: theme)
            let width = theme.layout.poster
            shelf.row.show(count: titles.count) { [unowned self] in
                self.discoveredCard(titles[$0], serverId: serverId, width: width)
            }
            return shelf
        }

        /// Cards in columns at least a poster wide, under a heading.
        private func grid(_ title: String?, _ cards: [NSView]) -> NSView {
            let grid = FlowGrid(minimum: theme.layout.poster, gutter: theme.layout.gutter, rowSpacing: MavioSpacing.s48)
            grid.setItems(cards)
            var views: [NSView] = []
            if let title { views.append(heading(title)) }
            views.append(grid)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            grid.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return margined(stack, top: MavioSpacing.s12, bottom: MavioSpacing.s12)
        }

        isolated deinit {
            observer?.cancel()
            searchTask?.cancel()
            titlesTask?.cancel()
            debounce?.cancel()
            NotificationCenter.default.removeObserver(self)
        }
    }

    /// Television mode's keyboard of Search (docs/tv.md §6.4): the query,
    /// whose blinking caret shows where it types whether or not it has
    /// focus; keys for a space, deleting a character and clearing; and the
    /// letters and digits in six columns. A computer's keyboard types into
    /// it as well, and back deletes a character, going back only once there
    /// is none. It is a region of its own beside the results.
    final class SearchKeyboard: FlippedView, FocusRegion, KeyHandling {
        /// Called as the query changes.
        var onChange: (@MainActor (String) -> Void)?
        private(set) var text = ""
        private let theme: Theme
        private let query: NSTextField
        private let placeholder: NSTextField
        private let caret: Box
        private var keys: [Pressable] = []

        private static let characters = Array("abcdefghijklmnopqrstuvwxyz1234567890")

        /// A keyboard in theme.
        init(theme: Theme) {
            self.theme = theme
            query = NSTextField.label("", theme.fonts.title3, color: theme.colors.textPrimary)
            placeholder = NSTextField.label(
                MavioStrings.searchPlaceholder, theme.fonts.title3, color: theme.colors.textTertiary)
            caret = Box(fill: theme.colors.accent, radius: 1)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            build()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The first letter's key, where focus starts.
        var firstKey: NSView? { keys.dropFirst(3).first }

        private func build() {
            query.lineBreakMode = .byTruncatingHead
            caret.size(width: 2, height: ceil(theme.fonts.title3.size * 1.1))
            let blink = CABasicAnimation(keyPath: "opacity")
            blink.fromValue = 1
            blink.toValue = 0
            blink.duration = 0.5
            blink.autoreverses = true
            blink.repeatCount = .infinity
            caret.wantsLayer = true
            caret.layer?.add(blink, forKey: "blink")
            let line = NSStackView.horizontal(
                [IconView("search", size: 24, color: theme.colors.textSecondary), query, caret, placeholder],
                spacing: MavioSpacing.s12)
            line.setCustomSpacing(1, after: query)
            let box = Box(fill: theme.colors.surfaceRaised, radius: MavioRadius.lg)
            box.addFilling(line, insets: .symmetric(horizontal: MavioSpacing.s20))
            box.heightAnchor.constraint(greaterThanOrEqualToConstant: 64).isActive = true
            box.setAccessibilityElement(true)
            box.setAccessibilityRole(.staticText)
            let actions = [
                key(MavioStrings.searchSpace) { [weak self] in self?.type(" ") },
                key(MavioStrings.searchBackspace) { [weak self] in self?.erase() },
                key(MavioStrings.searchClear) { [weak self] in self?.set("") },
            ]
            let actionRow = Columns(actions, columns: 3, spacing: MavioSpacing.s8, rowHeight: 56)
            let letters = Self.characters.map { c in
                key(String(c).uppercased()) { [weak self] in self?.type(String(c)) }
            }
            keys = actions + letters
            let grid = Columns(letters, columns: 6, spacing: MavioSpacing.s8, rowHeight: nil)
            let stack = NSStackView.vertical([box, actionRow, grid], spacing: MavioSpacing.s16)
            for v in stack.arrangedSubviews { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            addFilling(stack)
            update()
        }

        private func key(_ title: String, action: @escaping @MainActor () -> Void) -> Pressable {
            let label = NSTextField.label(title, theme.fonts.headline, color: theme.colors.textPrimary)
            label.alignment = .center
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(label)
            label.center(in: holder)
            label.widthAnchor.constraint(lessThanOrEqualTo: holder.widthAnchor, constant: -MavioSpacing.s8).isActive =
                true
            let key = Pressable(holder, look: .highlight, radius: MavioRadius.lg, theme: theme, action: action)
            key.restingFill = theme.colors.fill
            key.setAccessibilityLabel(title)
            return key
        }

        private func type(_ s: String) { set(text + s) }

        private func erase() {
            guard !text.isEmpty else { return }
            set(String(text.dropLast()))
        }

        private func set(_ value: String) {
            guard value != text else { return }
            text = value
            update()
            onChange?(value)
        }

        private func update() {
            query.stringValue = text
            query.isHidden = text.isEmpty
            placeholder.isHidden = !text.isEmpty
            setAccessibilityValue(text)
        }

        func handleKey(_ event: NSEvent) -> Bool {
            let modifiers = event.modifierFlags.intersection([.command, .control, .option])
            guard modifiers.isEmpty else { return false }
            switch event.keyCode {
            case 51, 53:
                // Delete and Escape delete a character, and go back once there is none.
                guard !text.isEmpty else { return false }
                erase()
                return true
            case 36, 76, 48, 123, 124, 125, 126:
                return false
            default:
                guard let chars = event.characters, chars.count == 1, let c = chars.first,
                    c.isLetter || c.isNumber || c.isPunctuation || c.isSymbol || c == " "
                else { return false }
                type(String(c).lowercased())
                return true
            }
        }
    }

    /// Views in a fixed number of equal columns, spacing apart, rowHeight
    /// tall, or square without one.
    private final class Columns: FlippedView {
        private let items: [NSView]
        private let columns: Int
        private let spacing: CGFloat
        private let rowHeight: CGFloat?
        private var height: CGFloat = 0

        init(_ views: [NSView], columns: Int, spacing: CGFloat, rowHeight: CGFloat?) {
            items = views
            self.columns = columns
            self.spacing = spacing
            self.rowHeight = rowHeight
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            for v in views {
                v.translatesAutoresizingMaskIntoConstraints = true
                addSubview(v)
            }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var intrinsicContentSize: NSSize { NSSize(width: NSView.noIntrinsicMetric, height: height) }

        override func layout() {
            super.layout()
            let width = bounds.width
            guard width > 0 else { return }
            let w = floor((width - spacing * CGFloat(columns - 1)) / CGFloat(columns))
            let h = rowHeight ?? w
            for (i, v) in items.enumerated() {
                v.frame = NSRect(
                    x: CGFloat(i % columns) * (w + spacing), y: CGFloat(i / columns) * (h + spacing), width: w,
                    height: h)
            }
            let rows = (items.count + columns - 1) / columns
            let total = CGFloat(rows) * h + CGFloat(max(0, rows - 1)) * spacing
            if total != height {
                height = total
                invalidateIntrinsicContentSize()
            }
        }
    }
#endif
