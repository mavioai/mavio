#if os(macOS)
    import AppKit
    import MavioMac

    /// A page of the settings.
    enum SettingsPage: Hashable {
        case account, appearance, playback, subtitlesAudio, assistant
        case server(ServerSection)
        case about

        /// The page's title.
        var title: String {
            switch self {
            case .account: MavioStrings.settingsAccount
            case .appearance: MavioStrings.settingsAppearance
            case .playback: MavioStrings.settingsPlayback
            case .subtitlesAudio: MavioStrings.settingsSubtitlesAudio
            case .assistant: MavioStrings.assistantTitle
            case .server(let section): section.title
            case .about: MavioStrings.settingsAbout
            }
        }

        /// The icon on the page's tile.
        var icon: String {
            switch self {
            case .account: "profile"
            case .appearance: "appearance"
            case .playback: "play"
            case .subtitlesAudio: "subtitles"
            case .assistant: "microphone"
            case .server(let section): section.icon
            case .about: "info"
            }
        }

        /// The color of the page's tile, as the web UI's.
        var tint: NSColor {
            switch self {
            case .account: NSColor(hex: 0x0A84FF)
            case .appearance: NSColor(hex: 0x5E5CE6)
            case .playback: NSColor(hex: 0xFF3B30)
            case .subtitlesAudio: NSColor(hex: 0x34C759)
            case .assistant: NSColor(hex: 0xFF2D55)
            case .server(let section): NSColor(hex: section.tint)
            case .about: NSColor(hex: 0x8E8E93)
            }
        }
    }

    /// What the settings show of the servers signed in to: the server
    /// chosen, whether its user administers it, and the groups of pages
    /// that follow; shared by the desktop's sidebar and television mode's
    /// list.
    @MainActor
    final class SettingsState {
        private let model: AppModel
        /// The server chosen, by its ID; empty for the first signed in to.
        var serverId = ""
        /// Whether a server's user administers it, once known, by the
        /// server and user: switching users asks again.
        private var admin: [String: Bool] = [:]
        private var loading: Task<Void, Never>?
        private var loadingKey = ""
        /// Called when what the user administers becomes known.
        var onAdmin: (@MainActor () -> Void)?

        /// The state of model's servers.
        init(model: AppModel) { self.model = model }

        isolated deinit { loading?.cancel() }

        /// The server whose sections the Server group shows.
        var chosen: ServerRecord? {
            model.signedInServers.first { $0.id == serverId } ?? model.signedInServers.first
        }

        /// The chosen server and its user.
        var key: String { chosen.map { "\($0.id)/\($0.userId)" } ?? "" }

        /// Whether the user administers the chosen server.
        var isAdmin: Bool { admin[key] == true }

        /// The pages of this device: Subtitles and Audio and the assistant
        /// need a server.
        var devicePages: [SettingsPage] {
            [.appearance, .playback] + (model.signedInServers.isEmpty ? [] : [.subtitlesAudio, .assistant])
        }

        /// The advanced tools, for the server's administrators.
        var advancedPages: [SettingsPage] {
            isAdmin ? [.server(.indexerSearch)] : []
        }

        /// The Server group's entries the user sees.
        var serverEntries: [ServerEntry] {
            chosen == nil ? [] : ServerEntry.shown(admin: isAdmin)
        }

        /// The Server group's sections, its menus' in their place, one
        /// level.
        var serverPages: [SettingsPage] {
            serverEntries.flatMap { entry -> [SettingsPage] in
                switch entry {
                case .section(let section): [.server(section)]
                case .menu(let menu): menu.sections(admin: true).map { .server($0) }
                }
            }
        }

        /// Whether the user sees page.
        func shows(_ page: SettingsPage) -> Bool {
            guard case .server(let section) = page else { return true }
            return chosen != nil && (section.forAnyone || isAdmin)
        }

        /// A Mavio account chooses among its servers; a standalone user is
        /// signed in to one.
        var choosesServer: Bool { model.account != nil && model.signedInServers.count > 1 }

        /// Asks whether the user administers the chosen server, once per
        /// server and user.
        func loadAdmin() {
            let key = key
            guard let server = chosen, key != loadingKey, admin[key] == nil else { return }
            loadingKey = key
            loading?.cancel()
            let model = model
            loading = Task { [weak self] in
                let known = await model.administers(server)
                guard let self, !Task.isCancelled else { return }
                loadingKey = ""
                if let known {
                    admin[key] = known
                    onAdmin?()
                }
            }
        }

        /// The page of page, of the chosen server for a server's section.
        func controller(_ page: SettingsPage, context: MacContext) -> NSViewController {
            switch page {
            case .account: AccountPage(context: context)
            case .appearance: AppearancePage(context: context)
            case .playback: PlaybackPage(context: context)
            case .subtitlesAudio: SubtitlesAudioPage(context: context)
            case .assistant: context.assistantPage(chosen, context)
            case .about: AboutPage(context: context)
            case .server(let section):
                if let server = chosen {
                    context.sectionPage(section, server, context)
                } else {
                    PendingPage(context: context, name: section.title)
                }
            }
        }
    }

    // MARK: Desktop

    /// The settings as the web UI's in a wide window (docs/clients.md §6.3):
    /// beside their own sidebar, which returns Home and lists the account,
    /// this device's pages and the sections of the server chosen, its
    /// menus' in their place; the page chosen in its own stack, so that the
    /// toolbar's back returns from what it opens. Escape returns Home.
    final class SettingsSplitController: NSSplitViewController {
        private let context: MacContext
        private let close: @MainActor () -> Void
        private let state: SettingsState
        private let sidebar: SettingsSidebar
        private let detail: SettingsDetail
        private var page: SettingsPage
        private var observer: Observer?
        private var shownKey = ""

        /// The settings in context, on the account's page when account;
        /// close returns Home.
        init(context: MacContext, account: Bool, close: @escaping @MainActor () -> Void) {
            self.context = context
            self.close = close
            state = SettingsState(model: context.model)
            page = account ? .account : .appearance
            sidebar = SettingsSidebar(context: context, state: state, close: close)
            detail = SettingsDetail(theme: context.theme)
            super.init(nibName: nil, bundle: nil)
            sidebar.onSelect = { [weak self] in self?.select($0) }
            state.onAdmin = { [weak self] in self?.refresh() }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { observer?.cancel() }

        override func viewDidLoad() {
            super.viewDidLoad()
            let side = NSSplitViewItem(sidebarWithViewController: sidebar)
            side.minimumThickness = 220
            side.maximumThickness = 400
            side.preferredThicknessFraction = 0.22
            side.canCollapse = true
            addSplitViewItem(side)
            let content = NSSplitViewItem(viewController: detail)
            content.titlebarSeparatorStyle = .none
            addSplitViewItem(content)
            splitView.autosaveName = "SettingsSidebar"
            observer = Observer { [weak self] in self?.refresh() }
        }

        /// Follows the servers signed in to: the sidebar's rows, whether the
        /// user administers the server, and the page shown.
        private func refresh() {
            let model = context.model
            _ = (model.account?.id, model.signedInServers.map(\.userId))
            state.loadAdmin()
            // A page the user no longer sees gives way to the appearance's.
            if !state.shows(page) { page = .appearance }
            sidebar.reload(selected: page)
            let key = "\(page)|\(state.key)"
            if key != shownKey { showPage() }
        }

        private func select(_ next: SettingsPage) {
            guard next != page else {
                detail.popToRoot()
                return
            }
            page = next
            showPage()
        }

        /// Shows the page chosen anew, as another server's section too.
        private func showPage() {
            shownKey = "\(page)|\(state.key)"
            let page = page
            detail.show(shownKey) { [state, context] in state.controller(page, context: context) }
        }

        override func cancelOperation(_ sender: Any?) {
            close()
        }
    }

    /// The settings' detail: the page chosen in a stack of its own, under
    /// the toolbar. A page opened before is kept with what it read and
    /// shows again at once, reading anew as it appears, so that moving
    /// between the settings does not load each page from nothing.
    private final class SettingsDetail: NSViewController {
        private let theme: Theme
        private var stacks: [String: NavigationController] = [:]
        private var stack: NavigationController?

        init(theme: Theme) {
            self.theme = theme
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let box = Box(fill: theme.colors.background)
            box.frame = NSRect(x: 0, y: 0, width: 900, height: 700)
            view = box
        }

        /// Shows the page kept under key, or the one make makes, in a stack
        /// of its own.
        func show(_ key: String, make: () -> NSViewController) {
            stack?.view.removeFromSuperview()
            let next: NavigationController
            if let kept = stacks[key] {
                next = kept
            } else {
                next = NavigationController(root: make())
                stacks[key] = next
                addChild(next)
                next.view.translatesAutoresizingMaskIntoConstraints = false
            }
            stack = next
            view.addSubview(next.view)
            NSLayoutConstraint.activate([
                next.view.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
                next.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                next.view.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                next.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            ])
            NotificationCenter.default.post(name: NavigationController.didChange, object: next)
        }

        /// Returns the page shown to its first.
        func popToRoot() { stack?.popToRoot() }
    }

    /// The settings' sidebar: Home and the title at its top, then the
    /// account, the advanced tools, this device's pages and the server's
    /// sections, flat, with room between them.
    private final class SettingsSidebar: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
        private enum Row {
            case account
            case header(String)
            case page(SettingsPage)
            case picker
        }

        private let context: MacContext
        private let state: SettingsState
        private let close: @MainActor () -> Void
        private let table = NSTableView()
        private var rows: [Row] = []
        private var selected: SettingsPage = .appearance
        private var updating = false
        var onSelect: (@MainActor (SettingsPage) -> Void)?

        init(context: MacContext, state: SettingsState, close: @escaping @MainActor () -> Void) {
            self.context = context
            self.state = state
            self.close = close
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let theme = context.theme
            let view = NSView(frame: NSRect(x: 0, y: 0, width: 260, height: 700))
            let column = NSTableColumn(identifier: .init("page"))
            table.addTableColumn(column)
            table.headerView = nil
            table.style = .sourceList
            table.floatsGroupRows = false
            table.intercellSpacing = NSSize(width: 0, height: 4)
            table.dataSource = self
            table.delegate = self
            table.backgroundColor = .clear
            let scroll = NSScrollView()
            scroll.documentView = table
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.translatesAutoresizingMaskIntoConstraints = false
            let home = MavioButton(MavioStrings.navHome, icon: "back", kind: .link, theme: theme) { [weak self] in
                self?.close()
            }
            home.toolTip = MavioStrings.navHome
            let heading = NSTextField.label(
                MavioStrings.navSettings, theme.fonts.title1, color: theme.colors.textPrimary)
            let top = NSStackView.vertical([home, heading], spacing: MavioSpacing.s8)
            view.addSubview(top)
            view.addSubview(scroll)
            NSLayoutConstraint.activate([
                top.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: MavioSpacing.s4),
                top.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: MavioSpacing.s24),
                top.trailingAnchor.constraint(lessThanOrEqualTo: view.trailingAnchor, constant: -MavioSpacing.s16),
                scroll.topAnchor.constraint(equalTo: top.bottomAnchor, constant: MavioSpacing.s12),
                scroll.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                scroll.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                scroll.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            ])
            self.view = view
        }

        /// Lists the rows anew, page selected.
        func reload(selected page: SettingsPage) {
            selected = page
            var next: [Row] = [.account]
            let advanced = state.advancedPages
            if !advanced.isEmpty {
                next.append(.header(MavioStrings.settingsAdvancedGroup))
                next.append(contentsOf: advanced.map { .page($0) })
            }
            next.append(.header(MavioStrings.settingsThisDevice))
            next.append(contentsOf: state.devicePages.map { .page($0) })
            if state.chosen != nil {
                next.append(.header(MavioStrings.settingsServerGroup))
                if state.choosesServer { next.append(.picker) }
                next.append(contentsOf: state.serverPages.map { .page($0) })
            }
            // About ends the settings, after room as a group's.
            next.append(.header(""))
            next.append(.page(.about))
            rows = next
            updating = true
            table.reloadData()
            let index = rows.firstIndex {
                switch $0 {
                case .account: page == .account
                case .page(let p): p == page
                default: false
                }
            }
            if let index {
                table.selectRowIndexes([index], byExtendingSelection: false)
            } else {
                table.deselectAll(nil)
            }
            updating = false
        }

        func numberOfRows(in tableView: NSTableView) -> Int { rows.count }

        func tableView(_ tableView: NSTableView, isGroupRow row: Int) -> Bool {
            if case .header = rows[row] { true } else { false }
        }

        func tableView(_ tableView: NSTableView, shouldSelectRow row: Int) -> Bool {
            switch rows[row] {
            case .account, .page: true
            case .header, .picker: false
            }
        }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            let theme = context.theme
            switch rows[row] {
            case .header(let title):
                let label = NSTextField.label(title, theme.fonts.caption, color: .secondaryLabelColor)
                return cell(label, leading: MavioSpacing.s8)
            case .account:
                let model = context.model
                let server = model.signedInServers.first
                let name = model.account?.name ?? server?.userName ?? ""
                let line = model.account != nil ? MavioStrings.authMavioAccount : server?.name ?? ""
                let labels = NSStackView.vertical(
                    [
                        NSTextField.label(name, theme.fonts.headline, color: .labelColor),
                        NSTextField.label(line, theme.fonts.footnote, color: .secondaryLabelColor),
                    ], spacing: 2)
                let stack = NSStackView.horizontal(
                    [AvatarView(name: name, size: 44, theme: theme), labels], spacing: MavioSpacing.s12)
                return cell(stack, leading: MavioSpacing.s8)
            case .page(let page):
                let tile = Box(fill: page.tint, radius: 7)
                tile.size(width: 28, height: 28)
                let icon = IconView(page.icon, fill: true, size: 17, color: .white)
                tile.addSubview(icon)
                icon.center(in: tile)
                let label = NSTextField.label(page.title, theme.fonts.body, color: .labelColor)
                return cell(NSStackView.horizontal([tile, label], spacing: MavioSpacing.s12), leading: MavioSpacing.s8)
            case .picker:
                let servers = context.model.signedInServers
                let chosen = state.chosen?.id
                let popup = ActionPopUp(
                    options: servers.map(\.name), selected: servers.firstIndex { $0.id == chosen } ?? 0
                ) { [weak self] index in
                    guard let self, servers.indices.contains(index) else { return }
                    state.serverId = servers[index].id
                    // The observer follows the model, not the choice.
                    (parent as? SettingsSplitController)?.serverChanged()
                }
                popup.setAccessibilityLabel(MavioStrings.authServer)
                let cell = cell(popup, leading: MavioSpacing.s8)
                popup.trailingAnchor.constraint(equalTo: cell.trailingAnchor, constant: -MavioSpacing.s8).isActive =
                    true
                return cell
            }
        }

        private func cell(_ content: NSView, leading: CGFloat) -> NSTableCellView {
            let cell = NSTableCellView()
            content.translatesAutoresizingMaskIntoConstraints = false
            cell.addSubview(content)
            NSLayoutConstraint.activate([
                content.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: leading),
                content.trailingAnchor.constraint(lessThanOrEqualTo: cell.trailingAnchor),
                content.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
            ])
            return cell
        }

        func tableViewSelectionDidChange(_ notification: Notification) {
            guard !updating else { return }
            let row = table.selectedRow
            guard rows.indices.contains(row) else { return }
            switch rows[row] {
            case .account: onSelect?(.account)
            case .page(let page): onSelect?(page)
            default: break
            }
        }

        func tableView(_ tableView: NSTableView, heightOfRow row: Int) -> CGFloat {
            switch rows[row] {
            case .header: 30
            case .account: 60
            case .page: 34
            case .picker: 32
            }
        }
    }

    extension SettingsSplitController {
        /// Follows another server chosen in the sidebar.
        fileprivate func serverChanged() { refresh() }
    }

    // MARK: Television

    /// The settings in television mode (docs/tv.md §6.6): the list of
    /// their sections, with no sidebar — the account, the users to switch
    /// to, the sections and menus by group — a section or a menu's list
    /// opening in its place. Back returns to the list with focus on the
    /// entry that opened it, and from the list to where the settings were
    /// opened from.
    final class SettingsListPage: Page {
        private let state: SettingsState
        private var observer: Observer?
        private var form: FormPage?
        private var switchUsers: SwitchUsers?
        private var loading: Task<Void, Never>?
        private var usersKey = ""
        private var opened: (title: String, row: FormRow?)?

        /// The list in context.
        override init(context: MacContext) {
            state = SettingsState(model: context.model)
            super.init(context: context)
            state.onAdmin = { [weak self] in self?.render() }
        }

        isolated deinit {
            observer?.cancel()
            loading?.cancel()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navSettings
            observer = Observer { [weak self] in self?.render() }
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            // Back returns focus to the entry that opened the page.
            if let row = opened?.row { FocusEngine.active(for: view)?.prefer(row) }
        }

        private func render() {
            let model = model
            let standalone = model.account == nil ? model.signedInServers.first : nil
            state.loadAdmin()
            let form = FormPage(title: MavioStrings.navSettings, theme: theme)
            self.form = form
            var sections: [NSView] = []
            let server = model.signedInServers.first
            let name = model.account?.name ?? server?.userName ?? ""
            sections.append(
                FormSection(
                    rows: [
                        link(
                            name, subtitle: model.account != nil ? MavioStrings.authMavioAccount : server?.name ?? "",
                            icon: "profile", tint: SettingsPage.account.tint
                        ) { [weak self] in
                            guard let self else { return nil }
                            return AccountPage(context: context)
                        }
                    ], theme: theme))
            if let standalone {
                let users = switchUsers ?? SwitchUsers(page: self, server: standalone)
                switchUsers = users
                sections.append(users.section)
                let key = "\(standalone.id)/\(standalone.userId)"
                if key != usersKey {
                    usersKey = key
                    loading?.cancel()
                    loading = Task { [weak self] in
                        let list = await model.switchableUsers(standalone)
                        guard !Task.isCancelled else { return }
                        self?.switchUsers?.set(list, server: standalone)
                    }
                }
            } else {
                switchUsers = nil
                usersKey = ""
            }
            let advanced = state.advancedPages
            if !advanced.isEmpty {
                sections.append(
                    FormSection(MavioStrings.settingsAdvancedGroup, rows: advanced.map(pageRow), theme: theme))
            }
            sections.append(
                FormSection(MavioStrings.settingsThisDevice, rows: state.devicePages.map(pageRow), theme: theme))
            if let chosen = state.chosen {
                var rows: [NSView] = []
                if state.choosesServer {
                    let servers = model.signedInServers
                    rows.append(
                        FormRow.choice(
                            MavioStrings.authServer, options: servers.map { ($0.name, $0.id) }, selected: chosen.id,
                            theme: theme
                        ) { [weak self] id in
                            self?.state.serverId = id
                            self?.render()
                        })
                }
                for entry in state.serverEntries {
                    switch entry {
                    case .section(let section):
                        rows.append(pageRow(.server(section)))
                    case .menu(let menu):
                        rows.append(
                            link(menu.title, icon: menu.icon, tint: NSColor(hex: menu.tint)) { [weak self] in
                                guard let self else { return nil }
                                return MenuListPage(context: context, menu: menu, state: state)
                            })
                    }
                }
                sections.append(FormSection(MavioStrings.settingsServerGroup, rows: rows, theme: theme))
            }
            sections.append(FormSection(rows: [pageRow(.about)], theme: theme))
            form.setSections(sections)
            show(form)
            FocusEngine.active(for: view)?.schedule()
        }

        private func pageRow(_ page: SettingsPage) -> NSView {
            link(page.title, icon: page.icon, tint: page.tint) { [weak self] in
                guard let self else { return nil }
                return state.controller(page, context: context)
            }
        }

        /// A row opening the page make makes, remembered as the one focus
        /// returns to.
        private func link(
            _ title: String, subtitle: String? = nil, icon: String, tint: NSColor,
            make: @escaping @MainActor () -> NSViewController?
        ) -> FormRow {
            let row = FormRow.link(title, subtitle: subtitle, icon: icon, tint: tint, theme: theme) {}
            row.onPress = { [weak self, weak row] in
                guard let self, let page = make() else { return }
                opened = (title, row)
                push(page)
            }
            if opened?.title == title { opened = (title, row) }
            return row
        }
    }

    /// A menu's list in television mode: its sections, opening in its
    /// place, as the settings' list opens a section.
    private final class MenuListPage: Page {
        private let serverMenu: ServerMenu
        private let state: SettingsState
        private weak var opened: FormRow?

        init(context: MacContext, menu: ServerMenu, state: SettingsState) {
            self.serverMenu = menu
            self.state = state
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = serverMenu.title
            let form = FormPage(title: serverMenu.title, theme: theme)
            // Only an administrator sees a menu.
            let rows = serverMenu.sections(admin: true).map { section -> NSView in
                let page = SettingsPage.server(section)
                let row = FormRow.link(page.title, icon: page.icon, tint: page.tint, theme: theme) {}
                row.onPress = { [weak self, weak row] in
                    guard let self else { return }
                    opened = row
                    push(state.controller(page, context: context))
                }
                return row
            }
            form.add(FormSection(rows: rows, theme: theme))
            show(form)
            FocusEngine.active(for: view)?.schedule()
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            if let opened { FocusEngine.active(for: view)?.prefer(opened) }
        }
    }
#endif
