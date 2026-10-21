package dev.mavio.ui

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.exp
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin

/**
 * Draws the assistant's orb (libs/design/components.md, Assistant), as the
 * web UI's canvas and the Apple apps' renderer do: a sphere of glass with
 * three colored lights flowing inside it. Its size, spread, speed and
 * brightness follow the voice session's state and the audio's level. Drawn
 * each frame, it eases toward its state's look.
 */
class OrbRenderer {
    private class Look(
        val size: Double,
        val spread: Double,
        val speed: Double,
        val bright: Double,
        val ring: Boolean = false,
        // How much the audio's level swells, spreads, speeds and brightens it.
        val swing: Double = 0.0,
    )

    private var size = 0.55
    private var spread = 0.12
    private var speed = 0.35
    private var bright = 0.45
    private var ring = 0.0
    private var swing = 0.0
    private var lv = 0.0
    private var phase = 0.0
    private var last: Double? = null
    private var mode = OrbMode.OPENING
    private var since = 0.0
    private var interrupts = 0
    private var flashAt = Double.NEGATIVE_INFINITY
    private var quietSince = 0.0
    private var shake = 0.0

    /** How far the orb is shaken sideways now, in dp, as an error shakes it once. */
    val offset: Float get() = shake.toFloat()

    /**
     * Moves the orb on to now, in seconds: mode is the session's state;
     * level the audio's level, the microphone's or the playback's as the
     * mode wants; interrupts the count of the session's interrupts, each
     * flashing it. Reduced motion keeps it still at its state's look.
     */
    fun step(now: Double, mode: OrbMode, level: Double, interrupts: Int, reduceMotion: Boolean) {
        if (mode != this.mode) {
            this.mode = mode
            since = now
        }
        if (interrupts != this.interrupts) {
            this.interrupts = interrupts
            flashAt = now
        }
        val dt = min(0.05, now - (last ?: now))
        last = now
        val motion = !reduceMotion
        val raw = if (motion && (mode == OrbMode.HEARING || mode == OrbMode.SPEAKING || mode == OrbMode.IDLE)) level else 0.0
        // The user falls silent but the turn goes on: the orb draws in.
        if (raw > 0.25) quietSince = now
        val look = look(mode, pausing = now - quietSince > 0.4)
        val k = if (motion) 1 - exp(-dt * 6) else 1.0
        size += (look.size - size) * k
        spread += (look.spread - spread) * k
        speed += (look.speed - speed) * k
        bright += (look.bright - bright) * k
        ring += ((if (look.ring) 1.0 else 0.0) - ring) * k
        swing += (look.swing - swing) * k
        // A syllable swells it at once; it settles back more slowly.
        val target = raw * swing
        lv += (target - lv) * (1 - exp(-dt * if (target > lv) 22 else 8))
        if (motion) phase += dt * (speed + lv * 2.6)
        val t = now - since
        shake = if (motion && mode == OrbMode.ERROR && t < 0.6) sin(t * 40) * 6 * (1 - t / 0.6) else 0.0
    }

    /** Draws the orb filling the square of the draw scope, at now, on a light or dark page. */
    fun draw(scope: DrawScope, now: Double, light: Boolean, reduceMotion: Boolean) = with(scope) {
        val w = size.width
        val r = w / 2
        val base = (r / BLEED) * 0.58f
        val flash = if (reduceMotion) 0.0 else max(0.0, 1 - (now - flashAt) / 0.45)
        val red = if (mode == OrbMode.ERROR) max(0.0, 1 - (now - since) / 1.4) else 0.0
        val shrink = (1 - flash * 0.35).toFloat()
        val orb = this@OrbRenderer
        val edge = base * (orb.size + lv * 0.9).toFloat() * shrink * 0.92f
        val center = Offset(r, r)

        fun color(c: IntArray, a: Double) = Color(c[0], c[1], c[2], (a.coerceIn(0.0, 1.0) * 255).toInt())

        // A radial gradient whose stops run from r0 to r1, the last color reaching beyond.
        fun radial(at: Offset, r0: Float, r1: Float, stops: List<Pair<Float, Color>>) {
            val s = stops.map { (p, c) -> (r0 + p * (r1 - r0)) / r1 to c }.toTypedArray()
            drawRect(Brush.radialGradient(*s, center = at, radius = r1))
        }

        val brightness = bright + lv * 0.1
        fun lights(alpha: Double, blend: BlendMode) {
            BLOBS.forEachIndexed { i, b ->
                val a = phase * b.sp * 2 + b.ph
                // Each light pulses on its own beat with the level, so that
                // the colors surge against each other as the user speaks.
                val pulse = 1 + lv * 0.35 * sin(phase * 5.3 + i * 2.1)
                val orbit = base * (spread * (1 - ring) + ring * 0.55 + lv * 0.4 * pulse).toFloat() * shrink
                val x = r + cos(a).toFloat() * orbit
                val y = r + sin(a * (1 + 0.3 * (1 - ring))).toFloat() * orbit
                val rad = base * (orb.size + lv * 0.9 * pulse).toFloat() * shrink
                val tint = intArrayOf(255, 70, 80)
                val c = (if (light) b.light else b.dark).let { v -> IntArray(3) { j -> (v[j] + (tint[j] - v[j]) * red).toInt() } }
                drawCircle(
                    Brush.radialGradient(
                        0f to color(c, min(1.0, 0.9 * alpha * brightness)),
                        0.55f to color(c, (if (light) 0.5 else 0.35) * alpha * brightness),
                        1f to color(c, 0.0),
                        center = Offset(x, y),
                        radius = rad,
                    ),
                    radius = rad,
                    center = Offset(x, y),
                    blendMode = blend,
                )
            }
        }

        // The shadows under the sphere on a light page: a tight dark one and
        // a wider one of the blue.
        if (light) {
            radial(center + Offset(0f, 10.dp.toPx()), 0f, edge + 18.dp.toPx(), listOf(0f to color(intArrayOf(90, 70, 245), 0.42), edge / (edge + 18.dp.toPx()) to color(intArrayOf(90, 70, 245), 0.3), 1f to Color.Transparent))
            radial(center + Offset(0f, 2.dp.toPx()), 0f, edge + 3.dp.toPx(), listOf(0f to color(intArrayOf(20, 12, 80), 0.22), edge / (edge + 3.dp.toPx()) to color(intArrayOf(20, 12, 80), 0.22), 1f to Color.Transparent))
        }
        // The glow around the sphere, on a dark page.
        if (!light) lights(0.3 + lv * 0.2, BlendMode.Plus)
        val clip = Path().apply {
            fillType = PathFillType.EvenOdd
            addOval(androidx.compose.ui.geometry.Rect(center, edge))
            if (ring > 0.02) addOval(androidx.compose.ui.geometry.Rect(center, edge * 0.5f * ring.toFloat()))
        }
        clipPath(clip) {
            drawRect(if (light) color(BLOBS[2].light, 0.55 * bright) else color(intArrayOf(18, 14, 58), 0.85 * bright))
            lights(1.0, if (light) BlendMode.SrcOver else BlendMode.Plus)
            val ink = intArrayOf(10, 8, 40)
            val white = intArrayOf(255, 255, 255)
            // Depth, darker toward the lower edge; light bounced through the
            // glass along the bottom; a soft reflection at the top left.
            radial(
                Offset(r - edge * 0.25f, r - edge * 0.3f),
                edge * 0.2f,
                edge * 1.3f,
                listOf(0f to color(ink, 0.0), 0.7f to color(ink, 0.0), 1f to color(ink, if (light) 0.42 else 0.45)),
            )
            radial(Offset(r + edge * 0.1f, r + edge * 0.78f), 0f, edge * 0.55f, listOf(0f to color(white, if (light) 0.22 else 0.12), 1f to color(white, 0.0)))
            withTransform({
                translate(r - edge * 0.32f, r - edge * 0.46f)
                scale(1f, 0.62f, pivot = Offset.Zero)
            }) {
                radial(
                    Offset.Zero,
                    0f,
                    edge * 0.5f,
                    listOf(0f to color(white, if (light) 0.9 else 0.55), 0.35f to color(white, if (light) 0.38 else 0.18), 1f to color(white, 0.0)),
                )
            }
        }
        if (flash > 0) radial(center, 0f, base * 0.9f, listOf(0f to color(intArrayOf(255, 255, 255), 0.8 * flash), 1f to Color.Transparent))
    }

    private class Blob(val dark: IntArray, val light: IntArray, val sp: Double, val ph: Double)

    companion object {
        /** The orb's side. */
        val SIZE = 84.dp

        /** How far the drawing reaches past the orb's box on every side, for its swell and glow. */
        const val BLEED = 1.5f

        // Magenta, cyan and blue: added up on a dark page, deeper and laid
        // over each other on a light one.
        private val BLOBS = listOf(
            Blob(intArrayOf(255, 70, 190), intArrayOf(235, 40, 160), 0.9, 0.0),
            Blob(intArrayOf(40, 220, 255), intArrayOf(0, 170, 235), -1.15, 2.1),
            Blob(intArrayOf(100, 90, 255), intArrayOf(90, 70, 245), 0.7, 4.2),
        )

        private fun look(mode: OrbMode, pausing: Boolean): Look = when (mode) {
            OrbMode.OPENING -> Look(0.55, 0.12, 0.35, 0.45)
            OrbMode.IDLE -> Look(0.78, 0.2, 0.6, 0.85, swing = 0.15)
            OrbMode.HEARING -> if (pausing) Look(0.7, 0.16, 0.5, 0.8) else Look(0.82, 0.24, 1.3, 1.0, swing = 1.0)
            OrbMode.THINKING -> Look(0.62, 0.4, 3.4, 0.95, ring = true)
            OrbMode.SPEAKING -> Look(0.8, 0.2, 1.2, 1.0, swing = 0.55)
            OrbMode.RESTING, OrbMode.ERROR, OrbMode.ENDED -> Look(0.76, 0.16, 0.18, 0.85)
        }
    }
}
