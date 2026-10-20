import MavioKit
import MavioUI
import SwiftUI

/// The libraries of every server, as tiles.
struct LibrariesView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var libraries: [String: Libraries] = [:]
    @State private var failed: [String: String] = [:]

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: MavioSpacing.s48) {
                ForEach(model.signedInServers, id: \.id) { server in
                    VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                        if model.signedIn.count > 1 {
                            Text(server.name).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                        }
                        if let entries = libraries[server.id]?.libraries {
                            LazyVGrid(
                                columns: Array(
                                    repeating: GridItem(.flexible(), spacing: theme.layout.gutter), count: 4),
                                spacing: theme.layout.gutter
                            ) {
                                ForEach(entries, id: \.self) { library in
                                    NavigationLink(value: Route.library(library)) {
                                        LibraryTile(library: library)
                                    }
                                    .buttonStyle(CardButtonStyle(wide: true))
                                }
                            }
                        } else if failed[server.id] != nil {
                            Text(MavioStrings.homeServerUnreachable(server: server.name))
                                .type(theme.type.callout).foregroundStyle(theme.colors.textTertiary)
                        } else {
                            ProgressView().frame(maxWidth: .infinity)
                        }
                    }
                }
            }
            .padding(.horizontal, theme.layout.margin)
            .padding(.top, navBarHeight)
            .padding(.bottom, 120)
        }
        .reportsScroll()
        .ignoresSafeArea()
        .toolbar(.hidden, for: .navigationBar)
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
private struct LibraryTile: View {
    let library: LibraryEntry
    @Environment(\.theme) private var theme

    var body: some View {
        ZStack(alignment: .bottomLeading) {
            ArtworkView(library.imageUrl.isEmpty ? nil : library.imageUrl, title: library.name, icon: "folder")
            LinearGradient(colors: [.clear, theme.colors.scrim], startPoint: .center, endPoint: .bottom)
            Text(library.name).type(theme.type.title3).foregroundStyle(.white).padding(MavioSpacing.s24)
        }
        .aspectRatio(16 / 9, contentMode: .fit)
        .focusRing()
    }
}

/// A library (docs/tv.md §6.3): its name at the top left beside its
/// Recommended and All in a pill, focus choosing without selecting; All's
/// filters in a pill alike, and the order in a menu, over the grid.
struct LibraryView: View {
    let library: LibraryEntry
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var tab = LibraryTab.recommended
    @State private var rows: [Row]?
    @State private var cards: [Card] = []
    @State private var total: UInt32?
    @State private var more = true
    @State private var loading = false
    @State private var sort: Sort = .name
    @State private var choice = Choice.all
    @State private var generation = 0

    enum LibraryTab: CaseIterable { case recommended, all }

    /// The filters, as the pill names them.
    enum Choice: CaseIterable {
        case all, unplayed, played, favorites

        var label: String {
            switch self {
            case .all: MavioStrings.libraryFilterAll
            case .unplayed: MavioStrings.libraryFilterUnplayed
            case .played: MavioStrings.libraryFilterPlayed
            case .favorites: MavioStrings.libraryFilterFavorites
            }
        }

        var filter: Filter {
            Filter(
                played: self == .unplayed ? false : self == .played ? true : nil, favorite: self == .favorites,
                genre: nil)
        }
    }

    private static let sorts: [(Sort, String)] = [
        (.name, MavioStrings.librarySortName), (.dateAdded, MavioStrings.librarySortDateAdded),
        (.released, MavioStrings.librarySortReleased), (.rating, MavioStrings.librarySortRating),
        (.lastPlayed, MavioStrings.librarySortLastPlayed), (.random, MavioStrings.librarySortRandom),
    ]

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: MavioSpacing.s40) {
                HStack(spacing: MavioSpacing.s32) {
                    Text(library.name).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
                    Pill(
                        items: LibraryTab.allCases, selected: $tab,
                        label: { $0 == .recommended ? MavioStrings.libraryRecommended : MavioStrings.libraryAll })
                }
                .padding(.horizontal, theme.layout.margin)
                switch tab {
                case .recommended:
                    if let rows {
                        if rows.isEmpty {
                            StatusView(title: MavioStrings.stateEmpty)
                        } else {
                            LazyVStack(alignment: .leading, spacing: MavioSpacing.s48) {
                                ForEach(rows, id: \.id) { RowView(row: $0, several: false) }
                            }
                        }
                    } else {
                        ProgressView().frame(maxWidth: .infinity)
                    }
                case .all:
                    all
                }
            }
            .padding(.top, MavioSpacing.s48)
            .padding(.bottom, 120)
        }
        .ignoresSafeArea()
        .toolbar(.hidden, for: .navigationBar)
        .task(id: model.revision(library.serverId)) {
            rows = try? await model.client.recommended(serverId: library.serverId, libraryId: library.id)
            if rows?.isEmpty == true { tab = .all }
            reset()
        }
        .onChange(of: sort) { reset() }
        .onChange(of: choice) { reset() }
    }

    @ViewBuilder private var all: some View {
        HStack(spacing: MavioSpacing.s24) {
            Pill(items: Choice.allCases, selected: $choice, label: \.label)
            Menu {
                Picker(MavioStrings.librarySort, selection: $sort) {
                    ForEach(Self.sorts, id: \.0) { Text($0.1).tag($0.0) }
                }
            } label: {
                Label {
                    Text(Self.sorts.first { $0.0 == sort }?.1 ?? "")
                } icon: {
                    Icon("sort", size: 28)
                }
            }
            .buttonStyle(TVButtonStyle())
            Spacer()
            if let total {
                Text(MavioStrings.libraryItemCount(count: Int(total))).type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
            }
        }
        .padding(.horizontal, theme.layout.margin)
        .focusSection()
        LazyVGrid(
            columns: [GridItem(.adaptive(minimum: theme.layout.poster), spacing: theme.layout.gutter)],
            spacing: MavioSpacing.s48
        ) {
            ForEach(Array(cards.enumerated()), id: \.offset) { index, card in
                CardLink(card: card, wide: false)
                    .onAppear { if index >= cards.count - 12 { Task { await loadMore() } } }
            }
        }
        .padding(.horizontal, theme.layout.margin)
        if cards.isEmpty && !more {
            StatusView(title: MavioStrings.stateEmpty)
        }
    }

    private func reset() {
        generation += 1
        (cards, total, more, loading) = ([], nil, true, false)
        Task { await loadMore() }
    }

    private func loadMore() async {
        guard more, !loading else { return }
        loading = true
        let started = generation
        defer { if started == generation { loading = false } }
        guard
            let page = try? await model.client.libraryItems(
                serverId: library.serverId, libraryId: library.id, sort: sort, filter: choice.filter,
                offset: UInt32(cards.count), limit: 60),
            started == generation
        else { return }
        cards += page.cards
        total = page.total
        more = page.more
    }
}

/// Choices in a pill like the tabs: the current one filled with
/// `color.fillSelected`, the focused one white; focus chooses (§6.3).
struct Pill<Item: Hashable>: View {
    let items: [Item]
    @Binding var selected: Item
    let label: (Item) -> String
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s4) {
            ForEach(items, id: \.self) { item in
                Button(label(item)) { selected = item }
                    .buttonStyle(TabButtonStyle(selected: item == selected))
                    .onFocusChange { if $0 { selected = item } }
            }
        }
        .padding(MavioSpacing.s4)
        .background(theme.colors.surfaceRaised.opacity(0.6), in: .capsule)
        .focusSection()
    }
}
