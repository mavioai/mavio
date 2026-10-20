#if os(macOS)
    import AppKit
    import MavioDesign

    /// The buttons of the web UI: the primary filled with `color.button`, the
    /// secondary with `color.fill`, round icon buttons, and those over
    /// artwork; a link in `color.accentText`, and a plain one that is its
    /// content alone. macOS's sizes outside television mode, 28 pt high with
    /// 12 at their sides; television's 64, white with focus (docs/tv.md §4).
    public final class MavioButton: NSControl, Focusable {
        /// The kinds of button.
        public enum Kind: Sendable {
            /// The screen's main action, filled with `color.button`.
            case primary
            /// Another action, filled with `color.fill`.
            case secondary
            /// An icon in a circle filled with `color.fill`.
            case circle
            /// The main action over artwork, glass until it is focused or
            /// hovered, as the web UI's Play.
            case artworkPrimary
            /// Another action over artwork: white over glass.
            case artworkSecondary
            /// An icon in a circle of glass over artwork.
            case artworkCircle
            /// A link: text in `color.accentText`.
            case link
            /// A destructive link: text in `color.danger`.
            case destructive
            /// The content alone, in the text's color.
            case plain
        }

        private let kind: Kind
        private let theme: Theme
        private let box: Box
        private var glass: NSVisualEffectView?
        private let stack: NSStackView
        private let titleLabel: NSTextField
        private let iconView: IconView?
        private var hovering = false
        private var pressed = false
        private var tracking: NSTrackingArea?
        /// What a click does.
        public var onPress: (@MainActor () -> Void)?
        /// Whether the button shows it is chosen, as a filter's.
        public var selected = false { didSet { restyle() } }

        /// A button of a kind showing title and icon, either optional;
        /// large is the 36 pt height of a page's main actions.
        public init(
            _ title: String? = nil, icon: String? = nil, kind: Kind = .secondary, large: Bool = false,
            theme: Theme = .desktop, action: (@MainActor () -> Void)? = nil
        ) {
            self.kind = kind
            self.theme = theme
            onPress = action
            box = Box()
            titleLabel = .label(title ?? "", theme.isTV ? theme.fonts.headline : theme.fonts.body)
            titleLabel.isHidden = title == nil
            let round = kind == .circle || kind == .artworkCircle
            let iconSize: CGFloat = theme.isTV ? 32 : (round ? 18 : 16)
            iconView = icon.map { IconView($0, size: iconSize) }
            stack = .horizontal([iconView, titleLabel].compactMap(\.self), spacing: MavioSpacing.s8)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            if kind == .artworkPrimary || kind == .artworkSecondary || kind == .artworkCircle {
                let glass = NSVisualEffectView()
                glass.material = .hudWindow
                glass.blendingMode = .withinWindow
                glass.state = .active
                glass.wantsLayer = true
                glass.layer?.masksToBounds = true
                addFilling(glass)
                self.glass = glass
            }
            addFilling(box)
            addSubview(stack)
            let height: CGFloat = theme.isTV ? 64 : (large ? 36 : 28)
            let padded = kind != .link && kind != .destructive && kind != .plain
            if round {
                size(width: theme.isTV ? 72 : (large ? 44 : 32), height: theme.isTV ? 72 : (large ? 44 : 32))
                stack.center(in: self)
            } else {
                let side = padded ? (theme.isTV ? MavioSpacing.s32 : (large ? MavioSpacing.s16 : 12)) : 0
                NSLayoutConstraint.activate([
                    stack.leadingAnchor.constraint(equalTo: leadingAnchor, constant: side),
                    stack.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -side),
                    stack.centerYAnchor.constraint(equalTo: centerYAnchor),
                    heightAnchor.constraint(greaterThanOrEqualToConstant: padded ? height : 20),
                    heightAnchor.constraint(greaterThanOrEqualTo: stack.heightAnchor),
                ])
            }
            titleLabel.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(title ?? icon)
            restyle()
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// The title shown.
        public var title: String {
            get { titleLabel.stringValue }
            set {
                titleLabel.stringValue = newValue
                titleLabel.isHidden = newValue.isEmpty
                setAccessibilityLabel(newValue)
            }
        }

        /// The icon shown, by name.
        public var icon: String? {
            get { iconView?.name }
            set { if let newValue { iconView?.name = newValue } }
        }

        override public var isEnabled: Bool { didSet { restyle() } }

        /// Whether television mode's focus is on the button.
        public var isFocused = false { didSet { restyle() } }

        /// Presses the button, as select does in television mode.
        public func activate() {
            guard isEnabled else { return }
            onPress?()
            if let action { sendAction(action, to: target) }
        }

        private func restyle() {
            let c = theme.colors
            let tv = theme.isTV
            var background: NSColor?
            var foreground: NSColor
            var glassShown = false
            switch kind {
            case .primary:
                (background, foreground) = (c.button, c.onButton)
            case .secondary, .circle:
                (background, foreground) = (selected ? c.fillSelected : c.fill, c.textPrimary)
            case .artworkPrimary:
                if hovering || isFocused {
                    (background, foreground) = (.white, .black)
                } else {
                    (background, foreground, glassShown) = (.white.withAlphaComponent(0.16), .white, true)
                }
            case .artworkSecondary, .artworkCircle:
                (background, foreground, glassShown) = (.white.withAlphaComponent(0.16), .white, true)
            case .link:
                (background, foreground) = (nil, c.accentText)
            case .destructive:
                (background, foreground) = (nil, c.danger)
            case .plain:
                (background, foreground) = (nil, c.textPrimary)
            }
            // Television: white with black content while focused (§4).
            if tv && isFocused {
                (background, foreground, glassShown) = (.white, .black, false)
                box.elevation = MavioNSShadow(color: c.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0)
            } else {
                box.elevation = nil
            }
            if !isEnabled { foreground = c.textDisabled }
            box.fill = background
            let round = kind == .circle || kind == .artworkCircle
            box.radius = round || tv ? 999 : (kind == .primary || kind == .secondary ? MavioRadius.md : 0)
            glass?.isHidden = !glassShown
            glass?.layer?.cornerRadius = round || tv ? 999 : MavioRadius.md
            titleLabel.textColor = foreground
            iconView?.contentTintColor = foreground
            alphaValue = pressed ? 0.8 : (isEnabled ? 1 : 0.6)
            let scale = tv && isFocused && !pressed ? theme.layout.focusScale : 1
            animateScale(scale)
        }

        private func animateScale(_ scale: CGFloat) {
            guard let layer else { return }
            wantsLayer = true
            let bounds = layer.bounds
            var transform = CATransform3DIdentity
            transform = CATransform3DTranslate(transform, bounds.midX, bounds.midY, 0)
            transform = CATransform3DScale(transform, scale, scale, 1)
            transform = CATransform3DTranslate(transform, -bounds.midX, -bounds.midY, 0)
            CATransaction.begin()
            CATransaction.setAnimationDuration(MavioDuration.fast)
            CATransaction.setAnimationTimingFunction(MavioEasings.standard.timingFunction)
            layer.transform = transform
            CATransaction.commit()
        }

        override public func layout() {
            super.layout()
            glass?.layer?.cornerRadius = min(glass?.layer?.cornerRadius ?? 0, bounds.height / 2)
        }

        // MARK: Pointer

        override public func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            guard !theme.isTV else { return }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override public func mouseEntered(with event: NSEvent) {
            hovering = true
            restyle()
        }

        override public func mouseExited(with event: NSEvent) {
            hovering = false
            restyle()
        }

        override public func mouseDown(with event: NSEvent) {
            guard isEnabled else { return }
            pressed = true
            restyle()
            // Tracks the pointer until it is released, pressing only inside.
            var inside = true
            while let next = window?.nextEvent(matching: [.leftMouseUp, .leftMouseDragged]) {
                inside = bounds.contains(convert(next.locationInWindow, from: nil))
                if pressed != inside {
                    pressed = inside
                    restyle()
                }
                if next.type == .leftMouseUp { break }
            }
            pressed = false
            restyle()
            if inside { activate() }
        }

        override public func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override public var acceptsFirstResponder: Bool { false }

        override public func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override public func resetCursorRects() {
            if kind == .link || kind == .destructive { addCursorRect(bounds, cursor: .pointingHand) }
        }
    }
#endif
