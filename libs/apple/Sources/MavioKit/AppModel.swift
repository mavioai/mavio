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
}

/// Receives the core's callbacks on its threads and queues them.
private final class Relay: CoreListener, HomeListener {
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

    func rows(rows: [Row]) {
        continuation.yield(.home(rows))
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
    /// The servers the device holds a token of.
    public private(set) var signedIn: Set<String> = []
    /// The state of each server's stream.
    public private(set) var states: [String: StreamState] = [:]
    /// The rows of Home.
    public private(set) var home: [Row] = []
    /// The folder of downloaded files.
    public let downloadsFolder: URL
    /// The downloads.
    public let downloads: Downloads
    private var revisions: [String: Int] = [:]
    @ObservationIgnored private let relay: Relay
    @ObservationIgnored private var homeTask: Task<Void, Never>?

    /// The core with its store in directory, Application Support by default.
    public init(directory: URL? = nil) throws {
        let (stream, continuation) = AsyncStream<Push>.makeStream()
        let relay = Relay(continuation)
        let dir = directory ?? URL.applicationSupportDirectory.appending(path: "Mavio", directoryHint: .isDirectory)
        downloadsFolder = dir.appending(path: "Downloads", directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: downloadsFolder, withIntermediateDirectories: true)
        self.relay = relay
        client = try Client(
            databasePath: dir.appending(path: "core.sqlite").path(percentEncoded: false),
            device: Self.device(),
            tokens: KeychainTokens(),
            listener: relay
        )
        downloads = Downloads(client: client, folder: downloadsFolder)
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
            }
        case let .home(rows):
            home = rows
        }
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

    /// Reads the servers again from the core.
    public func reloadServers() {
        servers = (try? client.servers()) ?? []
        signedIn = Set(servers.filter { client.signedIn(serverId: $0.id) }.map(\.id))
    }

    /// Follows the servers' event streams, when the app comes to the foreground.
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
}
