#if os(macOS)
    import AppKit
    import MavioMac

    /// Home: the hero, then the rows of every server, merged as Plex does
    /// (docs/clients.md §6.3.1), each server's drawn as it arrives in its
    /// fixed place. In television mode the hero reaches the top, behind the
    /// bar (docs/tv.md §3, §6.1).
    public final class HomePage: Page, TitleBarReaching {
        /// The hero goes behind the title bar, or, without one, the page
        /// starts below it.
        public var reachesTitleBar: Bool { true }

        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s8, right: 0))
        private var observer: Observer?
        private var hero: HeroView?
        private var heroHeight: NSLayoutConstraint?
        /// The hero's titles, which rebuild it only when they change.
        private var heroIds: [String] = []
        /// The title the hero shows, which a rebuilt hero keeps.
        private var heroShown: String?

        override public func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navHome
            observer = Observer { [weak self] in self?.update() }
            if model.home.isEmpty { model.refreshHome() }
        }

        private func update() {
            let rows = model.home
            // Home loads while a server is signed in; with none, it only
            // says which are signed out.
            guard !rows.isEmpty || model.signedIn.isEmpty else {
                show(LoadingView())
                return
            }
            if page.superview == nil { show(page) }
            let several = model.signedIn.count > 1
            var views: [NSView] = []
            let entries = HeroEntry.from(rows)
            if entries.isEmpty {
                hero = nil
                heroIds = []
            } else {
                let ids = entries.map(\.id)
                if ids != heroIds || hero == nil {
                    heroIds = ids
                    let made = HeroView(entries: entries, shown: heroShown, page: self)
                    made.onShow = { [weak self] in self?.heroShown = $0 }
                    hero = made
                    let height = made.heightAnchor.constraint(equalToConstant: heroTargetHeight)
                    height.isActive = true
                    heroHeight = height
                }
                if let hero {
                    views.append(inset(hero, NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s8, right: 0)))
                }
            }
            views += rows.map { rowView($0, several: several) }
            let signedOut = model.servers.filter { !model.signedIn.contains($0.id) }
            if !signedOut.isEmpty {
                let note = NSTextField.label(
                    signedOut.map { "\($0.name): \(MavioStrings.authSignedOut)" }.joined(separator: " · "),
                    theme.fonts.footnote, color: theme.colors.textTertiary, lines: 0)
                views.append(margined(note, top: MavioSpacing.s24))
            }
            page.setViews(views)
            if theme.isTV, let first = hero?.firstAction, let engine = FocusEngine.active(for: view),
                engine.focused == nil
            {
                engine.prefer(first)
            }
            FocusEngine.active(for: view)?.schedule()
        }

        /// The hero's height: 470 pt on the desktop, 78 % of the page's in
        /// television mode.
        private var heroTargetHeight: CGFloat {
            theme.isTV ? max(400, view.bounds.height * 0.78) : 470
        }

        override public func viewDidLayout() {
            super.viewDidLayout()
            if let heroHeight, heroHeight.constant != heroTargetHeight { heroHeight.constant = heroTargetHeight }
            reachTop()
        }

        /// Starts the page at the top of the window, behind the title bar
        /// or television mode's bar, which the shell keeps the page's
        /// scrolling below, when the hero leads it; without one, below it.
        private func reachTop() {
            guard let document = page.scroll.documentView else { return }
            let top = document.constraints.first {
                ($0.firstItem as? NSView) === page.stack && $0.firstAttribute == .top
            }
            let wanted =
                theme.isTV
                ? (hero == nil ? 0 : -page.scroll.contentInsets.top)
                : (hero == nil ? view.safeAreaInsets.top : 0)
            if let top, top.constant != wanted { top.constant = wanted }
        }

        isolated deinit { observer?.cancel() }
    }
#endif
