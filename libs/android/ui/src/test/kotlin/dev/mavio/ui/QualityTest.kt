package dev.mavio.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class QualityTest {
    @Test
    fun bitrate() {
        val cases = listOf(
            Triple(Quality.AUTO, true, 100_000_000L),
            Triple(Quality.AUTO, false, 40_000_000L),
            Triple(Quality.ORIGINAL, true, 200_000_000L),
            Triple(Quality.P1080, true, 10_000_000L),
            Triple(Quality.P480, false, 1_500_000L),
        )
        for ((quality, local, want) in cases) {
            assertEquals("$quality.bitrate($local)", want, quality.bitrate(local))
        }
    }
}
