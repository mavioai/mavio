#if os(macOS)
    import AppKit
    import MavioKit
    import MavioUI
    import SwiftUI

    /// AI Conversations (docs/clients.md §6.3.26), as the web UI's: the administrators' conversations with the
    /// models of the AI providers, a new one started with a model, instructions and its first message, and a
    /// conversation's messages with the answer as the model writes it, each with a button copying it and its time.
    struct ConversationsView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var conversations: [Mavio_Plugin_V1_AgentSession]?
        @State private var failure: String?
        @State private var noPlugin = false
        @State private var options: [AiPickerOption]?
        @State private var opened: ConversationsOpened?

        // A new conversation.
        @State private var chosen = ""
        @State private var instructions = ""
        @State private var text = ""
        @State private var creating = false
        @State private var createFailure: String?

        var body: some View {
            SettingsList(message: MavioStrings.conversationsMessage) {
                if noPlugin {
                    Section {
                        NavigationLink {
                            PluginsView(server: server)
                        } label: {
                            AiPageLabel(
                                icon: "plugins", color: Color(hex: 0xAF52DE), label: MavioStrings.conversationsNoPlugin,
                                detail: MavioStrings.aiNoGatewayDetail)
                        }
                        .rowLink()
                    } footer: {
                        Text(MavioStrings.conversationsNoPluginFooter)
                    }
                } else {
                    newSection
                    listSection
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsConversations)
            .inlineTitle()
            .navigationDestination(item: $opened) { o in
                ConversationsPage(server: server, id: o.id, first: o.first, changed: load)
            }
            .pullToRefresh { await load() }
            .task { await load() }
        }

        // MARK: New

        private var model_: String { chosen.isEmpty ? options?.first?.value ?? "" : chosen }

        @ViewBuilder private var newSection: some View {
            if let options, options.isEmpty {
                Section(MavioStrings.conversationsNew) {
                    Text(MavioStrings.conversationsNoModels).foregroundStyle(theme.colors.textSecondary)
                }
            } else {
                let current = model_
                Section {
                    FormRow(label: MavioStrings.conversationsModel) {
                        AiSearchPicker(
                            label: MavioStrings.conversationsModel, options: options ?? [],
                            value: Binding(get: { current }, set: { chosen = $0 }),
                            placeholder: MavioStrings.conversationsModel)
                    }
                    VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                        ConversationsField(
                            text: $instructions,
                            placeholder:
                                "\(MavioStrings.conversationsInstructions) (\(MavioStrings.conversationsInstructionsPlaceholder))",
                            limit: 20000)
                        ConversationsComposer(
                            text: $text,
                            label: MavioStrings.conversationsPlaceholder(
                                model: options?.first { $0.value == current }?.label ?? ""),
                            action: MavioStrings.conversationsStart, busy: creating, disabled: current.isEmpty,
                            onSend: start)
                        if let createFailure {
                            Text(createFailure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                        }
                    }
                    .padding(.vertical, MavioSpacing.s4)
                } header: {
                    Text(MavioStrings.conversationsNew)
                } footer: {
                    Text(MavioStrings.conversationsNewFooter)
                }
            }
        }

        private func start() {
            let chosen = model_
            let message = text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard let slash = chosen.firstIndex(of: "/"), !message.isEmpty, !creating else { return }
            creating = true
            createFailure = nil
            Task {
                defer { creating = false }
                var request = Mavio_System_V1_CreateAiConversationRequest()
                request.provider = String(chosen[..<slash])
                request.model = String(chosen[chosen.index(after: slash)...])
                request.instructions = instructions.trimmingCharacters(in: .whitespacesAndNewlines)
                request.title = String(
                    (message.split(separator: "\n", omittingEmptySubsequences: false).first ?? "").prefix(80))
                do {
                    let response = try await model.call(
                        server, Procedure.aiConversations("CreateAiConversation"), request,
                        as: Mavio_System_V1_CreateAiConversationResponse.self)
                    text = ""
                    instructions = ""
                    opened = ConversationsOpened(id: response.conversation.id, first: message)
                    await load()
                } catch {
                    createFailure = error.reason
                }
            }
        }

        // MARK: List

        private var listSection: some View {
            Section {
                ForEach(conversations ?? [], id: \.id) { c in
                    NavigationLink {
                        ConversationsPage(server: server, id: c.id, first: nil, changed: load)
                    } label: {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(c.title.isEmpty ? MavioStrings.conversationsUntitled : c.title).lineLimit(1)
                            Text(detail(c))
                                .type(theme.type.footnote)
                                .foregroundStyle(theme.colors.textSecondary)
                                .lineLimit(1)
                        }
                    }
                    .rowLink()
                }
                if conversations?.isEmpty == true {
                    Text(MavioStrings.conversationsNone).foregroundStyle(theme.colors.textSecondary)
                }
                if conversations == nil, failure == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                }
            } header: {
                Text(MavioStrings.conversationsList)
            }
        }

        private func detail(_ c: Mavio_Plugin_V1_AgentSession) -> String {
            [
                conversationsModelName(conversationsModelOf(options ?? [], c)),
                c.updated.time.map(fromNow),
                conversationsUsage(c.usage),
            ]
            .compactMap { $0 }.joined(separator: " · ")
        }

        private func load() async {
            async let models = model.aiModelOptions(server)
            do {
                conversations = try await model.call(
                    server, Procedure.aiConversations("ListAiConversations"),
                    Mavio_System_V1_ListAiConversationsRequest(), as: Mavio_System_V1_ListAiConversationsResponse.self
                ).conversations
                failure = nil
                noPlugin = false
            } catch is CancellationError {
            } catch {
                noPlugin = error.isNoAiGateway
                failure = error.reason
            }
            if let list = await models { options = list }
        }
    }

    /// A conversation opened from the list, with the message to send once it shows when it was just started.
    private struct ConversationsOpened: Hashable {
        let id: String
        let first: String?
    }

    /// A conversation's usage: its tokens in and out and its cost.
    private func conversationsUsage(_ u: Mavio_Plugin_V1_AiUsage) -> String {
        MavioStrings.conversationsUsage(
            input: u.inputTokens.formatted(), output: u.outputTokens.formatted(),
            cost: u.cost.formatted(.currency(code: "USD").precision(.significantDigits(1...3))))
    }

    /// A conversation's model as the picker lists it, else by its IDs, as for a model its provider no longer has.
    private func conversationsModelOf(_ options: [AiPickerOption], _ c: Mavio_Plugin_V1_AgentSession) -> AiPickerOption
    {
        let value = "\(c.provider)/\(c.model)"
        return options.first { $0.value == value }
            ?? AiPickerOption(value: value, label: c.model, provider: c.provider, providerName: c.provider)
    }

    /// A model's name and its provider's, on one line.
    private func conversationsModelName(_ m: AiPickerOption) -> String {
        "\(m.label) (\(m.providerName ?? m.provider ?? ""))"
    }

    // MARK: A conversation

    /// A message being answered: its text, the answer so far, and why it failed.
    private struct ConversationsExchange {
        var text: String
        var answer = ""
        var error: String?
    }

    /// A conversation's page: its messages, the one being answered with its answer as the model writes it, the
    /// field sending another, and deleting it.
    private struct ConversationsPage: View {
        let server: ServerRecord
        let id: String
        /// The new conversation's first message, sent once.
        let first: String?
        /// Refreshes the list of conversations.
        let changed: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var conversation: Mavio_System_V1_GetAiConversationResponse?
        @State private var failure: String?
        @State private var options: [AiPickerOption] = []
        @State private var exchange: ConversationsExchange?
        @State private var current = ""
        @State private var sentFirst = false
        @State private var text = ""
        @State private var confirming = false
        @State private var removing = false
        @State private var removeFailure: String?

        private var sending: Bool { exchange != nil && exchange?.error == nil }

        var body: some View {
            ScrollViewReader { proxy in
                ScrollView {
                    content
                        .frame(maxWidth: 900, alignment: .leading)
                        .padding(.horizontal, MavioSpacing.s48)
                        .padding(.vertical, MavioSpacing.s32)
                        .frame(maxWidth: .infinity)
                }
                .onChange(of: conversation?.messages.count) { proxy.scrollTo("end", anchor: .bottom) }
                .onChange(of: exchange?.answer) { proxy.scrollTo("end", anchor: .bottom) }
                .onChange(of: exchange?.text) { proxy.scrollTo("end", anchor: .bottom) }
            }
            .background(theme.colors.background)
            .navigationTitle(
                conversation.map {
                    $0.conversation.title.isEmpty ? MavioStrings.conversationsUntitled : $0.conversation.title
                } ?? MavioStrings.settingsConversations
            )
            .inlineTitle()
            .confirmationDialog(
                MavioStrings.conversationsDeleteConfirm, isPresented: $confirming, titleVisibility: .visible
            ) {
                Button(MavioStrings.conversationsDelete, role: .destructive, action: remove)
            } message: {
                Text(MavioStrings.conversationsDeleteMessage)
            }
            .task { await open() }
        }

        @ViewBuilder private var content: some View {
            if let conversation, conversation.hasConversation {
                let c = conversation.conversation
                let who = conversationsModelOf(options, c)
                VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                    Text("\(conversationsModelName(who)) · \(conversationsUsage(c.usage))")
                        .type(theme.type.body)
                        .foregroundStyle(theme.colors.textSecondary)
                    if let removeFailure {
                        Text(removeFailure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s24) {
                        ForEach(Array(conversation.messages.enumerated()), id: \.offset) { _, m in
                            ConversationsBubble(who: m.role == .user ? nil : who, text: m.text, at: m.time.time)
                        }
                        // A failed message shows as the conversation kept it.
                        if let exchange, exchange.error == nil {
                            ConversationsBubble(who: nil, text: exchange.text)
                            ConversationsBubble(who: who, text: exchange.answer, pending: true)
                        }
                    }
                    if let error = exchange?.error {
                        HStack(spacing: MavioSpacing.s12) {
                            Text(error).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                            Button(MavioStrings.conversationsRetry) {
                                if let text = exchange?.text { send(text) }
                            }
                            .buttonStyle(.bordered)
                        }
                        .type(theme.type.callout)
                    }
                    ConversationsComposer(
                        text: $text, label: MavioStrings.conversationsPlaceholder(model: who.label),
                        action: MavioStrings.conversationsSend, busy: sending, onSend: submit
                    )
                    .id("end")
                    Button(MavioStrings.conversationsDelete, role: .destructive) { confirming = true }
                        .buttonStyle(.bordered)
                        .disabled(removing)
                }
            } else if let failure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger).textSelection(.enabled)
            } else if conversation != nil {
                Text(MavioStrings.conversationsGone).type(theme.type.callout).foregroundStyle(
                    theme.colors.textSecondary)
            } else {
                ProgressView().frame(maxWidth: .infinity)
            }
        }

        private func open() async {
            async let models = model.aiModelOptions(server)
            await load()
            if let first, !sentFirst {
                sentFirst = true
                send(first)
            }
            options = await models ?? options
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
            } catch {
                failure = error.reason
            }
        }

        private func submit() {
            let message = text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !message.isEmpty, !sending else { return }
            text = ""
            send(message)
        }

        /// Sends a message, showing it and its answer as the model writes it; the conversation is read again with
        /// both once answered, or with what was kept of them once failed, the failure kept until it is sent again.
        private func send(_ message: String) {
            let requestId = UUID().uuidString
            current = requestId
            exchange = ConversationsExchange(text: message)
            Task {
                do {
                    try await conversationsSend(model, server, id, message, requestId) { answer in
                        if current == requestId { exchange?.answer = answer }
                    }
                    await load()
                    await changed()
                    if current == requestId { exchange = nil }
                } catch {
                    await load()
                    if current == requestId { exchange = ConversationsExchange(text: message, error: error.reason) }
                }
            }
        }

        private func remove() {
            removing = true
            removeFailure = nil
            Task {
                defer { removing = false }
                var request = Mavio_System_V1_DeleteAiConversationRequest()
                request.id = id
                do {
                    _ = try await model.call(
                        server, Procedure.aiConversations("DeleteAiConversation"), request,
                        as: Mavio_System_V1_DeleteAiConversationResponse.self)
                    await changed()
                    dismiss()
                } catch {
                    removeFailure = error.reason
                }
            }
        }
    }

    /// Sends a message to a conversation, giving the answer so far as the model writes it, until it is answered.
    @MainActor
    private func conversationsSend(
        _ model: AppModel, _ server: ServerRecord, _ id: String, _ text: String, _ requestId: String,
        _ answer: @MainActor (String) -> Void
    ) async throws {
        var request = Mavio_System_V1_SendAiMessageRequest()
        request.conversationID = id
        request.text = text
        request.requestID = requestId
        let stream = model.client.callStream(
            server.id, Procedure.aiConversations("SendAiMessage"), request,
            as: Mavio_System_V1_SendAiMessageResponse.self)
        for try await response in stream {
            if case .text(let so) = response.event { answer(so) }
        }
    }

    // MARK: Messages

    /// A message: the user's on the right, the model's on the left under the model as the model picker's rows
    /// show it, in a dashed border; each short of the other side, so that the two stand apart; under it, a button
    /// copying it and its time.
    private struct ConversationsBubble: View {
        let who: AiPickerOption?
        let text: String
        var pending = false
        var at: Date?
        @Environment(\.theme) private var theme

        var body: some View {
            let user = who == nil
            HStack(spacing: 0) {
                if user { Spacer(minLength: 80) }
                VStack(alignment: user ? .trailing : .leading, spacing: MavioSpacing.s4) {
                    if let who {
                        AiOptionLabel(option: who).type(theme.type.callout).padding(.horizontal, MavioSpacing.s4)
                    } else {
                        Text(MavioStrings.conversationsYou)
                            .type(theme.type.caption)
                            .foregroundStyle(theme.colors.textTertiary)
                            .padding(.horizontal, MavioSpacing.s4)
                    }
                    Text(text.isEmpty && pending ? "…" : text)
                        .type(theme.type.callout)
                        .textSelection(.enabled)
                        .fixedSize(horizontal: false, vertical: true)
                        .padding(.horizontal, MavioSpacing.s12)
                        .padding(.vertical, MavioSpacing.s8)
                        .background(theme.colors.surface, in: .rect(cornerRadius: 12))
                        .overlay {
                            if !user {
                                RoundedRectangle(cornerRadius: 12)
                                    .strokeBorder(theme.colors.accent.opacity(0.5), style: StrokeStyle(dash: [4, 3]))
                            }
                        }
                        .frame(maxWidth: 672, alignment: user ? .trailing : .leading)
                    if !pending, !text.isEmpty {
                        ConversationsMessageBar(text: text, at: at)
                    }
                }
                if !user { Spacer(minLength: 80) }
            }
        }
    }

    /// A button copying a message, then its time.
    private struct ConversationsMessageBar: View {
        let text: String
        let at: Date?
        @Environment(\.theme) private var theme
        @State private var copied = false

        var body: some View {
            HStack(spacing: 2) {
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(text, forType: .string)
                    copied = true
                    Task {
                        try? await Task.sleep(for: .seconds(2))
                        copied = false
                    }
                } label: {
                    Icon(copied ? "check" : "copy", size: 15)
                        .foregroundStyle(theme.colors.textTertiary)
                        .frame(width: 28, height: 28)
                        .contentShape(.rect)
                }
                .buttonStyle(.borderless)
                .help(copied ? MavioStrings.assistantCopiedMessage : MavioStrings.assistantCopyMessage)
                .accessibilityLabel(copied ? MavioStrings.assistantCopiedMessage : MavioStrings.assistantCopyMessage)
                if let at {
                    Text(
                        Calendar.current.isDateInToday(at)
                            ? at.formatted(date: .omitted, time: .shortened)
                            : at.formatted(date: .abbreviated, time: .shortened)
                    )
                    .type(theme.type.caption)
                    .foregroundStyle(theme.colors.textTertiary)
                    .padding(.horizontal, MavioSpacing.s4)
                    .help(at.formatted(date: .abbreviated, time: .shortened))
                }
            }
        }
    }

    // MARK: Fields

    /// A field of several lines with its placeholder, growing with its text up to a height, then scrolling.
    private struct ConversationsField: View {
        @Binding var text: String
        let placeholder: String
        var limit = 100_000
        @Environment(\.theme) private var theme

        var body: some View {
            TextEditor(text: Binding(get: { text }, set: { text = String($0.prefix(limit)) }))
                .type(theme.type.callout)
                .scrollContentBackground(.hidden)
                .frame(minHeight: 48, maxHeight: 160)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.horizontal, MavioSpacing.s8)
                .padding(.vertical, 6)
                .overlay(alignment: .topLeading) {
                    if text.isEmpty {
                        Text(placeholder)
                            .type(theme.type.callout)
                            .foregroundStyle(theme.colors.textTertiary)
                            .padding(.horizontal, MavioSpacing.s12)
                            .padding(.vertical, 6)
                            .allowsHitTesting(false)
                    }
                }
                .background(theme.colors.background, in: .rect(cornerRadius: 8))
                .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(theme.colors.border))
                .accessibilityLabel(placeholder)
        }
    }

    /// A field for a message, its sending button at its end, at the bottom; Return sends, Shift-Return starts a
    /// new line.
    private struct ConversationsComposer: View {
        @Binding var text: String
        let label: String
        let action: String
        let busy: Bool
        var disabled = false
        let onSend: () -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            let blocked = disabled || busy || text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            HStack(alignment: .bottom, spacing: MavioSpacing.s8) {
                TextEditor(text: Binding(get: { text }, set: { text = String($0.prefix(100_000)) }))
                    .type(theme.type.callout)
                    .scrollContentBackground(.hidden)
                    .frame(minHeight: 48, maxHeight: 240)
                    .fixedSize(horizontal: false, vertical: true)
                    .overlay(alignment: .topLeading) {
                        if text.isEmpty {
                            Text(label)
                                .type(theme.type.callout)
                                .foregroundStyle(theme.colors.textTertiary)
                                .padding(.leading, 5)
                                .allowsHitTesting(false)
                        }
                    }
                    .onKeyPress(.return, phases: .down) { press in
                        guard !press.modifiers.contains(.shift) else { return .ignored }
                        if !blocked { onSend() }
                        return .handled
                    }
                    .accessibilityLabel(label)
                Button(action: onSend) {
                    Group {
                        if busy {
                            ProgressView().controlSize(.small).tint(theme.colors.onButton)
                        } else {
                            Icon("send", size: 16).foregroundStyle(theme.colors.onButton)
                        }
                    }
                    .frame(width: 28, height: 28)
                    .background(theme.colors.button, in: .circle)
                    .opacity(blocked ? 0.4 : 1)
                }
                .buttonStyle(.plain)
                .disabled(blocked)
                .help(action)
                .accessibilityLabel(action)
            }
            .padding(.leading, MavioSpacing.s8)
            .padding(6)
            .background(theme.colors.surface, in: .rect(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(theme.colors.border))
        }
    }
#endif
