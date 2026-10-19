package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.ContentTransform
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.runtime.mutableStateMapOf
import dev.mavio.design.TabletLayout
import dev.mavio.ui.LocalTheme
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.key
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.TransformOrigin
import kotlin.math.roundToInt
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.graphics.luminance
import dev.mavio.ui.Artwork
import androidx.compose.foundation.layout.requiredHeight
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.draw.BlurredEdgeTreatment
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.layout.layout
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.SignInState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.blurAvailable
import dev.mavio.ui.StatusView
import dev.mavio.ui.Wordmark
import dev.mavio.ui.text
import dev.mavio.ui.theme

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

    /** A section of a server's settings. */
    data class ServerSetting(val serverId: String, val section: ServerSection) : Route

    /** A menu of a server's settings. */
    data class ServerMenuPage(val serverId: String, val menu: ServerMenu) : Route

    /** An issue reported on a server. */
    data class IssuePage(val serverId: String, val id: String) : Route

    /** An item's metadata edited. */
    data class EditMetadata(val serverId: String, val itemId: String) : Route

    /** An item identified from the providers' search results. */
    data class Identify(val serverId: String, val itemId: String) : Route

    /** An item's images chosen. */
    data class ItemImages(val serverId: String, val itemId: String) : Route

    /** A library of a server's settings, or a new one's when id is null. */
    data class ManagedLibrary(val serverId: String, val id: String?) : Route

    /** A user of a server, or a new one's page when id is null; standalone when the server keeps its own accounts. */
    data class ManagedUser(val serverId: String, val id: String?, val standalone: Boolean) : Route

    /** Adding an rclone remote to a server. */
    data class NewRemote(val serverId: String) : Route

    /** An rclone remote of a server. */
    data class StorageRemote(val serverId: String, val name: String) : Route

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
    val haze = rememberHazeState()
    // Each place keeps its screens' state while another shows.
    val saved = rememberSaveableStateHolder()
    val bars = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    val statusBar = WindowInsets.statusBars.asPaddingValues().calculateTopPadding()
    // The player and the sign-in take the whole screen.
    val chrome = stack.lastOrNull().let { it !is Route.Player && it != Route.AddServer }
    FollowGroup(model, navigator)
    BoxWithConstraints(Modifier.fillMaxSize().background(theme.colors.background)) {
        val wide = maxWidth >= 600.dp
        // The wide layout's settings have a sidebar of their own and show a section beside it.
        val settings = wide && place == Route.Settings
        LaunchedEffect(settings, stack.size) { if (settings && stack.size == 1) stack.add(FIRST_SETTING) }
        // Above the section the sidebar chose, Back leads to the page under it.
        val backShown = chrome && stack.size > if (settings) 2 else 1
        val content = @Composable { modifier: Modifier, rootInset: Dp ->
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
                                CompositionLocalProvider(LocalBottomInset provides if (route == stack.firstOrNull()) rootInset else bars) {
                                    Screen(model, it)
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
                // The pages scroll under the header and the floating tab bar, which blur them.
                LocalTopInset provides HEADER_HEIGHT + statusBar,
            ) {
                // The tab bar shows on the tabs' first screens alone.
                val tabBar = stack.size == 1
                content(Modifier.fillMaxSize(), TAB_BAR_HEIGHT + 24.dp + bars)
                if (chrome) {
                    Header(model, back = stack.size > 1, Modifier.align(Alignment.TopCenter), haze)
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
                is Route.ServerSetting -> ServerSettingScreen(model, route)
                is Route.ServerMenuPage -> ServerMenuScreen(route)
                is Route.IssuePage -> IssueScreen(model, route)
                is Route.EditMetadata -> EditMetadataScreen(model, route)
                is Route.Identify -> IdentifyScreen(model, route)
                is Route.ItemImages -> ItemImagesScreen(model, route)
                is Route.ManagedLibrary -> ManagedLibraryScreen(model, route)
                is Route.ManagedUser -> ManagedUserScreen(model, route)
                is Route.NewRemote -> NewRemoteScreen(model, route)
                is Route.StorageRemote -> StorageRemoteScreen(model, route)
                else -> StatusView(stringResource(R.string.state_loading))
            }
        }
    }
}

/**
 * The phone's top, as the web UI's: the wordmark, or Back on a page opened
 * from another, then search and the user's avatar, which opens the
 * settings.
 */
@Composable
private fun Header(model: AppModel, back: Boolean, modifier: Modifier, haze: HazeState) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    val state by model.signInState.collectAsStateWithLifecycle()
    val servers by model.servers.collectAsStateWithLifecycle()
    val name = (state as? SignInState.Account)?.v1?.name ?: servers.firstOrNull()?.userName.orEmpty()
    Row(
        modifier
            .fillMaxWidth()
            // The tab bar's glass, without its border.
            .glass(haze, glassTint(), radius = 12.dp, saturation = 1.8f)
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
                MavioIcon(R.drawable.ic_back, size = 24.dp, contentDescription = stringResource(R.string.action_back))
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
            MavioIcon(R.drawable.ic_search, size = 22.dp, contentDescription = stringResource(R.string.nav_search))
        }
        Spacer(Modifier.width(MavioSpacing.s8))
        Avatar(name, Modifier.clip(CircleShape).clickable { navigator.open(Route.Settings) })
    }
}

/** The first letter of a name in a circle. */
@Composable
fun Avatar(name: String, modifier: Modifier = Modifier, size: Dp = 32.dp) {
    Box(
        modifier
            .size(size)
            .background(theme.colors.fillSelected, CircleShape),
        Alignment.Center,
    ) {
        Text(
            name.take(1).uppercase(),
            fontSize = (size.value * 0.42f).sp,
            fontWeight = FontWeight.SemiBold,
            color = theme.colors.textSecondary,
        )
    }
}

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
 * What lies behind the header above a phone's hero: the hero's artwork, as
 * the hero crops it ([aspectRatio] of the screen's width, drawn by [art]),
 * its top rows stretched up to the top of the screen and softened, so that
 * the artwork's top seems to go on under the header's glass, meeting it at
 * a sharp edge. Placed first in the hero's box, which must not clip it.
 */
@Composable
fun HeroBleed(aspectRatio: Float, art: @Composable () -> Unit) {
    val top = LocalTopInset.current
    Box(
        Modifier
            .fillMaxWidth()
            .layout { measurable, constraints ->
                val height = top.roundToPx()
                val placeable = measurable.measure(constraints.copy(minHeight = height, maxHeight = height))
                // Above the hero, taking none of its room.
                layout(constraints.minWidth, 0) { placeable.place(0, -height) }
            }
            .clipToBounds(),
    ) {
        Box(
            Modifier
                // As tall as the hero, overflowing the strip from its top.
                .layout { measurable, constraints ->
                    val width = constraints.maxWidth
                    val placeable = measurable.measure(Constraints.fixed(width, (width / aspectRatio).roundToInt()))
                    layout(width, constraints.maxHeight) { placeable.place(0, 0) }
                }
                // The band at the artwork's top, stretched over the strip.
                .graphicsLayer {
                    transformOrigin = TransformOrigin(0.5f, 0f)
                    scaleY = top.toPx() / BLEED_BAND.toPx()
                }
                .then(if (blurAvailable) Modifier.blur(BLEED_BLUR, BlurredEdgeTreatment.Rectangle) else Modifier),
        ) { art() }
    }
}

/** How much of the hero's top [HeroBleed] stretches behind the header. */
private val BLEED_BAND = 6.dp

/** How far [HeroBleed] softens the band, so that its pixels stretched up don't streak. */
private val BLEED_BLUR = 24.dp

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
