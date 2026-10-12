import Foundation
import MavioCore
import Observation

/// Downloads items for offline playback with a background URLSession, which
/// goes on while the app is suspended; the core keeps the list.
@MainActor
@Observable
public final class Downloads {
    /// The downloads, newest first.
    public private(set) var entries: [DownloadEntry] = []
    @ObservationIgnored private let client: Client
    @ObservationIgnored private let folder: URL
    @ObservationIgnored private let delegate: Delegate
    @ObservationIgnored private let session: URLSession

    /// The downloads of client, their files in folder.
    public init(client: Client, folder: URL) {
        self.client = client
        self.folder = folder
        let (stream, continuation) = AsyncStream<Void>.makeStream(bufferingPolicy: .bufferingNewest(1))
        delegate = Delegate(client: client, folder: folder, changed: continuation)
        let config = URLSessionConfiguration.background(withIdentifier: "dev.mavio.downloads")
        config.sessionSendsLaunchEvents = true
        config.isDiscretionary = false
        session = URLSession(configuration: config, delegate: delegate, delegateQueue: nil)
        reload()
        Task { [weak self] in
            for await _ in stream {
                self?.reload()
                try? await Task.sleep(for: .milliseconds(500))
            }
        }
    }

    /// Reads the list again from the core.
    public func reload() {
        entries = (try? client.downloads()) ?? []
    }

    /// The download of an item.
    public func entry(serverId: String, itemId: String) -> DownloadEntry? {
        entries.first { $0.card.serverId == serverId && $0.card.itemId == itemId }
    }

    /// The downloaded file of an entry, once done.
    public func file(_ entry: DownloadEntry) -> URL? {
        guard entry.state == .done, !entry.file.isEmpty else { return nil }
        return folder.appending(path: entry.file)
    }

    /// Downloads an item as the server makes it for this device.
    public func add(serverId: String, itemId: String) async throws {
        let transfer = try await client.addDownload(
            serverId: serverId, itemId: itemId, capabilities: DeviceProfile.capabilities())
        start(transfer)
        reload()
    }

    /// Resumes a paused or failed download.
    public func resume(_ entry: DownloadEntry) async throws {
        let transfer = try await client.resumeDownload(
            serverId: entry.card.serverId, itemId: entry.card.itemId, capabilities: DeviceProfile.capabilities())
        start(transfer)
        reload()
    }

    /// Cancels a download and deletes its file.
    public func remove(_ entry: DownloadEntry) async {
        let key = Delegate.key(entry.card.serverId, entry.card.itemId)
        for task in await session.allTasks where task.taskDescription == key {
            task.cancel()
        }
        if let file = try? client.removeDownload(serverId: entry.card.serverId, itemId: entry.card.itemId),
            !file.isEmpty
        {
            try? FileManager.default.removeItem(at: folder.appending(path: file))
        }
        reload()
    }

    /// Lets the system know the session's events were handled, when the
    /// app was launched in the background for them.
    public func backgroundEventsHandled(_ completion: @escaping @Sendable () -> Void) {
        delegate.setBackgroundCompletion(completion)
    }

    private func start(_ transfer: Transfer) {
        guard let url = URL(string: transfer.url) else { return }
        let task = session.downloadTask(with: url)
        task.taskDescription = Delegate.key(transfer.serverId, transfer.itemId)
        task.resume()
    }
}

/// The session's delegate, called on its queue: reports progress to the
/// core and moves finished files into the folder.
private final class Delegate: NSObject, URLSessionDownloadDelegate, @unchecked Sendable {
    let client: Client
    let folder: URL
    let changed: AsyncStream<Void>.Continuation
    private let lock = NSLock()
    private var lastReport: [String: Date] = [:]
    private var backgroundCompletion: (@Sendable () -> Void)?

    init(client: Client, folder: URL, changed: AsyncStream<Void>.Continuation) {
        self.client = client
        self.folder = folder
        self.changed = changed
    }

    static func key(_ serverId: String, _ itemId: String) -> String {
        "\(serverId)/\(itemId)"
    }

    private static func ids(_ task: URLSessionTask) -> (String, String)? {
        guard let parts = task.taskDescription?.split(separator: "/", maxSplits: 1), parts.count == 2 else {
            return nil
        }
        return (String(parts[0]), String(parts[1]))
    }

    /// The file name of a download: the item's ID with the extension the
    /// server's response names.
    private static func fileName(_ task: URLSessionTask, itemId: String) -> String {
        let ext =
            (task.response?.suggestedFilename as NSString?)?.pathExtension.nilIfEmpty
            ?? task.originalRequest?.url?.pathExtension.nilIfEmpty
            ?? "mp4"
        return "\(itemId).\(ext)"
    }

    func setBackgroundCompletion(_ completion: @escaping @Sendable () -> Void) {
        lock.withLock { backgroundCompletion = completion }
    }

    func urlSession(
        _ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
        totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64
    ) {
        guard let (serverId, itemId) = Self.ids(downloadTask) else { return }
        let key = Self.key(serverId, itemId)
        let now = Date()
        let due = lock.withLock {
            if let last = lastReport[key], now.timeIntervalSince(last) < 1 { return false }
            lastReport[key] = now
            return true
        }
        guard due else { return }
        try? client.downloadProgress(
            serverId: serverId, itemId: itemId, file: Self.fileName(downloadTask, itemId: itemId),
            received: UInt64(max(0, totalBytesWritten)), total: UInt64(max(0, totalBytesExpectedToWrite)))
        changed.yield()
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
        guard let (serverId, itemId) = Self.ids(downloadTask) else { return }
        if let http = downloadTask.response as? HTTPURLResponse, !(200..<300).contains(http.statusCode) {
            try? client.downloadFailed(serverId: serverId, itemId: itemId, error: "HTTP \(http.statusCode)")
            changed.yield()
            return
        }
        let name = Self.fileName(downloadTask, itemId: itemId)
        let target = folder.appending(path: name)
        do {
            try? FileManager.default.removeItem(at: target)
            try FileManager.default.moveItem(at: location, to: target)
            var values = URLResourceValues()
            values.isExcludedFromBackup = true
            var url = target
            try? url.setResourceValues(values)
            let size = (try? target.resourceValues(forKeys: [.fileSizeKey]).fileSize).map(UInt64.init) ?? 0
            try? client.downloadProgress(serverId: serverId, itemId: itemId, file: name, received: size, total: size)
            try? client.downloadFinished(serverId: serverId, itemId: itemId)
        } catch {
            try? client.downloadFailed(serverId: serverId, itemId: itemId, error: error.localizedDescription)
        }
        changed.yield()
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        guard let error, let (serverId, itemId) = Self.ids(task) else { return }
        if (error as? URLError)?.code == .cancelled {
            try? client.downloadPaused(serverId: serverId, itemId: itemId)
        } else {
            try? client.downloadFailed(serverId: serverId, itemId: itemId, error: error.localizedDescription)
        }
        changed.yield()
    }

    func urlSessionDidFinishEvents(forBackgroundURLSession session: URLSession) {
        let completion = lock.withLock {
            let c = backgroundCompletion
            backgroundCompletion = nil
            return c
        }
        if let completion { DispatchQueue.main.async(execute: completion) }
    }
}

extension String {
    fileprivate var nilIfEmpty: String? { isEmpty ? nil : self }
}
