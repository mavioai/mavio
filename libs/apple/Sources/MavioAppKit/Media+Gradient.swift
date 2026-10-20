#if os(macOS)
    import AppKit
    import MavioDesign

    /// A linear gradient filling the view, over artwork: its colors at their
    /// locations from start to end, in unit coordinates from the top left,
    /// following the appearance.
    public final class GradientView: NSView {
        private let gradient = CAGradientLayer()
        /// The colors.
        public var colors: [NSColor] { didSet { needsDisplay = true } }

        /// A gradient of colors at locations, from start to end, each a
        /// point in unit coordinates with y growing down.
        public init(colors: [NSColor], locations: [CGFloat]? = nil, from start: CGPoint, to end: CGPoint) {
            self.colors = colors
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            layer?.addSublayer(gradient)
            gradient.locations = locations?.map { NSNumber(value: Double($0)) }
            // Layers' y grows up.
            gradient.startPoint = CGPoint(x: start.x, y: 1 - start.y)
            gradient.endPoint = CGPoint(x: end.x, y: 1 - end.y)
            setAccessibilityElement(false)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        override public var wantsUpdateLayer: Bool { true }

        override public func updateLayer() {
            gradient.colors = colors.map { $0.cg(in: self) }
        }

        override public func layout() {
            super.layout()
            CATransaction.begin()
            CATransaction.setDisableActions(true)
            gradient.frame = bounds
            CATransaction.commit()
        }

        override public func hitTest(_ point: NSPoint) -> NSView? { nil }
    }
#endif
