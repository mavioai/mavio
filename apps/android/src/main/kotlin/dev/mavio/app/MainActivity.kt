package dev.mavio.app

import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import dev.mavio.ui.MavioTheme

/** The app's only activity; the player is its own. */
class MainActivity : ComponentActivity() {
    private val model get() = (application as MavioApplication).model

    // Servers at home are on the local network, which Android 17 opens
    // only to apps the user allows; once allowed, the streams start again.
    private val localNetwork = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) {
            model.start()
            model.refreshHome()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        if (Build.VERSION.SDK_INT >= 37 && checkSelfPermission(LOCAL_NETWORK) != PackageManager.PERMISSION_GRANTED) {
            localNetwork.launch(LOCAL_NETWORK)
        }
        setContent {
            MavioTheme {
                Root(model)
            }
        }
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
