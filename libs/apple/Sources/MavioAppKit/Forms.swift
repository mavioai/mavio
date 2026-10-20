#if os(macOS)
    import AppKit
    import MavioDesign

    /// A page of settings as the web UI's in a wide window (apps/web's
    /// Group and Row): its title and message at the top, then its sections,
    /// each a heading, its rows on a rounded card with lines between and
    /// a footer, at most 900 points wide in the middle of the page. In
    /// television mode, the rows are a list that focus moves through.
    public final class FormPage: NSView {
        /// The scroll page holding the column.
        public let page: ScrollPage
        private let column: NSStackView
        private let theme: Theme
        private let head: NSStackView

        /// A page of title and message, either optional, in theme.
        public init(title: String? = nil, message: String? = nil, theme: Theme = .desktop) {
            self.theme = theme
            var heading: [NSView] = []
            if let title {
                heading.append(NSTextField.label(title, theme.fonts.title1, color: theme.colors.textPrimary))
            }
            if let message {
                heading.append(
                    NSTextField.label(message, theme.fonts.body, color: theme.colors.textSecondary, lines: 0))
            }
            head = .vertical(heading, spacing: MavioSpacing.s4)
            column = .vertical([], spacing: MavioSpacing.s32)
            if !heading.isEmpty { column.addArrangedSubview(head) }
            let holder = FlippedView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(column)
            let margin = theme.isTV ? theme.layout.margin : MavioSpacing.s48
            let vertical = theme.isTV ? MavioSpacing.s48 : MavioSpacing.s32
            NSLayoutConstraint.activate([
                column.topAnchor.constraint(equalTo: holder.topAnchor, constant: vertical),
                column.bottomAnchor.constraint(equalTo: holder.bottomAnchor, constant: -vertical),
                column.centerXAnchor.constraint(equalTo: holder.centerXAnchor),
                column.leadingAnchor.constraint(greaterThanOrEqualTo: holder.leadingAnchor, constant: margin),
                column.widthAnchor.constraint(lessThanOrEqualToConstant: theme.isTV ? 1100 : 900),
            ])
            let fill = column.leadingAnchor.constraint(equalTo: holder.leadingAnchor, constant: margin)
            fill.priority = .defaultHigh
            fill.isActive = true
            if !heading.isEmpty { head.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            page = ScrollPage([])
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            page.add(holder)
            addFilling(page)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Adds a section, or any view as wide as the column.
        public func add(_ view: NSView) {
            column.addArrangedSubview(view)
            view.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
        }

        /// Replaces the sections, keeping the title.
        public func setSections(_ views: [NSView]) {
            for view in column.arrangedSubviews where view !== head {
                column.removeArrangedSubview(view)
                view.removeFromSuperview()
            }
            for view in views { add(view) }
        }
    }

    /// A section of a form: its heading, its rows on a card, and its footer.
    public final class FormSection: NSView {
        private let card: Box
        private let rowStack: NSStackView
        private let theme: Theme
        private let footerLabel: NSTextField

        /// A section of rows with a heading and a footer, either optional.
        public init(_ header: String? = nil, footer: String? = nil, rows: [NSView] = [], theme: Theme = .desktop) {
            self.theme = theme
            card = Box(fill: theme.isTV ? nil : theme.colors.surface, radius: theme.isTV ? 0 : 14)
            rowStack = .vertical([], spacing: theme.isTV ? MavioSpacing.s8 : 0)
            card.addFilling(rowStack)
            footerLabel = .label(footer ?? "", theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0)
            footerLabel.isHidden = footer == nil
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            var views: [NSView] = []
            if let header {
                let label = NSTextField.label(header, theme.fonts.headline, color: theme.colors.textPrimary)
                views.append(inset(label))
            }
            views.append(card)
            views.append(inset(footerLabel))
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            addFilling(stack)
            card.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            setRows(rows)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        private func inset(_ view: NSView) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(view, insets: .symmetric(horizontal: MavioSpacing.s4))
            return holder
        }

        /// Sets the footer.
        public var footer: String? {
            get { footerLabel.isHidden ? nil : footerLabel.stringValue }
            set {
                footerLabel.stringValue = newValue ?? ""
                footerLabel.superview?.isHidden = newValue == nil
                footerLabel.isHidden = newValue == nil
            }
        }

        /// Replaces the rows; the card hides without any.
        public func setRows(_ rows: [NSView]) {
            rowStack.setViews([])
            for (i, row) in rows.enumerated() {
                if i > 0 && !theme.isTV {
                    let line = Box(fill: theme.colors.border)
                    line.size(height: 1)
                    let holder = NSView()
                    holder.translatesAutoresizingMaskIntoConstraints = false
                    holder.addFilling(line, insets: NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: 0, right: 0))
                    rowStack.addArrangedSubview(holder)
                    holder.widthAnchor.constraint(equalTo: rowStack.widthAnchor).isActive = true
                }
                rowStack.addArrangedSubview(row)
                row.widthAnchor.constraint(equalTo: rowStack.widthAnchor).isActive = true
            }
            card.isHidden = rows.isEmpty
        }
    }

    /// A row of a form: its label, the line under it, and at its end a
    /// switch, a choice, a value or a chevron. A row that does something
    /// is focusable in television mode, where focus shows it white.
    public final class FormRow: Box, Focusable {
        private let theme: Theme
        private let titleLabel: NSTextField
        private let subtitleLabel: NSTextField
        private let leading: NSStackView
        private let content: NSStackView
        /// What choosing the row does; a row without it is not focusable.
        public var onPress: (@MainActor () -> Void)?
        private var hovering = false
        private var tracking: NSTrackingArea?
        private weak var accessory: NSView?

        /// A row of title, subtitle and an accessory at its end, with an
        /// icon on a tile of tint at its start.
        public init(
            _ title: String, subtitle: String? = nil, icon: String? = nil, tint: NSColor? = nil,
            accessory: NSView? = nil, theme: Theme = .desktop, action: (@MainActor () -> Void)? = nil
        ) {
            self.theme = theme
            onPress = action
            titleLabel = .label(
                title, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary, lines: 0)
            subtitleLabel = .label(subtitle ?? "", theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0)
            subtitleLabel.isHidden = subtitle == nil
            leading = .vertical([titleLabel, subtitleLabel], spacing: 2)
            var views: [NSView] = []
            if let icon { views.append(Self.tile(icon, tint: tint ?? theme.colors.accent, theme: theme)) }
            views.append(leading)
            content = .horizontal(views, spacing: MavioSpacing.s12)
            super.init(radius: theme.isTV ? MavioRadius.lg : 0)
            addSubview(content)
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            let v: CGFloat = theme.isTV ? MavioSpacing.s16 : 10
            content.pin(to: self, insets: .symmetric(horizontal: h, vertical: v))
            heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 72 : 52).isActive = true
            leading.setContentHuggingPriority(.init(1), for: .horizontal)
            leading.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            if let accessory { setAccessory(accessory) }
            setAccessibilityElement(true)
            setAccessibilityLabel(title)
            setAccessibilityRole(action == nil ? .group : .button)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// An icon on a colored tile, as the settings' sections show.
        public static func tile(_ icon: String, tint: NSColor, theme: Theme = .desktop) -> NSView {
            let side: CGFloat = theme.isTV ? 44 : 28
            let tile = Box(fill: tint, radius: side / 4)
            tile.size(width: side, height: side)
            let image = IconView(icon, fill: true, size: side * 0.6, color: .white)
            tile.addSubview(image)
            image.center(in: tile)
            return tile
        }

        /// The row's title.
        public var title: String {
            get { titleLabel.stringValue }
            set { titleLabel.stringValue = newValue }
        }

        /// The line under the title.
        public var subtitle: String? {
            get { subtitleLabel.isHidden ? nil : subtitleLabel.stringValue }
            set {
                subtitleLabel.stringValue = newValue ?? ""
                subtitleLabel.isHidden = newValue == nil
            }
        }

        /// Puts a view at the row's end.
        public func setAccessory(_ view: NSView?) {
            if let accessory {
                content.removeArrangedSubview(accessory)
                accessory.removeFromSuperview()
            }
            accessory = view
            if let view {
                view.setContentHuggingPriority(.required, for: .horizontal)
                view.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
                content.addArrangedSubview(view)
            }
        }

        // MARK: Focus and pointer

        /// Whether it has television mode's focus.
        public var isFocused = false { didSet { restyle() } }

        /// Acts as a press of it.
        public func activate() {
            if let control = accessory as? NSControl, !control.isEnabled { return }
            if let control = accessory as? NSSwitch {
                control.state = control.state == .on ? .off : .on
                control.sendAction(control.action, to: control.target)
            } else if let popup = accessory as? NSPopUpButton {
                popup.performClick(nil)
            } else {
                onPress?()
            }
        }

        /// Whether focus may land on the row.
        public var canFocus: Bool { onPress != nil || accessory is NSControl }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : (hovering && onPress != nil && !theme.isTV ? theme.colors.fill : nil)
            titleLabel.textColor = focused ? .black : theme.colors.textPrimary
            subtitleLabel.textColor = focused ? NSColor.black.withAlphaComponent(0.6) : theme.colors.textSecondary
            if let chevron = accessory as? IconView {
                chevron.contentTintColor = focused ? .black : theme.colors.textTertiary
            }
            if let value = accessory as? NSTextField { value.textColor = focused ? .black : theme.colors.textSecondary }
            if theme.isTV {
                NSAnimationContext.runAnimationGroup { context in
                    context.duration = MavioDuration.fast
                    context.allowsImplicitAnimation = true
                    layer?.transform =
                        focused ? CATransform3DMakeScale(1.02, 1.02, 1) : CATransform3DIdentity
                }
                if focused { FocusEngine.active(for: self)?.scrollIntoView(self) }
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

        override public func mouseUp(with event: NSEvent) {
            guard onPress != nil, bounds.contains(convert(event.locationInWindow, from: nil)) else { return }
            onPress?()
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

    extension FormRow {
        /// A row opening a page: the whole row, a chevron at its end.
        public static func link(
            _ title: String, subtitle: String? = nil, value: String? = nil, icon: String? = nil, tint: NSColor? = nil,
            theme: Theme = .desktop, action: @escaping @MainActor () -> Void
        ) -> FormRow {
            let chevron = IconView("forward", size: 16, color: theme.colors.textTertiary)
            let end: NSView =
                if let value {
                    NSStackView.horizontal(
                        [NSTextField.label(value, theme.fonts.body, color: theme.colors.textSecondary), chevron],
                        spacing: MavioSpacing.s8)
                } else {
                    chevron
                }
            return FormRow(
                title, subtitle: subtitle, icon: icon, tint: tint, accessory: end, theme: theme, action: action)
        }

        /// A row of a switch.
        public static func toggle(
            _ title: String, subtitle: String? = nil, isOn: Bool, theme: Theme = .desktop,
            onChange: @escaping @MainActor (Bool) -> Void
        ) -> FormRow {
            let control = ActionSwitch(isOn: isOn, onChange: onChange)
            let row = FormRow(title, subtitle: subtitle, accessory: control, theme: theme)
            return row
        }

        /// A row of a choice among options, as a menu at its end.
        public static func choice<Value: Equatable>(
            _ title: String, subtitle: String? = nil, options: [(String, Value)], selected: Value,
            theme: Theme = .desktop, onChange: @escaping @MainActor (Value) -> Void
        ) -> FormRow {
            let popup = ActionPopUp(options: options.map(\.0), selected: options.firstIndex { $0.1 == selected } ?? 0) {
                onChange(options[$0].1)
            }
            return FormRow(title, subtitle: subtitle, accessory: popup, theme: theme)
        }

        /// A row of a value, selectable to copy.
        public static func value(_ title: String, _ value: String, theme: Theme = .desktop) -> FormRow {
            let label = NSTextField.label(value, theme.fonts.body, color: theme.colors.textSecondary, selectable: true)
            label.alignment = .right
            return FormRow(title, accessory: label, theme: theme)
        }

        /// A row of a text field at its end.
        public static func field(
            _ title: String, subtitle: String? = nil, text: String, placeholder: String? = nil, secure: Bool = false,
            width: CGFloat = 260, theme: Theme = .desktop, onCommit: @escaping @MainActor (String) -> Void
        ) -> FormRow {
            let field = ActionField(text: text, placeholder: placeholder, secure: secure, onCommit: onCommit)
            field.size(width: width)
            return FormRow(title, subtitle: subtitle, accessory: field, theme: theme)
        }

        /// A row that is an action: its title in the accent's color, or the
        /// danger's when destructive.
        public static func action(
            _ title: String, destructive: Bool = false, theme: Theme = .desktop,
            action: @escaping @MainActor () -> Void
        ) -> FormRow {
            let row = FormRow(title, theme: theme, action: action)
            row.titleLabel.textColor = destructive ? theme.colors.danger : theme.colors.accentText
            return row
        }
    }

    /// A switch that calls back with its state.
    public final class ActionSwitch: NSSwitch {
        private let onChange: @MainActor (Bool) -> Void

        /// A switch in state isOn.
        public init(isOn: Bool, onChange: @escaping @MainActor (Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            state = isOn ? .on : .off
            target = self
            action = #selector(changed)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(state == .on) }
    }

    /// A menu of options that calls back with the index chosen.
    public final class ActionPopUp: NSPopUpButton {
        private let onChange: @MainActor (Int) -> Void

        /// A menu of options, the one at selected chosen.
        public init(options: [String], selected: Int, onChange: @escaping @MainActor (Int) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero, pullsDown: false)
            translatesAutoresizingMaskIntoConstraints = false
            addItems(withTitles: options)
            // Titles alike would merge; set them item by item.
            for (i, title) in options.enumerated() where i < numberOfItems { item(at: i)?.title = title }
            while numberOfItems < options.count {
                menu?.addItem(withTitle: options[numberOfItems], action: nil, keyEquivalent: "")
            }
            selectItem(at: selected)
            target = self
            action = #selector(changed)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(indexOfSelectedItem) }
    }

    /// A text field that calls back with its text when it is committed.
    public final class ActionField: NSView, NSTextFieldDelegate {
        /// The field.
        public let field: NSTextField
        private let onCommit: @MainActor (String) -> Void

        /// A field of text, secure for a password.
        public init(
            text: String, placeholder: String? = nil, secure: Bool = false,
            onCommit: @escaping @MainActor (String) -> Void
        ) {
            self.onCommit = onCommit
            field = secure ? NSSecureTextField() : NSTextField()
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            field.stringValue = text
            field.placeholderString = placeholder
            field.bezelStyle = .roundedBezel
            field.delegate = self
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

        /// Keeps what was typed once the field's editing ends.
        public func controlTextDidEndEditing(_ obj: Notification) {
            onCommit(field.stringValue)
        }
    }
#endif
