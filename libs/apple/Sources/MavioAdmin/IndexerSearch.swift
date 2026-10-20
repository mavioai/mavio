#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftProtobuf
    import SwiftUI

    /// The indexer search, an advanced tool (docs/clients.md §6.3.21), as
    /// the web UI's: a search of the server's indexers by text, as Prowlarr's,
    /// whose releases are sent to a download client without a library item.
    struct IndexerSearchView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var query = ""
        @State private var type = Mavio_Acquisition_V1_ReleaseSearchType.unspecified
        /// The Newznab top category searched; 0 for all.
        @State private var category: Int32 = 0
        /// The indexer searched, as plugin/indexer; "all" for every one.
        @State private var indexer = "all"
        @State private var indexers: [Mavio_Acquisition_V1_Indexer] = []
        @State private var search: Search?
        /// Each search's number, so that what a release's actions told is
        /// forgotten by the next one.
        @State private var generation = 0
        @State private var sort: Sort?
        @State private var response: Mavio_Acquisition_V1_SearchReleasesResponse?
        @State private var fetching = false
        @State private var failure: String?
        @State private var page = 0
        @State private var width: CGFloat = 0

        /// A search as it was submitted.
        struct Search: Equatable {
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

        /// How the releases are sorted: by a column, either way.
        struct Sort: Equatable {
            var key: IndexerSearchSortKey
            var desc: Bool
        }

        var body: some View {
            SettingsList(message: MavioStrings.indexersMessage) {
                Section {
                } header: {
                    form
                } footer: {
                    if search == nil { Text(MavioStrings.indexersPrompt).type(theme.type.callout) }
                }
                if search != nil { results }
            }
            .onGeometryChange(for: CGFloat.self, of: { $0.size.width }, action: { width = $0 })
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.indexersTitle)
            .inlineTitle()
            .task { await loadIndexers() }
            .task(id: search) {
                guard let search else { return }
                await run(search)
            }
        }

        // MARK: The search

        private var form: some View {
            HStack(spacing: MavioSpacing.s12) {
                TextField(MavioStrings.indexersQuery, text: $query, prompt: Text(MavioStrings.indexersQuery))
                    .fieldLabelHidden()
                    .textFieldStyle(.roundedBorder)
                    .frame(minWidth: 160)
                    .onSubmit(submit)
                Picker(MavioStrings.indexersType, selection: $type) {
                    ForEach(searchTypes, id: \.type) { Text($0.label).tag($0.type) }
                }
                .labelsHidden()
                .fixedSize()
                .help(MavioStrings.indexersType)
                Picker(MavioStrings.indexersCategory, selection: $category) {
                    Text(MavioStrings.indexersAllCategories).tag(Int32(0))
                    ForEach(categories, id: \.code) { Text($0.label).tag($0.code) }
                }
                .labelsHidden()
                .fixedSize()
                .help(MavioStrings.indexersCategory)
                Picker(MavioStrings.indexersIndexer, selection: $indexer) {
                    Text(MavioStrings.indexersAllIndexers).tag("all")
                    ForEach(indexers, id: \.key) { Text($0.name).tag($0.key) }
                }
                .labelsHidden()
                .fixedSize()
                .help(MavioStrings.indexersIndexer)
                Button(MavioStrings.indexersSearch, action: submit)
                    .buttonStyle(.borderedProminent)
                    .disabled(trimmed.isEmpty)
            }
            .type(theme.type.callout)
            .foregroundStyle(theme.colors.textPrimary)
            .textCase(nil)
            .padding(.horizontal, -MavioSpacing.s4)
        }

        private var trimmed: String { query.trimmingCharacters(in: .whitespacesAndNewlines) }

        private func submit() {
            guard !trimmed.isEmpty else { return }
            let parts = indexer == "all" ? [] : indexer.split(separator: "/", maxSplits: 1).map(String.init)
            let next = Search(
                query: trimmed, type: type, categories: category == 0 ? [] : [category],
                indexers: parts.count == 2 ? [.init(pluginId: parts[0], indexerId: parts[1])] : [])
            if next != search {
                // A new search starts again at its first page.
                generation += 1
                page = 0
                search = next
            }
        }

        private func loadIndexers() async {
            let list = try? await model.call(
                server, Procedure.releases("ListIndexers"), Mavio_Acquisition_V1_ListIndexersRequest(),
                as: Mavio_Acquisition_V1_ListIndexersResponse.self)
            indexers = (list?.indexers ?? []).filter(\.enabled)
        }

        private func run(_ search: Search) async {
            fetching = true
            failure = nil
            response = nil
            defer { fetching = false }
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
            do {
                response = try await model.call(
                    server, Procedure.releases("SearchReleases"), request,
                    as: Mavio_Acquisition_V1_SearchReleasesResponse.self)
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }

        // MARK: The releases

        private var all: [Mavio_Acquisition_V1_Release] { response?.releases ?? [] }
        private var sorted: [Mavio_Acquisition_V1_Release] { sort.map { sortedBy(all, $0) } ?? all }
        private var pages: Int { max(1, (sorted.count + pageSize - 1) / pageSize) }
        private var shown: ArraySlice<Mavio_Acquisition_V1_Release> {
            let list = sorted
            let start = min(page * pageSize, list.count)
            return list[start..<min(start + pageSize, list.count)]
        }
        /// Whether the card is too narrow for the table: a release then
        /// shows as a line, as on a phone.
        private var compact: Bool { min(width - 2 * MavioSpacing.s48, 900) - 2 * MavioSpacing.s16 < 760 }

        private var results: some View {
            Section {
                ForEach(response?.failures ?? [], id: \.self) { f in
                    Text(MavioStrings.acquireIndexerFailed(indexer: indexerName(f), message: f.message))
                        .type(theme.type.footnote)
                        .foregroundStyle(theme.colors.danger)
                }
                if fetching && all.isEmpty {
                    Text(MavioStrings.acquireSearchingIndexers)
                        .type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                }
                if response != nil && all.isEmpty {
                    Text(MavioStrings.acquireNoReleases)
                        .type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                }
                if !shown.isEmpty {
                    if compact {
                        ForEach(shown, id: \.key) { r in
                            IndexerSearchItem(server: server, release: r).id("\(generation)/\(r.key)")
                        }
                    } else {
                        IndexerSearchHeader(sort: $sort)
                        ForEach(shown, id: \.key) { r in
                            IndexerSearchLine(server: server, release: r).id("\(generation)/\(r.key)")
                        }
                    }
                }
            } footer: {
                if pages > 1 { pager }
            }
        }

        private var pager: some View {
            HStack(spacing: MavioSpacing.s8) {
                Spacer(minLength: 0)
                Text(MavioStrings.indexersPage(page: String(page + 1), pages: String(pages)))
                pageButton("back", MavioStrings.indexersPrevious, disabled: page == 0) { page -= 1 }
                pageButton("forward", MavioStrings.indexersNext, disabled: page + 1 >= pages) { page += 1 }
            }
        }

        private func pageButton(
            _ icon: String, _ label: String, disabled: Bool, action: @escaping () -> Void
        ) -> some View {
            Button(action: action) {
                Icon(icon, size: 14).frame(width: 28, height: 28).contentShape(.rect)
            }
            .buttonStyle(.borderless)
            .foregroundStyle(disabled ? theme.colors.textDisabled : theme.colors.textPrimary)
            .disabled(disabled)
            .help(label)
            .accessibilityLabel(label)
        }

        private func indexerName(_ f: Mavio_Acquisition_V1_IndexerFailure) -> String {
            let name = all.first { $0.pluginID == f.pluginID && $0.indexerID == f.indexerID }?.indexerName ?? ""
            return !name.isEmpty ? name : !f.indexerID.isEmpty ? f.indexerID : f.pluginID
        }
    }

    /// The releases shown on a page.
    private let pageSize = 50

    /// The types of a search.
    private let searchTypes: [(type: Mavio_Acquisition_V1_ReleaseSearchType, label: String)] = [
        (.unspecified, MavioStrings.indexersTypeAny), (.movie, MavioStrings.indexersTypeMovie),
        (.tv, MavioStrings.indexersTypeTv),
    ]

    /// Newznab's top categories, by their first number.
    private let categories: [(code: Int32, label: String)] = [
        (1000, MavioStrings.indexersCategoryConsole), (2000, MavioStrings.indexersCategoryMovies),
        (3000, MavioStrings.indexersCategoryAudio), (4000, MavioStrings.indexersCategoryPc),
        (5000, MavioStrings.indexersCategoryTv), (6000, MavioStrings.indexersCategoryXxx),
        (7000, MavioStrings.indexersCategoryBooks), (8000, MavioStrings.indexersCategoryOther),
    ]

    /// The top categories of a release's categories, by name.
    private func categoryNames(_ list: [Int32]) -> String {
        var tops: [Int32] = []
        for c in list.map({ $0 / 1000 * 1000 }) where !tops.contains(c) { tops.append(c) }
        return tops.compactMap { c in categories.first { $0.code == c }?.label }.joined(separator: ", ")
    }

    /// How long ago a release was published: in days from a day, as Prowlarr tells.
    private func ageOf(_ r: Mavio_Acquisition_V1_Release) -> String {
        guard r.hasPublishDate else { return "" }
        let seconds = max(0, Date().timeIntervalSince(r.publishDate.date).rounded())
        let day: TimeInterval = 86_400
        return lengthOf(seconds >= day ? (seconds / day).rounded(.down) * day : seconds)
    }

    /// A file's size in megabytes, or gigabytes from a thousand megabytes.
    private func sizeOf(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let (value, unit, digits): (Double, UnitInformationStorage, Int) =
            n >= 1e9 ? (n / 1e9, .gigabytes, 2) : (n / 1e6, .megabytes, 1)
        return Measurement(value: value, unit: unit).formatted(
            .measurement(
                width: .abbreviated, usage: .asProvided,
                numberFormatStyle: .number.precision(.fractionLength(0...digits))))
    }

    /// What the releases are sorted by: a column of the table.
    enum IndexerSearchSortKey: Equatable {
        case age, title, indexer, size, grabs, peers
    }

    private func sortedBy(
        _ list: [Mavio_Acquisition_V1_Release], _ sort: IndexerSearchView.Sort
    ) -> [Mavio_Acquisition_V1_Release] {
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

    extension Mavio_Acquisition_V1_Indexer {
        /// The indexer's key, as plugin/indexer.
        fileprivate var key: String { "\(pluginID)/\(id)" }
    }

    /// The columns of the releases' table: sorted by their key, when they have
    /// one; the title takes what the others leave.
    private struct Column {
        let label: String
        var key: IndexerSearchSortKey?
        var width: CGFloat?
        var trailing = false
    }

    private let columns: [Column] = [
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

    extension View {
        /// The view in a column of the releases' table.
        fileprivate func cell(_ column: Column) -> some View {
            frame(
                minWidth: column.width == nil ? 120 : nil, maxWidth: column.width ?? .infinity,
                alignment: column.trailing ? .trailing : .leading
            )
            .frame(width: column.width)
        }
    }

    /// The headings of the releases' table: a column's heading sorts by it.
    private struct IndexerSearchHeader: View {
        @Binding var sort: IndexerSearchView.Sort?
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s8) {
                ForEach(columns.indices, id: \.self) { i in
                    let column = columns[i]
                    Group {
                        if let key = column.key {
                            Button {
                                sort = .init(
                                    key: key,
                                    desc: sort?.key == key
                                        ? !(sort?.desc ?? false) : [.size, .grabs, .peers].contains(key))
                            } label: {
                                HStack(spacing: MavioSpacing.s4) {
                                    Text(column.label)
                                    if let sort, sort.key == key {
                                        Icon("expand", size: 12).rotationEffect(.degrees(sort.desc ? 0 : 180))
                                    }
                                }
                            }
                            .buttonStyle(.plain)
                        } else {
                            Text(column.label)
                        }
                    }
                    .lineLimit(1)
                    .cell(column)
                }
            }
            .type(theme.type.footnote)
            .fontWeight(.medium)
            .foregroundStyle(theme.colors.textSecondary)
        }
    }

    /// A release as a line of the table, as Prowlarr's search lists them.
    private struct IndexerSearchLine: View {
        let server: ServerRecord
        let release: Mavio_Acquisition_V1_Release
        @Environment(\.theme) private var theme

        var body: some View {
            let r = release
            let torrent = r.protocol == .torrent
            HStack(spacing: MavioSpacing.s8) {
                IndexerSearchProtocol(torrent: torrent).cell(columns[0])
                Text(ageOf(r)).foregroundStyle(theme.colors.textSecondary).cell(columns[1])
                Text(r.title).fixedSize(horizontal: false, vertical: true).cell(columns[2])
                Text(r.indexerName).foregroundStyle(theme.colors.textSecondary).lineLimit(1).cell(columns[3])
                Text(r.size > 0 ? sizeOf(r.size) : "").cell(columns[4])
                Text(r.grabs > 0 ? String(r.grabs) : "").foregroundStyle(theme.colors.textSecondary).cell(columns[5])
                Text(torrent ? "\(r.seeders) / \(r.peers)" : "").cell(columns[6])
                Text(categoryNames(r.categories)).foregroundStyle(theme.colors.textSecondary).cell(columns[7])
                IndexerSearchActions(server: server, release: r).cell(columns[8])
            }
            .type(theme.type.footnote)
        }
    }

    /// A release in a narrow window: its title, and what the table's columns
    /// would tell, in a line under it.
    private struct IndexerSearchItem: View {
        let server: ServerRecord
        let release: Mavio_Acquisition_V1_Release
        @Environment(\.theme) private var theme

        var body: some View {
            let r = release
            let torrent = r.protocol == .torrent
            let detail = [
                torrent ? MavioStrings.indexersTorrent : MavioStrings.acquireUsenet,
                ageOf(r),
                r.indexerName,
                r.size > 0 ? sizeOf(r.size) : "",
                torrent ? MavioStrings.acquirePeers(seeders: String(r.seeders), peers: String(r.peers)) : "",
                categoryNames(r.categories),
            ]
            HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(r.title).type(theme.type.callout).fixedSize(horizontal: false, vertical: true)
                    Text(detail.filter { !$0.isEmpty }.joined(separator: " · "))
                        .type(theme.type.footnote)
                        .foregroundStyle(theme.colors.textSecondary)
                }
                Spacer(minLength: 0)
                IndexerSearchActions(server: server, release: r)
            }
        }
    }

    /// A release's protocol, on a badge.
    private struct IndexerSearchProtocol: View {
        let torrent: Bool
        @Environment(\.theme) private var theme

        var body: some View {
            Text(torrent ? MavioStrings.indexersTorrent : MavioStrings.acquireUsenet)
                .type(theme.type.caption)
                .fontWeight(.medium)
                .lineLimit(1)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .foregroundStyle(torrent ? theme.colors.textPrimary : theme.colors.textSecondary)
                .background(torrent ? theme.colors.fillSelected : theme.colors.fill, in: .rect(cornerRadius: 4))
        }
    }

    /// Sending a release to a download client, copying its magnet link, and
    /// its page on the indexer, in the browser.
    private struct IndexerSearchActions: View {
        let server: ServerRecord
        let release: Mavio_Acquisition_V1_Release
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.openURL) private var openURL
        @State private var copied = false
        @State private var sending = false
        /// The download client the release was sent to.
        @State private var sent: String?
        @State private var failure: String?

        var body: some View {
            HStack(spacing: 2) {
                if !release.infoURL.isEmpty, let url = URL(string: release.infoURL) {
                    action("external", MavioStrings.indexersInfo, color: theme.colors.textSecondary) { openURL(url) }
                }
                if !release.magnetURL.isEmpty {
                    action(
                        copied ? "check" : "magnet",
                        copied ? MavioStrings.indexersMagnetCopied : MavioStrings.indexersMagnet,
                        color: copied ? theme.colors.success : theme.colors.textSecondary
                    ) {
                        copyToClipboard(release.magnetURL)
                        copied = true
                    }
                }
                action(
                    sent != nil ? "check" : failure != nil ? "error" : "downloads",
                    sent.map { MavioStrings.indexersSent(client: $0) } ?? failure ?? MavioStrings.indexersSend,
                    color: failure != nil
                        ? theme.colors.danger : sent != nil ? theme.colors.success : theme.colors.accentText,
                    action: send
                )
                .opacity(sending ? 0.4 : 1)
            }
            .frame(maxWidth: .infinity, alignment: .trailing)
        }

        private func action(
            _ icon: String, _ label: String, color: Color, action: @escaping () -> Void
        ) -> some View {
            Button(action: action) {
                Icon(icon, size: 16).frame(width: 28, height: 28).contentShape(.rect)
            }
            .buttonStyle(.borderless)
            .foregroundStyle(color)
            .help(label)
            .accessibilityLabel(label)
        }

        private func send() {
            guard !sending, sent == nil else { return }
            sending = true
            failure = nil
            Task {
                defer { sending = false }
                var request = Mavio_Acquisition_V1_GrabReleaseRequest()
                request.release = release
                do {
                    sent = try await model.call(
                        server, Procedure.downloads("GrabRelease"), request,
                        as: Mavio_Acquisition_V1_GrabReleaseResponse.self
                    ).clientName
                } catch {
                    failure = error.reason
                }
            }
        }
    }
#endif
