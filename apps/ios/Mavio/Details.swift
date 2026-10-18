import MavioKit
import MavioUI
import SwiftUI

/// An item's details, as the web UI's (libs/design/components.md, Details
/// Header and Details Screen): the header with what to watch and how, a
/// series' episodes by season, an episode's season, the cast, the extras,
/// the collection, items like it, and its details and media.
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
        .task(id: model.revision(serverId)) { await load() }
    }

    private func load() async {
        do {
            details = try await model.client.details(serverId: serverId, itemId: itemId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if details == nil { failure = error.localizedDescription }
        }
    }
}

/// What the primary action plays: the item itself, or a series' or
/// season's episode to watch next.
private struct NextUp {
    let itemId: String
    let name: String
    let position: TimeInterval?
    let progress: Float?
    let runtime: TimeInterval?
    let played: Bool
    /// Whether it is the item shown.
    let own: Bool
    /// The episode's numbers, as "S1 E2", when it is not the item shown.
    let episode: String?

    /// Where it resumes, when it is partly watched.
    var resumable: Bool { (position ?? 0) > 0 && !played }

    /// How far it was watched, from 0 to 1.
    var fraction: Float? {
        if let progress, progress > 0 { return progress }
        guard let position, let runtime, runtime > 0 else { return nil }
        return Float(min(position / runtime, 1))
    }
}

private struct DetailsContent: View {
    let details: Details
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.openURL) private var openURL
    @State private var sourceIndex = 0
    @State private var overviewOpen = false
    @State private var copy: Source?

    private var card: Card { details.card }
    private var phone: Bool { sizeClass != .regular }
    /// The files of this server.
    private var own: [Source] { details.sources.filter { $0.serverId == card.serverId } }
    /// The same title on the user's other servers.
    private var copies: [Source] { details.sources.filter { $0.serverId != card.serverId } }
    private var source: Source? { own.indices.contains(sourceIndex) ? own[sourceIndex] : own.first }
    private var inSeries: Bool { card.kind == .episode || card.kind == .season }
    private var trailer: Extra? { details.extras.first { $0.kind == .trailer } }

    private var next: NextUp? {
        if card.kind == .series || card.kind == .season {
            return details.next.map { e in
                NextUp(
                    itemId: e.card.itemId, name: e.card.title, position: e.position, progress: e.card.progress,
                    runtime: e.card.runtime, played: e.card.played, own: false, episode: short(e))
            }
        }
        guard source != nil else { return nil }
        return NextUp(
            itemId: card.itemId, name: card.title, position: details.position, progress: card.progress,
            runtime: card.runtime, played: card.played, own: true, episode: nil)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                header
                if phone {
                    VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                        actions
                        if let overview = details.overview { InlineOverview(text: overview) }
                    }
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.top, MavioSpacing.s16)
                }
                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                    episodes
                    children
                    cast
                    extras
                    if let collection = details.collection {
                        CollectionBanner(collection: collection, details: details)
                    }
                    if !details.similar.isEmpty {
                        Shelf(MavioStrings.itemSimilar) {
                            ForEach(details.similar, id: \.self) { CardLink(card: $0, width: theme.layout.poster) }
                        }
                    }
                    AboutSection(details: details, source: source, overview: !phone)
                }
                .padding(.top, MavioSpacing.s24)
            }
            .padding(.bottom, MavioSpacing.s40)
        }
        .background(theme.colors.background)
        .navigationTitle(card.title)
        .modifier(HeroChrome(title: card.title, phone: phone))
        .navigationDestination(item: $copy) { copy in
            DetailsView(serverId: copy.serverId, itemId: copy.itemId)
        }
        .sheet(isPresented: $overviewOpen) { OverviewSheet(details: details) }
    }

    // MARK: Header

    /// The artwork, and over its lower left the series an episode or season
    /// belongs to, the logo or title, the facts and badges; on an iPad the
    /// tagline, overview and actions too.
    private var header: some View {
        // The artwork fills the header's frame rather than sizing it.
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
                    if !phone, image == nil, let poster = card.poster {
                        ArtworkView(poster.url, title: card.title, icon: card.kind.icon)
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
            .heroBleed((image ?? card.poster)?.url, phone: phone)
            .environment(\.colorScheme, .dark)
    }

    /// The backdrop, else an episode's still.
    private var image: Artwork? {
        details.backdrop ?? (card.kind == .episode ? card.thumb : nil)
    }

    @ViewBuilder private var artwork: some View {
        if let image {
            ArtworkView(image.url, icon: card.kind.icon)
        } else if let poster = card.poster {
            // Without a backdrop the poster, blurred, colors the header.
            ArtworkView(poster.url)
                .scaleEffect(1.25)
                .blur(radius: 48)
                .opacity(0.7)
                .background(theme.colors.surface)
        } else {
            theme.colors.surface
        }
    }

    private var headerText: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            if let series = details.series {
                NavigationLink(value: Route.item(serverId: card.serverId, itemId: series.itemId)) {
                    if let logo = series.logo {
                        FittedImage(logo.url, alignment: .leading) { Text(series.name) }
                            .frame(maxHeight: 48, alignment: .leading)
                            .frame(maxWidth: 240, alignment: .leading)
                            .accessibilityLabel(series.name)
                    } else {
                        Text(series.name).type(theme.type.headline)
                    }
                }
                .buttonStyle(.plain)
                .foregroundStyle(.white.opacity(0.85))
            }
            if let season = details.seasonNumber, let episode = details.episodeNumber {
                Text(MavioStrings.itemEpisodeNumber(season: String(season), episode: String(episode)))
                    .type(theme.type.headline)
                    .foregroundStyle(.white.opacity(0.85))
            }
            if !inSeries, let logo = details.logo {
                FittedImage(logo.url, alignment: .leading) { title }
                    .frame(maxWidth: 320, maxHeight: 112, alignment: .leading)
                    .accessibilityLabel(card.title)
            } else {
                title
            }
            if !facts.isEmpty {
                // Each separator keeps to the fact before it, so that no line starts with one.
                Text(facts.map { $0.replacingOccurrences(of: " ", with: "\u{00a0}") }.joined(separator: "\u{00a0}· "))
                    .type(theme.type.callout)
                    .foregroundStyle(.white.opacity(0.8))
            }
            if !badges.isEmpty || model.signedIn.count > 1 {
                Flow(spacing: MavioSpacing.s12, lineSpacing: 6) {
                    ForEach(badges, id: \.self) { badge in
                        Text(badge)
                            .type(theme.type.caption).fontWeight(.semibold)
                            .foregroundStyle(.white.opacity(0.9))
                            .padding(.horizontal, 6)
                            .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(.white.opacity(0.5)))
                    }
                    if model.signedIn.count > 1 {
                        Text(details.serverName)
                            .type(theme.type.caption)
                            .foregroundStyle(.white.opacity(0.8))
                            .padding(.horizontal, 6)
                            .background(.white.opacity(0.2), in: .rect(cornerRadius: 4))
                    }
                }
            }
            if !phone {
                if let tagline = details.tagline {
                    Text(tagline).type(theme.type.headline).foregroundStyle(.white.opacity(0.9))
                }
                if let overview = details.overview {
                    Button {
                        overviewOpen = true
                    } label: {
                        Text(overview).type(theme.type.body).lineLimit(3).multilineTextAlignment(.leading)
                            .foregroundStyle(.white.opacity(0.85))
                    }
                    .buttonStyle(.plain)
                }
                actions.padding(.top, MavioSpacing.s4)
            }
        }
        .foregroundStyle(.white)
    }

    private var title: some View {
        Text(card.title)
            .font(.system(size: phone ? 34 : 52, weight: .bold))
            .lineSpacing(0)
            .foregroundStyle(.white)
    }

    /// The year, or a series' years and seasons, a season's episodes, an
    /// episode's air date, the runtime and up to three genres.
    private var facts: [String] {
        var facts: [String] = []
        switch card.kind {
        case .series:
            if let years { facts.append(years) }
            if !details.children.isEmpty { facts.append(MavioStrings.itemSeasonCount(count: details.children.count)) }
        case .season:
            if let year = card.year { facts.append(String(year)) }
            if !details.episodes.isEmpty { facts.append(MavioStrings.itemEpisodeCount(count: details.episodes.count)) }
        case .episode:
            if let premiere = details.premiere { facts.append(premiere.formatted(date: .abbreviated, time: .omitted)) }
        default:
            if let year = card.year { facts.append(String(year)) }
        }
        if card.kind != .series, let runtime = card.runtime { facts.append(minutesText(runtime)) }
        if card.kind != .episode, !details.genres.isEmpty {
            facts.append(details.genres.prefix(3).joined(separator: " / "))
        }
        return facts
    }

    /// The years a series ran, as "2023–" while it continues.
    private var years: String? {
        guard let from = card.year else { return nil }
        let to = details.endDate.map { Calendar.current.component(.year, from: $0) }
        if let to, to != from { return "\(from)–\(to)" }
        return details.seriesStatus == .continuing ? "\(from)–" : String(from)
    }

    /// The official rating and the chosen version's resolution, range,
    /// surround and subtitles.
    private var badges: [String] {
        [
            details.officialRating, source?.resolution, source?.range, source?.surround,
            source?.subtitles.isEmpty == false ? "CC" : nil,
        ].compactMap(\.self)
    }

    // MARK: Actions

    /// Play, or what plays next, then as circles Play from Beginning, the
    /// trailer, favorite, played, the version menu and the download; under
    /// them the progress, the episode's name, the time left and when it would
    /// end.
    private var actions: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            if phone {
                if let next { playButton(next).frame(maxWidth: .infinity) }
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) { otherActions }
            } else {
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                    if let next { playButton(next) }
                    otherActions
                }
            }
            if let next { status(next) }
        }
    }

    private func playButton(_ next: NextUp) -> some View {
        let label =
            if let episode = next.episode {
                next.resumable
                    ? MavioStrings.itemResumeEpisode(episode: episode) : MavioStrings.itemPlayEpisode(episode: episode)
            } else {
                next.resumable ? MavioStrings.itemResume : MavioStrings.itemPlay
            }
        return Button {
            play(next, start: next.resumable ? next.position : nil)
        } label: {
            Label {
                Text(label).lineLimit(1)
            } icon: {
                Icon("play", fill: true, size: 20)
            }
            .frame(maxWidth: phone ? .infinity : nil)
        }
        .buttonStyle(MavioButtonStyle(phone ? .primary : .artworkPrimary, large: phone))
    }

    private var circle: MavioButtonStyle { phone ? .circle : .artworkCircle }

    @ViewBuilder private var otherActions: some View {
        if let next, next.resumable {
            Button {
                play(next, start: 0)
            } label: {
                Icon("previous", size: 20).accessibilityLabel(MavioStrings.itemFromStart)
            }
            .buttonStyle(circle)
        }
        if let trailer {
            Button {
                router.play(
                    PlayRequest(serverId: trailer.card.serverId, itemId: trailer.card.itemId, title: trailer.card.title)
                )
            } label: {
                Icon("movies", size: 20).accessibilityLabel(MavioStrings.itemTrailer)
            }
            .buttonStyle(circle)
        } else if let url = details.remoteTrailer.flatMap(URL.init(string:)) {
            Button {
                openURL(url)
            } label: {
                Icon("movies", size: 20).accessibilityLabel(MavioStrings.itemTrailer)
            }
            .buttonStyle(circle)
        }
        Button {
            Task { try? await model.setFavorite(card, !card.favorite) }
        } label: {
            Icon("favorite", fill: card.favorite, size: 20)
                .foregroundStyle(card.favorite ? Color(red: 1, green: 0.27, blue: 0.23) : .primary)
                .accessibilityLabel(MavioStrings.itemFavorite)
        }
        .buttonStyle(circle)
        Button {
            Task { try? await model.setPlayed(card, !card.played) }
        } label: {
            Icon("played", fill: card.played, size: 20)
                .accessibilityLabel(card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed)
        }
        .buttonStyle(circle)
        if own.count > 1 || !copies.isEmpty { versionMenu }
        if source != nil, !details.offline { downloadButton }
    }

    private func status(_ next: NextUp) -> some View {
        let parts = [
            next.episode == nil ? nil : next.name,
            next.resumable ? next.left.map { MavioStrings.itemTimeLeft(time: minutesText($0)) } : nil,
            next.endsAt.map { MavioStrings.itemEndsAt(time: $0.formatted(date: .omitted, time: .shortened)) },
        ].compactMap(\.self)
        return HStack(spacing: MavioSpacing.s12) {
            if next.resumable, let progress = next.fraction {
                Capsule()
                    .fill(phone ? theme.colors.fillSelected : .white.opacity(0.35))
                    .frame(width: 160, height: 4)
                    .overlay(alignment: .leading) {
                        Capsule()
                            .fill(phone ? theme.colors.progress : .white)
                            .frame(width: 160 * CGFloat(progress))
                    }
            }
            if !parts.isEmpty {
                Text(parts.joined(separator: " · "))
                    .type(theme.type.footnote)
                    .foregroundStyle(phone ? theme.colors.textSecondary : .white.opacity(0.8))
            }
        }
    }

    /// The version to play, then the same title on the user's other servers.
    private var versionMenu: some View {
        Menu {
            if own.count > 1 {
                Section(MavioStrings.itemVersions) {
                    ForEach(own.indices, id: \.self) { i in
                        // A toggle in a menu is checked when on, and shows its second text under the first.
                        Toggle(isOn: Binding(get: { sourceIndex == i }, set: { _ in sourceIndex = i })) {
                            Text(own[i].versionName)
                            Text(versionDetails(own[i]))
                        }
                    }
                }
            }
            Section {
                ForEach(copies, id: \.self) { copy in
                    Button(MavioStrings.itemOnServer(server: copy.serverName), systemImage: "server.rack") {
                        self.copy = copy
                    }
                }
            }
        } label: {
            HStack(spacing: MavioSpacing.s8) {
                Text(MavioStrings.itemVersion)
                    .foregroundStyle(phone ? theme.colors.textSecondary : .white.opacity(0.7))
                Text(source?.versionName ?? details.serverName)
                Icon("expand", size: 14)
            }
            .type(theme.type.callout)
            .fontWeight(.semibold)
            .foregroundStyle(phone ? theme.colors.textPrimary : .white)
            .padding(.horizontal, MavioSpacing.s16)
            .frame(height: 44)
            .modifier(PillFill(phone: phone))
        }
        .menuOrder(.fixed)
    }

    private func versionDetails(_ s: Source) -> String {
        [s.resolution, s.range, s.surround, s.format, s.sizeText].compactMap(\.self).joined(separator: " · ")
    }

    private var download: DownloadEntry? {
        model.downloads.entry(serverId: card.serverId, itemId: card.itemId)
    }

    @ViewBuilder private var downloadButton: some View {
        let downloads = model.downloads
        Menu {
            if let entry = download {
                if entry.state == .paused || entry.state == .failed {
                    Button(MavioStrings.downloadResume) { Task { try? await downloads.resume(entry) } }
                }
                Button(MavioStrings.itemRemoveDownload, role: .destructive) { Task { await downloads.remove(entry) } }
            } else {
                Button(MavioStrings.itemDownload) {
                    Task { try? await downloads.add(serverId: card.serverId, itemId: card.itemId) }
                }
            }
        } label: {
            Group {
                switch download?.state {
                case .running?, .queued?:
                    ProgressView(value: fraction(download)).progressViewStyle(.circular)
                case .done?: Icon("downloads", fill: true, size: 20)
                default: Icon("downloads", size: 20)
                }
            }
            .accessibilityLabel(MavioStrings.itemDownload)
        }
        .menuStyle(.button)
        .buttonStyle(circle)
    }

    private func fraction(_ entry: DownloadEntry?) -> Double {
        guard let entry, entry.total > 0 else { return 0 }
        return Double(entry.received) / Double(entry.total)
    }

    private func play(_ next: NextUp, start: TimeInterval?) {
        if next.own, let source {
            let here = source.itemId == card.itemId
            let file = here ? download.flatMap(model.downloads.file) : nil
            router.play(
                PlayRequest(
                    serverId: source.serverId, itemId: source.itemId, title: card.title,
                    mediaSourceId: source.mediaSourceId, start: here ? start : nil, file: file))
        } else {
            router.play(PlayRequest(serverId: card.serverId, itemId: next.itemId, title: next.name, start: start))
        }
    }

    // MARK: Sections

    /// A series' seasons and their episodes, a season's episodes, or an
    /// episode's season.
    @ViewBuilder private var episodes: some View {
        switch card.kind {
        case .series:
            SeasonsSection(seasons: details.children, episodes: details.episodes, current: details.next?.card.itemId)
        case .season:
            EpisodeList(episodes: details.episodes, current: details.next?.card.itemId)
        case .episode where details.episodes.count > 1:
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Text(MavioStrings.itemMoreFrom(season: details.seasonName ?? String(details.seasonNumber ?? 0)))
                    .type(theme.type.title3)
                    .padding(.horizontal, theme.layout.margin)
                EpisodeCards(episodes: details.episodes, current: card.itemId, scrollTo: card.itemId)
            }
            .padding(.vertical, MavioSpacing.s8)
        default:
            EmptyView()
        }
    }

    /// An album's tracks, a playlist's entries or a folder's items.
    @ViewBuilder private var children: some View {
        let kids = details.children
        if !kids.isEmpty {
            switch card.kind {
            case .series, .season, .episode:
                EmptyView()
            case .album, .playlist, .audiobook:
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Text(MavioStrings.itemTracks).type(theme.type.title3)
                    ForEach(kids, id: \.self) { TrackRow(card: $0) }
                }
                .padding(.horizontal, theme.layout.margin)
            default:
                CardGrid(items: kids) { CardLink(card: $0) }
            }
        }
    }

    /// The cast in billing order, then the creators, directors and writers.
    @ViewBuilder private var cast: some View {
        let shown = details.credits.filter { $0.job.isCast || [.creator, .director, .writer].contains($0.job) }
        if !shown.isEmpty {
            Shelf(MavioStrings.itemCast) {
                Spacer()
                NavigationLink(
                    MavioStrings.creditsAll,
                    value: Route.credits(serverId: card.serverId, itemId: card.itemId, crew: false)
                )
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.accentText)
            } content: {
                ForEach(Array(shown.enumerated()), id: \.offset) { _, credit in
                    PersonCard(credit, role: credit.shownRole)
                }
            }
        }
    }

    /// The trailers, featurettes and other extras, each played by itself.
    @ViewBuilder private var extras: some View {
        if !details.extras.isEmpty {
            Shelf(MavioStrings.itemExtras) {
                ForEach(details.extras, id: \.self) { extra in
                    Button {
                        router.play(
                            PlayRequest(
                                serverId: extra.card.serverId, itemId: extra.card.itemId, title: extra.card.title))
                    } label: {
                        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                            ArtworkView(
                                extra.card.thumb?.url ?? details.backdrop?.url, title: extra.card.title, icon: "shows"
                            )
                            .frame(width: theme.layout.thumbnail, height: theme.layout.thumbnail * 9 / 16)
                            .overlay(alignment: .bottomLeading) {
                                Icon("play", fill: true, size: 14)
                                    .foregroundStyle(.white)
                                    .padding(6)
                                    .background(theme.colors.scrim, in: Circle())
                                    .padding(MavioSpacing.s8)
                            }
                            .clipShape(.rect(cornerRadius: MavioRadius.md))
                            Text(extra.card.title).type(theme.type.headline).lineLimit(1)
                                .foregroundStyle(theme.colors.textPrimary)
                            Text(
                                [extra.kind.label, extra.card.runtime.map(minutesText)].compactMap(\.self).joined(
                                    separator: " · ")
                            )
                            .type(theme.type.footnote).lineLimit(1)
                            .foregroundStyle(theme.colors.textSecondary)
                        }
                        .frame(width: theme.layout.thumbnail)
                        .fixedSize(horizontal: false, vertical: true)
                        .contentShape(.rect)
                    }
                    .buttonStyle(.plain)
                }
            }
        }
    }

    private func short(_ e: Episode) -> String? {
        guard let number = e.number else { return nil }
        return MavioStrings.itemEpisodeShort(season: String(e.season ?? 0), episode: String(number))
    }
}

/// The header's size: on a phone a square as wide as the screen, else 70%
/// of the window's height, from 520 to 700 pt.
private struct HeaderSize: ViewModifier {
    let phone: Bool

    func body(content: Content) -> some View {
        if phone {
            content.aspectRatio(1, contentMode: .fit)
        } else {
            content.containerRelativeFrame(.vertical) { height, _ in min(max(520, height * 0.7), 700) }
        }
    }
}

/// A pill's fill: the page's fill on a phone, glass over artwork.
private struct PillFill: ViewModifier {
    let phone: Bool
    @Environment(\.theme) private var theme

    func body(content: Content) -> some View {
        if phone {
            content.background(theme.colors.fill, in: Capsule())
        } else {
            content.glass(in: Capsule())
        }
    }
}

extension NextUp {
    /// The time left.
    fileprivate var left: TimeInterval? {
        guard let runtime, let position else { return nil }
        return max(60, runtime - position)
    }

    /// When it would end if played now.
    fileprivate var endsAt: Date? {
        guard let runtime else { return nil }
        let left = runtime - (resumable ? position ?? 0 : 0)
        return left > 0 ? .now.addingTimeInterval(left) : nil
    }
}

/// A length of time, as "1h 52m".
func minutesText(_ seconds: TimeInterval) -> String {
    Duration.seconds((seconds / 60).rounded() * 60).formatted(.units(allowed: [.hours, .minutes], width: .narrow))
}

/// The overview, three lines long with More opening it whole in place.
private struct InlineOverview: View {
    let text: String
    @State private var open = false
    @State private var truncated = false
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            Text(text)
                .type(theme.type.body)
                .foregroundStyle(theme.colors.textSecondary)
                .lineLimit(open ? nil : 3)
                .background {
                    // The whole text, measured beside the three lines, tells whether they cut it.
                    ViewThatFits(in: .vertical) {
                        Text(text).type(theme.type.body).fixedSize(horizontal: false, vertical: true).hidden()
                        Color.clear.onAppear { truncated = true }
                    }
                }
            if truncated, !open {
                Button(MavioStrings.itemMore) { withAnimation { open = true } }
                    .type(theme.type.callout).fontWeight(.semibold)
                    .foregroundStyle(theme.colors.textPrimary)
            }
        }
    }
}

/// The tagline and the whole overview.
private struct OverviewSheet: View {
    let details: Details
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    if let tagline = details.tagline { Text(tagline).type(theme.type.headline) }
                    if let overview = details.overview {
                        Text(overview).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                .padding(MavioSpacing.s24)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .navigationTitle(details.card.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(MavioStrings.actionDone) { dismiss() }
                }
            }
        }
        .presentationDetents([.medium, .large])
    }
}

// MARK: - Episodes

/// A series' episodes by season: the seasons as pills, starting with the
/// season of the episode to watch next, and the chosen one's episodes in a
/// row, with See All opening the season.
private struct SeasonsSection: View {
    let seasons: [Card]
    let episodes: [Episode]
    let current: String?
    @State private var chosen: String?
    @Environment(\.theme) private var theme

    private var currentSeason: String? { episodes.first { $0.card.itemId == current }?.seasonId }
    private var season: Card? {
        seasons.first { $0.itemId == chosen } ?? seasons.first { $0.itemId == currentSeason } ?? seasons.first
    }

    var body: some View {
        if let season {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                HStack(spacing: MavioSpacing.s16) {
                    Text(MavioStrings.itemEpisodes).type(theme.type.title3).layoutPriority(1)
                    if seasons.count > 1 {
                        ScrollView(.horizontal) {
                            HStack(spacing: MavioSpacing.s4) {
                                ForEach(seasons, id: \.itemId) { s in
                                    let on = s.itemId == season.itemId
                                    Button(s.title) { chosen = s.itemId }
                                        .type(theme.type.callout).fontWeight(.semibold)
                                        .foregroundStyle(on ? theme.colors.textPrimary : theme.colors.textSecondary)
                                        .padding(.horizontal, MavioSpacing.s16)
                                        .padding(.vertical, 6)
                                        .background(on ? theme.colors.fillSelected : .clear, in: Capsule())
                                        .buttonStyle(.plain)
                                        .accessibilityAddTraits(on ? .isSelected : [])
                                }
                            }
                        }
                        .scrollIndicators(.hidden)
                    } else {
                        Text(season.title).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                        Spacer()
                    }
                    NavigationLink(
                        MavioStrings.homeMore, value: Route.item(serverId: season.serverId, itemId: season.itemId)
                    )
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.accentText)
                    .layoutPriority(1)
                }
                .padding(.horizontal, theme.layout.margin)
                EpisodeCards(
                    episodes: episodes.filter { $0.seasonId == season.itemId }, current: current, scrollTo: current
                )
                .id(season.itemId)
            }
            .padding(.vertical, MavioSpacing.s8)
        }
    }
}

/// Episodes side by side, each with its still, number, name, length and
/// overview, starting at scrollTo.
private struct EpisodeCards: View {
    let episodes: [Episode]
    let current: String?
    let scrollTo: String?
    @State private var position = ScrollPosition(idType: String.self)
    @Environment(\.theme) private var theme

    var body: some View {
        ScrollView(.horizontal) {
            LazyHStack(alignment: .top, spacing: theme.layout.gutter * 1.5) {
                ForEach(episodes, id: \.card.itemId) { e in
                    NavigationLink(value: Route.item(serverId: e.card.serverId, itemId: e.card.itemId)) {
                        EpisodeCard(episode: e, current: e.card.itemId == current)
                    }
                    .buttonStyle(.plain)
                }
            }
            .scrollTargetLayout()
        }
        .contentMargins(.horizontal, theme.layout.margin, for: .scrollContent)
        .scrollTargetBehavior(.viewAligned)
        .scrollIndicators(.hidden)
        .scrollPosition($position, anchor: .leading)
        .onAppear { if let scrollTo { position.scrollTo(id: scrollTo, anchor: .leading) } }
    }
}

/// An episode in a row (libs/design/components.md, Episode Card).
private struct EpisodeCard: View {
    let episode: Episode
    let current: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s2) {
            EpisodeStill(card: episode.card)
                .frame(width: theme.layout.thumbnail, height: theme.layout.thumbnail * 9 / 16)
                .padding(.bottom, MavioSpacing.s4)
            HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                if let number = episode.short {
                    Text(number).type(theme.type.footnote).fontWeight(.semibold)
                        .foregroundStyle(current ? theme.colors.accentText : theme.colors.textSecondary)
                }
                Text(episode.facts).type(theme.type.footnote).lineLimit(1)
                    .foregroundStyle(theme.colors.textTertiary)
            }
            Text(episode.card.title).type(theme.type.headline).lineLimit(1)
                .foregroundStyle(theme.colors.textPrimary)
            if let overview = episode.overview {
                Text(overview).type(theme.type.footnote).lineLimit(3)
                    .foregroundStyle(theme.colors.textSecondary)
            }
        }
        .frame(width: theme.layout.thumbnail, alignment: .topLeading)
        .fixedSize(horizontal: false, vertical: true)
        .contentShape(.rect)
    }
}

/// A season's episodes, one under another (libs/design/components.md,
/// Episode Row).
private struct EpisodeList: View {
    let episodes: [Episode]
    let current: String?
    @Environment(\.theme) private var theme

    var body: some View {
        if !episodes.isEmpty {
            VStack(alignment: .leading, spacing: 0) {
                Text(MavioStrings.itemEpisodes).type(theme.type.title3)
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.bottom, MavioSpacing.s8)
                ForEach(episodes, id: \.card.itemId) { e in
                    NavigationLink(value: Route.item(serverId: e.card.serverId, itemId: e.card.itemId)) {
                        HStack(alignment: .top, spacing: MavioSpacing.s20) {
                            EpisodeStill(card: e.card)
                                .containerRelativeFrame(.horizontal) { w, _ in
                                    min(max(140, w * 0.32), theme.layout.thumbnail)
                                }
                            VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                                if let number = e.short {
                                    Text(number).type(theme.type.footnote).fontWeight(.semibold)
                                        .foregroundStyle(
                                            e.card.itemId == current
                                                ? theme.colors.accentText : theme.colors.textSecondary)
                                }
                                Text(e.card.title).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                                Text(e.facts).type(theme.type.footnote).foregroundStyle(theme.colors.textTertiary)
                                if let overview = e.overview {
                                    Text(overview).type(theme.type.callout).lineLimit(3)
                                        .foregroundStyle(theme.colors.textSecondary)
                                        .padding(.top, 6)
                                }
                            }
                            .padding(.vertical, MavioSpacing.s4)
                            Spacer(minLength: 0)
                        }
                        .padding(.horizontal, theme.layout.margin)
                        .padding(.vertical, MavioSpacing.s12)
                        .contentShape(.rect)
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.vertical, MavioSpacing.s8)
        }
    }
}

/// An episode's still with its progress and the played check.
private struct EpisodeStill: View {
    let card: Card
    @Environment(\.theme) private var theme

    var body: some View {
        ArtworkView(card.thumb?.url ?? card.poster?.url, title: card.title, icon: "shows")
            .aspectRatio(16 / 9, contentMode: .fit)
            .overlay(alignment: .bottom) {
                if let progress = card.progress, progress > 0, progress < 1, !card.played {
                    Capsule().fill(.white.opacity(0.35)).frame(height: 4)
                        .overlay(alignment: .leading) {
                            GeometryReader { geo in
                                Capsule().fill(.white).frame(width: geo.size.width * CGFloat(progress))
                            }
                        }
                        .padding(10)
                }
            }
            .overlay(alignment: .topTrailing) {
                if card.played {
                    Icon("played", fill: true, size: 18)
                        .foregroundStyle(.white)
                        .padding(MavioSpacing.s2)
                        .background(theme.colors.scrim, in: Circle())
                        .padding(6)
                }
            }
            .clipShape(.rect(cornerRadius: MavioRadius.md))
    }
}

extension Episode {
    /// The numbers, as "S1 E2".
    fileprivate var short: String? {
        guard let number else { return nil }
        return MavioStrings.itemEpisodeShort(season: String(season ?? 0), episode: String(number))
    }

    /// The runtime and air date.
    fileprivate var facts: String {
        [card.runtime.map(minutesText), premiere?.formatted(date: .abbreviated, time: .omitted)]
            .compactMap(\.self).joined(separator: " · ")
    }
}

/// A track of an album or an entry of a playlist.
struct TrackRow: View {
    let card: Card
    @Environment(\.theme) private var theme

    var body: some View {
        NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
            HStack(spacing: MavioSpacing.s12) {
                Text(card.title).type(theme.type.body).lineLimit(1)
                    .foregroundStyle(theme.colors.textPrimary)
                Spacer(minLength: 0)
                if let runtime = card.runtime {
                    Text(minutesText(runtime)).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                }
            }
            .padding(.vertical, MavioSpacing.s8)
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }
}

// MARK: - Collection

/// The collection a movie belongs to, over its backdrop, else the movie's.
private struct CollectionBanner: View {
    let collection: CollectionLink
    let details: Details
    @Environment(\.theme) private var theme

    var body: some View {
        NavigationLink(value: Route.item(serverId: details.card.serverId, itemId: collection.itemId)) {
            HStack(spacing: MavioSpacing.s16) {
                VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                    Text(MavioStrings.itemCollection).type(theme.type.footnote).fontWeight(.semibold)
                        .foregroundStyle(.white.opacity(0.7))
                    Text(collection.name).type(theme.type.title3).lineLimit(1)
                }
                Spacer(minLength: 0)
                HStack(spacing: MavioSpacing.s4) {
                    Text(MavioStrings.itemViewCollection)
                    Icon("forward", size: 14)
                }
                .fixedSize()
                .type(theme.type.callout).fontWeight(.semibold)
                .padding(.horizontal, MavioSpacing.s16)
                .frame(height: 36)
                .glass(in: Capsule())
            }
            .foregroundStyle(.white)
            .padding(.horizontal, MavioSpacing.s24)
            .padding(.vertical, MavioSpacing.s20)
            .frame(maxWidth: .infinity, minHeight: 112)
            .background {
                ZStack {
                    ArtworkView((collection.backdrop ?? details.backdrop)?.url, icon: "collections")
                    LinearGradient(
                        colors: [.black.opacity(0.8), .black.opacity(0.55), .black.opacity(0.3)],
                        startPoint: .leading, endPoint: .trailing)
                }
            }
            .clipShape(.rect(cornerRadius: 14))
            .environment(\.colorScheme, .dark)
        }
        .buttonStyle(.plain)
        .padding(.horizontal, theme.layout.margin)
        .padding(.vertical, MavioSpacing.s12)
    }
}
