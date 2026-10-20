#if os(macOS)
    import AppKit
    import MavioDesign

    /// A view laid out from its top, as the screens are.
    open class FlippedView: NSView {
        override open var isFlipped: Bool { true }
    }

    /// A layer-backed box: a fill, rounded corners, a border and a shadow,
    /// its colors following the appearance.
    open class Box: FlippedView {
        /// The fill.
        public var fill: NSColor? { didSet { needsDisplay = true } }
        /// The border's color, drawn borderWidth wide inside the edge.
        public var border: NSColor? { didSet { needsDisplay = true } }
        /// The border's width.
        public var borderWidth: CGFloat = 1 { didSet { needsDisplay = true } }
        /// The corners' radius; half the height or more rounds the ends.
        public var radius: CGFloat = 0 { didSet { needsDisplay = true } }
        /// The shadow, if any.
        public var elevation: MavioNSShadow? { didSet { needsDisplay = true } }

        /// A box of fill with corners of radius.
        public init(fill: NSColor? = nil, radius: CGFloat = 0) {
            self.fill = fill
            self.radius = radius
            super.init(frame: .zero)
            wantsLayer = true
            translatesAutoresizingMaskIntoConstraints = false
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        override open var wantsUpdateLayer: Bool { true }

        override open func updateLayer() {
            guard let layer else { return }
            layer.backgroundColor = fill?.cg(in: self)
            layer.cornerRadius = min(radius, bounds.height / 2)
            layer.cornerCurve = .continuous
            layer.borderColor = border?.cg(in: self)
            layer.borderWidth = border == nil ? 0 : borderWidth
            if let shadow = elevation {
                layer.masksToBounds = false
                layer.shadowColor = shadow.color.cg(in: self)
                layer.shadowOpacity = 1
                layer.shadowOffset = CGSize(width: shadow.x, height: -shadow.y)
                layer.shadowRadius = shadow.blur / 2
            } else {
                layer.shadowOpacity = 0
            }
        }

        override open func layout() {
            super.layout()
            needsDisplay = true
        }
    }

    /// An icon of the design system, tinted.
    public final class IconView: NSImageView {
        /// The icon's name.
        public var name: String { didSet { image = .icon(name, fill: fillIcon) } }
        /// Whether the icon is the filled one, for an active state.
        public var fillIcon: Bool { didSet { image = .icon(name, fill: fillIcon) } }

        /// The icon of name, size points square, in color.
        public init(_ name: String, fill: Bool = false, size: CGFloat = 20, color: NSColor? = nil) {
            self.name = name
            fillIcon = fill
            super.init(frame: .zero)
            image = .icon(name, fill: fill)
            imageScaling = .scaleProportionallyUpOrDown
            contentTintColor = color
            translatesAutoresizingMaskIntoConstraints = false
            setAccessibilityElement(false)
            self.size(width: size, height: size)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }

    extension NSTextField {
        /// A label of text in a style and color, lines long at most (0 for
        /// any), truncated at its end; selectable when the text is worth
        /// copying.
        public static func label(
            _ text: String, _ style: MavioFontStyle, color: NSColor? = nil, lines: Int = 1, selectable: Bool = false
        ) -> NSTextField {
            let label = lines == 1 ? NSTextField(labelWithString: text) : NSTextField(wrappingLabelWithString: text)
            label.font = style.font
            label.textColor = color ?? .labelColor
            label.maximumNumberOfLines = lines
            label.lineBreakMode = lines == 1 ? .byTruncatingTail : .byWordWrapping
            label.cell?.truncatesLastVisibleLine = true
            label.isSelectable = selectable
            label.translatesAutoresizingMaskIntoConstraints = false
            label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            return label
        }
    }

    extension NSStackView {
        /// A vertical stack of views, aligned at their leading edges unless
        /// alignment says otherwise.
        public static func vertical(
            _ views: [NSView], spacing: CGFloat = 0, alignment: NSLayoutConstraint.Attribute = .leading
        ) -> NSStackView {
            let stack = NSStackView(views: views)
            stack.orientation = .vertical
            stack.alignment = alignment
            stack.spacing = spacing
            stack.translatesAutoresizingMaskIntoConstraints = false
            stack.detachesHiddenViews = true
            return stack
        }

        /// A horizontal stack of views, centered vertically unless alignment
        /// says otherwise.
        public static func horizontal(
            _ views: [NSView], spacing: CGFloat = 0, alignment: NSLayoutConstraint.Attribute = .centerY
        ) -> NSStackView {
            let stack = NSStackView(views: views)
            stack.orientation = .horizontal
            stack.alignment = alignment
            stack.spacing = spacing
            stack.translatesAutoresizingMaskIntoConstraints = false
            stack.detachesHiddenViews = true
            return stack
        }

        /// Replaces the stack's views.
        public func setViews(_ views: [NSView]) {
            for view in arrangedSubviews {
                removeArrangedSubview(view)
                view.removeFromSuperview()
            }
            for view in views { addArrangedSubview(view) }
        }
    }

    extension NSView {
        /// Pins the view's edges to its superview's, inset.
        @discardableResult
        public func pin(to other: NSView? = nil, insets: NSEdgeInsets = NSEdgeInsetsZero) -> [NSLayoutConstraint] {
            guard let target = other ?? superview else { return [] }
            translatesAutoresizingMaskIntoConstraints = false
            let constraints = [
                leadingAnchor.constraint(equalTo: target.leadingAnchor, constant: insets.left),
                trailingAnchor.constraint(equalTo: target.trailingAnchor, constant: -insets.right),
                topAnchor.constraint(equalTo: target.topAnchor, constant: insets.top),
                bottomAnchor.constraint(equalTo: target.bottomAnchor, constant: -insets.bottom),
            ]
            NSLayoutConstraint.activate(constraints)
            return constraints
        }

        /// Adds a subview pinned to the view's edges, inset.
        public func addFilling(_ subview: NSView, insets: NSEdgeInsets = NSEdgeInsetsZero) {
            addSubview(subview)
            subview.pin(to: self, insets: insets)
        }

        /// Fixes the view's width, height or both.
        @discardableResult
        public func size(width: CGFloat? = nil, height: CGFloat? = nil) -> Self {
            translatesAutoresizingMaskIntoConstraints = false
            if let width { widthAnchor.constraint(equalToConstant: width).isActive = true }
            if let height { heightAnchor.constraint(equalToConstant: height).isActive = true }
            return self
        }

        /// Keeps the view's width to its height at ratio.
        @discardableResult
        public func aspect(_ ratio: CGFloat) -> Self {
            translatesAutoresizingMaskIntoConstraints = false
            widthAnchor.constraint(equalTo: heightAnchor, multiplier: ratio).isActive = true
            return self
        }

        /// Centers the view in another.
        public func center(in other: NSView) {
            translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([
                centerXAnchor.constraint(equalTo: other.centerXAnchor),
                centerYAnchor.constraint(equalTo: other.centerYAnchor),
            ])
        }
    }

    extension NSEdgeInsets {
        /// The same inset on every edge.
        public static func all(_ value: CGFloat) -> NSEdgeInsets {
            NSEdgeInsets(top: value, left: value, bottom: value, right: value)
        }

        /// Insets horizontally and vertically.
        public static func symmetric(horizontal: CGFloat = 0, vertical: CGFloat = 0) -> NSEdgeInsets {
            NSEdgeInsets(top: vertical, left: horizontal, bottom: vertical, right: horizontal)
        }
    }

    /// A view that takes the space it is given and no more, between others
    /// in a stack.
    public final class Spacer: NSView {
        /// A flexible space.
        public init() {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setContentHuggingPriority(.init(1), for: .horizontal)
            setContentHuggingPriority(.init(1), for: .vertical)
            setContentCompressionResistancePriority(.init(1), for: .horizontal)
            setContentCompressionResistancePriority(.init(1), for: .vertical)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }
#endif
