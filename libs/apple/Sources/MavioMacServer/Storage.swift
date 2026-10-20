#if os(macOS)
    import AppKit
    import MavioMac

    extension AppModel {
        /// The rclone remotes of a server, by name.
        fileprivate func remotes(_ server: ServerRecord) async throws -> [Mavio_Storage_V1_Remote] {
            try await call(
                server, Procedure.storage("ListRemotes"), Mavio_Storage_V1_ListRemotesRequest(),
                as: Mavio_Storage_V1_ListRemotesResponse.self
            ).remotes.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }

        /// The folders in a folder of a remote, "rclone:<remote>:<path>".
        fileprivate func remoteFolders(_ server: ServerRecord, _ path: String) async throws -> [String] {
            var request = Mavio_Storage_V1_ListFoldersRequest()
            request.path = path
            return try await call(
                server, Procedure.storage("ListFolders"), request, as: Mavio_Storage_V1_ListFoldersResponse.self
            ).folders
        }

        /// The kinds of remotes a server's rclone has.
        fileprivate func providers(_ server: ServerRecord) async throws -> [Mavio_Storage_V1_Provider] {
            try await call(
                server, Procedure.storage("ListProviders"), Mavio_Storage_V1_ListProvidersRequest(),
                as: Mavio_Storage_V1_ListProvidersResponse.self
            ).providers
        }
    }

    /// A size in terabytes, gigabytes or megabytes as it is large.
    private func capacity(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let (value, unit): (Double, UnitInformationStorage) =
            n >= 1e12 ? (n / 1e12, .terabytes) : n >= 1e9 ? (n / 1e9, .gigabytes) : (n / 1e6, .megabytes)
        return Measurement(value: value, unit: unit).formatted(
            .measurement(
                width: .abbreviated, usage: .asProvided, numberFormatStyle: .number.precision(.fractionLength(0...1))))
    }

    /// The keys of the kinds of remotes whose names are not camelCase as they
    /// are.
    private let kindKeys = [
        "b2": "backblaze", "cloud115": "cloudOneFifteen", "cloud123": "cloudOneTwentyThree", "s3": "amazon",
    ]

    /// A kind of remote's name in the UI's language, else as rclone describes
    /// it.
    private func kindName(_ providers: [Mavio_Storage_V1_Provider], _ type: String) -> String {
        var key = kindKeys[type] ?? ""
        if key.isEmpty {
            var upper = false
            for c in type {
                if c == " " {
                    upper = true
                } else {
                    key += upper ? c.uppercased() : String(c)
                    upper = false
                }
            }
        }
        let missing = "\u{0}"
        let name = MavioStrings.bundle.localizedString(forKey: "storage.provider.\(key)", value: missing, table: nil)
        if name != missing { return name }
        let description = providers.first { $0.name == type }?.description_p ?? ""
        return description.isEmpty ? type : description
    }

    /// The name of a folder of a remote, after its last slash or the colon
    /// of its remote.
    private func folderName(_ f: String) -> String {
        guard let cut = f.lastIndex(of: "/") ?? f.lastIndex(of: ":") else { return f }
        return String(f[f.index(after: cut)...])
    }

    /// A bar of how much of a disk or a remote is used, orange when nearly
    /// full and red when full, and its sizes.
    @MainActor private func usageBar(used: Int64, total: Int64, theme: Theme) -> NSView {
        let share = total > 0 ? min(1, Double(used) / Double(total)) : 0
        let color =
            share >= 0.95 ? theme.colors.danger : share >= 0.85 ? NSColor(hex: 0xFF9500) : NSColor(hex: 0x007AFF)
        let bar = ShareBar(share, color: color, track: theme.colors.fill)
        let stack = NSStackView.vertical(
            [
                bar,
                NSTextField.label(
                    MavioStrings.storageUsage(used: capacity(used), total: capacity(total)), theme.fonts.footnote,
                    color: theme.colors.textSecondary),
            ], spacing: MavioSpacing.s4)
        bar.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
        return stack
    }

    /// A line of text cut short to one line.
    @MainActor private func line(_ text: String, _ style: MavioFontStyle, _ color: NSColor, lines: Int = 1)
        -> NSTextField
    {
        let label = NSTextField.label(text, style, color: color, lines: lines)
        label.maximumNumberOfLines = lines
        label.lineBreakMode = .byTruncatingTail
        return label
    }

    // MARK: Storage

    /// Storage (docs/clients.md §6.3.10): the disks of the server's
    /// computer, the rclone the server runs or reaches, and its remotes —
    /// added with a kind's settings and the questions their configuration
    /// asks, such as authorizing a cloud drive — and their folders.
    final class StoragePage: ServerPage {
        private var status: Mavio_Storage_V1_GetStatusResponse?
        private var statusFailure: String?
        private var disks: [Mavio_Storage_V1_Disk]?
        private var disksFailure: String?
        private var remotes: [Mavio_Storage_V1_Remote]?
        private var remotesFailure: String?
        private var providers: [Mavio_Storage_V1_Provider] = []
        private var checks: [Task<Void, Never>] = []
        private lazy var form = FormPage(
            title: MavioStrings.settingsStorage, message: MavioStrings.storageMessage, theme: theme)
        private lazy var disksSection = section(MavioStrings.storageDisks, footer: MavioStrings.storageDisksFooter)
        private lazy var remotesSection = section(
            MavioStrings.storageRemotes, footer: MavioStrings.storageRemotesFooter)
        private lazy var rcloneSection = section(MavioStrings.storageRclone)

        private var available: Bool { status?.available ?? false }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsStorage
            form.add(disksSection)
            form.add(remotesSection)
            form.add(rcloneSection)
            render()
            show(form)
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func reload() {
            run { [weak self] in await self?.load() }
        }

        private func render() {
            renderDisks()
            renderRemotes()
            renderRclone()
            refocus()
        }

        private func renderDisks() {
            var rows: [NSView] = (disks ?? []).map(diskRow)
            if disks?.isEmpty == true {
                rows.append(note(MavioStrings.storageNoDisks))
            } else if disks == nil && disksFailure == nil {
                rows.append(padded(LoadingView()))
            }
            if let disksFailure { rows.append(note(disksFailure, failed: true)) }
            disksSection.setRows(rows)
        }

        private func diskRow(_ disk: Mavio_Storage_V1_Disk) -> NSView {
            let k: (icon: String, color: UInt32, label: String) =
                switch disk.kind {
                case .ssd: ("disk", 0x5E5CE6, MavioStrings.storageDiskSsd)
                case .hdd: ("disk", 0x5E5CE6, MavioStrings.storageDiskHdd)
                case .network: ("network", 0x007AFF, MavioStrings.storageDiskNetwork)
                case .cloud: ("cloud", 0x30B0C7, MavioStrings.storageDiskCloud)
                default: ("disk", 0x8E8E93, MavioStrings.storageDiskUnknown)
                }
            let name = line(disk.name.isEmpty ? disk.path : disk.name, theme.fonts.body, theme.colors.textPrimary)
            let kind = line(
                [k.label, disk.fileSystem].filter { !$0.isEmpty }.joined(separator: " · "), theme.fonts.footnote,
                theme.colors.textSecondary)
            var lines: [NSView] = [
                NSStackView.horizontal([name, Spacer(), kind], spacing: MavioSpacing.s12, alignment: .firstBaseline)
            ]
            if !disk.name.isEmpty { lines.append(line(disk.path, theme.fonts.footnote, theme.colors.textSecondary)) }
            if disk.hasTotalBytes {
                lines.append(usageBar(used: disk.usedBytes, total: disk.totalBytes, theme: theme))
            } else {
                lines.append(line(MavioStrings.storageNoAnswer, theme.fonts.footnote, theme.colors.textTertiary))
            }
            let stack = NSStackView.vertical(lines, spacing: 2)
            for view in lines { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return padded(
                NSStackView.horizontal([colorTile(k.icon, k.color), stack], spacing: MavioSpacing.s12, alignment: .top))
        }

        private func renderRemotes() {
            remotesSection.isHidden = !available
            for check in checks { check.cancel() }
            checks = []
            guard available else { return }
            var rows: [NSView] = (remotes ?? []).map(remoteRow)
            if remotes?.isEmpty == true { rows.append(note(MavioStrings.storageNoRemotes)) }
            if let remotesFailure { rows.append(note(remotesFailure, failed: true)) }
            rows.append(
                FormRow.link(MavioStrings.storageAdd, icon: "add", tint: theme.colors.accent, theme: theme) {
                    [weak self] in
                    guard let self else { return }
                    push(
                        NewRemotePage(
                            context: context, server: server, taken: (remotes ?? []).map(\.name),
                            changed: { [weak self] in self?.reload() }))
                })
            remotesSection.setRows(rows)
        }

        /// A remote in the list, with a dot telling whether the server
        /// reaches it: listing its root, green when that succeeds and red,
        /// with the error, when it fails, as a remote whose login expired
        /// does; and how much of it is used, where its backend tells.
        private func remoteRow(_ remote: Mavio_Storage_V1_Remote) -> NSView {
            let theme = theme
            let state = dot(theme.colors.fill, size: 6)
            state.setAccessibilityElement(true)
            state.setAccessibilityLabel(MavioStrings.storageChecking)
            let name = line(remote.name, theme.isTV ? theme.fonts.headline : theme.fonts.body, theme.colors.textPrimary)
            let head = NSStackView.horizontal([name, state], spacing: MavioSpacing.s8)
            let extra = NSStackView.vertical([], spacing: 2)
            let lines = NSStackView.vertical(
                [
                    head, line(kindName(providers, remote.type), theme.fonts.footnote, theme.colors.textSecondary),
                    extra,
                ],
                spacing: 2)
            extra.widthAnchor.constraint(equalTo: lines.widthAnchor).isActive = true
            let content = NSStackView.horizontal(
                [colorTile("cloud", 0x30B0C7), lines], spacing: MavioSpacing.s12, alignment: .top)
            let row = ContentRow(content, theme: theme) { [weak self] in
                guard let self else { return }
                push(
                    RemotePage(
                        context: context, server: server, name: remote.name,
                        changed: { [weak self] in self?.reload() }))
            }
            row.setAccessibilityLabel(remote.name)
            let model = model
            let server = server
            checks.append(
                Task { @MainActor [weak extra] in
                    var request = Mavio_Storage_V1_GetRemoteUsageRequest()
                    request.remote = remote.name
                    async let usage = try? model.call(
                        server, Procedure.storage("GetRemoteUsage"), request,
                        as: Mavio_Storage_V1_GetRemoteUsageResponse.self)
                    var reached: String
                    do {
                        _ = try await model.remoteFolders(server, "rclone:\(remote.name):")
                        reached = ""
                    } catch is CancellationError {
                        return
                    } catch {
                        reached = error.reason
                    }
                    let u = await usage
                    guard !Task.isCancelled, let extra else { return }
                    state.fill = reached.isEmpty ? NSColor(hex: 0x34C759) : theme.colors.danger
                    state.setAccessibilityLabel(
                        reached.isEmpty ? MavioStrings.storageReached : MavioStrings.storageUnreached)
                    var views: [NSView] = []
                    if !reached.isEmpty {
                        views.append(line(reached, theme.fonts.footnote, theme.colors.danger, lines: 2))
                    } else if let u, let used = Self.used(u) {
                        if u.hasTotalBytes, u.totalBytes > 0 {
                            views.append(usageBar(used: used, total: u.totalBytes, theme: theme))
                        } else {
                            views.append(
                                line(
                                    MavioStrings.storageUsed(used: capacity(used)), theme.fonts.footnote,
                                    theme.colors.textSecondary))
                        }
                    }
                    extra.setViews(views)
                    for view in views { view.widthAnchor.constraint(equalTo: extra.widthAnchor).isActive = true }
                })
            return row
        }

        private static func used(_ usage: Mavio_Storage_V1_GetRemoteUsageResponse) -> Int64? {
            if usage.hasUsedBytes { return usage.usedBytes }
            if usage.hasTotalBytes && usage.hasFreeBytes { return usage.totalBytes - usage.freeBytes }
            return nil
        }

        private func renderRclone() {
            let color =
                status == nil && statusFailure == nil
                ? theme.colors.fill : available ? NSColor(hex: 0x34C759) : theme.colors.danger
            let text = NSTextField.label(statusText, theme.fonts.body, color: theme.colors.textSecondary, lines: 0)
            text.alignment = .right
            var rows: [NSView] = [
                FormRow(
                    MavioStrings.storageStatus,
                    accessory: NSStackView.horizontal([dot(color), text], spacing: MavioSpacing.s8), theme: theme)
            ]
            if let status, !status.version.isEmpty {
                rows.append(valueRow(MavioStrings.storageVersion, status.version))
            }
            if let status, available {
                rows.append(
                    valueRow(
                        MavioStrings.storageRcd,
                        status.managed ? MavioStrings.storageManaged : MavioStrings.storageExternal,
                        detail: status.managed ? MavioStrings.storageManagedDetail : MavioStrings.storageExternalDetail)
                )
            }
            rcloneSection.setRows(rows)
        }

        private var statusText: String {
            if let statusFailure { return statusFailure }
            guard let status else { return "…" }
            if status.available { return MavioStrings.storageAvailable }
            return status.error.isEmpty ? MavioStrings.storageUnavailable : status.error
        }

        private func load() async {
            async let d: Void = loadDisks()
            do {
                status = try await model.call(
                    server, Procedure.storage("GetStatus"), Mavio_Storage_V1_GetStatusRequest(),
                    as: Mavio_Storage_V1_GetStatusResponse.self)
                statusFailure = nil
            } catch is CancellationError {
                return
            } catch {
                statusFailure = error.reason
            }
            if available {
                async let p = try? model.providers(server)
                do {
                    remotes = try await model.remotes(server)
                    remotesFailure = nil
                } catch is CancellationError {
                    return
                } catch {
                    remotesFailure = error.reason
                }
                if let p = await p { providers = p }
            }
            await d
            render()
        }

        private func loadDisks() async {
            do {
                disks = try await model.call(
                    server, Procedure.storage("ListDisks"), Mavio_Storage_V1_ListDisksRequest(),
                    as: Mavio_Storage_V1_ListDisksResponse.self
                ).disks
                disksFailure = nil
            } catch is CancellationError {
            } catch {
                disksFailure = error.reason
            }
        }

        isolated deinit {
            for check in checks { check.cancel() }
        }
    }

    // MARK: Options

    /// Whether an rclone provider filter, "AWS,Ceph" or "!AWS,Ceph", takes a
    /// provider.
    private func providerMatches(_ filter: String, _ provider: String) -> Bool {
        if filter.isEmpty { return true }
        let negated = filter.hasPrefix("!")
        let list = (negated ? String(filter.dropFirst()) : filter).split(separator: ",").map {
            $0.trimmingCharacters(in: .whitespaces)
        }
        return list.contains(provider) != negated
    }

    /// The provider an S3-like kind's values choose, which its options depend
    /// on.
    private func providerOf(_ options: [Mavio_Storage_V1_Option], _ values: [String: String]) -> String {
        values["provider"] ?? options.first { $0.name == "provider" }?.defaultValue ?? ""
    }

    /// The options of a kind shown for the values set: those its provider
    /// takes.
    private func shownOptions(
        _ options: [Mavio_Storage_V1_Option], _ values: [String: String], advanced: Bool
    ) -> [Mavio_Storage_V1_Option] {
        let provider = providerOf(options, values)
        return options.filter { o in
            (advanced || !o.advanced || o.required) && providerMatches(o.provider, provider)
        }
    }

    /// The values to send: those set, differing from their defaults.
    private func changedValues(_ options: [Mavio_Storage_V1_Option], _ values: [String: String]) -> [String: String] {
        values.filter { k, v in
            guard let o = options.first(where: { $0.name == k }) else { return !v.isEmpty }
            return v != o.defaultValue && (!v.isEmpty || o.defaultValue.isEmpty)
                && !v.trimmingCharacters(in: .whitespaces).isEmpty
        }
    }

    /// An example's value with the first line of its help.
    private func exampleLabel(_ e: Mavio_Storage_V1_Example) -> String {
        let help = e.help.split(separator: "\n").first.map(String.init) ?? ""
        return help.isEmpty ? e.value : "\(e.value) — \(help)"
    }

    /// The control of an option's value: a switch, a choice among its
    /// examples, or a field suggesting them. edit tells a typed value,
    /// choose a value chosen, after which the form is built again.
    @MainActor private func optionControl(
        _ o: Mavio_Storage_V1_Option, value: String, provider: String, placeholder: String?, theme: Theme,
        edit: @escaping @MainActor (String) -> Void, choose: @escaping @MainActor (String) -> Void
    ) -> NSView {
        let examples = o.examples.filter { providerMatches($0.provider, provider) }
        let current = value.isEmpty ? o.defaultValue : value
        if o.type == "bool" {
            let toggle = ActionSwitch(isOn: current == "true") { choose($0 ? "true" : "false") }
            toggle.setAccessibilityLabel(o.name)
            return FocusFrame(toggle, theme: theme)
        }
        if !examples.isEmpty && o.exclusive {
            var options: [(String, String)] = []
            if !examples.contains(where: { $0.value == current }) {
                options.append((current.isEmpty ? "—" : current, current))
            }
            options += examples.map { (exampleLabel($0), $0.value) }
            let popup = ActionPopUp(
                options: options.map(\.0), selected: options.firstIndex { $0.1 == current } ?? 0
            ) { choose(options[$0].1) }
            popup.setAccessibilityLabel(o.name)
            return FocusFrame(popup, theme: theme)
        }
        let input = TextInput(value, placeholder: placeholder ?? o.defaultValue, secure: o.password)
        input.onChange = edit
        input.field.setAccessibilityLabel(o.name)
        var views: [NSView] = [FocusFrame(input, theme: theme)]
        if !examples.isEmpty && !o.password {
            let menu = MavioButton(icon: "expand", kind: .plain, theme: theme)
            menu.onPress = { [weak menu] in
                guard let menu else { return }
                let list = NSMenu()
                for e in examples { list.addItem(MenuAction(exampleLabel(e)) { choose(e.value) }) }
                list.popUp(positioning: nil, at: NSPoint(x: 0, y: menu.bounds.height + 4), in: menu)
            }
            menu.setAccessibilityLabel(o.name)
            views.append(menu)
        }
        return NSStackView.horizontal(views, spacing: MavioSpacing.s8)
    }

    /// The rows of a kind's options: each its name, its whole help and its
    /// control, with those rarely changed behind a switch.
    @MainActor private func optionsSection(
        _ options: [Mavio_Storage_V1_Option], values: [String: String], advanced: Bool, theme: Theme,
        placeholder: (Mavio_Storage_V1_Option) -> String? = { _ in nil },
        edit: @escaping @MainActor (String, String) -> Void, choose: @escaping @MainActor (String, String) -> Void,
        showAdvanced: @escaping @MainActor (Bool) -> Void
    ) -> FormSection {
        let provider = providerOf(options, values)
        var rows: [NSView] = []
        for o in shownOptions(options, values, advanced: advanced) {
            let help = o.help.trimmingCharacters(in: .whitespacesAndNewlines)
            let title = NSMutableAttributedString(
                string: o.name,
                attributes: [
                    .font: NSFont.monospacedSystemFont(ofSize: theme.fonts.body.size, weight: .regular),
                    .foregroundColor: theme.colors.textPrimary,
                ])
            if o.required {
                title.append(
                    NSAttributedString(
                        string: " *", attributes: [.font: theme.fonts.body.font, .foregroundColor: theme.colors.danger])
                )
            }
            let name = NSTextField(labelWithAttributedString: title)
            var label: [NSView] = [name]
            if !help.isEmpty {
                label.append(
                    NSTextField.label(help, theme.fonts.caption, color: theme.colors.textSecondary, lines: 0))
            }
            let text = NSStackView.vertical(label, spacing: 2)
            let control = optionControl(
                o, value: values[o.name] ?? "", provider: provider, placeholder: placeholder(o), theme: theme,
                edit: { edit(o.name, $0) }, choose: { choose(o.name, $0) })
            if o.type == "bool" {
                rows.append(ControlRow(text, controls: [control], theme: theme))
            } else {
                let stack = NSStackView.vertical([text, control], spacing: MavioSpacing.s8)
                control.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                text.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                rows.append(ControlRow(stack, theme: theme))
            }
        }
        if options.contains(where: { $0.advanced && !$0.required }) {
            rows.append(
                FormRow.toggle(MavioStrings.storageAdvanced, isOn: advanced, theme: theme, onChange: showAdvanced))
        }
        return FormSection(
            MavioStrings.storageSettings, footer: MavioStrings.storageSettingsFooter, rows: rows, theme: theme)
    }

    // MARK: Remotes

    /// The page adding a remote: its name, its kind and the kind's
    /// settings, then the questions of its configuration, and the remote's
    /// page once it is added.
    final class NewRemotePage: ServerPage {
        private let taken: [String]
        private let changed: @MainActor () -> Void
        private var providers: [Mavio_Storage_V1_Provider] = []
        private var name = ""
        private var type = ""
        private var values: [String: String] = [:]
        private var advanced = false
        private var saving = false
        private var failure: String?
        private var question: RemoteQuestion?
        private var nameRow: ControlRow?
        private lazy var form = FormPage(theme: theme)
        private lazy var createButton = saveButton(MavioStrings.storageCreate) { [weak self] in self?.create() }

        /// The page adding a remote to server, whose names taken it cannot
        /// have; changed is called once it is added or removed.
        init(context: MacContext, server: ServerRecord, taken: [String], changed: @escaping @MainActor () -> Void) {
            self.taken = taken
            self.changed = changed
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.storageNew
            rebuild()
            show(form)
            run { [weak self] in await self?.loadProviders() }
        }

        private var trimmed: String { name.trimmingCharacters(in: .whitespaces) }
        private var provider: Mavio_Storage_V1_Provider? { providers.first { $0.name == type } }
        private var valid: Bool {
            trimmed.wholeMatch(of: /[A-Za-z0-9_][A-Za-z0-9_.+@-]*/) != nil && !taken.contains(trimmed)
        }
        private var missing: Bool {
            guard let provider else { return true }
            return shownOptions(provider.options, values, advanced: false).contains { o in
                o.required && (values[o.name] ?? o.defaultValue).isEmpty
            }
        }

        private func refresh() {
            createButton.isEnabled = valid && !missing && !saving
            createButton.title = saving ? "…" : MavioStrings.storageCreate
        }

        private func rebuild() {
            var views: [NSView] = [heading(MavioStrings.storageNew, actions: [createButton])]
            let (frame, input) = field(name) { [weak self] v in
                guard let self else { return }
                let before = taken.contains(trimmed)
                name = v
                // The name's line says when it is taken.
                if before != taken.contains(trimmed) {
                    endEditing()
                    rebuild()
                    if let field = nameField { view.window?.makeFirstResponder(field) }
                }
                refresh()
            }
            nameField = input.field
            input.field.alignment = .right
            views.append(
                section(
                    nil,
                    [
                        ControlRow(
                            text(
                                MavioStrings.storageName,
                                taken.contains(trimmed) ? MavioStrings.storageNameTaken : MavioStrings.storageNameDetail
                            ),
                            controls: [frame], theme: theme),
                        choiceRow(
                            MavioStrings.storageKind, nil,
                            [(MavioStrings.storageChooseKind, "")]
                                + providers.map { ("\(kindName(providers, $0.name)) (\($0.name))", $0.name) }, type
                        ) { [weak self] v in
                            self?.type = v
                            self?.values = [:]
                            self?.advanced = false
                            self?.rebuild()
                        },
                    ]))
            if let provider {
                views.append(
                    optionsSection(
                        provider.options, values: values, advanced: advanced, theme: theme,
                        edit: { [weak self] k, v in
                            self?.values[k] = v
                            self?.refresh()
                        },
                        choose: { [weak self] k, v in
                            self?.values[k] = v
                            self?.rebuild()
                        },
                        showAdvanced: { [weak self] v in
                            self?.advanced = v
                            self?.rebuild()
                        }))
            }
            if let failure { views.append(section(nil, [note(failure, failed: true)])) }
            form.setSections(views)
            refresh()
            refocus()
        }

        private var nameField: NSTextField?

        private func loadProviders() async {
            do {
                let list = try await model.providers(server)
                providers = list.sorted {
                    kindName(list, $0.name).localizedStandardCompare(kindName(list, $1.name)) == .orderedAscending
                }
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            rebuild()
        }

        private func create() {
            guard let provider else { return }
            endEditing()
            saving = true
            failure = nil
            refresh()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Storage_V1_CreateRemoteRequest()
                request.name = trimmed
                request.type = type
                request.parameters = changedValues(provider.options, values)
                do {
                    let r = try await model.call(
                        server, Procedure.storage("CreateRemote"), request,
                        as: Mavio_Storage_V1_CreateRemoteResponse.self)
                    changed()
                    saving = false
                    if r.hasQuestion { ask(r.question) } else { created() }
                } catch {
                    failure = error.reason
                    saving = false
                    rebuild()
                }
            }
        }

        private func ask(_ q: Mavio_Storage_V1_ConfigQuestion) {
            let question = RemoteQuestion(
                page: self, name: trimmed, question: q,
                done: { [weak self] in self?.created() },
                cancel: { [weak self] in self?.cancel() })
            self.question = question
            show(question.form)
            refocus()
        }

        /// The remote's page takes this one's place.
        private func created() {
            let page = RemotePage(context: context, server: server, name: trimmed, changed: changed)
            guard let navigation else { return }
            navigation.pop(animated: false)
            navigation.push(page, animated: false)
        }

        /// Cancelling the questions of a new remote deletes it.
        private func cancel() {
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Storage_V1_DeleteRemoteRequest()
                request.name = trimmed
                _ = try? await model.call(
                    server, Procedure.storage("DeleteRemote"), request,
                    as: Mavio_Storage_V1_DeleteRemoteResponse.self)
                changed()
                dismiss()
            }
        }
    }

    /// A remote's page: its kind's settings, configuring it again, its
    /// folders and deleting it; the questions of its configuration take the
    /// page while they are answered.
    final class RemotePage: ServerPage {
        private let name: String
        private let changed: @MainActor () -> Void
        private var remote: Mavio_Storage_V1_Remote?
        private var gone = false
        private var provider: Mavio_Storage_V1_Provider?
        private var providers: [Mavio_Storage_V1_Provider] = []
        private var values: [String: String] = [:]
        private var advanced = false
        private var saving = false
        private var notice: String?
        private var failure: String?
        private var question: RemoteQuestion?
        private var started = false
        private var noticeSection: FormSection?
        private lazy var folders = RemoteFolders(page: self, remote: name)
        private lazy var form = FormPage(theme: theme)
        private lazy var save = saveButton { [weak self] in self?.update(nil, notice: MavioStrings.storageSaved) }

        /// The page of the remote of server named name; changed is called
        /// once it is changed or removed.
        init(context: MacContext, server: ServerRecord, name: String, changed: @escaping @MainActor () -> Void) {
            self.name = name
            self.changed = changed
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = name
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in await self?.load() }
        }

        /// Secrets are never listed: an empty one keeps what is set.
        private static func secret(_ o: Mavio_Storage_V1_Option) -> Bool { o.password || o.sensitive }

        private var changes: [String: String] {
            guard let remote, let provider else { return [:] }
            return values.filter { k, v in
                if let o = provider.options.first(where: { $0.name == k }), Self.secret(o) { return !v.isEmpty }
                return v != (remote.parameters[k] ?? "")
            }
        }

        private func refresh() {
            save.isEnabled = !changes.isEmpty && !saving
            save.title = saving ? "…" : MavioStrings.actionSave
            var rows: [NSView] = []
            if let notice, changes.isEmpty { rows.append(note(notice)) }
            if let failure { rows.append(note(failure, failed: true)) }
            noticeSection?.setRows(rows)
            noticeSection?.isHidden = rows.isEmpty
        }

        private func rebuild() {
            var views: [NSView] = []
            if let remote {
                let head = NSStackView.horizontal(
                    [
                        colorTile("cloud", 0x30B0C7, size: 40),
                        NSStackView.vertical(
                            [
                                NSTextField.label(remote.name, theme.fonts.title3, color: theme.colors.textPrimary),
                                NSTextField.label(
                                    kindName(providers, remote.type), theme.fonts.footnote,
                                    color: theme.colors.textSecondary),
                            ], spacing: 2),
                        Spacer(), save,
                    ], spacing: MavioSpacing.s12)
                views.append(head)
                if let provider {
                    views.append(
                        optionsSection(
                            provider.options, values: values, advanced: advanced, theme: theme,
                            placeholder: { Self.secret($0) ? MavioStrings.storageSecretKept : nil },
                            edit: { [weak self] k, v in
                                self?.values[k] = v
                                self?.notice = nil
                                self?.refresh()
                            },
                            choose: { [weak self] k, v in
                                self?.values[k] = v
                                self?.notice = nil
                                self?.rebuild()
                            },
                            showAdvanced: { [weak self] v in
                                self?.advanced = v
                                self?.rebuild()
                            }))
                }
                let reconfigure = FormRow.action(MavioStrings.storageReconfigure, theme: theme) { [weak self] in
                    guard let self, !saving else { return }
                    update([:], notice: MavioStrings.storageReconfigured)
                }
                views.append(section(nil, [reconfigure]))
                views.append(folders.section)
                views.append(
                    section(
                        nil,
                        [
                            FormRow.action(MavioStrings.storageRemove, destructive: true, theme: theme) {
                                [weak self] in self?.confirmDelete()
                            }
                        ]))
            } else if gone {
                views.append(section(nil, [note(MavioStrings.storageNoRemote)]))
            } else if failure == nil {
                views.append(LoadingView())
            }
            let noticeSection = section()
            self.noticeSection = noticeSection
            views.append(noticeSection)
            form.setSections(views)
            refresh()
            show(form)
            refocus()
        }

        private func load() async {
            do {
                async let p = model.providers(server)
                let found = try await model.remotes(server).first { $0.name == name }
                providers = try await p
                remote = found
                gone = found == nil
                provider = providers.first { $0.name == found?.type }
                values = found?.parameters ?? [:]
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            rebuild()
            if remote != nil { folders.go(to: "") }
        }

        /// Updates the remote with parameters, the changes when nil.
        private func update(_ parameters: [String: String]?, notice done: String) {
            endEditing()
            saving = true
            failure = nil
            refresh()
            let parameters = parameters ?? changes
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Storage_V1_UpdateRemoteRequest()
                request.name = name
                request.parameters = parameters
                do {
                    let r = try await model.call(
                        server, Procedure.storage("UpdateRemote"), request,
                        as: Mavio_Storage_V1_UpdateRemoteResponse.self)
                    saving = false
                    if r.hasQuestion {
                        ask(r.question)
                        return
                    }
                    if r.hasRemote {
                        remote = r.remote
                        values = r.remote.parameters
                    } else if let remote {
                        values = remote.parameters
                    }
                    notice = done
                    changed()
                    rebuild()
                } catch {
                    saving = false
                    failure = error.reason
                    refresh()
                }
            }
        }

        private func ask(_ q: Mavio_Storage_V1_ConfigQuestion) {
            let question = RemoteQuestion(
                page: self, name: name, question: q,
                done: { [weak self] in
                    guard let self else { return }
                    question = nil
                    notice = MavioStrings.storageSaved
                    run { [weak self] in await self?.load() }
                },
                cancel: { [weak self] in
                    self?.question = nil
                    self?.rebuild()
                })
            self.question = question
            show(question.form)
            refocus()
        }

        private func confirmDelete() {
            confirm(
                MavioStrings.storageRemoveConfirm(name: name), message: MavioStrings.storageRemoveDetail,
                button: MavioStrings.storageRemove
            ) { [weak self] in self?.delete() }
        }

        private func delete() {
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_Storage_V1_DeleteRemoteRequest()
                request.name = name
                do {
                    _ = try await model.call(
                        server, Procedure.storage("DeleteRemote"), request,
                        as: Mavio_Storage_V1_DeleteRemoteResponse.self)
                    changed()
                    dismiss()
                } catch {
                    failure = error.reason
                    refresh()
                }
            }
        }
    }

    /// A remote's folders, to walk and to see that it is reached: a section
    /// of a remote's page, rebuilt alone as it is walked.
    @MainActor
    private final class RemoteFolders {
        private weak var page: ServerPage?
        private let remote: String
        private var path = ""
        private var folders: [String]?
        private var failure: String?
        private var loading: Task<Void, Never>?
        /// The section the folders show in.
        let section: FormSection

        init(page: ServerPage, remote: String) {
            self.page = page
            self.remote = remote
            section = FormSection(
                MavioStrings.storageFolders, footer: MavioStrings.storageFoldersFooter, theme: page.theme)
        }

        private var root: String { "rclone:\(remote):" }

        /// Walks to a folder, the remote's root when empty.
        func go(to path: String) {
            self.path = path
            loading?.cancel()
            folders = nil
            failure = nil
            render()
            loading = Task { [weak self] in await self?.load() }
        }

        private func load() async {
            guard let page else { return }
            let here = path.isEmpty ? root : path
            do {
                folders = try await page.model.remoteFolders(page.server, here)
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func render() {
            guard let page else { return }
            let theme = page.theme
            let here = path.isEmpty ? root : path
            var rows: [NSView] = [
                page.padded(
                    NSTextField.label(
                        here, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0, selectable: true))
            ]
            let label = { (icon: String, text: String, color: NSColor) in
                NSStackView.horizontal(
                    [
                        IconView(icon, size: 18, color: icon == "back" ? color : theme.colors.textSecondary),
                        NSTextField.label(text, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: color),
                    ], spacing: MavioSpacing.s12)
            }
            if here != root {
                let up = here.lastIndex(of: "/").map { String(here[..<$0]) } ?? root
                rows.append(
                    ContentRow(
                        label("back", MavioStrings.librariesParentFolder, theme.colors.accentText), chevron: false,
                        theme: theme
                    ) { [weak self] in self?.go(to: up) })
            }
            for f in folders ?? [] {
                rows.append(
                    ContentRow(label("folder", folderName(f), theme.colors.textPrimary), theme: theme) {
                        [weak self] in self?.go(to: f)
                    })
            }
            if let failure {
                rows.append(page.note(failure, failed: true))
            } else if folders?.isEmpty == true {
                rows.append(page.note(MavioStrings.librariesNoFolders))
            } else if folders == nil {
                rows.append(page.padded(LoadingView()))
            }
            section.setRows(rows)
            page.refocus()
        }

        isolated deinit {
            loading?.cancel()
        }
    }

    // MARK: Questions

    /// A QR code an option's help draws for a terminal: rows of half blocks,
    /// each character two modules high, a block where a module is light;
    /// dark holds its modules, true where dark.
    private struct HelpQR: Equatable {
        var before: String
        var dark: [[Bool]]
        var after: String

        init?(_ help: String) {
            let lines = help.components(separatedBy: "\n")
            let art = { (l: String) in
                l.count >= 21 && l.allSatisfy { " █▀▄".contains($0) } && l.contains { "█▀▄".contains($0) }
            }
            guard let start = lines.firstIndex(where: art) else { return nil }
            var end = start
            while end < lines.count && art(lines[end]) { end += 1 }
            var dark: [[Bool]] = []
            for (i, l) in lines[start..<end].enumerated() {
                let chars = Array(l)
                dark.append(chars.map { $0 != "█" && $0 != "▀" })
                // An odd last row is drawn as the top halves of its line alone.
                if i < end - start - 1 || l.contains(where: { $0 == "█" || $0 == "▄" }) {
                    dark.append(chars.map { $0 != "█" && $0 != "▄" })
                }
            }
            guard dark.count >= 21 else { return nil }
            self.before = lines[..<start].joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
            self.dark = dark
            self.after = lines[end...].joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        }
    }

    /// A QR code, drawn dark on light whatever the theme, for a phone to
    /// scan, with a quiet zone around it as scanners need.
    private final class QRCodeView: NSView {
        private let dark: [[Bool]]
        private let quiet: CGFloat = 12

        init(dark: [[Bool]]) {
            self.dark = dark
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let width = CGFloat(dark.map(\.count).max() ?? 0)
            let height = CGFloat(max(dark.count, 1))
            let side: CGFloat = 224 - quiet * 2
            let module = min(side / max(width, 1), side / height)
            size(width: width * module + quiet * 2, height: height * module + quiet * 2)
            wantsLayer = true
            layer?.cornerRadius = 8
            layer?.masksToBounds = true
            setAccessibilityElement(true)
            setAccessibilityRole(.image)
            setAccessibilityLabel("QR")
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var isFlipped: Bool { true }

        override func draw(_ dirtyRect: NSRect) {
            NSColor.white.setFill()
            bounds.fill()
            let width = CGFloat(dark.map(\.count).max() ?? 0)
            let inner = bounds.insetBy(dx: quiet, dy: quiet)
            let module = min(inner.width / max(width, 1), inner.height / CGFloat(max(dark.count, 1)))
            NSColor.black.setFill()
            for (y, row) in dark.enumerated() {
                for (x, on) in row.enumerated() where on {
                    CGRect(
                        x: inner.minX + CGFloat(x) * module, y: inner.minY + CGFloat(y) * module, width: module,
                        height: module
                    ).fill()
                }
            }
        }
    }

    /// The questions of a remote's configuration — a yes or no, a choice, a
    /// value, or the token of authorizing a cloud drive — answered until
    /// none is left, shown on the page of the remote in place of its
    /// content. A question drawing a QR code, as 115's login asks, is
    /// answered yes again and again until the phone confirms the login, its
    /// code kept on screen while rclone asks again without it. Cancelling
    /// waits for the answer being sent, then calls cancel.
    @MainActor
    private final class RemoteQuestion {
        private weak var page: ServerPage?
        private let name: String
        private let done: @MainActor () -> Void
        private let cancel: @MainActor () -> Void
        private var q: Mavio_Storage_V1_ConfigQuestion
        private var answer: String
        private var scan: (state: String, qr: HelpQR)?
        private var pending = false
        private var failure: String?
        private var cancelled = false
        private var loop: Task<Void, Never>?
        private var sending: Task<Void, Never>?
        /// The form the question shows in.
        let form: FormPage

        init(
            page: ServerPage, name: String, question: Mavio_Storage_V1_ConfigQuestion,
            done: @escaping @MainActor () -> Void, cancel: @escaping @MainActor () -> Void
        ) {
            self.page = page
            self.name = name
            self.done = done
            self.cancel = cancel
            q = question
            answer = question.option.defaultValue
            form = FormPage(title: name, message: MavioStrings.storageQuestionMessage, theme: page.theme)
            render()
        }

        private var option: Mavio_Storage_V1_Option? { q.hasOption ? q.option : nil }
        private var drawn: HelpQR? { option.flatMap { HelpQR($0.help) } }
        private var qr: HelpQR? { drawn ?? (scan?.state == q.state ? scan?.qr : nil) }
        private var confirm: Bool { option?.type == "bool" }
        private var waiting: Bool { qr != nil && confirm && failure == nil && !cancelled }

        private func render() {
            guard let page else { return }
            let theme = page.theme
            var rows: [NSView] = []
            if let o = option {
                if let qr {
                    if !qr.before.isEmpty {
                        rows.append(page.note(qr.before))
                    }
                    let code = QRCodeView(dark: qr.dark)
                    let holder = NSView()
                    holder.translatesAutoresizingMaskIntoConstraints = false
                    holder.addSubview(code)
                    NSLayoutConstraint.activate([
                        code.centerXAnchor.constraint(equalTo: holder.centerXAnchor),
                        code.topAnchor.constraint(equalTo: holder.topAnchor),
                        code.bottomAnchor.constraint(equalTo: holder.bottomAnchor),
                    ])
                    rows.append(page.padded(holder))
                    if !cancelled {
                        rows.append(
                            page.padded(
                                NSTextField.label(
                                    waiting ? MavioStrings.storageWaitingScan : qr.after, theme.fonts.footnote,
                                    color: theme.colors.textSecondary, lines: 0)))
                    }
                } else {
                    let help = o.help.trimmingCharacters(in: .whitespacesAndNewlines)
                    if !help.isEmpty {
                        rows.append(
                            page.padded(
                                NSTextField.label(
                                    help, theme.fonts.body, color: theme.colors.textPrimary, lines: 0, selectable: true)
                            ))
                    }
                }
                if !confirm {
                    let control = optionControl(
                        o, value: answer, provider: "", placeholder: nil, theme: theme,
                        edit: { [weak self] v in self?.answer = v },
                        choose: { [weak self] v in
                            self?.answer = v
                            self?.render()
                        })
                    rows.append(ControlRow(control, theme: theme))
                }
            } else {
                rows.append(page.note(MavioStrings.storageQuestionNone))
            }
            if !q.error.isEmpty { rows.append(page.note(q.error, failed: true)) }
            let question = FormSection(
                nil, footer: option?.name == "config_token" ? MavioStrings.storageQuestionFooter : nil, rows: rows,
                theme: theme)
            var actions: [NSView] = []
            if !waiting && !cancelled {
                if confirm {
                    actions.append(
                        FormRow.action(MavioStrings.storageYes, theme: theme) { [weak self] in
                            guard self?.pending == false else { return }
                            self?.send("true")
                        })
                    actions.append(
                        FormRow.action(MavioStrings.storageNo, theme: theme) { [weak self] in
                            guard self?.pending == false else { return }
                            self?.send("false")
                        })
                } else {
                    actions.append(
                        FormRow.action(pending ? "…" : MavioStrings.storageContinue, theme: theme) { [weak self] in
                            guard let self, !pending else { return }
                            send(answer)
                        })
                }
            }
            actions.append(
                FormRow.action(
                    cancelled ? MavioStrings.storageCancelling : MavioStrings.actionCancel, destructive: true,
                    theme: theme
                ) { [weak self] in
                    guard let self, !cancelled else { return }
                    cancelled = true
                    loop?.cancel()
                    if pending { render() } else { cancel() }
                })
            if let failure { actions.append(page.note(failure, failed: true)) }
            form.setSections([question, FormSection(rows: actions, theme: theme)])
            page.refocus()
            // Each answer waits for the last; a code waiting to be scanned
            // is answered yes again every few seconds.
            loop?.cancel()
            if waiting && !pending {
                loop = Task { [weak self] in
                    try? await Task.sleep(for: .seconds(2.5))
                    guard !Task.isCancelled else { return }
                    self?.send("true")
                }
            }
        }

        private func send(_ value: String) {
            guard let page else { return }
            page.endEditing()
            pending = true
            failure = nil
            render()
            let drawn = drawn
            let model = page.model
            let server = page.server
            var request = Mavio_Storage_V1_ContinueRemoteRequest()
            request.name = name
            request.state = q.state
            request.answer = value
            sending = Task { [weak self] in
                do {
                    let r = try await model.call(
                        server, Procedure.storage("ContinueRemote"), request,
                        as: Mavio_Storage_V1_ContinueRemoteResponse.self)
                    guard let self else { return }
                    pending = false
                    if cancelled { return cancel() }
                    guard r.hasQuestion else { return done() }
                    if let drawn { scan = (q.state, drawn) }
                    q = r.question
                    answer = r.question.option.defaultValue
                } catch {
                    guard let self else { return }
                    pending = false
                    if cancelled { return cancel() }
                    failure = error.reason
                }
                self?.render()
            }
        }

        isolated deinit {
            loop?.cancel()
            sending?.cancel()
        }
    }
#endif
