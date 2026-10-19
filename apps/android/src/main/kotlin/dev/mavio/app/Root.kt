package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.movableContentOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.input.nestedscroll.NestedScrollConnection
import androidx.compose.ui.input.nestedscroll.NestedScrollSource
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation3.runtime.NavEntry
import androidx.navigation3.runtime.rememberSaveableStateHolderNavEntryDecorator
import androidx.navigation3.ui.NavDisplay
import dev.chrisbanes.haze.HazeInput
import dev.chrisbanes.haze.HazeState
import dev.chrisbanes.haze.blur.HazeBlurStyle
import dev.chrisbanes.haze.blur.HazeColorEffect
import dev.chrisbanes.haze.blur.hazeBlur
import dev.chrisbanes.haze.hazeSource
import dev.chrisbanes.haze.rememberHazeState
import dev.mavio.admin.AdminHost
import dev.mavio.admin.AdminPage
import dev.mavio.admin.AdminScreen
import dev.mavio.admin.EditMetadataScreen
import dev.mavio.admin.IdentifyScreen
import dev.mavio.admin.ItemImagesScreen
import dev.mavio.admin.LocalAdmin
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.SignInState
import dev.mavio.design.MavioSpacing
import dev.mavio.design.TabletLayout
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.DiscoverState
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.LocalTheme
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.Wordmark
import dev.mavio.ui.blurAvailable
import dev.mavio.ui.discovered
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlin.math.roundToInt

/** A screen pushed: it slides in from the end over the one before, which moves aside a little. */
private fun pushTransition(): ContentTransform =
    slideInHorizontally(tween(PAGE_MILLIS)) { it } togetherWith
        slideOutHorizontally(tween(PAGE_MILLIS)) { -it / 4 }

/**
 * Going back: the screen slides out to the end, uncovering the one before,
 * and follows the finger while the back gesture is under way.
 */
private fun popTransition(): ContentTransform =
    (slideInHorizontally(tween(PAGE_MILLIS)) { -it / 4 } togetherWith
        slideOutHorizontally(tween(PAGE_MILLIS)) { it }).apply { targetContentZIndex = -1f }

/** How long a screen takes to slide in or out. */
private const val PAGE_MILLIS = 300

/** A screen of a tab's stack. */
sealed interface Route {
    /** Home. */
    data object Home : Route

    /** The libraries of the servers. */
    data object Libraries : Route

    /** Search across the servers. */
    data object Search : Route

    /** The account, the servers and the settings. */
    data object Settings : Route

    /** A page of the settings. */
    data class Setting(val page: SettingsPage) : Route

    /** A page of a server's administration. */
    data class Admin(val page: AdminPage) : Route




    /** An item's metadata edited. */
    data class EditMetadata(val serverId: String, val itemId: String) : Route

    /** An item identified from the providers' search results. */
    data class Identify(val serverId: String, val itemId: String) : Route

    /** An item's images chosen. */
    data class ItemImages(val serverId: String, val itemId: String) : Route





    /** Signing in to one more server. */
    data object AddServer : Route

    /** Discover's catalogs. */
    data object Discover : Route

    /** The titles of a catalog, of a kind of title, those like a title when tmdbId is set, filtered alike when filters are set. */
    data class Catalog(
        val serverId: String,
        val catalogId: String,
        val mediaKind: Int,
        val tmdbId: Int = 0,
        val filters: CatalogFilters = CatalogFilters(),
    ) : Route

    /** A discovered movie or series. */
    data class Title(val serverId: String, val mediaKind: Int, val tmdbId: Int) : Route

    /** The cast and crew of a discovered title, or its crew alone. */
    data class TitleCredits(val serverId: String, val mediaKind: Int, val tmdbId: Int, val crew: Boolean) : Route

    /** A discovered person. */
    data class DiscoveredPerson(val serverId: String, val tmdbId: Int) : Route

    /** A discovered collection of movies. */
    data class DiscoveredCollection(val serverId: String, val tmdbId: Int) : Route

    /**
     * A library, opened on the items of a genre, keyword or studio when one
     * is given; its name looked up when not known.
     */
    data class Library(
        val serverId: String,
        val libraryId: String,
        val name: String?,
        val genre: String? = null,
        val tag: String? = null,
        val studio: String? = null,
    ) : Route

    /** An item's whole cast and crew, or its crew alone. */
    data class Credits(val serverId: String, val itemId: String, val crew: Boolean) : Route

    /** A person's page and the server's items crediting them. */
    data class Person(val serverId: String, val personId: String) : Route

    /** An item's details. */
    data class Item(val serverId: String, val itemId: String) : Route

    /** An item playing, from start or else where it was left. */
    data class Player(
        val serverId: String,
        val itemId: String,
        val title: String,
        val mediaSourceId: String?,
        val start: java.time.Duration?,
    ) : Route
}

/** The tabs of the phone's tab bar, as the web UI's. */
enum class Tab(val root: Route, @StringRes val label: Int, @DrawableRes val icon: Int, @DrawableRes val filled: Int) {
    Home(Route.Home, R.string.nav_home, R.drawable.ic_home, R.drawable.ic_home_fill),
    Libraries(Route.Libraries, R.string.nav_libraries, R.drawable.ic_movies, R.drawable.ic_movies_fill),

    /** Shown while a server discovers. */
    Discover(Route.Discover, R.string.nav_discover, R.drawable.ic_discover, R.drawable.ic_discover_fill),
}

/** Opens screens on the current tab's stack. */
class Navigator(private val stack: () -> SnapshotStateList<Route>) {
    /** The screen shown. */
    val top: Route? get() = stack().lastOrNull()

    /** Opens a route over the current screen. */
    fun open(route: Route) {
        stack().add(route)
    }

    /** Opens an item's details. */
    fun open(card: Card) = open(Route.Item(card.serverId, card.itemId))

    /** Plays an item, a version of it when mediaSourceId is set, from start when it is set. */
    fun play(serverId: String, itemId: String, title: String, mediaSourceId: String?, start: java.time.Duration?) =
        open(Route.Player(serverId, itemId, title, mediaSourceId, start))

    /** Opens a route in the current screen's place. */
    fun replace(route: Route) {
        stack().removeLastOrNull()
        stack().add(route)
    }

    /** Goes back a screen. */
    fun back() {
        stack().removeLastOrNull()
    }
}

/** The navigator of the composition. */
val LocalNavigator = staticCompositionLocalOf<Navigator> { error("no navigator") }

/**
 * The app's root: sign-in until a Mavio account or a server is signed in,
 * then the tabs, each with its own stack of screens.
 */
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
 * The places' stacks of screens, shown in the phone's layout or the wide
 * one as the window's width decides (docs/clients.md §6.3), which keep what
 * they hold as the window crosses from one to the other.
 */
@Composable
private fun Shell(model: AppModel) {
    // The place shown: a tab's, or in the wide layout one of the sidebar's.
    var place by remember { mutableStateOf<Route>(Route.Home) }
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val discover = remember { DiscoverState(model) }
    val search = remember { SearchCache() }
    LaunchedEffect(signedIn) { discover.load(model.signedInServers.map { it.id }) }
    // Discover's tab shows while a server discovers.
    val tabs = Tab.entries.filter { it != Tab.Discover || discover.discovery != null }
    LaunchedEffect(tabs) { if (place == Route.Discover && Tab.Discover !in tabs) place = Route.Home }
    val stacks = remember { mutableStateMapOf<Route, SnapshotStateList<Route>>().apply { Tab.entries.forEach { put(it.root, mutableStateListOf(it.root)) } } }
    val select = { p: Route ->
        if (p !in stacks) stacks[p] = mutableStateListOf(p)
        if (p == place) {
            // Choosing the place shown goes back to its first screen.
            stacks.getValue(p).let { while (it.size > 1) it.removeAt(it.lastIndex) }
        }
        place = p
    }
    val stack = stacks[place] ?: stacks.getValue(Route.Home)
    val navigator = remember { Navigator { stacks[place] ?: stacks.getValue(Route.Home) } }
    val admin = remember(navigator) {
        object : AdminHost {
            override fun open(page: AdminPage) = navigator.open(Route.Admin(page))
            override fun back() = navigator.back()
            override val remote: @Composable (AppModel, String, String, () -> Unit) -> Unit =
                { model, serverId, sessionId, onDismiss -> RemoteSheet(model, serverId, sessionId, onDismiss) }
        }
    }
    val haze = rememberHazeState()
    val backdrop = remember { HeaderBackdrop() }
    // Each place keeps its screens' state while another shows.
    val saved = rememberSaveableStateHolder()
    val bars = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    val statusBar = WindowInsets.statusBars.asPaddingValues().calculateTopPadding()
    // The player and the sign-in take the whole screen.
    val chrome = stack.lastOrNull().let { it !is Route.Player && it != Route.AddServer }
    FollowGroup(model, navigator)
    // The screens move between the phone's layout and the wide one with their state.
    val screens = remember {
        movableContentOf { modifier: Modifier, rootInset: Dp, settings: Boolean ->
            val stack = stacks[place] ?: stacks.getValue(Route.Home)
            val bars = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
            // Keyed by the place's text, which the saved state can hold.
            saved.SaveableStateProvider(place.toString()) {
                key(place) {
                    NavDisplay(
                        backStack = stack,
                        modifier = modifier.hazeSource(haze),
                        onBack = { if (settings && stack.size <= 2) select(Route.Home) else stack.removeLastOrNull() },
                        // A screen keeps its state, as where it was scrolled, under those opened over it.
                        entryDecorators = listOf(rememberSaveableStateHolderNavEntryDecorator()),
                        entryProvider = { route ->
                            NavEntry(route) {
                                CompositionLocalProvider(
                                    LocalBottomInset provides if (route == stack.firstOrNull()) rootInset else bars,
                                    LocalHeaderRoute provides route,
                                ) {
                                    // Opaque, so that the screen it slides over or uncovers does not show through it.
                                    Box(Modifier.fillMaxSize().background(theme.colors.background).nestedScroll(backdrop.tracking(route))) { Screen(model, it) }
                                }
                            }
                        },
                        transitionSpec = { pushTransition() },
                        popTransitionSpec = { popTransition() },
                        predictivePopTransitionSpec = { popTransition() },
                    )
                }
            }
        }
    }
    BoxWithConstraints(Modifier.fillMaxSize().background(theme.colors.background)) {
        val wide = maxWidth >= 600.dp
        // The wide layout's settings have a sidebar of their own and show a section beside it.
        val settings = wide && place == Route.Settings
        LaunchedEffect(settings, stack.size) { if (settings && stack.size == 1) stack.add(FIRST_SETTING) }
        // Above the section the sidebar chose, Back leads to the page under it.
        val backShown = chrome && stack.size > if (settings) 2 else 1
        val content = @Composable { modifier: Modifier, rootInset: Dp -> screens(modifier, rootInset, settings) }
        if (wide) {
            // Up to 840 dp the sidebar is a rail whose toggle lays the full
            // sidebar over the content; wider, the toggle collapses it.
            val overlay = maxWidth < 840.dp
            var expanded by rememberSaveable { mutableStateOf(true) }
            var over by remember { mutableStateOf(false) }
            LaunchedEffect(place, overlay) { over = false }
            val rail = overlay || !expanded
            val layout = TabletLayout
            CompositionLocalProvider(
                LocalTheme provides theme.copy(layout = layout),
                LocalNavigator provides navigator,
                LocalDiscover provides discover,
                LocalAdmin provides admin,
                LocalSearch provides search,
                LocalHeaderBackdrop provides backdrop,
                // A page opened from another keeps clear of Back above it.
                LocalTopInset provides statusBar + if (backShown) BACK_ROW else MavioSpacing.s12,
            ) {
                Row(Modifier.fillMaxSize()) {
                    if (chrome && settings) {
                        SettingsSidebar(model, stack.getOrNull(1), Modifier.width(layout.sidebar), onHome = { select(Route.Home) }) { r ->
                            while (stack.size > 1) stack.removeAt(stack.lastIndex)
                            stack.add(r)
                        }
                        Box(Modifier.fillMaxHeight().width(1.dp).background(theme.colors.border))
                    } else if (chrome) {
                        SidebarPanel(
                            model,
                            rail,
                            place,
                            discover.discovery != null,
                            Modifier.width(if (rail) layout.rail else layout.sidebar),
                            onToggle = { if (overlay) over = true else expanded = !expanded },
                            onSelect = select,
                        )
                        Box(Modifier.fillMaxHeight().width(1.dp).background(theme.colors.border))
                    }
                    Box(Modifier.weight(1f).fillMaxHeight()) {
                        // A section is centered in the space left, at most 900 dp wide.
                        content(if (settings) Modifier.fillMaxHeight().widthIn(max = 900.dp).align(Alignment.TopCenter) else Modifier.fillMaxSize(), bars)
                        if (backShown) {
                            Box(
                                Modifier
                                    .statusBarsPadding()
                                    .padding(12.dp)
                                    .size(36.dp)
                                    .shadow(8.dp, CircleShape)
                                    .clip(CircleShape)
                                    .glass(haze, glassTint(), radius = 12.dp, saturation = 1.8f)
                                    .clickable { navigator.back() },
                                Alignment.Center,
                            ) {
                                MavioIcon(R.drawable.ic_back, size = 18.dp, contentDescription = stringResource(R.string.action_back))
                            }
                        }
                    }
                }
                if (over && chrome) {
                    Box(
                        Modifier
                            .fillMaxSize()
                            .background(theme.colors.scrim)
                            .clickable(remember { MutableInteractionSource() }, null) { over = false },
                    ) {
                        SidebarPanel(
                            model,
                            rail = false,
                            place,
                            discover.discovery != null,
                            Modifier.width(layout.sidebar).shadow(16.dp).clickable(remember { MutableInteractionSource() }, null) {},
                            onToggle = { over = false },
                            onSelect = select,
                        )
                    }
                }
            }
        } else {
            // The tab whose place shows; a library chosen in the wide layout is the libraries'.
            val tab = Tab.entries.firstOrNull { it.root == place } ?: if (place is Route.Library) Tab.Libraries else Tab.Home
            CompositionLocalProvider(
                LocalNavigator provides navigator,
                LocalDiscover provides discover,
                LocalAdmin provides admin,
                LocalSearch provides search,
                LocalHeaderBackdrop provides backdrop,
                // The pages scroll under the header and the floating tab bar, which blur them.
                LocalTopInset provides HEADER_HEIGHT + statusBar,
            ) {
                // The tab bar shows on the tabs' first screens alone.
                val tabBar = stack.size == 1
                content(Modifier.fillMaxSize(), TAB_BAR_HEIGHT + 24.dp + bars)
                if (chrome) {
                    val top = stack.last()
                    Header(
                        model,
                        back = stack.size > 1,
                        backdrop,
                        top,
                        Modifier.align(Alignment.TopCenter),
                        haze,
                    )
                    AnimatedVisibility(
                        tabBar,
                        Modifier.align(Alignment.BottomCenter),
                        enter = slideInVertically(tween(PAGE_MILLIS)) { it },
                        exit = slideOutVertically(tween(PAGE_MILLIS)) { it },
                    ) {
                        TabBar(tabs, tab, haze) { select(it.root) }
                    }
                }
            }
        }
        if (chrome) {
            // The group's indicator, above the phone's tab bar where it shows.
            val tabBar = !wide && stack.size == 1
            CompositionLocalProvider(LocalNavigator provides navigator) {
                GroupIndicator(
                    model,
                    Modifier
                        .align(Alignment.BottomEnd)
                        .padding(end = 16.dp, bottom = bars + if (tabBar) TAB_BAR_HEIGHT + 24.dp else 16.dp),
                )
            }
        }
    }
}

/** The height of the row Back has above a page opened from another in the wide layout. */
private val BACK_ROW = 60.dp

/** The height of the tab bar's pill. */
private val TAB_BAR_HEIGHT = 60.dp

/** The height of the header, below the status bar. */
private val HEADER_HEIGHT = 52.dp

/** How far a page's content keeps clear of the top, under the header. */
val LocalTopInset = staticCompositionLocalOf { 0.dp }

/** How far a page's content keeps clear of the bottom, under the tab bar. */
val LocalBottomInset = staticCompositionLocalOf { 0.dp }

@Composable
private fun Screen(model: AppModel, route: Route) {
    when (route) {
        Route.Home -> HomeScreen(model)
        Route.Libraries -> LibrariesScreen(model)
        Route.Search -> SearchScreen(model)
        is Route.Library -> LibraryScreen(model, route)
        is Route.Item -> DetailsScreen(model, route.serverId, route.itemId)
        is Route.Player -> PlayerScreen(model, route)
        Route.Discover -> DiscoverScreen()
        is Route.Catalog -> CatalogScreen(model, route)
        is Route.Title -> TitleScreen(route)
        is Route.TitleCredits -> TitleCreditsScreen(route)
        is Route.Credits -> CreditsScreen(model, route)
        is Route.Person -> PersonScreen(model, route)
        is Route.DiscoveredPerson -> DiscoveredPersonScreen(route)
        is Route.DiscoveredCollection -> DiscoveredCollectionScreen(route)
        Route.AddServer -> {
            val navigator = LocalNavigator.current
            SignInScreen(model, adding = true) { navigator.back() }
        }
        else -> Box(Modifier.fillMaxSize().padding(top = LocalTopInset.current)) {
            when (route) {
                Route.Settings -> SettingsScreen(model)
                is Route.Setting -> SettingsPageScreen(model, route.page)
                is Route.Admin -> AdminScreen(model, route.page)
                is Route.EditMetadata -> EditMetadataScreen(model, route.serverId, route.itemId, LocalNavigator.current::back)
                is Route.Identify -> IdentifyScreen(model, route.serverId, route.itemId, LocalNavigator.current::back)
                is Route.ItemImages -> ItemImagesScreen(model, route.serverId, route.itemId)
                else -> StatusView(stringResource(R.string.state_loading))
            }
        }
    }
}

/**
 * The phone's top, clear as the web UI's and iOS's: the wordmark, or Back
 * on a page opened from another, then search and the user's initial on a
 * circle of the tab bar's glass, which opens the settings. Its icons turn
 * white over a hero reaching up under it ([backdrop] of [route]); once the page has
 * scrolled under it with no hero there, an edge fades in behind it: the
 * page blurred under the background, fading out a little below it.
 */
@Composable
private fun Header(model: AppModel, back: Boolean, backdrop: HeaderBackdrop, route: Route, modifier: Modifier, haze: HazeState) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    val state by model.signInState.collectAsStateWithLifecycle()
    val servers by model.servers.collectAsStateWithLifecycle()
    val name = (state as? SignInState.Account)?.v1?.name ?: servers.firstOrNull()?.userName.orEmpty()
    // Read as the page scrolls, so they recompose the header alone, and only as they change.
    val bottom = with(LocalDensity.current) { (HEADER_HEIGHT + WindowInsets.statusBars.asPaddingValues().calculateTopPadding()).toPx() }
    val overHero by remember(backdrop, route, bottom) { derivedStateOf { backdrop.overHero(route, bottom) } }
    val scrolled by remember(backdrop, route) { derivedStateOf { backdrop.scrolled(route) } }
    val edge by animateFloatAsState(if (scrolled && !overHero) 1f else 0f, tween(200), label = "edge")
    val ink = if (overHero) Color.White else colors.textPrimary
    Box(modifier.fillMaxWidth()) {
        if (edge > 0f) {
            Box(
                Modifier
                    .matchParentSize()
                    // Reaching below the header, taking none of its room.
                    .layout { measurable, constraints ->
                        val placeable = measurable.measure(Constraints.fixed(constraints.maxWidth, constraints.maxHeight + HEADER_EDGE.roundToPx()))
                        layout(constraints.maxWidth, constraints.maxHeight) { placeable.place(0, 0) }
                    }
                    // Haze reads its style once; the fade is the layer's.
                    .graphicsLayer { alpha = edge }
                    .edge(haze, colors.background),
            )
        }
        Row(
            Modifier
                .fillMaxWidth()
                .statusBarsPadding()
                .height(HEADER_HEIGHT)
                .padding(horizontal = theme.layout.margin),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (back) {
                Box(
                    Modifier
                        .size(36.dp)
                        .clip(CircleShape)
                        .clickable { navigator.back() },
                    Alignment.Center,
                ) {
                    MavioIcon(R.drawable.ic_back, size = 24.dp, tint = ink, contentDescription = stringResource(R.string.action_back))
                }
            } else {
                Wordmark(height = 20.dp)
            }
            Spacer(Modifier.weight(1f))
            Box(
                Modifier
                    .size(36.dp)
                    .clip(CircleShape)
                    .clickable { navigator.open(Route.Search) },
                Alignment.Center,
            ) {
                MavioIcon(R.drawable.ic_search, size = 22.dp, tint = ink, contentDescription = stringResource(R.string.nav_search))
            }
            Spacer(Modifier.width(MavioSpacing.s8))
            val dark = colors.background.luminance() < 0.5f
            Box(
                Modifier
                    .size(36.dp)
                    .shadow(if (dark) 16.dp else 8.dp, CircleShape)
                    .clip(CircleShape)
                    .glass(haze, glassTint(), radius = 12.dp, saturation = 1.8f)
                    .clickable { navigator.open(Route.Settings) },
                Alignment.Center,
            ) {
                Text(name.firstOrNull()?.uppercase().orEmpty(), style = theme.type.headline.text, color = colors.textPrimary)
            }
        }
    }
}

/** How far the header's edge reaches below it. */
private val HEADER_EDGE = 20.dp

/**
 * The header's edge, as the web UI's: what scrolls under it blurred and
 * more saturated, under the [background] from 90 % to transparent, fading
 * out downward.
 */
private fun Modifier.edge(haze: HazeState, background: Color): Modifier = hazeBlur(
    HazeInput.Sources(haze),
    HazeBlurStyle {
        blurEnabled(blurAvailable)
        blurRadius(12.dp)
        noiseFactor(0f)
        mask(Brush.verticalGradient(0.6f to Color.Black, 1f to Color.Transparent))
        colorEffects(
            listOf(
                HazeColorEffect.colorFilter(ColorFilter.colorMatrix(ColorMatrix().apply { setToSaturation(1.8f) })),
                HazeColorEffect.tint(Brush.verticalGradient(0f to background.copy(alpha = 0.9f), 0.5f to background.copy(alpha = 0.7f), 1f to Color.Transparent)),
            ),
        )
        fallbackColorEffect(HazeColorEffect.tint(Brush.verticalGradient(0f to background.copy(alpha = 0.95f), 1f to Color.Transparent)))
    },
)

/** The phone's tabs, as the web UI's: a pill of glass floating over the page. */
@Composable
private fun TabBar(tabs: List<Tab>, current: Tab, haze: HazeState, modifier: Modifier = Modifier, onSelect: (Tab) -> Unit) {
    val colors = theme.colors
    val dark = colors.background.luminance() < 0.5f
    val pill = RoundedCornerShape(50)
    Row(
        modifier
            .navigationBarsPadding()
            .padding(horizontal = theme.layout.margin, vertical = 12.dp)
            .widthIn(max = 448.dp)
            .fillMaxWidth()
            .shadow(if (dark) 16.dp else 8.dp, pill)
            .clip(pill)
            .glass(haze, glassTint(), radius = 12.dp, saturation = 1.8f)
            .border(1.dp, if (dark) Color.White.copy(alpha = 0.12f) else Color.White.copy(alpha = 0.55f), pill)
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        tabs.forEach { t ->
            val selected = t == current
            val tint = if (selected) colors.accentText else colors.textPrimary
            Column(
                Modifier
                    .weight(1f)
                    .height(TAB_BAR_HEIGHT - 8.dp)
                    .clip(pill)
                    .background(if (selected) colors.fillSelected else Color.Transparent)
                    .clickable(remember { MutableInteractionSource() }, null) { onSelect(t) },
                verticalArrangement = Arrangement.spacedBy(2.dp, Alignment.CenterVertically),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                MavioIcon(if (selected) t.filled else t.icon, size = 22.dp, tint = tint)
                Text(
                    stringResource(t.label),
                    style = theme.type.caption.text,
                    fontWeight = if (selected) FontWeight.SemiBold else null,
                    color = tint,
                )
            }
        }
    }
}

/** The tint of the header's and the tab bar's glass. */
@Composable
private fun glassTint(): Color = if (theme.colors.background.luminance() < 0.5f) Color(0x61252528) else Color(0x59FFFFFF)

/**
 * What lies under the phone's header on each screen (by its route): how far
 * the screen has scrolled from its top, as its scrolls tell through
 * [tracking], and where its hero, reaching up under the header, ends.
 */
@Stable
class HeaderBackdrop {
    private val offsets = mutableStateMapOf<Route, Float>()
    private val heroes = mutableStateMapOf<Route, Float>()

    /** Follows what a screen scrolls, upward from its top or back. */
    fun tracking(route: Route): NestedScrollConnection = object : NestedScrollConnection {
        override fun onPostScroll(consumed: Offset, available: Offset, source: NestedScrollSource): Offset {
            if (consumed.y != 0f) offsets[route] = ((offsets[route] ?: 0f) - consumed.y).coerceAtLeast(0f)
            // A list pulled past its top is at its top.
            if (available.y > 0f) offsets[route] = 0f
            return Offset.Zero
        }
    }

    /** Whether the screen has scrolled from its top. */
    fun scrolled(route: Route): Boolean = (offsets[route] ?: 0f) > 0.5f

    /** Whether the screen's hero still shows under a header ending [headerBottom] pixels down. */
    fun overHero(route: Route, headerBottom: Float): Boolean = (heroes[route] ?: 0f) - (offsets[route] ?: 0f) > headerBottom

    internal fun hero(route: Route, bottom: Float?) {
        // Where it ends on the page, as a list stops reporting what it no longer places.
        if (bottom == null) heroes.remove(route) else heroes[route] = bottom + (offsets[route] ?: 0f)
    }
}

/** The screens' [HeaderBackdrop]. */
val LocalHeaderBackdrop = staticCompositionLocalOf { HeaderBackdrop() }

/** The route of the screen composed, which its hero tells the header about. */
val LocalHeaderRoute = staticCompositionLocalOf<Route> { Route.Home }

/**
 * A phone's hero reaching up under the clear header, as iOS's: from the
 * screen's top, [aspectRatio] of its width and taller by the header and the
 * status bar ([LocalTopInset]), telling the header where it ends, which
 * turns the header's icons white while it shows under them.
 */
@Composable
fun PhoneHero(aspectRatio: Float, modifier: Modifier = Modifier, content: @Composable BoxScope.() -> Unit) {
    val top = LocalTopInset.current
    val backdrop = LocalHeaderBackdrop.current
    val route = LocalHeaderRoute.current
    DisposableEffect(backdrop, route) { onDispose { backdrop.hero(route, null) } }
    Box(
        modifier
            .fillMaxWidth()
            .layout { measurable, constraints ->
                val width = constraints.maxWidth
                val height = (width / aspectRatio).roundToInt() + top.roundToPx()
                val placeable = measurable.measure(Constraints.fixed(width, height))
                layout(width, height) { placeable.place(0, 0) }
            }
            .onGloballyPositioned { backdrop.hero(route, it.boundsInWindow().bottom) }
            .clipToBounds(),
        content = content,
    )
}

/**
 * Frosted glass over the pages, as the web UI's backdrop filter: what
 * scrolls under it blurred, more saturated if asked, and tinted; where
 * the device can't blur, the tint nearly opaque.
 */
private fun Modifier.glass(haze: HazeState, tint: Color, radius: Dp, saturation: Float = 1f): Modifier = hazeBlur(
    HazeInput.Sources(haze),
    HazeBlurStyle {
        blurEnabled(blurAvailable)
        blurRadius(radius)
        noiseFactor(0f)
        colorEffects(
            listOfNotNull(
                if (saturation != 1f) {
                    HazeColorEffect.colorFilter(ColorFilter.colorMatrix(ColorMatrix().apply { setToSaturation(saturation) }))
                } else {
                    null
                },
                HazeColorEffect.tint(tint),
            ),
        )
        fallbackColorEffect(HazeColorEffect.tint(tint.copy(alpha = 0.95f)))
    },
)
