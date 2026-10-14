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

/*
 * The snapshot tests of the shared components (docs/clients.md §6.8), drawn
 * at the sizes of libs/design/snapshots/screens.json. `gradle
 * recordRoborazziDebug` records the references in ui/snapshots; the unit
 * tests compare against them.
 *
 * Robolectric draws only the first capture in a JVM, later ones come out
 * empty, so each test class captures once and Gradle forks a JVM per class.
 */

// Text antialiases a little differently on other machines.
@OptIn(ExperimentalRoborazziApi::class)
private val options = RoborazziOptions(
    compareOptions = RoborazziOptions.CompareOptions(changeThreshold = 0.002F),
)

/** Captures the wordmark on the background at the test's screen size. */
internal fun captureWordmark(size: String) =
    captureRoboImage("snapshots/wordmark-$size.png", options) {
        Box(Modifier.fillMaxSize().background(DarkColors.background), Alignment.Center) {
            Wordmark()
        }
    }
