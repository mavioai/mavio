package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.SearchResults
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ItemCard
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleKey
import mavio.acquisition.v1.Discover.SearchTitlesResponse
import mavio.acquisition.v1.searchTitlesRequest

/**
 * The last search's results, kept while the app runs so that coming back
 * to Search from a result shows them without asking the servers again.
 */
class SearchCache {
    /** The query searched, with its results; null while none was. */
    var last by mutableStateOf<Pair<String, Result<List<SearchResults>>>?>(null)

    /** A query to search as Search shows, as the assistant asks; taken by it. */
    var asked by mutableStateOf<String?>(null)
}

/** Search's kept results. */
val LocalSearch = compositionLocalOf<SearchCache> { error("no search") }

/**
 * Search, as the web UI's phone layout (docs/clients.md §5.4): a field,
 * confirmed with the keyboard's search key, then a shelf of results for
 * each server, all asked at once.
 */
@Composable
fun SearchScreen(model: AppModel) {
    var text by rememberSaveable { mutableStateOf("") }
    var query by rememberSaveable { mutableStateOf("") }
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val several = signedIn.size > 1
    // Confirming the query searched again searches again.
    var again by rememberSaveable { mutableIntStateOf(0) }
    val cache = LocalSearch.current
    // The results of the query confirmed, kept; null while they are on their way.
    val results = cache.last?.takeIf { it.first == query }?.second
    LaunchedEffect(query, again) {
        if (query.isEmpty() || results != null) return@LaunchedEffect
        cache.last = query to runCatching { model.client.search(query) }
    }
    // The titles the server that discovers finds, from Discover.
    val discover = LocalDiscover.current
    val discovery = discover.discovery
    val request = remember(query) {
        searchTitlesRequest {
            this.query = query
            language = discoverLanguage
            page = 1
        }
    }
    val found = if (discovery == null || query.isEmpty()) null else {
        discovered(discovery.serverId, "SearchTitles", request, SearchTitlesResponse::parseFrom, again).value
    }
    val titles = found?.getOrNull()?.titlesList.orEmpty().distinctBy(::titleKey)
    val focus = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    LaunchedEffect(Unit) { if (query.isEmpty() && cache.asked == null) focus.requestFocus() }
    // The assistant's query, searched as it asks.
    LaunchedEffect(cache.asked) {
        val asked = cache.asked ?: return@LaunchedEffect
        cache.asked = null
        text = asked
        query = asked
    }
    val colors = theme.colors
    val navigator = LocalNavigator.current
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            top = LocalTopInset.current + MavioSpacing.s16,
            bottom = LocalBottomInset.current + MavioSpacing.s24,
        ),
    ) {
        item(key = "field") {
            Row(
                Modifier
                    .padding(horizontal = theme.layout.margin)
                    .padding(bottom = MavioSpacing.s8)
                    .fillMaxWidth()
                    .heightIn(min = 48.dp)
                    .background(colors.surfaceRaised, RoundedCornerShape(MavioRadius.lg))
                    .padding(horizontal = MavioSpacing.s16),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MavioIcon(R.drawable.ic_search, tint = colors.textSecondary)
                val style = theme.type.body.text.copy(color = colors.textPrimary)
                BasicTextField(
                    text,
                    { text = it },
                    Modifier.weight(1f).focusRequester(focus),
                    textStyle = style,
                    singleLine = true,
                    cursorBrush = SolidColor(colors.accent),
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                    keyboardActions = KeyboardActions(onSearch = {
                        if (text.trim() == query && query.isNotEmpty()) {
                            cache.last = null
                            discovery?.let { discover.forget(it.serverId, "SearchTitles", request) }
                            again++
                        }
                        query = text.trim()
                        keyboard?.hide()
                    }),
                    decorationBox = { field ->
                        Box {
                            if (text.isEmpty()) Text(stringResource(R.string.search_placeholder), style = style, color = colors.textTertiary)
                            field()
                        }
                    },
                )
                if (text.isNotEmpty()) {
                    MavioIcon(
                        R.drawable.ic_close,
                        Modifier.clickable {
                            text = ""
                            focus.requestFocus()
                            keyboard?.show()
                        },
                        size = 18.dp,
                        tint = colors.textSecondary,
                    )
                }
            }
        }
        if (query.isEmpty()) return@LazyColumn
        val list = results?.getOrNull().orEmpty()
        val searching = results == null || (discovery != null && found == null)
        val shown = list.any { it.cards.isNotEmpty() || it.error != null } || titles.isNotEmpty()
        when {
            results?.isFailure == true && titles.isEmpty() -> item {
                StatusView(stringResource(R.string.state_error), Modifier.padding(top = MavioSpacing.s48), icon = R.drawable.ic_error)
            }
            // One spinner while nothing is drawn yet, and No Results once all answered with nothing.
            !shown && searching -> item {
                Box(Modifier.fillMaxWidth().padding(top = MavioSpacing.s48), Alignment.Center) { CircularProgressIndicator() }
            }
            !shown -> item {
                StatusView(stringResource(R.string.search_no_results), Modifier.padding(top = MavioSpacing.s48), icon = R.drawable.ic_search)
            }
            // A server's row shows once it answers with results, never while it searches.
            else -> items(list, key = { it.serverId }) { found ->
                when {
                    found.error != null -> Row(
                        Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MavioIcon(R.drawable.ic_offline, size = 18.dp, tint = colors.textTertiary)
                        Text(
                            stringResource(R.string.home_server_unreachable, found.serverName),
                            style = theme.type.callout.text,
                            color = colors.textTertiary,
                        )
                    }
                    found.cards.isNotEmpty() -> Shelf(if (several) found.serverName else stringResource(R.string.search_server)) {
                        items(found.cards, key = { "${it.serverId}/${it.itemId}" }) { card ->
                            ItemCard(card, Modifier.clickable { navigator.open(card) })
                        }
                    }
                }
            }
        }
        if (discovery != null && titles.isNotEmpty()) {
            item(key = "discover") {
                Shelf(stringResource(R.string.search_discover)) {
                    items(titles, key = ::titleKey) { TitleCard(discovery.serverId, it) }
                }
            }
        }
    }
}
