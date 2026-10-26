package dev.mavio.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentSize
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import dev.mavio.core.Chapter
import dev.mavio.core.Trickplay

// The player's seek bar (docs/clients.md §6.5): its track with gaps where
// chapters start, and over a position the trickplay sheets' thumbnail with
// the chapter's name, as the web UI's.

/** The sheets of a width at least [wide] pixels, else the widest. */
fun chooseSheets(sheets: List<Trickplay>, wide: Int): Trickplay? =
    sheets.filter { it.width >= wide }.minByOrNull { it.width } ?: sheets.maxByOrNull { it.width }

/** The name of the chapter [ms] is in. */
fun chapterAt(chapters: List<Chapter>, ms: Long): String? =
    chapters.lastOrNull { it.start.toMillis() <= ms }?.name

/**
 * A seek bar's track [height] tall: what has played, [fraction] of it,
 * white, the rest faint, with a gap of 2 dp where each chapter after the
 * first starts, at [chapters], fractions of the whole.
 */
@Composable
fun ChapterTrack(fraction: Float, chapters: List<Float>, height: Dp, modifier: Modifier = Modifier) {
    Canvas(modifier.height(height)) {
        val gap = 1.dp.toPx()
        val starts = listOf(0f) + chapters.filter { it > 0f && it < 1f }.sorted()
        val radius = CornerRadius(size.height / 2)
        val played = fraction.coerceIn(0f, 1f) * size.width
        starts.forEachIndexed { i, from ->
            val left = from * size.width + if (i > 0) gap else 0f
            val right = (starts.getOrNull(i + 1) ?: 1f) * size.width - if (i + 1 < starts.size) gap else 0f
            if (right <= left) return@forEachIndexed
            drawRoundRect(Color.White.copy(alpha = 0.3f), Offset(left, 0f), Size(right - left, size.height), radius)
            val filled = played.coerceIn(left, right) - left
            if (filled > 0f) drawRoundRect(Color.White, Offset(left, 0f), Size(filled, size.height), radius)
        }
    }
}

/**
 * What a seek bar is over at [ms]: the thumbnail there from [sheets],
 * [width] wide, over the chapter's name and [time], in a dark box.
 */
@Composable
fun SeekPreview(sheets: Trickplay?, ms: Long, chapter: String?, time: String, width: Dp, style: TextStyle, modifier: Modifier = Modifier) {
    Column(
        modifier
            .clip(RoundedCornerShape(8.dp))
            .background(Color.Black.copy(alpha = 0.75f))
            .padding(start = 4.dp, end = 4.dp, top = 4.dp, bottom = 6.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        sheets?.let { TrickplayThumbnail(it, ms, width) }
        Text(
            listOfNotNull(chapter, time).joinToString(" · "),
            Modifier.width(width),
            style = style,
            fontWeight = FontWeight.Medium,
            color = Color.White,
            textAlign = TextAlign.Center,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/**
 * The thumbnail of [ms] from [sheets], [width] wide: its sheet, which the
 * image loader keeps, drawn so that the thumbnail alone shows.
 */
@Composable
fun TrickplayThumbnail(sheets: Trickplay, ms: Long, width: Dp) {
    val interval = sheets.interval.toMillis().coerceAtLeast(1)
    val index = (ms / interval).toInt().coerceIn(0, (sheets.count - 1).coerceAtLeast(0))
    val perSheet = sheets.columns * sheets.rows
    val tile = index % perSheet
    val height = width * sheets.height / sheets.width.coerceAtLeast(1)
    Box(Modifier.size(width, height).clip(RoundedCornerShape(6.dp))) {
        AsyncImage(
            "${sheets.url}/${index / perSheet}.jpg",
            null,
            Modifier
                .wrapContentSize(Alignment.TopStart, unbounded = true)
                .requiredWidth(width * sheets.columns)
                .offset(-width * (tile % sheets.columns), -height * (tile / sheets.columns)),
            alignment = Alignment.TopStart,
            contentScale = ContentScale.FillWidth,
        )
    }
}
