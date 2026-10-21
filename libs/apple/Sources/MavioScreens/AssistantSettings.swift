import MavioCommon
import MavioKit
import MavioUI
import SwiftUI

/// The Assistant settings of This Device (docs/clients.md §6.3.29), as the
/// web UI's on a phone: the user's memories on the server chosen, each
/// forgotten from its menu of three dots, a new one remembered and all
/// forgotten after confirming; and the user's conversations, each opened
/// with its messages, each copied and an answer's kept speech played, or
/// deleted.
struct AssistantSettingsView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var prefs: AssistantPrefs?
    @State private var text = ""
    @State private var forgettingAll = false

    var body: some View {
        SettingsList(message: MavioStrings.assistantSettingsMessage) {
            if let prefs, let info = prefs.info {
                if !info.enabled {
                    Section { Text(MavioStrings.assistantOff) }
                } else {
                    memories(prefs, info)
                    if info.ai { conversations(prefs) }
                }
            } else if let failure = prefs?.failure {
                Section { Text(failure).foregroundStyle(theme.colors.danger) }
            } else {
                Section { ProgressView().frame(maxWidth: .infinity) }
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.assistantTitle)
        .inlineTitle()
        .task(id: server.id) {
            let prefs = AssistantPrefs(model: model, server: server)
            self.prefs = prefs
            await prefs.load()
        }
        .onDisappear { prefs?.kept.stop() }
    }

    @ViewBuilder
    private func memories(_ prefs: AssistantPrefs, _ info: Mavio_Assistant_V1_GetAssistantResponse) -> some View {
        if !info.memory {
            Section(MavioStrings.assistantMemory) {
                Text(info.ai ? MavioStrings.assistantMemoryOff : MavioStrings.assistantMemoryNoAi)
            }
        } else {
            let list = prefs.memories ?? []
            Section {
                ForEach(list, id: \.id) { m in
                    HStack(alignment: .top, spacing: MavioSpacing.s8) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(m.text)
                            if m.hasUpdateTime {
                                Text(m.updateTime.date.formatted(date: .abbreviated, time: .shortened))
                                    .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                            }
                        }
                        Spacer(minLength: 0)
                        more(MavioStrings.assistantForget) { Task { await prefs.forget([m.id]) } }
                    }
                }
                if prefs.memories?.isEmpty == true {
                    Text(MavioStrings.assistantNoMemories).foregroundStyle(theme.colors.textSecondary)
                }
                HStack(spacing: MavioSpacing.s8) {
                    TextField(MavioStrings.assistantNewMemory, text: $text)
                        .onSubmit(remember)
                        .submitLabel(.done)
                    Button(MavioStrings.assistantRemember, action: remember)
                        .buttonStyle(.borderless)
                        .disabled(prefs.busy || text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                    if !list.isEmpty {
                        more(MavioStrings.assistantForgetAll) { forgettingAll = true }
                    }
                }
                if let failure = prefs.memoryFailure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                }
            } header: {
                Text(MavioStrings.assistantMemory)
            } footer: {
                Text(MavioStrings.assistantMemoryFooter)
            }
            .confirmationDialog(
                MavioStrings.assistantForgetAllConfirm, isPresented: $forgettingAll, titleVisibility: .visible
            ) {
                Button(MavioStrings.assistantForgetAll, role: .destructive) { Task { await prefs.forget([]) } }
                Button(MavioStrings.actionCancel, role: .cancel) {}
            }
        }
    }

    private func remember() {
        guard let prefs, !prefs.busy else { return }
        let text = text
        Task { if await prefs.remember(text) { self.text = "" } }
    }

    /// A button of three dots opening a menu of one destructive item.
    private func more(_ title: String, run: @escaping () -> Void) -> some View {
        Menu {
            Button(title, role: .destructive, action: run)
        } label: {
            Icon("moreVertical", size: 20).foregroundStyle(theme.colors.textSecondary)
                .frame(width: 32, height: 32)
                .contentShape(.rect)
        }
        .buttonStyle(.borderless)
        .disabled(prefs?.busy == true)
        .accessibilityLabel(MavioStrings.itemMore)
    }

    private func conversations(_ prefs: AssistantPrefs) -> some View {
        Section {
            ForEach(prefs.conversations ?? [], id: \.id) { c in
                let label = c.label
                Button {
                    Task { await prefs.toggle(c.id) }
                } label: {
                    HStack {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(label.title).foregroundStyle(theme.colors.textPrimary)
                            if !label.detail.isEmpty {
                                Text(label.detail).type(theme.type.footnote)
                                    .foregroundStyle(theme.colors.textSecondary)
                            }
                        }
                        Spacer()
                    }
                }
                .accessibilityAddTraits(prefs.shown == c.id ? .isSelected : [])
                if prefs.shown == c.id { messages(prefs, c.id) }
            }
            if prefs.conversations?.isEmpty == true {
                Text(MavioStrings.assistantNoConversations).foregroundStyle(theme.colors.textSecondary)
            }
            if let failure = prefs.conversationsFailure {
                Text(failure).foregroundStyle(theme.colors.danger)
            }
        } header: {
            Text(MavioStrings.assistantConversations)
        } footer: {
            Text(MavioStrings.assistantConversationsFooter)
        }
    }

    /// A conversation's messages and deleting it.
    private func messages(_ prefs: AssistantPrefs, _ id: String) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
            if let messages = prefs.messages {
                LazyVStack(spacing: MavioSpacing.s16) {
                    ForEach(Array(messages.enumerated()), id: \.offset) { i, m in
                        AssistantBubble(message: m, playing: prefs.kept.at == String(i)) {
                            prefs.kept.toggle(String(i), url: m.speechURL)
                        }
                    }
                }
            } else if prefs.messagesFailure == nil {
                ProgressView().frame(maxWidth: .infinity)
            }
            if let failure = prefs.messagesFailure {
                Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
            }
            Button(MavioStrings.assistantDeleteConversation, role: .destructive) {
                Task { await prefs.delete(id) }
            }
            .buttonStyle(.borderless)
            .disabled(prefs.busy)
        }
        .padding(.vertical, MavioSpacing.s8)
    }
}

/// A message of the user's conversation with the assistant: the user's on
/// the right in the button's color, the answer's on the left on the fill,
/// each short of the other side; under it a button copying it, one playing
/// an answer's kept speech, and its time.
struct AssistantBubble: View {
    let message: Mavio_Assistant_V1_AssistantMessage
    let playing: Bool
    let play: () -> Void
    @Environment(\.theme) private var theme
    @State private var copied = false

    var body: some View {
        let user = message.user
        VStack(alignment: user ? .trailing : .leading, spacing: 2) {
            Text(message.text)
                .type(theme.type.callout)
                .foregroundStyle(user ? theme.colors.onButton : theme.colors.textPrimary)
                .textSelection(.enabled)
                .padding(.horizontal, MavioSpacing.s12)
                .padding(.vertical, MavioSpacing.s8)
                .background(user ? theme.colors.button : theme.colors.fill, in: .rect(cornerRadius: 14))
            HStack(spacing: 2) {
                Button {
                    copyToClipboard(message.text)
                    copied = true
                } label: {
                    Icon(copied ? "check" : "copy", size: 16).frame(width: 28, height: 28)
                }
                .accessibilityLabel(copied ? MavioStrings.assistantCopiedMessage : MavioStrings.assistantCopyMessage)
                if !message.speechURL.isEmpty {
                    Button(action: play) {
                        Icon(playing ? "pause" : "play", size: 16).frame(width: 28, height: 28)
                    }
                    .accessibilityLabel(playing ? MavioStrings.assistantStopAnswer : MavioStrings.assistantPlayAnswer)
                }
                if message.hasTime {
                    Text(message.time.date.formatted(date: .omitted, time: .shortened))
                        .type(theme.type.caption).foregroundStyle(theme.colors.textTertiary)
                        .padding(.horizontal, MavioSpacing.s4)
                }
            }
            .buttonStyle(.borderless)
            .foregroundStyle(theme.colors.textSecondary)
        }
        .frame(maxWidth: .infinity, alignment: user ? .trailing : .leading)
        .padding(user ? .leading : .trailing, 48)
        .task(id: copied) {
            guard copied else { return }
            try? await Task.sleep(for: .seconds(2))
            copied = false
        }
    }
}
