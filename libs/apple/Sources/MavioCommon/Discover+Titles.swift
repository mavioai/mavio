import Foundation
import MavioDesign
import MavioKit

// What the pages of discovered titles, people and collections show, apart
// from how they draw it: the links elsewhere, the facts of a title, its
// leading crew, the order of its seasons, a person's age, a collection's
// status, and the names of a catalog's sorts, languages and countries.

/// The locale of TMDB's pages, as "en-US".
private var webLocale: String {
    (MavioStrings.bundle.preferredLocalizations.first ?? "en").replacingOccurrences(of: "_", with: "-")
}

/// A link to a title or person elsewhere, with the site's logo.
public struct TitleLink: Hashable, Sendable {
    /// The site's name.
    public let name: String
    /// The page.
    public let url: URL
    /// The logo, by its name in the services' logos.
    public let logo: String
    /// The logo's width, as Seerr's.
    public let width: CGFloat

    /// A link to url on the site named name, with its logo width wide.
    public init(name: String, url: URL, logo: String, width: CGFloat) {
        (self.name, self.url, self.logo, self.width) = (name, url, logo, width)
    }
}

/// A title's page on TMDB, in the app's language.
public func tmdbURL(_ kind: MediaKind, _ tmdbId: Int32) -> String {
    "https://www.themoviedb.org/\(kind == .series ? "tv" : "movie")/\(tmdbId)?language=\(webLocale)"
}

/// The logos linking a title elsewhere, as Seerr's, Rotten Tomatoes' once
/// its ratings tell its page.
public func titleLinks(_ title: TitleInfo, _ ratings: [Rating]) -> [TitleLink] {
    titleLinks(
        kind: title.mediaKind, tmdbId: title.tmdbID, imdb: title.externalIds["imdb"] ?? "",
        tvdb: title.externalIds["tvdb"] ?? (title.tvdbID > 0 ? String(title.tvdbID) : ""), ratings: ratings)
}

/// The logos linking a title elsewhere by its IDs.
public func titleLinks(kind: MediaKind, tmdbId: Int32, imdb: String, tvdb: String, ratings: [Rating]) -> [TitleLink] {
    let series = kind == .series
    let rt = ratings.first { $0.source != .imdb && !$0.url.isEmpty }?.url
    let simkl = imdb.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? imdb
    let links: [(String, String?, String, CGFloat)] = [
        ("TMDB", tmdbURL(kind, tmdbId), "tmdb", 32),
        ("TVDB", series && !tvdb.isEmpty ? "https://www.thetvdb.com/dereferrer/series/\(tvdb)" : nil, "tvdb", 36),
        ("IMDb", imdb.isEmpty ? nil : "https://www.imdb.com/title/\(imdb)", "imdb", 32),
        ("Rotten Tomatoes", rt, "rt", 56),
        ("Trakt", imdb.isEmpty ? nil : "https://trakt.tv/\(series ? "shows" : "movies")/\(imdb)", "trakt", 32),
        ("Simkl", imdb.isEmpty ? nil : "https://api.simkl.com/redirect?to=Simkl&imdb=\(simkl)", "simkl", 32),
        ("Letterboxd", series ? nil : "https://letterboxd.com/tmdb/\(tmdbId)", "letterboxd", 32),
    ]
    return links.compactMap { name, url, logo, width in
        url.flatMap(URL.init(string:)).map { TitleLink(name: name, url: $0, logo: logo, width: width) }
    }
}

/// An item's link elsewhere as its site's logo, for the sites the server
/// names its items' links by: TMDB, TheTVDB and IMDb; nil for another.
public func siteLink(name: String, url: String) -> TitleLink? {
    let logos: [String: (String, CGFloat)] = ["TMDB": ("tmdb", 32), "TheTVDB": ("tvdb", 36), "IMDb": ("imdb", 32)]
    guard let (logo, width) = logos[name], let url = URL(string: url) else { return nil }
    return TitleLink(name: name, url: url, logo: logo, width: width)
}

/// The logos linking a discovered person elsewhere: TMDB's and IMDb's.
public func personLinks(_ p: Mavio_Acquisition_V1_Person) -> [TitleLink] {
    var links: [TitleLink] = []
    if let id = p.externalIds["tmdb"], let url = URL(string: "https://www.themoviedb.org/person/\(id)") {
        links.append(TitleLink(name: "TMDB", url: url, logo: "tmdb", width: 32))
    }
    if let id = p.externalIds["imdb"], let url = URL(string: "https://www.imdb.com/name/\(id)") {
        links.append(TitleLink(name: "IMDb", url: url, logo: "imdb", width: 32))
    }
    return links
}

/// What a person's header says of them: born, where, and died.
public func personFacts(_ p: Mavio_Acquisition_V1_Person) -> [String] {
    [
        dayText(p.birthday).map { MavioStrings.personBorn(date: $0, age: String(personAge(p.birthday, p.deathday))) },
        p.placeOfBirth.isEmpty ? nil : p.placeOfBirth,
        dayText(p.deathday).map { MavioStrings.personDied(date: $0) },
    ].compactMap(\.self)
}

/// Whole years between two calendar dates, the second today when empty.
public func personAge(_ from: String, _ to: String) -> Int {
    let parse = { (s: String) in try? Date(s + "T00:00:00Z", strategy: .iso8601) }
    guard let a = parse(from) else { return 0 }
    var utc = Calendar(identifier: .gregorian)
    utc.timeZone = .gmt
    return utc.dateComponents([.year], from: a, to: parse(to) ?? .now).year ?? 0
}

/// What the server has of a collection, as Seerr marks it: available when
/// it has every movie, partially available when some, requested when every
/// other movie is requested, blocklisted when every movie is.
public func collectionStatus(_ parts: [DiscoveredTitle]) -> MediaStatus {
    guard !parts.isEmpty else { return .unspecified }
    if parts.allSatisfy({ $0.status == .blocklisted }) { return .blocklisted }
    if parts.allSatisfy({ $0.status == .available }) { return .available }
    if parts.contains(where: { $0.status == .available || $0.status == .partiallyAvailable }) {
        return .partiallyAvailable
    }
    if parts.allSatisfy({ $0.status == .requested || $0.status == .wanted }) { return .requested }
    return .unspecified
}

/// Rotten Tomatoes' least fresh score, and least upright audience score.
public let ratingFreshScore = 60.0

/// The ratings a title shows, in Seerr's order: Rotten Tomatoes' critics'
/// and audience's, then IMDb's.
public func shownRatings(_ ratings: [Rating]) -> [Rating] {
    let order: [Mavio_Plugin_V1_RatingSource] = [.rottenTomatoesCritics, .rottenTomatoesAudience, .imdb]
    return order.compactMap { source in ratings.first { $0.source == source } }
}

/// A rating as a title shows it: the logo, by its name in the services'
/// logos, and its height; what it reads aloud; and its value.
public struct RatingMark: Hashable, Sendable {
    /// The logo's name.
    public let logo: String
    /// What the rating is, read aloud.
    public let label: String
    /// The score.
    public let value: String
    /// Its page.
    public let url: URL?
    /// The logo's height.
    public let height: CGFloat
}

/// The ratings a title shows, as Seerr's: Rotten Tomatoes' and IMDb's, then
/// TMDB's score when given, each with its page.
public func ratingMarks(kind: MediaKind, tmdbId: Int32, ratings: [Rating], score: Double?) -> [RatingMark] {
    var marks = shownRatings(ratings).map { r -> RatingMark in
        let fresh = r.score >= ratingFreshScore
        let (logo, label, value): (String, String, String) =
            switch r.source {
            case .imdb:
                (
                    "imdb",
                    MavioStrings.discoverImdbScore(
                        votes: r.votes.formatted(.number.notation(.compactName).precision(.fractionLength(0...1)))),
                    r.score.formatted(.number.precision(.fractionLength(1)))
                )
            case .rottenTomatoesCritics:
                (
                    fresh ? "rt-fresh" : "rt-rotten",
                    r.certified ? MavioStrings.discoverTomatometerCertified : MavioStrings.discoverTomatometer,
                    "\(Int(r.score.rounded()))%"
                )
            default:
                (
                    fresh ? "rt-aud-fresh" : "rt-aud-rotten", MavioStrings.discoverAudienceScore,
                    "\(Int(r.score.rounded()))%"
                )
            }
        return RatingMark(logo: logo, label: label, value: value, url: URL(string: r.url), height: 20)
    }
    if let score {
        marks.append(
            RatingMark(
                logo: "tmdb-wide", label: MavioStrings.discoverTmdbScore, value: "\(Int((score * 10).rounded()))%",
                url: URL(string: tmdbURL(kind, tmdbId)), height: 16))
    }
    return marks
}

/// A fact's value: plain, or opening the popular titles alike.
public struct FactValue: Hashable, Sendable {
    /// The value.
    public let text: String
    /// The popular titles alike, if any.
    public var query: CatalogQuery?

    /// A value, opening query.
    public init(text: String, query: CatalogQuery? = nil) {
        self.text = text
        self.query = query
    }
}

/// How many of a title's studios show until the rest are asked for.
public let shownStudios = 5

/// The genres of a discovered title, by their IDs when it has them.
public func titleGenres(_ title: TitleInfo) -> [NamedValue] {
    title.genreRefs.isEmpty
        ? title.genres.map { NamedValue(id: "", name: $0) }
        : title.genreRefs.map { NamedValue(id: $0.id, name: $0.name) }
}

/// The keywords of a discovered title, by their IDs when it has them.
public func titleKeywords(_ title: TitleInfo) -> [NamedValue] {
    title.keywordRefs.isEmpty
        ? title.keywords.map { NamedValue(id: "", name: $0) }
        : title.keywordRefs.map { NamedValue(id: $0.id, name: $0.name) }
}

/// The studios of a movie or networks of a series, by their IDs when it has them.
public func titleStudios(_ title: TitleInfo) -> [NamedValue] {
    title.studioRefs.isEmpty
        ? title.studios.map { NamedValue(id: "", name: $0) }
        : title.studioRefs.map { NamedValue(id: $0.id, name: $0.name) }
}

/// As Seerr's header: the year, kind, rating and runtime of a title.
public func titleHeaderFacts(_ title: TitleInfo) -> [String] {
    [
        title.year > 0 ? String(title.year) : nil,
        title.mediaKind == .series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
        title.officialRating.isEmpty ? nil : title.officialRating,
        title.runtimeMinutes > 0 ? minutesText(TimeInterval(title.runtimeMinutes) * 60) : nil,
    ].compactMap(\.self)
}

/// The rows of a discovered title's box of facts, as Seerr's, each a label
/// and its values, the language and studios opening the popular titles
/// alike; allStudios shows every studio rather than the first few.
@MainActor
public func titleFactRows(_ title: TitleInfo, serverId: String, discovery: Discovery, allStudios: Bool)
    -> [(String, [FactValue])]
{
    let series = title.mediaKind == .series
    let plain = { (s: String?) -> [FactValue] in s.map { [FactValue(text: $0)] } ?? [] }
    // A date kept on one line.
    let unbroken = { (s: String) in s.replacingOccurrences(of: " ", with: "\u{00A0}") }
    var rows: [(String, [FactValue])] = []
    if !title.originalTitle.isEmpty, title.originalLanguage != discoverLanguage {
        rows.append((MavioStrings.itemOriginalTitle, plain(title.originalTitle)))
    }
    rows.append((MavioStrings.itemStatus, plain(title.productionStatus.text)))
    let releases =
        series
        ? []
        : [
            dayText(title.theatricalReleaseDate).map { "\(MavioStrings.discoverTheatricalRelease)  \(unbroken($0))" },
            dayText(title.digitalReleaseDate).map { "\(MavioStrings.itemDigitalRelease)  \(unbroken($0))" },
            dayText(title.physicalReleaseDate).map { "\(MavioStrings.itemPhysicalRelease)  \(unbroken($0))" },
        ].compactMap(\.self)
    if !releases.isEmpty {
        rows.append(
            (
                releases.count == 1 ? MavioStrings.discoverReleaseDate : MavioStrings.discoverReleaseDates,
                releases.map { FactValue(text: $0) }
            ))
    } else {
        rows.append(
            (series ? MavioStrings.itemFirstAired : MavioStrings.itemReleased, plain(dayText(title.premiereDate))))
    }
    if series, title.productionStatus == .ended {
        rows.append((MavioStrings.itemLastAired, plain(dayText(title.endDate))))
    }
    if series { rows.append((MavioStrings.itemNextAirDate, plain(dayText(title.nextAirDate)))) }
    if title.runtimeMinutes > 0 {
        rows.append(
            (
                series ? MavioStrings.discoverEpisodeRuntime : MavioStrings.itemRuntime,
                plain(minutesText(TimeInterval(title.runtimeMinutes) * 60))
            ))
    }
    rows.append((MavioStrings.itemRevenue, plain(moneyText(title.revenue))))
    rows.append((MavioStrings.itemBudget, plain(moneyText(title.budget))))
    if !title.originalLanguage.isEmpty {
        let name = languageText(title.originalLanguage)
        let query = discovery.alike(serverId, title.mediaKind, name) { $0.originalLanguage = title.originalLanguage }
        rows.append((MavioStrings.itemOriginalLanguage, [FactValue(text: name, query: query)]))
    }
    let countries =
        title.productionCountries.isEmpty ? title.productionLocations : title.productionCountries.map(countryText)
    rows.append((MavioStrings.itemCountry, countries.map { FactValue(text: $0) }))
    let studios = titleStudios(title)
    let shown = allStudios ? studios : Array(studios.prefix(shownStudios))
    rows.append(
        (
            studiosLabel(title),
            shown.map { s in
                FactValue(
                    text: s.name,
                    query: s.id.isEmpty
                        ? nil
                        : discovery.alike(serverId, title.mediaKind, s.name) {
                            if series { $0.networks = [s] } else { $0.studios = [s] }
                        })
            }
        ))
    return rows.filter { !$0.1.isEmpty }
}

/// The label of a title's studios, a series' networks.
public func studiosLabel(_ title: TitleInfo) -> String {
    title.mediaKind == .series ? MavioStrings.itemNetworks : MavioStrings.itemStudio
}

/// Whether a title has more studios than show at first.
public func hasMoreStudios(_ title: TitleInfo) -> Bool {
    max(title.studioRefs.count, title.studios.count) > shownStudios
}

/// Six of a title's crew, a series' creators first, then by the jobs
/// Seerr shows first.
public func leadingCrew(_ title: TitleInfo) -> [TitleCredit] {
    Array(
        title.credits.enumerated()
            .filter { !$0.element.isCast }
            .sorted { (crewRank($0.element), $0.offset) < (crewRank($1.element), $1.offset) }
            .map(\.element)
            .prefix(6))
}

extension TitleCredit {
    /// The job the leading crew names them by.
    public var jobLabel: String { kind.job == .other && !role.isEmpty ? role : kind.job.label }
}

/// A series' seasons as Seerr lists them: the latest first, specials last.
public func seasonOrder(_ seasons: [Mavio_Acquisition_V1_SeasonStatus]) -> [Mavio_Acquisition_V1_SeasonStatus] {
    seasons.sorted {
        ($0.seasonNumber == 0 ? 1 : 0, -$0.seasonNumber) < ($1.seasonNumber == 0 ? 1 : 0, -$1.seasonNumber)
    }
}

/// A season's name, Specials for season 0.
public func seasonName(_ number: Int32) -> String {
    number == 0 ? MavioStrings.discoverSpecials : MavioStrings.discoverSeason(number: String(number))
}

/// What a discovered title's card says under its name: its year and kind.
public func titleCardSubtitle(_ title: DiscoveredTitle) -> String {
    [
        title.year > 0 ? String(title.year) : nil,
        title.mediaKind == .series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
    ]
    .compactMap(\.self).joined(separator: " · ")
}

/// The name of a catalog's order.
public func discoverSortName(_ sort: Mavio_Plugin_V1_DiscoverSort) -> String {
    switch sort {
    case .popularity: MavioStrings.discoverSortPopularity
    case .releaseDate: MavioStrings.discoverSortReleaseDate
    case .rating: MavioStrings.discoverSortRating
    case .voteCount: MavioStrings.discoverSortVotes
    case .title: MavioStrings.discoverSortTitle
    default: MavioStrings.discoverSortDefault
    }
}

/// The country of the streaming services until another is chosen, the one
/// with the most of them.
public let defaultWatchRegion = "US"

/// The languages and countries a catalog's filters choose from, by their
/// codes, named in the app's language.
public enum DiscoverLocales {
    private static let locale = Locale(identifier: MavioStrings.bundle.preferredLocalizations.first ?? "en")

    /// ISO 639-1 languages.
    public static let languages: [(String, String)] = Locale.LanguageCode.isoLanguageCodes
        .map(\.identifier)
        .filter { $0.count == 2 }
        .compactMap { code in locale.localizedString(forLanguageCode: code).map { (code, $0) } }
        .sorted { $0.1.localizedCompare($1.1) == .orderedAscending }

    /// ISO 3166-1 countries.
    public static let countries: [(String, String)] = Locale.Region.isoRegions
        .map(\.identifier)
        .filter { $0.count == 2 && $0.allSatisfy(\.isLetter) }
        .compactMap { code in locale.localizedString(forRegionCode: code).map { (code, $0) } }
        .sorted { $0.1.localizedCompare($1.1) == .orderedAscending }
}

extension AppModel {
    /// The values of a catalog's filter its provider offers: those found by
    /// query, or in region for streaming services.
    public func filterValues(
        serverId: String, catalog: String, filter: Mavio_Plugin_V1_DiscoverFilter, mediaKind: MediaKind,
        query: String = "", region: String = ""
    ) async throws -> [Mavio_Plugin_V1_FilterValue] {
        var r = Mavio_Acquisition_V1_ListFilterValuesRequest()
        r.catalog = catalog
        r.filter = filter
        r.mediaKind = mediaKind
        r.query = query
        r.language = discoverLanguage
        r.region = region
        let response: Mavio_Acquisition_V1_ListFilterValuesResponse = try await client.call(
            serverId, Procedure.discover("ListFilterValues"), r)
        return response.values
    }

    /// A discovered person by their TMDB ID, with the titles they play in
    /// and work on.
    public func discoveredPerson(_ serverId: String, _ tmdbId: Int32) async throws -> Mavio_Acquisition_V1_Person {
        var r = Mavio_Acquisition_V1_GetPersonRequest()
        r.tmdbID = tmdbId
        r.language = discoverLanguage
        return try await client.call(
            serverId, Procedure.discover("GetPerson"), r, as: Mavio_Acquisition_V1_GetPersonResponse.self
        ).person
    }

    /// The ratings of a discovered title.
    public func ratings(_ serverId: String, _ kind: MediaKind, _ tmdbId: Int32) async throws -> [Rating] {
        var r = Mavio_Acquisition_V1_GetRatingsRequest()
        r.mediaKind = kind
        r.tmdbID = tmdbId
        return try await client.call(
            serverId, Procedure.discover("GetRatings"), r, as: Mavio_Acquisition_V1_GetRatingsResponse.self
        ).ratings
    }

    /// The episodes of a season of a discovered series.
    public func season(_ serverId: String, _ tmdbId: Int32, _ number: Int32) async throws
        -> [Mavio_Acquisition_V1_SeasonEpisode]
    {
        var r = Mavio_Acquisition_V1_GetSeasonRequest()
        r.tmdbID = tmdbId
        r.seasonNumber = number
        r.language = discoverLanguage
        return try await client.call(
            serverId, Procedure.discover("GetSeason"), r, as: Mavio_Acquisition_V1_GetSeasonResponse.self
        ).episodes
    }
}
