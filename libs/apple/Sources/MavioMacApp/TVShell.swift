#if os(macOS)
    import AppKit
    import MavioMac
    import MavioMacMedia

    /// Television mode's layout: Home, Libraries, Discover where a server
    /// discovers, and Search in a pill at the top, the menu button at the
    /// right, each tab's stack under them; deeper pages and Search hide the
    /// bar, and once a page scrolls a band of the background lies under it
    /// (docs/tv.md §3–4).
    @MainActor
    final class TVShellController: NSViewController {
        private enum Tab: Hashable {
            case home, libraries, discover, search
        }

        private unowned let root: RootController
        private var context: MacContext { root.context }
        private var theme: Theme { root.context.theme }
        private var tabs: [Tab] = []
        private var pill: PillTabs?
        private let bar = FlippedView()
        private let band = Box()
        private let content = FlippedView()
        private let menuButton: TVMenuButton
        private var stacks: [Tab: NavigationController] = [:]
        private var tab: Tab = .home
        private var observer: Observer?
        private var scrollObservation: NSObjectProtocol?
        private weak var watchedScroll: NSScrollView?

        /// The bar's height, which pages start below.
        static let barHeight: CGFloat = 120

        init(root: RootController) {
            self.root = root
            menuButton = TVMenuButton(theme: root.context.theme)
            super.init(nibName: nil, bundle: nil)
            menuButton.press = { [weak self] button in self?.root.toggleUserMenu(from: button) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit {
            observer?.cancel()
            if let scrollObservation { NotificationCenter.default.removeObserver(scrollObservation) }
            NotificationCenter.default.removeObserver(self)
        }

        override func loadView() {
            let view = Box(fill: theme.colors.tvBackground)
            view.frame = NSRect(x: 0, y: 0, width: 1280, height: 800)
            content.translatesAutoresizingMaskIntoConstraints = false
            view.addFilling(content)
            band.fill = theme.colors.tvBackground
            band.translatesAutoresizingMaskIntoConstraints = false
            band.isHidden = true
            view.addSubview(band)
            bar.translatesAutoresizingMaskIntoConstraints = false
            view.addSubview(bar)
            bar.addSubview(menuButton)
            NSLayoutConstraint.activate([
                band.topAnchor.constraint(equalTo: view.topAnchor),
                band.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                band.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                band.heightAnchor.constraint(equalToConstant: Self.barHeight),
                bar.topAnchor.constraint(equalTo: view.topAnchor),
                bar.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                bar.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                bar.heightAnchor.constraint(equalToConstant: Self.barHeight),
                menuButton.trailingAnchor.constraint(equalTo: bar.trailingAnchor, constant: -theme.layout.margin),
                menuButton.centerYAnchor.constraint(equalTo: bar.centerYAnchor),
            ])
            // The assistant button before the user's menu, as the web UI's.
            if let assistant = root.context.assistant {
                let button = AssistantButton(
                    assistant: assistant, theme: theme, side: 64, size: 30, color: theme.colors.textSecondary,
                    fill: theme.colors.surfaceRaised.withAlphaComponent(0.6))
                bar.addSubview(button)
                NSLayoutConstraint.activate([
                    button.trailingAnchor.constraint(equalTo: menuButton.leadingAnchor, constant: -MavioSpacing.s8),
                    button.centerYAnchor.constraint(equalTo: bar.centerYAnchor),
                ])
            }
            self.view = view
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            NotificationCenter.default.addObserver(
                self, selector: #selector(navigationChanged(_:)), name: NavigationController.didChange, object: nil)
            observer = Observer { [weak self] in
                guard let self else { return }
                let discovers = self.context.discovery.server != nil
                if discovers != self.tabs.contains(.discover) || self.tabs.isEmpty {
                    self.buildTabs(discovers: discovers)
                }
            }
            show(.home)
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            if let window = view.window {
                FocusEngine.of(window).onEdge = { [weak self] direction in self?.edge(direction) ?? false }
            }
        }

        private func buildTabs(discovers: Bool) {
            tabs = [.home, .libraries] + (discovers ? [.discover] : []) + [.search]
            if !tabs.contains(tab) { show(.home) }
            pill?.removeFromSuperview()
            let titles = tabs.map { tab in
                switch tab {
                case .home: MavioStrings.navHome
                case .libraries: MavioStrings.navLibraries
                case .discover: MavioStrings.navDiscover
                case .search: MavioStrings.navSearch
                }
            }
            let pill = PillTabs(titles, selected: tabs.firstIndex(of: tab) ?? 0, theme: theme)
            pill.chosenOnlyBySelect = [tabs.count - 1]
            pill.onChange = { [weak self] index in
                guard let self, self.tabs.indices.contains(index) else { return }
                self.choose(self.tabs[index])
            }
            pill.translatesAutoresizingMaskIntoConstraints = false
            bar.addSubview(pill)
            NSLayoutConstraint.activate([
                pill.centerXAnchor.constraint(equalTo: bar.centerXAnchor),
                pill.centerYAnchor.constraint(equalTo: bar.centerYAnchor),
            ])
            menuButton.heightAnchor.constraint(equalTo: pill.heightAnchor).isActive = true
            self.pill = pill
            updateBar()
        }

        // MARK: Tabs

        /// A tab chosen in the pill: its own page, or, chosen again, its top.
        private func choose(_ next: Tab) {
            if next == tab {
                stacks[next]?.popToRoot()
            } else {
                show(next)
            }
        }

        private func show(_ next: Tab) {
            if let shown = stacks[tab], next != tab {
                shown.view.removeFromSuperview()
                shown.removeFromParent()
            }
            tab = next
            pill?.choose(tabs.firstIndex(of: next) ?? 0)
            let stack = stacks[next] ?? NavigationController(root: rootPage(next))
            stacks[next] = stack
            if stack.parent == nil {
                addChild(stack)
                content.addFilling(stack.view)
            }
            NotificationCenter.default.post(name: NavigationController.didChange, object: stack)
        }

        private func rootPage(_ tab: Tab) -> NSViewController {
            switch tab {
            case .home: Routes.home(context)
            case .libraries: Routes.libraries(context)
            case .discover: Routes.discover(context)
            case .search: context.page(.search, context)
            }
        }

        /// Shows place: Search, Home, Discover, or a library over Libraries.
        func select(_ place: Place) {
            switch place {
            case .search: show(.search)
            case .home: show(.home)
            case .discover: show(tabs.contains(.discover) ? .discover : .home)
            case .downloads:
                show(.home)
                stacks[.home]?.push(context.page(.downloads, context))
            case .library(let library):
                show(.libraries)
                stacks[.libraries]?.popToRoot(animated: false)
                stacks[.libraries]?.push(context.page(.library(library), context))
            }
        }

        /// Opens route over Home.
        func open(_ route: Route) {
            show(.home)
            stacks[.home]?.push(context.page(route, context))
        }

        // MARK: The bar

        @objc private func navigationChanged(_ notification: Notification) {
            guard let nav = notification.object as? NavigationController, nav === stacks[tab] else { return }
            updateBar()
            DispatchQueue.main.async { [weak self] in self?.adoptScroll() }
        }

        /// Whether the bar shows: on a tab's own page, but Search's.
        private var barShown: Bool { tab != .search && stacks[tab]?.canPop != true }

        private func updateBar() {
            let shown = barShown
            bar.isHidden = !shown
            pill?.choosesOnFocus = shown
            if !shown { band.isHidden = true }
        }

        /// Starts the top page's scrolling below the bar, and watches it for
        /// the band.
        private func adoptScroll() {
            if let scrollObservation { NotificationCenter.default.removeObserver(scrollObservation) }
            scrollObservation = nil
            guard let top = stacks[tab]?.top, top.isViewLoaded, let scroll = Self.firstScroll(in: top.view) else {
                band.isHidden = true
                return
            }
            watchedScroll = scroll
            // Home's hero reaches the top, behind the bar; other pages start below it.
            let inset = barShown && tab != .home ? Self.barHeight : 0
            if scroll.contentInsets.top != inset {
                scroll.automaticallyAdjustsContentInsets = false
                scroll.contentInsets.top = inset
                scroll.contentView.scroll(to: NSPoint(x: 0, y: -inset))
                scroll.reflectScrolledClipView(scroll.contentView)
            }
            scroll.contentView.postsBoundsChangedNotifications = true
            scrollObservation = NotificationCenter.default.addObserver(
                forName: NSView.boundsDidChangeNotification, object: scroll.contentView, queue: .main
            ) { [weak self] _ in
                MainActor.assumeIsolated { self?.updateBand() }
            }
            updateBand()
        }

        private func updateBand() {
            guard barShown, let scroll = watchedScroll else {
                band.isHidden = true
                return
            }
            band.isHidden = scroll.contentView.bounds.minY <= -scroll.contentInsets.top + 1
        }

        private static func firstScroll(in view: NSView) -> NSScrollView? {
            var queue = [view]
            while !queue.isEmpty {
                let next = queue.removeFirst()
                if let scroll = next as? NSScrollView, !scroll.isHidden { return scroll }
                queue.append(contentsOf: next.subviews)
            }
            return nil
        }

        // MARK: Focus

        /// Up from the page's first row goes to the current tab.
        private func edge(_ direction: FocusDirection) -> Bool {
            guard direction == .up, barShown, let pill, let current = pill.currentChoice,
                let engine = FocusEngine.active(for: view),
                let focused = engine.focused, !focused.isDescendant(of: bar)
            else { return false }
            engine.focus(current)
            return true
        }

        /// Back: a deeper page returns to where it was opened from; on a
        /// tab's page focus goes to its tab with the page at its top; from a
        /// tab to Home's (§4).
        func back() -> Bool {
            if let stack = stacks[tab], stack.canPop {
                stack.pop()
                return true
            }
            if tab == .search {
                show(.home)
                focusTab()
                return true
            }
            guard let engine = FocusEngine.active(for: view) else { return false }
            if let focused = engine.focused, focused.isDescendant(of: bar) {
                guard tab != .home else { return false }
                show(.home)
                focusTab()
                return true
            }
            if let scroll = watchedScroll {
                scroll.contentView.scroll(to: NSPoint(x: 0, y: -scroll.contentInsets.top))
                scroll.reflectScrolledClipView(scroll.contentView)
            }
            focusTab()
            return true
        }

        private func focusTab() {
            guard let current = pill?.currentChoice, let engine = FocusEngine.active(for: view) else { return }
            engine.focus(current)
        }
    }

    /// The menu button: three vertical dots, bare until it has focus or its
    /// menu is open, then in the tabs' glass, white with focus (§3, §5).
    final class TVMenuButton: Box, Focusable {
        private let theme: Theme
        private let icon: IconView
        var press: (@MainActor (NSView) -> Void)?

        init(theme: Theme) {
            self.theme = theme
            icon = IconView("moreVertical", size: 28, color: theme.colors.textPrimary)
            super.init(radius: 999)
            addSubview(icon)
            icon.center(in: self)
            widthAnchor.constraint(equalTo: heightAnchor).isActive = true
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(MavioStrings.navUserMenu(name: ""))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        /// Whether its menu is open.
        var open = false { didSet { restyle() } }

        func activate() { press?(self) }

        private func restyle() {
            let focused = isFocused && !open
            fill =
                focused ? .white : open ? theme.colors.surfaceRaised.withAlphaComponent(0.6) : nil
            icon.contentTintColor = focused ? .black : theme.colors.textPrimary
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }
#endif
