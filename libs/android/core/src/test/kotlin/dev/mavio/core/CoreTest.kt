package dev.mavio.core

import org.junit.Assert.assertTrue
import org.junit.Test

class CoreTest {
    @Test
    fun version() {
        // A release's, else 0.0.0 and the commit it was built at, else dev.
        assertTrue("got = $version, want = x.y.z, 0.0.0-<commit> or dev", Regex("""\d+\.\d+\.\d+(-[0-9a-f]{12})?|dev""").matches(version))
    }
}
