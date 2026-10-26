package dev.mavio.app

import android.app.PictureInPictureParams
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.view.View
import android.content.Intent
import android.content.pm.ActivityInfo
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.util.Rational
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.LocalActivity
import androidx.annotation.DrawableRes
import androidx.annotation.OptIn
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.layout.positionInRoot
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.app.PictureInPictureModeChangedInfo
import androidx.core.util.Consumer
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.VideoSize
import androidx.media3.common.text.CueGroup
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.session.MediaSession
import androidx.media3.ui.SubtitleView
import androidx.media3.ui.compose.ContentFrame
import androidx.media3.ui.compose.PlayerSurface
import androidx.media3.ui.compose.state.rememberPlayPauseButtonState
import androidx.media3.ui.compose.state.rememberProgressStateWithTickInterval
import dev.mavio.app.R as AppR
import dev.mavio.core.AppModel
import dev.mavio.core.AudioStream
import dev.mavio.core.Details
import dev.mavio.core.PlayKind
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.PlaybackOptions
import dev.mavio.core.PlaybackSession
import dev.mavio.core.Source
import dev.mavio.core.SubtitleStream
import dev.mavio.player.GroupControls
import dev.mavio.player.MpvPlayer
import dev.mavio.player.Seek
import dev.mavio.player.rememberBuffering
import dev.mavio.player.rememberSeek
import dev.mavio.player.SubtitleShift
import dev.mavio.player.PlaybackRetry
import dev.mavio.player.PlayerEngine
import dev.mavio.player.chooseTracks
import dev.mavio.player.createPlayer
import dev.mavio.player.engineFor
import dev.mavio.player.playing
import dev.mavio.player.rememberGroupPlayer
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.ChapterTrack
import dev.mavio.ui.SeekPreview
import dev.mavio.ui.chapterAt
import dev.mavio.ui.chooseSheets
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.MavioTheme
import dev.mavio.ui.Quality
import dev.mavio.ui.R
import dev.mavio.ui.languageName
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import java.util.UUID
import kotlin.math.abs
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import dev.mavio.ui.Assistant
import dev.mavio.ui.AssistantButton
import dev.mavio.ui.AssistantPlayer
import dev.mavio.ui.AssistantPlayerEffect
import dev.mavio.ui.LocalAssistant
import mavio.assistant.v1.Assistant.ControlCommand

/** A playback the server started, with the engine that plays it. */
private class Playback(
    val session: PlaybackSession,
    val info: PlaybackInfo,
    /** The item's details, for its streams and its heading. */
    val details: Details?,
    /** The series and episode above the title, for an episode. */
    val heading: String?,
    val engine: PlayerEngine,
) {
    /** The source played, with its streams. */
    val source: Source? = details?.sources?.firstOrNull { it.mediaSourceId == info.mediaSourceId }
}

/** Where the player was, kept as it goes so the server hears it when the player closes. */
private class Position {
    var ms = 0L
    var ended = false
}

/**
 * The audio and subtitle streams the user picked, by the server's indexes
 * (-1 turns the subtitles off), and the quality.
 */
private data class Selection(val audio: Int? = null, val subtitle: Int? = null, val quality: Quality = Quality.AUTO)

/**
 * The player, as the web UI's (apps/web player): the video across the
 * screen in landscape, and over it, shown by a tap, Back and the title at
 * the top, and the timeline, play and pause, the seeks, the subtitles and
 * the options at the bottom, which open a panel of the audio, its speed
 * and the subtitles. Media3 plays what it can and every stream the server
 * processes; libmpv plays the other sources directly (docs/clients.md
 * §4.3), and takes over from Media3 at its position when Media3 cannot
 * play a source after all. Picking other streams starts again where it
 * was, as the web UI's. The server hears the progress as it plays and
 * where it stopped.
 */
@OptIn(UnstableApi::class)
@Composable
fun PlayerScreen(model: AppModel, route: Route.Player) {
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    var playback by remember(route) { mutableStateOf<Playback?>(null) }
    var failure by remember(route) { mutableStateOf<String?>(null) }
    val prefs = LocalPrefs.current
    var selection by remember(route) { mutableStateOf(Selection(quality = prefs.quality)) }
    // Where the next start begins: the resume position, then where the streams changed.
    var resume by remember(route) { mutableStateOf(route.start) }
    var sourceId by remember(route) { mutableStateOf(route.mediaSourceId) }
    var details by remember(route) { mutableStateOf<Details?>(null) }
    Immersive()
    LaunchedEffect(route, selection) {
        playback = null
        var session: PlaybackSession? = null
        try {
            val item = details ?: runCatching { model.client.details(route.serverId, route.itemId) }.getOrNull()
            details = item
            session = model.play(
                route.serverId,
                route.itemId,
                PlaybackOptions(sourceId, selection.audio, selection.subtitle, selection.quality.bitrate(model.local(route.serverId)), resume, false),
            )
            val info = session.info()
            sourceId = info.mediaSourceId
            val source = item?.sources?.firstOrNull { it.serverId == route.serverId && it.mediaSourceId == info.mediaSourceId }
            val engine = engineFor(info, source)
            playback = Playback(session, info, item, item?.let { heading(context, it) }, engine)
        } catch (e: CancellationException) {
            session?.let { model.finish(route.serverId, it, resume) }
            throw e
        } catch (e: Exception) {
            failure = e.message ?: e.toString()
        }
    }
    val p = playback
    // The episode after this one, which its end plays when the device says so.
    val next = p?.details?.next
    var engine by remember(p) { mutableStateOf(p?.engine) }
    var startMs by remember(p) { mutableLongStateOf(p?.info?.startPosition?.toMillis() ?: 0) }
    val position = remember(p) { Position().also { it.ms = startMs } }
    // The subtitle offset, kept as streams change, as the web UI's.
    val shift = remember(route) { SubtitleShift() }
    val player = remember(p, engine) {
        // A group's member waits for the group to play.
        val follows = model.syncPlay.joined.value.playing(route.serverId, route.itemId) != null
        p?.let { createPlayer(context, engine ?: it.engine, it.info, it.source, startMs, autoplay = !follows, subtitles = shift) }
    }
    // The SyncPlay group playing the item, which the controls ask instead of the player.
    val group = rememberGroupPlayer(model, route.serverId, route.itemId, player) { navigator.back() }
    val currentGroup by rememberUpdatedState(group)
    val controls = remember(player) { player?.let { GroupControls(it) { currentGroup } } }
    val playNext = when {
        group != null && (group.hasNext || next != null) -> { { group.next(listOfNotNull(next?.card?.itemId)) } }
        else -> next?.let { e -> { navigator.replace(Route.Player(route.serverId, e.card.itemId, e.card.title, null, e.position)) } }
    }
    DisposableEffect(p) {
        onDispose {
            if (p != null && !position.ended) model.finish(route.serverId, p.session, Duration.ofMillis(position.ms))
        }
    }
    DisposableEffect(player) {
        val retry = player?.let { PlaybackRetry(it) }
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED && p != null && !position.ended) {
                    position.ended = true
                    model.finish(route.serverId, p.session, null)
                    val g = currentGroup
                    when {
                        g != null -> g.ended()
                        playNext != null && prefs.autoplayNext -> playNext()
                        else -> navigator.back()
                    }
                }
            }

            override fun onTracksChanged(tracks: Tracks) {
                if (p != null && player is ExoPlayer) chooseTracks(player, tracks, p.info, p.source)
            }

            override fun onPlayerError(error: PlaybackException) {
                if (retry?.retries(error) == true) return
                if (p != null && engine == PlayerEngine.MEDIA3 && p.info.kind == PlayKind.DIRECT_PLAY) {
                    startMs = player?.currentPosition ?: startMs
                    engine = PlayerEngine.MPV
                } else {
                    failure = error.message ?: error.errorCodeName
                }
            }
        }
        player?.addListener(listener)
        retry?.let { player?.addListener(it) }
        onDispose {
            retry?.cancel()
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
    // Leaving the app, or closing picture in picture, pauses.
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) { player?.pause() }
    // The system's media controls, headsets' keys and picture in picture's
    // buttons drive the player through its session; the next episode's
    // starts while this one's player goes, so each has an ID of its own.
    DisposableEffect(controls) {
        val session = controls?.let { MediaSession.Builder(context, it).setId(UUID.randomUUID().toString()).build() }
        onDispose { session?.release() }
    }
    val pip = PictureInPicture(player)
    val seek = rememberSeek(player)
    val loading = rememberBuffering(player)
    // How far up the subtitles go, a fraction of the height, above the controls while they show.
    var lift by remember { mutableFloatStateOf(0f) }
    LaunchedEffect(player, lift) { (player as? MpvPlayer)?.subtitleLift = lift }
    // The assistant controls the player as the user does.
    val currentNext by rememberUpdatedState(playNext)
    val currentSelection by rememberUpdatedState(selection)
    val assistantPlayer = remember(p, player, controls) {
        val pl = controls ?: player
        if (p == null || pl == null) return@remember null
        object : AssistantPlayer {
            override val playing
                get() = Assistant.playing(
                    route.itemId,
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
                    ControlCommand.CONTROL_COMMAND_STOP -> navigator.back()
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

    // The player is dark in either theme, as the web UI's.
    MavioTheme(dark = true, layout = theme.layout, type = theme.type) {
        Box(Modifier.fillMaxSize().background(Color.Black)) {
            when {
                player != null && engine == PlayerEngine.MPV -> PlayerSurface(player, Modifier.fillMaxSize())
                player != null -> {
                    ContentFrame(player, Modifier.fillMaxSize())
                    Subtitles(player, hidden = seek != null, lift)
                }
            }
            // A seek that has not landed hides the frame from before it.
            if (seek?.hidden == true) Box(Modifier.fillMaxSize().background(Color.Black))
            val error = failure
            if (error != null) {
                Failure(error)
            } else if (player != null && p != null) {
                // Picture in picture shows the video and its subtitles alone, and the activity indicator while it waits.
                if (pip.active) {
                    if (loading || seek?.hidden == true) CircularProgressIndicator(Modifier.align(Alignment.Center).size(32.dp), color = Color.White, strokeWidth = 3.dp)
                } else {
                    val chip: (@Composable (onOpen: (Boolean) -> Unit) -> Unit)? = group?.let { g -> { onOpen -> GroupChip(model, route.serverId, g.group, onOpen) } }
                    Controls(controls ?: player, route.title, p, selection.quality, shift, seek, loading, playNext, pip.enter, chip, { lift = it }) { picked ->
                        resume = Duration.ofMillis(player.currentPosition)
                        // The quality stays this device's, as the web UI's.
                        prefs.quality = picked.quality
                        selection = picked
                    }
                }
            } else {
                CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
            }
        }
    }
}

/**
 * The subtitles Media3 decodes, drawn by its own view over the video:
 * the Compose frame draws only the picture. libmpv draws its own. They
 * hide while [hidden], and stand [lift] of the height up from the foot
 * when that is above where they stand.
 */
@OptIn(UnstableApi::class)
@Composable
private fun Subtitles(player: Player, hidden: Boolean, lift: Float) {
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
        update = {
            it.visibility = if (hidden) View.INVISIBLE else View.VISIBLE
            it.setBottomPaddingFraction(maxOf(SubtitleView.DEFAULT_BOTTOM_PADDING_FRACTION, lift))
        },
    )
}

/** An episode's series and numbers, as "Atlas Station · S1 E2". */
private fun heading(context: Context, details: Details): String? {
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

/** The players open in a window, and its orientation before the first. */
private class ImmersiveCount(val orientation: Int) {
    var players = 0
}

/**
 * Landscape across the whole screen without the system's bars, the screen
 * kept on. The next episode's player opens before this one's goes, so the
 * window comes back only once the last player has gone.
 */
@Composable
private fun Immersive() {
    val activity = LocalActivity.current ?: return
    DisposableEffect(activity) {
        val window = activity.window
        val decor = window.decorView
        val controller = WindowCompat.getInsetsController(window, decor)
        val count = decor.getTag(AppR.id.immersive) as? ImmersiveCount
            ?: ImmersiveCount(activity.requestedOrientation).also { decor.setTag(AppR.id.immersive, it) }
        count.players++
        activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
        controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        controller.hide(WindowInsetsCompat.Type.systemBars())
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        onDispose {
            if (--count.players == 0) {
                decor.setTag(AppR.id.immersive, null)
                window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                controller.show(WindowInsetsCompat.Type.systemBars())
                activity.requestedOrientation = count.orientation
            }
        }
    }
}

/** How long the controls stay after the last touch while playing. */
private const val CONTROLS_TIMEOUT_MS = 3000L

/**
 * The controls over the video, shown by a tap and hidden by another or
 * after a while of playing.
 */
@OptIn(UnstableApi::class)
@kotlin.OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun Controls(
    player: Player,
    title: String,
    playback: Playback,
    quality: Quality,
    shift: SubtitleShift,
    seek: Seek?,
    loading: Boolean,
    playNext: (() -> Unit)?,
    pictureInPicture: (() -> Unit)?,
    chip: (@Composable (onOpen: (Boolean) -> Unit) -> Unit)?,
    onLift: (Float) -> Unit,
    onPick: (Selection) -> Unit,
) {
    val navigator = LocalNavigator.current
    val heading = playback.heading
    var panel by remember { mutableStateOf<Panel?>(null) }
    val playPause = rememberPlayPauseButtonState(player)
    val progress = rememberProgressStateWithTickInterval(player, 500, rememberCoroutineScope())
    var shown by remember { mutableStateOf(true) }
    var touched by remember { mutableLongStateOf(0L) }
    var seeking by remember { mutableStateOf<Float?>(null) }
    // The group's sheet, opened from the chip, keeps the controls that host it.
    var grouping by remember { mutableStateOf(false) }
    val touch = { touched = System.currentTimeMillis() }
    LaunchedEffect(shown, touched, playPause.showPlay, seeking, panel, grouping) {
        if (shown && !playPause.showPlay && seeking == null && panel == null && !grouping) {
            delay(CONTROLS_TIMEOUT_MS)
            shown = false
        }
    }
    // The activity indicator shows with the frame a seek hides.
    val buffering = loading || seek?.hidden == true
    // The subtitles stand above the bottom bar while it shows, as the web UI's.
    val density = LocalDensity.current
    var height by remember { mutableFloatStateOf(0f) }
    var barTop by remember { mutableFloatStateOf(0f) }
    val lift = if (shown && height > 0f && barTop > 0f) (height - barTop + with(density) { SUBTITLE_GAP.toPx() }) / height else 0f
    LaunchedEffect(lift) { onLift(lift) }
    DisposableEffect(Unit) { onDispose { onLift(0f) } }
    // A press held plays at double speed, then at the speed it played at.
    var heldSpeed by remember { mutableStateOf<Float?>(null) }
    val swipe = rememberSwipeLevels()
    Box(
        Modifier
            .fillMaxSize()
            .onSizeChanged { height = it.height.toFloat() }
            .pointerInput(player) {
                detectTapGestures(
                    onTap = {
                        if (panel != null) panel = null else shown = !shown
                        touch()
                    },
                    // A double tap seeks back on the left half and forward on the right.
                    onDoubleTap = { at ->
                        val end = player.duration.takeIf { it > 0 } ?: Long.MAX_VALUE
                        val by = if (at.x < size.width / 2) -SEEK_MS else SEEK_MS
                        player.seekTo((player.currentPosition + by).coerceIn(0, end))
                        touch()
                    },
                    onLongPress = {
                        // A group's speed stays the group's.
                        if (panel == null && player.isCommandAvailable(Player.COMMAND_SET_SPEED_AND_PITCH)) {
                            heldSpeed = player.playbackParameters.speed
                            player.setPlaybackSpeed(2f)
                        }
                    },
                    onPress = {
                        tryAwaitRelease()
                        heldSpeed?.let { player.setPlaybackSpeed(it) }
                        heldSpeed = null
                    },
                )
            }
            .pointerInput(swipe) { swipe.detect(this) { panel == null && heldSpeed == null } },
    ) {
        if (buffering) CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
        val level = swipe.shown
        when {
            heldSpeed != null -> Indicator(R.drawable.ic_speed_fill, Modifier.align(Alignment.TopCenter)) {
                Text("2×", style = theme.type.callout.text, fontWeight = FontWeight.SemiBold, color = Color.White)
            }
            level != null -> Indicator(level.icon, Modifier.align(Alignment.TopCenter)) {
                Box(Modifier.size(120.dp, 4.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.25f))) {
                    Box(Modifier.fillMaxWidth(level.value).fillMaxHeight().background(Color.White))
                }
            }
        }
        AnimatedVisibility(shown, Modifier.fillMaxSize(), enter = fadeIn(), exit = fadeOut()) {
            Box(Modifier.fillMaxSize()) {
                Row(
                    Modifier
                        .align(Alignment.TopCenter)
                        .fillMaxWidth()
                        .background(Brush.verticalGradient(listOf(Color.Black.copy(alpha = 0.75f), Color.Transparent)))
                        .windowInsetsPadding(WindowInsets.safeDrawing)
                        .padding(start = 20.dp, end = 20.dp, top = 16.dp, bottom = 48.dp),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PlayerButton(R.drawable.ic_back, stringResource(R.string.player_back)) { navigator.back() }
                    Column(Modifier.weight(1f)) {
                        heading?.let {
                            Text(it, style = theme.type.footnote.text, color = Color.White.copy(alpha = 0.7f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                        Text(
                            title,
                            style = theme.type.title3.text,
                            fontWeight = FontWeight.SemiBold,
                            color = Color.White,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    chip?.invoke { grouping = it }
                }
                Column(
                    Modifier
                        .align(Alignment.BottomCenter)
                        .fillMaxWidth()
                        .background(
                            Brush.verticalGradient(
                                0f to Color.Transparent,
                                0.5f to Color.Black.copy(alpha = 0.4f),
                                1f to Color.Black.copy(alpha = 0.85f),
                            ),
                        )
                        .windowInsetsPadding(WindowInsets.safeDrawing)
                        .padding(start = 20.dp, end = 20.dp, top = 64.dp, bottom = 16.dp),
                ) {
                    val duration = progress.durationMs.coerceAtLeast(0)
                    // A seek's time shows at once.
                    val now = seeking?.toLong() ?: seek?.targetMs ?: progress.currentPositionMs
                    SeekBar(
                        now,
                        duration,
                        playback,
                        Modifier.onGloballyPositioned { barTop = it.positionInRoot().y },
                        onScrub = {
                            seeking = it?.toFloat()
                            touch()
                        },
                        onSeek = { player.seekTo(it) },
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
                        PlayerButton(
                            if (playPause.showPlay) R.drawable.ic_play_fill else R.drawable.ic_pause_fill,
                            stringResource(if (playPause.showPlay) R.string.player_play else R.string.player_pause),
                            large = true,
                            enabled = playPause.isEnabled,
                        ) {
                            playPause.onClick()
                            touch()
                        }
                        PlayerButton(R.drawable.ic_seek_back, pluralStringResource(R.plurals.player_seek_back, 10, 10)) {
                            player.seekTo((player.currentPosition - SEEK_MS).coerceAtLeast(0))
                            touch()
                        }
                        PlayerButton(R.drawable.ic_seek_forward, pluralStringResource(R.plurals.player_seek_forward, 10, 10)) {
                            val end = if (duration > 0) duration else Long.MAX_VALUE
                            player.seekTo((player.currentPosition + SEEK_MS).coerceAtMost(end))
                            touch()
                        }
                        Text(
                            "${clock(now)} / ${clock(duration)}",
                            Modifier.padding(start = 8.dp).weight(1f),
                            style = theme.type.footnote.text,
                            color = Color.White.copy(alpha = 0.85f),
                        )
                        playNext?.let { onClick ->
                            Row(
                                Modifier
                                    .padding(end = 4.dp)
                                    .height(36.dp)
                                    .clip(CircleShape)
                                    .background(Color.White.copy(alpha = 0.15f))
                                    .clickable(onClick = onClick)
                                    .padding(horizontal = 14.dp),
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                MavioIcon(R.drawable.ic_next_fill, size = 16.dp, tint = Color.White)
                                Text(
                                    stringResource(R.string.player_next_episode),
                                    style = theme.type.footnote.text,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color.White,
                                )
                            }
                        }
                        LocalAssistant.current?.let { AssistantButton(it, tint = Color.White, size = 20.dp) }
                        if (playback.info.subtitles.isNotEmpty()) {
                            val on = (playback.info.subtitleStreamIndex ?: -1) >= 0
                            PlayerButton(
                                if (on) R.drawable.ic_subtitles_fill else R.drawable.ic_subtitles,
                                stringResource(R.string.player_subtitles),
                                active = panel == Panel.SUBTITLES,
                            ) {
                                panel = if (panel == Panel.SUBTITLES) null else Panel.SUBTITLES
                                touch()
                            }
                        }
                        PlayerButton(
                            R.drawable.ic_settings,
                            stringResource(R.string.player_options),
                            active = panel != null && panel != Panel.SUBTITLES,
                        ) {
                            panel = if (panel == Panel.AUDIO) null else Panel.AUDIO
                            touch()
                        }
                        pictureInPicture?.let { enter ->
                            PlayerButton(R.drawable.ic_picture_in_picture, stringResource(R.string.player_picture_in_picture)) {
                                panel = null
                                enter()
                            }
                        }
                    }
                }
            }
        }
        panel?.let { open ->
            OptionsPanel(
                open,
                player,
                playback,
                quality,
                shift,
                Modifier
                    .align(Alignment.BottomEnd)
                    .windowInsetsPadding(WindowInsets.safeDrawing)
                    .padding(end = 16.dp, bottom = 88.dp),
                onTab = { panel = it },
            ) { picked ->
                panel = null
                onPick(picked)
            }
        }
    }
}

/** The tabs of the player's panel. */
private enum class Panel(val label: Int) {
    AUDIO(R.string.player_audio),
    SUBTITLES(R.string.player_subtitles),
    QUALITY(R.string.player_quality),
    INFO(R.string.player_info),
}

/** The speeds the player offers. */
private val SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f)

/**
 * The panel of the player's options, as the web UI's: dark glass over the
 * video's corner, with the tabs of the audio streams and the speed, and of
 * the subtitles.
 */
@Composable
private fun OptionsPanel(
    panel: Panel,
    player: Player,
    playback: Playback,
    quality: Quality,
    shift: SubtitleShift,
    modifier: Modifier,
    onTab: (Panel) -> Unit,
    onPick: (Selection) -> Unit,
) {
    val info = playback.info
    val source = playback.source
    val current = Selection(info.audioStreamIndex, info.subtitleStreamIndex ?: -1, quality)
    val context = LocalContext.current
    // Without subtitles there is no tab for them, as the web UI's.
    val tabs = Panel.entries.filter { it != Panel.SUBTITLES || info.subtitles.isNotEmpty() }
    Column(
        modifier
            .width(360.dp)
            .heightIn(max = 320.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(Color(0xD9171717))
            .border(1.dp, Color.White.copy(alpha = 0.1f), RoundedCornerShape(18.dp))
            // Taps inside stay inside.
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {},
    ) {
        Row(
            Modifier
                .padding(12.dp)
                .fillMaxWidth()
                .clip(RoundedCornerShape(9.dp))
                .background(Color.White.copy(alpha = 0.1f))
                .padding(2.dp),
        ) {
            tabs.forEach { tab ->
                val active = tab == panel
                Text(
                    stringResource(tab.label),
                    Modifier
                        .weight(1f)
                        .clip(RoundedCornerShape(7.dp))
                        .background(if (active) Color.White.copy(alpha = 0.2f) else Color.Transparent)
                        .clickable { onTab(tab) }
                        .padding(vertical = 4.dp),
                    style = theme.type.callout.text,
                    fontWeight = FontWeight.SemiBold,
                    color = if (active) Color.White else Color.White.copy(alpha = 0.6f),
                    textAlign = TextAlign.Center,
                )
            }
        }
        Column(
            Modifier
                .verticalScroll(rememberScrollState())
                .padding(start = 8.dp, end = 8.dp, bottom = 12.dp),
        ) {
            when (panel) {
                Panel.AUDIO -> {
                    source?.audio.orEmpty().forEach { a ->
                        Choice(a.index == info.audioStreamIndex, audioLabel(a)) { onPick(current.copy(audio = a.index)) }
                    }
                    Text(
                        stringResource(R.string.player_speed),
                        Modifier.padding(start = 10.dp, top = 12.dp, bottom = 6.dp),
                        style = theme.type.footnote.text,
                        fontWeight = FontWeight.SemiBold,
                        color = Color.White.copy(alpha = 0.6f),
                    )
                    var speed by remember { mutableFloatStateOf(player.playbackParameters.speed) }
                    FlowRow(
                        Modifier.padding(horizontal = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        // A group's speed stays the group's.
                        val free = player.isCommandAvailable(Player.COMMAND_SET_SPEED_AND_PITCH)
                        SPEEDS.forEach { r ->
                            val on = r == speed
                            Text(
                                "${if (r % 1f == 0f) r.toInt().toString() else r.toString()}×",
                                Modifier
                                    .alpha(if (free) 1f else 0.4f)
                                    .clip(CircleShape)
                                    .background(Color.White.copy(alpha = if (on) 0.3f else 0.1f))
                                    .clickable(enabled = free) {
                                        player.setPlaybackSpeed(r)
                                        speed = r
                                    }
                                    .padding(horizontal = 12.dp, vertical = 6.dp),
                                style = theme.type.footnote.text,
                                fontWeight = FontWeight.SemiBold,
                                color = Color.White,
                            )
                        }
                    }
                }
                Panel.SUBTITLES -> {
                    val chosen = info.subtitleStreamIndex ?: -1
                    Choice(chosen < 0, stringResource(R.string.player_subtitles_off) to "") { onPick(current.copy(subtitle = -1)) }
                    info.subtitles.forEach { t ->
                        val stream = source?.subtitles?.firstOrNull { it.index == t.streamIndex }
                        val label = stream?.let { subtitleLabel(it) }
                            ?: ((t.title ?: languageName(t.language) ?: "#${t.streamIndex}") to t.format.uppercase())
                        Choice(t.streamIndex == chosen, label) { onPick(current.copy(subtitle = t.streamIndex)) }
                    }
                    if (chosen >= 0) SubtitleOffset(shift)
                }
                Panel.QUALITY -> Quality.entries.forEach { q ->
                    Choice(q == quality, (q.label?.let { stringResource(it) } ?: q.key) to "") {
                        if (q != quality) onPick(current.copy(quality = q))
                    }
                }
                Panel.INFO -> Column(
                    Modifier.padding(horizontal = 10.dp).padding(top = 4.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
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
                        PanelAction(R.drawable.ic_copy, stringResource(R.string.player_copy_link)) {
                            context.getSystemService(ClipboardManager::class.java)
                                ?.setPrimaryClip(ClipData.newPlainText(null, info.url))
                        }
                    }
                }
            }
        }
    }
}

/** A fact of the playback in the panel: its label over its value. */
@Composable
private fun Fact(label: String, value: String) {
    Column {
        Text(label, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = Color.White.copy(alpha = 0.6f))
        Text(value, Modifier.padding(top = 2.dp), style = theme.type.callout.text, color = Color.White)
    }
}

/** An action in the panel's information: its icon and its name. */
@Composable
private fun PanelAction(icon: Int, label: String, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(icon, size = 18.dp, tint = Color.White.copy(alpha = 0.7f))
        Text(label, style = theme.type.callout.text, color = Color.White)
    }
}

/** Picture in picture: whether the player is in it, and how to go in, where the device has it. */
private class Pip(val active: Boolean, val enter: (() -> Unit)?)

/**
 * Picture in picture for the player, as the web UI's: its button, and on
 * Android 12 and later, going in by itself when the user leaves the app
 * while a video plays.
 */
@Composable
private fun PictureInPicture(player: Player?): Pip {
    val activity = LocalActivity.current as? ComponentActivity
    var active by remember { mutableStateOf(activity?.isInPictureInPictureMode == true) }
    val supported = activity?.packageManager?.hasSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE) == true
    if (activity == null || !supported || player == null) return Pip(active, null)
    // Back from picture in picture to the player plays on, as the web UI's;
    // closing it stops the activity, which pauses.
    var restoring by remember { mutableStateOf(false) }
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) {
        if (restoring) player.play()
        restoring = false
    }
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) { restoring = false }
    DisposableEffect(activity) {
        val listener = Consumer<PictureInPictureModeChangedInfo> {
            if (active && !it.isInPictureInPictureMode) restoring = true
            active = it.isInPictureInPictureMode
        }
        activity.addOnPictureInPictureModeChangedListener(listener)
        onDispose { activity.removeOnPictureInPictureModeChangedListener(listener) }
    }
    var size by remember(player) { mutableStateOf(player.videoSize) }
    var playing by remember(player) { mutableStateOf(player.isPlaying) }
    DisposableEffect(player) {
        val listener = object : Player.Listener {
            override fun onVideoSizeChanged(videoSize: VideoSize) {
                size = videoSize
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                playing = isPlaying
            }
        }
        player.addListener(listener)
        onDispose { player.removeListener(listener) }
    }
    val params = remember(size, playing) { pipParams(size, playing) }
    DisposableEffect(params) {
        activity.setPictureInPictureParams(params)
        onDispose { activity.setPictureInPictureParams(pipParams(VideoSize.UNKNOWN, false)) }
    }
    return Pip(active) { activity.enterPictureInPictureMode(params) }
}

/** The window of picture in picture: the video's shape, within what Android takes, and going in by itself while it plays. */
private fun pipParams(size: VideoSize, playing: Boolean): PictureInPictureParams {
    val builder = PictureInPictureParams.Builder()
    if (size.width > 0 && size.height > 0) {
        val ratio = (size.width * size.pixelWidthHeightRatio / size.height).coerceIn(1f / 2.39f, 2.39f)
        builder.setAspectRatio(Rational((ratio * 1000).toInt(), 1000))
    }
    if (Build.VERSION.SDK_INT >= 31) builder.setAutoEnterEnabled(playing)
    return builder.build()
}

/** A stream to pick in the panel: its name, its details below, and a check when it plays. */
@Composable
private fun Choice(checked: Boolean, label: Pair<String, String>, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(16.dp)) {
            if (checked) MavioIcon(R.drawable.ic_check, size = 16.dp, tint = theme.colors.accent)
        }
        Column(Modifier.weight(1f)) {
            Text(label.first, style = theme.type.callout.text, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (label.second.isNotEmpty()) {
                Text(
                    label.second,
                    style = theme.type.caption.text,
                    color = Color.White.copy(alpha = 0.6f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
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

/** How wide the seek bar's thumbnails are. */
private val PREVIEW_WIDTH = 160.dp

/**
 * The seek bar, as the web UI's: the track with gaps where chapters start,
 * inset by half its thumb so that the thumb stays inside the bar at either
 * end, and the thumb, which a touch moves; while it does, the trickplay
 * thumbnail there with the chapter's name and the time above it, kept
 * inside the bar's ends. [onScrub] hears where it is, and null once it
 * lets go; [onSeek] where it let go.
 */
@Composable
private fun SeekBar(positionMs: Long, durationMs: Long, playback: Playback, modifier: Modifier, onScrub: (Long?) -> Unit, onSeek: (Long) -> Unit) {
    val density = LocalDensity.current
    val chapters = playback.source?.chapters.orEmpty()
    val marks = remember(chapters, durationMs) {
        if (durationMs > 0) chapters.map { it.start.toMillis().toFloat() / durationMs } else emptyList()
    }
    val sheets = remember(playback.details) {
        chooseSheets(playback.details?.trickplay.orEmpty(), with(density) { PREVIEW_WIDTH.roundToPx() })
    }
    var scrub by remember { mutableStateOf<Long?>(null) }
    val fraction = if (durationMs > 0) (positionMs.toFloat() / durationMs).coerceIn(0f, 1f) else 0f
    val currentDuration by rememberUpdatedState(durationMs)
    Box(
        modifier
            .fillMaxWidth()
            .height(32.dp)
            .pointerInput(Unit) {
                val inset = THUMB_SIZE.toPx() / 2
                fun at(x: Float) = ((x - inset) / (size.width - 2 * inset).coerceAtLeast(1f)).coerceIn(0f, 1f).let { (it * currentDuration).toLong() }
                awaitEachGesture {
                    val down = awaitFirstDown()
                    if (currentDuration <= 0) return@awaitEachGesture
                    down.consume()
                    var ms = at(down.position.x)
                    scrub = ms
                    onScrub(ms)
                    while (true) {
                        val change = awaitPointerEvent().changes.firstOrNull { it.id == down.id } ?: break
                        if (!change.pressed) break
                        change.consume()
                        ms = at(change.position.x)
                        scrub = ms
                        onScrub(ms)
                    }
                    scrub = null
                    onSeek(ms)
                    onScrub(null)
                }
            },
        contentAlignment = Alignment.CenterStart,
    ) {
        Box(Modifier.fillMaxWidth().padding(horizontal = THUMB_SIZE / 2), contentAlignment = Alignment.CenterStart) {
            ChapterTrack(fraction, marks, if (scrub != null) 6.dp else 4.dp, Modifier.fillMaxWidth())
            // The thumb at the position, and the preview above it while dragged.
            Box(
                Modifier
                    .layout { measurable, constraints ->
                        val thumb = measurable.measure(constraints.copy(minWidth = 0, minHeight = 0))
                        layout(constraints.maxWidth, thumb.height) {
                            thumb.place((fraction * constraints.maxWidth - thumb.width / 2f).toInt(), 0)
                        }
                    }
                    .size(THUMB_SIZE)
                    .background(Color.White, CircleShape),
            )
            scrub?.let { ms ->
                Box(
                    Modifier.layout { measurable, constraints ->
                        val preview = measurable.measure(constraints.copy(minWidth = 0, minHeight = 0))
                        val x = (fraction * constraints.maxWidth - preview.width / 2f).toInt()
                            .coerceIn(0, (constraints.maxWidth - preview.width).coerceAtLeast(0))
                        layout(constraints.maxWidth, 0) { preview.place(x, -preview.height - 20.dp.roundToPx()) }
                    },
                ) {
                    SeekPreview(sheets, ms, chapterAt(chapters, ms), clock(ms), PREVIEW_WIDTH, theme.type.footnote.text)
                }
            }
        }
    }
}

/** How wide the seek bar's thumb is. */
private val THUMB_SIZE = 14.dp

/** How far above the bottom bar the subtitles stand while it shows. */
private val SUBTITLE_GAP = 8.dp

/**
 * The subtitle offset, as the web UI's: a tenth of a second earlier or
 * later at a time, and back to none.
 */
@Composable
private fun SubtitleOffset(shift: SubtitleShift) {
    var ms by remember { mutableLongStateOf(shift.ms) }
    val set = { to: Long ->
        ms = to
        shift.ms = to
    }
    Text(
        stringResource(R.string.player_subtitle_offset),
        Modifier.padding(start = 10.dp, top = 12.dp, bottom = 6.dp),
        style = theme.type.footnote.text,
        fontWeight = FontWeight.SemiBold,
        color = Color.White.copy(alpha = 0.6f),
    )
    Row(Modifier.padding(horizontal = 10.dp), horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
        OffsetButton(stringResource(R.string.player_offset_earlier)) { set(ms - 100) }
        Text(
            offsetText(ms),
            Modifier.widthIn(min = 64.dp),
            style = theme.type.callout.text,
            color = Color.White,
            textAlign = TextAlign.Center,
        )
        OffsetButton(stringResource(R.string.player_offset_later)) { set(ms + 100) }
        OffsetButton(stringResource(R.string.player_offset_reset), enabled = ms != 0L) { set(0) }
    }
}

/** A button of the subtitle offset, as the speeds'. */
@Composable
private fun OffsetButton(label: String, enabled: Boolean = true, onClick: () -> Unit) {
    Text(
        label,
        Modifier
            .alpha(if (enabled) 1f else 0.4f)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.1f))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 6.dp),
        style = theme.type.footnote.text,
        fontWeight = FontWeight.SemiBold,
        color = Color.White,
    )
}

/** An offset in milliseconds as "+0.3 s", as the web UI's. */
private fun offsetText(ms: Long): String =
    if (ms == 0L) "0 s" else "${if (ms > 0) "+" else "−"}${"%.1f".format(abs(ms) / 1000.0)} s"

/** How far the seek buttons move. */
private const val SEEK_MS = 10_000L

/** A round button of the player, its icon white. */
@Composable
private fun PlayerButton(
    @DrawableRes icon: Int,
    label: String,
    large: Boolean = false,
    enabled: Boolean = true,
    active: Boolean = false,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .size(if (large) 48.dp else 44.dp)
            .clip(CircleShape)
            .background(if (active) Color.White.copy(alpha = 0.15f) else Color.Transparent)
            .clickable(enabled = enabled, onClickLabel = label, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        MavioIcon(icon, size = if (large) 30.dp else 24.dp, tint = Color.White, contentDescription = label)
    }
}

/** Why the item could not play, and Back. */
@Composable
private fun Failure(message: String) {
    val navigator = LocalNavigator.current
    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.7f)), contentAlignment = Alignment.Center) {
        Column(
            Modifier.widthIn(max = 448.dp).padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            MavioIcon(R.drawable.ic_error, size = 48.dp, tint = Color.White.copy(alpha = 0.7f))
            Text(message, style = theme.type.body.text, color = Color.White)
            Text(
                stringResource(R.string.player_back),
                Modifier
                    .clip(RoundedCornerShape(10.dp))
                    .background(Color.White.copy(alpha = 0.15f))
                    .clickable { navigator.back() }
                    .padding(horizontal = 16.dp, vertical = 8.dp),
                style = theme.type.headline.text,
                color = Color.White,
            )
        }
    }
}

/** A time as "1:04:05", or "4:05" under an hour. */
private fun clock(ms: Long): String {
    val s = (ms / 1000).coerceAtLeast(0)
    val h = s / 3600
    val m = s % 3600 / 60
    val sec = s % 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
}
