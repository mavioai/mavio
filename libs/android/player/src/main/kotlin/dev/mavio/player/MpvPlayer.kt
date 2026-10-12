package dev.mavio.player

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.view.SurfaceHolder
import android.view.SurfaceView
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.PlaybackParameters
import androidx.media3.common.Player
import androidx.media3.common.SimpleBasePlayer
import androidx.media3.common.VideoSize
import androidx.media3.common.util.UnstableApi
import com.google.common.util.concurrent.Futures
import com.google.common.util.concurrent.ListenableFuture
import `is`.xyz.mpv.MPVLib
import `is`.xyz.mpv.MPVLib.MpvEvent
import `is`.xyz.mpv.MPVLib.MpvFormat

/**
 * libmpv as a Media3 [Player], so that the media session and the controls
 * treat it as they treat ExoPlayer. It renders with OpenGL ES into the
 * [SurfaceView] or [SurfaceHolder] given as its video output, and plays
 * audio alone without one. It decodes with MediaCodec where it can, or as
 * [hwdec] says. libmpv has one instance per process, so one player lives
 * at a time.
 */
@UnstableApi
class MpvPlayer(
    context: Context,
    looper: Looper = Looper.getMainLooper(),
    hwdec: String = "mediacodec,mediacodec-copy",
) :
    SimpleBasePlayer(looper),
    MPVLib.EventObserver,
    SurfaceHolder.Callback {
    private val handler = Handler(looper)
    private var items: List<MediaItem> = emptyList()
    private var index = 0
    private var startMs = 0L
    private var prepared = false
    private var loaded = false
    private var playWhenReady = false
    private var paused = true
    private var buffering = false
    private var ended = false
    private var positionMs = 0L
    private var durationMs = C.TIME_UNSET
    private var videoSize = VideoSize.UNKNOWN
    private var speed = 1f
    private var error: PlaybackException? = null
    private var holder: SurfaceHolder? = null
    private var surfaceReady = false
    private var released = false

    init {
        MPVLib.create(context.applicationContext)
        MPVLib.setOptionString("vo", "gpu")
        MPVLib.setOptionString("gpu-context", "android")
        MPVLib.setOptionString("opengl-es", "yes")
        MPVLib.setOptionString("hwdec", hwdec)
        MPVLib.setOptionString("hwdec-codecs", "h264,hevc,mpeg4,mpeg2video,vp8,vp9,av1")
        MPVLib.setOptionString("ao", "audiotrack,opensles")
        MPVLib.setOptionString("audio-set-media-role", "yes")
        // At the end the file stays loaded, paused on its last frame.
        MPVLib.setOptionString("keep-open", "yes")
        MPVLib.setOptionString("idle", "yes")
        MPVLib.setOptionString("input-default-bindings", "no")
        // libass finds no fontconfig configuration on Android; the
        // system's fonts draw the subtitles that embed none.
        MPVLib.setOptionString("sub-fonts-dir", "/system/fonts")
        MPVLib.init()
        MPVLib.setOptionString("force-window", "no")
        MPVLib.addObserver(this)
        for ((name, format) in OBSERVED) MPVLib.observeProperty(name, format)
    }

    override fun getState(): State {
        val playlist = items.mapIndexed { i, item ->
            MediaItemData.Builder(i)
                .setMediaItem(item)
                .setDurationUs(if (i == index && durationMs != C.TIME_UNSET) durationMs * 1000 else C.TIME_UNSET)
                .setIsSeekable(i == index && loaded)
                .build()
        }
        val state = when {
            !prepared || items.isEmpty() || error != null -> STATE_IDLE
            ended -> STATE_ENDED
            !loaded || buffering -> STATE_BUFFERING
            else -> STATE_READY
        }
        return State.Builder()
            .setAvailableCommands(COMMANDS)
            .setPlayWhenReady(playWhenReady, PLAY_WHEN_READY_CHANGE_REASON_USER_REQUEST)
            .setPlaybackState(state)
            .setPlayerError(error)
            .setPlaylist(playlist)
            .setCurrentMediaItemIndex(index)
            .setContentPositionMs { positionMs }
            .setVideoSize(videoSize)
            .setPlaybackParameters(PlaybackParameters(speed))
            .build()
    }

    override fun handleSetMediaItems(
        mediaItems: List<MediaItem>,
        startIndex: Int,
        startPositionMs: Long,
    ): ListenableFuture<*> {
        items = mediaItems
        index = if (startIndex == C.INDEX_UNSET) 0 else startIndex
        startMs = if (startPositionMs == C.TIME_UNSET) 0 else startPositionMs
        if (prepared) load()
        return Futures.immediateVoidFuture()
    }

    override fun handlePrepare(): ListenableFuture<*> {
        prepared = true
        error = null
        load()
        return Futures.immediateVoidFuture()
    }

    override fun handleSetPlayWhenReady(playWhenReady: Boolean): ListenableFuture<*> {
        this.playWhenReady = playWhenReady
        MPVLib.setPropertyBoolean("pause", !playWhenReady)
        return Futures.immediateVoidFuture()
    }

    override fun handleSeek(mediaItemIndex: Int, positionMs: Long, seekCommand: Int): ListenableFuture<*> {
        val target = if (positionMs == C.TIME_UNSET) 0 else positionMs
        if (mediaItemIndex != index) {
            index = mediaItemIndex
            startMs = target
            load()
        } else {
            this.positionMs = target
            ended = false
            MPVLib.command(arrayOf("seek", (target / 1000.0).toString(), "absolute"))
        }
        return Futures.immediateVoidFuture()
    }

    override fun handleSetPlaybackParameters(playbackParameters: PlaybackParameters): ListenableFuture<*> {
        speed = playbackParameters.speed
        MPVLib.setPropertyDouble("speed", speed.toDouble())
        return Futures.immediateVoidFuture()
    }

    override fun handleSetVideoOutput(videoOutput: Any): ListenableFuture<*> {
        val next = when (videoOutput) {
            is SurfaceView -> videoOutput.holder
            is SurfaceHolder -> videoOutput
            else -> throw IllegalArgumentException("mpv renders into a SurfaceView or SurfaceHolder")
        }
        if (next !== holder) {
            detach()
            holder = next
            next.addCallback(this)
            val surface = next.surface
            if (surface != null && surface.isValid) {
                surfaceCreated(next)
                val frame = next.surfaceFrame
                resize(frame.width(), frame.height())
            }
        }
        return Futures.immediateVoidFuture()
    }

    override fun handleClearVideoOutput(videoOutput: Any?): ListenableFuture<*> {
        detach()
        return Futures.immediateVoidFuture()
    }

    override fun handleStop(): ListenableFuture<*> {
        prepared = false
        loaded = false
        ended = false
        MPVLib.command(arrayOf("stop"))
        return Futures.immediateVoidFuture()
    }

    override fun handleRelease(): ListenableFuture<*> {
        detach()
        MPVLib.removeObserver(this)
        // libmpv ends the process when called after it is destroyed, so the
        // events still queued are dropped.
        released = true
        handler.removeCallbacksAndMessages(null)
        MPVLib.destroy()
        return Futures.immediateVoidFuture()
    }

    private fun load() {
        val item = items.getOrNull(index) ?: return
        val uri = item.localConfiguration?.uri ?: return
        loaded = false
        ended = false
        positionMs = startMs
        durationMs = C.TIME_UNSET
        // Without a surface the video is not drawn; the audio still plays.
        if (!surfaceReady) MPVLib.setPropertyString("vo", "null")
        MPVLib.setPropertyString("start", if (startMs > 0) (startMs / 1000.0).toString() else "none")
        MPVLib.setPropertyBoolean("pause", !playWhenReady)
        MPVLib.command(arrayOf("loadfile", uri.toString(), "replace"))
    }

    private fun detach() {
        val current = holder ?: return
        current.removeCallback(this)
        if (surfaceReady) surfaceDestroyed(current)
        holder = null
    }

    override fun surfaceCreated(holder: SurfaceHolder) {
        MPVLib.attachSurface(holder.surface)
        MPVLib.setOptionString("force-window", "yes")
        MPVLib.setPropertyString("vo", "gpu")
        surfaceReady = true
    }

    override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
        resize(width, height)
    }

    private fun resize(width: Int, height: Int) {
        MPVLib.setPropertyString("android-surface-size", "${width}x$height")
    }

    override fun surfaceDestroyed(holder: SurfaceHolder) {
        surfaceReady = false
        MPVLib.setPropertyString("vo", "null")
        MPVLib.setPropertyString("force-window", "no")
        MPVLib.detachSurface()
    }

    // libmpv's events arrive on its own thread; the state changes on the
    // player's.
    private fun update(change: () -> Unit) {
        handler.post {
            if (released) return@post
            change()
            invalidateState()
        }
    }

    override fun eventProperty(property: String) {}

    override fun eventProperty(property: String, value: Long) {
        update {
            when (property) {
                "dwidth" -> videoSize = VideoSize(value.toInt(), videoSize.height.coerceAtLeast(0))
                "dheight" -> videoSize = VideoSize(videoSize.width.coerceAtLeast(0), value.toInt())
            }
        }
    }

    override fun eventProperty(property: String, value: Boolean) {
        update {
            when (property) {
                "pause" -> paused = value
                "paused-for-cache" -> buffering = value
                "eof-reached" -> ended = value
            }
        }
    }

    override fun eventProperty(property: String, value: String) {}

    override fun eventProperty(property: String, value: Double) {
        update {
            when (property) {
                "time-pos" -> positionMs = (value * 1000).toLong()
                "duration" -> durationMs = (value * 1000).toLong()
            }
        }
    }

    override fun event(eventId: Int) {
        update {
            when (eventId) {
                MpvEvent.MPV_EVENT_FILE_LOADED -> {
                    loaded = true
                    MPVLib.setPropertyString("start", "none")
                }
                MpvEvent.MPV_EVENT_END_FILE -> if (!loaded && prepared) {
                    // keep-open keeps a played file loaded, so it ends
                    // only when it failed to open.
                    error = PlaybackException(
                        "libmpv could not play the source",
                        null,
                        PlaybackException.ERROR_CODE_IO_UNSPECIFIED,
                    )
                }
            }
        }
    }

    private companion object {
        val OBSERVED = listOf(
            "time-pos" to MpvFormat.MPV_FORMAT_DOUBLE,
            "duration" to MpvFormat.MPV_FORMAT_DOUBLE,
            "pause" to MpvFormat.MPV_FORMAT_FLAG,
            "paused-for-cache" to MpvFormat.MPV_FORMAT_FLAG,
            "eof-reached" to MpvFormat.MPV_FORMAT_FLAG,
            "dwidth" to MpvFormat.MPV_FORMAT_INT64,
            "dheight" to MpvFormat.MPV_FORMAT_INT64,
        )

        val COMMANDS: Player.Commands = Player.Commands.Builder()
            .addAll(
                COMMAND_PLAY_PAUSE,
                COMMAND_PREPARE,
                COMMAND_STOP,
                COMMAND_SEEK_TO_DEFAULT_POSITION,
                COMMAND_SEEK_IN_CURRENT_MEDIA_ITEM,
                COMMAND_SEEK_BACK,
                COMMAND_SEEK_FORWARD,
                COMMAND_SEEK_TO_MEDIA_ITEM,
                COMMAND_SET_SPEED_AND_PITCH,
                COMMAND_GET_CURRENT_MEDIA_ITEM,
                COMMAND_GET_TIMELINE,
                COMMAND_SET_MEDIA_ITEM,
                COMMAND_CHANGE_MEDIA_ITEMS,
                COMMAND_SET_VIDEO_SURFACE,
                COMMAND_RELEASE,
            )
            .build()
    }
}
