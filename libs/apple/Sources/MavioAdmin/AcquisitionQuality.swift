#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// The languages releases are parsed into, by their English names, as libs/release names them.
    let acquisitionReleaseLanguages = [
        "English", "French", "Spanish", "German", "Italian", "Danish", "Dutch", "Japanese", "Icelandic", "Chinese",
        "Russian", "Polish", "Vietnamese", "Swedish", "Norwegian", "Finnish", "Turkish", "Portuguese", "Flemish",
        "Greek", "Korean", "Hungarian", "Hebrew", "Lithuanian", "Czech", "Arabic", "Hindi", "Bulgarian", "Malayalam",
        "Ukrainian", "Slovak", "Thai", "Portuguese (Brazil)", "Spanish (Latino)", "Romanian", "Latvian", "Persian",
        "Catalan", "Croatian", "Serbian", "Bosnian", "Estonian", "Tamil", "Indonesian", "Macedonian", "Slovenian",
        "Bengali", "Telugu", "Kannada", "Albanian", "Afrikaans", "Marathi", "Tagalog", "Urdu", "Romansh", "Mongolian",
        "Georgian",
    ]

    // MARK: Quality definitions

    /// Acquisition's page of the sizes of each kind's qualities, as the web UI's.
    struct AcquisitionQualityDefinitionsView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var kind = Mavio_Acquisition_V1_MediaKind.movie
        @State private var list: [Mavio_Acquisition_V1_QualityDefinition]?
        @State private var edits: [Int32: Mavio_Acquisition_V1_QualityDefinition] = [:]
        @State private var failure: String?
        @State private var saving = false
        @State private var saveFailure: String?
        @State private var notice: String?

        var body: some View {
            SettingsList(message: MavioStrings.acquisitionQualitiesMessage) {
                Section {
                } header: {
                    Picker(MavioStrings.acquisitionKind, selection: $kind) {
                        ForEach(acquisitionMediaKinds, id: \.kind) { Text($0.title).tag($0.kind) }
                    }
                    .labelsHidden()
                    .fixedSize()
                }
                Section {
                    HStack(spacing: MavioSpacing.s12) {
                        Text(MavioStrings.acquisitionQuality).frame(maxWidth: .infinity, alignment: .leading)
                        column(MavioStrings.acquisitionMinSize)
                        column(MavioStrings.acquisitionPreferredSize)
                        column(MavioStrings.acquisitionMaxSize)
                    }
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
                    ForEach(list ?? [], id: \.quality) { saved in
                        let d = edits[saved.quality] ?? saved
                        HStack(spacing: MavioSpacing.s12) {
                            Text(d.title).frame(maxWidth: .infinity, alignment: .leading)
                            size(d, \.minSize, MavioStrings.acquisitionMinSize)
                            size(d, \.preferredSize, MavioStrings.acquisitionPreferredSize)
                            size(d, \.maxSize, MavioStrings.acquisitionMaxSize)
                        }
                    }
                    if list == nil && failure == nil {
                        ProgressView().frame(maxWidth: .infinity)
                    }
                    if let failure { AcquisitionNote(text: failure, failed: true) }
                } footer: {
                    Text(MavioStrings.acquisitionQualitiesFooter)
                }
                Section {
                } header: {
                    AcquisitionSaveBar(
                        title: MavioStrings.actionSave, enabled: !changed.isEmpty, busy: saving,
                        notice: changed.isEmpty ? notice : nil, failure: saveFailure, save: save)
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.acquisitionQualities)
            .inlineTitle()
            .task(id: kind) { await load() }
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

        private func column(_ title: String) -> some View {
            Text(title).frame(width: 110, alignment: .leading)
        }

        private func size(
            _ d: Mavio_Acquisition_V1_QualityDefinition,
            _ field: WritableKeyPath<Mavio_Acquisition_V1_QualityDefinition, Double>, _ label: String
        ) -> some View {
            AcquisitionNumberField(
                label: "\(d.title) \(label)",
                value: Binding(
                    get: { (edits[d.quality] ?? d)[keyPath: field] },
                    set: {
                        var e = edits[d.quality] ?? d
                        e[keyPath: field] = $0
                        edits[d.quality] = e
                        notice = nil
                    }),
                step: 0.1
            )
            .frame(width: 110, alignment: .leading)
        }

        private func load() async {
            list = nil
            edits = [:]
            notice = nil
            saveFailure = nil
            var request = Mavio_Acquisition_V1_ListQualityDefinitionsRequest()
            request.kind = kind
            do {
                list = try await model.call(
                    server, Procedure.profiles("ListQualityDefinitions"), request,
                    as: Mavio_Acquisition_V1_ListQualityDefinitionsResponse.self
                ).definitions
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }

        private func save() {
            endEditing()
            var request = Mavio_Acquisition_V1_UpdateQualityDefinitionsRequest()
            request.kind = kind
            request.definitions = changed
            saving = true
            saveFailure = nil
            Task {
                defer { saving = false }
                do {
                    list = try await model.call(
                        server, Procedure.profiles("UpdateQualityDefinitions"), request,
                        as: Mavio_Acquisition_V1_UpdateQualityDefinitionsResponse.self
                    ).definitions
                    edits = [:]
                    notice = MavioStrings.acquisitionSaved
                } catch {
                    saveFailure = error.reason
                }
            }
        }
    }

    // MARK: Quality profiles

    /// The ID an item is ranked by: a group's, else its quality's.
    private func itemID(_ it: Mavio_Acquisition_V1_QualityProfileItem) -> Int32 {
        it.id != 0 ? it.id : it.quality
    }

    /// Acquisition's page of a quality profile, as the web UI's; a new one of a kind when id is nil: its name,
    /// the qualities it allows, best first, where upgrades stop, the language of movies, and the scores of
    /// custom formats.
    struct AcquisitionProfileView: View {
        let server: ServerRecord
        let kind: Mavio_Acquisition_V1_MediaKind
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        /// The profile's ID; nil until a new one is created.
        @State private var id: String?
        @State private var saved: Mavio_Acquisition_V1_QualityProfile?
        @State private var draft: Mavio_Acquisition_V1_QualityProfile?
        @State private var titles: [Int32: String] = [:]
        @State private var formats: [Mavio_Acquisition_V1_CustomFormat]?
        @State private var gone = false
        @State private var failure: String?
        @State private var saving = false
        @State private var saveFailure: String?
        @State private var notice: String?
        @State private var removing = false
        @State private var removeFailure: String?

        init(server: ServerRecord, id: String?, kind: Mavio_Acquisition_V1_MediaKind) {
            self.server = server
            self.kind = kind
            self._id = State(initialValue: id)
        }

        var body: some View {
            SettingsList(message: acquisitionMediaKinds.first { $0.kind == kind }?.title) {
                if gone {
                    AcquisitionNote(text: MavioStrings.acquisitionNoProfile)
                } else if let draft, let saved {
                    form(draft, saved: saved)
                } else if failure == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
                if let failure { AcquisitionNote(text: failure, failed: true) }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(
                gone
                    ? MavioStrings.acquisitionProfile
                    : saved.flatMap { $0.name.isEmpty ? nil : $0.name } ?? MavioStrings.acquisitionNewProfile
            )
            .inlineTitle()
            .task { if draft == nil { await load() } }
        }

        private var isNew: Bool { saved?.id.isEmpty ?? true }

        private var binding: Binding<Mavio_Acquisition_V1_QualityProfile> {
            Binding(
                get: { draft ?? .init() },
                set: {
                    draft = $0
                    notice = nil
                })
        }

        private func nameOf(_ it: Mavio_Acquisition_V1_QualityProfileItem) -> String {
            it.id != 0 ? it.name : titles[it.quality] ?? String(it.quality)
        }

        @ViewBuilder
        private func form(
            _ draft: Mavio_Acquisition_V1_QualityProfile, saved: Mavio_Acquisition_V1_QualityProfile
        ) -> some View {
            let allowed = draft.items.filter(\.allowed)
            let changed = !acquisitionSame(draft, saved)
            Section {
                FormRow(label: MavioStrings.acquisitionProfileName) {
                    TextField(MavioStrings.acquisitionProfileName, text: binding.name)
                        .fieldLabelHidden()
                        .frame(width: 280)
                }
                Toggle(isOn: binding.upgradeAllowed) {
                    Text(MavioStrings.acquisitionUpgradeAllowed)
                    Text(MavioStrings.acquisitionUpgradeAllowedDetail)
                }
                if draft.upgradeAllowed {
                    Picker(selection: binding.cutoff) {
                        if !allowed.contains(where: { itemID($0) == draft.cutoff }) {
                            Text("—").tag(draft.cutoff)
                        }
                        ForEach(allowed.reversed(), id: \.self) { Text(nameOf($0)).tag(itemID($0)) }
                    } label: {
                        Text(MavioStrings.acquisitionCutoff)
                        Text(MavioStrings.acquisitionCutoffDetail)
                    }
                }
                if draft.kind == .movie {
                    Picker(
                        selection: Binding(
                            get: { draft.language == "Any" ? "" : draft.language },
                            set: { binding.wrappedValue.language = $0 })
                    ) {
                        Text(MavioStrings.acquisitionLanguageAny).tag("")
                        Text(MavioStrings.acquisitionLanguageOriginal).tag("Original")
                        Text(MavioStrings.acquisitionLanguageUnknown).tag("Unknown")
                        ForEach(
                            acquisitionReleaseLanguages.sorted { $0.localizedCompare($1) == .orderedAscending },
                            id: \.self
                        ) { Text($0).tag($0) }
                    } label: {
                        Text(MavioStrings.acquisitionLanguage)
                        Text(MavioStrings.acquisitionLanguageDetail)
                    }
                }
            }
            Section {
                ForEach(Array(draft.items.indices.reversed()), id: \.self) { i in
                    qualityRow(draft.items[i], at: i, of: draft.items.count)
                }
            } header: {
                Text(MavioStrings.acquisitionProfileQualities)
            } footer: {
                Text(MavioStrings.acquisitionProfileQualitiesFooter)
            }
            Section {
                ForEach(formats ?? [], id: \.id) { f in
                    FormRow(label: f.name) {
                        AcquisitionNumberField(label: f.name, value: score(f.id), range: -100_000...100_000)
                    }
                }
                if formats?.isEmpty == true {
                    AcquisitionNote(text: MavioStrings.acquisitionNoFormats)
                }
                scoreRow(
                    \.minFormatScore, MavioStrings.acquisitionMinFormatScore,
                    MavioStrings.acquisitionMinFormatScoreDetail)
                if draft.upgradeAllowed {
                    scoreRow(
                        \.cutoffFormatScore, MavioStrings.acquisitionCutoffFormatScore,
                        MavioStrings.acquisitionCutoffFormatScoreDetail)
                    scoreRow(
                        \.minUpgradeFormatScore, MavioStrings.acquisitionMinUpgradeFormatScore,
                        MavioStrings.acquisitionMinUpgradeFormatScoreDetail, min: 1)
                }
            } header: {
                Text(MavioStrings.acquisitionFormatScores)
            } footer: {
                Text(MavioStrings.acquisitionFormatScoresFooter)
            }
            Section {
            } header: {
                VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                    AcquisitionSaveBar(
                        title: isNew ? MavioStrings.acquisitionCreate : MavioStrings.actionSave,
                        enabled: changed && !draft.name.trimmingCharacters(in: .whitespaces).isEmpty
                            && !allowed.isEmpty,
                        busy: saving, notice: changed ? nil : notice, failure: saveFailure, save: save)
                    if !isNew {
                        Button(MavioStrings.acquisitionRemoveProfile, role: .destructive) { removing = true }
                            .buttonStyle(.bordered)
                            .confirmationDialog(
                                MavioStrings.acquisitionRemoveProfileConfirm(name: saved.name), isPresented: $removing,
                                titleVisibility: .visible
                            ) {
                                Button(MavioStrings.acquisitionRemoveProfile, role: .destructive, action: remove)
                            } message: {
                                Text(MavioStrings.acquisitionRemoveProfileDetail)
                            }
                        if let removeFailure {
                            Text(removeFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                        }
                    }
                }
            }
        }

        private func qualityRow(_ it: Mavio_Acquisition_V1_QualityProfileItem, at i: Int, of count: Int) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                Toggle(nameOf(it), isOn: Binding(get: { it.allowed }, set: { _ in toggle(i) }))
                    .labelsHidden()
                    .toggleStyle(.switch)
                VStack(alignment: .leading, spacing: 2) {
                    Text(nameOf(it)).foregroundStyle(it.allowed ? theme.colors.textPrimary : theme.colors.textSecondary)
                    if it.id > 0 {
                        Text(it.items.map(nameOf).joined(separator: ", "))
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                Spacer(minLength: 0)
                AcquisitionMoveButtons(up: i == count - 1 ? nil : { move(i, 1) }, down: i == 0 ? nil : { move(i, -1) })
            }
        }

        private func scoreRow(
            _ field: WritableKeyPath<Mavio_Acquisition_V1_QualityProfile, Int32>, _ label: String, _ detail: String,
            min: Double = -100_000
        ) -> some View {
            FormRow(label: label, detail: detail) {
                AcquisitionNumberField(label: label, value: binding[dynamicMember: field].double, range: min...100_000)
            }
        }

        private func score(_ formatID: String) -> Binding<Double> {
            Binding(
                get: { Double(draft?.formatScores.first { $0.formatID == formatID }?.score ?? 0) },
                set: { value in
                    var scores = binding.wrappedValue.formatScores.filter { $0.formatID != formatID }
                    if value != 0 {
                        var s = Mavio_Acquisition_V1_FormatScore()
                        s.formatID = formatID
                        s.score = Int32(value.rounded())
                        scores.append(s)
                    }
                    binding.wrappedValue.formatScores = scores
                })
        }

        /// The items set, the cutoff moved to the best allowed when it is no longer allowed.
        private func setItems(_ items: [Mavio_Acquisition_V1_QualityProfileItem]) {
            guard var d = draft else { return }
            let cutoffAllowed = items.contains { $0.allowed && itemID($0) == d.cutoff }
            if !cutoffAllowed, let top = items.last(where: \.allowed) { d.cutoff = itemID(top) }
            d.items = items
            binding.wrappedValue = d
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
            } catch {
                failure = error.reason
            }
        }

        private func save() {
            endEditing()
            guard var p = draft else { return }
            p.name = p.name.trimmingCharacters(in: .whitespaces)
            saving = true
            saveFailure = nil
            let creating = isNew
            Task {
                defer { saving = false }
                do {
                    let result: Mavio_Acquisition_V1_QualityProfile
                    if creating {
                        var request = Mavio_Acquisition_V1_CreateQualityProfileRequest()
                        request.profile = p
                        result = try await model.call(
                            server, Procedure.profiles("CreateQualityProfile"), request,
                            as: Mavio_Acquisition_V1_CreateQualityProfileResponse.self
                        ).profile
                        id = result.id
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateQualityProfileRequest()
                        request.profile = p
                        result = try await model.call(
                            server, Procedure.profiles("UpdateQualityProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateQualityProfileResponse.self
                        ).profile
                        notice = MavioStrings.acquisitionSaved
                    }
                    (saved, draft) = (result, result)
                } catch {
                    saveFailure = error.reason
                }
            }
        }

        private func remove() {
            guard let saved else { return }
            var request = Mavio_Acquisition_V1_DeleteQualityProfileRequest()
            request.id = saved.id
            removeFailure = nil
            Task {
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteQualityProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteQualityProfileResponse.self)
                    dismiss()
                } catch {
                    removeFailure = error.reason
                }
            }
        }
    }

    /// The buttons moving a row of an ordered list up and down; a nil action leaves its button disabled.
    struct AcquisitionMoveButtons: View {
        let up: (() -> Void)?
        let down: (() -> Void)?
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s4) {
                button(MavioStrings.acquisitionMoveUp, angle: 90, action: up)
                button(MavioStrings.acquisitionMoveDown, angle: -90, action: down)
            }
        }

        private func button(_ label: String, angle: Double, action: (() -> Void)?) -> some View {
            Button {
                action?()
            } label: {
                Icon("back", size: 16)
                    .rotationEffect(.degrees(angle))
                    .foregroundStyle(theme.colors.textSecondary)
                    .padding(6)
                    .contentShape(.rect)
            }
            .buttonStyle(.borderless)
            .disabled(action == nil)
            .opacity(action == nil ? 0.3 : 1)
            .accessibilityLabel(label)
        }
    }
#endif
