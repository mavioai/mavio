import MavioKit
import MavioUI
import SwiftUI

/// Discover (docs/tv.md §6.3): a row of each catalog of the first server
/// that discovers, each ending with a See All card that opens its catalog.
/// Requests are left out, as on iOS (docs/clients.md §4.2).
struct DiscoverView: View {
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme

    var body: some View {
        Group {
            if let server = discovery.server {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s48) {
                        ForEach(discovery.rows, id: \.self) { row in
                            CatalogShelf(
                                query: CatalogQuery(
                                    serverId: server.id, catalog: row.catalog.id, kind: row.kind, title: row.title))
                        }
                    }
                    .padding(.top, navBarHeight)
                    .padding(.bottom, 120)
                }
                .reportsScroll()
            } else if discovery.loaded {
                StatusView(
                    icon: "discover", title: MavioStrings.discoverNotSetUp,
                    message: MavioStrings.discoverNotSetUpMessage)
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .ignoresSafeArea()
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// A row of a catalog's first page, ending with See All when it has more;
/// nothing when it has no titles.
struct CatalogShelf: View {
    let query: CatalogQuery
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var titles: [DiscoveredTitle]?
    @State private var more = false

    var body: some View {
        Group {
            if let titles {
                if !titles.isEmpty {
                    CardRow(title: query.title) {
                        ForEach(titles, id: \.key) {
                            DiscoveredTitleLink(serverId: query.serverId, title: $0, width: theme.layout.poster)
                        }
                        if more {
                            NavigationLink(value: Route.catalog(query)) {
                                SeeAllCard()
                            }
                            .buttonStyle(CardButtonStyle())
                        }
                    }
                }
            } else {
                CardRow(title: query.title) {
                    ForEach(0..<8, id: \.self) { _ in
                        CardPlaceholder().frame(width: theme.layout.poster)
                    }
                }
                .redacted(reason: .placeholder)
            }
        }
        .task(id: query) {
            guard let page = try? await model.discover(query, page: 1) else {
                if !Task.isCancelled, titles == nil { titles = [] }
                return
            }
            titles = page.titles
            more = page.page < page.totalPages
        }
    }
}

/// The See All card ending a row of Discover, a poster's size.
private struct SeeAllCard: View {
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            ZStack {
                theme.colors.surfaceRaised
                VStack(spacing: MavioSpacing.s12) {
                    Icon("forward", size: 48)
                    Text(MavioStrings.homeMore).type(theme.type.headline)
                }
                .foregroundStyle(theme.colors.textPrimary)
            }
            .aspectRatio(2 / 3, contentMode: .fit)
            .focusRing()
            Text(" ").type(theme.type.headline)
            Text(" ").type(theme.type.footnote)
        }
        .frame(width: theme.layout.poster)
    }
}

/// The card of a discovered title, which opens it.
struct DiscoveredTitleLink: View {
    let serverId: String
    let title: DiscoveredTitle
    var role: String?
    /// The width; nil fills the space offered, as in grids.
    var width: CGFloat?

    var body: some View {
        NavigationLink(value: Route.title(serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbId)) {
            DiscoveredTitleCard(title: title, role: role, width: width)
        }
        .buttonStyle(CardButtonStyle())
    }
}

/// A discovered title's poster with its status, then its name and its
/// year and kind, or a person's role in it.
private struct DiscoveredTitleCard: View {
    let title: DiscoveredTitle
    let role: String?
    let width: CGFloat?
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            ArtworkView(
                sized(title.posterURL, 342), title: title.name, icon: title.mediaKind == .series ? "shows" : "movies"
            )
            .aspectRatio(2 / 3, contentMode: .fit)
            .overlay(alignment: .topTrailing) { StatusMark(status: title.status) }
            .focusRing()
            VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                Text(title.name).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                Text(subtitle).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            }
            .lineLimit(1)
        }
        .frame(width: width)
    }

    private var subtitle: String {
        if let role, !role.isEmpty { return role }
        return [
            title.year > 0 ? String(title.year) : nil,
            title.mediaKind == .series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
        ].compactMap(\.self).joined(separator: " · ")
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
            Icon(available ? "check" : "pending", size: 24)
                .foregroundStyle(.white)
                .padding(MavioSpacing.s4)
                .background(available ? theme.colors.success : theme.colors.scrim, in: .circle)
                .opacity(status == .partiallyAvailable ? 0.7 : 1)
                .padding(MavioSpacing.s12)
                .accessibilityLabel(statusLabel(status) ?? "")
        }
    }
}

/// All the titles of a catalog, page by page, under its name; focus starts
/// on its first title (§6.3). Its Filters button, at the right of its name
/// and reached up from the first row, opens its filters in a dialog.
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
            VStack(alignment: .leading, spacing: MavioSpacing.s32) {
                HStack {
                    Text(query.title).type(theme.type.title1).fontWeight(.bold)
                        .foregroundStyle(theme.colors.textPrimary)
                    Spacer()
                    if filterable != nil {
                        let n = current.filterCount
                        Button {
                            filtering = true
                        } label: {
                            Label {
                                Text(n > 0 ? "\(MavioStrings.discoverFilters) (\(n))" : MavioStrings.discoverFilters)
                            } icon: {
                                Icon(n > 0 || current.sort != .unspecified ? "filterFill" : "filter", size: 28)
                            }
                        }
                        .buttonStyle(TVButtonStyle())
                    }
                }
                .focusSection()
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: theme.layout.poster), spacing: theme.layout.gutter)],
                    spacing: MavioSpacing.s48
                ) {
                    ForEach(Array(titles.enumerated()), id: \.element.key) { index, title in
                        DiscoveredTitleLink(serverId: current.serverId, title: title)
                            .onAppear { if index >= titles.count - 12 { Task { await load() } } }
                    }
                }
                if loading || (titles.isEmpty && next != nil && !failed) {
                    ProgressView().frame(maxWidth: .infinity)
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
            .padding(.horizontal, theme.layout.margin)
            .padding(.vertical, MavioSpacing.s48)
        }
        .ignoresSafeArea(edges: .horizontal)
        .toolbar(.hidden, for: .navigationBar)
        .fullScreenCover(isPresented: $filtering) {
            if let catalog = filterable {
                CatalogFiltersDialog(catalog: catalog, value: current) { filtered = $0 }
                    .presentationBackground(.clear)
            }
        }
        .task(id: current) {
            // Coming back from a title keeps the pages; other filters start again.
            if titles.isEmpty, next != nil {
                await load()
                return
            }
            guard shown != current else { return }
            titles = []
            next = 1
            failed = false
            await load()
        }
    }

    /// The query the pages are of.
    @State private var shown: CatalogQuery?

    private func load() async {
        guard let page = next, !loading else { return }
        loading = true
        failed = false
        defer { loading = false }
        do {
            let asked = current
            let response = try await model.discover(asked, page: page)
            guard asked == current else { return }
            if shown != asked {
                titles = []
                shown = asked
            }
            var seen = Set(titles.map(\.key))
            titles += response.titles.filter { seen.insert($0.key).inserted }
            next = response.page < response.totalPages ? response.page + 1 : nil
        } catch {
            failed = !(error is CancellationError)
        }
    }
}

// MARK: Title

/// A discovered title (§6.3), laid out as an item's details: the backdrop,
/// the name, the facts and genres, the actions, then the overview beside
/// the box of facts, a series' seasons, the cast, and the titles
/// recommended for and similar to it. Nothing opens a web page, so the
/// trailer and the links elsewhere are left out (§1).
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
                TitleContent(serverId: serverId, title: title)
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .toolbar(.hidden, for: .navigationBar)
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
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    @State private var ratings: [Rating] = []

    private var series: Bool { title.mediaKind == .series }

    var body: some View {
        GeometryReader { geo in
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s48) {
                    header.frame(height: geo.size.height * 0.86)
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
                                    ? MavioStrings.discoverRecommendations : MavioStrings.discoverSimilar))
                    }
                }
                .padding(.bottom, 120)
            }
        }
        .ignoresSafeArea()
        .task(id: title.tmdbID) {
            var r = Mavio_Acquisition_V1_GetRatingsRequest()
            r.mediaKind = title.mediaKind
            r.tmdbID = title.tmdbID
            ratings =
                (try? await model.client.call(
                    serverId, Procedure.discover("GetRatings"), r, as: Mavio_Acquisition_V1_GetRatingsResponse.self))?
                .ratings ?? []
        }
    }

    // MARK: Header

    private var header: some View {
        ZStack(alignment: .bottomLeading) {
            artwork
            LinearGradient(
                stops: [.init(color: .black.opacity(0.65), location: 0), .init(color: .clear, location: 0.65)],
                startPoint: .leading, endPoint: .trailing)
            LinearGradient(
                stops: [
                    .init(color: theme.colors.tvBackground, location: 0),
                    .init(color: theme.colors.tvBackground.opacity(0.7), location: 0.3),
                    .init(color: .clear, location: 0.75),
                ], startPoint: .bottom, endPoint: .top)
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                Text(title.name).font(.system(size: 72, weight: .bold)).lineLimit(2)
                    .foregroundStyle(.white)
                    .frame(maxWidth: 1200, alignment: .leading)
                facts
                if !title.overview.isEmpty {
                    Text(title.overview).type(theme.type.body).lineLimit(2)
                        .foregroundStyle(.white.opacity(0.85))
                        .frame(maxWidth: 960, alignment: .leading)
                }
                actions
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, MavioSpacing.s48)
        }
        .clipped()
    }

    @ViewBuilder private var artwork: some View {
        if let backdrop = sized(title.backdropURL, 1280) {
            ArtworkView(backdrop, icon: series ? "shows" : "movies")
        } else if let poster = sized(title.posterURL, 342) {
            // Without a backdrop the poster, blurred, colors the header.
            ArtworkView(poster).scaleEffect(1.25).blur(radius: 48).opacity(0.7)
        } else {
            theme.colors.surface
        }
    }

    /// The year, kind, rating and runtime, the status, then the genres,
    /// each opening the popular titles alike.
    private var facts: some View {
        let parts = [
            title.year > 0 ? String(title.year) : nil,
            series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
            title.officialRating.isEmpty ? nil : title.officialRating,
            title.runtimeMinutes > 0 ? minutesText(TimeInterval(title.runtimeMinutes) * 60) : nil,
        ].compactMap(\.self)
        let genres =
            title.genreRefs.isEmpty
            ? title.genres.map { NamedValue(id: "", name: $0) }
            : title.genreRefs.map { NamedValue(id: $0.id, name: $0.name) }
        return HStack(spacing: MavioSpacing.s16) {
            Text(parts.joined(separator: " · ")).type(theme.type.callout).foregroundStyle(.white.opacity(0.75))
            if let status = statusLabel(title.status) {
                Text(status).type(theme.type.caption).fontWeight(.semibold)
                    .foregroundStyle(.white)
                    .padding(.horizontal, MavioSpacing.s8)
                    .padding(.vertical, MavioSpacing.s2)
                    .background(.white.opacity(0.2), in: .rect(cornerRadius: 6))
            }
            ForEach(Array(genres.prefix(3)), id: \.self) { g in
                if !g.id.isEmpty, let query = discovery.alike(serverId, title.mediaKind, g.name, { $0.genres = [g.id] })
                {
                    NavigationLink(value: Route.catalog(query)) { Text(g.name) }
                        .buttonStyle(ChipButtonStyle())
                } else {
                    Text(g.name).type(theme.type.callout).foregroundStyle(.white.opacity(0.75))
                }
            }
        }
        .focusSection()
    }

    /// Watch, when a library has it; nothing else, as requests are left out.
    @ViewBuilder private var actions: some View {
        if !title.itemID.isEmpty {
            HStack(spacing: MavioSpacing.s24) {
                NavigationLink(value: Route.item(serverId: serverId, itemId: title.itemID)) {
                    Label {
                        Text(MavioStrings.discoverWatch)
                    } icon: {
                        Icon("play", fill: true, size: 28)
                    }
                }
                .buttonStyle(TVButtonStyle())
            }
            .focusSection()
        }
    }

    // MARK: Cast

    /// The first twenty of the cast, each opening their page, then See All.
    @ViewBuilder private var cast: some View {
        let shown = title.credits.filter(\.isCast).sorted { $0.order < $1.order }.prefix(20)
        if !shown.isEmpty {
            CardRow(title: MavioStrings.itemCast) {
                ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                    TitlePersonLink(serverId: serverId, credit: c)
                }
                NavigationLink(
                    value: Route.titleCredits(
                        serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbID, crew: false)
                ) {
                    SeeAll()
                }
                .buttonStyle(PersonButtonStyle())
            }
        }
    }
}

/// A person credited on a discovered title, opening their page when TMDB
/// knows them.
private struct TitlePersonLink: View {
    let serverId: String
    let credit: TitleCredit

    var body: some View {
        let card = TVPersonCard(name: credit.name, photo: sized(credit.imageURL, 185), role: credit.shownRole)
        if credit.tmdbId > 0 {
            NavigationLink(value: Route.discoveredPerson(serverId: serverId, tmdbId: credit.tmdbId)) { card }
                .buttonStyle(PersonButtonStyle())
        } else {
            Button {
            } label: {
                card
            }
            .buttonStyle(PersonButtonStyle())
        }
    }
}

// MARK: Overview

/// The overview and the box of facts as two columns (§6.2): the tagline,
/// overview, leading crew with View Full Crew, and keywords beside the
/// ratings and facts, each genre, keyword, studio, network and language
/// opening the popular titles alike.
private struct TitleOverview: View {
    let serverId: String
    let title: TitleInfo
    let ratings: [Rating]
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(alignment: .top, spacing: MavioSpacing.s64) {
            OverviewLeading(serverId: serverId, title: title)
                .frame(maxWidth: .infinity, alignment: .leading)
                .focusSection()
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                CollectionCard(serverId: serverId, title: title)
                TitleFacts(serverId: serverId, title: title, ratings: ratings)
            }
            .frame(width: 560)
            .focusSection()
        }
        .padding(.horizontal, theme.layout.margin)
    }
}

private struct OverviewLeading: View {
    let serverId: String
    let title: TitleInfo
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    @State private var more = false

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
        VStack(alignment: .leading, spacing: MavioSpacing.s24) {
            if !title.tagline.isEmpty {
                Text(title.tagline).type(theme.type.title3).fontWeight(.regular).italic()
                    .foregroundStyle(theme.colors.textSecondary)
            }
            Text(MavioStrings.discoverOverview).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
            // The overview unfolds as it is selected, the only way the remote
            // reaches it whole.
            Button {
                withAnimation { more.toggle() }
            } label: {
                Text(title.overview.isEmpty ? MavioStrings.discoverNoOverview : title.overview)
                    .type(theme.type.body).lineLimit(more ? nil : 5)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.vertical, MavioSpacing.s16)
            }
            .buttonStyle(RowButtonStyle())
            .padding(.horizontal, -MavioSpacing.s24)
            if !crew.isEmpty {
                LazyVGrid(
                    columns: Array(
                        repeating: GridItem(.flexible(), spacing: MavioSpacing.s24, alignment: .topLeading), count: 2),
                    alignment: .leading, spacing: MavioSpacing.s16
                ) {
                    ForEach(Array(crew.enumerated()), id: \.offset) { _, c in
                        let label = VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                            Text(c.kind.job == .other && !c.role.isEmpty ? c.role : c.kind.job.label)
                                .fontWeight(.semibold)
                            Text(c.name).opacity(0.7).lineLimit(1)
                        }
                        .padding(.vertical, MavioSpacing.s12)
                        if c.tmdbId > 0 {
                            NavigationLink(value: Route.discoveredPerson(serverId: serverId, tmdbId: c.tmdbId)) {
                                label
                            }
                            .buttonStyle(RowButtonStyle())
                        } else {
                            label.type(theme.type.body).foregroundStyle(theme.colors.textPrimary)
                                .padding(.horizontal, MavioSpacing.s24)
                        }
                    }
                }
                NavigationLink(
                    value: Route.titleCredits(
                        serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbID, crew: true)
                ) {
                    HStack(spacing: MavioSpacing.s8) {
                        Text(MavioStrings.discoverFullCrew)
                        Icon("forward", size: 24)
                    }
                }
                .buttonStyle(ChipButtonStyle())
            }
            if !keywords.isEmpty {
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                    ForEach(keywords, id: \.self) { k in
                        if !k.id.isEmpty,
                            let query = discovery.alike(serverId, title.mediaKind, k.name, { $0.keywords = [k] })
                        {
                            NavigationLink(value: Route.catalog(query)) { Text(k.name) }
                                .buttonStyle(ChipButtonStyle())
                        } else {
                            Text(k.name).type(theme.type.callout).foregroundStyle(theme.colors.textPrimary)
                                .padding(.horizontal, MavioSpacing.s16)
                                .padding(.vertical, MavioSpacing.s8)
                                .background(theme.colors.fill, in: Capsule())
                        }
                    }
                }
            }
        }
    }
}

/// A movie's collection: its backdrop and name, opening its page.
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
                    Text(MavioStrings.collectionView).type(theme.type.footnote).fontWeight(.semibold)
                }
                .foregroundStyle(.white)
                .padding(MavioSpacing.s24)
                .frame(maxWidth: .infinity, minHeight: 120)
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
                .focusRing()
            }
            .buttonStyle(CardButtonStyle(wide: true))
            .task(id: id) { collection = try? await model.collection(serverId, id) }
        }
    }
}

/// A fact's value: plain, or opening the popular titles alike.
private struct FactValue: Hashable {
    let text: String
    var query: CatalogQuery?
}

/// The ratings, then a fact a row, and the services streaming the title,
/// in a bordered box.
private struct TitleFacts: View {
    let serverId: String
    let title: TitleInfo
    let ratings: [Rating]
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme

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
                dayText(title.theatricalReleaseDate).map { "\(MavioStrings.discoverTheatricalRelease)  \($0)" },
                dayText(title.digitalReleaseDate).map { "\(MavioStrings.itemDigitalRelease)  \($0)" },
                dayText(title.physicalReleaseDate).map { "\(MavioStrings.itemPhysicalRelease)  \($0)" },
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
        rows.append(
            (
                series ? MavioStrings.itemNetworks : MavioStrings.itemStudio,
                studios.map { s in
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
            Ratings(ratings: ratings, score: title.voteCount > 0 ? title.communityRating : nil)
                .padding(MavioSpacing.s16)
            ForEach(rows, id: \.0) { label, values in
                Divider().overlay(theme.colors.border)
                HStack(alignment: .top, spacing: MavioSpacing.s16) {
                    Text(label).fontWeight(.semibold).foregroundStyle(theme.colors.textPrimary)
                    Spacer(minLength: 0)
                    VStack(alignment: .trailing, spacing: MavioSpacing.s8) {
                        ForEach(values, id: \.self) { v in
                            if let query = v.query {
                                NavigationLink(value: Route.catalog(query)) { Text(v.text) }
                                    .buttonStyle(ChipButtonStyle())
                            } else {
                                Text(v.text)
                            }
                        }
                    }
                    .multilineTextAlignment(.trailing)
                    .foregroundStyle(theme.colors.textSecondary)
                }
                .type(theme.type.callout)
                .padding(.horizontal, MavioSpacing.s16)
                .padding(.vertical, MavioSpacing.s12)
            }
            if !title.watchProviders.isEmpty {
                Divider().overlay(theme.colors.border)
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Text(MavioStrings.itemStreaming).type(theme.type.callout).fontWeight(.semibold)
                    Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                        ForEach(title.watchProviders, id: \.name) { p in
                            if !p.logoURL.isEmpty {
                                ArtworkView(p.logoURL, title: p.name)
                                    .frame(width: 48, height: 48)
                                    .clipShape(.rect(cornerRadius: 8))
                                    .accessibilityLabel(p.name)
                            } else {
                                Text(p.name).type(theme.type.footnote)
                            }
                        }
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(MavioSpacing.s16)
            }
        }
        .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(theme.colors.border))
    }
}

/// A title's ratings, as Seerr's: Rotten Tomatoes' and IMDb's, then TMDB's
/// score when given; they only show, as nothing opens a web page.
private struct Ratings: View {
    let ratings: [Rating]
    let score: Double?
    @Environment(\.theme) private var theme

    var body: some View {
        let order: [Mavio_Plugin_V1_RatingSource] = [.rottenTomatoesCritics, .rottenTomatoesAudience, .imdb]
        let shown = order.compactMap { source in ratings.first { $0.source == source } }
        Flow(spacing: MavioSpacing.s24, lineSpacing: MavioSpacing.s8) {
            ForEach(shown, id: \.source) { r in
                let fresh = r.score >= 60
                switch r.source {
                case .imdb:
                    rating(
                        "imdb",
                        MavioStrings.discoverImdbScore(votes: r.votes.formatted(.number.notation(.compactName))),
                        r.score.formatted(.number.precision(.fractionLength(1))), height: 28)
                case .rottenTomatoesCritics:
                    rating(
                        fresh ? "rt-fresh" : "rt-rotten",
                        r.certified ? MavioStrings.discoverTomatometerCertified : MavioStrings.discoverTomatometer,
                        "\(Int(r.score.rounded()))%", height: 28)
                default:
                    rating(
                        fresh ? "rt-aud-fresh" : "rt-aud-rotten", MavioStrings.discoverAudienceScore,
                        "\(Int(r.score.rounded()))%", height: 28)
                }
            }
            if let score {
                rating("tmdb-wide", MavioStrings.discoverTmdbScore, "\(Int((score * 10).rounded()))%", height: 22)
            }
        }
        .frame(maxWidth: .infinity)
    }

    private func rating(_ logo: String, _ label: String, _ value: String, height: CGFloat) -> some View {
        HStack(spacing: MavioSpacing.s8) {
            Image.service(logo).resizable().scaledToFit().frame(height: height)
            Text(value).type(theme.type.callout).fontWeight(.semibold).foregroundStyle(theme.colors.textPrimary)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(label) \(value)")
    }
}

// MARK: Seasons

/// A discovered series' seasons, newest first with the specials last, as
/// boxes that unfold their episodes as they are selected.
private struct DiscoveredSeasons: View {
    let serverId: String
    let tmdbId: Int32
    let seasons: [Mavio_Acquisition_V1_SeasonStatus]
    @Environment(\.theme) private var theme

    var body: some View {
        let ordered = seasons.sorted {
            ($0.seasonNumber == 0 ? 1 : 0, -$0.seasonNumber) < ($1.seasonNumber == 0 ? 1 : 0, -$1.seasonNumber)
        }
        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
            Text(MavioStrings.itemSeasons).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
            ForEach(ordered, id: \.seasonNumber) { s in
                DiscoveredSeason(serverId: serverId, tmdbId: tmdbId, season: s)
            }
        }
        .padding(.horizontal, theme.layout.margin)
        .focusSection()
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
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            Button {
                withAnimation { open.toggle() }
            } label: {
                HStack(spacing: MavioSpacing.s16) {
                    Text(
                        season.seasonNumber == 0
                            ? MavioStrings.discoverSpecials
                            : MavioStrings.discoverSeason(number: String(season.seasonNumber))
                    )
                    .fontWeight(.semibold).lineLimit(1)
                    Spacer(minLength: 0)
                    if season.episodeCount > 0 {
                        Text(MavioStrings.itemEpisodeCount(count: Int(season.episodeCount))).opacity(0.7)
                    }
                    if let status = statusLabel(season.status) {
                        Text(status).type(theme.type.footnote).fontWeight(.semibold)
                    }
                    Icon("expand", size: 24).rotationEffect(.degrees(open ? 180 : 0))
                }
            }
            .buttonStyle(RowButtonStyle())
            if open { list }
        }
    }

    @ViewBuilder private var list: some View {
        Group {
            if let episodes, !episodes.isEmpty {
                ForEach(episodes.reversed(), id: \.episodeNumber) { e in
                    // Each episode takes focus, so the remote scrolls through them.
                    Button {
                    } label: {
                        HStack(alignment: .top, spacing: MavioSpacing.s24) {
                            if let still = sized(e.stillURL, 300) {
                                ArtworkView(still).aspectRatio(16 / 9, contentMode: .fit).frame(width: 240)
                                    .clipShape(.rect(cornerRadius: MavioRadius.md))
                            }
                            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                                Text(MavioStrings.discoverEpisodeTitle(number: String(e.episodeNumber), name: e.name))
                                    .fontWeight(.semibold)
                                if let day = dayText(e.airDate) {
                                    Text(day).type(theme.type.footnote).opacity(0.7)
                                }
                                if !e.overview.isEmpty {
                                    Text(e.overview).type(theme.type.callout).opacity(0.7).lineLimit(3)
                                }
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        .padding(.vertical, MavioSpacing.s16)
                    }
                    .buttonStyle(RowButtonStyle())
                }
            } else if episodes != nil || failed {
                Text(failed ? MavioStrings.stateError : MavioStrings.discoverNoEpisodes)
                    .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    .padding(.horizontal, MavioSpacing.s24)
            } else {
                ProgressView().frame(maxWidth: .infinity)
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
/// item's (§6.2).
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
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s32) {
                        Text(crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast)
                            .type(theme.type.title1).fontWeight(.bold)
                            .foregroundStyle(theme.colors.textPrimary)
                        if credits.isEmpty {
                            Text(MavioStrings.creditsNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                        ForEach([CreditJob.actor, .guestStar] + CreditJob.crewOrder, id: \.self) { job in
                            people(job.label, credits.filter { $0.kind.job == job })
                        }
                    }
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.vertical, MavioSpacing.s48)
                }
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .toolbar(.hidden, for: .navigationBar)
        .task { await load() }
    }

    @ViewBuilder private func people(_ title: String, _ shown: [TitleCredit]) -> some View {
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(title).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 200), spacing: theme.layout.gutter, alignment: .top)],
                    alignment: .leading, spacing: MavioSpacing.s24
                ) {
                    ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                        TitlePersonLink(serverId: serverId, credit: c)
                    }
                }
                .focusSection()
            }
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

// MARK: Person

/// A discovered person's page, as Seerr's: their photo, dates, other names
/// and biography, then the movies and series they play in and work on, the
/// department they are known for first, chosen among all, movies or series.
struct DiscoveredPersonView: View {
    let serverId: String
    let tmdbId: Int32
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var person: Mavio_Acquisition_V1_Person?
    @State private var failure: String?
    @State private var more = false
    @State private var kinds = Kinds.all

    private enum Kinds: Hashable, CaseIterable { case all, movies, series }

    var body: some View {
        Group {
            if let person {
                ScrollView {
                    VStack(alignment: .leading, spacing: MavioSpacing.s48) {
                        header(person)
                        let cast = section(MavioStrings.personAppearances, person.cast)
                        let crew = section(MavioStrings.personCrew, person.crew)
                        if person.knownForDepartment == "Acting" || person.knownForDepartment.isEmpty {
                            cast
                            crew
                        } else {
                            crew
                            cast
                        }
                    }
                    .padding(.vertical, MavioSpacing.s48)
                }
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .toolbar(.hidden, for: .navigationBar)
        .task { await load() }
    }

    private func header(_ p: Mavio_Acquisition_V1_Person) -> some View {
        let facts = [
            dayText(p.birthday).map { MavioStrings.personBorn(date: $0, age: String(age(p.birthday, p.deathday))) },
            p.placeOfBirth.isEmpty ? nil : p.placeOfBirth,
            dayText(p.deathday).map { MavioStrings.personDied(date: $0) },
        ].compactMap(\.self)
        return HStack(alignment: .top, spacing: MavioSpacing.s48) {
            ArtworkView(sized(p.imageURL, 342), title: p.name, icon: "profile")
                .frame(width: theme.layout.poster, height: theme.layout.poster)
                .clipShape(.circle)
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(p.name).type(theme.type.display).foregroundStyle(theme.colors.textPrimary)
                if !facts.isEmpty {
                    Text(facts.joined(separator: " | ")).type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                }
                if !p.alsoKnownAs.isEmpty {
                    Text(MavioStrings.personAlsoKnownAs(names: p.alsoKnownAs.joined(separator: ", ")))
                        .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary).lineLimit(2)
                }
                if !p.biography.isEmpty {
                    Button {
                        withAnimation { more.toggle() }
                    } label: {
                        Text(p.biography).type(theme.type.body).lineLimit(more ? nil : 5)
                            .multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.vertical, MavioSpacing.s16)
                    }
                    .buttonStyle(RowButtonStyle())
                    .padding(.horizontal, -MavioSpacing.s24)
                    .accessibilityHint(more ? MavioStrings.personLess : MavioStrings.personMore)
                }
                Pill(items: Kinds.allCases, selected: $kinds) {
                    switch $0 {
                    case .all: MavioStrings.personAllKinds
                    case .movies: MavioStrings.personMovies
                    case .series: MavioStrings.personSeries
                    }
                }
                .fixedSize()
            }
            .frame(maxWidth: 1100, alignment: .leading)
        }
        .padding(.horizontal, theme.layout.margin)
        .focusSection()
    }

    @ViewBuilder private func section(_ title: String, _ all: [Mavio_Acquisition_V1_PersonTitle]) -> some View {
        let shown = all.filter { kinds == .all || ($0.title.mediaKind == .series) == (kinds == .series) }
        if !shown.isEmpty {
            CardRow(title: "\(title)  \(shown.count)") {
                ForEach(Array(shown.enumerated()), id: \.offset) { _, t in
                    DiscoveredTitleLink(serverId: serverId, title: t.title, role: t.role, width: theme.layout.poster)
                }
            }
        }
    }

    private func load() async {
        var r = Mavio_Acquisition_V1_GetPersonRequest()
        r.tmdbID = tmdbId
        r.language = discoverLanguage
        do {
            person =
                try await model.client.call(
                    serverId, Procedure.discover("GetPerson"), r, as: Mavio_Acquisition_V1_GetPersonResponse.self
                ).person
            failure = nil
        } catch is CancellationError {
        } catch {
            if person == nil { failure = error.reason }
        }
    }

    /// Whole years between two calendar dates, the second today when empty.
    private func age(_ from: String, _ to: String) -> Int {
        let parse = { (s: String) in try? Date(s + "T00:00:00Z", strategy: .iso8601) }
        guard let a = parse(from) else { return 0 }
        var utc = Calendar(identifier: .gregorian)
        utc.timeZone = .gmt
        return utc.dateComponents([.year], from: a, to: parse(to) ?? .now).year ?? 0
    }
}

// MARK: Collection

/// A discovered collection's page, without requests: its backdrop, name,
/// number of movies, what the server has of them and overview, then its
/// movies in release order.
struct DiscoveredCollectionView: View {
    let serverId: String
    let tmdbId: Int32
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var collection: Mavio_Acquisition_V1_Collection?
    @State private var failure: String?

    var body: some View {
        Group {
            if let c = collection {
                GeometryReader { geo in
                    ScrollView {
                        VStack(alignment: .leading, spacing: MavioSpacing.s48) {
                            header(c).frame(height: geo.size.height * 0.6)
                            LazyVGrid(
                                columns: [
                                    GridItem(.adaptive(minimum: theme.layout.poster), spacing: theme.layout.gutter)
                                ],
                                spacing: MavioSpacing.s48
                            ) {
                                ForEach(c.parts, id: \.key) { part in
                                    DiscoveredTitleLink(serverId: serverId, title: part)
                                }
                            }
                            .padding(.horizontal, theme.layout.margin)
                        }
                        .padding(.bottom, 120)
                    }
                }
                .ignoresSafeArea()
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .toolbar(.hidden, for: .navigationBar)
        .task(id: model.revision(serverId)) { await load() }
    }

    private func header(_ c: Mavio_Acquisition_V1_Collection) -> some View {
        let facts = [MavioStrings.collectionMovieCount(count: c.parts.count), statusLabel(status(c.parts))]
            .compactMap(\.self)
        return ZStack(alignment: .bottomLeading) {
            if let backdrop = sized(c.backdropURL, 1280) {
                ArtworkView(backdrop, icon: "collections")
            } else {
                theme.colors.surface
            }
            LinearGradient(
                stops: [
                    .init(color: theme.colors.tvBackground, location: 0),
                    .init(color: theme.colors.tvBackground.opacity(0.6), location: 0.4),
                    .init(color: .clear, location: 1),
                ], startPoint: .bottom, endPoint: .top)
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(c.name).font(.system(size: 72, weight: .bold)).foregroundStyle(.white)
                Text(facts.joined(separator: " · ")).type(theme.type.callout).foregroundStyle(.white.opacity(0.75))
                if !c.overview.isEmpty {
                    Text(c.overview).type(theme.type.body).lineLimit(3).foregroundStyle(.white.opacity(0.85))
                        .frame(maxWidth: 960, alignment: .leading)
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, MavioSpacing.s32)
        }
        .clipped()
    }

    /// What the server has of a collection, as Seerr marks it.
    private func status(_ parts: [DiscoveredTitle]) -> MediaStatus {
        guard !parts.isEmpty else { return .unspecified }
        if parts.allSatisfy({ $0.status == .blocklisted }) { return .blocklisted }
        if parts.allSatisfy({ $0.status == .available }) { return .available }
        if parts.contains(where: { $0.status == .available || $0.status == .partiallyAvailable }) {
            return .partiallyAvailable
        }
        if parts.allSatisfy({ $0.status == .requested || $0.status == .wanted }) { return .requested }
        return .unspecified
    }

    private func load() async {
        do {
            collection = try await model.collection(serverId, tmdbId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if collection == nil { failure = error.reason }
        }
    }
}
