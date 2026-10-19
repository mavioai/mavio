package dev.mavio.app.tv

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.google.protobuf.MessageLite
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvChoice
import dev.mavio.ui.TvChoices
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.collectionStatus
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.longDayText
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleKey
import dev.mavio.ui.tmdbId
import dev.mavio.ui.yearsBetween
import mavio.acquisition.v1.Discover.GetCollectionResponse
import mavio.acquisition.v1.Discover.GetPersonResponse
import mavio.acquisition.v1.Discover.PersonTitle
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getCollectionRequest
import mavio.acquisition.v1.getPersonRequest

/**
 * A discovered page's response once loaded: a spinner while it loads, an
 * error with Retry, or empty when [missing] says the server found nothing.
 */
@Composable
private fun <T : MessageLite> DiscoveredLoaded(result: Result<T>?, missing: (T) -> Boolean, retry: () -> Unit, content: @Composable (T) -> Unit) {
    val found = result?.getOrNull()
    Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
        when {
            result == null -> CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
            found == null -> StatusView(
                stringResource(R.string.state_error),
                icon = R.drawable.ic_error,
                message = result.exceptionOrNull()?.message,
                action = stringResource(R.string.action_retry) to retry,
            )
            missing(found) -> StatusView(stringResource(R.string.state_empty), icon = R.drawable.ic_search)
            else -> content(found)
        }
    }
}

/** A grid of discovered titles as a catalog's, its [content] lines and cards. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun TitlesGrid(state: androidx.compose.foundation.lazy.grid.LazyGridState, content: LazyGridScope.() -> Unit) {
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyVerticalGrid(
            GridCells.Adaptive(theme.layout.poster),
            Modifier.fillMaxSize(),
            state = state,
            contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s48),
            horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
            content = content,
        )
    }
}

/**
 * A discovered person on a television (docs/tv.md §6.3), as the web UI's:
 * their photo, dates, other names and biography, which More shows whole,
 * the kinds of titles to show, where focus starts, then the movies and
 * series they play in and work on, the department they are known for
 * first. Their links to the web are left out.
 */
@Composable
fun PersonPage(page: Page.Person) {
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        page.serverId,
        "GetPerson",
        remember(page.tmdbId) {
            getPersonRequest {
                tmdbId = page.tmdbId
                language = discoverLanguage
            }
        },
        GetPersonResponse::parseFrom,
        attempt,
    )
    DiscoveredLoaded(result, { !it.hasPerson() }, { attempt++ }) { r ->
        val p = r.person
        val pages = LocalPages.current
        fun of(list: List<PersonTitle>) = list.filter {
            page.kinds == 0 || (it.title.mediaKind == MediaKind.MEDIA_KIND_SERIES) == (page.kinds == 2)
        }
        val facts = listOfNotNull(
            longDayText(p.birthday)?.let { stringResource(R.string.person_born, it, yearsBetween(p.birthday, p.deathday)?.toString().orEmpty()) },
            p.placeOfBirth.ifEmpty { null },
            longDayText(p.deathday)?.let { stringResource(R.string.person_died, it) },
        )
        val sections = listOf(R.string.person_appearances to of(p.castList), R.string.person_crew to of(p.crewList))
        val ordered = if (p.knownForDepartment.isEmpty() || p.knownForDepartment == "Acting") sections else sections.reversed()
        val kinds = listOf(R.string.person_all_kinds, R.string.person_movies, R.string.person_series)
        val choices = remember { List(3) { FocusRequester() } }
        LaunchedEffect(Unit) { page.focus.restore(choices[page.kinds]) }
        TitlesGrid(page.grid) {
            gridLine("header") {
                Row(Modifier.padding(top = MavioSpacing.s48, bottom = MavioSpacing.s8), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s40)) {
                    Artwork(
                        p.imageUrl.ifEmpty { null }?.let { sized(it, 342) },
                        Modifier.size(220.dp).clip(CircleShape),
                        icon = R.drawable.ic_profile,
                    )
                    Column(Modifier.widthIn(max = 1120.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                        Text(p.name, style = theme.type.title1.text, fontWeight = FontWeight.Bold)
                        if (facts.isNotEmpty()) {
                            Text(facts.joinToString(" | "), style = theme.type.callout.text, color = theme.colors.textSecondary)
                        }
                        if (p.alsoKnownAsCount > 0) {
                            Text(
                                stringResource(R.string.person_also_known_as, p.alsoKnownAsList.joinToString(", ")),
                                style = theme.type.callout.text,
                                color = theme.colors.textSecondary,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                        if (p.biography.isNotEmpty()) {
                            Text(p.biography, style = theme.type.body.text, maxLines = if (page.more) Int.MAX_VALUE else 6, overflow = TextOverflow.Ellipsis)
                            TvLink(
                                stringResource(if (page.more) R.string.person_less else R.string.person_more),
                                Modifier.focusKey(page.focus, "more"),
                                theme.colors.accentText,
                            ) { page.more = !page.more }
                        }
                        Box(Modifier.padding(top = MavioSpacing.s8)) {
                            TvChoices(choices[page.kinds]) {
                                kinds.forEachIndexed { i, label ->
                                    TvChoice(stringResource(label), page.kinds == i, Modifier.focusRequester(choices[i]), horizontal = 24.dp) { page.kinds = i }
                                }
                            }
                        }
                    }
                }
            }
            ordered.forEach { (label, list) ->
                if (list.isEmpty()) return@forEach
                gridLine("section/$label") {
                    Row(Modifier.padding(top = MavioSpacing.s16), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                        Text(stringResource(label), style = theme.type.title3.text)
                        Text(list.size.toString(), style = theme.type.callout.text, color = theme.colors.textSecondary)
                    }
                }
                items(list.size, key = { "$label/${titleKey(list[it].title)}/$it" }) { i ->
                    val t = list[i].title
                    TvTitleCard(t, Modifier.focusKey(page.focus, "$label/${titleKey(t)}/$i"), fill = true, role = list[i].role) {
                        pages.openTitle(page.serverId, t.mediaKindValue, t.tmdbId)
                    }
                }
            }
        }
    }
}

/**
 * A discovered collection on a television (docs/tv.md §6.3), as the web
 * UI's, its requests left out: its backdrop over 70 % of the screen's
 * height with its poster, its name at 72 points, its count, status and
 * two lines of its overview, then its movies, where focus starts.
 */
@Composable
fun CollectionPage(page: Page.Collection) {
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        page.serverId,
        "GetCollection",
        remember(page.tmdbId) {
            getCollectionRequest {
                tmdbCollectionId = page.tmdbId
                language = discoverLanguage
            }
        },
        GetCollectionResponse::parseFrom,
        attempt,
    )
    DiscoveredLoaded(result, { !it.hasCollection() }, { attempt++ }) { r ->
        val c = r.collection
        val pages = LocalPages.current
        val label = statusLabel(collectionStatus(c.partsList))
        val screen = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
        val first = remember { FocusRequester() }
        LaunchedEffect(Unit) { page.focus.restore(first) }
        TitlesGrid(page.grid) {
            gridLine("hero") {
                // The hero reaches the screen's edges, past the grid's margins.
                Box(Modifier.layoutWider(theme.layout.margin).fillMaxWidth().height(screen * 0.7f).background(theme.colors.surface)) {
                    if (c.backdropUrl.isNotEmpty()) Artwork(sized(c.backdropUrl, 1280), Modifier.fillMaxSize())
                    Box(
                        Modifier
                            .fillMaxSize()
                            .background(Brush.horizontalGradient(0f to Color.Black.copy(alpha = 0.82f), 0.45f to Color.Black.copy(alpha = 0.45f), 0.75f to Color.Transparent)),
                    )
                    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.75f))))
                    Row(
                        Modifier
                            .align(Alignment.BottomStart)
                            .padding(start = theme.layout.margin, end = theme.layout.margin, bottom = screen * 0.1f),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s40),
                        verticalAlignment = Alignment.Bottom,
                    ) {
                        if (c.posterUrl.isNotEmpty()) {
                            Artwork(
                                sized(c.posterUrl, 500),
                                Modifier
                                    .width(300.dp)
                                    .aspectRatio(2f / 3f)
                                    .shadow(24.dp, RoundedCornerShape(MavioRadius.lg))
                                    .clip(RoundedCornerShape(MavioRadius.lg)),
                            )
                        }
                        Column(Modifier.widthIn(max = 960.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                            Text(c.name, fontSize = 72.sp, lineHeight = 76.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 2, overflow = TextOverflow.Ellipsis)
                            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    pluralStringResource(R.plurals.collection_movie_count, c.partsCount, c.partsCount),
                                    style = theme.type.callout.text,
                                    color = Color.White.copy(alpha = 0.8f),
                                )
                                label?.let {
                                    Text(
                                        stringResource(it),
                                        Modifier.background(Color.White.copy(alpha = 0.2f), RoundedCornerShape(4.dp)).padding(horizontal = 6.dp),
                                        style = theme.type.caption.text,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color.White,
                                    )
                                }
                            }
                            if (c.overview.isNotEmpty()) {
                                Text(c.overview, style = theme.type.body.text, color = Color.White.copy(alpha = 0.85f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
            gridLine("movies") { Text(stringResource(R.string.collection_movies), Modifier.padding(top = MavioSpacing.s8), style = theme.type.title3.text) }
            items(c.partsCount, key = { titleKey(c.partsList[it]) }) { i ->
                val t = c.partsList[i]
                TvTitleCard(t, (if (i == 0) Modifier.focusRequester(first) else Modifier).focusKey(page.focus, titleKey(t)), fill = true) {
                    pages.openTitle(page.serverId, t.mediaKindValue, t.tmdbId)
                }
            }
        }
    }
}

/** Lays a grid's full-width item out wider by a margin on each side, to the screen's edges. */
private fun Modifier.layoutWider(margin: androidx.compose.ui.unit.Dp): Modifier = layout { measurable, constraints ->
    val extra = margin.roundToPx() * 2
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-extra / 2, 0) }
}
