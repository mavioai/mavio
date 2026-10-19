package dev.mavio.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalLocale
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.mavio.design.MavioSpacing
import dev.mavio.design.TabletLayout
import dev.mavio.ui.Artwork
import dev.mavio.ui.R
import dev.mavio.ui.available
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.tmdbId
import java.time.LocalDate
import java.time.Period
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import mavio.acquisition.v1.Discover.DiscoveredTitle
import mavio.acquisition.v1.Discover.GetCollectionResponse
import mavio.acquisition.v1.Discover.GetPersonResponse
import mavio.acquisition.v1.Discover.MediaStatus
import mavio.acquisition.v1.Discover.PersonTitle
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getCollectionRequest
import mavio.acquisition.v1.getPersonRequest

/** A date of the calendar as TMDB gives it, written out. */
private fun longDay(iso: String): String? = runCatching {
    LocalDate.parse(iso.take(10)).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG))
}.getOrNull()

/** Whole years between two calendar dates, to today when the second is empty. */
private fun age(from: String, to: String): Int? = runCatching {
    Period.between(LocalDate.parse(from.take(10)), if (to.isEmpty()) LocalDate.now() else LocalDate.parse(to.take(10))).years
}.getOrNull()

/**
 * A discovered person's page (docs/clients.md §6.3.2), as the web UI's on
 * a phone: their photo, dates, other names, links and biography, then the
 * movies and series they play in and work on, the department they are
 * known for first.
 */
@Composable
fun DiscoveredPersonScreen(route: Route.DiscoveredPerson) {
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        route.serverId,
        "GetPerson",
        remember(route.tmdbId) {
            getPersonRequest {
                tmdbId = route.tmdbId
                language = discoverLanguage
            }
        },
        GetPersonResponse::parseFrom,
        attempt,
    )
    Loaded(result, { !it.hasPerson() }, { attempt++ }) { r ->
        val p = r.person
        var kinds by rememberSaveable { mutableIntStateOf(0) }
        var more by rememberSaveable { mutableStateOf(false) }
        fun of(list: List<PersonTitle>) = list.filter {
            kinds == 0 || (it.title.mediaKind == MediaKind.MEDIA_KIND_SERIES) == (kinds == 2)
        }
        val facts = listOfNotNull(
            longDay(p.birthday)?.let { stringResource(R.string.person_born, it, age(p.birthday, p.deathday)?.toString().orEmpty()) },
            p.placeOfBirth.ifEmpty { null },
            longDay(p.deathday)?.let { stringResource(R.string.person_died, it) },
        )
        val language = LocalLocale.current.platformLocale.toLanguageTag()
        val links = listOfNotNull(
            p.externalIdsMap["tmdb"]?.let { ExternalLink("TMDB", "https://www.themoviedb.org/person/$it?language=$language") },
            p.externalIdsMap["imdb"]?.let { ExternalLink("IMDb", "https://www.imdb.com/name/$it") },
        )
        val cast = of(p.castList)
        val crew = of(p.crewList)
        val sections = listOf(R.string.person_appearances to cast, R.string.person_crew to crew)
        val ordered = if (p.knownForDepartment.isEmpty() || p.knownForDepartment == "Acting") sections else sections.reversed()
        TitleGrid(
            route.serverId,
            emptyList(),
            header = {
                line("header") {
                    // From 640 dp the photo stands beside the rest, as the web UI's.
                    val row = windowWidth() >= 640.dp
                    val align = if (row) TextAlign.Start else TextAlign.Center
                    val photo = @Composable {
                        Artwork(
                            p.imageUrl.ifEmpty { null }?.let { sized(it, 342) },
                            Modifier.size(if (row) 176.dp else 144.dp).clip(CircleShape),
                            icon = R.drawable.ic_profile,
                        )
                    }
                    val info = @Composable {
                        Text(p.name, Modifier.fillMaxWidth(), style = theme.type.title1.text, fontWeight = FontWeight.Bold, textAlign = align)
                        if (facts.isNotEmpty()) {
                            Text(facts.joinToString(" | "), Modifier.fillMaxWidth(), style = theme.type.callout.text, color = theme.colors.textSecondary, textAlign = align)
                        }
                        if (p.alsoKnownAsCount > 0) {
                            Text(
                                stringResource(R.string.person_also_known_as, p.alsoKnownAsList.joinToString(", ")),
                                Modifier.fillMaxWidth(),
                                style = theme.type.callout.text,
                                color = theme.colors.textSecondary,
                                textAlign = align,
                            )
                        }
                        if (links.isNotEmpty()) ExternalLinks(links, modifier = Modifier.fillMaxWidth(), start = row)
                        if (p.biography.isNotEmpty()) {
                            Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                                Text(p.biography, style = theme.type.body.text, maxLines = if (more) Int.MAX_VALUE else 6)
                                Text(
                                    stringResource(if (more) R.string.person_less else R.string.person_more),
                                    Modifier.clickable { more = !more },
                                    style = theme.type.callout.text,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.colors.accentText,
                                )
                            }
                        }
                        Box(Modifier.fillMaxWidth()) {
                            Segmented(
                                listOf(stringResource(R.string.person_all_kinds), stringResource(R.string.person_movies), stringResource(R.string.person_series)),
                                kinds,
                            ) { kinds = it }
                        }
                    }
                    if (row) {
                        Row(Modifier.fillMaxWidth().padding(top = MavioSpacing.s32), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24)) {
                            photo()
                            Column(Modifier.widthIn(max = 768.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) { info() }
                        }
                    } else {
                        Column(
                            Modifier.fillMaxWidth().padding(top = MavioSpacing.s32),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        ) {
                            photo()
                            info()
                        }
                    }
                }
                ordered.forEach { (label, list) -> personTitles(route.serverId, label, list) }
            },
        )
    }
}

/** A section of a person's titles under its heading and count, each with the person's role in it. */
private fun LazyGridScope.personTitles(serverId: String, label: Int, list: List<PersonTitle>) {
    if (list.isEmpty()) return
    line("section/$label") {
        Row(Modifier.padding(top = MavioSpacing.s16), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            Text(stringResource(label), style = theme.type.title3.text)
            Text(list.size.toString(), style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
    }
    items(list.size, key = { "$label/$it" }) { i -> TitleCard(serverId, list[i].title, width = null, role = list[i].role) }
}

/**
 * What the server has of a collection, as Seerr marks it: available when
 * it has every movie, partially available when some, requested when every
 * other movie is requested, blocklisted when every movie is.
 */
private fun collectionStatus(parts: List<DiscoveredTitle>): MediaStatus = when {
    parts.isEmpty() -> MediaStatus.MEDIA_STATUS_UNSPECIFIED
    parts.all { it.status == MediaStatus.MEDIA_STATUS_BLOCKLISTED } -> MediaStatus.MEDIA_STATUS_BLOCKLISTED
    parts.all { it.status == MediaStatus.MEDIA_STATUS_AVAILABLE } -> MediaStatus.MEDIA_STATUS_AVAILABLE
    parts.any { it.status.available() } -> MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE
    parts.all { it.status == MediaStatus.MEDIA_STATUS_REQUESTED || it.status == MediaStatus.MEDIA_STATUS_WANTED } -> MediaStatus.MEDIA_STATUS_REQUESTED
    else -> MediaStatus.MEDIA_STATUS_UNSPECIFIED
}

/**
 * A discovered collection's page (docs/clients.md §6.3.2), as the web UI's
 * on a phone, its requests left out: its backdrop, name, count and status,
 * its overview, then its movies.
 */
@Composable
fun DiscoveredCollectionScreen(route: Route.DiscoveredCollection) {
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        route.serverId,
        "GetCollection",
        remember(route.tmdbId) {
            getCollectionRequest {
                tmdbCollectionId = route.tmdbId
                language = discoverLanguage
            }
        },
        GetCollectionResponse::parseFrom,
        attempt,
    )
    Loaded(result, { !it.hasCollection() }, { attempt++ }) { r ->
        val c = r.collection
        val label = statusLabel(collectionStatus(c.partsList))
        // The wide layout's hero reaches the content's top and holds the overview.
        val wide = theme.layout == TabletLayout
        TitleGrid(
            route.serverId,
            c.partsList,
            top = 0.dp,
            header = {
                line("hero") {
                    // The hero reaches the screen's edges, past the grid's margins.
                    Box(Modifier.layoutWider(theme.layout.margin)) {
                        DiscoverHero(c.backdropUrl, c.posterUrl, COLLECTION_HERO) { big ->
                            Text(
                                c.name,
                                fontSize = if (big) 52.sp else 34.sp,
                                lineHeight = if (big) 55.sp else 36.sp,
                                fontWeight = FontWeight.Bold,
                                color = Color.White,
                            )
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    pluralStringResource(R.plurals.collection_movie_count, c.partsCount, c.partsCount),
                                    style = theme.type.callout.text,
                                    color = Color.White.copy(alpha = 0.8f),
                                )
                                label?.let { StatusBadge(stringResource(it)) }
                            }
                            if (wide && c.overview.isNotEmpty()) {
                                Text(
                                    c.overview,
                                    style = theme.type.body.text,
                                    color = Color.White.copy(alpha = 0.85f),
                                    maxLines = if (big) 4 else 2,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                        }
                    }
                }
                if (!wide && c.overview.isNotEmpty()) line("overview") { Text(c.overview, style = theme.type.body.text) }
                line("movies") { Text(stringResource(R.string.collection_movies), Modifier.padding(top = MavioSpacing.s8), style = theme.type.title3.text) }
            },
        )
    }
}

/** A discovered collection's hero in the wide layout, as the web UI's. */
private val COLLECTION_HERO = WideHero(0.6f, 440.dp, 600.dp)

/** Lays a grid's full-width item out wider by a margin on each side, to the screen's edges. */
private fun Modifier.layoutWider(margin: Dp): Modifier = layout { measurable, constraints ->
    val extra = margin.roundToPx() * 2
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-extra / 2, 0) }
}
