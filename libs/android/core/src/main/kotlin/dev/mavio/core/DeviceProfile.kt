package dev.mavio.core

import android.media.MediaCodecList
import android.media.MediaFormat

/**
 * What the apps play with their two engines (docs/clients.md §4.3): the
 * server plays a source directly when Media3 or libmpv can, and processes
 * it into HLS for Media3 only when neither can.
 */
object DeviceProfile {
    /** The containers Media3 extracts. */
    val media3Containers = setOf("mp4", "m4v", "mov", "mkv", "webm", "ts", "mpegts", "m2ts", "flv", "ogg", "avi")

    /**
     * The audio codecs Media3 decodes: the platform's, and with its FFmpeg
     * extension AC-3, E-AC-3, DTS, TrueHD, MLP and ALAC.
     */
    val media3AudioCodecs = setOf(
        "aac", "mp3", "mp2", "flac", "opus", "vorbis", "alac", "ac3", "eac3", "dts", "dca", "truehd", "mlp",
        "pcm_s16le", "pcm_s24le", "pcm_mulaw", "pcm_alaw", "amrnb", "amrwb",
    )

    /** The video codecs the device's decoders take, by the server's codec names. */
    fun videoCodecs(): Set<String> {
        val types = mapOf(
            MediaFormat.MIMETYPE_VIDEO_AVC to "h264",
            MediaFormat.MIMETYPE_VIDEO_HEVC to "hevc",
            MediaFormat.MIMETYPE_VIDEO_AV1 to "av1",
            MediaFormat.MIMETYPE_VIDEO_VP9 to "vp9",
            MediaFormat.MIMETYPE_VIDEO_VP8 to "vp8",
            MediaFormat.MIMETYPE_VIDEO_MPEG2 to "mpeg2video",
            MediaFormat.MIMETYPE_VIDEO_MPEG4 to "mpeg4",
        )
        val codecs = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos
            .filter { !it.isEncoder }
            .flatMap { it.supportedTypes.asList() }
            .mapNotNull { types[it.lowercase()] }
            .toSet()
        return codecs + "h264"
    }

    /**
     * Whether Media3 plays a source as it is: its container, its video with
     * the device's decoders and every audio stream; libmpv plays the rest.
     */
    fun media3Plays(source: Source): Boolean {
        val containers = source.container?.split(',').orEmpty().map { it.trim().lowercase() }
        val ext = source.fileName?.substringAfterLast('.', "")?.lowercase()
        if (containers.none { it in media3Containers } && ext !in media3Containers) return false
        val video = source.video
        if (video != null && video.codec.lowercase() !in videoCodecs()) return false
        return source.audio.all { it.codec.lowercase() in media3AudioCodecs }
    }

    /** The capabilities of this device. */
    fun capabilities(maxBitrate: Long = 0): Capabilities {
        val video = videoCodecs()
        val directPlay = listOf(
            // Media3, with the device's decoders.
            DirectPlayProfile(
                MediaType.VIDEO,
                media3Containers.sorted().joinToString(","),
                video.sorted().joinToString(","),
                media3AudioCodecs.sorted().joinToString(","),
            ),
            DirectPlayProfile(MediaType.AUDIO, "mp3,m4a,mp4,aac,flac,ogg,oga,opus,wav,mka,webm", "", ""),
            // libmpv demuxes with FFmpeg and decodes what the device cannot
            // in software.
            DirectPlayProfile(
                MediaType.VIDEO,
                "mkv,webm,mp4,m4v,mov,avi,ts,m2ts,mpegts,flv,wmv,asf,ogv,vob,mpg,mpeg",
                "h264,hevc,av1,vp9,vp8,mpeg2video,mpeg4,vc1,msmpeg4v3,wmv3",
                "aac,mp3,mp2,ac3,eac3,dts,truehd,flac,alac,opus,vorbis,wmav2,pcm_s16le,pcm_s24le",
            ),
            DirectPlayProfile(MediaType.AUDIO, "ape,wv,dsf,wma,aiff", "", ""),
        )
        val hls = { kind: MediaType, videoCodec: String, audioCodec: String ->
            TranscodingProfile(
                kind, false, Delivery.HLS, "mp4", videoCodec, audioCodec,
                maxAudioChannels = 6, subtitlesInManifest = true, segmentLengthSeconds = 0, conditions = emptyList(),
            )
        }
        val transcoding = listOf(
            hls(MediaType.VIDEO, if ("hevc" in video) "hevc,h264" else "h264", "aac,ac3,eac3"),
            hls(MediaType.AUDIO, "", "aac"),
            TranscodingProfile(
                MediaType.VIDEO, true, Delivery.HTTP, "mp4", "h264", "aac",
                maxAudioChannels = 2, subtitlesInManifest = false, segmentLengthSeconds = 0, conditions = emptyList(),
            ),
        )
        // libmpv draws every subtitle a file carries, styled ASS included,
        // and files of their own.
        val subtitles = listOf(SubtitleProfile("vtt", SubtitleDelivery.HLS)) +
            listOf("ass", "ssa", "subrip", "srt", "vtt", "webvtt", "mov_text", "pgssub", "dvdsub", "dvbsub")
                .map { SubtitleProfile(it, SubtitleDelivery.EMBED) } +
            listOf("srt", "subrip", "vtt", "ass", "ssa").map { SubtitleProfile(it, SubtitleDelivery.EXTERNAL) }
        return Capabilities(
            name = "Mavio",
            maxStreamingBitrate = maxBitrate,
            maxStaticBitrate = 0,
            directPlay = directPlay,
            transcoding = transcoding,
            containers = emptyList(),
            codecs = emptyList(),
            subtitles = subtitles,
        )
    }
}
