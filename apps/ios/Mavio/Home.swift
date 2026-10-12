import MavioKit
import MavioUI
import SwiftUI

/// Home: the rows of every server, merged as Plex does (docs/clients.md
/// §6.3.1), each server's drawn as it arrives in its fixed place.
struct HomeView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
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
            .padding(.vertical, MavioSpacing.s8)
        }
        .overlay {
            if model.home.isEmpty { ProgressView() }
        }
        .refreshable { model.refreshHome() }
        .navigationTitle(MavioStrings.navHome)
        .task { if model.home.isEmpty { model.refreshHome() } }
    }
}
