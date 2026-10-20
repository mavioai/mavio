#if os(macOS)
    import AppKit
    import MavioMac

    // The views the pages of this area share beyond the kit's: rows of any
    // content on a form's card, fields that tell each change, areas of
    // several lines, text in a box, tags, tiles, a wrapping flow, a list
    // in a table, and native controls made focusable in television mode.

    /// A row of a form's card holding any views, padded as FormRow's are;
    /// television mode's focus lands on the elements in it, not on it.
    class ContentRow: FlippedView {
        /// A row of view in theme.
        init(_ view: NSView, theme: Theme) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            addFilling(view, insets: .symmetric(horizontal: h, vertical: v))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// A row of views from its leading edge, as a group's buttons.
    @MainActor func bar(_ views: [NSView], spacing: CGFloat = MavioSpacing.s12) -> NSStackView {
        let stack = NSStackView.horizontal(views, spacing: spacing)
        stack.setHuggingPriority(.init(1), for: .horizontal)
        return stack
    }

    /// Text of a style and color on as many lines as it takes, selectable
    /// when it is worth copying, as a message or a failure.
    @MainActor func note(
        _ text: String, _ style: MavioFontStyle, _ color: NSColor, selectable: Bool = false
    ) -> NSTextField {
        let label = NSTextField.label(text, style, color: color, lines: 0, selectable: selectable)
        label.setContentCompressionResistancePriority(.init(1), for: .horizontal)
        return label
    }

    /// A row of a card showing text, as a failure or that a list is empty.
    @MainActor func noteRow(_ text: String, _ color: NSColor, theme: Theme, selectable: Bool = true) -> ContentRow {
        ContentRow(note(text, theme.fonts.body, color, selectable: selectable), theme: theme)
    }

    /// A spinner on a row of a card, while what it lists loads.
    @MainActor func spinnerRow(theme: Theme) -> ContentRow {
        let spinner = NSProgressIndicator()
        spinner.style = .spinning
        spinner.controlSize = .small
        spinner.startAnimation(nil)
        spinner.translatesAutoresizingMaskIntoConstraints = false
        let holder = NSView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        holder.addSubview(spinner)
        spinner.center(in: holder)
        holder.heightAnchor.constraint(equalTo: spinner.heightAnchor).isActive = true
        return ContentRow(holder, theme: theme)
    }

    /// A view of text alone, under the sections, as a group's failure or a
    /// list's state, inset as a heading is.
    @MainActor func plainNote(_ text: String, _ color: NSColor, theme: Theme) -> NSView {
        let holder = NSView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        holder.addFilling(
            note(text, theme.fonts.callout, color, selectable: true), insets: .symmetric(horizontal: MavioSpacing.s4))
        return holder
    }

    // MARK: Rows of controls

    /// A row of a switch, which is off and does nothing while not enabled.
    @MainActor func toggleRow(
        _ title: String, subtitle: String? = nil, isOn: Bool, enabled: Bool = true, theme: Theme,
        onChange: @escaping @MainActor (Bool) -> Void
    ) -> FormRow {
        let control = ActionSwitch(isOn: isOn, onChange: onChange)
        control.isEnabled = enabled
        return controlRow(title, subtitle: subtitle, control: control, enabled: enabled, theme: theme)
    }

    /// A row of a choice among options, as a menu at its end.
    @MainActor func choiceRow<Value: Equatable>(
        _ title: String, subtitle: String? = nil, options: [(String, Value)], selected: Value, enabled: Bool = true,
        theme: Theme, onChange: @escaping @MainActor (Value) -> Void
    ) -> FormRow {
        let popup = ActionPopUp(options: options.map(\.0), selected: options.firstIndex { $0.1 == selected } ?? 0) {
            onChange(options[$0].1)
        }
        popup.isEnabled = enabled
        return controlRow(title, subtitle: subtitle, control: popup, enabled: enabled, theme: theme)
    }

    /// A row of a control at its end: in television mode, select works the
    /// control, typing in a field or opening a picker; a row not enabled is
    /// dimmed and does nothing.
    @MainActor func controlRow(
        _ title: String, subtitle: String? = nil, control: NSView, enabled: Bool = true, theme: Theme
    ) -> FormRow {
        // A switch or menu the row holds directly is one the row's select
        // works even when disabled; held in a view of its own, it is not.
        let accessory: NSView
        if !enabled && (control is NSSwitch || control is NSPopUpButton) {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(control)
            accessory = holder
        } else {
            accessory = control
        }
        let row = FormRow(title, subtitle: subtitle, accessory: accessory, theme: theme)
        if !enabled { row.alphaValue = 0.5 }
        if theme.isTV, enabled, !(control is NSSwitch), !(control is NSPopUpButton) {
            row.onPress = { [weak control] in
                if let control { activateControl(control) }
            }
        }
        return row
    }

    /// Does to a control what a click does: toggles a switch or a check
    /// box, opens a menu or a picker, or puts the caret in a field.
    @MainActor func activateControl(_ control: NSView) {
        switch control {
        case let s as NSSwitch:
            guard s.isEnabled else { return }
            s.state = s.state == .on ? .off : .on
            s.sendAction(s.action, to: s.target)
        case let p as NSPopUpButton:
            if p.isEnabled { p.performClick(nil) }
        case let b as NSButton:
            if b.isEnabled { b.performClick(nil) }
        case let f as InputField:
            f.focus()
        case let f as NSTextField:
            f.window?.makeFirstResponder(f)
        case let a as TextArea:
            a.activate()
        case let p as SearchPicker:
            p.activate()
        case let m as MavioButton:
            m.activate()
        default:
            break
        }
    }

    /// A native control that television mode's focus lands on: a white ring
    /// of 3 pt shows focus (docs/tv.md §4), and select does what a click
    /// does.
    final class FocusControl: Box, Focusable {
        private let control: NSView

        /// The control, made focusable.
        init(_ control: NSView) {
            self.control = control
            super.init(radius: MavioRadius.md)
            addFilling(control, insets: .all(4))
            borderWidth = 3
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false {
            didSet {
                border = isFocused ? .white : nil
                if isFocused { FocusEngine.active(for: self)?.scrollIntoView(self) }
            }
        }

        func activate() { activateControl(control) }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// The control as television mode's focus reaches it: made focusable
    /// there, itself on the desktop.
    @MainActor func focusable(_ control: NSView, theme: Theme) -> NSView {
        theme.isTV ? FocusControl(control) : control
    }

    // MARK: Fields

    /// A field of one line that tells each change and Return, as SwiftUI's
    /// fields do; secure for a secret, which it can reveal.
    final class InputField: NSView, NSTextFieldDelegate {
        private(set) var field: NSTextField
        /// Called as the text changes.
        var onChange: (@MainActor (String) -> Void)?
        /// Called as Return is pressed.
        var onSubmit: (@MainActor () -> Void)?
        /// Called as the field stops being edited.
        var onEnd: (@MainActor () -> Void)?
        private let placeholder: String?

        /// A field of text with a placeholder, secure for a secret.
        init(text: String = "", placeholder: String? = nil, secure: Bool = false, width: CGFloat? = nil) {
            self.placeholder = placeholder
            field = Self.make(secure: secure)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            install(text: text)
            if let width { size(width: width) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private static func make(secure: Bool) -> NSTextField {
            let f = secure ? NSSecureTextField() : NSTextField()
            f.bezelStyle = .roundedBezel
            f.isAutomaticTextCompletionEnabled = false
            f.translatesAutoresizingMaskIntoConstraints = false
            f.lineBreakMode = .byTruncatingTail
            f.cell?.isScrollable = true
            return f
        }

        private func install(text: String) {
            field.stringValue = text
            field.placeholderString = placeholder
            field.delegate = self
            field.setContentHuggingPriority(.init(1), for: .horizontal)
            addFilling(field)
        }

        /// The text.
        var text: String {
            get { field.stringValue }
            set { field.stringValue = newValue }
        }

        /// Whether the text shows as dots.
        var secure: Bool {
            get { field is NSSecureTextField }
            set {
                guard newValue != secure else { return }
                let text = field.stringValue
                let alignment = field.alignment
                let enabled = field.isEnabled
                field.removeFromSuperview()
                field = Self.make(secure: newValue)
                field.alignment = alignment
                field.isEnabled = enabled
                install(text: text)
            }
        }

        /// Whether the field may be edited.
        var isEnabled: Bool {
            get { field.isEnabled }
            set { field.isEnabled = newValue }
        }

        /// Puts the caret in the field.
        func focus() {
            guard field.isEnabled else { return }
            window?.makeFirstResponder(field)
        }

        func controlTextDidChange(_ obj: Notification) {
            onChange?(field.stringValue)
        }

        func controlTextDidEndEditing(_ obj: Notification) {
            onEnd?()
        }

        func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            if selector == #selector(NSResponder.insertNewline(_:)), let onSubmit {
                onSubmit()
                return true
            }
            return false
        }
    }

    /// A field of several lines: its placeholder, a number of lines high or
    /// growing with its text between two heights, then scrolling; bordered,
    /// red while what it holds is bad. In television mode select puts the
    /// caret in it, and a white ring shows focus.
    final class TextArea: Box, Focusable, NSTextViewDelegate {
        /// The text view.
        let textView: NSTextView
        private let scroll: NSScrollView
        private let placeholderLabel: NSTextField
        private let theme: Theme
        private var heightConstraint: NSLayoutConstraint!
        private let lines: Int?
        private let minHeight: CGFloat
        private let maxHeight: CGFloat
        private let limit: Int
        /// Called as the text changes.
        var onChange: (@MainActor (String) -> Void)?
        /// Called as Return is pressed without Shift, which then starts no
        /// new line; nil lets Return start one.
        var onReturn: (@MainActor () -> Void)?

        /// An area of text, font, lines high, or growing from minHeight to
        /// maxHeight when lines is nil; limit characters at most.
        init(
            text: String = "", placeholder: String? = nil, font: NSFont, lines: Int? = nil, minHeight: CGFloat = 48,
            maxHeight: CGFloat = 160, limit: Int = 100_000, fill: NSColor? = nil, bordered: Bool = true, theme: Theme
        ) {
            self.theme = theme
            self.lines = lines
            self.minHeight = minHeight
            self.maxHeight = maxHeight
            self.limit = limit
            scroll = NSTextView.scrollableTextView()
            textView = scroll.documentView as! NSTextView
            placeholderLabel = .label(
                placeholder ?? "",
                MavioFontStyle(size: font.pointSize, lineHeight: font.pointSize * 1.3, weight: .regular, tracking: 0),
                color: theme.colors.textTertiary, lines: 0)
            super.init(fill: fill ?? theme.colors.surface, radius: 6)
            if bordered { border = theme.colors.border }
            placeholderLabel.font = font
            textView.font = font
            textView.string = text
            textView.isRichText = false
            textView.allowsUndo = true
            textView.drawsBackground = false
            textView.textColor = theme.colors.textPrimary
            textView.insertionPointColor = theme.colors.accent
            textView.isAutomaticQuoteSubstitutionEnabled = false
            textView.isAutomaticDashSubstitutionEnabled = false
            textView.isAutomaticSpellingCorrectionEnabled = false
            textView.isAutomaticTextReplacementEnabled = false
            textView.isContinuousSpellCheckingEnabled = false
            textView.textContainerInset = NSSize(width: 4, height: 6)
            textView.delegate = self
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.borderType = .noBorder
            scroll.translatesAutoresizingMaskIntoConstraints = false
            addFilling(scroll, insets: .all(1))
            addSubview(placeholderLabel)
            NSLayoutConstraint.activate([
                placeholderLabel.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 10),
                placeholderLabel.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -10),
                placeholderLabel.topAnchor.constraint(equalTo: topAnchor, constant: 7),
            ])
            placeholderLabel.isHidden = !text.isEmpty || placeholder == nil
            let initial = lines.map { CGFloat($0) * (font.pointSize * 1.25) + 14 } ?? minHeight
            heightConstraint = heightAnchor.constraint(equalToConstant: initial)
            heightConstraint.isActive = true
            setAccessibilityElement(false)
            textView.setAccessibilityLabel(placeholder)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The text.
        var text: String {
            get { textView.string }
            set {
                guard newValue != textView.string else { return }
                textView.string = newValue
                changed(notify: false)
            }
        }

        /// The text shown while the area is empty.
        var placeholder: String? {
            get { placeholderLabel.stringValue.isEmpty ? nil : placeholderLabel.stringValue }
            set {
                placeholderLabel.stringValue = newValue ?? ""
                placeholderLabel.isHidden = !textView.string.isEmpty || newValue?.isEmpty != false
            }
        }

        /// Whether what the area holds is bad, which reddens its border.
        var bad = false { didSet { restyle() } }

        /// Whether the area may be edited.
        var isEditable: Bool {
            get { textView.isEditable }
            set {
                textView.isEditable = newValue
                alphaValue = newValue ? 1 : 0.6
            }
        }

        private func restyle() {
            if isFocused && theme.isTV {
                border = .white
                borderWidth = 3
            } else {
                border = bad ? theme.colors.danger : theme.colors.border
                borderWidth = 1
            }
        }

        // MARK: Focus

        var isFocused = false {
            didSet {
                restyle()
                if isFocused { FocusEngine.active(for: self)?.scrollIntoView(self) }
            }
        }

        func activate() {
            guard textView.isEditable else { return }
            window?.makeFirstResponder(textView)
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        // MARK: Text

        func textDidChange(_ notification: Notification) {
            changed(notify: true)
        }

        private func changed(notify: Bool) {
            if textView.string.count > limit { textView.string = String(textView.string.prefix(limit)) }
            placeholderLabel.isHidden = !textView.string.isEmpty || placeholderLabel.stringValue.isEmpty
            fit()
            if notify { onChange?(textView.string) }
        }

        func textView(_ textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            guard selector == #selector(NSResponder.insertNewline(_:)), let onReturn else { return false }
            if NSApp.currentEvent?.modifierFlags.contains(.shift) == true {
                textView.insertNewlineIgnoringFieldEditor(nil)
            } else {
                onReturn()
            }
            return true
        }

        override func layout() {
            super.layout()
            fit()
        }

        /// Grows the area with its text, between its heights.
        private func fit() {
            guard lines == nil, let manager = textView.layoutManager, let container = textView.textContainer else {
                return
            }
            manager.ensureLayout(for: container)
            let used = manager.usedRect(for: container).height + textView.textContainerInset.height * 2 + 2
            let height = min(maxHeight, max(minHeight, ceil(used)))
            if heightConstraint.constant != height { heightConstraint.constant = height }
        }
    }

    // MARK: Marks

    /// Text in a box, as a command, selectable whole, red for an error.
    @MainActor func codeBox(_ text: String, danger: Bool = false, theme: Theme) -> NSView {
        let label = NSTextField.label(
            text, theme.fonts.footnote, color: danger ? theme.colors.danger : theme.colors.textPrimary, lines: 0,
            selectable: true)
        label.font = .monospacedSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
        let box = Box(fill: theme.colors.fill, radius: 6)
        box.addFilling(label, insets: .all(10))
        return box
    }

    /// A capsule of text, as a mark of a state or a badge.
    @MainActor func capsule(
        _ text: String, _ style: MavioFontStyle, color: NSColor, fill: NSColor?, horizontal: CGFloat = MavioSpacing.s8,
        vertical: CGFloat = 2, weight: NSFont.Weight? = nil
    ) -> Box {
        let label = NSTextField.label(text, style, color: color)
        if let weight { label.font = .systemFont(ofSize: style.size, weight: weight) }
        label.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
        let box = Box(fill: fill, radius: 999)
        box.addFilling(label, insets: .symmetric(horizontal: horizontal, vertical: vertical))
        return box
    }

    /// An icon on a tile of a color, its corners rounded to fit its size.
    @MainActor func colorTile(_ icon: String, color: NSColor, size: CGFloat) -> Box {
        let tile = Box(fill: color, radius: size * 0.22)
        tile.size(width: size, height: size)
        let image = IconView(icon, fill: true, size: (size * 0.55).rounded(), color: .white)
        tile.addSubview(image)
        image.center(in: tile)
        return tile
    }

    /// Text that shows another's when asked for, as a container's command
    /// or error.
    final class DisclosureView: NSView {
        private let button: MavioButton
        private let content: NSView

        /// A disclosure labeled label, showing text in a box.
        init(_ label: String, text: String, danger: Bool = false, theme: Theme) {
            content = codeBox(text, danger: danger, theme: theme)
            content.isHidden = true
            button = MavioButton(label, icon: "forward", kind: .plain, theme: theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let stack = NSStackView.vertical([button, content], spacing: 6)
            addFilling(stack)
            content.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            button.onPress = { [weak self] in self?.toggle() }
            button.setAccessibilityValue(false)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func toggle() {
            content.isHidden.toggle()
            button.icon = content.isHidden ? "forward" : "expand"
            button.setAccessibilityValue(!content.isHidden)
        }
    }

    /// A button copying text, its icon a check once it did.
    @MainActor func copyButton(_ text: String, label: String, copied: String, theme: Theme) -> MavioButton {
        let button = MavioButton(icon: "copy", kind: .plain, theme: theme)
        button.setAccessibilityLabel(label)
        if !theme.isTV { button.toolTip = label }
        button.onPress = { [weak button] in
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(text, forType: .string)
            button?.icon = "check"
            if !theme.isTV { button?.toolTip = copied }
            Task { @MainActor in
                try? await Task.sleep(for: .seconds(2))
                button?.icon = "copy"
                if !theme.isTV { button?.toolTip = label }
            }
        }
        return button
    }

    // MARK: Layout

    /// Views in lines, as many on a line as fit at their own sizes,
    /// wrapping, as tags or check boxes.
    final class WrapView: FlippedView {
        private let spacing: CGFloat
        private let lineSpacing: CGFloat
        private var items: [NSView] = []
        private var height: CGFloat = 0

        /// A flow of views spacing apart on a line, lineSpacing between
        /// lines.
        init(_ views: [NSView] = [], spacing: CGFloat, lineSpacing: CGFloat) {
            self.spacing = spacing
            self.lineSpacing = lineSpacing
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setItems(views)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Replaces the views.
        func setItems(_ views: [NSView]) {
            items.forEach { $0.removeFromSuperview() }
            items = views
            for v in views {
                v.translatesAutoresizingMaskIntoConstraints = true
                addSubview(v)
            }
            needsLayout = true
        }

        override var intrinsicContentSize: NSSize {
            NSSize(width: NSView.noIntrinsicMetric, height: height)
        }

        override func layout() {
            super.layout()
            let width = bounds.width
            var x: CGFloat = 0
            var y: CGFloat = 0
            var lineHeight: CGFloat = 0
            var line: [(NSView, NSSize)] = []
            func place() {
                var px: CGFloat = 0
                for (v, s) in line {
                    v.frame = NSRect(x: px, y: y + (lineHeight - s.height) / 2, width: s.width, height: s.height)
                    px += s.width + spacing
                }
            }
            for v in items {
                var s = v.fittingSize
                s.width = min(s.width, max(width, 1))
                if !line.isEmpty && x + s.width > width {
                    place()
                    y += lineHeight + lineSpacing
                    x = 0
                    lineHeight = 0
                    line = []
                }
                line.append((v, s))
                x += s.width + spacing
                lineHeight = max(lineHeight, s.height)
            }
            place()
            let total = items.isEmpty ? 0 : y + lineHeight
            if total != height {
                height = total
                invalidateIntrinsicContentSize()
            }
        }
    }

    /// A list of rows in a table, as tall as its rows up to a height, then
    /// scrolling: fixed heights, lines between them, and, when it chooses,
    /// a row clicked or picked with the keys.
    final class ListTable: NSView, NSTableViewDataSource, NSTableViewDelegate {
        /// The table.
        let table = NSTableView()
        private let scroll = NSScrollView()
        private var count = 0
        private var make: (@MainActor (Int) -> NSView)?
        private var rowHeight: (@MainActor (Int) -> CGFloat)?
        private var heightConstraint: NSLayoutConstraint!
        private let maxHeight: CGFloat
        /// Called with the row clicked or chosen with Return.
        var onChoose: (@MainActor (Int) -> Void)?

        /// A list at most maxHeight tall, with lines between its rows.
        init(maxHeight: CGFloat, lines: Bool = true, selectable: Bool = false, theme: Theme) {
            self.maxHeight = maxHeight
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let column = NSTableColumn(identifier: .init("row"))
            column.resizingMask = .autoresizingMask
            table.addTableColumn(column)
            table.headerView = nil
            table.style = .plain
            table.backgroundColor = .clear
            table.intercellSpacing = .zero
            table.columnAutoresizingStyle = .uniformColumnAutoresizingStyle
            table.selectionHighlightStyle = selectable ? .regular : .none
            table.allowsEmptySelection = true
            if lines {
                table.gridStyleMask = .solidHorizontalGridLineMask
                table.gridColor = theme.colors.border
            }
            table.dataSource = self
            table.delegate = self
            table.target = self
            table.action = #selector(clicked)
            scroll.documentView = table
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.automaticallyAdjustsContentInsets = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            addFilling(scroll)
            heightConstraint = heightAnchor.constraint(equalToConstant: 0)
            heightConstraint.isActive = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows count rows, each made by make, rowHeight tall.
        func show(
            count: Int, rowHeight: @escaping @MainActor (Int) -> CGFloat, make: @escaping @MainActor (Int) -> NSView
        ) {
            self.count = count
            self.make = make
            self.rowHeight = rowHeight
            table.reloadData()
            let total = (0..<count).reduce(CGFloat(0)) { $0 + rowHeight($1) }
            heightConstraint.constant = min(maxHeight, total)
        }

        /// Selects the row at index, showing it.
        func select(_ index: Int) {
            guard index >= 0, index < count else {
                table.deselectAll(nil)
                return
            }
            table.selectRowIndexes([index], byExtendingSelection: false)
            table.scrollRowToVisible(index)
        }

        @objc private func clicked() {
            let row = table.clickedRow
            if row >= 0 { onChoose?(row) }
        }

        func numberOfRows(in tableView: NSTableView) -> Int { count }

        func tableView(_ tableView: NSTableView, heightOfRow row: Int) -> CGFloat {
            rowHeight?(row) ?? 24
        }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            make?(row)
        }
    }

    /// A box of views scrolling within a height, as tall as they are below
    /// it, as a plugin's logs.
    final class CappedScroll: NSView {
        private let scroll = NSScrollView()
        private let document = FlippedView()
        private let content: NSView
        private var heightConstraint: NSLayoutConstraint!
        private let maxHeight: CGFloat

        /// The view content, maxHeight tall at most.
        init(_ content: NSView, maxHeight: CGFloat) {
            self.content = content
            self.maxHeight = maxHeight
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            document.translatesAutoresizingMaskIntoConstraints = false
            document.addFilling(content)
            scroll.documentView = document
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.automaticallyAdjustsContentInsets = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            addFilling(scroll)
            NSLayoutConstraint.activate([
                document.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
                document.trailingAnchor.constraint(equalTo: scroll.contentView.trailingAnchor),
                document.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
            ])
            heightConstraint = heightAnchor.constraint(equalToConstant: 0)
            heightConstraint.isActive = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func layout() {
            super.layout()
            let height = min(maxHeight, document.fittingSize.height)
            if abs(heightConstraint.constant - height) > 0.5 { heightConstraint.constant = height }
        }
    }

    // MARK: Views kept as they change

    /// A stack of views that stay the views they are as the stack changes:
    /// those kept are neither removed nor added again, so that a field
    /// being typed in keeps its caret while what is around it is drawn
    /// anew. With lines, a line is drawn between them, as a form's card
    /// does (FormSection), and none in television mode.
    final class KeptStack: NSView {
        private let stack: NSStackView
        private let lines: Bool
        private let theme: Theme
        private var views: [NSView] = []
        private var holders: [ObjectIdentifier: Holder] = [:]

        /// A view and the line above it.
        private final class Holder: FlippedView {
            let line: NSView

            init(_ view: NSView, lines: Bool, theme: Theme) {
                let line = NSView()
                line.translatesAutoresizingMaskIntoConstraints = false
                self.line = line
                super.init(frame: .zero)
                translatesAutoresizingMaskIntoConstraints = false
                let rule = Box(fill: theme.colors.border)
                rule.size(height: 1)
                line.addFilling(rule, insets: NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: 0, right: 0))
                let stack = NSStackView.vertical([line, view], spacing: 0)
                addFilling(stack)
                line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                line.isHidden = !lines
            }

            @available(*, unavailable)
            required init?(coder: NSCoder) { fatalError() }
        }

        /// A stack spacing apart, with lines between its views when lines.
        init(spacing: CGFloat, lines: Bool = false, theme: Theme) {
            self.lines = lines && !theme.isTV
            self.theme = theme
            stack = .vertical([], spacing: spacing)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(stack)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows views, in their order, keeping those already shown.
        func update(_ new: [NSView]) {
            let ids = new.map(ObjectIdentifier.init)
            let wanted = Set(ids)
            for view in views where !wanted.contains(ObjectIdentifier(view)) {
                holders.removeValue(forKey: ObjectIdentifier(view))?.removeFromSuperview()
            }
            let kept = views.map(ObjectIdentifier.init).filter { wanted.contains($0) }
            if kept != ids.filter({ holders[$0] != nil }) {
                // Moved: out of the arrangement, though not of the window.
                for id in kept { if let h = holders[id] { stack.removeArrangedSubview(h) } }
            }
            for (i, view) in new.enumerated() {
                let id = ObjectIdentifier(view)
                let holder: Holder
                if let h = holders[id] {
                    holder = h
                } else {
                    holder = Holder(view, lines: lines, theme: theme)
                    holders[id] = holder
                }
                if stack.arrangedSubviews.count <= i || stack.arrangedSubviews[i] !== holder {
                    stack.insertArrangedSubview(holder, at: i)
                    holder.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                }
                holder.line.isHidden = !lines || i == 0
            }
            views = new
        }
    }

    /// A section of a form whose rows stay the views they are as it changes
    /// (KeptStack): its heading, its rows on a card with lines between, and
    /// its footer; the card hides without rows.
    final class LiveSection: NSView {
        /// The section.
        let section: FormSection
        private let rows: KeptStack
        private var shown = false

        /// A section of a heading and a footer, either optional.
        init(_ header: String? = nil, footer: String? = nil, theme: Theme) {
            section = FormSection(header, footer: footer, rows: [], theme: theme)
            rows = KeptStack(spacing: theme.isTV ? MavioSpacing.s8 : 0, lines: true, theme: theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(section)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The footer.
        var footer: String? {
            get { section.footer }
            set { section.footer = newValue }
        }

        /// Shows rows, keeping those already shown.
        func setRows(_ new: [NSView]) {
            rows.update(new)
            if new.isEmpty == shown {
                shown = !new.isEmpty
                section.setRows(shown ? [rows] : [])
            }
        }
    }

    // MARK: Confirming

    /// Asks in a sheet over the page's window whether to do something,
    /// running confirm when the user does.
    @MainActor func confirm(
        _ title: String, message: String?, action: String, destructive: Bool = true, in view: NSView,
        confirm: @escaping @MainActor () -> Void
    ) {
        let alert = NSAlert()
        alert.messageText = title
        if let message { alert.informativeText = message }
        let button = alert.addButton(withTitle: action)
        button.hasDestructiveAction = destructive
        alert.addButton(withTitle: MavioStrings.actionCancel)
        guard let window = view.window else {
            if alert.runModal() == .alertFirstButtonReturn { confirm() }
            return
        }
        alert.beginSheetModal(for: window) { response in
            if response == .alertFirstButtonReturn { MainActor.assumeIsolated { confirm() } }
        }
    }

    // MARK: Times

    /// A time as a message shows it: the time alone today, else the day too.
    func shortTime(_ date: Date) -> String {
        Calendar.current.isDateInToday(date)
            ? date.formatted(date: .omitted, time: .shortened)
            : date.formatted(date: .abbreviated, time: .shortened)
    }
#endif
