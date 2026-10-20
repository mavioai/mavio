import MavioDesign
import MavioKit
import NukeUI
import SwiftUI

/// An image loaded from a server, over a placeholder of its final size: a
/// plain fill while it loads, into which it fades, and the icon of what it
/// shows only when there is none, so that no icon flashes before an image.
public struct ArtworkView: View {
    private let url: String?
    private let title: String?
    private let icon: String
    @Environment(\.theme) private var theme

    /// The image at url, which shows title; without one, icon on the
    /// placeholder.
    public init(_ url: String?, title: String? = nil, icon: String = "movies") {
        self.url = url
        self.title = title
        self.icon = icon
    }

    /// The image, or its placeholder.
    public var body: some View {
        // The fill takes the size offered; an image of other proportions,
        // filling it, cannot grow it.
        // The fill of controls, which shows on the background of either
        // theme, as the raised surface does not on the light one's.
        theme.colors.fill.overlay {
            if let url, let parsed = URL(string: url) {
                LazyImage(url: parsed, transaction: Transaction(animation: .easeOut(duration: MavioDuration.slow))) {
                    state in
                    if let image = state.image {
                        image.resizable().scaledToFill()
                    } else if state.error != nil {
                        placeholder
                    }
                }
            } else {
                placeholder
            }
        }
        .clipped()
        .accessibilityElement()
        .accessibilityLabel(title ?? "")
    }

    /// The icon over a fill fading from the top leading corner, as the web
    /// UI's `art-placeholder`.
    private var placeholder: some View {
        GeometryReader { geo in
            Icon(icon, size: min(geo.size.width * 0.26, 56))
                .foregroundStyle(theme.colors.textTertiary)
                .opacity(0.6)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(
                    LinearGradient(
                        stops: [.init(color: theme.colors.fill, location: 0), .init(color: .clear, location: 0.7)],
                        startPoint: UnitPoint(x: 0.33, y: 0), endPoint: UnitPoint(x: 0.67, y: 1)))
        }
    }
}

/// An image shown whole at its own proportions, such as a logo; fallback
/// while it loads and when it cannot.
public struct FittedImage<Fallback: View>: View {
    private let url: String
    private let alignment: Alignment
    private let fallback: Fallback

    /// The image at url, at alignment in the space it is given, else fallback.
    public init(_ url: String, alignment: Alignment = .center, @ViewBuilder fallback: () -> Fallback) {
        self.url = url
        self.alignment = alignment
        self.fallback = fallback()
    }

    /// The image.
    public var body: some View {
        LazyImage(url: URL(string: url), transaction: Transaction(animation: .easeOut(duration: MavioDuration.slow))) {
            state in
            if let image = state.image {
                image.resizable().scaledToFit()
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: alignment)
            } else if state.error != nil {
                fallback.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: alignment)
            } else {
                Color.clear
            }
        }
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
                Icon("played", fill: true, size: 18)
                    .foregroundStyle(.white)
                    .padding(MavioSpacing.s2)
                    .background(Circle().fill(theme.colors.scrim))
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
            ArtworkView(card.poster?.url ?? card.thumb?.url, title: card.title, icon: card.kind.icon)
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
            ArtworkView(card.thumb?.url ?? card.poster?.url, title: card.title, icon: card.kind.icon)
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

/// A card loading, as the web UI's: its artwork's shape and a bar where
/// its title will be, both of the fill artwork loads over, and as
/// tall as the card, so that nothing moves when it arrives.
public struct CardPlaceholder: View {
    private let wide: Bool
    @Environment(\.theme) private var theme

    /// A poster placeholder, or a thumbnail one when wide.
    public init(wide: Bool = false) {
        self.wide = wide
    }

    /// The placeholder.
    public var body: some View {
        let width = wide ? theme.layout.thumbnail : theme.layout.poster
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            RoundedRectangle(cornerRadius: MavioRadius.md)
                .fill(theme.colors.fill)
                .aspectRatio(wide ? 16 / 9 : 2 / 3, contentMode: .fit)
            // The title's and the subtitle's lines, the title's a bar.
            Text(" ").type(theme.type.headline)
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .leading) {
                    RoundedRectangle(cornerRadius: MavioRadius.sm)
                        .fill(theme.colors.fill)
                        .frame(width: width * 0.75, height: 14)
                }
            Text(" ").type(theme.type.footnote)
        }
        .frame(width: width)
        .accessibilityHidden(true)
    }
}

/// A cast or crew member: the photo in a circle, the name and the role
/// (libs/design/components.md, Person Card).
public struct PersonCard: View {
    private let name: String
    private let photo: String?
    private let role: String?
    @Environment(\.theme) private var theme

    /// The card of a credit, with its role: the character played or the job.
    public init(_ credit: CreditEntry, role: String?) {
        name = credit.name
        photo = credit.photo?.url
        self.role = role
    }

    /// The card of a person by their name, photo and role.
    public init(name: String, photo: String?, role: String?) {
        self.name = name
        self.photo = photo
        self.role = role
    }

    /// The photo with the name and role.
    public var body: some View {
        VStack(spacing: MavioSpacing.s8) {
            ArtworkView(photo, title: name, icon: "profile")
                .frame(width: 96, height: 96)
                .clipShape(Circle())
            VStack(spacing: 0) {
                Text(name).type(theme.type.callout).lineLimit(2)
                    .foregroundStyle(theme.colors.textPrimary)
                if let role {
                    Text(role).type(theme.type.footnote).lineLimit(2)
                        .foregroundStyle(theme.colors.textSecondary)
                }
            }
            .multilineTextAlignment(.center)
        }
        .frame(width: 112, alignment: .top)
    }
}
