import MavioKit
import MavioUI
import SwiftUI

/// Search across the servers (docs/tv.md §6.4) with the system's keyboard:
/// the results by server in grids, the search running as the query is
/// typed, after a pause of 300 ms.
struct SearchView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var query = ""
    @State private var results: [SearchResults]?

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: MavioSpacing.s48) {
                if let results {
                    if results.allSatisfy(\.cards.isEmpty) {
                        StatusView(icon: "search", title: MavioStrings.searchNoResults)
                    }
                    ForEach(results.filter { !$0.cards.isEmpty }, id: \.serverId) { result in
                        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                            if results.count > 1 {
                                Text(result.serverName).type(theme.type.title3)
                                    .foregroundStyle(theme.colors.textPrimary)
                            }
                            LazyVGrid(
                                columns: [
                                    GridItem(.adaptive(minimum: theme.layout.poster), spacing: theme.layout.gutter)
                                ],
                                spacing: MavioSpacing.s48
                            ) {
                                ForEach(result.cards, id: \.self) { CardLink(card: $0, wide: false) }
                            }
                        }
                    }
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, 120)
        }
        .searchable(text: $query, prompt: MavioStrings.searchPlaceholder)
        .task(id: query) {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty else {
                results = nil
                return
            }
            try? await Task.sleep(for: .milliseconds(300))
            guard !Task.isCancelled, let found = try? await model.client.search(query: q), !Task.isCancelled else {
                return
            }
            results = found
        }
    }
}
