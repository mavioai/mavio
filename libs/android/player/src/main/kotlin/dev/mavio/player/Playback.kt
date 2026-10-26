package dev.mavio.player

import android.content.Context
import android.net.Uri
import androidx.annotation.OptIn
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import dev.mavio.core.DeviceProfile
import dev.mavio.core.PlayKind
import dev.mavio.core.PlaybackInfo
import dev.mavio.core.Source

// The players of the apps: which engine plays what the server sends, and
// the streams it picks in each (docs/clients.md §4.3).

/**
 * The engine that plays [info]: libmpv for a file sent whole that Media3
 * does not play, else Media3, which plays every stream the server
 * processes.
 */
fun engineFor(info: PlaybackInfo, source: Source?): PlayerEngine =
    if (info.kind == PlayKind.DIRECT_PLAY && source != null && !DeviceProfile.media3Plays(source)) {
        PlayerEngine.MPV
    } else {
        PlayerEngine.MEDIA3
    }

/**
 * Media3 with its FFmpeg extension, or libmpv, playing [info] of [source]
 * from [startMs] once prepared, with the streams the server picked, its
 * subtitles shifted by [subtitles].
 */
@OptIn(UnstableApi::class)
fun createPlayer(
    context: Context,
    engine: PlayerEngine,
    info: PlaybackInfo,
    source: Source?,
    startMs: Long,
    autoplay: Boolean,
    subtitles: SubtitleShift = SubtitleShift(),
): Player {
    val player = when (engine) {
        PlayerEngine.MPV -> MpvPlayer(context).apply {
            mpvTracks(info, source)?.let { (audio, subtitle) -> selectTracks(audio, subtitle) }
            subtitleDelayMs = subtitles.ms
            subtitles.onChange = { subtitleDelayMs = it }
        }
        PlayerEngine.MEDIA3 -> {
            subtitles.onChange = null
            val renderers = ShiftedRenderersFactory(context, subtitles)
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
    player.playWhenReady = autoplay
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
private fun mpvTracks(info: PlaybackInfo, source: Source?): Pair<Int?, Int?>? {
    source ?: return null
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
fun chooseTracks(player: ExoPlayer, tracks: Tracks, info: PlaybackInfo, source: Source?) {
    val params = player.trackSelectionParameters.buildUpon()
    val audioGroups = tracks.groups.filter { it.type == C.TRACK_TYPE_AUDIO }
    val textGroups = tracks.groups.filter { it.type == C.TRACK_TYPE_TEXT }
    val direct = info.kind == PlayKind.DIRECT_PLAY
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
