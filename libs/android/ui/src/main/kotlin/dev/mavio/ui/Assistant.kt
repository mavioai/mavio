package dev.mavio.ui

import android.content.Context
import android.os.LocaleList
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.google.protobuf.Duration
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.StreamListener
import dev.mavio.core.VoiceSession
import dev.mavio.core.call
import java.util.TimeZone
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.assistant.v1.Assistant.AskContext
import mavio.assistant.v1.Assistant.AskPlaying
import mavio.assistant.v1.Assistant.AskRequest
import mavio.assistant.v1.Assistant.AskResponse
import mavio.assistant.v1.Assistant.ClientAction
import mavio.assistant.v1.Assistant.ConfirmationAnswer
import mavio.assistant.v1.Assistant.ControlCommand
import mavio.assistant.v1.Assistant.GetAssistantRequest
import mavio.assistant.v1.Assistant.GetAssistantResponse
import mavio.assistant.v1.Assistant.NavigateAction
import mavio.assistant.v1.Assistant.NavigateTarget
import mavio.assistant.v1.Assistant.PlayAction
import mavio.assistant.v1.Assistant.StartVoiceSessionRequest
import mavio.assistant.v1.Assistant.StartVoiceSessionResponse
import mavio.library.v1.ItemOuterClass.ImageKind
import mavio.library.v1.ItemOuterClass.Item
import mavio.library.v1.ItemOuterClass.ItemKind
import mavio.plugin.v1.Voice.VoiceMode
import org.json.JSONArray
import org.json.JSONObject

/** The player open, as the assistant controls it (docs/assistant.md §5.2). */
interface AssistantPlayer {
    /** What it plays, for the requests' context. */
    val playing: AskPlaying

    /** Carries out a command, value as `ControlAction` gives it; false when the player cannot. */
    fun control(command: ControlCommand, value: Double): Boolean

    /** Picks streams by their index in the media, -1 for no subtitles; null keeps one. False when the player cannot. */
    fun streams(audio: Int?, subtitle: Int?): Boolean
}

/**
 * What the app shows, as the assistant's requests tell it: the screen, as
 * `AskContext` names it ("home", "item", "library", "search", "settings",
 * "player"…), and the server, item and library it shows, if any.
 */
data class AssistantScreen(
    val name: String,
    val serverId: String? = null,
    val itemId: String = "",
    val libraryId: String = "",
)

/** The app around the assistant: what it shows and plays, and the client actions it carries out. */
interface AssistantHost {
    /** The screen shown. */
    val screen: AssistantScreen

    /** Plays the action's items; false when the app cannot. */
    fun play(action: PlayAction, serverId: String): Boolean

    /** Opens the action's screen; false when the app cannot. */
    fun navigate(action: NavigateAction, serverId: String): Boolean
}

/** The orb's states, taken from the voice session's events (libs/design/components.md, Assistant). */
enum class OrbMode { OPENING, IDLE, HEARING, THINKING, SPEAKING, RESTING, ERROR, ENDED }

/**
 * The assistant over the app (docs/assistant.md §9.1, §9.4;
 * libs/design/components.md, Assistant), as the web UI's and the Apple
 * apps': an orb floating on its own shows the voice session's state, and
 * cards beside it what the user said, the answer with its words lit as they
 * are spoken, the tools run, results to choose from and a pending
 * confirmation. The assistant button, or the orb, tapped starts a voice
 * session and tapped again ends it; held, or the remote's key held, it
 * talks while held. The app carries out the session's client actions. It
 * goes away on its own a few seconds after an answer with nothing to
 * choose, or after the session ends; and when it is dismissed. Its state
 * changes on the main thread.
 */
@Stable
class Assistant(private val model: AppModel, context: Context) {
    /** A tool run or running. */
    data class Activity(val id: String, val summary: String, val done: Boolean, val failed: Boolean)

    /** A confirmation the answer waits for. */
    data class Confirmation(val id: String, val text: String)

    /** A result, as its row shows it. */
    data class Result(val id: String, val name: String, val kind: ItemKind, val poster: String?, val subtitle: String?)

    private val context = context.applicationContext
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    /** Whether the orb and cards show. */
    var open by mutableStateOf(false)
        private set

    /** What the user said; partial while they speak. */
    var heard by mutableStateOf("")
        private set

    /** Whether what the user said is final. */
    var heardFinal by mutableStateOf(false)
        private set

    /** A voice session runs. */
    var listening by mutableStateOf(false)
        private set

    /** The microphone's audio has started coming. */
    var capturing by mutableStateOf(false)
        private set

    /** The answer is being worked out. */
    var working by mutableStateOf(false)
        private set

    /** The tools running or run. */
    var activity by mutableStateOf<List<Activity>>(emptyList())
        private set

    /** The answer, as it comes. */
    var answer by mutableStateOf("")
        private set

    /** Whether the answer ended. */
    var answered by mutableStateOf(false)
        private set

    /** Whether the answer is being spoken. */
    var speaking by mutableStateOf(false)
        private set

    /** Whether the answer comes with speech, whose playing lights its words. */
    var voiced by mutableStateOf(false)
        private set

    /** The user speaks: from user_started to user_stopped. */
    var userSpeaking by mutableStateOf(false)
        private set

    /** The session listens only while held. */
    var pushToTalk by mutableStateOf(false)
        private set

    /** Whether the user holds to talk now. */
    var talking by mutableStateOf(false)
        private set

    /** How many times the user barged in. */
    var interrupts by mutableIntStateOf(0)
        private set

    /** The orb and cards fade out. */
    var leaving by mutableStateOf(false)
        private set

    /** Results shown, to choose from when [choose] is set. */
    var items by mutableStateOf<List<Result>>(emptyList())
        private set

    /** Whether the results are to choose from. */
    var choose by mutableStateOf(false)
        private set

    /** The confirmation the answer waits for. */
    var confirmation by mutableStateOf<Confirmation?>(null)
        private set

    /** Why the assistant failed, as it shows. */
    var error by mutableStateOf<String?>(null)
        private set

    /** The server whose assistant answers, when it is on. */
    var server by mutableStateOf<ServerRecord?>(null)
        private set

    /** The app around the assistant. */
    var host: AssistantHost? = null

    /** The player open, if any; set by the player while it shows. */
    var player: AssistantPlayer? = null

    /** Asks the user for the microphone, answering whether it was allowed; set by the activity. */
    var askPermission: (suspend () -> Boolean)? = null

    private val infos = HashMap<String, GetAssistantResponse>()
    private val loading = HashSet<String>()
    private var voice: VoiceSession? = null
    private var running: Job? = null
    private var holdTimer: Job? = null
    private var held: Boolean? = null
    private var turn = ""
    private var lingerTimer: Job? = null
    private var leaveTimer: Job? = null
    private var failed = mutableListOf<String>()
    // Bumped by each capture, so that a session's late callbacks are dropped.
    private var generation = 0

    // MARK: The server

    /** The server the assistant asks: the screen's, else the first signed in to. */
    private val candidate: ServerRecord?
        get() {
            val signedIn = model.signedInServers
            val id = host?.screen?.serverId
            return signedIn.firstOrNull { it.id == id } ?: signedIn.firstOrNull()
        }

    /** Reads again whether the assistant is on for the server the app shows, as it changes or appears. */
    fun refresh() {
        val s = candidate
        if (s == null) {
            server = null
            return
        }
        infos[s.id]?.let { learned(s, it) }
        if (!loading.add(s.id)) return
        scope.launch {
            val request = GetAssistantRequest.newBuilder().setLanguage(uiLanguage()).build()
            val info = runCatching { model.client.call(s.id, procedure("GetAssistant"), request, GetAssistantResponse::parseFrom) }.getOrNull()
            loading.remove(s.id)
            if (info != null) infos[s.id] = info
            if (candidate?.id == s.id) infos[s.id]?.let { learned(s, it) }
        }
    }

    /** The server's assistant is known: on or off. A request waiting for it is made, or dropped. */
    private fun learned(s: ServerRecord, info: GetAssistantResponse) {
        server = if (info.enabled) s else null
        val r = requested ?: return
        requested = null
        if (info.enabled) request(r)
    }

    // What the system asked for before the server's assistant was known.
    private var requested: String? = null

    /**
     * What the system asks, as the app opens: spoken text from its voice
     * search, or empty to listen, from the launcher's shortcut or the Quick
     * Settings tile; made once the server's assistant is known to be on.
     */
    fun request(spoken: String) {
        if (server == null) {
            requested = spoken
            refresh()
            return
        }
        if (spoken.isNotEmpty()) {
            ask(spoken)
        } else if (!listening) {
            tap()
        }
    }

    private val info: GetAssistantResponse? get() = server?.let { infos[it.id] }

    // MARK: Pressing

    /** The button, orb or key went down. */
    fun press() {
        // A conversation ends when pressed again.
        voice?.let {
            if (!it.holdsToTalk) {
                close()
                return
            }
        }
        held = false
        holdTimer?.cancel()
        holdTimer = scope.launch {
            delay(HOLD_MS)
            held = true
            val s = voice
            if (s != null) {
                if (s.holdsToTalk) {
                    s.talk(true)
                    talking = true
                } else {
                    connect(pushToTalk = true)
                }
            }
        }
        if (voice == null) capture()
    }

    /** The button, orb or key went up. */
    fun release() {
        val h = held ?: return
        held = null
        holdTimer?.cancel()
        val v = voice
        when {
            h -> {
                v?.talk(false)
                talking = false
            }
            // A tap: a conversation starts, or a session held to talk ends.
            v?.holdsToTalk == true -> close()
            v != null -> connect(pushToTalk = false)
        }
    }

    /** A tap, as a remote's press: starts or ends a conversation. */
    fun tap() {
        press()
        release()
    }

    /** Ends the session, and the orb and cards fade out. */
    fun close() {
        lingerTimer?.cancel()
        running?.cancel()
        running = null
        holdTimer?.cancel()
        held = null
        generation++
        voice?.close()
        voice = null
        if (!open) return
        leaving = true
        listening = false
        speaking = false
        userSpeaking = false
        leaveTimer?.cancel()
        leaveTimer = scope.launch {
            delay(LEAVE_MS)
            reset()
        }
    }

    private fun reset() {
        open = false
        heard = ""
        heardFinal = false
        listening = false
        capturing = false
        working = false
        activity = emptyList()
        answer = ""
        answered = false
        speaking = false
        voiced = false
        userSpeaking = false
        pushToTalk = false
        talking = false
        interrupts = 0
        leaving = false
        items = emptyList()
        choose = false
        confirmation = null
        error = null
    }

    /** Holds the assistant while a card is touched. */
    fun hold() {
        lingerTimer?.cancel()
    }

    /** Lets the assistant go away again once the card is let go. */
    fun letGo() {
        if (voice == null && !working) lingerThenClose()
    }

    private fun lingerThenClose() {
        lingerTimer?.cancel()
        lingerTimer = scope.launch {
            delay(LINGER_MS)
            if (!listening && !working && confirmation == null && !choose && error == null) close()
        }
    }

    private fun show() {
        lingerTimer?.cancel()
        leaveTimer?.cancel()
        open = true
        leaving = false
    }

    // MARK: The orb

    /** The orb's state, from the session's. */
    val mode: OrbMode
        get() = when {
            leaving -> OrbMode.ENDED
            error != null -> OrbMode.ERROR
            speaking -> OrbMode.SPEAKING
            userSpeaking -> OrbMode.HEARING
            working -> OrbMode.THINKING
            listening && !capturing -> OrbMode.OPENING
            listening -> if (pushToTalk && !talking) OrbMode.RESTING else OrbMode.IDLE
            else -> OrbMode.RESTING
        }

    /** What the pill beside the orb says, if anything; phones show none. */
    val status: String?
        get() = when {
            mode == OrbMode.OPENING -> context.getString(R.string.state_loading)
            mode == OrbMode.IDLE && heard.isEmpty() -> context.getString(R.string.assistant_listening)
            else -> null
        }

    /** The audio the orb follows: the playback's while the answer is spoken, else the microphone's. */
    fun level(): Double {
        val v = voice ?: return 0.0
        return if (speaking) v.outputLevel() else v.inputLevel()
    }

    /** How much of the answer's speech has played, in milliseconds. */
    fun played(): Double = voice?.speech()?.first ?: 0.0

    /** Whether the answer's words are lit as they are spoken. */
    val answerLive: Boolean get() = voiced && (speaking || working || !answered)

    // MARK: Answering

    /** Answers the pending confirmation by its buttons. */
    fun confirm(confirmed: Boolean) {
        val c = confirmation ?: return
        ask(AskRequest.newBuilder().setAnswer(ConfirmationAnswer.newBuilder().setId(c.id).setConfirmed(confirmed)), choosing = false)
    }

    /** Chooses a result, or opens it when there is nothing to choose. */
    fun choose(item: Result) {
        val s = server ?: return
        if (choose) {
            ask(AskRequest.newBuilder().setChoice(item.id), choosing = true)
        } else {
            host?.navigate(NavigateAction.newBuilder().setTarget(NavigateTarget.NAVIGATE_TARGET_ITEM).setId(item.id).build(), s.id)
        }
    }

    /**
     * Asks with text the system's voice search gave (docs/assistant.md
     * §9.6), shown as a voice session's turn; false while no server's
     * assistant is on.
     */
    fun ask(spoken: String): Boolean {
        if (spoken.isBlank() || server == null) return false
        close()
        leaveTimer?.cancel()
        reset()
        heard = spoken
        heardFinal = true
        ask(AskRequest.newBuilder().setText(spoken).setSpoken(true), choosing = false)
        return true
    }

    /** Tells the session what the app shows, as it changes. */
    fun contextChanged() {
        val v = voice ?: return
        v.send(JSONObject().put("type", "context").put("context", contextJson(context())))
    }

    /** What the app shows and plays, as the requests' context. */
    private fun context(): AskContext {
        val screen = host?.screen ?: AssistantScreen("")
        return AskContext.newBuilder()
            .addAllLanguages(listOf(speechLanguage(), uiLanguage()).distinct())
            .setScreen(screen.name)
            .setItemId(screen.itemId)
            .setLibraryId(screen.libraryId)
            .setTimeZone(TimeZone.getDefault().id)
            .apply { player?.let { setPlaying(it.playing) } }
            .build()
    }

    private fun ask(request: AskRequest.Builder, choosing: Boolean) {
        val s = server ?: return
        show()
        running?.cancel()
        voiced = false
        working = true
        activity = emptyList()
        answer = ""
        answered = false
        error = null
        confirmation = null
        if (choosing) {
            items = emptyList()
            choose = false
        }
        request.setContext(context()).addAllFailedActions(failed)
        failed = mutableListOf()
        val body = request.build().toByteArray()
        running = scope.launch {
            // The core's thread hands each response over in order.
            val events = Channel<AskResponse>(Channel.UNLIMITED)
            launch(Dispatchers.Default) {
                try {
                    model.client.callServerStream(
                        s.id,
                        procedure("Ask"),
                        body,
                        object : StreamListener {
                            override fun message(body: ByteArray) {
                                runCatching { AskResponse.parseFrom(body) }.onSuccess { events.trySend(it) }
                            }
                        },
                    )
                    events.close()
                } catch (e: Exception) {
                    events.close(e)
                }
            }
            // Whether the answer waits for the user: a confirmation, or results to choose from.
            var waits = false
            try {
                for (e in events) {
                    when (e.eventCase) {
                        AskResponse.EventCase.UNDERSTOOD -> if (e.understood.text.isNotEmpty()) heard = e.understood.text
                        AskResponse.EventCase.TOOL -> e.tool.let { tool(Activity(it.id, it.summary, it.done, it.failed)) }
                        AskResponse.EventCase.ACTION -> {
                            val a = e.action
                            if (!perform(a, s)) failed += a.id
                            if (a.kindCase == ClientAction.KindCase.SHOW_ITEMS && a.showItems.choose) waits = true
                        }
                        AskResponse.EventCase.CONFIRMATION -> {
                            waits = true
                            confirmation = Confirmation(e.confirmation.id, e.confirmation.text)
                        }
                        AskResponse.EventCase.ANSWER -> answer += e.answer
                        AskResponse.EventCase.DONE -> if (e.done.answer.isNotEmpty()) answer = e.done.answer
                        else -> Unit
                    }
                }
                working = false
                answered = true
                if (failed.isNotEmpty() && answer.isEmpty()) error = context.getString(R.string.assistant_cant_do)
                if (!waits) lingerThenClose()
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                working = false
                error = e.message ?: e.toString()
            }
        }
    }

    private fun tool(t: Activity) {
        activity = activity.filter { it.id != t.id } + t
    }

    /** Carries out a client action; false when the app cannot. */
    private fun perform(a: ClientAction, s: ServerRecord): Boolean = when (a.kindCase) {
        ClientAction.KindCase.PLAY -> host?.play(a.play, s.id) ?: false
        ClientAction.KindCase.CONTROL -> player?.control(a.control.command, a.control.value) ?: false
        // The players shift no subtitles.
        ClientAction.KindCase.STREAMS -> a.streams.let { st ->
            val p = player
            if (p == null || st.subtitleOffset != 0.0) {
                false
            } else {
                p.streams(if (st.hasAudioStreamIndex()) st.audioStreamIndex else null, if (st.hasSubtitleStreamIndex()) st.subtitleStreamIndex else null)
            }
        }
        ClientAction.KindCase.NAVIGATE -> host?.navigate(a.navigate, s.id) ?: false
        ClientAction.KindCase.SHOW_ITEMS -> {
            items = a.showItems.itemsList.map { result(it, s) }
            choose = a.showItems.choose
            true
        }
        else -> false
    }

    // MARK: Voice

    /** Opens the microphone, and the voice session once it is known whether the user holds to talk. */
    private fun capture() {
        val s = server ?: return
        show()
        running?.cancel()
        if (info?.voice != true) {
            error = context.getString(R.string.assistant_no_voice)
            return
        }
        turn = ""
        listening = true
        capturing = false
        userSpeaking = false
        speaking = false
        voiced = false
        pushToTalk = false
        talking = false
        heard = ""
        heardFinal = false
        answer = ""
        answered = false
        activity = emptyList()
        error = null
        if (!choose) items = emptyList()
        val gen = ++generation
        scope.launch {
            if (!VoiceSession.permitted(context) && askPermission?.invoke() != true) {
                if (gen != generation) return@launch
                listening = false
                error = context.getString(R.string.assistant_microphone_denied)
                return@launch
            }
            if (gen != generation || !listening || leaving) return@launch
            val v = try {
                VoiceSession.capture(
                    context,
                    VoiceSession.Handlers(
                        event = { if (gen == generation) voiceEvent(it, s) },
                        closed = { unreachable, e ->
                            if (gen == generation) {
                                voice = null
                                listening = false
                                capturing = false
                                speaking = false
                                userSpeaking = false
                                working = false
                                when {
                                    unreachable -> error = context.getString(R.string.home_server_unreachable, s.name)
                                    e != null -> error = e
                                    else -> lingerThenClose()
                                }
                            }
                        },
                        capturing = { if (gen == generation) capturing = true },
                        silent = { device ->
                            if (gen == generation) {
                                error = context.getString(R.string.assistant_silent_microphone, device.ifEmpty { context.getString(R.string.assistant_microphone) })
                            }
                        },
                    ),
                )
            } catch (e: VoiceSession.Failure.Denied) {
                listening = false
                error = context.getString(R.string.assistant_microphone_denied)
                return@launch
            } catch (e: Exception) {
                listening = false
                error = context.getString(R.string.assistant_no_microphone, e.message ?: e.toString())
                return@launch
            }
            voice = v
            // Let go before the microphone opened: a tap.
            when (held) {
                null -> connect(pushToTalk = false)
                true -> connect(pushToTalk = true)
                false -> Unit
            }
        }
    }

    /** Starts the session on the server: a conversation, or talking while held. */
    private fun connect(pushToTalk: Boolean) {
        val v = voice ?: return
        val s = server ?: return
        val request = StartVoiceSessionRequest.newBuilder()
            .setLanguage(speechLanguage())
            .setMode(if (pushToTalk) VoiceMode.VOICE_MODE_PUSH_TO_TALK else VoiceMode.VOICE_MODE_CONVERSATION)
            .setContext(context())
            .build()
        v.connect(pushToTalk) {
            val r = model.client.call(s.id, procedure("StartVoiceSession"), request, StartVoiceSessionResponse::parseFrom)
            socketUrl(r.url, s)
        }
        this.pushToTalk = pushToTalk
        talking = pushToTalk
        if (pushToTalk) v.talk(true)
    }

    /** Carries out what the voice session tells (docs/assistant.md §8.6). */
    private fun voiceEvent(e: JSONObject, s: ServerRecord) {
        val text = e.optString("text")
        when (e.optString("type")) {
            "user_started" -> {
                lingerTimer?.cancel()
                userSpeaking = true
            }
            "user_stopped" -> userSpeaking = false
            "interrupt" -> {
                interrupts++
                speaking = false
                userSpeaking = true
            }
            "transcript" -> {
                val t = text.trim()
                val final = e.optBoolean("final")
                if (final) turn = listOf(turn, t).filter { it.isNotEmpty() }.joinToString(" ")
                heard = if (final) turn else listOf(turn, t).filter { it.isNotEmpty() }.joinToString(" ")
                heardFinal = false
            }
            "run" -> {
                turn = ""
                if (text.isNotEmpty()) heard = text
                heardFinal = true
                working = true
                userSpeaking = false
                voiced = false
                activity = emptyList()
                answer = ""
                answered = false
                error = null
                confirmation = null
                if (!choose) items = emptyList()
            }
            "intent" -> if (text.isNotEmpty()) heard = text
            "tool" -> tool(Activity(e.opt("id")?.toString().orEmpty(), e.optString("summary"), e.optBoolean("done"), e.optBoolean("failed")))
            "action" -> e.optJSONObject("action")?.let { json -> actionOf(json)?.let { perform(it, s) } }
            "confirm" -> confirmation = Confirmation(e.opt("id")?.toString().orEmpty(), text)
            "answer" -> answer += text
            "end" -> {
                working = false
                e.optString("answer").takeIf { it.isNotEmpty() }?.let { answer = it }
                answered = true
                if (e.optString("status") == "failed" && answer.isEmpty()) error = context.getString(R.string.assistant_cant_do)
            }
            "bot_started" -> {
                speaking = true
                voiced = true
            }
            "bot_stopped" -> speaking = false
            "error" -> {
                working = false
                error = e.optString("message").ifEmpty { e.opt("code")?.toString().orEmpty() }
            }
        }
    }

    /** The UI's language, as the app's strings were chosen in. */
    private fun uiLanguage(): String = context.resources.configuration.locales[0].toLanguageTag()

    /** The language spoken to the assistant: the user's of the UI's language, else the UI's. */
    private fun speechLanguage(): String {
        val ui = uiLanguage()
        val primary = { l: String -> l.substringBefore('-') }
        val system = LocaleList.getDefault()
        return List(system.size(), system::get).map { it.toLanguageTag() }.firstOrNull { primary(it) == primary(ui) } ?: ui
    }

    private fun result(item: Item, s: ServerRecord): Result {
        val image = item.imagesList.filter { it.kind == ImageKind.IMAGE_KIND_PRIMARY && it.url.isNotEmpty() }.minByOrNull { it.index }
        return Result(item.id, item.name, item.kind, image?.url?.let { absolute(it, s) + "?maxWidth=$POSTER_WIDTH" }, subtitle(item))
    }

    companion object {
        /** How long the button or key must be held to talk while held, rather than start a conversation. */
        private const val HOLD_MS = 400L

        /** How long an answer with nothing to choose stays before the assistant goes away. */
        private const val LINGER_MS = 6000L

        /** How long the orb and cards take to fade out. */
        const val LEAVE_MS = 400L

        // The results' posters, in pixels, twice their 34 points and some.
        private const val POSTER_WIDTH = 120

        private fun procedure(name: String) = "/mavio.assistant.v1.AssistantService/$name"

        private fun absolute(path: String, s: ServerRecord): String =
            if (path.startsWith("http")) path else s.address.trimEnd('/') + "/" + path.trimStart('/')

        /** The session's WebSocket URL, from the path the server gives. */
        internal fun socketUrl(path: String, s: ServerRecord): String {
            val url = if (path.startsWith("http") || path.startsWith("ws")) path else absolute(path, s)
            return when {
                url.startsWith("https:") -> "wss:" + url.removePrefix("https:")
                url.startsWith("http:") -> "ws:" + url.removePrefix("http:")
                else -> url
            }
        }

        /** What a result's row says under its title: the series and episode, the artists, or the year. */
        internal fun subtitle(item: Item): String? = when (item.kind) {
            ItemKind.ITEM_KIND_EPISODE -> {
                val n = when {
                    item.parentIndexNumber > 0 && item.indexNumber > 0 -> "S${item.parentIndexNumber}:E${item.indexNumber}"
                    item.indexNumber > 0 -> "E${item.indexNumber}"
                    else -> ""
                }
                listOf(item.seriesName, n).filter { it.isNotEmpty() }.joinToString(" · ").ifEmpty { null }
            }
            ItemKind.ITEM_KIND_SEASON -> item.seriesName.ifEmpty { null }
            ItemKind.ITEM_KIND_MUSIC_ALBUM, ItemKind.ITEM_KIND_TRACK ->
                (item.albumArtistsList.ifEmpty { item.artistsList }).joinToString(", ").ifEmpty { null }
            else -> if (item.productionYear > 0) item.productionYear.toString() else null
        }

        /**
         * What a player plays, as the requests' context tells it: the
         * streams by their index in the media, null for none.
         */
        fun playing(
            itemId: String,
            positionMs: Long,
            paused: Boolean,
            speed: Double,
            audio: Int?,
            subtitle: Int?,
            volume: Int = 100,
            muted: Boolean = false,
        ): AskPlaying {
            val ms = positionMs.coerceAtLeast(0)
            return AskPlaying.newBuilder()
                .setItemId(itemId)
                .setPosition(Duration.newBuilder().setSeconds(ms / 1000).setNanos((ms % 1000).toInt() * 1_000_000))
                .setPaused(paused)
                .setSpeed(speed)
                .setAudioStreamIndex(audio ?: -1)
                .setSubtitleStreamIndex(subtitle ?: -1)
                .setVolume(volume)
                .setMuted(muted)
                .build()
        }

        /** The context as the voice session's JSON carries it, protobuf's JSON mapping. */
        internal fun contextJson(c: AskContext): JSONObject = JSONObject().apply {
            put("languages", JSONArray(c.languagesList))
            if (c.screen.isNotEmpty()) put("screen", c.screen)
            if (c.itemId.isNotEmpty()) put("itemId", c.itemId)
            if (c.libraryId.isNotEmpty()) put("libraryId", c.libraryId)
            if (c.timeZone.isNotEmpty()) put("timeZone", c.timeZone)
            if (c.hasPlaying()) {
                val p = c.playing
                put(
                    "playing",
                    JSONObject().apply {
                        put("itemId", p.itemId)
                        put("position", durationJson(p.position))
                        put("paused", p.paused)
                        put("speed", p.speed)
                        put("audioStreamIndex", p.audioStreamIndex)
                        put("subtitleStreamIndex", p.subtitleStreamIndex)
                        put("volume", p.volume)
                        put("muted", p.muted)
                    },
                )
            }
        }

        // A duration as JSON gives it: seconds and an "s", such as "61.5s".
        private fun durationJson(d: Duration): String =
            d.seconds.toBigDecimal().add(d.nanos.toBigDecimal().movePointLeft(9)).stripTrailingZeros().toPlainString() + "s"

        /** A client action from the voice session's JSON, protobuf's JSON mapping; null when it has none. */
        internal fun actionOf(json: JSONObject): ClientAction? {
            val b = ClientAction.newBuilder().setId(json.optString("id"))
            json.optJSONObject("play")?.let { p ->
                b.setPlay(
                    PlayAction.newBuilder().apply {
                        p.optJSONArray("itemIds")?.let { a -> for (i in 0 until a.length()) addItemIds(a.getString(i)) }
                        setStartIndex(p.optInt("startIndex"))
                        p.optString("startPosition").takeIf { it.isNotEmpty() }?.let { setStartPosition(duration(it)) }
                        if (p.has("audioStreamIndex")) setAudioStreamIndex(p.getInt("audioStreamIndex"))
                        if (p.has("subtitleStreamIndex")) setSubtitleStreamIndex(p.getInt("subtitleStreamIndex"))
                    },
                )
            }
            json.optJSONObject("control")?.let { c ->
                b.setControl(
                    mavio.assistant.v1.Assistant.ControlAction.newBuilder()
                        .setCommand(enumOf(c.opt("command"), ControlCommand::valueOf, ControlCommand::forNumber) ?: ControlCommand.CONTROL_COMMAND_UNSPECIFIED)
                        .setValue(c.optDouble("value", 0.0)),
                )
            }
            json.optJSONObject("streams")?.let { st ->
                b.setStreams(
                    mavio.assistant.v1.Assistant.StreamsAction.newBuilder().apply {
                        if (st.has("audioStreamIndex")) setAudioStreamIndex(st.getInt("audioStreamIndex"))
                        if (st.has("subtitleStreamIndex")) setSubtitleStreamIndex(st.getInt("subtitleStreamIndex"))
                        setSubtitleOffset(st.optDouble("subtitleOffset", 0.0))
                    },
                )
            }
            json.optJSONObject("navigate")?.let { n ->
                b.setNavigate(
                    NavigateAction.newBuilder()
                        .setTarget(enumOf(n.opt("target"), NavigateTarget::valueOf, NavigateTarget::forNumber) ?: NavigateTarget.NAVIGATE_TARGET_UNSPECIFIED)
                        .setId(n.optString("id"))
                        .setQuery(n.optString("query")),
                )
            }
            json.optJSONObject("showItems")?.let { s ->
                b.setShowItems(
                    mavio.assistant.v1.Assistant.ShowItemsAction.newBuilder().apply {
                        s.optJSONArray("items")?.let { a -> for (i in 0 until a.length()) a.optJSONObject(i)?.let { addItems(itemOf(it)) } }
                        setChoose(s.optBoolean("choose"))
                    },
                )
            }
            return b.build().takeIf { it.kindCase != ClientAction.KindCase.KIND_NOT_SET }
        }

        /** An item from its JSON, with what the results' rows show. */
        private fun itemOf(j: JSONObject): Item = Item.newBuilder().apply {
            id = j.optString("id")
            name = j.optString("name")
            kind = enumOf(j.opt("kind"), ItemKind::valueOf, ItemKind::forNumber) ?: ItemKind.ITEM_KIND_UNSPECIFIED
            seriesName = j.optString("seriesName")
            indexNumber = j.optInt("indexNumber")
            parentIndexNumber = j.optInt("parentIndexNumber")
            productionYear = j.optInt("productionYear")
            j.optJSONArray("artists")?.let { a -> for (i in 0 until a.length()) addArtists(a.getString(i)) }
            j.optJSONArray("albumArtists")?.let { a -> for (i in 0 until a.length()) addAlbumArtists(a.getString(i)) }
            j.optJSONArray("images")?.let { a ->
                for (i in 0 until a.length()) {
                    val m = a.optJSONObject(i) ?: continue
                    addImages(
                        mavio.library.v1.ItemOuterClass.Image.newBuilder()
                            .setKind(enumOf(m.opt("kind"), ImageKind::valueOf, ImageKind::forNumber) ?: ImageKind.IMAGE_KIND_UNSPECIFIED)
                            .setIndex(m.optInt("index"))
                            .setUrl(m.optString("url")),
                    )
                }
            }
        }.build()

        // An enum by its name, or by its number as JSON may give it.
        private fun <E> enumOf(v: Any?, byName: (String) -> E, byNumber: (Int) -> E?): E? = when (v) {
            is Number -> byNumber(v.toInt())
            is String -> runCatching { byName(v) }.getOrNull()
            else -> null
        }

        // A duration as JSON gives it: seconds and an "s", such as "61.5s".
        private fun duration(s: String): Duration {
            val seconds = s.removeSuffix("s").toBigDecimalOrNull() ?: return Duration.getDefaultInstance()
            val whole = seconds.toLong()
            val nanos = seconds.subtract(whole.toBigDecimal()).movePointRight(9).toInt()
            return Duration.newBuilder().setSeconds(whole).setNanos(nanos).build()
        }
    }
}

/**
 * The answer's words, each with when it has been said, roughly, in
 * milliseconds from the speech's start: they are lit against how much of
 * the speech has played.
 */
object SpokenText {
    /** The words and the spaces between them of text, each with when it has been said. */
    fun segments(text: String): List<Pair<String, Double>> {
        val words = java.text.BreakIterator.getWordInstance()
        words.setText(text)
        val out = ArrayList<Pair<String, Double>>()
        var ms = 0.0
        var start = words.first()
        var end = words.next()
        while (end != java.text.BreakIterator.DONE) {
            val part = text.substring(start, end)
            ms += sayMs(part)
            out += part to ms
            start = end
            end = words.next()
        }
        return out
    }

    // How long a part takes to say, roughly.
    private fun sayMs(s: String): Double {
        var ms = 0.0
        var i = 0
        while (i < s.length) {
            val c = s.codePointAt(i)
            i += Character.charCount(c)
            ms += when {
                cjk(c) -> 230.0
                Character.isLetterOrDigit(c) -> 62.0
                ",.;:!?，。；：！？、".contains(String(Character.toChars(c))) -> 180.0
                else -> 0.0
            }
        }
        return ms
    }

    private fun cjk(c: Int): Boolean =
        c in 0x3040..0x30FF || c in 0x3400..0x4DBF || c in 0x4E00..0x9FFF || c in 0xAC00..0xD7AF || c in 0xF900..0xFAFF || c in 0x20000..0x2FFFF
}
