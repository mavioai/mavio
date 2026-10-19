package dev.mavio.player

import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.common.ForwardingPlayer
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import dev.mavio.core.AppModel
import dev.mavio.core.SyncPlay
import dev.mavio.core.duration
import dev.mavio.core.entry
import dev.mavio.core.ms
import dev.mavio.core.positionAt
import dev.mavio.core.timestamp
import kotlin.math.abs
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
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
import mavio.session.v1.nextItemRequest
import mavio.session.v1.pauseRequest
import mavio.session.v1.reportStateRequest
import mavio.session.v1.seekRequest
import mavio.session.v1.setQueueRequest
import mavio.session.v1.stopRequest
import mavio.session.v1.unpauseRequest

// The player following a SyncPlay group (docs/clients.md §6.3.24), shared
// by the phone's and the television's players.

/** The group playing an item, when the device follows one. */
fun SyncPlay.Joined?.playing(serverId: String, itemId: String): SyncPlayGroup? {
    val group = this?.takeIf { it.serverId == serverId }?.group ?: return null
    return group.takeIf { it.state != SyncPlayState.SYNC_PLAY_STATE_IDLE && it.entry?.itemId == itemId }
}

/** What the player does instead of acting on its own while it follows a group. */
class GroupPlayer(private val sp: SyncPlay, private val serverId: String, val group: SyncPlayGroup, private val scope: CoroutineScope) {
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
fun rememberGroupPlayer(model: AppModel, serverId: String, itemId: String, player: Player?, onStop: () -> Unit): GroupPlayer? {
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
class GroupControls(player: Player, private val group: () -> GroupPlayer?) : ForwardingPlayer(player) {
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
