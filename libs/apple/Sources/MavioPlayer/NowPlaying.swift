import Foundation
import MediaPlayer
import Nuke

/// What the system's Now Playing shows of the media a player plays, on the
/// Lock Screen, in Control Center and on a paired device, and the remote
/// commands it takes from them: play, pause, skips of 10 seconds, a new
/// position, and the next item when there is one. Both engines publish
/// through it, since neither publishes by itself.
@MainActor
public final class NowPlaying {
    /// What it shows.
    public struct Item: Equatable, Sendable {
        /// The title.
        public var title: String
        /// The line under it: the series and episode, or nothing.
        public var subtitle: String?
        /// The artwork, a picture of the item.
        public var artwork: URL?

        /// An item with its title, the line under it and its artwork.
        public init(title: String, subtitle: String? = nil, artwork: URL? = nil) {
            (self.title, self.subtitle, self.artwork) = (title, subtitle, artwork)
        }
    }

    /// How far the remote skips go, in seconds.
    public static let skip: TimeInterval = 10

    private weak var player: Player?
    private var item: Item?
    private var image: PlatformImage?
    private var artworkTask: Task<Void, Never>?
    private var targets: [(MPRemoteCommand, Any)] = []
    /// What it last published of the playback, which a tick republishes
    /// only when it changes: the system runs the clock between.
    private var published: (playing: Bool, rate: Float, duration: TimeInterval)?

    /// The next item, which the next track command plays; the command is
    /// off without one.
    public var next: (() -> Void)? {
        didSet { MPRemoteCommandCenter.shared().nextTrackCommand.isEnabled = next != nil }
    }

    init(player: Player) {
        self.player = player
    }

    /// Shows an item, taking the remote commands while it plays.
    public func show(_ item: Item) {
        if targets.isEmpty { register() }
        if item.artwork != self.item?.artwork {
            image = nil
            artworkTask?.cancel()
            if let url = item.artwork {
                artworkTask = Task { [weak self] in
                    guard let image = try? await ImagePipeline.shared.image(for: url) else { return }
                    guard !Task.isCancelled, let self else { return }
                    self.image = image
                    self.publish()
                }
            }
        }
        self.item = item
        publish()
    }

    /// Republishes the playback when it changed since it last did, or always
    /// when forced, as after a seek.
    func sync(force: Bool = false) {
        guard let player, item != nil else { return }
        let now = (playing: player.isPlaying, rate: player.rate, duration: player.duration)
        if !force, let published, published == now { return }
        publish()
    }

    /// Clears Now Playing and gives the remote commands back.
    func end() {
        artworkTask?.cancel()
        artworkTask = nil
        (item, image, published, next) = (nil, nil, nil, nil)
        for (command, target) in targets { command.removeTarget(target) }
        targets = []
        MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
        #if os(macOS)
            MPNowPlayingInfoCenter.default().playbackState = .stopped
        #endif
    }

    private func publish() {
        guard let player, let item else { return }
        var info: [String: Any] = [
            MPMediaItemPropertyTitle: item.title,
            MPNowPlayingInfoPropertyMediaType: MPNowPlayingInfoMediaType.video.rawValue,
            MPNowPlayingInfoPropertyElapsedPlaybackTime: player.currentTime,
            MPNowPlayingInfoPropertyPlaybackRate: player.isPlaying ? Double(player.rate) : 0,
            MPNowPlayingInfoPropertyDefaultPlaybackRate: Double(player.rate),
        ]
        if let subtitle = item.subtitle, !subtitle.isEmpty { info[MPMediaItemPropertyArtist] = subtitle }
        if player.duration > 0 { info[MPMediaItemPropertyPlaybackDuration] = player.duration }
        if let image {
            info[MPMediaItemPropertyArtwork] = Self.artwork(image)
        }
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info
        #if os(macOS)
            MPNowPlayingInfoCenter.default().playbackState = player.isPlaying ? .playing : .paused
        #endif
        published = (player.isPlaying, player.rate, player.duration)
    }

    /// The artwork of an image, made off the main actor: MediaPlayer asks
    /// for it on a queue of its own.
    nonisolated private static func artwork(_ image: PlatformImage) -> MPMediaItemArtwork {
        MPMediaItemArtwork(boundsSize: image.size) { _ in image }
    }

    private func register() {
        let center = MPRemoteCommandCenter.shared()
        add(center.playCommand) { player, _ in player.play() }
        add(center.pauseCommand) { player, _ in player.pause() }
        add(center.togglePlayPauseCommand) { player, _ in player.toggle() }
        center.skipForwardCommand.preferredIntervals = [NSNumber(value: Self.skip)]
        center.skipBackwardCommand.preferredIntervals = [NSNumber(value: Self.skip)]
        add(center.skipForwardCommand) { player, _ in Task { await player.skip(by: Self.skip) } }
        add(center.skipBackwardCommand) { player, _ in Task { await player.skip(by: -Self.skip) } }
        add(center.changePlaybackPositionCommand) { player, event in
            guard let event = event as? MPChangePlaybackPositionCommandEvent else { return }
            Task { await player.seek(to: event.positionTime) }
        }
        add(center.nextTrackCommand) { [weak self] _, _ in self?.next?() }
        center.nextTrackCommand.isEnabled = next != nil
        center.previousTrackCommand.isEnabled = false
    }

    /// Handles a command with the player, on the main thread, where the
    /// command center calls.
    private func add(_ command: MPRemoteCommand, _ handle: @escaping (Player, MPRemoteCommandEvent) -> Void) {
        command.isEnabled = true
        let target = command.addTarget { [weak self] event in
            MainActor.assumeIsolated {
                guard let player = self?.player else { return .noActionableNowPlayingItem }
                handle(player, event)
                return .success
            }
        }
        targets.append((command, target))
    }
}
