package dev.mavio.app.tv

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
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
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
import androidx.media3.ui.SubtitleView
import androidx.media3.ui.compose.ContentFrame
import androidx.media3.ui.compose.PlayerSurface
import androidx.media3.ui.compose.state.rememberPlayPauseButtonState
import androidx.media3.ui.compose.state.rememberProgressStateWithTickInterval
import dev.mavio.core.AppModel
import dev.mavio.core.AudioStream
import dev.mavio.core.Details
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.PlaybackOptions
import dev.mavio.core.PlaybackSession
import dev.mavio.core.Source
import dev.mavio.core.SubtitleStream
import dev.mavio.design.MavioSpacing
import dev.mavio.player.PlayerEngine
import dev.mavio.player.chooseTracks
import dev.mavio.player.createPlayer
import dev.mavio.player.engineFor
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.languageName
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay

/** The bitrate the server keeps to, as the other apps' Auto quality. */
private const val AUTO_BITRATE = 40_000_000L

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

/** The audio and subtitle streams picked, by the server's indexes; -1 turns the subtitles off. */
private data class Selection(val audio: Int? = null, val subtitle: Int? = null)

/** The side panel's pickers. */
private enum class Panel { AUDIO, SUBTITLES }

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
    var playback by remember(page) { mutableStateOf<Playback?>(null) }
    var failure by remember(page) { mutableStateOf<String?>(null) }
    var selection by remember(page) { mutableStateOf(Selection()) }
    // Where the next start begins: the resume position, then where the streams changed.
    var resume by remember(page) { mutableStateOf(page.start) }
    var sourceId by remember(page) { mutableStateOf(page.mediaSourceId) }
    var details by remember(page) { mutableStateOf<Details?>(null) }
    KeepScreenOn()
    LaunchedEffect(page, selection) {
        playback = null
        var session: PlaybackSession? = null
        try {
            val item = details ?: runCatching { model.client.details(page.serverId, page.itemId) }.getOrNull()
            details = item
            session = model.play(page.serverId, page.itemId, PlaybackOptions(sourceId, selection.audio, selection.subtitle, AUTO_BITRATE, resume, false))
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
    val player = remember(p, engine) { p?.let { createPlayer(context, engine ?: it.engine, it.info, it.source, startMs, autoplay = true) } }
    val playNext = next?.let { e -> { pages.replace(Page.Player(page.serverId, e.card.itemId, e.card.title, null, e.position)) } }
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
                    currentNext?.invoke() ?: pages.back()
                }
            }

            override fun onTracksChanged(tracks: Tracks) {
                if (p != null && player is ExoPlayer) chooseTracks(player, tracks, p.info, p.source)
            }

            override fun onPlayerError(error: PlaybackException) {
                if (p != null && engine == PlayerEngine.MEDIA3 && p.info.kind == dev.mavio.core.PlayKind.DIRECT_PLAY) {
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
            Controller(player, page.title, p, playNext) { picked ->
                resume = Duration.ofMillis(player.currentPosition)
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
 * timeline, and a row of the next episode, the subtitles and the audio.
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
private fun Controller(player: Player, title: String, playback: Playback, playNext: (() -> Unit)?, onPick: (Selection) -> Unit) {
    val pages = LocalPages.current
    val playPause = rememberPlayPauseButtonState(player)
    val progress = rememberProgressStateWithTickInterval(player, 500, rememberCoroutineScope())
    var shown by remember { mutableStateOf(false) }
    var panel by remember { mutableStateOf<Panel?>(null) }
    var pressed by remember { mutableLongStateOf(0L) }
    var inRow by remember { mutableStateOf(false) }
    val idle = remember { FocusRequester() }
    val row = remember { FocusRequester() }
    val openers = remember { Panel.entries.associateWith { FocusRequester() } }
    LaunchedEffect(shown, pressed, playPause.showPlay, panel) {
        if (shown && !playPause.showPlay && panel == null) {
            delay(CONTROLLER_TIMEOUT_MS)
            shown = false
        }
    }
    // Outside the row, focus stays on the page, which shows none.
    LaunchedEffect(shown) {
        if (shown) return@LaunchedEffect
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
                val opener = openers.getValue(panel!!)
                panel = null
                runCatching { opener.requestFocus() }
            }
            shown -> shown = false
            else -> pages.back()
        }
    }
    val buttons = playNext != null || playback.info.subtitles.isNotEmpty() || playback.source?.subtitles?.isNotEmpty() == true ||
        (playback.source?.audio?.size ?: 0) > 1
    val buffering = player.playbackState == Player.STATE_BUFFERING || player.playbackState == Player.STATE_IDLE
    Box(
        Modifier
            .fillMaxSize()
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
                            if (shown && buttons) runCatching { row.requestFocus() } else shown = true
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
        AnimatedVisibility(shown, Modifier.fillMaxSize(), enter = fadeIn(), exit = fadeOut()) {
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
                    Timeline(if (duration > 0) now.toFloat() / duration else 0f)
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                        // Whether it plays, which select changes.
                        MavioIcon(if (playPause.showPlay) R.drawable.ic_pause_fill else R.drawable.ic_play_fill, size = 28.dp, tint = Color.White)
                        Text(
                            "${clock(now)} / ${clock(duration)}",
                            Modifier.weight(1f),
                            style = theme.type.headline.text,
                            color = Color.White.copy(alpha = 0.85f),
                        )
                        if (buttons) {
                            Row(
                                Modifier
                                    .onFocusChanged { inRow = it.hasFocus }
                                    .focusRequester(row)
                                    .focusGroup(),
                                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                playNext?.let { TvButton(stringResource(R.string.player_next_episode), it, icon = R.drawable.ic_next_fill) }
                                if (playback.info.subtitles.isNotEmpty() || playback.source?.subtitles?.isNotEmpty() == true) {
                                    val on = (playback.info.subtitleStreamIndex ?: -1) >= 0
                                    TvCircleButton(
                                        if (on) R.drawable.ic_subtitles_fill else R.drawable.ic_subtitles,
                                        stringResource(R.string.player_subtitles),
                                        Modifier.focusRequester(openers.getValue(Panel.SUBTITLES)),
                                    ) { panel = Panel.SUBTITLES }
                                }
                                if ((playback.source?.audio?.size ?: 0) > 1) {
                                    TvCircleButton(
                                        R.drawable.ic_audio,
                                        stringResource(R.string.player_audio),
                                        Modifier.focusRequester(openers.getValue(Panel.AUDIO)),
                                    ) { panel = Panel.AUDIO }
                                }
                            }
                        }
                    }
                }
            }
        }
        panel?.let { open ->
            SidePanel(open, playback, Modifier.align(Alignment.CenterEnd)) { picked ->
                panel = null
                shown = false
                onPick(picked)
            }
        }
    }
}

/** How far the playback is, as a slim bar across the controller. */
@Composable
private fun Timeline(fraction: Float) {
    Box(Modifier.fillMaxWidth().height(8.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.3f))) {
        Box(Modifier.fillMaxWidth(fraction.coerceIn(0f, 1f)).fillMaxHeight().background(Color.White))
    }
}

/**
 * The side panel of the audio streams or the subtitles, dark glass at the
 * screen's end, with focus on the current choice.
 */
@Composable
private fun SidePanel(panel: Panel, playback: Playback, modifier: Modifier, onPick: (Selection) -> Unit) {
    val info = playback.info
    val source = playback.source
    val current = Selection(info.audioStreamIndex, info.subtitleStreamIndex ?: -1)
    val choices: List<Triple<Boolean, Pair<String, String>, Selection>> = when (panel) {
        Panel.AUDIO -> source?.audio.orEmpty().map { a -> Triple(a.index == info.audioStreamIndex, audioLabel(a), current.copy(audio = a.index)) }
        Panel.SUBTITLES -> listOf(Triple((info.subtitleStreamIndex ?: -1) < 0, stringResource(R.string.player_subtitles_off) to "", current.copy(subtitle = -1))) +
            source?.subtitles.orEmpty().map { s -> Triple(s.index == info.subtitleStreamIndex, subtitleLabel(s), current.copy(subtitle = s.index)) }
    }
    val chosen = remember { FocusRequester() }
    LaunchedEffect(panel) {
        withFrameNanos {}
        runCatching { chosen.requestFocus() }
    }
    Column(
        modifier
            .fillMaxHeight()
            .width(560.dp)
            .background(Color(0xE6171717))
            .border(1.dp, Color.White.copy(alpha = 0.1f))
            .padding(vertical = 64.dp),
    ) {
        Text(
            stringResource(if (panel == Panel.AUDIO) R.string.player_audio else R.string.player_subtitles),
            Modifier.padding(horizontal = 40.dp, vertical = MavioSpacing.s16),
            style = theme.type.title2.text,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        LazyColumn(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
            items(choices) { (checked, label, selection) ->
                Choice(checked, label, if (checked) Modifier.focusRequester(chosen) else Modifier) { onPick(selection) }
            }
        }
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
            .padding(horizontal = MavioSpacing.s24)
            .fillMaxWidth()
            .then(if (focused) Modifier.shadow(12.dp, RoundedCornerShape(12.dp)) else Modifier)
            .clip(RoundedCornerShape(12.dp))
            .background(if (focused) Color.White else Color.Transparent)
            .selectable(state, onClick)
            .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
    ) {
        Box(Modifier.size(28.dp), Alignment.Center) {
            if (checked) MavioIcon(R.drawable.ic_check, size = 28.dp, tint = color)
        }
        Column(Modifier.weight(1f)) {
            Text(label.first, style = theme.type.headline.text, color = color, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (label.second.isNotEmpty()) {
                Text(label.second, style = theme.type.footnote.text, color = color.copy(alpha = 0.7f), maxLines = 1)
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
