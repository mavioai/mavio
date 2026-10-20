#if os(macOS)
    import AppKit
    import MavioMac

    /// The items of a list, trimmed, without the empty ones.
    private func splitList(_ s: String, by separator: Character) -> [String] {
        s.split(separator: separator).map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
    }

    /// A delay in whole minutes.
    private func minutes(_ d: Google_Protobuf_Duration?) -> Int {
        guard let d else { return 0 }
        return Int(((Double(d.seconds) + Double(d.nanos) / 1e9) / 60).rounded())
    }

    /// The buttons under an entry's form: saving, cancelling, and removing
    /// it once confirmed by a second press.
    final class FormButtons: NSView {
        private let save: MavioButton
        private let failureLabel: NSTextField
        private var valid: Bool
        private var busy: Bool

        /// The buttons of a form valid or not, busy while it is written,
        /// with what writing it failed of; remove when it is saved.
        init(
            valid: Bool, busy: Bool, failure: String?, theme: Theme, save: @escaping @MainActor () -> Void,
            cancel: @escaping @MainActor () -> Void, remove: (@MainActor () -> Void)? = nil
        ) {
            self.valid = valid
            self.busy = busy
            self.save = MavioButton(busy ? "…" : MavioStrings.actionSave, kind: .primary, theme: theme, action: save)
            failureLabel = .label(
                failure ?? "", theme.fonts.callout, color: theme.colors.danger, lines: 0, selectable: true)
            failureLabel.isHidden = failure == nil
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            var views: [NSView] = [
                self.save, MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme, action: cancel),
            ]
            if let remove {
                let confirm = MavioButton(
                    MavioStrings.rulesRemoveConfirm, kind: .destructive, theme: theme, action: remove)
                confirm.isHidden = true
                confirm.isEnabled = !busy
                let first = MavioButton(MavioStrings.rulesRemove, kind: .destructive, theme: theme, action: {})
                first.onPress = { [weak first, weak confirm] in
                    first?.isHidden = true
                    confirm?.isHidden = false
                    if let confirm { FocusEngine.active(for: confirm)?.schedule() }
                }
                views += [first, confirm]
            }
            views.append(failureLabel)
            addFilling(NSStackView.horizontal(views, spacing: MavioSpacing.s12))
            update(valid: valid)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows whether the form may be saved.
        func update(valid: Bool) {
            self.valid = valid
            save.isEnabled = valid && !busy
        }
    }

    /// Lays a row opening an entry beside the buttons at its end, each
    /// focusable on its own.
    @MainActor private func entry(_ row: FormRow, trailing: NSView?) -> NSView {
        guard let trailing else { return row }
        let holder = NSView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        holder.addSubview(row)
        holder.addSubview(trailing)
        row.translatesAutoresizingMaskIntoConstraints = false
        trailing.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            row.leadingAnchor.constraint(equalTo: holder.leadingAnchor),
            row.topAnchor.constraint(equalTo: holder.topAnchor),
            row.bottomAnchor.constraint(equalTo: holder.bottomAnchor),
            trailing.leadingAnchor.constraint(equalTo: row.trailingAnchor, constant: MavioSpacing.s8),
            trailing.trailingAnchor.constraint(equalTo: holder.trailingAnchor, constant: -MavioSpacing.s16),
            trailing.centerYAnchor.constraint(equalTo: holder.centerYAnchor),
        ])
        return holder
    }

    // MARK: Delay profiles

    /// Acquisition's page of the delay profiles, as the web UI's: which
    /// hold releases back for better ones, in order.
    final class DelayProfilesPage: FormController {
        private var list: [Mavio_Acquisition_V1_DelayProfile]?
        private var failure: String?
        /// The profile edited, by ID; "new" for one to add.
        private var editing: String?
        private var draft = Draft()
        private var busy = false
        private var formFailure: String?
        private var reordering = false
        private var reorderFailure: String?
        private weak var buttons: FormButtons?

        /// What the form holds.
        private struct Draft {
            var profile = Mavio_Acquisition_V1_DelayProfile()
            var usenet = true
            var torrent = true
            var preferred = Mavio_Acquisition_V1_DownloadProtocol.usenet
            var usenetDelay: Int32 = 0
            var torrentDelay: Int32 = 0
            var bypassQuality = false
            var bypassScore = false
            var minScore: Int32 = 0
            var tags = ""

            init() {}

            init(_ p: Mavio_Acquisition_V1_DelayProfile) {
                profile = p
                usenet = p.enableUsenet
                torrent = p.enableTorrent
                preferred = p.preferredProtocol == .unspecified ? .usenet : p.preferredProtocol
                usenetDelay = Int32(minutes(p.hasUsenetDelay ? p.usenetDelay : nil))
                torrentDelay = Int32(minutes(p.hasTorrentDelay ? p.torrentDelay : nil))
                bypassQuality = p.bypassIfHighestQuality
                bypassScore = p.bypassIfAboveCustomFormatScore
                minScore = p.minimumCustomFormatScore
                tags = p.tags.joined(separator: ", ")
            }

            var valid: Bool { (usenet || torrent) && (profile.default || !splitList(tags, by: ",").isEmpty) }

            var result: Mavio_Acquisition_V1_DelayProfile {
                var p = profile
                p.enableUsenet = usenet
                p.enableTorrent = torrent
                p.preferredProtocol = usenet && torrent ? preferred : usenet ? .usenet : .torrent
                p.usenetDelay = Google_Protobuf_Duration(seconds: Int64(usenetDelay) * 60)
                p.torrentDelay = Google_Protobuf_Duration(seconds: Int64(torrentDelay) * 60)
                p.bypassIfHighestQuality = bypassQuality
                p.bypassIfAboveCustomFormatScore = bypassScore
                p.minimumCustomFormatScore = minScore
                p.tags = profile.default ? [] : splitList(tags, by: ",")
                return p
            }
        }

        override func viewDidLoad() {
            title = MavioStrings.acquisitionDelays
            message = MavioStrings.rulesDelaysMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        private var ordered: [Mavio_Acquisition_V1_DelayProfile] { (list ?? []).filter { !$0.default } }

        override func sections() -> [NSView] {
            var rows: [NSView] = []
            let ordered = ordered
            for p in list ?? [] {
                if editing == p.id {
                    rows += form(create: false)
                    continue
                }
                let row = FormRow(
                    p.default ? MavioStrings.rulesDefaultDelay : p.tags.joined(separator: ", "), subtitle: summary(p),
                    theme: theme)
                if editing == nil { row.onPress = { [weak self] in self?.open(p.id, p) } }
                var moves: NSView?
                if !p.default, let i = ordered.firstIndex(where: { $0.id == p.id }) {
                    moves = moveButtons(at: i, of: reordering ? 1 : ordered.count) { [weak self] i, by in
                        self?.move(i, by)
                    }
                }
                rows.append(entry(row, trailing: moves))
            }
            if editing == "new" { rows += form(create: true) }
            if list == nil && failure == nil { rows.append(spinnerRow()) }
            if let failure { rows.append(note(failure, failed: true)) }
            if let reorderFailure { rows.append(note(reorderFailure, failed: true)) }
            var add: (String, @MainActor () -> Void)?
            if editing == nil { add = (MavioStrings.rulesAdd, { [weak self] in self?.open("new", nil) }) }
            return [card(MavioStrings.rulesDelaysInOrder, button: add, footer: MavioStrings.rulesDelaysFooter, rows)]
        }

        private var blank: Mavio_Acquisition_V1_DelayProfile {
            var p = Mavio_Acquisition_V1_DelayProfile()
            p.enableUsenet = true
            p.enableTorrent = true
            p.preferredProtocol = .usenet
            return p
        }

        private func form(create: Bool) -> [NSView] {
            let d = draft
            var rows: [NSView] = []
            if !d.profile.default {
                rows.append(
                    fieldRow(MavioStrings.rulesTags, subtitle: MavioStrings.rulesDelayTagsDetail, text: d.tags) {
                        [weak self] text in
                        self?.draft.tags = text
                        self?.validate()
                    })
            }
            rows.append(
                toggleRow(MavioStrings.acquisitionUsenet, isOn: d.usenet) { [weak self] on in
                    self?.draft.usenet = on
                    self?.render()
                })
            rows.append(
                toggleRow(MavioStrings.acquisitionTorrent, isOn: d.torrent) { [weak self] on in
                    self?.draft.torrent = on
                    self?.render()
                })
            if d.usenet && d.torrent {
                rows.append(
                    choiceRow(
                        MavioStrings.rulesPreferred,
                        options: [
                            (MavioStrings.acquisitionUsenet, Mavio_Acquisition_V1_DownloadProtocol.usenet),
                            (MavioStrings.acquisitionTorrent, .torrent),
                        ], selected: d.preferred
                    ) { [weak self] p in self?.draft.preferred = p })
            }
            if d.usenet {
                rows.append(
                    numberRow(
                        MavioStrings.rulesUsenetDelay, subtitle: MavioStrings.rulesMinutes,
                        value: Double(d.usenetDelay),
                        range: 0...525_600
                    ) { [weak self] v in self?.draft.usenetDelay = Int32(v.rounded()) })
            }
            if d.torrent {
                rows.append(
                    numberRow(
                        MavioStrings.rulesTorrentDelay, subtitle: MavioStrings.rulesMinutes,
                        value: Double(d.torrentDelay), range: 0...525_600
                    ) { [weak self] v in self?.draft.torrentDelay = Int32(v.rounded()) })
            }
            rows.append(
                toggleRow(
                    MavioStrings.rulesBypassQuality, subtitle: MavioStrings.rulesBypassQualityDetail,
                    isOn: d.bypassQuality
                ) { [weak self] on in self?.draft.bypassQuality = on })
            rows.append(
                toggleRow(MavioStrings.rulesBypassScore, isOn: d.bypassScore) { [weak self] on in
                    self?.draft.bypassScore = on
                    self?.render()
                })
            if d.bypassScore {
                rows.append(
                    numberRow(MavioStrings.rulesMinScore, value: Double(d.minScore), range: -100_000...100_000) {
                        [weak self] v in self?.draft.minScore = Int32(v.rounded())
                    })
            }
            let id = d.profile.id
            var remove: (@MainActor () -> Void)?
            if !create && !d.profile.default { remove = { [weak self] in self?.remove(id) } }
            let buttons = FormButtons(
                valid: d.valid, busy: busy, failure: formFailure, theme: theme,
                save: { [weak self] in self?.write(create: create) }, cancel: { [weak self] in self?.close() },
                remove: remove)
            self.buttons = buttons
            rows.append(row(buttons))
            return rows
        }

        private func validate() { buttons?.update(valid: draft.valid) }

        private func summary(_ p: Mavio_Acquisition_V1_DelayProfile) -> String {
            let protocols = [
                p.enableUsenet ? MavioStrings.acquisitionUsenet : nil,
                p.enableTorrent ? MavioStrings.acquisitionTorrent : nil,
            ]
            .compactMap { $0 }.joined(separator: " + ")
            return [
                protocols,
                MavioStrings.rulesPrefers(
                    protocol: p.preferredProtocol == .torrent
                        ? MavioStrings.acquisitionTorrent : MavioStrings.acquisitionUsenet),
                p.enableUsenet ? MavioStrings.rulesUsenetDelayOf(minutes: String(minutes(p.usenetDelay))) : "",
                p.enableTorrent ? MavioStrings.rulesTorrentDelayOf(minutes: String(minutes(p.torrentDelay))) : "",
            ]
            .filter { !$0.isEmpty }.joined(separator: " · ")
        }

        private func open(_ id: String, _ p: Mavio_Acquisition_V1_DelayProfile?) {
            formFailure = nil
            draft = Draft(p ?? blank)
            editing = id
            render()
        }

        private func close() {
            editing = nil
            render()
        }

        private func move(_ i: Int, _ by: Int) {
            var ids = ordered.map(\.id)
            let id = ids.remove(at: i)
            ids.insert(id, at: i + by)
            var request = Mavio_Acquisition_V1_ReorderDelayProfilesRequest()
            request.ids = ids
            reordering = true
            render()
            run { [weak self, model, server] in
                do {
                    let list = try await model.call(
                        server, Procedure.profiles("ReorderDelayProfiles"), request,
                        as: Mavio_Acquisition_V1_ReorderDelayProfilesResponse.self
                    ).profiles
                    self?.list = list
                    self?.reorderFailure = nil
                } catch {
                    self?.reorderFailure = error.reason
                }
                self?.reordering = false
                self?.render()
            }
        }

        private func write(create: Bool) {
            endEditing()
            let p = draft.result
            busy = true
            formFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    if create {
                        var request = Mavio_Acquisition_V1_CreateDelayProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles("CreateDelayProfile"), request,
                            as: Mavio_Acquisition_V1_CreateDelayProfileResponse.self)
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateDelayProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles("UpdateDelayProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateDelayProfileResponse.self)
                    }
                    self?.busy = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.busy = false
                    self?.formFailure = error.reason
                    self?.render()
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteDelayProfileRequest()
            request.id = id
            busy = true
            formFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteDelayProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteDelayProfileResponse.self)
                    self?.busy = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.busy = false
                    self?.formFailure = error.reason
                    self?.render()
                }
            }
        }

        private func load() async {
            do {
                list = try await model.call(
                    server, Procedure.profiles("ListDelayProfiles"), Mavio_Acquisition_V1_ListDelayProfilesRequest(),
                    as: Mavio_Acquisition_V1_ListDelayProfilesResponse.self
                ).profiles
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }
    }

    // MARK: Release profiles

    /// Acquisition's page of the release profiles, as the web UI's: which
    /// require or ignore terms in releases' titles.
    final class ReleaseProfilesPage: FormController {
        private var list: [Mavio_Acquisition_V1_ReleaseProfile]?
        private var indexers: [Mavio_Acquisition_V1_Indexer] = []
        private var failure: String?
        /// The profile edited, by ID; "new" for one to add.
        private var editing: String?
        private var draft = Draft()
        private var busy = false
        private var formFailure: String?
        private weak var buttons: FormButtons?

        /// What the form holds.
        private struct Draft {
            var id = ""
            var name = ""
            var enabled = true
            var required = ""
            var ignored = ""
            /// The indexer as "plugin/indexer"; empty for all of them.
            var indexer = ""
            var tags = ""

            init() {}

            init(_ p: Mavio_Acquisition_V1_ReleaseProfile) {
                id = p.id
                name = p.name
                enabled = p.enabled
                required = p.required.joined(separator: "\n")
                ignored = p.ignored.joined(separator: "\n")
                indexer = p.hasIndexer ? "\(p.indexer.pluginID)/\(p.indexer.indexerID)" : ""
                tags = p.tags.joined(separator: ", ")
            }

            /// The indexer chosen, split into its plugin and its ID.
            var chosen: (plugin: String, id: String) {
                guard let slash = indexer.firstIndex(of: "/") else { return (indexer, "") }
                return (String(indexer[..<slash]), String(indexer[indexer.index(after: slash)...]))
            }

            var valid: Bool { splitList(required, by: "\n").count + splitList(ignored, by: "\n").count > 0 }

            var result: Mavio_Acquisition_V1_ReleaseProfile {
                var p = Mavio_Acquisition_V1_ReleaseProfile()
                p.id = id
                p.name = name.trimmingCharacters(in: .whitespaces)
                p.enabled = enabled
                p.required = splitList(required, by: "\n")
                p.ignored = splitList(ignored, by: "\n")
                if !indexer.isEmpty {
                    var ref = Mavio_Acquisition_V1_IndexerRef()
                    ref.pluginID = chosen.plugin
                    ref.indexerID = chosen.id
                    p.indexer = ref
                }
                p.tags = splitList(tags, by: ",")
                return p
            }
        }

        override func viewDidLoad() {
            title = MavioStrings.acquisitionReleases
            message = MavioStrings.rulesReleasesMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var rows: [NSView] = []
            for p in list ?? [] {
                if editing == p.id {
                    rows += form(create: false)
                    continue
                }
                let row = FormRow(
                    p.name.isEmpty ? MavioStrings.rulesUnnamed : p.name, subtitle: summary(p), theme: theme)
                if editing == nil { row.onPress = { [weak self] in self?.open(p.id, Draft(p)) } }
                rows.append(row)
            }
            if editing == "new" { rows += form(create: true) }
            if let list, list.isEmpty, editing == nil { rows.append(note(MavioStrings.rulesNoReleases)) }
            if list == nil && failure == nil { rows.append(spinnerRow()) }
            if let failure { rows.append(note(failure, failed: true)) }
            var add: (String, @MainActor () -> Void)?
            if editing == nil { add = (MavioStrings.rulesAdd, { [weak self] in self?.open("new", Draft()) }) }
            return [card(MavioStrings.acquisitionReleases, button: add, footer: MavioStrings.rulesReleasesFooter, rows)]
        }

        private func form(create: Bool) -> [NSView] {
            let d = draft
            var indexerOptions: [(String, String)] = [(MavioStrings.rulesAllIndexers, "")]
            indexerOptions += indexers.map { ($0.name, "\($0.pluginID)/\($0.id)") }
            if !d.indexer.isEmpty, !indexerOptions.contains(where: { $0.1 == d.indexer }) {
                indexerOptions.append((d.chosen.id.isEmpty ? d.indexer : d.chosen.id, d.indexer))
            }
            let id = d.id
            var remove: (@MainActor () -> Void)?
            if !create { remove = { [weak self] in self?.remove(id) } }
            let buttons = FormButtons(
                valid: d.valid, busy: busy, failure: formFailure, theme: theme,
                save: { [weak self] in self?.write(create: create) }, cancel: { [weak self] in self?.close() },
                remove: remove)
            self.buttons = buttons
            return [
                fieldRow(MavioStrings.rulesName, text: d.name) { [weak self] text in self?.draft.name = text },
                toggleRow(MavioStrings.rulesEnabled, isOn: d.enabled) { [weak self] on in self?.draft.enabled = on },
                terms(d.required, MavioStrings.rulesRequired, MavioStrings.rulesRequiredDetail) { [weak self] text in
                    self?.draft.required = text
                    self?.validate()
                },
                terms(d.ignored, MavioStrings.rulesIgnored, MavioStrings.rulesIgnoredDetail) { [weak self] text in
                    self?.draft.ignored = text
                    self?.validate()
                },
                choiceRow(MavioStrings.rulesIndexer, options: indexerOptions, selected: d.indexer) {
                    [weak self] value in
                    self?.draft.indexer = value
                },
                fieldRow(MavioStrings.rulesTags, subtitle: MavioStrings.rulesReleaseTagsDetail, text: d.tags) {
                    [weak self] text in self?.draft.tags = text
                },
                row(buttons),
            ]
        }

        /// A row of a list of terms, one a line.
        private func terms(
            _ text: String, _ title: String, _ detail: String, onChange: @escaping @MainActor (String) -> Void
        ) -> NSView {
            let block = TextBlock(text, label: title, lines: 3, theme: theme, onChange: onChange)
            let stack = NSStackView.vertical(
                [
                    NSTextField.label(title, theme.fonts.body, color: theme.colors.textPrimary),
                    NSTextField.label(detail, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                    block,
                ], spacing: MavioSpacing.s4)
            block.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return row(stack)
        }

        private func validate() { buttons?.update(valid: draft.valid) }

        private func indexerName(_ p: Mavio_Acquisition_V1_ReleaseProfile) -> String? {
            guard p.hasIndexer else { return nil }
            return indexers.first { $0.pluginID == p.indexer.pluginID && $0.id == p.indexer.indexerID }?.name
                ?? p.indexer.indexerID
        }

        private func summary(_ p: Mavio_Acquisition_V1_ReleaseProfile) -> String {
            dotted([
                p.enabled ? nil : MavioStrings.acquisitionOff,
                p.required.isEmpty ? nil : MavioStrings.rulesRequiredCount(count: p.required.count),
                p.ignored.isEmpty ? nil : MavioStrings.rulesIgnoredCount(count: p.ignored.count),
                indexerName(p),
                p.tags.joined(separator: ", "),
            ])
        }

        private func open(_ id: String, _ d: Draft) {
            formFailure = nil
            draft = d
            editing = id
            render()
        }

        private func close() {
            editing = nil
            render()
        }

        private func write(create: Bool) {
            endEditing()
            let p = draft.result
            busy = true
            formFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    if create {
                        var request = Mavio_Acquisition_V1_CreateReleaseProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles("CreateReleaseProfile"), request,
                            as: Mavio_Acquisition_V1_CreateReleaseProfileResponse.self)
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateReleaseProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles("UpdateReleaseProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateReleaseProfileResponse.self)
                    }
                    self?.busy = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.busy = false
                    self?.formFailure = error.reason
                    self?.render()
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteReleaseProfileRequest()
            request.id = id
            busy = true
            formFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteReleaseProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteReleaseProfileResponse.self)
                    self?.busy = false
                    self?.editing = nil
                    await self?.load()
                } catch {
                    self?.busy = false
                    self?.formFailure = error.reason
                    self?.render()
                }
            }
        }

        private func load() async {
            let model = model
            let server = server
            async let i = try? model.call(
                server, Procedure.releases("ListIndexers"), Mavio_Acquisition_V1_ListIndexersRequest(),
                as: Mavio_Acquisition_V1_ListIndexersResponse.self)
            do {
                list = try await model.call(
                    server, Procedure.profiles("ListReleaseProfiles"),
                    Mavio_Acquisition_V1_ListReleaseProfilesRequest(),
                    as: Mavio_Acquisition_V1_ListReleaseProfilesResponse.self
                ).profiles
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            if let found = await i { indexers = found.indexers }
            render()
        }
    }

    // MARK: Remote path mappings

    /// Acquisition's page of the remote path mappings, as the web UI's:
    /// which map the folders download clients on other hosts report.
    final class RemotePathMappingsPage: FormController {
        private var list: [Mavio_Acquisition_V1_RemotePathMapping]?
        private var failure: String?
        private var adding = false
        private var host = ""
        private var remote = ""
        private var local = ""
        private var creating = false
        private var createFailure: String?
        private var removing: String?
        private weak var buttons: FormButtons?

        override func viewDidLoad() {
            title = MavioStrings.acquisitionMappings
            message = MavioStrings.rulesMappingsMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var rows: [NSView] = (list ?? []).map { m in
                let id = m.id
                let remove = MavioButton(MavioStrings.rulesRemove, kind: .destructive, theme: theme) { [weak self] in
                    self?.remove(id)
                }
                remove.isEnabled = removing != m.id
                return FormRow(
                    m.host, subtitle: "\(m.remotePath) → \(m.localPath)", accessory: remove, theme: theme)
            }
            if adding {
                rows.append(
                    fieldRow(
                        MavioStrings.rulesHost, subtitle: MavioStrings.rulesHostDetail, text: host,
                        placeholder: "192.168.1.10"
                    ) { [weak self] text in
                        self?.host = text
                        self?.validate()
                    })
                rows.append(
                    fieldRow(
                        MavioStrings.rulesRemotePath, subtitle: MavioStrings.rulesRemotePathDetail, text: remote,
                        placeholder: "/downloads/"
                    ) { [weak self] text in
                        self?.remote = text
                        self?.validate()
                    })
                rows.append(
                    fieldRow(
                        MavioStrings.rulesLocalPath, subtitle: MavioStrings.rulesLocalPathDetail, text: local,
                        placeholder: "/mnt/downloads/"
                    ) { [weak self] text in
                        self?.local = text
                        self?.validate()
                    })
                let buttons = FormButtons(
                    valid: valid, busy: creating, failure: createFailure, theme: theme,
                    save: { [weak self] in self?.create() },
                    cancel: { [weak self] in
                        self?.adding = false
                        self?.render()
                    })
                self.buttons = buttons
                rows.append(row(buttons))
            }
            if let list, list.isEmpty, !adding { rows.append(note(MavioStrings.rulesNoMappings)) }
            if list == nil && failure == nil { rows.append(spinnerRow()) }
            if let failure { rows.append(note(failure, failed: true)) }
            var add: (String, @MainActor () -> Void)?
            if !adding {
                add = (
                    MavioStrings.rulesAdd,
                    { [weak self] in
                        self?.createFailure = nil
                        self?.adding = true
                        self?.render()
                    }
                )
            }
            return [card(MavioStrings.acquisitionMappings, button: add, rows)]
        }

        private var valid: Bool {
            let l = local.trimmingCharacters(in: .whitespaces)
            return !host.trimmingCharacters(in: .whitespaces).isEmpty
                && !remote.trimmingCharacters(in: .whitespaces).isEmpty
                && l.wholeMatch(of: /^(\/|[A-Za-z]:[\\\/]).*/) != nil
        }

        private func validate() { buttons?.update(valid: valid) }

        private func create() {
            endEditing()
            var mapping = Mavio_Acquisition_V1_RemotePathMapping()
            mapping.host = host.trimmingCharacters(in: .whitespaces)
            mapping.remotePath = remote.trimmingCharacters(in: .whitespaces)
            mapping.localPath = local.trimmingCharacters(in: .whitespaces)
            var request = Mavio_Acquisition_V1_CreateRemotePathMappingRequest()
            request.mapping = mapping
            creating = true
            createFailure = nil
            render()
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("CreateRemotePathMapping"), request,
                        as: Mavio_Acquisition_V1_CreateRemotePathMappingResponse.self)
                    guard let self else { return }
                    self.creating = false
                    self.adding = false
                    (self.host, self.remote, self.local) = ("", "", "")
                    await self.load()
                } catch {
                    self?.creating = false
                    self?.createFailure = error.reason
                    self?.render()
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteRemotePathMappingRequest()
            request.id = id
            removing = id
            render()
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteRemotePathMapping"), request,
                        as: Mavio_Acquisition_V1_DeleteRemotePathMappingResponse.self)
                    self?.removing = nil
                    await self?.load()
                } catch {
                    self?.removing = nil
                    self?.failure = error.reason
                    self?.render()
                }
            }
        }

        private func load() async {
            do {
                list = try await model.call(
                    server, Procedure.profiles("ListRemotePathMappings"),
                    Mavio_Acquisition_V1_ListRemotePathMappingsRequest(),
                    as: Mavio_Acquisition_V1_ListRemotePathMappingsResponse.self
                ).mappings
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }
    }
#endif
