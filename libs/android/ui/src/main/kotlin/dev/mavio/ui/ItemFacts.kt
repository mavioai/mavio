package dev.mavio.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withLink
import androidx.compose.ui.unit.dp
import dev.mavio.core.CardKind
import dev.mavio.core.Details
import dev.mavio.core.SeriesStatus
import dev.mavio.core.Source
import dev.mavio.design.MavioSpacing
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** A day, as the user's locale writes it in medium length. */
fun itemDay(instant: Instant): String =
    DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withZone(ZoneId.systemDefault()).format(instant)

/** How many studios show until Show More. */
const val MIN_ITEM_STUDIOS = 3

/** The release, status, runtime, rating, genres, studios, countries, languages and when it was added. */
@Composable
fun aboutFacts(details: Details, studiosKey: String, allStudios: Boolean): List<Pair<String, String>> {
    val card = details.card
    val rows = mutableListOf<Pair<String, String?>>()
    if (card.kind == CardKind.MOVIE) {
        val releases = listOfNotNull(
            details.premiere?.let { "${stringResource(R.string.discover_theatrical_release)}  ${itemDay(it)}" },
            details.digitalRelease?.let { "${stringResource(R.string.item_digital_release)}  ${itemDay(it)}" },
            details.physicalRelease?.let { "${stringResource(R.string.item_physical_release)}  ${itemDay(it)}" },
        )
        rows += if (releases.size > 1) {
            stringResource(R.string.discover_release_dates) to releases.joinToString("\n")
        } else {
            stringResource(R.string.discover_release_date) to details.premiere?.let(::itemDay)
        }
    } else {
        val key = when (card.kind) {
            CardKind.SERIES -> R.string.item_first_aired
            CardKind.EPISODE -> R.string.item_aired
            else -> R.string.item_released
        }
        rows += stringResource(key) to details.premiere?.let(::itemDay)
    }
    if (card.kind == CardKind.SERIES) {
        rows += stringResource(R.string.item_status) to when (details.seriesStatus) {
            SeriesStatus.CONTINUING -> stringResource(R.string.item_continuing)
            SeriesStatus.ENDED -> stringResource(R.string.item_ended)
            else -> null
        }
        rows += stringResource(R.string.item_last_aired) to
            if (details.seriesStatus == SeriesStatus.ENDED) details.endDate?.let(::itemDay) else null
    }
    rows += stringResource(R.string.item_runtime) to
        if (card.kind == CardKind.SERIES) null else card.runtime?.let { minutesText(it.toMinutes()) }
    rows += stringResource(R.string.item_rated) to details.officialRating
    rows += stringResource(R.string.item_genres) to details.genres.takeIf { it.isNotEmpty() }?.joinToString(", ")
    rows += studiosKey to details.studios.takeIf { it.isNotEmpty() }
        ?.let { if (allStudios) it else it.take(MIN_ITEM_STUDIOS) }?.joinToString(", ")
    rows += stringResource(R.string.item_country) to details.countries.takeIf { it.isNotEmpty() }?.joinToString("\n")
    rows += stringResource(R.string.item_original_language) to details.originalLanguage?.let { languageName(it) ?: it }
    rows += stringResource(R.string.item_original_title) to details.originalTitle?.takeIf { it != card.title }
    rows += stringResource(R.string.item_date_added) to details.dateAdded?.let(::itemDay)
    return rows.mapNotNull { (k, v) -> v?.let { k to it } }
}

/** The chosen version's video, audio, subtitles and file. */
@Composable
fun mediaFacts(source: Source, context: android.content.Context): List<Pair<String, String>> {
    val video = source.video?.let { v -> listOfNotNull(source.resolution, v.codec.uppercase(), source.range).joinToString(" · ") }
    val audio = source.audio.takeIf { it.isNotEmpty() }?.joinToString("\n") { a ->
        val channels = when {
            a.channels == 6 -> "5.1"
            a.channels == 8 -> "7.1"
            a.channels > 0 -> "${a.channels} ch"
            else -> null
        }
        listOfNotNull(a.title ?: languageName(a.language), a.codec.uppercase(), channels).joinToString(" · ")
    }
    val subtitles = source.subtitles.takeIf { it.isNotEmpty() }?.joinToString("\n") { s ->
        listOfNotNull(s.title ?: languageName(s.language), s.codec.uppercase()).joinToString(" · ")
    }
    val file = listOfNotNull(source.fileName, source.sizeText(context)).joinToString("\n").ifEmpty { null }
    return listOf(
        stringResource(R.string.item_video) to video,
        stringResource(R.string.item_audio_tracks) to audio,
        stringResource(R.string.item_subtitle_tracks) to subtitles,
        stringResource(R.string.item_file) to file,
    ).mapNotNull { (k, v) -> v?.let { k to it } }
}

/** A rating, large, with its caption and scale. */
@Composable
fun RatingCard(label: String, scale: String?, value: @Composable () -> Unit) {
    val colors = theme.colors
    Column(
        Modifier
            .widthIn(min = 144.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(colors.fill)
            .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s12),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
    ) {
        Text(label, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
            value()
            scale?.let { Text(it, style = theme.type.callout.text, color = colors.textTertiary) }
        }
    }
}

/** Facts, each a caption over its value, two to a line. */
@Composable
fun FactList(
    rows: List<Pair<String, String>>,
    links: Map<String, List<Pair<String, (() -> Unit)?>>>,
    extra: @Composable (String) -> Unit,
) {
    if (rows.isEmpty()) return
    val colors = theme.colors
    FlowRow(maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s20)) {
        rows.forEach { (key, value) ->
            Column(Modifier.weight(1f).padding(end = MavioSpacing.s32), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                Text(key, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
                val values = links[key]
                if (values != null && values.any { it.second != null }) {
                    // Each value opening what it links to.
                    Text(
                        buildAnnotatedString {
                            values.forEachIndexed { i, (text, open) ->
                                if (i > 0) append(", ")
                                if (open == null) {
                                    append(text)
                                } else {
                                    withLink(LinkAnnotation.Clickable(text, TextLinkStyles(SpanStyle(color = colors.textPrimary))) { open() }) { append(text) }
                                }
                            }
                        },
                        style = theme.type.callout.text,
                        color = colors.textPrimary,
                    )
                } else {
                    Text(value, style = theme.type.callout.text)
                }
                extra(key)
            }
        }
    }
}
