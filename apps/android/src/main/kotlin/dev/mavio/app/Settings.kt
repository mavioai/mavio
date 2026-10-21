package dev.mavio.app

import android.app.LocaleManager
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.annotation.DrawableRes
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.core.net.toUri
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.admin.serverGroup
import dev.mavio.core.Account
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.StreamState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.AboutSettings
import dev.mavio.ui.AccountCard
import dev.mavio.ui.Appearance
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.EntryRow
import dev.mavio.ui.Group
import dev.mavio.ui.LinkRow
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NameRow
import dev.mavio.ui.PlaybackSettings
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.SubtitlesAudioSettings
import dev.mavio.ui.AssistantSettings
import dev.mavio.ui.signedInAccount
import dev.mavio.ui.signedInServers
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/** A page the settings open. */
enum class SettingsPage {
    Account,
    Appearance,
    Playback,
    SubtitlesAudio,
    Assistant,
    About,
}

/**
 * The settings, as the web UI's on a phone: the account's card, opening
 * the account with its servers; this device's sections; About, with the
 * versions; and signing out.
 */
@Composable
fun SettingsScreen(model: AppModel) {
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val servers = signedInServers(model)
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    val standalone = servers.firstOrNull()?.takeIf { signedInAccount(model) == null }
    SettingsList(stringResource(R.string.nav_settings), large = true) {
        item {
            Group {
                AccountCard(model, Modifier.clickable { navigator.open(Route.Setting(SettingsPage.Account)) })
            }
        }
        standalone?.let { server -> item { SwitchUserGroup(model, server) } }
        item {
            Group(stringResource(R.string.settings_this_device)) {
                EntryRow(stringResource(R.string.settings_appearance), R.drawable.ic_appearance_fill, Color(0xFF5E5CE6)) {
                    navigator.open(Route.Setting(SettingsPage.Appearance))
                }
                RowDivider(tiled = true)
                EntryRow(stringResource(R.string.settings_playback), R.drawable.ic_play_fill, Color(0xFFFF3B30)) {
                    navigator.open(Route.Setting(SettingsPage.Playback))
                }
                if (signedIn.isNotEmpty()) {
                    RowDivider(tiled = true)
                    EntryRow(stringResource(R.string.settings_subtitles_audio), R.drawable.ic_subtitles_fill, Color(0xFF34C759)) {
                        navigator.open(Route.Setting(SettingsPage.SubtitlesAudio))
                    }
                    RowDivider(tiled = true)
                    EntryRow(stringResource(R.string.assistant_title), R.drawable.ic_microphone_fill, Color(0xFFFF2D55)) {
                        navigator.open(Route.Setting(SettingsPage.Assistant))
                    }
                }
            }
        }
        serverGroup(model, servers)
        item {
            Group {
                EntryRow(stringResource(R.string.settings_about), R.drawable.ic_info_fill, Color(0xFF8E8E93)) {
                    navigator.open(Route.Setting(SettingsPage.About))
                }
            }
        }
        item {
            // Signing the account out forgets its servers too.
            MavioButton(
                stringResource(R.string.auth_sign_out),
                { scope.launch { runCatching { model.signOut() } } },
                Modifier.fillMaxWidth(),
                kind = ButtonKind.Secondary,
                icon = R.drawable.ic_sign_out,
                large = true,
            )
        }
    }
}

/** A page of the settings. */
@Composable
fun SettingsPageScreen(model: AppModel, page: SettingsPage) {
    when (page) {
        SettingsPage.Account -> AccountPage(model)
        SettingsPage.Appearance -> AppearancePage()
        SettingsPage.Playback -> PlaybackSettings()
        SettingsPage.SubtitlesAudio -> SubtitlesAudioSettings(model)
        SettingsPage.Assistant -> AssistantSettings(model)
        SettingsPage.About -> AboutSettings(model)
    }
}

/**
 * The account: the Mavio account with its page at mavio.dev, or the user
 * of the server; the servers in the user's order, which orders Home; and
 * authorizing a device with Quick Connect.
 */
@Composable
private fun AccountPage(model: AppModel) {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    val account = signedInAccount(model)
    val server = signedInServers(model).firstOrNull()
    SettingsList(stringResource(R.string.settings_account)) {
        if (account != null) {
            item {
                Group(footer = stringResource(R.string.settings_mavio_account_message)) {
                    NameRow(account.name)
                    manage(account)?.let { url ->
                        RowDivider()
                        LinkRow(stringResource(R.string.settings_manage_at_mavio), R.drawable.ic_external) {
                            context.startActivity(Intent(Intent.ACTION_VIEW, url))
                        }
                    }
                }
            }
        } else if (server != null) {
            item {
                Group(footer = stringResource(R.string.settings_standalone_account_message, server.userName, server.name)) {
                    NameRow(server.userName)
                }
            }
            item { SwitchUserGroup(model, server) }
            item { SecurityGroups(model, server) }
        }
        item {
            Group(
                stringResource(R.string.auth_servers),
                stringResource(if (account != null) R.string.settings_servers_mavio_message else R.string.settings_servers_message),
            ) {
                servers.forEachIndexed { i, s ->
                    if (i > 0) RowDivider()
                    ServerRow(model, s, i, servers, s.id in signedIn)
                }
                // A Mavio account's servers are those it lists, which syncing
                // keeps, so a server added beside them would vanish.
                if (account == null) {
                    if (servers.isNotEmpty()) RowDivider()
                    LinkRow(stringResource(R.string.auth_add_server), R.drawable.ic_add) { navigator.open(Route.AddServer) }
                }
            }
        }
        if (signedIn.isNotEmpty()) item { AuthorizeDevice(model, servers.filter { it.id in signedIn }) }
    }
}

/** The account's page at the mavio.dev whose API it signed in with. */
internal fun manage(account: Account): Uri? {
    val api = account.api.trimEnd('/').takeIf { it.startsWith("http") } ?: return null
    return (api.removeSuffix("/api") + "/account").toUri()
}

/**
 * A server with the user signed in to it and the state of its connection;
 * its menu moves it in the order, copies its address and signs out of it.
 */
@Composable
private fun ServerRow(model: AppModel, server: ServerRecord, index: Int, servers: List<ServerRecord>, signedIn: Boolean) {
    val states by model.states.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    var open by remember { mutableStateOf(false) }
    val account = signedInAccount(model)
    val colors = theme.colors
    fun move(by: Int) {
        val ids = servers.map { it.id }.toMutableList()
        ids[index] = ids[index + by].also { ids[index + by] = ids[index] }
        model.order(ids)
    }
    Row(
        Modifier.fillMaxWidth().clickable { open = true }.padding(horizontal = 16.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(server.name, style = theme.type.headline.text, maxLines = 1)
            Text(
                if (signedIn) stringResource(R.string.auth_signed_in_as, server.userName) else stringResource(R.string.auth_signed_out),
                style = theme.type.footnote.text,
                color = colors.textSecondary,
                maxLines = 1,
            )
            Text(server.address, style = theme.type.caption.text, color = colors.textTertiary, maxLines = 1)
        }
        when (states[server.id]) {
            is StreamState.Online -> Box(Modifier.size(8.dp).background(colors.success, CircleShape))
            is StreamState.Offline -> MavioIcon(R.drawable.ic_offline, size = 18.dp, tint = colors.textTertiary)
            else -> {}
        }
        // The menu opens at its button.
        Box {
            MavioIcon(R.drawable.ic_more, size = 20.dp, tint = colors.textSecondary)
            DropdownMenu(open, onDismissRequest = { open = false }) {
                @Composable
                fun Item(label: String, @DrawableRes icon: Int, color: Color = colors.textPrimary, onClick: () -> Unit) = DropdownMenuItem(
                    text = { Text(label, style = theme.type.callout.text, color = color) },
                    leadingIcon = { MavioIcon(icon, size = 18.dp, tint = color) },
                    onClick = {
                        open = false
                        onClick()
                    },
                )
                if (index > 0) Item(stringResource(R.string.auth_move_up), R.drawable.ic_back) { move(-1) }
                if (index < servers.lastIndex) Item(stringResource(R.string.auth_move_down), R.drawable.ic_forward) { move(1) }
                Item(stringResource(R.string.settings_copy_address), R.drawable.ic_copy) {
                    context.getSystemService(ClipboardManager::class.java)
                        .setPrimaryClip(ClipData.newPlainText(server.name, server.address))
                }
                if (!signedIn && account == null) {
                    Item(stringResource(R.string.auth_sign_in), R.drawable.ic_profile) { navigator.open(Route.AddServer) }
                }
                Item(stringResource(R.string.auth_sign_out), R.drawable.ic_sign_out, colors.danger) {
                    scope.launch { runCatching { model.remove(server.id) } }
                }
            }
        }
    }
}

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
            ChoiceRow(
                stringResource(R.string.auth_server),
                servers.map { it.id to it.name },
                serverId,
            ) { serverId = it }
            RowDivider()
        }
        Row(
            Modifier.padding(16.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            SettingsField(
                code,
                { new -> code = new.filter(Char::isDigit).take(6) },
                stringResource(R.string.auth_quick_connect_title),
                Modifier.weight(1f),
                placeholder = "000000",
                keyboard = KeyboardType.NumberPassword,
            )
            MavioButton(
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
                Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp),
                style = theme.type.callout.text,
                color = if (done) theme.colors.textSecondary else theme.colors.danger,
            )
        }
    }
}

/** Appearance, as the web UI's: the theme as three pictures, and the language, which Android sets for each app. */
@Composable
private fun AppearancePage() {
    val prefs = LocalPrefs.current
    val context = LocalContext.current
    SettingsList(stringResource(R.string.settings_appearance)) {
        item {
            Group(footer = stringResource(R.string.settings_appearance_message)) {
                Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                    Appearance.entries.forEach { option ->
                        val on = option == prefs.appearance
                        Column(
                            Modifier.weight(1f).clip(RoundedCornerShape(12.dp)).clickable { prefs.appearance = option },
                            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            ThemePreview(option, on)
                            Text(
                                stringResource(option.label),
                                style = theme.type.callout.text,
                                fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal,
                                color = if (on) theme.colors.textPrimary else theme.colors.textSecondary,
                            )
                        }
                    }
                }
            }
        }
        // Android 13 and later choose each app's language in the system's settings.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            item {
                Group {
                    val language = language(context) ?: stringResource(R.string.settings_system_language)
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .clickable {
                                context.startActivity(
                                    Intent(Settings.ACTION_APP_LOCALE_SETTINGS, Uri.fromParts("package", context.packageName, null)),
                                )
                            }
                            .heightIn(min = 52.dp)
                            .padding(horizontal = 16.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(stringResource(R.string.settings_language), Modifier.weight(1f), style = theme.type.body.text)
                        Text(language, style = theme.type.body.text, color = theme.colors.textSecondary)
                        MavioIcon(R.drawable.ic_external, size = 16.dp, tint = theme.colors.textSecondary)
                    }
                }
            }
        }
    }
}

/** The language chosen for the app, in itself; null when it follows the system. */
private fun language(context: Context): String? {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return null
    val locales = context.getSystemService(LocaleManager::class.java).applicationLocales
    if (locales.isEmpty) return null
    val locale = locales[0]
    return locale.getDisplayName(locale).replaceFirstChar { it.titlecase(locale) }
}

/** A small window in a theme, as the web UI draws it; System shows both halves. */
@Composable
private fun ThemePreview(appearance: Appearance, on: Boolean) {
    val shape = RoundedCornerShape(12.dp)
    Row(
        Modifier
            .fillMaxWidth()
            .aspectRatio(16f / 10f)
            .clip(shape)
            .border(if (on) 2.dp else 1.dp, if (on) theme.colors.accent else theme.colors.border, shape),
    ) {
        when (appearance) {
            Appearance.SYSTEM -> {
                Pane(dark = false, Modifier.weight(1f))
                Pane(dark = true, Modifier.weight(1f))
            }
            Appearance.LIGHT -> Pane(dark = false, Modifier.weight(1f))
            Appearance.DARK -> Pane(dark = true, Modifier.weight(1f))
        }
    }
}

/** A pane of a theme's window: a sidebar, a picture and a line of text. */
@Composable
private fun Pane(dark: Boolean, modifier: Modifier) {
    Canvas(modifier.fillMaxSize().background(Color(if (dark) 0xFF1C1C1E else 0xFFF2F2F7))) {
        val pad = 8.dp.toPx()
        val gap = 6.dp.toPx()
        val radius = CornerRadius(4.dp.toPx())
        val w = size.width - 2 * pad
        val h = size.height - 2 * pad
        val side = (w - gap) / 4
        drawRoundRect(Color(if (dark) 0xFF2C2C2E else 0xFFFFFFFF), Offset(pad, pad), Size(side, h), radius)
        val x = pad + side + gap
        val picture = Size(w - side - gap, h / 2)
        drawRoundRect(
            Brush.linearGradient(
                listOf(Color(0xFFFFD34E), Color(0xFFFF9A2E), Color(0xFFF0412F)),
                start = Offset(x, pad),
                end = Offset(x + picture.width, pad + picture.height),
            ),
            Offset(x, pad),
            picture,
            radius,
        )
        val line = 6.dp.toPx()
        drawRoundRect(
            Color(if (dark) 0xFF3A3A3C else 0xFFD1D1D6),
            Offset(x, pad + picture.height + gap),
            Size(w * 0.5f, line),
            CornerRadius(line / 2),
        )
    }
}
