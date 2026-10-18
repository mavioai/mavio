import MavioKit
import MavioUI
import SwiftUI

/// An item's Details section (libs/design/components.md, Details Screen):
/// on an iPad the tagline and overview, the leading crew, the tags and
/// links, then the ratings, the facts and the chosen version's media.
/// Genres, keywords and studios open their items in the library, and the
/// crew their pages. A movie or series adds what the metadata providers
/// tell of it beyond what the library keeps, as Seerr shows: its ratings,
/// budget and takings, its next episode, the services streaming it and the
/// logos linking it elsewhere.
struct AboutSection: View {
    let details: Details
    let source: Source?
    /// Whether to show the overview, which a phone shows under the actions.
    let overview: Bool
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var allStudios = false
    /// The movie or series as the server discovers it.
    @State private var title: TitleInfo?
    /// Its ratings elsewhere; nil until asked.
    @State private var titleRatings: [Rating]?

    private var card: Card { details.card }
    private var mediaKind: MediaKind? { card.kind == .movie ? .movie : card.kind == .series ? .series : nil }
    /// A movie's or series' TMDB ID, which discovery knows it by; 0 otherwise.
    private var tmdbId: Int32 { mediaKind == nil ? 0 : Int32(details.externalIds["tmdb"] ?? "") ?? 0 }
    private var titled: Bool { tmdbId > 0 }
    private var inSeries: Bool { card.kind == .series || card.kind == .season || card.kind == .episode }

    /// Up to six of the crew, the creators, directors and writers first.
    private var crew: [CreditEntry] {
        let rank = { (c: CreditEntry) in CreditJob.crewOrder.firstIndex(of: c.job) ?? .max }
        return Array(
            details.credits.enumerated().filter { !$0.element.job.isCast }
                .sorted { (rank($0.element), $0.offset) < (rank($1.element), $1.offset) }
                .map(\.element)
                .prefix(6))
    }

    private var tags: [String] { card.kind == .episode || card.kind == .season ? [] : details.tags }
    private var text: String? { overview ? details.overview : nil }

    /// A genre, keyword or studio opening the library's items of it, when
    /// the server is reachable.
    private func narrowed(_ filter: Filter) -> Route? {
        guard let libraryId = details.libraryId, !details.offline else { return nil }
        return .filteredLibrary(serverId: card.serverId, libraryId: libraryId, filter: filter)
    }

    private static func filter(genre: String? = nil, tag: String? = nil, studio: String? = nil) -> Filter {
        Filter(played: nil, favorite: false, genre: genre, tag: tag, studio: studio)
    }

    /// The values of a fact that open the library, by the fact's caption.
    private var links: [String: [(String, Route?)]] {
        var links: [String: [(String, Route?)]] = [:]
        links[MavioStrings.itemGenres] = details.genres.map { ($0, narrowed(Self.filter(genre: $0))) }
        links[inSeries ? MavioStrings.itemNetworks : MavioStrings.itemStudio] = shownStudios.map {
            ($0, narrowed(Self.filter(studio: $0)))
        }
        return links
    }

    private var facts: [(String, String)] {
        let day = { (d: Date?) in d?.formatted(date: .abbreviated, time: .omitted) }
        let unbroken = { (s: String) in s.replacingOccurrences(of: " ", with: "\u{00A0}") }
        var rows: [(String, String?)] = []
        // A movie's releases on one row, as on its discovered title, what
        // the library keeps first.
        let releases =
            card.kind == .movie
            ? [
                (MavioStrings.discoverTheatricalRelease, details.premiere, title?.theatricalReleaseDate),
                (MavioStrings.itemDigitalRelease, details.digitalRelease, title?.digitalReleaseDate),
                (MavioStrings.itemPhysicalRelease, details.physicalRelease, title?.physicalReleaseDate),
            ].compactMap { label, kept, told in
                (day(kept) ?? dayText(told ?? "")).map { "\(label)  \(unbroken($0))" }
            }
            : []
        if !releases.isEmpty {
            rows.append(
                (
                    releases.count > 1 ? MavioStrings.discoverReleaseDates : MavioStrings.discoverReleaseDate,
                    releases.joined(separator: "\n")
                ))
        } else {
            rows.append(
                (
                    card.kind == .series
                        ? MavioStrings.itemFirstAired
                        : card.kind == .episode ? MavioStrings.itemAired : MavioStrings.itemReleased,
                    day(details.premiere)
                ))
        }
        rows.append(
            (
                MavioStrings.itemStatus,
                card.kind == .series && details.seriesStatus == .continuing
                    ? MavioStrings.itemContinuing
                    : card.kind == .series && details.seriesStatus == .ended
                        ? MavioStrings.itemEnded : title?.productionStatus.text
            ))
        if card.kind == .series {
            rows.append((MavioStrings.itemLastAired, details.seriesStatus == .ended ? day(details.endDate) : nil))
            rows.append((MavioStrings.itemNextAirDate, dayText(title?.nextAirDate ?? "")))
        }
        rows.append((MavioStrings.itemRuntime, card.kind == .series ? nil : card.runtime.map(minutesText)))
        rows.append((MavioStrings.itemRated, details.officialRating))
        rows.append((MavioStrings.itemGenres, details.genres.isEmpty ? nil : details.genres.joined(separator: ", ")))
        rows.append((inSeries ? MavioStrings.itemNetworks : MavioStrings.itemStudio, studios))
        rows.append((MavioStrings.itemCountry, countries.isEmpty ? nil : countries.joined(separator: "\n")))
        rows.append(
            (
                MavioStrings.itemOriginalLanguage,
                (details.originalLanguage ?? title?.originalLanguage.nonEmpty).map(languageText)
            ))
        rows.append(
            (MavioStrings.itemOriginalTitle, details.originalTitle.flatMap { $0 == card.title ? nil : $0 }))
        rows.append((MavioStrings.itemBudget, title.flatMap { moneyText($0.budget) }))
        rows.append((MavioStrings.itemRevenue, title.flatMap { moneyText($0.revenue) }))
        rows.append((MavioStrings.itemDateAdded, day(details.dateAdded)))
        return rows.compactMap { k, v in v.map { (k, $0) } }
    }

    /// The countries with their flags when the library keeps those the
    /// metadata providers tell, else as the library keeps them.
    private var countries: [String] {
        let stored = details.countries
        let told = title?.productionLocations ?? []
        if let title, !title.productionCountries.isEmpty,
            stored.isEmpty || (stored.count == told.count && stored.allSatisfy(told.contains))
        {
            return title.productionCountries.map(countryText)
        }
        return stored.isEmpty ? told : stored
    }

    private static let minStudios = 3

    private var shownStudios: [String] {
        allStudios ? details.studios : Array(details.studios.prefix(Self.minStudios))
    }

    private var studios: String? {
        shownStudios.isEmpty ? nil : shownStudios.joined(separator: ", ")
    }

    private var media: [(String, String)] {
        guard let source else { return [] }
        let video = source.video.map { v in
            [source.resolution, v.codec.uppercased(), source.range].compactMap(\.self).joined(separator: " · ")
        }
        let rows: [(String, String?)] = [
            (MavioStrings.itemVideo, video),
            (
                MavioStrings.itemAudioTracks,
                source.audio.isEmpty ? nil : source.audio.map(audioText).joined(separator: "\n")
            ),
            (
                MavioStrings.itemSubtitleTracks,
                source.subtitles.isEmpty ? nil : source.subtitles.map(subtitleText).joined(separator: "\n")
            ),
            (
                MavioStrings.itemFile,
                [source.fileName, source.sizeText].compactMap(\.self).joined(separator: "\n").nonEmpty
            ),
        ]
        return rows.compactMap { k, v in v.map { (k, $0) } }
    }

    private var ratings: Bool { (details.communityRating ?? 0) > 0 || (details.criticRating ?? 0) > 0 }

    /// The pages about it as the library tells, for what is not titled.
    private var pages: [MavioCore.ExternalLink] { titled ? [] : details.links }

    var body: some View {
        let empty =
            text == nil && !ratings && !titled && crew.isEmpty && tags.isEmpty && facts.isEmpty && media.isEmpty
            && pages.isEmpty
        VStack(spacing: 0) {
            if !empty { content }
        }
        .task(id: "\(card.serverId)/\(tmdbId)") { await loadTitle() }
    }

    private var content: some View {
        Group {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(MavioStrings.itemDetails).type(theme.type.title3)
                let layout =
                    sizeClass == .regular
                    ? AnyLayout(HStackLayout(alignment: .top, spacing: 64))
                    : AnyLayout(VStackLayout(alignment: .leading, spacing: 32))
                layout {
                    if text != nil || !crew.isEmpty || !tags.isEmpty || !pages.isEmpty || titled {
                        leading.frame(maxWidth: sizeClass == .regular ? 440 : .infinity, alignment: .leading)
                    }
                    VStack(alignment: .leading, spacing: 32) {
                        ratingsView
                        FactList(rows: facts, links: links) { key in
                            if key == (inSeries ? MavioStrings.itemNetworks : MavioStrings.itemStudio),
                                details.studios.count > Self.minStudios
                            {
                                Button {
                                    withAnimation { allStudios.toggle() }
                                } label: {
                                    HStack(spacing: MavioSpacing.s4) {
                                        Text(allStudios ? MavioStrings.actionShowLess : MavioStrings.discoverShowMore)
                                        Icon("expand", size: 14).rotationEffect(.degrees(allStudios ? 180 : 0))
                                    }
                                    .type(theme.type.callout)
                                    .foregroundStyle(theme.colors.textSecondary)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        WatchProviders(providers: title?.watchProviders ?? [])
                        FactList(rows: media, links: [:]) { _ in EmptyView() }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.top, MavioSpacing.s32)
            .padding(.bottom, MavioSpacing.s16)
        }
    }

    /// A movie's or series' ratings elsewhere once known, else the
    /// library's.
    @ViewBuilder private var ratingsView: some View {
        if let kind = mediaKind, titled {
            let score = title.flatMap { $0.voteCount > 0 ? $0.communityRating : nil }
            if let loaded = titleRatings {
                if !TitleRatings.shown(loaded).isEmpty || score != nil {
                    TitleRatings(kind: kind, tmdbId: tmdbId, ratings: loaded, score: score, alignment: .leading)
                } else if ratings {
                    ratingCards
                }
            }
        } else if ratings {
            ratingCards
        }
    }

    /// Asks the server for the movie or series as it discovers it, and its
    /// ratings elsewhere; a server that does not discover tells nothing.
    private func loadTitle() async {
        guard let kind = mediaKind, tmdbId > 0, !details.offline else { return }
        var r = Mavio_Acquisition_V1_GetRatingsRequest()
        r.mediaKind = kind
        r.tmdbID = tmdbId
        async let told = try? model.title(card.serverId, kind, tmdbId)
        async let rated = try? model.client.call(
            card.serverId, Procedure.discover("GetRatings"), r, as: Mavio_Acquisition_V1_GetRatingsResponse.self)
        title = await told
        titleRatings = await rated?.ratings ?? []
    }

    private var leading: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s20) {
            if let text {
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    if let tagline = details.tagline { Text(tagline).type(theme.type.headline) }
                    Text(text).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                }
            }
            if !crew.isEmpty {
                LazyVGrid(
                    columns: Array(
                        repeating: GridItem(.flexible(), spacing: MavioSpacing.s24, alignment: .topLeading), count: 2),
                    alignment: .leading, spacing: MavioSpacing.s16
                ) {
                    ForEach(Array(crew.enumerated()), id: \.offset) { _, c in
                        VStack(alignment: .leading, spacing: 0) {
                            Text(c.job == .other ? c.role ?? c.job.label : c.job.label).type(theme.type.headline)
                            let name = Text(c.name).type(theme.type.callout)
                                .foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                            if c.personId.isEmpty {
                                name
                            } else {
                                NavigationLink(value: Route.person(serverId: card.serverId, personId: c.personId)) {
                                    name
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                }
                NavigationLink(value: Route.credits(serverId: card.serverId, itemId: card.itemId, crew: true)) {
                    HStack(spacing: MavioSpacing.s4) {
                        Text(MavioStrings.discoverFullCrew)
                        Icon("forward", size: 16)
                    }
                    .type(theme.type.callout).fontWeight(.semibold)
                    .foregroundStyle(theme.colors.textPrimary)
                }
                .buttonStyle(.plain)
            }
            if !tags.isEmpty {
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    Text(MavioStrings.itemTags).type(theme.type.footnote).fontWeight(.semibold)
                        .foregroundStyle(theme.colors.textSecondary)
                    Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                        ForEach(tags, id: \.self) { tag in
                            let pill = Text(tag).type(theme.type.footnote)
                                .padding(.horizontal, MavioSpacing.s12)
                                .padding(.vertical, MavioSpacing.s4)
                                .overlay(Capsule().strokeBorder(theme.colors.border))
                            if let route = narrowed(Self.filter(tag: tag)) {
                                NavigationLink(value: route) { pill }.buttonStyle(.plain)
                            } else {
                                pill
                            }
                        }
                    }
                }
            }
            if let kind = mediaKind, titled {
                ExternalLinks(
                    links: titleLinks(
                        kind: kind, tmdbId: tmdbId, imdb: details.externalIds["imdb"] ?? "",
                        tvdb: details.externalIds["tvdb"] ?? "", ratings: titleRatings ?? []),
                    alignment: .leading
                )
                .padding(.top, MavioSpacing.s8)
            }
            if !pages.isEmpty {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                    ForEach(pages, id: \.url) { link in
                        if let url = URL(string: link.url) {
                            Link(destination: url) {
                                HStack(spacing: 6) {
                                    Text(link.name).foregroundStyle(theme.colors.textPrimary)
                                    Icon("external", size: 14).foregroundStyle(theme.colors.textSecondary)
                                }
                                .type(theme.type.footnote).fontWeight(.semibold)
                                .padding(.horizontal, 14)
                                .padding(.vertical, 6)
                                .background(theme.colors.fill, in: Capsule())
                            }
                        }
                    }
                }
            }
        }
    }

    /// The audience's rating out of 10 and the critics' score, large.
    private var ratingCards: some View {
        Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
            if let rating = details.communityRating, rating > 0 {
                RatingCard(label: MavioStrings.itemCommunityRating, scale: "/ 10") {
                    HStack(spacing: 6) {
                        Icon("rating", fill: true, size: 22).foregroundStyle(Color(red: 1, green: 0.84, blue: 0.04))
                        Text(rating, format: .number.precision(.fractionLength(1)))
                    }
                }
            }
            if let rating = details.criticRating, rating > 0 {
                RatingCard(label: MavioStrings.itemCriticRating, scale: nil) {
                    Text("\(Int(rating.rounded()))%")
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(MavioStrings.itemRatings)
    }

    private func audioText(_ a: AudioStream) -> String {
        let channels =
            switch a.channels {
            case 6: "5.1"
            case 8: "7.1"
            case let n where n > 0: "\(n) ch"
            default: ""
            }
        return [a.title ?? language(a.language), a.codec.uppercased(), channels.nonEmpty]
            .compactMap(\.self).joined(separator: " · ")
    }

    private func subtitleText(_ s: SubtitleStream) -> String {
        [s.title ?? language(s.language), s.codec.uppercased()].compactMap(\.self).joined(separator: " · ")
    }

    private func language(_ code: String?) -> String? {
        guard let code, code != "und" else { return nil }
        return Locale.current.localizedString(forLanguageCode: code) ?? code
    }
}

/// A rating, large, with its caption and scale.
private struct RatingCard<Value: View>: View {
    let label: String
    let scale: String?
    @ViewBuilder let value: Value
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            Text(label).type(theme.type.footnote).fontWeight(.semibold).foregroundStyle(theme.colors.textSecondary)
            HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s4) {
                value.type(theme.type.title2).fontWeight(.bold).monospacedDigit()
                if let scale {
                    Text(scale).type(theme.type.callout).foregroundStyle(theme.colors.textTertiary)
                }
            }
        }
        .padding(.horizontal, MavioSpacing.s16)
        .padding(.vertical, MavioSpacing.s12)
        .frame(minWidth: 144, alignment: .leading)
        .background(theme.colors.fill, in: .rect(cornerRadius: 12))
    }
}

/// Facts, each a caption over its value, in as many columns as fit; a
/// fact with links shows them in place of its value.
private struct FactList<Extra: View>: View {
    let rows: [(String, String)]
    let links: [String: [(String, Route?)]]
    @ViewBuilder let extra: (String) -> Extra
    @Environment(\.theme) private var theme

    var body: some View {
        if !rows.isEmpty {
            LazyVGrid(
                columns: [GridItem(.adaptive(minimum: 150), spacing: MavioSpacing.s32, alignment: .topLeading)],
                alignment: .leading, spacing: MavioSpacing.s20
            ) {
                ForEach(rows, id: \.0) { key, value in
                    VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                        Text(key).type(theme.type.footnote).fontWeight(.semibold)
                            .foregroundStyle(theme.colors.textSecondary)
                        if let values = links[key], values.contains(where: { $0.1 != nil }) {
                            Flow(spacing: 4, lineSpacing: 2) {
                                ForEach(Array(values.enumerated()), id: \.offset) { i, v in
                                    let text = Text(i < values.count - 1 ? "\(v.0)," : v.0)
                                    if let route = v.1 {
                                        NavigationLink(value: route) { text }.buttonStyle(.plain)
                                    } else {
                                        text
                                    }
                                }
                            }
                            .type(theme.type.callout).foregroundStyle(theme.colors.textPrimary)
                        } else {
                            Text(value).type(theme.type.callout).foregroundStyle(theme.colors.textPrimary)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        extra(key)
                    }
                }
            }
        }
    }
}

extension String {
    /// The string, or nil when it is empty.
    fileprivate var nonEmpty: String? { isEmpty ? nil : self }
}
