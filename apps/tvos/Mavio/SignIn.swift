import MavioKit
import MavioUI
import SwiftUI

/// Signing in (docs/tv.md §6.5): a Mavio account with a code entered at
/// mavio.dev/link; without one, a server by its address, typed or found
/// nearby, with Quick Connect first, or a user's name and password. Adding
/// a server starts at its address, and closes once signed in or on back.
struct SignInView: View {
    var adding = false

    private enum Step: Hashable {
        case link, address, credentials, quickConnect, setUp
    }

    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var step = Step.link
    @State private var history: [Step] = []
    @State private var info: ServerInfo?
    @State private var link: LinkCode?
    @State private var linkAttempt = 0
    @State private var quickConnect: QuickConnectRequest?
    @State private var found: [FoundServer] = []
    @State private var address = ""
    @State private var name = ""
    @State private var password = ""
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        HStack(alignment: .center, spacing: 120) {
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                Wordmark(height: 72)
                Text(heading).type(theme.type.display).foregroundStyle(theme.colors.textPrimary)
                if let subheading {
                    Text(subheading).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                }
            }
            .frame(width: 720, alignment: .leading)
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                content
            }
            .frame(width: 760, alignment: .leading)
            .focusSection()
        }
        .padding(theme.layout.margin)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .disabled(busy)
        .onExitCommand(perform: history.isEmpty ? (adding ? { dismiss() } : nil) : { back() })
        .onAppear { if adding { step = .address } }
        .task { found = (try? await model.client.discover(wait: 2)) ?? [] }
        .task(id: quickConnect.map(ObjectIdentifier.init)) { await pollQuickConnect() }
        .task(id: step == .link ? linkAttempt : nil) { if step == .link { await runLink() } }
    }

    private var heading: String {
        switch step {
        case .link: info.map { MavioStrings.authSignInTo(server: $0.name) } ?? MavioStrings.authWelcome
        case .address: MavioStrings.authConnectToServer
        case .credentials: info.map { MavioStrings.authSignInTo(server: $0.name) } ?? ""
        case .quickConnect: MavioStrings.authQuickConnectTitle
        case .setUp: info.map { MavioStrings.authSetUpElsewhere(server: $0.name) } ?? ""
        }
    }

    private var subheading: String? {
        switch step {
        case .link: info == nil ? nil : MavioStrings.authModeMavioDevMessage
        case .address: MavioStrings.authServerAddressMessage
        case .credentials: info.map { "\(MavioStrings.authStandaloneServer) · \($0.address)" }
        case .quickConnect: nil
        case .setUp: MavioStrings.authSetUpElsewhereMessage
        }
    }

    // MARK: Steps

    @ViewBuilder private var content: some View {
        switch step {
        case .link: linkStep
        case .address: addressStep
        case .credentials: credentialsStep
        case .quickConnect: quickConnectStep
        case .setUp: setUpStep
        }
    }

    @ViewBuilder private var linkStep: some View {
        Text(MavioStrings.authSignInWithMavio).type(theme.type.title2).foregroundStyle(theme.colors.textPrimary)
        if let link {
            Text(MavioStrings.authLinkMessage(url: link.verificationUri().replacing(/^https?:\/\//, with: "")))
                .type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
            CodeBoxes(code: link.userCode())
            Scan(text: link.verificationUriComplete())
        }
        if let error {
            ErrorLine(error)
            Button(MavioStrings.authTryAgain) { linkAttempt += 1 }.buttonStyle(TVButtonStyle())
        } else {
            Waiting()
        }
        if info == nil {
            Button {
                go(.address)
            } label: {
                Label {
                    Text(MavioStrings.authConnectToServer)
                } icon: {
                    Icon("server", size: 28)
                }
            }
            .buttonStyle(TVButtonStyle())
            .padding(.top, MavioSpacing.s24)
        }
    }

    @ViewBuilder private var addressStep: some View {
        TextField(text: $address, prompt: Text(verbatim: "http://192.168.1.5:8686")) {
            Text(MavioStrings.authServer)
        }
        .keyboardType(.URL)
        .textContentType(.URL)
        .autocorrectionDisabled()
        .onSubmit { Task { await probe(address) } }
        if let error { ErrorLine(error) }
        Button(MavioStrings.actionContinue) { Task { await probe(address) } }
            .buttonStyle(TVButtonStyle())
            .disabled(address.trimmingCharacters(in: .whitespaces).isEmpty)
        let known = Set(model.servers.map(\.id))
        let nearby = found.filter { !known.contains($0.id) }
        if !nearby.isEmpty {
            Text(MavioStrings.authNearby).type(theme.type.headline).foregroundStyle(theme.colors.textSecondary)
                .padding(.top, MavioSpacing.s16)
            ForEach(nearby, id: \.id) { server in
                Button {
                    address = server.address
                    Task { await probe(server.address) }
                } label: {
                    HStack(spacing: MavioSpacing.s16) {
                        Icon("server", size: 32)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(server.name).type(theme.type.headline)
                            Text(server.address).type(theme.type.footnote).opacity(0.7)
                        }
                        .lineLimit(1)
                    }
                    .padding(.vertical, MavioSpacing.s12)
                }
                .buttonStyle(RowButtonStyle())
            }
        }
    }

    @ViewBuilder private var credentialsStep: some View {
        if let info {
            Button {
                Task { await startQuickConnect(info) }
            } label: {
                Label {
                    Text(MavioStrings.authQuickConnect)
                } icon: {
                    Icon("devices", size: 28)
                }
            }
            .buttonStyle(TVButtonStyle())
            Text(MavioStrings.authOr).type(theme.type.footnote).foregroundStyle(theme.colors.textTertiary)
            TextField(text: $name) { Text(MavioStrings.authUsername) }
                .textContentType(.username)
                .autocorrectionDisabled()
            SecureField(text: $password) { Text(MavioStrings.authPassword) }
                .textContentType(.password)
                .onSubmit { Task { await signIn(info) } }
            if let error { ErrorLine(error) }
            Button(MavioStrings.authSignIn) { Task { await signIn(info) } }
                .buttonStyle(TVButtonStyle())
                .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty)
        }
    }

    @ViewBuilder private var quickConnectStep: some View {
        if let quickConnect {
            Text(MavioStrings.authQuickConnectCode(code: quickConnect.code()))
                .type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
            CodeBoxes(code: quickConnect.code())
        }
        if let error { ErrorLine(error) } else { Waiting() }
    }

    @ViewBuilder private var setUpStep: some View {
        if let info {
            Text(info.address.replacing(/^https?:\/\//, with: ""))
                .font(.system(size: 48, weight: .semibold, design: .monospaced))
                .foregroundStyle(theme.colors.textPrimary)
            Scan(text: info.address)
        }
    }

    // MARK: Navigation

    private func go(_ next: Step) {
        history.append(step)
        step = next
        error = nil
    }

    private func back() {
        guard let previous = history.popLast() else { return }
        step = previous
        quickConnect = nil
        link = nil
        if step == .link || step == .address { info = nil }
        error = nil
    }

    // MARK: Actions

    private func run(_ body: () async throws -> Void) async {
        busy = true
        error = nil
        defer { busy = false }
        do {
            try await body()
        } catch let e as CoreError where e.isUnauthenticated {
            error = MavioStrings.authWrongPassword
        } catch is CancellationError {
        } catch {
            self.error = error.reason
        }
    }

    private func probe(_ address: String) async {
        let address = address.trimmingCharacters(in: .whitespaces)
        guard !address.isEmpty else { return }
        await run {
            do {
                let found = try await model.client.probe(address: address)
                info = found
                go(found.mode == .mavioDev ? .link : found.setupRequired ? .setUp : .credentials)
            } catch let e as CoreError where !e.isUnauthenticated {
                if "\(e)".localizedCaseInsensitiveContains("not a Mavio") {
                    error = MavioStrings.authNotMavio
                    return
                }
                throw e
            }
        }
    }

    private func signIn(_ info: ServerInfo) async {
        await run {
            try await model.signIn(
                address: info.address, name: name.trimmingCharacters(in: .whitespaces), password: password)
            if adding { dismiss() }
        }
    }

    private func startQuickConnect(_ info: ServerInfo) async {
        await run {
            quickConnect = try await model.client.startQuickConnect(address: info.address)
            go(.quickConnect)
        }
    }

    /// Asks every three seconds whether the code was entered, until it expires.
    private func pollQuickConnect() async {
        guard let request = quickConnect else { return }
        while !Task.isCancelled, Date() < request.expires() {
            try? await Task.sleep(for: .seconds(3))
            guard !Task.isCancelled else { return }
            do {
                if try await model.pollQuickConnect(request) != nil {
                    if adding { dismiss() }
                    return
                }
            } catch {
                self.error = error.reason
                return
            }
        }
        if !Task.isCancelled, step == .quickConnect { back() }
    }

    /// The client ID of this app at mavio.dev.
    private static let clientId = "dev.mavio.tvos"

    /// The API of the mavio.dev the server belongs to, or the one this build
    /// signs in at (`MAVIO_DEV_API`).
    private var api: String {
        if let url = info?.mavioDevUrl, !url.isEmpty { return url }
        let built = Bundle.main.object(forInfoDictionaryKey: "MavioDevAPI") as? String ?? ""
        return built.isEmpty ? "https://mavio.dev/api" : built
    }

    /// Shows a link code and asks every interval whether it was approved,
    /// with a new code as each expires.
    private func runLink() async {
        error = nil
        do {
            while !Task.isCancelled {
                let code = try await model.client.startLink(api: api, clientId: Self.clientId)
                link = code
                while !Task.isCancelled, Date() < code.expires() {
                    try await Task.sleep(for: .seconds(max(code.interval(), 1)))
                    if try await model.pollLink(code) != nil { return }
                }
            }
        } catch is CancellationError {
        } catch {
            link = nil
            self.error = error.reason
        }
    }
}

/// A code in boxes, a character each.
private struct CodeBoxes: View {
    let code: String
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            ForEach(Array(code.enumerated()), id: \.offset) { _, c in
                if c == "-" {
                    Text(verbatim: "–").foregroundStyle(theme.colors.textTertiary).frame(width: 24)
                } else {
                    Text(String(c))
                        .font(.system(size: 52, weight: .semibold, design: .monospaced))
                        .frame(width: 72, height: 96)
                        .background(theme.colors.fill, in: .rect(cornerRadius: MavioRadius.lg))
                }
            }
        }
        .foregroundStyle(theme.colors.textPrimary)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Text(verbatim: code))
    }
}

/// The QR code of an address for a phone to scan, the line beside it: the
/// link page with the code filled in, or a new server's address.
private struct Scan: View {
    let text: String
    @Environment(\.theme) private var theme

    var body: some View {
        if let modules = QRModules(encoding: text) {
            HStack(spacing: MavioSpacing.s24) {
                QRCodeView(modules, quiet: 16, radius: MavioRadius.lg, label: text).frame(width: 240, height: 240)
                Text(MavioStrings.authScanLinkCode).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
            }
        }
    }
}

/// Waiting for another device.
private struct Waiting: View {
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            ProgressView()
            Text(MavioStrings.authWaiting).type(theme.type.body)
        }
        .foregroundStyle(theme.colors.textSecondary)
    }
}

/// What went wrong.
struct ErrorLine: View {
    let text: String
    @Environment(\.theme) private var theme

    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text).type(theme.type.body).foregroundStyle(theme.colors.danger)
    }
}
