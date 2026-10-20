import MavioKit
import MavioUI
import SwiftUI

/// An item's details (docs/tv.md §6.2): the backdrop over 86 % of the
/// screen's height, the logo or name at 72 pt, two lines of overview and the
/// actions, focus starting on the primary one; then a series' seasons and
/// episodes, the cast and the items like it. The navigation bar hides.
struct DetailsView: View {
    let serverId: String
    let itemId: String
    @Environment(AppModel.self) private var model
    @State private var details: Details?
    @State private var failure: String?

    var body: some View {
        Group {
            if let details {
                DetailsContent(details: details)
            } else if failure != nil {
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
            details = try await model.client.details(serverId: serverId, itemId: itemId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if details == nil { failure = error.reason }
        }
    }
}

private struct DetailsContent: View {
    let details: Details
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @State private var season: String?
    @FocusState private var playFocused: Bool

    private var card: Card { details.card }
    private var own: [Source] { details.sources.filter { $0.serverId == card.serverId } }

    /// What the primary action plays: the item, or a series' or season's
    /// episode to watch next.
    private var next: (itemId: String, title: String, position: TimeInterval?, played: Bool, episode: String?)? {
        if card.kind == .series || card.kind == .season {
            return details.next.map { e in
                (e.card.itemId, e.card.title, e.position, e.card.played, short(e))
            }
        }
        guard !own.isEmpty else { return nil }
        return (card.itemId, card.title, details.position, card.played, nil)
    }

    var body: some View {
        GeometryReader { geo in
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s48) {
                    header.frame(height: geo.size.height * 0.86)
                    episodes
                    if !details.credits.isEmpty { cast }
                    if !details.similar.isEmpty {
                        CardRow(title: MavioStrings.itemSimilar) {
                            ForEach(details.similar, id: \.self) {
                                CardLink(card: $0, wide: false, width: theme.layout.poster)
                            }
                        }
                    }
                }
                .padding(.bottom, 120)
            }
        }
        .ignoresSafeArea()
        .defaultFocus($playFocused, true)
    }

    // MARK: Header

    private var header: some View {
        ZStack(alignment: .bottomLeading) {
            ArtworkView((details.backdrop ?? card.backdrop)?.url, title: card.title, icon: card.kind.icon)
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
                if let series = details.series {
                    Text(series.name).type(theme.type.title3).foregroundStyle(.white.opacity(0.85))
                }
                TitleMark(name: card.title, logo: (details.logo ?? card.logo)?.url)
                facts
                if let overview = details.overview {
                    Text(overview).type(theme.type.body).lineLimit(2)
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

    /// The year, rating, runtime and genres, in one line.
    private var facts: some View {
        let parts: [String] = [
            details.episodeNumber.map {
                MavioStrings.itemEpisodeShort(season: String(details.seasonNumber ?? 0), episode: String($0))
            },
            card.year.map(String.init),
            details.officialRating,
            card.runtime.map { Duration.seconds($0).formatted(.units(allowed: [.hours, .minutes], width: .narrow)) },
            details.genres.prefix(3).joined(separator: ", "),
        ].compactMap { $0?.isEmpty == false ? $0 : nil }
        return Text(parts.joined(separator: " · ")).type(theme.type.callout).foregroundStyle(.white.opacity(0.75))
    }

    private var actions: some View {
        HStack(spacing: MavioSpacing.s24) {
            if let next {
                let resumable = (next.position ?? 0) > 0 && !next.played
                Button {
                    play(next.itemId, next.title, start: resumable ? next.position : 0)
                } label: {
                    Label {
                        Text(
                            next.episode.map {
                                resumable
                                    ? MavioStrings.itemResumeEpisode(episode: $0)
                                    : MavioStrings.itemPlayEpisode(episode: $0)
                            } ?? (resumable ? MavioStrings.itemResume : MavioStrings.itemPlay))
                    } icon: {
                        Icon("play", fill: true, size: 28)
                    }
                }
                .focused($playFocused)
                if resumable {
                    Button {
                        play(next.itemId, next.title, start: 0)
                    } label: {
                        Icon("previous", size: 28).accessibilityLabel(MavioStrings.itemFromStart)
                    }
                }
            }
            Button {
                Task { try? await model.setFavorite(card, !card.favorite) }
            } label: {
                Icon("favorite", fill: card.favorite, size: 28).accessibilityLabel(MavioStrings.itemFavorite)
            }
            Button {
                Task { try? await model.setPlayed(card, !card.played) }
            } label: {
                Icon("played", fill: card.played, size: 28)
                    .accessibilityLabel(card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed)
            }
        }
        .buttonStyle(TVButtonStyle())
        .focusSection()
    }

    // MARK: Rows

    /// A series' seasons as pills, focus choosing one, over its episodes; a
    /// season's or an episode's own episodes.
    @ViewBuilder private var episodes: some View {
        switch card.kind {
        case .series where !details.children.isEmpty:
            let current = season ?? details.next?.seasonId ?? details.children.first?.itemId
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                ScrollView(.horizontal) {
                    HStack(spacing: MavioSpacing.s16) {
                        ForEach(details.children, id: \.self) { s in
                            Button(s.title) { season = s.itemId }
                                .buttonStyle(TabButtonStyle(selected: s.itemId == current))
                                .onFocusChange { if $0 { season = s.itemId } }
                        }
                    }
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.vertical, MavioSpacing.s8)
                }
                .scrollClipDisabled()
                .focusSection()
                EpisodeRow(episodes: details.episodes.filter { $0.seasonId == current }, play: playEpisode)
            }
        case .season, .episode:
            if details.episodes.count > 1 || card.kind == .season {
                VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                    Text(MavioStrings.itemEpisodes).type(theme.type.title3).padding(.horizontal, theme.layout.margin)
                    EpisodeRow(episodes: details.episodes, play: playEpisode)
                }
            }
        default:
            if !details.children.isEmpty {
                CardRow(title: MavioStrings.itemEpisodes) {
                    ForEach(details.children, id: \.self) {
                        CardLink(
                            card: $0, wide: isWide($0), width: isWide($0) ? theme.layout.thumbnail : theme.layout.poster
                        )
                    }
                }
            }
        }
    }

    private var cast: some View {
        CardRow(title: MavioStrings.itemCast) {
            ForEach(details.credits.filter(\.cast), id: \.self) { credit in
                Button {
                } label: {
                    PersonCard(credit, role: credit.shownRole)
                        .scaleEffect(1.6)
                        .frame(width: 180, height: 260)
                }
                .buttonStyle(CardButtonStyle())
            }
        }
    }

    // MARK: Actions

    private func playEpisode(_ e: Episode) {
        play(e.card.itemId, e.card.title, start: (e.position ?? 0) > 0 && !e.card.played ? e.position : 0)
    }

    private func play(_ itemId: String, _ title: String, start: TimeInterval?) {
        router.play(PlayRequest(serverId: card.serverId, itemId: itemId, title: title, start: start))
    }

    private func short(_ e: Episode) -> String? {
        guard let number = e.number else { return nil }
        return MavioStrings.itemEpisodeShort(season: String(e.season ?? 0), episode: String(number))
    }
}

/// Episodes as thumbnails with their numbers, selecting one playing it.
private struct EpisodeRow: View {
    let episodes: [Episode]
    let play: (Episode) -> Void
    @Environment(\.theme) private var theme

    var body: some View {
        ScrollView(.horizontal) {
            LazyHStack(alignment: .top, spacing: theme.layout.gutter) {
                ForEach(episodes, id: \.card.itemId) { e in
                    Button {
                        play(e)
                    } label: {
                        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                            TVCard(card: e.card, wide: true, width: theme.layout.thumbnail)
                            if let overview = e.overview {
                                Text(overview).type(theme.type.footnote).lineLimit(3)
                                    .foregroundStyle(theme.colors.textSecondary)
                                    .frame(width: theme.layout.thumbnail, alignment: .leading)
                            }
                        }
                    }
                    .buttonStyle(CardButtonStyle(wide: true))
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.vertical, MavioSpacing.s32)
        }
        .scrollClipDisabled()
        .padding(.vertical, -MavioSpacing.s32)
        .focusSection()
    }
}

extension View {
    /// Calls action as focus enters or leaves the view.
    func onFocusChange(_ action: @escaping (Bool) -> Void) -> some View {
        modifier(FocusChange(action: action))
    }
}

private struct FocusChange: ViewModifier {
    let action: (Bool) -> Void
    @FocusState private var focused: Bool

    func body(content: Content) -> some View {
        content.focused($focused).onChange(of: focused) { _, f in action(f) }
    }
}
