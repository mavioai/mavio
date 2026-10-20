#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// The server's assistant settings, in its AI settings, as the web UI's AssistantServerSettings
    /// (docs/assistant.md §10): whether it is on, its model, code mode, who may use the AI path, how long
    /// conversations stay idle and are kept, memory and hints; and the recognizer and voice of each language with
    /// the timing of turns.
    struct AiAssistantSettings: View {
        let server: ServerRecord
        @Bindable var store: AiAssistantStore
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme

        var body: some View {
            if let saved = store.saved, saved.hasSettings {
                form(saved)
            } else if let failure = store.failure {
                Section {
                } footer: {
                    Text(failure).foregroundStyle(theme.colors.danger)
                }
            }
        }

        private var off: Bool { !store.draft.enabled }

        @ViewBuilder private func form(_ saved: Mavio_Assistant_V1_GetAssistantSettingsResponse) -> some View {
            let changed = store.draft != saved.settings
            Section(MavioStrings.assistantServer) {
                Toggle(isOn: binding(\.enabled)) {
                    Text(MavioStrings.assistantEnabled)
                    Text(MavioStrings.assistantEnabledDetail)
                }
                FormRow(label: MavioStrings.assistantModel) {
                    AiSearchPicker(
                        label: MavioStrings.assistantModel,
                        options: [AiPickerOption(value: "", label: MavioStrings.assistantNoModel)] + store.options,
                        value: binding(\.model), placeholder: MavioStrings.assistantNoModel, disabled: off)
                }
                Toggle(isOn: binding(\.codeMode)) {
                    Text(MavioStrings.assistantCodeMode)
                    Text(MavioStrings.assistantCodeModeDetail)
                }
                .disabled(off || store.draft.model.isEmpty)
                Picker(MavioStrings.assistantAiAccess, selection: access) {
                    Text(MavioStrings.assistantAccessAll).tag(Mavio_Assistant_V1_AssistantAccess.all)
                    Text(MavioStrings.assistantAccessAdmins).tag(Mavio_Assistant_V1_AssistantAccess.administrators)
                    Text(MavioStrings.assistantAccessSelected).tag(Mavio_Assistant_V1_AssistantAccess.selected)
                }
                .disabled(off || store.draft.model.isEmpty)
                if store.draft.aiAccess == .selected {
                    Flow(spacing: MavioSpacing.s20, lineSpacing: MavioSpacing.s8) {
                        ForEach(store.users, id: \.id) { u in
                            Toggle(u.name, isOn: chosen(u.id)).toggleStyle(.checkbox)
                        }
                    }
                }
                numberRow(MavioStrings.assistantIdleMinutes, count(\.idleMinutes), prompt: "30")
                numberRow(MavioStrings.assistantRetentionDays, count(\.retentionDays), prompt: "30")
                Toggle(isOn: binding(\.memory)) {
                    Text(MavioStrings.assistantServerMemory)
                    Text(MavioStrings.assistantServerMemoryDetail)
                }
                .disabled(off)
                Toggle(isOn: binding(\.hints)) {
                    Text(MavioStrings.assistantHints)
                    Text(MavioStrings.assistantHintsDetail)
                }
                .disabled(off)
            }
            Section {
                ForEach(store.draft.speech.indices, id: \.self) { i in
                    speechRow(i, saved)
                }
                Button(MavioStrings.assistantAddLanguage) {
                    var speech = Mavio_Assistant_V1_AssistantSpeech()
                    speech.language = Locale.current.language.languageCode?.identifier ?? "en"
                    store.draft.speech.append(speech)
                    store.notice = nil
                }
                .disabled(off)
                numberRow(
                    MavioStrings.assistantLongestPause,
                    seconds(\.longestPause, has: \.hasLongestPause, clear: { $0.clearLongestPause() }), prompt: "3",
                    decimals: true
                ).disabled(off)
                numberRow(
                    MavioStrings.assistantCommitDelay,
                    seconds(\.commitDelay, has: \.hasCommitDelay, clear: { $0.clearCommitDelay() }), prompt: "0.7",
                    decimals: true
                ).disabled(off)
                numberRow(
                    MavioStrings.assistantSessionIdle,
                    seconds(\.sessionIdle, has: \.hasSessionIdle, clear: { $0.clearSessionIdle() }), prompt: "8"
                )
                .disabled(off)
            } header: {
                Text(MavioStrings.assistantSpeech)
            } footer: {
                Text(MavioStrings.assistantSpeechFooter)
            }
            Section {
            } footer: {
                HStack(spacing: MavioSpacing.s12) {
                    Button(store.saving ? "…" : MavioStrings.actionSave, action: save)
                        .buttonStyle(.borderedProminent)
                        .disabled(!changed || store.saving)
                    if changed {
                        Button(MavioStrings.actionCancel) {
                            store.draft = saved.settings
                            store.saveFailure = nil
                        }
                        .buttonStyle(.borderless)
                    }
                    if let notice = store.notice, !changed {
                        Text(notice)
                    }
                    if let saveFailure = store.saveFailure {
                        Text(saveFailure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                    }
                }
                .type(theme.type.callout)
            }
        }

        private func speechRow(_ i: Int, _ saved: Mavio_Assistant_V1_GetAssistantSettingsResponse) -> some View {
            HStack(spacing: MavioSpacing.s8) {
                TextField(
                    MavioStrings.assistantLanguage,
                    text: Binding(
                        get: { store.draft.speech[safe: i]?.language ?? "" },
                        set: { v in edit(i) { $0.language = v.trimmingCharacters(in: .whitespaces) } }),
                    prompt: Text("en")
                )
                .labelsHidden()
                .textFieldStyle(.roundedBorder)
                .frame(width: 72)
                engines(
                    MavioStrings.assistantRecognizer, saved.recognizers,
                    Binding(
                        get: { store.draft.speech[safe: i]?.recognizer ?? "" },
                        set: { v in edit(i) { $0.recognizer = v } }))
                engines(
                    MavioStrings.assistantVoice, saved.voices,
                    Binding(get: { store.draft.speech[safe: i]?.voice ?? "" }, set: { v in edit(i) { $0.voice = v } }))
                Spacer(minLength: 0)
                Button(MavioStrings.actionDelete, role: .destructive) {
                    if store.draft.speech.indices.contains(i) { store.draft.speech.remove(at: i) }
                    store.notice = nil
                }
            }
            .disabled(off)
        }

        private func engines(
            _ label: String, _ list: [Mavio_Assistant_V1_VoiceEngine], _ value: Binding<String>
        ) -> some View {
            Picker(label, selection: value) {
                Text(MavioStrings.assistantFirstReady).tag("")
                ForEach(
                    choices(list.map { ($0.id, $0.name.isEmpty ? $0.id : $0.name) }, value.wrappedValue), id: \.code
                ) {
                    Text($0.name).tag($0.code)
                }
            }
            .labelsHidden()
            .frame(minWidth: 120, maxWidth: 220)
        }

        private func numberRow(_ label: String, _ value: Binding<Double?>, prompt: String, decimals: Bool = false)
            -> some View
        {
            FormRow(label: label) {
                TextField(
                    label, value: value,
                    format: .number.precision(.fractionLength(0...(decimals ? 2 : 0))).grouping(.never),
                    prompt: Text(prompt)
                )
                .fieldLabelHidden()
                .multilineTextAlignment(.trailing)
                .frame(width: 80)
            }
            .disabled(off)
        }

        // MARK: Bindings into the draft

        private func binding<T>(_ path: WritableKeyPath<Mavio_Assistant_V1_AssistantSettings, T>) -> Binding<T> {
            Binding(
                get: { store.draft[keyPath: path] },
                set: {
                    store.draft[keyPath: path] = $0
                    store.notice = nil
                })
        }

        private var access: Binding<Mavio_Assistant_V1_AssistantAccess> {
            Binding(
                get: { store.draft.aiAccess == .unspecified ? .all : store.draft.aiAccess },
                set: {
                    store.draft.aiAccess = $0
                    store.notice = nil
                })
        }

        private func chosen(_ id: String) -> Binding<Bool> {
            Binding(
                get: { store.draft.aiUserIds.contains(id) },
                set: { on in
                    store.draft.aiUserIds.removeAll { $0 == id }
                    if on { store.draft.aiUserIds.append(id) }
                    store.notice = nil
                })
        }

        /// A count of the draft, unset (0, the default) when the field is empty.
        private func count(_ path: WritableKeyPath<Mavio_Assistant_V1_AssistantSettings, Int32>) -> Binding<Double?> {
            Binding(
                get: { store.draft[keyPath: path] == 0 ? nil : Double(store.draft[keyPath: path]) },
                set: {
                    store.draft[keyPath: path] = Int32(($0 ?? 0).rounded())
                    store.notice = nil
                })
        }

        /// Seconds of a duration of the draft, unset when the field is empty, for the default.
        private func seconds(
            _ path: WritableKeyPath<Mavio_Assistant_V1_AssistantSettings, Google_Protobuf_Duration>,
            has: KeyPath<Mavio_Assistant_V1_AssistantSettings, Bool>,
            clear: @escaping (inout Mavio_Assistant_V1_AssistantSettings) -> Void
        ) -> Binding<Double?> {
            Binding(
                get: {
                    guard store.draft[keyPath: has] else { return nil }
                    let d = store.draft[keyPath: path]
                    return Double(d.seconds) + Double(d.nanos) / 1e9
                },
                set: { v in
                    if let v {
                        let ms = Int64((v * 1000).rounded())
                        store.draft[keyPath: path] = Google_Protobuf_Duration(
                            seconds: ms / 1000, nanos: Int32(ms % 1000) * 1_000_000)
                    } else {
                        // An unset duration: the message without it.
                        clear(&store.draft)
                    }
                    store.notice = nil
                })
        }

        private func edit(_ i: Int, _ change: (inout Mavio_Assistant_V1_AssistantSpeech) -> Void) {
            guard store.draft.speech.indices.contains(i) else { return }
            change(&store.draft.speech[i])
            store.notice = nil
        }

        private func save() {
            Task { await store.save(model, server) }
        }
    }

    /// The server's assistant settings as the AI page reads and saves them.
    @MainActor @Observable
    final class AiAssistantStore {
        private(set) var saved: Mavio_Assistant_V1_GetAssistantSettingsResponse?
        var draft = Mavio_Assistant_V1_AssistantSettings()
        private(set) var failure: String?
        private(set) var options: [AiPickerOption] = []
        private(set) var users: [Mavio_User_V1_User] = []
        private(set) var saving = false
        var saveFailure: String?
        var notice: String?

        /// Reads the settings, the models to choose and the users, leaving the draft as the server has it.
        func load(_ model: AppModel, _ server: ServerRecord) async {
            async let models = model.aiModelOptions(server, prefix: "ai/")
            async let list = try? model.call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self)
            do {
                let response = try await model.call(
                    server, Procedure.assistant("GetAssistantSettings"),
                    Mavio_Assistant_V1_GetAssistantSettingsRequest(),
                    as: Mavio_Assistant_V1_GetAssistantSettingsResponse.self)
                saved = response
                draft = response.settings
                failure = nil
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
            options = await models ?? []
            users = (await list)?.users ?? []
        }

        /// Saves the draft, saying so.
        func save(_ model: AppModel, _ server: ServerRecord) async {
            saving = true
            saveFailure = nil
            defer { saving = false }
            var request = Mavio_Assistant_V1_UpdateAssistantSettingsRequest()
            request.settings = draft
            do {
                let response = try await model.call(
                    server, Procedure.assistant("UpdateAssistantSettings"), request,
                    as: Mavio_Assistant_V1_UpdateAssistantSettingsResponse.self)
                saved?.settings = response.settings
                draft = response.settings
                notice = MavioStrings.assistantSaved
            } catch {
                saveFailure = error.reason
            }
        }
    }

    extension Array {
        /// The element at an index, nil past the end.
        fileprivate subscript(safe i: Int) -> Element? { indices.contains(i) ? self[i] : nil }
    }
#endif
