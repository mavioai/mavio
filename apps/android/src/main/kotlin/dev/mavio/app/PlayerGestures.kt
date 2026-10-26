package dev.mavio.app

import android.content.Context
import android.media.AudioManager
import android.provider.Settings
import android.view.Window
import android.view.WindowManager
import androidx.activity.compose.LocalActivity
import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import kotlin.math.roundToInt
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// The player's touch gestures beside taps (docs/clients.md §6.5), as
// iOS's: vertical swipes setting the brightness on the left half and the
// volume on the right, with the level shown at the top.

/** What a vertical swipe over the player sets, and its icon. */
internal enum class SwipeLevel(@DrawableRes val icon: Int) {
    BRIGHTNESS(R.drawable.ic_light_mode_fill),
    VOLUME(R.drawable.ic_volume_fill),
}

/** The level a swipe set, from 0 to 1, which the player shows. */
internal data class LevelShown(val kind: SwipeLevel, val value: Float) {
    val icon get() = kind.icon
}

/**
 * The player's vertical swipes: the window's brightness, which goes back
 * to the system's as the player closes, and the media volume.
 */
@Composable
internal fun rememberSwipeLevels(): SwipeLevels {
    val window = LocalActivity.current?.window
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val levels = remember(window) { SwipeLevels(context, window, scope) }
    DisposableEffect(window) {
        onDispose {
            window?.let { it.attributes = it.attributes.apply { screenBrightness = WindowManager.LayoutParams.BRIGHTNESS_OVERRIDE_NONE } }
        }
    }
    return levels
}

@Stable
internal class SwipeLevels(private val context: Context, private val window: Window?, private val scope: CoroutineScope) {
    private val audio = context.getSystemService(AudioManager::class.java)

    /** The level a swipe sets, while it does and briefly after. */
    var shown by mutableStateOf<LevelShown?>(null)
        private set
    private var hiding: Job? = null

    /**
     * Follows vertical swipes while [enabled]: one starting at the top or
     * the bottom, where the system's own gestures are, sets nothing.
     */
    suspend fun detect(input: PointerInputScope, enabled: () -> Boolean) = with(input) {
        var kind: SwipeLevel? = null
        var from = 0f
        var moved = 0f
        val end = {
            if (kind != null) {
                kind = null
                hiding = scope.launch {
                    delay(700)
                    shown = null
                }
            }
        }
        detectVerticalDragGestures(
            onDragStart = { at ->
                val edge = 60.dp.toPx()
                kind = when {
                    !enabled() || at.y < edge || at.y > size.height - edge -> null
                    at.x < size.width / 2 -> SwipeLevel.BRIGHTNESS
                    else -> SwipeLevel.VOLUME
                }
                kind?.let { from = level(it) }
                moved = 0f
                hiding?.cancel()
            },
            onDragEnd = end,
            onDragCancel = end,
        ) { change, dy ->
            val k = kind ?: return@detectVerticalDragGestures
            change.consume()
            moved += dy
            val value = (from - moved / (size.height * 0.6f)).coerceIn(0f, 1f)
            set(k, value)
            shown = LevelShown(k, value)
        }
    }

    private fun level(kind: SwipeLevel): Float = when (kind) {
        SwipeLevel.BRIGHTNESS -> window?.attributes?.screenBrightness?.takeIf { it >= 0f }
            ?: (Settings.System.getInt(context.contentResolver, Settings.System.SCREEN_BRIGHTNESS, 128) / 255f).coerceIn(0f, 1f)
        SwipeLevel.VOLUME -> audio.getStreamVolume(AudioManager.STREAM_MUSIC).toFloat() / audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC).coerceAtLeast(1)
    }

    private fun set(kind: SwipeLevel, value: Float) {
        when (kind) {
            SwipeLevel.BRIGHTNESS -> window?.let { it.attributes = it.attributes.apply { screenBrightness = value.coerceAtLeast(0.01f) } }
            SwipeLevel.VOLUME -> {
                val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
                audio.setStreamVolume(AudioManager.STREAM_MUSIC, (value * max).roundToInt(), 0)
            }
        }
    }
}

/** What a gesture sets, shown at the top of the player: its icon beside [content], in dark glass. */
@Composable
internal fun Indicator(@DrawableRes icon: Int, modifier: Modifier, content: @Composable RowScope.() -> Unit) {
    Row(
        modifier
            .windowInsetsPadding(WindowInsets.safeDrawing)
            .padding(top = 56.dp)
            .height(40.dp)
            .clip(CircleShape)
            .background(Color(0xD9171717))
            .border(1.dp, Color.White.copy(alpha = 0.1f), CircleShape)
            .padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(icon, size = 18.dp, tint = Color.White)
        content()
    }
}
