import Foundation
import MavioCore
import Observation

#if canImport(UIKit)
    import UIKit
#endif

/// What the core pushed, relayed to the main actor.
private enum Push: Sendable {
    case state(String, StreamState)
    case event(String, ServerEvent)
    case home([Row])
    case serversChanged
    case connected(String, String)
    case syncPlay(String, Mavio_Session_V1_SyncPlayUpdate)
}

/// Receives the core's callbacks on its threads and queues them.
private final class Relay: CoreListener, HomeListener, SyncPlayListener {
    let continuation: AsyncStream<Push>.Continuation

    init(_ continuation: AsyncStream<Push>.Continuation) {
        self.continuation = continuation
    }

    func serverState(serverId: String, state: StreamState) {
        continuation.yield(.state(serverId, state))
    }

    func event(serverId: String, event: ServerEvent) {
        continuation.yield(.event(serverId, event))
    }

    func serversChanged() {
        continuation.yield(.serversChanged)
    }

    func rows(rows: [Row]) {
        continuation.yield(.home(rows))
    }

    func connected(serverId: String, sessionId: String) {
        continuation.yield(.connected(serverId, sessionId))
    }

    func update(serverId: String, update: Data) {
        guard let update = try? Mavio_Session_V1_SyncPlayUpdate(serializedBytes: update) else { return }
        continuation.yield(.syncPlay(serverId, update))
    }
}

/// The app's state over the core: the servers, their streams and Home.
/// Screens reload a server's data when its revision moves, which the
/// server's events and the user's changes do.
@MainActor
@Observable
public final class AppModel {
    /// The core.
    public let client: Client
    /// The servers, in the user's order.
    public private(set) var servers: [ServerRecord] = []
    /// What the device is signed in with; signed out, the app shows its
    /// sign-in.
    public private(set) var signInState: SignInState = .signedOut
    /// The Mavio account signed in, whose servers the core follows.
    public var account: Account? {
        if case let .account(account) = signInState { account } else { nil }
    }
    /// The servers the device holds a token of.
    public private(set) var signedIn: Set<String> = []
    /// The state of each server's stream.
    public private(set) var states: [String: StreamState] = [:]
    /// The rows of Home.
    public private(set) var home: [Row] = []
    /// How far the libraries' scans have come, by server and library, as
    /// the servers tell administrators.
    public private(set) var scanProgress: [String: [String: ScanProgress]] = [:]
    /// The folder of downloaded files.
    public let downloadsFolder: URL
    /// The downloads.
    public let downloads: Downloads
    /// This device as the servers know it.
    public let device: DeviceInfo
    /// The SyncPlay group the device is in.
    public let syncPlay: SyncPlay
    private var revisions: [String: Int] = [:]
    @ObservationIgnored private let relay: Relay
    @ObservationIgnored private var homeTask: Task<Void, Never>?

    /// The core with its store in directory, Application Support by default,
    /// and its secrets in tokens, the Keychain by default.
    public init(directory: URL? = nil, tokens: (any SecureTokens)? = nil) throws {
        let (stream, continuation) = AsyncStream<Push>.makeStream()
        let relay = Relay(continuation)
        let dir = directory ?? URL.applicationSupportDirectory.appending(path: "Mavio", directoryHint: .isDirectory)
        downloadsFolder = dir.appending(path: "Downloads", directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: downloadsFolder, withIntermediateDirectories: true)
        self.relay = relay
        let device = Self.device()
        self.device = device
        // The servers name what they translate, as the plugins, in the
        // languages the app's strings are chosen from.
        setLanguages(tags: Locale.preferredLanguages)
        client = try Client(
            databasePath: dir.appending(path: "core.sqlite").path(percentEncoded: false),
            device: device,
            tokens: tokens ?? KeychainTokens(),
            listener: relay
        )
        downloads = Downloads(client: client, folder: downloadsFolder)
        syncPlay = SyncPlay(client: client)
        client.setSyncPlayListener(listener: relay)
        reloadServers()
        Task { [weak self] in
            for await push in stream {
                self?.receive(push)
            }
        }
    }

    /// This device as the servers know it; its ID stays across launches.
    private static func device() -> DeviceInfo {
        let defaults = UserDefaults.standard
        let id: String
        if let saved = defaults.string(forKey: "deviceId") {
            id = saved
        } else {
            id = UUID().uuidString.lowercased()
            defaults.set(id, forKey: "deviceId")
        }
        let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0.0.0"
        #if os(macOS)
            let name = Host.current().localizedName ?? "Mac"
            let client = "Mavio for Mac"
        #elseif os(tvOS)
            let name = UIDevice.current.name
            let client = "Mavio for Apple TV"
        #else
            let name = UIDevice.current.name
            let client = "Mavio for iOS"
        #endif
        return DeviceInfo(id: id, name: name, client: client, clientVersion: version)
    }

    private func receive(_ push: Push) {
        switch push {
        case let .state(id, state):
            let was = states[id]
            states[id] = state
            if state == .signedOut {
                reloadServers()
                refreshHome()
            } else if state == .online, was != .online {
                bump(id)
            }
        case let .event(id, event):
            switch event {
            case .connected, .libraryChanged, .userDataChanged:
                bump(id)
            case .sessionsChanged:
                break
            case let .scanProgressChanged(library, progress):
                scanProgress[id, default: [:]][library] = progress
            }
        case let .home(rows):
            home = rows
        case .serversChanged:
            reloadServers()
            refreshHome()
        case let .connected(id, session):
            syncPlay.connected(id, session)
        case let .syncPlay(id, update):
            syncPlay.update(id, update)
        }
    }

    /// Takes the scan progress of a server's libraries as listed, which the
    /// server's events then carry on.
    public func listedScans(_ serverId: String, _ libraries: [Mavio_Library_V1_Library]) {
        var scans: [String: ScanProgress] = [:]
        for l in libraries where l.hasScanProgress {
            let p = l.scanProgress
            scans[l.id] = ScanProgress(
                readingFolders: p.phase == .folders, foldersDone: p.foldersDone, foldersTotal: p.foldersTotal,
                itemsDone: p.itemsDone, itemsTotal: p.itemsTotal, fraction: p.fraction)
        }
        scanProgress[serverId] = scans
    }

    /// Marks a server's data changed, reloading Home and the screens showing it.
    public func bump(_ serverId: String) {
        revisions[serverId, default: 0] += 1
        refreshHome()
    }

    /// The revision of a server's data, which moves when it changes.
    public func revision(_ serverId: String) -> Int {
        revisions[serverId, default: 0]
    }

    /// The servers the user is signed in to, in their order.
    public var signedInServers: [ServerRecord] {
        servers.filter { signedIn.contains($0.id) }
    }

    /// A server by ID.
    public func server(_ id: String) -> ServerRecord? {
        servers.first { $0.id == id }
    }

    /// Asks a server its name and version again, as after an administrator
    /// renames it.
    public func refreshServerInfo(_ serverId: String) async {
        if (try? await client.refreshServerInfo(serverId: serverId)) != nil {
            reloadServers()
        }
    }

    /// Reads the servers again from the core.
    public func reloadServers() {
        signInState = (try? client.signInState()) ?? .signedOut
        servers = (try? client.servers()) ?? []
        signedIn = Set(servers.filter { client.signedIn(serverId: $0.id) }.map(\.id))
    }

    /// Follows the servers' event streams, when the app comes to the
    /// foreground, and the Mavio account's servers, among them those it could
    /// not reach before.
    public func start() {
        try? client.start()
    }

    /// Stops the event streams, when the app goes to the background.
    public func stop() {
        client.stop()
    }

    /// Loads Home again from every server.
    public func refreshHome() {
        homeTask?.cancel()
        let client = client
        let relay = relay
        homeTask = Task {
            try? await client.loadHome(listener: relay)
        }
    }

    /// Loads Home again from every server, returning once every server has
    /// answered, as pulling Home down waits for.
    public func reloadHome() async {
        refreshHome()
        await homeTask?.value
    }

    /// Signs in to the server at address with a name and password.
    @discardableResult
    public func signIn(address: String, name: String, password: String) async throws -> ServerRecord {
        let server = try await client.signIn(address: address, name: name, password: password)
        added()
        return server
    }

    /// Creates the first user of a new server and signs in as them.
    @discardableResult
    public func createFirstUser(address: String, name: String, password: String) async throws -> ServerRecord {
        let server = try await client.createFirstUser(address: address, name: name, password: password)
        added()
        return server
    }

    /// Asks whether a Quick Connect request was authorized, signing in when it was.
    public func pollQuickConnect(_ request: QuickConnectRequest) async throws -> ServerRecord? {
        let server = try await client.pollQuickConnect(request: request)
        if server != nil { added() }
        return server
    }

    /// Asks whether a link code was approved, signing the Mavio account in
    /// with its servers when it was.
    public func pollLink(_ link: LinkCode) async throws -> Account? {
        let account = try await client.pollLink(link: link)
        if account != nil { added() }
        return account
    }

    /// Waits for the browser to come back from a Mavio account's sign-in,
    /// and signs the account in with its servers.
    @discardableResult
    public func finishBrowserSignIn(_ signIn: BrowserSignIn) async throws -> Account {
        let account = try await client.finishBrowserSignIn(signIn: signIn)
        added()
        return account
    }

    /// Signs out of the Mavio account and its servers, or of every server.
    public func signOut() async throws {
        try await client.signOut()
        states = [:]
        reloadServers()
        refreshHome()
    }

    /// Signs out of a server and forgets it.
    public func remove(_ serverId: String) async throws {
        try await client.removeServer(serverId: serverId)
        states[serverId] = nil
        reloadServers()
        refreshHome()
    }

    /// Orders the servers as the user arranged them, which orders Home.
    public func order(_ serverIds: [String]) {
        try? client.orderServers(serverIds: serverIds)
        reloadServers()
        refreshHome()
    }

    /// Switches a standalone server's session to another of its users, with
    /// their PIN when they have one; everything shown of the server was the
    /// previous user's, so it reloads.
    public func switchUser(_ serverId: String, to userId: String, pin: String? = nil) async throws {
        _ = try await client.switchUser(serverId: serverId, userId: userId, pin: pin)
        reloadServers()
        bump(serverId)
    }

    /// Marks an item played or unplayed.
    public func setPlayed(_ card: Card, _ played: Bool) async throws {
        try await client.setPlayed(serverId: card.serverId, itemId: card.itemId, played: played)
        bump(card.serverId)
    }

    /// Marks an item a favorite or not.
    public func setFavorite(_ card: Card, _ favorite: Bool) async throws {
        try await client.setFavorite(serverId: card.serverId, itemId: card.itemId, favorite: favorite)
        bump(card.serverId)
    }

    private func added() {
        reloadServers()
        start()
        refreshHome()
    }
}

extension CoreError {
    /// Whether the server rejected the credentials or token.
    public var isUnauthenticated: Bool {
        if case .Unauthenticated = self { return true }
        return false
    }

    /// What happened, as the server or the core told it.
    public var reason: String {
        switch self {
        case .Unreachable(let reason), .Unauthenticated(let reason), .PermissionDenied(let reason),
            .NotFound(let reason), .Invalid(let reason), .FailedPrecondition(let reason),
            .Server(let reason), .Store(let reason):
            reason
        }
    }
}

extension Error {
    /// What happened, as the server or the core told it.
    public var reason: String { (self as? CoreError)?.reason ?? localizedDescription }
}
