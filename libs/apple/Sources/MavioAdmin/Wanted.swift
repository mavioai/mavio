#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    // The settings' Wanted (docs/clients.md §6.3.16), as the web UI's: the
    // monitored movies and episodes missing a file or below their profile's
    // cutoff, as Radarr's and Sonarr's Wanted, each searched for or
    // unmonitored, all searched at once, and an RSS sync.

    /// Wanted: the monitored movies or episodes missing a file or below their cutoff, page by page.
    struct WantedView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var filter = Mavio_Acquisition_V1_WantedFilter.missing
        @State private var kind = Mavio_Acquisition_V1_WantedKind.movie
        @State private var movies: [Mavio_Acquisition_V1_WantedMovie] = []
        @State private var episodes: [Mavio_Acquisition_V1_WantedEpisode] = []
        @State private var total: Int32 = 0
        @State private var loaded = false
        @State private var failure: String?
        @State private var queued: Int?
        @State private var synced = false
        @State private var searching = false
        @State private var syncing = false
        @State private var searchFailure: String?
        @State private var syncFailure: String?

        private static let page: Int32 = 50

        var body: some View {
            SettingsList(message: MavioStrings.wantedMessage) {
                Section {
                } header: {
                    VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                        toolbar
                        if let status {
                            Text(status)
                                .type(theme.type.callout)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                }
                Section {
                    if kind == .movie {
                        ForEach(movies, id: \.itemID) { movie in
                            WantedRow(
                                server: server, itemId: movie.itemID, movie: true, label: label(movie),
                                detail: detail(movie), changed: refresh
                            )
                            .onAppear { more(after: movie.itemID) }
                        }
                    } else {
                        ForEach(episodes, id: \.itemID) { episode in
                            WantedRow(
                                server: server, itemId: episode.itemID, movie: false, label: label(episode),
                                detail: detail(episode), changed: refresh
                            )
                            .onAppear { more(after: episode.itemID) }
                        }
                    }
                    if loaded && total == 0 {
                        Text(MavioStrings.wantedNone)
                            .type(theme.type.callout)
                            .foregroundStyle(theme.colors.textSecondary)
                    }
                    if let failure {
                        Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                } footer: {
                    if loaded { Text(MavioStrings.wantedCount(count: Int(total))) }
                }
            }
            .overlay { if !loaded && failure == nil { ProgressView() } }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsWanted)
            .inlineTitle()
            .task(id: "\(filter.rawValue)-\(kind.rawValue)") { await load(offset: 0) }
        }

        /// The choices of what is shown, and the buttons syncing RSS and searching for all of it.
        private var toolbar: some View {
            HStack(spacing: MavioSpacing.s8) {
                Picker(MavioStrings.wantedFilter, selection: $filter) {
                    Text(MavioStrings.wantedMissing).tag(Mavio_Acquisition_V1_WantedFilter.missing)
                    Text(MavioStrings.wantedCutoffUnmet).tag(Mavio_Acquisition_V1_WantedFilter.cutoffUnmet)
                }
                .labelsHidden()
                .fixedSize()
                .accessibilityLabel(MavioStrings.wantedFilter)
                Picker(MavioStrings.wantedKind, selection: $kind) {
                    Text(MavioStrings.wantedMovies).tag(Mavio_Acquisition_V1_WantedKind.movie)
                    Text(MavioStrings.wantedEpisodes).tag(Mavio_Acquisition_V1_WantedKind.episode)
                }
                .labelsHidden()
                .fixedSize()
                .accessibilityLabel(MavioStrings.wantedKind)
                Spacer(minLength: 0)
                Button(syncing ? "…" : MavioStrings.wantedRssSync) { syncRss() }
                    .buttonStyle(.bordered)
                    .disabled(syncing)
                Button(searching ? "…" : MavioStrings.wantedSearchAll) { searchAll() }
                    .buttonStyle(.borderedProminent)
                    .disabled(searching || total == 0)
            }
            .type(theme.type.body)
            .onChange(of: filter) { queued = nil }
            .onChange(of: kind) { queued = nil }
        }

        /// What the last actions did, or why they failed.
        private var status: String? {
            let parts = [
                queued.map { MavioStrings.wantedQueued(count: $0) },
                synced ? MavioStrings.wantedRssQueued : nil,
                searchFailure,
                syncFailure,
            ]
            .compactMap(\.self)
            return parts.isEmpty ? nil : parts.joined(separator: " · ")
        }

        private func label(_ m: Mavio_Acquisition_V1_WantedMovie) -> String {
            m.year > 0 ? "\(m.name) (\(m.year))" : m.name
        }

        private func detail(_ m: Mavio_Acquisition_V1_WantedMovie) -> String {
            [
                m.quality.isEmpty ? nil : MavioStrings.wantedHas(quality: m.quality),
                m.available ? nil : MavioStrings.wantedNotAvailable,
            ]
            .compactMap(\.self)
            .joined(separator: " · ")
        }

        private func label(_ e: Mavio_Acquisition_V1_WantedEpisode) -> String {
            let code = wantedEpisodeCode(season: e.seasonNumber, episode: e.episodeNumber)
            return "\(e.seriesName) · \(code)" + (e.name.isEmpty ? "" : " · \(e.name)")
        }

        private func detail(_ e: Mavio_Acquisition_V1_WantedEpisode) -> String {
            [
                e.hasAirTime ? e.airTime.time?.formatted(date: .abbreviated, time: .omitted) : nil,
                e.absoluteNumber > 0 ? MavioStrings.wantedAbsolute(number: String(e.absoluteNumber)) : nil,
                e.quality.isEmpty ? nil : MavioStrings.wantedHas(quality: e.quality),
            ]
            .compactMap(\.self)
            .joined(separator: " · ")
        }

        /// Loads the next page when the last row shows.
        private func more(after id: String) {
            let ids = kind == .movie ? movies.map(\.itemID) : episodes.map(\.itemID)
            guard id == ids.last, ids.count < total else { return }
            Task { await load(offset: Int32(ids.count)) }
        }

        /// Loads the rows shown again, after an action changed them.
        private func refresh() {
            Task { await load(offset: 0, limit: max(Self.page, Int32(kind == .movie ? movies.count : episodes.count))) }
        }

        private func load(offset: Int32, limit: Int32 = page) async {
            var request = Mavio_Acquisition_V1_ListWantedRequest()
            request.filter = filter
            request.kind = kind
            request.limit = min(limit, 1000)
            request.offset = offset
            let shownKind = kind
            do {
                let response = try await model.client.call(
                    server.id, Procedure.wanted("ListWanted"), request, as: Mavio_Acquisition_V1_ListWantedResponse.self
                )
                guard shownKind == kind, request.filter == filter else { return }
                if offset == 0 {
                    movies = response.movies
                    episodes = response.episodes
                } else {
                    movies += response.movies
                    episodes += response.episodes
                }
                total = response.total
                loaded = true
                failure = nil
            } catch is CancellationError {
            } catch {
                if offset == 0 { loaded = false }
                failure = error.reason
            }
        }

        private func searchAll() {
            searching = true
            searchFailure = nil
            var request = Mavio_Acquisition_V1_SearchWantedRequest()
            request.filter = filter
            request.kind = kind
            Task {
                defer { searching = false }
                do {
                    let response = try await model.client.call(
                        server.id, Procedure.wanted("SearchWanted"), request,
                        as: Mavio_Acquisition_V1_SearchWantedResponse.self)
                    queued = Int(response.queued)
                    refresh()
                } catch {
                    searchFailure = error.reason
                }
            }
        }

        private func syncRss() {
            syncing = true
            syncFailure = nil
            Task {
                defer { syncing = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.wanted("SyncRSS"), Mavio_Acquisition_V1_SyncRSSRequest(),
                        as: Mavio_Acquisition_V1_SyncRSSResponse.self)
                    synced = true
                } catch {
                    syncFailure = error.reason
                }
            }
        }
    }

    /// An episode's numbers, as "S01E02".
    private func wantedEpisodeCode(season: Int32, episode: Int32) -> String {
        String(format: "S%02dE%02d", season, episode)
    }

    /// A wanted movie or episode's row: its name, what it shows, and searching and unmonitoring it.
    private struct WantedRow: View {
        let server: ServerRecord
        let itemId: String
        let movie: Bool
        let label: String
        let detail: String
        let changed: () -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var searched = false
        @State private var searching = false
        @State private var unmonitoring = false
        @State private var failure: String?

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    ItemLink(server: server, itemId: itemId) { Text(label).type(theme.type.body) }
                    if !detail.isEmpty {
                        Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                    if let failure {
                        Text(failure).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                    }
                }
                Spacer(minLength: 0)
                Button(MavioStrings.wantedUnmonitor) { unmonitor() }
                    .disabled(unmonitoring)
                Button(searched ? MavioStrings.wantedSearching : MavioStrings.wantedSearch) { search() }
                    .buttonStyle(.bordered)
                    .disabled(searching || searched)
            }
        }

        private func search() {
            searching = true
            failure = nil
            var request = Mavio_Acquisition_V1_SearchWantedRequest()
            request.itemIds = [itemId]
            Task {
                defer { searching = false }
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.wanted("SearchWanted"), request,
                        as: Mavio_Acquisition_V1_SearchWantedResponse.self)
                    searched = true
                    changed()
                } catch {
                    failure = error.reason
                }
            }
        }

        /// Unmonitors the item: a movie through its wanted, an episode by SetMonitored.
        private func unmonitor() {
            unmonitoring = true
            failure = nil
            Task {
                defer { unmonitoring = false }
                do {
                    if movie {
                        var get = Mavio_Acquisition_V1_GetWantedRequest()
                        get.itemID = itemId
                        let response = try await model.client.call(
                            server.id, Procedure.wanted("GetWanted"), get,
                            as: Mavio_Acquisition_V1_GetWantedResponse.self)
                        guard response.hasWanted else {
                            failure = "not wanted"
                            return
                        }
                        var wanted = response.wanted
                        wanted.monitored = false
                        var update = Mavio_Acquisition_V1_UpdateWantedRequest()
                        update.wanted = wanted
                        _ = try await model.client.call(
                            server.id, Procedure.wanted("UpdateWanted"), update,
                            as: Mavio_Acquisition_V1_UpdateWantedResponse.self)
                    } else {
                        var items = Mavio_Acquisition_V1_MonitoredItems()
                        items.itemIds = [itemId]
                        items.monitored = false
                        var request = Mavio_Acquisition_V1_SetMonitoredRequest()
                        request.items = items
                        _ = try await model.client.call(
                            server.id, Procedure.wanted("SetMonitored"), request,
                            as: Mavio_Acquisition_V1_SetMonitoredResponse.self)
                    }
                    changed()
                } catch {
                    failure = error.reason
                }
            }
        }
    }
#endif
