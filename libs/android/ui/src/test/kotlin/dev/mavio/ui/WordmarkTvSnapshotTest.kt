package dev.mavio.ui

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class WordmarkTvSnapshotTest {
    @Test
    @Config(qualifiers = "w1920dp-h1080dp-land-television-mdpi")
    fun wordmark() = captureWordmark("tv")
}
