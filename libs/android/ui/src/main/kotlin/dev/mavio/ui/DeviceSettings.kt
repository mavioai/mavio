package dev.mavio.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.Account
import dev.mavio.core.AppModel
import dev.mavio.core.PlaybackPreferences
import dev.mavio.core.ServerRecord
import dev.mavio.core.SignInState
import dev.mavio.core.SubtitleMode
import dev.mavio.design.MavioSpacing
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

// The settings of the account and of this device that a phone, a tablet
// and a television show alike.

/** The user's card: the Mavio account, or the user of the server signed in to. */
@Composable
fun AccountCard(model: AppModel, modifier: Modifier, chevron: Boolean = true) {
    val account = signedInAccount(model)
    val server = signedInServers(model).firstOrNull()
    val name = account?.name ?: server?.userName.orEmpty()
    Row(
        modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 12.dp),
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
        if (chevron) Chevron()
    }
}

/** The Mavio account signed in, followed. */
@Composable
fun signedInAccount(model: AppModel): Account? {
    val state by model.signInState.collectAsStateWithLifecycle()
    return (state as? SignInState.Account)?.v1
}

/** The servers signed in to, in the user's order, followed. */
@Composable
fun signedInServers(model: AppModel): List<ServerRecord> {
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    return servers.filter { it.id in signedIn }
}

/**
 * Playback, as the web UI's: the highest quality streamed to this device,
 * which the player's panel also sets, and whether the next episode plays
 * when one ends.
 */
@Composable
fun PlaybackSettings() {
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
fun SubtitlesAudioSettings(model: AppModel) {
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
                        SettingsButton(
                            stringResource(R.string.action_save),
                            {
                                if (a == null || s == null) return@SettingsButton
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
