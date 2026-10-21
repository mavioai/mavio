package dev.mavio.ui

import com.google.protobuf.Duration
import dev.mavio.core.ServerRecord
import mavio.assistant.v1.Assistant.AskContext
import mavio.assistant.v1.Assistant.ClientAction
import mavio.assistant.v1.Assistant.ControlCommand
import mavio.assistant.v1.Assistant.NavigateTarget
import mavio.library.v1.ItemOuterClass.Item
import mavio.library.v1.ItemOuterClass.ItemKind
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

// org.json is Android's, so the tests run on Robolectric.
@RunWith(RobolectricTestRunner::class)
class AssistantTest {
    private val server = ServerRecord("s1", "https://media.example.com:8920/", "Home", "1.0", "u1", "Ann")

    @Test
    fun socketUrl() {
        val cases = listOf(
            "/assistant/voice/abc" to "wss://media.example.com:8920/assistant/voice/abc",
            "assistant/voice/abc" to "wss://media.example.com:8920/assistant/voice/abc",
            "http://10.0.0.2:8688/v" to "ws://10.0.0.2:8688/v",
            "wss://relay.example/v" to "wss://relay.example/v",
        )
        for ((path, want) in cases) {
            assertEquals("socketUrl($path)", want, Assistant.socketUrl(path, server))
        }
    }

    @Test
    fun actionOfPlay() {
        val a = Assistant.actionOf(JSONObject("""{"id":"a1","play":{"itemIds":["i1","i2"],"startIndex":1,"startPosition":"61.5s","subtitleStreamIndex":-1}}"""))!!
        assertEquals(ClientAction.KindCase.PLAY, a.kindCase)
        assertEquals("a1", a.id)
        assertEquals(listOf("i1", "i2"), a.play.itemIdsList)
        assertEquals(1, a.play.startIndex)
        assertEquals(61L, a.play.startPosition.seconds)
        assertEquals(500_000_000, a.play.startPosition.nanos)
        assertTrue(a.play.hasSubtitleStreamIndex())
        assertEquals(-1, a.play.subtitleStreamIndex)
        assertTrue(!a.play.hasAudioStreamIndex())
    }

    @Test
    fun actionOfEnums() {
        // Enums come by name, as protobuf's JSON gives them, or by number.
        val control = Assistant.actionOf(JSONObject("""{"id":"a","control":{"command":"CONTROL_COMMAND_SEEK_BY","value":-30}}"""))!!
        assertEquals(ControlCommand.CONTROL_COMMAND_SEEK_BY, control.control.command)
        assertEquals(-30.0, control.control.value, 0.0)
        val navigate = Assistant.actionOf(JSONObject("""{"id":"b","navigate":{"target":5,"query":"dune"}}"""))!!
        assertEquals(NavigateTarget.NAVIGATE_TARGET_SEARCH, navigate.navigate.target)
        assertEquals("dune", navigate.navigate.query)
    }

    @Test
    fun actionOfShowItems() {
        val a = Assistant.actionOf(
            JSONObject("""{"id":"c","showItems":{"choose":true,"items":[{"id":"e1","name":"Pilot","kind":"ITEM_KIND_EPISODE","seriesName":"Atlas","parentIndexNumber":1,"indexNumber":2}]}}"""),
        )!!
        assertTrue(a.showItems.choose)
        val item = a.showItems.getItems(0)
        assertEquals(ItemKind.ITEM_KIND_EPISODE, item.kind)
        assertEquals("Atlas · S1:E2", Assistant.subtitle(item))
    }

    @Test
    fun actionOfNone() {
        assertNull(Assistant.actionOf(JSONObject("""{"id":"d"}""")))
    }

    @Test
    fun subtitle() {
        val cases = listOf(
            Item.newBuilder().setKind(ItemKind.ITEM_KIND_MOVIE).setProductionYear(2021).build() to "2021",
            Item.newBuilder().setKind(ItemKind.ITEM_KIND_MOVIE).build() to null,
            Item.newBuilder().setKind(ItemKind.ITEM_KIND_TRACK).addArtists("A").addArtists("B").build() to "A, B",
            Item.newBuilder().setKind(ItemKind.ITEM_KIND_SEASON).setSeriesName("Atlas").build() to "Atlas",
        )
        for ((item, want) in cases) {
            assertEquals("subtitle(${item.kind})", want, Assistant.subtitle(item))
        }
    }

    @Test
    fun contextJson() {
        val c = AskContext.newBuilder()
            .setScreen("player")
            .setItemId("i1")
            .addLanguages("zh-TW")
            .setPlaying(Assistant.playing("i1", 61_500, paused = true, speed = 1.0, audio = 1, subtitle = null))
            .build()
        val j = Assistant.contextJson(c)
        assertEquals("player", j.getString("screen"))
        assertEquals("zh-TW", j.getJSONArray("languages").getString(0))
        val p = j.getJSONObject("playing")
        assertEquals("61.5s", p.getString("position"))
        assertEquals(true, p.getBoolean("paused"))
        assertEquals(-1, p.getInt("subtitleStreamIndex"))
        assertEquals(1, p.getInt("audioStreamIndex"))
    }

    @Test
    fun playingPosition() {
        val p = Assistant.playing("i", 1_250, paused = false, speed = 1.5, audio = null, subtitle = 2)
        assertEquals(Duration.newBuilder().setSeconds(1).setNanos(250_000_000).build(), p.position)
        assertEquals(-1, p.audioStreamIndex)
        assertEquals(2, p.subtitleStreamIndex)
    }

    @Test
    fun spokenText() {
        // The words and what is between them, each said later than the last.
        val segments = SpokenText.segments("Playing Dune, now.")
        assertEquals("Playing Dune, now.", segments.joinToString("") { it.first })
        assertTrue(segments.zipWithNext().all { (a, b) -> b.second >= a.second })
        // A CJK character takes longer to say than a Latin letter.
        val cjk = SpokenText.segments("好").last().second
        val latin = SpokenText.segments("a").last().second
        assertTrue("cjk $cjk > latin $latin", cjk > latin)
    }
}
