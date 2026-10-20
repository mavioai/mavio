#if os(macOS)
    import AppKit
    import MavioDesign

    /// A stack of pages, the top one shown: pushing slides a page in from
    /// the trailing edge, popping slides it back out. The window's toolbar
    /// shows the back button and the top page's title, told through
    /// onChange.
    public final class NavigationController: NSViewController {
        /// The pages, the root first.
        public private(set) var pages: [NSViewController] = []
        /// Called whenever the top page changes, as the notification
        /// `didChange` is posted.
        public var onChange: (@MainActor (NavigationController) -> Void)?
        /// Posted whenever a stack's top page changes, its object the stack.
        public static let didChange = Notification.Name("MavioNavigationDidChange")
        private var animating = false
        /// Whether pages start below the window's title bar, but those
        /// reaching it, set by the desktop's places; television mode's shell
        /// insets its pages itself.
        public var keepsBelowTitleBar = false

        /// A stack of root.
        public init(root: NSViewController) {
            super.init(nibName: nil, bundle: nil)
            pages = [root]
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// The page shown.
        public var top: NSViewController { pages[pages.count - 1] }
        /// Whether a page is below the top one.
        public var canPop: Bool { pages.count > 1 }

        override public func loadView() {
            let root = FlippedView()
            root.frame = NSRect(x: 0, y: 0, width: 800, height: 600)
            view = root
            install(top)
        }

        /// Opens page over the stack.
        public func push(_ page: NSViewController, animated: Bool = true) {
            let from = top
            pages.append(page)
            transition(from: from, to: page, forward: true, animated: animated)
        }

        /// Returns to the page below the top one.
        public func pop(animated: Bool = true) {
            guard canPop, !animating else { return }
            let from = pages.removeLast()
            transition(from: from, to: top, forward: false, animated: animated)
        }

        /// Returns to the root.
        public func popToRoot(animated: Bool = true) {
            guard canPop else { return }
            let from = top
            pages = [pages[0]]
            transition(from: from, to: top, forward: false, animated: animated)
        }

        private func changed() {
            onChange?(self)
            NotificationCenter.default.post(name: Self.didChange, object: self)
        }

        private func install(_ page: NSViewController) {
            addChild(page)
            let reaches = (page as? TitleBarReaching)?.reachesTitleBar ?? false
            guard keepsBelowTitleBar, !reaches else {
                view.addFilling(page.view)
                return
            }
            view.addSubview(page.view)
            page.view.translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([
                page.view.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
                page.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                page.view.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                page.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            ])
        }

        private func transition(from: NSViewController, to: NSViewController, forward: Bool, animated: Bool) {
            guard isViewLoaded else {
                changed()
                return
            }
            install(to)
            let done: @MainActor @Sendable () -> Void = { [weak self] in
                from.view.removeFromSuperview()
                from.removeFromParent()
                from.view.alphaValue = 1
                to.view.alphaValue = 1
                from.view.layer?.transform = CATransform3DIdentity
                to.view.layer?.transform = CATransform3DIdentity
                self?.animating = false
                self?.view.window?.makeFirstResponder(to.view)
            }
            changed()
            guard animated, !NSWorkspace.shared.accessibilityDisplayShouldReduceMotion else {
                done()
                return
            }
            animating = true
            view.layoutSubtreeIfNeeded()
            let shift = view.bounds.width * 0.25
            for page in [from, to] { page.view.wantsLayer = true }
            to.view.alphaValue = 0
            to.view.layer?.transform = CATransform3DMakeTranslation(forward ? shift : -shift, 0, 0)
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.normal
                context.timingFunction = MavioEasings.standard.timingFunction
                context.allowsImplicitAnimation = true
                to.view.animator().alphaValue = 1
                to.view.layer?.transform = CATransform3DIdentity
                from.view.animator().alphaValue = 0
                from.view.layer?.transform = CATransform3DMakeTranslation(forward ? -shift : shift, 0, 0)
            } completionHandler: {
                MainActor.assumeIsolated { done() }
            }
        }
    }

    /// A page told when it shows again after its place was hidden, which
    /// leaves it in the window rather than making it appear anew.
    @MainActor
    public protocol Reappearing {
        /// Called as the page shows again.
        func reappeared()
    }

    /// A page whose top goes behind the window's title bar, as one led by
    /// a hero or a backdrop.
    @MainActor
    public protocol TitleBarReaching {
        /// Whether the page's top goes behind the title bar.
        var reachesTitleBar: Bool { get }
    }

    extension NSViewController {
        /// The stack the page is in.
        public var navigation: NavigationController? {
            var parent = self.parent
            while let p = parent {
                if let nav = p as? NavigationController { return nav }
                parent = p.parent
            }
            return nil
        }
    }
#endif
