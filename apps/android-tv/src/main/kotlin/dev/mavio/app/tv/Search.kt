package dev.mavio.app.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.RowContent
import dev.mavio.core.SearchResults
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.Focusable
import dev.mavio.ui.LocalTheme
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.focused
import dev.mavio.ui.rowTitle
import dev.mavio.ui.selectable
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.delay

/** The keys of Search's keyboard, in six columns. */
private val KEYS = "abcdefghijklmnopqrstuvwxyz1234567890".toList()

/**
 * Search on a television (docs/tv.md §6.4), without the navigation bar:
 * on the left the query with its caret, space, delete and clear, and a
 * keyboard of letters and digits, where focus starts; on the right Home's
 * rows while the query is empty, and each server's results in a grid as
 * it is typed, after a pause. Within the keyboard back deletes a
 * character, and leaves the page only once there is none.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun SearchPage(model: AppModel, page: Page.Search) {
    // The search runs after a pause in typing.
    LaunchedEffect(page.text) {
        val query = page.text.trim()
        if (query == page.query) return@LaunchedEffect
        delay(300)
        page.query = query
    }
    LaunchedEffect(page.query) {
        val query = page.query
        if (query.isEmpty() || page.results?.first == query) return@LaunchedEffect
        page.results = null
        page.results = query to runCatching { model.client.search(query) }
    }
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) { page.focus.restore(first) }
    var typing by remember { mutableStateOf(false) }
    val erase = { page.text = page.text.dropLast(1) }
    BackHandler(typing && page.text.isNotEmpty(), erase)
    val type = { c: String -> page.text += c }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    Row(Modifier.fillMaxSize().padding(start = theme.layout.margin), horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter)) {
        Column(
            Modifier
                .width(480.dp)
                .padding(top = MavioSpacing.s48)
                .onFocusChanged { typing = it.hasFocus }
                .focusGroup(),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            Query(page.text)
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                val wide = Modifier.weight(1f).height(56.dp)
                SearchKey(stringResource(R.string.search_space), wide.focusKey(page.focus, "space")) { type(" ") }
                SearchKey(stringResource(R.string.search_backspace), wide.focusKey(page.focus, "erase"), erase)
                SearchKey(stringResource(R.string.search_clear), wide.focusKey(page.focus, "clear")) { page.text = "" }
            }
            KEYS.chunked(6).forEachIndexed { i, line ->
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    line.forEachIndexed { j, c ->
                        val key = Modifier.weight(1f).aspectRatio(1f)
                        SearchKey(
                            c.uppercase(),
                            (if (i == 0 && j == 0) key.focusRequester(first) else key).focusKey(page.focus, "key/$c"),
                        ) { type(c.toString()) }
                    }
                }
            }
        }
        // Its rows and grids keep only a little room for a focused card to grow.
        val narrow = theme.copy(layout = theme.layout.copy(margin = 24.dp))
        CompositionLocalProvider(LocalTheme provides narrow, LocalBringIntoViewSpec provides scrolling) {
            Box(Modifier.weight(1f).fillMaxHeight()) {
                if (page.query.isEmpty()) Suggestions(model, page) else Results(model, page)
            }
        }
    }
}

/** The query, where the keyboard types: its blinking accent caret, or the placeholder. */
@Composable
private fun Query(text: String) {
    // It blinks as a text field's caret does, a half second on and off.
    var shown by remember { mutableStateOf(true) }
    LaunchedEffect(text) {
        shown = true
        while (true) {
            delay(500)
            shown = !shown
        }
    }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .background(theme.colors.surfaceRaised, RoundedCornerShape(MavioRadius.lg))
            .padding(horizontal = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(R.drawable.ic_search, size = 28.dp, tint = theme.colors.textSecondary)
        Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
            val style = theme.type.title3.text
            Text(text, Modifier.weight(1f, fill = false), style = style, maxLines = 1, overflow = TextOverflow.StartEllipsis)
            Box(
                Modifier
                    .padding(horizontal = 1.dp)
                    .width(2.dp)
                    .height(30.dp)
                    .alpha(if (shown) 1f else 0f)
                    .background(theme.colors.accent, RoundedCornerShape(1.dp)),
            )
            if (text.isEmpty()) {
                Text(stringResource(R.string.search_placeholder), style = style, color = theme.colors.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** A key of the keyboard: `color.fill`, white with black text with focus. */
@Composable
private fun RowScope.SearchKey(label: String, modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Box(
        modifier
            .clip(RoundedCornerShape(MavioRadius.lg))
            .background(if (focused) Color.White else theme.colors.fill)
            .selectable(state, onClick),
        Alignment.Center,
    ) {
        Text(label, style = theme.type.headline.text, color = if (focused) Color.Black else theme.colors.textPrimary, maxLines = 1)
    }
}

/** Home's rows, while nothing is searched. */
@Composable
private fun Suggestions(model: AppModel, page: Page.Search) {
    val home by model.home.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    val actions = remember(pages) { HomeActions(open = pages::open, play = { pages.play(it.serverId, it.itemId, it.title) }) }
    if (home.isEmpty() || home.all { it.content == RowContent.Loading }) {
        StatusView(stringResource(R.string.state_loading))
        return
    }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = page.list,
        contentPadding = PaddingValues(top = MavioSpacing.s32, bottom = MavioSpacing.s48),
    ) {
        itemsIndexed(home, key = { _, row -> row.id }) { _, row ->
            HomeShelf(row, rowTitle(row), signedIn.size > 1, page.rows.of(row.id), entry = null, page.focus, actions)
        }
    }
}

/** Each server's results in a grid under its name, or that it can't be reached. */
@Composable
private fun Results(model: AppModel, page: Page.Search) {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val several = signedIn.size > 1
    val pages = LocalPages.current
    val result = page.results?.takeIf { it.first == page.query }?.second
    val list = result?.getOrNull()
    LazyVerticalGrid(
        GridCells.Adaptive(theme.layout.poster),
        Modifier.fillMaxSize(),
        state = page.grid,
        contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s48, bottom = MavioSpacing.s48),
        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
    ) {
        when {
            result == null -> servers.filter { it.id in signedIn }.forEach { server ->
                if (several) gridLine("pending/${server.id}") { ServerTitle(server.name) }
                items(6) { CardPlaceholder() }
            }
            list == null -> gridLine("error") {
                StatusView(stringResource(R.string.state_error), Modifier.padding(top = MavioSpacing.s48), icon = R.drawable.ic_error)
            }
            list.all { it.cards.isEmpty() && it.error == null } -> gridLine("none") {
                StatusView(stringResource(R.string.search_no_results), Modifier.padding(top = MavioSpacing.s48), icon = R.drawable.ic_search)
            }
            else -> list.forEach { found -> serverResults(found, several, page) { pages.open(it) } }
        }
    }
}

/** A server's results: its name when there are several, then its cards, or that it can't be reached. */
private fun LazyGridScope.serverResults(
    found: SearchResults,
    several: Boolean,
    page: Page.Search,
    open: (Card) -> Unit,
) {
    when {
        found.error != null -> gridLine("error/${found.serverId}") {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                MavioIcon(R.drawable.ic_offline, size = 24.dp, tint = theme.colors.textTertiary)
                Text(
                    stringResource(R.string.home_server_unreachable, found.serverName),
                    style = theme.type.callout.text,
                    color = theme.colors.textTertiary,
                )
            }
        }
        found.cards.isNotEmpty() -> {
            if (several) gridLine("server/${found.serverId}") { ServerTitle(found.serverName) }
            items(found.cards, key = { "${it.serverId}/${it.itemId}" }) { card ->
                TvItemCard(card, Modifier.focusKey(page.focus, "${card.serverId}/${card.itemId}"), fill = true) { open(card) }
            }
        }
    }
}

/** A server's name over its results. */
@Composable
private fun ServerTitle(name: String) {
    Column {
        Text(name, style = theme.type.title3.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Spacer(Modifier.height(MavioSpacing.s8))
    }
}
