import Foundation
import MavioCore
import VideoToolbox

/// What the apps play with their two engines (docs/clients.md §4.2): the
/// server plays a source directly when AVPlayer or AetherEngine can, and
/// processes it into HLS for AVPlayer only when neither can.
public enum DeviceProfile {
    /// The containers AVPlayer opens.
    public static let avPlayerContainers: Set<String> = ["mp4", "m4v", "mov"]
    /// The audio codecs AVPlayer decodes in them.
    public static let avPlayerAudioCodecs: Set<String> = ["aac", "mp3", "ac3", "eac3", "alac", "flac"]

    /// The video codecs the hardware decodes, which both engines play.
    static func hardwareVideoCodecs() -> [String] {
        var codecs = ["h264", "hevc"]
        if VTIsHardwareDecodeSupported(kCMVideoCodecType_AV1) { codecs.append("av1") }
        return codecs
    }

    /// The capabilities of this device; with airPlay, AVPlayer's alone,
    /// since only AVPlayer plays to an AirPlay receiver.
    public static func capabilities(airPlay: Bool = false, maxBitrate: Int64 = 0) -> Capabilities {
        let hardware = hardwareVideoCodecs()
        var directPlay = [
            DirectPlayProfile(
                kind: .video,
                container: avPlayerContainers.sorted().joined(separator: ","),
                videoCodec: hardware.joined(separator: ","),
                audioCodec: avPlayerAudioCodecs.sorted().joined(separator: ",")
            ),
            DirectPlayProfile(
                kind: .audio, container: "mp3,m4a,mp4,aac,flac,wav,aiff,alac", videoCodec: "", audioCodec: ""),
        ]
        if !airPlay {
            // AetherEngine demuxes with FFmpeg and decodes what the hardware
            // cannot with dav1d and libavcodec.
            directPlay += [
                DirectPlayProfile(
                    kind: .video,
                    container: "mkv,webm,mp4,m4v,mov,avi,ts,m2ts,mpegts,flv,wmv,asf,ogv",
                    videoCodec: "h264,hevc,av1,vp9,vp8,mpeg2video,mpeg4,vc1",
                    audioCodec: "aac,mp3,mp2,ac3,eac3,dts,truehd,flac,alac,opus,vorbis,pcm_s16le,pcm_s24le"
                ),
                DirectPlayProfile(
                    kind: .audio, container: "ogg,oga,opus,webm,mka,ape,wv,dsf", videoCodec: "", audioCodec: ""),
            ]
        }
        let hls = { (kind: MediaType, video: String, audio: String) in
            TranscodingProfile(
                kind: kind, download: false, delivery: .hls, container: "mp4", videoCodec: video, audioCodec: audio,
                maxAudioChannels: 6, subtitlesInManifest: true, segmentLengthSeconds: 0, conditions: []
            )
        }
        let transcoding = [
            hls(.video, "hevc,h264", "aac,ac3,eac3"),
            hls(.audio, "", "aac"),
            TranscodingProfile(
                kind: .video, download: true, delivery: .http, container: "mp4", videoCodec: "hevc,h264",
                audioCodec: "aac",
                maxAudioChannels: 2, subtitlesInManifest: false, segmentLengthSeconds: 0, conditions: []
            ),
        ]
        var subtitles = [SubtitleProfile(format: "vtt", delivery: .hls)]
        if !airPlay {
            // AetherEngine draws what the file carries, and files of their own.
            subtitles += ["ass", "ssa", "subrip", "srt", "vtt", "webvtt", "mov_text", "pgssub", "dvdsub", "dvbsub"].map
            {
                SubtitleProfile(format: $0, delivery: .embed)
            }
            subtitles += ["srt", "subrip", "vtt", "ass", "ssa"].map { SubtitleProfile(format: $0, delivery: .external) }
        }
        return Capabilities(
            name: airPlay ? "Mavio (AirPlay)" : "Mavio",
            maxStreamingBitrate: maxBitrate,
            maxStaticBitrate: 0,
            directPlay: directPlay,
            transcoding: transcoding,
            containers: [],
            codecs: [],
            subtitles: subtitles
        )
    }
}
