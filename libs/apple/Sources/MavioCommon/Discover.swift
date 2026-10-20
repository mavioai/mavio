import Foundation
import MavioDesign
import MavioKit
import Observation

// Discover's model (docs/clients.md §6.3.2): the server that discovers,
// its catalogs, the queries of their titles, and the helpers the screens
// of discovered titles share.

/// The language of discovered titles' names and overviews: the app's, as
/// ISO 639-1.
public let discoverLanguage = String((MavioStrings.bundle.preferredLocalizations.first ?? "en").prefix { $0 != "-" })

/// A catalog of Discover.
public typealias Catalog = Mavio_Acquisition_V1_Catalog
/// A title of a catalog.
public typealias DiscoveredTitle = Mavio_Acquisition_V1_DiscoveredTitle
/// A movie or a series.
public typealias MediaKind = Mavio_Acquisition_V1_MediaKind
/// What the server has of a title.
public typealias MediaStatus = Mavio_Acquisition_V1_MediaStatus

/// The server Discover asks (docs/clients.md §6.3.2): the first signed-in
/// server whose discovery providers offer catalogs, with its catalogs.
@MainActor
@Observable
public final class Discovery {
    /// A discovery that has not loaded yet.
    public init() {}

    /// The server, nil when none discovers.
    public private(set) var server: ServerRecord?
    /// Its catalogs.
    public private(set) var catalogs: [Catalog] = []
    /// Whether the servers have answered.
    public private(set) var loaded = false

    /// Asks the servers signed in to, in their order. A server that cannot
    /// be reached keeps Discover as it was, rather than taken for one that
    /// does not discover; it is asked again once it is reached.
    public func load(_ model: AppModel) async {
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
    public static func key(_ model: AppModel) -> [String] {
        model.signedInServers.map { "\($0.id):\(model.states[$0.id] == .online)" }
    }

    /// Discover's rows: Trending, Popular, Upcoming, Top Rated and the
    /// providers' lists, each of movies and of series unless it mixes them.
    public var rows: [CatalogRow] {
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
    public func popular(_ kind: MediaKind) -> Catalog? {
        catalogs.first { $0.kind == .popular && $0.mediaKinds.contains(kind) }
    }

    /// The catalogs of the titles recommended for and similar to a title of a kind.
    public func related(_ kind: MediaKind) -> [Catalog] {
        catalogs.filter { ($0.kind == .recommendations || $0.kind == .similar) && $0.mediaKinds.contains(kind) }
    }
}

/// A row of Discover: a catalog, of one kind of title unless it mixes them.
public struct CatalogRow: Hashable, Sendable {
    /// The catalog.
    public let catalog: Catalog
    /// Unspecified for a catalog listing movies and series together.
    public let kind: MediaKind

    /// A row of a catalog of a kind of title.
    public init(catalog: Catalog, kind: MediaKind) {
        self.catalog = catalog
        self.kind = kind
    }

    /// The row's name, by its catalog's kind and its kind of title.
    public var title: String {
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
public struct NamedValue: Hashable, Sendable {
    /// The provider's ID.
    public let id: String
    /// The name.
    public let name: String

    /// A value by its ID and name.
    public init(id: String, name: String) {
        self.id = id
        self.name = name
    }
}

/// The titles of a catalog, a page by page: of a kind, recommended for or
/// similar to a title, or filtered alike a title's genre, keyword, studio,
/// network or language.
public struct CatalogQuery: Hashable, Sendable {
    /// The server asked.
    public let serverId: String
    /// The catalog's ID.
    public let catalog: String
    /// The kind of title; unspecified for both.
    public var kind: MediaKind = .unspecified
    /// The title of recommendations and similar titles.
    public var tmdbId: Int32 = 0
    /// Genres, by ID.
    public var genres: [String] = []
    /// Keywords.
    public var keywords: [NamedValue] = []
    /// Studios.
    public var studios: [NamedValue] = []
    /// Networks.
    public var networks: [NamedValue] = []
    /// The original language, by ISO 639-1; empty for any.
    public var originalLanguage = ""
    /// The order; unspecified for the catalog's own.
    public var sort: Mavio_Plugin_V1_DiscoverSort = .unspecified
    /// Whether the order ascends.
    public var ascending = false
    /// Streaming services in a country, by ISO 3166-1.
    public var watchProviders: [String] = []
    /// The country of the streaming services.
    public var watchRegion = ""
    /// Years of release, inclusive; 0 for any.
    public var yearFrom = 0
    /// The last year of release.
    public var yearTo = 0
    /// Minutes; 0 for any.
    public var runtimeMin = 0
    /// The longest runtime.
    public var runtimeMax = 0
    /// 0–10; 0 for any.
    public var ratingMin = 0
    /// The fewest votes; 0 for any.
    public var votesMin = 0
    /// The page's title.
    public var title = ""

    /// A query of a catalog's titles, alike filtered.
    public init(
        serverId: String, catalog: String, kind: MediaKind = .unspecified, tmdbId: Int32 = 0, genres: [String] = [],
        keywords: [NamedValue] = [], studios: [NamedValue] = [], networks: [NamedValue] = [],
        originalLanguage: String = "", sort: Mavio_Plugin_V1_DiscoverSort = .unspecified, ascending: Bool = false,
        watchProviders: [String] = [], watchRegion: String = "", yearFrom: Int = 0, yearTo: Int = 0,
        runtimeMin: Int = 0, runtimeMax: Int = 0, ratingMin: Int = 0, votesMin: Int = 0, title: String = ""
    ) {
        self.serverId = serverId
        self.catalog = catalog
        self.kind = kind
        self.tmdbId = tmdbId
        self.genres = genres
        self.keywords = keywords
        self.studios = studios
        self.networks = networks
        self.originalLanguage = originalLanguage
        self.sort = sort
        self.ascending = ascending
        self.watchProviders = watchProviders
        self.watchRegion = watchRegion
        self.yearFrom = yearFrom
        self.yearTo = yearTo
        self.runtimeMin = runtimeMin
        self.runtimeMax = runtimeMax
        self.ratingMin = ratingMin
        self.votesMin = votesMin
        self.title = title
    }

    /// How many filters are set, the sort aside.
    public var filterCount: Int {
        [
            !genres.isEmpty, !studios.isEmpty, !networks.isEmpty, !keywords.isEmpty, !watchProviders.isEmpty,
            !originalLanguage.isEmpty, yearFrom > 0 || yearTo > 0, runtimeMin > 0 || runtimeMax > 0, ratingMin > 0,
            votesMin > 0,
        ].filter(\.self).count
    }

    /// The same catalog without filters or sort.
    public var unfiltered: CatalogQuery {
        CatalogQuery(serverId: serverId, catalog: catalog, kind: kind, tmdbId: tmdbId, title: title)
    }

    /// The request of a page, from 1.
    public func request(page: Int32) -> Mavio_Acquisition_V1_DiscoverRequest {
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
    public func discover(_ query: CatalogQuery, page: Int32) async throws -> Mavio_Acquisition_V1_DiscoverResponse {
        try await client.call(query.serverId, Procedure.discover("Discover"), query.request(page: page))
    }
}

/// The same TMDB image at a width, rather than its original size.
public func sized(_ url: String, _ width: Int) -> String? {
    url.isEmpty
        ? nil : url.replacingOccurrences(of: "image.tmdb.org/t/p/original/", with: "image.tmdb.org/t/p/w\(width)/")
}

extension DiscoveredTitle {
    /// Its TMDB ID.
    public var tmdbId: Int32 { Int32(externalIds["tmdb"] ?? "") ?? 0 }
    /// Identifies it among a catalog's titles.
    public var key: String { "\(mediaKind.rawValue)/\(externalIds["tmdb"] ?? name)" }
}

/// The status a title's card and header show, if any.
public func statusLabel(_ status: MediaStatus) -> String? {
    switch status {
    case .available: MavioStrings.discoverAvailable
    case .partiallyAvailable: MavioStrings.discoverPartiallyAvailable
    case .requested: MavioStrings.discoverRequested
    case .wanted: MavioStrings.discoverProcessing
    case .blocklisted: MavioStrings.discoverBlocklisted
    default: nil
    }
}

/// A discovered title's details.
public typealias TitleInfo = Mavio_Acquisition_V1_Title
/// A person credited on a discovered title.
public typealias TitleCredit = Mavio_Acquisition_V1_TitleCredit
/// A title's rating by a service.
public typealias Rating = Mavio_Plugin_V1_Rating

extension AppModel {
    /// A movie or series by its TMDB ID, with what the server has of it.
    public func title(_ serverId: String, _ kind: MediaKind, _ tmdbId: Int32) async throws -> TitleInfo {
        var r = Mavio_Acquisition_V1_GetTitleRequest()
        r.mediaKind = kind
        r.tmdbID = tmdbId
        r.language = discoverLanguage
        let response = try await client.call(
            serverId, Procedure.discover("GetTitle"), r, as: Mavio_Acquisition_V1_GetTitleResponse.self)
        return response.title
    }
}

extension Discovery {
    /// The popular titles of a kind filtered alike a title, titled by the
    /// value; nil without such a catalog.
    public func alike(_ serverId: String, _ kind: MediaKind, _ name: String, _ fill: (inout CatalogQuery) -> Void)
        -> CatalogQuery?
    {
        guard let popular = popular(kind) else { return nil }
        var query = CatalogQuery(serverId: serverId, catalog: popular.id, kind: kind, title: name)
        fill(&query)
        return query
    }
}

extension Mavio_Library_V1_CreditKind {
    /// The job, as an item's credits name it.
    public var job: CreditJob {
        switch self {
        case .actor: .actor
        case .guestStar: .guestStar
        case .director: .director
        case .writer: .writer
        case .producer: .producer
        case .creator: .creator
        case .composer: .composer
        case .conductor: .conductor
        case .lyricist: .lyricist
        case .artist: .artist
        case .author: .author
        case .narrator: .narrator
        default: .other
        }
    }
}

extension TitleCredit {
    /// Their TMDB ID, which opens their page; 0 when unknown.
    public var tmdbId: Int32 { Int32(externalIds["tmdb"] ?? "") ?? 0 }
    /// Whether they act.
    public var isCast: Bool { kind.job.isCast }
    /// What a card says they did: the character played, else the job.
    public var shownRole: String? {
        if isCast { return role.isEmpty ? nil : role }
        return kind.job == .other && !role.isEmpty ? role : kind.job.label
    }
}

/// The jobs Seerr shows first of a title's crew, in its order, after a
/// series' creators.
private let crewJobs = [
    "Director", "Creator", "Screenplay", "Writer", "Original Music Composer", "Composer", "Editor", "Producer",
    "Co-Producer", "Executive Producer", "Animation",
]

/// Where a credit stands among the leading crew, a series' creators first,
/// then by the jobs Seerr shows first.
public func crewRank(_ c: TitleCredit) -> Int {
    if c.kind == .creator { return -1 }
    let kindJob =
        switch c.kind {
        case .director: "Director"
        case .writer: "Writer"
        case .composer: "Composer"
        case .producer: "Producer"
        default: ""
        }
    let job = c.role.isEmpty ? kindJob : c.role
    return crewJobs.firstIndex(of: job) ?? crewJobs.count
}

/// An ISO 8601 calendar date, as "Nov 2, 2026"; nil when empty.
public func dayText(_ iso: String) -> String? {
    guard !iso.isEmpty, let date = try? Date(iso + "T00:00:00Z", strategy: .iso8601) else { return nil }
    return date.formatted(Date.FormatStyle(date: .abbreviated, time: .omitted, timeZone: .gmt))
}

/// An amount of US dollars; nil when 0, which stands for unknown.
public func moneyText(_ dollars: Int64) -> String? {
    dollars > 0 ? dollars.formatted(.currency(code: "USD").precision(.fractionLength(0))) : nil
}

/// The name of an ISO 639-1 language in the app's language.
public func languageText(_ code: String) -> String {
    Locale.current.localizedString(forLanguageCode: code) ?? code
}

/// The name of an ISO 3166-1 country in the app's language, after its flag.
public func countryText(_ code: String) -> String {
    let flag = code.unicodeScalars.compactMap { Unicode.Scalar(0x1F1A5 + $0.value) }.map(String.init).joined()
    return "\(flag) \(Locale.current.localizedString(forRegionCode: code) ?? code)"
}

extension AppModel {
    /// A collection of movies by its TMDB ID.
    public func collection(_ serverId: String, _ tmdbId: Int32) async throws -> Mavio_Acquisition_V1_Collection {
        var r = Mavio_Acquisition_V1_GetCollectionRequest()
        r.tmdbCollectionID = tmdbId
        r.language = discoverLanguage
        return try await client.call(
            serverId, Procedure.discover("GetCollection"), r, as: Mavio_Acquisition_V1_GetCollectionResponse.self
        ).collection
    }
}

extension Mavio_Plugin_V1_ProductionStatus {
    /// The status's name; nil when unknown.
    public var text: String? {
        switch self {
        case .rumored: MavioStrings.discoverStatusRumored
        case .planned: MavioStrings.discoverStatusPlanned
        case .inProduction: MavioStrings.discoverStatusInProduction
        case .postProduction: MavioStrings.discoverStatusPostProduction
        case .released: MavioStrings.discoverStatusReleased
        case .canceled: MavioStrings.discoverStatusCanceled
        case .pilot: MavioStrings.discoverStatusPilot
        case .returning: MavioStrings.discoverStatusReturning
        case .ended: MavioStrings.discoverStatusEnded
        default: nil
        }
    }
}

/// A length of time, as "1h 52m".
public func minutesText(_ seconds: TimeInterval) -> String {
    Duration.seconds((seconds / 60).rounded() * 60).formatted(.units(allowed: [.hours, .minutes], width: .narrow))
}

/// Three-letter language codes from a field: lowercase, separated by commas
/// or spaces; nil when one is not three letters.
public func languageCodes(_ s: String) -> [String]? {
    let list = s.lowercased()
        .split(whereSeparator: { $0.isWhitespace || ",，、".contains($0) })
        .map(String.init)
    return list.allSatisfy { $0.count == 3 && $0.allSatisfy { ("a"..."z").contains($0) } } ? list : nil
}
