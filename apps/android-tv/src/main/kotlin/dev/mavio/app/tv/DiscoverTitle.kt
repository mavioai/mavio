package dev.mavio.app.tv

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
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
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalLocale
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.CatalogRow
import dev.mavio.ui.Credit
import dev.mavio.ui.Focusable
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NamedValue
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvButton
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.countryText
import dev.mavio.ui.crewLabels
import dev.mavio.ui.crewRank
import dev.mavio.ui.dayText
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.focusArtwork
import dev.mavio.ui.focusCard
import dev.mavio.ui.focused
import dev.mavio.ui.languageName
import dev.mavio.ui.minutesText
import dev.mavio.ui.moneyText
import dev.mavio.ui.productionStatus
import dev.mavio.ui.seasonName
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleCredits
import kotlinx.coroutines.launch
import mavio.acquisition.v1.Discover.GetCollectionResponse
import mavio.acquisition.v1.Discover.GetRatingsResponse
import mavio.acquisition.v1.Discover.GetSeasonResponse
import mavio.acquisition.v1.Discover.GetTitleResponse
import mavio.acquisition.v1.Discover.SeasonStatus
import mavio.acquisition.v1.Discover.Title
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getCollectionRequest
import mavio.acquisition.v1.getRatingsRequest
import mavio.acquisition.v1.getSeasonRequest
import mavio.acquisition.v1.getTitleRequest
import mavio.library.v1.ItemOuterClass.CreditKind
import mavio.plugin.v1.Discovery.CatalogKind
import mavio.plugin.v1.MetadataOuterClass.ProductionStatus
import mavio.plugin.v1.Ratings.RatingSource

/**
 * A discovered title on a television (docs/tv.md §6.3), laid out as an
 * item's details, its requests left out as on the phone: the backdrop 86 %
 * of the screen's height with the name, the facts, two lines of overview
 * and Watch, on which focus starts; the overview beside the box of facts;
 * a series' seasons; the cast; recommendations and similar titles. Back
 * returns focus to where it was in [page].
 */
@Composable
fun TitlePage(page: Page.Title) {
    var attempt by remember { mutableIntStateOf(0) }
    val result by discovered(
        page.serverId,
        "GetTitle",
        remember(page) {
            getTitleRequest {
                mediaKindValue = page.mediaKind
                tmdbId = page.tmdbId
                language = discoverLanguage
            }
        },
        GetTitleResponse::parseFrom,
        attempt,
    )
    val r = result
    val found = r?.getOrNull()
    Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
        when {
            r == null -> CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
            found == null -> StatusView(
                stringResource(R.string.state_error),
                icon = R.drawable.ic_error,
                message = r.exceptionOrNull()?.message,
                action = stringResource(R.string.action_retry) to { attempt++ },
            )
            !found.hasTitle() -> StatusView(stringResource(R.string.state_empty), icon = R.drawable.ic_search)
            else -> TitleContent(page, found.title)
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun TitleContent(page: Page.Title, title: Title) {
    val pages = LocalPages.current
    val catalogs = LocalDiscover.current.discovery?.catalogs.orEmpty()
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val credits = remember(title) { titleCredits(title.creditsList) }
    val focus = page.focus
    val open = alike(page.serverId, title.mediaKind)
    val genres = title.genreRefsList.map { NamedValue(it.id, it.name) }.ifEmpty { title.genresList.map { NamedValue("", it) } }.take(3)
    // Focus starts on Watch, else on the first genre, else on the page's first element.
    val body = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        val start = when {
            title.itemId.isNotEmpty() -> focus.of("primary")
            open != null && genres.firstOrNull()?.id?.isNotEmpty() == true -> focus.of("genre/0")
            else -> body
        }
        focus.restore(start)
    }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(120.dp.toPx(), 120.dp.toPx()) } }
    val scope = rememberCoroutineScope()
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyColumn(Modifier.fillMaxSize().focusRequester(body), state = page.list, contentPadding = PaddingValues(bottom = MavioSpacing.s40)) {
            item(key = "header") {
                TitleHeader(
                    title,
                    genres,
                    focus,
                    open,
                    // Focus on the actions shows the page from its top.
                    Modifier.onFocusChanged { if (it.hasFocus) scope.launch { page.list.animateScrollToItem(0) } },
                ) {
                    if (title.itemId.isNotEmpty()) {
                        TvButton(
                            stringResource(R.string.discover_watch),
                            { pages.openItem(page.serverId, title.itemId) },
                            Modifier.focusKey(focus, "primary"),
                            icon = R.drawable.ic_play_fill,
                        )
                    }
                }
            }
            item(key = "overview") {
                Row(
                    Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s16),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s48),
                ) {
                    TitleOverview(
                        title,
                        credits,
                        focus,
                        open,
                        { pages.openPerson(page.serverId, it) },
                        { pages.openTitleCredits(page.serverId, title.mediaKindValue, title.tmdbId, crew = true) },
                        Modifier.weight(1f),
                    )
                    Column(Modifier.width(560.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                        CollectionCard(page.serverId, title, Modifier.focusKey(focus, "collection"))
                        TitleFacts(page.serverId, title)
                    }
                }
            }
            if (series && title.seasonsCount > 0) {
                item(key = "seasons") { Seasons(page, title.tmdbId, title.seasonsList) }
            }
            val cast = credits.filter { it.isCast }.sortedBy { it.order }.take(20)
            if (cast.isNotEmpty()) item(key = "cast") {
                TvShelf(stringResource(R.string.item_cast), page.rows.of("cast")) {
                    items(cast.size, key = { "${cast[it].tmdbId}/${cast[it].name}/$it" }) { i ->
                        CreditCard(cast[i], Modifier.focusKey(focus, "cast/$i")) { pages.openPerson(page.serverId, cast[i].tmdbId) }
                    }
                    item(key = "all") {
                        CastAllCard(Modifier.focusKey(focus, "cast/all")) { pages.openTitleCredits(page.serverId, title.mediaKindValue, title.tmdbId, crew = false) }
                    }
                }
            }
            val similar = catalogs.filter {
                (it.kind == CatalogKind.CATALOG_KIND_RECOMMENDATIONS || it.kind == CatalogKind.CATALOG_KIND_SIMILAR) && title.mediaKind in it.mediaKindsList
            }
            items(similar, key = { "catalog/${it.id}" }) { c ->
                TvCatalogShelf(
                    page.serverId,
                    CatalogRow(c, title.mediaKind),
                    stringResource(if (c.kind == CatalogKind.CATALOG_KIND_RECOMMENDATIONS) R.string.discover_recommendations else R.string.discover_similar),
                    page.rows.of("catalog/${c.id}"),
                    focus,
                    "catalog/${c.id}",
                    tmdbId = title.tmdbId,
                )
            }
        }
    }
}

/**
 * The top of the page, 86 % of the screen's height: the backdrop, else
 * the poster blurred, and over its lower left the poster, the name at 72
 * points, the facts and status, two lines of overview and the [actions],
 * no wider than 960 points.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleHeader(
    title: Title,
    genres: List<NamedValue>,
    focus: FocusMemory,
    open: ((CatalogFilters) -> Unit)?,
    modifier: Modifier,
    actions: @Composable () -> Unit,
) {
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val screen = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
    Box(modifier.fillMaxWidth().height(screen * 0.86f).background(theme.colors.surface)) {
        if (title.backdropUrl.isNotEmpty()) {
            Artwork(sized(title.backdropUrl, 1280), Modifier.fillMaxSize())
        } else if (title.posterUrl.isNotEmpty()) {
            Artwork(
                sized(title.posterUrl, 342),
                Modifier
                    .fillMaxSize()
                    .graphicsLayer {
                        scaleX = 1.25f
                        scaleY = 1.25f
                    }
                    .blur(48.dp)
                    .alpha(0.7f),
            )
        }
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
            if (title.posterUrl.isNotEmpty()) {
                Artwork(
                    sized(title.posterUrl, 500),
                    Modifier
                        .width(300.dp)
                        .aspectRatio(2f / 3f)
                        .shadow(24.dp, RoundedCornerShape(MavioRadius.lg))
                        .clip(RoundedCornerShape(MavioRadius.lg)),
                )
            }
            Column(Modifier.widthIn(max = 960.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                Text(title.name, fontSize = 72.sp, lineHeight = 76.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 2, overflow = TextOverflow.Ellipsis)
                val facts = listOfNotNull(
                    title.year.takeIf { it > 0 }?.toString(),
                    stringResource(if (series) R.string.discover_series else R.string.discover_movie),
                    title.officialRating.ifEmpty { null },
                    title.runtimeMinutes.takeIf { it > 0 }?.let { minutesText(it.toLong()) },
                )
                val status = statusLabel(title.status)
                if (facts.isNotEmpty() || genres.isNotEmpty() || status != null) {
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                        itemVerticalAlignment = Alignment.CenterVertically,
                    ) {
                        val faint = Color.White.copy(alpha = 0.8f)
                        if (facts.isNotEmpty()) Text(facts.joinToString("  ·  "), style = theme.type.callout.text, color = faint)
                        // The genres open the popular titles of each.
                        genres.forEachIndexed { i, g ->
                            Text(if (i == 0) (if (facts.isNotEmpty()) "·" else "") else "/", style = theme.type.callout.text, color = faint)
                            if (g.id.isNotEmpty() && open != null) {
                                TvLink(g.name, Modifier.focusKey(focus, "genre/$i"), color = faint) { open(CatalogFilters(genres = listOf(g.id))) }
                            } else {
                                Text(g.name, style = theme.type.callout.text, color = faint)
                            }
                        }
                        status?.let {
                            Text(
                                stringResource(it),
                                Modifier.background(Color.White.copy(alpha = 0.2f), RoundedCornerShape(4.dp)).padding(horizontal = 6.dp),
                                style = theme.type.caption.text,
                                fontWeight = FontWeight.SemiBold,
                                color = Color.White,
                            )
                        }
                    }
                }
                if (title.overview.isNotEmpty()) {
                    Text(title.overview, style = theme.type.body.text, color = Color.White.copy(alpha = 0.85f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
                Spacer(Modifier.height(MavioSpacing.s8))
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) { actions() }
            }
        }
    }
}

/**
 * The tagline, the overview, the leading crew as Seerr's (six of them, a
 * series' creators first, in two columns) and the keywords.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleOverview(
    title: Title,
    credits: List<Credit>,
    focus: FocusMemory,
    open: ((CatalogFilters) -> Unit)?,
    openPerson: (Int) -> Unit,
    onFullCrew: () -> Unit,
    modifier: Modifier,
) {
    val colors = theme.colors
    Column(modifier, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
        if (title.tagline.isNotEmpty()) {
            Text(title.tagline, style = theme.type.title3.text, fontWeight = FontWeight.Normal, fontStyle = FontStyle.Italic, color = colors.textSecondary)
        }
        Text(stringResource(R.string.discover_overview), style = theme.type.title3.text)
        Text(title.overview.ifEmpty { stringResource(R.string.discover_no_overview) }, style = theme.type.body.text)
        val keywords = title.keywordRefsList.map { NamedValue(it.id, it.name) }.ifEmpty { title.keywordsList.map { NamedValue("", it) } }
        val crew = credits.withIndex()
            .filter { (_, c) -> !c.isCast && crewLabels.any { it.first == c.kind } }
            .sortedWith(compareBy({ crewRank(it.value) }, { it.index }))
            .take(6)
            .map { it.value }
        if (crew.isNotEmpty()) {
            FlowRow(Modifier.padding(top = MavioSpacing.s8), maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                crew.forEachIndexed { i, c ->
                    // Each opens the person's page, white with black text with focus.
                    val state = remember { Focusable() }
                    val focused = state.focused()
                    Box(Modifier.weight(1f).padding(end = MavioSpacing.s16)) {
                        Column(
                            Modifier
                                .shakes()
                                .focusKey(focus, "crew/$i")
                                .then(
                                    // From the last row, the row's last person moves down to View Full
                                    // Crew, and the others past it, to the keywords.
                                    if (i >= (crew.size - 1) / 2 * 2) {
                                        Modifier.focusProperties {
                                            if (i % 2 == 1 || i == crew.lastIndex) {
                                                down = focus.of("fullCrew")
                                            } else if (keywords.firstOrNull()?.id?.isNotEmpty() == true && open != null) {
                                                down = focus.of("keyword/0")
                                            }
                                        }
                                    } else {
                                        Modifier
                                    },
                                )
                                .clip(RoundedCornerShape(10.dp))
                                .background(if (focused) Color.White else Color.Transparent)
                                .selectable(state) { if (c.tmdbId > 0) openPerson(c.tmdbId) }
                                .padding(horizontal = 10.dp, vertical = 6.dp),
                        ) {
                            Text(
                                if (c.kind == CreditKind.CREDIT_KIND_OTHER && c.role.isNotEmpty()) c.role else stringResource(crewLabels.first { it.first == c.kind }.second),
                                style = theme.type.headline.text,
                                color = if (focused) Color.Black else colors.textPrimary,
                            )
                            Text(c.name, style = theme.type.callout.text, color = if (focused) Color.Black.copy(alpha = 0.7f) else colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                }
            }
            FullCrew(Modifier.focusKey(focus, "fullCrew").focusProperties { up = focus.of("crew/${crew.lastIndex}") }, onFullCrew)
        }
        // The keywords open the popular titles of each.
        if (keywords.isNotEmpty()) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                keywords.forEachIndexed { i, k ->
                    val linked = k.id.isNotEmpty() && open != null
                    val state = remember { Focusable() }
                    val focused = linked && state.focused()
                    Text(
                        k.name,
                        Modifier
                            .shakes()
                            .then(if (linked) Modifier.focusKey(focus, "keyword/$i") else Modifier)
                            .clip(CircleShape)
                            .background(if (focused) Color.White else colors.fill)
                            .then(if (linked) Modifier.selectable(state) { open?.invoke(CatalogFilters(keywords = listOf(k))) } else Modifier)
                            .padding(horizontal = 14.dp, vertical = 6.dp),
                        style = theme.type.footnote.text,
                        color = if (focused) Color.Black else colors.textPrimary,
                    )
                }
            }
        }
    }
}

/** Opens the popular titles of a kind filtered alike; null without such a catalog. */
@Composable
private fun alike(serverId: String, kind: MediaKind): ((CatalogFilters) -> Unit)? {
    val pages = LocalPages.current
    val popular = LocalDiscover.current.discovery?.catalogs?.firstOrNull {
        it.kind == CatalogKind.CATALOG_KIND_POPULAR && kind in it.mediaKindsList
    } ?: return null
    return { f -> pages.openCatalog(serverId, popular.id, kind.number, filters = f) }
}

/** A link in a line of text: white with black text once it has focus. */
@Composable
internal fun TvLink(label: String, modifier: Modifier, color: Color, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Text(
        label,
        modifier
            .shakes()
            .clip(RoundedCornerShape(6.dp))
            .background(if (focused) Color.White else Color.Transparent)
            .selectable(state, onClick)
            .padding(horizontal = 6.dp, vertical = 2.dp),
        style = theme.type.callout.text,
        color = if (focused) Color.Black else color,
    )
}

/** A movie's collection as Seerr's card: its backdrop and name. */
@Composable
private fun CollectionCard(serverId: String, title: Title, modifier: Modifier) {
    val id = title.externalIdsMap["tmdb_collection"]?.toIntOrNull() ?: 0
    if (id <= 0) return
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
    val pages = LocalPages.current
    val state = remember { Focusable() }
    val focused = state.focused()
    Box(
        modifier
            .shakes()
            .fillMaxWidth()
            .heightIn(min = 96.dp)
            .focusCard(focused)
            .focusArtwork(focused, 14.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(theme.colors.surfaceRaised)
            .selectable(state) { pages.openCollection(serverId, id) },
    ) {
        if (backdrop.isNotEmpty()) Artwork(sized(backdrop, 780), Modifier.matchParentSize())
        Box(Modifier.matchParentSize().background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.45f), Color.Black.copy(alpha = 0.8f)))))
        Text(
            collection?.name?.ifEmpty { null } ?: title.collectionName.ifEmpty { stringResource(R.string.item_collection) },
            Modifier.align(Alignment.CenterStart).padding(20.dp),
            style = theme.type.headline.text,
            color = Color.White,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** The studios the facts show. */
private const val MAX_STUDIOS = 5

/** Rotten Tomatoes' least fresh score, and least upright audience score. */
private const val FRESH_SCORE = 60.0

/** A row of the facts: its label, and its values on the right. */
@Composable
private fun Fact(label: String, values: List<String>) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        Text(label, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
        Column(Modifier.weight(1f), horizontalAlignment = Alignment.End) {
            values.forEach { Text(it, style = theme.type.callout.text, color = theme.colors.textSecondary, textAlign = TextAlign.End) }
        }
    }
}

/**
 * A title's facts as Seerr's: the ratings, original title, status, dates,
 * runtime, money, language, countries, studios or networks and where it
 * streams; a television links nowhere else (docs/tv.md §1).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TitleFacts(serverId: String, title: Title) {
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val colors = theme.colors
    val rows = mutableListOf<@Composable () -> Unit>()
    val ratings = ratingsOf(serverId, title)
    if (ratings.isNotEmpty()) {
        rows += {
            FlowRow(
                Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24, Alignment.CenterHorizontally),
            ) {
                ratings.forEach { (label, score) ->
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(label, style = theme.type.footnote.text, fontWeight = FontWeight.Bold, color = colors.textSecondary)
                        Text(score, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
                    }
                }
            }
        }
    }
    if (title.originalTitle.isNotEmpty() && title.originalLanguage != discoverLanguage) {
        rows += { Fact(stringResource(R.string.item_original_title), listOf(title.originalTitle)) }
    }
    productionStatus(title.productionStatus)?.let { s -> rows += { Fact(stringResource(R.string.item_status), listOf(s)) } }
    if (series) {
        dayText(title.premiereDate)?.let { day -> rows += { Fact(stringResource(R.string.item_first_aired), listOf(day)) } }
        if (title.productionStatus == ProductionStatus.PRODUCTION_STATUS_ENDED) {
            dayText(title.endDate)?.let { day -> rows += { Fact(stringResource(R.string.item_last_aired), listOf(day)) } }
        }
        dayText(title.nextAirDate)?.let { day -> rows += { Fact(stringResource(R.string.item_next_air_date), listOf(day)) } }
    } else {
        val releases = listOfNotNull(
            dayText(title.theatricalReleaseDate)?.let { R.string.discover_theatrical_release to it },
            dayText(title.digitalReleaseDate)?.let { R.string.item_digital_release to it },
            dayText(title.physicalReleaseDate)?.let { R.string.item_physical_release to it },
        )
        if (releases.isNotEmpty()) {
            rows += {
                Fact(
                    stringResource(if (releases.size == 1) R.string.discover_release_date else R.string.discover_release_dates),
                    releases.map { (kind, day) -> "${stringResource(kind)}: $day" },
                )
            }
        } else {
            dayText(title.premiereDate)?.let { day -> rows += { Fact(stringResource(R.string.item_released), listOf(day)) } }
        }
    }
    if (title.runtimeMinutes > 0) {
        rows += { Fact(stringResource(if (series) R.string.discover_episode_runtime else R.string.item_runtime), listOf(minutesText(title.runtimeMinutes.toLong()))) }
    }
    moneyText(title.revenue)?.let { m -> rows += { Fact(stringResource(R.string.item_revenue), listOf(m)) } }
    moneyText(title.budget)?.let { m -> rows += { Fact(stringResource(R.string.item_budget), listOf(m)) } }
    if (title.originalLanguage.isNotEmpty()) {
        rows += { Fact(stringResource(R.string.item_original_language), listOf(languageName(title.originalLanguage) ?: title.originalLanguage)) }
    }
    val countries = title.productionCountriesList.map(::countryText).ifEmpty { title.productionLocationsList }
    if (countries.isNotEmpty()) rows += { Fact(stringResource(R.string.item_country), countries) }
    val studios = title.studioRefsList.map { it.name }.ifEmpty { title.studiosList }
    if (studios.isNotEmpty()) {
        rows += { Fact(stringResource(if (series) R.string.item_networks else R.string.item_studio), studios.take(MAX_STUDIOS)) }
    }
    if (title.watchProvidersCount > 0) {
        rows += {
            Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                Text(stringResource(R.string.item_streaming), style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    title.watchProvidersList.forEach { p ->
                        if (p.logoUrl.isNotEmpty()) {
                            Artwork(p.logoUrl, Modifier.size(40.dp).clip(RoundedCornerShape(8.dp)), contentDescription = p.name)
                        } else {
                            Text(p.name, style = theme.type.footnote.text)
                        }
                    }
                }
            }
        }
    }
    if (rows.isEmpty()) return
    Column(
        Modifier
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

/** A title's ratings as Seerr's: Rotten Tomatoes' and IMDb's, then TMDB's score. */
@Composable
private fun ratingsOf(serverId: String, title: Title): List<Pair<String, String>> {
    val result by discovered(
        serverId,
        "GetRatings",
        remember(title.tmdbId) {
            getRatingsRequest {
                mediaKind = title.mediaKind
                tmdbId = title.tmdbId
            }
        },
        GetRatingsResponse::parseFrom,
    )
    val ratings = result?.getOrNull()?.ratingsList.orEmpty()
    val locale = LocalLocale.current.platformLocale
    val order = listOf(RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_CRITICS, RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE, RatingSource.RATING_SOURCE_IMDB)
    val shown = order.mapNotNull { s -> ratings.firstOrNull { it.source == s } }.map { r ->
        when (r.source) {
            RatingSource.RATING_SOURCE_IMDB -> "IMDb" to String.format(locale, "%.1f", r.score)
            RatingSource.RATING_SOURCE_ROTTEN_TOMATOES_CRITICS -> (if (r.score >= FRESH_SCORE) "🍅" else "🟢") to "${Math.round(r.score)}%"
            else -> (if (r.score >= FRESH_SCORE) "🍿" else "🥀") to "${Math.round(r.score)}%"
        }
    }
    return if (title.voteCount > 0) shown + ("TMDB" to "${Math.round(title.communityRating * 10)}%") else shown
}

/**
 * A series' seasons with what the server has of each, as Seerr lists them:
 * the latest first, specials last, each a box that selecting expands to
 * its episodes.
 */
@Composable
private fun Seasons(page: Page.Title, tmdbId: Int, seasons: List<SeasonStatus>) {
    val ordered = seasons.sortedWith(compareBy<SeasonStatus> { it.seasonNumber == 0 }.thenByDescending { it.seasonNumber })
    Column(
        Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s16),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
    ) {
        Text(stringResource(R.string.item_seasons), style = theme.type.title3.text)
        ordered.forEach { s ->
            val open = s.seasonNumber in page.open
            Season(page.serverId, tmdbId, s, open, Modifier.focusKey(page.focus, "seasons/${s.seasonNumber}")) {
                if (open) page.open -= s.seasonNumber else page.open += s.seasonNumber
            }
        }
    }
}

/**
 * A season's box of `color.fill`: focus turns it white and raises it with
 * a shadow, scaled a little while it is folded, and only its row white
 * while it shows its episodes (docs/tv.md §6.3).
 */
@Composable
private fun Season(serverId: String, tmdbId: Int, s: SeasonStatus, open: Boolean, modifier: Modifier, onToggle: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val colors = theme.colors
    val white = focused && !open
    val scale by animateFloatAsState(if (white) 1.02f else 1f, tween(160), label = "scale")
    val shape = RoundedCornerShape(MavioRadius.lg)
    Column(
        Modifier
            .fillMaxWidth()
            .graphicsLayer {
                scaleX = scale
                scaleY = scale
            }
            .then(if (focused) Modifier.shadow(16.dp, shape, ambientColor = colors.tvFocusShadow, spotColor = colors.tvFocusShadow) else Modifier)
            .clip(shape)
            .background(if (white) Color.White else colors.fill),
    ) {
        val text = if (focused) Color.Black else colors.textPrimary
        val secondary = if (focused) Color.Black.copy(alpha = 0.6f) else colors.textSecondary
        Row(
            modifier
                .shakes()
                .fillMaxWidth()
                .background(if (focused) Color.White else Color.Transparent)
                .selectable(state, onToggle)
                .heightIn(min = 72.dp)
                .padding(horizontal = 24.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(seasonName(s), Modifier.weight(1f), style = theme.type.headline.text, color = text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (s.episodeCount > 0) {
                Text(pluralStringResource(R.plurals.item_episode_count, s.episodeCount, s.episodeCount), style = theme.type.callout.text, color = secondary)
            }
            statusLabel(s.status)?.let {
                Text(
                    stringResource(it),
                    Modifier
                        .background(if (focused) Color.Black.copy(alpha = 0.1f) else colors.fill, CircleShape)
                        .padding(horizontal = 12.dp, vertical = 2.dp),
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                    color = text,
                )
            }
            MavioIcon(R.drawable.ic_expand, Modifier.graphicsLayer { rotationZ = if (open) 180f else 0f }, size = 20.dp, tint = secondary)
        }
        if (open) SeasonEpisodes(serverId, tmdbId, s.seasonNumber)
    }
}

/** The episodes of an expanded season, the latest first, as Seerr's. */
@Composable
private fun SeasonEpisodes(serverId: String, tmdbId: Int, seasonNumber: Int) {
    val colors = theme.colors
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
    )
    val r = result
    val episodes = r?.getOrNull()?.episodesList
    when {
        r == null -> Box(Modifier.padding(24.dp)) { CircularProgressIndicator(Modifier.size(28.dp), color = Color.White, strokeWidth = 3.dp) }
        episodes == null || episodes.isEmpty() -> Text(
            stringResource(if (episodes == null) R.string.state_error else R.string.discover_no_episodes),
            Modifier.padding(24.dp),
            style = theme.type.callout.text,
            color = colors.textSecondary,
        )
        else -> Column(Modifier.padding(horizontal = 24.dp)) {
            episodes.reversed().forEach { e ->
                HorizontalDivider(color = colors.border)
                Row(Modifier.padding(vertical = 20.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24)) {
                    Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                            Text(stringResource(R.string.discover_episode_title, e.episodeNumber.toString(), e.name), style = theme.type.headline.text)
                            dayText(e.airDate)?.let {
                                Text(it, Modifier.background(colors.fill, CircleShape).padding(horizontal = 12.dp, vertical = 2.dp), style = theme.type.footnote.text)
                            }
                        }
                        if (e.overview.isNotEmpty()) {
                            Text(e.overview, style = theme.type.callout.text, color = colors.textSecondary, maxLines = 3, overflow = TextOverflow.Ellipsis)
                        }
                    }
                    if (e.stillUrl.isNotEmpty()) {
                        Artwork(sized(e.stillUrl, 300), Modifier.width(280.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(MavioRadius.md)))
                    }
                }
            }
        }
    }
}

/** A person of the cast: the photo in a circle, which grows with focus, the name and the role. */
@Composable
internal fun CreditCard(credit: Credit, modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Column(modifier.shakes().width(180.dp).focusCard(focused).selectable(state) { if (credit.tmdbId > 0) onClick() }, horizontalAlignment = Alignment.CenterHorizontally) {
        Box(
            Modifier
                .focusArtwork(focused, 80.dp)
                .size(160.dp)
                .clip(CircleShape)
                .background(theme.colors.surfaceRaised),
            Alignment.Center,
        ) {
            Text(
                credit.name.split(" ").filter { it.isNotEmpty() }.take(2).joinToString("") { it.take(1) },
                style = theme.type.title2.text,
                color = theme.colors.textSecondary,
            )
            credit.photo?.let { Artwork(it, Modifier.fillMaxSize(), contentDescription = credit.name) }
        }
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(credit.name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center)
        if (credit.role.isNotEmpty()) {
            Text(credit.role, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center)
        }
    }
}
