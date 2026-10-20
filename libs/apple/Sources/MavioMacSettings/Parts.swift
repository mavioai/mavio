#if os(macOS)
    import AppKit
    import MavioMac

    /// What the settings' pages build their rows of.
    @MainActor
    enum SettingsParts {
        /// A view as a form's row, inset as a row's content.
        static func padded(_ view: NSView, theme: Theme) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(
                view,
                insets: .symmetric(
                    horizontal: theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16, vertical: MavioSpacing.s12))
            return holder
        }

        /// A line of text as a form's row.
        static func text(_ text: String, color: NSColor, theme: Theme) -> NSView {
            padded(NSTextField.label(text, theme.fonts.callout, color: color, lines: 0), theme: theme)
        }

        /// The digits of text, at most count of them.
        static func digits(_ text: String, _ count: Int) -> String {
            String(text.filter(\.isNumber).prefix(count))
        }

        /// A field of a code of count digits, in the monospaced font.
        static func codeField(placeholder: String) -> NSTextField {
            let field = NSTextField()
            field.placeholderString = placeholder
            field.bezelStyle = .roundedBezel
            field.font = .monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular)
            field.translatesAutoresizingMaskIntoConstraints = false
            return field
        }

        /// A small spinner, turning.
        static func spinner() -> NSProgressIndicator {
            let spinner = NSProgressIndicator()
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.translatesAutoresizingMaskIntoConstraints = false
            spinner.startAnimation(nil)
            return spinner
        }
    }

    /// A text field that television mode's focus lands on: focus rings it
    /// white, and select starts typing in it, which Escape ends.
    final class FocusField: Box, Focusable {
        /// The field.
        let field: NSTextField
        private let theme: Theme

        /// The field, in theme.
        init(_ field: NSTextField, theme: Theme) {
            self.field = field
            self.theme = theme
            super.init(radius: theme.isTV ? MavioRadius.md : 0)
            field.translatesAutoresizingMaskIntoConstraints = false
            addFilling(field, insets: .all(theme.isTV ? 4 : 0))
            borderWidth = 3
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false {
            didSet { border = isFocused && theme.isTV ? .white : nil }
        }

        func activate() {
            if theme.isTV { TVKeyboard.present(for: field) } else { window?.makeFirstResponder(field) }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// Tells what a field holds as it is typed, and when Return commits it.
    @MainActor
    final class FieldWatcher: NSObject, NSTextFieldDelegate {
        private let onChange: @MainActor (String) -> Void
        private let onSubmit: (@MainActor () -> Void)?

        /// Watches field.
        init(
            _ field: NSTextField, onChange: @escaping @MainActor (String) -> Void,
            onSubmit: (@MainActor () -> Void)? = nil
        ) {
            self.onChange = onChange
            self.onSubmit = onSubmit
            super.init()
            field.delegate = self
        }

        func controlTextDidChange(_ obj: Notification) {
            guard let field = obj.object as? NSTextField else { return }
            onChange(field.stringValue)
        }

        func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
            guard commandSelector == #selector(NSResponder.insertNewline(_:)), let onSubmit else { return false }
            onSubmit()
            return true
        }
    }
#endif
