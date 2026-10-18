package dev.mavio.app

import android.content.Context
import androidx.annotation.StringRes
import androidx.core.content.edit
import dev.mavio.ui.R

/** The most the server sends this device, as the web UI's qualities. */
enum class Quality(val key: String, val bitrate: Long, @StringRes val label: Int?) {
    ORIGINAL("original", 200_000_000, R.string.player_quality_original),
    AUTO("auto", 40_000_000, R.string.player_quality_auto),
    P1080("1080p", 10_000_000, null),
    P720("720p", 4_000_000, null),
    P480("480p", 1_500_000, null),
}

/** This device's playback settings, as the web UI keeps them in the browser. */
class Prefs(context: Context) {
    private val prefs = context.getSharedPreferences("player", Context.MODE_PRIVATE)

    /** The highest quality streamed to this device. */
    var quality: Quality
        get() = prefs.getString(QUALITY, null).let { key -> Quality.entries.firstOrNull { it.key == key } } ?: Quality.AUTO
        set(value) = prefs.edit { putString(QUALITY, value.key) }

    /** Whether an episode's end plays the next. */
    var autoplayNext: Boolean
        get() = prefs.getBoolean(AUTOPLAY_NEXT, true)
        set(value) = prefs.edit { putBoolean(AUTOPLAY_NEXT, value) }

    private companion object {
        const val QUALITY = "quality"
        const val AUTOPLAY_NEXT = "autoplayNext"
    }
}
