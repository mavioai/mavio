import MavioKit
import MavioUI
import SwiftUI

/// Home (docs/tv.md §6.1): a hero 78 % of the screen's height behind the
/// navigation bar, then the rows of every server.
struct HomeView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        let rows = model.home
        let several = Set(rows.map(\.serverId)).count > 1
        GeometryReader { geo in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: MavioSpacing.s48) {
                    if let card = hero(rows) {
                        Hero(card: card).frame(height: geo.size.height * 0.78)
                    } else {
                        Color.clear.frame(height: navBarHeight)
                    }
                    ForEach(rows, id: \.id) { row in
                        RowView(row: row, several: several)
                    }
                }
                .padding(.bottom, 120)
            }
            .reportsScroll()
            .overlay {
                if rows.isEmpty {
                    StatusView(icon: "home", title: MavioStrings.stateEmpty)
                }
            }
        }
        .ignoresSafeArea()
        .toolbar(.hidden, for: .navigationBar)
    }

    /// The card the hero shows: the first begun or recently added with a
    /// backdrop.
    private func hero(_ rows: [Row]) -> Card? {
        for row in rows {
            if case let .items(cards) = row.content, let card = cards.first(where: { $0.backdrop != nil }) {
                return card
            }
        }
        return nil
    }
}

/// The backdrop of a title with its logo or name at 72 pt, two lines of its
/// overview and its actions, 64 pt high.
private struct Hero: View {
    let card: Card
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            ArtworkView(card.backdrop?.url, title: card.title, icon: card.kind.icon)
            LinearGradient(
                stops: [.init(color: .black.opacity(0.6), location: 0), .init(color: .clear, location: 0.6)],
                startPoint: .leading, endPoint: .trailing)
            LinearGradient(
                stops: [
                    .init(color: theme.colors.tvBackground, location: 0),
                    .init(color: theme.colors.tvBackground.opacity(0.6), location: 0.3),
                    .init(color: .clear, location: 0.7),
                ], startPoint: .bottom, endPoint: .top)
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                TitleMark(name: card.title, logo: card.logo?.url)
                if let overview = card.overview {
                    Text(overview).type(theme.type.body).lineLimit(2)
                        .foregroundStyle(.white.opacity(0.85))
                        .frame(maxWidth: 960, alignment: .leading)
                }
                HStack(spacing: MavioSpacing.s24) {
                    Button {
                        router.play(PlayRequest(serverId: card.serverId, itemId: card.itemId, title: card.title))
                    } label: {
                        Label {
                            Text((card.progress ?? 0) > 0 ? MavioStrings.itemResume : MavioStrings.itemPlay)
                        } icon: {
                            Icon("play", fill: true, size: 28)
                        }
                    }
                    NavigationLink(value: Route.item(serverId: card.serverId, itemId: card.itemId)) {
                        Label {
                            Text(MavioStrings.itemDetails)
                        } icon: {
                            Icon("info", size: 28)
                        }
                    }
                }
                .buttonStyle(TVButtonStyle())
                .focusSection()
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, MavioSpacing.s48)
        }
        .clipped()
    }
}

/// A title's logo, else its name, 72 pt high.
struct TitleMark: View {
    let name: String
    let logo: String?
    @Environment(\.theme) private var theme

    var body: some View {
        Group {
            if let logo {
                FittedImage(logo, alignment: .bottomLeading) { nameText }
                    .frame(maxWidth: 640, maxHeight: 160, alignment: .bottomLeading)
            } else {
                nameText
            }
        }
        .accessibilityElement()
        .accessibilityLabel(name)
    }

    private var nameText: some View {
        Text(name).font(.system(size: 72, weight: .bold)).lineLimit(2)
            .foregroundStyle(.white)
            .frame(maxWidth: 1200, alignment: .leading)
    }
}
