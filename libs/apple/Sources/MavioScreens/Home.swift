import MavioKit
import MavioUI
import SwiftUI

/// Home: the rows of every server, merged as Plex does (docs/clients.md
/// §6.3.1), each server's drawn as it arrives in its fixed place.
public struct HomeView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var hero: String?

    /// Home of every server signed in to.
    public init() {}

    /// The view's content.
    public var body: some View {
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
        .pullToRefresh { await model.reloadHome() }
        .modifier(HomeChrome(hero: !entries.isEmpty, loading: model.home.isEmpty))
        .overlay {
            if model.home.isEmpty { ProgressView() }
        }
        .navigationTitle(MavioStrings.navHome)
        .task { if model.home.isEmpty { model.refreshHome() } }
    }
}

/// Home's bar as a hero's when the hero leads it (HeroChrome), and without
/// its title while loading, which would show only until the hero hides it.
private struct HomeChrome: ViewModifier {
    let hero: Bool
    let loading: Bool
    @Environment(\.horizontalSizeClass) private var sizeClass

    func body(content: Content) -> some View {
        if hero {
            content.modifier(HeroChrome(title: MavioStrings.navHome, phone: sizeClass != .regular))
        } else if loading {
            content.inlineTitle().toolbar(removing: .title)
        } else {
            content
        }
    }
}
