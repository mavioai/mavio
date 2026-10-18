package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
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
import dev.mavio.ui.ItemCard
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.rowTitle
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter

/** How many items a page of a library's grid holds. */
private const val PAGE_SIZE = 60u

/** The libraries of every server signed in to, grouped by server in the user's order, as the web UI's. */
@Composable
fun LibrariesScreen(model: AppModel) {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val shown = servers.filter { it.id in signedIn }
    val navigator = LocalNavigator.current
    val margin = theme.layout.margin
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            start = margin,
            end = margin,
            top = LocalTopInset.current,
            bottom = LocalBottomInset.current + MavioSpacing.s24,
        ),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        item(key = "title") { ScreenTitle(stringResource(R.string.nav_libraries)) }
        shown.forEach { server ->
            if (shown.size > 1) {
                item(key = "server/${server.id}") {
                    Text(server.name, Modifier.padding(top = MavioSpacing.s12), style = theme.type.title3.text)
                }
            }
            item(key = "libraries/${server.id}") {
                // Each server's libraries load on their own, so that one
                // that can't be reached holds up none of the others.
                var libraries by remember(server.id) { mutableStateOf<Result<Libraries>?>(null) }
                LaunchedEffect(server.id, revisions[server.id]) {
                    libraries = runCatching { model.client.libraries(server.id) }
                }
                val result = libraries
                val list = result?.getOrNull()
                when {
                    result == null -> LibraryRow(List(3) { null }) {}
                    list == null || (list.offline && list.libraries.isEmpty()) -> Row(
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MavioIcon(R.drawable.ic_offline, size = 18.dp, tint = theme.colors.textTertiary)
                        Text(
                            stringResource(R.string.home_server_unreachable, server.name),
                            style = theme.type.callout.text,
                            color = theme.colors.textTertiary,
                        )
                    }
                    else -> LibraryRow(list.libraries) { library ->
                        navigator.open(Route.Library(server.id, library.id, library.name))
                    }
                }
            }
        }
    }
}

/**
 * A server's libraries as cards in rows filling the width, each at least a
 * thumbnail wide; nulls stand for libraries on their way.
 */
@Composable
private fun LibraryRow(libraries: List<LibraryEntry?>, onOpen: (LibraryEntry) -> Unit) {
    val gutter = theme.layout.gutter
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val columns = maxOf(1, ((maxWidth + gutter) / (theme.layout.thumbnail + gutter)).toInt())
        Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
            libraries.chunked(columns).forEach { line ->
                Row(horizontalArrangement = Arrangement.spacedBy(gutter)) {
                    line.forEach { library ->
                        Column(
                            Modifier
                                .weight(1f)
                                .then(if (library != null) Modifier.clickable { onOpen(library) } else Modifier),
                        ) {
                            val shape = RoundedCornerShape(MavioRadius.md)
                            if (library == null) {
                                Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f).background(theme.colors.surfaceRaised, shape))
                            } else {
                                Artwork(
                                    library.imageUrl,
                                    Modifier.fillMaxWidth().aspectRatio(16f / 9f).clip(shape),
                                    icon = R.drawable.ic_movies,
                                    contentDescription = library.name,
                                )
                                Spacer(Modifier.height(MavioSpacing.s8))
                                Text(library.name, style = theme.type.headline.text)
                            }
                        }
                    }
                    repeat(columns - line.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
    }
}

/** A screen's large title, as the web UI's. */
@Composable
private fun ScreenTitle(title: String) {
    Text(
        title,
        Modifier.padding(top = MavioSpacing.s24, bottom = MavioSpacing.s8),
        style = theme.type.title1.text,
        fontWeight = FontWeight.Bold,
    )
}

/** An item of a grid across its whole width. */
private fun LazyGridScope.line(key: String, content: @Composable () -> Unit) =
    item(key = key, span = { GridItemSpan(maxLineSpan) }) { content() }

/** What a library's grid shows of its items. */
private enum class Shown(val label: Int) {
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
 * A library, as the web UI's (docs/clients.md §6.3.1): its name and the
 * choice of its Recommended tab of shelves or the grid of all its items,
 * filtered and sorted; opened on a genre, the grid of that genre's items.
 */
@Composable
fun LibraryScreen(model: AppModel, route: Route.Library) {
    var all by rememberSaveable { mutableStateOf(route.genre != null) }
    val header: @Composable () -> Unit = {
        FlowRow(
            Modifier.padding(top = MavioSpacing.s24, bottom = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            itemVerticalAlignment = Alignment.Bottom,
        ) {
            Text(route.name, style = theme.type.title1.text, fontWeight = FontWeight.Bold)
            Segmented(
                listOf(stringResource(R.string.library_recommended), stringResource(R.string.library_all)),
                if (all) 1 else 0,
            ) { all = it == 1 }
        }
    }
    if (all) AllItems(model, route, header) else Recommended(model, route, header)
}

/** A choice of a few, side by side in a well, the chosen one raised, as the web UI's. */
@Composable
private fun Segmented(labels: List<String>, chosen: Int, onChoose: (Int) -> Unit) {
    val colors = theme.colors
    Row(
        Modifier
            .clip(RoundedCornerShape(9.dp))
            .background(colors.fill)
            .padding(2.dp),
    ) {
        labels.forEachIndexed { i, label ->
            val active = i == chosen
            Text(
                label,
                Modifier
                    .then(if (active) Modifier.shadow(1.dp, RoundedCornerShape(7.dp)) else Modifier)
                    .clip(RoundedCornerShape(7.dp))
                    .background(if (active) colors.surfaceOverlay else Color.Transparent)
                    .clickable { onChoose(i) }
                    .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s4),
                style = theme.type.callout.text,
                fontWeight = FontWeight.SemiBold,
                color = if (active) colors.textPrimary else colors.textSecondary,
            )
        }
    }
}

/** A library's Recommended tab: its shelves, as Home's. */
@Composable
private fun Recommended(model: AppModel, route: Route.Library, header: @Composable () -> Unit) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    var attempt by remember { mutableIntStateOf(0) }
    var rows by remember { mutableStateOf<Result<List<HomeRow>>?>(null) }
    LaunchedEffect(route.serverId, route.libraryId, revisions[route.serverId], attempt) {
        rows = runCatching { model.client.recommended(route.serverId, route.libraryId) }
    }
    val result = rows
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(top = LocalTopInset.current, bottom = LocalBottomInset.current + MavioSpacing.s24),
    ) {
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
            else -> items(list, key = { it.id }) { row -> HomeShelf(row, rowTitle(row), several = false) }
        }
    }
}

/** The pages of a library's grid loaded so far. */
private class Pages {
    val cards = mutableStateListOf<Card>()
    var total by mutableStateOf<UInt?>(null)
    var more by mutableStateOf(true)
    var failed by mutableStateOf(false)
    var loading by mutableStateOf(false)
}

/** A library's grid of all its items, with its filters, its order and how many there are. */
@Composable
private fun AllItems(model: AppModel, route: Route.Library, header: @Composable () -> Unit) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    var shown by rememberSaveable { mutableStateOf(Shown.ALL) }
    var sort by rememberSaveable { mutableStateOf(Sort.NAME) }
    var genre by rememberSaveable { mutableStateOf(route.genre) }
    var attempt by remember { mutableIntStateOf(0) }
    val filter = Filter(
        played = when (shown) {
            Shown.PLAYED -> true
            Shown.UNPLAYED -> false
            else -> null
        },
        favorite = shown == Shown.FAVORITES,
        genre = genre,
    )
    val pages = remember(filter, sort, revisions[route.serverId], attempt) { Pages() }
    suspend fun next() {
        if (pages.loading || !pages.more || pages.failed) return
        pages.loading = true
        runCatching {
            model.client.libraryItems(route.serverId, route.libraryId, sort, filter, pages.cards.size.toUInt(), PAGE_SIZE)
        }.onSuccess {
            pages.cards.addAll(it.cards)
            if (it.total != null) pages.total = it.total
            pages.more = it.more
        }.onFailure { pages.failed = true }
        pages.loading = false
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
    val navigator = LocalNavigator.current
    val margin = theme.layout.margin
    val colors = theme.colors
    LazyVerticalGrid(
        GridCells.Adaptive(theme.layout.poster),
        Modifier.fillMaxSize(),
        state = grid,
        contentPadding = PaddingValues(
            start = margin,
            end = margin,
            top = LocalTopInset.current,
            bottom = LocalBottomInset.current + MavioSpacing.s24,
        ),
        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter * 1.5f),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
    ) {
        line("header") { header() }
        line("filters") {
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                genre?.let { value ->
                    Row(
                        Modifier
                            .height(32.dp)
                            .clip(CircleShape)
                            .background(colors.accent)
                            .clickable { genre = null }
                            .padding(start = 14.dp, end = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            stringResource(R.string.library_genre_value, value),
                            style = theme.type.callout.text,
                            fontWeight = FontWeight.SemiBold,
                            color = colors.onAccent,
                        )
                        MavioIcon(R.drawable.ic_close, size = 14.dp, tint = colors.onAccent)
                    }
                }
                Shown.entries.forEach { k ->
                    val active = k == shown
                    Text(
                        stringResource(k.label),
                        Modifier
                            .height(32.dp)
                            .clip(CircleShape)
                            .background(if (active) colors.button else colors.fill)
                            .clickable { shown = k }
                            .padding(horizontal = 14.dp)
                            .wrapContentHeight(Alignment.CenterVertically),
                        style = theme.type.callout.text,
                        fontWeight = FontWeight.Medium,
                        color = if (active) colors.onButton else colors.textPrimary,
                    )
                }
                SortMenu(sort) { sort = it }
                pages.total?.let {
                    Text(
                        pluralStringResource(R.plurals.library_item_count, it.toInt(), it.toInt()),
                        style = theme.type.callout.text,
                        color = colors.textTertiary,
                    )
                }
            }
        }
        when {
            pages.cards.isEmpty() && pages.failed -> line("error") {
                StatusView(
                    stringResource(R.string.state_error),
                    Modifier.padding(top = MavioSpacing.s48),
                    icon = R.drawable.ic_error,
                    action = stringResource(R.string.action_retry) to { attempt++ },
                )
            }
            pages.cards.isEmpty() && !pages.more -> line("empty") {
                StatusView(stringResource(R.string.state_empty), Modifier.padding(top = MavioSpacing.s48))
            }
            pages.cards.isEmpty() -> items(12) { CardPlaceholder() }
            else -> items(pages.cards, key = { "${it.serverId}/${it.itemId}" }) { card ->
                ItemCard(card, Modifier.clickable { navigator.open(card) }, width = null)
            }
        }
    }
}

/** The button naming the order of the items, which opens the orders to choose from. */
@Composable
private fun SortMenu(sort: Sort, onSort: (Sort) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val colors = theme.colors
    Box {
        Row(
            Modifier
                .height(32.dp)
                .clip(CircleShape)
                .background(if (open) colors.fill else Color.Transparent)
                .clickable { open = true }
                .padding(horizontal = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(R.drawable.ic_sort, size = 16.dp, tint = colors.textSecondary)
            Text(
                stringResource(R.string.library_sort_by, stringResource(sort.label)),
                style = theme.type.callout.text,
                color = colors.textSecondary,
            )
        }
        DropdownMenu(open, onDismissRequest = { open = false }) {
            Sort.entries.forEach { k ->
                DropdownMenuItem(
                    text = { Text(stringResource(k.label), style = theme.type.callout.text) },
                    onClick = {
                        open = false
                        onSort(k)
                    },
                    trailingIcon = { if (k == sort) MavioIcon(R.drawable.ic_check, size = 18.dp) },
                )
            }
        }
    }
}
