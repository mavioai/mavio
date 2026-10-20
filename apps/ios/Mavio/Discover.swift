import MavioKit
import MavioUI
import SwiftUI

/// Discover: a row of each catalog of the first server that discovers.
struct DiscoverView: View {
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    /// The rows' first pages read again by the last pull.
    @State private var refreshed: [CatalogQuery: Mavio_Acquisition_V1_DiscoverResponse] = [:]

    var body: some View {
        Group {
            if let server = discovery.server {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        ForEach(discovery.rows, id: \.self) { row in
                            let query = Self.query(server, row)
                            CatalogShelf(query: query, refreshed: refreshed[query])
                        }
                    }
                    .padding(.bottom, MavioSpacing.s24)
                }
                .pullToRefresh { await refresh() }
            } else if discovery.loaded {
                StatusView(
                    icon: "discover", title: MavioStrings.discoverNotSetUp,
                    message: MavioStrings.discoverNotSetUpMessage)
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle(MavioStrings.navDiscover)
    }

    private static func query(_ server: ServerRecord, _ row: CatalogRow) -> CatalogQuery {
        CatalogQuery(serverId: server.id, catalog: row.catalog.id, kind: row.kind, title: row.title)
    }

    /// Reads the catalogs and every row's first page again, as pulling the
    /// page down does, the rows changing once all have answered.
    private func refresh() async {
        await discovery.load(model)
        guard let server = discovery.server else { return }
        let queries = discovery.rows.map { Self.query(server, $0) }
        var pages: [CatalogQuery: Mavio_Acquisition_V1_DiscoverResponse] = [:]
        await withTaskGroup(of: (CatalogQuery, Mavio_Acquisition_V1_DiscoverResponse?).self) { group in
            for query in queries {
                group.addTask { await (query, try? model.discover(query, page: 1)) }
            }
            for await (query, page) in group {
                if let page { pages[query] = page }
            }
        }
        refreshed = pages
    }
}

/// A row of a catalog's first page, with See All opening the catalog;
/// nothing when it has no titles.
struct CatalogShelf: View {
    let query: CatalogQuery
    /// How often the page it is on was pulled down, each time read again.
    var refreshes = 0
    /// Its first page as the page it is on read it again.
    var refreshed: Mavio_Acquisition_V1_DiscoverResponse?
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var titles: [DiscoveredTitle]?
    @State private var more = false

    var body: some View {
        Group {
            if let titles {
                if !titles.isEmpty {
                    Shelf(query.title) {
                        Spacer()
                        if more {
                            RouteLink(MavioStrings.homeMore, value: Route.catalog(query)).type(theme.type.callout)
                        }
                    } content: {
                        ForEach(titles, id: \.key) {
                            DiscoveredTitleCard(serverId: query.serverId, title: $0, width: theme.layout.poster)
                        }
                    }
                }
            } else {
                Shelf(query.title) {
                    ForEach(0..<8, id: \.self) { _ in CardPlaceholder() }
                }
            }
        }
        .task(id: ShelfKey(query: query, refreshes: refreshes)) {
            // Read again, the titles shown stay when it cannot be.
            guard let page = try? await model.discover(query, page: 1) else {
                if !Task.isCancelled, titles == nil { titles = [] }
                return
            }
            show(page)
        }
        .onChange(of: refreshed) { if let refreshed { show(refreshed) } }
    }

    private func show(_ page: Mavio_Acquisition_V1_DiscoverResponse) {
        titles = page.titles
        more = page.page < page.totalPages
    }
}

private struct ShelfKey: Equatable {
    let query: CatalogQuery
    let refreshes: Int
}

/// All the titles of a catalog, page by page.
struct CatalogView: View {
    let query: CatalogQuery
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    /// The query with the filters applied, once some are.
    @State private var filtered: CatalogQuery?
    @State private var filtering = false
    @State private var titles: [DiscoveredTitle] = []
    @State private var next: Int32? = 1
    @State private var loading = false
    @State private var failed = false
    /// The query the pages are of.
    @State private var shown: CatalogQuery?

    private var current: CatalogQuery { filtered ?? query }

    /// The catalog, when it takes filters or sorts; recommendations and
    /// similar titles take none.
    private var filterable: Catalog? {
        guard query.tmdbId == 0,
            let catalog = discovery.catalogs.first(where: { $0.id == query.catalog }),
            !catalog.filters.isEmpty || !catalog.sorts.isEmpty
        else { return nil }
        return catalog
    }

    var body: some View {
        ScrollView {
            CardGrid(items: titles.map(\.key)) { key in
                if let title = titles.first(where: { $0.key == key }) {
                    DiscoveredTitleCard(serverId: query.serverId, title: title)
                        .onAppear { if key == titles.last?.key { Task { await load() } } }
                }
            }
            .padding(.top, MavioSpacing.s8)
            if loading || (titles.isEmpty && next != nil && !failed) {
                ProgressView().padding(MavioSpacing.s24)
            } else if failed {
                StatusView(
                    icon: "error", title: MavioStrings.stateError,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else if titles.isEmpty, current.filterCount > 0 || current.sort != .unspecified {
                StatusView(
                    icon: "search", title: MavioStrings.discoverNoMatches,
                    action: (MavioStrings.discoverReset, { filtered = query.unfiltered }))
            } else if titles.isEmpty {
                StatusView(icon: "search", title: MavioStrings.discoverNoMatches)
            }
        }
        .pullToRefresh { await refresh() }
        .navigationTitle(query.title)
        .toolbar {
            if filterable != nil {
                ToolbarItem(placement: .topBarTrailing) {
                    let n = current.filterCount
                    Button {
                        filtering = true
                    } label: {
                        HStack(spacing: 4) {
                            Icon(n > 0 || current.sort != .unspecified ? "filterFill" : "filter", size: 20)
                            if n > 0 { Text(String(n)).type(theme.type.callout).fontWeight(.semibold) }
                        }
                    }
                    .accessibilityLabel(n > 0 ? "\(MavioStrings.discoverFilters) (\(n))" : MavioStrings.discoverFilters)
                }
            }
        }
        .sheet(isPresented: $filtering) {
            if let catalog = filterable {
                CatalogFiltersSheet(catalog: catalog, value: current) { filtered = $0 }
            }
        }
        .task(id: current) {
            // Coming back from a title keeps the pages; other filters start again.
            if shown != current {
                shown = current
                titles = []
                next = 1
                failed = false
                loading = false
            }
            if titles.isEmpty { await load() }
        }
    }

    private func load() async {
        guard let page = next, !loading else { return }
        let query = current
        loading = true
        failed = false
        defer { if query == current { loading = false } }
        do {
            let response = try await model.discover(query, page: page)
            // Filters applied meanwhile start the grid again.
            guard query == current else { return }
            var seen = Set(titles.map(\.key))
            titles += response.titles.filter { seen.insert($0.key).inserted }
            next = response.page < response.totalPages ? response.page + 1 : nil
        } catch {
            if query == current { failed = !(error is CancellationError) }
        }
    }

    /// Reads the first page again, as pulling the grid down does, in place
    /// of the pages shown once it arrives; they stay when it cannot be read.
    private func refresh() async {
        let query = current
        guard let response = try? await model.discover(query, page: 1), query == current else { return }
        var seen = Set<String>()
        titles = response.titles.filter { seen.insert($0.key).inserted }
        next = response.page < response.totalPages ? response.page + 1 : nil
        failed = false
    }
}

/// The card of a discovered title, which opens it; under its year and
/// kind, a person's role in it.
struct DiscoveredTitleCard: View {
    let serverId: String
    let title: DiscoveredTitle
    var role: String?
    /// The width; nil fills the space offered, as in grids.
    var width: CGFloat?
    @Environment(\.theme) private var theme

    var body: some View {
        RouteLink(value: Route.title(serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbId)) {
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                ArtworkView(
                    sized(title.posterURL, 342), title: title.name,
                    icon: title.mediaKind == .series ? "shows" : "movies"
                )
                .aspectRatio(2 / 3, contentMode: .fit)
                .clipShape(.rect(cornerRadius: MavioRadius.md))
                .overlay(alignment: .topTrailing) { StatusMark(status: title.status) }
                Text(title.name).type(theme.type.headline).lineLimit(1)
                    .foregroundStyle(theme.colors.textPrimary)
                Text(
                    [
                        title.year > 0 ? String(title.year) : nil,
                        title.mediaKind == .series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
                    ]
                    .compactMap(\.self).joined(separator: " · ")
                )
                .type(theme.type.footnote).lineLimit(1)
                .foregroundStyle(theme.colors.textSecondary)
                if let role, !role.isEmpty {
                    Text(role).type(theme.type.footnote).lineLimit(1).foregroundStyle(theme.colors.textSecondary)
                }
            }
            .frame(width: width)
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }
}

/// The mark of a card's status: a check once available, a clock while
/// requested or processing.
private struct StatusMark: View {
    let status: MediaStatus
    @Environment(\.theme) private var theme

    var body: some View {
        let available = status == .available || status == .partiallyAvailable
        if available || status == .requested || status == .wanted {
            Icon(available ? "check" : "pending", size: 16)
                .foregroundStyle(.white)
                .padding(2)
                .background(available ? theme.colors.success : theme.colors.scrim, in: .circle)
                .opacity(status == .partiallyAvailable ? 0.7 : 1)
                .padding(6)
                .accessibilityLabel(statusLabel(status) ?? "")
        }
    }
}
