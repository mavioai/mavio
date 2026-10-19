package dev.mavio.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.CreditEntry
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing

/** The shape of a card's artwork. */
enum class CardShape(val aspect: Float) {
    /** 2:3, of movies, series, seasons and collections. */
    Poster(2f / 3f),

    /** 16:9, of episodes, videos and items begun. */
    Thumb(16f / 9f),

    /** 1:1, of music and photos. */
    Square(1f),
}

/** The shape a kind of item's card takes. */
fun shapeOf(kind: CardKind): CardShape = when (kind) {
    CardKind.EPISODE, CardKind.VIDEO, CardKind.MUSIC_VIDEO -> CardShape.Thumb
    CardKind.ALBUM, CardKind.ARTIST, CardKind.TRACK, CardKind.PHOTO, CardKind.PHOTO_ALBUM -> CardShape.Square
    else -> CardShape.Poster
}

/** The icon of an item's kind, on its artwork's placeholder. */
@DrawableRes
fun itemIcon(kind: CardKind): Int = when (kind) {
    CardKind.SERIES, CardKind.SEASON, CardKind.EPISODE -> R.drawable.ic_shows
    CardKind.ARTIST, CardKind.ALBUM, CardKind.TRACK, CardKind.MUSIC_VIDEO -> R.drawable.ic_music
    CardKind.AUDIOBOOK, CardKind.BOOK -> R.drawable.ic_books
    CardKind.PHOTO_ALBUM, CardKind.PHOTO -> R.drawable.ic_photos
    CardKind.VIDEO -> R.drawable.ic_home_videos
    CardKind.FOLDER -> R.drawable.ic_folder
    CardKind.COLLECTION -> R.drawable.ic_collections
    else -> R.drawable.ic_movies
}

/**
 * An image loaded from a server, fading in over a plain fill of its final
 * size; with none, or when it fails, the placeholder stays, a quiet wash
 * with the icon of what it shows, so that no icon flashes before an image.
 * Cropped to its space at [alignment].
 */
@Composable
fun Artwork(
    url: String?,
    modifier: Modifier = Modifier,
    @DrawableRes icon: Int = R.drawable.ic_movies,
    contentDescription: String? = null,
    alignment: Alignment = Alignment.Center,
) {
    var failed by remember(url) { mutableStateOf(false) }
    val colors = theme.colors
    Box(modifier.background(colors.surfaceRaised)) {
        if (url == null || failed) {
            Box(
                Modifier
                    .fillMaxSize()
                    .background(Brush.linearGradient(listOf(colors.fill, Color.Transparent))),
                Alignment.Center,
            ) {
                MavioIcon(
                    icon,
                    Modifier.fillMaxWidth(0.26f).aspectRatio(1f),
                    tint = colors.textTertiary.copy(alpha = 0.6f),
                    contentDescription = contentDescription,
                )
            }
        } else {
            AsyncImage(
                model = rememberImageRequest(url),
                contentDescription = contentDescription,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop,
                alignment = alignment,
                onError = { failed = true },
            )
        }
    }
}

/**
 * A title's logo, fitted to the start of its space; without one, or when it
 * fails to load, the fallback, its name written out.
 */
@Composable
fun Logo(url: String?, name: String, modifier: Modifier = Modifier, fallback: @Composable () -> Unit) {
    var failed by remember(url) { mutableStateOf(false) }
    if (url == null || failed) {
        fallback()
    } else {
        AsyncImage(
            model = rememberImageRequest(url),
            contentDescription = name,
            modifier = modifier,
            contentScale = ContentScale.Fit,
            alignment = Alignment.CenterStart,
            onError = { failed = true },
        )
    }
}

/** The line under a card's title: its own subtitle, else its year. */
fun subtitleOf(card: Card): String? = card.subtitle ?: card.year?.toString()

/** A length of time as "1 h 21 min", as the web UI writes it. */
fun minutesText(minutes: Long): String =
    if (minutes >= 60) "${minutes / 60} h ${minutes % 60} min" else "$minutes min"

/** The card of an item: its artwork with watched progress and the played check, its title and subtitle. */
@Composable
fun ItemCard(
    card: Card,
    modifier: Modifier = Modifier,
    shape: CardShape = shapeOf(card.kind),
    width: Dp? = if (shape == CardShape.Thumb) theme.layout.thumbnail else theme.layout.poster,
) {
    val colors = theme.colors
    val type = theme.type
    Column(if (width != null) modifier.width(width) else modifier.fillMaxWidth()) {
        Box {
            val art = if (shape == CardShape.Thumb) card.thumb ?: card.poster else card.poster ?: card.thumb
            Artwork(
                art?.url,
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(shape.aspect)
                    .clip(RoundedCornerShape(MavioRadius.md)),
                icon = itemIcon(card.kind),
                contentDescription = card.title,
            )
            val progress = card.progress
            if (progress != null && progress > 0f && !card.played) {
                ProgressBar(
                    progress,
                    Modifier
                        .align(Alignment.BottomCenter)
                        .padding(10.dp)
                        .shadow(1.dp, CircleShape),
                )
            }
            if (card.played) {
                MavioIcon(
                    R.drawable.ic_played_fill,
                    Modifier
                        .align(Alignment.TopEnd)
                        .padding(6.dp)
                        .background(colors.scrim, CircleShape)
                        .padding(2.dp),
                    size = 18.dp,
                    tint = Color.White,
                )
            }
        }
        Spacer(Modifier.height(MavioSpacing.s8))
        Text(card.title, style = type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        subtitleOf(card)?.let {
            Text(
                it,
                style = type.footnote.text,
                color = colors.textSecondary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** A thin bar of how far an item was watched, white on a translucent track. */
@Composable
fun ProgressBar(progress: Float, modifier: Modifier = Modifier, color: Color = Color.White) {
    Box(
        modifier
            .fillMaxWidth()
            .height(4.dp)
            .clip(CircleShape)
            .background(color.copy(alpha = 0.35f)),
    ) {
        Box(
            Modifier
                .fillMaxWidth(progress.coerceIn(0f, 1f))
                .fillMaxHeight()
                .clip(CircleShape)
                .background(color),
        )
    }
}

/** A placeholder card while a shelf loads. */
@Composable
fun CardPlaceholder(shape: CardShape = CardShape.Poster) {
    val colors = theme.colors
    val width = if (shape == CardShape.Thumb) theme.layout.thumbnail else theme.layout.poster
    Column(Modifier.width(width)) {
        Box(
            Modifier
                .fillMaxWidth()
                .aspectRatio(shape.aspect)
                .clip(RoundedCornerShape(MavioRadius.md))
                .background(colors.surfaceRaised),
        )
        Spacer(Modifier.height(MavioSpacing.s8))
        Box(
            Modifier
                .fillMaxWidth(0.75f)
                .height(16.dp)
                .clip(RoundedCornerShape(MavioRadius.xs))
                .background(colors.surfaceRaised),
        )
    }
}

/** A cast or crew member: the photo in a circle, the name and the role. */
@Composable
fun PersonCard(credit: CreditEntry, modifier: Modifier = Modifier) {
    val colors = theme.colors
    val type = theme.type
    Column(modifier.width(100.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Box(
            Modifier
                .size(88.dp)
                .clip(CircleShape)
                .background(colors.surfaceRaised),
            Alignment.Center,
        ) {
            Text(initials(credit.name), style = type.title3.text, color = colors.textSecondary)
            credit.photo?.let { Artwork(it.url, Modifier.fillMaxSize(), contentDescription = credit.name) }
        }
        Spacer(Modifier.height(MavioSpacing.s4))
        Text(credit.name, style = type.callout.text, maxLines = 2, textAlign = TextAlign.Center)
        credit.role?.let {
            Text(it, style = type.footnote.text, color = colors.textSecondary, maxLines = 2, textAlign = TextAlign.Center)
        }
    }
}

private fun initials(name: String): String =
    name.split(" ").filter { it.isNotEmpty() }.take(2).joinToString("") { it.take(1) }
