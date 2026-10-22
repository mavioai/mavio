package dev.mavio.app

import android.net.Uri
import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.mavio.admin.reason
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.design.TabletLayout
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.CatalogRow
import dev.mavio.ui.Credit
import dev.mavio.ui.DiscoverPullToRefresh
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NamedValue
import dev.mavio.ui.R
import dev.mavio.ui.Reload
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.TitleRatings
import dev.mavio.ui.WatchProviders
import dev.mavio.ui.Wordmark
import dev.mavio.ui.blurAvailable
import dev.mavio.ui.countryText
import dev.mavio.ui.crewLabels
import dev.mavio.ui.crewRank
import dev.mavio.ui.dayText
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.languageName
import dev.mavio.ui.line
import dev.mavio.ui.minutesText
import dev.mavio.ui.moneyText
import dev.mavio.ui.pageTitle
import dev.mavio.ui.productionStatus
import dev.mavio.ui.rememberReload
import dev.mavio.ui.seasonName
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.titleRatings
import dev.mavio.ui.theme
import dev.mavio.ui.titleCredits
import java.util.Locale
import mavio.acquisition.v1.Discover.GetCollectionResponse
import mavio.acquisition.v1.Discover.GetSeasonResponse
import mavio.acquisition.v1.Discover.GetTitleResponse
import mavio.acquisition.v1.Discover.SeasonStatus
import mavio.acquisition.v1.Discover.Title
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getCollectionRequest
import mavio.acquisition.v1.getSeasonRequest
import mavio.acquisition.v1.getTitleRequest
import mavio.library.v1.ItemOuterClass.CreditKind
import mavio.plugin.v1.Discovery.CatalogKind
import mavio.plugin.v1.MetadataOuterClass.ProductionStatus
import mavio.plugin.v1.Ratings.Rating
import mavio.plugin.v1.Ratings.RatingSource

/** A discovered title as GetTitle answers, null while it is on its way. */
@Composable
internal fun discoveredTitle(serverId: String, mediaKind: Int, tmdbId: Int, reload: Reload? = null) = discovered(
    serverId,
    "GetTitle",
    remember(mediaKind, tmdbId) {
        getTitleRequest {
            mediaKindValue = mediaKind
            this.tmdbId = tmdbId
            language = discoverLanguage
        }
    },
    GetTitleResponse::parseFrom,
    reload = reload,
)

/**
 * What a page shows while what it shows loads, or why it can't; once
 * loaded, read again from [serverId] by pulling it down.
 */
@Composable
internal fun <T> Loaded(result: Result<T>?, missing: (T) -> Boolean, serverId: String, reload: Reload, content: @Composable (T) -> Unit) {
    val value = result?.getOrNull()
    when {
        result == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
        value == null -> StatusView(
            stringResource(R.string.state_error),
            Modifier.padding(top = LocalTopInset.current),
            icon = R.drawable.ic_error,
            message = result.exceptionOrNull()?.reason,
            action = stringResource(R.string.action_retry) to reload::retry,
        )
        missing(value) -> StatusView(stringResource(R.string.state_empty), Modifier.padding(top = LocalTopInset.current), icon = R.drawable.ic_search)
        else -> DiscoverPullToRefresh(serverId, reload, top = LocalTopInset.current) { content(value) }
    }
}

/** A discovered title (docs/clients.md §6.3.2), as the web UI's on a phone, its requests left out. */
@Composable
fun TitleScreen(route: Route.Title) {
    val reload = rememberReload()
    val result by discoveredTitle(route.serverId, route.mediaKind, route.tmdbId, reload)
    Loaded(result, { !it.hasTitle() }, route.serverId, reload) { TitleDetails(route.serverId, it.title) }
}

/** Opens the popular titles of a kind filtered alike; null without such a catalog. */
@Composable
private fun alike(serverId: String, kind: MediaKind): ((CatalogFilters) -> Unit)? {
    val navigator = LocalNavigator.current
    val popular = LocalDiscover.current.discovery?.catalogs?.firstOrNull {
        it.kind == CatalogKind.CATALOG_KIND_POPULAR && kind in it.mediaKindsList
    } ?: return null
    return { f -> navigator.open(Route.Catalog(serverId, popular.id, kind.number, 0, f)) }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleDetails(serverId: String, title: Title) {
    val navigator = LocalNavigator.current
    val uri = LocalUriHandler.current
    val catalogs = LocalDiscover.current.discovery?.catalogs.orEmpty()
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val credits = remember(title) { titleCredits(title.creditsList) }
    // The wide layout's hero reaches the content's top and holds the
    // actions; where the window is 1024 dp wide the facts stand beside the
    // overview, as the web UI's.
    val wide = theme.layout == TabletLayout
    val columns = wide && windowWidth() >= 1024.dp
    val margin = theme.layout.margin
    val actions = @Composable {
        // On a phone the actions fill their rows, as many on each as their names fit.
        FlowRow(
            if (wide) Modifier.padding(top = MavioSpacing.s8) else Modifier.padding(start = margin, end = margin, top = MavioSpacing.s16),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            val share = if (wide) Modifier else Modifier.weight(1f)
            if (title.itemId.isNotEmpty()) {
                MavioButton(
                    stringResource(R.string.discover_watch),
                    { navigator.open(Route.Item(serverId, title.itemId)) },
                    share,
                    icon = R.drawable.ic_play_fill,
                    large = true,
                )
            }
            title.trailerUrlsList.firstOrNull()?.let { trailer ->
                MavioButton(
                    stringResource(R.string.item_trailer),
                    { runCatching { uri.openUri(trailer) } },
                    share,
                    kind = if (wide) ButtonKind.Glass else ButtonKind.Secondary,
                    icon = R.drawable.ic_movies,
                    large = true,
                )
            }
        }
    }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = LocalBottomInset.current + MavioSpacing.s40)) {
        item(key = "header") { TitleHeader(serverId, title, if (wide) actions else null) }
        if (!wide) item(key = "actions") { actions() }
        item(key = "overview") {
            if (columns) {
                Row(Modifier.padding(horizontal = margin), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s32)) {
                    TitleOverview(serverId, title, credits, Modifier.weight(1f), collection = false)
                    Column(Modifier.width(380.dp).padding(top = MavioSpacing.s24), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                        CollectionCard(serverId, title)
                        TitleFacts(serverId, title, Modifier)
                    }
                }
            } else {
                // A column holds the facts under the overview, as the web UI's.
                Column(Modifier.padding(horizontal = margin), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24)) {
                    TitleOverview(serverId, title, credits, Modifier)
                    TitleFacts(serverId, title, Modifier.padding(bottom = MavioSpacing.s12))
                }
            }
        }
        if (series && title.seasonsCount > 0) item(key = "seasons") { Seasons(serverId, title.tmdbId, title.seasonsList) }
        item(key = "cast") {
            val cast = credits.filter { it.isCast }.sortedBy { it.order }.take(20)
            if (cast.isNotEmpty()) {
                Shelf(
                    stringResource(R.string.item_cast),
                    more = { MoreLink(stringResource(R.string.credits_all)) { navigator.open(Route.TitleCredits(serverId, title.mediaKindValue, title.tmdbId, crew = false)) } },
                ) {
                    items(cast.size) { i -> CreditTile(serverId, cast[i], cast[i].role, Modifier.width(112.dp)) }
                }
            }
        }
        val similar = catalogs.filter {
            (it.kind == CatalogKind.CATALOG_KIND_RECOMMENDATIONS || it.kind == CatalogKind.CATALOG_KIND_SIMILAR) && title.mediaKind in it.mediaKindsList
        }
        items(similar, key = { "catalog/${it.id}" }) { c ->
            CatalogShelf(
                serverId,
                CatalogRow(c, title.mediaKind),
                stringResource(if (c.kind == CatalogKind.CATALOG_KIND_RECOMMENDATIONS) R.string.discover_recommendations else R.string.discover_similar),
                tmdbId = title.tmdbId,
            )
        }
    }
}

/**
 * The hero of a discovered title: its backdrop, else its poster blurred,
 * and over it the title, its facts and its status, then [actions] where
 * the hero holds them.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleHeader(serverId: String, title: Title, actions: (@Composable () -> Unit)?) {
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val open = alike(serverId, title.mediaKind)
    DiscoverHero(title.backdropUrl, title.posterUrl, TITLE_HERO) { big ->
        Text(
            title.name,
            Modifier.pageTitle(title.name),
            fontSize = if (big) 52.sp else 34.sp,
            lineHeight = if (big) 55.sp else 36.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        val style = theme.type.callout.text
        val faint = Color.White.copy(alpha = 0.8f)
        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
            verticalArrangement = Arrangement.spacedBy(6.dp),
            itemVerticalAlignment = Alignment.CenterVertically,
        ) {
            val facts = listOfNotNull(
                title.year.takeIf { it > 0 }?.toString(),
                stringResource(if (series) R.string.discover_series else R.string.discover_movie),
                title.officialRating.ifEmpty { null },
                title.runtimeMinutes.takeIf { it > 0 }?.let { minutesText(it.toLong()) },
            )
            facts.forEachIndexed { i, f -> Text(if (i > 0) "· $f" else f, style = style, color = faint) }
            val genres = title.genreRefsList.map { it.id to it.name }.ifEmpty { title.genresList.map { "" to it } }.take(3)
            genres.forEachIndexed { i, (id, name) ->
                val prefix = if (i == 0) (if (facts.isNotEmpty()) "· " else "") else "/ "
                Text(
                    prefix + name,
                    if (id.isNotEmpty() && open != null) Modifier.clickable { open(CatalogFilters(genres = listOf(id))) } else Modifier,
                    style = style,
                    color = faint,
                )
            }
            statusLabel(title.status)?.let { StatusBadge(stringResource(it)) }
        }
        actions?.invoke()
    }
}

/** A title's status over its artwork. */
@Composable
internal fun StatusBadge(label: String) {
    Text(
        label,
        Modifier
            .padding(start = MavioSpacing.s8)
            .background(Color.White.copy(alpha = 0.2f), RoundedCornerShape(4.dp))
            .padding(horizontal = 6.dp),
        style = theme.type.caption.text,
        fontWeight = FontWeight.SemiBold,
        color = Color.White,
    )
}

/** The window's width, which the web UI's breakpoints are of. */
@Composable
internal fun windowWidth(): Dp = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.width.toDp() }

/** How high a discover hero is in the wide layout: a share of the window's height, between two bounds. */
internal class WideHero(val share: Float, val min: Dp, val max: Dp)

/** A discovered title's hero in the wide layout, as the web UI's. */
private val TITLE_HERO = WideHero(0.7f, 520.dp, 700.dp)

/**
 * A discover hero, its artwork the backdrop, else the poster blurred, and
 * over its lower part, darkened, the content, told whether its title is
 * large. On a phone a square as wide as the screen, right below the
 * header, its artwork's colors spreading up into the header; in the wide
 * layout as high as [wide] says but no taller than the window, shaded from
 * the left, with the poster beside the content, as the web UI's.
 */
@Composable
internal fun DiscoverHero(backdrop: String, poster: String, wide: WideHero, content: @Composable (big: Boolean) -> Unit) {
    val art: @Composable () -> Unit = {
        when {
            backdrop.isNotEmpty() -> Artwork(sized(backdrop, 1280), Modifier.fillMaxSize())
            poster.isNotEmpty() -> Artwork(
                sized(poster, 342),
                Modifier
                    .fillMaxSize()
                    .graphicsLayer {
                        scaleX = 1.25f
                        scaleY = 1.25f
                    }
                    .then(if (blurAvailable) Modifier.blur(48.dp) else Modifier)
                    .alpha(0.7f),
            )
        }
    }
    if (theme.layout == TabletLayout) {
        val window = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
        val height = (window * wide.share).coerceIn(wide.min, wide.max).coerceAtMost(window)
        val short = height < wide.min
        Box(Modifier.fillMaxWidth().height(height).clipToBounds().background(theme.colors.surface)) {
            art()
            Box(
                Modifier
                    .fillMaxSize()
                    .background(Brush.horizontalGradient(0f to Color.Black.copy(alpha = 0.82f), 0.45f to Color.Black.copy(alpha = 0.45f), 0.75f to Color.Transparent)),
            )
            Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.75f))))
            Row(
                Modifier
                    .align(Alignment.BottomStart)
                    .padding(horizontal = MavioSpacing.s40, vertical = if (short) MavioSpacing.s24 else MavioSpacing.s40),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s40),
                verticalAlignment = Alignment.Bottom,
            ) {
                if (poster.isNotEmpty()) {
                    Artwork(
                        sized(poster, 500),
                        Modifier
                            .width(if (short) 140.dp else 200.dp)
                            .aspectRatio(2f / 3f)
                            .shadow(12.dp, RoundedCornerShape(8.dp))
                            .clip(RoundedCornerShape(8.dp)),
                    )
                }
                Column(Modifier.widthIn(max = 640.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) { content(!short) }
            }
        }
        return
    }
    PhoneHero(1f) {
        Box(Modifier.matchParentSize().clipToBounds().background(theme.colors.surface)) {
            art()
            Box(
                Modifier
                    .fillMaxSize()
                    .background(Brush.verticalGradient(0f to Color.Transparent, 0.5f to Color.Black.copy(alpha = 0.5f), 1f to Color.Black)),
            )
            Column(
                Modifier
                    .align(Alignment.BottomStart)
                    .padding(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s24)
                    .widthIn(max = 640.dp),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            ) { content(false) }
        }
    }
}

// Overview

/**
 * The tagline, the overview, the leading crew as Seerr's (six of them, a
 * series' creators first), the link to the whole crew, the keywords, and
 * the collection a movie belongs to.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleOverview(serverId: String, title: Title, credits: List<Credit>, modifier: Modifier, collection: Boolean = true) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    val open = alike(serverId, title.mediaKind)
    Column(
        modifier.padding(top = MavioSpacing.s24, bottom = MavioSpacing.s12),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        if (title.tagline.isNotEmpty()) {
            Text(title.tagline, style = theme.type.title3.text, fontWeight = FontWeight.Normal, fontStyle = FontStyle.Italic, color = colors.textSecondary)
        }
        Text(stringResource(R.string.discover_overview), style = theme.type.title3.text)
        Text(title.overview.ifEmpty { stringResource(R.string.discover_no_overview) }, style = theme.type.body.text)
        val crew = credits.withIndex()
            .filter { (_, c) -> !c.isCast && crewLabels.any { it.first == c.kind } }
            .sortedWith(compareBy({ crewRank(it.value) }, { it.index }))
            .take(6)
            .map { it.value }
        if (crew.isNotEmpty()) {
            FlowRow(Modifier.padding(top = MavioSpacing.s8), maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                crew.forEach { c ->
                    Column(Modifier.weight(1f).padding(end = MavioSpacing.s16)) {
                        Text(
                            if (c.kind == CreditKind.CREDIT_KIND_OTHER && c.role.isNotEmpty()) c.role else stringResource(crewLabels.first { it.first == c.kind }.second),
                            style = theme.type.headline.text,
                        )
                        Text(
                            c.name,
                            if (c.tmdbId > 0) Modifier.clickable { navigator.open(Route.DiscoveredPerson(serverId, c.tmdbId)) } else Modifier,
                            style = theme.type.callout.text,
                            color = colors.textSecondary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
            Row(
                Modifier
                    .align(Alignment.End)
                    .clip(RoundedCornerShape(MavioRadius.xs))
                    .clickable { navigator.open(Route.TitleCredits(serverId, title.mediaKindValue, title.tmdbId, crew = true)) }
                    .padding(4.dp),
                horizontalArrangement = Arrangement.spacedBy(4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(stringResource(R.string.discover_full_crew), style = theme.type.callout.text, color = colors.textSecondary)
                MavioIcon(R.drawable.ic_forward, size = 16.dp, tint = colors.textSecondary)
            }
        }
        val keywords = title.keywordRefsList.map { it.id to it.name }.ifEmpty { title.keywordsList.map { "" to it } }
        if (keywords.isNotEmpty()) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                keywords.forEach { (id, name) ->
                    Text(
                        name,
                        Modifier
                            .clip(CircleShape)
                            .background(colors.fill)
                            .then(
                                if (id.isNotEmpty() && open != null) {
                                    Modifier.clickable { open(CatalogFilters(keywords = listOf(NamedValue(id, name)))) }
                                } else {
                                    Modifier
                                },
                            )
                            .padding(horizontal = 12.dp, vertical = 4.dp),
                        style = theme.type.footnote.text,
                    )
                }
            }
        }
        if (collection) CollectionCard(serverId, title)
    }
}

/** A movie's collection as Seerr's card: its backdrop and name, opening its page. */
@Composable
private fun CollectionCard(serverId: String, title: Title) {
    val id = title.externalIdsMap["tmdb_collection"]?.toIntOrNull() ?: 0
    if (id <= 0) return
    val navigator = LocalNavigator.current
    val result by discovered(
        serverId,
        "GetCollection",
        remember(id) {
            getCollectionRequest {
                tmdbCollectionId = id
                language = discoverLanguage
            }
        },
        GetCollectionResponse::parseFrom,
    )
    val collection = result?.getOrNull()?.collection
    val backdrop = collection?.backdropUrl?.ifEmpty { collection.posterUrl }.orEmpty()
    Box(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 80.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(theme.colors.surfaceRaised)
            .clickable { navigator.open(Route.DiscoveredCollection(serverId, id)) },
    ) {
        if (backdrop.isNotEmpty()) Artwork(sized(backdrop, 780), Modifier.matchParentSize())
        Box(Modifier.matchParentSize().background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.45f), Color.Black.copy(alpha = 0.8f)))))
        Row(
            Modifier.fillMaxWidth().heightIn(min = 80.dp).padding(16.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                collection?.name?.ifEmpty { null } ?: title.collectionName.ifEmpty { stringResource(R.string.item_collection) },
                Modifier.weight(1f),
                style = theme.type.headline.text,
                color = Color.White,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                stringResource(R.string.collection_view),
                Modifier.background(Color.White.copy(alpha = 0.2f), CircleShape).padding(horizontal = 12.dp, vertical = 4.dp),
                style = theme.type.footnote.text,
                fontWeight = FontWeight.SemiBold,
                color = Color.White,
            )
        }
    }
}

// Facts

/** The studios shown before Show More. */
private const val MIN_STUDIOS = 5

/** A row of the facts: its label, and its values on the right. */
@Composable
private fun Fact(label: String, values: @Composable () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        Text(label, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
        Column(Modifier.weight(1f), horizontalAlignment = Alignment.End) { values() }
    }
}

/** A value of the facts, opening what it names when it can. */
@Composable
private fun FactValue(text: String, onClick: (() -> Unit)? = null) {
    Text(
        text,
        if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier,
        style = theme.type.callout.text,
        color = theme.colors.textSecondary,
        textAlign = TextAlign.End,
    )
}

/**
 * A title's facts as Seerr's: the ratings, original title, status, dates,
 * runtime, money, language, countries, studios or networks and where it
 * streams, each language, studio and network opening the popular titles
 * alike, and the links to it elsewhere.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleFacts(serverId: String, title: Title, modifier: Modifier) {
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val open = alike(serverId, title.mediaKind)
    val navigator = LocalNavigator.current
    var allStudios by rememberSaveable { mutableStateOf(false) }
    val colors = theme.colors
    val rows = mutableListOf<@Composable () -> Unit>()
    val ratings = titleRatings(serverId, title.mediaKind, title.tmdbId)
    rows += {
        TitleRatings(
            title.mediaKind,
            title.tmdbId,
            ratings.orEmpty(),
            title.communityRating.takeIf { title.voteCount > 0 },
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        )
    }
    if (title.originalTitle.isNotEmpty() && title.originalLanguage != discoverLanguage) {
        rows += { Fact(stringResource(R.string.item_original_title)) { FactValue(title.originalTitle) } }
    }
    productionStatus(title.productionStatus)?.let { s -> rows += { Fact(stringResource(R.string.item_status)) { FactValue(s) } } }
    val releases = if (series) {
        emptyList()
    } else {
        listOfNotNull(
            dayText(title.theatricalReleaseDate)?.let { R.string.discover_theatrical_release to it },
            dayText(title.digitalReleaseDate)?.let { R.string.item_digital_release to it },
            dayText(title.physicalReleaseDate)?.let { R.string.item_physical_release to it },
        )
    }
    if (releases.isNotEmpty()) {
        rows += {
            Fact(stringResource(if (releases.size == 1) R.string.discover_release_date else R.string.discover_release_dates)) {
                releases.forEach { (kind, day) -> FactValue("${stringResource(kind)}: $day") }
            }
        }
    } else {
        dayText(title.premiereDate)?.let { day ->
            rows += { Fact(stringResource(if (series) R.string.item_first_aired else R.string.item_released)) { FactValue(day) } }
        }
    }
    if (series && title.productionStatus == ProductionStatus.PRODUCTION_STATUS_ENDED) {
        dayText(title.endDate)?.let { day -> rows += { Fact(stringResource(R.string.item_last_aired)) { FactValue(day) } } }
    }
    if (series) dayText(title.nextAirDate)?.let { day -> rows += { Fact(stringResource(R.string.item_next_air_date)) { FactValue(day) } } }
    if (title.runtimeMinutes > 0) {
        rows += {
            Fact(stringResource(if (series) R.string.discover_episode_runtime else R.string.item_runtime)) {
                FactValue(minutesText(title.runtimeMinutes.toLong()))
            }
        }
    }
    moneyText(title.revenue)?.let { m -> rows += { Fact(stringResource(R.string.item_revenue)) { FactValue(m) } } }
    moneyText(title.budget)?.let { m -> rows += { Fact(stringResource(R.string.item_budget)) { FactValue(m) } } }
    if (title.originalLanguage.isNotEmpty()) {
        rows += {
            Fact(stringResource(R.string.item_original_language)) {
                FactValue(languageName(title.originalLanguage) ?: title.originalLanguage, open?.let { { it(CatalogFilters(originalLanguage = title.originalLanguage)) } })
            }
        }
    }
    val countries = title.productionCountriesList.map(::countryText).ifEmpty { title.productionLocationsList }
    if (countries.isNotEmpty()) rows += { Fact(stringResource(R.string.item_country)) { countries.forEach { FactValue(it) } } }
    val studios = title.studioRefsList.map { NamedValue(it.id, it.name) }.ifEmpty { title.studiosList.map { NamedValue("", it) } }
    if (studios.isNotEmpty()) {
        rows += {
            Fact(stringResource(if (series) R.string.item_networks else R.string.item_studio)) {
                (if (allStudios) studios else studios.take(MIN_STUDIOS)).forEach { s ->
                    FactValue(
                        s.name,
                        if (s.id.isNotEmpty() && open != null) {
                            { open(if (series) CatalogFilters(networks = listOf(s)) else CatalogFilters(studios = listOf(s))) }
                        } else {
                            null
                        },
                    )
                }
                if (studios.size > MIN_STUDIOS) {
                    Text(
                        stringResource(if (allStudios) R.string.action_show_less else R.string.discover_show_more),
                        Modifier.clickable { allStudios = !allStudios },
                        style = theme.type.callout.text,
                        color = colors.accentText,
                    )
                }
            }
        }
    }
    if (title.watchProvidersCount > 0) rows += { WatchProviders(title.watchProvidersList, Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp)) }
    rows += {
        ExternalLinks(
            titleLinks(
                title.mediaKind,
                title.tmdbId,
                title.externalIdsMap["imdb"].orEmpty(),
                title.externalIdsMap["tvdb"]?.ifEmpty { null } ?: title.tvdbId.takeIf { it > 0 }?.toString().orEmpty(),
                ratings.orEmpty(),
            ),
            if (title.itemId.isNotEmpty()) ({ navigator.open(Route.Item(serverId, title.itemId)) }) else null,
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
            start = true,
        )
    }
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .border(1.dp, colors.border, RoundedCornerShape(14.dp)),
    ) {
        rows.forEachIndexed { i, row ->
            if (i > 0) HorizontalDivider(color = colors.border)
            row()
        }
    }
}

/**
 * A link to a title or person elsewhere, with the site's [logo] [width]
 * wide, as Seerr's; a [mono] logo is white alone, drawn in the text's color.
 */
internal data class ExternalLink(val name: String, val url: String, val logo: Int, val width: Dp = 32.dp, val mono: Boolean = false)

/** The sites linking a title elsewhere, as Seerr's, Rotten Tomatoes' once its [ratings] tell its page. */
internal fun titleLinks(kind: MediaKind, tmdbId: Int, imdb: String, tvdb: String, ratings: List<Rating>): List<ExternalLink> {
    val series = kind == MediaKind.MEDIA_KIND_SERIES
    val language = Locale.getDefault().toLanguageTag()
    val rt = ratings.firstOrNull { it.source != RatingSource.RATING_SOURCE_IMDB && it.url.isNotEmpty() }?.url
    return listOfNotNull(
        ExternalLink("TMDB", "https://www.themoviedb.org/${if (series) "tv" else "movie"}/$tmdbId?language=$language", R.drawable.service_tmdb),
        if (series && tvdb.isNotEmpty()) ExternalLink("TVDB", "https://www.thetvdb.com/dereferrer/series/$tvdb", R.drawable.service_tvdb, 36.dp, mono = true) else null,
        if (imdb.isNotEmpty()) ExternalLink("IMDb", "https://www.imdb.com/title/$imdb", R.drawable.service_imdb) else null,
        rt?.let { ExternalLink("Rotten Tomatoes", it, R.drawable.service_rt, 56.dp) },
        if (imdb.isNotEmpty()) ExternalLink("Trakt", "https://trakt.tv/${if (series) "shows" else "movies"}/$imdb", R.drawable.service_trakt) else null,
        if (imdb.isNotEmpty()) {
            ExternalLink("Simkl", "https://api.simkl.com/redirect?to=Simkl&imdb=${Uri.encode(imdb)}", R.drawable.service_simkl, mono = true)
        } else {
            null
        },
        if (!series) ExternalLink("Letterboxd", "https://letterboxd.com/tmdb/$tmdbId", R.drawable.service_letterboxd) else null,
    )
}

/** The sites an item links with a logo, by the name the library gives them. */
private val siteLogos = mapOf(
    "TMDB" to ExternalLink("TMDB", "", R.drawable.service_tmdb),
    "TheTVDB" to ExternalLink("TheTVDB", "", R.drawable.service_tvdb, 36.dp, mono = true),
    "IMDb" to ExternalLink("IMDb", "", R.drawable.service_imdb),
)

/** An item's [links] elsewhere: those to sites with a logo as logos, and the others. */
internal fun itemLinks(links: List<dev.mavio.core.ExternalLink>): Pair<List<ExternalLink>, List<dev.mavio.core.ExternalLink>> =
    links.mapNotNull { l -> siteLogos[l.name]?.copy(url = l.url) } to links.filter { it.name !in siteLogos }

/**
 * The logos linking a title or person elsewhere, as Seerr's, faint, centered
 * unless [start]; first, Mavio's opening its item when the server has it,
 * where Seerr links its media server.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun ExternalLinks(
    links: List<ExternalLink>,
    openItem: (() -> Unit)? = null,
    modifier: Modifier = Modifier.fillMaxWidth().padding(12.dp),
    start: Boolean = false,
) {
    val uri = LocalUriHandler.current
    FlowRow(
        modifier,
        horizontalArrangement = Arrangement.spacedBy(20.dp, if (start) Alignment.Start else Alignment.CenterHorizontally),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        itemVerticalAlignment = Alignment.CenterVertically,
    ) {
        val faint = Modifier.alpha(0.5f)
        val label = stringResource(R.string.discover_open_item)
        openItem?.let {
            Box(faint.clickable(onClick = it).semantics { contentDescription = label }) { Wordmark(height = 14.dp) }
        }
        links.forEach { l ->
            Image(
                painterResource(l.logo),
                l.name,
                faint
                    .clickable { runCatching { uri.openUri(l.url) } }
                    .width(l.width)
                    .then(if (l.mono) Modifier.height(24.dp) else Modifier),
                contentScale = ContentScale.Fit,
                colorFilter = if (l.mono) ColorFilter.tint(theme.colors.textPrimary) else null,
            )
        }
    }
}

// Seasons

/**
 * A series' seasons with what the server has of each, as Seerr lists them:
 * the latest first, specials last, each expanding to its episodes.
 */
@Composable
private fun Seasons(serverId: String, tmdbId: Int, seasons: List<SeasonStatus>) {
    val ordered = seasons.sortedWith(compareBy<SeasonStatus> { it.seasonNumber == 0 }.thenByDescending { it.seasonNumber })
    Column(
        Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
    ) {
        Text(stringResource(R.string.item_seasons), style = theme.type.title3.text)
        ordered.forEach { Season(serverId, tmdbId, it) }
    }
}

@Composable
private fun Season(serverId: String, tmdbId: Int, s: SeasonStatus) {
    var open by rememberSaveable(s.seasonNumber) { mutableStateOf(false) }
    val colors = theme.colors
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .border(1.dp, colors.border, RoundedCornerShape(10.dp))
            .background(colors.surfaceRaised)
            .animateContentSize(),
    ) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { open = !open }.padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(seasonName(s), Modifier.weight(1f), style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (s.episodeCount > 0) {
                Text(pluralStringResource(R.plurals.item_episode_count, s.episodeCount, s.episodeCount), style = theme.type.callout.text, color = colors.textSecondary)
            }
            statusLabel(s.status)?.let {
                Text(
                    stringResource(it),
                    Modifier.background(colors.fill, CircleShape).padding(horizontal = 10.dp, vertical = 2.dp),
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            MavioIcon(R.drawable.ic_expand, Modifier.graphicsLayer { rotationZ = if (open) 180f else 0f }, size = 16.dp, tint = colors.textSecondary)
        }
        if (open) SeasonEpisodes(serverId, tmdbId, s.seasonNumber)
    }
}

/** The episodes of an expanded season, the latest first, as Seerr's. */
@Composable
private fun SeasonEpisodes(serverId: String, tmdbId: Int, seasonNumber: Int) {
    val colors = theme.colors
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        serverId,
        "GetSeason",
        remember(tmdbId, seasonNumber) {
            getSeasonRequest {
                this.tmdbId = tmdbId
                this.seasonNumber = seasonNumber
                language = discoverLanguage
            }
        },
        GetSeasonResponse::parseFrom,
        attempt,
    )
    val r = result
    val episodes = r?.getOrNull()?.episodesList
    when {
        r == null -> Box(Modifier.padding(start = 16.dp, bottom = 16.dp)) { CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp) }
        episodes == null || episodes.isEmpty() -> Text(
            stringResource(if (episodes == null) R.string.state_error else R.string.discover_no_episodes),
            Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp).then(if (episodes == null) Modifier.clickable { attempt++ } else Modifier),
            style = theme.type.callout.text,
            color = colors.textSecondary,
        )
        else -> Column(Modifier.padding(horizontal = 16.dp)) {
            episodes.reversed().forEach { e ->
                HorizontalDivider(color = colors.border)
                Column(Modifier.padding(vertical = 16.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    Text(stringResource(R.string.discover_episode_title, e.episodeNumber.toString(), e.name), style = theme.type.headline.text)
                    dayText(e.airDate)?.let {
                        Text(it, Modifier.background(colors.fill, CircleShape).padding(horizontal = 10.dp, vertical = 2.dp), style = theme.type.footnote.text)
                    }
                    if (e.overview.isNotEmpty()) Text(e.overview, style = theme.type.callout.text, color = colors.textSecondary)
                    if (e.stillUrl.isNotEmpty()) {
                        Artwork(sized(e.stillUrl, 300), Modifier.fillMaxWidth().aspectRatio(16f / 9f).clip(RoundedCornerShape(8.dp)))
                    }
                }
            }
        }
    }
}

// People

/**
 * A credited person: their photo, else the placeholder of missing artwork, their name, and
 * under it a label, such as the character they play or their job,
 * opening their discovered page.
 */
@Composable
internal fun CreditTile(serverId: String, c: Credit, label: String, modifier: Modifier = Modifier) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    Column(
        modifier.then(if (c.tmdbId > 0) Modifier.clickable { navigator.open(Route.DiscoveredPerson(serverId, c.tmdbId)) } else Modifier),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Artwork(c.photo, Modifier.size(96.dp).clip(CircleShape), icon = R.drawable.ic_profile, contentDescription = c.name)
        Spacer(Modifier.height(MavioSpacing.s8))
        Text(c.name, style = theme.type.callout.text, maxLines = 2, textAlign = TextAlign.Center)
        if (label.isNotEmpty()) {
            Text(label, style = theme.type.footnote.text, color = colors.textSecondary, maxLines = 2, textAlign = TextAlign.Center)
        }
    }
}

/** A discovered title's cast and crew, or its crew alone, as the web UI's. */
@Composable
fun TitleCreditsScreen(route: Route.TitleCredits) {
    val reload = rememberReload()
    val result by discoveredTitle(route.serverId, route.mediaKind, route.tmdbId, reload)
    Loaded(result, { !it.hasTitle() }, route.serverId, reload) { r ->
        val all = remember(r) { titleCredits(r.title.creditsList) }
        val credits = if (route.crew) all.filter { !it.isCast } else all
        val groups = (listOf(CreditKind.CREDIT_KIND_ACTOR to R.string.credits_cast, CreditKind.CREDIT_KIND_GUEST_STAR to R.string.credits_guest_stars) + crewLabels)
            .map { (kind, label) -> label to credits.filter { it.kind == kind }.sortedBy { it.order } }
            .filter { it.second.isNotEmpty() }
        androidx.compose.foundation.lazy.grid.LazyVerticalGrid(
            androidx.compose.foundation.lazy.grid.GridCells.Adaptive(112.dp),
            Modifier.fillMaxSize(),
            contentPadding = PaddingValues(
                start = theme.layout.margin,
                end = theme.layout.margin,
                top = LocalTopInset.current,
                bottom = LocalBottomInset.current + MavioSpacing.s24,
            ),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
        ) {
            line("title") { ScreenTitle(stringResource(if (route.crew) R.string.credits_full_crew else R.string.item_cast)) }
            if (credits.isEmpty()) {
                line("none") { Text(stringResource(R.string.credits_none), style = theme.type.body.text, color = theme.colors.textSecondary) }
            }
            groups.forEach { (label, people) ->
                line("group/$label") { Text(stringResource(label), Modifier.padding(top = MavioSpacing.s8), style = theme.type.title3.text) }
                items(people.size, key = { "$label/$it" }) { i -> CreditTile(route.serverId, people[i], people[i].role) }
            }
        }
    }
}
