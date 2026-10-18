package dev.mavio.app

import android.content.Context
import android.content.pm.ActivityInfo
import android.view.WindowManager
import androidx.activity.compose.LocalActivity
import androidx.annotation.DrawableRes
import androidx.annotation.OptIn
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.compose.ContentFrame
import androidx.media3.ui.compose.PlayerSurface
import androidx.media3.ui.compose.state.rememberPlayPauseButtonState
import androidx.media3.ui.compose.state.rememberProgressStateWithTickInterval
import dev.mavio.core.AppModel
import dev.mavio.core.Details
import dev.mavio.core.DeviceProfile
import dev.mavio.core.PlayKind
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.PlaybackOptions
import dev.mavio.core.PlaybackSession
import dev.mavio.player.MpvPlayer
import dev.mavio.player.PlayerEngine
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay

/** A playback the server started, with the engine that plays it. */
private class Playback(
    val session: PlaybackSession,
    val info: PlaybackInfo,
    /** The series and episode above the title, for an episode. */
    val heading: String?,
    val engine: PlayerEngine,
)

/** Where the player was, kept as it goes so the server hears it when the player closes. */
private class Position {
    var ms = 0L
    var ended = false
}

/**
 * The player, as the web UI's (apps/web player): the video across the
 * screen in landscape, and over it, shown by a tap, Back and the title at
 * the top, and the timeline, play and pause, and the seeks at the bottom.
 * Media3 plays what it can and every stream the server processes; libmpv
 * plays the other sources directly (docs/clients.md §4.3), and takes over
 * from Media3 at its position when Media3 cannot play a source after all.
 * The server hears the progress as it plays and where it stopped.
 */
@OptIn(UnstableApi::class)
@Composable
fun PlayerScreen(model: AppModel, route: Route.Player) {
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    var playback by remember(route) { mutableStateOf<Playback?>(null) }
    var failure by remember(route) { mutableStateOf<String?>(null) }
    Immersive()
    LaunchedEffect(route) {
        var session: PlaybackSession? = null
        try {
            val details = runCatching { model.client.details(route.serverId, route.itemId) }.getOrNull()
            session = model.play(
                route.serverId,
                route.itemId,
                PlaybackOptions(route.mediaSourceId, null, null, null, route.start, false),
            )
            val info = session.info()
            val source = details?.sources?.firstOrNull { it.serverId == route.serverId && it.mediaSourceId == info.mediaSourceId }
            val engine = if (info.kind == PlayKind.DIRECT_PLAY && source != null && !DeviceProfile.media3Plays(source)) {
                PlayerEngine.MPV
            } else {
                PlayerEngine.MEDIA3
            }
            playback = Playback(session, info, details?.let { heading(context, it) }, engine)
        } catch (e: CancellationException) {
            session?.let { model.finish(route.serverId, it, route.start) }
            throw e
        } catch (e: Exception) {
            failure = e.message ?: e.toString()
        }
    }
    val p = playback
    var engine by remember(p) { mutableStateOf(p?.engine) }
    var startMs by remember(p) { mutableLongStateOf(p?.info?.startPosition?.toMillis() ?: 0) }
    val position = remember(p) { Position().also { it.ms = startMs } }
    val player = remember(p, engine) { p?.let { createPlayer(context, engine ?: it.engine, it.info, startMs) } }
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
            player != null -> ContentFrame(player, Modifier.fillMaxSize())
        }
        val error = failure
        if (error != null) {
            Failure(error)
        } else if (player != null && p != null) {
            Controls(player, route.title, p.heading)
        } else {
            CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
        }
    }
}

/** Media3 with its FFmpeg extension, or libmpv, playing from startMs once prepared. */
@OptIn(UnstableApi::class)
private fun createPlayer(context: Context, engine: PlayerEngine, info: PlaybackInfo, startMs: Long): Player {
    val player = when (engine) {
        PlayerEngine.MPV -> MpvPlayer(context)
        PlayerEngine.MEDIA3 -> {
            val renderers = DefaultRenderersFactory(context)
                .setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
            ExoPlayer.Builder(context, renderers).build()
        }
    }
    val item = MediaItem.Builder().setUri(info.url)
    if (".m3u8" in info.url) item.setMimeType(MimeTypes.APPLICATION_M3U8)
    player.setMediaItem(item.build(), startMs)
    player.prepare()
    player.playWhenReady = true
    return player
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
private fun Controls(player: Player, title: String, heading: String?) {
    val navigator = LocalNavigator.current
    val playPause = rememberPlayPauseButtonState(player)
    val progress = rememberProgressStateWithTickInterval(player, 500, rememberCoroutineScope())
    var shown by remember { mutableStateOf(true) }
    var touched by remember { mutableLongStateOf(0L) }
    var seeking by remember { mutableStateOf<Float?>(null) }
    val touch = { touched = System.currentTimeMillis() }
    LaunchedEffect(shown, touched, playPause.showPlay, seeking) {
        if (shown && !playPause.showPlay && seeking == null) {
            delay(CONTROLS_TIMEOUT_MS)
            shown = false
        }
    }
    val buffering = player.playbackState == Player.STATE_BUFFERING || player.playbackState == Player.STATE_IDLE
    Box(
        Modifier
            .fillMaxSize()
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {
                shown = !shown
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
                            Modifier.padding(start = 8.dp),
                            style = theme.type.footnote.text,
                            color = Color.White.copy(alpha = 0.85f),
                        )
                    }
                }
            }
        }
    }
}

/** How far the seek buttons move. */
private const val SEEK_MS = 10_000L

/** A round button of the player, its icon white. */
@Composable
private fun PlayerButton(
    @DrawableRes icon: Int,
    label: String,
    large: Boolean = false,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .size(if (large) 48.dp else 44.dp)
            .clip(CircleShape)
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
