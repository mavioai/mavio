package dev.mavio.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import com.github.takahirom.roborazzi.ExperimentalRoborazziApi
import com.github.takahirom.roborazzi.RoborazziOptions
import com.github.takahirom.roborazzi.captureRoboImage
import dev.mavio.design.DarkColors
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The snapshot tests of the shared components (docs/clients.md §6.8), drawn
 * at the sizes of libs/design/snapshots/screens.json. `gradle
 * recordRoborazziDebug` records the references in ui/snapshots; the unit
 * tests compare against them.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class SnapshotTest {
    @Test
    @Config(qualifiers = "w390dp-h844dp-xxhdpi")
    fun wordmarkPhone() = wordmark("phone")

    @Test
    @Config(qualifiers = "w1920dp-h1080dp-land-television-mdpi")
    fun wordmarkTv() = wordmark("tv")

    // Text antialiases a little differently on other machines.
    @OptIn(ExperimentalRoborazziApi::class)
    private val options = RoborazziOptions(
        compareOptions = RoborazziOptions.CompareOptions(changeThreshold = 0.002F),
    )

    private fun wordmark(size: String) =
        captureRoboImage("snapshots/wordmark-$size.png", options) {
            Box(Modifier.fillMaxSize().background(DarkColors.background), Alignment.Center) {
                Wordmark()
            }
        }
}
