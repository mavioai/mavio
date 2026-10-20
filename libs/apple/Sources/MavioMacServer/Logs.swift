#if os(macOS)
    import AppKit
    import MavioMac

    /// Logs: the server's recent log records at a level or above, from a
    /// period, filtered by words.
    final class LogsPage: ServerPage {
        private var records: [Mavio_System_V1_LogRecord]?
        private var failure: String?
        private var level = Mavio_System_V1_LogLevel.info
        private var hours = 24
        private var filter = ""
        private var loading: Task<Void, Never>?
        private var started = false
        private lazy var form = FormPage(theme: theme)
        private lazy var list = section()

        private static let levels: [Mavio_System_V1_LogLevel] = [.debug, .info, .warn, .error]

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsLogs
            let levels = Self.levels
            let level = ActionPopUp(options: levels.map(Self.name), selected: levels.firstIndex(of: .info) ?? 0) {
                [weak self] i in
                self?.level = levels[i]
                self?.reload()
            }
            level.setAccessibilityLabel(MavioStrings.logsLevel)
            level.toolTip = MavioStrings.logsLevel
            let periods = [(MavioStrings.logsLastHour, 1), (MavioStrings.logsLastDay, 24), (MavioStrings.logsAll, 0)]
            let period = ActionPopUp(options: periods.map(\.0), selected: 1) { [weak self] i in
                self?.hours = periods[i].1
                self?.reload()
            }
            period.setAccessibilityLabel(MavioStrings.logsPeriod)
            period.toolTip = MavioStrings.logsPeriod
            let (search, _) = field("", placeholder: MavioStrings.logsFilter, width: 200) { [weak self] text in
                self?.filter = text
                self?.render()
            }
            form.add(
                heading(
                    MavioStrings.settingsLogs,
                    actions: [search, FocusFrame(level, theme: theme), FocusFrame(period, theme: theme)]))
            form.add(list)
            render()
            show(form)
        }

        override func appeared() {
            guard !started else { return }
            started = true
            reload()
        }

        private func reload() {
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
            let shown = (records ?? []).filter { r in
                let text = ([r.message] + r.attributes.flatMap { [$0.key, $0.value] }).joined(separator: " ")
                    .lowercased()
                return words.allSatisfy { text.contains($0) }
            }
            var rows: [NSView] = shown.map(row)
            if let failure {
                rows.append(note(failure, failed: true))
            } else if records != nil && shown.isEmpty {
                rows.append(note(MavioStrings.logsNone))
            } else if records == nil {
                rows.append(padded(LoadingView()))
            }
            list.setRows(rows)
            list.footer = records != nil ? MavioStrings.logsCount(count: shown.count) : nil
            refocus()
        }

        private func row(_ record: Mavio_System_V1_LogRecord) -> NSView {
            let color: NSColor =
                switch record.level {
                case .debug: theme.colors.textTertiary
                case .warn: NSColor(hex: 0xFF9F0A)
                case .error: theme.colors.danger
                default: theme.colors.textSecondary
                }
            var head: [NSView] = []
            if let time = record.time.time {
                let label = NSTextField.label(
                    time.formatted(date: .numeric, time: .standard), theme.fonts.footnote,
                    color: theme.colors.textSecondary)
                label.font = .monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
                head.append(label)
            }
            let level = NSTextField.label(Self.name(record.level), theme.fonts.footnote, color: color)
            level.font = .systemFont(ofSize: theme.fonts.footnote.size, weight: .semibold)
            head.append(level)
            var lines: [NSView] = [
                NSStackView.horizontal(head, spacing: MavioSpacing.s8),
                NSTextField.label(
                    record.message, theme.fonts.callout, color: theme.colors.textPrimary, lines: 0, selectable: true),
            ]
            let attrs = record.attributes.sorted { $0.key < $1.key }
            if !attrs.isEmpty {
                let label = NSTextField.label(
                    attrs.map { "\($0.key)=\($0.value)" }.joined(separator: "  "), theme.fonts.caption,
                    color: theme.colors.textSecondary, lines: 0, selectable: true)
                label.font = .monospacedSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
                lines.append(label)
            }
            return padded(NSStackView.vertical(lines, spacing: 2))
        }

        private func load() async {
            records = nil
            failure = nil
            render()
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
#endif
