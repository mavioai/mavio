package dev.mavio.app

import android.Manifest
import android.app.SearchManager
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import dev.mavio.ui.Appearance
import dev.mavio.ui.LocalAssistant
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.MavioTheme
import dev.mavio.ui.installLaunch
import kotlinx.coroutines.CompletableDeferred

/** The app's only activity; the player is its own. */
class MainActivity : ComponentActivity() {
    private val app get() = application as MavioApplication
    private val model get() = app.model

    // Servers at home are on the local network, which Android 17 opens
    // only to apps the user allows; once allowed, the streams start again.
    private val localNetwork = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) {
            model.start()
            model.refreshHome()
        }
    }

    // The microphone, asked for as the assistant first listens.
    private var microphoneAnswer: CompletableDeferred<Boolean>? = null
    private val microphone = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        microphoneAnswer?.complete(granted)
        microphoneAnswer = null
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        installLaunch(model)
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        app.assistant.askPermission = {
            microphoneAnswer?.cancel()
            CompletableDeferred<Boolean>().also {
                microphoneAnswer = it
                microphone.launch(Manifest.permission.RECORD_AUDIO)
            }.await()
        }
        if (savedInstanceState == null) redirected(intent)
        if (Build.VERSION.SDK_INT >= 37 && checkSelfPermission(LOCAL_NETWORK) != PackageManager.PERMISSION_GRANTED) {
            localNetwork.launch(LOCAL_NETWORK)
        }
        setContent {
            val dark = when (app.prefs.appearance) {
                Appearance.SYSTEM -> isSystemInDarkTheme()
                Appearance.LIGHT -> false
                Appearance.DARK -> true
            }
            // The status and navigation bars' icons follow the theme chosen.
            DisposableEffect(dark) {
                val style = SystemBarStyle.auto(Color.TRANSPARENT, Color.TRANSPARENT) { dark }
                enableEdgeToEdge(style, style)
                onDispose {}
            }
            CompositionLocalProvider(LocalPrefs provides app.prefs, LocalAssistant provides app.assistant) {
                MavioTheme(dark) {
                    Root(model)
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        redirected(intent)
    }

    /**
     * Hands a sign-in the browser coming back from mavio.dev in a Custom
     * Tab; and the assistant what the system's voice search heard, or the
     * launcher's shortcut and the Quick Settings tile asking it to listen.
     */
    private fun redirected(intent: Intent) {
        when (intent.action) {
            MEDIA_PLAY_FROM_SEARCH, SEARCH_ACTION -> {
                intent.getStringExtra(SearchManager.QUERY)?.takeIf { it.isNotBlank() }?.let { q -> pendingAsk = q }
                return
            }
            ACTION_TALK -> {
                pendingAsk = ""
                return
            }
        }
        val uri = intent.data ?: return
        if (intent.action == Intent.ACTION_VIEW && uri.scheme == CLIENT_ID) app.signInRedirect.value = uri.toString()
    }

    // What the assistant is asked for as the app shows: a query, or empty to listen.
    private var pendingAsk: String? = null

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
}

/** The action of the launcher's shortcut and the Quick Settings tile, which start the assistant listening. */
const val ACTION_TALK = "dev.mavio.action.TALK"

/** The action of a voice search for something to play. */
private const val MEDIA_PLAY_FROM_SEARCH = "android.media.action.MEDIA_PLAY_FROM_SEARCH"

/** The action of a voice search in the app. */
private const val SEARCH_ACTION = "com.google.android.gms.actions.SEARCH_ACTION"

/** The permission to reach devices on the local network. */
private const val LOCAL_NETWORK = "android.permission.ACCESS_LOCAL_NETWORK"
