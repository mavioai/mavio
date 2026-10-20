#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftProtobuf
    import SwiftUI

    /// The items of a list, trimmed, without the empty ones.
    private func splitList(_ s: String, by separator: Character) -> [String] {
        s.split(separator: separator).map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
    }

    /// A delay in whole minutes.
    private func minutes(_ d: Google_Protobuf_Duration?) -> Int {
        guard let d else { return 0 }
        return Int(((Double(d.seconds) + Double(d.nanos) / 1e9) / 60).rounded())
    }

    /// A row of a list that opens into its form, and what ends it.
    private struct EntryRow<Trailing: View>: View {
        let label: String
        var detail: String?
        var disabled = false
        let open: () -> Void
        @ViewBuilder var trailing: Trailing
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s4) {
                Button(action: open) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(label).lineLimit(1).foregroundStyle(theme.colors.textPrimary)
                        if let detail, !detail.isEmpty {
                            Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .contentShape(.rect)
                }
                .buttonStyle(.plain)
                .disabled(disabled)
                trailing
            }
        }
    }

    /// The buttons under an entry's form: saving, cancelling, and removing it once confirmed.
    private struct FormButtons: View {
        let valid: Bool
        let busy: Bool
        var failure: String?
        let save: () -> Void
        let cancel: () -> Void
        var remove: (() -> Void)?
        @State private var removing = false
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                Button(busy ? "…" : MavioStrings.actionSave, action: save)
                    .buttonStyle(.borderedProminent)
                    .disabled(!valid || busy)
                Button(MavioStrings.actionCancel, action: cancel)
                if let remove {
                    if removing {
                        Button(MavioStrings.rulesRemoveConfirm, role: .destructive, action: remove)
                            .buttonStyle(.bordered)
                            .disabled(busy)
                    } else {
                        Button(MavioStrings.rulesRemove, role: .destructive) { removing = true }
                            .foregroundStyle(theme.colors.danger)
                    }
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
        }
    }

    /// A text field's row of a form, its field at the row's end.
    private struct FieldRow: View {
        let label: String
        var detail: String?
        @Binding var text: String
        var prompt: String?

        var body: some View {
            FormRow(label: label, detail: detail) {
                TextField(label, text: $text, prompt: prompt.map { Text($0) })
                    .fieldLabelHidden()
                    .frame(width: 280)
            }
        }
    }

    /// The header of a group with its Add button at its end, while nothing is being edited.
    private struct AddHeader: View {
        let title: String
        let showsAdd: Bool
        let add: () -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                Text(title)
                Spacer(minLength: 0)
                if showsAdd {
                    Button(MavioStrings.rulesAdd, action: add).type(theme.type.body)
                }
            }
        }
    }

    // MARK: Delay profiles

    /// Acquisition's page of the delay profiles, as the web UI's: which hold releases back for better ones, in
    /// order.
    struct AcquisitionDelayProfilesView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var list: [Mavio_Acquisition_V1_DelayProfile]?
        @State private var failure: String?
        /// The profile edited, by ID; "new" for one to add.
        @State private var editing: String?
        @State private var busy = false
        @State private var formFailure: String?
        @State private var reordering = false
        @State private var reorderFailure: String?

        var body: some View {
            SettingsList(message: MavioStrings.rulesDelaysMessage) {
                Section {
                    ForEach(list ?? [], id: \.id) { p in
                        if editing == p.id {
                            DelayForm(
                                profile: p, busy: busy, failure: formFailure, cancel: close,
                                save: { write($0, "UpdateDelayProfile") },
                                remove: p.default ? nil : { remove(p.id) })
                        } else {
                            EntryRow(
                                label: p.default ? MavioStrings.rulesDefaultDelay : p.tags.joined(separator: ", "),
                                detail: summary(p), disabled: editing != nil, open: { open(p.id) }
                            ) {
                                if !p.default {
                                    let i = ordered.firstIndex { $0.id == p.id } ?? 0
                                    AcquisitionMoveButtons(
                                        up: p.order == ordered.first?.order || reordering ? nil : { move(i, -1) },
                                        down: p.id == ordered.last?.id || reordering ? nil : { move(i, 1) })
                                }
                            }
                        }
                    }
                    if editing == "new" {
                        DelayForm(
                            profile: blank, busy: busy, failure: formFailure, cancel: close,
                            save: { write($0, "CreateDelayProfile") }, remove: nil)
                    }
                    if list == nil && failure == nil { ProgressView().frame(maxWidth: .infinity) }
                    if let failure { AcquisitionNote(text: failure, failed: true) }
                    if let reorderFailure { AcquisitionNote(text: reorderFailure, failed: true) }
                } header: {
                    AddHeader(title: MavioStrings.rulesDelaysInOrder, showsAdd: editing == nil) { open("new") }
                } footer: {
                    Text(MavioStrings.rulesDelaysFooter)
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.acquisitionDelays)
            .inlineTitle()
            .task { await load() }
        }

        private var ordered: [Mavio_Acquisition_V1_DelayProfile] { (list ?? []).filter { !$0.default } }

        private var blank: Mavio_Acquisition_V1_DelayProfile {
            var p = Mavio_Acquisition_V1_DelayProfile()
            p.enableUsenet = true
            p.enableTorrent = true
            p.preferredProtocol = .usenet
            return p
        }

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
                p.enableUsenet
                    ? MavioStrings.rulesUsenetDelayOf(minutes: String(minutes(p.usenetDelay))) : "",
                p.enableTorrent
                    ? MavioStrings.rulesTorrentDelayOf(minutes: String(minutes(p.torrentDelay))) : "",
            ]
            .filter { !$0.isEmpty }.joined(separator: " · ")
        }

        private func open(_ id: String) {
            formFailure = nil
            editing = id
        }

        private func close() { editing = nil }

        private func move(_ i: Int, _ by: Int) {
            var ids = ordered.map(\.id)
            let id = ids.remove(at: i)
            ids.insert(id, at: i + by)
            var request = Mavio_Acquisition_V1_ReorderDelayProfilesRequest()
            request.ids = ids
            reordering = true
            Task {
                defer { reordering = false }
                do {
                    list = try await model.call(
                        server, Procedure.profiles("ReorderDelayProfiles"), request,
                        as: Mavio_Acquisition_V1_ReorderDelayProfilesResponse.self
                    ).profiles
                    reorderFailure = nil
                } catch {
                    reorderFailure = error.reason
                }
            }
        }

        private func write(_ p: Mavio_Acquisition_V1_DelayProfile, _ method: String) {
            busy = true
            formFailure = nil
            Task {
                defer { busy = false }
                do {
                    if method == "CreateDelayProfile" {
                        var request = Mavio_Acquisition_V1_CreateDelayProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles(method), request,
                            as: Mavio_Acquisition_V1_CreateDelayProfileResponse.self)
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateDelayProfileRequest()
                        request.profile = p
                        _ = try await model.call(
                            server, Procedure.profiles(method), request,
                            as: Mavio_Acquisition_V1_UpdateDelayProfileResponse.self)
                    }
                    close()
                    await load()
                } catch {
                    formFailure = error.reason
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteDelayProfileRequest()
            request.id = id
            busy = true
            formFailure = nil
            Task {
                defer { busy = false }
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteDelayProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteDelayProfileResponse.self)
                    close()
                    await load()
                } catch {
                    formFailure = error.reason
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
            } catch {
                failure = error.reason
            }
        }
    }

    /// The form of a delay profile, its rows in the group of the list.
    private struct DelayForm: View {
        let profile: Mavio_Acquisition_V1_DelayProfile
        let busy: Bool
        let failure: String?
        let cancel: () -> Void
        let save: (Mavio_Acquisition_V1_DelayProfile) -> Void
        let remove: (() -> Void)?
        @State private var d: Draft

        /// What the form holds.
        private struct Draft {
            var usenet: Bool
            var torrent: Bool
            var preferred: Mavio_Acquisition_V1_DownloadProtocol
            var usenetDelay: Int32
            var torrentDelay: Int32
            var bypassQuality: Bool
            var bypassScore: Bool
            var minScore: Int32
            var tags: String
        }

        init(
            profile: Mavio_Acquisition_V1_DelayProfile, busy: Bool, failure: String?, cancel: @escaping () -> Void,
            save: @escaping (Mavio_Acquisition_V1_DelayProfile) -> Void, remove: (() -> Void)?
        ) {
            self.profile = profile
            self.busy = busy
            self.failure = failure
            self.cancel = cancel
            self.save = save
            self.remove = remove
            _d = State(
                initialValue: Draft(
                    usenet: profile.enableUsenet, torrent: profile.enableTorrent,
                    preferred: profile.preferredProtocol == .unspecified ? .usenet : profile.preferredProtocol,
                    usenetDelay: Int32(minutes(profile.hasUsenetDelay ? profile.usenetDelay : nil)),
                    torrentDelay: Int32(minutes(profile.hasTorrentDelay ? profile.torrentDelay : nil)),
                    bypassQuality: profile.bypassIfHighestQuality,
                    bypassScore: profile.bypassIfAboveCustomFormatScore,
                    minScore: profile.minimumCustomFormatScore, tags: profile.tags.joined(separator: ", ")))
        }

        private var tags: [String] { splitList(d.tags, by: ",") }
        private var valid: Bool { (d.usenet || d.torrent) && (profile.default || !tags.isEmpty) }

        var body: some View {
            if !profile.default {
                FieldRow(label: MavioStrings.rulesTags, detail: MavioStrings.rulesDelayTagsDetail, text: $d.tags)
            }
            Toggle(MavioStrings.acquisitionUsenet, isOn: $d.usenet)
            Toggle(MavioStrings.acquisitionTorrent, isOn: $d.torrent)
            if d.usenet && d.torrent {
                Picker(MavioStrings.rulesPreferred, selection: $d.preferred) {
                    Text(MavioStrings.acquisitionUsenet).tag(Mavio_Acquisition_V1_DownloadProtocol.usenet)
                    Text(MavioStrings.acquisitionTorrent).tag(Mavio_Acquisition_V1_DownloadProtocol.torrent)
                }
            }
            if d.usenet {
                FormRow(label: MavioStrings.rulesUsenetDelay, detail: MavioStrings.rulesMinutes) {
                    AcquisitionNumberField(
                        label: MavioStrings.rulesUsenetDelay, value: $d.usenetDelay.double, range: 0...525_600)
                }
            }
            if d.torrent {
                FormRow(label: MavioStrings.rulesTorrentDelay, detail: MavioStrings.rulesMinutes) {
                    AcquisitionNumberField(
                        label: MavioStrings.rulesTorrentDelay, value: $d.torrentDelay.double, range: 0...525_600)
                }
            }
            Toggle(isOn: $d.bypassQuality) {
                Text(MavioStrings.rulesBypassQuality)
                Text(MavioStrings.rulesBypassQualityDetail)
            }
            Toggle(MavioStrings.rulesBypassScore, isOn: $d.bypassScore)
            if d.bypassScore {
                FormRow(label: MavioStrings.rulesMinScore) {
                    AcquisitionNumberField(
                        label: MavioStrings.rulesMinScore, value: $d.minScore.double, range: -100_000...100_000)
                }
            }
            FormButtons(valid: valid, busy: busy, failure: failure, save: submit, cancel: cancel, remove: remove)
        }

        private func submit() {
            endEditing()
            var p = profile
            p.enableUsenet = d.usenet
            p.enableTorrent = d.torrent
            p.preferredProtocol = d.usenet && d.torrent ? d.preferred : d.usenet ? .usenet : .torrent
            p.usenetDelay = Google_Protobuf_Duration(seconds: Int64(d.usenetDelay) * 60)
            p.torrentDelay = Google_Protobuf_Duration(seconds: Int64(d.torrentDelay) * 60)
            p.bypassIfHighestQuality = d.bypassQuality
            p.bypassIfAboveCustomFormatScore = d.bypassScore
            p.minimumCustomFormatScore = d.minScore
            p.tags = profile.default ? [] : tags
            save(p)
        }
    }

    // MARK: Release profiles

    /// Acquisition's page of the release profiles, as the web UI's: which require or ignore terms in releases'
    /// titles.
    struct AcquisitionReleaseProfilesView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var list: [Mavio_Acquisition_V1_ReleaseProfile]?
        @State private var indexers: [Mavio_Acquisition_V1_Indexer] = []
        @State private var failure: String?
        /// The profile edited, by ID; "new" for one to add.
        @State private var editing: String?
        @State private var busy = false
        @State private var formFailure: String?

        var body: some View {
            SettingsList(message: MavioStrings.rulesReleasesMessage) {
                Section {
                    ForEach(list ?? [], id: \.id) { p in
                        if editing == p.id {
                            form(p, isNew: false)
                        } else {
                            EntryRow(
                                label: p.name.isEmpty ? MavioStrings.rulesUnnamed : p.name, detail: summary(p),
                                disabled: editing != nil, open: { open(p.id) }
                            ) {}
                        }
                    }
                    if editing == "new" {
                        form(newProfile, isNew: true)
                    }
                    if let list, list.isEmpty, editing == nil {
                        AcquisitionNote(text: MavioStrings.rulesNoReleases)
                    }
                    if list == nil && failure == nil { ProgressView().frame(maxWidth: .infinity) }
                    if let failure { AcquisitionNote(text: failure, failed: true) }
                } header: {
                    AddHeader(title: MavioStrings.acquisitionReleases, showsAdd: editing == nil) { open("new") }
                } footer: {
                    Text(MavioStrings.rulesReleasesFooter)
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.acquisitionReleases)
            .inlineTitle()
            .task { await load() }
        }

        private var newProfile: Mavio_Acquisition_V1_ReleaseProfile {
            var p = Mavio_Acquisition_V1_ReleaseProfile()
            p.enabled = true
            return p
        }

        private func form(_ p: Mavio_Acquisition_V1_ReleaseProfile, isNew: Bool) -> some View {
            ReleaseForm(
                profile: p, indexers: indexers, busy: busy, failure: formFailure, cancel: { editing = nil },
                save: { write($0, isNew: isNew) }, remove: isNew ? nil : { remove(p.id) })
        }

        private func indexerName(_ p: Mavio_Acquisition_V1_ReleaseProfile) -> String? {
            guard p.hasIndexer else { return nil }
            return indexers.first { $0.pluginID == p.indexer.pluginID && $0.id == p.indexer.indexerID }?.name
                ?? p.indexer.indexerID
        }

        private func summary(_ p: Mavio_Acquisition_V1_ReleaseProfile) -> String {
            [
                p.enabled ? nil : MavioStrings.acquisitionOff,
                p.required.isEmpty ? nil : MavioStrings.rulesRequiredCount(count: p.required.count),
                p.ignored.isEmpty ? nil : MavioStrings.rulesIgnoredCount(count: p.ignored.count),
                indexerName(p),
                p.tags.joined(separator: ", "),
            ]
            .compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
        }

        private func open(_ id: String) {
            formFailure = nil
            editing = id
        }

        private func write(_ p: Mavio_Acquisition_V1_ReleaseProfile, isNew: Bool) {
            busy = true
            formFailure = nil
            Task {
                defer { busy = false }
                do {
                    if isNew {
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
                    editing = nil
                    await load()
                } catch {
                    formFailure = error.reason
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteReleaseProfileRequest()
            request.id = id
            busy = true
            formFailure = nil
            Task {
                defer { busy = false }
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteReleaseProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteReleaseProfileResponse.self)
                    editing = nil
                    await load()
                } catch {
                    formFailure = error.reason
                }
            }
        }

        private func load() async {
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
            } catch {
                failure = error.reason
            }
            if let found = await i { indexers = found.indexers }
        }
    }

    /// The form of a release profile, its rows in the group of the list.
    private struct ReleaseForm: View {
        let profile: Mavio_Acquisition_V1_ReleaseProfile
        let indexers: [Mavio_Acquisition_V1_Indexer]
        let busy: Bool
        let failure: String?
        let cancel: () -> Void
        let save: (Mavio_Acquisition_V1_ReleaseProfile) -> Void
        let remove: (() -> Void)?
        @State private var d: Draft
        @Environment(\.theme) private var theme

        /// What the form holds.
        private struct Draft {
            var name: String
            var enabled: Bool
            var required: String
            var ignored: String
            /// The indexer as "plugin/indexer"; empty for all of them.
            var indexer: String
            var tags: String
        }

        init(
            profile: Mavio_Acquisition_V1_ReleaseProfile, indexers: [Mavio_Acquisition_V1_Indexer], busy: Bool,
            failure: String?, cancel: @escaping () -> Void,
            save: @escaping (Mavio_Acquisition_V1_ReleaseProfile) -> Void, remove: (() -> Void)?
        ) {
            self.profile = profile
            self.indexers = indexers
            self.busy = busy
            self.failure = failure
            self.cancel = cancel
            self.save = save
            self.remove = remove
            _d = State(
                initialValue: Draft(
                    name: profile.name, enabled: profile.enabled, required: profile.required.joined(separator: "\n"),
                    ignored: profile.ignored.joined(separator: "\n"),
                    indexer: profile.hasIndexer ? "\(profile.indexer.pluginID)/\(profile.indexer.indexerID)" : "",
                    tags: profile.tags.joined(separator: ", ")))
        }

        private var required: [String] { splitList(d.required, by: "\n") }
        private var ignored: [String] { splitList(d.ignored, by: "\n") }

        /// The indexer chosen, split into its plugin and its ID.
        private var chosen: (plugin: String, id: String) {
            guard let slash = d.indexer.firstIndex(of: "/") else { return (d.indexer, "") }
            return (String(d.indexer[..<slash]), String(d.indexer[d.indexer.index(after: slash)...]))
        }

        var body: some View {
            FieldRow(label: MavioStrings.rulesName, text: $d.name)
            Toggle(MavioStrings.rulesEnabled, isOn: $d.enabled)
            terms($d.required, MavioStrings.rulesRequired, MavioStrings.rulesRequiredDetail)
            terms($d.ignored, MavioStrings.rulesIgnored, MavioStrings.rulesIgnoredDetail)
            Picker(MavioStrings.rulesIndexer, selection: $d.indexer) {
                Text(MavioStrings.rulesAllIndexers).tag("")
                ForEach(indexers, id: \.self) { x in Text(x.name).tag("\(x.pluginID)/\(x.id)") }
                if !d.indexer.isEmpty, !indexers.contains(where: { "\($0.pluginID)/\($0.id)" == d.indexer }) {
                    Text(chosen.id.isEmpty ? d.indexer : chosen.id).tag(d.indexer)
                }
            }
            FieldRow(label: MavioStrings.rulesTags, detail: MavioStrings.rulesReleaseTagsDetail, text: $d.tags)
            FormButtons(
                valid: required.count + ignored.count > 0, busy: busy, failure: failure, save: submit, cancel: cancel,
                remove: remove)
        }

        private func terms(_ text: Binding<String>, _ label: String, _ detail: String) -> some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                Text(label)
                Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                TextEditor(text: text)
                    .font(.system(.body, design: .monospaced))
                    .frame(height: 64)
                    .scrollContentBackground(.hidden)
                    .padding(4)
                    .background(theme.colors.background, in: .rect(cornerRadius: 6))
                    .accessibilityLabel(label)
            }
        }

        private func submit() {
            endEditing()
            var p = Mavio_Acquisition_V1_ReleaseProfile()
            p.id = profile.id
            p.name = d.name.trimmingCharacters(in: .whitespaces)
            p.enabled = d.enabled
            p.required = required
            p.ignored = ignored
            if !d.indexer.isEmpty {
                var ref = Mavio_Acquisition_V1_IndexerRef()
                ref.pluginID = chosen.plugin
                ref.indexerID = chosen.id
                p.indexer = ref
            }
            p.tags = splitList(d.tags, by: ",")
            save(p)
        }
    }

    // MARK: Remote path mappings

    /// Acquisition's page of the remote path mappings, as the web UI's: which map the folders download clients
    /// on other hosts report.
    struct AcquisitionRemotePathMappingsView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var list: [Mavio_Acquisition_V1_RemotePathMapping]?
        @State private var failure: String?
        @State private var adding = false
        @State private var host = ""
        @State private var remote = ""
        @State private var local = ""
        @State private var creating = false
        @State private var createFailure: String?
        @State private var removing: String?

        var body: some View {
            SettingsList(message: MavioStrings.rulesMappingsMessage) {
                Section {
                    ForEach(list ?? [], id: \.id) { m in
                        HStack(spacing: MavioSpacing.s12) {
                            RowLabel(label: m.host, detail: "\(m.remotePath) → \(m.localPath)")
                            Spacer(minLength: 0)
                            Button(MavioStrings.rulesRemove, role: .destructive) { remove(m.id) }
                                .foregroundStyle(theme.colors.danger)
                                .disabled(removing == m.id)
                        }
                    }
                    if adding {
                        FieldRow(
                            label: MavioStrings.rulesHost, detail: MavioStrings.rulesHostDetail, text: $host,
                            prompt: "192.168.1.10")
                        FieldRow(
                            label: MavioStrings.rulesRemotePath, detail: MavioStrings.rulesRemotePathDetail,
                            text: $remote, prompt: "/downloads/")
                        FieldRow(
                            label: MavioStrings.rulesLocalPath, detail: MavioStrings.rulesLocalPathDetail,
                            text: $local, prompt: "/mnt/downloads/")
                        FormButtons(
                            valid: valid, busy: creating, failure: createFailure, save: create,
                            cancel: { adding = false })
                    }
                    if let list, list.isEmpty, !adding {
                        AcquisitionNote(text: MavioStrings.rulesNoMappings)
                    }
                    if list == nil && failure == nil { ProgressView().frame(maxWidth: .infinity) }
                    if let failure { AcquisitionNote(text: failure, failed: true) }
                } header: {
                    AddHeader(title: MavioStrings.acquisitionMappings, showsAdd: !adding) {
                        createFailure = nil
                        adding = true
                    }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.acquisitionMappings)
            .inlineTitle()
            .task { await load() }
        }

        private var valid: Bool {
            let l = local.trimmingCharacters(in: .whitespaces)
            return !host.trimmingCharacters(in: .whitespaces).isEmpty
                && !remote.trimmingCharacters(in: .whitespaces).isEmpty
                && l.wholeMatch(of: /^(\/|[A-Za-z]:[\\\/]).*/) != nil
        }

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
            Task {
                defer { creating = false }
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("CreateRemotePathMapping"), request,
                        as: Mavio_Acquisition_V1_CreateRemotePathMappingResponse.self)
                    adding = false
                    (host, remote, local) = ("", "", "")
                    await load()
                } catch {
                    createFailure = error.reason
                }
            }
        }

        private func remove(_ id: String) {
            var request = Mavio_Acquisition_V1_DeleteRemotePathMappingRequest()
            request.id = id
            removing = id
            Task {
                defer { removing = nil }
                do {
                    _ = try await model.call(
                        server, Procedure.profiles("DeleteRemotePathMapping"), request,
                        as: Mavio_Acquisition_V1_DeleteRemotePathMappingResponse.self)
                    await load()
                } catch {
                    failure = error.reason
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
            } catch {
                failure = error.reason
            }
        }
    }
#endif
