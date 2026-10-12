import MavioDesign
import MavioKit
import SwiftUI

/// An image loaded from a server, over a placeholder of its final size.
public struct ArtworkView: View {
    private let url: String?
    private let title: String?
    @Environment(\.theme) private var theme

    /// The image at url; without one, title on the placeholder.
    public init(_ url: String?, title: String? = nil) {
        self.url = url
        self.title = title
    }

    /// The image, or its placeholder.
    public var body: some View {
        ZStack {
            theme.colors.surfaceRaised
            if let url, let parsed = URL(string: url) {
                AsyncImage(url: parsed, transaction: Transaction(animation: .easeOut(duration: MavioDuration.fast))) {
                    phase in
                    if let image = phase.image {
                        image.resizable().scaledToFill()
                    } else if phase.error != nil, let title {
                        fallback(title)
                    }
                }
            } else if let title {
                fallback(title)
            }
        }
        .clipped()
    }

    private func fallback(_ title: String) -> some View {
        Text(title)
            .type(theme.type.title3)
            .foregroundStyle(theme.colors.textSecondary)
            .multilineTextAlignment(.center)
            .padding(MavioSpacing.s8)
    }
}

/// The overlays of a card's artwork: watched progress, the played check and
/// the time left.
struct CardOverlays: View {
    let card: Card
    let timeLeft: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        ZStack {
            if card.played {
                Icon("played", fill: true, size: 22)
                    .foregroundStyle(theme.colors.accent)
                    .background(Circle().fill(theme.colors.onAccent).padding(3))
                    .padding(MavioSpacing.s8)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
            }
            if let progress = card.progress, progress > 0, !card.played {
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    Spacer()
                    if timeLeft, let left = minutesLeft(card) {
                        Text(MavioStrings.itemMinutesLeft(count: left))
                            .type(theme.type.caption)
                            .foregroundStyle(.white)
                            .padding(.horizontal, MavioSpacing.s4)
                            .background(theme.colors.scrim, in: .rect(cornerRadius: MavioRadius.sm))
                            .padding(.horizontal, MavioSpacing.s8)
                    }
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            theme.colors.scrim
                            theme.colors.progress.frame(width: geo.size.width * CGFloat(min(progress, 1)))
                        }
                    }
                    .frame(height: 3)
                }
            }
        }
    }
}

/// The minutes left of a card begun, rounded up.
public func minutesLeft(_ card: Card) -> Int? {
    guard let runtime = card.runtime, let progress = card.progress else { return nil }
    return max(1, Int((runtime * Double(1 - progress) / 60).rounded(.up)))
}

/// A 2:3 card of a movie, series, season or collection.
public struct PosterCard: View {
    private let card: Card
    private let width: CGFloat?
    @Environment(\.theme) private var theme

    /// The card, layout.poster wide unless width says otherwise; nil fills
    /// the space offered, as in grids.
    public init(_ card: Card, width: CGFloat? = nil) {
        self.card = card
        self.width = width
    }

    /// The artwork with the title and subtitle below it.
    public var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            ArtworkView(card.poster?.url ?? card.thumb?.url, title: card.title)
                .aspectRatio(2 / 3, contentMode: .fit)
                .overlay { CardOverlays(card: card, timeLeft: false) }
                .clipShape(.rect(cornerRadius: MavioRadius.md))
            Text(card.title).type(theme.type.headline).lineLimit(1)
                .foregroundStyle(theme.colors.textPrimary)
            Text(subtitle(card) ?? " ").type(theme.type.footnote).lineLimit(1)
                .foregroundStyle(theme.colors.textSecondary)
        }
        .frame(width: width)
        .contentShape(.rect)
    }
}

/// A 16:9 card of an episode, a video or an item begun.
public struct ThumbCard: View {
    private let card: Card
    private let width: CGFloat?
    @Environment(\.theme) private var theme

    /// The card, width wide or filling the space offered.
    public init(_ card: Card, width: CGFloat? = nil) {
        self.card = card
        self.width = width
    }

    /// The artwork with the title and subtitle below it.
    public var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            ArtworkView(card.thumb?.url ?? card.poster?.url, title: card.title)
                .aspectRatio(16 / 9, contentMode: .fit)
                .overlay { CardOverlays(card: card, timeLeft: true) }
                .clipShape(.rect(cornerRadius: MavioRadius.md))
            Text(card.title).type(theme.type.headline).lineLimit(1)
                .foregroundStyle(theme.colors.textPrimary)
            Text(subtitle(card) ?? " ").type(theme.type.footnote).lineLimit(1)
                .foregroundStyle(theme.colors.textSecondary)
        }
        .frame(width: width)
        .contentShape(.rect)
    }
}

/// The subtitle line of a card: its own subtitle, else its year.
func subtitle(_ card: Card) -> String? {
    card.subtitle ?? card.year.map(String.init)
}

/// Whether a card shows as a 16:9 thumbnail rather than a poster.
public func isWide(_ card: Card) -> Bool {
    switch card.kind {
    case .episode, .video, .musicVideo, .photo: true
    default: false
    }
}

/// A square-ish placeholder of a card loading.
public struct CardPlaceholder: View {
    private let wide: Bool
    @Environment(\.theme) private var theme

    /// A poster placeholder, or a thumbnail one when wide.
    public init(wide: Bool = false) {
        self.wide = wide
    }

    /// The placeholder.
    public var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            RoundedRectangle(cornerRadius: MavioRadius.md)
                .fill(theme.colors.surfaceRaised)
                .aspectRatio(wide ? 16 / 9 : 2 / 3, contentMode: .fit)
            RoundedRectangle(cornerRadius: MavioRadius.sm)
                .fill(theme.colors.surfaceRaised)
                .frame(width: 80, height: 12)
        }
        .frame(width: wide ? theme.layout.thumbnail : theme.layout.poster)
    }
}

/// A cast or crew member.
public struct PersonCard: View {
    private let credit: CreditEntry
    @Environment(\.theme) private var theme

    /// The card of a credit.
    public init(_ credit: CreditEntry) {
        self.credit = credit
    }

    /// The photo with the name and role.
    public var body: some View {
        VStack(spacing: MavioSpacing.s4) {
            ZStack {
                theme.colors.surfaceRaised
                Text(initials(credit.name)).type(theme.type.title3).foregroundStyle(theme.colors.textSecondary)
                if let photo = credit.photo { ArtworkView(photo.url) }
            }
            .frame(width: 88, height: 88)
            .clipShape(Circle())
            Text(credit.name).type(theme.type.callout).lineLimit(2).multilineTextAlignment(.center)
                .foregroundStyle(theme.colors.textPrimary)
            if let role = credit.role {
                Text(role).type(theme.type.footnote).lineLimit(2).multilineTextAlignment(.center)
                    .foregroundStyle(theme.colors.textSecondary)
            }
        }
        .frame(width: 100, alignment: .top)
    }

    private func initials(_ name: String) -> String {
        name.split(separator: " ").prefix(2).compactMap(\.first).map(String.init).joined()
    }
}
