import MavioKit
import MavioUI
import SwiftUI

/// The language of discovered titles' names and overviews: the app's, as
/// ISO 639-1.
let discoverLanguage = String((MavioStrings.bundle.preferredLocalizations.first ?? "en").prefix { $0 != "-" })

extension Procedure {
    static func discover(_ method: String) -> String { "/mavio.acquisition.v1.DiscoverService/\(method)" }
}

typealias Catalog = Mavio_Acquisition_V1_Catalog
typealias DiscoveredTitle = Mavio_Acquisition_V1_DiscoveredTitle
typealias MediaKind = Mavio_Acquisition_V1_MediaKind
typealias MediaStatus = Mavio_Acquisition_V1_MediaStatus

/// The server Discover asks (docs/clients.md §6.3.2): the first signed-in
/// server whose discovery providers offer catalogs, with its catalogs.
@MainActor
@Observable
final class Discovery {
    /// The server, nil when none discovers.
    private(set) var server: ServerRecord?
    /// Its catalogs.
    private(set) var catalogs: [Catalog] = []
    /// Whether the servers have answered.
    private(set) var loaded = false

    /// Asks the servers signed in to, in their order. A server that cannot
    /// be reached keeps Discover as it was, rather than taken for one that
    /// does not discover; it is asked again once it is reached.
    func load(_ model: AppModel) async {
        var request = Mavio_Acquisition_V1_ListCatalogsRequest()
        request.language = discoverLanguage
        var unreachable = false
        for server in model.signedInServers {
            let response: Mavio_Acquisition_V1_ListCatalogsResponse
            do {
                response = try await model.call(
                    server, Procedure.discover("ListCatalogs"), request,
                    as: Mavio_Acquisition_V1_ListCatalogsResponse.self)
            } catch {
                if Task.isCancelled { return }
                unreachable = true
                continue
            }
            if !response.catalogs.isEmpty {
                self.server = server
                catalogs = response.catalogs
                loaded = true
                return
            }
        }
        if !unreachable || self.server.map({ s in !model.signedIn.contains(s.id) }) ?? true {
            server = nil
            catalogs = []
        }
        loaded = true
    }

    /// What Discover is asked again on: the servers signed in to, and
    /// whether each is reached.
    static func key(_ model: AppModel) -> [String] {
        model.signedInServers.map { "\($0.id):\(model.states[$0.id] == .online)" }
    }

    /// Discover's rows: Trending, Popular, Upcoming, Top Rated and the
    /// providers' lists, each of movies and of series unless it mixes them.
    var rows: [CatalogRow] {
        let kinds: [Mavio_Plugin_V1_CatalogKind] = [.trending, .popular, .upcoming, .topRated, .list]
        return kinds.flatMap { kind in
            catalogs.filter { $0.kind == kind }.flatMap { catalog -> [CatalogRow] in
                if catalog.mixed || catalog.mediaKinds.count < 2 {
                    return [
                        CatalogRow(
                            catalog: catalog,
                            kind: catalog.mixed ? .unspecified : catalog.mediaKinds.first ?? .unspecified)
                    ]
                }
                return [MediaKind.movie, .series].filter(catalog.mediaKinds.contains).map {
                    CatalogRow(catalog: catalog, kind: $0)
                }
            }
        }
    }

    /// The catalog of the popular titles of a kind, which a title's genres,
    /// keywords and studios open filtered.
    func popular(_ kind: MediaKind) -> Catalog? {
        catalogs.first { $0.kind == .popular && $0.mediaKinds.contains(kind) }
    }

    /// The catalogs of the titles recommended for and similar to a title of a kind.
    func related(_ kind: MediaKind) -> [Catalog] {
        catalogs.filter { ($0.kind == .recommendations || $0.kind == .similar) && $0.mediaKinds.contains(kind) }
    }
}

/// A row of Discover: a catalog, of one kind of title unless it mixes them.
struct CatalogRow: Hashable {
    let catalog: Catalog
    /// Unspecified for a catalog listing movies and series together.
    let kind: MediaKind

    /// The row's name, by its catalog's kind and its kind of title.
    var title: String {
        let series = kind == .series
        switch catalog.kind {
        case .trending: return kind == .unspecified ? MavioStrings.discoverTrending : catalog.name
        case .popular: return series ? MavioStrings.discoverPopularSeries : MavioStrings.discoverPopularMovies
        case .upcoming: return series ? MavioStrings.discoverUpcomingSeries : MavioStrings.discoverUpcomingMovies
        case .topRated: return series ? MavioStrings.discoverTopRatedSeries : MavioStrings.discoverTopRatedMovies
        default: return catalog.name
        }
    }
}

/// A filter value by its provider's ID, with its name to show.
struct NamedValue: Hashable {
    let id: String
    let name: String
}

/// The titles of a catalog, a page by page: of a kind, recommended for or
/// similar to a title, or filtered alike a title's genre, keyword, studio,
/// network or language.
struct CatalogQuery: Hashable {
    let serverId: String
    let catalog: String
    var kind: MediaKind = .unspecified
    /// The title of recommendations and similar titles.
    var tmdbId: Int32 = 0
    var genres: [String] = []
    var keywords: [NamedValue] = []
    var studios: [NamedValue] = []
    var networks: [NamedValue] = []
    var originalLanguage = ""
    /// The order; unspecified for the catalog's own.
    var sort: Mavio_Plugin_V1_DiscoverSort = .unspecified
    var ascending = false
    /// Streaming services in a country, by ISO 3166-1.
    var watchProviders: [String] = []
    var watchRegion = ""
    /// Years of release, inclusive; 0 for any.
    var yearFrom = 0
    var yearTo = 0
    /// Minutes; 0 for any.
    var runtimeMin = 0
    var runtimeMax = 0
    /// 0–10; 0 for any.
    var ratingMin = 0
    var votesMin = 0
    /// The page's title.
    var title = ""

    /// How many filters are set, the sort aside.
    var filterCount: Int {
        [
            !genres.isEmpty, !studios.isEmpty, !networks.isEmpty, !keywords.isEmpty, !watchProviders.isEmpty,
            !originalLanguage.isEmpty, yearFrom > 0 || yearTo > 0, runtimeMin > 0 || runtimeMax > 0, ratingMin > 0,
            votesMin > 0,
        ].filter(\.self).count
    }

    /// The same catalog without filters or sort.
    var unfiltered: CatalogQuery {
        CatalogQuery(serverId: serverId, catalog: catalog, kind: kind, tmdbId: tmdbId, title: title)
    }

    func request(page: Int32) -> Mavio_Acquisition_V1_DiscoverRequest {
        var r = Mavio_Acquisition_V1_DiscoverRequest()
        r.catalog = catalog
        r.mediaKind = kind
        r.tmdbID = tmdbId
        r.genres = genres
        r.keywords = keywords.map(\.id)
        r.studios = studios.map(\.id)
        r.networks = networks.map(\.id)
        r.originalLanguage = originalLanguage
        r.sort = sort
        r.ascending = sort != .unspecified && ascending
        r.watchProviders = watchProviders
        r.watchRegion = watchProviders.isEmpty ? "" : watchRegion
        r.releasedFrom = yearFrom > 0 ? String(format: "%04d-01-01", yearFrom) : ""
        r.releasedTo = yearTo > 0 ? String(format: "%04d-12-31", yearTo) : ""
        r.runtimeMin = Int32(runtimeMin)
        r.runtimeMax = Int32(runtimeMax)
        r.ratingMin = Double(ratingMin)
        r.votesMin = Int32(votesMin)
        r.language = discoverLanguage
        r.page = page
        return r
    }
}

extension AppModel {
    /// A page of a catalog, from 1.
    func discover(_ query: CatalogQuery, page: Int32) async throws -> Mavio_Acquisition_V1_DiscoverResponse {
        try await client.call(query.serverId, Procedure.discover("Discover"), query.request(page: page))
    }
}

/// The same TMDB image at a width, rather than its original size.
func sized(_ url: String, _ width: Int) -> String? {
    url.isEmpty
        ? nil : url.replacingOccurrences(of: "image.tmdb.org/t/p/original/", with: "image.tmdb.org/t/p/w\(width)/")
}

extension DiscoveredTitle {
    /// Its TMDB ID.
    var tmdbId: Int32 { Int32(externalIds["tmdb"] ?? "") ?? 0 }
    /// Identifies it among a catalog's titles.
    var key: String { "\(mediaKind.rawValue)/\(externalIds["tmdb"] ?? name)" }
}

/// The status a title's card and header show, if any.
func statusLabel(_ status: MediaStatus) -> String? {
    switch status {
    case .available: MavioStrings.discoverAvailable
    case .partiallyAvailable: MavioStrings.discoverPartiallyAvailable
    case .requested: MavioStrings.discoverRequested
    case .wanted: MavioStrings.discoverProcessing
    case .blocklisted: MavioStrings.discoverBlocklisted
    default: nil
    }
}

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
