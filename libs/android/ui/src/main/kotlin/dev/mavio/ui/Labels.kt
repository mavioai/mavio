package dev.mavio.ui

import android.text.format.Formatter
import android.content.Context
import androidx.annotation.StringRes
import dev.mavio.core.CreditEntry
import dev.mavio.core.CreditJob
import dev.mavio.core.ExtraKind
import dev.mavio.core.Source
import dev.mavio.core.VideoRange
import java.util.Locale

/** Whether the person acts. */
val CreditJob.isCast: Boolean
    get() = this == CreditJob.ACTOR || this == CreditJob.GUEST_STAR

/** The job's name as the crew lists it, as the web UI's. */
@get:StringRes
val CreditJob.label: Int
    get() = when (this) {
        CreditJob.ACTOR -> R.string.credits_cast
        CreditJob.GUEST_STAR -> R.string.credits_guest_stars
        CreditJob.CREATOR -> R.string.item_creators
        CreditJob.DIRECTOR -> R.string.item_director
        CreditJob.WRITER -> R.string.item_writers
        CreditJob.AUTHOR -> R.string.credits_authors
        CreditJob.PRODUCER -> R.string.item_producers
        CreditJob.COMPOSER -> R.string.item_music
        CreditJob.CONDUCTOR -> R.string.credits_conductors
        CreditJob.LYRICIST -> R.string.credits_lyricists
        CreditJob.ARTIST -> R.string.credits_artists
        CreditJob.NARRATOR -> R.string.credits_narrators
        CreditJob.OTHER -> R.string.credits_other
    }

/** The crew's jobs in the order the credits list them. */
val crewOrder = listOf(
    CreditJob.CREATOR, CreditJob.DIRECTOR, CreditJob.WRITER, CreditJob.AUTHOR, CreditJob.PRODUCER,
    CreditJob.COMPOSER, CreditJob.CONDUCTOR, CreditJob.LYRICIST, CreditJob.ARTIST, CreditJob.NARRATOR,
    CreditJob.OTHER,
)

/** What a card says the person did: the character played, else the job. */
fun CreditEntry.shownRole(context: Context): String? = when {
    job.isCast -> role
    job == CreditJob.OTHER -> role ?: context.getString(job.label)
    else -> context.getString(job.label)
}

/** What the extra is, as its card says. */
@get:StringRes
val ExtraKind.label: Int
    get() = when (this) {
        ExtraKind.TRAILER -> R.string.extra_trailer
        ExtraKind.CLIP -> R.string.extra_clip
        ExtraKind.BEHIND_THE_SCENES -> R.string.extra_behind_the_scenes
        ExtraKind.DELETED_SCENE -> R.string.extra_deleted_scene
        ExtraKind.INTERVIEW -> R.string.extra_interview
        ExtraKind.SCENE -> R.string.extra_scene
        ExtraKind.SAMPLE -> R.string.extra_sample
        ExtraKind.FEATURETTE -> R.string.extra_featurette
        ExtraKind.SHORT -> R.string.extra_short
        ExtraKind.OTHER -> R.string.extra_other
    }

/** The resolution of the video, as "4K" or "1080p". */
val Source.resolution: String?
    get() {
        val v = video ?: return null
        if (v.height <= 0) return null
        return when {
            v.width >= 3200 || v.height >= 2000 -> "4K"
            v.width >= 1800 || v.height >= 1000 -> "1080p"
            v.width >= 1200 || v.height >= 700 -> "720p"
            else -> "${v.height}p"
        }
    }

/** The dynamic range of the video when it is HDR. */
val Source.range: String?
    get() = when (video?.range) {
        VideoRange.HDR10 -> "HDR10"
        VideoRange.HDR10_PLUS -> "HDR10+"
        VideoRange.HLG -> "HLG"
        VideoRange.DOLBY_VISION -> "Dolby Vision"
        else -> null
    }

/** The most channels of the audio, as "5.1" or "7.1", when it is surround. */
val Source.surround: String?
    get() {
        val most = audio.maxOfOrNull { it.channels } ?: 0
        return if (most >= 8) "7.1" else if (most >= 6) "5.1" else null
    }

/** The container, by the file's extension, else the first format the probe named. */
val Source.format: String?
    get() {
        val ext = fileName?.substringAfterLast('.', "")
        if (!ext.isNullOrEmpty()) return ext.uppercase(Locale.ROOT)
        return container?.split(',')?.firstOrNull()?.uppercase(Locale.ROOT)
    }

/** The version's name: its own, else its resolution and range, else its container. */
val Source.versionName: String
    get() {
        name?.let { return it }
        val parts = listOfNotNull(resolution, range)
        return if (parts.isEmpty()) format ?: serverName else parts.joinToString(" ")
    }

/** The size, as "4.2 GB". */
fun Source.sizeText(context: Context): String? = size?.let { Formatter.formatFileSize(context, it.toLong()) }

/** A language's name in the user's language, from its code; nothing for an undetermined one. */
fun languageName(code: String?): String? {
    if (code == null || code == "und") return null
    val name = Locale.forLanguageTag(code).getDisplayLanguage(Locale.getDefault())
    return name.ifEmpty { code }
}
