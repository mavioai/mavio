#if os(macOS)
    import AppKit
    import MavioMac

    // MARK: Quality definitions

    /// Acquisition's page of the sizes of each kind's qualities, as the web
    /// UI's: a definition's minimum, preferred and maximum size, saved
    /// together.
    final class QualityDefinitionsPage: FormController {
        private var kind = Mavio_Acquisition_V1_MediaKind.movie
        private var list: [Mavio_Acquisition_V1_QualityDefinition]?
        private var edits: [Int32: Mavio_Acquisition_V1_QualityDefinition] = [:]
        private var failure: String?
        private var saving = false
        private var saveFailure: String?
        private var notice: String?
        private var loading: Task<Void, Never>?
        private lazy var saveBar = SaveBar(MavioStrings.actionSave, theme: theme) { [weak self] in self?.save() }

        override func viewDidLoad() {
            title = MavioStrings.acquisitionQualities
            message = MavioStrings.acquisitionQualitiesMessage
            super.viewDidLoad()
            load()
        }

        isolated deinit { loading?.cancel() }

        override func sections() -> [NSView] {
            let kinds = acquisitionMediaKinds
            let picker = PopUp(
                kinds.map(\.title), selected: kinds.firstIndex { $0.kind == kind } ?? 0,
                label: MavioStrings.acquisitionKind
            ) { [weak self] i in
                guard let self else { return }
                self.kind = kinds[i].kind
                self.load()
            }
            var rows: [NSView] = []
            let heads = [
                MavioStrings.acquisitionMinSize, MavioStrings.acquisitionPreferredSize, MavioStrings.acquisitionMaxSize,
            ]
            rows.append(
                row(
                    line(
                        NSTextField.label(
                            MavioStrings.acquisitionQuality, theme.fonts.footnote, color: theme.colors.textSecondary),
                        heads.map { NSTextField.label($0, theme.fonts.footnote, color: theme.colors.textSecondary) })))
            for saved in list ?? [] {
                let d = edits[saved.quality] ?? saved
                rows.append(
                    row(
                        line(
                            NSTextField.label(d.title, theme.fonts.body, color: theme.colors.textPrimary),
                            [
                                size(d, \.minSize, MavioStrings.acquisitionMinSize),
                                size(d, \.preferredSize, MavioStrings.acquisitionPreferredSize),
                                size(d, \.maxSize, MavioStrings.acquisitionMaxSize),
                            ])))
            }
            if list == nil && failure == nil { rows.append(spinnerRow()) }
            if let failure { rows.append(note(failure, failed: true)) }
            refresh()
            return [
                plain(NSStackView.horizontal([picker, Spacer()])),
                card(footer: MavioStrings.acquisitionQualitiesFooter, rows),
                saveBar,
            ]
        }

        /// A line of the table: the quality, then its three sizes.
        private func line(_ first: NSView, _ cells: [NSView]) -> NSView {
            var views: [NSView] = [first]
            for cell in cells {
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addSubview(cell)
                cell.translatesAutoresizingMaskIntoConstraints = false
                NSLayoutConstraint.activate([
                    cell.leadingAnchor.constraint(equalTo: holder.leadingAnchor),
                    cell.centerYAnchor.constraint(equalTo: holder.centerYAnchor),
                    holder.heightAnchor.constraint(greaterThanOrEqualTo: cell.heightAnchor),
                    holder.widthAnchor.constraint(equalToConstant: 110),
                ])
                views.append(holder)
            }
            first.setContentHuggingPriority(.init(1), for: .horizontal)
            return NSStackView.horizontal(views, spacing: MavioSpacing.s12)
        }

        private func size(
            _ d: Mavio_Acquisition_V1_QualityDefinition,
            _ field: WritableKeyPath<Mavio_Acquisition_V1_QualityDefinition, Double>, _ label: String
        ) -> NSView {
            let quality = d.quality
            return NumberField("\(d.title) \(label)", value: d[keyPath: field], step: 0.1, theme: theme) {
                [weak self] value in
                guard let self, let saved = self.list?.first(where: { $0.quality == quality }) else { return }
                var e = self.edits[quality] ?? saved
                e[keyPath: field] = value
                self.edits[quality] = e
                self.notice = nil
                self.refresh()
            }
        }

        /// The definitions changed from what is saved.
        private var changed: [Mavio_Acquisition_V1_QualityDefinition] {
            edits.values.filter { d in
                guard let saved = list?.first(where: { $0.quality == d.quality }) else { return false }
                return saved.minSize != d.minSize || saved.preferredSize != d.preferredSize
                    || saved.maxSize != d.maxSize
            }
            .sorted { $0.quality < $1.quality }
        }

        private func refresh() {
            let changed = changed
            saveBar.update(
                enabled: !changed.isEmpty, busy: saving, notice: changed.isEmpty ? notice : nil, failure: saveFailure)
        }

        private func load() {
            loading?.cancel()
            list = nil
            edits = [:]
            notice = nil
            saveFailure = nil
            failure = nil
            render()
            var request = Mavio_Acquisition_V1_ListQualityDefinitionsRequest()
            request.kind = kind
            loading = Task { [weak self, model, server] in
                do {
                    let definitions = try await model.call(
                        server, Procedure.profiles("ListQualityDefinitions"), request,
                        as: Mavio_Acquisition_V1_ListQualityDefinitionsResponse.self
                    ).definitions
                    guard let self, !Task.isCancelled else { return }
                    self.list = definitions
                } catch is CancellationError {
                    return
                } catch {
                    self?.failure = error.reason
                }
                self?.render()
            }
        }

        private func save() {
            endEditing()
            var request = Mavio_Acquisition_V1_UpdateQualityDefinitionsRequest()
            request.kind = kind
            request.definitions = changed
            saving = true
            saveFailure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let list = try await model.call(
                        server, Procedure.profiles("UpdateQualityDefinitions"), request,
                        as: Mavio_Acquisition_V1_UpdateQualityDefinitionsResponse.self
                    ).definitions
                    guard let self else { return }
                    self.list = list
                    self.edits = [:]
                    self.notice = MavioStrings.acquisitionSaved
                    self.saving = false
                    self.render()
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }
    }

    // MARK: Quality profiles

    /// The ID an item is ranked by: a group's, else its quality's.
    private func itemID(_ it: Mavio_Acquisition_V1_QualityProfileItem) -> Int32 {
        it.id != 0 ? it.id : it.quality
    }

    /// Acquisition's page of a quality profile, as the web UI's; a new one
    /// of a kind when id is nil: its name, the qualities it allows, best
    /// first, where upgrades stop, the language of movies, and the scores
    /// of custom formats.
    final class QualityProfilePage: FormController {
        private let kind: Mavio_Acquisition_V1_MediaKind
        /// The profile's ID; nil until a new one is created.
        private var id: String?
        private var saved: Mavio_Acquisition_V1_QualityProfile?
        private var draft: Mavio_Acquisition_V1_QualityProfile?
        private var titles: [Int32: String] = [:]
        private var formats: [Mavio_Acquisition_V1_CustomFormat]?
        private var gone = false
        private var failure: String?
        private var saving = false
        private var saveFailure: String?
        private var notice: String?
        private var removeFailure: String?
        private var saveBar: SaveBar?

        /// The page of profile id, or of a new one of kind when id is nil.
        init(context: MacContext, server: ServerRecord, id: String?, kind: Mavio_Acquisition_V1_MediaKind) {
            self.id = id
            self.kind = kind
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            message = acquisitionMediaKinds.first { $0.kind == kind }?.title
            retitle()
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        private func retitle() {
            title =
                gone
                ? MavioStrings.acquisitionProfile
                : saved.flatMap { $0.name.isEmpty ? nil : $0.name } ?? MavioStrings.acquisitionNewProfile
        }

        private var isNew: Bool { saved?.id.isEmpty ?? true }

        private func nameOf(_ it: Mavio_Acquisition_V1_QualityProfileItem) -> String {
            it.id != 0 ? it.name : titles[it.quality] ?? String(it.quality)
        }

        override func sections() -> [NSView] {
            if gone { return [card([note(MavioStrings.acquisitionNoProfile)])] }
            guard let draft else {
                return [card([failure.map { note($0, failed: true) } ?? spinnerRow()])]
            }
            var views = form(draft)
            if let failure { views.append(card([note(failure, failed: true)])) }
            return views
        }

        private func form(_ draft: Mavio_Acquisition_V1_QualityProfile) -> [NSView] {
            let allowed = draft.items.filter(\.allowed)
            var general: [NSView] = [
                fieldRow(MavioStrings.acquisitionProfileName, text: draft.name) { [weak self] text in
                    self?.draft?.name = text
                    self?.edited(render: false)
                },
                toggleRow(
                    MavioStrings.acquisitionUpgradeAllowed, subtitle: MavioStrings.acquisitionUpgradeAllowedDetail,
                    isOn: draft.upgradeAllowed
                ) { [weak self] on in
                    self?.draft?.upgradeAllowed = on
                    self?.edited()
                },
            ]
            if draft.upgradeAllowed {
                var options: [(String, Int32)] = []
                if !allowed.contains(where: { itemID($0) == draft.cutoff }) { options.append(("—", draft.cutoff)) }
                options += allowed.reversed().map { (nameOf($0), itemID($0)) }
                general.append(
                    choiceRow(
                        MavioStrings.acquisitionCutoff, subtitle: MavioStrings.acquisitionCutoffDetail,
                        options: options,
                        selected: draft.cutoff
                    ) { [weak self] value in
                        self?.draft?.cutoff = value
                        self?.edited()
                    })
            }
            if draft.kind == .movie {
                let options: [(String, String)] =
                    [
                        (MavioStrings.acquisitionLanguageAny, ""),
                        (MavioStrings.acquisitionLanguageOriginal, "Original"),
                        (MavioStrings.acquisitionLanguageUnknown, "Unknown"),
                    ]
                    + acquisitionReleaseLanguages.sorted { $0.localizedCompare($1) == .orderedAscending }.map {
                        ($0, $0)
                    }
                general.append(
                    choiceRow(
                        MavioStrings.acquisitionLanguage, subtitle: MavioStrings.acquisitionLanguageDetail,
                        options: options, selected: draft.language == "Any" ? "" : draft.language
                    ) { [weak self] value in
                        self?.draft?.language = value
                        self?.edited()
                    })
            }
            let count = draft.items.count
            let qualities: [NSView] = draft.items.indices.reversed().map { i in
                qualityRow(draft.items[i], at: i, of: count)
            }
            var scores: [NSView] = (formats ?? []).map { f in
                let id = f.id
                return numberRow(
                    f.name, value: Double(draft.formatScores.first { $0.formatID == id }?.score ?? 0),
                    range: -100_000...100_000
                ) { [weak self] value in self?.setScore(id, value) }
            }
            if formats?.isEmpty == true { scores.append(note(MavioStrings.acquisitionNoFormats)) }
            scores.append(
                scoreRow(
                    \.minFormatScore, MavioStrings.acquisitionMinFormatScore,
                    MavioStrings.acquisitionMinFormatScoreDetail))
            if draft.upgradeAllowed {
                scores.append(
                    scoreRow(
                        \.cutoffFormatScore, MavioStrings.acquisitionCutoffFormatScore,
                        MavioStrings.acquisitionCutoffFormatScoreDetail))
                scores.append(
                    scoreRow(
                        \.minUpgradeFormatScore, MavioStrings.acquisitionMinUpgradeFormatScore,
                        MavioStrings.acquisitionMinUpgradeFormatScoreDetail, min: 1))
            }
            var extra: [NSView] = []
            if !isNew {
                extra.append(
                    MavioButton(MavioStrings.acquisitionRemoveProfile, kind: .destructive, theme: theme) {
                        [weak self] in
                        guard let self, let saved = self.saved else { return }
                        self.confirm(
                            MavioStrings.acquisitionRemoveProfileConfirm(name: saved.name),
                            message: MavioStrings.acquisitionRemoveProfileDetail,
                            action: MavioStrings.acquisitionRemoveProfile
                        ) { [weak self] in self?.remove() }
                    })
            }
            let bar = SaveBar(
                isNew ? MavioStrings.acquisitionCreate : MavioStrings.actionSave, theme: theme, extra: extra
            ) {
                [weak self] in self?.save()
            }
            saveBar = bar
            refresh()
            var views: [NSView] = [
                card(general),
                card(
                    MavioStrings.acquisitionProfileQualities, footer: MavioStrings.acquisitionProfileQualitiesFooter,
                    qualities),
                card(MavioStrings.acquisitionFormatScores, footer: MavioStrings.acquisitionFormatScoresFooter, scores),
                bar,
            ]
            if let removeFailure { views.append(plain(label(removeFailure, theme.colors.danger))) }
            return views
        }

        private func qualityRow(_ it: Mavio_Acquisition_V1_QualityProfileItem, at i: Int, of count: Int) -> NSView {
            let toggle = Switch(isOn: it.allowed, label: nameOf(it)) { [weak self] _ in self?.toggle(i) }
            var labels: [NSView] = [
                NSTextField.label(
                    nameOf(it), theme.fonts.body,
                    color: it.allowed ? theme.colors.textPrimary : theme.colors.textSecondary)
            ]
            if it.id > 0 {
                labels.append(
                    NSTextField.label(
                        it.items.map(nameOf).joined(separator: ", "), theme.fonts.footnote,
                        color: theme.colors.textSecondary, lines: 0))
            }
            let text = NSStackView.vertical(labels, spacing: 2)
            text.setContentHuggingPriority(.init(1), for: .horizontal)
            // The list shows best first, the items are kept worst first.
            let moves = moveButtons(at: count - 1 - i, of: count) { [weak self] _, by in self?.move(i, -by) }
            return row(NSStackView.horizontal([toggle, text, moves], spacing: MavioSpacing.s12))
        }

        private func scoreRow(
            _ field: WritableKeyPath<Mavio_Acquisition_V1_QualityProfile, Int32>, _ title: String, _ detail: String,
            min: Double = -100_000
        ) -> FormRow {
            numberRow(title, subtitle: detail, value: Double(draft?[keyPath: field] ?? 0), range: min...100_000) {
                [weak self] value in
                self?.draft?[keyPath: field] = Int32(value.rounded())
                self?.edited(render: false)
            }
        }

        private func setScore(_ formatID: String, _ value: Double) {
            guard var d = draft else { return }
            var scores = d.formatScores.filter { $0.formatID != formatID }
            if value != 0 {
                var s = Mavio_Acquisition_V1_FormatScore()
                s.formatID = formatID
                s.score = Int32(value.rounded())
                scores.append(s)
            }
            d.formatScores = scores
            draft = d
            edited(render: false)
        }

        /// Notes the draft changed, drawing the page again when render.
        private func edited(render again: Bool = true) {
            notice = nil
            if again { render() } else { refresh() }
        }

        private func refresh() {
            guard let draft, let saved, let saveBar else { return }
            let changed = !acquisitionSame(draft, saved)
            saveBar.update(
                enabled: changed && !draft.name.trimmingCharacters(in: .whitespaces).isEmpty
                    && draft.items.contains(where: \.allowed),
                busy: saving, notice: changed ? nil : notice, failure: saveFailure)
        }

        /// The items set, the cutoff moved to the best allowed when it is no longer allowed.
        private func setItems(_ items: [Mavio_Acquisition_V1_QualityProfileItem]) {
            guard var d = draft else { return }
            let cutoffAllowed = items.contains { $0.allowed && itemID($0) == d.cutoff }
            if !cutoffAllowed, let top = items.last(where: \.allowed) { d.cutoff = itemID(top) }
            d.items = items
            draft = d
            edited()
        }

        private func toggle(_ i: Int) {
            guard var items = draft?.items else { return }
            let on = !items[i].allowed
            items[i].allowed = on
            for j in items[i].items.indices { items[i].items[j].allowed = on }
            setItems(items)
        }

        private func move(_ i: Int, _ by: Int) {
            guard var items = draft?.items else { return }
            let it = items.remove(at: i)
            items.insert(it, at: i + by)
            setItems(items)
        }

        private func load() async {
            let model = model
            let server = server
            let kind = kind
            do {
                async let fs = model.call(
                    server, Procedure.profiles("ListCustomFormats"), Mavio_Acquisition_V1_ListCustomFormatsRequest(),
                    as: Mavio_Acquisition_V1_ListCustomFormatsResponse.self)
                var dr = Mavio_Acquisition_V1_ListQualityDefinitionsRequest()
                dr.kind = kind
                let definitions = try await model.call(
                    server, Procedure.profiles("ListQualityDefinitions"), dr,
                    as: Mavio_Acquisition_V1_ListQualityDefinitionsResponse.self
                ).definitions
                titles = Dictionary(definitions.map { ($0.quality, $0.title) }, uniquingKeysWith: { a, _ in a })
                if let id {
                    let profiles = try await model.call(
                        server, Procedure.profiles("ListQualityProfiles"),
                        Mavio_Acquisition_V1_ListQualityProfilesRequest(),
                        as: Mavio_Acquisition_V1_ListQualityProfilesResponse.self
                    ).profiles
                    guard let p = profiles.first(where: { $0.id == id }) else {
                        gone = true
                        retitle()
                        render()
                        return
                    }
                    (saved, draft) = (p, p)
                } else {
                    var p = Mavio_Acquisition_V1_QualityProfile()
                    p.kind = kind
                    p.upgradeAllowed = true
                    p.items = definitions.map {
                        var it = Mavio_Acquisition_V1_QualityProfileItem()
                        it.quality = $0.quality
                        it.allowed = false
                        return it
                    }
                    (saved, draft) = (p, p)
                }
                formats = try await fs.formats.sorted { $0.name.localizedCompare($1.name) == .orderedAscending }
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            retitle()
            render()
        }

        private func save() {
            endEditing()
            guard var p = draft else { return }
            p.name = p.name.trimmingCharacters(in: .whitespaces)
            saving = true
            saveFailure = nil
            refresh()
            let creating = isNew
            run { [weak self, model, server] in
                do {
                    let result: Mavio_Acquisition_V1_QualityProfile
                    var notice: String?
                    if creating {
                        var request = Mavio_Acquisition_V1_CreateQualityProfileRequest()
                        request.profile = p
                        result = try await model.call(
                            server, Procedure.profiles("CreateQualityProfile"), request,
                            as: Mavio_Acquisition_V1_CreateQualityProfileResponse.self
                        ).profile
                        self?.id = result.id
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateQualityProfileRequest()
                        request.profile = p
                        result = try await model.call(
                            server, Procedure.profiles("UpdateQualityProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateQualityProfileResponse.self
                        ).profile
                        notice = MavioStrings.acquisitionSaved
                    }
                    guard let self else { return }
                    (self.saved, self.draft) = (result, result)
                    if let notice { self.notice = notice }
                    self.saving = false
                    self.retitle()
                    self.render()
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }

        private func remove() {
            guard let saved else { return }
            var request = Mavio_Acquisition_V1_DeleteQualityProfileRequest()
            request.id = saved.id
            removeFailure = nil
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteQualityProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteQualityProfileResponse.self)
                    self?.dismiss()
                } catch {
                    self?.removeFailure = error.reason
                    self?.render()
                }
            }
        }
    }
#endif
