package dev.mavio.app

import androidx.annotation.OptIn
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
import androidx.compose.runtime.rememberUpdatedState
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
import androidx.media3.common.ForwardingPlayer
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import dev.mavio.core.AppModel
import dev.mavio.core.Details
import dev.mavio.core.SyncPlay
import dev.mavio.core.duration
import dev.mavio.core.entry
import dev.mavio.core.ms
import dev.mavio.core.positionAt
import dev.mavio.core.timestamp
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlin.math.abs
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.session.v1.Syncplay.CreateGroupResponse
import mavio.session.v1.Syncplay.JoinGroupResponse
import mavio.session.v1.Syncplay.ListGroupsResponse
import mavio.session.v1.Syncplay.NextItemResponse
import mavio.session.v1.Syncplay.PauseResponse
import mavio.session.v1.Syncplay.ReportStateResponse
import mavio.session.v1.Syncplay.SeekResponse
import mavio.session.v1.Syncplay.SetQueueResponse
import mavio.session.v1.Syncplay.StopResponse
import mavio.session.v1.Syncplay.SyncPlayCommandKind
import mavio.session.v1.Syncplay.SyncPlayGroup
import mavio.session.v1.Syncplay.SyncPlayState
import mavio.session.v1.Syncplay.UnpauseResponse
import mavio.session.v1.createGroupRequest
import mavio.session.v1.joinGroupRequest
import mavio.session.v1.listGroupsRequest
import mavio.session.v1.nextItemRequest
import mavio.session.v1.pauseRequest
import mavio.session.v1.reportStateRequest
import mavio.session.v1.seekRequest
import mavio.session.v1.setQueueRequest
import mavio.session.v1.stopRequest
import mavio.session.v1.unpauseRequest

// SyncPlay (docs/clients.md §6.3.24), as the web UI's: the sheet creating,
// joining and leaving a group and playing an item in it, the indicator the
// screens show while in one, bringing the player to the group's entry when
// the group waits for the device, and the player following the group.

/** Who a group waits for. */
@Composable
private fun waitingFor(group: SyncPlayGroup): String =
    stringResource(R.string.syncplay_waiting_for, group.membersList.filter { it.buffering }.joinToString(", ") { it.deviceName.ifEmpty { it.userName } })

/** A group's name and size. */
@Composable
private fun chip(group: SyncPlayGroup): String = stringResource(R.string.syncplay_chip, group.name, group.membersCount.toString())

/** What a group does: its state and the name of its entry. */
@Composable
private fun GroupState(model: AppModel, serverId: String, group: SyncPlayGroup) {
    val entry = group.entry
    var name by remember(entry?.itemId) { mutableStateOf("…") }
    LaunchedEffect(entry?.itemId) {
        entry?.let { e -> runCatching { model.client.details(serverId, e.itemId) }.onSuccess { name = it.card.title } }
    }
    val text = when (group.state) {
        SyncPlayState.SYNC_PLAY_STATE_WAITING -> waitingFor(group)
        SyncPlayState.SYNC_PLAY_STATE_PAUSED -> stringResource(R.string.syncplay_paused, name)
        SyncPlayState.SYNC_PLAY_STATE_PLAYING -> stringResource(R.string.syncplay_playing, name)
        else -> stringResource(R.string.syncplay_idle)
    }
    Text(text, style = theme.type.callout.text, color = theme.colors.textSecondary)
}

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
        MavioIcon(R.drawable.ic_users_fill, size = 18.dp, tint = colors.accentText)
        Text(chip(j.group), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
    if (open) SyncPlaySheet(model, j.serverId, null) { open = false }
}

/** The player's chip of the group it follows: who it waits for, else the group's name and size; it opens the group. */
@Composable
internal fun GroupChip(model: AppModel, serverId: String, group: SyncPlayGroup, modifier: Modifier = Modifier) {
    var open by remember { mutableStateOf(false) }
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
            if (group.state == SyncPlayState.SYNC_PLAY_STATE_WAITING) waitingFor(group) else chip(group),
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

/** The group playing an item, when the device follows one. */
internal fun SyncPlay.Joined?.playing(serverId: String, itemId: String): SyncPlayGroup? {
    val group = this?.takeIf { it.serverId == serverId }?.group ?: return null
    return group.takeIf { it.state != SyncPlayState.SYNC_PLAY_STATE_IDLE && it.entry?.itemId == itemId }
}

/** What the player does instead of acting on its own while it follows a group. */
internal class GroupPlayer(private val sp: SyncPlay, private val serverId: String, val group: SyncPlayGroup, private val scope: CoroutineScope) {
    private fun send(f: suspend () -> Unit) {
        scope.launch { runCatching { f() } }
    }

    fun unpause() = send { sp.call(serverId, "Unpause", unpauseRequest {}, UnpauseResponse::parseFrom) }

    fun pause() = send { sp.call(serverId, "Pause", pauseRequest {}, PauseResponse::parseFrom) }

    /** Seeks the group to a position in milliseconds. */
    fun seek(ms: Long) = send { sp.call(serverId, "Seek", seekRequest { position = duration(ms.coerceAtLeast(0)) }, SeekResponse::parseFrom) }

    /** Whether the group's queue has an entry after this one. */
    val hasNext: Boolean get() = group.currentIndex < group.queueCount - 1

    /** Plays the group's next entry, else items after it. */
    fun next(itemIds: List<String>) = send {
        if (hasNext) {
            sp.call(serverId, "NextItem", nextItemRequest {}, NextItemResponse::parseFrom)
        } else if (itemIds.isNotEmpty()) {
            sp.call(serverId, "SetQueue", setQueueRequest { this.itemIds += itemIds }, SetQueueResponse::parseFrom)
        }
    }

    /** The video ended: the group's first member moves it on. */
    fun ended() {
        if (group.membersList.firstOrNull()?.sessionId != sp.sessionId(serverId)) return
        send {
            if (hasNext) {
                sp.call(serverId, "NextItem", nextItemRequest {}, NextItemResponse::parseFrom)
            } else {
                sp.call(serverId, "Stop", stopRequest {}, StopResponse::parseFrom)
            }
        }
    }
}

/**
 * Has a player follow the group while the group plays its item: it waits
 * paused at the group's position until the group plays, reports when it is
 * ready or buffering, carries out the group's commands at their time, and
 * keeps up with the group's position. Null while the player plays on its own.
 */
@Composable
internal fun rememberGroupPlayer(model: AppModel, serverId: String, itemId: String, player: Player?, onStop: () -> Unit): GroupPlayer? {
    val sp = model.syncPlay
    val joined by sp.joined.collectAsStateWithLifecycle()
    val group = joined.playing(serverId, itemId)
    val scope = rememberCoroutineScope()
    val latest = rememberUpdatedState(group)
    val stop = rememberUpdatedState(onStop)
    val ready = remember { arrayOfNulls<Job>(1) }

    // Tells the group where the player is, and whether it can play on.
    val report = { pl: Player, isReady: Boolean ->
        latest.value?.entry?.let { entry ->
            val position = pl.currentPosition.coerceAtLeast(0)
            val playing = pl.isPlaying
            scope.launch {
                runCatching {
                    sp.call(
                        serverId,
                        "ReportState",
                        reportStateRequest {
                            entryId = entry.id
                            this.position = duration(position)
                            when_ = timestamp(sp.serverNow())
                            this.playing = playing
                            this.ready = isReady
                        },
                        ReportStateResponse::parseFrom,
                    )
                }
            }
        }
    }
    // Pauses at a position and reports ready once the player can play there.
    val readyAt = { pl: Player, ms: Long ->
        ready[0]?.cancel()
        ready[0] = scope.launch {
            pl.pause()
            pl.setPlaybackSpeed(1f)
            if (abs(pl.currentPosition - ms) > 250) {
                pl.seekTo(ms)
                delay(150)
            }
            while (pl.playbackState != Player.STATE_READY) delay(100)
            report(pl, true)
        }
    }

    // A member the group waits for gets ready at the group's position.
    val me = joined?.let { sp.me(it) }
    val waiting = group?.state == SyncPlayState.SYNC_PLAY_STATE_WAITING && me?.buffering == true
    val positionMs = if (group?.hasPosition() == true) group.position.ms else 0
    LaunchedEffect(player, waiting, positionMs) {
        if (waiting && player != null) readyAt(player, positionMs)
    }

    // The player restarted its stream, as when the user picks other tracks, or joined a group playing.
    LaunchedEffect(player) {
        if (player != null && latest.value?.state == SyncPlayState.SYNC_PLAY_STATE_PLAYING) report(player, false)
    }

    // The group's commands, at their time.
    LaunchedEffect(player) {
        val pl = player ?: return@LaunchedEffect
        var timer: Job? = null
        sp.updates.collect { (s, u) ->
            if (s != serverId || !u.hasCommand() || !u.hasGroup() || u.group.entry?.itemId != itemId) return@collect
            val c = u.command
            timer?.cancel()
            val at = if (c.hasPosition()) c.position.ms else 0
            when (c.kind) {
                SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_PAUSE -> {
                    ready[0]?.cancel()
                    pl.pause()
                    if (abs(pl.currentPosition - at) > 500) pl.seekTo(at)
                }
                SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_SEEK -> readyAt(pl, at)
                SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_STOP -> stop.value()
                SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_UNPAUSE -> {
                    ready[0]?.cancel()
                    // when is in the server's time; the player starts then, or catches up when it passed.
                    val wait = (if (c.hasWhen()) c.`when`.ms else sp.serverNow()) - sp.serverNow()
                    pl.setPlaybackSpeed(1f)
                    if (wait > 0) {
                        pl.seekTo(at)
                        timer = launch {
                            delay(wait)
                            pl.play()
                        }
                    } else {
                        pl.seekTo(at - wait)
                        pl.play()
                    }
                }
                else -> Unit
            }
        }
    }

    // Buffering while the group plays makes the group wait.
    val playing = group?.state == SyncPlayState.SYNC_PLAY_STATE_PLAYING
    DisposableEffect(player, playing) {
        val pl = player
        if (!playing || pl == null) return@DisposableEffect onDispose {}
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_BUFFERING) report(pl, false)
            }
        }
        pl.addListener(listener)
        onDispose { pl.removeListener(listener) }
    }

    // Keeps up with the group: far behind or ahead it skips, near it speeds up or slows down a little.
    LaunchedEffect(player, playing) {
        val pl = player ?: return@LaunchedEffect
        if (!playing) return@LaunchedEffect
        try {
            while (true) {
                delay(1000)
                val g = latest.value ?: continue
                if (!pl.isPlaying || pl.playbackState != Player.STATE_READY) continue
                val off = (pl.currentPosition - g.positionAt(sp.serverNow())) / 1000f
                if (abs(off) > 2) {
                    pl.setPlaybackSpeed(1f)
                    pl.seekTo((pl.currentPosition - off * 1000).toLong().coerceAtLeast(0))
                } else {
                    pl.setPlaybackSpeed(if (abs(off) < 0.05f) 1f else 1 - off.coerceIn(-0.5f, 0.5f) / 5)
                }
            }
        } finally {
            pl.setPlaybackSpeed(1f)
        }
    }

    return remember(group) { group?.let { GroupPlayer(sp, serverId, it, scope) } }
}

/**
 * The player as its controls, the system's media controls and headsets see
 * it: while it follows a group, playing, pausing and seeking ask the group
 * instead, and the speed stays the group's.
 */
@OptIn(UnstableApi::class)
internal class GroupControls(player: Player, private val group: () -> GroupPlayer?) : ForwardingPlayer(player) {
    override fun play() {
        group()?.unpause() ?: super.play()
    }

    override fun pause() {
        group()?.pause() ?: super.pause()
    }

    override fun setPlayWhenReady(playWhenReady: Boolean) {
        if (group() == null) super.setPlayWhenReady(playWhenReady) else if (playWhenReady) play() else pause()
    }

    override fun seekTo(positionMs: Long) {
        group()?.seek(positionMs) ?: super.seekTo(positionMs)
    }

    override fun seekTo(mediaItemIndex: Int, positionMs: Long) {
        group()?.seek(positionMs) ?: super.seekTo(mediaItemIndex, positionMs)
    }

    override fun seekBack() {
        if (group() == null) super.seekBack() else seekTo((currentPosition - seekBackIncrement).coerceAtLeast(0))
    }

    override fun seekForward() {
        if (group() == null) super.seekForward() else seekTo(currentPosition + seekForwardIncrement)
    }

    override fun setPlaybackSpeed(speed: Float) {
        if (group() == null) super.setPlaybackSpeed(speed)
    }

    override fun isCommandAvailable(command: Int): Boolean =
        if (command == COMMAND_SET_SPEED_AND_PITCH && group() != null) false else super.isCommandAvailable(command)

    override fun getAvailableCommands(): Player.Commands =
        if (group() != null) super.getAvailableCommands().buildUpon().remove(COMMAND_SET_SPEED_AND_PITCH).build() else super.getAvailableCommands()
}
