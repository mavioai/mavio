package dev.mavio.app.tv

import android.Manifest
import android.app.SearchManager
import android.content.Intent
import android.os.Bundle
import android.view.KeyEvent
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.CompositionLocalProvider
import dev.mavio.design.TvLayout
import dev.mavio.design.TvTypography
import dev.mavio.ui.LocalAssistant
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.MavioTheme
import dev.mavio.ui.TvCanvas
import dev.mavio.ui.installLaunch
import kotlinx.coroutines.CompletableDeferred

/** The app's only activity. */
class MainActivity : ComponentActivity() {
    private val app get() = application as MavioApplication
    private val model get() = app.model

    // The microphone, asked for as the assistant first listens.
    private var microphoneAnswer: CompletableDeferred<Boolean>? = null
    private val microphone = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        microphoneAnswer?.complete(granted)
        microphoneAnswer = null
    }

    // What the system's voice search asked the assistant, until the app shows.
    private var pendingAsk: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        installLaunch(model)
        super.onCreate(savedInstanceState)
        app.assistant.askPermission = {
            microphoneAnswer?.cancel()
            CompletableDeferred<Boolean>().also {
                microphoneAnswer = it
                microphone.launch(Manifest.permission.RECORD_AUDIO)
            }.await()
        }
        if (savedInstanceState == null) searched(intent)
        setContent {
            // A television starts dark, as it is watched in dim rooms (docs/tv.md §1).
            MavioTheme(dark = true, layout = TvLayout, type = TvTypography) {
                TvCanvas {
                    CompositionLocalProvider(LocalPrefs provides app.prefs, LocalAssistant provides app.assistant) {
                        Root(model)
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        searched(intent)
    }

    /** Hands the assistant what the system's voice search heard. */
    private fun searched(intent: Intent) {
        if (intent.action != MEDIA_PLAY_FROM_SEARCH && intent.action != SEARCH_ACTION) return
        intent.getStringExtra(SearchManager.QUERY)?.takeIf { it.isNotBlank() }?.let { pendingAsk = it }
    }

    override fun onStart() {
        super.onStart()
        model.start()
        model.refreshHome()
    }

    override fun onResume() {
        super.onResume()
        val q = pendingAsk ?: return
        pendingAsk = null
        app.assistant.request(q)
    }

    override fun onStop() {
        super.onStop()
        model.stop()
    }

    /** The remote's search and assistant keys, which the system lets the app have, press the assistant: held, it talks while held. */
    override fun onKeyDown(keyCode: Int, event: KeyEvent): Boolean {
        if (keyCode !in TALK_KEYS || app.assistant.server == null) return super.onKeyDown(keyCode, event)
        if (event.repeatCount == 0) app.assistant.press()
        return true
    }

    override fun onKeyUp(keyCode: Int, event: KeyEvent): Boolean {
        if (keyCode !in TALK_KEYS || app.assistant.server == null) return super.onKeyUp(keyCode, event)
        app.assistant.release()
        return true
    }
}

/** The remote's keys that talk to the assistant. */
private val TALK_KEYS = setOf(KeyEvent.KEYCODE_SEARCH, KeyEvent.KEYCODE_VOICE_ASSIST, KeyEvent.KEYCODE_ASSIST)

/** The action of a voice search for something to play. */
private const val MEDIA_PLAY_FROM_SEARCH = "android.media.action.MEDIA_PLAY_FROM_SEARCH"

/** The action of a voice search in the app. */
private const val SEARCH_ACTION = "com.google.android.gms.actions.SEARCH_ACTION"
