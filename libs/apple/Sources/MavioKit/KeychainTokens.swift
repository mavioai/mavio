import Foundation
import MavioCore
import Security

/// Keeps the servers' access tokens in the Keychain, a generic password per
/// server, readable only on this device after its first unlock.
public final class KeychainTokens: SecureTokens {
    private let service: String

    /// Tokens kept under service.
    public init(service: String = "dev.mavio.tokens") {
        self.service = service
    }

    private func query(_ serverId: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: serverId,
        ]
    }

    /// The token of a server, if the device is signed in to it.
    public func token(serverId: String) -> String? {
        var q = query(serverId)
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let data = out as? Data else {
            return nil
        }
        return String(data: data, encoding: .utf8)
    }

    /// Keeps the token of a server, replacing an earlier one.
    public func setToken(serverId: String, token: String) {
        SecItemDelete(query(serverId) as CFDictionary)
        var q = query(serverId)
        q[kSecValueData as String] = Data(token.utf8)
        q[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        SecItemAdd(q as CFDictionary, nil)
    }

    /// Forgets the token of a server.
    public func deleteToken(serverId: String) {
        SecItemDelete(query(serverId) as CFDictionary)
    }
}
