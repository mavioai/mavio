import Foundation
import MavioDesign
import MavioKit
import MavioPlayer
import Observation

// The player following a SyncPlay group (docs/clients.md §6.3.24), shared by
// the apps' players, and what the apps say of a group.

/// Has a player follow the group while the group plays its item: it waits
/// paused at the group's position until the group plays, reports when it is
/// ready or buffering, carries out the group's commands at their time, and
/// keeps up with the group's position, as the web UI's. Its controls ask
/// the group instead meanwhile.
@MainActor
@Observable
public final class GroupFollower: PlayerGroup {
    /// The group playing the item, nil while the player plays on its own.
    public var group: Mavio_Session_V1_SyncPlayGroup? {
        guard let joined = syncPlay.joined, joined.serverId == serverId, joined.group.plays(itemId) else {
            return nil
        }
        return joined.group
    }

    @ObservationIgnored private let syncPlay: SyncPlay
    @ObservationIgnored private let player: Player
    @ObservationIgnored private let serverId: String
    @ObservationIgnored private let itemId: String
    @ObservationIgnored private let onStop: @MainActor () -> Void
    @ObservationIgnored private var tasks: [Task<Void, Never>] = []
    @ObservationIgnored private var ready: Task<Void, Never>?
    @ObservationIgnored private var timer: Task<Void, Never>?

    /// Follows the group with player, which plays an item of a server;
    /// onStop leaves the player as the group stops.
    public init(
        model: AppModel, player: Player, serverId: String, itemId: String,
        onStop: @escaping @MainActor () -> Void
    ) {
        syncPlay = model.syncPlay
        self.player = player
        self.serverId = serverId
        self.itemId = itemId
        self.onStop = onStop
    }

    /// Starts following, as the player loads the item.
    public func start() {
        stop()
        // The player restarted its stream, or joined a group playing.
        if group?.state == .playing { report(ready: false) }
        let updates = syncPlay.updates()
        tasks.append(
            Task { [weak self] in
                for await u in updates {
                    self?.receive(u)
                }
            })
        tasks.append(Task { [weak self] in await self?.watch() })
    }

    /// Stops following, as the player closes.
    public func stop() {
        for task in tasks { task.cancel() }
        tasks = []
        ready?.cancel()
        timer?.cancel()
        if player.group === self { player.group = nil }
        player.groupRate(1)
    }

    // MARK: Asking the group

    private func send(_ method: String, _ request: some SwiftProtobuf.Message) {
        let (syncPlay, serverId) = (syncPlay, serverId)
        Task { try? await syncPlay.call(serverId, method, request, as: SwiftProtobuf.Google_Protobuf_Empty.self) }
    }

    /// Asks the group to play.
    public func unpause() { send("Unpause", Mavio_Session_V1_UnpauseRequest()) }

    /// Asks the group to pause.
    public func pause() { send("Pause", Mavio_Session_V1_PauseRequest()) }

    /// Asks the group to move to a position, in seconds.
    public func seek(to seconds: TimeInterval) {
        var request = Mavio_Session_V1_SeekRequest()
        request.position = Google_Protobuf_Duration(timeInterval: max(0, seconds))
        send("Seek", request)
    }

    /// Whether the group's queue has an entry after this one.
    public var hasNext: Bool {
        guard let group else { return false }
        return Int(group.currentIndex) < group.queue.count - 1
    }

    /// Plays the group's next entry, else items after it.
    public func next(_ itemIds: [String]) {
        if hasNext {
            send("NextItem", Mavio_Session_V1_NextItemRequest())
        } else if !itemIds.isEmpty {
            var request = Mavio_Session_V1_SetQueueRequest()
            request.itemIds = itemIds
            send("SetQueue", request)
        }
    }

    /// The video ended: the group's first member moves it on.
    private func ended() {
        guard let group, group.members.first?.sessionID == syncPlay.sessionId(serverId) else { return }
        if hasNext {
            send("NextItem", Mavio_Session_V1_NextItemRequest())
        } else {
            send("Stop", Mavio_Session_V1_StopRequest())
        }
    }

    // MARK: Following

    /// Tells the group where the player is, and whether it can play on.
    private func report(ready: Bool) {
        guard let entry = group?.entry else { return }
        var request = Mavio_Session_V1_ReportStateRequest()
        request.entryID = entry.id
        request.position = Google_Protobuf_Duration(timeInterval: max(0, player.currentTime))
        request.when = Google_Protobuf_Timestamp(date: syncPlay.serverNow())
        request.playing = player.isPlaying
        request.ready = ready
        send("ReportState", request)
    }

    /// Pauses at a position and reports ready once the player can play there.
    private func ready(at position: TimeInterval) {
        ready?.cancel()
        ready = Task { [weak self, player] in
            player.groupPause()
            player.groupRate(1)
            if abs(player.currentTime - position) > 0.25 { await player.groupSeek(to: position) }
            while !Task.isCancelled, player.isBuffering || player.duration == 0 {
                try? await Task.sleep(for: .milliseconds(100))
            }
            guard !Task.isCancelled else { return }
            self?.report(ready: true)
        }
    }

    private func receive(_ u: SyncPlay.Update) {
        guard u.serverId == serverId, u.update.hasGroup, u.update.group.entry?.itemID == itemId else { return }
        // An update without a command, as after the stream came back, brings
        // the player to where the group is.
        let update = u.update.hasCommand ? u.update : SyncPlay.caughtUp(u.update.group, at: syncPlay.serverNow())
        guard update.hasCommand else { return }
        let command = update.command
        let position = command.hasPosition ? command.position.timeInterval : 0
        switch command.kind {
        case .pause:
            timer?.cancel()
            ready?.cancel()
            player.groupPause()
            if abs(player.currentTime - position) > 0.5 { Task { await player.groupSeek(to: position) } }
        case .seek:
            timer?.cancel()
            ready(at: position)
        case .stop:
            timer?.cancel()
            onStop()
        case .unpause:
            // An update without its own command catches up only when the
            // player paused while the group plays.
            if !u.update.hasCommand, player.isPlaying { return }
            timer?.cancel()
            ready?.cancel()
            // when is by the server's clock: the player starts then, or
            // catches up when it passed.
            let wait = command.hasWhen ? command.when.date.timeIntervalSince(syncPlay.serverNow()) : 0
            player.groupRate(1)
            timer = Task { [player] in
                if wait > 0 {
                    await player.groupSeek(to: position)
                    try? await Task.sleep(for: .seconds(wait))
                    guard !Task.isCancelled else { return }
                } else {
                    await player.groupSeek(to: position - wait)
                }
                player.groupPlay()
            }
        default:
            break
        }
    }

    /// Hands the controls to the group while it plays the item, gets ready
    /// when the group waits for the device, makes the group wait while the
    /// player buffers, keeps up with the group's position and moves the
    /// group on at the end.
    private func watch() async {
        var waitedAt: TimeInterval?
        var buffering = false
        var ended = false
        var tick = 0
        while !Task.isCancelled {
            let group = self.group
            player.group = group == nil ? nil : self
            if let group, let joined = syncPlay.joined {
                // A member the group waits for gets ready at its position.
                let position = group.hasPosition ? group.position.timeInterval : 0
                if group.state == .waiting, syncPlay.me(joined)?.buffering == true {
                    if waitedAt != position {
                        waitedAt = position
                        ready(at: position)
                    }
                } else {
                    waitedAt = nil
                }
                // Buffering while the group plays makes the group wait.
                if group.state == .playing, player.isBuffering, !buffering { report(ready: false) }
                if player.ended, !ended { self.ended() }
                if group.state == .playing, tick % 4 == 0 { keepUp(group) }
                if group.state != .playing, player.rate != 1 { player.groupRate(1) }
            }
            buffering = player.isBuffering
            ended = player.ended
            tick += 1
            try? await Task.sleep(for: .milliseconds(250))
        }
    }

    /// Keeps up with the group: far behind or ahead it skips, near it speeds
    /// up or slows down a little.
    private func keepUp(_ group: Mavio_Session_V1_SyncPlayGroup) {
        guard player.isPlaying, !player.isBuffering else { return }
        let off = player.currentTime - group.position(at: syncPlay.serverNow())
        if abs(off) > 2 {
            player.groupRate(1)
            Task { [player] in await player.groupSeek(to: max(0, player.currentTime - off)) }
        } else {
            player.groupRate(abs(off) < 0.05 ? 1 : Float(1 - min(0.5, max(-0.5, off)) / 5))
        }
    }
}

/// Who a group waits for.
public func groupWaitingFor(_ group: Mavio_Session_V1_SyncPlayGroup) -> String {
    MavioStrings.syncplayWaitingFor(
        names: group.members.filter(\.buffering).map { $0.deviceName.isEmpty ? $0.userName : $0.deviceName }
            .joined(separator: ", "))
}

/// A group's name and size.
public func groupChip(_ group: Mavio_Session_V1_SyncPlayGroup) -> String {
    MavioStrings.syncplayChip(name: group.name, count: "\(group.members.count)")
}

/// What a group does, with the name of its entry.
public func groupState(_ group: Mavio_Session_V1_SyncPlayGroup, entryName: String) -> String {
    switch group.state {
    case .waiting: groupWaitingFor(group)
    case .paused: MavioStrings.syncplayPaused(name: entryName)
    case .playing: MavioStrings.syncplayPlaying(name: entryName)
    default: MavioStrings.syncplayIdle
    }
}

extension AppModel {
    /// The groups on a server, to join.
    public func syncPlayGroups(_ serverId: String) async throws -> [Mavio_Session_V1_SyncPlayGroup] {
        try await syncPlay.call(
            serverId, "ListGroups", Mavio_Session_V1_ListGroupsRequest(),
            as: Mavio_Session_V1_ListGroupsResponse.self
        ).groups
    }

    /// Creates a group on a server, which the device joins.
    public func createSyncPlayGroup(_ serverId: String, name: String) async throws {
        var request = Mavio_Session_V1_CreateGroupRequest()
        request.name = name
        let r = try await syncPlay.call(serverId, "CreateGroup", request, as: Mavio_Session_V1_CreateGroupResponse.self)
        syncPlay.enter(serverId, r.group)
    }

    /// Joins a group of a server.
    public func joinSyncPlayGroup(_ serverId: String, groupId: String) async throws {
        var request = Mavio_Session_V1_JoinGroupRequest()
        request.groupID = groupId
        let r = try await syncPlay.call(serverId, "JoinGroup", request, as: Mavio_Session_V1_JoinGroupResponse.self)
        syncPlay.enter(serverId, r.group)
    }

    /// Plays an item in the group from a position.
    public func playInGroup(_ serverId: String, itemId: String, from position: TimeInterval) async throws {
        var request = Mavio_Session_V1_SetQueueRequest()
        request.itemIds = [itemId]
        if position > 0 { request.startPosition = Google_Protobuf_Duration(timeInterval: position) }
        try await syncPlay.call(serverId, "SetQueue", request, as: Mavio_Session_V1_SetQueueResponse.self)
    }
}

extension Router {
    /// Opens the player on what the device's group plays, in the place of
    /// the player open, as an update has the group wait for the device on
    /// an entry the player does not show; until the apps close.
    public func follow(_ syncPlay: SyncPlay) async {
        for await u in syncPlay.updates() {
            if let request = groupPlayback(
                u, syncPlay: syncPlay, playing: playing.map { ($0.serverId, $0.itemId) })
            {
                playing = request
            }
        }
    }
}

/// The playback to open as an update has the group wait for the device on
/// an entry other than the one playing, nil when there is none.
@MainActor public func groupPlayback(
    _ u: SyncPlay.Update, syncPlay: SyncPlay, playing: (serverId: String, itemId: String)?
) -> PlayRequest? {
    let group = u.update.group
    guard u.update.hasGroup, group.state != .idle, let entry = group.entry,
        let joined = syncPlay.joined, joined.serverId == u.serverId, syncPlay.me(joined)?.buffering == true,
        playing?.serverId != u.serverId || playing?.itemId != entry.itemID
    else { return nil }
    let start = group.hasPosition ? group.position.timeInterval : 0
    return PlayRequest(serverId: u.serverId, itemId: entry.itemID, title: "", start: start > 0 ? start : nil)
}
