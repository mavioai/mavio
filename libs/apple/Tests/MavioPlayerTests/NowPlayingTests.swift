import MavioPlayer
import MediaPlayer
import Testing

/// Now Playing is the system's, one for the process: its tests run one at
/// a time.
@MainActor
@Suite(.serialized)
struct NowPlayingTests {
    @Test func showsTheItemUntilThePlayerStops() {
        let player = Player()
        player.nowPlaying.show(NowPlaying.Item(title: "Arrival", subtitle: "Atlas Station · S1 E2"))
        let info = MPNowPlayingInfoCenter.default().nowPlayingInfo ?? [:]
        #expect(info[MPMediaItemPropertyTitle] as? String == "Arrival")
        #expect(info[MPMediaItemPropertyArtist] as? String == "Atlas Station · S1 E2")
        #expect(info[MPMediaItemPropertyPlaybackDuration] == nil)

        player.stop()
        #expect(MPNowPlayingInfoCenter.default().nowPlayingInfo == nil)
    }

    @Test func takesTheNextTrackOnlyWithANextItem() {
        let player = Player()
        let next = MPRemoteCommandCenter.shared().nextTrackCommand
        player.nowPlaying.show(NowPlaying.Item(title: "Arrival"))
        #expect(!next.isEnabled)
        player.nowPlaying.next = {}
        #expect(next.isEnabled)
        #expect(MPRemoteCommandCenter.shared().playCommand.isEnabled)

        player.stop()
        #expect(!next.isEnabled)
    }
}
