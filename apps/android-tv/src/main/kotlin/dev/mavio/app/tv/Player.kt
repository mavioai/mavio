package dev.mavio.app.tv

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.net.Uri
import android.view.WindowManager
import androidx.activity.compose.BackHandler
import androidx.activity.compose.LocalActivity
import androidx.annotation.OptIn
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.layout.boundsInRoot
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.text.CueGroup
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.session.MediaSession
import androidx.media3.ui.SubtitleView
import androidx.media3.ui.compose.ContentFrame
import androidx.media3.ui.compose.PlayerSurface
import androidx.media3.ui.compose.state.rememberPlayPauseButtonState
import androidx.media3.ui.compose.state.rememberProgressStateWithTickInterval
import dev.mavio.core.AppModel
import dev.mavio.core.AudioStream
import dev.mavio.core.Details
import dev.mavio.core.PlayKind
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.PlaybackOptions
import dev.mavio.core.PlaybackSession
import dev.mavio.core.Source
import dev.mavio.core.SubtitleStream
import dev.mavio.design.MavioSpacing
import dev.mavio.player.GroupControls
import dev.mavio.player.PlayerEngine
import dev.mavio.player.chooseTracks
import dev.mavio.player.createPlayer
import dev.mavio.player.engineFor
import dev.mavio.player.playing
import dev.mavio.player.rememberGroupPlayer
import dev.mavio.ui.Focusable
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.Quality
import dev.mavio.ui.R
import dev.mavio.ui.TvButton
import dev.mavio.ui.focused
import dev.mavio.ui.languageName
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import dev.mavio.ui.Assistant
import dev.mavio.ui.AssistantPlayer
import dev.mavio.ui.AssistantPlayerEffect
import dev.mavio.ui.LocalAssistant
import dev.mavio.ui.TvAssistantButton
import mavio.assistant.v1.Assistant.ControlCommand

/** How far left and right seek. */
private const val SEEK_MS = 10_000L

/** How long the controller stays after the last press while playing. */
private const val CONTROLLER_TIMEOUT_MS = 3000L

/** A playback the server started, with the engine that plays it. */
private class Playback(val session: PlaybackSession, val info: PlaybackInfo, val details: Details?, val heading: String?, val engine: PlayerEngine) {
    /** The source played, with its streams. */
    val source: Source? = details?.sources?.firstOrNull { it.mediaSourceId == info.mediaSourceId }
}

/** Where the player was, kept as it goes so the server hears it when the player closes. */
private class Position {
    var ms = 0L
    var ended = false
}

/**
 * The audio and subtitle streams picked, by the server's indexes (-1 turns
 * the subtitles off), and the quality.
 */
private data class Selection(val audio: Int? = null, val subtitle: Int? = null, val quality: Quality = Quality.AUTO)

/** The tabs of the side panel, as the web UI's. */
private enum class Panel(val label: Int) {
    AUDIO(R.string.player_audio),
    SUBTITLES(R.string.player_subtitles),
    QUALITY(R.string.player_quality),
    INFO(R.string.player_info),
}

/** The side panel open, which stays while the streams change, and the row's button that opened it. */
private class Options {
    var panel by mutableStateOf<Panel?>(null)
    var opener = Panel.AUDIO
}

/** The speeds the player offers. */
private val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)

/**
 * The player on a television (docs/tv.md §7): the video over the whole
 * screen, and over it the controller, which follows focus. Media3 plays
 * what it can and every stream the server processes; libmpv plays the
 * other sources directly, and takes over from Media3 at its position when
 * Media3 cannot play a source after all. Picking other streams starts
 * again where it was. The server hears the progress as it plays and where
 * it stopped; at the end the next episode plays, else back returns.
 */
@OptIn(UnstableApi::class)
@Composable
fun PlayerPage(model: AppModel, page: Page.Player) {
    val context = LocalContext.current
    val pages = LocalPages.current
    val prefs = LocalPrefs.current
    var playback by remember(page) { mutableStateOf<Playback?>(null) }
    var failure by remember(page) { mutableStateOf<String?>(null) }
    var selection by remember(page) { mutableStateOf(Selection(quality = prefs.quality)) }
    // Where the next start begins: the resume position, then where the streams changed.
    var resume by remember(page) { mutableStateOf(page.start) }
    var sourceId by remember(page) { mutableStateOf(page.mediaSourceId) }
    var details by remember(page) { mutableStateOf<Details?>(null) }
    val options = remember(page) { Options() }
    KeepScreenOn()
    LaunchedEffect(page, selection) {
        playback = null
        var session: PlaybackSession? = null
        try {
            val item = details ?: runCatching { model.client.details(page.serverId, page.itemId) }.getOrNull()
            details = item
            session = model.play(page.serverId, page.itemId, PlaybackOptions(sourceId, selection.audio, selection.subtitle, selection.quality.bitrate(model.local(page.serverId)), resume, false))
            val info = session.info()
            sourceId = info.mediaSourceId
            val source = item?.sources?.firstOrNull { it.serverId == page.serverId && it.mediaSourceId == info.mediaSourceId }
            playback = Playback(session, info, item, item?.let { heading(context, it) }, engineFor(info, source))
        } catch (e: CancellationException) {
            session?.let { model.finish(page.serverId, it, resume) }
            throw e
        } catch (e: Exception) {
            failure = e.message ?: e.toString()
        }
    }
    val p = playback
    val next = p?.details?.next
    var engine by remember(p) { mutableStateOf(p?.engine) }
    var startMs by remember(p) { mutableLongStateOf(p?.info?.startPosition?.toMillis() ?: 0) }
    val position = remember(p) { Position().also { it.ms = startMs } }
    val player = remember(p, engine) {
        // A group's member waits for the group to play.
        val follows = model.syncPlay.joined.value.playing(page.serverId, page.itemId) != null
        p?.let { createPlayer(context, engine ?: it.engine, it.info, it.source, startMs, autoplay = !follows) }
    }
    // The SyncPlay group playing the item, which the controller asks instead of the player.
    val group = rememberGroupPlayer(model, page.serverId, page.itemId, player) { pages.back() }
    val currentGroup by rememberUpdatedState(group)
    val controls = remember(player) { player?.let { GroupControls(it) { currentGroup } } }
    val playNext = when {
        group != null && (group.hasNext || next != null) -> { { group.next(listOfNotNull(next?.card?.itemId)) } }
        else -> next?.let { e -> { pages.replace(Page.Player(page.serverId, e.card.itemId, e.card.title, null, e.position)) } }
    }
    val currentNext by rememberUpdatedState(playNext)
    DisposableEffect(p) {
        onDispose {
            if (p != null && !position.ended) model.finish(page.serverId, p.session, Duration.ofMillis(position.ms))
        }
    }
    DisposableEffect(player) {
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED && p != null && !position.ended) {
                    position.ended = true
                    model.finish(page.serverId, p.session, null)
                    val g = currentGroup
                    when {
                        g != null -> g.ended()
                        // The next episode plays only while the settings say so.
                        else -> currentNext?.takeIf { prefs.autoplayNext }?.invoke() ?: pages.back()
                    }
                }
            }

            override fun onTracksChanged(tracks: Tracks) {
                if (p != null && player is ExoPlayer) chooseTracks(player, tracks, p.info, p.source)
            }

            override fun onPlayerError(error: PlaybackException) {
                if (p != null && engine == PlayerEngine.MEDIA3 && p.info.kind == PlayKind.DIRECT_PLAY) {
                    startMs = player?.currentPosition ?: startMs
                    engine = PlayerEngine.MPV
                } else {
                    failure = error.message ?: error.errorCodeName
                }
            }
        }
        player?.addListener(listener)
        onDispose {
            player?.removeListener(listener)
            player?.release()
        }
    }
    // The server hears the position every tick; it reports every ten
    // seconds and on pause and seek.
    LaunchedEffect(player) {
        val pl = player ?: return@LaunchedEffect
        val session = p?.session ?: return@LaunchedEffect
        while (true) {
            delay(1000)
            if (pl.playbackState == Player.STATE_READY || pl.playbackState == Player.STATE_BUFFERING) {
                position.ms = pl.currentPosition
                runCatching { session.update(Duration.ofMillis(position.ms), !pl.isPlaying) }
            }
        }
    }
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) { player?.pause() }
    // The system's media controls, the remote's media keys and the
    // assistant drive the player through its session; the next episode's
    // starts while this one's player goes, so each has an ID of its own.
    DisposableEffect(controls) {
        val session = controls?.let { MediaSession.Builder(context, it).setId(UUID.randomUUID().toString()).build() }
        onDispose { session?.release() }
    }
    // The assistant controls the player as the remote does.
    val currentSelection by rememberUpdatedState(selection)
    val assistantPlayer = remember(p, player, controls) {
        val pl = controls ?: player
        if (p == null || pl == null) return@remember null
        object : AssistantPlayer {
            override val playing
                get() = Assistant.playing(
                    page.itemId,
                    pl.currentPosition,
                    paused = !pl.playWhenReady,
                    speed = pl.playbackParameters.speed.toDouble(),
                    audio = p.info.audioStreamIndex,
                    subtitle = p.info.subtitleStreamIndex,
                )

            override fun control(command: ControlCommand, value: Double): Boolean {
                when (command) {
                    ControlCommand.CONTROL_COMMAND_PAUSE -> pl.pause()
                    ControlCommand.CONTROL_COMMAND_RESUME -> pl.play()
                    ControlCommand.CONTROL_COMMAND_STOP -> pages.back()
                    ControlCommand.CONTROL_COMMAND_SEEK -> pl.seekTo((value * 1000).toLong().coerceAtLeast(0))
                    ControlCommand.CONTROL_COMMAND_SEEK_BY -> pl.seekTo((pl.currentPosition + (value * 1000).toLong()).coerceAtLeast(0))
                    ControlCommand.CONTROL_COMMAND_SPEED -> {
                        if (value <= 0) return false
                        pl.setPlaybackSpeed(value.toFloat())
                    }
                    ControlCommand.CONTROL_COMMAND_NEXT -> (currentNext ?: return false)()
                    // The player fills the screen already.
                    ControlCommand.CONTROL_COMMAND_FULLSCREEN -> Unit
                    else -> return false
                }
                return true
            }

            override fun streams(audio: Int?, subtitle: Int?): Boolean {
                val current = Selection(p.info.audioStreamIndex, p.info.subtitleStreamIndex ?: -1, currentSelection.quality)
                resume = Duration.ofMillis(pl.currentPosition)
                selection = current.copy(audio = audio ?: current.audio, subtitle = subtitle ?: current.subtitle)
                return true
            }
        }
    }
    AssistantPlayerEffect(assistantPlayer)

    Box(Modifier.fillMaxSize().background(Color.Black)) {
        when {
            player != null && engine == PlayerEngine.MPV -> PlayerSurface(player, Modifier.fillMaxSize())
            player != null -> {
                ContentFrame(player, Modifier.fillMaxSize())
                Subtitles(player)
            }
        }
        val error = failure
        if (error != null) {
            Failure(error)
        } else if (player != null && p != null) {
            val chip: (@Composable (onOpen: (Boolean) -> Unit) -> Unit)? = group?.let { g -> { onOpen -> GroupChip(model, page.serverId, g.group, onOpen) } }
            Controller(controls ?: player, page.title, p, selection.quality, options, playNext, chip) { picked ->
                resume = Duration.ofMillis(player.currentPosition)
                // The quality stays this device's, as the web UI's.
                prefs.quality = picked.quality
                selection = picked
            }
        } else {
            CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
        }
    }
}

/** Keeps the screen on while the player is open. */
@Composable
private fun KeepScreenOn() {
    val activity = LocalActivity.current ?: return
    DisposableEffect(activity) {
        activity.window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        onDispose { activity.window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON) }
    }
}

/** The subtitles Media3 decodes, drawn by its own view over the video; libmpv draws its own. */
@OptIn(UnstableApi::class)
@Composable
private fun Subtitles(player: Player) {
    var view by remember { mutableStateOf<SubtitleView?>(null) }
    DisposableEffect(player, view) {
        val v = view ?: return@DisposableEffect onDispose {}
        val listener = object : Player.Listener {
            override fun onCues(cueGroup: CueGroup) = v.setCues(cueGroup.cues)
        }
        player.addListener(listener)
        onDispose { player.removeListener(listener) }
    }
    AndroidView(
        { context -> SubtitleView(context).apply { setUserDefaultStyle(); setUserDefaultTextSize() }.also { view = it } },
        Modifier.fillMaxSize(),
    )
}

/** An episode's series and numbers, as "Atlas Station · S1 E2". */
private fun heading(context: android.content.Context, details: Details): String? {
    val series = details.series?.name ?: return null
    val season = details.seasonNumber
    val episode = details.episodeNumber
    val numbers = if (season != null && episode != null) {
        context.getString(R.string.item_episode_short, season.toString(), episode.toString())
    } else {
        null
    }
    return listOfNotNull(series, numbers).joinToString(" · ")
}

/**
 * The controller over the video (docs/tv.md §7): the title and the
 * timeline, and a row of the SyncPlay group's chip while the player
 * follows one, the next episode, the subtitles and the audio.
 * Down shows it; left and right seek by 10 seconds and show it; select
 * pauses, showing it while paused, and plays on, hiding it. It hides once
 * nothing is pressed for 3 seconds during playback. Down again moves focus
 * into its row, where left and right move among the buttons and up leaves
 * it. The audio and subtitles open in a side panel with focus on the
 * current choice, and closing it returns focus to its button. Back closes
 * the panel, then the controller, then the player.
 */
@OptIn(UnstableApi::class)
@Composable
private fun Controller(
    player: Player,
    title: String,
    playback: Playback,
    quality: Quality,
    options: Options,
    playNext: (() -> Unit)?,
    chip: (@Composable (onOpen: (Boolean) -> Unit) -> Unit)?,
    onPick: (Selection) -> Unit,
) {
    val pages = LocalPages.current
    val playPause = rememberPlayPauseButtonState(player)
    val progress = rememberProgressStateWithTickInterval(player, 500, rememberCoroutineScope())
    var shown by remember { mutableStateOf(false) }
    var panel by options::panel
    // The group's dialog, opened from the chip, keeps the controller that hosts it.
    var held by remember { mutableStateOf(false) }
    // An open panel keeps the controller up, as the web UI's.
    val visible = shown || panel != null || held
    // How far the timeline's top is from the screen's foot, the panel sitting above it, as the web UI's.
    var bottom by remember { mutableFloatStateOf(0f) }
    var timeline by remember { mutableFloatStateOf(0f) }
    val density = LocalDensity.current
    var pressed by remember { mutableLongStateOf(0L) }
    var inRow by remember { mutableStateOf(false) }
    val idle = remember { FocusRequester() }
    val row = remember { FocusRequester() }
    val openers = remember { Panel.entries.associateWith { FocusRequester() } }
    LaunchedEffect(shown, pressed, playPause.showPlay, panel, held) {
        if (shown && !playPause.showPlay && panel == null && !held) {
            delay(CONTROLLER_TIMEOUT_MS)
            shown = false
        }
    }
    // Outside the row, focus stays on the page, which shows none.
    LaunchedEffect(visible) {
        if (visible) return@LaunchedEffect
        withFrameNanos {}
        runCatching { idle.requestFocus() }
    }
    val seek = { by: Long ->
        val duration = player.duration.takeIf { it > 0 } ?: Long.MAX_VALUE
        player.seekTo((player.currentPosition + by).coerceIn(0, duration))
        shown = true
    }
    val toggle = {
        val paused = !playPause.showPlay
        playPause.onClick()
        // Paused, the controller stays; playing on, it goes.
        shown = paused
    }
    BackHandler {
        when {
            panel != null -> {
                panel = null
                shown = true
                runCatching { openers.getValue(options.opener).requestFocus() }
            }
            shown -> shown = false
            else -> pages.back()
        }
    }
    val buffering = rememberBuffering(player)
    Box(
        Modifier
            .fillMaxSize()
            .onGloballyPositioned { bottom = it.boundsInRoot().bottom }
            .onPreviewKeyEvent { e ->
                if (e.type == KeyEventType.KeyDown) pressed = System.nanoTime()
                val down = e.type == KeyEventType.KeyDown
                when {
                    e.key == Key.MediaPlayPause || e.key == Key.MediaPlay || e.key == Key.MediaPause -> {
                        if (down) toggle()
                        true
                    }
                    e.key == Key.MediaRewind || e.key == Key.MediaFastForward -> {
                        if (down) seek(if (e.key == Key.MediaRewind) -SEEK_MS else SEEK_MS)
                        true
                    }
                    // The side panel and the row move focus as usual, and up leaves the row.
                    panel != null -> false
                    inRow -> {
                        if (e.key == Key.DirectionUp) {
                            if (down) runCatching { idle.requestFocus() }
                            true
                        } else {
                            e.key == Key.DirectionDown
                        }
                    }
                    e.key == Key.DirectionLeft || e.key == Key.DirectionRight -> {
                        if (down) seek(if (e.key == Key.DirectionLeft) -SEEK_MS else SEEK_MS)
                        true
                    }
                    e.key == Key.DirectionDown -> {
                        if (down) {
                            if (shown) runCatching { row.requestFocus() } else shown = true
                        }
                        true
                    }
                    e.key == Key.DirectionUp -> {
                        if (down) shown = true
                        true
                    }
                    e.key == Key.DirectionCenter || e.key == Key.Enter || e.key == Key.NumPadEnter -> {
                        if (down) toggle()
                        true
                    }
                    else -> false
                }
            }
            .focusRequester(idle)
            .focusable(),
    ) {
        if (buffering) CircularProgressIndicator(Modifier.align(Alignment.Center).size(72.dp), color = Color.White, strokeWidth = 5.dp)
        AnimatedVisibility(visible, Modifier.fillMaxSize(), enter = fadeIn(), exit = fadeOut()) {
            Box(Modifier.fillMaxSize()) {
                Column(
                    Modifier
                        .align(Alignment.BottomCenter)
                        .fillMaxWidth()
                        .background(Brush.verticalGradient(0f to Color.Transparent, 0.4f to Color.Black.copy(alpha = 0.45f), 1f to Color.Black.copy(alpha = 0.9f)))
                        .padding(start = theme.layout.margin, end = theme.layout.margin, top = 160.dp, bottom = 64.dp),
                    verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                ) {
                    playback.heading?.let { Text(it, style = theme.type.headline.text, color = Color.White.copy(alpha = 0.75f), maxLines = 1) }
                    Text(title, style = theme.type.title1.text, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    val duration = progress.durationMs.coerceAtLeast(0)
                    val now = progress.currentPositionMs.coerceAtLeast(0)
                    Timeline(if (duration > 0) now.toFloat() / duration else 0f, Modifier.onGloballyPositioned { timeline = it.boundsInRoot().top })
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                        // Whether it plays, which select changes.
                        MavioIcon(if (playPause.showPlay) R.drawable.ic_pause_fill else R.drawable.ic_play_fill, size = 28.dp, tint = Color.White)
                        Text(
                            "${clock(now)} / ${clock(duration)}",
                            Modifier.weight(1f),
                            style = theme.type.headline.text,
                            color = Color.White.copy(alpha = 0.85f),
                        )
                        Row(
                            Modifier
                                .onFocusChanged { inRow = it.hasFocus }
                                .focusRequester(row)
                                .focusGroup(),
                            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            chip?.invoke { held = it }
                            playNext?.let { TvButton(stringResource(R.string.player_next_episode), it, icon = R.drawable.ic_next_fill) }
                            LocalAssistant.current?.let { TvAssistantButton(it, 64.dp, fill = Color.White.copy(alpha = 0.2f), tint = Color.White) }
                            if (playback.info.subtitles.isNotEmpty()) {
                                val on = (playback.info.subtitleStreamIndex ?: -1) >= 0
                                TvCircleButton(
                                    if (on) R.drawable.ic_subtitles_fill else R.drawable.ic_subtitles,
                                    stringResource(R.string.player_subtitles),
                                    Modifier.focusRequester(openers.getValue(Panel.SUBTITLES)),
                                ) {
                                    options.opener = Panel.SUBTITLES
                                    panel = Panel.SUBTITLES
                                }
                            }
                            TvCircleButton(
                                R.drawable.ic_audio,
                                stringResource(R.string.player_audio),
                                Modifier.focusRequester(openers.getValue(Panel.AUDIO)),
                            ) {
                                options.opener = Panel.AUDIO
                                panel = Panel.AUDIO
                            }
                        }
                    }
                }
            }
        }
        panel?.let { open ->
            SidePanel(open, player, playback, quality, with(density) { (bottom - timeline).coerceAtLeast(0f).toDp() } + MavioSpacing.s16, Modifier.align(Alignment.BottomEnd), onTab = { panel = it }, onPick = onPick)
        }
    }
}

/** Whether [player] waits for media, following its state as it changes. */
@Composable
private fun rememberBuffering(player: Player): Boolean {
    fun waiting() = player.playbackState == Player.STATE_BUFFERING || player.playbackState == Player.STATE_IDLE
    var buffering by remember(player) { mutableStateOf(waiting()) }
    DisposableEffect(player) {
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                buffering = waiting()
            }
        }
        player.addListener(listener)
        buffering = waiting()
        onDispose { player.removeListener(listener) }
    }
    return buffering
}

/** How far the playback is, as a slim bar across the controller. */
@Composable
private fun Timeline(fraction: Float, modifier: Modifier) {
    Box(modifier.fillMaxWidth().height(8.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.3f))) {
        Box(Modifier.fillMaxWidth(fraction.coerceIn(0f, 1f)).fillMaxHeight().background(Color.White))
    }
}

/**
 * The side panel of the player's options, as the web UI's: dark glass over
 * the video's corner, [above] its foot, over the timeline, with the
 * tabs of the audio streams and the speed, the subtitles, the quality and
 * the playback's information. Focus starts
 * on the current choice of the tab opened and stays within the panel; a
 * tab shows as focus moves to it, and focus entering the tabs lands on the
 * current one.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SidePanel(
    panel: Panel,
    player: Player,
    playback: Playback,
    quality: Quality,
    above: Dp,
    modifier: Modifier,
    onTab: (Panel) -> Unit,
    onPick: (Selection) -> Unit,
) {
    val context = LocalContext.current
    val info = playback.info
    val source = playback.source
    val current = Selection(info.audioStreamIndex, info.subtitleStreamIndex ?: -1, quality)
    // Without subtitles there is no tab for them, as the web UI's.
    val tabs = Panel.entries.filter { it != Panel.SUBTITLES || info.subtitles.isNotEmpty() }
    val chosen = remember { FocusRequester() }
    val tabFocus = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        withFrameNanos {}
        if (!runCatching { chosen.requestFocus() }.getOrDefault(false)) runCatching { tabFocus.requestFocus() }
    }
    Column(
        modifier
            .padding(start = MavioSpacing.s16, top = MavioSpacing.s16, end = theme.layout.margin, bottom = above)
            .width(360.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(Color(0xBF171717))
            .border(1.dp, Color.White.copy(alpha = 0.1f), RoundedCornerShape(18.dp))
            .focusProperties { onExit = { if (requestedFocusDirection != FocusDirection.Enter) cancelFocusChange() } }
            .focusGroup(),
    ) {
        Row(
            Modifier
                .padding(12.dp)
                .fillMaxWidth()
                .clip(RoundedCornerShape(9.dp))
                .background(Color.White.copy(alpha = 0.1f))
                .padding(2.dp)
                .focusProperties { onEnter = { tabFocus.requestFocus() } }
                .focusGroup(),
        ) {
            tabs.forEach { tab ->
                Tab(stringResource(tab.label), tab == panel, Modifier.weight(1f).then(if (tab == panel) Modifier.focusRequester(tabFocus) else Modifier)) { onTab(tab) }
            }
        }
        Column(
            Modifier
                .weight(1f, fill = false)
                .verticalScroll(rememberScrollState())
                .padding(start = 8.dp, end = 8.dp, bottom = 12.dp),
        ) {
            val first = { checked: Boolean -> if (checked) Modifier.focusRequester(chosen) else Modifier }
            when (panel) {
                Panel.AUDIO -> {
                    source?.audio.orEmpty().forEach { a ->
                        val checked = a.index == info.audioStreamIndex
                        Choice(checked, audioLabel(a), first(checked)) { onPick(current.copy(audio = a.index)) }
                    }
                    Heading(stringResource(R.string.player_speed))
                    var speed by remember { mutableFloatStateOf(player.playbackParameters.speed) }
                    // A group's speed stays the group's.
                    val free = player.isCommandAvailable(Player.COMMAND_SET_SPEED_AND_PITCH)
                    FlowRow(
                        Modifier.padding(horizontal = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        SPEEDS.forEach { r ->
                            Speed("${if (r % 1f == 0f) r.toInt().toString() else r.toString()}×", r == speed, free) {
                                player.setPlaybackSpeed(r)
                                speed = r
                            }
                        }
                    }
                }
                Panel.SUBTITLES -> {
                    val picked = info.subtitleStreamIndex ?: -1
                    Choice(picked < 0, stringResource(R.string.player_subtitles_off) to "", first(picked < 0)) { onPick(current.copy(subtitle = -1)) }
                    info.subtitles.forEach { t ->
                        val stream = source?.subtitles?.firstOrNull { it.index == t.streamIndex }
                        val label = stream?.let { subtitleLabel(it) }
                            ?: ((t.title ?: languageName(t.language) ?: "#${t.streamIndex}") to t.format.uppercase())
                        val checked = t.streamIndex == picked
                        Choice(checked, label, first(checked)) { onPick(current.copy(subtitle = t.streamIndex)) }
                    }
                }
                Panel.QUALITY -> Quality.entries.forEach { q ->
                    Choice(q == quality, (q.label?.let { stringResource(it) } ?: q.key) to "", first(q == quality)) {
                        if (q != quality) onPick(current.copy(quality = q))
                    }
                }
                Panel.INFO -> Column(
                    Modifier.padding(top = 4.dp),
                    verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                ) {
                    Fact(
                        stringResource(R.string.player_method),
                        stringResource(
                            when (info.kind) {
                                PlayKind.DIRECT_PLAY -> R.string.player_direct_play
                                PlayKind.DIRECT_STREAM -> R.string.player_direct_stream
                                PlayKind.TRANSCODE -> R.string.player_transcode
                            },
                        ),
                    )
                    source?.let { s ->
                        Fact(
                            stringResource(R.string.player_source),
                            listOfNotNull(s.name, s.container?.split(',')?.first()?.uppercase()).joinToString(" · "),
                        )
                    }
                    Column {
                        PanelAction(R.drawable.ic_external, stringResource(R.string.player_open_external)) {
                            val intent = Intent(Intent.ACTION_VIEW).setDataAndType(Uri.parse(info.url), "video/*")
                            runCatching { context.startActivity(Intent.createChooser(intent, null)) }
                        }
                        PanelAction(null, stringResource(R.string.player_copy_link)) {
                            context.getSystemService(ClipboardManager::class.java)?.setPrimaryClip(ClipData.newPlainText(null, info.url))
                        }
                    }
                }
            }
        }
    }
}

/** A tab of the side panel, chosen as focus moves to it: filled when chosen, white with focus. */
@Composable
private fun Tab(label: String, chosen: Boolean, modifier: Modifier, onChoose: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Text(
        label,
        modifier
            .shakes()
            .onFocusChanged { if (it.isFocused && !chosen) onChoose() }
            .clip(RoundedCornerShape(7.dp))
            .background(
                when {
                    focused -> Color.White
                    chosen -> Color.White.copy(alpha = 0.25f)
                    else -> Color.Transparent
                },
            )
            .selectable(state, onChoose)
            .padding(horizontal = 8.dp, vertical = 4.dp),
        style = theme.type.footnote.text,
        fontWeight = FontWeight.SemiBold,
        color = when {
            focused -> Color.Black
            chosen -> Color.White
            else -> Color.White.copy(alpha = 0.7f)
        },
        textAlign = TextAlign.Center,
        maxLines = 1,
    )
}

/**
 * A speed of the side panel: filled when chosen and white with focus, so
 * that focus on the chosen one shows.
 */
@Composable
private fun Speed(label: String, chosen: Boolean, enabled: Boolean, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Text(
        label,
        Modifier
            .shakes()
            .alpha(if (enabled) 1f else 0.4f)
            .clip(CircleShape)
            .background(
                when {
                    focused -> Color.White
                    chosen -> Color.White.copy(alpha = 0.3f)
                    else -> Color.White.copy(alpha = 0.1f)
                },
            )
            .then(if (enabled) Modifier.selectable(state, onClick) else Modifier)
            .padding(horizontal = 12.dp, vertical = 4.dp),
        style = theme.type.footnote.text,
        fontWeight = FontWeight.SemiBold,
        color = if (focused) Color.Black else Color.White,
    )
}

/** A heading in the side panel. */
@Composable
private fun Heading(label: String) {
    Text(
        label,
        Modifier.padding(start = 10.dp, top = MavioSpacing.s16, bottom = MavioSpacing.s8),
        style = theme.type.footnote.text,
        fontWeight = FontWeight.SemiBold,
        color = Color.White.copy(alpha = 0.6f),
    )
}

/** An action in the panel's information: its icon and its name, white with focus. */
@Composable
private fun PanelAction(icon: Int?, label: String, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val color = if (focused) Color.Black else Color.White
    Row(
        Modifier
            .shakes()
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .background(if (focused) Color.White else Color.Transparent)
            .selectable(state, onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        icon?.let { MavioIcon(it, size = 20.dp, tint = color.copy(alpha = 0.7f)) }
        Text(label, style = theme.type.callout.text, color = color)
    }
}

/** A fact of the playback in the side panel: its label over its value. */
@Composable
private fun Fact(label: String, value: String) {
    Column(Modifier.padding(horizontal = 10.dp)) {
        Text(label, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = Color.White.copy(alpha = 0.6f))
        Text(value, Modifier.padding(top = 2.dp), style = theme.type.callout.text, color = Color.White)
    }
}

/** A choice of the side panel: its name and details, checked when current, white with focus. */
@Composable
private fun Choice(checked: Boolean, label: Pair<String, String>, modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val color = if (focused) Color.Black else Color.White
    Row(
        modifier
            .shakes()
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .background(if (focused) Color.White else Color.Transparent)
            .selectable(state, onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
    ) {
        Box(Modifier.width(20.dp), Alignment.Center) {
            if (checked) MavioIcon(R.drawable.ic_check, size = 20.dp, tint = theme.colors.accent)
        }
        Column(Modifier.weight(1f)) {
            Text(label.first, style = theme.type.callout.text, color = color, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (label.second.isNotEmpty()) {
                Text(label.second, style = theme.type.caption.text, color = color.copy(alpha = 0.6f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** An audio stream's name and details, as the web UI's: "English" and "AAC · 5.1". */
private fun audioLabel(a: AudioStream): Pair<String, String> {
    val channels = when (a.channels) {
        0 -> null
        6 -> "5.1"
        8 -> "7.1"
        else -> "${a.channels} ch"
    }
    return (a.title ?: languageName(a.language) ?: "#${a.index}") to
        listOfNotNull(a.codec.uppercase(), channels).joinToString(" · ")
}

/** A subtitle stream's name and details, as the web UI's: "English" and "SUBRIP · Forced". */
private fun subtitleLabel(s: SubtitleStream): Pair<String, String> =
    (s.title ?: languageName(s.language) ?: "#${s.index}") to
        listOfNotNull(s.codec.uppercase(), "Forced".takeIf { s.forced }, "SDH".takeIf { s.hearingImpaired }).joinToString(" · ")

/** That the item can't be played, and why. */
@Composable
private fun Failure(message: String) {
    Column(Modifier.fillMaxSize().padding(theme.layout.margin), Arrangement.Center, Alignment.CenterHorizontally) {
        MavioIcon(R.drawable.ic_error, size = 64.dp, tint = Color.White)
        Spacer(Modifier.height(MavioSpacing.s16))
        Text(stringResource(R.string.state_error), style = theme.type.title2.text, color = Color.White)
        Text(message, style = theme.type.body.text, color = Color.White.copy(alpha = 0.7f))
    }
}

/** A position as "1:02:03" or "2:03". */
private fun clock(ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    val h = s / 3600
    val m = s % 3600 / 60
    val sec = s % 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
}
