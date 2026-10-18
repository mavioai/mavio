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
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.net.toUri
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.Account
import dev.mavio.core.AppModel
import dev.mavio.core.PlaybackPreferences
import dev.mavio.core.ServerRecord
import dev.mavio.core.SignInState
import dev.mavio.core.StreamState
import dev.mavio.core.SubtitleMode
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
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
}

/**
 * The settings, as the web UI's on a phone: the account's card, opening
 * the account with its servers; this device's sections; the versions; and
 * signing out.
 */
@Composable
fun SettingsScreen(model: AppModel) {
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    SettingsList(stringResource(R.string.nav_settings), large = true) {
        item {
            Group {
                AccountCard(model, Modifier.clickable { navigator.open(Route.Setting(SettingsPage.Account)) })
            }
        }
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
        SettingsPage.Playback -> PlaybackPage()
        SettingsPage.SubtitlesAudio -> SubtitlesAudioPage(model)
    }
}

/** The user's card: the Mavio account, or the user of the server signed in to. */
@Composable
private fun AccountCard(model: AppModel, modifier: Modifier) {
    val account = signedInAccount(model)
    val server = signedInServers(model).firstOrNull()
    val name = account?.name ?: server?.userName.orEmpty()
    Row(
        modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Avatar(name, size = 44.dp)
        Column(Modifier.weight(1f)) {
            Text(name, style = theme.type.headline.text, maxLines = 1)
            Text(
                if (account != null) stringResource(R.string.auth_mavio_account) else server?.name.orEmpty(),
                style = theme.type.footnote.text,
                color = theme.colors.textSecondary,
                maxLines = 1,
            )
        }
        Chevron()
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
                if (servers.isNotEmpty()) RowDivider()
                LinkRow(stringResource(R.string.auth_add_server), R.drawable.ic_add) { navigator.open(Route.AddServer) }
            }
        }
        if (signedIn.isNotEmpty()) item { AuthorizeDevice(model, servers.filter { it.id in signedIn }) }
    }
}

/** The Mavio account signed in, followed. */
@Composable
private fun signedInAccount(model: AppModel): Account? {
    val state by model.signInState.collectAsStateWithLifecycle()
    return (state as? SignInState.Account)?.v1
}

/** The servers signed in to, in the user's order, followed. */
@Composable
private fun signedInServers(model: AppModel): List<ServerRecord> {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    return servers.filter { it.id in signedIn }
}

/** The account's page at the mavio.dev whose API it signed in with. */
private fun manage(account: Account): Uri? {
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
                if (!signedIn) Item(stringResource(R.string.auth_sign_in), R.drawable.ic_profile) { navigator.open(Route.AddServer) }
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

/**
 * Playback, as the web UI's: the highest quality streamed to this device,
 * which the player's panel also sets, and whether the next episode plays
 * when one ends.
 */
@Composable
private fun PlaybackPage() {
    val prefs = LocalPrefs.current
    val context = LocalContext.current
    SettingsList(stringResource(R.string.settings_playback)) {
        item {
            Group(footer = stringResource(R.string.settings_appearance_message)) {
                ChoiceRow(
                    stringResource(R.string.settings_max_quality),
                    Quality.entries.map { it to (it.label?.let(context::getString) ?: it.key) },
                    prefs.quality,
                ) { prefs.quality = it }
                RowDivider()
                SwitchRow(stringResource(R.string.settings_autoplay_next), prefs.autoplayNext) { prefs.autoplayNext = it }
            }
        }
    }
}

/** When subtitles show, in the web UI's order. */
private val SUBTITLE_MODES = listOf(
    SubtitleMode.DEFAULT to R.string.settings_subtitle_mode_default,
    SubtitleMode.ALWAYS to R.string.settings_subtitle_mode_always,
    SubtitleMode.FOREIGN to R.string.settings_subtitle_mode_foreign,
    SubtitleMode.SMART to R.string.settings_subtitle_mode_smart,
    SubtitleMode.FORCED to R.string.settings_subtitle_mode_forced,
    SubtitleMode.NEVER to R.string.settings_subtitle_mode_none,
)

/** Three-letter codes from a field: lowercase, separated by commas or spaces; null when one is not. */
private fun languages(text: String): List<String>? {
    val list = text.lowercase().split(Regex("[\\s,，、]+")).filter { it.isNotEmpty() }
    return list.takeIf { codes -> codes.all { it.matches(Regex("[a-z]{3}")) } }
}

/**
 * The user's languages and when subtitles show, read from the first
 * server signed in to and saved to each of them.
 */
@Composable
private fun SubtitlesAudioPage(model: AppModel) {
    // The preferences as saved; null while they load.
    var saved by remember { mutableStateOf<Result<PlaybackPreferences>?>(null) }
    LaunchedEffect(Unit) { saved = runCatching { model.client.playbackPreferences() } }
    var audio by remember { mutableStateOf("") }
    var subtitles by remember { mutableStateOf("") }
    var mode by remember { mutableStateOf(SubtitleMode.DEFAULT) }
    LaunchedEffect(saved) {
        saved?.getOrNull()?.let {
            audio = it.audioLanguages.joinToString(", ")
            subtitles = it.subtitleLanguages.joinToString(", ")
            mode = it.subtitleMode
        }
    }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    SettingsList(stringResource(R.string.settings_subtitles_audio), stringResource(R.string.settings_subtitles_audio_message)) {
        val prefs = saved?.getOrNull()
        when {
            saved == null -> item { StatusView(stringResource(R.string.state_loading)) }
            prefs == null -> item { StatusView(stringResource(R.string.state_error), icon = R.drawable.ic_error) }
            else -> {
                val a = languages(audio)
                val s = languages(subtitles)
                val changed = audio != prefs.audioLanguages.joinToString(", ") ||
                    subtitles != prefs.subtitleLanguages.joinToString(", ") ||
                    mode != prefs.subtitleMode
                item {
                    Group(footer = stringResource(R.string.settings_languages_hint)) {
                        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                            SettingsField(audio, { audio = it }, stringResource(R.string.settings_audio_languages), placeholder = "chi, eng", error = a == null)
                            SettingsField(subtitles, { subtitles = it }, stringResource(R.string.settings_subtitle_languages), placeholder = "chi, eng", error = s == null)
                        }
                    }
                }
                item {
                    Group {
                        ChoiceRow(
                            stringResource(R.string.settings_subtitle_mode),
                            SUBTITLE_MODES.map { (m, label) -> m to stringResource(label) },
                            mode,
                        ) { mode = it }
                    }
                }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                        MavioButton(
                            stringResource(R.string.action_save),
                            {
                                if (a == null || s == null) return@MavioButton
                                busy = true
                                error = null
                                scope.launch {
                                    try {
                                        val next = PlaybackPreferences(a, s, mode, prefs.playDefaultAudioTrack)
                                        model.client.setPlaybackPreferences(next)
                                        saved = Result.success(next)
                                    } catch (e: CancellationException) {
                                        throw e
                                    } catch (e: Exception) {
                                        error = e.message ?: e.toString()
                                    } finally {
                                        busy = false
                                    }
                                }
                            },
                            enabled = changed && a != null && s != null && !busy,
                        )
                        error?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
                    }
                }
            }
        }
    }
}

/** A page of settings: its title, an optional message, then its groups. */
@Composable
private fun SettingsList(title: String, message: String? = null, large: Boolean = false, content: LazyListScope.() -> Unit) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            start = theme.layout.margin,
            end = theme.layout.margin,
            top = MavioSpacing.s16,
            // No tab bar below the settings.
            bottom = MavioSpacing.s24 + WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding(),
        ),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
    ) {
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                Text(title, style = (if (large) theme.type.title1 else theme.type.title2).text, fontWeight = FontWeight.Bold)
                if (message != null) Text(message, style = theme.type.callout.text, color = theme.colors.textSecondary)
            }
        }
        content()
    }
}

/** A group of rows on a card, with an optional title above and footer below. */
@Composable
private fun Group(title: String? = null, footer: String? = null, content: @Composable ColumnScope.() -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        if (title != null) {
            Text(title, Modifier.padding(horizontal = 4.dp), style = theme.type.headline.text)
        }
        Column(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(theme.colors.surface),
            content = content,
        )
        if (footer != null) {
            Text(footer, Modifier.padding(horizontal = 4.dp), style = theme.type.footnote.text, color = theme.colors.textSecondary)
        }
    }
}

/** The line between rows, after the tiles when tiled. */
@Composable
private fun RowDivider(tiled: Boolean = false) {
    HorizontalDivider(Modifier.padding(start = if (tiled) 60.dp else 16.dp), color = theme.colors.border)
}

/** A row opening a section: its icon on a colored tile, and its title. */
@Composable
private fun EntryRow(title: String, @DrawableRes icon: Int, color: Color, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 52.dp).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(28.dp).background(color, RoundedCornerShape(7.dp)), Alignment.Center) {
            MavioIcon(icon, size = 17.dp, tint = Color.White)
        }
        Text(title, Modifier.weight(1f), style = theme.type.body.text)
        Chevron()
    }
}

/** The arrow of a row opening a page. */
@Composable
private fun Chevron() {
    MavioIcon(R.drawable.ic_forward, size = 18.dp, tint = theme.colors.textTertiary)
}

/** A label and its value. */
@Composable
private fun ValueRow(label: String, value: String) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f), style = theme.type.body.text)
        Text(value, style = theme.type.body.text, color = theme.colors.textSecondary)
    }
}

/** A person's initial and name. */
@Composable
private fun NameRow(name: String) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Avatar(name, size = 44.dp)
        Text(name, style = theme.type.headline.text)
    }
}

/** An action in the accent's color, with its icon. */
@Composable
private fun LinkRow(label: String, @DrawableRes icon: Int, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 48.dp).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(icon, size = 18.dp, tint = theme.colors.accentText)
        Text(label, style = theme.type.body.text, color = theme.colors.accentText)
    }
}

/** A setting turned on and off. */
@Composable
private fun SwitchRow(label: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable { onChange(!checked) }.heightIn(min = 52.dp).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f), style = theme.type.body.text)
        Switch(
            checked,
            onChange,
            colors = SwitchDefaults.colors(
                checkedTrackColor = theme.colors.success,
                checkedThumbColor = Color.White,
                uncheckedTrackColor = theme.colors.fill,
                uncheckedThumbColor = Color.White,
                uncheckedBorderColor = Color.Transparent,
            ),
        )
    }
}

/** A setting chosen from a menu, showing its choice. */
@Composable
private fun <T> ChoiceRow(label: String, choices: List<Pair<T, String>>, chosen: T, onChoose: (T) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Row(
        Modifier.fillMaxWidth().clickable { open = true }.heightIn(min = 52.dp).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f), style = theme.type.body.text)
        // The menu opens at the choice.
        Box {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    choices.firstOrNull { it.first == chosen }?.second.orEmpty(),
                    Modifier.widthIn(max = 200.dp),
                    style = theme.type.body.text,
                    color = theme.colors.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                MavioIcon(R.drawable.ic_selector, size = 16.dp, tint = theme.colors.textSecondary)
            }
            DropdownMenu(open, onDismissRequest = { open = false }) {
                choices.forEach { (value, text) ->
                    DropdownMenuItem(
                        text = { Text(text, style = theme.type.callout.text) },
                        onClick = {
                            open = false
                            onChoose(value)
                        },
                        trailingIcon = { if (value == chosen) MavioIcon(R.drawable.ic_check, size = 18.dp) },
                    )
                }
            }
        }
    }
}

/** A text field of the settings. */
@Composable
private fun SettingsField(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier.fillMaxWidth(),
    placeholder: String? = null,
    keyboard: KeyboardType = KeyboardType.Text,
    error: Boolean = false,
) {
    OutlinedTextField(
        value,
        onChange,
        modifier,
        label = { Text(label) },
        placeholder = placeholder?.let { { Text(it) } },
        singleLine = true,
        isError = error,
        keyboardOptions = KeyboardOptions(keyboardType = keyboard, autoCorrectEnabled = false),
        shape = RoundedCornerShape(10.dp),
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = theme.colors.fill,
            unfocusedContainerColor = theme.colors.fill,
            focusedBorderColor = theme.colors.textSecondary,
            unfocusedBorderColor = theme.colors.border,
            focusedLabelColor = theme.colors.textSecondary,
            cursorColor = theme.colors.textPrimary,
        ),
    )
}
