package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
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
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation3.runtime.NavEntry
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
}

/** Opens screens on the current tab's stack. */
class Navigator(private val stack: () -> SnapshotStateList<Route>) {
    /** Opens a route over the current screen. */
    fun open(route: Route) {
        stack().add(route)
    }

    /** Opens an item's details. */
    fun open(card: Card) = open(Route.Item(card.serverId, card.itemId))

    /** Plays an item, a version of it when mediaSourceId is set, from start when it is set. */
    fun play(serverId: String, itemId: String, title: String, mediaSourceId: String?, start: java.time.Duration?) =
        open(Route.Player(serverId, itemId, title, mediaSourceId, start))

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

@Composable
private fun Shell(model: AppModel) {
    var tab by rememberSaveable { mutableStateOf(Tab.Home) }
    val stacks = remember { Tab.entries.associateWith { mutableStateListOf(it.root) } }
    val navigator = remember { Navigator { stacks.getValue(tab) } }
    val stack = stacks.getValue(tab)
    val haze = rememberHazeState()
    CompositionLocalProvider(
        LocalNavigator provides navigator,
        // The pages scroll under the header and the floating tab bar, which blur them.
        LocalTopInset provides HEADER_HEIGHT + WindowInsets.statusBars.asPaddingValues().calculateTopPadding(),
        LocalBottomInset provides TAB_BAR_HEIGHT + 24.dp + WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding(),
    ) {
        Box(Modifier.fillMaxSize().background(theme.colors.background)) {
            NavDisplay(
                backStack = stack,
                modifier = Modifier.fillMaxSize().hazeSource(haze),
                onBack = { stack.removeLastOrNull() },
                entryProvider = { route -> NavEntry(route) { Screen(model, it) } },
            )
            // The player takes the whole screen.
            if (stack.lastOrNull() !is Route.Player) {
                Header(model, back = stack.size > 1, Modifier.align(Alignment.TopCenter), haze)
                TabBar(tab, haze, Modifier.align(Alignment.BottomCenter)) {
                    if (it == tab) {
                        // Tapping the current tab goes back to its first screen.
                        while (stack.size > 1) stack.removeAt(stack.lastIndex)
                    }
                    tab = it
                }
            }
        }
    }
}

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
        is Route.Item -> DetailsScreen(model, route.serverId, route.itemId)
        is Route.Player -> PlayerScreen(model, route)
        else -> Box(Modifier.fillMaxSize().padding(top = LocalTopInset.current)) {
            when (route) {
                Route.Libraries -> StatusView(stringResource(R.string.nav_libraries), icon = R.drawable.ic_movies)
                Route.Search -> StatusView(stringResource(R.string.nav_search), icon = R.drawable.ic_search)
                Route.Settings -> SettingsScreen(model)
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
            // Half see-through, so that Home's hero shows under it.
            .glass(haze, colors.background.copy(alpha = 0.5f), radius = 24.dp)
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
private fun TabBar(current: Tab, haze: HazeState, modifier: Modifier = Modifier, onSelect: (Tab) -> Unit) {
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
            .glass(haze, if (dark) Color(0x61252528) else Color(0x59FFFFFF), radius = 12.dp, saturation = 1.8f)
            .border(1.dp, if (dark) Color.White.copy(alpha = 0.12f) else Color.White.copy(alpha = 0.55f), pill)
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Tab.entries.forEach { t ->
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
