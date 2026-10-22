#if os(macOS)
    import AppKit
    import MavioDesign

    /// The options of a long menu in a popover under it, as the web UI's
    /// selects of sixteen options or more: a field searching them by
    /// their names and the options holding what is typed, the one chosen
    /// checked. The arrows move through them, Return chooses and Escape
    /// closes.
    final class OptionSearch: NSViewController, NSSearchFieldDelegate, NSTableViewDataSource, NSTableViewDelegate {
        private let options: [String]
        private let chosen: Int
        private let onChoose: @MainActor (Int) -> Void
        /// The indexes of the options shown.
        private var shown: [Int] = []
        private let field = NSSearchField()
        private let table = NSTableView()
        private let scroll = NSScrollView()
        private let empty = NSTextField.label(
            MavioStrings.schemaNoMatches, Theme.desktop.fonts.callout, color: Theme.desktop.colors.textSecondary)

        /// The options, the one at chosen checked; onChoose is called with
        /// the index of the one chosen.
        init(options: [String], chosen: Int, onChoose: @escaping @MainActor (Int) -> Void) {
            self.options = options
            self.chosen = chosen
            self.onChoose = onChoose
            super.init(nibName: nil, bundle: nil)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The field to type into first.
        var searchField: NSSearchField { field }
        /// The popover showing the options, closed by Escape.
        weak var popover: NSPopover?

        override func loadView() {
            field.placeholderString = MavioStrings.navSearch
            field.delegate = self
            field.translatesAutoresizingMaskIntoConstraints = false
            let column = NSTableColumn(identifier: .init("option"))
            table.addTableColumn(column)
            table.headerView = nil
            table.style = .plain
            table.rowHeight = 24
            table.intercellSpacing = NSSize(width: 0, height: 0)
            table.backgroundColor = .clear
            table.dataSource = self
            table.delegate = self
            table.target = self
            table.action = #selector(clicked)
            scroll.documentView = table
            scroll.hasVerticalScroller = true
            scroll.drawsBackground = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            scroll.heightAnchor.constraint(equalToConstant: 320).isActive = true
            empty.isHidden = true
            empty.translatesAutoresizingMaskIntoConstraints = false
            scroll.addSubview(empty)
            NSLayoutConstraint.activate([
                empty.centerXAnchor.constraint(equalTo: scroll.centerXAnchor),
                empty.topAnchor.constraint(equalTo: scroll.topAnchor, constant: MavioSpacing.s16),
            ])
            let stack = NSStackView.vertical([field, scroll], spacing: MavioSpacing.s8)
            let view = NSView()
            view.addFilling(stack, insets: NSEdgeInsets(top: 10, left: 10, bottom: 10, right: 10))
            for v in [field, scroll] { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            view.widthAnchor.constraint(equalToConstant: 280).isActive = true
            self.view = view
            filter()
        }

        private func filter() {
            let query = field.stringValue.trimmingCharacters(in: .whitespaces)
            shown = options.indices.filter {
                query.isEmpty
                    || options[$0].range(of: query, options: [.caseInsensitive, .diacriticInsensitive]) != nil
            }
            table.reloadData()
            empty.isHidden = !shown.isEmpty
            if let at = shown.firstIndex(of: chosen), query.isEmpty {
                select(at)
            } else if !shown.isEmpty {
                select(0)
            }
        }

        private func select(_ row: Int) {
            guard shown.indices.contains(row) else { return }
            table.selectRowIndexes([row], byExtendingSelection: false)
            table.scrollRowToVisible(row)
        }

        private func choose(_ row: Int) {
            guard shown.indices.contains(row) else { return }
            onChoose(shown[row])
        }

        @objc private func clicked() { choose(table.clickedRow) }

        func numberOfRows(in tableView: NSTableView) -> Int { shown.count }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            let theme = Theme.desktop
            let index = shown[row]
            var views: [NSView] = [NSTextField.label(options[index], theme.fonts.body, color: theme.colors.textPrimary)]
            views.append(Spacer())
            if index == chosen { views.append(IconView("check", size: 14, color: theme.colors.accent)) }
            let line = NSStackView.horizontal(views, spacing: MavioSpacing.s8)
            let holder = NSView()
            holder.addFilling(line, insets: NSEdgeInsets(top: 0, left: 6, bottom: 0, right: 6))
            return holder
        }

        func controlTextDidChange(_ obj: Notification) { filter() }

        func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
            switch selector {
            case #selector(NSResponder.moveDown(_:)):
                select(min(shown.count - 1, table.selectedRow + 1))
                return true
            case #selector(NSResponder.moveUp(_:)):
                select(max(0, table.selectedRow - 1))
                return true
            case #selector(NSResponder.insertNewline(_:)):
                choose(table.selectedRow)
                return true
            case #selector(NSResponder.cancelOperation(_:)):
                popover?.performClose(nil)
                return true
            default:
                return false
            }
        }
    }
#endif
