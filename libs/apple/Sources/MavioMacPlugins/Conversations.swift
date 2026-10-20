#if os(macOS)
    import AppKit
    import MavioMac

    /// AI Conversations (docs/clients.md §6.3.26), as the web UI's: the
    /// administrators' conversations with the models of the AI providers, a
    /// new one started with a model, instructions and its first message, and
    /// a conversation's messages with the answer as the model writes it, each
    /// with a button copying it and its time.
    final class ConversationsPage: Page {
        private let server: ServerRecord
        private lazy var form = FormPage(
            title: MavioStrings.settingsConversations, message: MavioStrings.conversationsMessage, theme: theme)
        private lazy var newSection = LiveSection(
            MavioStrings.conversationsNew, footer: MavioStrings.conversationsNewFooter, theme: theme)
        private let holder = NSStackView.vertical([], spacing: MavioSpacing.s32)
        private var conversations: [Mavio_Plugin_V1_AgentSession]?
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
        private lazy var composer = Composer(action: MavioStrings.conversationsStart, theme: theme) { [weak self] in
            self?.start()
        }
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

        /// The conversations of server.
        init(context: MacContext, server: ServerRecord) {
            self.server = server
            super.init(context: context)
        }

        isolated deinit {
            loading?.cancel()
            creatingTask?.cancel()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsConversations
            picker.setAccessibilityLabel(MavioStrings.conversationsModel)
            form.add(holder)
            show(form)
            render()
            reload()
        }

        private func reload() {
            loading?.cancel()
            loading = Task { [weak self] in await self?.load() }
        }

        /// Reads the conversations and the models to start one with.
        func load() async {
            let server = server
            let model = model
            async let models = model.aiModelItems(server)
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
                    guard let self else { return }
                    self.push(PluginsPage(context: self.context, server: self.server))
                }
                views = [FormSection(footer: MavioStrings.conversationsNoPluginFooter, rows: [row], theme: theme)]
            } else {
                renderNew()
                views = [newSection, listSection()]
            }
            holder.setViews(views)
            for v in views { v.widthAnchor.constraint(equalTo: holder.widthAnchor).isActive = true }
            FocusEngine.active(for: view)?.schedule()
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
                FormRow.link(
                    c.title.isEmpty ? MavioStrings.conversationsUntitled : c.title, subtitle: detail(c), theme: theme
                ) { [weak self] in self?.openConversation(c.id, first: nil) }
            }
            if conversations?.isEmpty == true {
                rows.append(noteRow(MavioStrings.conversationsNone, theme.colors.textSecondary, theme: theme))
            }
            if conversations == nil, failure == nil { rows.append(spinnerRow(theme: theme)) }
            if let failure { rows.append(noteRow(failure, theme.colors.danger, theme: theme)) }
            return FormSection(MavioStrings.conversationsList, rows: rows, theme: theme)
        }

        private func detail(_ c: Mavio_Plugin_V1_AgentSession) -> String {
            [
                conversationsModelName(conversationsModelOf(options ?? [], c)), c.updated.time.map(fromNow),
                conversationsUsage(c.usage),
            ]
            .compactMap { $0 }.joined(separator: " · ")
        }

        private func openConversation(_ id: String, first: String?) {
            push(
                ConversationPage(context: context, server: server, id: id, first: first) { [weak self] in
                    await self?.load()
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
                    let response = try await self.model.call(
                        self.server, Procedure.aiConversations("CreateAiConversation"), request,
                        as: Mavio_System_V1_CreateAiConversationResponse.self)
                    self.creating = false
                    self.composer.text = ""
                    self.instructions.text = ""
                    self.renderNew()
                    self.openConversation(response.conversation.id, first: message)
                    await self.load()
                } catch {
                    self.creating = false
                    self.createFailure = error.reason
                    self.renderNew()
                }
            }
        }
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
        private lazy var composer = Composer(action: MavioStrings.conversationsSend, theme: theme) { [weak self] in
            self?.submit()
        }
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
            async let models = model.aiModelItems(server)
            await load()
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
                title = c.title.isEmpty ? MavioStrings.conversationsUntitled : c.title
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
            var views: [NSView] = conversation.messages.map { m in
                Bubble(who: m.role == .user ? nil : who, text: m.text, at: m.time.time, theme: theme)
            }
            pendingAnswer = nil
            // A failed message shows as the conversation kept it.
            if let exchange, exchange.error == nil {
                views.append(Bubble(who: nil, text: exchange.text, theme: theme))
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

    /// A message: the user's on the right, the model's on the left under the
    /// model as the model picker's rows show it, in a dashed border; each
    /// short of the other side, so that the two stand apart; under it, a
    /// button copying it and its time.
    final class Bubble: NSView {
        /// The message's text.
        let label: NSTextField

        /// A message of who, nil for the user's, sent at a time; pending
        /// while it is being written.
        init(who: PickerItem?, text: String, pending: Bool = false, at: Date? = nil, theme: Theme) {
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
                heading = NSTextField.label(
                    MavioStrings.conversationsYou, theme.fonts.caption, color: theme.colors.textTertiary)
            }
            let headingHolder = NSView()
            headingHolder.translatesAutoresizingMaskIntoConstraints = false
            headingHolder.addFilling(heading, insets: .symmetric(horizontal: MavioSpacing.s4))
            let box = DashedBox(dashed: !user, theme: theme)
            box.addFilling(label, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s8))
            box.widthAnchor.constraint(lessThanOrEqualToConstant: 672).isActive = true
            var parts: [NSView] = [headingHolder, box]
            if !pending, !text.isEmpty { parts.append(messageBar(text, at: at, theme: theme)) }
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

    /// A button copying a message, then its time.
    @MainActor private func messageBar(_ text: String, at: Date?, theme: Theme) -> NSView {
        var views: [NSView] = [
            copyButton(
                text, label: MavioStrings.assistantCopyMessage, copied: MavioStrings.assistantCopiedMessage,
                theme: theme)
        ]
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

    /// A field for a message, its sending button at its end, at the bottom;
    /// Return sends, Shift-Return starts a new line.
    final class Composer: Box {
        private let area: TextArea
        private let send: MavioButton
        private let spinner = NSProgressIndicator()
        private let onSend: @MainActor () -> Void

        /// A composer whose button is labeled action, calling onSend.
        init(action: String, theme: Theme, onSend: @escaping @MainActor () -> Void) {
            self.onSend = onSend
            area = TextArea(
                font: theme.fonts.callout.font, minHeight: 48, maxHeight: 240, fill: .clear, bordered: false,
                theme: theme)
            send = MavioButton(icon: "send", kind: .primary, theme: theme)
            super.init(fill: theme.colors.surface, radius: 8)
            border = theme.colors.border
            send.setAccessibilityLabel(action)
            if !theme.isTV { send.toolTip = action }
            spinner.style = .spinning
            spinner.controlSize = .small
            spinner.isHidden = true
            spinner.translatesAutoresizingMaskIntoConstraints = false
            let row = NSStackView.horizontal([area, spinner, send], spacing: MavioSpacing.s8, alignment: .bottom)
            addFilling(row, insets: NSEdgeInsets(top: 6, left: MavioSpacing.s8 + 6, bottom: 6, right: 6))
            area.onReturn = { [weak self] in self?.trySend() }
            area.onChange = { [weak self] _ in self?.restyle() }
            send.onPress = { [weak self] in self?.trySend() }
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The text.
        var text: String {
            get { area.text }
            set {
                area.text = newValue
                restyle()
            }
        }

        /// The placeholder, as the model to write to.
        var label: String = "" {
            didSet {
                area.textView.setAccessibilityLabel(label)
                area.placeholder = label
            }
        }

        /// Whether a message is being sent.
        var busy = false { didSet { restyle() } }

        /// Whether nothing can be sent, as without a model.
        var disabled = false { didSet { restyle() } }

        private var blocked: Bool {
            disabled || busy || area.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }

        private func trySend() {
            if !blocked { onSend() }
        }

        private func restyle() {
            send.isEnabled = !blocked
            send.isHidden = busy
            spinner.isHidden = !busy
            if busy { spinner.startAnimation(nil) } else { spinner.stopAnimation(nil) }
        }
    }
#endif
