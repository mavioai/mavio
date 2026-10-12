import MavioKit
import MavioUI
import SwiftUI

/// Sign-in to a server: its address, typed or found on the network, then a
/// password, the first user of a new server, or Quick Connect.
struct SignInView: View {
    /// Whether a server is added to those signed in to, in a sheet.
    var adding = false
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @Environment(\.theme) private var theme
    @State private var address = ""
    @State private var info: ServerInfo?
    @State private var found: [FoundServer] = []
    @State private var name = ""
    @State private var password = ""
    @State private var quickConnect: QuickConnectRequest?
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        NavigationStack {
            Form {
                if let info {
                    credentials(info)
                } else {
                    addressStep
                }
            }
            .scrollContentBackground(.hidden)
            .background(theme.colors.background)
            .navigationTitle(title)
            .toolbar {
                if adding {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(MavioStrings.actionCancel) { dismiss() }
                    }
                }
                if info != nil {
                    ToolbarItem(placement: .navigation) {
                        Button {
                            info = nil
                            quickConnect = nil
                            error = nil
                        } label: {
                            Image(systemName: "chevron.backward")
                        }
                    }
                }
            }
            .disabled(busy)
        }
        .task {
            found = (try? await model.client.discover(wait: 2)) ?? []
        }
        .task(id: quickConnect.map(ObjectIdentifier.init)) { await pollQuickConnect() }
    }

    private var title: String {
        if let info { return info.setupRequired ? MavioStrings.authSetUp(server: info.name) : info.name }
        return adding ? MavioStrings.authAddServer : MavioStrings.authWelcome
    }

    @ViewBuilder private var addressStep: some View {
        Section {
            TextField(MavioStrings.authServer, text: $address, prompt: Text(verbatim: "https://media.example.com"))
                .textContentType(.URL)
                .keyboardType(.URL)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .onSubmit { Task { await probe(address) } }
            if let error { errorText(error) }
            Button(MavioStrings.authSignIn) { Task { await probe(address) } }
                .disabled(address.trimmingCharacters(in: .whitespaces).isEmpty)
        }
        let known = Set(model.servers.map(\.id))
        let nearby = found.filter { !known.contains($0.id) }
        if !nearby.isEmpty {
            Section(MavioStrings.authNearby) {
                ForEach(nearby, id: \.id) { server in
                    Button {
                        address = server.address
                        Task { await probe(server.address) }
                    } label: {
                        VStack(alignment: .leading) {
                            Text(server.name)
                            Text(server.address).font(.footnote).foregroundStyle(.secondary)
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder private func credentials(_ info: ServerInfo) -> some View {
        if let quickConnect {
            Section {
                Text(MavioStrings.authQuickConnectCode(code: quickConnect.code()))
                Text(quickConnect.code())
                    .font(.system(size: 44, weight: .semibold, design: .monospaced))
                    .tracking(8)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, MavioSpacing.s8)
                Button(MavioStrings.actionCancel) { self.quickConnect = nil }
            }
        } else {
            Section {
                if info.setupRequired { Text(MavioStrings.authSetUpMessage).foregroundStyle(.secondary) }
                TextField(MavioStrings.authUsername, text: $name)
                    .textContentType(.username)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                SecureField(MavioStrings.authPassword, text: $password)
                    .textContentType(info.setupRequired ? .newPassword : .password)
                    .onSubmit { Task { await signIn(info) } }
                if let error { errorText(error) }
                Button(info.setupRequired ? MavioStrings.authCreateAccount : MavioStrings.authSignIn) {
                    Task { await signIn(info) }
                }
                .disabled(name.isEmpty)
            } footer: {
                Text(info.address)
            }
            if !info.setupRequired {
                Section {
                    Button(MavioStrings.authQuickConnect) { Task { await startQuickConnect(info) } }
                }
            }
        }
    }

    private func errorText(_ message: String) -> some View {
        Text(message).foregroundStyle(theme.colors.danger).font(.callout)
    }

    private func run(_ body: () async throws -> Void) async {
        busy = true
        error = nil
        defer { busy = false }
        do {
            try await body()
        } catch let e as CoreError where e.isUnauthenticated {
            error = MavioStrings.authWrongPassword
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func probe(_ address: String) async {
        let address = address.trimmingCharacters(in: .whitespaces)
        guard !address.isEmpty else { return }
        await run {
            do {
                info = try await model.client.probe(address: address)
            } catch let e as CoreError where !e.isUnauthenticated {
                if "\(e)".localizedCaseInsensitiveContains("not a Mavio") { throw NotMavio() }
                throw e
            }
        }
    }

    private func signIn(_ info: ServerInfo) async {
        await run {
            if info.setupRequired {
                try await model.createFirstUser(address: info.address, name: name, password: password)
            } else {
                try await model.signIn(address: info.address, name: name, password: password)
            }
            finish()
        }
    }

    private func startQuickConnect(_ info: ServerInfo) async {
        await run { quickConnect = try await model.client.startQuickConnect(address: info.address) }
    }

    /// Asks every three seconds whether the code was entered, until it expires.
    private func pollQuickConnect() async {
        guard let request = quickConnect else { return }
        while !Task.isCancelled, Date() < request.expires() {
            try? await Task.sleep(for: .seconds(3))
            guard !Task.isCancelled else { return }
            do {
                if try await model.pollQuickConnect(request) != nil {
                    finish()
                    return
                }
            } catch {
                self.error = error.localizedDescription
                quickConnect = nil
                return
            }
        }
        quickConnect = nil
    }

    private func finish() {
        if adding { dismiss() }
    }
}

/// The address answers but is not a Mavio server.
private struct NotMavio: LocalizedError {
    var errorDescription: String? { MavioStrings.authNotMavio }
}
