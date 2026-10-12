import MavioKit
import MavioUI
import SwiftUI

/// The profile: the servers in the user's order, which orders Home, signing
/// out, adding a server and authorizing a device with Quick Connect.
struct ProfileView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @State private var authorizing = false

    var body: some View {
        List {
            Section(MavioStrings.authServers) {
                ForEach(model.servers, id: \.id) { server in
                    ServerRow(server: server)
                        .swipeActions {
                            Button(MavioStrings.authSignOut, role: .destructive) {
                                Task { try? await model.remove(server.id) }
                            }
                        }
                        .contextMenu {
                            if !model.signedIn.contains(server.id) {
                                Button(MavioStrings.authSignIn) { router.addingServer = true }
                            }
                            Button(MavioStrings.authSignOut, role: .destructive) {
                                Task { try? await model.remove(server.id) }
                            }
                        }
                }
                .onMove { from, to in
                    var ids = model.servers.map(\.id)
                    ids.move(fromOffsets: from, toOffset: to)
                    model.order(ids)
                }
                Button(MavioStrings.authAddServer) { router.addingServer = true }
            }
            Section {
                Button(MavioStrings.authAuthorizeDevice) { authorizing = true }
                    .disabled(model.signedIn.isEmpty)
            }
            Section {
                LabeledContent(
                    "Mavio",
                    value: Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "")
                LabeledContent("Core", value: MavioKit.coreVersion)
            }
        }
        .scrollContentBackground(.hidden)
        .navigationTitle(MavioStrings.navProfile)
        .toolbar { EditButton() }
        .sheet(isPresented: $authorizing) { AuthorizeDeviceView() }
    }
}

/// A server with the user signed in to it and the state of its connection.
private struct ServerRow: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(server.name).type(theme.type.headline)
                Text(
                    model.signedIn.contains(server.id)
                        ? MavioStrings.authSignedInAs(name: server.userName) : MavioStrings.authSignedOut
                )
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.textSecondary)
                Text(server.address).type(theme.type.caption).foregroundStyle(theme.colors.textTertiary)
            }
            Spacer()
            switch model.states[server.id] {
            case .online?: Circle().fill(theme.colors.success).frame(width: 8, height: 8)
            case .offline?: Icon("offline", size: 18).foregroundStyle(theme.colors.textTertiary)
            default: EmptyView()
            }
        }
    }
}

/// Approves the Quick Connect code another device shows.
private struct AuthorizeDeviceView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var serverId = ""
    @State private var code = ""
    @State private var message: String?
    @State private var done = false

    var body: some View {
        NavigationStack {
            Form {
                if model.signedInServers.count > 1 {
                    Picker(MavioStrings.authServer, selection: $serverId) {
                        ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
                    }
                }
                TextField("000000", text: $code)
                    .keyboardType(.numberPad)
                    .font(.system(.title, design: .monospaced))
                if let message { Text(message).foregroundStyle(done ? .green : .red) }
                Button(MavioStrings.authAuthorize) {
                    Task {
                        do {
                            try await model.client.authorizeQuickConnect(serverId: serverId, code: code)
                            done = true
                            message = MavioStrings.authAuthorized
                            code = ""
                        } catch {
                            done = false
                            message = error.localizedDescription
                        }
                    }
                }
                .disabled(code.count < 6 || serverId.isEmpty)
            }
            .navigationTitle(MavioStrings.authAuthorizeDevice)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(MavioStrings.actionDone) { dismiss() }
                }
            }
        }
        .onAppear { serverId = model.signedInServers.first?.id ?? "" }
    }
}
