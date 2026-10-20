#if os(macOS)
    import AppKit
    import MavioDesign

    /// A person's initial in a circle.
    public final class AvatarView: Box {
        /// The initial of name in a circle size wide.
        public init(name: String, size: CGFloat, theme: Theme = .desktop) {
            super.init(fill: theme.colors.fill, radius: size / 2)
            self.size(width: size, height: size)
            let initial = NSTextField.label(
                name.first.map { String($0).uppercased() } ?? "",
                MavioFontStyle(size: size * 0.42, lineHeight: size * 0.5, weight: .semibold, tracking: 0),
                color: theme.colors.textPrimary)
            addSubview(initial)
            initial.center(in: self)
            setAccessibilityElement(false)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }

    /// Choices in a pill, the current one filled with `color.fillSelected`,
    /// as the television's tabs and a library's Recommended and All. In
    /// television mode the focused one is white, and focus moving onto one
    /// chooses it when choosesOnFocus (docs/tv.md §3, §6.3).
    public final class PillTabs: Box, FocusChoice, FocusRegion {
        private let theme: Theme
        private var buttons: [PillTab] = []
        /// Called with the index chosen.
        public var onChange: (@MainActor (Int) -> Void)?
        /// Whether focus landing on a tab chooses it.
        public var choosesOnFocus = false
        /// The tabs focus never chooses, as Search, which hides the tabs.
        public var chosenOnlyBySelect: Set<Int> = []
        /// The index chosen.
        public private(set) var selected: Int

        /// A pill of titles, selected chosen.
        public init(_ titles: [String], selected: Int = 0, theme: Theme = .desktop) {
            self.theme = theme
            self.selected = selected
            super.init(fill: theme.colors.surfaceRaised.withAlphaComponent(0.6), radius: 999)
            let stack = NSStackView.horizontal([], spacing: theme.isTV ? MavioSpacing.s8 : 2)
            addFilling(stack, insets: .all(theme.isTV ? 6 : 2))
            for (i, title) in titles.enumerated() {
                let tab = PillTab(title, theme: theme) { [weak self] in self?.choose(i, notify: true) }
                tab.onFocus = { [weak self] in
                    guard let self, self.choosesOnFocus, self.selected != i, !self.chosenOnlyBySelect.contains(i) else {
                        return
                    }
                    self.choose(i, notify: true)
                }
                buttons.append(tab)
                stack.addArrangedSubview(tab)
            }
            restyle()
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Chooses index, telling onChange when notify.
        public func choose(_ index: Int, notify: Bool = false) {
            guard buttons.indices.contains(index) else { return }
            selected = index
            restyle()
            if notify { onChange?(index) }
        }

        /// The tab focus enters at.
        public var currentChoice: NSView? { buttons.indices.contains(selected) ? buttons[selected] : nil }

        private func restyle() {
            for (i, tab) in buttons.enumerated() { tab.chosen = i == selected }
        }
    }

    /// A tab of a pill.
    final class PillTab: Box, Focusable {
        private let theme: Theme
        private let label: NSTextField
        private let action: @MainActor () -> Void
        var onFocus: (@MainActor () -> Void)?
        var chosen = false { didSet { restyle() } }
        private var hovering = false
        private var tracking: NSTrackingArea?

        init(_ title: String, theme: Theme, action: @escaping @MainActor () -> Void) {
            self.theme = theme
            self.action = action
            label = .label(
                title, theme.isTV ? theme.fonts.headline : theme.fonts.callout, color: theme.colors.textPrimary)
            super.init(radius: 999)
            addFilling(
                label,
                insets: theme.isTV
                    ? .symmetric(horizontal: MavioSpacing.s24, vertical: MavioSpacing.s8)
                    : .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s4))
            setAccessibilityElement(true)
            setAccessibilityRole(.radioButton)
            setAccessibilityLabel(title)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false {
            didSet {
                restyle()
                if isFocused { onFocus?() }
            }
        }

        func activate() { action() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill =
                focused ? .white : chosen ? theme.colors.fillSelected : hovering ? theme.colors.fill : nil
            label.textColor = focused ? .black : theme.colors.textPrimary
            setAccessibilityValue(chosen)
        }

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { action() }
        }

        override func mouseDown(with event: NSEvent) {}

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            guard !theme.isTV else { return }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseEntered(with event: NSEvent) {
            hovering = true
            restyle()
        }

        override func mouseExited(with event: NSEvent) {
            hovering = false
            restyle()
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override func accessibilityPerformPress() -> Bool {
            action()
            return true
        }
    }
#endif
