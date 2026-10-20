#if os(macOS)
    import AppKit
    import MavioMac

    /// The settings' Wanted (docs/clients.md §6.3.16), as the web UI's: the
    /// monitored movies and episodes missing a file or below their
    /// profile's cutoff, as Radarr's and Sonarr's Wanted, page by page, each
    /// searched for or unmonitored, all searched at once, and an RSS sync.
    final class WantedPage: FormController {
        private var filter = Mavio_Acquisition_V1_WantedFilter.missing
        private var kind = Mavio_Acquisition_V1_WantedKind.movie
        private var movies: [Mavio_Acquisition_V1_WantedMovie] = []
        private var episodes: [Mavio_Acquisition_V1_WantedEpisode] = []
        private var total: Int32 = 0
        private var loaded = false
        private var loadingMore = false
        private var failure: String?
        private var queued: Int?
        private var synced = false
        private var searching = false
        private var syncing = false
        private var searchFailure: String?
        private var syncFailure: String?
        /// The rows' state, by their items' IDs.
        private var searched: Set<String> = []
        private var busy: Set<String> = []
        private var rowFailures: [String: String] = [:]
        private var loading: Task<Void, Never>?

        private static let page: Int32 = 50

        override func viewDidLoad() {
            title = MavioStrings.settingsWanted
            message = MavioStrings.wantedMessage
            super.viewDidLoad()
            onNearEnd = { [weak self] in self?.more() }
            reload()
        }

        isolated deinit { loading?.cancel() }

        override func sections() -> [NSView] {
            var rows: [NSView] = []
            if kind == .movie {
                rows = movies.map { m in
                    wantedRow(
                        itemId: m.itemID, movie: true, label: m.year > 0 ? "\(m.name) (\(m.year))" : m.name,
                        detail: detail(m))
                }
            } else {
                rows = episodes.map { e in wantedRow(itemId: e.itemID, movie: false, label: label(e), detail: detail(e))
                }
            }
            if loaded && total == 0 { rows.append(note(MavioStrings.wantedNone)) }
            if let failure { rows.append(note(failure, failed: true)) }
            if !loaded && failure == nil || loadingMore { rows.append(spinnerRow()) }
            var head: [NSView] = [plain(toolbar())]
            if let status { head.append(plain(label(status, theme.colors.textSecondary))) }
            return head + [card(footer: loaded ? MavioStrings.wantedCount(count: Int(total)) : nil, rows)]
        }

        /// The choices of what is shown, and the buttons syncing RSS and
        /// searching for all of it.
        private func toolbar() -> NSView {
            let filters: [(String, Mavio_Acquisition_V1_WantedFilter)] = [
                (MavioStrings.wantedMissing, .missing), (MavioStrings.wantedCutoffUnmet, .cutoffUnmet),
            ]
            let kinds: [(String, Mavio_Acquisition_V1_WantedKind)] = [
                (MavioStrings.wantedMovies, .movie), (MavioStrings.wantedEpisodes, .episode),
            ]
            let filterMenu = PopUp(
                filters.map(\.0), selected: filters.firstIndex { $0.1 == filter } ?? 0, label: MavioStrings.wantedFilter
            ) { [weak self] i in
                self?.filter = filters[i].1
                self?.reload()
            }
            let kindMenu = PopUp(
                kinds.map(\.0), selected: kinds.firstIndex { $0.1 == kind } ?? 0, label: MavioStrings.wantedKind
            ) { [weak self] i in
                self?.kind = kinds[i].1
                self?.reload()
            }
            let sync = MavioButton(syncing ? "…" : MavioStrings.wantedRssSync, kind: .secondary, theme: theme) {
                [weak self] in self?.syncRss()
            }
            sync.isEnabled = !syncing
            let all = MavioButton(searching ? "…" : MavioStrings.wantedSearchAll, kind: .primary, theme: theme) {
                [weak self] in self?.searchAll()
            }
            all.isEnabled = !searching && total > 0
            return NSStackView.horizontal([filterMenu, kindMenu, Spacer(), sync, all], spacing: MavioSpacing.s8)
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

        private func detail(_ m: Mavio_Acquisition_V1_WantedMovie) -> String {
            dotted([
                m.quality.isEmpty ? nil : MavioStrings.wantedHas(quality: m.quality),
                m.available ? nil : MavioStrings.wantedNotAvailable,
            ])
        }

        private func label(_ e: Mavio_Acquisition_V1_WantedEpisode) -> String {
            let code = String(format: "S%02dE%02d", e.seasonNumber, e.episodeNumber)
            return "\(e.seriesName) · \(code)" + (e.name.isEmpty ? "" : " · \(e.name)")
        }

        private func detail(_ e: Mavio_Acquisition_V1_WantedEpisode) -> String {
            dotted([
                e.hasAirTime ? e.airTime.time?.formatted(date: .abbreviated, time: .omitted) : nil,
                e.absoluteNumber > 0 ? MavioStrings.wantedAbsolute(number: String(e.absoluteNumber)) : nil,
                e.quality.isEmpty ? nil : MavioStrings.wantedHas(quality: e.quality),
            ])
        }

        /// A wanted movie or episode's row: its name, what it shows, and
        /// searching and unmonitoring it.
        private func wantedRow(itemId: String, movie: Bool, label: String, detail: String) -> NSView {
            var text: [NSView] = [itemLink(label, itemId: itemId)]
            if !detail.isEmpty {
                text.append(
                    NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0))
            }
            if let failure = rowFailures[itemId] {
                text.append(NSTextField.label(failure, theme.fonts.footnote, color: theme.colors.danger, lines: 0))
            }
            let stack = NSStackView.vertical(text, spacing: 2)
            stack.setContentHuggingPriority(.init(1), for: .horizontal)
            let unmonitor = MavioButton(MavioStrings.wantedUnmonitor, kind: .link, theme: theme) { [weak self] in
                self?.unmonitor(itemId, movie: movie)
            }
            unmonitor.isEnabled = !busy.contains(itemId)
            let done = searched.contains(itemId)
            let search = MavioButton(
                done ? MavioStrings.wantedSearching : MavioStrings.wantedSearch, kind: .secondary, theme: theme
            ) { [weak self] in self?.search(itemId) }
            search.isEnabled = !done && !busy.contains(itemId)
            return row(NSStackView.horizontal([stack, unmonitor, search], spacing: MavioSpacing.s12))
        }

        // MARK: Loading

        private var shown: Int { kind == .movie ? movies.count : episodes.count }

        /// Loads the first page of what is chosen.
        private func reload() {
            queued = nil
            loaded = false
            movies = []
            episodes = []
            failure = nil
            render()
            load(offset: 0)
        }

        /// Loads the next page when the end of the list shows.
        private func more() {
            guard loaded, !loadingMore, failure == nil, shown < total else { return }
            loadingMore = true
            load(offset: Int32(shown))
            render()
        }

        /// Loads the rows shown again, after an action changed them.
        private func refresh() {
            load(offset: 0, limit: max(Self.page, Int32(shown)))
        }

        private func load(offset: Int32, limit: Int32 = page) {
            var request = Mavio_Acquisition_V1_ListWantedRequest()
            request.filter = filter
            request.kind = kind
            request.limit = min(limit, 1000)
            request.offset = offset
            loading?.cancel()
            loading = Task { [weak self, model, server] in
                do {
                    let response = try await model.client.call(
                        server.id, Procedure.wanted("ListWanted"), request,
                        as: Mavio_Acquisition_V1_ListWantedResponse.self)
                    guard let self, !Task.isCancelled, request.kind == self.kind, request.filter == self.filter
                    else { return }
                    if offset == 0 {
                        self.movies = response.movies
                        self.episodes = response.episodes
                    } else {
                        self.movies += response.movies
                        self.episodes += response.episodes
                    }
                    self.total = response.total
                    self.loaded = true
                    self.failure = nil
                } catch is CancellationError {
                    return
                } catch {
                    guard let self, !Task.isCancelled else { return }
                    if offset == 0 { self.loaded = false }
                    self.failure = error.reason
                }
                self?.loadingMore = false
                self?.render()
            }
        }

        // MARK: Actions

        private func searchAll() {
            searching = true
            searchFailure = nil
            render()
            var request = Mavio_Acquisition_V1_SearchWantedRequest()
            request.filter = filter
            request.kind = kind
            run { [weak self, model, server] in
                do {
                    let response = try await model.client.call(
                        server.id, Procedure.wanted("SearchWanted"), request,
                        as: Mavio_Acquisition_V1_SearchWantedResponse.self)
                    self?.queued = Int(response.queued)
                    self?.refresh()
                } catch {
                    self?.searchFailure = error.reason
                }
                self?.searching = false
                self?.render()
            }
        }

        private func syncRss() {
            syncing = true
            syncFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.wanted("SyncRSS"), Mavio_Acquisition_V1_SyncRSSRequest(),
                        as: Mavio_Acquisition_V1_SyncRSSResponse.self)
                    self?.synced = true
                } catch {
                    self?.syncFailure = error.reason
                }
                self?.syncing = false
                self?.render()
            }
        }

        private func search(_ itemId: String) {
            busy.insert(itemId)
            rowFailures[itemId] = nil
            render()
            var request = Mavio_Acquisition_V1_SearchWantedRequest()
            request.itemIds = [itemId]
            run { [weak self, model, server] in
                do {
                    _ = try await model.client.call(
                        server.id, Procedure.wanted("SearchWanted"), request,
                        as: Mavio_Acquisition_V1_SearchWantedResponse.self)
                    self?.searched.insert(itemId)
                    self?.refresh()
                } catch {
                    self?.rowFailures[itemId] = error.reason
                }
                self?.busy.remove(itemId)
                self?.render()
            }
        }

        /// Unmonitors an item: a movie through its wanted, an episode by
        /// SetMonitored.
        private func unmonitor(_ itemId: String, movie: Bool) {
            busy.insert(itemId)
            rowFailures[itemId] = nil
            render()
            run { [weak self, model, server] in
                do {
                    if movie {
                        var get = Mavio_Acquisition_V1_GetWantedRequest()
                        get.itemID = itemId
                        let response = try await model.client.call(
                            server.id, Procedure.wanted("GetWanted"), get,
                            as: Mavio_Acquisition_V1_GetWantedResponse.self)
                        guard response.hasWanted else {
                            self?.rowFailures[itemId] = "not wanted"
                            self?.busy.remove(itemId)
                            self?.render()
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
                    self?.refresh()
                } catch {
                    self?.rowFailures[itemId] = error.reason
                }
                self?.busy.remove(itemId)
                self?.render()
            }
        }
    }
#endif
