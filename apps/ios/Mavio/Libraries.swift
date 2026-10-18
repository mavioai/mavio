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

/// A library: its Recommended tab of hubs beside the grid of all its items,
/// sorted and filtered, as the web UI's: the tabs under the name, and over
/// the grid the genre it is narrowed to, the filters, the order and the
/// count.
struct LibraryView: View {
    let library: LibraryEntry
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var tab = Tab.recommended
    @State private var rows: [Row]?
    @State private var page = Paged()
    @State private var sort: Sort = .name
    @State private var filter = Filter(played: nil, favorite: false, genre: nil)

    private enum Tab { case recommended, all }

    var body: some View {
        ScrollView {
            Picker(MavioStrings.libraryRecommended, selection: $tab) {
                Text(MavioStrings.libraryRecommended).tag(Tab.recommended)
                Text(MavioStrings.libraryAll).tag(Tab.all)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .fixedSize()
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, theme.layout.margin)
            .padding(.bottom, MavioSpacing.s12)
            switch tab {
            case .recommended:
                LazyVStack(alignment: .leading, spacing: 0) {
                    if let rows {
                        ForEach(rows, id: \.id) { RowView(row: $0, several: false) }
                        if rows.isEmpty { StatusView(title: MavioStrings.stateEmpty) }
                    } else {
                        ProgressView().padding(MavioSpacing.s40)
                    }
                }
            case .all:
                LibraryFilters(sort: $sort, filter: $filter, total: page.total)
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.bottom, MavioSpacing.s16)
                PagedGrid(page: $page) { offset, limit in
                    try await model.client.libraryItems(
                        serverId: library.serverId, libraryId: library.id, sort: sort, filter: filter, offset: offset,
                        limit: limit)
                }
            }
        }
        .navigationTitle(library.name)
        .task(id: model.revision(library.serverId)) {
            rows = try? await model.client.recommended(serverId: library.serverId, libraryId: library.id)
            if rows?.isEmpty == true { tab = .all }
        }
        .onChange(of: model.revision(library.serverId)) { page.reset() }
        .onChange(of: sort) { page.reset() }
        .onChange(of: filter) { page.reset() }
    }
}

/// Over a library's grid: the genre it is narrowed to, which clears it, the
/// filters as pills, the order's menu and the count, wrapping as the width
/// ends.
private struct LibraryFilters: View {
    @Binding var sort: Sort
    @Binding var filter: Filter
    let total: UInt32?
    @Environment(\.theme) private var theme

    /// The filters, as the pills name them.
    private enum Choice: CaseIterable {
        case all, unplayed, played, favorites

        var label: String {
            switch self {
            case .all: MavioStrings.libraryFilterAll
            case .unplayed: MavioStrings.libraryFilterUnplayed
            case .played: MavioStrings.libraryFilterPlayed
            case .favorites: MavioStrings.libraryFilterFavorites
            }
        }

        func filter(genre: String?) -> Filter {
            switch self {
            case .all: Filter(played: nil, favorite: false, genre: genre)
            case .unplayed: Filter(played: false, favorite: false, genre: genre)
            case .played: Filter(played: true, favorite: false, genre: genre)
            case .favorites: Filter(played: nil, favorite: true, genre: genre)
            }
        }
    }

    private static let sorts: [(Sort, String)] = [
        (.name, MavioStrings.librarySortName), (.dateAdded, MavioStrings.librarySortDateAdded),
        (.released, MavioStrings.librarySortReleased), (.rating, MavioStrings.librarySortRating),
        (.lastPlayed, MavioStrings.librarySortLastPlayed), (.random, MavioStrings.librarySortRandom),
    ]

    var body: some View {
        Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
            if let genre = filter.genre {
                Button {
                    filter.genre = nil
                } label: {
                    HStack(spacing: 6) {
                        Text(MavioStrings.libraryGenreValue(value: genre))
                        Icon("close", size: 14)
                    }
                    .type(theme.type.callout).fontWeight(.semibold)
                    .foregroundStyle(theme.colors.onAccent)
                    .padding(.leading, 14).padding(.trailing, 10)
                    .frame(height: 32)
                    .background(theme.colors.accent, in: .capsule)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(
                    "\(MavioStrings.libraryGenreValue(value: genre)), \(MavioStrings.libraryClearValue)")
            }
            ForEach(Choice.allCases, id: \.self) { choice in
                let on = choice.filter(genre: filter.genre) == filter
                Button {
                    filter = choice.filter(genre: filter.genre)
                } label: {
                    Text(choice.label)
                        .type(theme.type.callout).fontWeight(.medium)
                        .foregroundStyle(on ? theme.colors.onButton : theme.colors.textPrimary)
                        .padding(.horizontal, 14)
                        .frame(height: 32)
                        .background(on ? theme.colors.button : theme.colors.fill, in: .capsule)
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(on ? .isSelected : [])
            }
            Menu {
                Picker(MavioStrings.librarySort, selection: $sort) {
                    ForEach(Self.sorts, id: \.0) { Text($0.1).tag($0.0) }
                }
            } label: {
                HStack(spacing: 6) {
                    Icon("sort", size: 16)
                    Text(MavioStrings.librarySortBy(sort: Self.sorts.first { $0.0 == sort }?.1 ?? ""))
                }
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.textSecondary)
                .padding(.leading, MavioSpacing.s4).padding(.trailing, MavioSpacing.s12)
                .frame(height: 32)
                .contentShape(.capsule)
            }
            if let total {
                Text(MavioStrings.libraryItemCount(count: Int(total)))
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textTertiary)
                    .frame(height: 32)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .animation(.easeOut(duration: MavioDuration.fast), value: filter)
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
