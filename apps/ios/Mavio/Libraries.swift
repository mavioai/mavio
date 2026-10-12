import MavioKit
import MavioUI
import SwiftUI

/// The libraries of every server the user is signed in to, by server.
struct LibrariesView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var libraries: [String: Libraries] = [:]
    @State private var failed: [String: String] = [:]

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: MavioSpacing.s24) {
                ForEach(model.signedInServers, id: \.id) { server in
                    VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                        if model.signedIn.count > 1 {
                            Text(server.name).type(theme.type.title3).padding(.horizontal, theme.layout.margin)
                        }
                        if let entries = libraries[server.id]?.libraries {
                            CardGrid(items: entries, wide: true) { library in
                                NavigationLink(value: Route.library(library)) {
                                    LibraryCard(library: library)
                                }
                                .buttonStyle(.plain)
                            }
                        } else if let message = failed[server.id] {
                            Label(MavioStrings.homeServerUnreachable(server: server.name), systemImage: "wifi.slash")
                                .help(message)
                                .foregroundStyle(theme.colors.textTertiary)
                                .padding(.horizontal, theme.layout.margin)
                        } else {
                            ProgressView().frame(maxWidth: .infinity)
                        }
                    }
                }
            }
            .padding(.vertical, MavioSpacing.s16)
        }
        .navigationTitle(MavioStrings.navLibraries)
        .refreshable { await load() }
        .task(id: model.signedInServers.map { "\($0.id):\(model.revision($0.id))" }) { await load() }
    }

    private func load() async {
        await withTaskGroup(of: (String, Result<Libraries, Error>).self) { group in
            for server in model.signedInServers {
                let client = model.client
                group.addTask {
                    do {
                        return (server.id, .success(try await client.libraries(serverId: server.id)))
                    } catch {
                        return (server.id, .failure(error))
                    }
                }
            }
            for await (id, result) in group {
                switch result {
                case let .success(value):
                    libraries[id] = value
                    failed[id] = nil
                case let .failure(error):
                    if libraries[id] == nil { failed[id] = error.localizedDescription }
                }
            }
        }
    }
}

/// A library's tile: its image with its name.
struct LibraryCard: View {
    let library: LibraryEntry
    @Environment(\.theme) private var theme

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            ArtworkView(library.imageUrl.isEmpty ? nil : library.imageUrl)
                .aspectRatio(16 / 9, contentMode: .fit)
            LinearGradient(colors: [.clear, theme.colors.scrim], startPoint: .center, endPoint: .bottom)
            Text(library.name)
                .type(theme.type.headline)
                .foregroundStyle(.white)
                .padding(MavioSpacing.s12)
        }
        .clipShape(.rect(cornerRadius: MavioRadius.md))
    }
}

/// A library: its Recommended tab, its hubs, beside the grid of all its items.
struct LibraryView: View {
    let library: LibraryEntry
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var tab = 0
    @State private var rows: [Row]?
    @State private var page = Paged()
    @State private var sort: Sort = .name
    @State private var filter = Filter(played: nil, favorite: false, genre: nil)

    var body: some View {
        ScrollView {
            Picker("", selection: $tab) {
                Text(MavioStrings.libraryRecommended).tag(0)
                Text(MavioStrings.libraryAll).tag(1)
            }
            .pickerStyle(.segmented)
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, MavioSpacing.s8)
            if tab == 0 {
                LazyVStack(alignment: .leading, spacing: 0) {
                    if let rows {
                        ForEach(rows, id: \.id) { RowView(row: $0, several: false) }
                        if rows.isEmpty { StatusView(title: MavioStrings.stateEmpty) }
                    } else {
                        ProgressView().padding(MavioSpacing.s40)
                    }
                }
            } else {
                if let total = page.total {
                    Text(MavioStrings.libraryItemCount(count: Int(total)))
                        .type(theme.type.footnote)
                        .foregroundStyle(theme.colors.textSecondary)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, theme.layout.margin)
                }
                PagedGrid(page: $page) { offset, limit in
                    try await model.client.libraryItems(
                        serverId: library.serverId, libraryId: library.id, sort: sort, filter: filter, offset: offset,
                        limit: limit)
                }
            }
        }
        .navigationTitle(library.name)
        .toolbar {
            if tab == 1 {
                ToolbarItemGroup {
                    sortMenu
                    filterMenu
                }
            }
        }
        .task(id: model.revision(library.serverId)) {
            rows = try? await model.client.recommended(serverId: library.serverId, libraryId: library.id)
            if rows?.isEmpty == true { tab = 1 }
        }
        .onChange(of: model.revision(library.serverId)) { page.reset() }
        .onChange(of: sort) { page.reset() }
        .onChange(of: filter) { page.reset() }
    }

    private var sortMenu: some View {
        Menu {
            Picker(MavioStrings.librarySort, selection: $sort) {
                Text(MavioStrings.librarySortName).tag(Sort.name)
                Text(MavioStrings.librarySortDateAdded).tag(Sort.dateAdded)
                Text(MavioStrings.librarySortReleased).tag(Sort.released)
                Text(MavioStrings.librarySortRating).tag(Sort.rating)
                Text(MavioStrings.librarySortLastPlayed).tag(Sort.lastPlayed)
                Text(MavioStrings.librarySortRandom).tag(Sort.random)
            }
        } label: {
            Label {
                Text(MavioStrings.librarySort)
            } icon: {
                Icon("sort")
            }
        }
    }

    private var filterMenu: some View {
        Menu {
            Picker(MavioStrings.libraryFilter, selection: $filter) {
                Text(MavioStrings.libraryFilterAll).tag(Filter(played: nil, favorite: false, genre: nil))
                Text(MavioStrings.libraryFilterUnplayed).tag(Filter(played: false, favorite: false, genre: nil))
                Text(MavioStrings.libraryFilterPlayed).tag(Filter(played: true, favorite: false, genre: nil))
                Text(MavioStrings.libraryFilterFavorites).tag(Filter(played: nil, favorite: true, genre: nil))
            }
        } label: {
            Label {
                Text(MavioStrings.libraryFilter)
            } icon: {
                Icon("filter", fill: filter != Filter(played: nil, favorite: false, genre: nil))
            }
        }
    }
}

/// Every item of a hub.
struct HubView: View {
    let serverId: String
    let hubId: String
    let title: String
    @Environment(AppModel.self) private var model
    @State private var page = Paged()

    var body: some View {
        ScrollView {
            PagedGrid(page: $page) { offset, limit in
                try await model.client.hub(serverId: serverId, hubId: hubId, offset: offset, limit: limit)
            }
        }
        .navigationTitle(title)
        .onChange(of: model.revision(serverId)) { page.reset() }
    }
}

/// The pages of a grid loaded so far.
struct Paged: Equatable {
    var cards: [Card] = []
    var total: UInt32?
    var more = true
    var loading = false
    var failed = false
    /// Changes when the grid starts again, which drops pages in flight.
    var generation = 0

    mutating func reset() {
        self = Paged(generation: generation + 1)
    }
}

/// A grid loading its pages as it scrolls.
struct PagedGrid: View {
    @Binding var page: Paged
    let load: (UInt32, UInt32) async throws -> Page
    @Environment(\.theme) private var theme

    private let limit: UInt32 = 60

    var body: some View {
        CardGrid(items: page.cards, wide: !page.cards.isEmpty && page.cards.allSatisfy(isWide)) { card in
            CardLink(card: card)
                .onAppear { if card == page.cards.last { Task { await next() } } }
        }
        if page.loading || (page.more && page.cards.isEmpty && !page.failed) {
            ProgressView()
                .padding(MavioSpacing.s24)
                .task(id: page.generation) { if page.cards.isEmpty { await next() } }
        } else if page.failed {
            StatusView(
                icon: "error", title: MavioStrings.stateError,
                action: (MavioStrings.actionRetry, { Task { await next() } }))
        } else if page.cards.isEmpty {
            StatusView(title: MavioStrings.stateEmpty)
        }
    }

    private func next() async {
        guard page.more, !page.loading else { return }
        let generation = page.generation
        page.loading = true
        page.failed = false
        do {
            let result = try await load(UInt32(page.cards.count), limit)
            guard generation == page.generation else { return }
            page.cards += result.cards
            page.total = result.total ?? page.total
            page.more = result.more && !result.cards.isEmpty
        } catch {
            guard generation == page.generation else { return }
            page.failed = true
        }
        page.loading = false
    }
}
