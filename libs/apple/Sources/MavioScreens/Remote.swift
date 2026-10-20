import MavioAdmin
import MavioKit
import MavioUI
import SwiftUI

/// Controls another of the user's devices, as the web UI's remote: what it
/// plays with its position, the playback's buttons, the volume, and a
/// message to show on it.
public struct RemoteView: View {
    let server: ServerRecord
    let sessionId: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var device: DeviceSession?
    @State private var failure: String?
    @State private var dragged: TimeInterval?
    @State private var volume = 100.0
    @State private var text = ""
    @State private var sent = false

    /// How often what the device plays is read again.
    private static let refresh = Duration.seconds(2)

    /// The remote of a device's session on a server.
    public init(server: ServerRecord, sessionId: String) { (self.server, self.sessionId) = (server, sessionId) }

    /// The view's content.
    public var body: some View {
        NavigationStack {
            List {
                if let device {
                    if !device.online {
                        Text(MavioStrings.remoteOffline).foregroundStyle(theme.colors.textSecondary)
                    } else if let playing = device.playing {
                        Section { nowPlaying(playing) }
                    } else {
                        Text(MavioStrings.remoteIdle).foregroundStyle(theme.colors.textSecondary)
                    }
                    if device.online {
                        Section(MavioStrings.remoteVolume) { volumeRow }
                        Section(MavioStrings.remoteMessage) { messageRow }
                    }
                }
                if let failure {
                    Text(failure).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                }
            }
            .overlay { if device == nil && failure == nil { ProgressView() } }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(device.map(\.shownName) ?? MavioStrings.remoteTitle)
            .inlineTitle()
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(MavioStrings.actionDone) { dismiss() }
                }
            }
        }
        .tint(theme.colors.accent)
        .task {
            while !Task.isCancelled {
                await load()
                try? await Task.sleep(for: Self.refresh)
            }
        }
    }

    private func nowPlaying(_ p: RemotePlayback) -> some View {
        let runtime = p.card.runtime ?? 0
        let position = min(dragged ?? p.position, runtime > 0 ? runtime : p.position)
        return VStack(spacing: MavioSpacing.s16) {
            HStack(spacing: MavioSpacing.s12) {
                ArtworkView((p.card.thumb ?? p.card.backdrop ?? p.card.poster)?.url, icon: p.card.kind.icon)
                    .frame(width: 96, height: 54)
                    .clipShape(.rect(cornerRadius: 6))
                VStack(alignment: .leading, spacing: 2) {
                    Text(p.card.title).type(theme.type.headline).lineLimit(1)
                    if let subtitle = p.card.subtitle {
                        Text(subtitle)
                            .type(theme.type.footnote)
                            .foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
            }
            if runtime > 0 {
                VStack(spacing: MavioSpacing.s4) {
                    Slider(
                        value: Binding(get: { position }, set: { dragged = $0 }),
                        in: 0...runtime,
                        step: 1
                    ) { editing in
                        // The position goes once the slider is let go.
                        if !editing, let target = dragged {
                            send(.seek(position: target))
                            dragged = nil
                        }
                    }
                    .accessibilityLabel(MavioStrings.remotePosition)
                    HStack {
                        Text(Self.clock(position))
                        Spacer()
                        Text(Self.clock(runtime))
                    }
                    .type(theme.type.caption)
                    .monospacedDigit()
                    .foregroundStyle(theme.colors.textSecondary)
                }
            }
            HStack(spacing: MavioSpacing.s8) {
                control("previous", MavioStrings.remotePrevious) { send(.previous) }
                control("seekBack", MavioStrings.playerSeekBack(count: 10)) {
                    send(.seek(position: max(0, p.position - 10)))
                }
                control(
                    p.paused ? "play" : "pause", p.paused ? MavioStrings.playerPlay : MavioStrings.playerPause,
                    large: true
                ) { send(p.paused ? .unpause : .pause) }
                control("seekForward", MavioStrings.playerSeekForward(count: 10)) {
                    send(.seek(position: p.position + 10))
                }
                control("next", MavioStrings.remoteNext) { send(.next) }
            }
            Button(MavioStrings.remoteStop, role: .destructive) { send(.stop) }
                .buttonStyle(.borderless)
        }
        .padding(.vertical, MavioSpacing.s8)
    }

    private var volumeRow: some View {
        HStack(spacing: MavioSpacing.s12) {
            control("mute", MavioStrings.playerMute) { send(.volume(level: nil, muted: true)) }
            Slider(value: $volume, in: 0...100, step: 1) { editing in
                if !editing { send(.volume(level: UInt32(volume), muted: false)) }
            }
            .accessibilityLabel(MavioStrings.remoteVolume)
            Text("\(Int(volume))")
                .type(theme.type.footnote)
                .monospacedDigit()
                .foregroundStyle(theme.colors.textSecondary)
                .frame(width: 32, alignment: .trailing)
        }
    }

    private var messageRow: some View {
        HStack {
            TextField(MavioStrings.remoteMessage, text: $text)
                .onChange(of: text) { _, new in
                    sent = false
                    if new.count > 1000 { text = String(new.prefix(1000)) }
                }
                .onSubmit(sendMessage)
                .submitLabel(.send)
            Button(sent ? MavioStrings.remoteSent : MavioStrings.remoteSend, action: sendMessage)
                .buttonStyle(.borderless)
                .disabled(text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
    }

    private func control(_ icon: String, _ label: String, large: Bool = false, action: @escaping () -> Void)
        -> some View
    {
        Button(action: action) {
            Icon(icon, fill: large, size: large ? 28 : 22)
                .frame(width: large ? 56 : 44, height: large ? 56 : 44)
                .background(large ? theme.colors.fill : .clear, in: .circle)
                .contentShape(.circle)
        }
        .buttonStyle(.borderless)
        .tint(theme.colors.textPrimary)
        .accessibilityLabel(label)
        .frame(maxWidth: .infinity)
    }

    private func sendMessage() {
        let message = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !message.isEmpty else { return }
        Task {
            if await run(.message(text: message)) {
                text = ""
                sent = true
            }
        }
    }

    private func send(_ command: RemoteCommand) {
        Task {
            if await run(command) { await load() }
        }
    }

    @discardableResult private func run(_ command: RemoteCommand) async -> Bool {
        do {
            try await model.client.sendCommand(serverId: server.id, sessionId: sessionId, command: command)
            failure = nil
            return true
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
            return false
        }
    }

    private func load() async {
        do {
            let devices = try await model.client.devices(serverId: server.id)
            device = devices.first { $0.id == sessionId }
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    private static func clock(_ seconds: TimeInterval) -> String {
        let s = Int(max(0, seconds))
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }
}
