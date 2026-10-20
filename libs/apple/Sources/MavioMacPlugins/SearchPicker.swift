#if os(macOS)
    import AppKit
    import MavioMac

    /// A tone of a picker's badge.
    enum PickerTone {
        /// A plain mark on a fill, as a custom provider's.
        case plain
        /// Quiet, as a protocol.
        case neutral
        /// Green, as public.
        case success
        /// Orange, as semi-private.
        case warning
        /// Red, as private.
        case danger
    }

    /// An option of a searching picker, as the web UI's model picker and
    /// searching picker list them: its value and name, a line of details
    /// under it, quiet text beside its name, badges, the keys its filters
    /// match, and, for a model, its provider, whose logo and name show.
    struct PickerItem {
        /// The value chosen.
        var value: String
        /// The name.
        var label: String
        /// The line under the name.
        var detail: String = ""
        /// Quiet text beside the name, such as a language.
        var meta: String?
        /// The badges after the name.
        var badges: [(label: String, tone: PickerTone)] = []
        /// The keys the filters match.
        var keys: [String] = []
        /// The provider's ID, whose logo shows before the name.
        var provider: String?
        /// The provider's name, quiet after the name.
        var providerName: String?

        /// Whether the option holds what was typed: in its name, value,
        /// details, provider or badges.
        func matches(_ query: String) -> Bool {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty else { return true }
            return ([label, value, detail, meta ?? "", providerName ?? ""] + badges.map(\.label)).contains {
                $0.localizedCaseInsensitiveContains(q)
            }
        }
    }

    /// The logo of an AI provider, from libs/design's: in its own colors, or
    /// the text's for a monochrome one; a chip's icon for a provider
    /// without, as the web UI's ProviderLogo.
    @MainActor func providerLogo(_ provider: String, size: CGFloat = 16, theme: Theme) -> NSView {
        if let name = MavioProviderLogos.assetName(for: provider),
            let image = MavioStrings.bundle.image(forResource: name)
        {
            let view = NSImageView(image: image)
            view.imageScaling = .scaleProportionallyUpOrDown
            view.contentTintColor = theme.colors.textPrimary
            view.setAccessibilityElement(false)
            return view.size(width: size, height: size)
        }
        return IconView("cpu", size: size, color: theme.colors.textTertiary)
    }

    /// The color of a badge's tone.
    @MainActor func toneColor(_ tone: PickerTone, theme: Theme) -> NSColor {
        switch tone {
        case .plain, .neutral: theme.colors.textSecondary
        case .success: NSColor(hex: 0x34C759)
        case .warning: NSColor(hex: 0xFF9500)
        case .danger: theme.colors.danger
        }
    }

    /// An option as the pickers show it on one line: its provider's logo,
    /// its name, its provider's name smaller and quieter, its badges and
    /// what is beside them, as the web UI's ModelLabel.
    @MainActor func pickerLabel(_ item: PickerItem, style: MavioFontStyle, theme: Theme) -> NSStackView {
        var views: [NSView] = []
        if let provider = item.provider { views.append(providerLogo(provider, theme: theme)) }
        let name = NSTextField.label(item.label, style, color: theme.colors.textPrimary)
        name.setContentCompressionResistancePriority(.init(260), for: .horizontal)
        views.append(name)
        if let p = item.providerName {
            views.append(
                NSTextField.label(
                    p, MavioFontStyle(size: 11, lineHeight: 14, weight: .regular, tracking: 0),
                    color: theme.colors.textTertiary))
        }
        for b in item.badges {
            let color = toneColor(b.tone, theme: theme)
            views.append(
                capsule(
                    b.label, theme.fonts.caption, color: color,
                    fill: b.tone == .plain ? theme.colors.fill : color.withAlphaComponent(0.15), horizontal: 6,
                    vertical: 1, weight: b.tone == .plain ? nil : .medium))
        }
        if let meta = item.meta {
            views.append(NSTextField.label(meta, theme.fonts.caption, color: theme.colors.textSecondary))
        }
        return .horizontal(views, spacing: MavioSpacing.s8)
    }

    /// A choice among many options, as a button showing the option chosen
    /// or the placeholder, whose popover lists the options holding what is
    /// typed into its field, one row each with its details under it, under
    /// filters chosen one per group and the count of those matching. The
    /// arrows move through the list and Return chooses. In television mode
    /// it is focusable, white with focus, and select opens it.
    final class SearchPicker: Box, Focusable, NSSearchFieldDelegate, NSPopoverDelegate {
        private let theme: Theme
        private var items: [PickerItem]
        private let filters: [[(key: String, label: String)]]
        private let placeholder: String
        private let limit: Int?
        private let popoverWidth: CGFloat
        private let bordered: Bool
        private let content = NSStackView.horizontal([], spacing: MavioSpacing.s8)
        private let chevron: IconView
        private var hovering = false
        private var tracking: NSTrackingArea?
        /// Called with the value chosen.
        var onChange: (@MainActor (String) -> Void)?
        /// The value chosen.
        var value: String? { didSet { showValue() } }

        // The popover's state.
        private var popover: NSPopover?
        private var query = ""
        private var chosenFilters: [Int: String] = [:]
        private var matching: [PickerItem] = []
        private var list: ListTable?
        private var countLabel: NSTextField?
        private var empty: NSTextField?
        private var search: NSSearchField?

        /// A picker of items, value chosen, width wide; filters groups of
        /// keys to tell the items apart by, limit how many the list shows at
        /// once; bordered on the surface, as the plugins' form's, else filled.
        init(
            items: [PickerItem], filters: [[(key: String, label: String)]] = [], value: String?, placeholder: String,
            width: CGFloat? = 288, popoverWidth: CGFloat = 420, limit: Int? = nil, bordered: Bool = false,
            theme: Theme, onChange: (@MainActor (String) -> Void)? = nil
        ) {
            self.theme = theme
            self.items = items
            self.filters = filters
            self.placeholder = placeholder
            self.limit = limit
            self.popoverWidth = popoverWidth
            self.bordered = bordered
            self.value = value
            self.onChange = onChange
            chevron = IconView(bordered ? "expand" : "selector", size: 14, color: theme.colors.textSecondary)
            super.init(radius: 6)
            let row = NSStackView.horizontal([content, Spacer(), chevron], spacing: MavioSpacing.s8)
            addFilling(row, insets: .symmetric(horizontal: 10, vertical: theme.isTV ? MavioSpacing.s8 : 4))
            heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 56 : 28).isActive = true
            if let width {
                let w = widthAnchor.constraint(equalToConstant: width)
                w.priority = .defaultHigh
                w.isActive = true
            }
            setAccessibilityElement(true)
            setAccessibilityRole(.popUpButton)
            setAccessibilityLabel(placeholder)
            showValue()
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Replaces the options.
        func setItems(_ items: [PickerItem]) {
            self.items = items
            showValue()
        }

        /// Whether the picker may be opened.
        var isEnabled = true { didSet { restyle() } }

        private func showValue() {
            content.setViews([])
            if let chosen = items.first(where: { $0.value == value }) {
                let label = pickerLabel(
                    PickerItem(
                        value: chosen.value, label: chosen.label, badges: chosen.badges.filter { $0.tone == .plain },
                        provider: chosen.provider, providerName: chosen.providerName),
                    style: theme.isTV ? theme.fonts.headline : theme.fonts.body, theme: theme)
                content.addArrangedSubview(label)
                setAccessibilityValue(chosen.label)
            } else {
                content.addArrangedSubview(
                    NSTextField.label(
                        placeholder, theme.isTV ? theme.fonts.headline : theme.fonts.body,
                        color: theme.colors.textTertiary))
                setAccessibilityValue(nil)
            }
            restyle()
        }

        private func restyle() {
            let focused = isFocused && theme.isTV
            if focused {
                fill = .white
                border = nil
            } else if bordered {
                fill = theme.colors.surface
                border = theme.colors.border
            } else {
                fill = hovering ? theme.colors.fillSelected : theme.colors.fill
                border = nil
            }
            for case let label as NSTextField in content.arrangedSubviews.flatMap({ [$0] + $0.subviews }) {
                if focused { label.textColor = .black }
            }
            chevron.contentTintColor = focused ? .black : theme.colors.textSecondary
            alphaValue = isEnabled ? 1 : 0.5
            if theme.isTV {
                NSAnimationContext.runAnimationGroup { context in
                    context.duration = MavioDuration.fast
                    context.allowsImplicitAnimation = true
                    layer?.transform = focused ? CATransform3DMakeScale(1.02, 1.02, 1) : CATransform3DIdentity
                }
            }
        }

        // MARK: Focus and pointer

        var isFocused = false {
            didSet {
                if oldValue && !isFocused { showValue() } else { restyle() }
                if isFocused { FocusEngine.active(for: self)?.scrollIntoView(self) }
            }
        }

        func activate() { open() }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { open() }
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

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override func accessibilityPerformPress() -> Bool {
            open()
            return true
        }

        // MARK: The popover

        /// Opens the list under the picker.
        func open() {
            guard isEnabled, popover == nil, window != nil else { return }
            query = ""
            let pop = NSPopover()
            pop.behavior = .transient
            pop.delegate = self
            let controller = NSViewController()
            controller.view = makeList()
            pop.contentViewController = controller
            popover = pop
            filter()
            pop.show(relativeTo: bounds, of: self, preferredEdge: .maxY)
            controller.view.window?.makeFirstResponder(search)
        }

        func popoverDidClose(_ notification: Notification) {
            popover = nil
            list = nil
            search = nil
        }

        private func makeList() -> NSView {
            let desk = Theme.desktop
            let field = NSSearchField()
            field.placeholderString = MavioStrings.navSearch
            field.delegate = self
            field.translatesAutoresizingMaskIntoConstraints = false
            search = field
            var views: [NSView] = [field]
            if !filters.isEmpty || limit != nil {
                var parts: [NSView] = []
                for (g, group) in filters.enumerated() {
                    let tabs = PillTabs([MavioStrings.schemaAll] + group.map(\.label), theme: desk)
                    tabs.onChange = { [weak self] i in
                        self?.chosenFilters[g] = i == 0 ? nil : group[i - 1].key
                        self?.filter()
                    }
                    parts.append(tabs)
                }
                parts.append(Spacer())
                let count = NSTextField.label("", desk.fonts.caption, color: desk.colors.textSecondary)
                countLabel = count
                parts.append(count)
                views.append(NSStackView.horizontal(parts, spacing: MavioSpacing.s12))
            }
            let none = NSTextField.label(
                MavioStrings.schemaNoMatches, desk.fonts.callout, color: desk.colors.textSecondary)
            empty = none
            views.append(none)
            let table = ListTable(maxHeight: 360, lines: false, selectable: true, theme: desk)
            table.onChoose = { [weak self] i in self?.choose(i) }
            list = table
            views.append(table)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            let holder = NSView()
            holder.addFilling(stack, insets: .all(MavioSpacing.s12))
            for v in views where v !== none { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            holder.widthAnchor.constraint(equalToConstant: popoverWidth).isActive = true
            return holder
        }

        private func filter() {
            let pool = items.filter { o in chosenFilters.values.allSatisfy { o.keys.contains($0) } }
            let all = pool.filter { $0.matches(query) }
            let shown = limit.map { Array(all.prefix($0)) } ?? all
            matching = shown
            if let limit, let countLabel {
                countLabel.stringValue =
                    all.count > limit
                    ? MavioStrings.schemaMatchesFirst(shown: String(limit), count: String(all.count))
                    : MavioStrings.schemaMatches(count: all.count)
            }
            empty?.isHidden = !shown.isEmpty
            list?.isHidden = shown.isEmpty
            let theme = Theme.desktop
            list?.show(
                count: shown.count,
                rowHeight: { [weak self] i in
                    guard let self, self.matching.indices.contains(i) else { return 28 }
                    return self.matching[i].detail.isEmpty ? 30 : 46
                },
                make: { [weak self] i in
                    guard let self, self.matching.indices.contains(i) else { return NSView() }
                    return self.row(self.matching[i], theme: theme)
                })
            if let at = shown.firstIndex(where: { $0.value == value }) { list?.select(at) }
        }

        private func row(_ o: PickerItem, theme: Theme) -> NSView {
            var lines: [NSView] = [pickerLabel(o, style: theme.fonts.callout, theme: theme)]
            if !o.detail.isEmpty {
                lines.append(NSTextField.label(o.detail, theme.fonts.caption, color: theme.colors.textSecondary))
            }
            let text = NSStackView.vertical(lines, spacing: 2)
            var views: [NSView] = [text, Spacer()]
            if o.value == value { views.append(IconView("check", size: 14, color: theme.colors.accent)) }
            let line = NSStackView.horizontal(views, spacing: MavioSpacing.s8)
            let holder = NSView()
            holder.addFilling(line, insets: .symmetric(horizontal: MavioSpacing.s8, vertical: 4))
            return holder
        }

        private func choose(_ index: Int) {
            guard matching.indices.contains(index) else { return }
            let chosen = matching[index].value
            popover?.close()
            value = chosen
            onChange?(chosen)
        }

        // MARK: The search field

        func controlTextDidChange(_ obj: Notification) {
            query = search?.stringValue ?? ""
            filter()
        }

        func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            guard let table = list?.table else { return false }
            switch selector {
            case #selector(NSResponder.moveDown(_:)):
                list?.select(min(matching.count - 1, table.selectedRow + 1))
                return true
            case #selector(NSResponder.moveUp(_:)):
                list?.select(max(0, table.selectedRow - 1))
                return true
            case #selector(NSResponder.insertNewline(_:)):
                choose(table.selectedRow >= 0 ? table.selectedRow : (matching.count == 1 ? 0 : -1))
                return true
            case #selector(NSResponder.cancelOperation(_:)):
                popover?.close()
                return true
            default:
                return false
            }
        }
    }
#endif
