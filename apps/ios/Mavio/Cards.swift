import MavioKit
import MavioUI
import SwiftUI

/// A card opening an item's details, with its actions in a context menu.
struct CardLink: View {
    let card: Card
    /// Draws a thumbnail rather than a poster.
    var wide: Bool? = nil
    /// The width; nil fills the space offered, as in grids.
    var width: CGFloat? = nil
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
            if wide ?? isWide(card) {
                ThumbCard(card, width: width)
            } else {
                PosterCard(card, width: width)
            }
        }
        .buttonStyle(.plain)
        .contextMenu {
            Button(card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed) {
                Task { try? await model.setPlayed(card, !card.played) }
            }
            Button(MavioStrings.itemFavorite, systemImage: card.favorite ? "heart.fill" : "heart") {
                Task { try? await model.setFavorite(card, !card.favorite) }
            }
        }
    }
}

/// A grid of cards, as many columns as fit the form factor's card width.
struct CardGrid<Item: Hashable, Content: View>: View {
    let items: [Item]
    var wide = false
    @ViewBuilder let content: (Item) -> Content
    @Environment(\.theme) private var theme

    var body: some View {
        LazyVGrid(
            columns: [
                GridItem(
                    .adaptive(minimum: wide ? theme.layout.thumbnail * 0.8 : theme.layout.poster),
                    spacing: theme.layout.gutter, alignment: .top)
            ],
            spacing: theme.layout.gutter
        ) {
            ForEach(items, id: \.self) { content($0) }
        }
        .padding(.horizontal, theme.layout.margin)
    }
}

/// A row of Home or of a library's Recommended tab.
struct RowView: View {
    let row: Row
    /// Whether the rows come from several servers, which the badges name.
    let several: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        switch row.content {
        case let .unavailable(message):
            Label {
                Text(MavioStrings.homeServerUnreachable(server: row.serverName ?? ""))
            } icon: {
                Icon("offline", size: 18)
            }
            .type(theme.type.callout)
            .foregroundStyle(theme.colors.textTertiary)
            .help(message)
            .padding(.horizontal, theme.layout.margin)
            .padding(.vertical, MavioSpacing.s12)
        case .loading:
            Shelf(row.serverName ?? "") {
                ForEach(0..<6, id: \.self) { _ in CardPlaceholder() }
            }
            .redacted(reason: .placeholder)
        case let .items(cards):
            let wide = row.kind == .continueWatching || cards.allSatisfy(isWide)
            Shelf(rowTitle(row)) {
                accessory
            } content: {
                ForEach(cards, id: \.self) { card in
                    CardLink(
                        card: card, wide: wide, width: wide ? theme.layout.thumbnail : theme.layout.poster)
                }
            }
        case let .titles(titles):
            Shelf(rowTitle(row)) {
                accessory
            } content: {
                ForEach(titles, id: \.self) { title in
                    TitleCardView(title: title)
                }
            }
        }
    }

    @ViewBuilder private var accessory: some View {
        if several, let name = row.serverName, row.kind != .continueWatching {
            Text(name)
                .type(theme.type.caption)
                .foregroundStyle(theme.colors.textSecondary)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .background(theme.colors.surfaceRaised, in: .rect(cornerRadius: MavioRadius.sm))
        }
        if row.offline {
            Label {
                Text(MavioStrings.homeOffline)
            } icon: {
                Icon("offline", size: 14)
            }
            .type(theme.type.caption)
            .foregroundStyle(theme.colors.textTertiary)
        }
        Spacer()
        if row.more, let serverId = row.serverId, let hubId = row.hubId {
            NavigationLink(
                MavioStrings.homeMore, value: Route.hub(serverId: serverId, hubId: hubId, title: rowTitle(row))
            )
            .type(theme.type.callout)
        }
    }
}

/// A title discovery offers, opening the copy a server has.
struct TitleCardView: View {
    let title: TitleCard
    @Environment(\.theme) private var theme

    var body: some View {
        let poster = VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            ArtworkView(title.posterUrl, title: title.title)
                .aspectRatio(2 / 3, contentMode: .fit)
                .clipShape(.rect(cornerRadius: MavioRadius.md))
            Text(title.title).type(theme.type.headline).lineLimit(1)
            Text(title.year.map(String.init) ?? " ").type(theme.type.footnote).lineLimit(1)
                .foregroundStyle(theme.colors.textSecondary)
        }
        .frame(width: theme.layout.poster)
        if let copy = title.available.first {
            NavigationLink(value: Route.item(serverId: copy.serverId, itemId: copy.itemId)) { poster }
                .buttonStyle(.plain)
        } else {
            poster
        }
    }
}
