package dev.mavio.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dev.mavio.core.Row as HomeRow
import dev.mavio.core.RowKind
import dev.mavio.design.MavioSpacing

/**
 * A titled row of cards that scrolls sideways, with a badge after the
 * title and the more link at its end.
 */
@Composable
fun Shelf(
    title: String,
    modifier: Modifier = Modifier,
    badge: @Composable () -> Unit = {},
    more: (@Composable () -> Unit)? = null,
    content: LazyListScope.() -> Unit,
) {
    val layout = theme.layout
    Column(modifier.padding(vertical = MavioSpacing.s12)) {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = layout.margin),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            // The title gives way to the badge and the more link, which keep their width.
            Row(
                Modifier.weight(1f),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            ) {
                Text(
                    title,
                    Modifier.weight(1f, fill = false),
                    style = theme.type.title3.text,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                badge()
            }
            more?.invoke()
        }
        Spacer(Modifier.height(MavioSpacing.s8))
        LazyRow(
            contentPadding = PaddingValues(horizontal = layout.margin),
            // Wider than the gutter, as the web UI's shelves.
            horizontalArrangement = Arrangement.spacedBy(layout.gutter * 1.5f),
            content = content,
        )
    }
}

/** The empty, error and offline states of a screen, centered. */
@Composable
fun StatusView(
    title: String,
    modifier: Modifier = Modifier,
    icon: Int? = null,
    message: String? = null,
    action: Pair<String, () -> Unit>? = null,
) {
    val colors = theme.colors
    Box(modifier.fillMaxSize(), Alignment.Center) {
        Column(
            Modifier.padding(theme.layout.margin),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            icon?.let { MavioIcon(it, size = 48.dp, tint = colors.textTertiary) }
            Text(title, style = theme.type.title3.text, textAlign = TextAlign.Center)
            message?.let {
                Text(it, style = theme.type.body.text, color = colors.textSecondary, textAlign = TextAlign.Center)
            }
            action?.let { (label, run) -> MavioButton(label, run, kind = ButtonKind.Secondary) }
        }
    }
}

/** The title of a row of Home or of a library's Recommended tab, named by its kind, library and seed. */
@Composable
fun rowTitle(row: HomeRow): String {
    val library = row.libraryName.orEmpty()
    val seed = row.seedName.orEmpty()
    return when (row.kind) {
        RowKind.CONTINUE_WATCHING -> stringResource(R.string.home_continue_watching)
        RowKind.AVAILABLE_REQUESTS -> stringResource(R.string.home_available_requests)
        RowKind.RECENTLY_ADDED -> stringResource(R.string.home_latest, library)
        RowKind.RECENTLY_RELEASED -> stringResource(R.string.home_recently_released, library)
        RowKind.RECENTLY_AIRED -> stringResource(R.string.home_recently_aired, library)
        RowKind.TOP_RATED -> stringResource(R.string.home_top_rated, library)
        RowKind.TOP_UNPLAYED -> stringResource(R.string.home_top_unplayed, library)
        RowKind.START_WATCHING -> stringResource(R.string.home_start_watching, library)
        RowKind.REDISCOVER -> stringResource(R.string.home_rediscover, library)
        RowKind.BECAUSE_YOU_WATCHED -> stringResource(R.string.home_because_you_watched, seed)
        RowKind.BECAUSE_YOU_LIKE -> stringResource(R.string.home_because_you_like, seed)
        RowKind.MORE_IN_GENRE -> stringResource(R.string.home_more_in_genre, seed)
        RowKind.STARRING -> stringResource(R.string.home_starring, seed)
        RowKind.DIRECTED_BY -> stringResource(R.string.home_directed_by, seed)
        RowKind.TRENDING -> stringResource(R.string.home_trending)
        RowKind.SERVER -> row.serverName.orEmpty()
    }
}
