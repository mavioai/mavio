package dev.mavio.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** The wordmark's path, on its grid of 244 × 64 (libs/design/brand/wordmark.svg). */
private val wordmarkPath = PathParser().parsePathString(
    "M0 64V0H14L30 30L46 0H60V64H48V24L34 50H26L12 24V64ZM80 20H114V64H80L70 54V30ZM86 32L82 36V48L86 52H102V32Z" +
        "M124 20H137L146 44.7L155 20H168L152 64H140ZM178 20H190V64H178ZM178 2H190V14H178ZM210 20H234L244 30V54L234 " +
        "64H210L200 54V30ZM216 32L212 36V48L216 52H228L232 48V36L228 32Z",
).toPath().apply { fillType = PathFillType.EvenOdd }

/** The logo's gradient, from the top left to the bottom right of the grid. */
private val gradient = Brush.linearGradient(
    0f to Color(0xFFFFD34E),
    0.45f to Color(0xFFFF9A2E),
    1f to Color(0xFFF0412F),
    start = Offset.Zero,
    end = Offset(244f, 64f),
)

/** The Mavio wordmark in the logo's gradient, height tall. */
@Composable
fun Wordmark(modifier: Modifier = Modifier, height: Dp = 28.dp) {
    Canvas(
        modifier
            .height(height)
            .width(height * 244f / 64f)
            .semantics { contentDescription = "Mavio" },
    ) {
        scale(size.height / 64f, pivot = Offset.Zero) {
            drawPath(wordmarkPath, gradient)
        }
    }
}

@Preview
@Composable
private fun WordmarkPreview() {
    Wordmark()
}
