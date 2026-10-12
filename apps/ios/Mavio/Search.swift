import MavioKit
import MavioUI
import SwiftUI

/// Search across every server, the results by server.
struct SearchView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var query = ""
    @State private var results: [SearchResults]?

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: MavioSpacing.s16) {
                if let results {
                    ForEach(results, id: \.serverId) { group in
                        if !group.cards.isEmpty || group.error != nil {
                            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                                if results.count > 1 {
                                    Text(group.serverName).type(theme.type.title3)
                                        .padding(.horizontal, theme.layout.margin)
                                }
                                if group.error != nil {
                                    Text(MavioStrings.homeServerUnreachable(server: group.serverName))
                                        .foregroundStyle(theme.colors.textTertiary)
                                        .padding(.horizontal, theme.layout.margin)
                                }
                                CardGrid(items: group.cards) { CardLink(card: $0) }
                            }
                        }
                    }
                    if results.allSatisfy({ $0.cards.isEmpty }) {
                        StatusView(title: MavioStrings.searchNoResults).padding(.top, MavioSpacing.s40)
                    }
                }
            }
            .padding(.vertical, MavioSpacing.s8)
        }
        .navigationTitle(MavioStrings.navSearch)
        .searchable(
            text: $query, placement: .navigationBarDrawer(displayMode: .always), prompt: MavioStrings.searchPlaceholder
        )
        .task(id: query) {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty else {
                results = nil
                return
            }
            try? await Task.sleep(for: .milliseconds(300))
            guard !Task.isCancelled else { return }
            if let found = try? await model.client.search(query: q), !Task.isCancelled {
                results = found
            }
        }
    }
}
