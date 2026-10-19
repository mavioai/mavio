package dev.mavio.core

import android.content.Context
import android.os.Build
import android.provider.Settings
import java.io.File
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * The app's state over the core: the servers, their streams and Home.
 * Screens reload a server's data when its revision moves, which the
 * server's events and the user's changes do. Its state changes on the main
 * thread.
 */
class AppModel(context: Context, clientName: String) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val relay = Relay()

    /** The folder of downloaded files. */
    val downloadsFolder: File

    /** The core. */
    val client: Client

    private val _servers = MutableStateFlow<List<ServerRecord>>(emptyList())

    /** The servers, in the user's order. */
    val servers: StateFlow<List<ServerRecord>> = _servers.asStateFlow()

    private val _signInState = MutableStateFlow<SignInState>(SignInState.SignedOut)

    /** What the device is signed in with; signed out, the app shows its sign-in. */
    val signInState: StateFlow<SignInState> = _signInState.asStateFlow()

    private val _signedIn = MutableStateFlow<Set<String>>(emptySet())

    /** The servers the device holds a session of. */
    val signedIn: StateFlow<Set<String>> = _signedIn.asStateFlow()

    private val _states = MutableStateFlow<Map<String, StreamState>>(emptyMap())

    /** The state of each server's stream. */
    val states: StateFlow<Map<String, StreamState>> = _states.asStateFlow()

    private val _scanProgress = MutableStateFlow<Map<String, Map<String, ScanProgress>>>(emptyMap())

    /**
     * How far the libraries' scans have come, by server and library, as the
     * servers tell administrators.
     */
    val scanProgress: StateFlow<Map<String, Map<String, ScanProgress>>> = _scanProgress.asStateFlow()

    private val _home = MutableStateFlow<List<Row>>(emptyList())

    /** The rows of Home. */
    val home: StateFlow<List<Row>> = _home.asStateFlow()

    private val _revisions = MutableStateFlow<Map<String, Int>>(emptyMap())

    /** The revision of each server's data, which moves when it changes. */
    val revisions: StateFlow<Map<String, Int>> = _revisions.asStateFlow()

    private var homeJob: Job? = null

    /** Each server's libraries as last loaded, with the user and revision they were loaded for. */
    private val loadedLibraries = ConcurrentHashMap<String, Triple<String, Int, Libraries>>()

    /** The group the device watches together with others in. */
    val syncPlay: SyncPlay

    init {
        val dir = context.filesDir.resolve("mavio").apply { mkdirs() }
        downloadsFolder = dir.resolve("Downloads").apply { mkdirs() }
        client = Client(
            dir.resolve("core.sqlite").path,
            device(context, clientName),
            KeystoreTokens(context),
            relay,
        )
        syncPlay = SyncPlay(client, scope)
        client.setSyncPlayListener(relay)
        reloadServers()
    }

    /** The account signed in with, if a Mavio account. */
    val account: Account? get() = (signInState.value as? SignInState.Account)?.v1

    /** The servers the user is signed in to, in their order. */
    val signedInServers: List<ServerRecord> get() = servers.value.filter { it.id in signedIn.value }

    /** A server by ID. */
    fun server(id: String): ServerRecord? = servers.value.firstOrNull { it.id == id }

    /** Whether the server is reached at an address of the local network. */
    fun local(id: String): Boolean = server(id)?.let { isLocalAddress(it.address) } ?: false

    /** Reads the servers again from the core. */
    fun reloadServers() {
        _signInState.value = runCatching { client.signInState() }.getOrDefault(SignInState.SignedOut)
        val servers = runCatching { client.servers() }.getOrDefault(emptyList())
        _servers.value = servers
        _signedIn.value = servers.filter { client.signedIn(it.id) }.map { it.id }.toSet()
    }

    /**
     * Follows the servers' event streams, as the app comes to the
     * foreground, and the Mavio account's servers.
     */
    fun start() {
        runCatching { client.start() }
    }

    /** Stops the event streams, as the app goes to the background. */
    fun stop() {
        client.stop()
    }

    /** Loads Home again from every server; the job ends with the load, whether or not the rows changed. */
    fun refreshHome(): Job {
        homeJob?.cancel()
        return scope.launch {
            runCatching { client.loadHome(relay) }
        }.also { homeJob = it }
    }

    /**
     * A server's libraries as last loaded, while still current for its
     * user and revision, so that the Libraries tab shows them at once on
     * its return rather than loading them again.
     */
    fun heldLibraries(serverId: String): Libraries? {
        val (user, revision, libraries) = loadedLibraries[serverId] ?: return null
        return libraries.takeIf { user == server(serverId)?.userId && revision == (revisions.value[serverId] ?: 0) }
    }

    /** Loads a server's libraries, keeping them for [heldLibraries]. */
    suspend fun libraries(serverId: String): Libraries {
        val user = server(serverId)?.userId
        val revision = revisions.value[serverId] ?: 0
        val libraries = client.libraries(serverId)
        // Those of a server that could not be reached are not kept.
        if (user != null && !libraries.offline) loadedLibraries[serverId] = Triple(user, revision, libraries)
        return libraries
    }

    /** Marks a server's data changed, reloading Home and the screens showing it. */
    fun bump(serverId: String) {
        _revisions.update { it + (serverId to (it[serverId] ?: 0) + 1) }
        refreshHome()
    }

    /** Switches a server's session to another of its users, with its PIN when it has one. */
    suspend fun switchUser(serverId: String, userId: String, pin: String?) {
        client.switchUser(serverId, userId, pin)
        reloadServers()
        bump(serverId)
    }

    /** Signs in to the server at address with a name and password. */
    suspend fun signIn(address: String, name: String, password: String): ServerRecord =
        client.signIn(address, name, password).also { added() }

    /** Creates the first user of a new server and signs in as them. */
    suspend fun createFirstUser(address: String, name: String, password: String): ServerRecord =
        client.createFirstUser(address, name, password).also { added() }

    /** Asks whether a Quick Connect request was authorized, signing in when it was. */
    suspend fun pollQuickConnect(request: QuickConnectRequest): ServerRecord? =
        client.pollQuickConnect(request).also { if (it != null) added() }

    /** Asks whether a link code was approved, signing the account in when it was. */
    suspend fun pollLink(link: LinkCode): Account? = client.pollLink(link).also { if (it != null) added() }

    /** Signs a Mavio account in from where mavio.dev sent the browser back to. */
    suspend fun finishAppSignIn(signIn: BrowserSignIn, callback: String): Account =
        client.finishAppSignIn(signIn, callback).also { added() }

    /** Signs out of the Mavio account and its servers, or of every server. */
    suspend fun signOut() {
        client.signOut()
        _states.value = emptyMap()
        _home.value = emptyList()
        reloadServers()
    }

    /**
     * Takes the scan progress of a server's libraries as listed, which the
     * server's events then carry on.
     */
    fun listedScans(serverId: String, libraries: List<mavio.library.v1.LibraryOuterClass.Library>) {
        val scans = libraries.filter { it.hasScanProgress() }.associate { l ->
            val p = l.scanProgress
            l.id to ScanProgress(
                readingFolders = p.phase == mavio.library.v1.LibraryOuterClass.ScanPhase.SCAN_PHASE_FOLDERS,
                foldersDone = p.foldersDone,
                foldersTotal = p.foldersTotal,
                itemsDone = p.itemsDone,
                itemsTotal = p.itemsTotal,
                fraction = p.fraction,
            )
        }
        _scanProgress.update { it + (serverId to scans) }
    }

    /** Signs out of a server and forgets it. */
    suspend fun remove(serverId: String) {
        client.removeServer(serverId)
        _states.update { it - serverId }
        reloadServers()
        refreshHome()
    }

    /** Orders the servers as the user arranged them, which orders Home. */
    fun order(serverIds: List<String>) {
        runCatching { client.orderServers(serverIds) }
        reloadServers()
        refreshHome()
    }

    /** Marks an item played or unplayed. */
    suspend fun setPlayed(card: Card, played: Boolean) {
        client.setPlayed(card.serverId, card.itemId, played)
        bump(card.serverId)
    }

    /** Marks an item a favorite or not. */
    suspend fun setFavorite(card: Card, favorite: Boolean) {
        client.setFavorite(card.serverId, card.itemId, favorite)
        bump(card.serverId)
    }

    /** Starts playing an item with what the device plays. */
    suspend fun play(serverId: String, itemId: String, options: PlaybackOptions): PlaybackSession =
        client.play(serverId, itemId, DeviceProfile.capabilities(), options)

    /**
     * Stops a playback at position, unset when played to the end, even as
     * the player goes, and loads the server's items again.
     */
    fun finish(serverId: String, session: PlaybackSession, position: java.time.Duration?) {
        scope.launch {
            runCatching { session.stop(position) }
            bump(serverId)
        }
    }

    private fun added() {
        reloadServers()
        start()
        refreshHome()
    }

    private fun receive(push: Push) {
        when (push) {
            is Push.State -> {
                val was = _states.value[push.serverId]
                _states.update { it + (push.serverId to push.state) }
                if (push.state is StreamState.SignedOut) {
                    reloadServers()
                    refreshHome()
                } else if (push.state is StreamState.Online && was !is StreamState.Online) {
                    bump(push.serverId)
                }
            }
            is Push.Event -> when (push.event) {
                is ServerEvent.SessionsChanged -> Unit
                is ServerEvent.ScanProgressChanged -> _scanProgress.update {
                    val e = push.event
                    val scans = it[push.serverId].orEmpty()
                    it + (push.serverId to if (e.progress == null) scans - e.libraryId else scans + (e.libraryId to e.progress))
                }
                else -> bump(push.serverId)
            }
            is Push.Home -> _home.value = push.rows
            Push.ServersChanged -> {
                reloadServers()
                refreshHome()
            }
            is Push.Connected -> syncPlay.connected(push.serverId, push.sessionId)
            is Push.SyncPlayUpdate -> syncPlay.update(push.serverId, push.update)
        }
    }

    /** What the core pushed, relayed to the main thread. */
    private sealed interface Push {
        data class State(val serverId: String, val state: StreamState) : Push
        data class Event(val serverId: String, val event: ServerEvent) : Push
        data class Home(val rows: List<Row>) : Push
        data object ServersChanged : Push
        data class Connected(val serverId: String, val sessionId: String) : Push
        data class SyncPlayUpdate(val serverId: String, val update: mavio.session.v1.Syncplay.SyncPlayUpdate) : Push
    }

    /** Receives the core's callbacks on its threads and relays them. */
    private inner class Relay : CoreListener, HomeListener, SyncPlayListener {
        private fun push(push: Push) {
            scope.launch { receive(push) }
        }

        override fun serverState(serverId: String, state: StreamState) = push(Push.State(serverId, state))

        override fun event(serverId: String, event: ServerEvent) = push(Push.Event(serverId, event))

        override fun serversChanged() = push(Push.ServersChanged)

        override fun rows(rows: List<Row>) = push(Push.Home(rows))

        override fun connected(serverId: String, sessionId: String) = push(Push.Connected(serverId, sessionId))

        override fun update(serverId: String, update: ByteArray) {
            runCatching { mavio.session.v1.Syncplay.SyncPlayUpdate.parseFrom(update) }.onSuccess { push(Push.SyncPlayUpdate(serverId, it)) }
        }
    }

    private companion object {
        /** This device as the servers know it; its ID stays across launches. */
        fun device(context: Context, clientName: String): DeviceInfo {
            val prefs = context.getSharedPreferences("device", Context.MODE_PRIVATE)
            val id = prefs.getString("id", null) ?: UUID.randomUUID().toString().also {
                prefs.edit().putString("id", it).apply()
            }
            val name = Settings.Global.getString(context.contentResolver, Settings.Global.DEVICE_NAME)
                ?: Build.MODEL
            val version = context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: "0.0.0"
            return DeviceInfo(id, name, clientName, version)
        }
    }
}

/** Whether the server rejected the credentials or token. */
val Throwable.isUnauthenticated: Boolean get() = this is CoreException.Unauthenticated
