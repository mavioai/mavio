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
