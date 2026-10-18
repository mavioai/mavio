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
    /// The renderer of the ASS track AetherEngine shows, which draws it in
    /// its styles with the cues' event lines added; nil for other tracks,
    /// whose cues the overlay draws itself.
    public private(set) var styledSubtitles: ASSRenderer?
    /// The picture at the position AetherEngine moved to while paused, which
    /// its display shows only once it plays again; nil otherwise.
    public private(set) var still: CGImage?
    /// AetherEngine's picture size, in square pixels; zero until known.
    public private(set) var videoSize: CGSize = .zero
    /// Picture in Picture of AVPlayer's layer, once the surface made it.
    public var pictureInPicture: AVPictureInPictureController?
    /// Now Playing and the remote commands, which the app gives the item.
    @ObservationIgnored public private(set) lazy var nowPlaying = NowPlaying(player: self)

    @ObservationIgnored private var poll: Task<Void, Never>?
    #if os(iOS)
        /// Picture in Picture while AetherEngine plays.
        @ObservationIgnored private var aetherPictureInPicture: AetherPictureInPicture?
    #endif
    @ObservationIgnored private var endObserver: NSObjectProtocol?
    /// The file AVPlayer plays directly, whose pictures the scrubber shows.
    @ObservationIgnored private var thumbnails: Thumbnails?
    /// The still being fetched after a paused seek.
    @ObservationIgnored private var stillTask: Task<Void, Never>?
    /// The script header the styled renderer was made for.
    @ObservationIgnored private var styledHeader: String?
    /// The cues the styled renderer has, by their ID and start.
    @ObservationIgnored private var styledCues: Set<StyledCue> = []

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
        #if os(iOS) || os(tvOS)
            try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .moviePlayback)
            try? AVAudioSession.sharedInstance().setActive(true)
        #endif
        switch media.engine {
        case .avPlayer: await loadAVPlayer(media)
        case .aether: await loadAether(media)
        }
        startPolling()
    }

    private func loadAVPlayer(_ media: Media) async {
        let item = AVPlayerItem(url: media.url)
        let player = AVPlayer(playerItem: item)
        player.allowsExternalPlayback = true
        player.defaultRate = rate
        avPlayer = player
        engine = .avPlayer
        if media.url.pathExtension.lowercased() != "m3u8" { thumbnails = Thumbnails(asset: item.asset) }
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
                // ASS tracks keep their event lines, which libass draws.
                options: LoadOptions(preserveASSMarkup: true),
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
                self?.nowPlaying.sync()
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
            updateStyledSubtitles(engine)
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
            #if os(iOS)
                // A session's layer, once it has one, takes Picture in Picture.
                let layer = AetherPictureInPicture.layer(of: engine)
                if layer !== aetherPictureInPicture?.layer {
                    aetherPictureInPicture = layer == nil ? nil : AetherPictureInPicture(engine: engine)
                    pictureInPicture = aetherPictureInPicture?.controller
                }
            #endif
        }
    }

    @ObservationIgnored private var isSeeking = false

    /// Plays.
    public func play() {
        clearStill()
        avPlayer?.play()
        aether?.play()
        isPlaying = true
        #if os(iOS)
            aetherPictureInPicture?.invalidate()
        #endif
        nowPlaying.sync()
    }

    /// Pauses.
    public func pause() {
        avPlayer?.pause()
        aether?.pause()
        isPlaying = false
        #if os(iOS)
            aetherPictureInPicture?.invalidate()
        #endif
        nowPlaying.sync()
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
            clearStill()
            await engine.seek(to: target)
            if !isPlaying { showStill(of: engine, at: target) }
        }
        nowPlaying.sync(force: true)
    }

    /// A small picture of a position, for the scrubber: AetherEngine's from
    /// what it has read, or a file's that AVPlayer plays directly; nil for
    /// HLS on AVPlayer and while none is at hand.
    public func thumbnail(at seconds: TimeInterval) async -> CGImage? {
        if let engine = aether {
            return await engine.scrubThumbnail(atSeconds: seconds, maxWidth: 320)
        }
        return await thumbnails?.image(at: seconds)
    }

    /// Shows the frame at a position AetherEngine moved to while paused, which
    /// its software path draws only once it plays.
    private func showStill(of engine: AetherEngine, at seconds: TimeInterval) {
        let width = max(Int(videoSize.width), 640)
        stillTask = Task { [weak self] in
            let frame = await engine.scrubPreviewFrame(atSeconds: seconds, refined: true, maxWidth: width) {
                false
            }
            guard !Task.isCancelled, let self, !self.isPlaying, let frame else { return }
            self.still = frame.image
        }
    }

    private func clearStill() {
        stillTask?.cancel()
        stillTask = nil
        still = nil
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
        nowPlaying.sync()
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
        nowPlaying.end()
    }

    /// Makes the styled renderer of the track AetherEngine shows when it is
    /// ASS, with the fonts the file attaches, and adds the cues it lacks.
    private func updateStyledSubtitles(_ engine: AetherEngine) {
        // An embedded track carries its header; an external one's is the
        // sidecar's, which the engine may keep after another track is chosen.
        let track = engine.activeSubtitleTrackIndex.flatMap { index in
            engine.subtitleTracks.first { $0.id == index }
        }
        let header =
            !engine.isSubtitleActive
            ? nil
            : track.map { $0.isExternal ? engine.sidecarASSHeader : $0.assHeader } ?? engine.sidecarASSHeader
        if header != styledHeader {
            styledHeader = header
            styledCues = []
            let fonts = Dictionary(
                engine.fontAttachments.map { ($0.filename, $0.data) }, uniquingKeysWith: { first, _ in first })
            styledSubtitles = header.flatMap { ASSRenderer(header: $0, fonts: fonts) }
        }
        guard let styledSubtitles else { return }
        for cue in cues where styledCues.insert(StyledCue(id: cue.id, start: cue.startTime)).inserted {
            styledSubtitles.add(cue: cue)
        }
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
        #if os(iOS)
            aetherPictureInPicture = nil
        #endif
        aether?.stop()
        aether = nil
        engine = nil
        cues = []
        clearStill()
        thumbnails = nil
        (styledSubtitles, styledHeader, styledCues) = (nil, nil, [])
        videoSize = .zero
        isPlaying = false
    }
}

/// A cue the styled renderer has: its ID, which a new decoder may give
/// again, with its start.
private struct StyledCue: Hashable {
    let id: Int
    let start: TimeInterval
}

/// The pictures of a file AVPlayer plays directly, for the scrubber, within
/// two seconds of where they are asked for. The generator answers on a
/// queue of its own, so one is shared across requests.
private final class Thumbnails: @unchecked Sendable {
    private let generator: AVAssetImageGenerator

    init(asset: AVAsset) {
        generator = AVAssetImageGenerator(asset: asset)
        generator.appliesPreferredTrackTransform = true
        generator.maximumSize = CGSize(width: 320, height: 320)
        let tolerance = CMTime(seconds: 2, preferredTimescale: 600)
        (generator.requestedTimeToleranceBefore, generator.requestedTimeToleranceAfter) = (tolerance, tolerance)
    }

    func image(at seconds: TimeInterval) async -> CGImage? {
        try? await generator.image(at: CMTime(seconds: seconds, preferredTimescale: 600)).image
    }
}
