package dev.mavio.ui

import android.content.Context
import androidx.annotation.StringRes

/** The most the server sends this device, as the web UI's qualities. */
enum class Quality(val key: String, val bitrate: Long, @StringRes val label: Int?) {
    ORIGINAL("original", 200_000_000, R.string.player_quality_original),
    AUTO("auto", 40_000_000, R.string.player_quality_auto),
    P1080("1080p", 10_000_000, null),
    P720("720p", 4_000_000, null),
    P480("480p", 1_500_000, null),
    ;

    /** The most the server sends: [AUTO] is 100 Mbit/s from a server on the local network, as iOS's. */
    fun bitrate(local: Boolean): Long = if (this == AUTO && local) 100_000_000 else bitrate

    companion object {
        /** The quality this device keeps in [context], [AUTO] unless chosen. */
        fun stored(context: Context): Quality =
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY, null)
                .let { key -> entries.firstOrNull { it.key == key } } ?: AUTO

        /** Keeps [quality] as this device's in [context]. */
        fun store(context: Context, quality: Quality) {
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString(KEY, quality.key).apply()
        }

        private const val PREFS = "player"
        private const val KEY = "quality"
    }
}
