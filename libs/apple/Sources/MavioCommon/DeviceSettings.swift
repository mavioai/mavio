import MavioDesign

/// The theme the app takes on this device.
public enum Appearance: String, CaseIterable, Sendable {
    case system, light, dark

    /// Its name.
    public var label: String {
        switch self {
        case .system: MavioStrings.settingsSystem
        case .light: MavioStrings.settingsLight
        case .dark: MavioStrings.settingsDark
        }
    }
}

/// The keys of this device's settings.
public enum DeviceSetting {
    /// The theme, an `Appearance`.
    public static let appearance = "appearance"
    /// Whether the next episode plays when one ends.
    public static let autoplayNext = "player.autoplayNext"
    /// Whether each video plays in a window of its own, on the Mac, rather
    /// than in place of what plays in the player's window.
    public static let playerWindows = "player.windows"
    /// Whether the Mac shows television mode (docs/tv.md).
    public static let tvMode = "tvMode"
}

/// The most the server sends this device, as the web UI's qualities.
public enum PlaybackQuality: String, CaseIterable, Sendable {
    case original, auto
    case p1080 = "1080p"
    case p720 = "720p"
    case p480 = "480p"

    /// Where this device keeps it.
    public static let key = "player.quality"

    /// The bitrate it limits the stream to: Auto sends more from a server on
    /// the local network than over the internet.
    public func bitrate(local: Bool) -> Int64 {
        switch self {
        case .original: 200_000_000
        case .auto: local ? 100_000_000 : 40_000_000
        case .p1080: 10_000_000
        case .p720: 4_000_000
        case .p480: 1_500_000
        }
    }

    /// Its name.
    public var label: String {
        switch self {
        case .original: MavioStrings.playerQualityOriginal
        case .auto: MavioStrings.playerQualityAuto
        default: rawValue
        }
    }
}
