import MavioCommon
import MavioKit
import MavioUI
import SwiftUI

/// The Assistant settings of This Device (docs/clients.md §6.3.29), as the
/// web UI's TV mode: the user's memories on the server chosen, each
/// forgotten from the menu its row opens, a new one remembered and all
/// forgotten after confirming; and the user's conversations, each opened
/// with its messages, an answer's kept speech played as it is selected, or
/// deleted. Every message takes focus, so that the remote scrolls through
/// them.
struct AssistantSettingsView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var prefs: AssistantPrefs?
    @State private var text = ""
    @State private var forgetting: Mavio_Assistant_V1_AssistantMemory?
    @State private var forgettingAll = false

    var body: some View {
        SettingsPage(MavioStrings.assistantTitle) {
            Note(MavioStrings.assistantSettingsMessage)
            if let prefs, let info = prefs.info {
                if !info.enabled {
                    row(MavioStrings.assistantOff)
                } else {
                    memories(prefs, info)
                    if info.ai { conversations(prefs) }
                }
            } else if let failure = prefs?.failure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            } else {
                ProgressView().frame(maxWidth: .infinity)
            }
        }
        .task(id: server.id) {
            let prefs = AssistantPrefs(model: model, server: server)
            self.prefs = prefs
            await prefs.load()
        }
        .onDisappear { prefs?.kept.stop() }
    }

    /// A row that only says something, focusable so that the remote reaches
    /// it.
    private func row(_ text: String) -> some View {
        Button {
        } label: {
            Text(text)
        }
        .buttonStyle(RowButtonStyle())
    }

    @ViewBuilder
    private func memories(_ prefs: AssistantPrefs, _ info: Mavio_Assistant_V1_GetAssistantResponse) -> some View {
        Heading(MavioStrings.assistantMemory)
        if !info.memory {
            row(info.ai ? MavioStrings.assistantMemoryOff : MavioStrings.assistantMemoryNoAi)
        } else {
            let list = prefs.memories ?? []
            ForEach(list, id: \.id) { m in
                Button {
                    forgetting = m
                } label: {
                    VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                        Text(m.text).fixedSize(horizontal: false, vertical: true)
                        if m.hasUpdateTime {
                            Text(m.updateTime.date.formatted(date: .abbreviated, time: .shortened))
                                .type(theme.type.callout).opacity(0.7)
                        }
                    }
                    .padding(.vertical, MavioSpacing.s12)
                }
                .buttonStyle(RowButtonStyle())
                .disabled(prefs.busy)
            }
            if prefs.memories?.isEmpty == true { row(MavioStrings.assistantNoMemories) }
            HStack(spacing: MavioSpacing.s16) {
                TextField(MavioStrings.assistantNewMemory, text: $text)
                    .onSubmit(remember)
                Button(MavioStrings.assistantRemember, action: remember)
                    .buttonStyle(TVButtonStyle())
                    .disabled(prefs.busy || text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
            if !list.isEmpty {
                Button {
                    forgettingAll = true
                } label: {
                    Text(MavioStrings.assistantForgetAll).foregroundStyle(theme.colors.danger)
                }
                .buttonStyle(TVButtonStyle())
                .disabled(prefs.busy)
            }
            if let failure = prefs.memoryFailure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            }
            Note(MavioStrings.assistantMemoryFooter)
                .confirmationDialog(
                    forgetting?.text ?? "", isPresented: forgettingShown, titleVisibility: .visible,
                    presenting: forgetting
                ) { m in
                    Button(MavioStrings.assistantForget, role: .destructive) { Task { await prefs.forget([m.id]) } }
                    Button(MavioStrings.actionCancel, role: .cancel) {}
                }
                .confirmationDialog(
                    MavioStrings.assistantForgetAllConfirm, isPresented: $forgettingAll, titleVisibility: .visible
                ) {
                    Button(MavioStrings.assistantForgetAll, role: .destructive) { Task { await prefs.forget([]) } }
                    Button(MavioStrings.actionCancel, role: .cancel) {}
                }
        }
    }

    private var forgettingShown: Binding<Bool> {
        Binding {
            forgetting != nil
        } set: {
            if !$0 { forgetting = nil }
        }
    }

    private func remember() {
        guard let prefs, !prefs.busy else { return }
        let text = text
        Task { if await prefs.remember(text) { self.text = "" } }
    }

    @ViewBuilder
    private func conversations(_ prefs: AssistantPrefs) -> some View {
        Heading(MavioStrings.assistantConversations)
        ForEach(prefs.conversations ?? [], id: \.id) { c in
            let label = c.label
            Button {
                Task { await prefs.toggle(c.id) }
            } label: {
                HStack(spacing: MavioSpacing.s16) {
                    VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                        Text(label.title)
                        if !label.detail.isEmpty { Text(label.detail).type(theme.type.callout).opacity(0.7) }
                    }
                    Spacer()
                    Icon(prefs.shown == c.id ? "expand" : "forward", size: 28).opacity(0.6)
                }
                .padding(.vertical, MavioSpacing.s12)
            }
            .buttonStyle(RowButtonStyle())
            if prefs.shown == c.id { messages(prefs, c.id) }
        }
        if prefs.conversations?.isEmpty == true { row(MavioStrings.assistantNoConversations) }
        if let failure = prefs.conversationsFailure {
            Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
        }
        Note(MavioStrings.assistantConversationsFooter)
    }

    /// A conversation's messages and deleting it.
    private func messages(_ prefs: AssistantPrefs, _ id: String) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s24) {
            if let messages = prefs.messages {
                LazyVStack(spacing: MavioSpacing.s24) {
                    ForEach(Array(messages.enumerated()), id: \.offset) { i, m in
                        Button {
                            if !m.speechURL.isEmpty { prefs.kept.toggle(String(i), url: m.speechURL) }
                        } label: {
                            Bubble(message: m, playing: prefs.kept.at == String(i))
                        }
                        .buttonStyle(BubbleButtonStyle())
                        .frame(maxWidth: .infinity, alignment: m.user ? .trailing : .leading)
                        .padding(m.user ? .leading : .trailing, 160)
                    }
                }
            } else if prefs.messagesFailure == nil {
                ProgressView().frame(maxWidth: .infinity)
            }
            if let failure = prefs.messagesFailure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            }
            Button {
                Task { await prefs.delete(id) }
            } label: {
                Text(MavioStrings.assistantDeleteConversation).foregroundStyle(theme.colors.danger)
            }
            .buttonStyle(TVButtonStyle())
            .disabled(prefs.busy)
        }
        .padding(.vertical, MavioSpacing.s16)
    }
}

/// A message: the user's in the button's color, the answer's on the fill;
/// under it, an answer's kept speech to play and the message's time.
private struct Bubble: View {
    let message: Mavio_Assistant_V1_AssistantMessage
    let playing: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        let user = message.user
        VStack(alignment: user ? .trailing : .leading, spacing: MavioSpacing.s8) {
            Text(message.text)
                .type(theme.type.body)
                .foregroundStyle(user ? theme.colors.onButton : theme.colors.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
                .multilineTextAlignment(.leading)
                .padding(.horizontal, MavioSpacing.s24)
                .padding(.vertical, MavioSpacing.s16)
                .background(user ? theme.colors.button : theme.colors.fill, in: .rect(cornerRadius: 24))
            HStack(spacing: MavioSpacing.s12) {
                if !message.speechURL.isEmpty {
                    Icon(playing ? "pause" : "play", size: 24)
                        .accessibilityLabel(
                            playing ? MavioStrings.assistantStopAnswer : MavioStrings.assistantPlayAnswer)
                }
                if message.hasTime {
                    Text(message.time.date.formatted(date: .omitted, time: .shortened)).type(theme.type.caption)
                }
            }
            .foregroundStyle(theme.colors.textTertiary)
            .padding(.horizontal, MavioSpacing.s8)
        }
    }
}

/// A message taking focus: grown with a white border.
private struct BubbleButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        BubbleFocus(label: configuration.label)
    }
}

private struct BubbleFocus<Label: View>: View {
    let label: Label
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .padding(MavioSpacing.s8)
            .overlay {
                RoundedRectangle(cornerRadius: 30).strokeBorder(.white, lineWidth: focused ? 4 : 0)
            }
            .scaleEffect(focused ? 1.02 : 1)
            .animation(focusAnimation, value: focused)
    }
}
