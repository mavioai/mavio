import MavioKit
import MavioUI
import SwiftUI

/// A title Home's hero shows, and the line above it: the row it is from.
struct HeroEntry: Identifiable {
    let card: Card
    let eyebrow: String
    var id: String { "\(card.serverId)/\(card.itemId)" }

    private static let kinds: Set<CardKind> = [.movie, .series, .video, .musicVideo]

    /// Up to five titles with backdrops from Home's rows, Continue Watching
    /// aside, as the web UI's.
    static func from(_ rows: [Row]) -> [HeroEntry] {
        var out: [HeroEntry] = []
        var seen: Set<String> = []
        for row in rows where row.kind != .continueWatching {
            guard case let .items(cards) = row.content else { continue }
            for card in cards where kinds.contains(card.kind) && card.backdrop != nil {
                let entry = HeroEntry(card: card, eyebrow: rowTitle(row))
                guard seen.insert(entry.id).inserted else { continue }
                out.append(entry)
                if out.count == 5 { return out }
            }
        }
        return out
    }
}

/// The hero at the top of Home (docs/clients.md §6.3): the latest films and
/// shows over their backdrops, one after another, each with its facts and
/// the buttons to play it and open its details; swiped through, or with the
/// dots.
struct HeroView: View {
    let entries: [HeroEntry]
    /// The title shown, kept by Home so that Back finds the title it left for.
    @Binding var shown: String?
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var dragging = false

    /// How long each title shows before the next.
    private static let interval = Duration.seconds(9)

    private var phone: Bool { sizeClass != .regular }
    private var index: Int { entries.firstIndex { $0.id == shown } ?? 0 }

    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: 0) {
                ForEach(entries) { entry in
                    HeroPage(entry: entry, phone: phone)
                        .containerRelativeFrame(.horizontal)
                }
            }
            .scrollTargetLayout()
        }
        .scrollTargetBehavior(.paging)
        .scrollIndicators(.hidden)
        .scrollPosition(id: $shown)
        // A phone's pages draw their colors above the hero, behind the bar.
        .scrollClipDisabled(phone)
        .onScrollPhaseChange { _, phase in dragging = phase == .interacting }
        .modifier(HeroSize(phone: phone))
        .overlay(alignment: phone ? .topTrailing : .bottomTrailing) {
            if entries.count > 1 { dots }
        }
        .background(theme.colors.surface)
        .environment(\.colorScheme, .dark)
        .accessibilityElement(children: .contain)
        .task(id: "\(shown ?? "")\(dragging)") {
            // The next title after a while, unless the user is swiping.
            guard entries.count > 1, !dragging else { return }
            try? await Task.sleep(for: Self.interval)
            guard !Task.isCancelled else { return }
            withAnimation(.easeInOut(duration: MavioDuration.slow)) {
                shown = entries[(index + 1) % entries.count].id
            }
        }
    }

    private var dots: some View {
        HStack(spacing: 6) {
            ForEach(entries.indices, id: \.self) { i in
                Button {
                    withAnimation { shown = entries[i].id }
                } label: {
                    Capsule()
                        .fill(i == index ? .white : .white.opacity(0.45))
                        .frame(width: i == index ? 20 : 6, height: 6)
                        .padding(.vertical, 8)
                        .contentShape(.rect)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(entries[i].card.title)
                .accessibilityAddTraits(i == index ? .isSelected : [])
            }
        }
        .animation(.easeOut(duration: MavioDuration.fast), value: index)
        .padding(.horizontal, phone ? theme.layout.margin : MavioSpacing.s32)
        // On a phone the dots sit at the top.
        .padding(.top, phone ? MavioSpacing.s16 : 0)
        .padding(.bottom, phone ? 0 : MavioSpacing.s24)
    }
}

/// The hero's size: on a phone 4:5, by the screen's width, else 470 pt
/// high.
private struct HeroSize: ViewModifier {
    let phone: Bool

    func body(content: Content) -> some View {
        if phone {
            content.frame(maxWidth: .infinity).aspectRatio(4 / 5, contentMode: .fit)
        } else {
            content.frame(height: 470)
        }
    }
}

/// One title of the hero.
private struct HeroPage: View {
    let entry: HeroEntry
    let phone: Bool
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme

    private var card: Card { entry.card }

    var body: some View {
        Color.clear
            .background {
                ArtworkView(card.backdrop?.url, icon: card.kind.icon)
            }
            .overlay {
                if phone {
                    LinearGradient(
                        stops: [
                            .init(color: .black.opacity(0.9), location: 0),
                            .init(color: .black.opacity(0.4), location: 0.5),
                            .init(color: .clear, location: 1),
                        ],
                        startPoint: .bottom, endPoint: .top)
                } else {
                    LinearGradient(
                        stops: [
                            .init(color: .black.opacity(0.78), location: 0),
                            .init(color: .black.opacity(0.35), location: 0.45),
                            .init(color: .clear, location: 0.75),
                        ],
                        startPoint: .leading, endPoint: .trailing)
                    LinearGradient(
                        stops: [.init(color: .black.opacity(0.55), location: 0), .init(color: .clear, location: 0.45)],
                        startPoint: .bottom, endPoint: .top)
                }
            }
            .overlay(alignment: .bottomLeading) {
                text
                    .frame(maxWidth: phone ? .infinity : 560, alignment: .leading)
                    .padding(.horizontal, phone ? theme.layout.margin : MavioSpacing.s40)
                    .padding(.bottom, phone ? MavioSpacing.s20 : MavioSpacing.s40)
            }
            .clipped()
            .heroBleed(card.backdrop?.url, phone: phone)
    }

    private var text: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            Text(entry.eyebrow.uppercased())
                .type(theme.type.footnote).fontWeight(.semibold).tracking(0.6)
                .foregroundStyle(.white.opacity(0.75))
            if let logo = card.logo {
                FittedImage(logo.url, alignment: .leading) { title }
                    .frame(maxWidth: phone ? 220 : 280, maxHeight: phone ? 64 : 96, alignment: .leading)
                    .accessibilityLabel(card.title)
            } else {
                title
            }
            HStack(spacing: MavioSpacing.s8) {
                if !facts.isEmpty {
                    Text(facts.joined(separator: " · ")).lineLimit(1)
                }
                if let rating = card.officialRating {
                    Text(rating)
                        .type(theme.type.caption)
                        .foregroundStyle(.white.opacity(0.9))
                        .padding(.horizontal, 6)
                        .overlay(RoundedRectangle(cornerRadius: 4).strokeBorder(.white.opacity(0.5)))
                }
            }
            .type(theme.type.callout)
            .foregroundStyle(.white.opacity(0.8))
            if !phone, let overview = card.overview {
                Text(overview).type(theme.type.body).lineLimit(3).foregroundStyle(.white.opacity(0.85))
            }
            HStack(spacing: MavioSpacing.s12) {
                if card.kind != .series {
                    Button {
                        let start = both(card.progress, card.runtime).map { TimeInterval($0) * $1 }
                        router.play(
                            PlayRequest(serverId: card.serverId, itemId: card.itemId, title: card.title, start: start))
                    } label: {
                        Label {
                            Text(MavioStrings.playerPlay)
                        } icon: {
                            Icon("play", fill: true, size: 20)
                        }
                        .frame(maxWidth: phone ? .infinity : nil)
                    }
                    .buttonStyle(MavioButtonStyle(.artworkPrimary))
                }
                NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
                    Label {
                        Text(MavioStrings.itemDetails)
                    } icon: {
                        Icon("info", size: 20)
                    }
                    .frame(maxWidth: phone ? .infinity : nil)
                }
                .buttonStyle(MavioButtonStyle(.artworkSecondary))
            }
            .padding(.top, MavioSpacing.s8)
        }
        .foregroundStyle(.white)
    }

    private var title: some View {
        Text(card.title)
            .font(.system(size: phone ? 28 : 52, weight: .bold))
            .lineLimit(2)
            .foregroundStyle(.white)
    }

    /// The year, the runtime and two genres.
    private var facts: [String] {
        [card.year.map(String.init), card.runtime.map(minutesText)].compactMap(\.self) + card.genres.prefix(2)
    }
}

/// Both values, when there are both.
private func both<A, B>(_ a: A?, _ b: B?) -> (A, B)? {
    guard let a, let b else { return nil }
    return (a, b)
}

/// How far a phone's hero's colors reach up behind the navigation bar and
/// the status bar, more than they take on any phone.
private let bleedHeight: CGFloat = 400

extension View {
    /// On a phone, the hero's artwork, enlarged and blurred, above it behind
    /// the navigation bar, so that its colors spread up into the bar's glass
    /// and meet the hero at a sharp edge, as the web UI's.
    @ViewBuilder func heroBleed(_ url: String?, phone: Bool) -> some View {
        if phone, let url {
            background(alignment: .top) {
                // The artwork fills the bleed's frame rather than sizing it.
                Color.clear
                    .frame(maxWidth: .infinity)
                    .frame(height: bleedHeight)
                    .overlay {
                        ArtworkView(url)
                            .scaleEffect(1.25)
                            .blur(radius: 40, opaque: true)
                    }
                    .clipped()
                    .offset(y: -bleedHeight)
                    .allowsHitTesting(false)
                    .accessibilityHidden(true)
            }
        } else {
            self
        }
    }
}

/// A screen led by a hero. On a phone the hero sits below the navigation
/// bar, which is of the tab bar's glass over the hero's colors and what
/// scrolls under it, meeting them at a sharp edge; on an iPad the hero
/// reaches the top under a clear bar. The bar's title appears once the hero
/// is scrolled past.
struct HeroChrome: ViewModifier {
    let title: String
    let phone: Bool
    @Environment(\.theme) private var theme
    @State private var past = false

    func body(content: Content) -> some View {
        content
            .ignoresSafeArea(edges: phone ? [] : .top)
            .onScrollGeometryChange(for: Bool.self) { geo in
                geo.contentOffset.y + geo.contentInsets.top > geo.containerSize.height * 0.5
            } action: { _, now in
                withAnimation(.easeOut(duration: MavioDuration.fast)) { past = now }
            }
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    Text(title).type(theme.type.headline).lineLimit(1).opacity(past ? 1 : 0)
                }
            }
            .modifier(HeroBarGlass(phone: phone, past: past))
            .toolbarColorScheme(past ? nil : .dark, for: .navigationBar)
    }
}

/// The navigation bar's glass: on a phone always, with a hard edge, where
/// iOS draws its scroll edge effect, else the bar's material; on an iPad
/// none over the hero.
private struct HeroBarGlass: ViewModifier {
    let phone: Bool
    let past: Bool

    func body(content: Content) -> some View {
        if #available(iOS 26, *) {
            if phone {
                content
                    .toolbarBackground(.hidden, for: .navigationBar)
                    .scrollEdgeEffectStyle(.hard, for: .top)
            } else {
                content
                    .toolbarBackground(past ? .visible : .hidden, for: .navigationBar)
                    .scrollEdgeEffectHidden(!past, for: .top)
            }
        } else {
            content.toolbarBackground(phone || past ? .visible : .hidden, for: .navigationBar)
        }
    }
}
