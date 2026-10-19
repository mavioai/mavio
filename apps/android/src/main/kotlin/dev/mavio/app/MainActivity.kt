package dev.mavio.app

import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import dev.mavio.ui.MavioTheme
import dev.mavio.ui.installLaunch

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

    override fun onCreate(savedInstanceState: Bundle?) {
        installLaunch(model)
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        redirected(intent)
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
            CompositionLocalProvider(LocalPrefs provides app.prefs) {
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

    /** Hands a sign-in the browser coming back from mavio.dev in a Custom Tab. */
    private fun redirected(intent: Intent) {
        val uri = intent.data ?: return
        if (intent.action == Intent.ACTION_VIEW && uri.scheme == CLIENT_ID) app.signInRedirect.value = uri.toString()
    }

    override fun onStart() {
        super.onStart()
        model.start()
        model.refreshHome()
    }

    override fun onStop() {
        super.onStop()
        model.stop()
    }
}

/** The permission to reach devices on the local network. */
private const val LOCAL_NETWORK = "android.permission.ACCESS_LOCAL_NETWORK"
