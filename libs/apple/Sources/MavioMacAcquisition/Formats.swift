#if os(macOS)
    import AppKit
    import MavioMac

    private typealias Kind = Mavio_Acquisition_V1_CustomFormatSpecKind

    /// The kinds of conditions, named.
    @MainActor private var kinds: [(kind: Kind, title: String)] {
        [
            (.releaseTitle, MavioStrings.formatsReleaseTitle),
            (.edition, MavioStrings.formatsEdition),
            (.language, MavioStrings.formatsLanguage),
            (.indexerFlag, MavioStrings.formatsIndexerFlag),
            (.source, MavioStrings.formatsSource),
            (.resolution, MavioStrings.formatsResolution),
            (.qualityModifier, MavioStrings.formatsModifier),
            (.size, MavioStrings.formatsSize),
            (.releaseGroup, MavioStrings.formatsReleaseGroup),
            (.year, MavioStrings.formatsYear),
            (.releaseType, MavioStrings.formatsReleaseType),
        ]
    }

    @MainActor private func kindTitle(_ k: Kind) -> String { kinds.first { $0.kind == k }?.title ?? "" }

    /// The values of the conditions that take one of a list, by the names libs/acquire gives them.
    private func valueChoices(_ k: Kind) -> [(value: String, label: String)]? {
        switch k {
        case .language:
            (["Original", "Unknown"] + acquisitionReleaseLanguages).map { ($0, $0) }
        case .indexerFlag:
            ["freeleech", "halfleech", "doubleupload", "internal", "scene", "exclusive", "nuked", "neutralleech"]
                .map { ($0, $0) }
        case .source:
            [
                ("cam", "CAM"), ("telesync", "Telesync"), ("telecine", "Telecine"), ("workprint", "Workprint"),
                ("dvd", "DVD"), ("tv", "TV"), ("webdl", "WEB-DL"), ("webrip", "WEBRip"), ("bluray", "Blu-ray"),
                ("unknown", "Unknown"),
            ]
        case .resolution:
            ["360", "480", "540", "576", "720", "1080", "2160"].map { ($0, "\($0)p") }
        case .qualityModifier:
            [
                ("none", "None"), ("regional", "Regional"), ("screener", "Screener"), ("rawhd", "Raw-HD"),
                ("brdisk", "BR-DISK"), ("remux", "Remux"),
            ]
        case .releaseType:
            [
                ("single_episode", "Single episode"), ("multi_episode", "Multi-episode"),
                ("season_pack", "Season pack"), ("unknown", "Unknown"),
            ]
        default: nil
        }
    }

    private func patterned(_ k: Kind) -> Bool { k == .releaseTitle || k == .edition || k == .releaseGroup }
    private func ranged(_ k: Kind) -> Bool { k == .size || k == .year }

    /// Acquisition's page of the custom formats, as the web UI's: named
    /// sets of conditions on releases, which quality profiles score.
    final class FormatsPage: FormController {
        private var formats: [Mavio_Acquisition_V1_CustomFormat]?
        private var failure: String?

        override func viewDidLoad() {
            title = MavioStrings.acquisitionFormats
            message = MavioStrings.formatsMessage
            super.viewDidLoad()
        }

        override func viewWillAppear() {
            super.viewWillAppear()
            // Coming back from a format, its changes show.
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var rows: [NSView] = (formats ?? []).map { f in
                FormRow.link(f.name, subtitle: detail(f), theme: theme) { [weak self] in self?.open(f.id) }
            }
            rows.append(addRow(MavioStrings.formatsAdd) { [weak self] in self?.open(nil) })
            if formats == nil && failure == nil { rows.append(spinnerRow()) }
            if let failure { rows.append(note(failure, failed: true)) }
            return [card(rows)]
        }

        private func open(_ id: String?) {
            push(FormatPage(context: context, server: server, id: id))
        }

        /// The kinds of a format's conditions, each once.
        private func detail(_ f: Mavio_Acquisition_V1_CustomFormat) -> String? {
            var seen: [String] = []
            for s in f.specifications {
                let t = kindTitle(s.kind)
                if !seen.contains(t) { seen.append(t) }
            }
            let d = seen.joined(separator: ", ")
            return d.isEmpty ? nil : d
        }

        private func load() async {
            do {
                formats = try await model.call(
                    server, Procedure.profiles("ListCustomFormats"), Mavio_Acquisition_V1_ListCustomFormatsRequest(),
                    as: Mavio_Acquisition_V1_ListCustomFormatsResponse.self
                ).formats.sorted { $0.name.localizedCompare($1.name) == .orderedAscending }
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }
    }

    /// Acquisition's page of a custom format, as the web UI's; a new one
    /// when id is nil: its name, whether renaming includes it, and its
    /// conditions.
    final class FormatPage: FormController {
        private let id: String?
        private var saved: Mavio_Acquisition_V1_CustomFormat?
        private var draft = Mavio_Acquisition_V1_CustomFormat()
        private var gone = false
        private var failure: String?
        private var saving = false
        private var saveFailure: String?
        private var notice: String?
        private var removeFailure: String?
        private var saveBar: SaveBar?

        /// The page of format id, or of a new one when id is nil.
        init(context: MacContext, server: ServerRecord, id: String?) {
            self.id = id
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            retitle()
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        private func retitle() {
            title =
                gone
                ? MavioStrings.acquisitionFormats
                : saved.flatMap { $0.name.isEmpty ? nil : $0.name } ?? MavioStrings.formatsNew
        }

        override func sections() -> [NSView] {
            var views: [NSView] = []
            if gone {
                views.append(card([note(MavioStrings.formatsNone)]))
            } else if let saved {
                views += form(saved)
            } else if failure == nil {
                views.append(card([spinnerRow()]))
            }
            if let failure { views.append(card([note(failure, failed: true)])) }
            return views
        }

        private func form(_ saved: Mavio_Acquisition_V1_CustomFormat) -> [NSView] {
            let isNew = saved.id.isEmpty
            var specs: [NSView] = draft.specifications.indices.map { specRow($0) }
            if draft.specifications.isEmpty { specs.append(note(MavioStrings.formatsNoConditions)) }
            var extra: [NSView] = []
            if !isNew {
                extra.append(
                    MavioButton(MavioStrings.formatsRemove, kind: .destructive, theme: theme) { [weak self] in
                        guard let self, let saved = self.saved else { return }
                        self.confirm(
                            MavioStrings.formatsRemoveConfirm(name: saved.name),
                            message: MavioStrings.formatsRemoveDetail,
                            action: MavioStrings.formatsRemove
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
                card([
                    fieldRow(MavioStrings.formatsName, text: draft.name) { [weak self] text in
                        self?.draft.name = text
                        self?.edited(render: false)
                    },
                    toggleRow(
                        MavioStrings.formatsRename, subtitle: MavioStrings.formatsRenameDetail,
                        isOn: draft.includeWhenRenaming
                    ) { [weak self] on in
                        self?.draft.includeWhenRenaming = on
                        self?.edited(render: false)
                    },
                ]),
                card(
                    MavioStrings.formatsConditions,
                    button: (
                        MavioStrings.formatsAddCondition,
                        { [weak self] in
                            var s = Mavio_Acquisition_V1_CustomFormatSpec()
                            s.kind = .releaseTitle
                            self?.draft.specifications.append(s)
                            self?.edited()
                        }
                    ), footer: MavioStrings.formatsConditionsFooter, specs),
                bar,
            ]
            if let removeFailure { views.append(plain(label(removeFailure, theme.colors.danger))) }
            return views
        }

        /// A condition: its kind and name, what it matches, and how it counts.
        private func specRow(_ i: Int) -> NSView {
            let spec = draft.specifications[i]
            let list = kinds
            let kind = PopUp(
                list.map(\.title), selected: list.firstIndex { $0.kind == spec.kind } ?? 0,
                label: MavioStrings.formatsKind
            ) { [weak self] k in
                self?.change(i, render: true) {
                    $0.kind = list[k].kind
                    $0.value = ""
                    $0.min = 0
                    $0.max = 0
                    $0.exceptLanguage = false
                }
            }
            let name = TextInput(spec.name, placeholder: MavioStrings.formatsConditionName) { [weak self] text in
                self?.change(i) { $0.name = text }
            }
            name.widthAnchor.constraint(greaterThanOrEqualToConstant: 160).isActive = true
            name.setContentHuggingPriority(.init(1), for: .horizontal)
            let remove = MavioButton(MavioStrings.formatsRemoveCondition, kind: .destructive, theme: theme) {
                [weak self] in
                self?.draft.specifications.remove(at: i)
                self?.edited()
            }
            var details: [NSView] = []
            if patterned(spec.kind) {
                let pattern = TextInput(spec.value, placeholder: MavioStrings.formatsPattern, monospaced: true) {
                    [weak self] text in self?.change(i) { $0.value = text }
                }
                pattern.widthAnchor.constraint(greaterThanOrEqualToConstant: 240).isActive = true
                details.append(pattern)
            }
            if let values = valueChoices(spec.kind) {
                var options = values.map { (label: $0.label, value: $0.value) }
                if !values.contains(where: { $0.value == spec.value }) {
                    options.insert(
                        (spec.value.isEmpty ? MavioStrings.formatsChoose : spec.value, spec.value), at: 0)
                }
                details.append(
                    PopUp(
                        options.map(\.label), selected: options.firstIndex { $0.value == spec.value } ?? 0,
                        label: MavioStrings.formatsValue
                    ) { [weak self] k in self?.change(i, render: true) { $0.value = options[k].value } })
            }
            if ranged(spec.kind) {
                let step = spec.kind == .size ? 0.1 : 1
                var range: [NSView] = [
                    NumberField(MavioStrings.formatsMin, value: spec.min, step: step, theme: theme) { [weak self] v in
                        self?.change(i) { $0.min = v }
                    },
                    NSTextField.label("–", theme.fonts.callout, color: theme.colors.textSecondary),
                    NumberField(MavioStrings.formatsMax, value: spec.max, step: step, theme: theme) { [weak self] v in
                        self?.change(i) { $0.max = v }
                    },
                ]
                if spec.kind == .size {
                    range.append(
                        NSTextField.label(
                            MavioStrings.formatsGigabytes, theme.fonts.callout, color: theme.colors.textSecondary))
                }
                details.append(NSStackView.horizontal(range, spacing: MavioSpacing.s8))
            }
            if spec.kind == .language {
                details.append(
                    Check(MavioStrings.formatsExceptLanguage, isOn: spec.exceptLanguage, theme: theme) {
                        [weak self] on in
                        self?.change(i) { $0.exceptLanguage = on }
                    })
            }
            details.append(
                Check(MavioStrings.formatsNegate, isOn: spec.negate, theme: theme) { [weak self] on in
                    self?.change(i) { $0.negate = on }
                })
            details.append(
                Check(MavioStrings.formatsRequired, isOn: spec.required, theme: theme) { [weak self] on in
                    self?.change(i) { $0.required = on }
                })
            details.append(Spacer())
            let top = NSStackView.horizontal([kind, name, remove], spacing: MavioSpacing.s8)
            let bottom = NSStackView.horizontal(details, spacing: MavioSpacing.s16)
            let stack = NSStackView.vertical([top, bottom], spacing: MavioSpacing.s8)
            for view in [top, bottom] { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return row(stack)
        }

        /// Changes condition i, drawing the page again when render.
        private func change(
            _ i: Int, render again: Bool = false, _ body: (inout Mavio_Acquisition_V1_CustomFormatSpec) -> Void
        ) {
            guard draft.specifications.indices.contains(i) else { return }
            body(&draft.specifications[i])
            edited(render: again)
        }

        private func edited(render again: Bool = true) {
            notice = nil
            if again { render() } else { refresh() }
        }

        private func refresh() {
            guard let saved, let saveBar else { return }
            let changed = !acquisitionSame(draft, saved)
            let complete = draft.specifications.allSatisfy { s in
                !s.name.trimmingCharacters(in: .whitespaces).isEmpty
                    && (ranged(s.kind) ? s.max > s.min : !s.value.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            saveBar.update(
                enabled: changed && !draft.name.trimmingCharacters(in: .whitespaces).isEmpty && complete,
                busy: saving, notice: changed ? nil : notice, failure: saveFailure)
        }

        private func load() async {
            guard let id else {
                (saved, draft) = (.init(), .init())
                render()
                return
            }
            do {
                let formats = try await model.call(
                    server, Procedure.profiles("ListCustomFormats"), Mavio_Acquisition_V1_ListCustomFormatsRequest(),
                    as: Mavio_Acquisition_V1_ListCustomFormatsResponse.self
                ).formats
                if let f = formats.first(where: { $0.id == id }) {
                    (saved, draft) = (f, f)
                } else {
                    gone = true
                }
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
            guard let saved else { return }
            var f = draft
            f.name = f.name.trimmingCharacters(in: .whitespaces)
            saving = true
            saveFailure = nil
            refresh()
            let creating = saved.id.isEmpty
            run { [weak self, model, server] in
                do {
                    if creating {
                        var request = Mavio_Acquisition_V1_CreateCustomFormatRequest()
                        request.format = f
                        _ = try await model.call(
                            server, Procedure.profiles("CreateCustomFormat"), request,
                            as: Mavio_Acquisition_V1_CreateCustomFormatResponse.self)
                        self?.saving = false
                        self?.dismiss()
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateCustomFormatRequest()
                        request.format = f
                        let result = try await model.call(
                            server, Procedure.profiles("UpdateCustomFormat"), request,
                            as: Mavio_Acquisition_V1_UpdateCustomFormatResponse.self
                        ).format
                        guard let self else { return }
                        (self.saved, self.draft) = (result, result)
                        self.notice = MavioStrings.acquisitionSaved
                        self.saving = false
                        self.retitle()
                        self.render()
                    }
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }

        private func remove() {
            guard let saved else { return }
            var request = Mavio_Acquisition_V1_DeleteCustomFormatRequest()
            request.id = saved.id
            removeFailure = nil
            run { [weak self, model, server] in
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteCustomFormat"), request,
                        as: Mavio_Acquisition_V1_DeleteCustomFormatResponse.self)
                    self?.dismiss()
                } catch {
                    self?.removeFailure = error.reason
                    self?.render()
                }
            }
        }
    }
#endif
