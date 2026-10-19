package dev.mavio.app.tv

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.SwitchableUser
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Avatar
import dev.mavio.ui.Focusable
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.OnCanvas
import dev.mavio.ui.R
import dev.mavio.ui.focused
import dev.mavio.ui.movesFocus
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.signedInAccount
import dev.mavio.ui.signedInServers
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/**
 * Switching the standalone server's user, as the user menu and the
 * settings' members do: its other users, loaded by [load], the one asking
 * for its PIN, and what went wrong.
 */
@Stable
class UserSwitching(private val model: AppModel, private val scope: CoroutineScope) {
    /** The standalone server signed in to, whose users switch; null with a Mavio account. */
    var server by mutableStateOf<ServerRecord?>(null)

    /** The server's users, the current one among them. */
    var users by mutableStateOf<List<SwitchableUser>>(emptyList())
        private set

    /** The user whose PIN the pad asks for. */
    var asking by mutableStateOf<SwitchableUser?>(null)

    /** Whether a switch is under way. */
    var busy by mutableStateOf(false)
        private set

    /** Why the last switch failed. */
    var failure by mutableStateOf<String?>(null)

    /** Loads the server's users. */
    suspend fun load() {
        users = server?.let { s -> runCatching { model.client.switchableUsers(s.id) }.getOrNull() }.orEmpty()
    }

    /** Switches to [user], asking for its PIN first when it has one; [done] runs once switched. */
    fun choose(user: SwitchableUser, done: () -> Unit) {
        if (user.hasPin) asking = user else switchTo(user, null, done)
    }

    /** Switches to [user] with [pin]; [done] runs once switched. */
    fun switchTo(user: SwitchableUser, pin: String?, done: () -> Unit) {
        val s = server ?: return
        busy = true
        failure = null
        scope.launch {
            try {
                model.switchUser(s.id, user.id, pin)
                asking = null
                done()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failure = e.message ?: e.toString()
            }
            busy = false
        }
    }
}

/** [UserSwitching] for the standalone server signed in to, followed. */
@Composable
fun rememberUserSwitching(model: AppModel): UserSwitching {
    val scope = rememberCoroutineScope()
    val switching = remember(model) { UserSwitching(model, scope) }
    val servers = signedInServers(model)
    switching.server = servers.firstOrNull()?.takeIf { signedInAccount(model) == null }
    return switching
}

/** The PIN pad of [switching]'s user asking for one; [done] runs once switched or cancelled. */
@Composable
fun SwitchingPin(switching: UserSwitching, done: () -> Unit) {
    switching.asking?.let { user ->
        TvPinPad(user.name, switching.busy, switching.failure, { switching.switchTo(user, it, done) }) {
            switching.asking = null
            switching.failure = null
            done()
        }
    }
}

/**
 * The menu button at the navigation bar's right and the user menu it opens
 * (docs/tv.md §3, §5): three dots, bare, and in the tabs' glass with focus
 * or while its menu is open, white with focus. The menu is headed by the
 * user's avatar and name, then the server's other users to switch to, the
 * settings, opened through [onSettings], and signing out; it opens with
 * focus on Settings, and back closes it, returning focus to the button,
 * [button].
 */
@Composable
fun MenuButton(model: AppModel, button: FocusRequester, modifier: Modifier = Modifier, onSettings: () -> Unit) {
    val account = signedInAccount(model)
    val shown = signedInServers(model)
    val name = account?.name ?: shown.firstOrNull()?.userName.orEmpty()
    val switching = rememberUserSwitching(model)
    val scope = rememberCoroutineScope()
    var open by remember { mutableStateOf(false) }
    LaunchedEffect(switching.server?.id, switching.server?.userId, open) {
        if (open) switching.load()
    }
    fun close() {
        open = false
        runCatching { button.requestFocus() }
    }
    val focus = remember { Focusable() }
    val focused = focus.focused()
    val label = stringResource(R.string.nav_user_menu, name)
    Box(modifier) {
        Box(
            Modifier
                .shakes()
                .focusRequester(button)
                .size(BUTTON)
                .clip(CircleShape)
                .background(
                    when {
                        // While its menu is open, focus in the menu, it is as without focus.
                        open -> theme.colors.surfaceRaised.copy(alpha = 0.6f)
                        focused -> Color.White
                        else -> Color.Transparent
                    },
                )
                .selectable(focus) { if (open) close() else open = true }
                .semantics { contentDescription = label },
            Alignment.Center,
        ) {
            MavioIcon(R.drawable.ic_more_vertical, size = 32.dp, tint = if (focused && !open) Color.Black else theme.colors.textSecondary)
        }
        if (open) {
            val line = if (account != null) {
                stringResource(R.string.auth_mavio_account)
            } else {
                stringResource(R.string.auth_standalone_user, switching.server?.name.orEmpty())
            }
            UserMenu(
                name,
                line,
                switching.users,
                switching.busy,
                switching.failure.takeIf { switching.asking == null },
                onDismiss = ::close,
                onUser = { u ->
                    if (u.hasPin) open = false
                    switching.choose(u, ::close)
                },
                onSettings = {
                    open = false
                    onSettings()
                },
            ) {
                open = false
                // Signing the account out forgets its servers too.
                scope.launch { runCatching { model.signOut() } }
            }
        }
    }
    SwitchingPin(switching) { runCatching { button.requestFocus() } }
}

/** The menu button's size, the height of the tabs' pill. */
private val BUTTON = 64.dp

/** The user menu below the button, at the screen's right, opening with focus on Settings. */
@Composable
private fun UserMenu(
    name: String,
    line: String,
    users: List<SwitchableUser>,
    busy: Boolean,
    failure: String?,
    onDismiss: () -> Unit,
    onUser: (SwitchableUser) -> Unit,
    onSettings: () -> Unit,
    onSignOut: () -> Unit,
) {
    val density = LocalDensity.current
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        withFrameNanos {}
        runCatching { first.requestFocus() }
    }
    Popup(
        alignment = Alignment.TopEnd,
        offset = with(density) { IntOffset(0, (BUTTON + MavioSpacing.s8).roundToPx()) },
        onDismissRequest = onDismiss,
        properties = PopupProperties(focusable = true),
    ) {
        OnCanvas(density) {
            Column(
                Modifier
                    .width(400.dp)
                    .shadow(24.dp, RoundedCornerShape(MavioRadius.lg), ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow)
                    .clip(RoundedCornerShape(MavioRadius.lg))
                    // Without a blur behind it, the glass is all but opaque.
                    .background(theme.colors.surfaceOverlay.copy(alpha = 0.95f))
                    .border(1.dp, theme.colors.border, RoundedCornerShape(MavioRadius.lg))
                    .padding(MavioSpacing.s8),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s12),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Avatar(name, size = 56.dp)
                    Column(Modifier.weight(1f)) {
                        Text(name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(line, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                if (users.size > 1) {
                    MenuRule()
                    Text(
                        stringResource(R.string.auth_switch_user),
                        Modifier.padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s4),
                        style = theme.type.footnote.text,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.colors.textTertiary,
                    )
                    users.forEach { u ->
                        MenuEntry(
                            u.name,
                            Modifier,
                            leading = { Avatar(u.name, size = 36.dp) },
                            trailing = when {
                                u.current -> R.drawable.ic_check
                                u.hasPin -> R.drawable.ic_lock
                                else -> null
                            },
                            enabled = !u.current && !busy,
                        ) { onUser(u) }
                    }
                    failure?.let {
                        Text(it, Modifier.padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s4), style = theme.type.footnote.text, color = theme.colors.danger)
                    }
                }
                MenuRule()
                MenuEntry(
                    stringResource(R.string.nav_settings),
                    Modifier.focusRequester(first),
                    leading = { MavioIcon(R.drawable.ic_settings, size = 28.dp, tint = theme.colors.textSecondary) },
                    onClick = onSettings,
                )
                MenuRule()
                MenuEntry(
                    stringResource(R.string.auth_sign_out),
                    Modifier,
                    leading = { MavioIcon(R.drawable.ic_sign_out, size = 28.dp, tint = theme.colors.textSecondary) },
                    onClick = onSignOut,
                )
            }
        }
    }
}

/** An entry of the user menu, filled with `color.fill` with focus. */
@Composable
private fun MenuEntry(
    label: String,
    modifier: Modifier,
    leading: @Composable () -> Unit,
    @DrawableRes trailing: Int? = null,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Row(
        modifier
            .shakes()
            .fillMaxWidth()
            .clip(RoundedCornerShape(MavioRadius.md))
            .background(if (focused) theme.colors.fill else Color.Transparent)
            .selectable(state) { if (enabled) onClick() }
            .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s12),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        leading()
        Text(label, Modifier.weight(1f), style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        trailing?.let { MavioIcon(it, size = 24.dp, tint = if (it == R.drawable.ic_check) theme.colors.accentText else theme.colors.textTertiary) }
    }
}

/** A rule between the user menu's groups. */
@Composable
private fun MenuRule() {
    Box(Modifier.padding(vertical = MavioSpacing.s4).fillMaxWidth().height(1.dp).background(theme.colors.border))
}

/**
 * Asks for a user's four-digit PIN on a keypad over the page, as the web
 * UI's on a television: the fourth digit sends it and a wrong one clears
 * the dots; back deletes a digit, and closes the pad once there is none
 * (docs/tv.md §4).
 */
@Composable
fun TvPinPad(name: String, busy: Boolean, error: String?, enter: (String) -> Unit, cancel: () -> Unit) {
    var pin by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    LaunchedEffect(error) { if (error != null) pin = "" }
    LaunchedEffect(Unit) {
        withFrameNanos {}
        runCatching { first.requestFocus() }
    }
    val canvas = LocalDensity.current
    fun press(k: String) {
        when (k) {
            "⌫" -> pin = pin.dropLast(1)
            else -> if (pin.length < 4 && !busy) {
                pin += k
                if (pin.length == 4) enter(pin)
            }
        }
    }
    Dialog(
        onDismissRequest = { if (pin.isEmpty()) cancel() else pin = pin.dropLast(1) },
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        OnCanvas(canvas) {
            Column(
                Modifier
                    .movesFocus()
                    .clip(RoundedCornerShape(MavioRadius.xl))
                    .background(theme.colors.surfaceOverlay)
                    .padding(MavioSpacing.s40),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s20),
            ) {
                Avatar(name, size = 80.dp)
                Text(stringResource(R.string.auth_enter_pin, name), style = theme.type.title3.text)
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s20)) {
                    repeat(4) { i ->
                        Box(
                            Modifier
                                .size(18.dp)
                                .border(2.dp, theme.colors.textPrimary, CircleShape)
                                .background(if (i < pin.length) theme.colors.textPrimary else Color.Transparent, CircleShape),
                        )
                    }
                }
                Text(error ?: " ", style = theme.type.footnote.text, color = theme.colors.danger)
                Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"), listOf("0", "⌫")).forEach { row ->
                        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                            row.forEach { k ->
                                val state = remember { Focusable() }
                                val focused = state.focused()
                                Box(
                                    (if (k == "1") Modifier.focusRequester(first) else Modifier).shakes()
                                        .size(width = if (k == "0") 192.dp else 90.dp, height = 90.dp)
                                        .clip(RoundedCornerShape(MavioRadius.lg))
                                        .background(if (focused) Color.White else theme.colors.fill)
                                        .selectable(state) { press(k) },
                                    Alignment.Center,
                                ) {
                                    val tint = if (focused) Color.Black else theme.colors.textPrimary
                                    if (k == "⌫") {
                                        MavioIcon(R.drawable.ic_backspace, size = 32.dp, tint = tint, contentDescription = stringResource(R.string.search_backspace))
                                    } else {
                                        Text(k, fontSize = 36.sp, color = tint)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
