#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// A subtitle profile's page, as the web UI's, or a new one's: its name, its languages, the cutoff, the
    /// original format and the tag, and what release names must and must not contain; saved, or removed.
    struct SubtitlesAdminProfileView: View {
        typealias Profile = Mavio_Acquisition_V1_SubtitleProfile
        typealias Item = Mavio_Acquisition_V1_SubtitleProfileItem
        typealias HearingImpaired = Mavio_Acquisition_V1_SubtitleHearingImpaired

        let server: ServerRecord
        /// Called once the profile is saved or removed, for the list to load again.
        let onChange: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var profile: Profile
        @State private var draft: Profile
        @State private var mustContain: String
        @State private var mustNotContain: String
        @State private var known: [Choice] = []
        @State private var saving = false
        @State private var confirming = false
        @State private var failure: String?

        /// The page of a profile, or of a new one when profile is nil.
        init(server: ServerRecord, profile: Profile?, onChange: @escaping () async -> Void) {
            self.server = server
            self.onChange = onChange
            let p =
                profile
                ?? {
                    var p = Profile()
                    var item = Item()
                    item.id = 1
                    item.language = "en"
                    p.items = [item]
                    return p
                }()
            _profile = State(initialValue: p)
            _draft = State(initialValue: p)
            _mustContain = State(initialValue: p.mustContain.joined(separator: "\n"))
            _mustNotContain = State(initialValue: p.mustNotContain.joined(separator: "\n"))
        }

        private var isNew: Bool { profile.id.isEmpty }

        private var result: Profile {
            var r = draft
            r.name = draft.name.trimmingCharacters(in: .whitespaces)
            r.tag = draft.tag.trimmingCharacters(in: .whitespaces)
            r.mustContain = Self.lines(mustContain)
            r.mustNotContain = Self.lines(mustNotContain)
            return r
        }

        private var changed: Bool { isNew || result != profile }

        private var valid: Bool {
            let r = result
            return !r.name.isEmpty && !r.items.isEmpty && !r.tag.contains { $0.isWhitespace }
        }

        private var languages: [Choice] { SubtitlesAdmin.languages(known, draft.items.map(\.language)) }

        var body: some View {
            SettingsList {
                Section {
                    TextRow(label: MavioStrings.rulesName, text: $draft.name)
                }
                Section {
                    ForEach(draft.items, id: \.id) { itemRow($0.id) }
                } header: {
                    HStack {
                        Text(MavioStrings.subtitlesLanguages)
                        Spacer()
                        Button(MavioStrings.rulesAdd, action: addItem).buttonStyle(.borderless)
                    }
                } footer: {
                    Text(MavioStrings.subtitlesLanguagesFooter)
                }
                Section {
                    Picker(selection: $draft.cutoff) {
                        Text(MavioStrings.subtitlesCutoffNone).tag(Int32(0))
                        Text(MavioStrings.subtitlesCutoffAny).tag(SubtitlesAdmin.cutoffAny)
                        ForEach(draft.items, id: \.id) { Text(SubtitlesAdmin.itemName($0)).tag($0.id) }
                    } label: {
                        RowLabel(label: MavioStrings.subtitlesCutoff, detail: MavioStrings.subtitlesCutoffDetail)
                    }
                    .listPicker()
                    Toggle(isOn: $draft.originalFormat) {
                        RowLabel(
                            label: MavioStrings.subtitlesOriginalFormat,
                            detail: MavioStrings.subtitlesOriginalFormatDetail)
                    }
                    TextRow(label: MavioStrings.subtitlesTag, detail: MavioStrings.subtitlesTagDetail, text: $draft.tag)
                }
                Section {
                    textBlock(MavioStrings.subtitlesMustContain, $mustContain)
                    textBlock(MavioStrings.subtitlesMustNotContain, $mustNotContain)
                } header: {
                    Text(MavioStrings.subtitlesReleaseNames)
                } footer: {
                    Text(MavioStrings.subtitlesReleaseNamesFooter)
                }
                if !isNew {
                    Section {
                        Button(MavioStrings.subtitlesRemove, role: .destructive) { confirming = true }
                            .confirmationDialog(
                                MavioStrings.subtitlesRemoveTitle(name: profile.name), isPresented: $confirming,
                                titleVisibility: .visible
                            ) {
                                Button(MavioStrings.subtitlesRemove, role: .destructive, action: remove)
                            } message: {
                                Text(MavioStrings.subtitlesRemoveMessage)
                            }
                            .disabled(saving)
                    }
                }
                if let failure {
                    Section {
                        Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(profile.name.isEmpty ? MavioStrings.subtitlesNewProfile : profile.name)
            .inlineTitle()
            .toolbar {
                if saving {
                    ProgressView()
                } else {
                    Button(MavioStrings.actionSave, action: save).disabled(!changed || !valid)
                }
            }
            .task { known = await model.localization(server).languages }
        }

        private func itemRow(_ id: Int32) -> some View {
            let item = binding(id)
            return VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                HStack(spacing: MavioSpacing.s8) {
                    Picker(MavioStrings.subtitlesLanguage, selection: item.language) {
                        ForEach(languages, id: \.code) { Text($0.name).tag($0.code) }
                    }
                    .labelsHidden()
                    .frame(width: 192)
                    Picker(MavioStrings.subtitlesHearingImpaired, selection: item.hearingImpaired) {
                        Text(MavioStrings.subtitlesHiAllowed).tag(HearingImpaired.unspecified)
                        Text(MavioStrings.subtitlesHiRequired).tag(HearingImpaired.required)
                        Text(MavioStrings.subtitlesHiExcluded).tag(HearingImpaired.excluded)
                    }
                    .labelsHidden()
                    .frame(width: 192)
                    Spacer(minLength: 0)
                    Button {
                        removeItem(id)
                    } label: {
                        Icon("close", size: 16).foregroundStyle(theme.colors.textSecondary)
                    }
                    .buttonStyle(.borderless)
                    .disabled(draft.items.count == 1)
                    .accessibilityLabel(MavioStrings.rulesRemove)
                }
                HStack(spacing: MavioSpacing.s16) {
                    Toggle(MavioStrings.subtitlesForcedOnly, isOn: item.forced)
                    Toggle(
                        MavioStrings.subtitlesAudioExclude,
                        isOn: Binding(
                            get: { item.wrappedValue.audioExclude },
                            set: {
                                item.wrappedValue.audioExclude = $0
                                if $0 { item.wrappedValue.audioOnlyInclude = false }
                            }))
                    Toggle(
                        MavioStrings.subtitlesAudioOnlyInclude,
                        isOn: Binding(
                            get: { item.wrappedValue.audioOnlyInclude },
                            set: {
                                item.wrappedValue.audioOnlyInclude = $0
                                if $0 { item.wrappedValue.audioExclude = false }
                            }))
                }
                .toggleStyle(.checkbox)
            }
            .padding(.vertical, MavioSpacing.s4)
        }

        private func textBlock(_ label: String, _ text: Binding<String>) -> some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(label)
                TextEditor(text: text)
                    .font(.system(.body, design: .monospaced))
                    .autocorrectionDisabled()
                    .frame(height: 52)
                    .accessibilityLabel(label)
            }
        }

        private func binding(_ id: Int32) -> Binding<Item> {
            Binding(
                get: { draft.items.first { $0.id == id } ?? Item() },
                set: { value in
                    if let i = draft.items.firstIndex(where: { $0.id == id }) { draft.items[i] = value }
                })
        }

        private func addItem() {
            let next = (draft.items.map(\.id).max() ?? 0) + 1
            let used = Set(draft.items.map(\.language))
            let own = Locale.current.language.languageCode?.identifier ?? "en"
            var item = Item()
            item.id = next
            item.language = ([own, "en"] + languages.map(\.code)).first { !used.contains($0) } ?? "en"
            draft.items.append(item)
        }

        private func removeItem(_ id: Int32) {
            draft.items.removeAll { $0.id == id }
            if draft.cutoff == id { draft.cutoff = 0 }
        }

        private func save() {
            let result = result
            let isNew = isNew
            saving = true
            failure = nil
            Task {
                defer { saving = false }
                do {
                    let saved: Profile
                    if isNew {
                        var request = Mavio_Acquisition_V1_CreateSubtitleProfileRequest()
                        request.profile = result
                        saved = try await model.call(
                            server, Procedure.subtitles("CreateSubtitleProfile"), request,
                            as: Mavio_Acquisition_V1_CreateSubtitleProfileResponse.self
                        ).profile
                    } else {
                        var request = Mavio_Acquisition_V1_UpdateSubtitleProfileRequest()
                        request.profile = result
                        saved = try await model.call(
                            server, Procedure.subtitles("UpdateSubtitleProfile"), request,
                            as: Mavio_Acquisition_V1_UpdateSubtitleProfileResponse.self
                        ).profile
                    }
                    profile = saved
                    draft = saved
                    mustContain = saved.mustContain.joined(separator: "\n")
                    mustNotContain = saved.mustNotContain.joined(separator: "\n")
                    await onChange()
                } catch {
                    failure = error.reason
                }
            }
        }

        private func remove() {
            saving = true
            failure = nil
            Task {
                defer { saving = false }
                do {
                    var request = Mavio_Acquisition_V1_DeleteSubtitleProfileRequest()
                    request.id = profile.id
                    _ = try await model.call(
                        server, Procedure.subtitles("DeleteSubtitleProfile"), request,
                        as: Mavio_Acquisition_V1_DeleteSubtitleProfileResponse.self)
                    await onChange()
                    dismiss()
                } catch {
                    failure = error.reason
                }
            }
        }

        private static func lines(_ s: String) -> [String] {
            s.split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        }
    }
#endif
