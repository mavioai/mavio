#if os(macOS)
    import AppKit
    import MavioMac

    /// The look of sign-in's steps, as the web UI's: their type and the
    /// grouped fields' fill.
    @MainActor
    enum SignInStyle {
        /// A step's title: 30 pt bold, or television's first title.
        static func title(_ theme: Theme) -> MavioFontStyle {
            theme.isTV ? theme.fonts.title1 : MavioFontStyle(size: 30, lineHeight: 36, weight: .bold, tracking: -0.3)
        }

        /// A step's text, 16 pt.
        static func body(_ theme: Theme) -> MavioFontStyle {
            theme.isTV ? theme.fonts.body : MavioFontStyle(size: 16, lineHeight: 22, weight: .regular)
        }

        /// A small caption, 13 pt.
        static func small(_ theme: Theme, weight: NSFont.Weight = .regular) -> MavioFontStyle {
            theme.isTV
                ? theme.fonts.footnote : MavioFontStyle(size: 13, lineHeight: 18, weight: weight)
        }

        /// The label of a step's line of text.
        static func text(_ text: String, _ theme: Theme) -> NSTextField {
            let label = NSTextField.label(text, body(theme), color: theme.colors.textSecondary, lines: 0)
            return label
        }

        /// A step's error.
        static func error(_ text: String, _ theme: Theme) -> NSTextField {
            NSTextField.label(
                text, theme.isTV ? theme.fonts.callout : MavioFontStyle(size: 15, lineHeight: 20, weight: .regular),
                color: theme.colors.danger, lines: 0)
        }

        /// A step's eyebrow, title and the line under it.
        static func heading(_ title: String, eyebrow: String? = nil, subtitle: String? = nil, theme: Theme) -> NSView {
            var views: [NSView] = []
            if let eyebrow {
                views.append(
                    NSTextField.label(eyebrow, small(theme, weight: .semibold), color: theme.colors.textTertiary))
            }
            views.append(NSTextField.label(title, Self.title(theme), color: theme.colors.textPrimary, lines: 0))
            if let subtitle {
                views.append(
                    NSTextField.label(
                        subtitle,
                        theme.isTV ? theme.fonts.callout : MavioFontStyle(size: 15, lineHeight: 20, weight: .regular),
                        color: theme.colors.textTertiary))
            }
            return NSStackView.vertical(views, spacing: 6)
        }

        /// A line between ways of going on, broken by "or".
        static func or(_ theme: Theme) -> NSView {
            let left = Box(fill: .separatorColor)
            let right = Box(fill: .separatorColor)
            left.size(height: 1)
            right.size(height: 1)
            let stack = NSStackView.horizontal(
                [left, NSTextField.label(MavioStrings.authOr, small(theme), color: theme.colors.textTertiary), right],
                spacing: MavioSpacing.s12)
            left.widthAnchor.constraint(equalTo: right.widthAnchor).isActive = true
            return stack
        }

        /// Waiting for another device or the browser.
        static func waiting(_ theme: Theme) -> NSView {
            let spinner = NSProgressIndicator()
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.startAnimation(nil)
            let row = NSStackView.horizontal(
                [spinner, NSTextField.label(MavioStrings.authWaiting, body(theme), color: theme.colors.textSecondary)],
                spacing: MavioSpacing.s8)
            return centered(row)
        }

        /// The QR code of the link page with the code filled in, for a
        /// phone to scan, and the line under it; nil when it cannot be
        /// encoded.
        static func scan(_ text: String, _ theme: Theme) -> NSView? {
            guard let modules = QRModules(encoding: text) else { return nil }
            let code = QRView(
                modules, side: theme.isTV ? 260 : 168, quiet: theme.isTV ? 18 : 12, radius: theme.isTV ? 22 : 12,
                label: text)
            let line = NSTextField.label(MavioStrings.authScanLinkCode, small(theme), color: theme.colors.textTertiary)
            line.alignment = .center
            return centered(NSStackView.vertical([code, line], spacing: 10, alignment: .centerX))
        }

        /// A view centered in the step's width.
        static func centered(_ view: NSView) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(view)
            NSLayoutConstraint.activate([
                view.centerXAnchor.constraint(equalTo: holder.centerXAnchor),
                view.topAnchor.constraint(equalTo: holder.topAnchor),
                view.bottomAnchor.constraint(equalTo: holder.bottomAnchor),
                view.leadingAnchor.constraint(greaterThanOrEqualTo: holder.leadingAnchor),
            ])
            return holder
        }
    }

    /// A button as wide as the step: the primary filled with `color.button`,
    /// the others with `color.fill`; white with focus in television mode.
    final class StepButton: Box, Focusable {
        private let theme: Theme
        private let primary: Bool
        private let label: NSTextField
        private let iconView: IconView?
        private let action: @MainActor () -> Void
        private var pressed = false

        /// A button of title and icon, primary or not.
        init(
            _ title: String, icon: String? = nil, primary: Bool = true, theme: Theme,
            action: @escaping @MainActor () -> Void
        ) {
            self.theme = theme
            self.primary = primary
            self.action = action
            label = .label(
                title,
                theme.isTV ? theme.fonts.headline : MavioFontStyle(size: 17, lineHeight: 22, weight: .semibold))
            label.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
            iconView = icon.map { IconView($0, size: theme.isTV ? 28 : 20) }
            super.init(radius: theme.isTV ? 999 : 14)
            let row = NSStackView.horizontal([iconView, label].compactMap(\.self), spacing: MavioSpacing.s8)
            addSubview(row)
            row.center(in: self)
            NSLayoutConstraint.activate([
                row.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: MavioSpacing.s20),
                heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 50),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(title)
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Whether the button can be pressed.
        var isEnabled = true { didSet { restyle() } }

        var isFocused = false { didSet { restyle() } }

        func activate() {
            guard isEnabled else { return }
            action()
        }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : primary ? theme.colors.button : theme.colors.fill
            let color: NSColor = focused ? .black : primary ? theme.colors.onButton : theme.colors.textPrimary
            label.textColor = color
            iconView?.contentTintColor = color
            alphaValue = !isEnabled ? 0.5 : pressed ? 0.8 : 1
            elevation =
                focused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
        }

        override func mouseDown(with event: NSEvent) {
            guard isEnabled else { return }
            pressed = true
            restyle()
        }

        override func mouseUp(with event: NSEvent) {
            guard pressed else { return }
            pressed = false
            restyle()
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }
    }

    /// Fields as the rows of an inset group, lines between them.
    final class FieldGroup: Box {
        /// A group of rows.
        init(_ rows: [NSView], theme: Theme) {
            super.init(radius: 12)
            layer?.masksToBounds = true
            let stack = NSStackView.vertical([], spacing: 0)
            for (i, row) in rows.enumerated() {
                if i > 0 {
                    let line = Box(fill: .separatorColor)
                    line.size(height: 1)
                    stack.addArrangedSubview(line)
                    line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                }
                stack.addArrangedSubview(row)
                row.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            addFilling(stack)
            fill = .appearance(light: NSColor(hex: 0x767680, alpha: 0.08), dark: NSColor(hex: 0x767680, alpha: 0.14))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// A field's row: its label, 104 pt wide, beside the field.
        static func row(_ label: String, field: NSTextField, theme: Theme) -> NSView {
            let title = NSTextField.label(
                label, theme.isTV ? theme.fonts.body : MavioFontStyle(size: 16, lineHeight: 22, weight: .regular),
                color: theme.colors.textPrimary)
            title.size(width: theme.isTV ? 180 : 104)
            field.font = .systemFont(ofSize: theme.isTV ? theme.fonts.body.size : 17)
            field.isBordered = false
            field.drawsBackground = false
            field.focusRingType = .none
            field.translatesAutoresizingMaskIntoConstraints = false
            let row = NSStackView.horizontal([title, field], spacing: 0)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(
                row, insets: NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: 0, right: MavioSpacing.s16))
            holder.heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 48).isActive = true
            return holder
        }
    }

    /// A code to enter on another device, a character to a box.
    final class CodeBoxes: NSView {
        /// The boxes of code.
        init(_ code: String, theme: Theme) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let scale: CGFloat = theme.isTV ? 1.5 : 1
            let views = code.map { c -> NSView in
                if c == "-" {
                    let dash = NSTextField.label("–", SignInStyle.body(theme), color: theme.colors.textTertiary)
                    dash.alignment = .center
                    dash.size(width: 12 * scale)
                    return dash
                }
                let box = Box(fill: theme.colors.fill, radius: 10)
                box.size(width: 40 * scale, height: 52 * scale)
                let letter = NSTextField.label(String(c), SignInStyle.body(theme), color: theme.colors.textPrimary)
                letter.font = .monospacedSystemFont(ofSize: 26 * scale, weight: .semibold)
                box.addSubview(letter)
                letter.center(in: box)
                return box
            }
            let row = NSStackView.horizontal(views, spacing: 10)
            addSubview(row)
            NSLayoutConstraint.activate([
                row.centerXAnchor.constraint(equalTo: centerXAnchor),
                row.topAnchor.constraint(equalTo: topAnchor),
                row.bottomAnchor.constraint(equalTo: bottomAnchor),
                row.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.staticText)
            setAccessibilityLabel(code)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// A server found on the network: its icon, name and address, which
    /// choosing connects to.
    final class NearbyRow: Box, Focusable {
        private let theme: Theme
        private let action: @MainActor () -> Void
        private let name: NSTextField
        private let address: NSTextField
        private let icon: IconView
        private var hovering = false
        private var tracking: NSTrackingArea?

        init(name: String, address: String, theme: Theme, action: @escaping @MainActor () -> Void) {
            self.theme = theme
            self.action = action
            self.name = .label(
                name, theme.isTV ? theme.fonts.headline : MavioFontStyle(size: 17, lineHeight: 22, weight: .semibold),
                color: theme.colors.textPrimary)
            self.address = .label(address, theme.fonts.footnote, color: theme.colors.textSecondary)
            icon = IconView("server", size: theme.isTV ? 32 : 22, color: theme.colors.textPrimary)
            super.init(radius: theme.isTV ? 12 : 0)
            let text = NSStackView.vertical([self.name, self.address], spacing: 2)
            let row = NSStackView.horizontal([icon, text, Spacer()], spacing: MavioSpacing.s12)
            addFilling(row, insets: .symmetric(horizontal: 14, vertical: 10))
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(name)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        func activate() { action() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : hovering ? theme.colors.fill : nil
            name.textColor = focused ? .black : theme.colors.textPrimary
            address.textColor = focused ? NSColor.black.withAlphaComponent(0.6) : theme.colors.textSecondary
            icon.contentTintColor = focused ? .black : theme.colors.textPrimary
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

    /// The card of sign-in on the wall of posters: frosted, tinted with the
    /// theme, its edge and shadow as the web UI's.
    final class SignInCard: Box {
        private let tint = Box(radius: 26)

        /// A card holding content, inset.
        init(_ content: NSView, insets: NSEdgeInsets) {
            super.init(radius: 26)
            let glass = NSVisualEffectView()
            glass.material = .popover
            glass.blendingMode = .withinWindow
            glass.state = .active
            glass.wantsLayer = true
            glass.layer?.cornerRadius = 26
            glass.layer?.cornerCurve = .continuous
            glass.layer?.masksToBounds = true
            addFilling(glass)
            addFilling(tint)
            addFilling(content, insets: insets)
            borderWidth = 1
            tint.fill = .appearance(light: NSColor(white: 1, alpha: 0.84), dark: NSColor(white: 0.11, alpha: 0.82))
            border = .appearance(light: NSColor(white: 0, alpha: 0.08), dark: NSColor(white: 0.33, alpha: 0.5))
            elevation = MavioNSShadow(
                color: .appearance(light: NSColor(white: 0, alpha: 0.14), dark: NSColor(white: 0, alpha: 0.5)),
                x: 0, y: 30, blur: 80, spread: 0)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }
#endif
