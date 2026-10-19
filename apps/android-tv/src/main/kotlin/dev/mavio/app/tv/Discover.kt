package dev.mavio.app.tv

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.CatalogRow
import dev.mavio.ui.Focusable
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.catalogRequest
import dev.mavio.ui.discovered
import dev.mavio.ui.focusArtwork
import dev.mavio.ui.focusCard
import dev.mavio.ui.focused
import dev.mavio.ui.rowTitle
import dev.mavio.ui.rowsOf
import dev.mavio.ui.selectable
import dev.mavio.ui.sized
import dev.mavio.ui.statusLabel
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleKey
import dev.mavio.ui.tmdbId
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import mavio.acquisition.v1.Discover.DiscoverResponse
import mavio.acquisition.v1.Discover.DiscoveredTitle
import mavio.acquisition.v1.Discover.MediaStatus
import mavio.acquisition.v1.Profile.MediaKind

/**
 * Discover on a television (docs/tv.md §6.3): a row of each catalog of
 * the server that discovers, each ending in See All. Focus starts on the
 * first title; [entry] takes it there. While [claim] the page takes focus
 * back to where it was.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun DiscoverPage(list: LazyListState, rows: RowStates, focus: FocusMemory, entry: FocusRequester, claim: Boolean) {
    val discovery = LocalDiscover.current.discovery ?: return
    val shelves = remember(discovery) { rowsOf(discovery.catalogs) }
    // The first title of the topmost row loaded takes focus.
    val loaded = remember { mutableStateListOf<String>() }
    val first = shelves.firstOrNull { "${it.catalog.id}/${it.mediaKind}" in loaded }?.let { "${it.catalog.id}/${it.mediaKind}" }
    LaunchedEffect(first) { if (claim && first != null) focus.restore(entry) }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyColumn(Modifier.fillMaxSize(), state = list, contentPadding = PaddingValues(top = 104.dp, bottom = MavioSpacing.s24)) {
            items(shelves, key = { "${it.catalog.id}/${it.mediaKind}" }) { row ->
                val key = "${row.catalog.id}/${row.mediaKind}"
                TvCatalogShelf(
                    discovery.serverId,
                    row,
                    rowTitle(row),
                    rows.of(key),
                    focus,
                    key,
                    entry.takeIf { first == key },
                ) { if (key !in loaded) loaded += key }
            }
        }
    }
}

/**
 * A row of a catalog's first page, ending in a See All card that opens
 * the catalog where it has more; nothing when the catalog has no titles
 * or can't be listed. [entry] focuses its first title, and [onLoaded]
 * tells that it has some.
 */
@Composable
fun TvCatalogShelf(
    serverId: String,
    row: CatalogRow,
    title: String,
    state: LazyListState,
    focus: FocusMemory,
    key: String,
    entry: FocusRequester? = null,
    tmdbId: Int = 0,
    onLoaded: () -> Unit = {},
) {
    val pages = LocalPages.current
    val request = remember(row, tmdbId) { catalogRequest(row.catalog.id, row.mediaKind.number, tmdbId, CatalogFilters(), 1) }
    val result by discovered(serverId, "Discover", request, DiscoverResponse::parseFrom)
    val page = result?.getOrNull()
    if (result?.isFailure == true || (page != null && page.titlesCount == 0)) return
    if (page != null) LaunchedEffect(Unit) { onLoaded() }
    TvShelf(title, state) {
        if (page == null) {
            items(8) { CardPlaceholder() }
        } else {
            val titles = page.titlesList.distinctBy(::titleKey)
            items(titles.size, key = { titleKey(titles[it]) }) { i ->
                val t = titles[i]
                TvTitleCard(
                    t,
                    (if (i == 0 && entry != null) Modifier.focusRequester(entry) else Modifier).focusKey(focus, "$key/${titleKey(t)}"),
                ) { pages.openTitle(serverId, t.mediaKindValue, t.tmdbId) }
            }
            if (page.page < page.totalPages) {
                item(key = "more") {
                    SeeAllCard(Modifier.focusKey(focus, "$key/more")) {
                        pages.openCatalog(serverId, row.catalog.id, row.mediaKind.number, tmdbId)
                    }
                }
            }
        }
    }
}

/**
 * The card ending a row, in place of the link beside its title on other
 * layouts: a poster of `color.surfaceRaised` with the forward arrow and
 * See All.
 */
@Composable
fun SeeAllCard(modifier: Modifier = Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Column(modifier.width(theme.layout.poster).focusCard(focused).selectable(state, onClick)) {
        Column(
            Modifier
                .focusArtwork(focused, MavioRadius.md)
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(MavioRadius.md))
                .background(theme.colors.surfaceRaised),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12, Alignment.CenterVertically),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            MavioIcon(R.drawable.ic_forward, size = 40.dp)
            Text(stringResource(R.string.home_more), style = theme.type.headline.text)
        }
    }
}

/**
 * The card of a discovered title on a television, which opens it: its
 * poster with its status, its name, its year and kind; with focus it
 * grows and its poster is framed (docs/tv.md §4). It fills its cell when
 * [fill], as in a grid.
 */
@Composable
fun TvTitleCard(title: DiscoveredTitle, modifier: Modifier = Modifier, fill: Boolean = false, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val series = title.mediaKind == MediaKind.MEDIA_KIND_SERIES
    Column(modifier.then(if (fill) Modifier.fillMaxWidth() else Modifier.width(theme.layout.poster)).focusCard(focused).selectable(state, onClick)) {
        Box(Modifier.focusArtwork(focused, MavioRadius.md)) {
            Artwork(
                sized(title.posterUrl, 342),
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(MavioRadius.md)),
                icon = if (series) R.drawable.ic_shows else R.drawable.ic_movies,
                contentDescription = title.name,
            )
            StatusMark(title.status, Modifier.align(Alignment.TopEnd).padding(10.dp))
        }
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(title.name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(
            listOfNotNull(title.year.takeIf { it > 0 }?.toString(), stringResource(if (series) R.string.discover_series else R.string.discover_movie)).joinToString(" · "),
            style = theme.type.footnote.text,
            color = theme.colors.textSecondary,
            maxLines = 1,
        )
    }
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
            .background(if (available) theme.colors.success else theme.colors.scrim, CircleShape)
            .padding(3.dp),
        size = 24.dp,
        tint = Color.White,
        contentDescription = statusLabel(status)?.let { stringResource(it) },
    )
}

/** A catalog's titles as they load, page by page. */
internal class TitlePages {
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

    /** Starts again from the first page, after a failure. */
    fun reset() {
        titles.clear()
        seen.clear()
        next = 1
        more = true
        failed = false
    }
}

/**
 * A catalog of Discover on a television (docs/tv.md §6.3), without the
 * navigation bar: its name, then a grid of its titles loading page by
 * page, focus starting on the first. Back returns to where it was opened
 * from.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun CatalogPage(page: Page.Catalog) {
    val discover = LocalDiscover.current
    val pages = LocalPages.current
    val found = discover.discovery?.catalogs?.firstOrNull { it.id == page.catalogId }
    val loaded = page.titles
    var attempt by remember { mutableIntStateOf(0) }
    suspend fun next() {
        if (loaded.loading || !loaded.more || loaded.failed) return
        loaded.loading = true
        try {
            loaded.add(
                discover.call(
                    page.serverId,
                    "Discover",
                    catalogRequest(page.catalogId, page.mediaKind, page.tmdbId, page.filters, loaded.next),
                    DiscoverResponse::parseFrom,
                ),
            )
        } catch (e: CancellationException) {
            throw e
        } catch (_: Exception) {
            loaded.failed = true
        } finally {
            loaded.loading = false
        }
    }
    val grid = page.grid
    LaunchedEffect(attempt) {
        next()
        // The next page as the end comes near.
        snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .distinctUntilChanged()
            .filter { it >= grid.layoutInfo.totalItemsCount - 18 }
            .collect { next() }
    }
    val first = remember { FocusRequester() }
    val ready = loaded.titles.isNotEmpty()
    LaunchedEffect(ready) { if (ready) page.focus.restore(first) }
    val kind = MediaKind.forNumber(page.mediaKind) ?: MediaKind.MEDIA_KIND_UNSPECIFIED
    val title = when {
        found == null -> ""
        page.tmdbId > 0 -> found.name
        else -> rowTitle(CatalogRow(found, kind))
    }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyVerticalGrid(
            GridCells.Adaptive(theme.layout.poster),
            Modifier.fillMaxSize(),
            state = grid,
            contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s48),
            horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
        ) {
            gridLine("header") {
                Text(
                    title.ifEmpty { " " },
                    Modifier.padding(top = MavioSpacing.s48, bottom = MavioSpacing.s8),
                    style = theme.type.title1.text,
                    fontWeight = FontWeight.Bold,
                    maxLines = 1,
                )
            }
            when {
                loaded.titles.isEmpty() && loaded.failed -> gridLine("error") {
                    StatusView(
                        stringResource(R.string.state_error),
                        Modifier.padding(top = MavioSpacing.s48),
                        icon = R.drawable.ic_error,
                        action = stringResource(R.string.action_retry) to {
                            loaded.reset()
                            attempt++
                        },
                    )
                }
                loaded.titles.isEmpty() && !loaded.more -> gridLine("empty") {
                    StatusView(stringResource(R.string.state_empty), Modifier.padding(top = MavioSpacing.s48))
                }
                loaded.titles.isEmpty() -> items(12) { CardPlaceholder() }
                else -> {
                    items(loaded.titles.size, key = { titleKey(loaded.titles[it]) }) { i ->
                        val t = loaded.titles[i]
                        TvTitleCard(
                            t,
                            (if (i == 0) Modifier.focusRequester(first) else Modifier).focusKey(page.focus, titleKey(t)),
                            fill = true,
                        ) { pages.openTitle(page.serverId, t.mediaKindValue, t.tmdbId) }
                    }
                    if (loaded.more) {
                        gridLine("more") {
                            Box(Modifier.fillMaxWidth().padding(MavioSpacing.s16), Alignment.Center) {
                                CircularProgressIndicator(Modifier.width(32.dp), color = Color.White)
                            }
                        }
                    }
                }
            }
        }
    }
}
