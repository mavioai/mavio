import Foundation
import MavioCore

/// Keeps the core's secrets in a JSON file readable only by the user, for a
/// development build of the Mac app: signed anew with each build, it would
/// have the Keychain ask for the login password at each launch.
public final class FileTokens: SecureTokens, @unchecked Sendable {
    private let url: URL
    private let lock = NSLock()
    private var values: [String: String]

    /// Secrets kept in the file at url, which is made when first written.
    public init(url: URL) {
        self.url = url
        values = (try? JSONDecoder().decode([String: String].self, from: Data(contentsOf: url))) ?? [:]
    }

    /// What is kept under an ID, if anything.
    public func token(serverId: String) -> String? {
        lock.withLock { values[serverId] }
    }

    /// Keeps a value under an ID, replacing an earlier one.
    public func setToken(serverId: String, token: String) {
        lock.withLock {
            values[serverId] = token
            save()
        }
    }

    /// Forgets what is kept under an ID.
    public func deleteToken(serverId: String) {
        lock.withLock {
            values[serverId] = nil
            save()
        }
    }

    private func save() {
        guard let data = try? JSONEncoder().encode(values) else { return }
        let path = url.path(percentEncoded: false)
        try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        if FileManager.default.createFile(atPath: path, contents: data, attributes: [.posixPermissions: 0o600]) {
            try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: path)
        }
    }
}
