package dev.mavio.ui

import android.os.Build
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.unit.dp

/** The colors of the posters, top to bottom: warm, cool and dusk, as films are. */
private val tones = listOf(
    0xFFFF9F0A to 0xFF6B2400,
    0xFF2A6F97 to 0xFF0A1A2E,
    0xFF8E5CF7 to 0xFF24104D,
    0xFFF0412F to 0xFF3A0A0A,
    0xFF30B07A to 0xFF0A2E20,
    0xFFFFD34E to 0xFF7A3E00,
    0xFF5AC8FA to 0xFF10325A,
    0xFFFF6482 to 0xFF420D26,
    0xFFC7A17A to 0xFF33241A,
    0xFF64D2FF to 0xFF1A2A66,
    0xFFE07A5F to 0xFF2E1A3A,
).map { (from, to) -> Color(from) to Color(to) }

/**
 * The screen behind sign-in, as the web UI's: a wall of posters drawn from
 * gradients, tilted and blurred under a veil, darker or lighter with the
 * theme; on a television wider, more blurred and less tilted.
 */
@Composable
fun PosterWall(modifier: Modifier = Modifier, tv: Boolean = false) {
    val dark = theme.colors.background.luminance() < 0.5f
    val columns = if (tv) 10 else 8
    BoxWithConstraints(modifier.fillMaxSize().clipToBounds()) {
        val width = maxWidth * 1.5f
        val height = maxHeight * 1.5f
        val gap = width * 0.01f
        // Enough posters to fill the wall's height, however tall the screen.
        val rows = (height / ((width - gap * (columns - 1)) / columns * 1.5f + gap)).toInt() + 2
        Row(
            Modifier
                .align(Alignment.Center)
                .requiredSize(width, height)
                .graphicsLayer {
                    rotationZ = if (tv) -6f else -8f
                    scaleX = 1.15f
                    scaleY = 1.15f
                }
                .then(if (blurAvailable) Modifier.blur(if (tv) 30.dp else 2.dp) else Modifier)
                .alpha(if (dark) 0.7f else 0.6f),
            horizontalArrangement = Arrangement.spacedBy(gap),
        ) {
            repeat(columns) { col ->
                Column(
                    Modifier
                        .weight(1f)
                        .wrapContentHeight(Alignment.Top, unbounded = true)
                        .offset(y = if (col % 2 == 1) -width * 0.06f else 0.dp),
                    verticalArrangement = Arrangement.spacedBy(gap),
                ) {
                    repeat(rows) { r ->
                        val (from, to) = tones[(col * 4 + r * 7) % tones.size]
                        val shape = RoundedCornerShape(8.dp)
                        Box(
                            Modifier
                                .fillMaxWidth()
                                .aspectRatio(2f / 3f)
                                .clip(shape)
                                .background(Brush.linearGradient(listOf(from, to))),
                        ) {
                            Box(
                                Modifier
                                    .fillMaxSize()
                                    .background(
                                        Brush.linearGradient(
                                            0f to Color.White.copy(alpha = 0.28f),
                                            0.55f to Color.White.copy(alpha = 0f),
                                        ),
                                    ),
                            )
                        }
                    }
                }
            }
        }
        Box(
            Modifier
                .fillMaxSize()
                .background(if (dark) Color.Black.copy(alpha = 0.6f) else Color.White.copy(alpha = 0.6f)),
        )
    }
}

/**
 * Whether to blur what lies behind glass and the poster wall: from
 * Android 12, which blurs with RenderEffect, but not on the emulator,
 * whose GPU takes seconds to draw a blurred frame; there the tint alone
 * stands in for the glass.
 */
val blurAvailable: Boolean = Build.VERSION.SDK_INT >= Build.VERSION_CODES.S && Build.HARDWARE != "ranchu"
