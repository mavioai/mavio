#if os(macOS)
    import AppKit
    import MavioMac

    /// A place of the sidebar (docs/clients.md §6.3, Desktop).
    enum Place: Hashable {
        case search, home, discover, downloads
        case library(LibraryEntry)
    }

    /// The main window's content: signing in until a server is added, then
    /// the sidebar's places, the settings, or television mode's tabs; the
    /// user's menu over them.
    @MainActor
    final class RootController: NSViewController {
        unowned let app: MacApp
        private(set) var context: MacContext
        private var observer: Observer?
        private var content: NSViewController?
        private var browse: BrowseController?
        private var tv: TVShellController?
        private var signedOut: Bool?
        private var userMenu: UserMenuOverlay?
        /// Whether the settings show in place of the places.
        private var settingsShown = false

        init(app: MacApp, context: MacContext) {
            self.app = app
            self.context = context
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let root = Box(fill: context.theme.colors.background)
            root.frame = NSRect(x: 0, y: 0, width: 1280, height: 800)
            view = root
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            observer = Observer { [weak self] in
                guard let self else { return }
                let out: Bool =
                    if case .signedOut = self.context.model.signInState { true } else { false }
                guard out != self.signedOut else { return }
                self.signedOut = out
                self.settingsShown = false
                self.update()
            }
        }

        /// Rebuilds the content in a new context, as television mode
        /// switches the theme.
        func reload(context: MacContext) {
            self.context = context
            (view as? Box)?.fill = context.theme.colors.background
            closeUserMenu()
            browse = nil
            tv = nil
            settingsShown = false
            update()
        }

        private func update() {
            let next: NSViewController
            if signedOut == true {
                next = Routes.signIn(context, adding: false) {}
                FocusEngine.of(view.window ?? NSApp.mainWindow ?? NSWindow()).stop()
            } else if settingsShown {
                next = Routes.settings(context, account: settingsAccount) { [weak self] in
                    self?.settingsShown = false
                    self?.update()
                }
            } else if context.theme.isTV {
                let shell = tv ?? TVShellController(root: self)
                tv = shell
                next = shell
            } else {
                let split = browse ?? BrowseController(root: self)
                browse = split
                next = split
            }
            show(next)
            updateToolbar()
            updateFocus()
        }

        /// Sign-in and television mode show no toolbar.
        private func updateToolbar() {
            view.window?.toolbar?.isVisible = signedOut != true && !context.theme.isTV
        }

        private var settingsAccount = false

        private func show(_ next: NSViewController) {
            guard next !== content else { return }
            if let content {
                content.view.removeFromSuperview()
                content.removeFromParent()
            }
            addChild(next)
            view.addSubview(next.view, positioned: .below, relativeTo: userMenu)
            next.view.pin(to: view)
            content = next
            app.navigationChanged()
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            updateToolbar()
            updateFocus()
        }

        /// Starts television mode's focus in the window, or stops it.
        private func updateFocus() {
            guard let window = view.window else { return }
            let engine = FocusEngine.of(window)
            if context.theme.isTV {
                engine.onBack = { [weak self] in self?.back() ?? false }
                engine.start()
                engine.schedule()
            } else {
                engine.stop()
            }
        }

        /// Back in television mode: the menu closes, the settings return,
        /// else the shell goes up a level (docs/tv.md §4).
        private func back() -> Bool {
            if userMenu != nil {
                closeUserMenu()
                return true
            }
            if settingsShown, content?.children.isEmpty == false, let nav = activeNavigation, nav.canPop {
                nav.pop()
                return true
            }
            if settingsShown {
                settingsShown = false
                update()
                return true
            }
            return tv?.back() ?? false
        }

        /// The stack shown, whose top page the toolbar's back and title
        /// follow.
        var activeNavigation: NavigationController? {
            guard let content else { return nil }
            return Self.visibleNavigation(in: content)
        }

        private static func visibleNavigation(in controller: NSViewController) -> NavigationController? {
            var found = controller as? NavigationController
            for child in controller.children where child.isViewLoaded && child.view.window != nil {
                if child.view.isHiddenOrHasHiddenAncestor { continue }
                if let deeper = visibleNavigation(in: child) { found = deeper }
            }
            return found
        }

        /// The window's title where no page names it.
        var fallbackTitle: String {
            settingsShown ? MavioStrings.navSettings : "Mavio"
        }

        // MARK: Destinations

        /// Shows the settings, on the account's page when account.
        func showSettings(account: Bool) {
            closeUserMenu()
            guard signedOut == false else { return }
            settingsAccount = account
            settingsShown = true
            update()
        }

        /// Shows Search: the sidebar's field on the desktop, the tab in
        /// television mode.
        func showSearch() {
            closeUserMenu()
            guard signedOut == false else { return }
            settingsShown = false
            update()
            browse?.focusSearch()
            tv?.select(.search)
        }

        /// Opens route on Home.
        func openOnHome(_ route: Route) {
            guard signedOut == false else { return }
            settingsShown = false
            update()
            browse?.open(route, on: .home)
            tv?.open(route)
        }

        /// Shows the sheet adding a server.
        func addServer() {
            let sheet = NSViewController()
            let page = Routes.signIn(context, adding: true) { [weak self, weak sheet] in
                guard let sheet else { return }
                self?.dismiss(sheet)
            }
            sheet.view = NSView(frame: NSRect(x: 0, y: 0, width: 480, height: 600))
            sheet.addChild(page)
            sheet.view.addFilling(page.view)
            presentAsSheet(sheet)
        }

        // MARK: The user's menu

        /// Opens the user's menu from button, or closes it if it is open.
        func toggleUserMenu(from button: NSView) {
            if userMenu != nil {
                closeUserMenu()
                return
            }
            let overlay = UserMenuOverlay(root: self, anchor: button)
            view.addFilling(overlay)
            userMenu = overlay
            overlay.appear()
        }

        /// Closes the user's menu.
        func closeUserMenu() {
            guard let menu = userMenu else { return }
            self.userMenu = nil
            menu.removeFromSuperview()
            FocusEngine.active(for: view)?.schedule()
        }
    }
#endif
