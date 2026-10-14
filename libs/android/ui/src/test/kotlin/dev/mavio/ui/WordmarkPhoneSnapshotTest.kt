package dev.mavio.ui

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class WordmarkPhoneSnapshotTest {
    @Test
    @Config(qualifiers = "w390dp-h844dp-xxhdpi")
    fun wordmark() = captureWordmark("phone")
}
