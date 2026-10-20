#if os(macOS)
    import AppKit
    import MavioDesign

    /// Television mode's keyboard, over the page in a dialog named by the
    /// field it types into: what is typed with its caret, a password as
    /// dots; keys of digits and letters with `-`, `_`, `.` and `@` in ten
    /// columns, which capitals and a set of symbols replace, and a space; a
    /// keypad for a number; keys deleting, clearing, and Done. What it types
    /// goes into the field as it is typed; Done keeps it, and back deletes a
    /// character or, with none, puts back what the field held and closes
    /// (docs/tv.md §4).
    @MainActor
    public final class TVKeyboard: FlippedView, FocusScope, KeyHandling {
        private enum KeySet { case lower, upper, symbols }

        private weak var field: NSTextField?
        private let original: String
        private let theme = Theme.tv
        private let number: Bool
        private let fraction: Bool
        private let secure: Bool
        private var text: String { didSet { typed() } }
        private var keySet = KeySet.lower
        private let display: NSTextField
        private let caret = Box()
        private let keys = NSStackView.vertical([], spacing: MavioSpacing.s8)
        private var blink: Timer?

        /// Opens the keyboard over the window of field, typing into it.
        public static func present(for field: NSTextField) {
            guard let content = field.window?.contentView else { return }
            let keyboard = TVKeyboard(field: field)
            content.addSubview(keyboard)
            keyboard.pin(to: content)
            keyboard.focusFirst()
        }

        private init(field: NSTextField) {
            self.field = field
            original = field.stringValue
            text = field.stringValue
            secure = field is NSSecureTextField
            let formatter = field.formatter as? NumberFormatter
            number = formatter != nil
            fraction = (formatter?.maximumFractionDigits ?? 0) > 0
            display = .label("", theme.fonts.title2, color: theme.colors.textPrimary)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let dim = Box(fill: NSColor.black.withAlphaComponent(0.55))
            addFilling(dim)
            let panel = Box(fill: theme.colors.surfaceOverlay, radius: 24)
            panel.translatesAutoresizingMaskIntoConstraints = false
            addSubview(panel)
            let name =
                field.placeholderString?.nilIfEmpty ?? field.accessibilityLabel()?.nilIfEmpty
                ?? (field.superview?.accessibilityLabel() ?? "")
            let title = NSTextField.label(name, theme.fonts.headline, color: theme.colors.textSecondary)
            caret.fill = theme.colors.accent
            caret.size(width: 3, height: 40)
            let line = NSStackView.horizontal([display, caret], spacing: 2)
            let shown = Box(fill: theme.colors.fill, radius: 12)
            shown.addFilling(line, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
            shown.heightAnchor.constraint(greaterThanOrEqualToConstant: 72).isActive = true
            let column = NSStackView.vertical([title, shown, keys], spacing: MavioSpacing.s16)
            column.alignment = .leading
            panel.addFilling(column, insets: .all(MavioSpacing.s32))
            shown.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            NSLayoutConstraint.activate([
                panel.centerXAnchor.constraint(equalTo: centerXAnchor),
                panel.centerYAnchor.constraint(equalTo: centerYAnchor),
                panel.widthAnchor.constraint(equalToConstant: number ? 520 : 1000),
            ])
            buildKeys()
            typed()
            blink = Timer.scheduledTimer(withTimeInterval: 0.53, repeats: true) { [weak self] _ in
                MainActor.assumeIsolated { self.map { $0.caret.isHidden.toggle() } }
            }
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        isolated deinit { blink?.invalidate() }

        private func focusFirst() {
            guard let engine = FocusEngine.active(for: self) else { return }
            engine.focus(firstKey)
        }

        private weak var firstKey: NSView?

        // MARK: Keys

        private func buildKeys() {
            var rows: [[KeyboardKey]] = []
            if number {
                for chars in ["123", "456", "789"] { rows.append(chars.map { char(String($0)) }) }
                rows.append([char("-"), char("0")] + (fraction ? [char(".")] : []))
            } else {
                let sets: [String] =
                    switch keySet {
                    case .lower: ["1234567890", "abcdefghij", "klmnopqrst", "uvwxyz-_.@"]
                    case .upper: ["1234567890", "ABCDEFGHIJ", "KLMNOPQRST", "UVWXYZ-_.@"]
                    case .symbols: ["!?#$%&*+=/", "()[]{}<>\\|", ":;,'\"`~^-_", "€£¥°§·.@"]
                    }
                for set in sets {
                    rows.append(set.map { char(String($0)) })
                }
                rows.append([
                    word("ABC", wide: 2, chosen: keySet == .upper) { [weak self] in
                        self?.switchLayer(self?.keySet == .upper ? .lower : .upper)
                    },
                    word("#+=", wide: 2, chosen: keySet == .symbols) { [weak self] in
                        self?.switchLayer(self?.keySet == .symbols ? .lower : .symbols)
                    },
                    word(MavioStrings.searchSpace, wide: 6) { [weak self] in self?.text.append(" ") },
                ])
            }
            rows.append([
                icon("backspace") { [weak self] in self?.deleteOne() },
                word(MavioStrings.searchClear, wide: number ? 1 : 3) { [weak self] in self?.text = "" },
                word(MavioStrings.actionDone, wide: number ? 1 : 5, accent: true) { [weak self] in self?.done() },
            ])
            keys.setViews(
                rows.map { row in
                    let stack = NSStackView.horizontal(row, spacing: MavioSpacing.s8)
                    stack.distribution = .fill
                    return stack
                })
            firstKey = rows.first?.first
        }

        private func char(_ value: String) -> KeyboardKey {
            KeyboardKey(title: value, theme: theme) { [weak self] in self?.text.append(value) }
        }

        private func word(
            _ title: String, wide: Int, chosen: Bool = false, accent: Bool = false,
            action: @escaping @MainActor () -> Void
        ) -> KeyboardKey {
            let key = KeyboardKey(title: title, theme: theme, units: wide, accent: accent, action: action)
            key.chosen = chosen
            return key
        }

        private func icon(_ name: String, action: @escaping @MainActor () -> Void) -> KeyboardKey {
            KeyboardKey(icon: name, theme: theme, action: action)
        }

        private func switchLayer(_ next: KeySet) {
            keySet = next
            buildKeys()
            focusFirst()
        }

        // MARK: Typing

        private func typed() {
            display.stringValue = secure ? String(repeating: "•", count: text.count) : text
            caret.isHidden = false
            guard let field, field.stringValue != text else { return }
            field.stringValue = text
            NotificationCenter.default.post(name: NSControl.textDidChangeNotification, object: field)
            field.sendAction(field.action, to: field.target)
        }

        private func deleteOne() {
            if !text.isEmpty { text.removeLast() }
        }

        private func done() {
            let field = field
            close()
            guard let field else { return }
            NotificationCenter.default.post(name: NSControl.textDidEndEditingNotification, object: field)
            field.sendAction(field.action, to: field.target)
            FocusEngine.active(for: field)?.focus(field)
        }

        private func cancel() {
            text = original
            let field = field
            close()
            if let field { FocusEngine.active(for: field)?.focus(field) }
        }

        private func close() {
            blink?.invalidate()
            let engine = FocusEngine.active(for: self)
            removeFromSuperview()
            engine?.schedule()
        }

        /// Back deletes a character, or closes with none; a computer's
        /// keyboard types into the keyboard as well.
        public func handleKey(_ event: NSEvent) -> Bool {
            switch event.keyCode {
            case 53, 51:
                if text.isEmpty { cancel() } else { deleteOne() }
                return true
            case 36, 76, 123, 124, 125, 126:
                return false
            default:
                guard let chars = event.characters, !chars.isEmpty,
                    chars.unicodeScalars.allSatisfy({ !CharacterSet.controlCharacters.contains($0) })
                else { return false }
                if number && !chars.allSatisfy({ $0.isNumber || $0 == "-" || ($0 == "." && fraction) }) { return true }
                text.append(chars)
                return true
            }
        }

        override public func mouseDown(with event: NSEvent) {}
    }

    /// A key of the keyboard: white with focus, Done in the accent.
    final class KeyboardKey: Box, Focusable {
        private let theme: Theme
        private let action: @MainActor () -> Void
        private let label: NSTextField?
        private let glyph: IconView?
        private let accent: Bool
        var chosen = false { didSet { restyle() } }

        init(
            title: String? = nil, icon: String? = nil, theme: Theme, units: Int = 1, accent: Bool = false,
            action: @escaping @MainActor () -> Void
        ) {
            self.theme = theme
            self.action = action
            self.accent = accent
            label = title.map { .label($0, theme.fonts.headline, color: theme.colors.textPrimary) }
            glyph = icon.map { IconView($0, size: 28, color: theme.colors.textPrimary) }
            super.init(radius: 10)
            let unit: CGFloat = 84
            size(width: unit * CGFloat(units) + MavioSpacing.s8 * CGFloat(units - 1), height: 64)
            if let content: NSView = label ?? glyph {
                addSubview(content)
                content.center(in: self)
            }
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(title ?? icon)
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        func activate() { action() }

        private func restyle() {
            let content: NSColor
            if isFocused {
                fill = .white
                content = .black
            } else if accent {
                fill = theme.colors.accent
                content = .white
            } else {
                fill = chosen ? theme.colors.fillSelected : theme.colors.fill
                content = theme.colors.textPrimary
            }
            label?.textColor = content
            glyph?.contentTintColor = content
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    extension String {
        fileprivate var nilIfEmpty: String? { isEmpty ? nil : self }
    }
#endif
