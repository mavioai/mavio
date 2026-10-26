package dev.mavio.player

import android.content.Context
import android.os.Looper
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ForwardingRenderer
import androidx.media3.exoplayer.Renderer
import androidx.media3.exoplayer.text.TextOutput

/**
 * How far a player's subtitles are shifted from their times, as the web
 * UI's subtitle offset: later when positive, earlier when negative. Given
 * to [createPlayer], it applies as it changes: libmpv delays its
 * subtitles, and Media3's text renderer renders them at the position less
 * the offset, so an earlier one shows the cues it has read ahead.
 */
class SubtitleShift {
    /** The offset, in milliseconds. */
    @Volatile
    var ms: Long = 0
        set(value) {
            field = value
            onChange?.invoke(value)
        }

    /** What applies the offset to libmpv as it changes. */
    internal var onChange: ((Long) -> Unit)? = null
}

/** Media3's renderers, its text renderers rendering at the position less [shift]'s offset. */
@UnstableApi
internal class ShiftedRenderersFactory(context: Context, private val shift: SubtitleShift) : DefaultRenderersFactory(context) {
    override fun buildTextRenderers(
        context: Context,
        output: TextOutput,
        outputLooper: Looper,
        extensionRendererMode: Int,
        out: ArrayList<Renderer>,
    ) {
        val built = ArrayList<Renderer>()
        super.buildTextRenderers(context, output, outputLooper, extensionRendererMode, built)
        built.mapTo(out) { ShiftedRenderer(it, shift) }
    }
}

/** A renderer at its position less [shift]'s offset. */
@UnstableApi
private class ShiftedRenderer(renderer: Renderer, private val shift: SubtitleShift) : ForwardingRenderer(renderer) {
    private fun shifted(positionUs: Long) = (positionUs - shift.ms * 1000).coerceAtLeast(0)

    override fun render(positionUs: Long, elapsedRealtimeUs: Long) = super.render(shifted(positionUs), elapsedRealtimeUs)

    override fun resetPosition(positionUs: Long, sampleStreamIsResetToKeyFrame: Boolean) =
        super.resetPosition(shifted(positionUs), sampleStreamIsResetToKeyFrame)
}
