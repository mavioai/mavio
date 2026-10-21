import Foundation
import Observation

/// SyncPlay (docs/clients.md §6.3.24): the group the device is in, kept from
/// its servers' events, with the clock kept in step with the group's server,
/// as the web UI's.
@MainActor
@Observable
public final class SyncPlay {
    /// The group the device is in, on its server.
    public struct Joined: Equatable, Sendable {
        /// The group's server.
        public let serverId: String
        /// The group.
        public let group: Mavio_Session_V1_SyncPlayGroup
    }

    /// An update of a server's group, with the command it carries out.
    public struct Update: Sendable {
        /// The group's server.
        public let serverId: String
        /// The update.
        public let update: Mavio_Session_V1_SyncPlayUpdate
    }

    /// The group the device is in.
    public private(set) var joined: Joined?

    @ObservationIgnored private let client: Client
    @ObservationIgnored private var sessions: [String: String] = [:]
    @ObservationIgnored private var offset: TimeInterval = 0
    @ObservationIgnored private var clock: Task<Void, Never>?
    @ObservationIgnored private var listeners: [UUID: AsyncStream<Update>.Continuation] = [:]

    init(client: Client) {
        self.client = client
    }

    /// The groups' updates, as they come, for the player that follows the
    /// group.
    public func updates() -> AsyncStream<Update> {
        let (stream, continuation) = AsyncStream<Update>.makeStream(bufferingPolicy: .bufferingNewest(16))
        let id = UUID()
        listeners[id] = continuation
        continuation.onTermination = { [weak self] _ in
            Task { @MainActor in self?.listeners[id] = nil }
        }
        return stream
    }

    /// The ID of the device's session on a server, which names it among a
    /// group's members.
    public func sessionId(_ serverId: String) -> String? { sessions[serverId] }

    /// The device among a group's members.
    public func me(_ joined: Joined) -> Mavio_Session_V1_SyncPlayMember? {
        joined.group.members.first { $0.sessionID == sessions[joined.serverId] }
    }

    /// The time by the group's server's clock.
    public func serverNow() -> Date { Date().addingTimeInterval(offset) }

    /// Calls a procedure of the SyncPlay service.
    @discardableResult
    public func call<Response: SwiftProtobuf.Message>(
        _ serverId: String, _ method: String, _ request: some SwiftProtobuf.Message,
        as _: Response.Type = Response.self
    ) async throws -> Response {
        try await client.call(serverId, Procedure.syncPlay(method), request, as: Response.self)
    }

    /// Takes the group a request to join or create answered with.
    public func enter(_ serverId: String, _ group: Mavio_Session_V1_SyncPlayGroup) {
        set(Joined(serverId: serverId, group: group))
    }

    /// Leaves the group.
    public func leave() async throws {
        guard let joined else { return }
        try await call(
            joined.serverId, "LeaveGroup", Mavio_Session_V1_LeaveGroupRequest(),
            as: Mavio_Session_V1_LeaveGroupResponse.self)
        set(nil)
    }

    func connected(_ serverId: String, _ sessionId: String) {
        let again = sessions.updateValue(sessionId, forKey: serverId) != nil
        if again, let joined, joined.serverId == serverId {
            Task { try? await resume(joined) }
        }
    }

    /// Takes the group up again after the server's stream came back: the
    /// server drops a device whose stream goes away, and the device misses
    /// the group's updates meanwhile. A device dropped joins again, which
    /// makes the group wait for it; one still in the group carries out where
    /// the group is now.
    private func resume(_ was: Joined) async throws {
        let serverId = was.serverId
        let groups = try await call(
            serverId, "ListGroups", Mavio_Session_V1_ListGroupsRequest(),
            as: Mavio_Session_V1_ListGroupsResponse.self)
        guard joined?.group.id == was.group.id else { return }
        guard let group = groups.groups.first(where: { $0.id == was.group.id }) else {
            return update(serverId, Mavio_Session_V1_SyncPlayUpdate())
        }
        if group.members.contains(where: { $0.sessionID == sessions[serverId] }) {
            update(serverId, Self.caughtUp(group, at: serverNow()))
        } else {
            var request = Mavio_Session_V1_JoinGroupRequest()
            request.groupID = group.id
            let joined = try await call(
                serverId, "JoinGroup", request, as: Mavio_Session_V1_JoinGroupResponse.self)
            enter(serverId, joined.group)
        }
    }

    func update(_ serverId: String, _ update: Mavio_Session_V1_SyncPlayUpdate) {
        if update.hasGroup {
            set(Joined(serverId: serverId, group: update.group))
        } else if joined?.serverId == serverId {
            set(nil)
        }
        for listener in listeners.values { listener.yield(Update(serverId: serverId, update: update)) }
    }

    private func set(_ joined: Joined?) {
        let was = self.joined?.serverId
        self.joined = joined
        guard joined?.serverId != was else { return }
        clock?.cancel()
        clock = joined.map { j in Task { await measure(j.serverId) } }
    }

    /// Measures the server's clock as the group starts and every half
    /// minute, and tells the group the round trip.
    private func measure(_ serverId: String) async {
        while !Task.isCancelled {
            var best: (offset: TimeInterval, rtt: TimeInterval)?
            for _ in 0..<4 {
                let t0 = Date()
                guard
                    let r = try? await call(
                        serverId, "GetTime", Mavio_Session_V1_GetTimeRequest(),
                        as: Mavio_Session_V1_GetTimeResponse.self),
                    r.hasReceiveTime, r.hasSendTime
                else { continue }
                let s = Self.clockSample(t0, r.receiveTime.date, r.sendTime.date, Date())
                if best == nil || s.rtt < best!.rtt { best = s }
            }
            if let best {
                offset = best.offset
                var request = Mavio_Session_V1_ReportPingRequest()
                request.ping = Google_Protobuf_Duration(timeInterval: min(best.rtt, 60))
                _ = try? await call(serverId, "ReportPing", request, as: Mavio_Session_V1_ReportPingResponse.self)
            }
            try? await Task.sleep(for: .seconds(30))
        }
    }

    /// The update that brings a player to where a group is at a time of the
    /// server's clock.
    public nonisolated static func caughtUp(_ group: Mavio_Session_V1_SyncPlayGroup, at now: Date)
        -> Mavio_Session_V1_SyncPlayUpdate
    {
        var update = Mavio_Session_V1_SyncPlayUpdate()
        update.group = group
        let kind: Mavio_Session_V1_SyncPlayCommandKind? =
            switch group.state {
            case .playing: .unpause
            case .paused: .pause
            default: nil
            }
        if let kind {
            var command = Mavio_Session_V1_SyncPlayCommand()
            command.kind = kind
            command.position = Google_Protobuf_Duration(timeInterval: group.position(at: now))
            command.when = Google_Protobuf_Timestamp(date: now)
            if let entry = group.entry { command.entryID = entry.id }
            update.command = command
        }
        return update
    }

    /// The offset and round trip of an exchange sent at t0 and answered at
    /// t3 by the device's clock, which the server received at t1 and
    /// answered at t2 by its own.
    public nonisolated static func clockSample(_ t0: Date, _ t1: Date, _ t2: Date, _ t3: Date)
        -> (offset: TimeInterval, rtt: TimeInterval)
    {
        let offset = (t1.timeIntervalSince(t0) + t2.timeIntervalSince(t3)) / 2
        let rtt = max(0, t3.timeIntervalSince(t0) - t2.timeIntervalSince(t1))
        return (offset, rtt)
    }
}

extension Mavio_Session_V1_SyncPlayGroup {
    /// Where the group is at a time of the server's clock, in seconds:
    /// advancing from its position while it plays.
    public func position(at now: Date) -> TimeInterval {
        let position = hasPosition ? self.position.timeInterval : 0
        let at = hasPositionTime ? positionTime.date : now
        return state == .playing && now > at ? position + now.timeIntervalSince(at) : position
    }

    /// The group's current entry.
    public var entry: Mavio_Session_V1_SyncPlayQueueEntry? {
        queue.indices.contains(Int(currentIndex)) ? queue[Int(currentIndex)] : nil
    }

    /// Whether the group plays an item of a server, as a player of it
    /// follows the group.
    public func plays(_ itemId: String) -> Bool {
        state != .idle && entry?.itemID == itemId
    }
}
