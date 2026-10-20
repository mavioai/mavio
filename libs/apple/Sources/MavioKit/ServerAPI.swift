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
