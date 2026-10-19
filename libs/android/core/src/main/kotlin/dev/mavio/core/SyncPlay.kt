package dev.mavio.core

import com.google.protobuf.Duration
import com.google.protobuf.MessageLite
import com.google.protobuf.Timestamp
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import mavio.session.v1.Syncplay.GetTimeResponse
import mavio.session.v1.Syncplay.JoinGroupResponse
import mavio.session.v1.Syncplay.LeaveGroupResponse
import mavio.session.v1.Syncplay.ListGroupsResponse
import mavio.session.v1.Syncplay.ReportPingResponse
import mavio.session.v1.Syncplay.SyncPlayGroup
import mavio.session.v1.Syncplay.SyncPlayMember
import mavio.session.v1.Syncplay.SyncPlayCommandKind
import mavio.session.v1.Syncplay.SyncPlayState
import mavio.session.v1.Syncplay.SyncPlayUpdate
import mavio.session.v1.getTimeRequest
import mavio.session.v1.joinGroupRequest
import mavio.session.v1.leaveGroupRequest
import mavio.session.v1.listGroupsRequest
import mavio.session.v1.reportPingRequest
import mavio.session.v1.syncPlayCommand
import mavio.session.v1.syncPlayUpdate

/**
 * SyncPlay (docs/clients.md §6.3.24): the group the device is in, kept
 * from its servers' events, with the clock kept in step with the group's
 * server, as the web UI's. Its state changes on the main thread.
 */
class SyncPlay internal constructor(private val client: Client, private val scope: CoroutineScope) {
    /** The group the device is in, on its server. */
    data class Joined(val serverId: String, val group: SyncPlayGroup)

    /** An update of a server's group, with the command it carries out. */
    data class Update(val serverId: String, val update: SyncPlayUpdate)

    private val _joined = MutableStateFlow<Joined?>(null)

    /** The group the device is in. */
    val joined: StateFlow<Joined?> = _joined.asStateFlow()

    private val _updates = MutableSharedFlow<Update>(extraBufferCapacity = 16)

    /** The groups' updates, as they come, for the player that follows the group. */
    val updates: SharedFlow<Update> = _updates.asSharedFlow()

    private val sessions = mutableMapOf<String, String>()
    private var offset = 0L
    private var clock: Job? = null

    /** The ID of the device's session on a server, which names it among a group's members. */
    fun sessionId(serverId: String): String? = sessions[serverId]

    /** The device among a group's members. */
    fun me(joined: Joined): SyncPlayMember? = joined.group.membersList.firstOrNull { it.sessionId == sessions[joined.serverId] }

    /** The time by the group's server's clock, in milliseconds. */
    fun serverNow(): Long = System.currentTimeMillis() + offset

    /** Calls a procedure of the SyncPlay service. */
    suspend fun <T : MessageLite> call(serverId: String, method: String, request: MessageLite, parse: (ByteArray) -> T): T =
        client.call(serverId, "/mavio.session.v1.SyncPlayService/$method", request, parse)

    /** Takes the group a request to join or create answered with. */
    fun enter(serverId: String, group: SyncPlayGroup) = set(Joined(serverId, group))

    /** Leaves the group. */
    suspend fun leave() {
        val j = _joined.value ?: return
        call(j.serverId, "LeaveGroup", leaveGroupRequest {}, LeaveGroupResponse::parseFrom)
        set(null)
    }

    internal fun connected(serverId: String, sessionId: String) {
        val again = sessions.put(serverId, sessionId) != null
        val j = _joined.value
        if (again && j?.serverId == serverId) scope.launch { runCatching { resume(j) } }
    }

    /**
     * Takes the group up again after the server's stream came back: the
     * server drops a device whose stream goes away, and the device misses
     * the group's updates meanwhile. A device dropped joins again, which
     * makes the group wait for it; one still in the group carries out where
     * the group is now.
     */
    private suspend fun resume(was: Joined) {
        val serverId = was.serverId
        val group = call(serverId, "ListGroups", listGroupsRequest {}, ListGroupsResponse::parseFrom)
            .groupsList.firstOrNull { it.id == was.group.id }
        if (_joined.value?.group?.id != was.group.id) return
        when {
            group == null -> update(serverId, SyncPlayUpdate.getDefaultInstance())
            group.membersList.none { it.sessionId == sessions[serverId] } ->
                enter(serverId, call(serverId, "JoinGroup", joinGroupRequest { groupId = group.id }, JoinGroupResponse::parseFrom).group)
            else -> update(serverId, caughtUp(group, serverNow()))
        }
    }

    internal fun update(serverId: String, update: SyncPlayUpdate) {
        if (update.hasGroup()) {
            set(Joined(serverId, update.group))
        } else if (_joined.value?.serverId == serverId) {
            set(null)
        }
        _updates.tryEmit(Update(serverId, update))
    }

    private fun set(joined: Joined?) {
        val was = _joined.value?.serverId
        _joined.value = joined
        if (joined?.serverId != was) {
            clock?.cancel()
            clock = joined?.let { j -> scope.launch { measure(j.serverId) } }
        }
    }

    /** Measures the server's clock as the group starts and every half minute, and tells the group the round trip. */
    private suspend fun measure(serverId: String) {
        while (true) {
            runCatching {
                var best: Pair<Long, Long>? = null
                repeat(4) {
                    val t0 = System.currentTimeMillis()
                    val r = call(serverId, "GetTime", getTimeRequest {}, GetTimeResponse::parseFrom)
                    val t3 = System.currentTimeMillis()
                    if (r.hasReceiveTime() && r.hasSendTime()) {
                        val s = clockSample(t0, r.receiveTime.ms, r.sendTime.ms, t3)
                        if (best == null || s.second < best!!.second) best = s
                    }
                }
                best?.let { (o, rtt) ->
                    offset = o
                    call(serverId, "ReportPing", reportPingRequest { ping = duration(rtt.coerceAtMost(60_000)) }, ReportPingResponse::parseFrom)
                }
            }
            delay(30_000)
        }
    }

    companion object {
        /** The update that brings a player to where a group is at a time of the server's clock. */
        fun caughtUp(group: SyncPlayGroup, serverMs: Long): SyncPlayUpdate {
            val kind = when (group.state) {
                SyncPlayState.SYNC_PLAY_STATE_PLAYING -> SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_UNPAUSE
                SyncPlayState.SYNC_PLAY_STATE_PAUSED -> SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_PAUSE
                else -> null
            }
            return syncPlayUpdate {
                this.group = group
                if (kind != null) {
                    command = syncPlayCommand {
                        this.kind = kind
                        position = duration(group.positionAt(serverMs))
                        when_ = timestamp(serverMs)
                        group.entry?.let { entryId = it.id }
                    }
                }
            }
        }

        /**
         * The offset and round trip of an exchange sent at t0 and answered
         * at t3 by the device's clock, which the server received at t1 and
         * answered at t2 by its own.
         */
        fun clockSample(t0: Long, t1: Long, t2: Long, t3: Long): Pair<Long, Long> =
            (t1 - t0 + (t2 - t3)) / 2 to maxOf(0, t3 - t0 - (t2 - t1))
    }
}

/** Where a group is at a time of the server's clock, in milliseconds: advancing from its position while it plays. */
fun SyncPlayGroup.positionAt(serverMs: Long): Long {
    val position = if (hasPosition()) position.ms else 0
    val at = if (hasPositionTime()) positionTime.ms else serverMs
    return if (state == SyncPlayState.SYNC_PLAY_STATE_PLAYING && serverMs > at) position + (serverMs - at) else position
}

/** The group's current entry. */
val SyncPlayGroup.entry get() = queueList.getOrNull(currentIndex)

/** A duration in milliseconds. */
val Duration.ms: Long get() = seconds * 1000 + nanos / 1_000_000

/** A time in milliseconds since the epoch. */
val Timestamp.ms: Long get() = seconds * 1000 + nanos / 1_000_000

/** A duration of milliseconds. */
fun duration(ms: Long): Duration = Duration.newBuilder().setSeconds(ms / 1000).setNanos(((ms % 1000) * 1_000_000).toInt()).build()

/** A time of milliseconds since the epoch. */
fun timestamp(ms: Long): Timestamp = Timestamp.newBuilder().setSeconds(ms / 1000).setNanos(((ms % 1000) * 1_000_000).toInt()).build()
