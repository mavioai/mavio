import MavioAdmin
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
    @State private var sourceIndex = 0
    /// Whether the user may report problems on the server: administrators,
    /// and users allowed to.
    @State private var mayReport = false
    @State private var reporting = false
    @FocusState private var playFocused: Bool

    private var card: Card { details.card }
    /// The files of this server.
    private var own: [Source] { details.sources.filter { $0.serverId == card.serverId } }
    /// The same title on the user's other servers.
    private var copies: [Source] { details.sources.filter { $0.serverId != card.serverId } }
    /// The version to play.
    private var source: Source? { own.indices.contains(sourceIndex) ? own[sourceIndex] : own.first }
    /// The trailer among the extras; a trailer elsewhere would open a web
    /// page, which Apple TV does not (§1).
    private var trailer: Extra? { details.extras.first { $0.kind == .trailer } }

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
        .fullScreenCover(isPresented: $reporting) {
            if let server = model.server(card.serverId) {
                ReportIssueView(
                    server: server, itemId: card.itemId, name: card.title, series: card.kind == .series
                ) {}
            }
        }
        .task(id: card.serverId) { await loadMayReport() }
    }

    private func loadMayReport() async {
        guard !details.offline else { return }
        let response = try? await model.client.call(
            card.serverId, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
            as: Mavio_User_V1_GetCurrentUserResponse.self)
        if let user = response?.user { mayReport = user.admin || user.policy.reportIssues }
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
            if let trailer {
                Button {
                    router.play(
                        PlayRequest(
                            serverId: trailer.card.serverId, itemId: trailer.card.itemId, title: trailer.card.title))
                } label: {
                    Icon("movies", size: 28).accessibilityLabel(MavioStrings.itemTrailer)
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
            if own.count > 1 || !copies.isEmpty { versionMenu }
            if mayReport, [.movie, .series, .season, .episode].contains(card.kind) {
                Menu {
                    Button(MavioStrings.issuesReport) { reporting = true }
                } label: {
                    Icon("more", size: 28).accessibilityLabel(MavioStrings.listsMore)
                }
            }
        }
        .buttonStyle(TVButtonStyle())
        .focusSection()
    }

    /// The version to play, checked, then the same title on the user's
    /// other servers, opening it there.
    private var versionMenu: some View {
        Menu {
            if own.count > 1 {
                Section(MavioStrings.itemVersions) {
                    ForEach(own.indices, id: \.self) { i in
                        Button {
                            sourceIndex = i
                        } label: {
                            if i == sourceIndex {
                                Label(own[i].versionName, systemImage: "checkmark")
                            } else {
                                Text(own[i].versionName)
                            }
                            Text(versionDetails(own[i]))
                        }
                    }
                }
            }
            Section {
                ForEach(copies, id: \.self) { copy in
                    NavigationLink(value: Route.item(serverId: copy.serverId, itemId: copy.itemId)) {
                        Text(MavioStrings.itemOnServer(server: copy.serverName))
                    }
                }
            }
        } label: {
            HStack(spacing: MavioSpacing.s8) {
                Text(source?.versionName ?? details.serverName)
                Icon("expand", size: 20)
            }
            .accessibilityLabel(MavioStrings.itemVersion)
        }
        .menuOrder(.fixed)
    }

    private func versionDetails(_ s: Source) -> String {
        [s.resolution, s.range, s.surround, s.format, s.sizeText].compactMap(\.self).joined(separator: " · ")
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
                PersonLink(serverId: card.serverId, credit: credit, role: credit.shownRole)
            }
            NavigationLink(value: Route.credits(serverId: card.serverId, itemId: card.itemId, crew: false)) {
                SeeAll()
            }
            .buttonStyle(PersonButtonStyle())
        }
    }

    // MARK: Actions

    private func playEpisode(_ e: Episode) {
        play(e.card.itemId, e.card.title, start: (e.position ?? 0) > 0 && !e.card.played ? e.position : 0)
    }

    /// Plays the item in the version chosen, or an episode.
    private func play(_ itemId: String, _ title: String, start: TimeInterval?) {
        if itemId == card.itemId, let source {
            router.play(
                PlayRequest(
                    serverId: source.serverId, itemId: source.itemId, title: title,
                    mediaSourceId: source.mediaSourceId, start: start))
        } else {
            router.play(PlayRequest(serverId: card.serverId, itemId: itemId, title: title, start: start))
        }
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
