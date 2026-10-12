/// One player interface over two engines, AVPlayer and AetherEngine, with
/// Picture in Picture.
import AetherEngine
import AVFoundation

/// The engine that plays a source.
public enum PlayerEngine: Sendable {
    /// AVPlayer, for what AVFoundation plays and every HLS stream of the
    /// server.
    case avPlayer
    /// AetherEngine, for what AVFoundation does not play.
    case aether

    /// The engine of a playback (docs/clients.md §4.2): HLS goes to AVPlayer;
    /// a file played directly goes to AVPlayer when AVFoundation plays it as
    /// it is, else to AetherEngine, which also draws the subtitles a file
    /// carries or comes with and plays an audio stream other than its
    /// default: selectsTracks.
    public static func choose(
        hls: Bool,
        container: String?,
        audioCodec: String?,
        selectsTracks: Bool,
        avPlayerContainers: Set<String>,
        avPlayerAudioCodecs: Set<String>
    ) -> PlayerEngine {
        if hls { return .avPlayer }
        if selectsTracks { return .aether }
        guard let container,
            container.split(separator: ",").contains(where: { avPlayerContainers.contains(String($0)) })
        else { return .aether }
        if let audioCodec, !avPlayerAudioCodecs.contains(audioCodec) { return .aether }
        return .avPlayer
    }
}

/// A subtitle file delivered on its own.
public struct ExternalSubtitle: Sendable, Equatable {
    /// The file.
    public var url: URL
    /// Its language.
    public var language: String?
    /// Its title.
    public var title: String?

    /// A subtitle file.
    public init(url: URL, language: String?, title: String?) {
        self.url = url
        self.language = language
        self.title = title
    }
}

/// What to play and how.
public struct Media: Sendable {
    /// The file or playlist.
    public var url: URL
    /// The engine.
    public var engine: PlayerEngine
    /// Where to start, in seconds.
    public var start: TimeInterval
    /// The audio stream of a file played directly, by its index in the file.
    public var audioStream: Int?
    /// The embedded subtitle stream to draw, by its index in the file.
    public var subtitleStream: Int?
    /// The subtitle file to draw.
    public var subtitleFile: ExternalSubtitle?
    /// The language of the HLS subtitle rendition to show.
    public var subtitleRendition: String?

    /// Media to play.
    public init(
        url: URL,
        engine: PlayerEngine,
        start: TimeInterval = 0,
        audioStream: Int? = nil,
        subtitleStream: Int? = nil,
        subtitleFile: ExternalSubtitle? = nil,
        subtitleRendition: String? = nil
    ) {
        self.url = url
        self.engine = engine
        self.start = start
        self.audioStream = audioStream
        self.subtitleStream = subtitleStream
        self.subtitleFile = subtitleFile
        self.subtitleRendition = subtitleRendition
    }
}
