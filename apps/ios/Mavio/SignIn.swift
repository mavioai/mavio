import AuthenticationServices
import MavioKit
import MavioUI
import SwiftUI

/// The steps of signing in, as the web UI's and the Android app's.
private enum Step: Equatable {
    /// The way in: a Mavio account, or a server's address.
    case welcome
    /// A link code approved on another device.
    case link
    /// A server's address, typed or found on the network.
    case address
    /// The name and password of a server's user.
    case credentials
    /// A Quick Connect code entered on a device signed in.
    case quickConnect
    /// A server not set up yet, which its setup in a browser does.
    case setUp
}

/// Sign-in (docs/accounts.md §6), as the web UI's on a phone: a Mavio
/// account, in the browser or with a link code, over the wall of posters;
/// or a server's address, then its users' name and password or Quick
/// Connect. A server in mavio.dev mode goes back to the Mavio account, at
/// its mavio.dev.
struct SignInView: View {
    /// Whether a server is added to those signed in to, in a sheet.
    var adding = false
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @Environment(\.theme) private var theme
    @Environment(\.openURL) private var openURL
    @Environment(\.webAuthenticationSession) private var webAuthenticationSession
    @State private var step = Step.welcome
    @State private var history: [Step] = []
    /// The server connected to, whose mavio.dev a Mavio account signs in at.
    @State private var info: ServerInfo?
    @State private var address = ""
    @State private var found: [FoundServer] = []
    @State private var name = ""
    @State private var password = ""
    @State private var quickConnect: QuickConnectRequest?
    @State private var link: LinkCode?
    @State private var linkAttempt = 0
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        ZStack {
            theme.colors.background.ignoresSafeArea()
            if step == .welcome {
                welcome
            } else {
                page
            }
        }
        .overlay(alignment: .topLeading) {
            if adding && history.isEmpty {
                Button(MavioStrings.actionCancel) { dismiss() }
                    .type(theme.type.body)
                    .foregroundStyle(theme.colors.accentText)
                    .padding(.horizontal, MavioSpacing.s20)
                    .padding(.top, MavioSpacing.s16)
            }
        }
        .disabled(busy)
        .onAppear { if adding { step = .address } }
        .task { found = (try? await model.client.discover(wait: 2)) ?? [] }
        .task(id: quickConnect.map(ObjectIdentifier.init)) { await pollQuickConnect() }
        .task(id: step == .link ? linkAttempt : nil) { if step == .link { await runLink() } }
    }

    // MARK: Layout

    /// The first step: the wall of posters over the top of the screen, and
    /// the way in at the bottom.
    private var welcome: some View {
        GeometryReader { geo in
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Wordmark(height: 40).padding(.bottom, MavioSpacing.s4)
                    Text(info.map { MavioStrings.authSignInTo(server: $0.name) } ?? MavioStrings.authWelcome)
                        .modifier(Title())
                    if info != nil {
                        Text(MavioStrings.authModeMavioDevMessage).modifier(BodyText())
                    }
                    Spacer().frame(height: MavioSpacing.s4)
                    if let error { ErrorLine(error) }
                    Action(MavioStrings.authSignInWithMavio) { Task { await browserSignIn() } }
                    Action(MavioStrings.authUseLinkCode, icon: "devices", primary: false) { go(.link) }
                    if info == nil {
                        Or()
                        Action(MavioStrings.authConnectToServer, icon: "server", primary: false) { go(.address) }
                    }
                }
                .frame(maxWidth: 420, alignment: .leading)
                .padding(.horizontal, MavioSpacing.s24)
                .padding(.bottom, MavioSpacing.s24)
                .frame(maxWidth: .infinity, minHeight: geo.size.height, alignment: .bottom)
            }
            .scrollBounceBehavior(.basedOnSize)
            .background(alignment: .top) {
                PosterWall()
                    .frame(height: (geo.size.height + geo.safeAreaInsets.top + geo.safeAreaInsets.bottom) * 0.68)
                    .ignoresSafeArea()
            }
        }
    }

    /// The other steps: a back link, the step's title and its content, on
    /// the background.
    private var page: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                if !history.isEmpty {
                    Button(action: back) {
                        HStack(spacing: 2) {
                            Icon("back", size: 22)
                            Text(MavioStrings.actionBack)
                        }
                        .font(.system(size: 17))
                        .foregroundStyle(theme.colors.accentText)
                    }
                    .padding(.leading, -4)
                }
                switch step {
                case .welcome: EmptyView()
                case .link: linkStep
                case .address: addressStep
                case .credentials: credentialsStep
                case .quickConnect: quickConnectStep
                case .setUp: setUpStep
                }
            }
            .frame(maxWidth: 420, alignment: .leading)
            .padding(.horizontal, MavioSpacing.s20)
            .padding(.top, adding && history.isEmpty ? 56 : MavioSpacing.s16)
            .padding(.bottom, 40)
            .frame(maxWidth: .infinity)
        }
        .scrollDismissesKeyboard(.interactively)
    }

    // MARK: Steps

    @ViewBuilder private var linkStep: some View {
        Heading(title: MavioStrings.authSignInWithMavio, subtitle: info?.name)
        if let link {
            Text(MavioStrings.authLinkMessage(url: link.verificationUri().replacing(/^https?:\/\//, with: "")))
                .modifier(BodyText())
            CodeBoxes(code: link.userCode())
        }
        if let error {
            ErrorLine(error)
            Action(MavioStrings.authTryAgain, primary: false) { linkAttempt += 1 }
        } else {
            Waiting()
        }
    }

    @ViewBuilder private var addressStep: some View {
        Heading(title: MavioStrings.authConnectToServer)
        Text(MavioStrings.authServerAddressMessage).modifier(BodyText())
        FieldGroup {
            FieldRow(label: MavioStrings.authServer) {
                TextField(text: $address, prompt: Text(verbatim: "http://192.168.1.5:8686")) { EmptyView() }
                    .textContentType(.URL)
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .submitLabel(.go)
                    .onSubmit { Task { await probe(address) } }
            }
        }
        if let error { ErrorLine(error) }
        Action(MavioStrings.actionContinue) { Task { await probe(address) } }
            .disabled(address.trimmingCharacters(in: .whitespaces).isEmpty)
        let known = Set(model.servers.map(\.id))
        let nearby = found.filter { !known.contains($0.id) }
        if !nearby.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(MavioStrings.authNearby.uppercased())
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(theme.colors.textTertiary)
                    .padding(.horizontal, MavioSpacing.s16)
                FieldGroup {
                    ForEach(nearby, id: \.id) { server in
                        Button {
                            address = server.address
                            Task { await probe(server.address) }
                        } label: {
                            HStack(spacing: MavioSpacing.s12) {
                                Icon("server", size: 22).foregroundStyle(theme.colors.accent)
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(server.name).font(.system(size: 17, weight: .semibold))
                                        .foregroundStyle(theme.colors.textPrimary)
                                    Text(server.address).type(theme.type.footnote)
                                        .foregroundStyle(theme.colors.textSecondary)
                                }
                                .lineLimit(1)
                                Spacer(minLength: 0)
                            }
                            .padding(.horizontal, 14)
                            .padding(.vertical, 10)
                            .contentShape(.rect)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
        }
    }

    @ViewBuilder private var credentialsStep: some View {
        if let info {
            Heading(
                title: MavioStrings.authSignInTo(server: info.name),
                subtitle: "\(MavioStrings.authStandaloneServer) · \(info.address)")
            FieldGroup {
                FieldRow(label: MavioStrings.authUsername) {
                    TextField(text: $name) { EmptyView() }
                        .textContentType(.username)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .submitLabel(.next)
                }
                FieldRow(label: MavioStrings.authPassword) {
                    SecureField(text: $password) { EmptyView() }
                        .textContentType(.password)
                        .submitLabel(.go)
                        .onSubmit { Task { await signIn(info) } }
                }
            }
            if let error { ErrorLine(error) }
            Action(MavioStrings.authSignIn) { Task { await signIn(info) } }
                .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty)
            Or()
            Action(MavioStrings.authQuickConnect, icon: "devices", primary: false) {
                Task { await startQuickConnect(info) }
            }
        }
    }

    @ViewBuilder private var quickConnectStep: some View {
        Heading(title: MavioStrings.authQuickConnectTitle)
        if let quickConnect {
            Text(MavioStrings.authQuickConnectCode(code: quickConnect.code())).modifier(BodyText())
            CodeBoxes(code: quickConnect.code())
        }
        if let error { ErrorLine(error) } else { Waiting() }
    }

    @ViewBuilder private var setUpStep: some View {
        if let info {
            Heading(
                title: MavioStrings.authSetUpElsewhere(server: info.name), eyebrow: MavioStrings.authNewServerSetup)
            Text(MavioStrings.authSetUpElsewhereMessage).modifier(BodyText())
            Text(info.address.replacing(/^https?:\/\//, with: ""))
                .font(.system(size: 30, weight: .semibold, design: .monospaced))
                .textSelection(.enabled)
            Action(MavioStrings.authOpenInBrowser, icon: "external") {
                if let url = URL(string: info.address) { openURL(url) }
            }
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
        if step == .welcome || step == .address { info = nil }
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
                go(found.mode == .mavioDev ? .welcome : found.setupRequired ? .setUp : .credentials)
            } catch let e as CoreError where !e.isUnauthenticated {
                if "\(e)".localizedCaseInsensitiveContains("not a Mavio") { throw NotMavio() }
                throw e
            }
        }
    }

    private func signIn(_ info: ServerInfo) async {
        await run {
            try await model.signIn(
                address: info.address, name: name.trimmingCharacters(in: .whitespaces), password: password)
            finish()
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
                    finish()
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
    private static let clientId = "dev.mavio.ios"

    /// The API of the mavio.dev the server belongs to, or the one this build
    /// signs in at (`MAVIO_DEV_API`).
    private var api: String {
        if let url = info?.mavioDevUrl, !url.isEmpty { return url }
        let built = Bundle.main.object(forInfoDictionaryKey: "MavioDevAPI") as? String ?? ""
        return built.isEmpty ? "https://mavio.dev/api" : built
    }

    /// Signs the account in in a web authentication session. mavio.dev sends
    /// the browser back to the core's loopback address (RFC 8252), which the
    /// session cannot catch, so the session is closed once the core has the
    /// code; closing it first cancels the sign-in.
    private func browserSignIn() async {
        await run {
            let signIn = try await model.client.startBrowserSignIn(api: api, clientId: Self.clientId)
            guard let url = URL(string: signIn.url()) else { return }
            let browser = webAuthenticationSession
            let browsing = Task {
                _ = try? await browser.authenticate(using: url, callbackURLScheme: "mavio")
                // The browser closed before the core had the code: the
                // person cancelled.
                signIn.cancel()
            }
            defer { browsing.cancel() }
            do {
                try await model.finishBrowserSignIn(signIn)
            } catch CoreError.NotFound {
                return
            }
            finish()
        }
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
                    if try await model.pollLink(code) != nil {
                        finish()
                        return
                    }
                }
            }
        } catch is CancellationError {
        } catch {
            link = nil
            self.error = error.reason
        }
    }

    private func finish() {
        if adding { dismiss() }
    }
}

// MARK: - Parts

/// The title of a step, 30 pt bold as the web UI's on a phone.
private struct Title: ViewModifier {
    func body(content: Content) -> some View {
        content.font(.system(size: 30, weight: .bold)).tracking(-0.3).fixedSize(horizontal: false, vertical: true)
    }
}

/// A step's line of text.
private struct BodyText: ViewModifier {
    @Environment(\.theme) private var theme

    func body(content: Content) -> some View {
        content.font(.system(size: 16)).lineSpacing(3).foregroundStyle(theme.colors.textSecondary)
            .fixedSize(horizontal: false, vertical: true)
    }
}

/// A step's eyebrow, title and the line under it.
private struct Heading: View {
    let title: String
    var eyebrow: String?
    var subtitle: String?
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let eyebrow {
                Text(eyebrow).font(.system(size: 13, weight: .semibold)).foregroundStyle(theme.colors.textTertiary)
            }
            Text(title).modifier(Title())
            if let subtitle {
                Text(subtitle).font(.system(size: 15)).foregroundStyle(theme.colors.textTertiary).lineLimit(1)
            }
        }
    }
}

/// A button as wide as the step: the primary filled with `color.button`,
/// the others with `color.fill`.
private struct Action: View {
    let title: String
    var icon: String?
    var primary = true
    let run: () -> Void
    @Environment(\.theme) private var theme
    @Environment(\.isEnabled) private var enabled

    init(_ title: String, icon: String? = nil, primary: Bool = true, run: @escaping () -> Void) {
        self.title = title
        self.icon = icon
        self.primary = primary
        self.run = run
    }

    var body: some View {
        Button(action: run) {
            HStack(spacing: MavioSpacing.s8) {
                if let icon { Icon(icon, size: 20) }
                Text(title)
            }
            .font(.system(size: 17, weight: .semibold))
            .foregroundStyle(primary ? theme.colors.onButton : theme.colors.textPrimary)
            .padding(.horizontal, MavioSpacing.s20)
            .frame(maxWidth: .infinity, minHeight: 50)
            .background(primary ? theme.colors.button : theme.colors.fill, in: .rect(cornerRadius: 14))
            .contentShape(.rect(cornerRadius: 14))
        }
        .buttonStyle(Pressed())
        .opacity(enabled ? 1 : 0.5)
    }
}

/// A button dimmed while pressed.
private struct Pressed: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label.opacity(configuration.isPressed ? 0.8 : 1)
    }
}

/// A line between ways of going on, broken by "or".
private struct Or: View {
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            line
            Text(MavioStrings.authOr).font(.system(size: 13)).foregroundStyle(theme.colors.textTertiary)
            line
        }
    }

    private var line: some View { Rectangle().fill(Separator.color).frame(height: 1 / 3) }
}

/// The sign-in's separators and grouped rows, as the web UI's.
private enum Separator {
    static let color = Color(uiColor: .separator)
    static let group = Color(.sRGB, red: 118 / 255, green: 118 / 255, blue: 128 / 255, opacity: 1)
}

/// Fields as the rows of an inset group.
private struct FieldGroup<Content: View>: View {
    @ViewBuilder let content: Content
    @Environment(\.colorScheme) private var scheme

    var body: some View {
        VStack(spacing: 0) {
            Group(subviews: content) { rows in
                ForEach(Array(rows.enumerated()), id: \.offset) { i, row in
                    if i > 0 { Rectangle().fill(Separator.color).frame(height: 1 / 3) }
                    row
                }
            }
        }
        .background(Separator.group.opacity(scheme == .dark ? 0.14 : 0.08), in: .rect(cornerRadius: 12))
        .clipShape(.rect(cornerRadius: 12))
    }
}

/// A field's row: its label, 104 pt wide, beside it.
private struct FieldRow<Field: View>: View {
    let label: String
    @ViewBuilder let field: Field
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: 0) {
            Text(label).font(.system(size: 16)).foregroundStyle(theme.colors.textPrimary)
                .frame(width: 104, alignment: .leading)
                .padding(.leading, MavioSpacing.s16)
            field.font(.system(size: 17)).padding(.trailing, MavioSpacing.s16)
        }
        .frame(minHeight: 48)
    }
}

/// A code to enter on another device, a character to a box.
private struct CodeBoxes: View {
    let code: String
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: 10) {
            ForEach(Array(code.enumerated()), id: \.offset) { _, c in
                if c == "-" {
                    Text(verbatim: "–").foregroundStyle(theme.colors.textTertiary).frame(width: 12)
                } else {
                    Text(String(c))
                        .font(.system(size: 26, weight: .semibold, design: .monospaced))
                        .frame(width: 40, height: 52)
                        .background(theme.colors.fill, in: .rect(cornerRadius: 10))
                }
            }
        }
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(Text(verbatim: code))
    }
}

/// Waiting for another device or the browser.
private struct Waiting: View {
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s8) {
            ProgressView().controlSize(.small)
            Text(MavioStrings.authWaiting)
        }
        .font(.system(size: 16))
        .foregroundStyle(theme.colors.textSecondary)
        .frame(maxWidth: .infinity)
    }
}

private struct ErrorLine: View {
    let message: String
    @Environment(\.theme) private var theme

    init(_ message: String) { self.message = message }

    var body: some View {
        Text(message).font(.system(size: 15)).foregroundStyle(theme.colors.danger)
            .fixedSize(horizontal: false, vertical: true)
    }
}

/// The address answers but is not a Mavio server.
private struct NotMavio: LocalizedError {
    var errorDescription: String? { MavioStrings.authNotMavio }
}
