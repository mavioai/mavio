package dev.mavio.ui

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.res.stringResource
import dev.mavio.core.AppModel
import dev.mavio.core.entry
import mavio.session.v1.Syncplay.SyncPlayGroup
import mavio.session.v1.Syncplay.SyncPlayState

// What the apps say of a SyncPlay group (docs/clients.md §6.3.24).

/** Who a group waits for. */
@Composable
fun groupWaitingFor(group: SyncPlayGroup): String =
    stringResource(R.string.syncplay_waiting_for, group.membersList.filter { it.buffering }.joinToString(", ") { it.deviceName.ifEmpty { it.userName } })

/** A group's name and size. */
@Composable
fun groupChip(group: SyncPlayGroup): String = stringResource(R.string.syncplay_chip, group.name, group.membersCount.toString())

/** What a group does: its state and the name of its entry. */
@Composable
fun GroupState(model: AppModel, serverId: String, group: SyncPlayGroup) {
    val entry = group.entry
    var name by remember(entry?.itemId) { mutableStateOf("…") }
    LaunchedEffect(entry?.itemId) {
        entry?.let { e -> runCatching { model.client.details(serverId, e.itemId) }.onSuccess { name = it.card.title } }
    }
    val text = when (group.state) {
        SyncPlayState.SYNC_PLAY_STATE_WAITING -> groupWaitingFor(group)
        SyncPlayState.SYNC_PLAY_STATE_PAUSED -> stringResource(R.string.syncplay_paused, name)
        SyncPlayState.SYNC_PLAY_STATE_PLAYING -> stringResource(R.string.syncplay_playing, name)
        else -> stringResource(R.string.syncplay_idle)
    }
    Text(text, style = theme.type.callout.text, color = theme.colors.textSecondary)
}

