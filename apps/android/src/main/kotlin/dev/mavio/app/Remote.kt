package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import dev.mavio.admin.deviceName
import dev.mavio.admin.reason
import dev.mavio.core.AppModel
import dev.mavio.core.DeviceSession
import dev.mavio.core.RemoteCommand
import dev.mavio.core.RemotePlayback
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.Group
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** How often what the device plays is read again. */
private const val REFRESH = 2_000L

/** A position as a clock, hours shown when there are any. */
private fun clock(seconds: Long): String {
    val s = seconds.coerceAtLeast(0)
    return if (s >= 3600) "%d:%02d:%02d".format(s / 3600, s / 60 % 60, s % 60) else "%d:%02d".format(s / 60, s % 60)
}

/**
 * Controls another of the user's devices, as the web UI's remote: what it
 * plays with its position, the playback's buttons, the volume, and a
 * message to show on it.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun RemoteSheet(model: AppModel, serverId: String, sessionId: String, onDismiss: () -> Unit) {
    var device by remember { mutableStateOf<DeviceSession?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var volume by remember { mutableFloatStateOf(100f) }
    var text by remember { mutableStateOf("") }
    var sent by remember { mutableStateOf(false) }
    var refresh by remember { mutableStateOf(0) }
    val scope = rememberCoroutineScope()

    suspend fun load() {
        try {
            device = model.client.devices(serverId).firstOrNull { it.id == sessionId }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }

    suspend fun run(command: RemoteCommand): Boolean = try {
        model.client.sendCommand(serverId, sessionId, command)
        failure = null
        true
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        failure = e.reason
        false
    }

    fun send(command: RemoteCommand) {
        scope.launch { if (run(command)) refresh++ }
    }

    fun sendMessage() {
        val message = text.trim()
        if (message.isEmpty()) return
        scope.launch {
            if (run(RemoteCommand.Message(message))) {
                text = ""
                sent = true
            }
        }
    }
    LaunchedEffect(refresh) {
        while (true) {
            load()
            delay(REFRESH)
        }
    }
    ModalBottomSheet(onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = theme.colors.background) {
        Column(
            Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(horizontal = theme.layout.margin).padding(bottom = MavioSpacing.s24)
                .navigationBarsPadding(),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            Text(
                device?.let(::deviceName) ?: stringResource(R.string.remote_title),
                style = theme.type.title3.text,
                fontWeight = FontWeight.SemiBold,
            )
            val d = device
            when {
                d == null && failure == null -> Text(stringResource(R.string.state_loading), style = theme.type.body.text, color = theme.colors.textSecondary)
                d == null -> {}
                !d.online -> Text(stringResource(R.string.remote_offline), style = theme.type.body.text, color = theme.colors.textSecondary)
                d.playing != null -> NowPlaying(d.playing!!, ::send)
                else -> Text(stringResource(R.string.remote_idle), style = theme.type.body.text, color = theme.colors.textSecondary)
            }
            if (d?.online == true) {
                Group(stringResource(R.string.remote_volume)) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Control(R.drawable.ic_mute, stringResource(R.string.player_mute)) { send(RemoteCommand.Volume(null, true)) }
                        val label = stringResource(R.string.remote_volume)
                        Slider(
                            volume,
                            { volume = it },
                            Modifier.weight(1f).semantics { contentDescription = label },
                            valueRange = 0f..100f,
                            onValueChangeFinished = { send(RemoteCommand.Volume(volume.toInt().toUInt(), false)) },
                            colors = sliderColors(),
                        )
                        Text(
                            volume.toInt().toString(),
                            Modifier.width(32.dp),
                            style = theme.type.footnote.text,
                            color = theme.colors.textSecondary,
                        )
                    }
                }
                Group(stringResource(R.string.remote_message)) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        OutlinedTextField(
                            text,
                            {
                                text = it.take(1000)
                                sent = false
                            },
                            Modifier.weight(1f),
                            placeholder = { Text(stringResource(R.string.remote_message)) },
                            singleLine = true,
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                            keyboardActions = KeyboardActions(onSend = { sendMessage() }),
                            shape = RoundedCornerShape(10.dp),
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedContainerColor = theme.colors.fill,
                                unfocusedContainerColor = theme.colors.fill,
                                focusedBorderColor = theme.colors.textSecondary,
                                unfocusedBorderColor = theme.colors.border,
                                cursorColor = theme.colors.textPrimary,
                            ),
                        )
                        val enabled = text.isNotBlank()
                        Text(
                            stringResource(if (sent) R.string.remote_sent else R.string.remote_send),
                            Modifier.clickable(enabled = enabled) { sendMessage() }.padding(vertical = 8.dp),
                            style = theme.type.body.text,
                            fontWeight = FontWeight.SemiBold,
                            color = if (enabled) theme.colors.accentText else theme.colors.textTertiary,
                        )
                    }
                }
            }
            failure?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.danger) }
        }
    }
}

@Composable
private fun sliderColors() = SliderDefaults.colors(
    thumbColor = theme.colors.textPrimary,
    activeTrackColor = theme.colors.textPrimary,
    inactiveTrackColor = theme.colors.fill,
)

/** What the device plays: its artwork and titles, its position to drag, and the playback's buttons. */
@Composable
private fun NowPlaying(p: RemotePlayback, send: (RemoteCommand) -> Unit) {
    val card = p.card
    val runtime = card.runtime?.seconds ?: 0L
    val at = p.position.seconds
    // The position dragged, sent once the slider is let go.
    var dragged by remember { mutableStateOf<Float?>(null) }
    val position = minOf(dragged?.toLong() ?: at, if (runtime > 0) runtime else at)
    Column(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(theme.colors.surface).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(96.dp, 54.dp).clip(RoundedCornerShape(6.dp))) {
                Artwork((card.thumb ?: card.backdrop ?: card.poster)?.url, Modifier.size(96.dp, 54.dp))
            }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(card.title, style = theme.type.headline.text, maxLines = 1)
                card.subtitle?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1) }
            }
        }
        if (runtime > 0) {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                val label = stringResource(R.string.remote_position)
                Slider(
                    position.toFloat(),
                    { dragged = it },
                    Modifier.semantics { contentDescription = label },
                    valueRange = 0f..runtime.toFloat(),
                    onValueChangeFinished = {
                        dragged?.let { send(RemoteCommand.Seek(Duration.ofSeconds(it.toLong()))) }
                        dragged = null
                    },
                    colors = sliderColors(),
                )
                Row {
                    Text(clock(position), Modifier.weight(1f), style = theme.type.caption.text, color = theme.colors.textSecondary)
                    Text(clock(runtime), style = theme.type.caption.text, color = theme.colors.textSecondary)
                }
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalAlignment = Alignment.CenterVertically) {
            Control(R.drawable.ic_previous, stringResource(R.string.remote_previous)) { send(RemoteCommand.Previous) }
            Control(R.drawable.ic_seek_back, pluralStringResource(R.plurals.player_seek_back, 10, 10)) {
                send(RemoteCommand.Seek(Duration.ofSeconds(maxOf(0, at - 10))))
            }
            Control(
                if (p.paused) R.drawable.ic_play_fill else R.drawable.ic_pause_fill,
                stringResource(if (p.paused) R.string.player_play else R.string.player_pause),
                large = true,
            ) { send(if (p.paused) RemoteCommand.Unpause else RemoteCommand.Pause) }
            Control(R.drawable.ic_seek_forward, pluralStringResource(R.plurals.player_seek_forward, 10, 10)) {
                send(RemoteCommand.Seek(Duration.ofSeconds(at + 10)))
            }
            Control(R.drawable.ic_next, stringResource(R.string.remote_next)) { send(RemoteCommand.Next) }
        }
        Text(
            stringResource(R.string.remote_stop),
            Modifier.align(Alignment.CenterHorizontally).clip(RoundedCornerShape(8.dp)).clickable { send(RemoteCommand.Stop) }
                .padding(horizontal = 12.dp, vertical = 6.dp),
            style = theme.type.body.text,
            color = theme.colors.danger,
        )
    }
}

/** A round button of the remote, larger for playing and pausing. */
@Composable
private fun Control(@DrawableRes icon: Int, label: String, large: Boolean = false, onClick: () -> Unit) {
    Box(
        Modifier.size(if (large) 56.dp else 44.dp).clip(CircleShape).background(if (large) theme.colors.fill else theme.colors.surface.copy(alpha = 0f))
            .clickable(onClickLabel = label, onClick = onClick).semantics { contentDescription = label },
        Alignment.Center,
    ) {
        MavioIcon(icon, size = if (large) 28.dp else 22.dp, tint = theme.colors.textPrimary)
    }
}
