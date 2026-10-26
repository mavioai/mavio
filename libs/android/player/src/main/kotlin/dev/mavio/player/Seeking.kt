package dev.mavio.player

import android.os.Handler
import android.os.Looper
import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi

/**
 * A seek under way, as the web UI's: where it goes, which the seek bar
 * shows at once, and whether the frame from before it is hidden behind the
 * activity indicator, as it is once the seek has not landed within a beat.
 * The subtitles hide from the seek on, so that they never run ahead of the
 * frame shown.
 */
data class Seek(val targetMs: Long, val hidden: Boolean)

/** How long a seek runs before the frame from before it hides. */
private const val HIDE_AFTER_MS = 200L

/**
 * The seek [player] makes, from the seek until the first frame at the new
 * time shows, or, without video, until it can play; null between seeks.
 * A seek while the old frame is hidden keeps it hidden. libmpv's subtitles
 * hide meanwhile; Media3's are drawn by the caller, which hides them.
 */
@OptIn(UnstableApi::class)
@Composable
fun rememberSeek(player: Player?): Seek? {
    var seek by remember(player) { mutableStateOf<Seek?>(null) }
    DisposableEffect(player) {
        if (player == null) return@DisposableEffect onDispose {}
        val handler = Handler(Looper.getMainLooper())
        val hide = Runnable { seek = seek?.copy(hidden = true) }
        val land = {
            handler.removeCallbacks(hide)
            seek = null
        }
        val listener = object : Player.Listener {
            override fun onPositionDiscontinuity(oldPosition: Player.PositionInfo, newPosition: Player.PositionInfo, reason: Int) {
                if (reason != Player.DISCONTINUITY_REASON_SEEK) return
                val hidden = seek?.hidden == true
                seek = Seek(newPosition.positionMs, hidden)
                if (!hidden) {
                    handler.removeCallbacks(hide)
                    handler.postDelayed(hide, HIDE_AFTER_MS)
                }
            }

            override fun onRenderedFirstFrame() = land()

            override fun onPlaybackStateChanged(playbackState: Int) {
                val stopped = playbackState == Player.STATE_IDLE || playbackState == Player.STATE_ENDED
                // Audio alone has no frame to wait for.
                val audio = playbackState == Player.STATE_READY && player.videoSize.width == 0
                if (stopped || audio) land()
            }
        }
        player.addListener(listener)
        onDispose {
            handler.removeCallbacks(hide)
            player.removeListener(listener)
        }
    }
    val seeking = seek != null
    LaunchedEffect(player, seeking) { (player as? MpvPlayer)?.subtitlesHidden = seeking }
    return seek
}

/** Whether [player] waits for media, following its state as it changes. */
@Composable
fun rememberBuffering(player: Player?): Boolean {
    fun waiting() = player != null && (player.playbackState == Player.STATE_BUFFERING || player.playbackState == Player.STATE_IDLE)
    var buffering by remember(player) { mutableStateOf(waiting()) }
    DisposableEffect(player) {
        if (player == null) return@DisposableEffect onDispose {}
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                buffering = waiting()
            }
        }
        player.addListener(listener)
        buffering = waiting()
        onDispose { player.removeListener(listener) }
    }
    return buffering
}
