#if os(macOS)
    import AppKit
    import MavioMac

    /// Shows television mode's focus on a control: a white ring of 3 pt.
    @MainActor
    func showFocusRing(_ view: NSView, _ focused: Bool) {
        view.wantsLayer = true
        view.layer?.borderWidth = focused ? 3 : 0
        view.layer?.borderColor = NSColor.white.cgColor
        view.layer?.cornerRadius = 6
    }

    /// A menu of options that calls back with the index chosen; focusable
    /// in television mode, where select opens it.
    @MainActor
    final class PopUp: NSPopUpButton, Focusable {
        private let onChange: @MainActor (Int) -> Void

        /// A menu of options, the one at selected chosen, labelled for
        /// accessibility.
        init(_ options: [String], selected: Int, label: String? = nil, onChange: @escaping @MainActor (Int) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero, pullsDown: false)
            translatesAutoresizingMaskIntoConstraints = false
            // Titles alike would merge; add them item by item.
            for title in options { menu?.addItem(withTitle: title, action: nil, keyEquivalent: "") }
            selectItem(at: selected)
            target = self
            action = #selector(changed)
            setAccessibilityLabel(label)
            setContentHuggingPriority(.required, for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(indexOfSelectedItem) }

        /// Whether television mode's focus is on it.
        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        /// Opens the menu.
        func activate() { performClick(nil) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A switch of a label that calls back with its state; focusable in
    /// television mode, where select turns it.
    @MainActor
    final class LabeledSwitch: NSView, Focusable {
        private let control = NSSwitch()
        private let onChange: @MainActor (Bool) -> Void

        /// A switch of title in state isOn.
        init(_ title: String, isOn: Bool, theme: Theme, onChange: @escaping @MainActor (Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            control.translatesAutoresizingMaskIntoConstraints = false
            control.state = isOn ? .on : .off
            control.target = self
            control.action = #selector(changed)
            control.setAccessibilityLabel(title)
            let text = NSTextField.label(title, theme.fonts.callout, color: theme.colors.textPrimary)
            addFilling(NSStackView.horizontal([control, text], spacing: MavioSpacing.s8), insets: .all(2))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(control.state == .on) }

        /// Whether television mode's focus is on it.
        var isFocused = false { didSet { showFocusRing(self, isFocused) } }

        /// Turns it.
        func activate() {
            control.state = control.state == .on ? .off : .on
            changed()
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A field of text that tells its text as it is typed; focusable in
    /// television mode, where select opens the keyboard over the page.
    @MainActor
    final class InputField: NSView, NSTextFieldDelegate, Focusable {
        /// The field.
        let field: NSTextField
        /// Called with the text as it is typed.
        var onChange: (@MainActor (String) -> Void)?
        private let theme: Theme

        /// A field of text, its placeholder naming it, as many lines tall
        /// as lines, of digits alone for a number.
        init(_ text: String, placeholder: String, lines: Int = 1, number: Bool = false, theme: Theme) {
            self.theme = theme
            field = NSTextField()
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            field.stringValue = text
            field.placeholderString = placeholder
            field.setAccessibilityLabel(placeholder)
            field.bezelStyle = .roundedBezel
            field.font = theme.fonts.body.font
            field.delegate = self
            if lines > 1 {
                field.usesSingleLineMode = false
                field.cell?.wraps = true
                field.cell?.isScrollable = false
                field.lineBreakMode = .byWordWrapping
                field.heightAnchor.constraint(equalToConstant: CGFloat(lines) * 20 + 8).isActive = true
            }
            if number, theme.isTV {
                // The keyboard of a number is a keypad.
                let formatter = NumberFormatter()
                formatter.maximumFractionDigits = 0
                formatter.allowsFloats = false
                field.formatter = formatter
            }
            addFilling(field)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The field's text.
        var text: String {
            get { field.stringValue }
            set { field.stringValue = newValue }
        }

        /// Tells onChange the text typed.
        func controlTextDidChange(_ obj: Notification) { onChange?(field.stringValue) }

        /// Whether television mode's focus is on it.
        var isFocused = false { didSet { showFocusRing(field, isFocused) } }

        /// Opens the keyboard over the page in television mode, else puts
        /// the caret in the field.
        func activate() {
            if theme.isTV {
                TVKeyboard.present(for: field)
            } else {
                window?.makeFirstResponder(field)
            }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// A whole number in a field, at most max; empty for 0, any.
    @MainActor
    final class WholeNumberField: NSView {
        private let input: InputField
        private let max: Int
        /// Called with the number as it is typed.
        var onChange: (@MainActor (Int) -> Void)?

        /// A field of value, its placeholder label.
        init(_ label: String, value: Int, max: Int, theme: Theme) {
            self.max = max
            input = InputField(value > 0 ? String(value) : "", placeholder: label, number: true, theme: theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(input)
            size(width: theme.isTV ? 180 : 120)
            input.onChange = { [weak self] in self?.typed($0) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func typed(_ text: String) {
            let digits = String(text.filter(\.isNumber).prefix(7))
            let value = Swift.min(Int(digits) ?? 0, max)
            let shown = value > 0 ? String(value) : ""
            if input.text != shown, !(digits.isEmpty && text.isEmpty) { input.text = shown }
            onChange?(value)
        }
    }
#endif
