import AVFoundation
import AVKit
import AetherEngine
import Observation

/// Plays media on one of the two engines behind one interface, which the
/// apps' controls drive.
@MainActor
@Observable
public final class Player {
    /// The engine playing, unset before a load.
    public private(set) var engine: PlayerEngine?
    /// AVPlayer, when it plays.
    public private(set) var avPlayer: AVPlayer?
    /// AetherEngine, when it plays.
    public private(set) var aether: AetherEngine?
    /// The position, in seconds.
    public private(set) var currentTime: TimeInterval = 0
    /// The duration, in seconds; 0 until known.
    public private(set) var duration: TimeInterval = 0
    /// Whether it plays.
    public private(set) var isPlaying = false
    /// Whether it waits for data.
    public private(set) var isBuffering = false
    /// Whether it played to the end.
    public private(set) var ended = false
    /// Why it failed.
    public private(set) var failure: String?
    /// The speed.
    public private(set) var rate: Float = 1
    /// AetherEngine's subtitle cues, which the overlay draws.
    public private(set) var cues: [SubtitleCue] = []
    /// The source time the cues are drawn against.
    public private(set) var cueTime: TimeInterval = 0
    /// AetherEngine's picture size, in square pixels; zero until known.
    public private(set) var videoSize: CGSize = .zero
    /// Picture in Picture of AVPlayer's layer, once the surface made it.
    public var pictureInPicture: AVPictureInPictureController?

    @ObservationIgnored private var poll: Task<Void, Never>?
    @ObservationIgnored private var endObserver: NSObjectProtocol?

    /// A player with nothing loaded.
    public init() {}

    /// Plays media, replacing what played.
    public func load(_ media: Media) async {
        teardown()
        ended = false
        failure = nil
        currentTime = media.start
        duration = 0
        isBuffering = true
        switch media.engine {
        case .avPlayer: await loadAVPlayer(media)
        case .aether: await loadAether(media)
        }
        startPolling()
    }

    private func loadAVPlayer(_ media: Media) async {
        #if os(iOS) || os(tvOS)
            try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .moviePlayback)
            try? AVAudioSession.sharedInstance().setActive(true)
        #endif
        let item = AVPlayerItem(url: media.url)
        let player = AVPlayer(playerItem: item)
        player.allowsExternalPlayback = true
        player.defaultRate = rate
        avPlayer = player
        engine = .avPlayer
        endObserver = NotificationCenter.default.addObserver(
            forName: AVPlayerItem.didPlayToEndTimeNotification, object: item, queue: .main
        ) { [weak self] _ in
            MainActor.assumeIsolated {
                self?.ended = true
                self?.isPlaying = false
            }
        }
        if media.start > 0 {
            await player.seek(
                to: CMTime(seconds: media.start, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero
            )
        }
        player.play()
        isPlaying = true
        await selectRendition(media.subtitleRendition)
    }

    /// Shows the HLS subtitle rendition in a language, the first when none
    /// matches, or none.
    private func selectRendition(_ language: String?) async {
        guard let item = avPlayer?.currentItem,
            let group = try? await item.asset.loadMediaSelectionGroup(for: .legible)
        else { return }
        guard let language else {
            item.select(nil, in: group)
            return
        }
        let option =
            group.options.first { $0.extendedLanguageTag?.lowercased() == language.lowercased() }
            ?? group.options.first
        item.select(option, in: group)
    }

    private func loadAether(_ media: Media) async {
        do {
            let engine = try AetherEngine()
            aether = engine
            self.engine = .aether
            _ = try await engine.load(
                url: media.url,
                startPosition: media.start > 0 ? media.start : nil,
                audioSourceStreamIndex: media.audioStream.map(Int32.init)
            )
            if let file = media.subtitleFile {
                let track = engine.addExternalSubtitleTrack(
                    ExternalSubtitleTrack(url: file.url, name: file.title, language: file.language))
                engine.selectSubtitleTrack(index: track.id)
            } else if let stream = media.subtitleStream {
                engine.selectSubtitleTrack(index: stream)
            }
            engine.setRate(rate)
            engine.play()
            isPlaying = true
        } catch is CancellationError {
        } catch {
            failure = error.localizedDescription
        }
    }

    private func startPolling() {
        poll = Task { [weak self] in
            while !Task.isCancelled {
                self?.tick()
                try? await Task.sleep(for: .milliseconds(200))
            }
        }
    }

    private func tick() {
        if let player = avPlayer, let item = player.currentItem {
            let t = player.currentTime().seconds
            if t.isFinite, !isSeeking { currentTime = t }
            let d = item.duration.seconds
            if d.isFinite, d > 0 { duration = d }
            isPlaying = player.timeControlStatus != .paused
            isBuffering = player.timeControlStatus == .waitingToPlayAtSpecifiedRate || item.status == .unknown
            if item.status == .failed, failure == nil {
                failure = item.error?.localizedDescription ?? "AVPlayer failed"
            }
        } else if let engine = aether {
            if !isSeeking { currentTime = engine.clock.currentTime }
            duration = engine.duration
            cues = engine.subtitleCues
            cueTime = engine.clock.sourceTime
            let size = CGSize(
                width: Double(engine.sourceVideoWidth) * engine.sourceVideoPixelAspectRatio,
                height: Double(engine.sourceVideoHeight))
            if size != videoSize { videoSize = size }
            switch engine.state {
            case .playing: isPlaying = true
            case .ended:
                isPlaying = false
                ended = true
            case let .error(message):
                isPlaying = false
                if failure == nil { failure = message }
            default: isPlaying = false
            }
            isBuffering = engine.isBuffering || engine.state == .loading
        }
    }

    @ObservationIgnored private var isSeeking = false

    /// Plays.
    public func play() {
        avPlayer?.play()
        aether?.play()
        isPlaying = true
    }

    /// Pauses.
    public func pause() {
        avPlayer?.pause()
        aether?.pause()
        isPlaying = false
    }

    /// Plays or pauses.
    public func toggle() {
        isPlaying ? pause() : play()
    }

    /// Moves to a position, in seconds.
    public func seek(to seconds: TimeInterval) async {
        let target = max(0, duration > 0 ? min(seconds, duration) : seconds)
        currentTime = target
        isSeeking = true
        defer { isSeeking = false }
        if let player = avPlayer {
            await player.seek(to: CMTime(seconds: target, preferredTimescale: 600))
        }
        if let engine = aether {
            await engine.seek(to: target)
        }
    }

    /// Moves by an offset, in seconds.
    public func skip(by seconds: TimeInterval) async {
        await seek(to: currentTime + seconds)
    }

    /// Plays at a speed.
    public func setRate(_ rate: Float) {
        self.rate = rate
        if let player = avPlayer {
            player.defaultRate = rate
            if isPlaying { player.rate = rate }
        }
        aether?.setRate(rate)
    }

    /// Whether Picture in Picture can start.
    public var canPictureInPicture: Bool {
        AVPictureInPictureController.isPictureInPictureSupported() && pictureInPicture != nil
    }

    /// Starts or stops Picture in Picture.
    public func togglePictureInPicture() {
        guard let pip = pictureInPicture else { return }
        if pip.isPictureInPictureActive {
            pip.stopPictureInPicture()
        } else {
            pip.startPictureInPicture()
        }
    }

    /// Stops playing and lets the engine go.
    public func stop() {
        teardown()
    }

    private func teardown() {
        poll?.cancel()
        poll = nil
        if let endObserver { NotificationCenter.default.removeObserver(endObserver) }
        endObserver = nil
        avPlayer?.pause()
        avPlayer?.replaceCurrentItem(with: nil)
        avPlayer = nil
        pictureInPicture = nil
        aether?.stop()
        aether = nil
        engine = nil
        cues = []
        videoSize = .zero
        isPlaying = false
    }
}
