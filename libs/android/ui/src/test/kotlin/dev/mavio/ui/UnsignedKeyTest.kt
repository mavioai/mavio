package dev.mavio.ui

import org.junit.Assert.assertEquals
import org.junit.Test

class UnsignedKeyTest {
    @Test
    fun unsignedKey() {
        val cases = listOf(
            "http://host:8688/signed/abc.def/images/items/1/primary?w=342" to "http://host:8688/images/items/1/primary?w=342",
            "https://host/base/signed/t/images/2" to "https://host/base/images/2",
            "https://image.tmdb.org/t/p/w342/a.jpg" to "https://image.tmdb.org/t/p/w342/a.jpg",
            "http://host/signed/token" to "http://host/signed/token",
        )
        for ((url, want) in cases) {
            assertEquals("unsignedKey($url)", want, unsignedKey(url))
        }
    }
}
