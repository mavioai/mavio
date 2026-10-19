package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.common.Player
import dev.mavio.admin.reason
import dev.mavio.core.AppModel
import dev.mavio.core.Details
import dev.mavio.core.SyncPlay
import dev.mavio.core.duration
import dev.mavio.core.entry
import dev.mavio.core.ms
import dev.mavio.core.positionAt
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.GroupState
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.SettingsField
import dev.mavio.ui.groupChip
import dev.mavio.ui.groupWaitingFor
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch
import mavio.session.v1.Syncplay.CreateGroupResponse
import mavio.session.v1.Syncplay.JoinGroupResponse
import mavio.session.v1.Syncplay.ListGroupsResponse
import mavio.session.v1.Syncplay.SetQueueResponse
import mavio.session.v1.Syncplay.SyncPlayGroup
import mavio.session.v1.Syncplay.SyncPlayState
import mavio.session.v1.createGroupRequest
import mavio.session.v1.joinGroupRequest
import mavio.session.v1.listGroupsRequest
import mavio.session.v1.setQueueRequest

// SyncPlay (docs/clients.md §6.3.24), as the web UI's: the sheet creating,
// joining and leaving a group and playing an item in it, the indicator the
// screens show while in one, and bringing the player to the group's entry
// when the group waits for the device; the player follows the group with
// libs/android/player.

/**
 * The device's group, or the server's groups to join and a new one; in a
 * group on the item's server, the item plays in it from its resume
 * position.
 */
@Composable
internal fun SyncPlaySheet(model: AppModel, serverId: String, details: Details?, onDismiss: () -> Unit) {
    val sp = model.syncPlay
    val joined by sp.joined.collectAsStateWithLifecycle()
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    val run = { f: suspend () -> Unit ->
        busy = true
        failure = null
        scope.launch {
            runCatching { f() }.onFailure { failure = it.reason }
            busy = false
        }
    }
    val j = joined
    if (j != null) {
        val group = j.group
        val entry = group.entry
        val top = navigator.top
        val watching = entry != null && group.state != SyncPlayState.SYNC_PLAY_STATE_IDLE &&
            !(top is Route.Player && top.serverId == j.serverId && top.itemId == entry.itemId)
        Sheet(group.name, onDismiss) {
            GroupState(model, j.serverId, group)
            Column {
                group.membersList.forEach { m ->
                    val detail = listOfNotNull(
                        m.deviceName.ifEmpty { null },
                        if (m.sessionId == sp.sessionId(j.serverId)) stringResource(R.string.settings_this_device) else null,
                    ).joinToString(" · ")
                    SheetRow(R.drawable.ic_devices, m.userName, detail.ifEmpty { null }, enabled = false, onClick = {}) {
                        if (m.buffering) Text(stringResource(R.string.syncplay_loading), style = theme.type.footnote.text, color = theme.colors.textSecondary)
                    }
                }
            }
            SheetFailure(failure)
            FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.End), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                MavioButton(stringResource(R.string.syncplay_leave), {
                    run {
                        sp.leave()
                        onDismiss()
                    }
                }, kind = ButtonKind.Secondary, enabled = !busy)
                if (watching) {
                    MavioButton(stringResource(R.string.syncplay_watch), {
                        // The group's position, taken as it is followed.
                        val at = group.positionAt(sp.serverNow())
                        onDismiss()
                        scope.launch {
                            val title = runCatching { model.client.details(j.serverId, entry.itemId).card.title }.getOrDefault("")
                            navigator.play(j.serverId, entry.itemId, title, null, java.time.Duration.ofMillis(at))
                        }
                    }, kind = ButtonKind.Secondary)
                }
                if (details != null && j.serverId == serverId) {
                    MavioButton(stringResource(R.string.syncplay_play_here, details.card.title), {
                        run {
                            sp.call(
                                j.serverId,
                                "SetQueue",
                                setQueueRequest {
                                    itemIds += details.card.itemId
                                    details.position?.let { startPosition = duration(it.toMillis()) }
                                },
                                SetQueueResponse::parseFrom,
                            )
                            onDismiss()
                        }
                    }, enabled = !busy)
                } else {
                    MavioButton(stringResource(R.string.keys_done), onDismiss)
                }
            }
        }
        return
    }

    var groups by remember { mutableStateOf<List<SyncPlayGroup>?>(null) }
    LaunchedEffect(serverId) {
        runCatching { sp.call(serverId, "ListGroups", listGroupsRequest {}, ListGroupsResponse::parseFrom).groupsList }
            .onSuccess { groups = it }
            .onFailure { failure = it.reason }
    }
    val user = model.server(serverId)?.userName.orEmpty()
    val defaultName = stringResource(R.string.syncplay_default_name, user)
    var name by remember { mutableStateOf(defaultName) }
    Sheet(stringResource(R.string.syncplay_title), onDismiss) {
        Text(stringResource(R.string.syncplay_intro), style = theme.type.callout.text, color = theme.colors.textSecondary)
        Column {
            groups?.forEach { g ->
                SheetRow(R.drawable.ic_users, g.name, g.membersList.joinToString(", ") { it.userName }, enabled = false, onClick = {}) {
                    MavioButton(stringResource(R.string.syncplay_join), {
                        run {
                            val r = sp.call(serverId, "JoinGroup", joinGroupRequest { groupId = g.id }, JoinGroupResponse::parseFrom)
                            if (r.hasGroup()) sp.enter(serverId, r.group)
                        }
                    }, kind = ButtonKind.Secondary, enabled = !busy)
                }
            }
            if (groups?.isEmpty() == true) {
                Text(stringResource(R.string.syncplay_no_groups), style = theme.type.callout.text, color = theme.colors.textSecondary)
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                SettingsField(name, { if (it.length <= 200) name = it }, stringResource(R.string.syncplay_group_name))
            }
            MavioButton(stringResource(R.string.syncplay_create), {
                run {
                    val r = sp.call(serverId, "CreateGroup", createGroupRequest { this.name = name.trim() }, CreateGroupResponse::parseFrom)
                    if (r.hasGroup()) sp.enter(serverId, r.group)
                }
            }, kind = ButtonKind.Secondary, enabled = !busy && name.isNotBlank())
        }
        SheetFailure(failure)
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            MavioButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
        }
    }
}

/** The screens' indicator of the device's group, opening it. */
@Composable
internal fun GroupIndicator(model: AppModel, modifier: Modifier) {
    val joined by model.syncPlay.joined.collectAsStateWithLifecycle()
    val j = joined ?: return
    var open by remember { mutableStateOf(false) }
    val colors = theme.colors
    Row(
        modifier
            .widthIn(max = 360.dp)
            .height(40.dp)
            .shadow(8.dp, CircleShape)
            .clip(CircleShape)
            .background(colors.surfaceOverlay)
            .border(1.dp, colors.border, CircleShape)
            .clickable { open = true }
            .padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(R.drawable.ic_users_fill, size = 18.dp, tint = colors.textPrimary)
        Text(groupChip(j.group), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
    if (open) SyncPlaySheet(model, j.serverId, null) { open = false }
}

/** The player's chip of the group it follows: who it waits for, else the group's name and size; it opens the group. */
@Composable
internal fun GroupChip(model: AppModel, serverId: String, group: SyncPlayGroup, onOpen: (Boolean) -> Unit, modifier: Modifier = Modifier) {
    var open by remember { mutableStateOf(false) }
    DisposableEffect(open) {
        onOpen(open)
        onDispose { onOpen(false) }
    }
    Row(
        modifier
            .height(36.dp)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.15f))
            .clickable { open = true }
            .padding(horizontal = 14.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(R.drawable.ic_users_fill, size = 16.dp, tint = Color.White)
        Text(
            if (group.state == SyncPlayState.SYNC_PLAY_STATE_WAITING) groupWaitingFor(group) else groupChip(group),
            style = theme.type.footnote.text,
            fontWeight = FontWeight.SemiBold,
            color = Color.White,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
    if (open) SyncPlaySheet(model, serverId, null) { open = false }
}

/** Brings the player to the group's entry, at its position, when the group waits for the device. */
@Composable
internal fun FollowGroup(model: AppModel, navigator: Navigator) {
    LaunchedEffect(Unit) {
        model.syncPlay.joined.collect { j ->
            val group = j?.group ?: return@collect
            val entry = group.entry ?: return@collect
            if (group.state == SyncPlayState.SYNC_PLAY_STATE_IDLE || model.syncPlay.me(j)?.buffering != true) return@collect
            val top = navigator.top
            if (top is Route.Player && top.serverId == j.serverId && top.itemId == entry.itemId) return@collect
            val title = runCatching { model.client.details(j.serverId, entry.itemId).card.title }.getOrDefault("")
            val route = Route.Player(j.serverId, entry.itemId, title, null, java.time.Duration.ofMillis(if (group.hasPosition()) group.position.ms else 0))
            // A player moves on to the group's next entry in place.
            if (navigator.top is Route.Player) navigator.replace(route) else navigator.open(route)
        }
    }
}
