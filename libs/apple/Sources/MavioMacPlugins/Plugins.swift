#if os(macOS)
    import AppKit
    import MavioMac
    import Observation

    // The settings' Plugins (docs/clients.md §6.3.3), as the web UI's: the
    // plugins of the server chosen, as cards in two tabs, Available, what its
    // catalogs offer, installed or not, and Installed, filtered by category;
    // a plugin's page, to install, update, enable, disable and uninstall it,
    // with its configuration, containers, network, information, logs and
    // versions; and the catalogs, to add and remove.

    /// A plugin as shown: what its catalog offers of it and what is
    /// installed, either or both.
    struct PluginsEntry: Equatable {
        /// The plugin's ID.
        let id: String
        /// What its catalog offers of it.
        let offered: Mavio_System_V1_CatalogPlugin?
        /// The plugin installed.
        let installed: Mavio_System_V1_Plugin?

        private var manifest: Mavio_Plugin_V1_Manifest? { installed?.manifest }

        private static func first(_ values: String?...) -> String {
            values.compactMap(\.self).first { !$0.isEmpty } ?? ""
        }

        /// Its name.
        var name: String { Self.first(manifest?.name, offered?.name, id) }
        /// Its description.
        var description: String { Self.first(manifest?.description_p, offered?.description_p) }
        /// Its author.
        var author: String { Self.first(manifest?.author, offered?.author) }
        /// Its category.
        var category: Mavio_Plugin_V1_Category {
            if let c = manifest?.category, c != .unspecified { return c }
            return offered?.category ?? .unspecified
        }

        /// The newest version the catalog offers when newer than the one
        /// installed.
        var update: String? {
            guard let installed, let newest = offered?.versions.first?.version, !newest.isEmpty else { return nil }
            let current = installed.manifest.version
            return !current.isEmpty && newest != current ? newest : nil
        }
    }

    /// The plugins of a server, those its catalogs offer and those
    /// installed, and its catalogs, which the section's pages share and
    /// refresh after what is done to them. What is read again and has not
    /// changed is left as it was, so that the pages observing it are not
    /// drawn anew.
    @MainActor @Observable final class PluginsStore {
        /// The server.
        let server: ServerRecord
        /// The model calling it.
        let model: AppModel
        /// The plugins installed, nil until read.
        private(set) var installed: [Mavio_System_V1_Plugin]?
        /// Why they could not be read.
        private(set) var installedFailure: String?
        /// The plugins the catalogs offer, nil until read.
        private(set) var catalog: [Mavio_System_V1_CatalogPlugin]?
        /// Why they could not be read.
        private(set) var catalogFailure: String?
        /// The catalogs' URLs, nil until read.
        private(set) var catalogs: [String]?
        /// Why they could not be read.
        private(set) var catalogsFailure: String?

        /// The plugins of server, read with model.
        init(server: ServerRecord, model: AppModel) {
            self.server = server
            self.model = model
        }

        /// The plugins its catalogs offer, installed or not.
        var available: [PluginsEntry] {
            let byId = Dictionary((installed ?? []).map { ($0.manifest.id, $0) }, uniquingKeysWith: { a, _ in a })
            return (catalog ?? []).map { PluginsEntry(id: $0.id, offered: $0, installed: byId[$0.id]) }
        }

        /// The plugins installed.
        var installedEntries: [PluginsEntry] {
            let offered = Dictionary((catalog ?? []).map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
            return (installed ?? []).map {
                PluginsEntry(id: $0.manifest.id, offered: offered[$0.manifest.id], installed: $0)
            }
        }

        /// A plugin by its ID, nil when neither offered nor installed.
        func entry(_ id: String) -> PluginsEntry? {
            let offered = catalog?.first { $0.id == id }
            let installed = installed?.first { $0.manifest.id == id }
            return offered == nil && installed == nil
                ? nil : PluginsEntry(id: id, offered: offered, installed: installed)
        }

        /// Reads everything again.
        func refresh() async {
            async let a: Void = loadInstalled()
            async let b: Void = loadCatalog()
            async let c: Void = loadCatalogs()
            _ = await (a, b, c)
        }

        /// Reads the plugins installed.
        func loadInstalled() async {
            do {
                let list = try await model.call(
                    server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
                    as: Mavio_System_V1_ListPluginsResponse.self
                ).plugins
                if installed != list { installed = list }
                if installedFailure != nil { installedFailure = nil }
            } catch is CancellationError {
            } catch {
                installedFailure = error.reason
            }
        }

        /// Reads the plugins the catalogs offer.
        func loadCatalog() async {
            do {
                let list = try await model.call(
                    server, Procedure.plugins("ListCatalogPlugins"), Mavio_System_V1_ListCatalogPluginsRequest(),
                    as: Mavio_System_V1_ListCatalogPluginsResponse.self
                ).plugins
                if catalog != list { catalog = list }
                if catalogFailure != nil { catalogFailure = nil }
            } catch is CancellationError {
            } catch {
                catalogFailure = error.reason
            }
        }

        /// Reads the catalogs' URLs.
        func loadCatalogs() async {
            do {
                let list = try await model.call(
                    server, Procedure.plugins("GetPluginCatalogs"), Mavio_System_V1_GetPluginCatalogsRequest(),
                    as: Mavio_System_V1_GetPluginCatalogsResponse.self
                ).urls
                if catalogs != list { catalogs = list }
                if catalogsFailure != nil { catalogsFailure = nil }
            } catch is CancellationError {
            } catch {
                catalogsFailure = error.reason
            }
        }

        /// Replaces the catalogs' URLs.
        func setCatalogs(_ urls: [String]) async throws {
            var request = Mavio_System_V1_UpdatePluginCatalogsRequest()
            request.urls = urls
            _ = try await model.call(
                server, Procedure.plugins("UpdatePluginCatalogs"), request,
                as: Mavio_System_V1_UpdatePluginCatalogsResponse.self)
        }
    }

    /// A plugin category's title, icon and color.
    struct PluginsCategory {
        /// Its title.
        let title: String
        /// Its icon.
        let icon: String
        /// Its color.
        let color: NSColor

        /// The look of category c.
        @MainActor init(_ c: Mavio_Plugin_V1_Category) {
            switch c {
            case .metadata:
                (title, icon, color) = (MavioStrings.pluginsCategoryMetadata, "info", NSColor(hex: 0x007AFF))
            case .subtitles:
                (title, icon, color) = (MavioStrings.pluginsCategorySubtitles, "subtitles", NSColor(hex: 0x34C759))
            case .downloads:
                (title, icon, color) = (MavioStrings.pluginsCategoryDownloads, "downloads", NSColor(hex: 0xFF9500))
            case .authentication:
                (title, icon, color) = (MavioStrings.pluginsCategoryAuthentication, "key", NSColor(hex: 0xFF2D55))
            case .notifications:
                (title, icon, color) = (MavioStrings.pluginsCategoryNotifications, "pending", NSColor(hex: 0xFF3B30))
            case .devices:
                (title, icon, color) = (MavioStrings.pluginsCategoryDevices, "television", NSColor(hex: 0x32ADE6))
            case .ai: (title, icon, color) = (MavioStrings.pluginsCategoryAi, "plugins", NSColor(hex: 0xAF52DE))
            case .speech: (title, icon, color) = (MavioStrings.pluginsCategorySpeech, "plugins", NSColor(hex: 0x5856D6))
            default: (title, icon, color) = (MavioStrings.pluginsCategoryOther, "plugins", NSColor(hex: 0x8E8E93))
            }
        }
    }

    /// A plugin state's word.
    @MainActor func pluginsStateName(_ s: Mavio_System_V1_PluginState) -> String {
        switch s {
        case .ready: MavioStrings.pluginsStateReady
        case .unconfigured: MavioStrings.pluginsStateUnconfigured
        case .failed: MavioStrings.pluginsStateFailed
        case .disabled: MavioStrings.pluginsStateDisabled
        default: ""
        }
    }

    /// A category's icon on a tile of its color.
    @MainActor func categoryTile(_ category: Mavio_Plugin_V1_Category, size: CGFloat = 40) -> Box {
        let c = PluginsCategory(category)
        return colorTile(c.icon, color: c.color, size: size)
    }

    // MARK: The list

    /// The section of the server chosen in the settings: the plugins as
    /// cards in two tabs, filtered by category.
    final class PluginsPage: Page {
        private let server: ServerRecord
        private lazy var store = PluginsStore(server: server, model: model)
        private lazy var form = FormPage(
            title: MavioStrings.settingsPlugins, message: MavioStrings.settingsPluginsMessage, theme: theme)
        private var installedTab = false
        private var category: Mavio_Plugin_V1_Category?
        private var categories: [Mavio_Plugin_V1_Category] = []
        private var observer: Observer?
        private var loading: Task<Void, Never>?
        private var shownList: [PluginsEntry]?

        private lazy var tabs: NSView = {
            let titles = [MavioStrings.pluginsAvailable, MavioStrings.pluginsInstalled]
            if theme.isTV {
                let pills = PillTabs(titles, selected: 0, theme: theme)
                pills.onChange = { [weak self] i in self?.choose(installed: i == 1) }
                return pills
            }
            let control = NSSegmentedControl(
                labels: titles, trackingMode: .selectOne, target: self, action: #selector(tabChanged(_:)))
            control.selectedSegment = 0
            control.setAccessibilityLabel(MavioStrings.settingsPlugins)
            return control
        }()
        private lazy var categoryPopup: NSPopUpButton = {
            let popup = NSPopUpButton(frame: .zero, pullsDown: false)
            popup.target = self
            popup.action = #selector(categoryChanged(_:))
            popup.setAccessibilityLabel(MavioStrings.pluginsCategory)
            return popup
        }()
        private lazy var catalogsButton = MavioButton(
            MavioStrings.pluginsManageCatalogs, kind: .secondary, theme: theme
        ) { [weak self] in
            guard let self else { return }
            self.push(PluginCatalogsPage(context: self.context, store: self.store))
        }
        private lazy var catalogNote = plainNote("", theme.colors.danger, theme: theme)
        private lazy var grid = FlowGrid(minimum: 260, gutter: MavioSpacing.s16, rowSpacing: MavioSpacing.s16)
        private lazy var status = NSStackView.vertical([], spacing: 0, alignment: .centerX)

        /// The plugins of server.
        init(context: MacContext, server: ServerRecord) {
            self.server = server
            super.init(context: context)
        }

        isolated deinit {
            observer?.cancel()
            loading?.cancel()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsPlugins
            let controls = NSStackView.horizontal(
                [focusable(tabs, theme: theme), Spacer(), focusable(categoryPopup, theme: theme), catalogsButton],
                spacing: MavioSpacing.s12)
            for v in [controls, catalogNote, status, grid] as [NSView] { form.add(v) }
            show(form)
            observer = Observer { [weak self] in self?.render() }
            let store = store
            loading = Task {
                // The catalogs are fetched once; the plugins under
                // development again every few seconds, as the server
                // restarts them when they are built.
                async let catalog: Void = store.loadCatalog()
                await store.loadInstalled()
                while !Task.isCancelled {
                    try? await Task.sleep(for: .seconds(2))
                    if store.installed?.contains(where: { !$0.devFolder.isEmpty }) == true {
                        await store.loadInstalled()
                    }
                }
                await catalog
            }
        }

        @objc private func tabChanged(_ sender: NSSegmentedControl) {
            choose(installed: sender.selectedSegment == 1)
        }

        private func choose(installed: Bool) {
            installedTab = installed
            render()
        }

        @objc private func categoryChanged(_ sender: NSPopUpButton) {
            let i = sender.indexOfSelectedItem
            category = i <= 0 ? nil : categories[i - 1]
            render()
        }

        private func render() {
            let shown = installedTab ? store.installedEntries : store.available
            var present = Array(Set(shown.map(\.category))).sorted { $0.rawValue < $1.rawValue }
            if let category, !present.contains(category) { present.append(category) }
            if present != categories || categoryPopup.numberOfItems == 0 {
                categories = present
                categoryPopup.removeAllItems()
                categoryPopup.addItems(
                    withTitles: [MavioStrings.pluginsAllCategories] + present.map { PluginsCategory($0).title })
            }
            categoryPopup.selectItem(at: category.flatMap { categories.firstIndex(of: $0) }.map { $0 + 1 } ?? 0)
            let list = shown.filter { category == nil || $0.category == category }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            let loaded = installedTab ? store.installed != nil : store.catalog != nil
            let failure = installedTab ? store.installedFailure : store.catalogFailure
            if !installedTab, let reason = store.catalogFailure {
                (catalogNote.subviews.first as? NSTextField)?.stringValue = MavioStrings.pluginsCatalogsUnreachable(
                    reason: reason)
                catalogNote.isHidden = false
            } else {
                catalogNote.isHidden = true
            }
            if loaded && list.isEmpty {
                let text = NSTextField.label(
                    category != nil
                        ? MavioStrings.pluginsNoneInCategory
                        : installedTab ? MavioStrings.pluginsNoneInstalled : MavioStrings.pluginsNoneAvailable,
                    theme.fonts.body, color: theme.colors.textSecondary)
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addFilling(text, insets: .symmetric(vertical: 40))
                status.setViews([holder])
                status.isHidden = false
            } else if !loaded && failure == nil {
                status.setViews([LoadingView()])
                status.isHidden = false
            } else {
                status.setViews([])
                status.isHidden = true
            }
            // The cards are made anew only as what they show changes.
            if list != shownList {
                shownList = list
                grid.setItems(list.map(card))
            }
            grid.isHidden = list.isEmpty
            FocusEngine.active(for: view)?.schedule()
        }

        /// A plugin's card: its tile, name, category and version, description
        /// and state.
        private func card(_ e: PluginsEntry) -> NSView {
            let version = e.installed?.manifest.version ?? ""
            let name = NSTextField.label(e.name, theme.fonts.headline, color: theme.colors.textPrimary)
            let sub = NSTextField.label(
                [PluginsCategory(e.category).title, version].filter { !$0.isEmpty }.joined(separator: " · "),
                theme.fonts.footnote, color: theme.colors.textSecondary)
            let titles = NSStackView.vertical([name, sub], spacing: 0)
            let top = NSStackView.horizontal([categoryTile(e.category), titles], spacing: MavioSpacing.s12)
            let description = NSTextField.label(
                e.description, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 3)
            description.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            let column = NSStackView.vertical([top, description, Spacer(), badge(e)], spacing: MavioSpacing.s12)
            description.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            let content = NSView()
            content.translatesAutoresizingMaskIntoConstraints = false
            content.addFilling(column, insets: .all(MavioSpacing.s16))
            content.heightAnchor.constraint(greaterThanOrEqualToConstant: 160).isActive = true
            let card = Pressable(content, look: .lift, radius: 14, theme: theme) { [weak self] in
                guard let self else { return }
                self.push(PluginPage(context: self.context, store: self.store, id: e.id))
            }
            card.restingFill = theme.colors.surface
            card.setAccessibilityLabel(e.name)
            return card
        }

        /// The plugin's state as a short mark.
        private func badge(_ e: PluginsEntry) -> NSView {
            func mark(_ text: String, _ color: NSColor) -> NSTextField {
                let label = NSTextField.label(text, theme.fonts.caption, color: color)
                label.font = .systemFont(ofSize: theme.fonts.caption.size, weight: .semibold)
                return label
            }
            if let installed = e.installed {
                let state = installed.state
                var marks: [NSView] = [
                    installed.devFolder.isEmpty
                        ? mark(MavioStrings.pluginsInstalled, theme.colors.textSecondary)
                        : mark(MavioStrings.pluginsDevelopment, theme.colors.accentText),
                    mark(
                        pluginsStateName(state),
                        state == .failed
                            ? theme.colors.danger
                            : state == .ready ? NSColor(hex: 0x34C759) : theme.colors.textSecondary),
                ]
                if e.update != nil { marks.append(mark(MavioStrings.pluginsUpdateAvailable, theme.colors.textPrimary)) }
                return NSStackView.horizontal(marks, spacing: MavioSpacing.s8)
            } else if let v = e.offered?.versions.first?.version, !v.isEmpty {
                return mark(v, theme.colors.textSecondary)
            } else {
                return mark(MavioStrings.pluginsNoVersion, theme.colors.textTertiary)
            }
        }
    }

    // MARK: Catalogs

    /// The catalogs' URLs, to add to and remove from.
    final class PluginCatalogsPage: Page {
        private let store: PluginsStore
        private lazy var form = FormPage(
            title: MavioStrings.pluginsCatalogs, message: MavioStrings.pluginsCatalogsMessage, theme: theme)
        private lazy var section = LiveSection(theme: theme)
        private lazy var field = InputField(placeholder: MavioStrings.pluginsCatalogAddress)
        private lazy var addButton = MavioButton(MavioStrings.pluginsAddCatalog, kind: .secondary, theme: theme) {
            [weak self] in self?.add()
        }
        private lazy var addRow: ContentRow = ContentRow(
            bar([focusable(field, theme: theme), addButton]), theme: theme)
        private var saving = false
        private var failure: String?
        private var observer: Observer?
        private var tasks: [Task<Void, Never>] = []
        private var rows: [String: NSView] = [:]

        /// The catalogs of store's server.
        init(context: MacContext, store: PluginsStore) {
            self.store = store
            super.init(context: context)
        }

        isolated deinit {
            observer?.cancel()
            tasks.forEach { $0.cancel() }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.pluginsCatalogs
            field.setAccessibilityLabel(MavioStrings.pluginsCatalogAddress)
            field.onChange = { [weak self] _ in self?.render() }
            field.onSubmit = { [weak self] in self?.add() }
            form.add(section)
            show(form)
            observer = Observer { [weak self] in self?.render() }
            let store = store
            tasks.append(Task { await store.loadCatalogs() })
        }

        private func render() {
            let list = store.catalogs ?? []
            var views: [NSView] = []
            var kept: [String: NSView] = [:]
            for c in list {
                let row =
                    rows[c]
                    ?? {
                        let text = NSTextField.label(
                            c, theme.fonts.body, color: theme.colors.textPrimary, selectable: true)
                        text.setContentCompressionResistancePriority(.init(1), for: .horizontal)
                        let remove = MavioButton(MavioStrings.pluginsRemoveCatalog, kind: .destructive, theme: theme) {
                            [weak self] in
                            guard let self else { return }
                            self.save((self.store.catalogs ?? []).filter { $0 != c })
                        }
                        return ContentRow(bar([text, Spacer(), remove]), theme: theme)
                    }()
                kept[c] = row
                views.append(row)
            }
            rows = kept
            for row in kept.values {
                for case let b as MavioButton in row.subviews.first?.subviews ?? [] { b.isEnabled = !saving }
            }
            views.append(addRow)
            addButton.isEnabled = !field.text.trimmingCharacters(in: .whitespaces).isEmpty && !saving
            section.setRows(views)
            section.footer = failure ?? store.catalogsFailure
            FocusEngine.active(for: view)?.schedule()
        }

        private func add() {
            let list = store.catalogs ?? []
            let u = field.text.trimmingCharacters(in: .whitespaces)
            guard !u.isEmpty, !list.contains(u), !saving else { return }
            save(list + [u]) { [weak self] in self?.field.text = "" }
        }

        private func save(_ urls: [String], then done: @escaping @MainActor () -> Void = {}) {
            saving = true
            render()
            let store = store
            tasks.append(
                Task { [weak self] in
                    var failure: String?
                    do {
                        try await store.setCatalogs(urls)
                    } catch {
                        failure = error.reason
                    }
                    if failure == nil { done() }
                    self?.failure = failure
                    await store.refresh()
                    self?.saving = false
                    self?.render()
                })
        }
    }
#endif
