package dev.mavio.app

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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.CatalogRow
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.PullToRefresh
import dev.mavio.ui.R
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.catalogRequest
import dev.mavio.ui.discovered
import dev.mavio.ui.filterable
import dev.mavio.ui.line
import dev.mavio.ui.rowTitle
import dev.mavio.ui.rowsOf
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleKey
import dev.mavio.ui.tmdbId
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.launch
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Discover.DiscoverResponse
import mavio.acquisition.v1.Discover.DiscoveredTitle
import mavio.acquisition.v1.Discover.MediaStatus
import mavio.acquisition.v1.Profile.MediaKind

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
            .background(if (available) theme.colors.success else theme.colors.scrim, CircleShape)
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
    val discover = LocalDiscover.current
    val rows = rowsOf(discovery.catalogs)
    var attempt by remember { mutableIntStateOf(0) }
    val top = LocalTopInset.current
    // A pull asks every row's first page again, then the rows show them.
    PullToRefresh(
        refresh = {
            discover.forget(discovery.serverId)
            coroutineScope {
                rows.forEach { row ->
                    launch {
                        val request = catalogRequest(row.catalog.id, row.mediaKind.number, 0, CatalogFilters(), 1)
                        runCatching { discover.call(discovery.serverId, "Discover", request, DiscoverResponse::parseFrom) }
                    }
                }
            }
            attempt++
        },
        top = top,
    ) {
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = top, bottom = LocalBottomInset.current)) {
            item(key = "title") {
                Box(Modifier.padding(horizontal = theme.layout.margin)) { ScreenTitle(stringResource(R.string.nav_discover)) }
            }
            items(rows, key = { "${it.catalog.id}/${it.mediaKind}" }) { row ->
                CatalogShelf(discovery.serverId, row, rowTitle(row), attempt = attempt)
            }
        }
    }
}

/**
 * A row of a catalog's first page, with See All at its title opening the
 * catalog, unless it is filtered; nothing when the catalog has no titles
 * or can't be listed.
 */
@Composable
internal fun CatalogShelf(serverId: String, row: CatalogRow, title: String, tmdbId: Int = 0, filters: CatalogFilters? = null, attempt: Int = 0) {
    val navigator = LocalNavigator.current
    val request = remember(row, tmdbId, filters) { catalogRequest(row.catalog.id, row.mediaKind.number, tmdbId, filters ?: CatalogFilters(), 1) }
    val result by discovered(serverId, "Discover", request, DiscoverResponse::parseFrom, attempt)
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
    var pages by remember(filters, attempt) { mutableStateOf(TitlePages()) }
    suspend fun next(pages: TitlePages) {
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
        val loading = pages
        next(loading)
        // The next page as the end comes near.
        snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .distinctUntilChanged()
            .filter { it >= grid.layoutInfo.totalItemsCount - 12 }
            .collect { next(loading) }
    }
    val kind = MediaKind.forNumber(route.mediaKind) ?: MediaKind.MEDIA_KIND_UNSPECIFIED
    val title = when {
        found == null -> ""
        route.tmdbId > 0 -> found.name
        else -> rowTitle(CatalogRow(found, kind))
    }
    // Recommendations and similar titles take no filters.
    val filterable = found?.filterable(route.tmdbId) == true
    val n = filters.count
    // A pull asks the first page again, the grid keeping its titles until it comes.
    PullToRefresh(
        refresh = {
            discover.forget(route.serverId)
            val fresh = TitlePages()
            next(fresh)
            pages = fresh
        },
        top = LocalTopInset.current,
    ) {
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
    top: Dp = LocalTopInset.current,
) {
    val margin = theme.layout.margin
    LazyVerticalGrid(
        GridCells.Adaptive(theme.layout.poster),
        Modifier.fillMaxSize(),
        state = state,
        contentPadding = PaddingValues(
            start = margin,
            end = margin,
            top = top,
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
