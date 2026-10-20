import MavioKit
import MavioUI
import SwiftUI

/// The locale of TMDB's pages, as "en-US".
private var webLocale: String {
    (MavioStrings.bundle.preferredLocalizations.first ?? "en").replacingOccurrences(of: "_", with: "-")
}

/// A movie's minutes, as "1h 52m".
private func minutes(_ n: Int32) -> String { minutesText(TimeInterval(n) * 60) }

/// A discovered title (docs/clients.md §6.3.2), as Seerr's page, without
/// requests but on the Mac.
struct TitleView: View {
    let serverId: String
    let kind: MediaKind
    let tmdbId: Int32
    @Environment(AppModel.self) private var model
    @State private var title: TitleInfo?
    @State private var failure: String?

    var body: some View {
        Group {
            if let title {
                TitleContent(serverId: serverId, title: title, reload: load)
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .task(id: model.revision(serverId)) { await load() }
    }

    private func load() async {
        do {
            title = try await model.title(serverId, kind, tmdbId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if title == nil { failure = error.reason }
        }
    }
}

private struct TitleContent: View {
    let serverId: String
    let title: TitleInfo
    /// Reads the title again, as pulling the page down does.
    let reload: () async -> Void
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.openURL) private var openURL
    @State private var ratings: [Rating] = []
    @State private var refreshes = 0
    #if os(macOS)
        /// The user signed in, whose role the requests' actions follow.
        @State private var user: Mavio_User_V1_User?
    #endif

    private var phone: Bool { sizeClass != .regular }
    private var series: Bool { title.mediaKind == .series }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                header
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    TitleOverview(serverId: serverId, title: title, ratings: ratings)
                    if series, !title.seasons.isEmpty {
                        DiscoveredSeasons(serverId: serverId, tmdbId: title.tmdbID, seasons: title.seasons)
                    }
                    cast
                    ForEach(discovery.related(title.mediaKind), id: \.id) { c in
                        CatalogShelf(
                            query: CatalogQuery(
                                serverId: serverId, catalog: c.id, kind: title.mediaKind, tmdbId: title.tmdbID,
                                title: c.kind == .recommendations
                                    ? MavioStrings.discoverRecommendations : MavioStrings.discoverSimilar),
                            refreshes: refreshes)
                    }
                }
            }
            .padding(.bottom, MavioSpacing.s40)
        }
        .pullToRefresh {
            await reload()
            refreshes += 1
        }
        .background(theme.colors.background)
        .navigationTitle(title.name)
        .modifier(HeroChrome(title: title.name, phone: phone))
        .task(id: title.tmdbID) {
            var r = Mavio_Acquisition_V1_GetRatingsRequest()
            r.mediaKind = title.mediaKind
            r.tmdbID = title.tmdbID
            ratings =
                (try? await model.client.call(
                    serverId, Procedure.discover("GetRatings"), r, as: Mavio_Acquisition_V1_GetRatingsResponse.self))?
                .ratings ?? []
        }
        #if os(macOS)
            .task(id: serverId) { user = await RequestsAPI.currentUser(model, serverId) }
        #endif
    }

    // MARK: Header

    /// The backdrop, and over its lower left the name, the facts, the
    /// status and the actions; beside them on an iPad the poster.
    private var header: some View {
        Color.clear
            .frame(maxWidth: .infinity)
            .modifier(HeaderSize(phone: phone))
            .background { artwork }
            .overlay {
                if phone {
                    LinearGradient(
                        stops: [
                            .init(color: .black, location: 0), .init(color: .black.opacity(0.5), location: 0.5),
                            .init(color: .clear, location: 1),
                        ],
                        startPoint: .bottom, endPoint: .top)
                } else {
                    LinearGradient(
                        stops: [
                            .init(color: .black.opacity(0.82), location: 0),
                            .init(color: .black.opacity(0.45), location: 0.45),
                            .init(color: .clear, location: 0.75),
                        ],
                        startPoint: .leading, endPoint: .trailing)
                    LinearGradient(
                        stops: [.init(color: .black.opacity(0.75), location: 0), .init(color: .clear, location: 0.55)],
                        startPoint: .bottom, endPoint: .top)
                }
            }
            .overlay(alignment: .bottomLeading) {
                HStack(alignment: .bottom, spacing: MavioSpacing.s40) {
                    if !phone, let poster = sized(title.posterURL, 500) {
                        ArtworkView(poster, title: title.name)
                            .aspectRatio(2 / 3, contentMode: .fit)
                            .frame(width: 200)
                            .clipShape(.rect(cornerRadius: MavioRadius.lg))
                            .shadow(color: .black.opacity(0.36), radius: 16, y: 12)
                    }
                    headerText.frame(maxWidth: 640, alignment: .leading)
                }
                .padding(.horizontal, phone ? theme.layout.margin : MavioSpacing.s40)
                .padding(.bottom, phone ? MavioSpacing.s24 : MavioSpacing.s40)
            }
            .clipped()
            .heroBleed(phone: phone, aspect: 1) { artwork }
            .environment(\.colorScheme, .dark)
    }

    @ViewBuilder private var artwork: some View {
        if let backdrop = sized(title.backdropURL, 1280) {
            ArtworkView(backdrop, icon: series ? "shows" : "movies")
        } else if let poster = sized(title.posterURL, 342) {
            // Without a backdrop the poster, blurred, colors the header.
            ArtworkView(poster)
                .scaleEffect(1.25)
                .blur(radius: 48)
                .opacity(0.7)
                .background(theme.colors.surface)
        } else {
            theme.colors.surface
        }
    }

    /// As Seerr's: the year, kind, rating and runtime, then the genres.
    private var facts: [String] {
        [
            title.year > 0 ? String(title.year) : nil,
            series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
            title.officialRating.isEmpty ? nil : title.officialRating,
            title.runtimeMinutes > 0 ? minutes(title.runtimeMinutes) : nil,
        ].compactMap(\.self)
    }

    private var headerText: some View {
        let genres =
            title.genreRefs.isEmpty
            ? title.genres.map { NamedValue(id: "", name: $0) }
            : title.genreRefs.map { NamedValue(id: $0.id, name: $0.name) }
        let shown = Array(genres.prefix(3))
        return VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            Text(title.name)
                .font(.system(size: phone ? 34 : 52, weight: .bold))
                .foregroundStyle(.white)
            Flow(spacing: 0, lineSpacing: 6) {
                Text(facts.joined(separator: " · ") + (shown.isEmpty ? "" : " · "))
                ForEach(Array(shown.enumerated()), id: \.offset) { i, g in
                    let label = Text(g.name + (i < shown.count - 1 ? " / " : ""))
                    if !g.id.isEmpty,
                        let query = discovery.alike(serverId, title.mediaKind, g.name, { $0.genres = [g.id] })
                    {
                        NavigationLink(value: Route.catalog(query)) { label }.buttonStyle(.plain)
                    } else {
                        label
                    }
                }
                if let status = statusLabel(title.status) {
                    Text(status)
                        .type(theme.type.caption).fontWeight(.semibold)
                        .padding(.horizontal, 6)
                        .background(.white.opacity(0.2), in: .rect(cornerRadius: 4))
                        .padding(.leading, MavioSpacing.s12)
                }
                #if os(macOS)
                    if user?.admin == true, !title.itemID.isEmpty {
                        RequestsDownloadBadge(serverId: serverId, itemId: title.itemID)
                    }
                #endif
            }
            .type(theme.type.callout)
            .foregroundStyle(.white.opacity(0.8))
            #if os(macOS)
                RequestsTitleActions(serverId: serverId, title: title, user: user, reload: reload) { actions }
                    .padding(.top, MavioSpacing.s4)
            #else
                actions.padding(.top, MavioSpacing.s4)
            #endif
        }
    }

    /// Watch, when a library has it, and the trailer.
    private var actions: some View {
        HStack(spacing: MavioSpacing.s12) {
            if !title.itemID.isEmpty {
                NavigationLink(value: Route.item(serverId: serverId, itemId: title.itemID)) {
                    Label {
                        Text(MavioStrings.discoverWatch).lineLimit(1)
                    } icon: {
                        Icon("play", fill: true, size: 20)
                    }
                    .frame(maxWidth: phone ? .infinity : nil)
                }
                .buttonStyle(MavioButtonStyle(.artworkPrimary, large: phone))
            }
            if let url = title.trailerUrls.first.flatMap(URL.init(string:)) {
                Button {
                    openURL(url)
                } label: {
                    Label {
                        Text(MavioStrings.itemTrailer).lineLimit(1)
                    } icon: {
                        Icon("movies", size: 20)
                    }
                    .frame(maxWidth: phone ? .infinity : nil)
                }
                .buttonStyle(MavioButtonStyle(.artworkSecondary, large: phone))
            }
        }
    }

    // MARK: Cast

    /// The first twenty of the cast, each opening their page, with See All
    /// opening the whole cast and crew.
    @ViewBuilder private var cast: some View {
        let shown = title.credits.filter(\.isCast).sorted { $0.order < $1.order }.prefix(20)
        if !shown.isEmpty {
            Shelf(MavioStrings.itemCast) {
                Spacer()
                NavigationLink(
                    MavioStrings.creditsAll,
                    value: Route.titleCredits(
                        serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbID, crew: false)
                )
                .type(theme.type.callout)
            } content: {
                ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                    TitlePersonCard(serverId: serverId, credit: c)
                }
            }
        }
    }
}

/// A person credited on a discovered title, opening their page when TMDB
/// knows them.
struct TitlePersonCard: View {
    let serverId: String
    let credit: TitleCredit

    var body: some View {
        let card = PersonCard(name: credit.name, photo: sized(credit.imageURL, 185), role: credit.shownRole)
        if credit.tmdbId > 0 {
            NavigationLink(value: Route.discoveredPerson(serverId: serverId, tmdbId: credit.tmdbId)) { card }
                .buttonStyle(.plain)
        } else {
            card
        }
    }
}

// MARK: Overview

/// The overview and facts of a discovered title, in two columns where they
/// fit, as Seerr's: the tagline, overview, leading crew and keywords beside
/// its collection, ratings and facts, each genre, keyword, studio, network
/// and language opening the popular titles alike.
private struct TitleOverview: View {
    let serverId: String
    let title: TitleInfo
    let ratings: [Rating]
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass

    var body: some View {
        let layout =
            sizeClass == .regular
            ? AnyLayout(HStackLayout(alignment: .top, spacing: MavioSpacing.s32))
            : AnyLayout(VStackLayout(alignment: .leading, spacing: MavioSpacing.s32))
        layout {
            OverviewLeading(serverId: serverId, title: title)
                .frame(maxWidth: .infinity, alignment: .leading)
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                CollectionCard(serverId: serverId, title: title)
                TitleFacts(serverId: serverId, title: title, ratings: ratings)
            }
            .frame(maxWidth: sizeClass == .regular ? 380 : .infinity)
        }
        .padding(.horizontal, theme.layout.margin)
        .padding(.top, MavioSpacing.s24)
        .padding(.bottom, MavioSpacing.s12)
    }
}

private struct OverviewLeading: View {
    let serverId: String
    let title: TitleInfo
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme

    /// Six of the crew, a series' creators first, then by the jobs Seerr shows first.
    private var crew: [TitleCredit] {
        Array(
            title.credits.enumerated()
                .filter { !$0.element.isCast }
                .sorted { (crewRank($0.element), $0.offset) < (crewRank($1.element), $1.offset) }
                .map(\.element)
                .prefix(6))
    }

    var body: some View {
        let keywords =
            title.keywordRefs.isEmpty
            ? title.keywords.map { NamedValue(id: "", name: $0) }
            : title.keywordRefs.map { NamedValue(id: $0.id, name: $0.name) }
        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
            if !title.tagline.isEmpty {
                Text(title.tagline).type(theme.type.title3).fontWeight(.regular).italic()
                    .foregroundStyle(theme.colors.textSecondary)
            }
            Text(MavioStrings.discoverOverview).type(theme.type.title3)
            Text(title.overview.isEmpty ? MavioStrings.discoverNoOverview : title.overview).type(theme.type.body)
            if !crew.isEmpty {
                LazyVGrid(
                    columns: Array(
                        repeating: GridItem(.flexible(), spacing: MavioSpacing.s24, alignment: .topLeading), count: 2),
                    alignment: .leading, spacing: MavioSpacing.s16
                ) {
                    ForEach(Array(crew.enumerated()), id: \.offset) { _, c in
                        VStack(alignment: .leading, spacing: 0) {
                            Text(c.kind.job == .other && !c.role.isEmpty ? c.role : c.kind.job.label)
                                .type(theme.type.headline)
                            let name = Text(c.name).type(theme.type.callout)
                                .foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                            if c.tmdbId > 0 {
                                NavigationLink(value: Route.discoveredPerson(serverId: serverId, tmdbId: c.tmdbId)) {
                                    name
                                }
                                .buttonStyle(.plain)
                            } else {
                                name
                            }
                        }
                    }
                }
                .padding(.top, MavioSpacing.s8)
                NavigationLink(
                    value: Route.titleCredits(
                        serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbID, crew: true)
                ) {
                    HStack(spacing: MavioSpacing.s4) {
                        Text(MavioStrings.discoverFullCrew)
                        Icon("forward", size: 16)
                    }
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                }
                .buttonStyle(.plain)
                .frame(maxWidth: .infinity, alignment: .trailing)
            }
            if !keywords.isEmpty {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                    ForEach(keywords, id: \.self) { k in
                        let pill = Text(k.name).type(theme.type.footnote)
                            .foregroundStyle(theme.colors.textPrimary)
                            .padding(.horizontal, MavioSpacing.s12)
                            .padding(.vertical, MavioSpacing.s4)
                            .background(theme.colors.fill, in: Capsule())
                        if !k.id.isEmpty,
                            let query = discovery.alike(serverId, title.mediaKind, k.name, { $0.keywords = [k] })
                        {
                            NavigationLink(value: Route.catalog(query)) { pill }.buttonStyle(.plain)
                        } else {
                            pill
                        }
                    }
                }
                .padding(.top, MavioSpacing.s8)
            }
        }
    }
}

/// A movie's collection as Seerr's card: its backdrop and name, opening
/// its page.
private struct CollectionCard: View {
    let serverId: String
    let title: TitleInfo
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var collection: Mavio_Acquisition_V1_Collection?

    private var id: Int32 { Int32(title.externalIds["tmdb_collection"] ?? "") ?? 0 }

    var body: some View {
        if id > 0 {
            NavigationLink(value: Route.discoveredCollection(serverId: serverId, tmdbId: id)) {
                HStack(spacing: MavioSpacing.s12) {
                    Text(
                        collection.map(\.name).flatMap { $0.isEmpty ? nil : $0 }
                            ?? (title.collectionName.isEmpty ? MavioStrings.itemCollection : title.collectionName)
                    )
                    .type(theme.type.headline).lineLimit(1)
                    Spacer()
                    Text(MavioStrings.collectionView)
                        .type(theme.type.footnote).fontWeight(.semibold)
                        .padding(.horizontal, MavioSpacing.s12)
                        .padding(.vertical, MavioSpacing.s4)
                        .background(.white.opacity(0.2), in: Capsule())
                }
                .foregroundStyle(.white)
                .padding(MavioSpacing.s16)
                .frame(maxWidth: .infinity, minHeight: 80)
                .background {
                    ZStack {
                        theme.colors.surfaceRaised
                        if let c = collection, let art = sized(c.backdropURL.isEmpty ? c.posterURL : c.backdropURL, 780)
                        {
                            ArtworkView(art)
                        }
                        LinearGradient(
                            colors: [.black.opacity(0.45), .black.opacity(0.8)], startPoint: .top, endPoint: .bottom)
                    }
                }
                .clipShape(.rect(cornerRadius: 14))
            }
            .buttonStyle(.plain)
            .task(id: id) { collection = try? await model.collection(serverId, id) }
        }
    }
}

/// A fact's value: plain, or opening the popular titles alike.
private struct FactValue: Hashable {
    let text: String
    var query: CatalogQuery?
}

/// The facts of a discovered title in a bordered box, as Seerr's: the
/// ratings, then a fact a row, the services streaming it and the links
/// elsewhere.
private struct TitleFacts: View {
    let serverId: String
    let title: TitleInfo
    let ratings: [Rating]
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    @State private var allStudios = false

    private static let minStudios = 5
    private var series: Bool { title.mediaKind == .series }

    private var rows: [(String, [FactValue])] {
        let plain = { (s: String?) -> [FactValue] in s.map { [FactValue(text: $0)] } ?? [] }
        var rows: [(String, [FactValue])] = []
        if !title.originalTitle.isEmpty, title.originalLanguage != discoverLanguage {
            rows.append((MavioStrings.itemOriginalTitle, plain(title.originalTitle)))
        }
        rows.append((MavioStrings.itemStatus, plain(title.productionStatus.text)))
        let releases =
            series
            ? []
            : [
                dayText(title.theatricalReleaseDate).map {
                    "\(MavioStrings.discoverTheatricalRelease)  \(unbroken($0))"
                },
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
                    plain(minutes(title.runtimeMinutes))
                ))
        }
        rows.append((MavioStrings.itemRevenue, plain(moneyText(title.revenue))))
        rows.append((MavioStrings.itemBudget, plain(moneyText(title.budget))))
        if !title.originalLanguage.isEmpty {
            let name = languageText(title.originalLanguage)
            rows.append(
                (
                    MavioStrings.itemOriginalLanguage,
                    [
                        FactValue(
                            text: name,
                            query: discovery.alike(serverId, title.mediaKind, name) {
                                $0.originalLanguage = title.originalLanguage
                            })
                    ]
                ))
        }
        let countries =
            title.productionCountries.isEmpty ? title.productionLocations : title.productionCountries.map(countryText)
        rows.append((MavioStrings.itemCountry, countries.map { FactValue(text: $0) }))
        let studios =
            title.studioRefs.isEmpty
            ? title.studios.map { NamedValue(id: "", name: $0) }
            : title.studioRefs.map { NamedValue(id: $0.id, name: $0.name) }
        let shown = allStudios ? studios : Array(studios.prefix(Self.minStudios))
        rows.append(
            (
                series ? MavioStrings.itemNetworks : MavioStrings.itemStudio,
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

    var body: some View {
        VStack(spacing: 0) {
            TitleRatings(
                kind: title.mediaKind, tmdbId: title.tmdbID, ratings: ratings,
                score: title.voteCount > 0 ? title.communityRating : nil
            )
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, 10)
            ForEach(rows, id: \.0) { label, values in
                Divider().overlay(theme.colors.border)
                HStack(alignment: .top, spacing: MavioSpacing.s16) {
                    Text(label).type(theme.type.callout).fontWeight(.semibold)
                    Spacer(minLength: 0)
                    VStack(alignment: .trailing, spacing: 2) {
                        ForEach(values, id: \.self) { v in
                            if let query = v.query {
                                NavigationLink(value: Route.catalog(query)) { Text(v.text) }.buttonStyle(.plain)
                            } else {
                                Text(v.text)
                            }
                        }
                        if label == (series ? MavioStrings.itemNetworks : MavioStrings.itemStudio),
                            max(title.studioRefs.count, title.studios.count) > Self.minStudios
                        {
                            Button {
                                withAnimation { allStudios.toggle() }
                            } label: {
                                HStack(spacing: MavioSpacing.s4) {
                                    Text(allStudios ? MavioStrings.actionShowLess : MavioStrings.discoverShowMore)
                                    Icon("expand", size: 14).rotationEffect(.degrees(allStudios ? 180 : 0))
                                }
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .multilineTextAlignment(.trailing)
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                }
                .padding(.horizontal, MavioSpacing.s16)
                .padding(.vertical, 10)
            }
            if !title.watchProviders.isEmpty {
                Divider().overlay(theme.colors.border)
                WatchProviders(providers: title.watchProviders)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, MavioSpacing.s16)
                    .padding(.vertical, 10)
            }
            Divider().overlay(theme.colors.border)
            ExternalLinks(
                links: titleLinks(title, ratings),
                item: title.itemID.isEmpty ? nil : Route.item(serverId: serverId, itemId: title.itemID)
            )
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, MavioSpacing.s12)
        }
        .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(theme.colors.border))
    }

    /// A date kept on one line.
    private func unbroken(_ s: String) -> String {
        s.replacingOccurrences(of: " ", with: "\u{00A0}")
    }
}

/// The services streaming a title, by their logos.
struct WatchProviders: View {
    let providers: [Mavio_Plugin_V1_WatchProvider]
    @Environment(\.theme) private var theme

    var body: some View {
        if !providers.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(MavioStrings.itemStreaming).type(theme.type.callout).fontWeight(.semibold)
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                    ForEach(providers, id: \.name) { p in
                        if let logo = p.logoURL.isEmpty ? nil : p.logoURL {
                            ArtworkView(logo, title: p.name)
                                .frame(width: 32, height: 32)
                                .clipShape(.rect(cornerRadius: 6))
                                .opacity(0.7)
                                .accessibilityLabel(p.name)
                        } else {
                            Text(p.name).type(theme.type.footnote)
                        }
                    }
                }
            }
        }
    }
}

/// Rotten Tomatoes' least fresh score, and least upright audience score.
private let freshScore = 60.0

/// A title's ratings as Seerr's: Rotten Tomatoes' and IMDb's, then TMDB's
/// score when given, each opening its page.
struct TitleRatings: View {
    let kind: MediaKind
    let tmdbId: Int32
    let ratings: [Rating]
    let score: Double?
    var alignment: Alignment = .center
    @Environment(\.theme) private var theme

    /// The ratings shown, in Seerr's order.
    static func shown(_ ratings: [Rating]) -> [Rating] {
        let order: [Mavio_Plugin_V1_RatingSource] = [.rottenTomatoesCritics, .rottenTomatoesAudience, .imdb]
        return order.compactMap { source in ratings.first { $0.source == source } }
    }

    var body: some View {
        let shown = Self.shown(ratings)
        Flow(spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s4) {
            ForEach(shown, id: \.source) { r in
                let fresh = r.score >= freshScore
                let (logo, label, value): (String, String, String) =
                    switch r.source {
                    case .imdb:
                        (
                            "imdb",
                            MavioStrings.discoverImdbScore(
                                votes: r.votes.formatted(
                                    .number.notation(.compactName).precision(.fractionLength(0...1)))),
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
                rating(logo, label, value, URL(string: r.url), height: 20)
            }
            if let score {
                rating(
                    "tmdb-wide", MavioStrings.discoverTmdbScore, "\(Int((score * 10).rounded()))%",
                    URL(string: tmdbURL(kind, tmdbId)), height: 16)
            }
        }
        .frame(maxWidth: .infinity, alignment: alignment)
    }

    @ViewBuilder private func rating(_ logo: String, _ label: String, _ value: String, _ url: URL?, height: CGFloat)
        -> some View
    {
        let content = HStack(spacing: 6) {
            Image.service(logo).resizable().scaledToFit().frame(height: height)
            Text(value).type(theme.type.callout).fontWeight(.semibold).foregroundStyle(theme.colors.textPrimary)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(label) \(value)")
        if let url {
            Link(destination: url) { content }
        } else {
            content
        }
    }
}

/// A title's page on TMDB, in the app's language.
private func tmdbURL(_ kind: MediaKind, _ tmdbId: Int32) -> String {
    "https://www.themoviedb.org/\(kind == .series ? "tv" : "movie")/\(tmdbId)?language=\(webLocale)"
}

/// A link to a title or person elsewhere, with the site's logo.
struct ExternalLink: Hashable {
    let name: String
    let url: URL
    /// The logo, by `Image.service`.
    let logo: String
    /// The logo's width, as Seerr's.
    let width: CGFloat
}

/// The logos linking a title elsewhere, as Seerr's, Rotten Tomatoes' once
/// its ratings tell its page.
private func titleLinks(_ title: TitleInfo, _ ratings: [Rating]) -> [ExternalLink] {
    titleLinks(
        kind: title.mediaKind, tmdbId: title.tmdbID, imdb: title.externalIds["imdb"] ?? "",
        tvdb: title.externalIds["tvdb"] ?? (title.tvdbID > 0 ? String(title.tvdbID) : ""), ratings: ratings)
}

/// The logos linking a title elsewhere by its IDs.
func titleLinks(kind: MediaKind, tmdbId: Int32, imdb: String, tvdb: String, ratings: [Rating]) -> [ExternalLink] {
    let series = kind == .series
    let rt = ratings.first { $0.source != .imdb && !$0.url.isEmpty }?.url
    let links: [(String, String?, String, CGFloat)] = [
        ("TMDB", tmdbURL(kind, tmdbId), "tmdb", 32),
        ("TVDB", series && !tvdb.isEmpty ? "https://www.thetvdb.com/dereferrer/series/\(tvdb)" : nil, "tvdb", 36),
        ("IMDb", imdb.isEmpty ? nil : "https://www.imdb.com/title/\(imdb)", "imdb", 32),
        ("Rotten Tomatoes", rt, "rt", 56),
        ("Trakt", imdb.isEmpty ? nil : "https://trakt.tv/\(series ? "shows" : "movies")/\(imdb)", "trakt", 32),
        (
            "Simkl",
            imdb.isEmpty
                ? nil
                : "https://api.simkl.com/redirect?to=Simkl&imdb=\(imdb.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? imdb)",
            "simkl", 32
        ),
        ("Letterboxd", series ? nil : "https://letterboxd.com/tmdb/\(tmdbId)", "letterboxd", 32),
    ]
    return links.compactMap { name, url, logo, width in
        url.flatMap(URL.init(string:)).map { ExternalLink(name: name, url: $0, logo: logo, width: width) }
    }
}

/// The logos linking a title or person elsewhere, as Seerr's, faint;
/// first, Mavio's opening its item when the server has it.
struct ExternalLinks: View {
    let links: [ExternalLink]
    var item: Route?
    var alignment: HorizontalAlignment = .center
    @Environment(\.theme) private var theme

    var body: some View {
        Flow(spacing: MavioSpacing.s20, lineSpacing: MavioSpacing.s12) {
            if let item {
                NavigationLink(value: item) {
                    Wordmark(height: 14)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(MavioStrings.discoverOpenItem)
            }
            ForEach(links, id: \.self) { l in
                Link(destination: l.url) {
                    Image.service(l.logo).resizable().scaledToFit().frame(width: l.width, height: 24)
                        .foregroundStyle(theme.colors.textPrimary)
                }
                .accessibilityLabel(l.name)
            }
        }
        .opacity(0.5)
        .frame(maxWidth: .infinity, alignment: alignment == .center ? .center : .leading)
    }
}

// MARK: Seasons

/// A series' seasons with what the server has of each, as Seerr lists them:
/// the latest first, specials last, each expanding to its episodes.
private struct DiscoveredSeasons: View {
    let serverId: String
    let tmdbId: Int32
    let seasons: [Mavio_Acquisition_V1_SeasonStatus]
    @Environment(\.theme) private var theme

    var body: some View {
        let ordered = seasons.sorted {
            ($0.seasonNumber == 0 ? 1 : 0, -$0.seasonNumber) < ($1.seasonNumber == 0 ? 1 : 0, -$1.seasonNumber)
        }
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            Text(MavioStrings.itemSeasons).type(theme.type.title3)
            ForEach(ordered, id: \.seasonNumber) { s in
                DiscoveredSeason(serverId: serverId, tmdbId: tmdbId, season: s)
            }
        }
        .padding(.horizontal, theme.layout.margin)
        .padding(.vertical, MavioSpacing.s12)
    }
}

private struct DiscoveredSeason: View {
    let serverId: String
    let tmdbId: Int32
    let season: Mavio_Acquisition_V1_SeasonStatus
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var open = false
    @State private var episodes: [Mavio_Acquisition_V1_SeasonEpisode]?
    @State private var failed = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button {
                withAnimation { open.toggle() }
            } label: {
                HStack(spacing: MavioSpacing.s12) {
                    Text(
                        season.seasonNumber == 0
                            ? MavioStrings.discoverSpecials
                            : MavioStrings.discoverSeason(number: String(season.seasonNumber))
                    )
                    .type(theme.type.headline).lineLimit(1)
                    Spacer(minLength: 0)
                    if season.episodeCount > 0 {
                        Text(MavioStrings.itemEpisodeCount(count: Int(season.episodeCount)))
                            .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    }
                    if let status = statusLabel(season.status) {
                        Text(status).type(theme.type.footnote).fontWeight(.semibold)
                            .padding(.horizontal, 10).padding(.vertical, 2)
                            .background(theme.colors.fill, in: Capsule())
                    }
                    Icon("expand", size: 16).foregroundStyle(theme.colors.textSecondary)
                        .rotationEffect(.degrees(open ? 180 : 0))
                }
                .padding(.horizontal, MavioSpacing.s16)
                .frame(minHeight: 48)
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .accessibilityAddTraits(open ? .isSelected : [])
            if open { list }
        }
        .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(theme.colors.border))
    }

    @ViewBuilder private var list: some View {
        Group {
            if let episodes, !episodes.isEmpty {
                VStack(alignment: .leading, spacing: 0) {
                    ForEach(episodes.reversed(), id: \.episodeNumber) { e in
                        Divider().overlay(theme.colors.border)
                        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                            Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s4) {
                                Text(MavioStrings.discoverEpisodeTitle(number: String(e.episodeNumber), name: e.name))
                                    .type(theme.type.headline)
                                if let day = dayText(e.airDate) {
                                    Text(day).type(theme.type.footnote)
                                        .padding(.horizontal, 10).padding(.vertical, 2)
                                        .background(theme.colors.fill, in: Capsule())
                                }
                            }
                            if !e.overview.isEmpty {
                                Text(e.overview).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                            }
                            if let still = sized(e.stillURL, 300) {
                                ArtworkView(still).aspectRatio(16 / 9, contentMode: .fit)
                                    .clipShape(.rect(cornerRadius: MavioRadius.lg))
                            }
                        }
                        .padding(.vertical, MavioSpacing.s16)
                    }
                }
                .padding(.horizontal, MavioSpacing.s16)
            } else if episodes != nil || failed {
                Text(failed ? MavioStrings.stateError : MavioStrings.discoverNoEpisodes)
                    .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    .padding([.horizontal, .bottom], MavioSpacing.s16)
            } else {
                ProgressView().padding([.horizontal, .bottom], MavioSpacing.s16)
            }
        }
        .task {
            guard episodes == nil else { return }
            var r = Mavio_Acquisition_V1_GetSeasonRequest()
            r.tmdbID = tmdbId
            r.seasonNumber = season.seasonNumber
            r.language = discoverLanguage
            do {
                episodes =
                    try await model.client.call(
                        serverId, Procedure.discover("GetSeason"), r, as: Mavio_Acquisition_V1_GetSeasonResponse.self
                    ).episodes
            } catch {
                failed = !(error is CancellationError)
            }
        }
    }
}

// MARK: Credits

/// A discovered title's whole cast and crew, or its crew alone, as an
/// item's (libs/design/components.md, Credits Screen).
struct TitleCreditsView: View {
    let serverId: String
    let kind: MediaKind
    let tmdbId: Int32
    let crew: Bool
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var credits: [TitleCredit]?
    @State private var failure: String?

    var body: some View {
        Group {
            if let credits {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s24) {
                        if credits.isEmpty {
                            Text(MavioStrings.creditsNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                                .padding(.horizontal, theme.layout.margin)
                        }
                        ForEach([CreditJob.actor, .guestStar] + CreditJob.crewOrder, id: \.self) { job in
                            people(job.label, credits.filter { $0.kind.job == job })
                        }
                    }
                    .padding(.vertical, MavioSpacing.s16)
                }
                .pullToRefresh { await load() }
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle(crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast)
        .largeTitle()
        .task { await load() }
    }

    @ViewBuilder private func people(_ title: String, _ shown: [TitleCredit]) -> some View {
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Text(title).type(theme.type.title3)
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 104), spacing: MavioSpacing.s16, alignment: .top)],
                    spacing: MavioSpacing.s24
                ) {
                    ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                        TitlePersonCard(serverId: serverId, credit: c)
                    }
                }
            }
            .padding(.horizontal, theme.layout.margin)
        }
    }

    private func load() async {
        do {
            let all = try await model.title(serverId, kind, tmdbId).credits
            // The cast comes in billing order.
            credits = crew ? all.filter { !$0.isCast } : all
            failure = nil
        } catch is CancellationError {
        } catch {
            if credits == nil { failure = error.reason }
        }
    }
}
