import MavioKit
import MavioUI
import SwiftUI

/// A card on television: the artwork, which the ring of focus surrounds,
/// with the title and subtitle below it.
struct TVCard: View {
    let card: Card
    let wide: Bool
    /// The width; nil fills the space offered, as in grids.
    var width: CGFloat?
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            ArtworkView(
                wide ? card.thumb?.url ?? card.backdrop?.url ?? card.poster?.url : card.poster?.url ?? card.thumb?.url,
                title: card.title, icon: card.kind.icon
            )
            .aspectRatio(wide ? 16 / 9 : 2 / 3, contentMode: .fit)
            .overlay { Progress(card: card) }
            .focusRing()
            VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                Text(card.title).type(theme.type.headline).foregroundStyle(theme.colors.textPrimary)
                Text(card.subtitle ?? card.year.map(String.init) ?? " ").type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
            }
            .lineLimit(1)
        }
        .frame(width: width)
    }
}

/// The played check and the progress of a card begun.
private struct Progress: View {
    let card: Card
    @Environment(\.theme) private var theme

    var body: some View {
        ZStack {
            if card.played {
                Icon("played", fill: true, size: 28)
                    .foregroundStyle(.white)
                    .padding(MavioSpacing.s4)
                    .background(Circle().fill(theme.colors.scrim))
                    .padding(MavioSpacing.s12)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
            }
            if let progress = card.progress, progress > 0, !card.played {
                GeometryReader { geo in
                    ZStack(alignment: .leading) {
                        theme.colors.scrim
                        theme.colors.progress.frame(width: geo.size.width * CGFloat(min(progress, 1)))
                    }
                }
                .frame(height: 6)
                .frame(maxHeight: .infinity, alignment: .bottom)
            }
        }
    }
}

/// A card that opens its item's details.
struct CardLink: View {
    let card: Card
    let wide: Bool
    var width: CGFloat?

    var body: some View {
        NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
            TVCard(card: card, wide: wide, width: width)
        }
        .buttonStyle(CardButtonStyle(wide: wide))
    }
}

/// A row of Home or a library: its title and its cards, `layout.gutter`
/// apart, scrolling sideways with room for a card to grow (docs/tv.md §4,
/// §6.1).
struct RowView: View {
    let row: Row
    /// Whether the rows come from several servers, which the titles name.
    let several: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        switch row.content {
        case let .unavailable(message):
            Label {
                Text(MavioStrings.homeServerUnreachable(server: row.serverName ?? ""))
            } icon: {
                Icon("offline", size: 28)
            }
            .type(theme.type.callout)
            .foregroundStyle(theme.colors.textTertiary)
            .accessibilityHint(message)
            .padding(.horizontal, theme.layout.margin)
        case .loading:
            CardRow(title: row.serverName ?? "") {
                ForEach(0..<8, id: \.self) { _ in
                    CardPlaceholder().frame(width: theme.layout.poster)
                }
            }
            .redacted(reason: .placeholder)
        case let .items(cards):
            let wide = row.kind == .continueWatching || cards.allSatisfy(isWide)
            CardRow(title: title) {
                ForEach(cards, id: \.self) { card in
                    CardLink(card: card, wide: wide, width: wide ? theme.layout.thumbnail : theme.layout.poster)
                }
            }
        case .titles:
            // Discovered titles open in Discover, which Apple TV does not
            // have yet.
            EmptyView()
        }
    }

    private var title: String {
        guard several, let name = row.serverName, row.kind != .continueWatching else { return rowTitle(row) }
        return "\(rowTitle(row)) · \(name)"
    }
}

/// A titled row of cards scrolling sideways.
struct CardRow<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
            Text(title).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                .padding(.horizontal, theme.layout.margin)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: theme.layout.gutter) {
                    content
                }
                .padding(.horizontal, theme.layout.margin)
                .padding(.vertical, MavioSpacing.s32)
            }
            .scrollClipDisabled()
            .padding(.vertical, -MavioSpacing.s32)
        }
        .focusSection()
    }
}
