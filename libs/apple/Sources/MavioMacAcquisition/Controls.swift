#if os(macOS)
    import AppKit
    import MavioMac

    /// Shows television mode's focus on a control that is not a row or a
    /// button: a white ring 3 pt wide around it (docs/tv.md §4).
    @MainActor
    func showFocusRing(_ view: NSView, _ focused: Bool) {
        view.wantsLayer = true
        view.layer?.borderWidth = focused ? 3 : 0
        view.layer?.borderColor = NSColor.white.cgColor
        view.layer?.cornerRadius = 6
    }

    /// A field of text, or of a password, that tells its text as it is
    /// typed and when Return is pressed; focusable in television mode,
    /// where select puts the caret in it.
    final class TextInput: NSView, NSTextFieldDelegate, Focusable {
        /// The field.
        let field: NSTextField
        /// Called with the text as it is typed.
        var onChange: (@MainActor (String) -> Void)?
        /// Called when Return is pressed.
        var onSubmit: (@MainActor () -> Void)?

        /// A field of text, secure for a password, in a monospaced font
        /// for a pattern or a path.
        init(
            _ text: String, placeholder: String? = nil, secure: Bool = false, monospaced: Bool = false,
            width: CGFloat? = nil, onChange: (@MainActor (String) -> Void)? = nil
        ) {
            field = secure ? NSSecureTextField() : NSTextField()
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            field.stringValue = text
            field.placeholderString = placeholder
            field.bezelStyle = .roundedBezel
            field.lineBreakMode = .byTruncatingTail
            field.cell?.isScrollable = true
            field.cell?.sendsActionOnEndEditing = false
            if monospaced { field.font = .monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular) }
            field.delegate = self
            field.target = self
            field.action = #selector(submitted)
            field.setAccessibilityLabel(placeholder)
            addFilling(field)
            if let width { size(width: width) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The field's text.
        var text: String {
            get { field.stringValue }
            set { field.stringValue = newValue }
        }

        func controlTextDidChange(_ obj: Notification) { onChange?(field.stringValue) }

        @objc private func submitted() { onSubmit?() }

        var isFocused = false { didSet { showFocusRing(field, isFocused) } }

        func activate() { window?.makeFirstResponder(field) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A number in a field, kept within its range, with a stepper beside it
    /// when stepped; without one, a number out of its range shows red.
    final class NumberField: NSView, NSTextFieldDelegate, Focusable {
        private let field = NSTextField()
        private let stepper: NSStepper?
        private let range: ClosedRange<Double>
        private let formatter = NumberFormatter()
        private let theme: Theme
        private let onChange: @MainActor (Double) -> Void

        /// A field of value within range, step apart when stepped, labelled
        /// for accessibility; decimals when it may have a fraction.
        init(
            _ label: String, value: Double, range: ClosedRange<Double> = 0...Double(Int32.max), step: Double = 1,
            stepped: Bool = true, decimals: Int = 0, width: CGFloat = 80, theme: Theme,
            onChange: @escaping @MainActor (Double) -> Void
        ) {
            self.range = range
            self.theme = theme
            self.onChange = onChange
            stepper = stepped ? NSStepper() : nil
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            formatter.numberStyle = .decimal
            formatter.usesGroupingSeparator = false
            formatter.minimumFractionDigits = 0
            formatter.maximumFractionDigits = decimals > 0 ? decimals : (step < 1 ? 2 : 0)
            field.bezelStyle = .roundedBezel
            field.alignment = .right
            field.delegate = self
            field.stringValue = formatter.string(from: value as NSNumber) ?? ""
            field.setAccessibilityLabel(label)
            field.size(width: width)
            var views: [NSView] = [field]
            if let stepper {
                stepper.minValue = range.lowerBound
                stepper.maxValue = range.upperBound
                stepper.increment = step
                stepper.valueWraps = false
                stepper.doubleValue = value
                stepper.target = self
                stepper.action = #selector(stepperChanged)
                stepper.setAccessibilityLabel(label)
                views.append(stepper)
            }
            addFilling(NSStackView.horizontal(views, spacing: 2))
            recolor(value)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func recolor(_ value: Double?) {
            let out = value.map { !range.contains($0) } ?? true
            field.textColor = stepper == nil && out ? theme.colors.danger : theme.colors.textPrimary
        }

        func controlTextDidChange(_ obj: Notification) {
            let typed = formatter.number(from: field.stringValue)?.doubleValue ?? Double(field.stringValue)
            recolor(typed)
            guard var value = typed else { return }
            if stepper != nil { value = min(max(value, range.lowerBound), range.upperBound) }
            stepper?.doubleValue = value
            onChange(value)
        }

        func controlTextDidEndEditing(_ obj: Notification) {
            guard let stepper else { return }
            field.stringValue = formatter.string(from: stepper.doubleValue as NSNumber) ?? ""
        }

        @objc private func stepperChanged() {
            guard let stepper else { return }
            field.stringValue = formatter.string(from: stepper.doubleValue as NSNumber) ?? ""
            onChange(stepper.doubleValue)
        }

        var isFocused = false { didSet { showFocusRing(field, isFocused) } }

        func activate() { window?.makeFirstResponder(field) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A box of several lines of text, as a prompt or a list of terms, in a
    /// monospaced font, telling its text as it is typed.
    final class TextBlock: Box, NSTextViewDelegate, Focusable {
        private let textView: NSTextView
        private let onChange: @MainActor (String) -> Void

        /// A box of text lines high, labelled for accessibility.
        init(
            _ text: String, label: String, lines: Int = 4, theme: Theme,
            onChange: @escaping @MainActor (String) -> Void
        ) {
            self.onChange = onChange
            let scroll = NSTextView.scrollableTextView()
            textView = scroll.documentView as! NSTextView
            super.init(fill: theme.colors.background, radius: 6)
            border = theme.colors.border
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            textView.drawsBackground = false
            textView.font = .monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular)
            textView.textColor = theme.colors.textPrimary
            textView.isRichText = false
            textView.isAutomaticQuoteSubstitutionEnabled = false
            textView.isAutomaticDashSubstitutionEnabled = false
            textView.isAutomaticTextReplacementEnabled = false
            textView.isAutomaticSpellingCorrectionEnabled = false
            textView.isContinuousSpellCheckingEnabled = false
            textView.allowsUndo = true
            textView.string = text
            textView.delegate = self
            textView.setAccessibilityLabel(label)
            addFilling(scroll, insets: .all(4))
            size(height: CGFloat(lines) * 18 + 8)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        func textDidChange(_ notification: Notification) { onChange(textView.string) }

        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        func activate() { window?.makeFirstResponder(textView) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A menu of options that calls back with the index chosen, focusable in
    /// television mode, where select opens it.
    final class PopUp: NSPopUpButton, Focusable {
        private let onChange: @MainActor (Int) -> Void

        /// A menu of options, the one at selected chosen, labelled for
        /// accessibility.
        init(
            _ options: [String], selected: Int, label: String? = nil, onChange: @escaping @MainActor (Int) -> Void
        ) {
            self.onChange = onChange
            super.init(frame: .zero, pullsDown: false)
            translatesAutoresizingMaskIntoConstraints = false
            // Titles alike would merge; add them item by item.
            for title in options { menu?.addItem(withTitle: title, action: nil, keyEquivalent: "") }
            selectItem(at: selected)
            target = self
            action = #selector(changed)
            setAccessibilityLabel(label)
            toolTip = label
            setContentHuggingPriority(.required, for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(indexOfSelectedItem) }

        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        func activate() { performClick(nil) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A checkbox that calls back with its state, focusable in television
    /// mode, where select checks it.
    final class Check: NSButton, Focusable {
        private let onChange: @MainActor (Bool) -> Void

        /// A checkbox of title, checked when isOn.
        init(_ title: String, isOn: Bool, theme: Theme, onChange: @escaping @MainActor (Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setButtonType(.switch)
            self.title = title
            font = theme.fonts.callout.font
            state = isOn ? .on : .off
            target = self
            action = #selector(changed)
            setContentHuggingPriority(.required, for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(state == .on) }

        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        func activate() {
            state = state == .on ? .off : .on
            changed()
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A switch that calls back with its state, focusable in television
    /// mode, where select turns it.
    final class Switch: NSSwitch, Focusable {
        private let onChange: @MainActor (Bool) -> Void

        /// A switch in state isOn, labelled for accessibility.
        init(isOn: Bool, label: String, onChange: @escaping @MainActor (Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            state = isOn ? .on : .off
            target = self
            action = #selector(changed)
            setAccessibilityLabel(label)
            setContentHuggingPriority(.required, for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(state == .on) }

        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        func activate() {
            state = state == .on ? .off : .on
            changed()
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A button that is an icon alone, as moving a row up or removing it:
    /// its label a tooltip, dimmed while disabled, white with focus in
    /// television mode.
    final class IconAction: Box, Focusable {
        private let theme: Theme
        private let iconView: IconView
        private var hovering = false
        private var tracking: NSTrackingArea?
        /// What a click does; nil leaves the button disabled.
        var onPress: (@MainActor () -> Void)? { didSet { restyle() } }
        /// The icon's color.
        var color: NSColor { didSet { restyle() } }
        /// How far the icon turns, in degrees, as "back" turned up.
        var rotation: CGFloat = 0 { didSet { needsLayout = true } }

        /// A button of icon, labelled label, in color.
        init(
            _ icon: String, label: String, size: CGFloat = 16, color: NSColor? = nil, rotation: CGFloat = 0,
            theme: Theme, action: (@MainActor () -> Void)?
        ) {
            self.theme = theme
            self.color = color ?? theme.colors.textSecondary
            self.rotation = rotation
            onPress = action
            iconView = IconView(icon, size: theme.isTV ? size * 1.5 : size)
            super.init(radius: 999)
            let side: CGFloat = theme.isTV ? 56 : 28
            self.size(width: side, height: side)
            addSubview(iconView)
            iconView.center(in: self)
            toolTip = label
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(label)
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The icon, by name.
        var icon: String {
            get { iconView.name }
            set { iconView.name = newValue }
        }

        /// The label, told as a tooltip and to accessibility.
        var label: String {
            get { toolTip ?? "" }
            set {
                toolTip = newValue
                setAccessibilityLabel(newValue)
            }
        }

        var isFocused = false { didSet { restyle() } }

        func activate() { onPress?() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : (hovering && onPress != nil ? theme.colors.fill : nil)
            iconView.contentTintColor = focused ? .black : color
            alphaValue = onPress == nil ? 0.3 : 1
        }

        override func layout() {
            super.layout()
            iconView.frameCenterRotation = rotation
        }

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

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            guard bounds.contains(convert(event.locationInWindow, from: nil)) else { return }
            onPress?()
        }

        override func accessibilityPerformPress() -> Bool {
            guard let onPress else { return false }
            onPress()
            return true
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A badge of a state or an event: red when it failed.
    final class Badge: Box {
        /// A badge of text, red when failed.
        init(_ text: String, failed: Bool, theme: Theme) {
            super.init(fill: failed ? theme.colors.danger : theme.colors.fill, radius: 999)
            let label = NSTextField.label(
                text, theme.fonts.caption, color: failed ? .white : theme.colors.textPrimary)
            label.font = NSFont.systemFont(ofSize: theme.fonts.caption.size, weight: .semibold)
            addFilling(label, insets: .symmetric(horizontal: MavioSpacing.s8, vertical: 2))
            setContentHuggingPriority(.required, for: .horizontal)
            setContentCompressionResistancePriority(.required, for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }
#endif
