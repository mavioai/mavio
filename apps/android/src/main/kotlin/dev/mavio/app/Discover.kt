package dev.mavio.app

import android.util.Base64
import android.util.LruCache
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.google.protobuf.MessageLite
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Discover.DiscoverResponse
import mavio.acquisition.v1.Discover.DiscoveredTitle
import mavio.acquisition.v1.Discover.ListCatalogsResponse
import mavio.acquisition.v1.Discover.MediaStatus
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.discoverRequest
import mavio.acquisition.v1.listCatalogsRequest
import mavio.plugin.v1.Discovery.CatalogKind
import java.io.Serializable
import java.util.Locale

/** The server Discover asks, the first signed-in server whose discovery providers offer catalogs, with its catalogs. */
data class Discovery(val serverId: String, val catalogs: List<Catalog>)

/**
 * Discover's calls (docs/clients.md §6.3.2), their answers kept a while so
 * that a shelf scrolled back to or a page gone back to shows at once, and
 * the server that discovers once the servers have answered.
 */
@Stable
class DiscoverState(private val model: AppModel) {
    /** The server that discovers, or null while none does or the servers have not answered. */
    var discovery by mutableStateOf<Discovery?>(null)
        private set

    private val cache = LruCache<String, MessageLite>(CACHE_SIZE)

    private fun key(serverId: String, method: String, request: MessageLite) =
        "$serverId/$method/" + Base64.encodeToString(request.toByteArray(), Base64.NO_WRAP)

    /** A DiscoverService call's answer kept from before, if any. */
    @Suppress("UNCHECKED_CAST")
    fun <T : MessageLite> peek(serverId: String, method: String, request: MessageLite): T? = cache.get(key(serverId, method, request)) as T?

    /** Calls a method of DiscoverService, answering from what was kept when it can. */
    suspend fun <T : MessageLite> call(serverId: String, method: String, request: MessageLite, parse: (ByteArray) -> T): T {
        peek<T>(serverId, method, request)?.let { return it }
        return model.client.call(serverId, "/mavio.acquisition.v1.DiscoverService/$method", request, parse)
            .also { cache.put(key(serverId, method, request), it) }
    }

    /** Asks the servers for their catalogs, the first with any discovering. */
    suspend fun load(serverIds: List<String>) {
        val found = coroutineScope {
            serverIds.map { id ->
                async {
                    runCatching {
                        call(id, "ListCatalogs", listCatalogsRequest { language = discoverLanguage }, ListCatalogsResponse::parseFrom)
                    }.getOrNull()?.catalogsList.orEmpty()
                }
            }.awaitAll()
        }
        discovery = serverIds.zip(found).firstOrNull { it.second.isNotEmpty() }?.let { Discovery(it.first, it.second) }
    }

    private companion object {
        const val CACHE_SIZE = 200
    }
}

/** Discover's calls and its server. */
val LocalDiscover = compositionLocalOf<DiscoverState> { error("no discover") }

/** The language Discover asks in, the UI's (ISO 639-1). */
val discoverLanguage: String get() = Locale.getDefault().language

/**
 * A DiscoverService call's answer, null while it is on its way, at once
 * when it was kept from before.
 */
@Composable
internal fun <T : MessageLite> discovered(
    serverId: String,
    method: String,
    request: MessageLite,
    parse: (ByteArray) -> T,
    attempt: Int = 0,
): State<Result<T>?> {
    val discover = LocalDiscover.current
    val initial = remember(serverId, method, request) { discover.peek<T>(serverId, method, request)?.let { Result.success(it) } }
    return produceState(initial, serverId, method, request, attempt) {
        if (value?.isSuccess == true) return@produceState
        value = try {
            Result.success(discover.call(serverId, method, request, parse))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
}

/** A filter value chosen by its provider's ID, with its name to show. */
data class NamedValue(val id: String, val name: String) : Serializable

/** How a catalog's titles are filtered and sorted; unset fields do not filter. */
data class CatalogFilters(
    val sort: Int = 0,
    val ascending: Boolean = false,
    val genres: List<String> = emptyList(),
    /** Production companies, of movies. */
    val studios: List<NamedValue> = emptyList(),
    /** Television networks, of series. */
    val networks: List<NamedValue> = emptyList(),
    val keywords: List<NamedValue> = emptyList(),
    val watchProviders: List<String> = emptyList(),
    /** ISO 3166-1 country of the watch providers. */
    val watchRegion: String = "",
    val originalLanguage: String = "",
    /** Years of release, inclusive; 0 when unset. */
    val yearFrom: Int = 0,
    val yearTo: Int = 0,
    /** Minutes; 0 when unset. */
    val runtimeMin: Int = 0,
    val runtimeMax: Int = 0,
    /** 0–10. */
    val ratingMin: Int = 0,
    val votesMin: Int = 0,
) : Serializable {
    /** How many filters are set, the sort aside. */
    val count: Int
        get() = listOf(
            genres.isNotEmpty(),
            studios.isNotEmpty(),
            networks.isNotEmpty(),
            keywords.isNotEmpty(),
            watchProviders.isNotEmpty(),
            originalLanguage.isNotEmpty(),
            yearFrom > 0 || yearTo > 0,
            runtimeMin > 0 || runtimeMax > 0,
            ratingMin > 0,
            votesMin > 0,
        ).count { it }
}

/** A catalog's page of titles, filtered and sorted; tmdbId names the title of recommendations and similar titles. */
internal fun catalogRequest(catalog: String, mediaKind: Int, tmdbId: Int, f: CatalogFilters, page: Int) = discoverRequest {
    this.catalog = catalog
    mediaKindValue = mediaKind
    this.tmdbId = tmdbId
    language = discoverLanguage
    this.page = page
    sortValue = f.sort
    ascending = f.ascending
    genres += f.genres
    studios += f.studios.map { it.id }
    networks += f.networks.map { it.id }
    keywords += f.keywords.map { it.id }
    watchProviders += f.watchProviders
    if (f.watchProviders.isNotEmpty()) watchRegion = f.watchRegion
    // Only the filters chosen are sent, as the web UI's: a field set is a filter even at zero.
    if (f.originalLanguage.isNotEmpty()) originalLanguage = f.originalLanguage
    if (f.yearFrom > 0) releasedFrom = "${f.yearFrom}-01-01"
    if (f.yearTo > 0) releasedTo = "${f.yearTo}-12-31"
    if (f.runtimeMin > 0) runtimeMin = f.runtimeMin
    if (f.runtimeMax > 0) runtimeMax = f.runtimeMax
    if (f.ratingMin > 0) ratingMin = f.ratingMin.toDouble()
    if (f.votesMin > 0) votesMin = f.votesMin
}

/** A row of Discover: a catalog, of one kind of title unless it mixes them. */
data class CatalogRow(val catalog: Catalog, val mediaKind: MediaKind)

/** The catalogs Discover shows, in its order; those that need a title to start from are left out. */
private val rowKinds = listOf(
    CatalogKind.CATALOG_KIND_TRENDING,
    CatalogKind.CATALOG_KIND_POPULAR,
    CatalogKind.CATALOG_KIND_UPCOMING,
    CatalogKind.CATALOG_KIND_TOP_RATED,
    CatalogKind.CATALOG_KIND_LIST,
)

/** The catalogs' rows: one, or one per kind of title a catalog lists apart. */
internal fun rowsOf(catalogs: List<Catalog>): List<CatalogRow> = rowKinds.flatMap { kind ->
    catalogs.filter { it.kind == kind }.flatMap { c ->
        if (c.mixed || c.mediaKindsCount < 2) {
            listOf(CatalogRow(c, if (c.mixed) MediaKind.MEDIA_KIND_UNSPECIFIED else c.mediaKindsList.firstOrNull() ?: MediaKind.MEDIA_KIND_UNSPECIFIED))
        } else {
            listOf(MediaKind.MEDIA_KIND_MOVIE, MediaKind.MEDIA_KIND_SERIES).filter { it in c.mediaKindsList }.map { CatalogRow(c, it) }
        }
    }
}

/** The name of a row of Discover, by its catalog's kind and its kind of title. */
@Composable
internal fun rowTitle(row: CatalogRow): String {
    val series = row.mediaKind == MediaKind.MEDIA_KIND_SERIES
    return when (row.catalog.kind) {
        CatalogKind.CATALOG_KIND_TRENDING ->
            if (row.mediaKind == MediaKind.MEDIA_KIND_UNSPECIFIED) stringResource(R.string.discover_trending) else row.catalog.name
        CatalogKind.CATALOG_KIND_POPULAR ->
            stringResource(if (series) R.string.discover_popular_series else R.string.discover_popular_movies)
        CatalogKind.CATALOG_KIND_UPCOMING ->
            stringResource(if (series) R.string.discover_upcoming_series else R.string.discover_upcoming_movies)
        CatalogKind.CATALOG_KIND_TOP_RATED ->
            stringResource(if (series) R.string.discover_top_rated_series else R.string.discover_top_rated_movies)
        else -> row.catalog.name
    }
}

/** The same TMDB image at a width, rather than its original size. */
internal fun sized(url: String, width: Int): String? =
    url.takeIf { it.isNotEmpty() }?.replace("image.tmdb.org/t/p/original/", "image.tmdb.org/t/p/w$width/")

/** A title's TMDB ID. */
internal val DiscoveredTitle.tmdbId: Int get() = externalIdsMap["tmdb"]?.toIntOrNull() ?: 0

/** The status a title's card and header show, if any. */
internal fun statusLabel(status: MediaStatus): Int? = when (status) {
    MediaStatus.MEDIA_STATUS_AVAILABLE -> R.string.discover_available
    MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE -> R.string.discover_partially_available
    MediaStatus.MEDIA_STATUS_REQUESTED -> R.string.discover_requested
    MediaStatus.MEDIA_STATUS_WANTED -> R.string.discover_processing
    MediaStatus.MEDIA_STATUS_BLOCKLISTED -> R.string.discover_blocklisted
    else -> null
}

/** The mark of a card's status: a check once available, a clock while requested or processing. */
@Composable
private fun StatusMark(status: MediaStatus, modifier: Modifier) {
    val available = status == MediaStatus.MEDIA_STATUS_AVAILABLE || status == MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE
    val waiting = status == MediaStatus.MEDIA_STATUS_REQUESTED || status == MediaStatus.MEDIA_STATUS_WANTED
    if (!available && !waiting) return
    MavioIcon(
        if (available) R.drawable.ic_check else R.drawable.ic_pending,
        modifier
            .alpha(if (status == MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE) 0.7f else 1f)
            .background(if (available) theme.colors.accent else theme.colors.scrim, CircleShape)
            .padding(2.dp),
        size = 16.dp,
        tint = Color.White,
        contentDescription = statusLabel(status)?.let { stringResource(it) },
    )
}

/**
 * The card of a discovered title, which opens it: its poster with its
 * status, its name, its year and kind, and under them a person's role in
 * it.
 */
@Composable
internal fun TitleCard(serverId: String, title: DiscoveredTitle, width: Dp? = theme.layout.poster, role: String? = null) {
    val navigator = LocalNavigator.current
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    val kind = stringResource(if (series) R.string.discover_series else R.string.discover_movie)
    Column(
        (if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clickable { navigator.open(Route.Title(serverId, title.mediaKindValue, title.tmdbId)) },
    ) {
        Box {
            Artwork(
                sized(title.posterUrl, 342),
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(MavioRadius.md)),
                icon = if (series) R.drawable.ic_shows else R.drawable.ic_movies,
                contentDescription = title.name,
            )
            StatusMark(title.status, Modifier.align(Alignment.TopEnd).padding(6.dp))
        }
        Spacer(Modifier.height(MavioSpacing.s8))
        Text(title.name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(
            listOfNotNull(title.year.takeIf { it > 0 }?.toString(), kind).joinToString(" · "),
            style = theme.type.footnote.text,
            color = theme.colors.textSecondary,
            maxLines = 1,
        )
        if (!role.isNullOrEmpty()) {
            Text(role, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** The key of a discovered title in a list. */
internal fun titleKey(t: DiscoveredTitle) = "${t.mediaKindValue}/${t.externalIdsMap["tmdb"] ?: t.name}"

/** Discover: a row of each catalog of the first server that discovers. */
@Composable
fun DiscoverScreen() {
    val discovery = LocalDiscover.current.discovery
    if (discovery == null) {
        StatusView(
            stringResource(R.string.discover_not_set_up),
            Modifier.padding(top = LocalTopInset.current),
            icon = R.drawable.ic_discover,
            message = stringResource(R.string.discover_not_set_up_message),
        )
        return
    }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = LocalTopInset.current, bottom = LocalBottomInset.current)) {
        item(key = "title") {
            Box(Modifier.padding(horizontal = theme.layout.margin)) { ScreenTitle(stringResource(R.string.nav_discover)) }
        }
        items(rowsOf(discovery.catalogs), key = { "${it.catalog.id}/${it.mediaKind}" }) { row ->
            CatalogShelf(discovery.serverId, row, rowTitle(row))
        }
    }
}

/**
 * A row of a catalog's first page, with See All at its title opening the
 * catalog, unless it is filtered; nothing when the catalog has no titles
 * or can't be listed.
 */
@Composable
internal fun CatalogShelf(serverId: String, row: CatalogRow, title: String, tmdbId: Int = 0, filters: CatalogFilters? = null) {
    val navigator = LocalNavigator.current
    val request = remember(row, tmdbId, filters) { catalogRequest(row.catalog.id, row.mediaKind.number, tmdbId, filters ?: CatalogFilters(), 1) }
    val result by discovered(serverId, "Discover", request, DiscoverResponse::parseFrom)
    val page = result?.getOrNull()
    if (result?.isFailure == true || (page != null && page.titlesCount == 0)) return
    val more: (@Composable () -> Unit)? = if (page != null && page.page < page.totalPages && filters == null) {
        { MoreLink { navigator.open(Route.Catalog(serverId, row.catalog.id, row.mediaKind.number, tmdbId)) } }
    } else {
        null
    }
    Shelf(title, more = more) {
        if (page == null) {
            items(8) { CardPlaceholder() }
        } else {
            items(page.titlesList.distinctBy(::titleKey), key = ::titleKey) { TitleCard(serverId, it) }
        }
    }
}

/** See All at a shelf's title. */
@Composable
internal fun MoreLink(label: String = stringResource(R.string.home_more), onClick: () -> Unit) {
    Text(
        label,
        Modifier.clip(RoundedCornerShape(MavioRadius.xs)).clickable(onClick = onClick).padding(horizontal = 4.dp, vertical = 2.dp),
        style = theme.type.callout.text,
        color = theme.colors.accentText,
        maxLines = 1,
    )
}

/** A catalog's titles as they load, page by page. */
private class TitlePages {
    val titles = mutableStateListOf<DiscoveredTitle>()
    private val seen = HashSet<String>()
    var next by mutableIntStateOf(1)
    var more by mutableStateOf(true)
    var failed by mutableStateOf(false)
    var loading by mutableStateOf(false)

    fun add(page: DiscoverResponse) {
        titles.addAll(page.titlesList.filter { seen.add(titleKey(it)) })
        next = page.page + 1
        more = page.page < page.totalPages
    }
}

/** All the titles of a catalog, page by page, filtered and sorted. */
@Composable
fun CatalogScreen(model: AppModel, route: Route.Catalog) {
    val discover = LocalDiscover.current
    val found = discover.discovery?.catalogs?.firstOrNull { it.id == route.catalogId }
    var filters by rememberSaveable { mutableStateOf(route.filters) }
    var filtering by remember { mutableStateOf(false) }
    var attempt by remember { mutableIntStateOf(0) }
    val pages = remember(filters, attempt) { TitlePages() }
    suspend fun next() {
        if (pages.loading || !pages.more || pages.failed) return
        pages.loading = true
        try {
            pages.add(
                discover.call(
                    route.serverId,
                    "Discover",
                    catalogRequest(route.catalogId, route.mediaKind, route.tmdbId, filters, pages.next),
                    DiscoverResponse::parseFrom,
                ),
            )
        } catch (e: CancellationException) {
            throw e
        } catch (_: Exception) {
            pages.failed = true
        } finally {
            pages.loading = false
        }
    }
    val grid = rememberLazyGridState()
    LaunchedEffect(pages) {
        next()
        // The next page as the end comes near.
        snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .distinctUntilChanged()
            .filter { it >= grid.layoutInfo.totalItemsCount - 12 }
            .collect { next() }
    }
    val kind = MediaKind.forNumber(route.mediaKind) ?: MediaKind.MEDIA_KIND_UNSPECIFIED
    val title = when {
        found == null -> ""
        route.tmdbId > 0 -> found.name
        else -> rowTitle(CatalogRow(found, kind))
    }
    // Recommendations and similar titles take no filters.
    val filterable = found != null && route.tmdbId == 0 && (found.filtersCount > 0 || found.sortsCount > 0)
    val n = filters.count
    TitleGrid(
        route.serverId,
        pages.titles,
        state = grid,
        header = {
            line("header") {
                Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    Box(Modifier.weight(1f)) { ScreenTitle(title.ifEmpty { " " }) }
                    if (filterable) {
                        MavioButton(
                            if (n > 0) "${stringResource(R.string.discover_filters)} ($n)" else stringResource(R.string.discover_filters),
                            { filtering = true },
                            Modifier.padding(bottom = MavioSpacing.s8),
                            kind = if (n > 0 || filters.sort != 0) ButtonKind.Primary else ButtonKind.Secondary,
                            icon = R.drawable.ic_filter,
                        )
                    }
                }
            }
        },
        empty = {
            when {
                pages.failed -> line("error") {
                    StatusView(
                        stringResource(R.string.state_error),
                        Modifier.padding(top = MavioSpacing.s48),
                        icon = R.drawable.ic_error,
                        action = stringResource(R.string.action_retry) to { attempt++ },
                    )
                }
                pages.more || pages.loading -> items(12) { CardPlaceholder() }
                n > 0 || filters.sort != 0 -> line("none") {
                    StatusView(
                        stringResource(R.string.discover_no_matches),
                        Modifier.padding(top = MavioSpacing.s48),
                        icon = R.drawable.ic_search,
                        action = stringResource(R.string.discover_reset) to { filters = CatalogFilters() },
                    )
                }
                else -> line("empty") { StatusView(stringResource(R.string.state_empty), Modifier.padding(top = MavioSpacing.s48)) }
            }
        },
        footer = {
            if (pages.titles.isNotEmpty() && pages.more) {
                line("more") {
                    Box(Modifier.fillMaxWidth().padding(MavioSpacing.s16), Alignment.Center) { CircularProgressIndicator(Modifier.width(24.dp)) }
                }
            }
        },
    )
    if (filtering && found != null) {
        FiltersSheet(model, route.serverId, found, kind, filters, onDismiss = { filtering = false }) {
            filtering = false
            filters = it
        }
    }
}

/**
 * A grid of discovered titles filling the screen's width, under a header,
 * with what shows in their place while there are none.
 */
@Composable
internal fun TitleGrid(
    serverId: String,
    titles: List<DiscoveredTitle>,
    state: androidx.compose.foundation.lazy.grid.LazyGridState = rememberLazyGridState(),
    header: LazyGridScope.() -> Unit,
    empty: LazyGridScope.() -> Unit = {},
    footer: LazyGridScope.() -> Unit = {},
    roles: List<String>? = null,
) {
    val margin = theme.layout.margin
    LazyVerticalGrid(
        GridCells.Adaptive(theme.layout.poster),
        Modifier.fillMaxSize(),
        state = state,
        contentPadding = PaddingValues(
            start = margin,
            end = margin,
            top = LocalTopInset.current,
            bottom = LocalBottomInset.current + MavioSpacing.s24,
        ),
        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter * 1.5f),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
    ) {
        header()
        if (titles.isEmpty()) {
            empty()
        } else {
            items(titles.size, key = { titleKey(titles[it]) + "/$it" }) { i ->
                TitleCard(serverId, titles[i], width = null, role = roles?.getOrNull(i))
            }
        }
        footer()
    }
}
