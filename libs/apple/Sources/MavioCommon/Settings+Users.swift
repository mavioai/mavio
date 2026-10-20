import MavioKit

extension AppModel {
    /// The users of server the session can switch to; none when the server
    /// does not say.
    public func switchableUsers(_ server: ServerRecord) async -> [SwitchableUser] {
        (try? await client.switchableUsers(serverId: server.id)) ?? []
    }

    /// How the user signed in to server signs in: the password, the
    /// passkeys, the authenticator app and the PIN; nil when the server
    /// does not say.
    public func accountSecurity(_ server: ServerRecord) async -> AccountSecurity? {
        try? await client.security(serverId: server.id)
    }

    /// Whether the user signed in to server administers it, nil if the
    /// server does not say.
    public func administers(_ server: ServerRecord) async -> Bool? {
        let response = try? await client.call(
            server.id, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
            as: Mavio_User_V1_GetCurrentUserResponse.self)
        return response?.user.admin
    }
}
