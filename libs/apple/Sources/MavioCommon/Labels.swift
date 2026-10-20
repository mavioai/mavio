import Foundation
import MavioDesign
import MavioKit

extension CreditJob {
    /// Whether the person acts.
    public var isCast: Bool { self == .actor || self == .guestStar }

    /// The job's name as the crew lists it, as the web UI's.
    public var label: String {
        switch self {
        case .actor: MavioStrings.creditsCast
        case .guestStar: MavioStrings.creditsGuestStars
        case .creator: MavioStrings.itemCreators
        case .director: MavioStrings.itemDirector
        case .writer: MavioStrings.itemWriters
        case .author: MavioStrings.creditsAuthors
        case .producer: MavioStrings.itemProducers
        case .composer: MavioStrings.itemMusic
        case .conductor: MavioStrings.creditsConductors
        case .lyricist: MavioStrings.creditsLyricists
        case .artist: MavioStrings.creditsArtists
        case .narrator: MavioStrings.creditsNarrators
        case .other: MavioStrings.creditsOther
        }
    }

    /// The crew's jobs in the order the credits list them.
    public static let crewOrder: [CreditJob] = [
        .creator, .director, .writer, .author, .producer, .composer, .conductor, .lyricist, .artist, .narrator, .other,
    ]
}

extension CreditEntry {
    /// What a card says the person did: the character played, else the job.
    public var shownRole: String? {
        if job.isCast { return role }
        return job == .other ? role ?? job.label : job.label
    }
}

extension ExtraKind {
    /// What the extra is, as its card says.
    public var label: String {
        switch self {
        case .trailer: MavioStrings.extraTrailer
        case .clip: MavioStrings.extraClip
        case .behindTheScenes: MavioStrings.extraBehindTheScenes
        case .deletedScene: MavioStrings.extraDeletedScene
        case .interview: MavioStrings.extraInterview
        case .scene: MavioStrings.extraScene
        case .sample: MavioStrings.extraSample
        case .featurette: MavioStrings.extraFeaturette
        case .short: MavioStrings.extraShort
        case .other: MavioStrings.extraOther
        }
    }
}

extension Source {
    /// The resolution of the video, as "4K" or "1080p".
    public var resolution: String? {
        guard let v = video, v.height > 0 else { return nil }
        if v.width >= 3200 || v.height >= 2000 { return "4K" }
        if v.width >= 1800 || v.height >= 1000 { return "1080p" }
        if v.width >= 1200 || v.height >= 700 { return "720p" }
        return "\(v.height)p"
    }

    /// The dynamic range of the video when it is HDR.
    public var range: String? {
        switch video?.range {
        case .hdr10?: "HDR10"
        case .hdr10Plus?: "HDR10+"
        case .hlg?: "HLG"
        case .dolbyVision?: "Dolby Vision"
        default: nil
        }
    }

    /// The most channels of the audio, as "5.1" or "7.1", when it is surround.
    public var surround: String? {
        let most = audio.map(\.channels).max() ?? 0
        return most >= 8 ? "7.1" : most >= 6 ? "5.1" : nil
    }

    /// The container, by the file's extension, else the first format the
    /// probe named.
    public var format: String? {
        if let ext = fileName.flatMap({ URL(fileURLWithPath: $0).pathExtension }), !ext.isEmpty {
            return ext.uppercased()
        }
        return container?.split(separator: ",").first.map { $0.uppercased() }
    }

    /// The version's name: its own, else its resolution and range, else its
    /// container.
    public var versionName: String {
        if let name { return name }
        let parts = [resolution, range].compactMap(\.self)
        return parts.isEmpty ? format ?? serverName : parts.joined(separator: " ")
    }

    /// The size, as "4.2 GB".
    public var sizeText: String? {
        size.map { ByteCountFormatter.string(fromByteCount: Int64($0), countStyle: .file) }
    }
}
