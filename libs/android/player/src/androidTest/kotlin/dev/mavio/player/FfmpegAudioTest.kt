package dev.mavio.player

import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.util.UnstableApi
import androidx.media3.decoder.ffmpeg.FfmpegLibrary
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.analytics.AnalyticsListener
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Media3's FFmpeg decoder extension decoding the audio formats Android's
 * decoders lack. The media come from the instrumentation argument
 * `mavio.audio`, comma-separated URLs of files of at least ten seconds
 * whose audio is such a format, as DTS and TrueHD; skipped without it.
 */
@UnstableApi
@RunWith(AndroidJUnit4::class)
class FfmpegAudioTest {
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
    fun decodesTheAudioTheDeviceLacks() {
        val urls = InstrumentationRegistry.getArguments().getString("mavio.audio")
        assumeTrue("no mavio.audio argument", urls != null)
        val context = instrumentation.targetContext
        instrumentation.uiAutomation.grantRuntimePermission(
            context.packageName,
            "android.permission.ACCESS_LOCAL_NETWORK",
        )
        assertTrue("the extension's library did not load", FfmpegLibrary.isAvailable())
        for (url in urls!!.split(",")) {
            // The extension decodes what the platform's decoders cannot.
            val renderers = DefaultRenderersFactory(context)
                .setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
            val player = main { ExoPlayer.Builder(context, renderers).build() }
            var decoder = ""
            main {
                player.addAnalyticsListener(object : AnalyticsListener {
                    override fun onAudioDecoderInitialized(
                        eventTime: AnalyticsListener.EventTime,
                        decoderName: String,
                        initializedTimestampMs: Long,
                        initializationDurationMs: Long,
                    ) {
                        decoder = decoderName
                    }
                })
            }
            val state = {
                "$url: state ${player.playbackState}, at ${player.currentPosition}, " +
                    "decoder $decoder, error ${player.playerError}"
            }
            try {
                main {
                    player.setMediaItem(MediaItem.fromUri(url))
                    player.prepare()
                    player.play()
                }
                waitFor("playing past 5 s", state) { player.isPlaying && player.currentPosition > 5_000 }
                assertTrue("got = decoder $decoder, want = FFmpeg's (${main(state)})", decoder.startsWith("ffmpeg"))
                assertTrue("no audio track", main { player.currentTracks.isTypeSelected(C.TRACK_TYPE_AUDIO) })
                val buffers = main { player.audioDecoderCounters!!.renderedOutputBufferCount }
                assertTrue("got = $buffers audio buffers", buffers > 100)
            } finally {
                main { player.release() }
            }
        }
    }
}
