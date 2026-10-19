package dev.mavio.app.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Details
import dev.mavio.core.duration
import dev.mavio.core.entry
import dev.mavio.core.ms
import dev.mavio.core.positionAt
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.Group
import dev.mavio.ui.GroupState
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsField
import dev.mavio.ui.TvButton
import dev.mavio.ui.TvDialog
import dev.mavio.ui.groupChip
import dev.mavio.ui.groupWaitingFor
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
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

// SyncPlay on the television (docs/clients.md §6.3.24, docs/tv.md §5), as
// the web UI's TV mode: the dialog creating, joining and leaving a group
// and playing an item in it, the indicator in the navigation bar while in
// one, the controller's chip, and bringing the player to the group's entry
// when the group waits for the device; the player follows the group with
// libs/android/player.

/**
 * The device's group, or the server's groups to join and a new one; in a
 * group on the item's server, the item plays in it from its resume
 * position.
 */
@Composable
internal fun SyncPlayDialog(model: AppModel, serverId: String, details: Details?, onDismiss: () -> Unit) {
    val sp = model.syncPlay
    val joined by sp.joined.collectAsStateWithLifecycle()
    val pages = LocalPages.current
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    val run = { f: suspend () -> Unit ->
        busy = true
        failure = null
        scope.launch {
            runCatching { f() }.onFailure { failure = it.message ?: it.toString() }
            busy = false
        }
    }
    val j = joined
    if (j != null) {
        val group = j.group
        val entry = group.entry
        val top = pages.top
        val watching = entry != null && group.state != SyncPlayState.SYNC_PLAY_STATE_IDLE &&
            !(top is Page.Player && top.serverId == j.serverId && top.itemId == entry.itemId)
        // Wide enough for the buttons, the item's name among them.
        TvDialog(group.name, onDismiss, width = 820.dp) {
            GroupState(model, j.serverId, group)
            Box(Modifier.heightIn(max = 360.dp).verticalScroll(rememberScrollState())) {
                Group {
                    group.membersList.forEachIndexed { i, m ->
                        if (i > 0) RowDivider()
                        val detail = listOfNotNull(
                            m.deviceName.ifEmpty { null },
                            if (m.sessionId == sp.sessionId(j.serverId)) stringResource(R.string.settings_this_device) else null,
                        ).joinToString(" · ")
                        Row(
                            Modifier.fillMaxWidth().heightIn(min = 72.dp).padding(horizontal = MavioSpacing.s20, vertical = MavioSpacing.s8),
                            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            MavioIcon(R.drawable.ic_devices, size = 24.dp, tint = theme.colors.textSecondary)
                            RowLabel(m.userName, detail.ifEmpty { null }, Modifier.weight(1f))
                            if (m.buffering) Text(stringResource(R.string.syncplay_loading), style = theme.type.callout.text, color = theme.colors.textSecondary)
                        }
                    }
                }
            }
            DialogFailure(failure)
            DialogButtons {
                SettingsButton(stringResource(R.string.syncplay_leave), {
                    run {
                        sp.leave()
                        onDismiss()
                    }
                }, kind = ButtonKind.Secondary, enabled = !busy)
                if (watching) {
                    SettingsButton(stringResource(R.string.syncplay_watch), {
                        // The group's position, taken as it is followed.
                        val at = group.positionAt(sp.serverNow())
                        onDismiss()
                        scope.launch {
                            val title = runCatching { model.client.details(j.serverId, entry.itemId).card.title }.getOrDefault("")
                            pages.play(j.serverId, entry.itemId, title, null, Duration.ofMillis(at))
                        }
                    }, kind = ButtonKind.Secondary)
                }
                if (details != null && j.serverId == serverId) {
                    SettingsButton(stringResource(R.string.syncplay_play_here, details.card.title), {
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
                    SettingsButton(stringResource(R.string.keys_done), onDismiss)
                }
            }
        }
        return
    }

    var groups by remember { mutableStateOf<List<SyncPlayGroup>?>(null) }
    LaunchedEffect(serverId) {
        runCatching { sp.call(serverId, "ListGroups", listGroupsRequest {}, ListGroupsResponse::parseFrom).groupsList }
            .onSuccess { groups = it }
            .onFailure { failure = it.message ?: it.toString() }
    }
    val user = model.server(serverId)?.userName.orEmpty()
    val defaultName = stringResource(R.string.syncplay_default_name, user)
    var name by remember { mutableStateOf(defaultName) }
    TvDialog(stringResource(R.string.syncplay_title), onDismiss) {
        Text(stringResource(R.string.syncplay_intro), style = theme.type.callout.text, color = theme.colors.textSecondary)
        val shown = groups.orEmpty()
        if (shown.isNotEmpty()) {
            Box(Modifier.heightIn(max = 300.dp).verticalScroll(rememberScrollState())) {
                Group {
                    shown.forEachIndexed { i, g ->
                        if (i > 0) RowDivider()
                        // The row joins its group.
                        DialogRow(
                            R.drawable.ic_users,
                            g.name,
                            g.membersList.joinToString(", ") { it.userName },
                            onClick = {
                                if (!busy) {
                                    run {
                                        val r = sp.call(serverId, "JoinGroup", joinGroupRequest { groupId = g.id }, JoinGroupResponse::parseFrom)
                                        if (r.hasGroup()) sp.enter(serverId, r.group)
                                    }
                                }
                            },
                        ) {
                            Text(stringResource(R.string.syncplay_join), style = theme.type.callout.text, color = theme.colors.accentText)
                        }
                    }
                }
            }
        }
        if (groups?.isEmpty() == true) {
            Text(stringResource(R.string.syncplay_no_groups), style = theme.type.body.text, color = theme.colors.textSecondary)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                SettingsField(name, { if (it.length <= 200) name = it }, stringResource(R.string.syncplay_group_name))
            }
            SettingsButton(stringResource(R.string.syncplay_create), {
                run {
                    val r = sp.call(serverId, "CreateGroup", createGroupRequest { this.name = name.trim() }, CreateGroupResponse::parseFrom)
                    if (r.hasGroup()) sp.enter(serverId, r.group)
                }
            }, kind = ButtonKind.Secondary, enabled = !busy && name.isNotBlank())
        }
        DialogFailure(failure)
        DialogButtons { SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary) }
    }
}

/** The navigation bar's indicator of the device's group, opening it. */
@Composable
internal fun GroupIndicator(model: AppModel, modifier: Modifier = Modifier) {
    val joined by model.syncPlay.joined.collectAsStateWithLifecycle()
    val j = joined ?: return
    var open by remember { mutableStateOf(false) }
    TvButton(groupChip(j.group), { open = true }, modifier.widthIn(max = 420.dp), icon = R.drawable.ic_users_fill)
    if (open) SyncPlayDialog(model, j.serverId, null) { open = false }
}

/** The controller's chip of the group the player follows: who it waits for, else the group's name and size; it opens the group. */
@Composable
internal fun GroupChip(model: AppModel, serverId: String, group: SyncPlayGroup, onOpen: (Boolean) -> Unit) {
    var open by remember { mutableStateOf(false) }
    DisposableEffect(open) {
        onOpen(open)
        onDispose { onOpen(false) }
    }
    TvButton(
        if (group.state == SyncPlayState.SYNC_PLAY_STATE_WAITING) groupWaitingFor(group) else groupChip(group),
        { open = true },
        Modifier.widthIn(max = 480.dp),
        icon = R.drawable.ic_users_fill,
    )
    if (open) SyncPlayDialog(model, serverId, null) { open = false }
}

/** Brings the player to the group's entry, at its position, when the group waits for the device. */
@Composable
internal fun FollowGroup(model: AppModel, pages: Pages) {
    LaunchedEffect(pages) {
        model.syncPlay.joined.collect { j ->
            val group = j?.group ?: return@collect
            val entry = group.entry ?: return@collect
            if (group.state == SyncPlayState.SYNC_PLAY_STATE_IDLE || model.syncPlay.me(j)?.buffering != true) return@collect
            val top = pages.top
            if (top is Page.Player && top.serverId == j.serverId && top.itemId == entry.itemId) return@collect
            val title = runCatching { model.client.details(j.serverId, entry.itemId).card.title }.getOrDefault("")
            val page = Page.Player(j.serverId, entry.itemId, title, null, Duration.ofMillis(if (group.hasPosition()) group.position.ms else 0))
            // A player moves on to the group's next entry in place.
            if (pages.top is Page.Player) pages.replace(page) else pages.play(page.serverId, page.itemId, page.title, null, page.start)
        }
    }
}
