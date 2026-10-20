package dev.mavio.app.tv

import androidx.activity.compose.BackHandler
import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.admin.serverGroup
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.StreamState
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.AboutSettings
import dev.mavio.ui.AccountCard
import dev.mavio.ui.Avatar
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.EntryRow
import dev.mavio.ui.Group
import dev.mavio.ui.LinkRow
import dev.mavio.ui.LocalTheme
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NameRow
import dev.mavio.ui.PlaybackSettings
import dev.mavio.ui.Pressable
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.SubtitlesAudioSettings
import dev.mavio.ui.TvMenu
import dev.mavio.ui.TvMenuItem
import dev.mavio.ui.signedInAccount
import dev.mavio.ui.signedInServers
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/** A section of the settings, shown in the list's place. */
enum class SettingsSection {
    Account,
    Playback,
    SubtitlesAudio,
    About,
}

/**
 * The settings (docs/tv.md §6.6), as the web UI's on a television: the
 * list of their sections, the phone's settings in a centered column, and
 * a section in its place when one is selected. Back returns from a section
 * to the list, with focus on the entry that opened it.
 */
@Composable
fun SettingsPage(model: AppModel, page: Page.Settings) {
    val section = page.section
    BackHandler(section != null) { page.section = null }
    when (section) {
        null -> SettingsListPage(model, page)
        SettingsSection.Account -> AccountSection(model)
        SettingsSection.Playback -> PlaybackSettings()
        SettingsSection.SubtitlesAudio -> SubtitlesAudioSettings(model)
        SettingsSection.About -> AboutSettings(model)
    }
}

/**
 * The list: the user's card, opening the account; the users to switch to
 * on a standalone server; this device's sections; About, with the
 * versions; and signing out. It opens with focus on its first element, or the entry a section
 * was opened from.
 */
@Composable
private fun SettingsListPage(model: AppModel, page: Page.Settings) {
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val switching = rememberUserSwitching(model)
    val servers = signedInServers(model)
    val scope = rememberCoroutineScope()
    val focus = page.focus
    val first = focus.of("account")
    LaunchedEffect(switching.server?.id, switching.server?.userId) { switching.load() }
    LaunchedEffect(Unit) { focus.restore(first) }
    fun open(section: SettingsSection) {
        page.section = section
    }
    SettingsList(stringResource(R.string.nav_settings), large = true) {
        item {
            Group {
                Pressable({ open(SettingsSection.Account) }, Modifier.focusKey(focus, "account")) {
                    AccountCard(model, Modifier)
                }
            }
        }
        if (switching.users.size > 1) {
            item {
                Group(stringResource(R.string.auth_switch_user), switching.failure?.takeIf { switching.asking == null }) {
                    Row(
                        Modifier.fillMaxWidth().padding(MavioSpacing.s16),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                    ) {
                        switching.users.forEach { user ->
                            Member(user.name, user.current, user.hasPin, Modifier.focusKey(focus, "user/${user.id}")) {
                                if (!user.current && !switching.busy) switching.choose(user) {}
                            }
                        }
                    }
                }
            }
        }
        item {
            Group(stringResource(R.string.settings_this_device)) {
                EntryRow(
                    stringResource(R.string.settings_playback),
                    R.drawable.ic_play_fill,
                    Color(0xFFFF3B30),
                    Modifier.focusKey(focus, "playback"),
                ) { open(SettingsSection.Playback) }
                if (signedIn.isNotEmpty()) {
                    RowDivider(tiled = true)
                    EntryRow(
                        stringResource(R.string.settings_subtitles_audio),
                        R.drawable.ic_subtitles_fill,
                        Color(0xFF34C759),
                        Modifier.focusKey(focus, "subtitles"),
                    ) { open(SettingsSection.SubtitlesAudio) }
                }
            }
        }
        serverGroup(model, servers, page.admins) { Modifier.focusKey(focus, it) }
        item {
            Group {
                EntryRow(
                    stringResource(R.string.settings_about),
                    R.drawable.ic_info_fill,
                    Color(0xFF8E8E93),
                    Modifier.focusKey(focus, "about"),
                ) { open(SettingsSection.About) }
            }
        }
        item {
            // Signing the account out forgets its servers too.
            SettingsButton(
                stringResource(R.string.auth_sign_out),
                { scope.launch { runCatching { model.signOut() } } },
                Modifier.fillMaxWidth().focusKey(focus, "signOut"),
                kind = ButtonKind.Secondary,
                icon = R.drawable.ic_sign_out,
            )
        }
    }
    SwitchingPin(switching) { scope.launch { focus.restore(first) } }
}

/** A user to switch to: the avatar over the name, checked when current, locked with a PIN. */
@Composable
private fun Member(name: String, current: Boolean, locked: Boolean, modifier: Modifier, onClick: () -> Unit) {
    Pressable(onClick, modifier.width(140.dp).clip(RoundedCornerShape(MavioRadius.lg))) {
        Column(
            Modifier.fillMaxWidth().padding(MavioSpacing.s12),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Avatar(name, size = 72.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    name,
                    Modifier.weight(1f, fill = false),
                    style = theme.type.callout.text,
                    textAlign = TextAlign.Center,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                when {
                    current -> MavioIcon(R.drawable.ic_check, size = 18.dp, tint = theme.colors.accentText)
                    locked -> MavioIcon(R.drawable.ic_lock, size = 18.dp, tint = theme.colors.textTertiary)
                }
            }
        }
    }
}

/**
 * The account: the Mavio account, or the user of the standalone server;
 * the servers in the user's order, which orders Home, each with a menu,
 * and adding one beside a standalone server's; and authorizing a device
 * with Quick Connect.
 */
@Composable
private fun AccountSection(model: AppModel) {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    val account = signedInAccount(model)
    val server = signedInServers(model).firstOrNull()
    SettingsList(stringResource(R.string.settings_account)) {
        if (account != null) {
            item {
                Group(footer = stringResource(R.string.settings_mavio_account_message)) { NameRow(account.name) }
            }
        } else if (server != null) {
            item {
                Group(footer = stringResource(R.string.settings_standalone_account_message, server.userName, server.name)) {
                    NameRow(server.userName)
                }
            }
        }
        item {
            Group(
                stringResource(R.string.auth_servers),
                stringResource(if (account != null) R.string.settings_servers_mavio_message else R.string.settings_servers_message),
            ) {
                servers.forEachIndexed { i, s ->
                    if (i > 0) RowDivider()
                    ServerRow(model, s, i, servers, s.id in signedIn, mavio = account != null)
                }
                // A Mavio account's servers are those it lists, which syncing
                // keeps, so a server added beside them would vanish.
                if (account == null) {
                    if (servers.isNotEmpty()) RowDivider()
                    LinkRow(stringResource(R.string.auth_add_server), R.drawable.ic_add) { pages.openAddServer() }
                }
            }
        }
        if (signedIn.isNotEmpty()) item { AuthorizeDevice(model, servers.filter { it.id in signedIn }) }
    }
}

/**
 * A server with the user signed in to it and the state of its connection;
 * its menu, opening below it, moves it in the order, signs in to it again
 * and signs out of it.
 */
@Composable
private fun ServerRow(model: AppModel, server: ServerRecord, index: Int, servers: List<ServerRecord>, signedIn: Boolean, mavio: Boolean) {
    val states by model.states.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    val scope = rememberCoroutineScope()
    var open by remember { mutableStateOf(false) }
    var height by remember { mutableIntStateOf(0) }
    val row = remember { FocusRequester() }
    val first = remember { FocusRequester() }
    val colors = theme.colors
    // The menu keeps the page's colors, whatever the focused row's.
    val page = theme
    fun close() {
        open = false
        runCatching { row.requestFocus() }
    }
    fun move(by: Int) {
        val ids = servers.map { it.id }.toMutableList()
        ids[index] = ids[index + by].also { ids[index + by] = ids[index] }
        model.order(ids)
    }
    val entries = buildList {
        if (index > 0) add(MenuEntry(R.string.auth_move_up, R.drawable.ic_back) { move(-1) })
        if (index < servers.lastIndex) add(MenuEntry(R.string.auth_move_down, R.drawable.ic_forward) { move(1) })
        if (!signedIn && !mavio) add(MenuEntry(R.string.auth_sign_in, R.drawable.ic_profile) { pages.openAddServer() })
        add(MenuEntry(R.string.auth_sign_out, R.drawable.ic_sign_out, danger = true) { scope.launch { runCatching { model.remove(server.id) } } })
    }
    Box(Modifier.fillMaxWidth().onSizeChanged { height = it.height }) {
        Pressable({ open = true }, Modifier.fillMaxWidth().focusRequester(row)) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = MavioSpacing.s24, vertical = MavioSpacing.s16),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    Text(server.name, style = theme.type.headline.text, maxLines = 1)
                    Text(
                        if (signedIn) stringResource(R.string.auth_signed_in_as, server.userName) else stringResource(R.string.auth_signed_out),
                        style = theme.type.callout.text,
                        color = theme.colors.textSecondary,
                        maxLines = 1,
                    )
                    Text(server.address, style = theme.type.footnote.text, color = theme.colors.textTertiary, maxLines = 1)
                }
                when (states[server.id]) {
                    is StreamState.Online -> Box(Modifier.size(12.dp).background(colors.success, CircleShape))
                    is StreamState.Offline -> MavioIcon(R.drawable.ic_offline, size = 24.dp, tint = theme.colors.textTertiary)
                    else -> {}
                }
                MavioIcon(R.drawable.ic_more, size = 28.dp, tint = theme.colors.textSecondary)
            }
        }
        if (open) {
            CompositionLocalProvider(LocalTheme provides page) {
                TvMenu(first, 400.dp, Alignment.TopEnd, with(LocalDensity.current) { height.toDp() } + MavioSpacing.s8, ::close) {
                    entries.forEachIndexed { i, e ->
                        val color = if (e.danger) page.colors.danger else page.colors.textPrimary
                        TvMenuItem(
                            {
                                close()
                                e.action()
                            },
                            if (i == 0) Modifier.focusRequester(first) else Modifier,
                        ) {
                            MavioIcon(e.icon, size = 24.dp, tint = color)
                            Text(stringResource(e.label), style = theme.type.headline.text, color = color, maxLines = 1)
                        }
                    }
                }
            }
        }
    }
}

/** An item of a menu: its label and icon, red when it cannot be undone, and what it does. */
private class MenuEntry(@StringRes val label: Int, @DrawableRes val icon: Int, val danger: Boolean = false, val action: () -> Unit)

/** Approves the Quick Connect code another device shows, on the server chosen when there are several. */
@Composable
private fun AuthorizeDevice(model: AppModel, servers: List<ServerRecord>) {
    var serverId by remember { mutableStateOf(servers.first().id) }
    var code by remember { mutableStateOf("") }
    var message by remember { mutableStateOf<String?>(null) }
    var done by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val authorized = stringResource(R.string.auth_authorized)
    Group(stringResource(R.string.auth_authorize_device)) {
        if (servers.size > 1) {
            ChoiceRow(stringResource(R.string.auth_server), servers.map { it.id to it.name }, serverId) { serverId = it }
            RowDivider()
        }
        Row(
            Modifier.padding(MavioSpacing.s24),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.Bottom,
        ) {
            SettingsField(
                code,
                { new -> code = new.filter(Char::isDigit).take(6) },
                stringResource(R.string.auth_quick_connect_title),
                Modifier.weight(1f),
                placeholder = "000000",
                keyboard = KeyboardType.Number,
            )
            SettingsButton(
                stringResource(R.string.auth_authorize),
                {
                    busy = true
                    scope.launch {
                        try {
                            model.client.authorizeQuickConnect(serverId, code)
                            done = true
                            message = authorized
                            code = ""
                        } catch (e: CancellationException) {
                            throw e
                        } catch (e: Exception) {
                            done = false
                            message = e.message ?: e.toString()
                        } finally {
                            busy = false
                        }
                    }
                },
                enabled = code.length == 6 && !busy,
            )
        }
        message?.let {
            Text(
                it,
                Modifier.padding(start = MavioSpacing.s24, end = MavioSpacing.s24, bottom = MavioSpacing.s24),
                style = theme.type.callout.text,
                color = if (done) theme.colors.textSecondary else theme.colors.danger,
            )
        }
    }
}
