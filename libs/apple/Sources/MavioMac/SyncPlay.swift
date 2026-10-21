#if os(macOS)
    import AppKit
    import MavioAppKit
    import MavioCommon
    import MavioKit

    /// Watch Together (docs/clients.md §6.3.24), as the web UI's dialog: the
    /// device's group — its state, members, and watching what it plays,
    /// playing the item in it or leaving it — or a server's groups to join
    /// and a new one.
    @MainActor
    public final class SyncPlayDialog {
        private let dialog: Dialog
        private let context: MacContext
        private let server: ServerRecord
        private let item: (id: String, name: String, position: TimeInterval?)?
        private let watching: Bool
        private let name: TextInput
        private let failure: NSTextField
        private var groups: [Mavio_Session_V1_SyncPlayGroup]?
        private var entryName = "…"
        private var entryId: String?
        private var busy = false
        private var observer: Observer?
        private var tasks: [Task<Void, Never>] = []
        private var retained: SyncPlayDialog?

        /// The dialog of a server's groups in context; item, when given,
        /// plays in the group, from position when partly watched; a player
        /// showing what the group plays is watching, which needs no Watch.
        public init(
            context: MacContext, server: ServerRecord,
            item: (id: String, name: String, position: TimeInterval?)? = nil, watching: Bool = false
        ) {
            self.context = context
            self.server = server
            self.item = item
            self.watching = watching
            dialog = Dialog(title: MavioStrings.syncplayTitle, width: 520, theme: context.theme)
            name = TextInput(MavioStrings.syncplayDefaultName(name: server.userName))
            failure = .label("", context.theme.fonts.footnote, color: context.theme.colors.danger, lines: 0)
            failure.isHidden = true
            name.onChange = { [weak self] _ in self?.render() }
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        private var theme: Theme { context.theme }
        private var model: AppModel { context.model }

        /// Shows the dialog over page.
        public func present(over page: NSViewController) {
            retained = self
            dialog.onClose = { [weak self] in
                self?.observer?.cancel()
                self?.retained = nil
            }
            dialog.present(over: page)
            observer = Observer { [weak self] in
                guard let self else { return }
                _ = self.model.syncPlay.joined
                self.render()
            }
            loadGroups()
        }

        private func loadGroups() {
            let (model, serverId) = (model, server.id)
            tasks.append(
                Task { [weak self] in
                    var failed: String?
                    let groups: [Mavio_Session_V1_SyncPlayGroup]
                    do {
                        groups = try await model.syncPlayGroups(serverId)
                    } catch {
                        groups = []
                        failed = error.reason
                    }
                    guard let self, !Task.isCancelled else { return }
                    self.groups = groups
                    self.render()
                    if let failed { self.fail(failed) }
                })
        }

        private func fail(_ message: String?) {
            failure.stringValue = message ?? ""
            failure.isHidden = message == nil
        }

        private func run(_ action: @escaping @MainActor () async throws -> Void) {
            busy = true
            fail(nil)
            render()
            tasks.append(
                Task { [weak self] in
                    do {
                        try await action()
                    } catch {
                        self?.fail(error.reason)
                    }
                    self?.busy = false
                    self?.render()
                })
        }

        private func render() {
            let done = MavioButton(
                model.syncPlay.joined == nil ? MavioStrings.actionCancel : MavioStrings.actionDone,
                kind: model.syncPlay.joined == nil ? .secondary : .primary, theme: theme
            ) { [weak self] in self?.dialog.close() }
            if let joined = model.syncPlay.joined {
                dialog.title = joined.group.name
                dialog.setContent(group(joined))
                var leading: [NSView] = []
                let leave = MavioButton(MavioStrings.syncplayLeave, kind: .destructive, theme: theme) {
                    [weak self] in
                    guard let self else { return }
                    let syncPlay = self.model.syncPlay
                    self.run {
                        try await syncPlay.leave()
                        self.dialog.close()
                    }
                }
                leave.isEnabled = !busy
                leading.append(leave)
                dialog.setButtons(leading: leading, trailing: [done])
            } else {
                dialog.title = MavioStrings.syncplayTitle
                dialog.setContent(groupsList())
                let create = MavioButton(MavioStrings.syncplayCreate, kind: .primary, theme: theme) { [weak self] in
                    guard let self else { return }
                    let name = String(self.name.text.trimmingCharacters(in: .whitespaces).prefix(200))
                    let (model, serverId) = (self.model, self.server.id)
                    self.run { try await model.createSyncPlayGroup(serverId, name: name) }
                }
                create.isEnabled = !busy && !name.text.trimmingCharacters(in: .whitespaces).isEmpty
                dialog.setButtons(trailing: [done, create])
            }
        }

        private func group(_ joined: SyncPlay.Joined) -> [NSView] {
            let group = joined.group
            let entry = group.entry
            if let entry, entry.itemID != entryId {
                entryId = entry.itemID
                let (model, serverId) = (model, joined.serverId)
                tasks.append(
                    Task { [weak self] in
                        guard let d = try? await model.client.details(serverId: serverId, itemId: entry.itemID),
                            let self, !Task.isCancelled
                        else { return }
                        self.entryName = d.card.title
                        self.render()
                    })
            }
            let me = model.syncPlay.sessionId(joined.serverId)
            let members = group.members.map { m -> NSView in
                let detail = [m.deviceName, m.sessionID == me ? MavioStrings.settingsThisDevice : ""]
                    .filter { !$0.isEmpty }.joined(separator: " · ")
                var controls: [NSView] = []
                if m.buffering {
                    controls.append(
                        NSTextField.label(
                            MavioStrings.syncplayLoading, theme.fonts.footnote, color: theme.colors.textSecondary))
                }
                return ControlRow(
                    m.userName, subtitle: detail.isEmpty ? nil : detail, icon: "devices",
                    tint: theme.colors.fill, controls: controls, theme: theme)
            }
            var actions: [NSView] = []
            if let entry, group.state != .idle, !watching {
                actions.append(
                    FormRow.action(MavioStrings.syncplayWatch, theme: theme) { [weak self] in
                        guard let self else { return }
                        let request = PlayRequest(
                            serverId: joined.serverId, itemId: entry.itemID, title: self.entryName,
                            start: group.position(at: self.model.syncPlay.serverNow()))
                        self.dialog.close()
                        self.context.play(request)
                    })
            }
            if let item, joined.serverId == server.id {
                actions.append(
                    FormRow.action(MavioStrings.syncplayPlayHere(name: item.name), theme: theme) { [weak self] in
                        guard let self, !self.busy else { return }
                        let (model, serverId) = (self.model, self.server.id)
                        self.run {
                            try await model.playInGroup(serverId, itemId: item.id, from: item.position ?? 0)
                            self.dialog.close()
                        }
                    })
            }
            var views: [NSView] = [
                NSTextField.label(
                    groupState(group, entryName: entryName), theme.fonts.callout, color: theme.colors.textSecondary,
                    lines: 0),
                FormSection(MavioStrings.syncplayMembers, rows: members, theme: theme),
            ]
            if !actions.isEmpty { views.append(FormSection(rows: actions, theme: theme)) }
            views.append(failure)
            return views
        }

        private func groupsList() -> [NSView] {
            var rows: [NSView] = []
            if let groups {
                if groups.isEmpty {
                    rows.append(
                        ControlRow(
                            NSTextField.label(
                                MavioStrings.syncplayNoGroups, theme.fonts.body, color: theme.colors.textSecondary),
                            theme: theme))
                }
                for g in groups {
                    let join = MavioButton(MavioStrings.syncplayJoin, kind: .secondary, theme: theme) {
                        [weak self] in
                        guard let self else { return }
                        let (model, serverId) = (self.model, self.server.id)
                        self.run { try await model.joinSyncPlayGroup(serverId, groupId: g.id) }
                    }
                    join.isEnabled = !busy
                    rows.append(
                        ControlRow(
                            g.name, subtitle: g.members.map(\.userName).joined(separator: ", "), icon: "users",
                            tint: theme.colors.fill, controls: [join], theme: theme))
                }
            } else {
                rows.append(ControlRow(LoadingView(), theme: theme))
            }
            return [
                NSTextField.label(
                    MavioStrings.syncplayIntro, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0),
                FormSection(rows: rows, theme: theme),
                FormSection(
                    rows: [
                        ControlRow(
                            MavioStrings.syncplayGroupName,
                            controls: [FocusFrame(name, theme: theme).size(width: 240)], theme: theme)
                    ], theme: theme),
                failure,
            ]
        }
    }

    /// The device's Watch Together group, a pill naming it and its size
    /// while the device is in one, which opens its dialog: floating at the
    /// bottom right of the pages on the desktop, in the navigation bar in
    /// television mode, as the web UI's.
    @MainActor
    public final class GroupIndicator: NSView {
        private let context: MacContext
        private let button: MavioButton
        private var observer: Observer?
        /// The page the dialog opens over.
        public weak var page: NSViewController?

        /// The indicator in context.
        public init(context: MacContext) {
            self.context = context
            let theme = context.theme
            button = MavioButton("", icon: "users", kind: .secondary, theme: theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(button)
            if !theme.isTV {
                wantsLayer = true
                shadow = NSShadow()
                layer?.shadowColor = NSColor.black.withAlphaComponent(0.25).cgColor
                layer?.shadowOpacity = 1
                layer?.shadowRadius = 8
                layer?.shadowOffset = CGSize(width: 0, height: -4)
            }
            button.setAccessibilityHelp(MavioStrings.syncplayTitle)
            button.onPress = { [weak self] in self?.open() }
            widthAnchor.constraint(lessThanOrEqualToConstant: theme.isTV ? 400 : 320).isActive = true
            observer = Observer { [weak self] in self?.update() }
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        private func update() {
            let joined = context.model.syncPlay.joined
            isHidden = joined == nil
            if let joined { button.title = groupChip(joined.group) }
        }

        private func open() {
            guard let joined = context.model.syncPlay.joined, let server = context.model.server(joined.serverId),
                let page = page ?? window?.contentViewController
            else { return }
            SyncPlayDialog(context: context, server: server).present(over: page)
        }
    }
#endif
