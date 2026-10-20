import MavioKit
import MavioUI
import SwiftUI

/// Search across every server, as the web UI's (docs/clients.md §5.4,
/// §6.3.28): a row of each server's results as it answers, in the order of
/// the servers, then the titles the server that discovers finds, to
/// request. A search runs when it is confirmed with Return.
public struct SearchView: View {
    @Environment(AppModel.self) private var model
    @Environment(Discovery.self) private var discovery
    @Environment(\.theme) private var theme
    @State private var query = ""
    /// The query searched, empty before the first search.
    @State private var searched = ""
    /// How many times a query has been confirmed: confirming the same one
    /// again searches again.
    @State private var submitted = 0
    /// The servers asked, and what each has answered.
    @State private var servers: [ServerRecord] = []
    @State private var results: [String: SearchResults] = [:]
    /// The titles found to discover, nil while they are being searched.
    @State private var titles: [DiscoveredTitle]?
    /// What every server, and what the server that discovers, has answered
    /// in full, which coming back from a result keeps rather than asks
    /// again.
    @State private var answered: SearchKey?
    @State private var titlesAnswered: DiscoverSearch?
    @FocusState private var focused: Bool

    /// Search across the servers.
    public init() {}

    /// The view's content.
    public var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                if !searched.isEmpty {
                    ForEach(servers, id: \.id) { server in
                        row(server)
                    }
                    if let server = discovery.server, let titles, !titles.isEmpty {
                        Shelf(MavioStrings.searchDiscover) {
                            ForEach(titles, id: \.key) {
                                DiscoveredTitleCard(serverId: server.id, title: $0, width: theme.layout.poster)
                            }
                        }
                    }
                    if waiting {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                            .padding(.top, MavioSpacing.s40)
                    } else if none {
                        StatusView(icon: "search", title: MavioStrings.searchNoResults)
                            .padding(.top, MavioSpacing.s40)
                    }
                }
            }
            .padding(.vertical, MavioSpacing.s8)
        }
        .navigationTitle(MavioStrings.navSearch)
        .searchable(
            text: $query, placement: .pageField, prompt: MavioStrings.searchPlaceholder
        )
        // Typing keeps the title in place, instead of the bar sliding up
        // over it and back.
        .searchPresentationToolbarBehavior(.avoidHidingContent)
        // Opened from the top bar's button before anything is searched, it
        // is ready to type into.
        .searchFocused($focused)
        .onAppear { focused = searched.isEmpty }
        .onSubmit(of: .search) {
            searched = query.trimmingCharacters(in: .whitespaces)
            submitted += 1
        }
        .task(id: searchKey) { await search(searchKey) }
        .task(id: discoverSearch) { await searchTitles(discoverSearch) }
    }

    /// A server's row once it has answered: a line when it cannot be
    /// reached, and nothing when it found nothing, so that no row shows
    /// while it searches only to vanish.
    @ViewBuilder private func row(_ server: ServerRecord) -> some View {
        let title = servers.count > 1 ? server.name : MavioStrings.searchServer
        if let found = results[server.id] {
            if let error = found.error {
                Label {
                    Text(MavioStrings.homeServerUnreachable(server: server.name))
                } icon: {
                    Icon("offline", size: 18)
                }
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.textTertiary)
                .help(error)
                .padding(.horizontal, theme.layout.margin)
                .padding(.vertical, MavioSpacing.s12)
            } else if !found.cards.isEmpty {
                Shelf(title) {
                    ForEach(found.cards, id: \.self) { CardLink(card: $0, width: theme.layout.poster) }
                }
            }
        }
    }

    /// Whether something is still searching and nothing has been drawn yet.
    private var waiting: Bool {
        let answering =
            servers.contains { results[$0.id] == nil } || !discovery.loaded
            || (discovery.server != nil && titles == nil)
        let drawn =
            servers.contains { results[$0.id].map { $0.error != nil || !$0.cards.isEmpty } ?? false }
            || titles?.isEmpty == false
        return answering && !drawn
    }

    /// Whether everything has answered and found nothing.
    private var none: Bool {
        servers.allSatisfy { results[$0.id]?.cards.isEmpty ?? false } && discovery.loaded && titles?.isEmpty == true
    }

    private var searchKey: SearchKey { SearchKey(query: searched, submitted: submitted) }

    private var discoverSearch: DiscoverSearch {
        DiscoverSearch(search: searchKey, server: discovery.server?.id, loaded: discovery.loaded)
    }

    /// Asks every signed-in server at once, drawing each as it answers.
    private func search(_ key: SearchKey) async {
        guard key != answered else { return }
        let q = key.query
        servers = q.isEmpty ? [] : model.signedInServers
        results = [:]
        guard !q.isEmpty else { return }
        await withTaskGroup(of: SearchResults?.self) { group in
            for server in servers {
                group.addTask { [client = model.client] in
                    try? await client.searchServer(serverId: server.id, query: q)
                }
            }
            for await found in group {
                guard let found, !Task.isCancelled else { continue }
                results[found.serverId] = found
            }
        }
        if !Task.isCancelled { answered = key }
    }

    /// Asks the server that discovers for titles of the query.
    private func searchTitles(_ key: DiscoverSearch) async {
        guard key != titlesAnswered else { return }
        let q = key.search.query
        titles = nil
        guard !q.isEmpty, discovery.loaded else { return }
        guard let server = discovery.server else {
            titles = []
            return
        }
        var request = Mavio_Acquisition_V1_SearchTitlesRequest()
        request.query = q
        request.language = discoverLanguage
        let response = try? await model.call(
            server, Procedure.discover("SearchTitles"), request, as: Mavio_Acquisition_V1_SearchTitlesResponse.self)
        guard !Task.isCancelled else { return }
        var seen = Set<String>()
        titles = (response?.titles ?? []).filter { seen.insert($0.key).inserted }
        titlesAnswered = key
    }
}

/// A confirmed query: what has been answered, which coming back from a
/// result does not ask again but confirming it again does.
private struct SearchKey: Equatable {
    let query: String
    let submitted: Int
}

/// What the titles to discover are searched by.
private struct DiscoverSearch: Equatable {
    let search: SearchKey
    let server: String?
    let loaded: Bool
}
