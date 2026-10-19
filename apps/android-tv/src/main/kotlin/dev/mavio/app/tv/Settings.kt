package dev.mavio.app.tv

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.AccountCard
import dev.mavio.ui.Avatar
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.EntryRow
import dev.mavio.ui.Group
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NameRow
import dev.mavio.ui.PlaybackSettings
import dev.mavio.ui.Pressable
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsList
import dev.mavio.ui.SubtitlesAudioSettings
import dev.mavio.ui.ValueRow
import dev.mavio.ui.signedInAccount
import dev.mavio.ui.signedInServers
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch

/** A section of the settings, shown in the list's place. */
enum class SettingsSection {
    Account,
    Playback,
    SubtitlesAudio,
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
    }
}

/**
 * The list: the user's card, opening the account; the users to switch to
 * on a standalone server; this device's sections; the versions; and signing
 * out. It opens with focus on its first element, or the entry a section
 * was opened from.
 */
@Composable
private fun SettingsListPage(model: AppModel, page: Page.Settings) {
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val switching = rememberUserSwitching(model)
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
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
        item {
            Group {
                val app = remember { context.packageManager.getPackageInfo(context.packageName, 0).versionName.orEmpty() }
                ValueRow("Mavio", app)
                RowDivider()
                ValueRow("Core", dev.mavio.core.version)
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
 * The account: the Mavio account, or the user of the standalone server,
 * and the servers signed in to.
 */
@Composable
private fun AccountSection(model: AppModel) {
    val account = signedInAccount(model)
    val servers = signedInServers(model)
    val server = servers.firstOrNull()
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
                    ServerRow(s)
                }
            }
        }
    }
}

/** A server signed in to: its name and its user. */
@Composable
private fun ServerRow(server: ServerRecord) {
    // Focus reaches each server, so that the section has an element to hold it.
    Pressable({}, Modifier.fillMaxWidth()) {
        ValueRow(server.name, server.userName)
    }
}
