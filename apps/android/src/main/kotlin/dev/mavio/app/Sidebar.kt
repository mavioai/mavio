package dev.mavio.app

import android.content.Intent
import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.LibraryEntry
import dev.mavio.core.LibraryType
import dev.mavio.core.ServerRecord
import dev.mavio.core.SignInState
import dev.mavio.core.SwitchableUser
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Avatar
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.Wordmark
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

// The sidebar of the wide layout (docs/clients.md §6.3), as the web UI's:
// the wordmark, Search, Home, Discover while a server discovers, the
// libraries of the servers signed in to, and the user at its foot, opening
// the settings. Collapsed, it is a rail of icons.

/** The icon of a library of a kind. */
@DrawableRes
internal fun libraryIcon(kind: LibraryType) = when (kind) {
    LibraryType.MOVIES -> R.drawable.ic_movies
    LibraryType.SHOWS -> R.drawable.ic_shows
    LibraryType.MUSIC -> R.drawable.ic_music
    LibraryType.MUSIC_VIDEOS -> R.drawable.ic_music
    LibraryType.HOME_VIDEOS -> R.drawable.ic_home_videos
    LibraryType.BOOKS -> R.drawable.ic_books
    LibraryType.PHOTOS -> R.drawable.ic_photos
    LibraryType.COLLECTIONS -> R.drawable.ic_collections
    LibraryType.PLAYLISTS -> R.drawable.ic_playlists
    LibraryType.MIXED -> R.drawable.ic_folder
}

/** A library's place in the sidebar. */
internal fun libraryPlace(library: LibraryEntry) = Route.Library(library.serverId, library.id, library.name)

/**
 * The sidebar's panel, expanded or as a rail; [place] is the place shown,
 * chosen with [onSelect], and [onToggle] expands or collapses it.
 */
@Composable
internal fun SidebarPanel(
    model: AppModel,
    rail: Boolean,
    place: Route,
    discovers: Boolean,
    modifier: Modifier = Modifier,
    onToggle: () -> Unit,
    onSelect: (Route) -> Unit,
) {
    val colors = theme.colors
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val state by model.signInState.collectAsStateWithLifecycle()
    val shown = servers.filter { it.id in signedIn }
    // Each server's libraries, as they come.
    val libraries = remember { mutableStateMapOf<String, List<LibraryEntry>>() }
    shown.forEach { server ->
        LaunchedEffect(server.id, revisions[server.id]) {
            runCatching { model.client.libraries(server.id) }.onSuccess { libraries[server.id] = it.libraries }
        }
    }
    val several = shown.size > 1
    val name = (state as? SignInState.Account)?.v1?.name ?: shown.firstOrNull()?.userName.orEmpty()
    Column(
        modifier
            .fillMaxHeight()
            .background(colors.sidebar)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        Row(
            Modifier.fillMaxWidth().height(56.dp).padding(start = if (rail) 0.dp else 16.dp, end = if (rail) 0.dp else 10.dp),
            horizontalArrangement = if (rail) Arrangement.Center else Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (!rail) Wordmark(height = 22.dp)
            val toggle = stringResource(if (rail) R.string.nav_expand_sidebar else R.string.nav_collapse_sidebar)
            Box(
                Modifier.size(36.dp).clip(RoundedCornerShape(10.dp)).clickable(onClick = onToggle).semantics { contentDescription = toggle },
                Alignment.Center,
            ) {
                MavioIcon(R.drawable.ic_sidebar, size = 18.dp, tint = colors.textSecondary)
            }
        }
        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = if (rail) 8.dp else 10.dp)
                .padding(bottom = MavioSpacing.s8),
            verticalArrangement = Arrangement.spacedBy(if (rail) 8.dp else 2.dp),
            horizontalAlignment = if (rail) Alignment.CenterHorizontally else Alignment.Start,
        ) {
            NavItem(rail, R.drawable.ic_search, R.drawable.ic_search_fill, stringResource(R.string.nav_search), place == Route.Search) { onSelect(Route.Search) }
            NavItem(rail, R.drawable.ic_home, R.drawable.ic_home_fill, stringResource(R.string.nav_home), place == Route.Home) { onSelect(Route.Home) }
            if (discovers) {
                NavItem(rail, R.drawable.ic_discover, R.drawable.ic_discover_fill, stringResource(R.string.nav_discover), place == Route.Discover) {
                    onSelect(Route.Discover)
                }
            }
            Caption(rail, stringResource(R.string.nav_libraries))
            shown.forEach { server ->
                val list = libraries[server.id].orEmpty()
                if (several && list.isNotEmpty() && !rail) {
                    Text(
                        server.name,
                        Modifier.padding(start = 10.dp, top = 8.dp, bottom = 2.dp),
                        style = theme.type.caption.text,
                        color = colors.textTertiary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                list.forEach { library ->
                    val p = libraryPlace(library)
                    NavItem(rail, libraryIcon(library.kind), null, library.name, place == p) { onSelect(p) }
                }
            }
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(colors.border))
        UserMenu(model, rail, name, shown, place == Route.Settings, onSelect)
    }
}

/** A heading of the sidebar's places, or a rule between them on the rail. */
@Composable
private fun Caption(rail: Boolean, text: String) {
    if (rail) {
        Box(Modifier.padding(vertical = 4.dp).width(32.dp).height(1.dp).background(theme.colors.border))
    } else {
        Text(
            text,
            Modifier.padding(start = 10.dp, top = 16.dp, bottom = 4.dp),
            style = theme.type.caption.text,
            fontWeight = FontWeight.SemiBold,
            color = theme.colors.textTertiary,
        )
    }
}

/** A place of the sidebar: its icon, filled while chosen, and on the expanded sidebar its name. */
@Composable
private fun NavItem(
    rail: Boolean,
    @DrawableRes icon: Int,
    @DrawableRes filled: Int?,
    label: String,
    selected: Boolean,
    iconTint: androidx.compose.ui.graphics.Color? = null,
    onClick: () -> Unit,
) {
    val colors = theme.colors
    val shape = RoundedCornerShape(10.dp)
    Row(
        Modifier
            .then(if (rail) Modifier.size(44.dp) else Modifier.fillMaxWidth().heightIn(min = 40.dp))
            .clip(shape)
            .background(if (selected) colors.fillSelected else colors.sidebar)
            .clickable(onClick = onClick)
            .then(if (rail) Modifier.semantics { contentDescription = label } else Modifier.padding(horizontal = 10.dp)),
        horizontalArrangement = if (rail) Arrangement.Center else Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(if (selected && filled != null) filled else icon, size = 20.dp, tint = iconTint ?: colors.textPrimary)
        if (!rail) {
            Text(
                label,
                style = theme.type.callout.text,
                fontWeight = if (selected) FontWeight.SemiBold else null,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.weight(1f))
        }
    }
}

/**
 * The user at the sidebar's foot and the menu it opens, as the web UI's:
 * the user with the appearance beside it, the server's other users to
 * switch to, the settings, the account at mavio.dev or the servers, and
 * signing out.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun UserMenu(model: AppModel, rail: Boolean, name: String, servers: List<ServerRecord>, selected: Boolean, onSelect: (Route) -> Unit) {
    val colors = theme.colors
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    val account = signedInAccount(model)
    val standalone = servers.firstOrNull()?.takeIf { account == null }
    val line = if (account != null) pluralStringResource(R.plurals.menu_server_count, servers.size, servers.size) else standalone?.name.orEmpty()
    var open by remember { mutableStateOf(false) }
    var users by remember { mutableStateOf<List<SwitchableUser>>(emptyList()) }
    var asking by remember { mutableStateOf<SwitchableUser?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(standalone?.id, standalone?.userId, open) {
        if (open) users = standalone?.let { s -> runCatching { model.client.switchableUsers(s.id) }.getOrNull() }.orEmpty()
    }
    fun switchTo(user: SwitchableUser, pin: String?) {
        val server = standalone ?: return
        busy = true
        failure = null
        scope.launch {
            try {
                model.switchUser(server.id, user.id, pin)
                asking = null
                open = false
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failure = e.reason
            }
            busy = false
        }
    }
    val label = stringResource(R.string.nav_user_menu, name)
    Box(Modifier.padding(if (rail) 8.dp else 6.dp)) {
        Row(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(10.dp))
                .background(if (selected || open) colors.fillSelected else colors.sidebar)
                .clickable { open = true }
                .semantics { contentDescription = label }
                .padding(6.dp),
            horizontalArrangement = if (rail) Arrangement.Center else Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Avatar(name, size = if (rail) 28.dp else 32.dp)
            if (!rail) {
                Column(Modifier.weight(1f)) {
                    Text(name, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (line.isNotEmpty()) {
                        Text(line, style = theme.type.caption.text, color = colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                MavioIcon(R.drawable.ic_selector, size = 16.dp, tint = colors.textTertiary)
            }
        }
        DropdownMenu(
            open,
            onDismissRequest = { open = false },
            modifier = Modifier.width(300.dp),
            shape = RoundedCornerShape(14.dp),
            containerColor = colors.surfaceOverlay,
        ) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Avatar(name, size = 36.dp)
                Column(Modifier.weight(1f)) {
                    Text(name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(
                        if (account != null) stringResource(R.string.auth_mavio_account) else stringResource(R.string.auth_standalone_user, line),
                        style = theme.type.footnote.text,
                        color = colors.textSecondary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                ThemeSwitch()
            }
            if (users.size > 1) {
                MenuRule()
                Text(
                    stringResource(R.string.auth_switch_user),
                    Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
                    style = theme.type.caption.text,
                    fontWeight = FontWeight.SemiBold,
                    color = colors.textTertiary,
                )
                users.forEach { u ->
                    DropdownMenuItem(
                        text = { Text(u.name, style = theme.type.callout.text, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                        onClick = {
                            if (u.hasPin) {
                                open = false
                                asking = u
                            } else {
                                switchTo(u, null)
                            }
                        },
                        leadingIcon = { Avatar(u.name, size = 24.dp) },
                        trailingIcon = when {
                            u.current -> ({ MavioIcon(R.drawable.ic_check, size = 16.dp, tint = colors.accentText) })
                            u.hasPin -> ({ MavioIcon(R.drawable.ic_lock, size = 14.dp, tint = colors.textTertiary) })
                            else -> null
                        },
                        enabled = !u.current && !busy,
                    )
                }
                failure?.takeIf { asking == null }?.let {
                    Text(it, Modifier.padding(horizontal = 12.dp, vertical = 4.dp), style = theme.type.footnote.text, color = colors.danger)
                }
            }
            MenuRule()
            MenuEntry(R.drawable.ic_settings, stringResource(R.string.nav_settings)) {
                open = false
                onSelect(Route.Settings)
            }
            val manage = account?.let { manage(it) }
            if (manage != null) {
                MenuEntry(R.drawable.ic_external, stringResource(R.string.menu_manage_account)) {
                    open = false
                    context.startActivity(Intent(Intent.ACTION_VIEW, manage))
                }
            } else if (account == null) {
                MenuEntry(R.drawable.ic_server, stringResource(R.string.menu_manage_servers)) {
                    open = false
                    onSelect(Route.Settings)
                    val page = Route.Setting(SettingsPage.Account)
                    if (navigator.top != page) navigator.open(page)
                }
            }
            MenuRule()
            MenuEntry(R.drawable.ic_sign_out, stringResource(R.string.auth_sign_out)) {
                open = false
                // Signing the account out forgets its servers too.
                scope.launch { runCatching { model.signOut() } }
            }
        }
    }
    asking?.let { user ->
        ModalBottomSheet(
            {
                asking = null
                failure = null
            },
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
            containerColor = colors.background,
        ) {
            PinPad(user.name, busy, failure, { switchTo(user, it) }) {
                asking = null
                failure = null
            }
        }
    }
}

/** An entry of the user menu: its icon and label. */
@Composable
private fun MenuEntry(@DrawableRes icon: Int, label: String, onClick: () -> Unit) {
    DropdownMenuItem(
        text = { Text(label, style = theme.type.callout.text) },
        onClick = onClick,
        leadingIcon = { MavioIcon(icon, size = 18.dp, tint = theme.colors.textSecondary) },
    )
}

/** A rule between the user menu's groups. */
@Composable
private fun MenuRule() {
    Box(Modifier.padding(vertical = 4.dp).fillMaxWidth().height(1.dp).background(theme.colors.border))
}

/** The appearance, chosen beside the user's name: the system's, light or dark. */
@Composable
private fun ThemeSwitch() {
    val prefs = LocalPrefs.current
    val colors = theme.colors
    Row(Modifier.clip(CircleShape).background(colors.fill).padding(2.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
        listOf(
            Triple(Appearance.SYSTEM, R.drawable.ic_appearance, R.string.settings_system),
            Triple(Appearance.LIGHT, R.drawable.ic_light_mode, R.string.settings_light),
            Triple(Appearance.DARK, R.drawable.ic_dark_mode, R.string.settings_dark),
        ).forEach { (value, icon, label) ->
            val on = prefs.appearance == value
            val description = stringResource(label)
            Box(
                Modifier
                    .size(28.dp)
                    .clip(CircleShape)
                    .background(if (on) colors.surfaceRaised else colors.fill)
                    .clickable { prefs.appearance = value }
                    .semantics {
                        contentDescription = description
                        this.selected = on
                    },
                Alignment.Center,
            ) {
                MavioIcon(icon, size = 16.dp, tint = if (on) colors.textPrimary else colors.textSecondary)
            }
        }
    }
}
