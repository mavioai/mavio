#if os(macOS)
    import AppKit
    import MavioDesign

    /// A page that scrolls vertically: its views stacked from the top, as
    /// wide as the page.
    public final class ScrollPage: NSView {
        /// The scroll view.
        public let scroll = NSScrollView()
        /// The stack of the page's views.
        public let stack: NSStackView
        private let document = FlippedView()
        /// Called as the page scrolls, with whether it left its top.
        public var onScroll: (@MainActor (Bool) -> Void)?
        private var scrolled = false

        /// A page of views, spacing apart, inset.
        public init(_ views: [NSView] = [], spacing: CGFloat = 0, insets: NSEdgeInsets = NSEdgeInsetsZero) {
            stack = .vertical(views, spacing: spacing, alignment: .leading)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            document.translatesAutoresizingMaskIntoConstraints = false
            document.addSubview(stack)
            stack.pin(to: document, insets: insets)
            scroll.documentView = document
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.automaticallyAdjustsContentInsets = false
            addFilling(scroll)
            NSLayoutConstraint.activate([
                document.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
                document.trailingAnchor.constraint(equalTo: scroll.contentView.trailingAnchor),
                document.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
            ])
            scroll.contentView.postsBoundsChangedNotifications = true
            NotificationCenter.default.addObserver(
                self, selector: #selector(boundsChanged), name: NSView.boundsDidChangeNotification,
                object: scroll.contentView)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Adds a view at the page's end, as wide as the page unless
        /// fullWidth is false.
        public func add(_ view: NSView, fullWidth: Bool = true) {
            stack.addArrangedSubview(view)
            if fullWidth {
                view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
        }

        /// Replaces the page's views, each as wide as the page.
        public func setViews(_ views: [NSView]) {
            stack.setViews([])
            for v in views { add(v) }
        }

        /// Returns the page to its top.
        public func scrollToTop() {
            scroll.contentView.setBoundsOrigin(.zero)
            scroll.reflectScrolledClipView(scroll.contentView)
        }

        @objc private func boundsChanged() {
            let now = scroll.contentView.bounds.origin.y > 1
            guard now != scrolled else { return }
            scrolled = now
            onScroll?(now)
        }
    }

    /// The empty, error and offline states of a screen: an icon, a title, a
    /// message and an action, centered.
    public final class StatusView: NSView {
        /// A state with an icon, a title, a message and an action.
        public init(
            icon: String? = nil, title: String, message: String? = nil, action: (String, @MainActor () -> Void)? = nil,
            theme: Theme = .desktop
        ) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            var views: [NSView] = []
            if let icon { views.append(IconView(icon, size: 48, color: theme.colors.textTertiary)) }
            views.append(NSTextField.label(title, theme.fonts.title3, color: theme.colors.textPrimary))
            if let message {
                let label = NSTextField.label(message, theme.fonts.body, color: theme.colors.textSecondary, lines: 0)
                label.alignment = .center
                label.preferredMaxLayoutWidth = 480
                views.append(label)
            }
            if let action {
                views.append(MavioButton(action.0, kind: .secondary, theme: theme, action: action.1))
            }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12, alignment: .centerX)
            addSubview(stack)
            stack.center(in: self)
            NSLayoutConstraint.activate([
                stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: theme.layout.margin),
                stack.topAnchor.constraint(greaterThanOrEqualTo: topAnchor, constant: theme.layout.margin),
            ])
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }

    /// A spinner in the middle of the space it is given, while a screen loads.
    public final class LoadingView: NSView {
        /// A spinner.
        public init() {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let spinner = NSProgressIndicator()
            spinner.style = .spinning
            spinner.controlSize = .regular
            spinner.startAnimation(nil)
            addSubview(spinner)
            spinner.center(in: self)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }

    /// A view controller whose view is the screen's content: one of a
    /// spinner, a state or the loaded page, swapped as it loads.
    open class ScreenController: NSViewController {
        /// The theme the screen draws with.
        public let theme: Theme
        private var content: NSView?

        /// A screen in theme.
        public init(theme: Theme) {
            self.theme = theme
            super.init(nibName: nil, bundle: nil)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        override open func loadView() {
            let root = Box(fill: theme.isTV ? theme.colors.tvBackground : theme.colors.background)
            root.frame = NSRect(x: 0, y: 0, width: 800, height: 600)
            view = root
            show(LoadingView())
        }

        /// Shows a view as the screen's content.
        public func show(_ view: NSView) {
            content?.removeFromSuperview()
            content = view
            self.view.addFilling(view)
        }

        /// Shows an error with a retry.
        public func showError(_ message: String, retry: @escaping @MainActor () -> Void) {
            show(
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: message,
                    action: (MavioStrings.actionRetry, retry), theme: theme))
        }
    }
#endif
