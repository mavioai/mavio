package dev.mavio.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.media.AudioAttributes
import android.media.MediaPlayer
import android.net.Uri
import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
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
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.assistant.v1.Assistant.AssistantConversation
import mavio.assistant.v1.Assistant.AssistantMemory
import mavio.assistant.v1.Assistant.AssistantMessage
import mavio.assistant.v1.Assistant.DeleteAssistantConversationRequest
import mavio.assistant.v1.Assistant.DeleteAssistantConversationResponse
import mavio.assistant.v1.Assistant.DeleteAssistantMemoriesRequest
import mavio.assistant.v1.Assistant.DeleteAssistantMemoriesResponse
import mavio.assistant.v1.Assistant.GetAssistantConversationRequest
import mavio.assistant.v1.Assistant.GetAssistantConversationResponse
import mavio.assistant.v1.Assistant.GetAssistantRequest
import mavio.assistant.v1.Assistant.GetAssistantResponse
import mavio.assistant.v1.Assistant.ListAssistantConversationsRequest
import mavio.assistant.v1.Assistant.ListAssistantConversationsResponse
import mavio.assistant.v1.Assistant.ListAssistantMemoriesRequest
import mavio.assistant.v1.Assistant.ListAssistantMemoriesResponse
import mavio.assistant.v1.Assistant.UpdateAssistantMemoryRequest
import mavio.assistant.v1.Assistant.UpdateAssistantMemoryResponse

private fun procedure(name: String) = "/mavio.assistant.v1.AssistantService/$name"

private val Throwable.why: String get() = message ?: toString()

/**
 * The Assistant settings of this device (docs/clients.md §6.3.29), as the
 * web UI's: the user's memories on the first server signed in to, each
 * forgotten from a menu of three dots, a new one remembered, all of them
 * forgotten after confirming; and the conversations, newest first, named by
 * when they were and the device they started on, each read or deleted:
 * under each message a button copying it, under an answer whose speech was
 * kept one playing it, and its time. A phone, a tablet and a television
 * show it alike, the television without copying.
 */
@Composable
fun AssistantSettings(model: AppModel) {
    val server = signedInServers(model).firstOrNull()
    val context = LocalContext.current
    val language = LocalConfiguration.current.locales[0].toLanguageTag()
    var info by remember(server?.id) { mutableStateOf<Result<GetAssistantResponse>?>(null) }
    LaunchedEffect(server?.id, server?.userId) {
        val s = server ?: return@LaunchedEffect
        val request = GetAssistantRequest.newBuilder().setLanguage(language).build()
        info = try {
            Result.success(model.client.call(s.id, procedure("GetAssistant"), request, GetAssistantResponse::parseFrom))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
    val state = remember(server?.id, server?.userId) { server?.let { AssistantPrefs(model, it, context) } }
    DisposableEffect(state) { onDispose { state?.kept?.stop() } }
    val on = info?.getOrNull()
    LaunchedEffect(state, on) {
        if (state == null || on == null || !on.enabled) return@LaunchedEffect
        launch { if (on.memory) state.loadMemories() }
        launch { if (on.ai) state.loadConversations() }
    }
    SettingsList(stringResource(R.string.assistant_title), stringResource(R.string.assistant_settings_message)) {
        val failure = info?.exceptionOrNull()
        when {
            server == null || state == null -> {}
            failure != null -> item { Text(failure.why, style = theme.type.callout.text, color = theme.colors.danger) }
            on == null -> item { StatusView(stringResource(R.string.state_loading)) }
            !on.enabled -> item { Group { ValueRow(stringResource(R.string.assistant_off), "") } }
            else -> {
                item { Memories(state, on) }
                if (on.ai) item { Conversations(state) }
            }
        }
    }
}

/** The memories and conversations of the user on a server, as the page reads and changes them. */
private class AssistantPrefs(val model: AppModel, val server: ServerRecord, context: Context) {
    var memories by mutableStateOf<List<AssistantMemory>?>(null)
    var memoryFailure by mutableStateOf<String?>(null)
    var conversations by mutableStateOf<List<AssistantConversation>?>(null)
    var conversationsFailure by mutableStateOf<String?>(null)
    var shown by mutableStateOf<String?>(null)
    var messages by mutableStateOf<List<AssistantMessage>?>(null)
    var messagesFailure by mutableStateOf<String?>(null)
    var busy by mutableStateOf(false)
    val kept = KeptSpeech(model, server, context)

    suspend fun loadMemories() {
        try {
            memories = model.client.call(server.id, procedure("ListAssistantMemories"), ListAssistantMemoriesRequest.getDefaultInstance(), ListAssistantMemoriesResponse::parseFrom).memoriesList
            memoryFailure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            memoryFailure = e.why
        }
    }

    suspend fun loadConversations() {
        try {
            conversations = model.client.call(
                server.id,
                procedure("ListAssistantConversations"),
                ListAssistantConversationsRequest.getDefaultInstance(),
                ListAssistantConversationsResponse::parseFrom,
            ).conversationsList
            conversationsFailure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            conversationsFailure = e.why
        }
    }

    /** Remembers text, answering whether it did. */
    suspend fun remember(text: String): Boolean = change {
        model.client.call(server.id, procedure("UpdateAssistantMemory"), UpdateAssistantMemoryRequest.newBuilder().setText(text).build(), UpdateAssistantMemoryResponse::parseFrom)
        loadMemories()
    }

    /** Forgets the memories of ids, every one for none. */
    suspend fun forget(ids: List<String>) {
        change {
            model.client.call(server.id, procedure("DeleteAssistantMemories"), DeleteAssistantMemoriesRequest.newBuilder().addAllIds(ids).build(), DeleteAssistantMemoriesResponse::parseFrom)
            loadMemories()
        }
    }

    private suspend fun change(work: suspend () -> Unit): Boolean {
        busy = true
        memoryFailure = null
        return try {
            work()
            true
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            memoryFailure = e.why
            false
        } finally {
            busy = false
        }
    }

    /** Opens the conversation of id, or closes it when it is open. */
    suspend fun toggle(id: String) {
        kept.stop()
        if (shown == id) {
            shown = null
            return
        }
        shown = id
        messages = null
        messagesFailure = null
        try {
            val r = model.client.call(
                server.id,
                procedure("GetAssistantConversation"),
                GetAssistantConversationRequest.newBuilder().setId(id).build(),
                GetAssistantConversationResponse::parseFrom,
            )
            if (shown == id) messages = r.messagesList
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (shown == id) messagesFailure = e.why
        }
    }

    suspend fun delete(id: String) {
        busy = true
        try {
            model.client.call(server.id, procedure("DeleteAssistantConversation"), DeleteAssistantConversationRequest.newBuilder().setId(id).build(), DeleteAssistantConversationResponse::parseFrom)
            kept.stop()
            shown = null
            loadConversations()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            messagesFailure = e.why
        } finally {
            busy = false
        }
    }
}

@Composable
private fun Memories(state: AssistantPrefs, info: GetAssistantResponse) {
    val scope = rememberCoroutineScope()
    var text by remember { mutableStateOf("") }
    var asking by remember { mutableStateOf(false) }
    if (!info.memory) {
        Group(stringResource(R.string.assistant_memory)) {
            Note(stringResource(if (info.ai) R.string.assistant_memory_off else R.string.assistant_memory_no_ai))
        }
        return
    }
    if (asking) {
        ConfirmDialog(
            stringResource(R.string.assistant_forget_all_confirm),
            null,
            stringResource(R.string.assistant_forget_all),
            { scope.launch { state.forget(emptyList()) } },
        ) { asking = false }
    }
    val list = state.memories.orEmpty()
    Group(stringResource(R.string.assistant_memory), stringResource(R.string.assistant_memory_footer)) {
        list.forEachIndexed { i, m ->
            if (i > 0) RowDivider()
            Row(
                Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(start = rowPadding, end = MavioSpacing.s8, top = 8.dp, bottom = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RowLabel(m.text, m.updateTime.instant?.let(::dateTime), Modifier.weight(1f))
                More(listOf(stringResource(R.string.assistant_forget) to { scope.launch { state.forget(listOf(m.id)) } }), enabled = !state.busy)
            }
        }
        if (state.memories?.isEmpty() == true) Note(stringResource(R.string.assistant_no_memories))
        if (list.isNotEmpty() || state.memories?.isEmpty() == true) RowDivider()
        Row(
            Modifier.fillMaxWidth().padding(start = rowPadding, end = MavioSpacing.s8, top = 10.dp, bottom = 10.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            SettingsField(text, { text = it.take(1000) }, null, Modifier.weight(1f), placeholder = stringResource(R.string.assistant_new_memory))
            SettingsButton(
                stringResource(R.string.assistant_remember),
                { scope.launch { if (state.remember(text.trim())) text = "" } },
                kind = ButtonKind.Secondary,
                enabled = text.isNotBlank() && !state.busy,
            )
            if (list.isNotEmpty()) More(listOf(stringResource(R.string.assistant_forget_all) to { asking = true }), enabled = !state.busy)
        }
        state.memoryFailure?.let { Note(it, theme.colors.danger) }
    }
}

@Composable
private fun Conversations(state: AssistantPrefs) {
    val scope = rememberCoroutineScope()
    Group(stringResource(R.string.assistant_conversations), stringResource(R.string.assistant_conversations_footer)) {
        state.conversations.orEmpty().forEachIndexed { i, c ->
            if (i > 0) RowDivider()
            // The assistant's conversations have no titles: they go by when they were, and where.
            val time = c.updateTime.instant?.let(::dateTime).orEmpty()
            val detail = listOf(c.deviceName, if (c.title.isNotEmpty()) time else "").filter { it.isNotEmpty() }.joinToString(" · ")
            Pressable({ scope.launch { state.toggle(c.id) } }, Modifier.fillMaxWidth()) {
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(horizontal = rowPadding, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    RowLabel(c.title.ifEmpty { time }, detail.ifEmpty { null }, Modifier.weight(1f))
                }
            }
            if (state.shown == c.id) Messages(state, c.id)
        }
        if (state.conversations?.isEmpty() == true) Note(stringResource(R.string.assistant_no_conversations))
        state.conversationsFailure?.let { Note(it, theme.colors.danger) }
    }
}

/** A conversation's messages, and deleting it. */
@Composable
private fun Messages(state: AssistantPrefs, id: String) {
    val scope = rememberCoroutineScope()
    Column(
        Modifier.fillMaxWidth().padding(start = rowPadding, end = rowPadding, top = MavioSpacing.s4, bottom = MavioSpacing.s12),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        val messages = state.messages
        messages?.forEachIndexed { i, m ->
            val key = i.toString()
            Bubble(m, playing = state.kept.at == key, onPlay = m.speechUrl.takeIf { it.isNotEmpty() }?.let { url -> { state.kept.toggle(key, url) } })
        }
        if (messages == null && state.messagesFailure == null) {
            Box(Modifier.fillMaxWidth().heightIn(min = 32.dp), Alignment.Center) {
                CircularProgressIndicator(Modifier.size(20.dp), color = theme.colors.textSecondary, strokeWidth = 2.dp)
            }
        }
        state.messagesFailure?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
        SettingsButton(
            stringResource(R.string.assistant_delete_conversation),
            { scope.launch { state.delete(id) } },
            kind = ButtonKind.Secondary,
            enabled = !state.busy,
        )
    }
}

/**
 * A message: the user's on the right in the button's color, the answer's on
 * the left on the fill, each short of the other side; under it its bar. On
 * a television it takes focus, so that the page scrolls through the
 * conversation, and selecting an answer plays its kept speech.
 */
@Composable
private fun Bubble(m: AssistantMessage, playing: Boolean, onPlay: (() -> Unit)?) {
    val tv = LocalTelevision.current
    Box(Modifier.fillMaxWidth(), if (m.user) Alignment.CenterEnd else Alignment.CenterStart) {
        Column(
            Modifier.fillMaxWidth(0.85f),
            horizontalAlignment = if (m.user) Alignment.End else Alignment.Start,
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            val bubble = @Composable {
                Text(
                    m.text,
                    Modifier
                        .clip(RoundedCornerShape(14.dp))
                        .background(if (m.user) theme.colors.button else theme.colors.fill)
                        .padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s8),
                    style = theme.type.callout.text,
                    color = if (m.user) theme.colors.onButton else theme.colors.textPrimary,
                )
            }
            if (tv) Pressable({ onPlay?.invoke() }) { Box(Modifier.padding(4.dp)) { bubble() } } else bubble()
            MessageBar(m.text, m.time.instant, playing, onPlay)
        }
    }
}

/** Buttons copying a message and, given onPlay, playing it, then its time; a television copies nothing. */
@Composable
private fun MessageBar(text: String, at: Instant?, playing: Boolean, onPlay: (() -> Unit)?) {
    val context = LocalContext.current
    val tv = LocalTelevision.current
    var copied by remember { mutableStateOf(false) }
    LaunchedEffect(copied) {
        if (copied) {
            delay(2000)
            copied = false
        }
    }
    Row(horizontalArrangement = Arrangement.spacedBy(2.dp), verticalAlignment = Alignment.CenterVertically) {
        if (!tv) {
            BarButton(if (copied) R.drawable.ic_check else R.drawable.ic_copy, stringResource(if (copied) R.string.assistant_copied_message else R.string.assistant_copy_message)) {
                context.getSystemService(ClipboardManager::class.java)?.setPrimaryClip(ClipData.newPlainText(null, text))
                copied = true
            }
            if (onPlay != null) {
                BarButton(if (playing) R.drawable.ic_pause else R.drawable.ic_play, stringResource(if (playing) R.string.assistant_stop_answer else R.string.assistant_play_answer), onPlay)
            }
        } else if (playing) {
            MavioIcon(R.drawable.ic_pause, size = 18.dp, tint = theme.colors.textTertiary)
        }
        at?.let { Text(shortTime(it), Modifier.padding(horizontal = 4.dp), style = theme.type.caption.text, color = theme.colors.textTertiary) }
    }
}

@Composable
private fun BarButton(@DrawableRes icon: Int, label: String, onClick: () -> Unit) {
    Box(
        Modifier
            .size(32.dp)
            .clip(RoundedCornerShape(8.dp))
            .clickable(onClick = onClick)
            .semantics { contentDescription = label },
        Alignment.Center,
    ) {
        MavioIcon(icon, size = 16.dp, tint = theme.colors.textTertiary)
    }
}

/** A line of text in a group, in [color] or the secondary text's. */
@Composable
private fun Note(text: String, color: Color = theme.colors.textSecondary) {
    Text(text, Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 14.dp), style = theme.type.body.text, color = color)
}

/**
 * A button of three dots opening a menu of destructive items, each a label
 * and what it does: on a phone a dropdown, on a television a menu below it.
 */
@Composable
private fun More(items: List<Pair<String, () -> Unit>>, enabled: Boolean) {
    var open by remember { mutableStateOf(false) }
    val tv = LocalTelevision.current
    val label = stringResource(R.string.item_more)
    if (!tv) {
        Box {
            Box(
                Modifier
                    .size(36.dp)
                    .clip(CircleShape)
                    .clickable(enabled = enabled) { open = true }
                    .semantics { contentDescription = label },
                Alignment.Center,
            ) {
                MavioIcon(R.drawable.ic_more_vertical, size = 20.dp, tint = theme.colors.textSecondary)
            }
            DropdownMenu(open, onDismissRequest = { open = false }) {
                items.forEach { (text, run) ->
                    DropdownMenuItem(
                        text = { Text(text, style = theme.type.callout.text, color = theme.colors.danger) },
                        onClick = {
                            open = false
                            run()
                        },
                    )
                }
            }
        }
        return
    }
    val button = remember { FocusRequester() }
    val focus = remember { Focusable() }
    val focused = focus.focused()
    fun close() {
        open = false
        runCatching { button.requestFocus() }
    }
    Box {
        Box(
            Modifier
                .shakes()
                .focusRequester(button)
                .size(56.dp)
                .clip(CircleShape)
                .background(if (focused && !open) Color.White else Color.Transparent)
                .selectable(focus) { if (enabled) open = true }
                .semantics { contentDescription = label },
            Alignment.Center,
        ) {
            MavioIcon(R.drawable.ic_more_vertical, size = 28.dp, tint = if (focused && !open) Color.Black else theme.colors.textSecondary)
        }
        if (open) {
            val first = remember { FocusRequester() }
            TvMenu(first, 360.dp, Alignment.TopEnd, 56.dp + MavioSpacing.s8, ::close) {
                items.forEachIndexed { i, (text, run) ->
                    TvMenuItem(
                        {
                            close()
                            run()
                        },
                        if (i == 0) Modifier.focusRequester(first) else Modifier,
                    ) {
                        Text(text, Modifier.weight(1f), style = theme.type.headline.text, color = theme.colors.danger)
                    }
                }
            }
        }
    }
}

/** A question before doing what cannot be undone, its action in red. */
@Composable
fun ConfirmDialog(title: String, message: String?, action: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    // The television's opens with focus on Cancel.
    if (LocalTelevision.current) {
        TvDialog(title, onDismiss) {
            message?.let { Text(it, style = theme.type.body.text, color = theme.colors.textSecondary) }
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
                TvButton(
                    action,
                    {
                        onDismiss()
                        onConfirm()
                    },
                    fill = theme.colors.danger,
                )
            }
        }
        return
    }
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            androidx.compose.material3.TextButton({
                onDismiss()
                onConfirm()
            }) { Text(action, color = theme.colors.danger) }
        },
        dismissButton = {
            androidx.compose.material3.TextButton(onDismiss) { Text(stringResource(R.string.action_cancel), color = theme.colors.textPrimary) }
        },
        title = { Text(title, style = theme.type.headline.text) },
        text = message?.let { { Text(it, style = theme.type.callout.text, color = theme.colors.textSecondary) } },
        containerColor = theme.colors.surfaceRaised,
    )
}

private val dateTimeFormat = DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM, FormatStyle.SHORT)
private val timeFormat = DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT)

/** A date and time in the user's locale and zone. */
private fun dateTime(at: Instant): String = dateTimeFormat.format(at.atZone(ZoneId.systemDefault()))

/** A message's time: the time alone today, else the date too. */
private fun shortTime(at: Instant): String {
    val zoned = at.atZone(ZoneId.systemDefault())
    return if (zoned.toLocalDate() == LocalDate.now()) timeFormat.format(zoned) else dateTimeFormat.format(zoned)
}

/**
 * Plays the speech kept with a server's answers, one at a time, fetched
 * with the session's token in an encoding the device plays.
 */
class KeptSpeech(private val model: AppModel, private val server: ServerRecord, private val context: Context) {
    /** The answer playing, by its key. */
    var at by mutableStateOf<String?>(null)
        private set
    private var player: MediaPlayer? = null
    private val scope = kotlinx.coroutines.MainScope()

    /** Plays the speech at url as the answer key, or stops it when it plays. */
    fun toggle(key: String, url: String) {
        val again = at == key
        stop()
        if (again) return
        at = key
        val path = url.trimStart('/') + (if ('?' in url) "&" else "?") + "formats=aac,mp3,flac,wav"
        scope.launch {
            val target = runCatching { model.client.serverRequest(server.id, path) }.getOrNull()
            if (target == null || at != key) {
                if (at == key) at = null
                return@launch
            }
            val p = MediaPlayer()
            player = p
            try {
                p.setAudioAttributes(AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_ASSISTANT).setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build())
                p.setDataSource(context, Uri.parse(target.url), mapOf("Authorization" to "Bearer ${target.token}"))
                p.setOnPreparedListener { if (player === it) it.start() }
                p.setOnCompletionListener { if (player === it) stop() }
                p.setOnErrorListener { mp, _, _ ->
                    if (player === mp) stop()
                    true
                }
                p.prepareAsync()
            } catch (e: Exception) {
                stop()
            }
        }
    }

    /** Stops what plays. */
    fun stop() {
        player?.let {
            player = null
            runCatching { it.release() }
        }
        at = null
    }
}
