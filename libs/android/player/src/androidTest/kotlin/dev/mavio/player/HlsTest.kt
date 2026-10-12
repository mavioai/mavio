package dev.mavio.player

import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Media3 playing the server's fMP4 HLS across segments and into a
 * transcode restarted by a seek; the server starts encoded audio, which
 * has priming, at zero, as Media3 rejects a negative decode time. The
 * master playlist comes from the instrumentation argument `mavio.hls`, a
 * playback of at least 90 seconds with video and transcoded AAC audio;
 * skipped without it.
 */
@UnstableApi
@RunWith(AndroidJUnit4::class)
class HlsTest {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()

    private fun <T> main(block: () -> T): T {
        var result: Result<T>? = null
        instrumentation.runOnMainSync { result = runCatching(block) }
        return result!!.getOrThrow()
    }

    private fun waitFor(what: String, state: () -> String, condition: () -> Boolean) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(60)
        while (!main(condition)) {
            assertTrue("timed out waiting for $what: ${main(state)}", System.nanoTime() < deadline)
            Thread.sleep(100)
        }
    }

    @Test
    fun playsAcrossSegmentsAndRestartedTranscodes() {
        val url = InstrumentationRegistry.getArguments().getString("mavio.hls")
        assumeTrue("no mavio.hls argument", url != null)
        val context = instrumentation.targetContext
        instrumentation.uiAutomation.grantRuntimePermission(
            context.packageName,
            "android.permission.ACCESS_LOCAL_NETWORK",
        )
        ActivityScenario.launch(SurfaceActivity::class.java).use { scenario ->
            lateinit var activity: SurfaceActivity
            scenario.onActivity { activity = it }
            val player = main { ExoPlayer.Builder(context).build() }
            var rebuffers = 0
            main {
                player.addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(state: Int) {
                        if (state == Player.STATE_BUFFERING && player.totalBufferedDuration == 0L &&
                            player.currentPosition > 0
                        ) {
                            rebuffers++
                        }
                    }
                })
            }
            val state = {
                "state ${player.playbackState}, playing ${player.isPlaying}, " +
                    "at ${player.currentPosition} of ${player.duration}, error ${player.playerError}"
            }
            try {
                main {
                    player.setVideoSurfaceView(activity.surface)
                    player.setMediaItem(MediaItem.fromUri(url!!))
                    player.prepare()
                    player.play()
                }
                // Past the first segment boundary.
                waitFor("playing past 8 s", state) { player.isPlaying && player.currentPosition > 8_000 }
                assertTrue("no duration", main { player.duration } > 90_000)
                val tracks = main { player.currentTracks }
                assertTrue("no video track", tracks.isTypeSelected(C.TRACK_TYPE_VIDEO))
                assertTrue("no audio track", tracks.isTypeSelected(C.TRACK_TYPE_AUDIO))
                val audio = main { player.audioDecoderCounters!!.renderedOutputBufferCount }
                val video = main { player.videoDecoderCounters!!.renderedOutputBufferCount }
                // At 24 frames a second, 8 seconds of video.
                assertTrue("got = $video frames, want about 190", video > 150)
                assertTrue("got = $audio audio buffers", audio > 0)
                assertEquals("rebuffered while playing", 0, rebuffers)

                // The server restarts the transcode at the segment sought.
                main { player.seekTo(60_000) }
                waitFor("playing past 68 s", state) { player.isPlaying && player.currentPosition > 68_000 }
                val skipped = main { player.videoDecoderCounters!!.droppedBufferCount }
                assertTrue("got = $skipped dropped frames", skipped < 24)

                main { player.seekTo(player.duration - 2_000) }
                waitFor("the end", state) { player.playbackState == Player.STATE_ENDED }
                assertEquals(null, main { player.playerError })
            } finally {
                main { player.release() }
            }
        }
    }
}
