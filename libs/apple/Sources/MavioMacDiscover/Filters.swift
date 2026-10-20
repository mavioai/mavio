#if os(macOS)
    import AppKit
    import MavioMac

    /// A catalog's filters and sorts in a dialog (docs/clients.md §6.3.28),
    /// as the web UI's: the order of its titles and the filters it takes —
    /// genres, studios, keywords and streaming services by their values,
    /// the original language, and ranges of years, runtime, rating and
    /// votes — its sections scrolling above Reset, Cancel and Apply. Its
    /// values are those of the catalog's kind of title, movies when it
    /// mixes them.
    @MainActor
    final class FiltersDialog {
        private let catalog: Catalog
        private weak var page: Page?
        private let apply: @MainActor (CatalogQuery) -> Void
        private let dialog: Dialog
        private let theme: Theme
        private var f: CatalogQuery
        private var reset: MavioButton?
        /// The sections of values, which load and keep their own state.
        private var parts: [AnyObject] = []

        /// The filters of catalog, set as value, over page; apply takes
        /// the filters applied.
        init(catalog: Catalog, value: CatalogQuery, page: Page, apply: @escaping @MainActor (CatalogQuery) -> Void) {
            self.catalog = catalog
            self.page = page
            self.apply = apply
            theme = page.theme
            f = value
            dialog = Dialog(title: MavioStrings.discoverFilters, width: 560, height: 640, theme: page.theme)
        }

        private var mediaKind: MediaKind { f.kind == .unspecified ? .movie : f.kind }
        private var region: String { f.watchRegion.isEmpty ? defaultWatchRegion : f.watchRegion }
        private func takes(_ filter: Mavio_Plugin_V1_DiscoverFilter) -> Bool { catalog.filters.contains(filter) }

        /// Shows the dialog over its page.
        func present() {
            guard let page else { return }
            let reset = MavioButton(MavioStrings.discoverReset, kind: .secondary, theme: theme) { [weak self] in
                guard let self else { return }
                self.f = self.f.unfiltered
                self.build()
            }
            self.reset = reset
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                self?.dialog.close()
            }
            let applyButton = MavioButton(MavioStrings.discoverApply, kind: .primary, theme: theme) { [weak self] in
                guard let self else { return }
                let value = self.f
                self.dialog.close()
                self.apply(value)
            }
            dialog.setButtons(leading: [reset], trailing: [cancel, applyButton])
            build()
            dialog.present(over: page)
            // Kept while shown.
            retained = self
            dialog.onClose = { [weak self] in
                self?.parts = []
                self?.retained = nil
            }
        }

        private var retained: FiltersDialog?

        /// Whether Reset has anything to reset.
        private func changed() {
            reset?.isEnabled = f != f.unfiltered
        }

        private func build() {
            guard let page else { return }
            parts = []
            var views: [NSView] = []
            if !catalog.sorts.isEmpty { views.append(sortSection()) }
            if takes(.genre) {
                views.append(
                    listed(
                        .genre, title: MavioStrings.discoverGenres, get: { $0.genres }, set: { $0.genres = $1 },
                        page: page))
            }
            if takes(.studio), mediaKind == .movie {
                views.append(
                    searched(
                        .studio, title: MavioStrings.discoverStudios, get: { $0.studios }, set: { $0.studios = $1 },
                        page: page))
            }
            if mediaKind == .series, !f.networks.isEmpty {
                views.append(
                    searched(
                        .network, title: MavioStrings.discoverNetworks, get: { $0.networks },
                        set: { $0.networks = $1 }, searchable: false, page: page))
            }
            if takes(.keyword) {
                views.append(
                    searched(
                        .keyword, title: MavioStrings.discoverKeywords, get: { $0.keywords },
                        set: { $0.keywords = $1 }, page: page))
            }
            if takes(.watchProvider) { views.append(providers(page: page)) }
            if takes(.originalLanguage) {
                let options = [("", MavioStrings.discoverAny)] + DiscoverLocales.languages
                views.append(
                    section(
                        MavioStrings.discoverOriginalLanguage,
                        [
                            PopUp(
                                options.map(\.1), selected: options.firstIndex { $0.0 == f.originalLanguage } ?? 0,
                                label: MavioStrings.discoverOriginalLanguage
                            ) { [weak self] in
                                self?.f.originalLanguage = options[$0].0
                                self?.changed()
                            }
                        ]))
            }
            if takes(.releaseDate) {
                views.append(
                    range(
                        MavioStrings.discoverReleased, from: f.yearFrom, to: f.yearTo,
                        max: Calendar.current.component(.year, from: .now) + 5,
                        set: { $0.yearFrom = $1 }, { $0.yearTo = $1 }))
            }
            if takes(.runtime) {
                views.append(
                    range(
                        MavioStrings.discoverRuntime, from: f.runtimeMin, to: f.runtimeMax, max: 1000,
                        set: { $0.runtimeMin = $1 }, { $0.runtimeMax = $1 }))
            }
            if takes(.rating) {
                let options = (0..<10).map { $0 == 0 ? MavioStrings.discoverAny : $0.formatted() }
                views.append(
                    section(
                        MavioStrings.discoverRatingMin,
                        [
                            PopUp(options, selected: min(max(f.ratingMin, 0), 9), label: MavioStrings.discoverRatingMin)
                            {
                                [weak self] in
                                self?.f.ratingMin = $0
                                self?.changed()
                            }
                        ]))
            }
            if takes(.voteCount) {
                let field = WholeNumberField(MavioStrings.discoverAny, value: f.votesMin, max: 1_000_000, theme: theme)
                field.setAccessibilityLabel(MavioStrings.discoverVotesMin)
                field.onChange = { [weak self] in
                    self?.f.votesMin = $0
                    self?.changed()
                }
                views.append(section(MavioStrings.discoverVotesMin, [field]))
            }
            dialog.setContent(views)
            changed()
        }

        /// A heading over a filter's controls.
        private func section(_ title: String, _ controls: [NSView]) -> NSView {
            let stack = NSStackView.vertical(
                [NSTextField.label(title, theme.fonts.headline, color: theme.colors.textPrimary)] + controls,
                spacing: MavioSpacing.s8)
            for v in controls where v is WrapStack || v is NSScrollView {
                v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            return stack
        }

        private func sortSection() -> NSView {
            let sorts: [Mavio_Plugin_V1_DiscoverSort] = [.unspecified] + catalog.sorts
            let sort = PopUp(
                sorts.map(discoverSortName), selected: sorts.firstIndex(of: f.sort) ?? 0,
                label: MavioStrings.discoverSortBy
            ) { [weak self] in
                guard let self else { return }
                let had = self.f.sort != .unspecified
                self.f.sort = sorts[$0]
                // The order shows only once there is a sort.
                if had != (self.f.sort != .unspecified) { self.build() } else { self.changed() }
            }
            var controls: [NSView] = [sort]
            if f.sort != .unspecified {
                controls.append(
                    PopUp(
                        [MavioStrings.discoverDescending, MavioStrings.discoverAscending], selected: f.ascending ? 1 : 0
                    ) { [weak self] in
                        self?.f.ascending = $0 == 1
                        self?.changed()
                    })
            }
            return section(MavioStrings.discoverSortBy, [NSStackView.horizontal(controls, spacing: MavioSpacing.s8)])
        }

        private func range(
            _ title: String, from: Int, to: Int, max: Int, set setFrom: @escaping (inout CatalogQuery, Int) -> Void,
            _ setTo: @escaping (inout CatalogQuery, Int) -> Void
        ) -> NSView {
            let low = WholeNumberField(MavioStrings.discoverFrom, value: from, max: max, theme: theme)
            low.onChange = { [weak self] in
                guard let self else { return }
                setFrom(&self.f, $0)
                self.changed()
            }
            let high = WholeNumberField(MavioStrings.discoverTo, value: to, max: max, theme: theme)
            high.onChange = { [weak self] in
                guard let self else { return }
                setTo(&self.f, $0)
                self.changed()
            }
            let dash = NSTextField.label("–", theme.fonts.body, color: theme.colors.textSecondary)
            return section(title, [NSStackView.horizontal([low, dash, high], spacing: MavioSpacing.s8)])
        }

        // MARK: Values

        private func listed(
            _ filter: Mavio_Plugin_V1_DiscoverFilter, title: String, get: @escaping (CatalogQuery) -> [String],
            set: @escaping (inout CatalogQuery, [String]) -> Void, page: Page
        ) -> NSView {
            let values = ListedValues(
                page: page, serverId: f.serverId, catalog: catalog.id, filter: filter, mediaKind: mediaKind,
                title: title,
                value: get(f)
            ) { [weak self] in
                guard let self else { return }
                set(&self.f, $0)
                self.changed()
            }
            parts.append(values)
            values.load(region: "")
            return values.view
        }

        private func providers(page: Page) -> NSView {
            let countries = DiscoverLocales.countries
            let start = region
            let values = ListedValues(
                page: page, serverId: f.serverId, catalog: catalog.id, filter: .watchProvider, mediaKind: mediaKind,
                title: MavioStrings.discoverWatchProviders, value: f.watchProviders, alwaysShown: true
            ) { [weak self] in
                guard let self else { return }
                self.f.watchProviders = $0
                self.f.watchRegion = self.region
                self.changed()
            }
            let popup = PopUp(
                countries.map(\.1), selected: countries.firstIndex { $0.0 == start } ?? 0,
                label: MavioStrings.discoverWatchProviders
            ) { [weak self, weak values] i in
                guard let self else { return }
                self.f.watchRegion = countries[i].0
                self.f.watchProviders = []
                values?.value = []
                values?.load(region: countries[i].0)
                self.changed()
            }
            values.control = popup
            parts.append(values)
            values.load(region: start)
            return values.view
        }

        private func searched(
            _ filter: Mavio_Plugin_V1_DiscoverFilter, title: String, get: @escaping (CatalogQuery) -> [NamedValue],
            set: @escaping (inout CatalogQuery, [NamedValue]) -> Void, searchable: Bool = true, page: Page
        ) -> NSView {
            let values = SearchedValues(
                page: page, serverId: f.serverId, catalog: catalog.id, filter: filter, mediaKind: mediaKind,
                title: title,
                value: get(f), searchable: searchable
            ) { [weak self] in
                guard let self else { return }
                set(&self.f, $0)
                self.changed()
            }
            parts.append(values)
            return values.view
        }
    }

    /// The chip of a filter's value: filled with `color.button` while
    /// chosen; toggling it as it is pressed.
    @MainActor
    private func valueChip(
        _ page: Page, _ name: String, image: String? = nil, on: Bool, removable: Bool = false,
        toggle: @escaping @MainActor () -> Void
    ) -> NSView {
        let theme = page.theme
        let chip = page.chip(
            name, image: image, fill: on ? theme.colors.button : theme.colors.fill,
            color: on ? theme.colors.onButton : theme.colors.textPrimary, removable: removable, action: toggle)
        chip.setAccessibilitySelected(on)
        return chip
    }

    /// Values listed whole, as genres and streaming services are, to pick
    /// from; with a control choosing them, as the country of streaming
    /// services, the section shows even when none is listed. A long list
    /// scrolls in a box, or in television mode shows its first 30 and
    /// those picked until Show More.
    @MainActor
    private final class ListedValues {
        /// The section's view.
        let view: NSStackView
        /// The values chosen.
        var value: [String]
        /// The control above the values, as the country.
        var control: NSView? { didSet { render() } }
        private weak var page: Page?
        private let serverId: String
        private let catalog: String
        private let filter: Mavio_Plugin_V1_DiscoverFilter
        private let mediaKind: MediaKind
        private let title: String
        private let alwaysShown: Bool
        private let change: @MainActor ([String]) -> Void
        private var values: [Mavio_Plugin_V1_FilterValue]?
        private var task: Task<Void, Never>?
        private var all = false

        /// The values of filter, value chosen, titled; alwaysShown, the
        /// section shows even without values. change is told the values
        /// chosen.
        init(
            page: Page, serverId: String, catalog: String, filter: Mavio_Plugin_V1_DiscoverFilter, mediaKind: MediaKind,
            title: String, value: [String], alwaysShown: Bool = false, change: @escaping @MainActor ([String]) -> Void
        ) {
            self.page = page
            self.serverId = serverId
            self.catalog = catalog
            self.filter = filter
            self.mediaKind = mediaKind
            self.title = title
            self.value = value
            self.alwaysShown = alwaysShown
            self.change = change
            view = .vertical([], spacing: MavioSpacing.s8)
        }

        isolated deinit { task?.cancel() }

        /// Reads the values, of region for streaming services.
        func load(region: String) {
            guard let model = page?.model else { return }
            values = nil
            all = false
            render()
            let (serverId, catalog, filter, mediaKind) = (serverId, catalog, filter, mediaKind)
            task?.cancel()
            task = Task { [weak self] in
                let values =
                    (try? await model.filterValues(
                        serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, region: region))
                    ?? []
                guard let self, !Task.isCancelled else { return }
                self.values = values
                self.render()
            }
        }

        private func render() {
            guard let page else { return }
            let theme = page.theme
            view.isHidden = values?.isEmpty == true && !alwaysShown
            var views: [NSView] = [NSTextField.label(title, theme.fonts.headline, color: theme.colors.textPrimary)]
            if let control { views.append(control) }
            var wide: [NSView] = []
            if let values {
                var shown = values
                let long = values.count > 30
                if long, theme.isTV, !all {
                    shown = values.enumerated().filter { $0.offset < 30 || value.contains($0.element.id) }.map(
                        \.element)
                }
                let chips = shown.map { v in
                    let on = value.contains(v.id)
                    return valueChip(page, v.name, image: v.imageURL.isEmpty ? nil : v.imageURL, on: on) {
                        [weak self] in self?.toggle(v.id)
                    }
                }
                let wrap = WrapStack(chips, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
                if long, !theme.isTV {
                    let box = NSScrollView()
                    box.translatesAutoresizingMaskIntoConstraints = false
                    box.drawsBackground = false
                    box.hasVerticalScroller = true
                    box.autohidesScrollers = true
                    let document = FlippedView()
                    document.translatesAutoresizingMaskIntoConstraints = false
                    document.addFilling(wrap)
                    box.documentView = document
                    NSLayoutConstraint.activate([
                        document.leadingAnchor.constraint(equalTo: box.contentView.leadingAnchor),
                        document.trailingAnchor.constraint(equalTo: box.contentView.trailingAnchor),
                        document.topAnchor.constraint(equalTo: box.contentView.topAnchor),
                        box.heightAnchor.constraint(equalToConstant: 200),
                    ])
                    views.append(box)
                    wide.append(box)
                } else {
                    views.append(wrap)
                    wide.append(wrap)
                }
                if long, theme.isTV, !all {
                    views.append(
                        MavioButton(MavioStrings.discoverShowMore, kind: .secondary, theme: theme) { [weak self] in
                            self?.all = true
                            self?.render()
                        })
                }
            } else {
                let spinner = NSProgressIndicator()
                spinner.style = .spinning
                spinner.controlSize = .small
                spinner.startAnimation(nil)
                views.append(spinner)
            }
            view.setViews(views)
            for v in wide { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
            FocusEngine.active(for: view)?.schedule()
        }

        private func toggle(_ id: String) {
            if value.contains(id) { value.removeAll { $0 == id } } else { value.append(id) }
            change(value)
            render()
        }
    }

    /// Values found by name, as studios and keywords are: the chosen ones,
    /// a field, and up to 20 it finds not chosen yet; networks, looked up
    /// by ID alone, show the chosen ones only.
    @MainActor
    private final class SearchedValues {
        /// The section's view.
        let view: NSStackView
        private weak var page: Page?
        private let serverId: String
        private let catalog: String
        private let filter: Mavio_Plugin_V1_DiscoverFilter
        private let mediaKind: MediaKind
        private var value: [NamedValue]
        private let change: @MainActor ([NamedValue]) -> Void
        private let chosen = WrapStack([], spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
        private let found = WrapStack([], spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
        private var field: InputField?
        private var results: [Mavio_Plugin_V1_FilterValue] = []
        private var task: Task<Void, Never>?

        /// The values of filter, value chosen, titled; searchable, a field
        /// finds more. change is told the values chosen.
        init(
            page: Page, serverId: String, catalog: String, filter: Mavio_Plugin_V1_DiscoverFilter, mediaKind: MediaKind,
            title: String, value: [NamedValue], searchable: Bool, change: @escaping @MainActor ([NamedValue]) -> Void
        ) {
            self.page = page
            self.serverId = serverId
            self.catalog = catalog
            self.filter = filter
            self.mediaKind = mediaKind
            self.value = value
            self.change = change
            let theme = page.theme
            var views: [NSView] = [
                NSTextField.label(title, theme.fonts.headline, color: theme.colors.textPrimary), chosen,
            ]
            if searchable {
                let field = InputField("", placeholder: MavioStrings.discoverSearchValues(filter: title), theme: theme)
                field.field.isAutomaticTextCompletionEnabled = false
                self.field = field
                views += [field, found]
            }
            view = .vertical(views, spacing: MavioSpacing.s8)
            for v in views.dropFirst() { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
            field?.onChange = { [weak self] in self?.search($0) }
            renderChosen()
        }

        isolated deinit { task?.cancel() }

        private func renderChosen() {
            guard let page else { return }
            chosen.setViews(
                value.map { v in
                    valueChip(page, v.name, on: true, removable: true) { [weak self] in
                        guard let self else { return }
                        self.value.removeAll { $0.id == v.id }
                        self.change(self.value)
                        self.renderChosen()
                        self.renderFound()
                    }
                })
            chosen.isHidden = value.isEmpty
            FocusEngine.active(for: view)?.schedule()
        }

        private func renderFound() {
            guard let page else { return }
            let ids = Set(value.map(\.id))
            let shown = results.filter { !ids.contains($0.id) }.prefix(20)
            found.setViews(
                shown.map { v in
                    valueChip(page, v.name, on: false) { [weak self] in
                        guard let self else { return }
                        self.value.append(NamedValue(id: v.id, name: v.name))
                        self.change(self.value)
                        self.field?.text = ""
                        self.results = []
                        self.renderChosen()
                        self.renderFound()
                    }
                })
            found.isHidden = shown.isEmpty
            FocusEngine.active(for: view)?.schedule()
        }

        /// Finds values by text, 300 ms after the last change.
        private func search(_ text: String) {
            task?.cancel()
            let query = text.trimmingCharacters(in: .whitespaces)
            guard !query.isEmpty, let model = page?.model else {
                results = []
                renderFound()
                return
            }
            let (serverId, catalog, filter, mediaKind) = (serverId, catalog, filter, mediaKind)
            task = Task { [weak self] in
                do {
                    try await Task.sleep(for: .milliseconds(300))
                    let values = try await model.filterValues(
                        serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, query: query)
                    guard let self, !Task.isCancelled else { return }
                    self.results = values
                } catch is CancellationError {
                    return
                } catch {
                    self?.results = []
                }
                self?.renderFound()
            }
        }
    }
#endif
