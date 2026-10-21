#if os(macOS)
    import AVFoundation
    import AppKit
    import MavioMac

    /// The Assistant settings of This Device (docs/clients.md §6.3.29), as
    /// the web UI's: the microphone voice sessions listen with, System
    /// default first; and the user's memories and conversations on the
    /// server chosen, each memory forgotten, a new one remembered, and each
    /// conversation, named by when it was, read or deleted, under each
    /// message a button copying it, under an answer whose speech was kept
    /// one playing it, and its time.
    final class AssistantPrefsPage: Page {
        private let server: ServerRecord?
        private lazy var form = FormPage(
            title: MavioStrings.assistantTitle, message: MavioStrings.assistantSettingsMessage, theme: theme)
        private let holder = NSStackView.vertical([], spacing: MavioSpacing.s32)
        private var info: Mavio_Assistant_V1_GetAssistantResponse?
        private var failure: String?
        private var memories: [Mavio_Assistant_V1_AssistantMemory]?
        private var memoryFailure: String?
        private var conversations: [Mavio_Assistant_V1_AssistantConversation]?
        private var conversationsFailure: String?
        private var shown: String?
        private var messages: [Mavio_Assistant_V1_AssistantMessage]?
        private var messagesFailure: String?
        private var busy = false
        private var tasks: [Task<Void, Never>] = []
        private var observers: [NSObjectProtocol] = []
        private lazy var kept: KeptSpeech? = server.map { server in
            let kept = KeptSpeech(model: model, server: server)
            kept.onChange = { [weak self] in self?.render() }
            return kept
        }
        private lazy var newMemory: InputField = {
            let field = InputField(text: "", placeholder: MavioStrings.assistantNewMemory)
            field.setAccessibilityLabel(MavioStrings.assistantNewMemory)
            field.onChange = { [weak self] _ in self?.rememberButton.isEnabled = self?.canRemember == true }
            field.onSubmit = { [weak self] in self?.remember() }
            return field
        }()
        private lazy var rememberButton = MavioButton(MavioStrings.assistantRemember, theme: theme) { [weak self] in
            self?.remember()
        }

        /// The Assistant settings of the user on server, none when signed
        /// in to none.
        init(context: MacContext, server: ServerRecord?) {
            self.server = server
            super.init(context: context)
        }

        isolated deinit {
            tasks.forEach { $0.cancel() }
            observers.forEach { NotificationCenter.default.removeObserver($0) }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.assistantTitle
            form.add(holder)
            show(form)
            // Microphones come and go.
            for name in [AVCaptureDevice.wasConnectedNotification, AVCaptureDevice.wasDisconnectedNotification] {
                observers.append(
                    NotificationCenter.default.addObserver(forName: name, object: nil, queue: .main) {
                        [weak self] _ in MainActor.assumeIsolated { self?.render() }
                    })
            }
            render()
            run { [weak self] in await self?.load() }
        }

        private func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        private func load() async {
            guard let server else { return }
            var request = Mavio_Assistant_V1_GetAssistantRequest()
            request.language = Assistant.uiLanguage
            do {
                info = try await model.call(
                    server, Procedure.assistant("GetAssistant"), request,
                    as: Mavio_Assistant_V1_GetAssistantResponse.self)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
            guard let info, info.enabled else { return }
            async let m: Void = loadMemories()
            async let c: Void = info.ai ? loadConversations() : ()
            _ = await (m, c)
        }

        private func loadMemories() async {
            guard let server, info?.memory == true else { return }
            do {
                memories = try await model.call(
                    server, Procedure.assistant("ListAssistantMemories"),
                    Mavio_Assistant_V1_ListAssistantMemoriesRequest(),
                    as: Mavio_Assistant_V1_ListAssistantMemoriesResponse.self
                ).memories
                memoryFailure = nil
            } catch is CancellationError {
                return
            } catch {
                memoryFailure = error.reason
            }
            render()
        }

        private func loadConversations() async {
            guard let server else { return }
            do {
                conversations = try await model.call(
                    server, Procedure.assistant("ListAssistantConversations"),
                    Mavio_Assistant_V1_ListAssistantConversationsRequest(),
                    as: Mavio_Assistant_V1_ListAssistantConversationsResponse.self
                ).conversations
                conversationsFailure = nil
            } catch is CancellationError {
                return
            } catch {
                conversationsFailure = error.reason
            }
            render()
        }

        // MARK: Drawing

        private func render() {
            var views: [NSView] = []
            if let info {
                if !info.enabled {
                    views = [FormSection(rows: [FormRow(MavioStrings.assistantOff, theme: theme)], theme: theme)]
                } else {
                    if info.voice { views.append(microphoneSection()) }
                    views.append(memorySection(info))
                    if info.ai { views.append(conversationsSection()) }
                }
            } else if let failure {
                views = [plainNote(failure, theme.colors.danger, theme: theme)]
            } else if server != nil {
                views = [LoadingView()]
            }
            holder.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: holder.widthAnchor).isActive = true }
            FocusEngine.active(for: view)?.schedule()
        }

        /// The microphone voice sessions listen with on this Mac.
        private func microphoneSection() -> NSView {
            let all = Microphones.all()
            let chosen = Microphones.chosen.flatMap { id in all.contains { $0.id == id } ? id : nil }
            let options: [(String, String?)] =
                [(MavioStrings.assistantSystemMicrophone, nil)] + all.map { ($0.name, $0.id) }
            let row = choiceRow(
                MavioStrings.assistantMicrophoneChoice, options: options, selected: chosen, theme: theme
            ) { id in Microphones.chosen = id }
            return FormSection(footer: MavioStrings.assistantDeviceMicrophoneFooter, rows: [row], theme: theme)
        }

        private func memorySection(_ info: Mavio_Assistant_V1_GetAssistantResponse) -> NSView {
            guard info.memory else {
                return FormSection(
                    MavioStrings.assistantMemory,
                    rows: [
                        FormRow(
                            info.ai ? MavioStrings.assistantMemoryOff : MavioStrings.assistantMemoryNoAi, theme: theme)
                    ], theme: theme)
            }
            let list = memories ?? []
            var rows: [NSView] = list.map { m in
                let forget = more(MavioStrings.assistantForget) { [weak self] in self?.forget([m.id]) }
                return FormRow(
                    m.text,
                    subtitle: m.hasUpdateTime ? m.updateTime.date.formatted(date: .abbreviated, time: .shortened) : nil,
                    accessory: forget, theme: theme)
            }
            if memories?.isEmpty == true { rows.append(FormRow(MavioStrings.assistantNoMemories, theme: theme)) }
            rememberButton.isEnabled = canRemember
            var bar: [NSView] = [newMemory, rememberButton]
            if !list.isEmpty {
                bar.append(
                    more(MavioStrings.assistantForgetAll) { [weak self] in
                        guard let self else { return }
                        confirm(
                            MavioStrings.assistantForgetAllConfirm, message: nil,
                            action: MavioStrings.assistantForgetAll, in: view
                        ) { [weak self] in self?.forget([]) }
                    })
            }
            let stack = NSStackView.horizontal(bar, spacing: MavioSpacing.s8)
            newMemory.setContentHuggingPriority(.init(1), for: .horizontal)
            rows.append(ContentRow(stack, theme: theme))
            if let memoryFailure { rows.append(noteRow(memoryFailure, theme.colors.danger, theme: theme)) }
            return FormSection(
                MavioStrings.assistantMemory, footer: MavioStrings.assistantMemoryFooter, rows: rows, theme: theme)
        }

        /// A button of three dots opening a menu of one destructive item,
        /// titled title, doing run.
        private func more(_ title: String, run: @escaping @MainActor () -> Void) -> NSView {
            let button = MavioButton(icon: "moreVertical", kind: .plain, theme: theme)
            button.setAccessibilityLabel(MavioStrings.itemMore)
            button.toolTip = MavioStrings.itemMore
            button.isEnabled = !busy
            button.onPress = { [weak button, theme] in
                guard let button else { return }
                let menu = NSMenu()
                let item = MenuAction(title, run: run)
                item.attributedTitle = NSAttributedString(
                    string: title,
                    attributes: [.foregroundColor: theme.colors.danger, .font: NSFont.menuFont(ofSize: 0)])
                menu.addItem(item)
                menu.popUp(
                    positioning: nil, at: NSPoint(x: 0, y: button.isFlipped ? button.bounds.height + 4 : -4), in: button
                )
            }
            return button
        }

        private func conversationsSection() -> NSView {
            var rows: [NSView] = []
            for c in conversations ?? [] {
                // The assistant's conversations have no titles: they go by
                // when they were, and where.
                let when = c.hasUpdateTime ? c.updateTime.date.formatted(date: .abbreviated, time: .shortened) : ""
                let detail = [c.deviceName, c.title.isEmpty ? "" : when].filter { !$0.isEmpty }.joined(separator: " · ")
                let row = FormRow(
                    c.title.isEmpty ? when : c.title, subtitle: detail.isEmpty ? nil : detail, theme: theme
                ) { [weak self] in self?.toggle(c.id) }
                row.setAccessibilityExpanded(shown == c.id)
                guard shown == c.id else {
                    rows.append(row)
                    continue
                }
                // The conversation opened and its messages are one item.
                let item = NSStackView.vertical([row, messagesRow(c.id)], spacing: 0)
                for v in item.arrangedSubviews { v.widthAnchor.constraint(equalTo: item.widthAnchor).isActive = true }
                rows.append(item)
            }
            if conversations?.isEmpty == true {
                rows.append(FormRow(MavioStrings.assistantNoConversations, theme: theme))
            }
            if let conversationsFailure {
                rows.append(noteRow(conversationsFailure, theme.colors.danger, theme: theme))
            }
            return FormSection(
                MavioStrings.assistantConversations, footer: MavioStrings.assistantConversationsFooter, rows: rows,
                flush: true, theme: theme)
        }

        /// A conversation's messages, with room above them, and deleting it.
        private func messagesRow(_ id: String) -> NSView {
            var views: [NSView] = (messages ?? []).enumerated().map { i, m in
                let key = String(i)
                return AssistantBubble(
                    m, playing: kept?.at == key,
                    onPlay: m.speechURL.isEmpty ? nil : { [weak self] in self?.kept?.toggle(key, url: m.speechURL) },
                    theme: theme)
            }
            if messages == nil, messagesFailure == nil {
                let spinner = NSProgressIndicator()
                spinner.style = .spinning
                spinner.controlSize = .small
                spinner.startAnimation(nil)
                let loading = NSView()
                loading.translatesAutoresizingMaskIntoConstraints = false
                loading.addSubview(spinner)
                spinner.translatesAutoresizingMaskIntoConstraints = false
                NSLayoutConstraint.activate([
                    spinner.centerXAnchor.constraint(equalTo: loading.centerXAnchor),
                    spinner.centerYAnchor.constraint(equalTo: loading.centerYAnchor),
                    loading.heightAnchor.constraint(equalToConstant: 32),
                ])
                views.append(loading)
            }
            if let messagesFailure {
                views.append(note(messagesFailure, theme.fonts.callout, theme.colors.danger, selectable: true))
            }
            let delete = MavioButton(MavioStrings.assistantDeleteConversation, kind: .destructive, theme: theme) {
                [weak self] in self?.delete(id)
            }
            delete.isEnabled = !busy
            views.append(delete)
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s16)
            for v in views where !(v is MavioButton) {
                v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            }
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(column, insets: NSEdgeInsets(top: MavioSpacing.s4, left: 0, bottom: 0, right: 0))
            return ContentRow(holder, theme: theme)
        }

        // MARK: Changing

        private var canRemember: Bool {
            !busy && !newMemory.field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }

        private func remember() {
            guard let server, canRemember else { return }
            var request = Mavio_Assistant_V1_UpdateAssistantMemoryRequest()
            request.text = newMemory.field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
            change { [weak self] in
                guard let self else { return }
                _ = try await model.call(
                    server, Procedure.assistant("UpdateAssistantMemory"), request,
                    as: Mavio_Assistant_V1_UpdateAssistantMemoryResponse.self)
                newMemory.field.stringValue = ""
                await loadMemories()
            }
        }

        /// Forgets the memories of ids, all of them for none.
        private func forget(_ ids: [String]) {
            guard let server else { return }
            var request = Mavio_Assistant_V1_DeleteAssistantMemoriesRequest()
            request.ids = ids
            change { [weak self] in
                guard let self else { return }
                _ = try await model.call(
                    server, Procedure.assistant("DeleteAssistantMemories"), request,
                    as: Mavio_Assistant_V1_DeleteAssistantMemoriesResponse.self)
                await loadMemories()
            }
        }

        private func change(_ work: @escaping @MainActor () async throws -> Void) {
            busy = true
            memoryFailure = nil
            render()
            run { [weak self] in
                do {
                    try await work()
                } catch is CancellationError {
                } catch {
                    self?.memoryFailure = error.reason
                }
                self?.busy = false
                self?.render()
            }
        }

        private func toggle(_ id: String) {
            kept?.stop()
            guard shown != id, let server else {
                shown = nil
                render()
                return
            }
            shown = id
            messages = nil
            messagesFailure = nil
            render()
            var request = Mavio_Assistant_V1_GetAssistantConversationRequest()
            request.id = id
            run { [weak self] in
                guard let self else { return }
                do {
                    let r = try await model.call(
                        server, Procedure.assistant("GetAssistantConversation"), request,
                        as: Mavio_Assistant_V1_GetAssistantConversationResponse.self)
                    guard shown == id else { return }
                    messages = r.messages
                } catch is CancellationError {
                    return
                } catch {
                    guard shown == id else { return }
                    messagesFailure = error.reason
                }
                render()
            }
        }

        private func delete(_ id: String) {
            guard let server else { return }
            var request = Mavio_Assistant_V1_DeleteAssistantConversationRequest()
            request.id = id
            busy = true
            render()
            run { [weak self] in
                guard let self else { return }
                do {
                    _ = try await model.call(
                        server, Procedure.assistant("DeleteAssistantConversation"), request,
                        as: Mavio_Assistant_V1_DeleteAssistantConversationResponse.self)
                    kept?.stop()
                    shown = nil
                    await loadConversations()
                } catch is CancellationError {
                } catch {
                    messagesFailure = error.reason
                }
                busy = false
                render()
            }
        }
    }

    /// A message of the user's conversation with the assistant: the user's
    /// on the right in the button's color, the answer's on the left on the
    /// fill, each short of the other side; under it its bar.
    final class AssistantBubble: NSView {
        /// The message m; onPlay plays its kept speech, playing while it
        /// does.
        init(
            _ m: Mavio_Assistant_V1_AssistantMessage, playing: Bool, onPlay: (@MainActor () -> Void)?, theme: Theme
        ) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let label = NSTextField.label(
                m.text, theme.fonts.callout, color: m.user ? theme.colors.onButton : theme.colors.textPrimary, lines: 0,
                selectable: true)
            let box = Box(fill: m.user ? theme.colors.button : theme.colors.fill, radius: 14)
            box.addFilling(label, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s8))
            let bar = messageBar(
                m.text, at: m.hasTime ? m.time.date : nil, playing: playing, onPlay: onPlay, theme: theme)
            let stack = NSStackView.vertical([box, bar], spacing: 2, alignment: m.user ? .trailing : .leading)
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.topAnchor.constraint(equalTo: topAnchor),
                stack.bottomAnchor.constraint(equalTo: bottomAnchor),
                m.user
                    ? stack.trailingAnchor.constraint(equalTo: trailingAnchor)
                    : stack.leadingAnchor.constraint(equalTo: leadingAnchor),
                stack.widthAnchor.constraint(lessThanOrEqualTo: widthAnchor, multiplier: 0.85),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }
#endif
