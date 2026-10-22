package dev.mavio.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalLocale
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.design.MavioSpacing
import java.util.Locale
import mavio.acquisition.v1.Discover.GetRatingsResponse
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getRatingsRequest
import mavio.plugin.v1.MetadataOuterClass.WatchProvider
import mavio.plugin.v1.Ratings.Rating
import mavio.plugin.v1.Ratings.RatingSource

/** A title's ratings elsewhere, as its ratings and its links show them; null while asked. */
@Composable
fun titleRatings(serverId: String, kind: MediaKind, tmdbId: Int): List<Rating>? {
    val result by discovered(
        serverId,
        "GetRatings",
        remember(kind, tmdbId) {
            getRatingsRequest {
                mediaKind = kind
                this.tmdbId = tmdbId
            }
        },
        GetRatingsResponse::parseFrom,
    )
    return result?.let { r -> r.getOrNull()?.ratingsList.orEmpty() }
}

/** Rotten Tomatoes' least fresh score, and least upright audience score. */
private const val FRESH_SCORE = 60.0

/** The ratings shown, in Seerr's order. */
private val ratingOrder = listOf(RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_CRITICS, RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE, RatingSource.RATING_SOURCE_IMDB)

/** Of a title's [ratings], those [TitleRatings] shows. */
fun shownRatings(ratings: List<Rating>): List<Rating> = ratingOrder.mapNotNull { s -> ratings.firstOrNull { it.source == s } }

/**
 * A title's ratings as Seerr's, each with its site's logo and, but on a
 * television, opening its page: Rotten Tomatoes' and IMDb's, then TMDB's
 * [score] out of 10 when given; centered unless [start].
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun TitleRatings(kind: MediaKind, tmdbId: Int, ratings: List<Rating>, score: Double?, modifier: Modifier = Modifier, start: Boolean = false) {
    val uri = LocalUriHandler.current
    // A television opens no web pages.
    val opens = !LocalTelevision.current
    val language = LocalLocale.current.platformLocale.toLanguageTag()
    val series = kind == MediaKind.MEDIA_KIND_SERIES
    FlowRow(
        modifier,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16, if (start) Alignment.Start else Alignment.CenterHorizontally),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
        itemVerticalAlignment = Alignment.CenterVertically,
    ) {
        shownRatings(ratings).forEach { r ->
            val fresh = r.score >= FRESH_SCORE
            val (logo, label, text) = when (r.source) {
                RatingSource.RATING_SOURCE_IMDB ->
                    Triple(R.drawable.service_imdb, "IMDb", String.format(Locale.getDefault(), "%.1f", r.score))
                RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_CRITICS ->
                    Triple(if (fresh) R.drawable.service_rt_fresh else R.drawable.service_rt_rotten, stringResource(R.string.discover_tomatometer), "${Math.round(r.score)}%")
                else ->
                    Triple(if (fresh) R.drawable.service_rt_aud_fresh else R.drawable.service_rt_aud_rotten, stringResource(R.string.discover_audience_score), "${Math.round(r.score)}%")
            }
            RatingLink(logo, label, text, 20.dp, if (opens && r.url.isNotEmpty()) ({ runCatching { uri.openUri(r.url) } }) else null)
        }
        if (score != null) {
            RatingLink(
                R.drawable.service_tmdb_wide,
                "TMDB",
                "${Math.round(score * 10)}%",
                16.dp,
                if (opens) ({ runCatching { uri.openUri("https://www.themoviedb.org/${if (series) "tv" else "movie"}/$tmdbId?language=$language") } }) else null,
            )
        }
    }
}

@Composable
private fun RatingLink(logo: Int, label: String, score: String, height: Dp, onClick: (() -> Unit)?) {
    Row(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier, horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
        Image(painterResource(logo), label, Modifier.height(height), contentScale = ContentScale.Fit)
        Text(score, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
    }
}

/** The services streaming a title, under their caption, each by its logo. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun WatchProviders(providers: List<WatchProvider>, modifier: Modifier = Modifier) {
    if (providers.isEmpty()) return
    Column(modifier, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Text(stringResource(R.string.item_streaming), style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            providers.forEach { p ->
                if (p.logoUrl.isNotEmpty()) {
                    Artwork(p.logoUrl, Modifier.size(32.dp).clip(RoundedCornerShape(6.dp)), contentDescription = p.name)
                } else {
                    Text(p.name, style = theme.type.footnote.text)
                }
            }
        }
    }
}
