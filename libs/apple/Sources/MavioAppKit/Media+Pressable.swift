#if os(macOS)
    import AppKit
    import MavioDesign

    /// Any content that opens or does something when clicked or selected, as
    /// an episode's card, a row of a list or a chip: focusable in television
    /// mode, where a lifted one scales and gets the cards' `color.accent`
    /// ring and shadow, and a highlighted one turns white with black content
    /// (docs/tv.md §4). On the desktop a lifted one dims under the pointer
    /// and a highlighted one fills with `color.fill`.
    public final class Pressable: Box, Focusable {
        /// How focus and the pointer show on it.
        public enum Look: Sendable {
            /// Scaled with a ring, as a card.
            case lift
            /// Filled, as a row or a chip.
            case highlight
        }

        private let theme: Theme
        private let look: Look
        private let ring = Box()
        private var tracking: NSTrackingArea?
        private var hovering = false
        private var colors: [ObjectIdentifier: NSColor] = [:]
        private var tints: [ObjectIdentifier: NSColor?] = [:]
        /// The fill shown when neither focused nor hovered.
        public var restingFill: NSColor? { didSet { restyle() } }
        /// What a click or select does.
        public var onPress: (@MainActor () -> Void)?
        /// The menu a secondary click shows, if any.
        public var contextMenu: (@MainActor () -> NSMenu?)?
        /// Called as focus comes to it or leaves.
        public var onFocus: (@MainActor (Bool) -> Void)?

        /// Content in a box of radius, inset, showing focus and the pointer
        /// by look.
        public init(
            _ content: NSView, look: Look = .highlight, radius: CGFloat = MavioRadius.md,
            insets: NSEdgeInsets = NSEdgeInsetsZero, theme: Theme, action: (@MainActor () -> Void)? = nil
        ) {
            self.theme = theme
            self.look = look
            onPress = action
            super.init(radius: radius)
            addFilling(content, insets: insets)
            ring.border = theme.colors.accent
            ring.borderWidth = 3
            ring.radius = radius >= 999 ? 999 : radius + 4.5
            ring.isHidden = true
            addSubview(ring)
            NSLayoutConstraint.activate([
                ring.leadingAnchor.constraint(equalTo: leadingAnchor, constant: -4.5),
                ring.trailingAnchor.constraint(equalTo: trailingAnchor, constant: 4.5),
                ring.topAnchor.constraint(equalTo: topAnchor, constant: -4.5),
                ring.bottomAnchor.constraint(equalTo: bottomAnchor, constant: 4.5),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Whether television mode's focus is on it.
        public var isFocused = false {
            didSet {
                guard oldValue != isFocused else { return }
                restyle()
                onFocus?(isFocused)
            }
        }

        /// Does what a click does.
        public func activate() { onPress?() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            switch look {
            case .lift:
                ring.isHidden = !focused
                elevation =
                    focused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
                fill = restingFill
                alphaValue = hovering ? 0.85 : 1
                scale(focused ? theme.layout.focusScale : 1)
            case .highlight:
                fill = focused ? .white : hovering ? theme.colors.fill : restingFill
                elevation =
                    focused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
                recolor(focused)
            }
        }

        /// Turns the content's text and icons black while focused, and back.
        private func recolor(_ focused: Bool) {
            for v in descendants(of: self) {
                let key = ObjectIdentifier(v)
                if let label = v as? NSTextField {
                    if focused {
                        if colors[key] == nil { colors[key] = label.textColor ?? .labelColor }
                        label.textColor = .black
                    } else if let color = colors[key] {
                        label.textColor = color
                    }
                } else if let image = v as? NSImageView, image.image?.isTemplate == true {
                    if focused {
                        if tints[key] == nil { tints[key] = image.contentTintColor }
                        image.contentTintColor = .black
                    } else if let tint = tints[key] {
                        image.contentTintColor = tint
                    }
                }
            }
            if !focused {
                colors = [:]
                tints = [:]
            }
        }

        private func descendants(of view: NSView) -> [NSView] {
            view.subviews.flatMap { [$0] + descendants(of: $0) }
        }

        private func scale(_ factor: CGFloat) {
            guard let layer else { return }
            let b = layer.bounds
            var t = CATransform3DMakeTranslation(b.midX, b.midY, 0)
            t = CATransform3DScale(t, factor, factor, 1)
            t = CATransform3DTranslate(t, -b.midX, -b.midY, 0)
            CATransaction.begin()
            CATransaction.setAnimationDuration(MavioDuration.fast)
            CATransaction.setAnimationTimingFunction(MavioEasings.standard.timingFunction)
            layer.transform = t
            CATransaction.commit()
        }

        // MARK: Pointer

        override public func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override public func mouseDown(with event: NSEvent) {}

        override public func menu(for event: NSEvent) -> NSMenu? {
            contextMenu?()
        }

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

        override public func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override public func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }
#endif
