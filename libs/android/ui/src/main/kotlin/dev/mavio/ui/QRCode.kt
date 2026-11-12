package dev.mavio.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel
import com.google.zxing.qrcode.encoder.Encoder

/**
 * The modules of the QR code of [text], encoded by ZXing at level M in
 * UTF-8 without a quiet zone: rows of them, true where dark.
 */
fun qrModules(text: String): List<List<Boolean>> {
    val matrix = Encoder.encode(text, ErrorCorrectionLevel.M, mapOf(EncodeHintType.CHARACTER_SET to "UTF-8")).matrix
    return List(matrix.height) { y -> List(matrix.width) { x -> matrix.get(x, y).toInt() == 1 } }
}

/**
 * A QR code, drawn dark on light whatever the theme, for a phone to scan,
 * in a white frame [quiet] wide that scanners need, rounded by [radius].
 */
@Composable
fun QRCode(
    dark: List<List<Boolean>>,
    modifier: Modifier = Modifier,
    quiet: Dp = 12.dp,
    radius: Dp = 8.dp,
    label: String = "QR",
) {
    val width = dark.maxOfOrNull { it.size } ?: 0
    Box(
        modifier
            .clip(RoundedCornerShape(radius))
            .background(Color.White)
            .padding(quiet)
            .semantics { contentDescription = label },
    ) {
        Canvas(Modifier.fillMaxWidth().aspectRatio(width.toFloat().coerceAtLeast(1f) / dark.size.coerceAtLeast(1))) {
            val module = minOf(size.width / width.coerceAtLeast(1), size.height / dark.size.coerceAtLeast(1))
            dark.forEachIndexed { y, row ->
                row.forEachIndexed { x, on ->
                    if (on) drawRect(Color.Black, Offset(x * module, y * module), Size(module + 0.5f, module + 0.5f))
                }
            }
        }
    }
}

/** The QR code of [text], as [QRCode] draws it. */
@Composable
fun QRCodeOf(text: String, modifier: Modifier = Modifier, quiet: Dp = 12.dp, radius: Dp = 8.dp) {
    val dark = remember(text) { runCatching { qrModules(text) }.getOrNull() } ?: return
    QRCode(dark, modifier, quiet, radius, label = text)
}
