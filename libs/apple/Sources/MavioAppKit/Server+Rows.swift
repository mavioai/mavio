#if os(macOS)
    import AppKit
    import MavioDesign

    /// A row of a form holding any content — a tile, lines of text, a bar —
    /// with a view and a chevron at its end; choosing it does its action, as
    /// a FormRow's link. In television mode it is focusable, white with
    /// focus and its text black (docs/tv.md §4).
    public final class ContentRow: Box, Focusable {
        private let theme: Theme
        /// What choosing the row does.
        public var onPress: (@MainActor () -> Void)?
        /// The menu a secondary click shows, if any.
        public var contextMenu: (@MainActor () -> NSMenu?)?
        private var hovering = false
        private var tracking: NSTrackingArea?
        private var colors: [ObjectIdentifier: NSColor] = [:]

        /// A row of content, an accessory and a chevron at its end when
        /// chevron, doing action when chosen.
        public init(
            _ content: NSView, accessory: NSView? = nil, chevron: Bool = true, theme: Theme = .desktop,
            action: (@MainActor () -> Void)? = nil
        ) {
            self.theme = theme
            onPress = action
            super.init(radius: theme.isTV ? MavioRadius.lg : 0)
            var views: [NSView] = [content]
            // The content takes the rest of the row, a stack too, which hugs
            // its views otherwise.
            content.setContentHuggingPriority(.init(1), for: .horizontal)
            (content as? NSStackView)?.setHuggingPriority(.init(1), for: .horizontal)
            content.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            if let accessory {
                accessory.setContentHuggingPriority(.required, for: .horizontal)
                accessory.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
                views.append(accessory)
            }
            if chevron { views.append(IconView("forward", size: 16, color: theme.colors.textTertiary)) }
            let stack = NSStackView.horizontal(views, spacing: MavioSpacing.s12)
            stack.distribution = .fill
            addSubview(stack)
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            stack.pin(to: self, insets: .symmetric(horizontal: h, vertical: v))
            heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 52).isActive = true
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Whether it has television mode's focus.
        public var isFocused = false { didSet { restyle() } }

        /// Acts as a press of it.
        public func activate() { onPress?() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : (hovering && onPress != nil && !theme.isTV ? theme.colors.fill : nil)
            recolor(self, focused: focused)
            if theme.isTV {
                NSAnimationContext.runAnimationGroup { context in
                    context.duration = MavioDuration.fast
                    context.allowsImplicitAnimation = true
                    layer?.transform = focused ? CATransform3DMakeScale(1.02, 1.02, 1) : CATransform3DIdentity
                }
                if focused { FocusEngine.active(for: self)?.scrollIntoView(self) }
            }
        }

        /// Turns the row's text and icons black with focus, and back.
        private func recolor(_ view: NSView, focused: Bool) {
            for sub in view.subviews {
                if let label = sub as? NSTextField {
                    let key = ObjectIdentifier(label)
                    if focused {
                        if colors[key] == nil { colors[key] = label.textColor }
                        label.textColor = .black
                    } else if let color = colors[key] {
                        label.textColor = color
                    }
                } else if let icon = sub as? IconView {
                    let key = ObjectIdentifier(icon)
                    if focused {
                        if colors[key] == nil { colors[key] = icon.contentTintColor }
                        icon.contentTintColor = .black
                    } else if let color = colors[key] {
                        icon.contentTintColor = color
                    }
                } else {
                    recolor(sub, focused: focused)
                }
            }
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

        override public func mouseDown(with event: NSEvent) {}

        override public func mouseUp(with event: NSEvent) {
            guard onPress != nil, bounds.contains(convert(event.locationInWindow, from: nil)) else { return }
            onPress?()
        }

        override public func menu(for event: NSEvent) -> NSMenu? {
            contextMenu?() ?? super.menu(for: event)
        }

        override public func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override public func accessibilityPerformPress() -> Bool {
            guard onPress != nil else { return false }
            onPress?()
            return true
        }
    }

    /// A row of a form whose controls are its own — buttons, fields, menus —
    /// rather than the row being chosen: its content and the controls at
    /// its end. In television mode focus moves among the controls, the row
    /// itself taking none.
    public final class ControlRow: Box {
        /// A row of content and controls at its end, in theme.
        public init(_ content: NSView, controls: [NSView] = [], theme: Theme = .desktop) {
            super.init(radius: theme.isTV ? MavioRadius.lg : 0)
            // The content takes the room left, so that the controls end the
            // row at their own width; a stack's hugging is its own setting.
            (content as? NSStackView)?.setHuggingPriority(.init(1), for: .horizontal)
            // The content takes the rest of the row, a stack too, which hugs
            // its views otherwise.
            content.setContentHuggingPriority(.init(1), for: .horizontal)
            (content as? NSStackView)?.setHuggingPriority(.init(1), for: .horizontal)
            content.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            for control in controls {
                control.setContentHuggingPriority(.required, for: .horizontal)
                control.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
            }
            let stack = NSStackView.horizontal([content] + controls, spacing: MavioSpacing.s12)
            addSubview(stack)
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            stack.pin(to: self, insets: .symmetric(horizontal: h, vertical: v))
            heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 52).isActive = true
        }

        /// A row of a title and the line under it, an icon on a tile of
        /// tint at its start, and controls at its end.
        public convenience init(
            _ title: String, subtitle: String? = nil, icon: String? = nil, tint: NSColor? = nil,
            controls: [NSView] = [], theme: Theme = .desktop
        ) {
            var views: [NSView] = []
            if let icon { views.append(FormRow.tile(icon, tint: tint ?? theme.colors.accent, theme: theme)) }
            views.append(Self.text(title, subtitle: subtitle, theme: theme))
            self.init(NSStackView.horizontal(views, spacing: MavioSpacing.s12), controls: controls, theme: theme)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// A row's title and the line under it, as FormRow shows them.
        public static func text(_ title: String, subtitle: String? = nil, theme: Theme = .desktop) -> NSStackView {
            var views: [NSView] = [
                NSTextField.label(
                    title, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary,
                    lines: 0)
            ]
            if let subtitle {
                views.append(
                    NSTextField.label(subtitle, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0))
            }
            return .vertical(views, spacing: 2)
        }
    }

    /// A control focus can land on in television mode: a text field, a menu,
    /// a switch or a button within a row, which select edits, opens, flips
    /// or presses; with focus it gets a white ring of 3 pt (docs/tv.md §4).
    public final class FocusFrame: Box, Focusable {
        /// The control.
        public let control: NSView
        private let theme: Theme

        /// A frame around control, in theme.
        public init(_ control: NSView, theme: Theme = .desktop) {
            self.control = control
            self.theme = theme
            super.init(radius: MavioRadius.sm)
            addFilling(control, insets: .all(theme.isTV ? 4 : 0))
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Whether it has television mode's focus.
        public var isFocused = false {
            didSet {
                border = isFocused && theme.isTV ? .white : nil
                borderWidth = 3
                if isFocused && theme.isTV { FocusEngine.active(for: self)?.scrollIntoView(self) }
            }
        }

        /// Acts as a press of it.
        public func activate() {
            switch control {
            case let input as TextInput: edit(input.field)
            case let field as NSTextField: edit(field)
            case let popup as NSPopUpButton: popup.performClick(nil)
            case let toggle as NSSwitch:
                toggle.state = toggle.state == .on ? .off : .on
                toggle.sendAction(toggle.action, to: toggle.target)
            case let button as NSButton: button.performClick(nil)
            default: break
            }
        }

        /// Types into field: on television mode's keyboard, else in place.
        private func edit(_ field: NSTextField) {
            if theme.isTV { TVKeyboard.present(for: field) } else { window?.makeFirstResponder(field) }
        }

        override public func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A text field telling its text as it is typed and when Return is
    /// pressed, secure for a password.
    public final class TextInput: NSView, NSTextFieldDelegate {
        /// The field.
        public let field: NSTextField
        /// Called with the text as it changes.
        public var onChange: (@MainActor (String) -> Void)?
        /// Called when Return is pressed.
        public var onSubmit: (@MainActor () -> Void)?
        /// Called when editing ends, with the text.
        public var onEnd: (@MainActor (String) -> Void)?

        /// A field of text with a placeholder, secure for a password, lines
        /// tall at most when it wraps (1 for a single line).
        public init(_ text: String = "", placeholder: String? = nil, secure: Bool = false, lines: Int = 1) {
            field = secure ? NSSecureTextField() : NSTextField()
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            field.stringValue = text
            field.placeholderString = placeholder
            field.bezelStyle = .roundedBezel
            field.delegate = self
            if lines != 1 {
                field.usesSingleLineMode = false
                field.cell?.wraps = true
                field.cell?.isScrollable = false
                field.maximumNumberOfLines = lines
                field.lineBreakMode = .byWordWrapping
            }
            field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            addFilling(field)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// The field's text.
        public var text: String {
            get { field.stringValue }
            set { field.stringValue = newValue }
        }

        /// Follows the text as it is typed.
        public func controlTextDidChange(_ obj: Notification) {
            onChange?(field.stringValue)
        }

        /// Keeps what was typed once the field's editing ends.
        public func controlTextDidEndEditing(_ obj: Notification) {
            onEnd?(field.stringValue)
        }

        /// Handles a command of the field's editor, such as Return or Escape.
        public func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            guard selector == #selector(NSResponder.insertNewline(_:)), let onSubmit else { return false }
            onSubmit()
            return true
        }
    }
#endif
