import Foundation

extension Client {
    /// Calls a unary procedure of a server signed in to as its session, for
    /// the screens the core does not model, such as the settings, as the web
    /// UI calls the server's API: procedure is its path, such as
    /// "/mavio.system.v1.SystemService/GetSystemInfo".
    public func call<Response: SwiftProtobuf.Message>(
        _ serverId: String, _ procedure: String, _ request: some SwiftProtobuf.Message,
        as _: Response.Type = Response.self
    ) async throws -> Response {
        let response = try await callServer(
            serverId: serverId, procedure: procedure, request: try request.serializedBytes())
        return try Response(serializedBytes: response.body)
    }

    /// Calls a server-streaming procedure of a server signed in to as its
    /// session, as `call` a unary one: its responses as they come, until
    /// the stream ends; ending the iteration cancels the call.
    public func callStream<Response: SwiftProtobuf.Message>(
        _ serverId: String, _ procedure: String, _ request: some SwiftProtobuf.Message,
        as _: Response.Type = Response.self
    ) -> AsyncThrowingStream<Response, any Error> {
        let body: Data
        do {
            body = try request.serializedBytes()
        } catch {
            return AsyncThrowingStream { $0.finish(throwing: error) }
        }
        return AsyncThrowingStream { continuation in
            let listener = StreamMessages { data in
                do {
                    continuation.yield(try Response(serializedBytes: data))
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            let task = Task {
                do {
                    _ = try await callServerStream(
                        serverId: serverId, procedure: procedure, request: body, listener: listener)
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }
}

/// Receives a streaming call's responses on the core's threads.
private final class StreamMessages: StreamListener {
    let receive: @Sendable (Data) -> Void

    init(_ receive: @escaping @Sendable (Data) -> Void) {
        self.receive = receive
    }

    func message(body: Data) {
        receive(body)
    }
}

extension Google_Protobuf_Timestamp {
    /// The time, unset when it is the zero timestamp a message leaves out.
    public var time: Date? {
        seconds == 0 && nanos == 0 ? nil : date
    }
}

/// The paths of the server's procedures the screens call that the core
/// does not model.
public enum Procedure {
    /// The path of a method of `mavio.system.v1.SystemService`.
    public static func system(_ method: String) -> String { "/mavio.system.v1.SystemService/\(method)" }
    /// The path of a method of `mavio.system.v1.PluginManagementService`.
    public static func plugins(_ method: String) -> String { "/mavio.system.v1.PluginManagementService/\(method)" }
    /// The path of a method of `mavio.user.v1.UserService`.
    public static func users(_ method: String) -> String { "/mavio.user.v1.UserService/\(method)" }
    /// The path of a method of `mavio.system.v1.ActivityService`.
    public static func activity(_ method: String) -> String { "/mavio.system.v1.ActivityService/\(method)" }
    /// The path of a method of `mavio.system.v1.TaskService`.
    public static func tasks(_ method: String) -> String { "/mavio.system.v1.TaskService/\(method)" }
    /// The path of a method of `mavio.system.v1.BackupService`.
    public static func backups(_ method: String) -> String { "/mavio.system.v1.BackupService/\(method)" }
    /// The path of a method of `mavio.session.v1.SessionService`.
    public static func sessions(_ method: String) -> String { "/mavio.session.v1.SessionService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.IssueService`.
    public static func issues(_ method: String) -> String { "/mavio.acquisition.v1.IssueService/\(method)" }
    /// The path of a method of `mavio.library.v1.ItemService`.
    public static func items(_ method: String) -> String { "/mavio.library.v1.ItemService/\(method)" }
    /// The path of a method of `mavio.library.v1.LibraryService`.
    public static func libraries(_ method: String) -> String { "/mavio.library.v1.LibraryService/\(method)" }
    /// The path of a method of `mavio.storage.v1.StorageService`.
    public static func storage(_ method: String) -> String { "/mavio.storage.v1.StorageService/\(method)" }
    /// The path of a method of `mavio.system.v1.LocalizationService`.
    public static func localization(_ method: String) -> String { "/mavio.system.v1.LocalizationService/\(method)" }
    /// The path of a method of `mavio.auth.v1.AuthService`.
    public static func auth(_ method: String) -> String { "/mavio.auth.v1.AuthService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.DiscoverService`.
    public static func discover(_ method: String) -> String { "/mavio.acquisition.v1.DiscoverService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.ProfileService`.
    public static func profiles(_ method: String) -> String { "/mavio.acquisition.v1.ProfileService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.ReleaseService`.
    public static func releases(_ method: String) -> String { "/mavio.acquisition.v1.ReleaseService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.DownloadService`.
    public static func downloads(_ method: String) -> String { "/mavio.acquisition.v1.DownloadService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.WantedService`.
    public static func wanted(_ method: String) -> String { "/mavio.acquisition.v1.WantedService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.SubtitleService`.
    public static func subtitles(_ method: String) -> String { "/mavio.acquisition.v1.SubtitleService/\(method)" }
    /// The path of a method of `mavio.acquisition.v1.RequestService`.
    public static func requests(_ method: String) -> String { "/mavio.acquisition.v1.RequestService/\(method)" }
    /// The path of a method of `mavio.system.v1.AiProviderService`.
    public static func aiProviders(_ method: String) -> String { "/mavio.system.v1.AiProviderService/\(method)" }
    /// The path of a method of `mavio.system.v1.AiConversationService`.
    public static func aiConversations(_ method: String) -> String {
        "/mavio.system.v1.AiConversationService/\(method)"
    }
    /// The path of a method of `mavio.system.v1.ProxyService`.
    public static func proxies(_ method: String) -> String { "/mavio.system.v1.ProxyService/\(method)" }
    /// The path of a method of `mavio.session.v1.SyncPlayService`.
    public static func syncPlay(_ method: String) -> String { "/mavio.session.v1.SyncPlayService/\(method)" }
    /// The path of a method of `mavio.library.v1.MetadataService`.
    public static func metadata(_ method: String) -> String { "/mavio.library.v1.MetadataService/\(method)" }
    /// The path of a method of `mavio.playback.v1.PlaybackService`.
    public static func playback(_ method: String) -> String { "/mavio.playback.v1.PlaybackService/\(method)" }
    /// The path of a method of `mavio.assistant.v1.AssistantService`.
    public static func assistant(_ method: String) -> String { "/mavio.assistant.v1.AssistantService/\(method)" }
}

extension AppModel {
    /// Calls a procedure of a server as its session.
    public func call<Response: SwiftProtobuf.Message>(
        _ server: ServerRecord, _ procedure: String, _ request: some SwiftProtobuf.Message,
        as _: Response.Type = Response.self
    ) async throws -> Response {
        try await client.call(server.id, procedure, request, as: Response.self)
    }
}
