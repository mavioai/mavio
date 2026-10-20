#if os(macOS)
    import AppKit
    import MavioMac

    /// The server's assistant settings, in its AI settings, as the web UI's
    /// AssistantServerSettings (docs/assistant.md §10): whether it is on, its
    /// model, code mode, who may use the AI path, how long conversations stay
    /// idle and are kept, memory and hints; and the recognizer and voice of
    /// each language with the timing of turns. Its sections are drawn anew
    /// as a choice changes; typing changes only its Save bar.
    @MainActor final class AssistantSettings {
        /// The sections, to place in the page's column.
        let view = NSStackView.vertical([], spacing: MavioSpacing.s32)
        private let server: ServerRecord
        private let model: AppModel
        private let theme: Theme
        private var saved: Mavio_Assistant_V1_GetAssistantSettingsResponse?
        private var draft = Mavio_Assistant_V1_AssistantSettings()
        private var failure: String?
        private var options: [PickerItem] = []
        private var users: [Mavio_User_V1_User] = []
        private var saving = false
        private var saveFailure: String?
        private var notice: String?
        private var tasks: [Task<Void, Never>] = []
        private lazy var saveButton = MavioButton(MavioStrings.actionSave, kind: .primary, theme: theme) {
            [weak self] in self?.save()
        }
        private lazy var cancelButton = MavioButton(MavioStrings.actionCancel, kind: .plain, theme: theme) {
            [weak self] in
            guard let self, let saved = self.saved else { return }
            self.draft = saved.settings
            self.saveFailure = nil
            self.render()
        }
        private lazy var noticeLabel = note("", theme.fonts.callout, theme.colors.textSecondary)
        private lazy var failureLabel = note("", theme.fonts.callout, theme.colors.danger, selectable: true)

        /// The settings of server, read and saved as page's model does.
        init(server: ServerRecord, page: Page) {
            self.server = server
            model = page.model
            theme = page.theme
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        /// Reads the settings, the models to choose and the users, leaving
        /// the draft as the server has it.
        func load() {
            tasks.append(
                Task { [weak self, model, server] in
                    async let models = model.aiModelItems(server, prefix: "ai/")
                    async let list = try? model.call(
                        server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                        as: Mavio_User_V1_ListUsersResponse.self)
                    let result: Result<Mavio_Assistant_V1_GetAssistantSettingsResponse, any Error>
                    do {
                        result = .success(
                            try await model.call(
                                server, Procedure.assistant("GetAssistantSettings"),
                                Mavio_Assistant_V1_GetAssistantSettingsRequest(),
                                as: Mavio_Assistant_V1_GetAssistantSettingsResponse.self))
                    } catch {
                        result = .failure(error)
                    }
                    let options = await models ?? []
                    let users = (await list)?.users ?? []
                    guard let self, !Task.isCancelled else { return }
                    switch result {
                    case .success(let response):
                        self.saved = response
                        self.draft = response.settings
                        self.failure = nil
                    case .failure(let error):
                        if error is CancellationError { return }
                        self.failure = error.reason
                    }
                    self.options = options
                    self.users = users
                    self.render()
                })
        }

        private var off: Bool { !draft.enabled }

        // MARK: Drawing

        private func render() {
            var views: [NSView] = []
            if let saved, saved.hasSettings {
                views = [serverSection(), speechSection(saved), saveBar()]
            } else if let failure {
                views = [plainNote(failure, theme.colors.danger, theme: theme)]
            }
            view.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
            updateBar()
            FocusEngine.active(for: view)?.schedule()
        }

        /// Edits the draft; a choice draws the sections anew, typing only the
        /// Save bar.
        private func edit(redraw: Bool = true, _ change: (inout Mavio_Assistant_V1_AssistantSettings) -> Void) {
            change(&draft)
            notice = nil
            if redraw { render() } else { updateBar() }
        }

        private func serverSection() -> NSView {
            var rows: [NSView] = []
            rows.append(
                toggleRow(
                    MavioStrings.assistantEnabled, subtitle: MavioStrings.assistantEnabledDetail, isOn: draft.enabled,
                    theme: theme
                ) { [weak self] on in self?.edit { $0.enabled = on } })
            let picker = SearchPicker(
                items: [PickerItem(value: "", label: MavioStrings.assistantNoModel)] + options, value: draft.model,
                placeholder: MavioStrings.assistantNoModel, theme: theme
            ) { [weak self] v in self?.edit { $0.model = v } }
            picker.isEnabled = !off
            picker.setAccessibilityLabel(MavioStrings.assistantModel)
            rows.append(controlRow(MavioStrings.assistantModel, control: picker, enabled: !off, theme: theme))
            let aiOn = !off && !draft.model.isEmpty
            rows.append(
                toggleRow(
                    MavioStrings.assistantCodeMode, subtitle: MavioStrings.assistantCodeModeDetail,
                    isOn: draft.codeMode, enabled: aiOn, theme: theme
                ) { [weak self] on in self?.edit { $0.codeMode = on } })
            rows.append(
                choiceRow(
                    MavioStrings.assistantAiAccess,
                    options: [
                        (MavioStrings.assistantAccessAll, Mavio_Assistant_V1_AssistantAccess.all),
                        (MavioStrings.assistantAccessAdmins, .administrators),
                        (MavioStrings.assistantAccessSelected, .selected),
                    ],
                    selected: draft.aiAccess == .unspecified ? .all : draft.aiAccess, enabled: aiOn, theme: theme
                ) { [weak self] v in self?.edit { $0.aiAccess = v } })
            if draft.aiAccess == .selected, !users.isEmpty {
                let boxes = users.map { u in
                    focusable(
                        ActionCheckBox(u.name, isOn: draft.aiUserIds.contains(u.id)) { [weak self] on in
                            self?.edit(redraw: false) {
                                $0.aiUserIds.removeAll { $0 == u.id }
                                if on { $0.aiUserIds.append(u.id) }
                            }
                        }, theme: theme)
                }
                rows.append(
                    ContentRow(WrapView(boxes, spacing: MavioSpacing.s20, lineSpacing: MavioSpacing.s8), theme: theme))
            }
            rows.append(
                numberRow(MavioStrings.assistantIdleMinutes, count(draft.idleMinutes), prompt: "30") { [weak self] v in
                    self?.edit(redraw: false) { $0.idleMinutes = Int32((v ?? 0).rounded()) }
                })
            rows.append(
                numberRow(MavioStrings.assistantRetentionDays, count(draft.retentionDays), prompt: "30") {
                    [weak self] v in
                    self?.edit(redraw: false) { $0.retentionDays = Int32((v ?? 0).rounded()) }
                })
            rows.append(
                toggleRow(
                    MavioStrings.assistantServerMemory, subtitle: MavioStrings.assistantServerMemoryDetail,
                    isOn: draft.memory, enabled: !off, theme: theme
                ) { [weak self] on in self?.edit { $0.memory = on } })
            rows.append(
                toggleRow(
                    MavioStrings.assistantHints, subtitle: MavioStrings.assistantHintsDetail, isOn: draft.hints,
                    enabled: !off, theme: theme
                ) { [weak self] on in self?.edit { $0.hints = on } })
            return FormSection(MavioStrings.assistantServer, rows: rows, theme: theme)
        }

        private func speechSection(_ saved: Mavio_Assistant_V1_GetAssistantSettingsResponse) -> NSView {
            var rows: [NSView] = draft.speech.indices.map { speechRow($0, saved) }
            let add = FormRow.action(MavioStrings.assistantAddLanguage, theme: theme) { [weak self] in
                guard let self, !self.off else { return }
                var speech = Mavio_Assistant_V1_AssistantSpeech()
                speech.language = Locale.current.language.languageCode?.identifier ?? "en"
                self.edit { $0.speech.append(speech) }
            }
            if off {
                add.onPress = nil
                add.alphaValue = 0.5
            }
            rows.append(add)
            rows.append(
                numberRow(
                    MavioStrings.assistantLongestPause, seconds(draft.longestPause, has: draft.hasLongestPause),
                    prompt: "3", decimals: true
                ) { [weak self] v in
                    self?.edit(redraw: false) { d in
                        if let v { d.longestPause = duration(v) } else { d.clearLongestPause() }
                    }
                })
            rows.append(
                numberRow(
                    MavioStrings.assistantCommitDelay, seconds(draft.commitDelay, has: draft.hasCommitDelay),
                    prompt: "0.7", decimals: true
                ) { [weak self] v in
                    self?.edit(redraw: false) { d in
                        if let v { d.commitDelay = duration(v) } else { d.clearCommitDelay() }
                    }
                })
            rows.append(
                numberRow(
                    MavioStrings.assistantSessionIdle, seconds(draft.sessionIdle, has: draft.hasSessionIdle),
                    prompt: "8"
                ) { [weak self] v in
                    self?.edit(redraw: false) { d in
                        if let v { d.sessionIdle = duration(v) } else { d.clearSessionIdle() }
                    }
                })
            return FormSection(
                MavioStrings.assistantSpeech, footer: MavioStrings.assistantSpeechFooter, rows: rows, theme: theme)
        }

        private func speechRow(_ i: Int, _ saved: Mavio_Assistant_V1_GetAssistantSettingsResponse) -> NSView {
            let speech = draft.speech[i]
            let language = InputField(text: speech.language, placeholder: "en", width: 72)
            language.setAccessibilityLabel(MavioStrings.assistantLanguage)
            language.isEnabled = !off
            language.onChange = { [weak self] v in
                self?.edit(redraw: false) { d in
                    if d.speech.indices.contains(i) { d.speech[i].language = v.trimmingCharacters(in: .whitespaces) }
                }
            }
            let recognizer = engines(MavioStrings.assistantRecognizer, saved.recognizers, speech.recognizer) {
                [weak self] v in
                self?.edit { d in if d.speech.indices.contains(i) { d.speech[i].recognizer = v } }
            }
            let voice = engines(MavioStrings.assistantVoice, saved.voices, speech.voice) { [weak self] v in
                self?.edit { d in if d.speech.indices.contains(i) { d.speech[i].voice = v } }
            }
            let delete = MavioButton(MavioStrings.actionDelete, kind: .destructive, theme: theme) { [weak self] in
                self?.edit { d in if d.speech.indices.contains(i) { d.speech.remove(at: i) } }
            }
            delete.isEnabled = !off
            let views = [focusable(language, theme: theme), recognizer, voice, Spacer(), delete]
            return ContentRow(bar(views, spacing: MavioSpacing.s8), theme: theme)
        }

        private func engines(
            _ label: String, _ list: [Mavio_Assistant_V1_VoiceEngine], _ value: String,
            onChange: @escaping @MainActor (String) -> Void
        ) -> NSView {
            var all = list.map { (code: $0.id, name: $0.name.isEmpty ? $0.id : $0.name) }
            if !value.isEmpty, !all.contains(where: { $0.code == value }) { all.append((value, value)) }
            let codes = [""] + all.map(\.code)
            let popup = ActionPopUp(
                options: [MavioStrings.assistantFirstReady] + all.map(\.name),
                selected: codes.firstIndex(of: value) ?? 0
            ) { onChange(codes[$0]) }
            popup.setAccessibilityLabel(label)
            popup.isEnabled = !off
            popup.widthAnchor.constraint(greaterThanOrEqualToConstant: 120).isActive = true
            popup.widthAnchor.constraint(lessThanOrEqualToConstant: 220).isActive = true
            return focusable(popup, theme: theme)
        }

        /// A row of a number, set to nil when the field is emptied; what is
        /// not a number is left aside.
        private func numberRow(
            _ label: String, _ value: Double?, prompt: String, decimals: Bool = false,
            onChange: @escaping @MainActor (Double?) -> Void
        ) -> FormRow {
            let text = value.map {
                $0.formatted(.number.precision(.fractionLength(0...(decimals ? 2 : 0))).grouping(.never))
            }
            let field = InputField(text: text ?? "", placeholder: prompt, width: 80)
            field.field.alignment = .right
            field.isEnabled = !off
            field.onChange = { t in
                let trimmed = t.trimmingCharacters(in: .whitespaces).replacingOccurrences(of: ",", with: ".")
                if trimmed.isEmpty {
                    onChange(nil)
                } else if let n = Double(trimmed) {
                    onChange(decimals ? n : n.rounded())
                }
            }
            return controlRow(label, control: field, enabled: !off, theme: theme)
        }

        /// A count, unset (0, the default) when the field is empty.
        private func count(_ v: Int32) -> Double? { v == 0 ? nil : Double(v) }

        /// Seconds of a duration, unset when the field is empty, for the default.
        private func seconds(_ d: Google_Protobuf_Duration, has: Bool) -> Double? {
            has ? Double(d.seconds) + Double(d.nanos) / 1e9 : nil
        }

        // MARK: Saving

        private func saveBar() -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(
                bar([saveButton, cancelButton, noticeLabel, failureLabel]),
                insets: .symmetric(horizontal: MavioSpacing.s4))
            return holder
        }

        private func updateBar() {
            guard let saved else { return }
            let changed = draft != saved.settings
            saveButton.title = saving ? "…" : MavioStrings.actionSave
            saveButton.isEnabled = changed && !saving
            cancelButton.isHidden = !changed
            noticeLabel.stringValue = notice ?? ""
            noticeLabel.isHidden = notice == nil || changed
            failureLabel.stringValue = saveFailure ?? ""
            failureLabel.isHidden = saveFailure == nil
        }

        /// Saves the draft, saying so.
        private func save() {
            guard !saving else { return }
            saving = true
            saveFailure = nil
            updateBar()
            var request = Mavio_Assistant_V1_UpdateAssistantSettingsRequest()
            request.settings = draft
            tasks.append(
                Task { [weak self, model, server] in
                    let result: Result<Mavio_Assistant_V1_UpdateAssistantSettingsResponse, any Error>
                    do {
                        result = .success(
                            try await model.call(
                                server, Procedure.assistant("UpdateAssistantSettings"), request,
                                as: Mavio_Assistant_V1_UpdateAssistantSettingsResponse.self))
                    } catch {
                        result = .failure(error)
                    }
                    guard let self else { return }
                    self.saving = false
                    switch result {
                    case .success(let response):
                        self.saved?.settings = response.settings
                        self.draft = response.settings
                        self.notice = MavioStrings.assistantSaved
                        self.render()
                    case .failure(let error):
                        self.saveFailure = error.reason
                        self.updateBar()
                    }
                })
        }
    }

    /// A duration of seconds, to the millisecond.
    private func duration(_ v: Double) -> Google_Protobuf_Duration {
        let ms = Int64((v * 1000).rounded())
        return Google_Protobuf_Duration(seconds: ms / 1000, nanos: Int32(ms % 1000) * 1_000_000)
    }
#endif
