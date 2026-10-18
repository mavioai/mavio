import MavioPlayer
import Testing

struct PlayerEngineTests {
    struct Case: Sendable, CustomTestStringConvertible {
        let name: String
        let hls: Bool
        var subtitleFile = false
        var container: String? = "mp4"
        var audioCodec: String? = "aac"
        var selectsTracks = false
        let want: PlayerEngine

        var testDescription: String { name }
    }

    @Test(arguments: [
        Case(name: "HLS", hls: true, want: .avPlayer),
        Case(name: "HLS with a subtitle file", hls: true, subtitleFile: true, want: .aether),
        Case(name: "HLS selecting tracks", hls: true, selectsTracks: true, want: .avPlayer),
        Case(name: "MP4 played directly", hls: false, want: .avPlayer),
        Case(name: "Matroska played directly", hls: false, container: "mkv", want: .aether),
        Case(name: "an audio codec AVPlayer lacks", hls: false, audioCodec: "dts", want: .aether),
        Case(name: "tracks selected", hls: false, selectsTracks: true, want: .aether),
    ])
    func choosesTheEngine(_ c: Case) {
        let got = PlayerEngine.choose(
            hls: c.hls, subtitleFile: c.subtitleFile, container: c.container, audioCodec: c.audioCodec,
            selectsTracks: c.selectsTracks, avPlayerContainers: ["mp4", "mov"], avPlayerAudioCodecs: ["aac"])
        #expect(got == c.want, "got = \(got), want = \(c.want)")
    }
}
