#if os(macOS)
    import AppKit
    import MavioMac

    /// A page of a server's settings of this area: a form of sections under
    /// the page's title and message, drawn again from what the page holds
    /// whenever it changes, and the tasks it runs, cancelled as it goes.
    class FormController: Page {
        /// The server whose settings the page shows.
        let server: ServerRecord
        /// The form.
        private(set) lazy var form = FormPage(theme: theme)
        /// What the page says it is for, under its title.
        final var message: String?
        private var tasks: [Task<Void, Never>] = []
        /// Called when the page scrolls near its end, to load more.
        final var onNearEnd: (@MainActor () -> Void)? {
            didSet { watchScroll() }
        }
        private var scrollObserver: (any NSObjectProtocol)?

        /// A page of server in context.
        init(context: MacContext, server: ServerRecord) {
            self.server = server
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            render()
            show(form)
        }

        /// The page's sections, below its title; the subclass's.
        func sections() -> [NSView] { [] }

        /// Draws the page again from what it holds.
        final func render() {
            guard isViewLoaded else { return }
            var views: [NSView] = [heading()]
            views += sections()
            form.setSections(views)
            FocusEngine.active(for: view)?.schedule()
            if onNearEnd != nil { DispatchQueue.main.async { [weak self] in self?.checkEnd() } }
        }

        private func watchScroll() {
            guard scrollObserver == nil, onNearEnd != nil else { return }
            let clip = form.page.scroll.contentView
            clip.postsBoundsChangedNotifications = true
            scrollObserver = NotificationCenter.default.addObserver(
                forName: NSView.boundsDidChangeNotification, object: clip, queue: .main
            ) { [weak self] _ in MainActor.assumeIsolated { self?.checkEnd() } }
        }

        /// Calls onNearEnd when less than a screen is left below what shows.
        private func checkEnd() {
            let scroll = form.page.scroll
            guard let document = scroll.documentView else { return }
            let visible = scroll.contentView.bounds
            if document.frame.height - visible.maxY < max(visible.height, 400) { onNearEnd?() }
        }

        private func heading() -> NSView {
            var views: [NSView] = [NSTextField.label(title ?? "", theme.fonts.title1, color: theme.colors.textPrimary)]
            if let message {
                views.append(NSTextField.label(message, theme.fonts.body, color: theme.colors.textSecondary, lines: 0))
            }
            return NSStackView.vertical(views, spacing: MavioSpacing.s4)
        }

        /// Runs work while the page is open.
        final func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        isolated deinit {
            for task in tasks { task.cancel() }
            if let scrollObserver { NotificationCenter.default.removeObserver(scrollObserver) }
        }

        // MARK: Rows

        /// A section of rows, without a heading.
        final func card(_ rows: [NSView]) -> FormSection {
            FormSection(rows: rows, theme: theme)
        }

        /// A section of rows in theme.
        final func card(_ header: String? = nil, footer: String? = nil, _ rows: [NSView]) -> FormSection {
            FormSection(header, footer: footer, rows: rows, theme: theme)
        }

        /// A section whose heading has a button at its end, as Add.
        final func card(
            _ header: String, button: (String, @MainActor () -> Void)?, footer: String? = nil, _ rows: [NSView]
        ) -> NSView {
            var head: [NSView] = [NSTextField.label(header, theme.fonts.headline, color: theme.colors.textPrimary)]
            if let button {
                head.append(Spacer())
                head.append(MavioButton(button.0, kind: .link, theme: theme, action: button.1))
            }
            let top = NSView()
            top.translatesAutoresizingMaskIntoConstraints = false
            top.addFilling(
                NSStackView.horizontal(head, spacing: MavioSpacing.s12), insets: .symmetric(horizontal: MavioSpacing.s4)
            )
            let body = FormSection(footer: footer, rows: rows, theme: theme)
            let stack = NSStackView.vertical([top, body], spacing: MavioSpacing.s8)
            for view in [top, body] { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return stack
        }

        /// A row of a card holding any view, inset as rows are.
        final func row(_ content: NSView) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            holder.addFilling(content, insets: .symmetric(horizontal: h, vertical: v))
            holder.heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 44).isActive = true
            return holder
        }

        /// A line of a card telling what it lacks, or what failed.
        final func note(_ text: String, failed: Bool = false) -> NSView {
            row(label(text, failed ? theme.colors.danger : theme.colors.textSecondary))
        }

        /// A selectable label of text in the callout's type, wrapping.
        final func label(_ text: String, _ color: NSColor? = nil, style: MavioFontStyle? = nil) -> NSTextField {
            NSTextField.label(
                text, style ?? theme.fonts.callout, color: color ?? theme.colors.textPrimary, lines: 0, selectable: true
            )
        }

        /// A spinner as a card's row, while it loads.
        final func spinnerRow() -> NSView {
            let spinner = NSProgressIndicator()
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.startAnimation(nil)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(spinner)
            spinner.center(in: holder)
            holder.heightAnchor.constraint(equalToConstant: 44).isActive = true
            return holder
        }

        /// A row of a text field at its end, select putting the caret in it.
        final func fieldRow(
            _ title: String, subtitle: String? = nil, text: String, placeholder: String? = nil, secure: Bool = false,
            width: CGFloat = 280, onChange: @escaping @MainActor (String) -> Void
        ) -> FormRow {
            let input = TextInput(
                text, placeholder: placeholder ?? (secure ? nil : title), secure: secure, width: width,
                onChange: onChange)
            input.field.setAccessibilityLabel(title)
            let row = FormRow(title, subtitle: subtitle, accessory: input, theme: theme)
            if theme.isTV { row.onPress = { [weak input] in input?.activate() } }
            return row
        }

        /// A row of a number at its end, select putting the caret in it.
        final func numberRow(
            _ title: String, subtitle: String? = nil, value: Double, range: ClosedRange<Double> = 0...Double(Int32.max),
            step: Double = 1, stepped: Bool = true, decimals: Int = 0,
            onChange: @escaping @MainActor (Double) -> Void
        ) -> FormRow {
            let field = NumberField(
                title, value: value, range: range, step: step, stepped: stepped, decimals: decimals, theme: theme,
                onChange: onChange)
            let row = FormRow(title, subtitle: subtitle, accessory: field, theme: theme)
            if theme.isTV { row.onPress = { [weak field] in field?.activate() } }
            return row
        }

        /// A row of a switch.
        final func toggleRow(
            _ title: String, subtitle: String? = nil, isOn: Bool, onChange: @escaping @MainActor (Bool) -> Void
        ) -> FormRow {
            FormRow.toggle(title, subtitle: subtitle, isOn: isOn, theme: theme, onChange: onChange)
        }

        /// A row of a choice among options, as a menu at its end.
        final func choiceRow<Value: Equatable>(
            _ title: String, subtitle: String? = nil, options: [(String, Value)], selected: Value,
            onChange: @escaping @MainActor (Value) -> Void
        ) -> FormRow {
            FormRow.choice(
                title, subtitle: subtitle, options: options, selected: selected, theme: theme, onChange: onChange)
        }

        /// A row of an icon on a colored tile, its title and the line under
        /// it, opening a page when it has an action.
        final func tileRow(
            _ title: String, subtitle: String?, icon: String, tint: UInt32,
            action: (@MainActor () -> Void)? = nil
        ) -> FormRow {
            let subtitle = subtitle?.isEmpty == false ? subtitle : nil
            if let action {
                return .link(
                    title, subtitle: subtitle, icon: icon, tint: NSColor(hex: tint), theme: theme, action: action)
            }
            return FormRow(title, subtitle: subtitle, icon: icon, tint: NSColor(hex: tint), theme: theme)
        }

        /// A row in the accent's color adding something, opening its page.
        final func addRow(_ title: String, action: @escaping @MainActor () -> Void) -> FormRow {
            .action(title, theme: theme, action: action)
        }

        /// The buttons moving row i of an ordered list of count rows up
        /// and down, by one; the ends' buttons disabled.
        final func moveButtons(at i: Int, of count: Int, move: @escaping @MainActor (Int, Int) -> Void) -> NSView {
            let up = IconAction("back", label: MavioStrings.acquisitionMoveUp, rotation: 90, theme: theme, action: nil)
            let down = IconAction(
                "back", label: MavioStrings.acquisitionMoveDown, rotation: -90, theme: theme, action: nil)
            if i > 0 { up.onPress = { move(i, -1) } }
            if i < count - 1 { down.onPress = { move(i, 1) } }
            return NSStackView.horizontal([up, down], spacing: MavioSpacing.s4)
        }

        /// A button removing a row, as an icon.
        final func removeButton(_ action: (@MainActor () -> Void)?) -> IconAction {
            IconAction("close", label: MavioStrings.rulesRemove, theme: theme, action: action)
        }

        /// A link of text opening an item of the server.
        final func itemLink(_ text: String, itemId: String) -> MavioButton {
            let server = server
            let button = MavioButton(text, kind: .plain, theme: theme) { [weak self] in
                self?.open(.item(serverId: server.id, itemId: itemId))
            }
            button.setContentHuggingPriority(.required, for: .horizontal)
            return button
        }

        /// A view outside the cards, inset as their headings are.
        final func plain(_ view: NSView) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(view, insets: .symmetric(horizontal: MavioSpacing.s4))
            return holder
        }

        // MARK: Dialogs

        /// Asks to confirm a destructive action, as a sheet over the window.
        final func confirm(
            _ title: String, message: String? = nil, action: String, run: @escaping @MainActor () -> Void
        ) {
            let alert = NSAlert()
            alert.messageText = title
            alert.informativeText = message ?? ""
            alert.alertStyle = .warning
            let button = alert.addButton(withTitle: action)
            button.hasDestructiveAction = true
            alert.addButton(withTitle: MavioStrings.actionCancel)
            guard let window = view.window else {
                if alert.runModal() == .alertFirstButtonReturn { run() }
                return
            }
            alert.beginSheetModal(for: window) { response in
                if response == .alertFirstButtonReturn { MainActor.assumeIsolated { run() } }
            }
        }

        /// Leaves the field being typed in, keeping what it holds.
        final func endEditing() {
            view.window?.makeFirstResponder(nil)
        }

        /// Returns to the page this one was opened from.
        final func dismiss() {
            if navigation?.top === self { navigation?.pop() }
        }
    }

    /// The buttons of a form under its groups: saving, with what saving
    /// told; and removing, when the form's thing is saved.
    final class SaveBar: NSView {
        /// The button saving.
        let button: MavioButton
        private let title: String
        private let noticeLabel: NSTextField
        private let failureLabel: NSTextField

        /// A bar of a button titled title, saving.
        init(_ title: String, theme: Theme, extra: [NSView] = [], save: @escaping @MainActor () -> Void) {
            self.title = title
            button = MavioButton(title, kind: .primary, theme: theme, action: save)
            noticeLabel = .label("", theme.fonts.callout, color: theme.colors.textSecondary)
            failureLabel = .label("", theme.fonts.callout, color: theme.colors.danger, lines: 0, selectable: true)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let stack = NSStackView.horizontal(
                [button] + extra + [noticeLabel, failureLabel], spacing: MavioSpacing.s12)
            addFilling(stack, insets: .symmetric(horizontal: MavioSpacing.s4))
            update(enabled: false, busy: false, notice: nil, failure: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows whether saving may be pressed, is running, and what it told.
        func update(enabled: Bool, busy: Bool, notice: String?, failure: String?) {
            button.title = busy ? "…" : title
            button.isEnabled = enabled && !busy
            noticeLabel.stringValue = notice ?? ""
            noticeLabel.isHidden = notice == nil
            failureLabel.stringValue = failure ?? ""
            failureLabel.isHidden = failure == nil
        }
    }

    extension Result where Failure == any Error {
        /// The result of an asynchronous call.
        init(catching body: () async throws -> Success) async {
            do { self = .success(try await body()) } catch { self = .failure(error) }
        }
    }
#endif
