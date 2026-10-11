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
}
