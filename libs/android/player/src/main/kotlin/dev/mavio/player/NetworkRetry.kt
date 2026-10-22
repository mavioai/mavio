package dev.mavio.player

import android.os.Handler
import android.os.Looper
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer

/** The network errors a playback retries. */
private val networkErrors = setOf(
    PlaybackException.ERROR_CODE_IO_UNSPECIFIED,
    PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_FAILED,
    PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_TIMEOUT,
    PlaybackException.ERROR_CODE_IO_BAD_HTTP_STATUS,
)

/** How long each retry waits, about twenty seconds in all. */
private val retryDelaysMs = longArrayOf(1000, 2000, 4000, 8000, 8000)

/**
 * Retries a Media3 player's network errors, as when the device's network
 * changes under it, for about twenty seconds, twice as long apart each
 * time, before they stand, as the web UI's; playing again starts over.
 * Added to [player] as its listener.
 */
class NetworkRetry(private val player: Player) : Player.Listener {
    private val handler = Handler(Looper.getMainLooper())
    private var retries = 0

    /** Whether [error] is retried, preparing the player again after a while. */
    fun retries(error: PlaybackException): Boolean {
        if (player !is ExoPlayer || error.errorCode !in networkErrors || retries >= retryDelaysMs.size) return false
        handler.postDelayed({ player.prepare() }, retryDelaysMs[retries++])
        return true
    }

    override fun onIsPlayingChanged(isPlaying: Boolean) {
        if (isPlaying) retries = 0
    }

    /** Cancels the retry waiting, before the player is released. */
    fun cancel() = handler.removeCallbacksAndMessages(null)
}
