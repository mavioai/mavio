#if os(macOS)
    import AppKit
    import MavioMac

    /// A page of a server's settings: its server, the work it runs, which
    /// stops as the page goes, and the polling it does while it shows.
    class ServerPage: Page {
        /// The server whose settings the page shows.
        let server: ServerRecord
        private var tasks: [Task<Void, Never>] = []
        private var polls: [String: Task<Void, Never>] = [:]

        /// A page of server's, in context.
        init(context: MacContext, server: ServerRecord) {
            self.server = server
            super.init(context: context)
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            appeared()
        }

        override func viewDidDisappear() {
            super.viewDidDisappear()
            stopPolling()
        }

        /// Called each time the page shows, as it opens and as the page over
        /// it closes: where it reads what it shows and starts polling, as the
        /// SwiftUI screens' tasks do.
        func appeared() {}

        /// Runs work, cancelled if the page goes first.
        final func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.append(Task { await work() })
        }

        /// Runs work again and again, interval apart, while the page shows,
        /// in place of the polling of the same key; work should hold the page
        /// weakly.
        final func poll(
            _ key: String = "", every interval: @escaping @MainActor () -> Duration,
            _ work: @escaping @MainActor () async -> Void
        ) {
            polls[key]?.cancel()
            polls[key] = Task {
                while !Task.isCancelled {
                    await work()
                    try? await Task.sleep(for: interval())
                }
            }
        }

        /// Stops the polling that runs.
        final func stopPolling() {
            for poll in polls.values { poll.cancel() }
            polls = [:]
        }

        isolated deinit {
            for task in tasks + polls.values { task.cancel() }
        }

        // MARK: Views

        /// A section of rows with a heading and a footer.
        final func section(_ header: String? = nil, footer: String? = nil, _ rows: [NSView] = []) -> FormSection {
            FormSection(header, footer: footer, rows: rows, theme: theme)
        }

        /// A page's title and message at its top, with its actions, as Save,
        /// at its end where the SwiftUI screens put them in the toolbar.
        final func heading(_ title: String, message: String? = nil, actions: [NSView] = []) -> NSView {
            var text: [NSView] = [NSTextField.label(title, theme.fonts.title1, color: theme.colors.textPrimary)]
            if let message {
                text.append(NSTextField.label(message, theme.fonts.body, color: theme.colors.textSecondary, lines: 0))
            }
            let left = NSStackView.vertical(text, spacing: MavioSpacing.s4)
            left.setContentHuggingPriority(.init(1), for: .horizontal)
            left.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            return NSStackView.horizontal([left] + actions, spacing: MavioSpacing.s12, alignment: .top)
        }

        /// A line in a card saying a list is empty, or why it could not be
        /// read, in the danger's color when failed.
        final func note(_ text: String, failed: Bool = false) -> NSView {
            let label = NSTextField.label(
                text, theme.fonts.body, color: failed ? theme.colors.danger : theme.colors.textSecondary, lines: 0,
                selectable: failed)
            return padded(label)
        }

        /// A view inset as a row's content is.
        final func padded(_ view: NSView) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            holder.addFilling(view, insets: .symmetric(horizontal: h, vertical: v))
            holder.heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 44).isActive = true
            return holder
        }

        /// A row's title and the line under it.
        final func text(_ title: String, _ subtitle: String? = nil) -> NSStackView {
            ControlRow.text(title, subtitle: subtitle, theme: theme)
        }

        /// A row of a value at its end, selectable to copy.
        final func valueRow(_ title: String, _ value: String, detail: String? = nil) -> FormRow {
            let label = NSTextField.label(
                value, theme.fonts.body, color: theme.colors.textSecondary, lines: 0, selectable: true)
            label.alignment = .right
            return FormRow(title, subtitle: detail, accessory: label, theme: theme)
        }

        /// A row of a switch.
        final func toggleRow(
            _ title: String, _ detail: String? = nil, _ isOn: Bool, _ onChange: @escaping @MainActor (Bool) -> Void
        ) -> FormRow {
            FormRow.toggle(title, subtitle: detail, isOn: isOn, theme: theme, onChange: onChange)
        }

        /// A row of a choice among options.
        final func choiceRow<Value: Equatable>(
            _ title: String, _ detail: String? = nil, _ options: [(String, Value)], _ selected: Value,
            _ onChange: @escaping @MainActor (Value) -> Void
        ) -> FormRow {
            FormRow.choice(
                title, subtitle: detail, options: options, selected: selected, theme: theme, onChange: onChange)
        }

        /// A field of text, focusable in television mode.
        final func field(
            _ text: String, placeholder: String? = nil, secure: Bool = false, width: CGFloat? = 200,
            onChange: @escaping @MainActor (String) -> Void
        ) -> (FocusFrame, TextInput) {
            let input = TextInput(text, placeholder: placeholder, secure: secure)
            input.onChange = onChange
            if let width { input.size(width: width) }
            return (FocusFrame(input, theme: theme), input)
        }

        /// A row of a text field at its end.
        final func fieldRow(
            _ title: String, _ detail: String? = nil, text: String, placeholder: String? = nil, secure: Bool = false,
            onChange: @escaping @MainActor (String) -> Void
        ) -> ControlRow {
            let (frame, input) = field(text, placeholder: placeholder, secure: secure, onChange: onChange)
            input.field.alignment = .right
            return ControlRow(self.text(title, detail), controls: [frame], theme: theme)
        }

        /// A row of a field of a number, red while it is outside its range
        /// or not a number; onChange tells the number, nil when it is not one.
        final func numberRow(
            _ title: String, _ detail: String? = nil, value: Double, range: ClosedRange<Double>, decimals: Bool = false,
            onChange: @escaping @MainActor (Double?) -> Void, onEnd: (@MainActor () -> Void)? = nil
        ) -> ControlRow {
            let shown =
                decimals
                ? value.formatted(.number.precision(.fractionLength(0...2)).grouping(.never))
                : String(Int64(value.rounded()))
            let (frame, input) = field(shown, width: 80) { _ in }
            input.field.alignment = .right
            let theme = theme
            let color = { (text: String) in
                let n = Self.number(text)
                input.field.textColor = n.map(range.contains) == true ? theme.colors.textPrimary : theme.colors.danger
            }
            color(shown)
            input.onChange = { text in
                color(text)
                onChange(Self.number(text))
            }
            if let onEnd { input.onEnd = { _ in onEnd() } }
            return ControlRow(self.text(title, detail), controls: [frame], theme: theme)
        }

        /// The number text holds, in the user's format or plain.
        static func number(_ text: String) -> Double? {
            let trimmed = text.trimmingCharacters(in: .whitespaces)
            if let n = Double(trimmed) { return n }
            let formatter = NumberFormatter()
            formatter.numberStyle = .decimal
            return formatter.number(from: trimmed)?.doubleValue
        }

        /// A button of a kind.
        final func button(
            _ title: String, kind: MavioButton.Kind = .secondary, action: @escaping @MainActor () -> Void
        ) -> MavioButton {
            MavioButton(title, kind: kind, theme: theme, action: action)
        }

        /// The Save button of a page, which the page enables.
        final func saveButton(_ title: String = MavioStrings.actionSave, action: @escaping @MainActor () -> Void)
            -> MavioButton
        {
            MavioButton(title, kind: .primary, theme: theme, action: action)
        }

        /// Asks to confirm a destructive action, as the SwiftUI screens'
        /// confirmation dialogs, doing it once confirmed.
        final func confirm(
            _ title: String, message: String? = nil, button: String, destructive: Bool = true,
            action: @escaping @MainActor () -> Void
        ) {
            let alert = NSAlert()
            alert.messageText = title
            if let message { alert.informativeText = message }
            let ok = alert.addButton(withTitle: button)
            ok.hasDestructiveAction = destructive
            alert.addButton(withTitle: MavioStrings.actionCancel)
            guard let window = view.window else {
                if alert.runModal() == .alertFirstButtonReturn { action() }
                return
            }
            alert.beginSheetModal(for: window) { response in
                if response == .alertFirstButtonReturn { MainActor.assumeIsolated { action() } }
            }
        }

        /// Has focus land again in television mode, after the content changed.
        final func refocus() {
            FocusEngine.active(for: view)?.schedule()
        }

        /// Leaves the field being typed in, so that what it holds is kept.
        final func endEditing() {
            view.window?.makeFirstResponder(nil)
        }

        /// Returns to the page below this one.
        final func dismiss() {
            if navigation?.top === self { navigation?.pop() }
        }
    }

    /// Copies text to the clipboard.
    @MainActor func copyToClipboard(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    /// The choices of a list, with the value chosen though the list lacks it.
    func choices(_ list: [(code: String, name: String)], _ chosen: String) -> [(code: String, name: String)] {
        chosen.isEmpty || list.contains { $0.code == chosen } ? list : list + [(chosen, chosen)]
    }

    /// A colored dot, of a state or severity.
    @MainActor func dot(_ color: NSColor, size: CGFloat = 8) -> Box {
        let dot = Box(fill: color, radius: size / 2)
        dot.size(width: size, height: size)
        return dot
    }

    /// A tile of a color with an icon, as the settings' entries have.
    @MainActor func colorTile(_ icon: String, _ hex: UInt32, size: CGFloat = 28) -> NSView {
        let tile = Box(fill: NSColor(hex: hex), radius: size / 4)
        tile.size(width: size, height: size)
        let image = IconView(icon, fill: true, size: size * 0.6, color: .white)
        tile.addSubview(image)
        image.center(in: tile)
        return tile
    }

    /// A word on a rounded ground, as a user's or an issue's badge.
    @MainActor func badge(
        _ text: String, _ style: MavioFontStyle, color: NSColor, fill: NSColor, radius: CGFloat = 4,
        horizontal: CGFloat = 6, vertical: CGFloat = 0
    ) -> Box {
        let box = Box(fill: fill, radius: radius)
        box.addFilling(
            NSTextField.label(text, style, color: color), insets: .symmetric(horizontal: horizontal, vertical: vertical)
        )
        box.setContentHuggingPriority(.required, for: .horizontal)
        return box
    }

    /// A bar of a share from 0 to 1, filled in a color over a track.
    final class ShareBar: FlippedView {
        private let track = Box()
        private let bar = Box()
        /// The share shown.
        var share: Double { didSet { needsLayout = true } }

        /// A bar of share in color, height high.
        init(_ share: Double, color: NSColor, track: NSColor, height: CGFloat = 4) {
            self.share = share
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            self.track.fill = track
            self.track.radius = height / 2
            bar.fill = color
            bar.radius = height / 2
            for box in [self.track, bar] {
                box.translatesAutoresizingMaskIntoConstraints = true
                addSubview(box)
            }
            size(height: height)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func layout() {
            super.layout()
            track.frame = bounds
            bar.frame = CGRect(x: 0, y: 0, width: bounds.width * min(1, max(0, share)), height: bounds.height)
        }
    }

    /// A day, in the user's language.
    func day(_ ts: Google_Protobuf_Timestamp) -> String {
        ts.date.formatted(date: .abbreviated, time: .omitted)
    }
#endif
