#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    private typealias Kind = Mavio_Acquisition_V1_CustomFormatSpecKind

    /// The kinds of conditions, named.
    private var kinds: [(kind: Kind, title: String)] {
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

    private func kindTitle(_ k: Kind) -> String { kinds.first { $0.kind == k }?.title ?? "" }

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

    /// Acquisition's page of the custom formats, as the web UI's: named sets of conditions on releases, which
    /// quality profiles score.
    struct AcquisitionFormatsView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var formats: [Mavio_Acquisition_V1_CustomFormat]?
        @State private var failure: String?

        var body: some View {
            SettingsList(message: MavioStrings.formatsMessage) {
                Section {
                    ForEach(formats ?? [], id: \.id) { f in
                        NavigationLink {
                            AcquisitionFormatView(server: server, id: f.id)
                        } label: {
                            RowLabel(label: f.name, detail: detail(f))
                        }
                        .rowLink()
                    }
                    NavigationLink {
                        AcquisitionFormatView(server: server, id: nil)
                    } label: {
                        Text(MavioStrings.formatsAdd).foregroundStyle(theme.colors.accentText)
                    }
                    .rowLink()
                    if let failure { AcquisitionNote(text: failure, failed: true) }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.acquisitionFormats)
            .inlineTitle()
            .task { await load() }
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
            } catch {
                failure = error.reason
            }
        }
    }

    /// Acquisition's page of a custom format, as the web UI's; a new one when id is nil: its name, whether
    /// renaming includes it, and its conditions.
    struct AcquisitionFormatView: View {
        let server: ServerRecord
        let id: String?
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var saved: Mavio_Acquisition_V1_CustomFormat?
        @State private var draft = Mavio_Acquisition_V1_CustomFormat()
        @State private var gone = false
        @State private var failure: String?
        @State private var saving = false
        @State private var saveFailure: String?
        @State private var notice: String?
        @State private var removing = false
        @State private var removeFailure: String?

        var body: some View {
            SettingsList {
                if gone {
                    AcquisitionNote(text: MavioStrings.formatsNone)
                } else if let saved {
                    form(saved)
                } else if failure == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
                if let failure { AcquisitionNote(text: failure, failed: true) }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(
                gone
                    ? MavioStrings.acquisitionFormats
                    : saved.flatMap { $0.name.isEmpty ? nil : $0.name } ?? MavioStrings.formatsNew
            )
            .inlineTitle()
            .task { if saved == nil { await load() } }
        }

        private var binding: Binding<Mavio_Acquisition_V1_CustomFormat> {
            Binding(
                get: { draft },
                set: {
                    draft = $0
                    notice = nil
                })
        }

        @ViewBuilder private func form(_ saved: Mavio_Acquisition_V1_CustomFormat) -> some View {
            let isNew = saved.id.isEmpty
            let changed = !acquisitionSame(draft, saved)
            let complete = draft.specifications.allSatisfy { s in
                !s.name.trimmingCharacters(in: .whitespaces).isEmpty
                    && (ranged(s.kind) ? s.max > s.min : !s.value.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            Section {
                FormRow(label: MavioStrings.formatsName) {
                    TextField(MavioStrings.formatsName, text: binding.name)
                        .fieldLabelHidden()
                        .frame(width: 280)
                }
                Toggle(isOn: binding.includeWhenRenaming) {
                    Text(MavioStrings.formatsRename)
                    Text(MavioStrings.formatsRenameDetail)
                }
            }
            Section {
                ForEach(draft.specifications.indices, id: \.self) { i in
                    SpecRow(
                        spec: Binding(
                            get: { draft.specifications.indices.contains(i) ? draft.specifications[i] : .init() },
                            set: {
                                if binding.wrappedValue.specifications.indices.contains(i) {
                                    binding.wrappedValue.specifications[i] = $0
                                }
                            }),
                        remove: { binding.wrappedValue.specifications.remove(at: i) })
                }
                if draft.specifications.isEmpty {
                    AcquisitionNote(text: MavioStrings.formatsNoConditions)
                }
            } header: {
                HStack(spacing: MavioSpacing.s12) {
                    Text(MavioStrings.formatsConditions)
                    Spacer(minLength: 0)
                    Button(MavioStrings.formatsAddCondition) {
                        var s = Mavio_Acquisition_V1_CustomFormatSpec()
                        s.kind = .releaseTitle
                        binding.wrappedValue.specifications.append(s)
                    }
                    .type(theme.type.body)
                }
            } footer: {
                Text(MavioStrings.formatsConditionsFooter)
            }
            Section {
            } header: {
                VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                    AcquisitionSaveBar(
                        title: isNew ? MavioStrings.acquisitionCreate : MavioStrings.actionSave,
                        enabled: changed && !draft.name.trimmingCharacters(in: .whitespaces).isEmpty && complete,
                        busy: saving, notice: changed ? nil : notice, failure: saveFailure, save: save)
                    if !isNew {
                        Button(MavioStrings.formatsRemove, role: .destructive) { removing = true }
                            .buttonStyle(.bordered)
                            .confirmationDialog(
                                MavioStrings.formatsRemoveConfirm(name: saved.name), isPresented: $removing,
                                titleVisibility: .visible
                            ) {
                                Button(MavioStrings.formatsRemove, role: .destructive, action: remove)
                            } message: {
                                Text(MavioStrings.formatsRemoveDetail)
                            }
                        if let removeFailure {
                            Text(removeFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                        }
                    }
                }
            }
        }

        private func load() async {
            guard let id else {
                (saved, draft) = (.init(), .init())
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
            } catch {
                failure = error.reason
            }
        }

        private func save() {
            endEditing()
            guard let saved else { return }
            var f = draft
            f.name = f.name.trimmingCharacters(in: .whitespaces)
            saving = true
            saveFailure = nil
            Task {
                defer { saving = false }
                do {
                    if saved.id.isEmpty {
                        var request = Mavio_Acquisition_V1_CreateCustomFormatRequest()
                        request.format = f
                        _ = try await model.call(
                            server, Procedure.profiles("CreateCustomFormat"), request,
                            as: Mavio_Acquisition_V1_CreateCustomFormatResponse.self)
                        dismiss()
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateCustomFormatRequest()
                        request.format = f
                        let result = try await model.call(
                            server, Procedure.profiles("UpdateCustomFormat"), request,
                            as: Mavio_Acquisition_V1_UpdateCustomFormatResponse.self
                        ).format
                        (self.saved, draft) = (result, result)
                        notice = MavioStrings.acquisitionSaved
                    }
                } catch {
                    saveFailure = error.reason
                }
            }
        }

        private func remove() {
            guard let saved else { return }
            var request = Mavio_Acquisition_V1_DeleteCustomFormatRequest()
            request.id = saved.id
            removeFailure = nil
            Task {
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteCustomFormat"), request,
                        as: Mavio_Acquisition_V1_DeleteCustomFormatResponse.self)
                    dismiss()
                } catch {
                    removeFailure = error.reason
                }
            }
        }
    }

    /// A condition of a custom format: its kind and name, what it matches, and how it counts.
    private struct SpecRow: View {
        @Binding var spec: Mavio_Acquisition_V1_CustomFormatSpec
        let remove: () -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                HStack(spacing: MavioSpacing.s8) {
                    Picker(
                        MavioStrings.formatsKind,
                        selection: Binding(
                            get: { spec.kind },
                            set: {
                                spec.kind = $0
                                spec.value = ""
                                spec.min = 0
                                spec.max = 0
                                spec.exceptLanguage = false
                            })
                    ) {
                        ForEach(kinds, id: \.kind) { Text($0.title).tag($0.kind) }
                    }
                    .labelsHidden()
                    .fixedSize()
                    TextField(
                        MavioStrings.formatsConditionName, text: $spec.name,
                        prompt: Text(MavioStrings.formatsConditionName)
                    )
                    .labelsHidden()
                    .frame(minWidth: 160)
                    Button(MavioStrings.formatsRemoveCondition, role: .destructive, action: remove)
                        .foregroundStyle(theme.colors.danger)
                }
                HStack(spacing: MavioSpacing.s16) {
                    if patterned(spec.kind) {
                        TextField(
                            MavioStrings.formatsPattern, text: $spec.value, prompt: Text(MavioStrings.formatsPattern)
                        )
                        .labelsHidden()
                        .font(.system(.body, design: .monospaced))
                        .frame(minWidth: 240)
                    }
                    if let list = valueChoices(spec.kind) {
                        Picker(MavioStrings.formatsValue, selection: $spec.value) {
                            if !list.contains(where: { $0.value == spec.value }) {
                                Text(spec.value.isEmpty ? MavioStrings.formatsChoose : spec.value).tag(spec.value)
                            }
                            ForEach(list, id: \.value) { Text($0.label).tag($0.value) }
                        }
                        .labelsHidden()
                        .fixedSize()
                    }
                    if ranged(spec.kind) {
                        HStack(spacing: MavioSpacing.s8) {
                            AcquisitionNumberField(
                                label: MavioStrings.formatsMin, value: $spec.min, step: spec.kind == .size ? 0.1 : 1)
                            Text("–")
                            AcquisitionNumberField(
                                label: MavioStrings.formatsMax, value: $spec.max, step: spec.kind == .size ? 0.1 : 1)
                            if spec.kind == .size { Text(MavioStrings.formatsGigabytes) }
                        }
                        .type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                    }
                    if spec.kind == .language {
                        check(MavioStrings.formatsExceptLanguage, $spec.exceptLanguage)
                    }
                    check(MavioStrings.formatsNegate, $spec.negate)
                    check(MavioStrings.formatsRequired, $spec.required)
                }
            }
            .padding(.vertical, MavioSpacing.s4)
        }

        private func check(_ label: String, _ on: Binding<Bool>) -> some View {
            Toggle(label, isOn: on).toggleStyle(.checkbox).type(theme.type.callout)
        }
    }
#endif
