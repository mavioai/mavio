#if os(macOS)
    import AppKit
    import MavioMac

    /// Logs: the server's recent log records at a level or above, from a
    /// period, filtered by words. Up to a thousand records are listed in a
    /// table on the card under the title, which makes the rows it shows
    /// alone and reuses them, so that reading them and filtering them never
    /// holds up the app; each shows again with what it read while it reads
    /// anew.
    final class LogsPage: ServerPage, NSTableViewDataSource, NSTableViewDelegate {
        private var records: [Mavio_System_V1_LogRecord]?
        private var shown: [Mavio_System_V1_LogRecord] = []
        private var failure: String?
        private var level = Mavio_System_V1_LogLevel.info
        private var hours = 24
        private var filter = ""
        private var loading: Task<Void, Never>?
        private let table = NSTableView()
        private lazy var scroll = NSScrollView()
        private lazy var status = NSStackView.vertical([], spacing: 0, alignment: .centerX)
        private lazy var count = NSTextField.label(
            "", theme.fonts.footnote, color: theme.colors.textSecondary)

        private static let levels: [Mavio_System_V1_LogLevel] = [.debug, .info, .warn, .error]
        private static let cell = NSUserInterfaceItemIdentifier("record")

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsLogs
            let levels = Self.levels
            let level = ActionPopUp(options: levels.map(Self.name), selected: levels.firstIndex(of: .info) ?? 0) {
                [weak self] i in
                self?.level = levels[i]
                self?.reload(clearing: true)
            }
            level.setAccessibilityLabel(MavioStrings.logsLevel)
            level.toolTip = MavioStrings.logsLevel
            let periods = [(MavioStrings.logsLastHour, 1), (MavioStrings.logsLastDay, 24), (MavioStrings.logsAll, 0)]
            let period = ActionPopUp(options: periods.map(\.0), selected: 1) { [weak self] i in
                self?.hours = periods[i].1
                self?.reload(clearing: true)
            }
            period.setAccessibilityLabel(MavioStrings.logsPeriod)
            period.toolTip = MavioStrings.logsPeriod
            let (search, _) = field("", placeholder: MavioStrings.logsFilter, width: 200) { [weak self] text in
                self?.filter = text
                self?.render()
            }
            let head = heading(
                MavioStrings.settingsLogs,
                actions: [search, FocusFrame(level, theme: theme), FocusFrame(period, theme: theme)])
            show(layout(head))
            render()
        }

        override func appeared() {
            reload(clearing: records == nil)
        }

        /// The title above the card and the count under it, in the column
        /// a form's sections take, the card taking the height left.
        private func layout(_ head: NSView) -> NSView {
            let column = TableColumn(identifier: .init("record"))
            table.addTableColumn(column)
            table.headerView = nil
            table.style = .plain
            table.backgroundColor = .clear
            table.usesAutomaticRowHeights = true
            table.intercellSpacing = .zero
            table.selectionHighlightStyle = .none
            table.columnAutoresizingStyle = .firstColumnOnlyAutoresizingStyle
            table.dataSource = self
            table.delegate = self
            table.setAccessibilityLabel(MavioStrings.settingsLogs)
            scroll.documentView = table
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            let card = Box(fill: theme.isTV ? nil : theme.colors.surface, radius: theme.isTV ? 0 : 14)
            card.layer?.masksToBounds = true
            card.addFilling(scroll)
            card.addSubview(status)
            NSLayoutConstraint.activate([
                status.leadingAnchor.constraint(equalTo: card.leadingAnchor),
                status.trailingAnchor.constraint(equalTo: card.trailingAnchor),
                status.topAnchor.constraint(equalTo: card.topAnchor),
            ])
            let stack = NSStackView.vertical([head, card, count], spacing: MavioSpacing.s16)
            stack.setCustomSpacing(MavioSpacing.s32, after: head)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(stack)
            let margin = theme.isTV ? theme.layout.margin : MavioSpacing.s48
            let vertical = theme.isTV ? MavioSpacing.s48 : MavioSpacing.s32
            NSLayoutConstraint.activate([
                stack.topAnchor.constraint(equalTo: holder.topAnchor, constant: vertical),
                stack.bottomAnchor.constraint(equalTo: holder.bottomAnchor, constant: -vertical),
                stack.centerXAnchor.constraint(equalTo: holder.centerXAnchor),
                stack.leadingAnchor.constraint(greaterThanOrEqualTo: holder.leadingAnchor, constant: margin),
                stack.widthAnchor.constraint(lessThanOrEqualToConstant: theme.isTV ? 1100 : 900),
                head.widthAnchor.constraint(equalTo: stack.widthAnchor),
                card.widthAnchor.constraint(equalTo: stack.widthAnchor),
            ])
            let fill = stack.leadingAnchor.constraint(equalTo: holder.leadingAnchor, constant: margin)
            fill.priority = .defaultHigh
            fill.isActive = true
            card.setContentHuggingPriority(.init(1), for: .vertical)
            return holder
        }

        /// Reads the records anew, showing those read until then unless
        /// clearing, as when the level or the period changes.
        private func reload(clearing: Bool) {
            if clearing {
                records = nil
                failure = nil
                render()
            }
            loading?.cancel()
            loading = Task { [weak self] in await self?.load() }
        }

        static func name(_ level: Mavio_System_V1_LogLevel) -> String {
            switch level {
            case .debug: MavioStrings.logsDebug
            case .warn: MavioStrings.logsWarn
            case .error: MavioStrings.logsError
            default: MavioStrings.logsInfo
            }
        }

        private func render() {
            let words = filter.lowercased().split(whereSeparator: \.isWhitespace)
            shown = (records ?? []).filter { r in
                guard !words.isEmpty else { return true }
                let text = ([r.message] + r.attributes.flatMap { [$0.key, $0.value] }).joined(separator: " ")
                    .lowercased()
                return words.allSatisfy { text.contains($0) }
            }
            table.reloadData()
            var notes: [NSView] = []
            if let failure, records == nil || shown.isEmpty {
                notes.append(note(failure, failed: true))
            } else if records != nil && shown.isEmpty {
                notes.append(note(MavioStrings.logsNone))
            } else if records == nil {
                notes.append(padded(LoadingView()))
            }
            status.setViews(notes)
            status.isHidden = notes.isEmpty
            scroll.isHidden = shown.isEmpty
            count.stringValue = records != nil ? MavioStrings.logsCount(count: shown.count) : ""
            refocus()
        }

        // MARK: Table

        func numberOfRows(in tableView: NSTableView) -> Int { shown.count }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            let cell =
                tableView.makeView(withIdentifier: Self.cell, owner: nil) as? RecordCell
                ?? RecordCell(theme: theme, identifier: Self.cell)
            cell.show(shown[row])
            return cell
        }

        func tableView(_ tableView: NSTableView, shouldSelectRow row: Int) -> Bool { false }

        private func load() async {
            var request = Mavio_System_V1_ListLogsRequest()
            request.minLevel = level
            request.limit = 1000
            if hours > 0 {
                request.maxAge = Google_Protobuf_Duration(seconds: Int64(hours * 3600))
            }
            do {
                records = try await model.call(
                    server, Procedure.system("ListLogs"), request, as: Mavio_System_V1_ListLogsResponse.self
                ).records
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        isolated deinit {
            loading?.cancel()
        }
    }

    /// The table's one column, as wide as the table.
    private final class TableColumn: NSTableColumn {
        override init(identifier: NSUserInterfaceItemIdentifier) {
            super.init(identifier: identifier)
            resizingMask = .autoresizingMask
        }

        @available(*, unavailable)
        required init(coder: NSCoder) { fatalError() }
    }

    /// A record's row: its time and level on the first line, its message,
    /// and its attributes, in one label that wraps and can be selected.
    private final class RecordCell: NSTableCellView {
        private let theme: Theme
        private let label = NSTextField(wrappingLabelWithString: "")
        private static let time = Date.FormatStyle(date: .numeric, time: .standard)

        init(theme: Theme, identifier: NSUserInterfaceItemIdentifier) {
            self.theme = theme
            super.init(frame: .zero)
            self.identifier = identifier
            label.isSelectable = true
            label.drawsBackground = false
            label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            let h: CGFloat = theme.isTV ? MavioSpacing.s24 : MavioSpacing.s16
            addFilling(label, insets: .symmetric(horizontal: h, vertical: 10))
            // A line between records, none under the empty space below.
            let line = Box(fill: .separatorColor)
            line.translatesAutoresizingMaskIntoConstraints = false
            addSubview(line)
            NSLayoutConstraint.activate([
                line.leadingAnchor.constraint(equalTo: leadingAnchor, constant: h),
                line.trailingAnchor.constraint(equalTo: trailingAnchor),
                line.bottomAnchor.constraint(equalTo: bottomAnchor),
                line.heightAnchor.constraint(equalToConstant: 1),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        func show(_ record: Mavio_System_V1_LogRecord) {
            let fonts = theme.fonts
            let colors = theme.colors
            let color: NSColor =
                switch record.level {
                case .debug: colors.textTertiary
                case .warn: NSColor(hex: 0xFF9F0A)
                case .error: colors.danger
                default: colors.textSecondary
                }
            let text = NSMutableAttributedString()
            let paragraph = NSMutableParagraphStyle()
            paragraph.paragraphSpacing = 2
            if let time = record.time.time {
                text.append(
                    NSAttributedString(
                        string: time.formatted(Self.time) + "  ",
                        attributes: [
                            .font: NSFont.monospacedDigitSystemFont(ofSize: fonts.footnote.size, weight: .regular),
                            .foregroundColor: colors.textSecondary,
                        ]))
            }
            text.append(
                NSAttributedString(
                    string: LogsPage.name(record.level) + "\n",
                    attributes: [
                        .font: NSFont.systemFont(ofSize: fonts.footnote.size, weight: .semibold),
                        .foregroundColor: color,
                    ]))
            text.append(
                NSAttributedString(
                    string: record.message,
                    attributes: [.font: fonts.callout.font, .foregroundColor: colors.textPrimary]))
            let attrs = record.attributes.sorted { $0.key < $1.key }
            if !attrs.isEmpty {
                text.append(
                    NSAttributedString(
                        string: "\n" + attrs.map { "\($0.key)=\($0.value)" }.joined(separator: "  "),
                        attributes: [
                            .font: NSFont.monospacedSystemFont(ofSize: fonts.caption.size, weight: .regular),
                            .foregroundColor: colors.textSecondary,
                        ]))
            }
            text.addAttribute(.paragraphStyle, value: paragraph, range: NSRange(location: 0, length: text.length))
            label.attributedStringValue = text
            setAccessibilityLabel(record.message)
        }
    }
#endif
