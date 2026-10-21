#if os(macOS)
    import AppKit
    import MavioMac

    /// The AI settings' conversations (docs/clients.md §6.3.26), as the web
    /// UI's, at the end of the AI page: the administrators' conversations
    /// with the models of the AI providers, the users' with the assistant
    /// among them, and a new one started with a model, instructions and its
    /// first message; a conversation's page has its messages with the answer
    /// as the model writes it, each with a button copying it, an answer of
    /// the user's assistant conversation whose speech was kept a button
    /// playing it, and its time.
    @MainActor final class AIConversations {
        /// The sections, to place in the page's column.
        let view = NSStackView.vertical([], spacing: MavioSpacing.s32)
        private weak var page: Page?
        private let server: ServerRecord
        private let model: AppModel
        private let theme: Theme
        private lazy var newSection = LiveSection(
            MavioStrings.conversationsNew, footer: MavioStrings.conversationsNewFooter, theme: theme)
        private var conversations: [Mavio_Plugin_V1_AgentSession]?
        private var names: [String: String] = [:]
        private var failure: String?
        private var noPlugin = false
        private var options: [PickerItem]?
        private var loading: Task<Void, Never>?
        private var creatingTask: Task<Void, Never>?

        // A new conversation.
        private var chosen = ""
        private var creating = false
        private var createFailure: String?
        private lazy var picker = SearchPicker(
            items: [], value: nil, placeholder: MavioStrings.conversationsModel, theme: theme
        ) { [weak self] v in
            self?.chosen = v
            self?.renderNew()
        }
        private lazy var pickerRow = controlRow(MavioStrings.conversationsModel, control: picker, theme: theme)
        private lazy var instructions = TextArea(
            placeholder:
                "\(MavioStrings.conversationsInstructions) (\(MavioStrings.conversationsInstructionsPlaceholder))",
            font: theme.fonts.callout.font, limit: 20000, fill: theme.colors.background, theme: theme)
        private lazy var composer = Composer(
            action: MavioStrings.conversationsStart, dictation: Dictation(model: model, server: server), theme: theme
        ) { [weak self] in self?.start() }
        private lazy var createNote = note("", theme.fonts.callout, theme.colors.danger, selectable: true)
        private lazy var fieldsRow: ContentRow = {
            let column = NSStackView.vertical([instructions, composer, createNote], spacing: MavioSpacing.s12)
            for v in [instructions, composer] as [NSView] {
                v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            }
            return ContentRow(column, theme: theme)
        }()
        private lazy var noModelsRow = noteRow(
            MavioStrings.conversationsNoModels, theme.colors.textSecondary, theme: theme)

        /// The conversations of server, on page.
        init(server: ServerRecord, page: Page) {
            self.server = server
            self.page = page
            model = page.model
            theme = page.theme
            picker.setAccessibilityLabel(MavioStrings.conversationsModel)
        }

        isolated deinit {
            loading?.cancel()
            creatingTask?.cancel()
        }

        /// Reads the conversations, the models to start one with and the
        /// users whose assistant conversations they are.
        func load() {
            render()
            loading?.cancel()
            loading = Task { [weak self] in await self?.reload() }
        }

        private func reload() async {
            let server = server
            let model = model
            async let models = model.aiModelItems(server)
            async let users = try? model.call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self)
            do {
                conversations = try await model.call(
                    server, Procedure.aiConversations("ListAiConversations"),
                    Mavio_System_V1_ListAiConversationsRequest(), as: Mavio_System_V1_ListAiConversationsResponse.self
                ).conversations
                failure = nil
                noPlugin = false
            } catch is CancellationError {
                return
            } catch {
                noPlugin = isNoAiGateway(error)
                failure = error.reason
            }
            if let list = await models { options = list }
            if let list = await users?.users {
                names = Dictionary(list.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
            }
            render()
        }

        // MARK: Drawing

        private func render() {
            var views: [NSView]
            if noPlugin {
                let row = pageLinkRow(
                    icon: "plugins", color: NSColor(hex: 0xAF52DE), label: MavioStrings.conversationsNoPlugin,
                    detail: MavioStrings.aiNoGatewayDetail, theme: theme
                ) { [weak self] in
                    guard let self, let page else { return }
                    page.push(PluginsPage(context: page.context, server: server))
                }
                views = [
                    FormSection(
                        MavioStrings.settingsConversations, footer: MavioStrings.conversationsNoPluginFooter,
                        rows: [row], theme: theme)
                ]
            } else {
                renderNew()
                views = [newSection, listSection()]
            }
            view.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: view.widthAnchor).isActive = true }
            if let page { FocusEngine.active(for: page.view)?.schedule() }
        }

        private var current: String { chosen.isEmpty ? options?.first?.value ?? "" : chosen }

        private func renderNew() {
            if let options, options.isEmpty {
                newSection.setRows([noModelsRow])
                newSection.footer = nil
                return
            }
            let current = current
            picker.setItems(options ?? [])
            picker.value = current
            composer.label = MavioStrings.conversationsPlaceholder(
                model: options?.first { $0.value == current }?.label ?? "")
            composer.busy = creating
            composer.disabled = current.isEmpty
            createNote.stringValue = createFailure ?? ""
            createNote.isHidden = createFailure == nil
            newSection.setRows([pickerRow, fieldsRow])
            newSection.footer = MavioStrings.conversationsNewFooter
        }

        private func listSection() -> NSView {
            var rows: [NSView] = (conversations ?? []).map { c in
                FormRow.link(conversationsTitle(c), subtitle: detail(c), theme: theme) { [weak self] in
                    self?.openConversation(c.id, first: nil)
                }
            }
            if conversations?.isEmpty == true {
                rows.append(noteRow(MavioStrings.conversationsNone, theme.colors.textSecondary, theme: theme))
            }
            if conversations == nil, failure == nil { rows.append(spinnerRow(theme: theme)) }
            if let failure { rows.append(noteRow(failure, theme.colors.danger, theme: theme)) }
            return FormSection(
                MavioStrings.conversationsList, footer: MavioStrings.conversationsMessage, rows: rows, theme: theme)
        }

        private func detail(_ c: Mavio_Plugin_V1_AgentSession) -> String {
            // An assistant's conversation says whose it is, and where it
            // started.
            let assistant = conversationsIsAssistant(c)
            return [
                assistant ? names[c.owner] : nil, assistant ? c.labels["device"] : nil,
                conversationsModelName(conversationsModelOf(options ?? [], c)), c.updated.time.map(fromNow),
                conversationsUsage(c.usage),
            ]
            .compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
        }

        private func openConversation(_ id: String, first: String?) {
            guard let page else { return }
            page.push(
                ConversationPage(context: page.context, server: server, id: id, first: first) { [weak self] in
                    await self?.reload()
                })
        }

        // MARK: Starting

        private func start() {
            let chosen = current
            let message = composer.text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard let slash = chosen.firstIndex(of: "/"), !message.isEmpty, !creating else { return }
            creating = true
            createFailure = nil
            renderNew()
            var request = Mavio_System_V1_CreateAiConversationRequest()
            request.provider = String(chosen[..<slash])
            request.model = String(chosen[chosen.index(after: slash)...])
            request.instructions = instructions.text.trimmingCharacters(in: .whitespacesAndNewlines)
            request.title = String(
                (message.split(separator: "\n", omittingEmptySubsequences: false).first ?? "").prefix(80))
            creatingTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let response = try await model.call(
                        server, Procedure.aiConversations("CreateAiConversation"), request,
                        as: Mavio_System_V1_CreateAiConversationResponse.self)
                    creating = false
                    composer.text = ""
                    instructions.text = ""
                    renderNew()
                    openConversation(response.conversation.id, first: message)
                    await reload()
                } catch {
                    creating = false
                    createFailure = error.reason
                    renderNew()
                }
            }
        }
    }

    /// Whether a conversation is one of the assistant's: the server labels
    /// them so.
    func conversationsIsAssistant(_ c: Mavio_Plugin_V1_AgentSession) -> Bool {
        !(c.labels["mavio.assistant"] ?? "").isEmpty
    }

    /// A conversation's title; the assistant's have none, and are named so.
    func conversationsTitle(_ c: Mavio_Plugin_V1_AgentSession) -> String {
        if !c.title.isEmpty { return c.title }
        return conversationsIsAssistant(c) ? MavioStrings.conversationsAssistant : MavioStrings.conversationsUntitled
    }

    /// A message's text, a user's message to the assistant parted from the
    /// context line the assistant put before it for the model, as the
    /// assistant's own conversations leave it out (docs/assistant.md).
    func conversationsParted(_ assistant: Bool, _ text: String) -> (context: String?, text: String) {
        guard assistant, text.hasPrefix("["), let end = text.range(of: "]\n") else { return (nil, text) }
        return (String(text[..<text.index(after: end.lowerBound)]), String(text[end.upperBound...]))
    }

    /// A conversation's usage: its tokens in and out and its cost.
    func conversationsUsage(_ u: Mavio_Plugin_V1_AiUsage) -> String {
        MavioStrings.conversationsUsage(
            input: u.inputTokens.formatted(), output: u.outputTokens.formatted(),
            cost: u.cost.formatted(.currency(code: "USD").precision(.significantDigits(1...3))))
    }

    /// A conversation's model as the picker lists it, else by its IDs, as
    /// for a model its provider no longer has.
    func conversationsModelOf(_ options: [PickerItem], _ c: Mavio_Plugin_V1_AgentSession) -> PickerItem {
        let value = "\(c.provider)/\(c.model)"
        return options.first { $0.value == value }
            ?? PickerItem(value: value, label: c.model, provider: c.provider, providerName: c.provider)
    }

    /// A model's name and its provider's, on one line.
    func conversationsModelName(_ m: PickerItem) -> String {
        "\(m.label) (\(m.providerName ?? m.provider ?? ""))"
    }

    // MARK: A conversation

    /// A message being answered: its text, the answer so far, and why it
    /// failed.
    private struct Exchange {
        var text: String
        var answer = ""
        var error: String?
    }

    /// A conversation's page: its messages, the one being answered with its
    /// answer as the model writes it, the field sending another, and
    /// deleting it.
    final class ConversationPage: Page {
        private let server: ServerRecord
        private let id: String
        /// The new conversation's first message, sent once.
        private let first: String?
        /// Refreshes the list of conversations.
        private let changed: @MainActor () async -> Void
        private var conversation: Mavio_System_V1_GetAiConversationResponse?
        private var failure: String?
        private var options: [PickerItem] = []
        private var exchange: Exchange?
        private var current = ""
        private var sentFirst = false
        private var removing = false
        private var removeFailure: String?
        private var tasks: [Task<Void, Never>] = []
        private var shownMessages = -1
        private var pendingAnswer: NSTextField?

        private lazy var page = ScrollPage([])
        private lazy var column = NSStackView.vertical([], spacing: MavioSpacing.s24)
        private lazy var header = note("", theme.fonts.body, theme.colors.textSecondary)
        private lazy var removeNote = note("", theme.fonts.callout, theme.colors.danger, selectable: true)
        private lazy var messages = NSStackView.vertical([], spacing: MavioSpacing.s24)
        private lazy var errorText = note("", theme.fonts.callout, theme.colors.danger, selectable: true)
        private lazy var errorBar = bar([
            errorText,
            MavioButton(MavioStrings.conversationsRetry, kind: .secondary, theme: theme) { [weak self] in
                if let text = self?.exchange?.text { self?.send(text) }
            },
        ])
        private lazy var composer = Composer(
            action: MavioStrings.conversationsSend, dictation: Dictation(model: model, server: server), theme: theme
        ) { [weak self] in self?.submit() }
        private lazy var kept: KeptSpeech = {
            let kept = KeptSpeech(model: model, server: server)
            kept.onChange = { [weak self] in
                self?.shownMessages = -1
                self?.render()
            }
            return kept
        }()
        /// The users' names, by their IDs.
        private var names: [String: String] = [:]
        /// The speech kept of the answers of an assistant conversation of
        /// the user's, by their text.
        private var speech: [String: String] = [:]
        private lazy var deleteButton = MavioButton(
            MavioStrings.conversationsDelete, kind: .destructive, theme: theme
        ) { [weak self] in
            guard let self else { return }
            confirm(
                MavioStrings.conversationsDeleteConfirm, message: MavioStrings.conversationsDeleteMessage,
                action: MavioStrings.conversationsDelete, in: self.view
            ) { [weak self] in self?.remove() }
        }
        private lazy var status = NSStackView.vertical([], spacing: 0)

        private var sending: Bool { exchange != nil && exchange?.error == nil }

        /// The conversation of id, sending first once it shows; changed
        /// refreshes the list of conversations.
        init(
            context: MacContext, server: ServerRecord, id: String, first: String?,
            changed: @escaping @MainActor () async -> Void
        ) {
            self.server = server
            self.id = id
            self.first = first
            self.changed = changed
            super.init(context: context)
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsConversations
            let views: [NSView] = [header, removeNote, messages, errorBar, composer, deleteButton, status]
            column.setViews(views)
            for v in [header, removeNote, messages, errorBar, composer, status] as [NSView] {
                v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            }
            let holder = FlippedView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(column)
            let margin = theme.isTV ? theme.layout.margin : MavioSpacing.s48
            NSLayoutConstraint.activate([
                column.topAnchor.constraint(equalTo: holder.topAnchor, constant: MavioSpacing.s32),
                column.bottomAnchor.constraint(equalTo: holder.bottomAnchor, constant: -MavioSpacing.s32),
                column.centerXAnchor.constraint(equalTo: holder.centerXAnchor),
                column.leadingAnchor.constraint(greaterThanOrEqualTo: holder.leadingAnchor, constant: margin),
                column.widthAnchor.constraint(lessThanOrEqualToConstant: 900),
            ])
            let fill = column.leadingAnchor.constraint(equalTo: holder.leadingAnchor, constant: margin)
            fill.priority = .defaultHigh
            fill.isActive = true
            page.add(holder)
            show(page)
            render()
            run { [weak self] in await self?.open() }
        }

        private func run(_ work: @escaping @MainActor () async -> Void) {
            tasks.removeAll { $0.isCancelled }
            tasks.append(Task { await work() })
        }

        private func open() async {
            let server = server
            let model = model
            async let models = model.aiModelItems(server)
            async let users = try? model.call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self)
            var request = Mavio_Assistant_V1_GetAssistantConversationRequest()
            request.id = id
            // An assistant's conversation of the user's has the speech kept
            // of its answers.
            async let assistant = try? model.call(
                server, Procedure.assistant("GetAssistantConversation"), request,
                as: Mavio_Assistant_V1_GetAssistantConversationResponse.self)
            await load()
            if let list = await users?.users {
                names = Dictionary(list.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
            }
            for m in await assistant?.messages ?? [] where !m.user && !m.speechURL.isEmpty {
                speech[m.text] = m.speechURL
            }
            shownMessages = -1
            render()
            if let first, !sentFirst {
                sentFirst = true
                send(first)
            }
            if let list = await models {
                options = list
                shownMessages = -1
                render()
            }
        }

        private func load() async {
            var request = Mavio_System_V1_GetAiConversationRequest()
            request.id = id
            do {
                conversation = try await model.call(
                    server, Procedure.aiConversations("GetAiConversation"), request,
                    as: Mavio_System_V1_GetAiConversationResponse.self)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            shownMessages = -1
            render()
        }

        // MARK: Drawing

        private func render() {
            let loaded = conversation?.hasConversation == true
            for v in [header, messages, composer, deleteButton] as [NSView] { v.isHidden = !loaded }
            if let conversation, loaded {
                let c = conversation.conversation
                title = conversationsTitle(c)
                let who = conversationsModelOf(options, c)
                header.stringValue = "\(conversationsModelName(who)) · \(conversationsUsage(c.usage))"
                removeNote.stringValue = removeFailure ?? ""
                removeNote.isHidden = removeFailure == nil
                renderMessages(conversation, who: who)
                errorText.stringValue = exchange?.error ?? ""
                errorBar.isHidden = exchange?.error == nil
                composer.label = MavioStrings.conversationsPlaceholder(model: who.label)
                composer.busy = sending
                deleteButton.isEnabled = !removing
                status.setViews([])
            } else {
                removeNote.isHidden = true
                errorBar.isHidden = true
                if let failure {
                    status.setViews([note(failure, theme.fonts.callout, theme.colors.danger, selectable: true)])
                } else if conversation != nil {
                    status.setViews([
                        note(MavioStrings.conversationsGone, theme.fonts.callout, theme.colors.textSecondary)
                    ])
                } else {
                    status.setViews([LoadingView()])
                }
            }
            FocusEngine.active(for: view)?.schedule()
        }

        private func renderMessages(_ conversation: Mavio_System_V1_GetAiConversationResponse, who: PickerItem) {
            // The messages are drawn anew as they change; the answer being
            // written changes its text alone.
            let count = conversation.messages.count + (exchange.map { $0.error == nil ? 2 : 0 } ?? 0)
            if count == shownMessages, let pendingAnswer, let exchange {
                pendingAnswer.stringValue = exchange.answer.isEmpty ? "…" : exchange.answer
                scrollToEnd()
                return
            }
            shownMessages = count
            let c = conversation.conversation
            // The user's messages are the caller's, or those of the user
            // whose conversation it is: another's assistant conversation,
            // or one another administrator started.
            let asker =
                c.owner.isEmpty || c.owner == server.userId
                ? MavioStrings.conversationsYou : names[c.owner] ?? MavioStrings.conversationsUser
            let assistant = conversationsIsAssistant(c)
            var views: [NSView] = conversation.messages.enumerated().map { i, m in
                let user = m.role == .user
                let parts = conversationsParted(user && assistant, m.text)
                let key = String(i)
                let url = user ? nil : speech[m.text]
                return Bubble(
                    who: user ? nil : who, asker: asker, context: parts.context, text: parts.text, at: m.time.time,
                    playing: kept.at == key,
                    onPlay: url.map { u in { [weak self] in self?.kept.toggle(key, url: u) } }, theme: theme)
            }
            pendingAnswer = nil
            // A failed message shows as the conversation kept it.
            if let exchange, exchange.error == nil {
                views.append(Bubble(who: nil, asker: asker, text: exchange.text, theme: theme))
                let answer = Bubble(who: who, text: exchange.answer, pending: true, theme: theme)
                pendingAnswer = answer.label
                views.append(answer)
            }
            messages.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: messages.widthAnchor).isActive = true }
            scrollToEnd()
        }

        private func scrollToEnd() {
            view.layoutSubtreeIfNeeded()
            guard let document = page.scroll.documentView else { return }
            let y = max(0, document.frame.height - page.scroll.contentView.bounds.height)
            page.scroll.contentView.setBoundsOrigin(NSPoint(x: 0, y: y))
            page.scroll.reflectScrolledClipView(page.scroll.contentView)
        }

        // MARK: Sending

        private func submit() {
            let message = composer.text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !message.isEmpty, !sending else { return }
            composer.text = ""
            send(message)
        }

        /// Sends a message, showing it and its answer as the model writes
        /// it; the conversation is read again with both once answered, or
        /// with what was kept of them once failed, the failure kept until it
        /// is sent again.
        private func send(_ message: String) {
            let requestId = UUID().uuidString
            current = requestId
            exchange = Exchange(text: message)
            render()
            var request = Mavio_System_V1_SendAiMessageRequest()
            request.conversationID = id
            request.text = message
            request.requestID = requestId
            run { [weak self] in
                guard let self else { return }
                let stream = self.model.client.callStream(
                    self.server.id, Procedure.aiConversations("SendAiMessage"), request,
                    as: Mavio_System_V1_SendAiMessageResponse.self)
                do {
                    for try await response in stream {
                        if case .text(let so) = response.event, self.current == requestId {
                            self.exchange?.answer = so
                            self.render()
                        }
                    }
                    await self.load()
                    await self.changed()
                    if self.current == requestId { self.exchange = nil }
                    self.shownMessages = -1
                    self.render()
                } catch is CancellationError {
                } catch {
                    await self.load()
                    if self.current == requestId { self.exchange = Exchange(text: message, error: error.reason) }
                    self.shownMessages = -1
                    self.render()
                }
            }
        }

        private func remove() {
            removing = true
            removeFailure = nil
            render()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_DeleteAiConversationRequest()
                request.id = self.id
                do {
                    _ = try await self.model.call(
                        self.server, Procedure.aiConversations("DeleteAiConversation"), request,
                        as: Mavio_System_V1_DeleteAiConversationResponse.self)
                    await self.changed()
                    self.navigation?.pop()
                } catch {
                    self.removing = false
                    self.removeFailure = error.reason
                    self.render()
                }
            }
        }
    }

    // MARK: Messages

    /// A message: the user's on the right, labeled by who asked, the
    /// model's on the left under the model as the model picker's rows show
    /// it, in a dashed border; a user's message to the assistant headed by
    /// its context line, in small monospaced text over a divider; each short
    /// of the other side, so that the two stand apart; under it, a button
    /// copying it, one playing an answer's kept speech, and its time.
    final class Bubble: NSView {
        /// The message's text.
        let label: NSTextField

        /// A message of who, nil for the user's, which asker asked, sent at
        /// a time; pending while it is being written; onPlay plays its kept
        /// speech, playing while it does.
        init(
            who: PickerItem?, asker: String = MavioStrings.conversationsYou, context: String? = nil, text: String,
            pending: Bool = false, at: Date? = nil, playing: Bool = false, onPlay: (@MainActor () -> Void)? = nil,
            theme: Theme
        ) {
            let user = who == nil
            label = NSTextField.label(
                text.isEmpty && pending ? "…" : text, theme.fonts.callout, color: theme.colors.textPrimary, lines: 0,
                selectable: true)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let heading: NSView
            if let who {
                heading = pickerLabel(who, style: theme.fonts.callout, theme: theme)
            } else {
                heading = NSTextField.label(asker, theme.fonts.caption, color: theme.colors.textTertiary)
            }
            let headingHolder = NSView()
            headingHolder.translatesAutoresizingMaskIntoConstraints = false
            headingHolder.addFilling(heading, insets: .symmetric(horizontal: MavioSpacing.s4))
            let box = DashedBox(dashed: !user, theme: theme)
            var content: NSView = label
            if let context {
                let line = NSTextField.label(
                    context, theme.fonts.caption, color: theme.colors.textTertiary, lines: 0, selectable: true)
                line.font = .monospacedSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
                // As wide as the line on one, within the box, rather than
                // only as wide as a short message under it.
                let wide = line.widthAnchor.constraint(
                    greaterThanOrEqualToConstant: min(ceil(line.attributedStringValue.size().width) + 4, 672 - 24))
                wide.priority = .defaultHigh
                wide.isActive = true
                let divider = NSBox()
                divider.boxType = .custom
                divider.borderWidth = 0
                divider.fillColor = theme.colors.border
                divider.translatesAutoresizingMaskIntoConstraints = false
                divider.heightAnchor.constraint(equalToConstant: 1).isActive = true
                let stack = NSStackView.vertical([line, divider, label], spacing: 6, alignment: .leading)
                for v in [line, divider, label] { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                content = stack
            }
            box.addFilling(content, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s8))
            box.widthAnchor.constraint(lessThanOrEqualToConstant: 672).isActive = true
            var parts: [NSView] = [headingHolder, box]
            if !pending, !text.isEmpty {
                parts.append(messageBar(text, at: at, playing: playing, onPlay: onPlay, theme: theme))
            }
            let stack = NSStackView.vertical(parts, spacing: MavioSpacing.s4, alignment: user ? .trailing : .leading)
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.topAnchor.constraint(equalTo: topAnchor),
                stack.bottomAnchor.constraint(equalTo: bottomAnchor),
                user
                    ? stack.trailingAnchor.constraint(equalTo: trailingAnchor)
                    : stack.leadingAnchor.constraint(equalTo: leadingAnchor),
                user
                    ? stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 80)
                    : stack.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -80),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// A button copying a message and, given onPlay, one playing its kept
    /// speech, then its time.
    @MainActor func messageBar(
        _ text: String, at: Date?, playing: Bool = false, onPlay: (@MainActor () -> Void)? = nil, theme: Theme
    ) -> NSView {
        var views: [NSView] = [
            copyButton(
                text, label: MavioStrings.assistantCopyMessage, copied: MavioStrings.assistantCopiedMessage,
                theme: theme)
        ]
        if let onPlay {
            let label = playing ? MavioStrings.assistantStopAnswer : MavioStrings.assistantPlayAnswer
            let play = MavioButton(icon: playing ? "pause" : "play", kind: .plain, theme: theme, action: onPlay)
            play.setAccessibilityLabel(label)
            if !theme.isTV { play.toolTip = label }
            views.append(play)
        }
        if let at {
            let time = NSTextField.label(shortTime(at), theme.fonts.caption, color: theme.colors.textTertiary)
            if !theme.isTV { time.toolTip = at.formatted(date: .abbreviated, time: .shortened) }
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(time, insets: .symmetric(horizontal: MavioSpacing.s4))
            views.append(holder)
        }
        return NSStackView.horizontal(views, spacing: 2)
    }

    /// A message's box: the surface, its corners rounded, dashed in the
    /// accent's color at half its strength when dashed.
    final class DashedBox: NSView {
        private let dashed: Bool
        private let theme: Theme

        /// A box, dashed or not.
        init(dashed: Bool, theme: Theme) {
            self.dashed = dashed
            self.theme = theme
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func draw(_ dirtyRect: NSRect) {
            let path = NSBezierPath(roundedRect: bounds.insetBy(dx: 0.5, dy: 0.5), xRadius: 12, yRadius: 12)
            theme.colors.surface.setFill()
            path.fill()
            guard dashed else { return }
            theme.colors.accent.withAlphaComponent(0.5).setStroke()
            path.lineWidth = 1
            path.setLineDash([4, 3], count: 2, phase: 0)
            path.stroke()
        }
    }

    // MARK: The composer

    /// A field for a message, its buttons in a column of their own at its
    /// end, at the bottom: dictation over sending; Return sends,
    /// Shift-Return starts a new line. While it is dictated to, it is not
    /// typed in, and what is heard shows in it as it is said; why dictation
    /// could not listen shows under it until dismissed.
    final class Composer: NSView {
        private let area: TextArea
        private let send: MavioButton
        private let spinner = NSProgressIndicator()
        private let dictate: MavioButton
        private let listeningButton: Box
        private let errorBox: Box
        private let errorText: NSTextField
        private let dictation: Dictation?
        private let onSend: @MainActor () -> Void
        private var placeholder = ""
        // What was typed, which what is said is joined to.
        private var typed = ""

        /// A composer whose button is labeled action, calling onSend,
        /// dictated to through dictation.
        init(
            action: String, dictation: Dictation? = nil, theme: Theme, onSend: @escaping @MainActor () -> Void
        ) {
            self.onSend = onSend
            self.dictation = dictation
            area = TextArea(
                font: theme.fonts.callout.font, minHeight: 48, maxHeight: 240, fill: .clear, bordered: false,
                theme: theme)
            send = MavioButton(icon: "send", kind: .primary, theme: theme)
            dictate = MavioButton(icon: "microphone", kind: .plain, theme: theme)
            listeningButton = Box(fill: theme.colors.danger, radius: 14)
            errorBox = Box(fill: theme.colors.danger.withAlphaComponent(0.1), radius: 8)
            errorText = note("", theme.fonts.footnote, theme.colors.danger, selectable: true)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let field = Box(fill: theme.colors.surface, radius: 8)
            field.border = theme.colors.border
            send.setAccessibilityLabel(action)
            if !theme.isTV { send.toolTip = action }
            dictate.setAccessibilityLabel(MavioStrings.conversationsDictate)
            if !theme.isTV { dictate.toolTip = MavioStrings.conversationsDictate }
            dictate.isHidden = dictation == nil
            let bars = VoiceBars(color: .white)
            listeningButton.addSubview(bars)
            bars.center(in: listeningButton)
            listeningButton.size(width: 28, height: 28)
            listeningButton.isHidden = true
            listeningButton.setAccessibilityElement(true)
            listeningButton.setAccessibilityRole(.button)
            listeningButton.setAccessibilityLabel(MavioStrings.assistantStopListening)
            if !theme.isTV { listeningButton.toolTip = MavioStrings.assistantStopListening }
            listeningButton.addGestureRecognizer(
                NSClickGestureRecognizer(target: self, action: #selector(stopListening)))
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.isHidden = true
            spinner.translatesAutoresizingMaskIntoConstraints = false
            let buttons = NSStackView.vertical(
                [dictate, listeningButton, spinner, send], spacing: 6, alignment: .centerX)
            let row = NSStackView.horizontal([area, buttons], spacing: MavioSpacing.s8, alignment: .bottom)
            field.addFilling(row, insets: NSEdgeInsets(top: 6, left: MavioSpacing.s8 + 6, bottom: 6, right: 6))
            let dismiss = MavioButton(icon: "close", kind: .plain, theme: theme) { [weak dictation] in
                dictation?.dismiss()
            }
            dismiss.setAccessibilityLabel(MavioStrings.remoteDismiss)
            errorBox.border = theme.colors.danger.withAlphaComponent(0.3)
            errorBox.addFilling(
                NSStackView.horizontal([errorText, dismiss], spacing: MavioSpacing.s8, alignment: .top),
                insets: NSEdgeInsets(top: 6, left: MavioSpacing.s12, bottom: 6, right: 6))
            errorBox.isHidden = true
            let column = NSStackView.vertical([field, errorBox], spacing: 6)
            addFilling(column)
            for v in [field, errorBox] { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            area.onReturn = { [weak self] in self?.trySend() }
            area.onChange = { [weak self] t in
                self?.typed = t
                self?.restyle()
            }
            send.onPress = { [weak self] in self?.trySend() }
            dictate.onPress = { [weak dictation] in dictation?.toggle() }
            dictation?.onText = { [weak self] said in
                guard let self else { return }
                typed = dictationJoined(typed, said)
                showText()
            }
            dictation?.onChange = { [weak self] in
                self?.showText()
                self?.restyle()
            }
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { dictation?.close() }

        /// The text.
        var text: String {
            get { typed }
            set {
                typed = newValue
                showText()
                restyle()
            }
        }

        /// The placeholder, as the model to write to.
        var label: String = "" {
            didSet {
                area.textView.setAccessibilityLabel(label)
                restyle()
            }
        }

        /// Whether a message is being sent.
        var busy = false { didSet { restyle() } }

        /// Whether nothing can be sent, as without a model.
        var disabled = false { didSet { restyle() } }

        private var listening: Bool { dictation?.listening == true }

        private var blocked: Bool {
            disabled || busy || listening || typed.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }

        @objc private func stopListening() { dictation?.toggle() }

        private func showText() {
            area.text = listening ? dictationJoined(typed, dictation?.heard ?? "") : typed
        }

        private func trySend() {
            if !blocked { onSend() }
        }

        private func restyle() {
            send.isEnabled = !blocked
            send.isHidden = busy
            spinner.isHidden = !busy
            if busy { spinner.startAnimation(nil) } else { spinner.stopAnimation(nil) }
            area.textView.isEditable = !listening
            area.placeholder = listening ? MavioStrings.assistantListening : label
            if dictation != nil {
                dictate.isHidden = listening
                dictate.isEnabled = !disabled && !busy
                listeningButton.isHidden = !listening
            }
            errorText.stringValue = dictation?.error ?? ""
            errorBox.isHidden = dictation?.error == nil
        }
    }
#endif
