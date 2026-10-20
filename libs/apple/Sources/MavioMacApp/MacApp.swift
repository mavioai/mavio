#if os(macOS)
    import AppKit
    @_exported import MavioMac
    import MavioMacPlayer

    /// The Mac app's windows: the main window, with the sidebar and its
    /// places or, in television mode, the tabs at its top; and a window of
    /// its own for each playback.
    @MainActor
    public final class MacApp: NSObject, NSWindowDelegate, NSToolbarDelegate {
        /// The servers, the user and Home.
        public let model: AppModel
        private let discovery = Discovery()
        private var window: NSWindow?
        private var root: RootController?
        private var discoveryObserver: Observer?
        private var discoveryTask: Task<Void, Never>?
        private var titleObservation: NSKeyValueObservation?
        private weak var backItem: NSToolbarItem?
        /// The windows playing, the frontmost last.
        private var players: [NSWindow] = []
        /// Whether television mode put the window in full screen, which
        /// leaving it undoes.
        private var tvFullScreen = false

        /// The app of model.
        public init(model: AppModel) {
            self.model = model
            super.init()
            discoveryObserver = Observer { [weak self] in
                guard let self else { return }
                let key = Discovery.key(self.model)
                self.discoveryTask?.cancel()
                self.discoveryTask = Task { [discovery, model] in
                    _ = key
                    await discovery.load(model)
                }
            }
            NotificationCenter.default.addObserver(
                self, selector: #selector(navigationChanged), name: NavigationController.didChange, object: nil)
            NotificationCenter.default.addObserver(
                self, selector: #selector(defaultsChanged), name: UserDefaults.didChangeNotification, object: nil)
        }

        /// Whether the main window shows television mode.
        public var isTVMode: Bool { UserDefaults.standard.bool(forKey: DeviceSetting.tvMode) }

        // MARK: The main window

        /// Shows the main window, making it the first time.
        @objc public func showMainWindow() {
            if let window {
                window.makeKeyAndOrderFront(nil)
                return
            }
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 1280, height: 800),
                styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                backing: .buffered, defer: false)
            window.title = "Mavio"
            window.contentMinSize = NSSize(width: 1024, height: 680)
            window.isReleasedWhenClosed = false
            window.tabbingMode = .disallowed
            window.collectionBehavior.insert(.fullScreenPrimary)
            window.delegate = self
            let toolbar = NSToolbar(identifier: "Main")
            toolbar.delegate = self
            toolbar.displayMode = .iconOnly
            toolbar.allowsUserCustomization = false
            window.toolbar = toolbar
            window.toolbarStyle = .unified
            // The content shows no title or line under the toolbar: pages
            // carry their own headings, as the web UI's do.
            window.titleVisibility = .hidden
            window.titlebarSeparatorStyle = .none
            self.window = window
            let root = RootController(app: self, context: makeContext())
            self.root = root
            window.contentViewController = root
            window.setContentSize(NSSize(width: 1280, height: 800))
            if !window.setFrameUsingName("Main") { window.center() }
            window.setFrameAutosaveName("Main")
            applyMode(animated: false)
            window.makeKeyAndOrderFront(nil)
        }

        /// A context of the app's state, in the theme of the mode shown.
        func makeContext() -> MacContext {
            let context = MacContext(
                model: model, discovery: discovery, theme: isTVMode ? .tv : .desktop,
                play: { [weak self] in self?.play($0) })
            Routes.wire(context)
            context.openSettings = { [weak self] account in self?.root?.showSettings(account: account) }
            context.addServer = { [weak self] in self?.root?.addServer() }
            context.openRemote = { [weak self] server, sessionId in self?.openRemote(server, sessionId) }
            return context
        }

        /// Opens a link of the app: `mavio://item/<server>/<item>` shows an
        /// item's details on Home, `mavio://person/<server>/<person>` a person.
        public func open(_ url: URL) {
            showMainWindow()
            let parts = url.pathComponents.filter { $0 != "/" }
            guard url.scheme == "mavio", parts.count == 2 else { return }
            let (serverId, id) = (parts[0], parts[1])
            let route: Route? =
                switch url.host() {
                case "item": .item(serverId: serverId, itemId: id)
                case "person": .person(serverId: serverId, personId: id)
                default: nil
                }
            if let route { root?.openOnHome(route) }
        }

        /// Opens the settings.
        @objc public func openSettings() {
            showMainWindow()
            root?.showSettings(account: false)
        }

        /// Opens Search.
        @objc public func showSearch() {
            showMainWindow()
            root?.showSearch()
        }

        /// Returns from the page shown to the one below it.
        @objc public func goBack() {
            root?.activeNavigation?.pop()
        }

        /// Switches television mode on or off.
        @objc public func toggleTVMode() {
            UserDefaults.standard.set(!isTVMode, forKey: DeviceSetting.tvMode)
        }

        /// Enters or leaves full screen.
        @objc public func toggleFullScreen() {
            showMainWindow()
            window?.toggleFullScreen(nil)
        }

        /// Whether the main window is in full screen.
        var isFullScreen: Bool { window?.styleMask.contains(.fullScreen) == true }

        private var shownTV: Bool?

        @objc private func defaultsChanged() {
            Appearance.apply(tv: isTVMode)
            guard window != nil, shownTV != isTVMode else { return }
            applyMode(animated: true)
        }

        /// Shows the mode the setting asks for: television mode's tabs in
        /// full screen, or the sidebar's places.
        private func applyMode(animated: Bool) {
            guard let window, let root else { return }
            let tv = isTVMode
            let changed = shownTV != nil
            shownTV = tv
            Appearance.apply(tv: tv)
            window.toolbar?.isVisible = !tv
            window.titlebarAppearsTransparent = true
            if changed { root.reload(context: makeContext()) }
            if tv && !isFullScreen {
                tvFullScreen = true
                window.toggleFullScreen(nil)
            } else if !tv && tvFullScreen {
                tvFullScreen = false
                if isFullScreen { window.toggleFullScreen(nil) }
            }
            navigationChanged()
        }

        // MARK: Toolbar

        private static let back = NSToolbarItem.Identifier("back")

        /// The toolbar's items.
        public func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
            [.sidebarTrackingSeparator, Self.back]
        }

        /// The items the toolbar may hold, which are its items.
        public func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
            toolbarDefaultItemIdentifiers(toolbar)
        }

        /// The toolbar item of an identifier: back.
        public func toolbar(
            _ toolbar: NSToolbar, itemForItemIdentifier itemIdentifier: NSToolbarItem.Identifier,
            willBeInsertedIntoToolbar flag: Bool
        ) -> NSToolbarItem? {
            guard itemIdentifier == Self.back else { return nil }
            let item = NSToolbarItem(itemIdentifier: itemIdentifier)
            item.image = NSImage(
                systemSymbolName: "chevron.backward", accessibilityDescription: MavioStrings.actionBack)
            item.label = MavioStrings.actionBack
            item.toolTip = MavioStrings.actionBack
            item.isNavigational = true
            item.isBordered = true
            item.target = self
            item.action = #selector(goBack)
            item.autovalidates = false
            backItem = item
            return item
        }

        /// Shows the active stack's top page's title, and whether back can
        /// go anywhere.
        @objc func navigationChanged() {
            let nav = root?.activeNavigation
            backItem?.isEnabled = nav?.canPop == true
            backItem?.isHidden = nav?.canPop != true
            titleObservation = nil
            guard let top = nav?.top else {
                window?.title = root?.fallbackTitle ?? "Mavio"
                return
            }
            window?.title = top.title ?? root?.fallbackTitle ?? "Mavio"
            titleObservation = top.observe(\.title) { [weak self] page, _ in
                MainActor.assumeIsolated {
                    self?.window?.title = page.title ?? self?.root?.fallbackTitle ?? "Mavio"
                }
            }
        }

        // MARK: Playing

        /// Plays request in the player's window, in place of what plays
        /// there, or, as the setting asks, in a new window for each video
        /// so that several play at once (docs/clients.md §4.4). The first
        /// window of the player's keeps where it was left; the others open
        /// beside it.
        public func play(_ request: PlayRequest) {
            let each = UserDefaults.standard.bool(forKey: DeviceSetting.playerWindows)
            if !each, let player = players.last {
                player.title = request.title
                player.contentViewController = playerContent(request, in: player)
                player.makeKeyAndOrderFront(nil)
                return
            }
            let player = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 1280, height: 720),
                styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                backing: .buffered, defer: false)
            player.title = request.title
            player.titlebarAppearsTransparent = true
            player.titleVisibility = .hidden
            player.backgroundColor = .black
            player.isReleasedWhenClosed = false
            player.isMovableByWindowBackground = true
            player.tabbingMode = .disallowed
            player.collectionBehavior.insert(.fullScreenPrimary)
            player.delegate = self
            player.contentMinSize = NSSize(width: 480, height: 270)
            player.contentViewController = playerContent(request, in: player)
            player.setContentSize(NSSize(width: 1280, height: 720))
            if let previous = players.last {
                player.setFrame(previous.frame, display: false)
                player.setFrameTopLeftPoint(
                    player.cascadeTopLeft(from: NSPoint(x: previous.frame.minX, y: previous.frame.maxY)))
            } else if !player.setFrameUsingName("Player") {
                player.center()
            }
            if players.isEmpty { player.setFrameAutosaveName("Player") }
            player.makeKeyAndOrderFront(nil)
            players.append(player)
        }

        private func playerContent(_ request: PlayRequest, in player: NSWindow) -> NSViewController {
            let context = makeContext().with(theme: isTVMode ? .tv : .desktop)
            return PlayerPages.player(context: context, request: request) { [weak player] in player?.close() }
        }

        /// Shows the remote control of a session in a window of its own.
        private func openRemote(_ server: ServerRecord, _ sessionId: String) {
            let window = NSWindow(
                contentRect: NSRect(x: 0, y: 0, width: 420, height: 560),
                styleMask: [.titled, .closable, .resizable, .fullSizeContentView], backing: .buffered, defer: false)
            window.isReleasedWhenClosed = false
            window.titlebarAppearsTransparent = true
            window.contentMinSize = NSSize(width: 420, height: 520)
            window.contentViewController = Routes.remote(makeContext(), server, sessionId)
            window.center()
            window.makeKeyAndOrderFront(nil)
            remotes.append(window)
            window.delegate = self
        }

        private var remotes: [NSWindow] = []

        /// Lets a closed window go.
        public func windowWillClose(_ notification: Notification) {
            guard let closing = notification.object as? NSWindow else { return }
            if players.contains(where: { $0 === closing }) {
                players.removeAll { $0 === closing }
                // The player's controller stops its playback as it goes.
                closing.contentViewController = nil
            }
            remotes.removeAll { $0 === closing }
        }

        /// Follows a window leaving full screen.
        public func windowDidExitFullScreen(_ notification: Notification) {
            if (notification.object as? NSWindow) === window { tvFullScreen = false }
        }
    }

    extension Appearance {
        /// Gives the app the appearance this device keeps; television mode
        /// starts dark, as televisions are watched in dim rooms, unless the
        /// user chose light (docs/tv.md §1).
        @MainActor public static func apply(tv: Bool = false) {
            let raw = UserDefaults.standard.string(forKey: DeviceSetting.appearance) ?? ""
            switch Appearance(rawValue: raw) ?? .system {
            case .system: NSApp.appearance = tv ? NSAppearance(named: .darkAqua) : nil
            case .light: NSApp.appearance = NSAppearance(named: .aqua)
            case .dark: NSApp.appearance = NSAppearance(named: .darkAqua)
            }
        }
    }
#endif
