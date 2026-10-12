package dev.mavio.player

import android.graphics.Bitmap
import android.graphics.Color
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaController
import androidx.media3.session.MediaSession
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
 * libmpv on a device: it renders into a [android.view.SurfaceView] and a
 * media session publishes it as it publishes ExoPlayer. The media comes
 * from the URL in the instrumentation argument `mavio.media`, a Matroska
 * file of at least 30 seconds with video, audio and subtitles; skipped
 * without it.
 */
@UnstableApi
@RunWith(AndroidJUnit4::class)
class MpvPlayerTest {
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
    fun rendersIntoASurfaceViewBehindAMediaSession() {
        val url = InstrumentationRegistry.getArguments().getString("mavio.media")
        assumeTrue("no mavio.media argument", url != null)
        val context = instrumentation.targetContext
        instrumentation.uiAutomation.grantRuntimePermission(
            context.packageName,
            "android.permission.ACCESS_LOCAL_NETWORK",
        )
        ActivityScenario.launch(SurfaceActivity::class.java).use { scenario ->
            lateinit var activity: SurfaceActivity
            scenario.onActivity { activity = it }
            // The emulator's MediaCodec decoders stall libmpv; it decodes in
            // software there.
            val player = main { MpvPlayer(context, hwdec = "no") }
            val session = main { MediaSession.Builder(context, player).build() }
            try {
                lateinit var controller: MediaController
                val state = {
                    "state ${controller.playbackState}, playing ${controller.isPlaying}, " +
                        "at ${controller.currentPosition} of ${controller.duration}, error ${controller.playerError}"
                }
                controller = main { MediaController.Builder(context, session.token).buildAsync() }
                    .get(10, TimeUnit.SECONDS)
                main {
                    player.setVideoSurfaceView(activity.surface)
                    controller.setMediaItem(MediaItem.fromUri(url!!), 10_000)
                    controller.prepare()
                    controller.play()
                }
                waitFor("playing past 12 s through the session", state) {
                    controller.isPlaying && controller.currentPosition > 12_000
                }
                assertEquals(Player.STATE_READY, main { controller.playbackState })
                assertTrue("no duration", main { controller.duration } > 30_000)
                assertTrue("no video size", main { player.videoSize.width } > 0)

                // The first frame may follow the audio by seconds on an
                // emulator.
                var screen = instrumentation.uiAutomation.takeScreenshot()
                val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(60)
                while (colors(screen) <= 16 && System.nanoTime() < deadline) {
                    Thread.sleep(500)
                    screen = instrumentation.uiAutomation.takeScreenshot()
                }
                context.getExternalFilesDir(null)?.resolve("mpv.png")?.outputStream()?.use {
                    screen.compress(Bitmap.CompressFormat.PNG, 100, it)
                }
                assertTrue("the surface shows no picture", colors(screen) > 16)

                main { controller.pause() }
                waitFor("paused", state) { !player.isPlaying }
                main { controller.seekTo(controller.duration - 1_000) }
                main { controller.play() }
                waitFor("the end", state) { controller.playbackState == Player.STATE_ENDED }
                main { controller.release() }
            } finally {
                main {
                    session.release()
                    player.release()
                }
            }
        }
    }

    // The distinct colors of a grid of samples over the middle of the screen.
    private fun colors(bitmap: Bitmap): Int {
        val samples = mutableSetOf<Int>()
        for (y in bitmap.height / 4 until bitmap.height * 3 / 4 step bitmap.height / 40) {
            for (x in 0 until bitmap.width step bitmap.width / 40) {
                val c = bitmap.getPixel(x, y)
                samples += Color.rgb(Color.red(c) / 16, Color.green(c) / 16, Color.blue(c) / 16)
            }
        }
        return samples.size
    }
}
