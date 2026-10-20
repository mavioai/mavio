import MavioKit
import MavioUI
import SwiftUI

/// Devices, as the web UI's: the user's devices signed in to the server,
/// to sign the others out.
public struct DevicesView: View {
    let server: ServerRecord

    /// The devices of the user on a server.
    public init(server: ServerRecord) { self.server = server }
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var devices: [DeviceSession]?
    @State private var failure: String?
    @State private var busy = false
    @State private var controlling: String?
    @Environment(\.remoteControl) private var remoteControl

    /// The view's content.
    public var body: some View {
        List {
            Section {
                ForEach(devices ?? [], id: \.id) { device in
                    HStack(spacing: MavioSpacing.s12) {
                        Icon("devices", size: 20)
                            .frame(width: 36, height: 36)
                            .background(theme.colors.fill, in: .rect(cornerRadius: 9))
                        VStack(alignment: .leading, spacing: 2) {
                            Text(device.shownName).lineLimit(1)
                            Text(detail(device))
                                .type(theme.type.footnote)
                                .foregroundStyle(theme.colors.textSecondary)
                                .lineLimit(1)
                        }
                        Spacer(minLength: 0)
                        if device.current {
                            Text(MavioStrings.settingsThisDevice)
                                .type(theme.type.callout)
                                .foregroundStyle(theme.colors.textSecondary)
                        } else {
                            if device.online, remoteControl != nil {
                                Button(MavioStrings.remoteControl) { controlling = device.id }
                                    .buttonStyle(.borderless)
                            }
                            Button(MavioStrings.authSignOut, role: .destructive) { signOut(device) }
                                .buttonStyle(.borderless)
                                .disabled(busy)
                        }
                    }
                }
            } header: {
                Text(MavioStrings.settingsDevicesMessage)
            } footer: {
                if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
            }
        }
        .overlay { if devices == nil && failure == nil { ProgressView() } }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsDevices)
        .inlineTitle()
        .pullToRefresh { await load() }
        .sheet(
            isPresented: Binding(get: { controlling != nil }, set: { if !$0 { controlling = nil } }),
            onDismiss: { Task { await load() } }
        ) {
            if let controlling, let remoteControl {
                remoteControl(server, controlling)
            }
        }
        .task { await load() }
    }

    private func detail(_ d: DeviceSession) -> String {
        let state: String? =
            if let playing = d.playing {
                MavioStrings.remotePlaying(name: playing.card.title)
            } else if d.online {
                MavioStrings.remoteOnline
            } else {
                d.lastSeen.map { $0.formatted(date: .abbreviated, time: .shortened) }
            }
        return [d.client].appending(state).filter { !$0.isEmpty }.joined(separator: " · ")
    }

    private func load() async {
        do {
            devices = try await model.client.devices(serverId: server.id)
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    private func signOut(_ device: DeviceSession) {
        busy = true
        Task {
            defer { busy = false }
            do {
                try await model.client.signOutDevice(serverId: server.id, sessionId: device.id)
            } catch {
                failure = (error as? CoreError)?.reason ?? error.localizedDescription
            }
            await load()
        }
    }
}

extension Array {
    /// The array with element after it, when there is one.
    fileprivate func appending(_ element: Element?) -> [Element] {
        guard let element else { return self }
        return self + [element]
    }
}

extension DeviceSession {
    /// The device's name, else its client's.
    public var shownName: String {
        deviceName.isEmpty ? (client.isEmpty ? "—" : client) : deviceName
    }
}

extension EnvironmentValues {
    /// The remote control of another device of a server's, by its session,
    /// which Devices opens for an online device; nil where the app has none.
    @Entry public var remoteControl: (@MainActor (ServerRecord, String) -> AnyView)?
}
