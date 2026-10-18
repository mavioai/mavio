import MavioKit
import MavioUI
import SwiftUI

/// An item's Details section (libs/design/components.md, Details Screen):
/// on an iPad the tagline and overview, the leading crew, the tags and
/// links, then the ratings, the facts and the chosen version's media.
struct AboutSection: View {
    let details: Details
    let source: Source?
    /// Whether to show the overview, which a phone shows under the actions.
    let overview: Bool
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var allStudios = false

    private var card: Card { details.card }
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

    private var facts: [(String, String)] {
        let day = { (d: Date?) in d?.formatted(date: .abbreviated, time: .omitted) }
        var rows: [(String, String?)] = []
        if card.kind == .movie {
            let releases = [
                day(details.premiere).map { "\(MavioStrings.discoverTheatricalRelease)  \($0)" },
                day(details.digitalRelease).map { "\(MavioStrings.itemDigitalRelease)  \($0)" },
                day(details.physicalRelease).map { "\(MavioStrings.itemPhysicalRelease)  \($0)" },
            ].compactMap(\.self)
            rows.append(
                (
                    releases.count > 1 ? MavioStrings.discoverReleaseDates : MavioStrings.discoverReleaseDate,
                    releases.count > 1 ? releases.joined(separator: "\n") : day(details.premiere)
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
        if card.kind == .series {
            rows.append(
                (
                    MavioStrings.itemStatus,
                    details.seriesStatus == .continuing
                        ? MavioStrings.itemContinuing : details.seriesStatus == .ended ? MavioStrings.itemEnded : nil
                ))
            rows.append((MavioStrings.itemLastAired, details.seriesStatus == .ended ? day(details.endDate) : nil))
        }
        rows.append((MavioStrings.itemRuntime, card.kind == .series ? nil : card.runtime.map(minutesText)))
        rows.append((MavioStrings.itemRated, details.officialRating))
        rows.append((MavioStrings.itemGenres, details.genres.isEmpty ? nil : details.genres.joined(separator: ", ")))
        rows.append((inSeries ? MavioStrings.itemNetworks : MavioStrings.itemStudio, studios))
        rows.append(
            (MavioStrings.itemCountry, details.countries.isEmpty ? nil : details.countries.joined(separator: "\n")))
        rows.append(
            (
                MavioStrings.itemOriginalLanguage,
                details.originalLanguage.map { Locale.current.localizedString(forLanguageCode: $0) ?? $0 }
            ))
        rows.append(
            (MavioStrings.itemOriginalTitle, details.originalTitle.flatMap { $0 == card.title ? nil : $0 }))
        rows.append((MavioStrings.itemDateAdded, day(details.dateAdded)))
        return rows.compactMap { k, v in v.map { (k, $0) } }
    }

    private static let minStudios = 3

    private var studios: String? {
        guard !details.studios.isEmpty else { return nil }
        let shown = allStudios ? details.studios : Array(details.studios.prefix(Self.minStudios))
        return shown.joined(separator: ", ")
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

    var body: some View {
        let empty =
            text == nil && !ratings && crew.isEmpty && tags.isEmpty && facts.isEmpty && media.isEmpty
            && details.links.isEmpty
        if !empty {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(MavioStrings.itemDetails).type(theme.type.title3)
                let layout =
                    sizeClass == .regular
                    ? AnyLayout(HStackLayout(alignment: .top, spacing: 64))
                    : AnyLayout(VStackLayout(alignment: .leading, spacing: 32))
                layout {
                    if text != nil || !crew.isEmpty || !tags.isEmpty || !details.links.isEmpty {
                        leading.frame(maxWidth: sizeClass == .regular ? 440 : .infinity, alignment: .leading)
                    }
                    VStack(alignment: .leading, spacing: 32) {
                        if ratings { ratingCards }
                        FactList(rows: facts) { key in
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
                        FactList(rows: media) { _ in EmptyView() }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.top, MavioSpacing.s32)
            .padding(.bottom, MavioSpacing.s16)
        }
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
                            Text(c.name).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary).lineLimit(
                                1)
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
                            Text(tag).type(theme.type.footnote)
                                .padding(.horizontal, MavioSpacing.s12)
                                .padding(.vertical, MavioSpacing.s4)
                                .overlay(Capsule().strokeBorder(theme.colors.border))
                        }
                    }
                }
            }
            if !details.links.isEmpty {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                    ForEach(details.links, id: \.url) { link in
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

/// Facts, each a caption over its value, in as many columns as fit.
private struct FactList<Extra: View>: View {
    let rows: [(String, String)]
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
                        Text(value).type(theme.type.callout).foregroundStyle(theme.colors.textPrimary)
                            .fixedSize(horizontal: false, vertical: true)
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
