package dev.mavio.app.tv

import androidx.activity.compose.BackHandler
import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.SignInState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch

/** The tabs of the navigation bar (docs/tv.md §3). */
enum class Tab(val title: Int) {
    Home(R.string.nav_home),
}

/** The app: its sign-in until a Mavio account signs in, then its tabs. */
@Composable
fun Root(model: AppModel) {
    val state by model.signInState.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    if (state is SignInState.SignedOut && signedIn.isEmpty()) {
        SignInScreen(model)
    } else {
        Shell(model)
    }
}

/**
 * The tabs' pages under the navigation bar (docs/tv.md §3): moving focus
 * to a tab opens it in place; back moves focus from a page to its tab and
 * returns the page to its top, and from a tab to Home's.
 */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun Shell(model: AppModel) {
    var tab by rememberSaveable { mutableStateOf(Tab.Home) }
    val tabs = remember { Tab.entries.associateWith { FocusRequester() } }
    var inPage by remember { mutableStateOf(false) }
    val list = rememberLazyListState()
    val rows = remember { RowStates() }
    val focus = remember { FocusMemory() }
    val entry = remember { FocusRequester() }
    val scope = rememberCoroutineScope()
    val pages = remember { Pages() }
    val actions = remember(pages) { HomeActions(open = pages::open, play = { pages.play(it.serverId, it.itemId, it.title) }) }
    val top = pages.top
    BackHandler(top == null && (inPage || tab != Tab.Home)) {
        if (inPage) {
            focus.last = null
            tabs.getValue(tab).requestFocus()
            scope.launch {
                list.scrollToItem(0)
                rows.reset()
            }
        } else {
            tabs.getValue(Tab.Home).requestFocus()
        }
    }
    // Back from a page returns to what opened it.
    BackHandler(top != null) { pages.back() }
    CompositionLocalProvider(LocalPages provides pages) {
        Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
            // A page covers the tabs, which keep where they were for its return.
            when (top) {
                is Page.Details -> key(top) { DetailsPage(model, top) }
                is Page.Player -> key(top) { PlayerPage(model, top) }
                null -> {
                    Box(Modifier.fillMaxSize().onFocusChanged { inPage = it.hasFocus }) {
                        when (tab) {
                            Tab.Home -> HomePage(model, list, rows, focus, entry, actions)
                        }
                    }
                    NavBar(tab, tabs, scrolled(list), entry) { tab = it }
                }
            }
        }
    }
}

/** Whether a page has scrolled from its top. */
@Composable
private fun scrolled(list: LazyListState): Boolean {
    val scrolled by remember(list) { derivedStateOf { list.firstVisibleItemIndex > 0 || list.firstVisibleItemScrollOffset > 0 } }
    return scrolled
}

/**
 * The navigation bar: the tabs in a pill centered at the top, the current
 * one filled, over a band of the background once the page scrolls; focus
 * entering the tabs goes to the current one, and down from them to the
 * page's first element, [page].
 */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun NavBar(current: Tab, tabs: Map<Tab, FocusRequester>, scrolled: Boolean, page: FocusRequester, onTab: (Tab) -> Unit) {
    val band by animateColorAsState(if (scrolled) theme.colors.tvBackground else Color.Transparent, label = "band")
    Box(
        Modifier
            .fillMaxWidth()
            .background(band)
            .padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s24),
        Alignment.Center,
    ) {
        Row(
            Modifier
                .clip(CircleShape)
                .background(theme.colors.surfaceRaised.copy(alpha = 0.6f))
                .focusRestorer(tabs.getValue(current))
                .focusGroup()
                .padding(6.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Tab.entries.forEach { t ->
                TabPill(t, t == current, Modifier.focusRequester(tabs.getValue(t)).focusProperties { down = page }) { onTab(t) }
            }
        }
    }
}

/** A tab: filled while current, white with focus, which opens it. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun TabPill(tab: Tab, current: Boolean, modifier: Modifier, onOpen: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val fill = when {
        focused -> Color.White
        current -> theme.colors.fillSelected
        else -> Color.Transparent
    }
    Text(
        stringResource(tab.title),
        modifier
            .clip(CircleShape)
            .background(fill)
            .onFocusChanged { if (it.isFocused && !current) onOpen() }
            .selectable(state, onOpen)
            .padding(horizontal = 28.dp, vertical = 10.dp),
        style = theme.type.headline.text,
        color = when {
            focused -> Color.Black
            current -> theme.colors.textPrimary
            else -> theme.colors.textSecondary
        },
    )
}
