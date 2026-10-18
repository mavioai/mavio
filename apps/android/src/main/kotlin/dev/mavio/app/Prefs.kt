package dev.mavio.app

import android.content.Context
import androidx.annotation.StringRes
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
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

/** The theme the app takes on this device. */
enum class Appearance(val key: String, @StringRes val label: Int) {
    SYSTEM("system", R.string.settings_system),
    LIGHT("light", R.string.settings_light),
    DARK("dark", R.string.settings_dark),
}

/**
 * This device's settings, as the web UI keeps them in the browser; the
 * screens showing them follow their changes.
 */
class Prefs(context: Context) {
    private val prefs = context.getSharedPreferences("player", Context.MODE_PRIVATE)

    private var quality0 by mutableStateOf(
        prefs.getString(QUALITY, null).let { key -> Quality.entries.firstOrNull { it.key == key } } ?: Quality.AUTO,
    )
    private var autoplayNext0 by mutableStateOf(prefs.getBoolean(AUTOPLAY_NEXT, true))
    private var appearance0 by mutableStateOf(
        prefs.getString(APPEARANCE, null).let { key -> Appearance.entries.firstOrNull { it.key == key } } ?: Appearance.SYSTEM,
    )

    /** The highest quality streamed to this device. */
    var quality: Quality
        get() = quality0
        set(value) {
            quality0 = value
            prefs.edit { putString(QUALITY, value.key) }
        }

    /** Whether an episode's end plays the next. */
    var autoplayNext: Boolean
        get() = autoplayNext0
        set(value) {
            autoplayNext0 = value
            prefs.edit { putBoolean(AUTOPLAY_NEXT, value) }
        }

    /** The theme, the system's unless chosen. */
    var appearance: Appearance
        get() = appearance0
        set(value) {
            appearance0 = value
            prefs.edit { putString(APPEARANCE, value.key) }
        }

    private companion object {
        const val QUALITY = "quality"
        const val AUTOPLAY_NEXT = "autoplayNext"
        const val APPEARANCE = "appearance"
    }
}

/** This device's settings, which the app owns. */
val LocalPrefs = staticCompositionLocalOf<Prefs> { error("no settings") }
