#if os(macOS)
    import AppKit
    import MavioAppKit

    /// The width of the sidebars, as the web UI's: the layout's unless
    /// the user drags an edge, within 200 to 320 points; remembered, and
    /// shared by the window's sidebar and the settings'.
    @MainActor public enum SidebarWidth {
        /// The narrowest a sidebar is dragged.
        public static let minimum: CGFloat = 200
        /// The widest a sidebar is dragged.
        public static let maximum: CGFloat = 320
        private static let key = "sidebarWidth"

        /// The width chosen, nil for the layout's.
        public static var chosen: CGFloat? {
            get {
                let w = UserDefaults.standard.double(forKey: key)
                return w > 0 ? min(maximum, max(minimum, w)) : nil
            }
            set {
                if let newValue {
                    UserDefaults.standard.set(Double(min(maximum, max(minimum, newValue.rounded()))), forKey: key)
                } else {
                    UserDefaults.standard.removeObject(forKey: key)
                }
                NotificationCenter.default.post(name: changed, object: nil)
            }
        }

        /// Posted as the width chosen changes.
        public static let changed = Notification.Name("MavioSidebarWidthChanged")

        /// The width a sidebar takes in theme.
        public static func current(_ theme: Theme) -> CGFloat {
            chosen ?? min(maximum, max(minimum, theme.layout.sidebar))
        }
    }

    /// A split view controller whose first item is a sidebar as wide as
    /// SidebarWidth says, as the web UI's: its edge dragged within its
    /// limits, double-clicked to return to the layout's width, and drawn in
    /// the accent's color while the pointer is on it or drags it.
    @MainActor open class SidebarSplitController: NSSplitViewController {
        private let theme: Theme
        private var observer: NSObjectProtocol?
        private var applying = false

        /// A split view controller in theme.
        public init(theme: Theme) {
            self.theme = theme
            super.init(nibName: nil, bundle: nil)
            splitView = EdgeSplitView(theme: theme)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        isolated deinit {
            if let observer { NotificationCenter.default.removeObserver(observer) }
        }

        /// Adds the sidebar of controller, as wide as SidebarWidth says.
        @discardableResult
        public func addSidebar(_ controller: NSViewController) -> NSSplitViewItem {
            let side = NSSplitViewItem(sidebarWithViewController: controller)
            side.canCollapse = false
            // The sidebar keeps its width as the window resizes.
            side.holdingPriority = .init(260)
            addSplitViewItem(side)
            (splitView as? EdgeSplitView)?.onReset = { SidebarWidth.chosen = nil }
            observer = NotificationCenter.default.addObserver(
                forName: SidebarWidth.changed, object: nil, queue: .main
            ) { [weak self] _ in MainActor.assumeIsolated { self?.applyWidth() } }
            applyWidth()
            return side
        }

        override open func viewWillAppear() {
            super.viewWillAppear()
            applyWidth()
        }

        private func applyWidth() {
            guard let side = splitViewItems.first else { return }
            let width = SidebarWidth.current(theme)
            side.minimumThickness = SidebarWidth.minimum
            side.maximumThickness = SidebarWidth.maximum
            guard abs(side.viewController.view.frame.width - width) >= 0.5 || !isViewLoaded else { return }
            applying = true
            splitView.setPosition(width, ofDividerAt: 0)
            applying = false
        }

        override open func splitViewDidResizeSubviews(_ notification: Notification) {
            super.splitViewDidResizeSubviews(notification)
            // Only the user's dragging chooses the width.
            guard !applying, (splitView as? EdgeSplitView)?.dragging == true,
                let side = splitViewItems.first
            else { return }
            let width = side.viewController.view.frame.width.rounded()
            if width != SidebarWidth.chosen { SidebarWidth.chosen = width }
        }

        override open func splitView(
            _ splitView: NSSplitView, effectiveRect proposedEffectiveRect: NSRect, forDrawnRect drawnRect: NSRect,
            ofDividerAt dividerIndex: Int
        ) -> NSRect {
            // A handle 8 points wide over the edge, as the web UI's.
            return drawnRect.insetBy(dx: -(8 - drawnRect.width) / 2, dy: 0)
        }
    }

    /// A split view whose first divider shows in the accent's color, two
    /// points wide, while the pointer is on it or drags it, and returns the
    /// sidebar to its width when double-clicked.
    final class EdgeSplitView: NSSplitView {
        private let theme: Theme
        private let line = Box()
        private var tracking: NSTrackingArea?
        private var hovering = false
        /// Whether the user drags the divider.
        private(set) var dragging = false
        /// Called as the divider is double-clicked.
        var onReset: (@MainActor () -> Void)?

        init(theme: Theme) {
            self.theme = theme
            super.init(frame: .zero)
            isVertical = true
            dividerStyle = .thin
            // The line over the divider is not a pane.
            arrangesAllSubviews = false
            line.fill = theme.colors.accent
            line.isHidden = true
            line.translatesAutoresizingMaskIntoConstraints = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The divider's handle, 8 points wide.
        private var handle: NSRect? {
            guard arrangedSubviews.count > 1 else { return nil }
            let x = arrangedSubviews[0].frame.maxX + dividerThickness / 2
            return NSRect(x: x - 4, y: 0, width: 8, height: bounds.height)
        }

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            let area = NSTrackingArea(
                rect: .zero, options: [.mouseMoved, .mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect],
                owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseMoved(with event: NSEvent) {
            super.mouseMoved(with: event)
            hover(event)
        }

        override func mouseEntered(with event: NSEvent) {
            super.mouseEntered(with: event)
            hover(event)
        }

        override func mouseExited(with event: NSEvent) {
            super.mouseExited(with: event)
            hovering = false
            update()
        }

        private func hover(_ event: NSEvent) {
            let point = convert(event.locationInWindow, from: nil)
            let on = handle?.contains(point) == true
            guard on != hovering else { return }
            hovering = on
            update()
        }

        override func mouseDown(with event: NSEvent) {
            let point = convert(event.locationInWindow, from: nil)
            guard handle?.contains(point) == true else { return super.mouseDown(with: event) }
            if event.clickCount == 2 {
                onReset?()
                return
            }
            dragging = true
            update()
            // Tracks the drag until the button is released.
            super.mouseDown(with: event)
            dragging = false
            hovering = handle?.contains(convert(window?.mouseLocationOutsideOfEventStream ?? .zero, from: nil)) == true
            update()
        }

        override func layout() {
            super.layout()
            update()
        }

        private func update() {
            let shown = hovering || dragging
            if line.superview !== self { addSubview(line, positioned: .above, relativeTo: nil) }
            line.isHidden = !shown
            guard shown, arrangedSubviews.count > 1 else { return }
            let x = arrangedSubviews[0].frame.maxX + dividerThickness / 2
            line.frame = NSRect(x: x - 1, y: 0, width: 2, height: bounds.height)
        }
    }
#endif
