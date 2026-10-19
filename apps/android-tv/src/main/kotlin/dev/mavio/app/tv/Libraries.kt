package dev.mavio.app.tv

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.Filter
import dev.mavio.core.Libraries
import dev.mavio.core.LibraryEntry
import dev.mavio.core.Row as HomeRow
import dev.mavio.core.Sort
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.Focusable
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvChoice
import dev.mavio.ui.TvChoiceMenu
import dev.mavio.ui.TvChoices
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.focusArtwork
import dev.mavio.ui.focusCard
import dev.mavio.ui.focused
import dev.mavio.ui.rowTitle
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

/** How many items a page of a library's grid holds. */
private const val PAGE_SIZE = 60u

/** The items a library's All shows. */
enum class Shown(val label: Int) {
    ALL(R.string.library_filter_all),
    UNPLAYED(R.string.library_filter_unplayed),
    PLAYED(R.string.library_filter_played),
    FAVORITES(R.string.library_filter_favorites),
}

/** The name of an order of a library's items. */
private val Sort.label: Int
    get() = when (this) {
        Sort.NAME -> R.string.library_sort_name
        Sort.DATE_ADDED -> R.string.library_sort_date_added
        Sort.RELEASED -> R.string.library_sort_released
        Sort.RATING -> R.string.library_sort_rating
        Sort.LAST_PLAYED -> R.string.library_sort_last_played
        Sort.RANDOM -> R.string.library_sort_random
    }

/**
 * The Libraries tab (docs/tv.md §3): the libraries of every server signed
 * in to, grouped by server in the user's order, as the web UI's, each a
 * card that opens it. [entry] focuses the first; while [claim] the page
 * takes focus back to where it was.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun LibrariesPage(model: AppModel, list: LazyListState, focus: FocusMemory, entry: FocusRequester, claim: Boolean) {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val shown = servers.filter { it.id in signedIn }
    val pages = LocalPages.current
    // Each server's libraries load on their own, so that one that can't
    // be reached holds up none of the others. Those loaded before show at
    // once on the tab's return.
    val libraries = remember {
        mutableStateOf(shown.mapNotNull { s -> model.heldLibraries(s.id)?.let { s.id to Result.success(it) } }.toMap())
    }
    shown.forEach { server ->
        LaunchedEffect(server.id, revisions[server.id]) {
            if (model.heldLibraries(server.id) != null) return@LaunchedEffect
            val result = runCatching { model.libraries(server.id) }
            libraries.value += server.id to result
        }
    }
    val first = shown.firstNotNullOfOrNull { libraries.value[it.id]?.getOrNull()?.libraries?.firstOrNull()?.let { l -> it.id to l.id } }
    LaunchedEffect(first) { if (claim && first != null) focus.restore(entry) }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyColumn(
            Modifier.fillMaxSize(),
            state = list,
            contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, top = 128.dp, bottom = MavioSpacing.s48),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            item(key = "title") { Text(stringResource(R.string.nav_libraries), style = theme.type.title1.text, fontWeight = FontWeight.Bold) }
            shown.forEach { server ->
                if (shown.size > 1) {
                    item(key = "server/${server.id}") {
                        Text(server.name, Modifier.padding(top = MavioSpacing.s16), style = theme.type.title3.text)
                    }
                }
                val result = libraries.value[server.id]
                val found = result?.getOrNull()
                when {
                    result == null -> item(key = "loading/${server.id}") { LibraryLine(List(3) { null }, server.id, focus, null) }
                    found == null || (found.offline && found.libraries.isEmpty()) -> item(key = "offline/${server.id}") {
                        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                            MavioIcon(R.drawable.ic_offline, size = 24.dp, tint = theme.colors.textTertiary)
                            Text(stringResource(R.string.home_server_unreachable, server.name), style = theme.type.callout.text, color = theme.colors.textTertiary)
                        }
                    }
                    else -> found.libraries.chunked(LIBRARY_COLUMNS).forEachIndexed { i, line ->
                        item(key = "libraries/${server.id}/$i") {
                            LibraryLine(line, server.id, focus, entry.takeIf { first == server.id to line.first().id }) { library ->
                                pages.openLibrary(server.id, library.id, library.name)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** How many libraries a line holds: as many thumbnails as fit across the screen. */
private const val LIBRARY_COLUMNS = 3

/**
 * A line of a server's libraries, each a thumbnail's card of its artwork
 * and name; nulls stand for libraries on their way. [entry] focuses the
 * first.
 */
@Composable
private fun LibraryLine(libraries: List<LibraryEntry?>, serverId: String, focus: FocusMemory, entry: FocusRequester?, onOpen: (LibraryEntry) -> Unit = {}) {
    // Room above and below for a focused card to grow.
    Row(Modifier.padding(vertical = MavioSpacing.s12), horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter)) {
        libraries.forEachIndexed { i, library ->
            val shape = RoundedCornerShape(MavioRadius.md)
            if (library == null) {
                Box(Modifier.weight(1f).aspectRatio(16f / 9f).background(theme.colors.surfaceRaised, shape))
                return@forEachIndexed
            }
            val state = remember { Focusable() }
            val focused = state.focused()
            Column(
                (if (i == 0 && entry != null) Modifier.focusRequester(entry) else Modifier)
                    .shakes()
                    .focusKey(focus, "$serverId/${library.id}")
                    .weight(1f)
                    .focusCard(focused)
                    .selectable(state) { onOpen(library) },
            ) {
                Artwork(
                    library.imageUrl,
                    Modifier
                        .focusArtwork(focused, MavioRadius.md)
                        .fillMaxWidth()
                        .aspectRatio(16f / 9f)
                        .clip(shape),
                    icon = R.drawable.ic_movies,
                    contentDescription = library.name,
                )
                Spacer(Modifier.height(MavioSpacing.s12))
                Text(library.name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        repeat(LIBRARY_COLUMNS - libraries.size) { Spacer(Modifier.weight(1f)) }
    }
}

/**
 * A library (docs/tv.md §6.3), without the navigation bar: its name at the
 * top left beside its Recommended and All, which focus chooses, starting
 * on the current one; Recommended's rows, or All's filters, order and
 * grid. Back returns to where it was opened from.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun LibraryPage(model: AppModel, page: Page.Library) {
    val choice = remember { mapOf(false to FocusRequester(), true to FocusRequester()) }
    // Choosing replaces the list the choice lies in; focus stays on it.
    var refocus by remember { mutableStateOf(false) }
    LaunchedEffect(page.all) {
        if (!refocus) return@LaunchedEffect
        refocus = false
        repeat(5) {
            withFrameNanos {}
            if (runCatching { choice.getValue(page.all).requestFocus() }.getOrDefault(false)) return@LaunchedEffect
        }
    }
    LaunchedEffect(Unit) { page.focus.restore(choice.getValue(page.all)) }
    val header: @Composable () -> Unit = {
        Row(
            Modifier.padding(top = MavioSpacing.s48, bottom = MavioSpacing.s8),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(page.name, style = theme.type.title1.text, fontWeight = FontWeight.Bold, maxLines = 1)
            TvChoices(choice.getValue(page.all)) {
                listOf(false to R.string.library_recommended, true to R.string.library_all).forEach { (all, label) ->
                    TvChoice(stringResource(label), page.all == all, Modifier.focusRequester(choice.getValue(all)), horizontal = 24.dp) {
                        if (page.all != all) {
                            refocus = true
                            page.all = all
                        }
                    }
                }
            }
        }
    }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        if (page.all) AllItems(model, page, header) else Recommended(model, page, header)
    }
}

/** A library's Recommended: its rows, as Home's. */
@Composable
private fun Recommended(model: AppModel, page: Page.Library, header: @Composable () -> Unit) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    val actions = remember(pages) { HomeActions(open = pages::open, play = { pages.play(it.serverId, it.itemId, it.title) }) }
    var attempt by remember { mutableIntStateOf(0) }
    // Kept by the page, so returning from an item's details finds its card.
    LaunchedEffect(page.serverId, page.libraryId, revisions[page.serverId], attempt) {
        page.recommended = runCatching { model.client.recommended(page.serverId, page.libraryId) }
    }
    val result = page.recommended
    LazyColumn(Modifier.fillMaxSize(), state = page.list, contentPadding = PaddingValues(bottom = MavioSpacing.s48)) {
        item(key = "header") { Box(Modifier.padding(horizontal = theme.layout.margin)) { header() } }
        val list = result?.getOrNull()
        when {
            result == null -> item { StatusView(stringResource(R.string.state_loading), Modifier.padding(top = MavioSpacing.s64)) }
            list == null -> item {
                StatusView(
                    stringResource(R.string.state_error),
                    Modifier.padding(top = MavioSpacing.s64),
                    icon = R.drawable.ic_error,
                    action = stringResource(R.string.action_retry) to { attempt++ },
                )
            }
            list.isEmpty() -> item { StatusView(stringResource(R.string.state_empty), Modifier.padding(top = MavioSpacing.s64)) }
            else -> items(list, key = { it.id }) { row ->
                HomeShelf(row, rowTitle(row), several = false, page.rows.of(row.id), entry = null, page.focus, actions)
            }
        }
    }
}

/** The pages of a library's grid loaded so far. */
/** All's items loaded so far, kept by the page for [key]'s filter and order. */
internal class LibraryItems(val key: Any) {
    val cards = mutableStateListOf<Card>()
    var total by mutableStateOf<UInt?>(null)
    var more by mutableStateOf(true)
    var failed by mutableStateOf(false)
    var loading by mutableStateOf(false)
}

/**
 * A library's All: its filters, which focus applies, the button of its
 * order, how many items there are, and the grid of its items.
 */
@Composable
private fun AllItems(model: AppModel, page: Page.Library, header: @Composable () -> Unit) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    var attempt by remember { mutableIntStateOf(0) }
    val filter = Filter(
        played = when (page.shown) {
            Shown.PLAYED -> true
            Shown.UNPLAYED -> false
            else -> null
        },
        favorite = page.shown == Shown.FAVORITES,
        genre = null,
        tag = null,
        studio = null,
    )
    // Kept by the page, so returning from an item's details finds its card.
    val key = listOf(filter, page.sort, revisions[page.serverId])
    val loaded = page.items?.takeIf { it.key == key } ?: LibraryItems(key).also { page.items = it }
    suspend fun next() {
        if (loaded.loading || !loaded.more || loaded.failed) return
        loaded.loading = true
        runCatching {
            model.client.libraryItems(page.serverId, page.libraryId, page.sort, filter, loaded.cards.size.toUInt(), PAGE_SIZE)
        }.onSuccess {
            loaded.cards.addAll(it.cards)
            if (it.total != null) loaded.total = it.total
            loaded.more = it.more
        }.onFailure { loaded.failed = true }
        loaded.loading = false
    }
    val grid = page.grid
    LaunchedEffect(loaded) {
        if (loaded.cards.isEmpty()) next()
        // The next page as the end comes near.
        snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .distinctUntilChanged()
            .filter { it >= grid.layoutInfo.totalItemsCount - 18 }
            .collect { next() }
    }
    val filters = remember { Shown.entries.associateWith { FocusRequester() } }
    LazyVerticalGrid(
        GridCells.Adaptive(theme.layout.poster),
        Modifier.fillMaxSize(),
        state = grid,
        contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s48),
        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
    ) {
        gridLine("header") { header() }
        gridLine("filters") {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16), verticalAlignment = Alignment.CenterVertically) {
                TvChoices(filters.getValue(page.shown)) {
                    Shown.entries.forEach { k ->
                        TvChoice(stringResource(k.label), page.shown == k, Modifier.focusRequester(filters.getValue(k)), horizontal = 20.dp) { page.shown = k }
                    }
                }
                SortButton(page.sort) { page.sort = it }
                Spacer(Modifier.weight(1f))
                loaded.total?.let {
                    Text(
                        pluralStringResource(R.plurals.library_item_count, it.toInt(), it.toInt()),
                        style = theme.type.callout.text,
                        color = theme.colors.textTertiary,
                    )
                }
            }
        }
        when {
            loaded.cards.isEmpty() && loaded.failed -> gridLine("error") {
                StatusView(
                    stringResource(R.string.state_error),
                    Modifier.padding(top = MavioSpacing.s48),
                    icon = R.drawable.ic_error,
                    action = stringResource(R.string.action_retry) to {
                        page.items = null
                        attempt++
                    },
                )
            }
            loaded.cards.isEmpty() && !loaded.more -> gridLine("empty") {
                StatusView(stringResource(R.string.state_empty), Modifier.padding(top = MavioSpacing.s48))
            }
            loaded.cards.isEmpty() -> items(12) { CardPlaceholder() }
            else -> items(loaded.cards, key = { "${it.serverId}/${it.itemId}" }) { card ->
                TvItemCard(card, Modifier.focusKey(page.focus, "${card.serverId}/${card.itemId}"), fill = true) { pages.open(card) }
            }
        }
    }
}

/** An item of a grid across all its columns. */
internal fun LazyGridScope.gridLine(key: String, content: @Composable () -> Unit) =
    item(key = key, span = { GridItemSpan(maxLineSpan) }) { content() }

/**
 * The button naming the order of the items, white with focus, which opens
 * the orders to choose from with focus on the current one; while they are
 * open it keeps a `color.fill` highlight (docs/tv.md §5).
 */
@Composable
private fun SortButton(sort: Sort, onSort: (Sort) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val button = remember { FocusRequester() }
    var reopen by remember { mutableIntStateOf(0) }
    // Closing the menu returns focus to the button.
    LaunchedEffect(reopen) {
        if (reopen == 0) return@LaunchedEffect
        withFrameNanos {}
        runCatching { button.requestFocus() }
    }
    val state = remember { Focusable() }
    val focused = state.focused()
    val color = if (focused) Color.Black else theme.colors.textSecondary
    Box {
        Row(
            Modifier
                .shakes()
                .focusRequester(button)
                // Nothing focusable sits to its right: the count is text.
                .focusProperties { right = FocusRequester.Cancel }
                .height(48.dp)
                .clip(CircleShape)
                .background(
                    when {
                        focused -> Color.White
                        open -> theme.colors.fill
                        else -> Color.Transparent
                    },
                )
                .selectable(state) { open = true }
                .padding(horizontal = 20.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(R.drawable.ic_sort, size = 22.dp, tint = color)
            Text(stringResource(R.string.library_sort_by, stringResource(sort.label)), style = theme.type.headline.text, color = color)
        }
        if (open) {
            val close = {
                open = false
                reopen += 1
            }
            TvChoiceMenu(Sort.entries, sort, below = 48.dp, width = 320.dp, label = { stringResource(it.label) }, onDismiss = close) {
                close()
                onSort(it)
            }
        }
    }
}
