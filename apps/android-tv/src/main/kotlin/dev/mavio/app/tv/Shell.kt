package dev.mavio.app.tv

import androidx.activity.compose.BackHandler
import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.key
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.admin.AdminHost
import dev.mavio.admin.AdminPage
import dev.mavio.admin.AdminScreen
import dev.mavio.admin.EditMetadataScreen
import dev.mavio.admin.IdentifyScreen
import dev.mavio.admin.ItemImagesScreen
import dev.mavio.admin.LocalAdmin
import dev.mavio.core.AppModel
import dev.mavio.core.SignInState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.FocusShake
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.LocalFocusShake
import dev.mavio.ui.R
import dev.mavio.ui.RecallingFocus
import dev.mavio.ui.TvChoice
import dev.mavio.ui.movesFocus
import dev.mavio.ui.rememberDiscover
import dev.mavio.ui.theme
import kotlinx.coroutines.launch

/** The tabs of the navigation bar (docs/tv.md §3). */
enum class Tab(val title: Int) {
    Home(R.string.nav_home),
    Libraries(R.string.nav_libraries),

    /** Shown while a server discovers. */
    Discover(R.string.nav_discover),
}

/** What a tab's page keeps while another shows: its scroll, its rows' and the element that had focus. */
private class TabState {
    val list = LazyListState()
    val rows = RowStates()
    val focus = FocusMemory()

    /** Its first element, which down from the tabs reaches. */
    val entry = FocusRequester()
}

/**
 * The app: its sign-in until a Mavio account signs in, then its tabs. The
 * D-pad moves focus, and where focus cannot move the focused element
 * shakes (docs/tv.md §4).
 */
@Composable
fun Root(model: AppModel) {
    val state by model.signInState.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    CompositionLocalProvider(LocalFocusShake provides remember { FocusShake() }) {
        Box(Modifier.fillMaxSize().movesFocus()) {
            if (state is SignInState.SignedOut && signedIn.isEmpty()) {
                SignInScreen(model)
            } else {
                val discover = rememberDiscover(model)
                CompositionLocalProvider(LocalDiscover provides discover) { Shell(model) }
            }
        }
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
    var chosen by rememberSaveable { mutableStateOf(Tab.Home) }
    // Discover's tab shows while a server discovers.
    val shown = Tab.entries.filter { it != Tab.Discover || LocalDiscover.current.discovery != null }
    val tab = chosen.takeIf { it in shown } ?: Tab.Home
    val tabs = remember { Tab.entries.associateWith { FocusRequester() } }
    var inPage by remember { mutableStateOf(false) }
    val states = remember { Tab.entries.associateWith { TabState() } }
    val state = states.getValue(tab)
    // Whether the tab's page takes focus as it shows: as the app starts and
    // as a page over it closes, not as focus on the tabs opens it.
    var claim by remember { mutableStateOf(true) }
    val scope = rememberCoroutineScope()
    val pages = remember { Pages() }
    val actions = remember(pages) { HomeActions(open = pages::open, play = { pages.play(it.serverId, it.itemId, it.title) }) }
    // Search and the settings opened from the bar return focus to what
    // opened them, its pill or the menu button.
    val search = remember { FocusRequester() }
    val menu = remember { FocusRequester() }
    var fromBar by remember { mutableStateOf<FocusRequester?>(null) }
    var onOpener by remember { mutableStateOf(false) }
    val top = pages.top
    if (top != null) claim = fromBar == null
    LaunchedEffect(top == null) {
        val opener = fromBar
        if (top != null || opener == null) return@LaunchedEffect
        fromBar = null
        // The window gives focus back to the current tab a frame or two late.
        repeat(10) {
            withFrameNanos {}
            if (!onOpener) runCatching { opener.requestFocus() }
        }
    }
    BackHandler(top == null && (inPage || tab != Tab.Home)) {
        if (inPage) {
            state.focus.last = null
            tabs.getValue(tab).requestFocus()
            scope.launch {
                state.list.scrollToItem(0)
                state.rows.reset()
            }
        } else {
            tabs.getValue(Tab.Home).requestFocus()
        }
    }
    // Back from a page returns to what opened it.
    BackHandler(top != null) { pages.back() }
    FollowGroup(model, pages)
    // The administration's pages open over the settings; the television controls no other device.
    val admin = remember(pages) {
        object : AdminHost {
            override fun open(page: AdminPage) = pages.openAdmin(page)
            override fun back() = pages.back()
            override val remote = null
        }
    }
    // The administration's pages keep their state while pages open over them.
    val kept = rememberSaveableStateHolder()
    val admins = remember { mutableSetOf<Int>() }
    val open = pages.all.filterIsInstance<Page.Admin>().map { it.id }.toSet()
    LaunchedEffect(open) {
        (admins - open).forEach { kept.removeState(it) }
        admins.clear()
        admins += open
    }
    CompositionLocalProvider(LocalPages provides pages, LocalAdmin provides admin) {
        Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
            // A page covers the tabs, which keep where they were for its return.
            when (top) {
                is Page.Details -> key(top) { DetailsPage(model, top) }
                is Page.Library -> key(top) { LibraryPage(model, top) }
                is Page.Credits -> key(top) { CreditsPage(model, top) }
                is Page.Catalog -> key(top) { CatalogPage(model, top) }
                is Page.Title -> key(top) { TitlePage(top) }
                is Page.Person -> key(top) { PersonPage(top) }
                is Page.Collection -> key(top) { CollectionPage(top) }
                is Page.TitleCredits -> key(top) { TitleCreditsPage(top) }
                is Page.Search -> key(top) { SearchPage(model, top) }
                is Page.Player -> key(top) { PlayerPage(model, top) }
                is Page.Metadata -> key(top) {
                    when (top.kind) {
                        MetadataPage.Edit -> EditMetadataScreen(model, top.serverId, top.itemId, pages::back)
                        MetadataPage.Identify -> IdentifyScreen(model, top.serverId, top.itemId, pages::back)
                        MetadataPage.Images -> ItemImagesScreen(model, top.serverId, top.itemId)
                    }
                }
                is Page.Settings -> key(top) { SettingsPage(model, top) }
                is Page.AddServer -> key(top) { SignInScreen(model, adding = true) { pages.back() } }
                is Page.Admin -> key(top) {
                    kept.SaveableStateProvider(top.id) {
                        RecallingFocus(top.recall) { AdminScreen(model, top.page) }
                    }
                }
                null -> {
                    Box(Modifier.fillMaxSize().onFocusChanged { inPage = it.hasFocus }) {
                        key(tab) {
                            when (tab) {
                                Tab.Home -> HomePage(model, state.list, state.rows, state.focus, state.entry, claim, actions)
                                Tab.Libraries -> LibrariesPage(model, state.list, state.focus, state.entry, claim)
                                Tab.Discover -> DiscoverPage(state.list, state.rows, state.focus, state.entry, claim)
                            }
                        }
                    }
                    NavBar(
                        model,
                        tab,
                        shown,
                        tabs,
                        search,
                        menu,
                        { onOpener = it },
                        scrolled(state.list),
                        state.entry,
                        onSearch = {
                            fromBar = search
                            pages.openSearch()
                        },
                        onSettings = {
                            fromBar = menu
                            pages.openSettings()
                        },
                    ) {
                        claim = false
                        chosen = it
                    }
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
 * page's first element, [page]. Search follows the tabs, opening only
 * when selected; at the end, the device's SyncPlay group while it is in
 * one, and the menu button.
 */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun NavBar(
    model: AppModel,
    current: Tab,
    shown: List<Tab>,
    tabs: Map<Tab, FocusRequester>,
    search: FocusRequester,
    menu: FocusRequester,
    onOpenerFocus: (Boolean) -> Unit,
    scrolled: Boolean,
    page: FocusRequester,
    onSearch: () -> Unit,
    onSettings: () -> Unit,
    onTab: (Tab) -> Unit,
) {
    val band by animateColorAsState(if (scrolled) theme.colors.tvBackground else Color.Transparent, label = "band")
    Box(
        Modifier
            .fillMaxWidth()
            .background(band)
            // Focus entering the bar from the page goes to the current tab.
            .focusProperties { onEnter = { tabs.getValue(current).requestFocus() } }
            .focusGroup()
            .padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s24),
        Alignment.Center,
    ) {
        Row(
            Modifier
                .clip(CircleShape)
                .background(theme.colors.surfaceRaised.copy(alpha = 0.6f))
                .padding(6.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            shown.forEach { t ->
                TabPill(t, t == current, Modifier.focusRequester(tabs.getValue(t)).focusProperties { down = page }) { onTab(t) }
            }
            // Search hides the tabs, so it opens only when selected.
            TvChoice(
                stringResource(R.string.nav_search),
                false,
                Modifier.onFocusChanged { onOpenerFocus(it.isFocused) }.focusRequester(search).focusProperties { down = page },
                onFocus = false,
                onChoose = onSearch,
            )
        }
        Row(
            Modifier.align(Alignment.CenterEnd),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // The device's group, while it is in one, before the menu button.
            GroupIndicator(model, Modifier.focusProperties { down = page })
            MenuButton(
                model,
                menu,
                Modifier.onFocusChanged { onOpenerFocus(it.hasFocus) }.focusProperties { down = page },
                onSettings,
            )
        }
    }
}

/** A tab: filled while current, white with focus, which opens it. */
@Composable
private fun TabPill(tab: Tab, current: Boolean, modifier: Modifier, onOpen: () -> Unit) {
    TvChoice(stringResource(tab.title), current, modifier, onChoose = onOpen)
}
