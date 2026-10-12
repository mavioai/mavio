import MavioKit
import MavioUI
import SwiftUI

/// An item's details: the header with its actions, its versions on this and
/// the user's other servers, its seasons, episodes or tracks, its cast and
/// items like it (libs/design/components.md).
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

private struct DetailsContent: View {
    let details: Details
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var sourceIndex = 0
    @State private var overviewExpanded = false

    private var card: Card { details.card }
    private var source: Source? {
        details.sources.indices.contains(sourceIndex) ? details.sources[sourceIndex] : details.sources.first
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                header
                VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                    if details.backdrop == nil, let poster = card.poster {
                        HStack(alignment: .bottom, spacing: MavioSpacing.s16) {
                            ArtworkView(poster.url, title: card.title)
                                .aspectRatio(2 / 3, contentMode: .fit)
                                .frame(width: theme.layout.poster)
                                .clipShape(.rect(cornerRadius: MavioRadius.md))
                            titleBlock
                        }
                    } else {
                        titleBlock
                    }
                    actions
                    if details.sources.count > 1 { versions }
                    if let overview = details.overview, !overview.isEmpty {
                        Text(overview)
                            .type(theme.type.body)
                            .foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(overviewExpanded || sizeClass == .regular ? nil : 3)
                            .onTapGesture { overviewExpanded.toggle() }
                    }
                    if !details.genres.isEmpty {
                        Text(details.genres.joined(separator: " · "))
                            .type(theme.type.footnote)
                            .foregroundStyle(theme.colors.textTertiary)
                    }
                }
                .padding(.horizontal, theme.layout.margin)
                .frame(maxWidth: 820, alignment: .leading)
                children
                if !details.credits.isEmpty {
                    Shelf(MavioStrings.itemCast) {
                        ForEach(details.credits, id: \.self) { PersonCard($0) }
                    }
                }
                if !details.similar.isEmpty {
                    Shelf(MavioStrings.itemSimilar) {
                        ForEach(details.similar, id: \.self) { CardLink(card: $0, width: theme.layout.poster) }
                    }
                }
            }
            .padding(.bottom, MavioSpacing.s32)
        }
        .ignoresSafeArea(edges: details.backdrop == nil ? [] : .top)
        .navigationTitle(card.title)
        .navigationBarTitleDisplayMode(.inline)
        .toolbarBackground(.hidden, for: .navigationBar)
    }

    @ViewBuilder private var header: some View {
        if let backdrop = details.backdrop {
            ArtworkView(backdrop.url)
                .aspectRatio(16 / 9, contentMode: .fill)
                .frame(maxWidth: .infinity, maxHeight: 520)
                .clipped()
                .overlay {
                    LinearGradient(
                        colors: [.clear, theme.colors.background], startPoint: .center, endPoint: .bottom)
                }
        }
    }

    private var titleBlock: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            if let logo = details.logo, let url = URL(string: logo.url) {
                AsyncImage(url: url) { image in
                    image.resizable().scaledToFit()
                } placeholder: {
                    Text(card.title).type(theme.type.title1)
                }
                .frame(maxWidth: 320, maxHeight: 120, alignment: .leading)
                .accessibilityLabel(card.title)
            } else {
                Text(card.title).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
            }
            if let subtitle = card.subtitle {
                Text(subtitle).type(theme.type.headline).foregroundStyle(theme.colors.textSecondary)
            }
            if let tagline = details.tagline, !tagline.isEmpty {
                Text(tagline).type(theme.type.callout).italic().foregroundStyle(theme.colors.textSecondary)
            }
            Text(facts.joined(separator: " · "))
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.textSecondary)
            if model.signedIn.count > 1 {
                Text(MavioStrings.itemOnServer(server: details.serverName))
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                    .lineLimit(1)
            }
        }
    }

    private var facts: [String] {
        var facts: [String] = []
        if let year = card.year { facts.append(String(year)) }
        if let rating = details.officialRating, !rating.isEmpty { facts.append(rating) }
        if let rating = details.communityRating { facts.append(String(format: "★ %.1f", rating)) }
        if let runtime = card.runtime, runtime > 0 {
            facts.append(Duration.seconds(runtime).formatted(.units(allowed: [.hours, .minutes], width: .narrow)))
        }
        return facts
    }

    private var download: DownloadEntry? {
        model.downloads.entry(serverId: card.serverId, itemId: card.itemId)
    }

    /// Play beside the other actions, or above them on a phone, where five
    /// buttons do not fit one row.
    @ViewBuilder private var actions: some View {
        if sizeClass == .compact {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                playButton.frame(maxWidth: .infinity)
                HStack(spacing: MavioSpacing.s12) { otherActions }
            }
            .tint(theme.colors.accent)
        } else {
            HStack(spacing: MavioSpacing.s12) {
                playButton.frame(minWidth: 120)
                otherActions
            }
            .tint(theme.colors.accent)
        }
    }

    @ViewBuilder private var playButton: some View {
        if let source {
            let resume = (details.position ?? 0) > 0 && source.itemId == card.itemId
            Button {
                play(source, start: resume ? details.position : nil)
            } label: {
                Label {
                    Text(resume ? MavioStrings.itemResume : MavioStrings.itemPlay).lineLimit(1)
                } icon: {
                    Icon("play", fill: true, size: 18)
                }
                .frame(maxWidth: sizeClass == .compact ? .infinity : nil)
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)
        }
    }

    /// Play from the start when there is a position to resume, favorite,
    /// played and download.
    @ViewBuilder private var otherActions: some View {
        if let source, (details.position ?? 0) > 0, source.itemId == card.itemId {
            Button {
                play(source, start: 0)
            } label: {
                Icon("retry", size: 20).accessibilityLabel(MavioStrings.itemPlay)
            }
            .buttonStyle(.bordered)
            .controlSize(.large)
        }
        Button {
            Task { try? await model.setFavorite(card, !card.favorite) }
        } label: {
            Icon("favorite", fill: card.favorite, size: 20)
                .accessibilityLabel(MavioStrings.itemFavorite)
        }
        .buttonStyle(.bordered)
        .controlSize(.large)
        Button {
            Task { try? await model.setPlayed(card, !card.played) }
        } label: {
            Icon("played", fill: card.played, size: 20)
                .accessibilityLabel(card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed)
        }
        .buttonStyle(.bordered)
        .controlSize(.large)
        if source != nil, !details.offline {
            downloadButton
        }
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
        .buttonStyle(.bordered)
        .controlSize(.large)
    }

    private func fraction(_ entry: DownloadEntry?) -> Double {
        guard let entry, entry.total > 0 else { return 0 }
        return Double(entry.received) / Double(entry.total)
    }

    private var versions: some View {
        Picker(MavioStrings.itemVersions, selection: $sourceIndex) {
            ForEach(details.sources.indices, id: \.self) { i in
                Text(label(details.sources[i])).tag(i)
            }
        }
        .pickerStyle(.menu)
    }

    private func label(_ source: Source) -> String {
        var parts = [source.name ?? source.container?.uppercased() ?? card.title]
        if let size = source.size {
            parts.append(ByteCountFormatter.string(fromByteCount: Int64(size), countStyle: .file))
        }
        if source.serverId != card.serverId { parts.append(MavioStrings.itemOnServer(server: source.serverName)) }
        return parts.joined(separator: " · ")
    }

    private func play(_ source: Source, start: TimeInterval?) {
        let file = source.itemId == card.itemId ? download.flatMap(model.downloads.file) : nil
        router.play(
            PlayRequest(
                serverId: source.serverId, itemId: source.itemId, title: card.title,
                mediaSourceId: source.mediaSourceId, start: start, file: file))
    }

    @ViewBuilder private var children: some View {
        let kids = details.children
        if !kids.isEmpty {
            switch card.kind {
            case .series:
                Shelf(MavioStrings.itemSeasons) {
                    ForEach(kids, id: \.self) { CardLink(card: $0, wide: false, width: theme.layout.poster) }
                }
            case .season, .album, .playlist, .audiobook:
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Text(card.kind == .season ? MavioStrings.itemEpisodes : MavioStrings.itemTracks)
                        .type(theme.type.title3)
                    ForEach(kids, id: \.self) { EpisodeRow(card: $0) }
                }
                .padding(.horizontal, theme.layout.margin)
            default:
                CardGrid(items: kids) { CardLink(card: $0) }
            }
        }
    }
}

/// An episode or track in a list.
struct EpisodeRow: View {
    let card: Card
    @Environment(\.theme) private var theme

    var body: some View {
        NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
            HStack(alignment: .top, spacing: MavioSpacing.s12) {
                if card.thumb != nil || card.kind == .episode {
                    ThumbCardArtwork(card: card).frame(width: theme.layout.thumbnail / 2)
                }
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    Text(card.title).type(theme.type.headline).lineLimit(2)
                        .foregroundStyle(theme.colors.textPrimary)
                    if let line = [card.subtitle, card.runtime.map(runtimeText)].compactMap({ $0 }).first {
                        Text(line).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                Spacer(minLength: 0)
                if card.played {
                    Icon("played", fill: true, size: 18).foregroundStyle(theme.colors.accent)
                }
            }
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }

    private func runtimeText(_ seconds: TimeInterval) -> String {
        Duration.seconds(seconds).formatted(.units(allowed: [.hours, .minutes], width: .narrow))
    }
}

/// A thumbnail card's artwork alone, with its progress.
struct ThumbCardArtwork: View {
    let card: Card
    @Environment(\.theme) private var theme

    var body: some View {
        ArtworkView(card.thumb?.url ?? card.poster?.url)
            .aspectRatio(16 / 9, contentMode: .fit)
            .overlay(alignment: .bottom) {
                if let progress = card.progress, progress > 0, !card.played {
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            theme.colors.scrim
                            theme.colors.progress.frame(width: geo.size.width * CGFloat(min(progress, 1)))
                        }
                    }
                    .frame(height: 3)
                }
            }
            .clipShape(.rect(cornerRadius: MavioRadius.md))
    }
}
