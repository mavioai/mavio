#if os(macOS)
    import AppKit
    import MavioMac

    /// The releases shown on a page.
    private let pageSize = 50

    /// The types of a search.
    @MainActor private var searchTypes: [(type: Mavio_Acquisition_V1_ReleaseSearchType, label: String)] {
        [
            (.unspecified, MavioStrings.indexersTypeAny), (.movie, MavioStrings.indexersTypeMovie),
            (.tv, MavioStrings.indexersTypeTv),
        ]
    }

    /// Newznab's top categories, by their first number.
    @MainActor private var categories: [(code: Int32, label: String)] {
        [
            (1000, MavioStrings.indexersCategoryConsole), (2000, MavioStrings.indexersCategoryMovies),
            (3000, MavioStrings.indexersCategoryAudio), (4000, MavioStrings.indexersCategoryPc),
            (5000, MavioStrings.indexersCategoryTv), (6000, MavioStrings.indexersCategoryXxx),
            (7000, MavioStrings.indexersCategoryBooks), (8000, MavioStrings.indexersCategoryOther),
        ]
    }

    /// The top categories of a release's categories, by name.
    @MainActor private func categoryNames(_ list: [Int32]) -> String {
        var tops: [Int32] = []
        for c in list.map({ $0 / 1000 * 1000 }) where !tops.contains(c) { tops.append(c) }
        let all = categories
        return tops.compactMap { c in all.first { $0.code == c }?.label }.joined(separator: ", ")
    }

    /// How long ago a release was published: in days from a day, as Prowlarr tells.
    private func ageOf(_ r: Mavio_Acquisition_V1_Release) -> String {
        guard r.hasPublishDate else { return "" }
        let seconds = max(0, Date().timeIntervalSince(r.publishDate.date).rounded())
        let day: TimeInterval = 86_400
        return lengthOf(seconds >= day ? (seconds / day).rounded(.down) * day : seconds)
    }

    /// What the releases are sorted by: a column of the table.
    private enum SortKey: Equatable {
        case age, title, indexer, size, grabs, peers
    }

    /// How the releases are sorted: by a column, either way.
    private struct Sort: Equatable {
        var key: SortKey
        var desc: Bool
    }

    /// The releases sorted as the table's heading chose.
    private func sortedBy(_ list: [Mavio_Acquisition_V1_Release], _ sort: Sort) -> [Mavio_Acquisition_V1_Release] {
        func ordered(_ a: Mavio_Acquisition_V1_Release, _ b: Mavio_Acquisition_V1_Release) -> ComparisonResult {
            func number(_ x: Int64, _ y: Int64) -> ComparisonResult {
                x < y ? .orderedAscending : x > y ? .orderedDescending : .orderedSame
            }
            switch sort.key {
            case .age:
                return number(
                    -(a.hasPublishDate ? a.publishDate.seconds : 0), -(b.hasPublishDate ? b.publishDate.seconds : 0))
            case .title: return a.title.lowercased().localizedCompare(b.title.lowercased())
            case .indexer: return a.indexerName.lowercased().localizedCompare(b.indexerName.lowercased())
            case .size: return number(a.size, b.size)
            case .grabs: return number(Int64(a.grabs), Int64(b.grabs))
            case .peers: return number(Int64(a.seeders), Int64(b.seeders))
            }
        }
        return list.sorted { a, b in
            let c = ordered(a, b)
            return sort.desc ? c == .orderedDescending : c == .orderedAscending
        }
    }

    extension Mavio_Acquisition_V1_Release {
        /// The release's key among a search's: its indexer's and its own.
        fileprivate var key: String { "\(pluginID)/\(indexerID)/\(guid)" }
    }

    /// A column of the releases' table: sorted by its key, when it has one;
    /// the title takes what the others leave.
    private struct Column {
        let label: String
        var key: SortKey?
        var width: CGFloat?
        var trailing = false
    }

    /// The columns of the releases' table.
    @MainActor private var columns: [Column] {
        [
            Column(label: MavioStrings.indexersProtocol, width: 64),
            Column(label: MavioStrings.indexersAge, key: .age, width: 72),
            Column(label: MavioStrings.indexersReleaseTitle, key: .title),
            Column(label: MavioStrings.indexersIndexer, key: .indexer, width: 104),
            Column(label: MavioStrings.indexersSize, key: .size, width: 72, trailing: true),
            Column(label: MavioStrings.indexersGrabs, key: .grabs, width: 48, trailing: true),
            Column(label: MavioStrings.indexersPeers, key: .peers, width: 64, trailing: true),
            Column(label: MavioStrings.indexersCategory, width: 96),
            Column(label: "", width: 88),
        ]
    }

    /// The indexer search, an advanced tool (docs/clients.md §6.3.21), as
    /// the web UI's: a search of the server's indexers by text, as
    /// Prowlarr's, whose releases are sent to a download client without a
    /// library item; a table of them sorted by its headings, page by page,
    /// or lines of them when the window is narrow.
    final class IndexerSearchPage: FormController {
        /// A search as it was submitted.
        private struct Search: Equatable {
            var query: String
            var type: Mavio_Acquisition_V1_ReleaseSearchType
            var categories: [Int32]
            var indexers: [Pair]

            /// An indexer searched: its plugin's ID and its own.
            struct Pair: Equatable {
                var pluginId: String
                var indexerId: String
            }
        }

        /// What a release's actions told.
        private struct Told {
            var copied = false
            var sending = false
            /// The download client the release was sent to.
            var sent: String?
            var failure: String?
        }

        private var query = ""
        private var type = Mavio_Acquisition_V1_ReleaseSearchType.unspecified
        /// The Newznab top category searched; 0 for all.
        private var category: Int32 = 0
        /// The indexer searched, as plugin/indexer; "all" for every one.
        private var indexer = "all"
        private var indexers: [Mavio_Acquisition_V1_Indexer] = []
        private var search: Search?
        private var sort: Sort?
        private var response: Mavio_Acquisition_V1_SearchReleasesResponse?
        private var fetching = false
        private var failure: String?
        private var page = 0
        private var told: [String: Told] = [:]
        private var searching: Task<Void, Never>?
        private var compact = false
        private lazy var queryInput: TextInput = {
            let input = TextInput("", placeholder: MavioStrings.indexersQuery)
            input.widthAnchor.constraint(greaterThanOrEqualToConstant: 160).isActive = true
            input.setContentHuggingPriority(.init(1), for: .horizontal)
            input.onChange = { [weak self] text in
                self?.query = text
                self?.searchButton.isEnabled = !(self?.trimmed.isEmpty ?? true)
            }
            input.onSubmit = { [weak self] in self?.submit() }
            return input
        }()
        private lazy var searchButton: MavioButton = {
            let button = MavioButton(MavioStrings.indexersSearch, kind: .primary, theme: theme) { [weak self] in
                self?.submit()
            }
            button.isEnabled = false
            return button
        }()

        override func viewDidLoad() {
            title = MavioStrings.indexersTitle
            message = MavioStrings.indexersMessage
            compact = isCompact
            super.viewDidLoad()
            run { [weak self] in await self?.loadIndexers() }
        }

        isolated deinit { searching?.cancel() }

        /// Whether the page is too narrow for the table, or on a television:
        /// a release then shows as a line, as on a phone.
        private var isCompact: Bool {
            theme.isTV || min(view.bounds.width - 2 * MavioSpacing.s48, 900) - 2 * MavioSpacing.s16 < 760
        }

        override func viewDidLayout() {
            super.viewDidLayout()
            let now = isCompact
            if now != compact {
                compact = now
                render()
            }
        }

        override func sections() -> [NSView] {
            var views: [NSView] = [plain(form())]
            if search == nil {
                views.append(plain(label(MavioStrings.indexersPrompt, theme.colors.textSecondary)))
            } else {
                views.append(results())
                if pages > 1 { views.append(plain(pager())) }
            }
            return views
        }

        // MARK: The search

        /// The search's field, its menus of type, category and indexer, and
        /// Search.
        private func form() -> NSView {
            let types = searchTypes
            let typeMenu = PopUp(
                types.map(\.label), selected: types.firstIndex { $0.type == type } ?? 0,
                label: MavioStrings.indexersType
            ) { [weak self] i in self?.type = types[i].type }
            let cats: [(code: Int32, label: String)] = [(0, MavioStrings.indexersAllCategories)] + categories
            let categoryMenu = PopUp(
                cats.map(\.label), selected: cats.firstIndex { $0.code == category } ?? 0,
                label: MavioStrings.indexersCategory
            ) { [weak self] i in self?.category = cats[i].code }
            let options: [(key: String, name: String)] =
                [("all", MavioStrings.indexersAllIndexers)] + indexers.map { ("\($0.pluginID)/\($0.id)", $0.name) }
            let indexerMenu = PopUp(
                options.map(\.name), selected: options.firstIndex { $0.key == indexer } ?? 0,
                label: MavioStrings.indexersIndexer
            ) { [weak self] i in self?.indexer = options[i].key }
            let views: [NSView] = [queryInput, typeMenu, categoryMenu, indexerMenu, searchButton]
            if compact && !theme.isTV {
                // Narrow: the field above its menus.
                let menus = NSStackView.horizontal(
                    [typeMenu, categoryMenu, indexerMenu, Spacer(), searchButton], spacing: MavioSpacing.s8)
                let stack = NSStackView.vertical([queryInput, menus], spacing: MavioSpacing.s8)
                for view in [queryInput, menus] {
                    view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                }
                return stack
            }
            return NSStackView.horizontal(views, spacing: MavioSpacing.s12)
        }

        /// The text searched for, without the spaces around it.
        private var trimmed: String { query.trimmingCharacters(in: .whitespacesAndNewlines) }

        /// Searches for what the form holds, unless it was just searched.
        private func submit() {
            guard !trimmed.isEmpty else { return }
            let parts = indexer == "all" ? [] : indexer.split(separator: "/", maxSplits: 1).map(String.init)
            let next = Search(
                query: trimmed, type: type, categories: category == 0 ? [] : [category],
                indexers: parts.count == 2 ? [.init(pluginId: parts[0], indexerId: parts[1])] : [])
            guard next != search else { return }
            // A new search starts again at its first page, forgetting what
            // the last one's actions told.
            page = 0
            told = [:]
            search = next
            start(next)
        }

        /// Loads the enabled indexers, the choices of the indexer's menu.
        private func loadIndexers() async {
            let list = try? await model.call(
                server, Procedure.releases("ListIndexers"), Mavio_Acquisition_V1_ListIndexersRequest(),
                as: Mavio_Acquisition_V1_ListIndexersResponse.self)
            guard !Task.isCancelled else { return }
            indexers = (list?.indexers ?? []).filter(\.enabled)
            render()
        }

        /// Runs a search, forgetting the last one's releases.
        private func start(_ search: Search) {
            searching?.cancel()
            fetching = true
            failure = nil
            response = nil
            render()
            var request = Mavio_Acquisition_V1_SearchReleasesRequest()
            request.query = search.query
            request.type = search.type
            request.categories = search.categories
            request.indexers = search.indexers.map {
                var ref = Mavio_Acquisition_V1_IndexerRef()
                ref.pluginID = $0.pluginId
                ref.indexerID = $0.indexerId
                return ref
            }
            searching = Task { [weak self, model, server] in
                do {
                    let response = try await model.call(
                        server, Procedure.releases("SearchReleases"), request,
                        as: Mavio_Acquisition_V1_SearchReleasesResponse.self)
                    guard let self, !Task.isCancelled else { return }
                    self.response = response
                } catch is CancellationError {
                    return
                } catch {
                    guard let self, !Task.isCancelled else { return }
                    self.failure = error.reason
                }
                self?.fetching = false
                self?.render()
            }
        }

        // MARK: The releases

        /// The releases found, as the server sent them.
        private var all: [Mavio_Acquisition_V1_Release] { response?.releases ?? [] }
        /// The releases found, sorted as chosen.
        private var sorted: [Mavio_Acquisition_V1_Release] { sort.map { sortedBy(all, $0) } ?? all }
        /// The pages of the releases, at least one.
        private var pages: Int { max(1, (all.count + pageSize - 1) / pageSize) }

        /// The releases of the page shown.
        private var shown: ArraySlice<Mavio_Acquisition_V1_Release> {
            let list = sorted
            let start = min(page * pageSize, list.count)
            return list[start..<min(start + pageSize, list.count)]
        }

        /// The card of the releases: the indexers that failed, what the
        /// search lacks, and the releases of the page.
        private func results() -> NSView {
            var rows: [NSView] = (response?.failures ?? []).map { f in
                row(
                    NSTextField.label(
                        MavioStrings.acquireIndexerFailed(indexer: indexerName(f), message: f.message),
                        theme.fonts.footnote, color: theme.colors.danger, lines: 0, selectable: true))
            }
            if fetching && all.isEmpty { rows.append(note(MavioStrings.acquireSearchingIndexers)) }
            if let failure { rows.append(note(failure, failed: true)) }
            if response != nil && all.isEmpty { rows.append(note(MavioStrings.acquireNoReleases)) }
            let shown = shown
            if !shown.isEmpty {
                if compact {
                    rows += shown.map(item)
                } else {
                    rows.append(row(header()))
                    rows += shown.map(line)
                }
            }
            return card(rows)
        }

        /// The page shown of how many, and the buttons turning it.
        private func pager() -> NSView {
            let back = IconAction("back", label: MavioStrings.indexersPrevious, size: 14, theme: theme, action: nil)
            if page > 0 {
                back.onPress = { [weak self] in
                    self?.page -= 1
                    self?.turned()
                }
            }
            let forward = IconAction("forward", label: MavioStrings.indexersNext, size: 14, theme: theme, action: nil)
            if page + 1 < pages {
                forward.onPress = { [weak self] in
                    self?.page += 1
                    self?.turned()
                }
            }
            for button in [back, forward] { button.color = theme.colors.textPrimary }
            return NSStackView.horizontal(
                [
                    Spacer(),
                    NSTextField.label(
                        MavioStrings.indexersPage(page: String(page + 1), pages: String(pages)), theme.fonts.callout,
                        color: theme.colors.textSecondary),
                    back, forward,
                ], spacing: MavioSpacing.s8)
        }

        /// Shows the page turned to from its top.
        private func turned() {
            render()
            form.page.scrollToTop()
        }

        /// The name of an indexer that failed, by its releases, else its ID.
        private func indexerName(_ f: Mavio_Acquisition_V1_IndexerFailure) -> String {
            let name = all.first { $0.pluginID == f.pluginID && $0.indexerID == f.indexerID }?.indexerName ?? ""
            return !name.isEmpty ? name : !f.indexerID.isEmpty ? f.indexerID : f.pluginID
        }

        // MARK: The table

        /// A view in a column of the table.
        private func cell(_ view: NSView, _ column: Column) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(view)
            view.translatesAutoresizingMaskIntoConstraints = false
            var constraints = [
                view.centerYAnchor.constraint(equalTo: holder.centerYAnchor),
                view.topAnchor.constraint(greaterThanOrEqualTo: holder.topAnchor),
                view.leadingAnchor.constraint(greaterThanOrEqualTo: holder.leadingAnchor),
                view.trailingAnchor.constraint(lessThanOrEqualTo: holder.trailingAnchor),
                column.trailing
                    ? view.trailingAnchor.constraint(equalTo: holder.trailingAnchor)
                    : view.leadingAnchor.constraint(equalTo: holder.leadingAnchor),
            ]
            if let width = column.width {
                constraints.append(holder.widthAnchor.constraint(equalToConstant: width))
            } else {
                constraints.append(holder.widthAnchor.constraint(greaterThanOrEqualToConstant: 120))
                constraints.append(view.trailingAnchor.constraint(equalTo: holder.trailingAnchor))
                holder.setContentHuggingPriority(.init(1), for: .horizontal)
            }
            NSLayoutConstraint.activate(constraints)
            return holder
        }

        /// A label of a cell in the footnote's type, truncated on a line.
        private func text(_ s: String, _ color: NSColor? = nil, lines: Int = 1) -> NSTextField {
            let field = NSTextField.label(
                s, theme.fonts.footnote, color: color ?? theme.colors.textPrimary, lines: lines)
            if lines == 1 { field.lineBreakMode = .byTruncatingTail }
            field.setContentCompressionResistancePriority(.init(250), for: .horizontal)
            return field
        }

        /// The headings of the table: a column's heading sorts by it.
        private func header() -> NSView {
            let cols = columns
            let cells: [NSView] = cols.map { column in
                let title = NSTextField.label(column.label, theme.fonts.footnote, color: theme.colors.textSecondary)
                title.font = NSFont.systemFont(ofSize: theme.fonts.footnote.size, weight: .medium)
                guard let key = column.key else { return cell(title, column) }
                var views: [NSView] = [title]
                if let sort, sort.key == key {
                    let arrow = IconView("expand", size: 12)
                    arrow.contentTintColor = theme.colors.textSecondary
                    arrow.wantsLayer = true
                    arrow.frameCenterRotation = sort.desc ? 0 : 180
                    views.append(arrow)
                }
                let heading = SortHeading(NSStackView.horizontal(views, spacing: MavioSpacing.s4), label: column.label)
                {
                    [weak self] in
                    guard let self else { return }
                    self.sort = Sort(
                        key: key,
                        desc: self.sort?.key == key
                            ? !(self.sort?.desc ?? false) : [.size, .grabs, .peers].contains(key))
                    self.render()
                }
                return cell(heading, column)
            }
            return NSStackView.horizontal(cells, spacing: MavioSpacing.s8)
        }

        /// A release as a line of the table, as Prowlarr's search lists them.
        private func line(_ r: Mavio_Acquisition_V1_Release) -> NSView {
            let cols = columns
            let torrent = r.protocol == .torrent
            let cells: [NSView] = [
                cell(protocolBadge(torrent), cols[0]),
                cell(text(ageOf(r), theme.colors.textSecondary), cols[1]),
                cell(text(r.title, lines: 0), cols[2]),
                cell(text(r.indexerName, theme.colors.textSecondary), cols[3]),
                cell(text(r.size > 0 ? acquisitionSize(r.size) : ""), cols[4]),
                cell(text(r.grabs > 0 ? String(r.grabs) : "", theme.colors.textSecondary), cols[5]),
                cell(text(torrent ? "\(r.seeders) / \(r.peers)" : ""), cols[6]),
                cell(text(categoryNames(r.categories), theme.colors.textSecondary, lines: 0), cols[7]),
                cell(actions(r), cols[8]),
            ]
            return row(NSStackView.horizontal(cells, spacing: MavioSpacing.s8))
        }

        /// A release in a narrow window: its title, and what the table's
        /// columns would tell, in a line under it.
        private func item(_ r: Mavio_Acquisition_V1_Release) -> NSView {
            let torrent = r.protocol == .torrent
            let detail = dotted([
                torrent ? MavioStrings.indexersTorrent : MavioStrings.acquireUsenet,
                ageOf(r),
                r.indexerName,
                r.size > 0 ? acquisitionSize(r.size) : nil,
                torrent ? MavioStrings.acquirePeers(seeders: String(r.seeders), peers: String(r.peers)) : nil,
                categoryNames(r.categories),
            ])
            let stack = NSStackView.vertical(
                [
                    NSTextField.label(r.title, theme.fonts.callout, color: theme.colors.textPrimary, lines: 0),
                    NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                ], spacing: 2)
            stack.setContentHuggingPriority(.init(1), for: .horizontal)
            return row(NSStackView.horizontal([stack, actions(r)], spacing: MavioSpacing.s12))
        }

        /// A release's protocol, on a badge.
        private func protocolBadge(_ torrent: Bool) -> NSView {
            let box = Box(fill: torrent ? theme.colors.fillSelected : theme.colors.fill, radius: 4)
            let label = NSTextField.label(
                torrent ? MavioStrings.indexersTorrent : MavioStrings.acquireUsenet, theme.fonts.caption,
                color: torrent ? theme.colors.textPrimary : theme.colors.textSecondary)
            label.font = NSFont.systemFont(ofSize: theme.fonts.caption.size, weight: .medium)
            box.addFilling(label, insets: .symmetric(horizontal: 6, vertical: 2))
            return box
        }

        /// Sending a release to a download client, copying its magnet link,
        /// and its page on the indexer, in the browser.
        private func actions(_ r: Mavio_Acquisition_V1_Release) -> NSView {
            let key = r.key
            let state = told[key] ?? Told()
            var views: [NSView] = []
            // A television has no browser to open the page in.
            if !theme.isTV, !r.infoURL.isEmpty, let url = URL(string: r.infoURL) {
                views.append(
                    IconAction("external", label: MavioStrings.indexersInfo, theme: theme) {
                        NSWorkspace.shared.open(url)
                    })
            }
            if !r.magnetURL.isEmpty {
                let magnet = r.magnetURL
                views.append(
                    IconAction(
                        state.copied ? "check" : "magnet",
                        label: state.copied ? MavioStrings.indexersMagnetCopied : MavioStrings.indexersMagnet,
                        color: state.copied ? theme.colors.success : theme.colors.textSecondary, theme: theme
                    ) { [weak self] in
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(magnet, forType: .string)
                        self?.told[key, default: Told()].copied = true
                        self?.render()
                    })
            }
            let send = IconAction(
                state.sent != nil ? "check" : state.failure != nil ? "error" : "downloads",
                label: state.sent.map { MavioStrings.indexersSent(client: $0) } ?? state.failure
                    ?? MavioStrings.indexersSend,
                color: state.failure != nil
                    ? theme.colors.danger : state.sent != nil ? theme.colors.success : theme.colors.accentText,
                theme: theme, action: nil)
            if !state.sending && state.sent == nil { send.onPress = { [weak self] in self?.send(r) } }
            views.append(send)
            let stack = NSStackView.horizontal(views, spacing: 2)
            stack.setContentHuggingPriority(.required, for: .horizontal)
            return stack
        }

        /// Sends a release to its download client, telling which.
        private func send(_ release: Mavio_Acquisition_V1_Release) {
            let key = release.key
            told[key, default: Told()].sending = true
            told[key]?.failure = nil
            render()
            var request = Mavio_Acquisition_V1_GrabReleaseRequest()
            request.release = release
            run { [weak self, model, server] in
                do {
                    let client = try await model.call(
                        server, Procedure.downloads("GrabRelease"), request,
                        as: Mavio_Acquisition_V1_GrabReleaseResponse.self
                    ).clientName
                    self?.told[key]?.sent = client
                } catch {
                    self?.told[key]?.failure = error.reason
                }
                self?.told[key]?.sending = false
                self?.render()
            }
        }
    }

    /// A heading of the table that sorts by its column when clicked.
    private final class SortHeading: NSView {
        private let action: @MainActor () -> Void

        /// A heading of content, labelled for accessibility, running action
        /// when clicked.
        init(_ content: NSView, label: String, action: @escaping @MainActor () -> Void) {
            self.action = action
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(content)
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(label)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            guard bounds.contains(convert(event.locationInWindow, from: nil)) else { return }
            action()
        }

        override func accessibilityPerformPress() -> Bool {
            action()
            return true
        }

        override func resetCursorRects() {
            addCursorRect(bounds, cursor: .pointingHand)
        }
    }
#endif
