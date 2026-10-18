package dev.mavio.app

import android.content.Context
import android.content.pm.ActivityInfo
import android.net.Uri
import android.view.WindowManager
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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
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
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.common.text.CueGroup
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.SubtitleView
import androidx.media3.ui.compose.ContentFrame
import androidx.media3.ui.compose.PlayerSurface
import androidx.media3.ui.compose.state.rememberPlayPauseButtonState
import androidx.media3.ui.compose.state.rememberProgressStateWithTickInterval
import dev.mavio.core.AppModel
import dev.mavio.core.AudioStream
import dev.mavio.core.Details
import dev.mavio.core.DeviceProfile
import dev.mavio.core.PlayKind
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.PlaybackOptions
import dev.mavio.core.PlaybackSession
import dev.mavio.core.Source
import dev.mavio.core.SubtitleStream
import dev.mavio.player.MpvPlayer
import dev.mavio.player.PlayerEngine
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.languageName
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay

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

/** The audio and subtitle streams the user picked, by the server's indexes; -1 turns the subtitles off. */
private data class Selection(val audio: Int? = null, val subtitle: Int? = null)

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
    var selection by remember(route) { mutableStateOf(Selection()) }
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
                PlaybackOptions(sourceId, selection.audio, selection.subtitle, null, resume, false),
            )
            val info = session.info()
            sourceId = info.mediaSourceId
            val source = item?.sources?.firstOrNull { it.serverId == route.serverId && it.mediaSourceId == info.mediaSourceId }
            val engine = if (info.kind == PlayKind.DIRECT_PLAY && source != null && !DeviceProfile.media3Plays(source)) {
                PlayerEngine.MPV
            } else {
                PlayerEngine.MEDIA3
            }
            playback = Playback(session, info, item, item?.let { heading(context, it) }, engine)
        } catch (e: CancellationException) {
            session?.let { model.finish(route.serverId, it, resume) }
            throw e
        } catch (e: Exception) {
            failure = e.message ?: e.toString()
        }
    }
    val p = playback
    var engine by remember(p) { mutableStateOf(p?.engine) }
    var startMs by remember(p) { mutableLongStateOf(p?.info?.startPosition?.toMillis() ?: 0) }
    val position = remember(p) { Position().also { it.ms = startMs } }
    val player = remember(p, engine) { p?.let { createPlayer(context, engine ?: it.engine, it, startMs) } }
    DisposableEffect(p) {
        onDispose {
            if (p != null && !position.ended) model.finish(route.serverId, p.session, Duration.ofMillis(position.ms))
        }
    }
    DisposableEffect(player) {
        val listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED && p != null && !position.ended) {
                    position.ended = true
                    model.finish(route.serverId, p.session, null)
                    navigator.back()
                }
            }

            override fun onTracksChanged(tracks: Tracks) {
                if (p != null && player is ExoPlayer) chooseTracks(player, tracks, p)
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
            Controls(player, route.title, p) { picked ->
                resume = Duration.ofMillis(player.currentPosition)
                selection = picked
            }
        } else {
            CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
        }
    }
}

/**
 * The subtitles Media3 decodes, drawn by its own view over the video:
 * the Compose frame draws only the picture. libmpv draws its own.
 */
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

/** Media3 with its FFmpeg extension, or libmpv, playing from startMs once prepared, with the streams picked. */
@OptIn(UnstableApi::class)
private fun createPlayer(context: Context, engine: PlayerEngine, playback: Playback, startMs: Long): Player {
    val info = playback.info
    val player = when (engine) {
        PlayerEngine.MPV -> MpvPlayer(context).apply { mpvTracks(playback)?.let { (audio, subtitle) -> selectTracks(audio, subtitle) } }
        PlayerEngine.MEDIA3 -> {
            val renderers = DefaultRenderersFactory(context)
                .setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
            ExoPlayer.Builder(context, renderers).build().apply {
                // Off unless a subtitle was picked; chooseTracks picks it.
                trackSelectionParameters = trackSelectionParameters.buildUpon()
                    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, (info.subtitleStreamIndex ?: -1) < 0)
                    .build()
            }
        }
    }
    val item = MediaItem.Builder().setUri(info.url)
    if (".m3u8" in info.url) item.setMimeType(MimeTypes.APPLICATION_M3U8)
    // The subtitle picked, when it is a file of its own.
    val external = info.subtitles.firstOrNull { it.streamIndex == info.subtitleStreamIndex && it.url != null }
    val mime = external?.let { subtitleMime(it.format) }
    if (external != null && mime != null) {
        item.setSubtitleConfigurations(
            listOf(
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(external.url))
                    .setId(EXTERNAL_SUBTITLE)
                    .setMimeType(mime)
                    .setLanguage(external.language)
                    .setLabel(external.title)
                    .setSelectionFlags(C.SELECTION_FLAG_DEFAULT)
                    .build(),
            ),
        )
    }
    player.setMediaItem(item.build(), startMs)
    player.prepare()
    player.playWhenReady = true
    return player
}

/** The ID Media3 gives the subtitle file added beside the media. */
private const val EXTERNAL_SUBTITLE = "mavio-external"

/** The MIME type of a subtitle file's format, for those Media3 reads. */
private fun subtitleMime(format: String): String? = when (format.lowercase()) {
    "srt", "subrip" -> MimeTypes.APPLICATION_SUBRIP
    "vtt", "webvtt" -> MimeTypes.TEXT_VTT
    "ass", "ssa" -> MimeTypes.TEXT_SSA
    "ttml" -> MimeTypes.APPLICATION_TTML
    else -> null
}

/**
 * The tracks libmpv plays from the file the server sends whole: the
 * position of the picked streams among the file's audio and its embedded
 * subtitles, from 1, and 0 for the subtitles off or shown from a file of
 * their own.
 */
private fun mpvTracks(playback: Playback): Pair<Int?, Int?>? {
    val source = playback.source ?: return null
    val info = playback.info
    val audio = info.audioStreamIndex?.let { i -> source.audio.sortedBy { it.index }.indexOfFirst { it.index == i } + 1 }?.takeIf { it > 0 }
    val chosen = info.subtitleStreamIndex ?: -1
    val embedded = source.subtitles.filter { !it.external }.sortedBy { it.index }
    val subtitle = if (chosen < 0) 0 else embedded.indexOfFirst { it.index == chosen } + 1
    return audio to subtitle
}

/**
 * Picks in Media3 the streams the server says play: in a file sent whole,
 * the audio and the embedded subtitle at their positions among the file's
 * tracks; the subtitle file added beside it; in a stream the server
 * processes, its one subtitle track.
 */
@OptIn(UnstableApi::class)
private fun chooseTracks(player: ExoPlayer, tracks: Tracks, playback: Playback) {
    val info = playback.info
    val params = player.trackSelectionParameters.buildUpon()
    val audioGroups = tracks.groups.filter { it.type == C.TRACK_TYPE_AUDIO }
    val textGroups = tracks.groups.filter { it.type == C.TRACK_TYPE_TEXT }
    val direct = info.kind == PlayKind.DIRECT_PLAY
    val source = playback.source
    if (direct && source != null && audioGroups.size > 1) {
        val at = source.audio.sortedBy { it.index }.indexOfFirst { it.index == info.audioStreamIndex }
        audioGroups.getOrNull(at)?.let { params.setOverrideForType(TrackSelectionOverride(it.mediaTrackGroup, 0)) }
    }
    val chosen = info.subtitleStreamIndex ?: -1
    if (chosen >= 0) {
        val external = textGroups.firstOrNull { g -> (0 until g.length).any { g.getTrackFormat(it).id?.contains(EXTERNAL_SUBTITLE) == true } }
        val own = textGroups.filter { it != external }
        val group = when {
            external != null -> external
            direct && source != null -> {
                val at = source.subtitles.filter { !it.external }.sortedBy { it.index }.indexOfFirst { it.index == chosen }
                own.getOrNull(at)
            }
            else -> own.firstOrNull()
        }
        group?.let { params.setOverrideForType(TrackSelectionOverride(it.mediaTrackGroup, 0)) }
    }
    val built = params.build()
    if (built != player.trackSelectionParameters) player.trackSelectionParameters = built
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

/** Landscape across the whole screen without the system's bars, the screen kept on. */
@Composable
private fun Immersive() {
    val activity = LocalActivity.current ?: return
    DisposableEffect(activity) {
        val window = activity.window
        val controller = WindowCompat.getInsetsController(window, window.decorView)
        val orientation = activity.requestedOrientation
        activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
        controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        controller.hide(WindowInsetsCompat.Type.systemBars())
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        onDispose {
            window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
            controller.show(WindowInsetsCompat.Type.systemBars())
            activity.requestedOrientation = orientation
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
private fun Controls(player: Player, title: String, playback: Playback, onPick: (Selection) -> Unit) {
    val navigator = LocalNavigator.current
    val heading = playback.heading
    var panel by remember { mutableStateOf<Panel?>(null) }
    val playPause = rememberPlayPauseButtonState(player)
    val progress = rememberProgressStateWithTickInterval(player, 500, rememberCoroutineScope())
    var shown by remember { mutableStateOf(true) }
    var touched by remember { mutableLongStateOf(0L) }
    var seeking by remember { mutableStateOf<Float?>(null) }
    val touch = { touched = System.currentTimeMillis() }
    LaunchedEffect(shown, touched, playPause.showPlay, seeking, panel) {
        if (shown && !playPause.showPlay && seeking == null && panel == null) {
            delay(CONTROLS_TIMEOUT_MS)
            shown = false
        }
    }
    val buffering = player.playbackState == Player.STATE_BUFFERING || player.playbackState == Player.STATE_IDLE
    Box(
        Modifier
            .fillMaxSize()
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {
                if (panel != null) {
                    panel = null
                } else {
                    shown = !shown
                }
                touch()
            },
    ) {
        if (buffering) CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
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
                    Column {
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
                    val now = seeking?.toLong() ?: progress.currentPositionMs
                    Slider(
                        value = now.coerceIn(0, duration.coerceAtLeast(1)).toFloat(),
                        onValueChange = {
                            seeking = it
                            touch()
                        },
                        onValueChangeFinished = {
                            seeking?.let { player.seekTo(it.toLong()) }
                            seeking = null
                        },
                        valueRange = 0f..duration.coerceAtLeast(1).toFloat(),
                        // A slim bar, as the web UI's timeline.
                        thumb = { Box(Modifier.size(14.dp).background(Color.White, CircleShape)) },
                        track = { state ->
                            SliderDefaults.Track(
                                state,
                                Modifier.height(4.dp),
                                colors = SliderDefaults.colors(
                                    activeTrackColor = Color.White,
                                    inactiveTrackColor = Color.White.copy(alpha = 0.3f),
                                ),
                                drawStopIndicator = null,
                                thumbTrackGapSize = 0.dp,
                            )
                        },
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
                            active = panel == Panel.AUDIO,
                        ) {
                            panel = if (panel == Panel.AUDIO) null else Panel.AUDIO
                            touch()
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
    modifier: Modifier,
    onTab: (Panel) -> Unit,
    onPick: (Selection) -> Unit,
) {
    val info = playback.info
    val source = playback.source
    val current = Selection(info.audioStreamIndex, info.subtitleStreamIndex ?: -1)
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
            Panel.entries.forEach { tab ->
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
                        SPEEDS.forEach { r ->
                            val on = r == speed
                            Text(
                                "${if (r % 1f == 0f) r.toInt().toString() else r.toString()}×",
                                Modifier
                                    .clip(CircleShape)
                                    .background(if (on) Color.White else Color.White.copy(alpha = 0.1f))
                                    .clickable {
                                        player.setPlaybackSpeed(r)
                                        speed = r
                                    }
                                    .padding(horizontal = 12.dp, vertical = 6.dp),
                                style = theme.type.footnote.text,
                                fontWeight = FontWeight.SemiBold,
                                color = if (on) Color.Black else Color.White,
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
                }
            }
        }
    }
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
