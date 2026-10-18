import MavioKit
import MavioUI
import SwiftUI

/// Home: the rows of every server, merged as Plex does (docs/clients.md
/// §6.3.1), each server's drawn as it arrives in its fixed place.
struct HomeView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var hero: String?

    var body: some View {
        let entries = HeroEntry.from(model.home)
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                if !entries.isEmpty {
                    HeroView(entries: entries, shown: $hero)
                        .padding(.bottom, MavioSpacing.s8)
                }
                ForEach(model.home, id: \.id) { row in
                    RowView(row: row, several: model.signedIn.count > 1)
                }
                let signedOut = model.servers.filter { !model.signedIn.contains($0.id) }
                if !signedOut.isEmpty {
                    Text(signedOut.map { "\($0.name): \(MavioStrings.authSignedOut)" }.joined(separator: " · "))
                        .type(theme.type.footnote)
                        .foregroundStyle(theme.colors.textTertiary)
                        .padding(.horizontal, theme.layout.margin)
                        .padding(.top, MavioSpacing.s24)
                }
            }
            .padding(.bottom, MavioSpacing.s8)
        }
        .modifier(HeroEdge(hero: !entries.isEmpty))
        .overlay {
            if model.home.isEmpty { ProgressView() }
        }
        .refreshable { model.refreshHome() }
        .navigationTitle(MavioStrings.navHome)
        .task { if model.home.isEmpty { model.refreshHome() } }
    }
}

/// Home under the status bar when the hero leads it, the bar clear over
/// the hero with its title appearing past it, as details are.
private struct HeroEdge: ViewModifier {
    let hero: Bool
    @Environment(\.theme) private var theme
    @State private var past = false

    func body(content: Content) -> some View {
        if hero {
            content
                .ignoresSafeArea(edges: .top)
                .onScrollGeometryChange(for: Bool.self) { geo in
                    geo.contentOffset.y + geo.contentInsets.top > geo.containerSize.height * 0.5
                } action: { _, now in
                    withAnimation(.easeOut(duration: MavioDuration.fast)) { past = now }
                }
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .principal) {
                        Text(MavioStrings.navHome).type(theme.type.headline).opacity(past ? 1 : 0)
                    }
                }
                .toolbarBackground(past ? .visible : .hidden, for: .navigationBar)
                .modifier(EdgeEffect(hidden: !past))
                .toolbarColorScheme(past ? nil : .dark, for: .navigationBar)
        } else {
            content
        }
    }
}

/// The scroll edge's effect, hidden over the hero.
private struct EdgeEffect: ViewModifier {
    let hidden: Bool

    func body(content: Content) -> some View {
        if #available(iOS 26, *) {
            content.scrollEdgeEffectHidden(hidden, for: .top)
        } else {
            content
        }
    }
}
