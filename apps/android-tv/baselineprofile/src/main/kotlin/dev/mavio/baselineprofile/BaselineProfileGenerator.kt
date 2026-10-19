package dev.mavio.baselineprofile

import androidx.benchmark.macro.junit4.BaselineProfileRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The app's start and Home, which its baseline profile compiles ahead of
 * time. The app must be signed in to a server, whose Home it then shows.
 */
@RunWith(AndroidJUnit4::class)
class BaselineProfileGenerator {
    @get:Rule
    val rule = BaselineProfileRule()

    @Test
    fun startAndHome() =
        rule.collect(PACKAGE, includeInStartupProfile = true) {
            pressHome()
            startActivityAndWait()
            // Home's shelves, as the servers send them.
            Thread.sleep(HOME_MS)
            device.waitForIdle()
            // Down the shelves and along one, as a remote does.
            repeat(4) {
                device.pressDPadDown()
                device.waitForIdle()
            }
            repeat(4) {
                device.pressDPadRight()
                device.waitForIdle()
            }
        }
}
