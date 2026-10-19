package dev.mavio.core

import mavio.session.v1.Syncplay.SyncPlayCommandKind
import mavio.session.v1.Syncplay.SyncPlayGroup
import mavio.session.v1.Syncplay.SyncPlayQueueEntry
import mavio.session.v1.Syncplay.SyncPlayState
import org.junit.Assert.assertEquals
import org.junit.Test

class SyncPlayTest {
    @Test
    fun clockSample() {
        // Sent at 1000 and answered at 1100 by the device; received at 5040 and answered at 5050 by the server.
        val (offset, rtt) = SyncPlay.clockSample(1000, 5040, 5050, 1100)
        assertEquals("offset", 3995L, offset)
        assertEquals("rtt", 90L, rtt)
    }

    @Test
    fun positionAt() {
        val cases = listOf(
            Triple("playing advances", SyncPlayState.SYNC_PLAY_STATE_PLAYING, 15_000L) to 65_000L,
            Triple("playing waits for its start", SyncPlayState.SYNC_PLAY_STATE_PLAYING, 5_000L) to 60_000L,
            Triple("paused stays", SyncPlayState.SYNC_PLAY_STATE_PAUSED, 15_000L) to 60_000L,
            Triple("waiting stays", SyncPlayState.SYNC_PLAY_STATE_WAITING, 15_000L) to 60_000L,
        )
        for ((case, want) in cases) {
            val (name, state, at) = case
            val group = SyncPlayGroup.newBuilder().setState(state).setPosition(duration(60_000)).setPositionTime(timestamp(10_000)).build()
            assertEquals(name, want, group.positionAt(at))
        }
    }

    @Test
    fun durations() {
        assertEquals(1_234L, duration(1_234).ms)
        assertEquals(1_700_000_000_123L, timestamp(1_700_000_000_123).ms)
    }

    @Test
    fun caughtUp() {
        val cases = listOf(
            Triple("playing plays on from where it is", SyncPlayState.SYNC_PLAY_STATE_PLAYING, SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_UNPAUSE to 65_000L),
            Triple("paused pauses where it stopped", SyncPlayState.SYNC_PLAY_STATE_PAUSED, SyncPlayCommandKind.SYNC_PLAY_COMMAND_KIND_PAUSE to 60_000L),
            Triple("waiting carries nothing out", SyncPlayState.SYNC_PLAY_STATE_WAITING, null),
        )
        for ((name, state, want) in cases) {
            val group = SyncPlayGroup.newBuilder().setState(state).setPosition(duration(60_000)).setPositionTime(timestamp(10_000))
                .addQueue(SyncPlayQueueEntry.newBuilder().setId("e").setItemId("i")).build()
            val u = SyncPlay.caughtUp(group, 15_000)
            assertEquals(name, group, u.group)
            if (want == null) {
                assertEquals(name, false, u.hasCommand())
                continue
            }
            assertEquals(name, want.first, u.command.kind)
            assertEquals(name, want.second, u.command.position.ms)
            assertEquals(name, 15_000L, u.command.`when`.ms)
            assertEquals(name, "e", u.command.entryId)
        }
    }
}
