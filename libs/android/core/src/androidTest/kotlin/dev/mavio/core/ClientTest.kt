package dev.mavio.core

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The core on a device against a running server, named by the
 * instrumentation arguments `mavio.address`, `mavio.user` and
 * `mavio.password`; skipped without them. From the emulator the host's
 * loopback is `10.0.2.2`.
 */
@RunWith(AndroidJUnit4::class)
class ClientTest {
    private class MemoryTokens : SecureTokens {
        val tokens = ConcurrentHashMap<String, String>()

        override fun token(serverId: String): String? = tokens[serverId]

        override fun setToken(serverId: String, token: String) {
            tokens[serverId] = token
        }

        override fun deleteToken(serverId: String) {
            tokens.remove(serverId)
        }
    }

    private class Recorder : CoreListener, HomeListener {
        val state = MutableStateFlow<StreamState?>(null)
        val events = MutableStateFlow(0)
        val rows = MutableStateFlow<List<Row>>(emptyList())

        override fun serverState(serverId: String, state: StreamState) {
            this.state.value = state
        }

        override fun event(serverId: String, event: ServerEvent) {
            events.value += 1
        }

        override fun rows(rows: List<Row>) {
            this.rows.value = rows
        }
    }

    @Test
    fun signsInStreamsEventsAndLoadsHome() = runBlocking {
        val args = InstrumentationRegistry.getArguments()
        val address = args.getString("mavio.address")
        assumeTrue("no mavio.address argument", address != null)
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val context = instrumentation.targetContext
        // Servers on the local network need the runtime permission.
        instrumentation.uiAutomation.grantRuntimePermission(
            context.packageName,
            "android.permission.ACCESS_LOCAL_NETWORK",
        )
        val database = context.cacheDir.resolve("core-${UUID.randomUUID()}.sqlite")
        val recorder = Recorder()
        val client = Client(
            database.path,
            DeviceInfo(UUID.randomUUID().toString(), "Instrumented test", "Mavio for Android", "0.0.0"),
            MemoryTokens(),
            recorder,
        )
        try {
            val server = client.signIn(address!!, args.getString("mavio.user", ""), args.getString("mavio.password", ""))
            assertTrue("got = ${client.servers()}, want = the server", client.servers().any { it.id == server.id })

            client.start()
            withTimeout(15_000) { recorder.state.first { it is StreamState.Online } }
            withTimeout(15_000) { recorder.events.first { it > 0 } }

            client.loadHome(recorder)
            val rows = withTimeout(15_000) { recorder.rows.first { it.isNotEmpty() } }
            assertTrue("got = no row from ${server.name}", rows.any { it.serverId == server.id })
        } finally {
            client.stop()
            database.delete()
        }
    }
}
