package dev.mavio.app.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.CompositionLocalProvider
import dev.mavio.design.TvLayout
import dev.mavio.design.TvTypography
import dev.mavio.ui.LocalPrefs
import dev.mavio.ui.MavioTheme
import dev.mavio.ui.TvCanvas
import dev.mavio.ui.installLaunch

/** The app's only activity. */
class MainActivity : ComponentActivity() {
    private val model get() = (application as MavioApplication).model

    override fun onCreate(savedInstanceState: Bundle?) {
        installLaunch(model)
        super.onCreate(savedInstanceState)
        setContent {
            // A television starts dark, as it is watched in dim rooms (docs/tv.md §1).
            MavioTheme(dark = true, layout = TvLayout, type = TvTypography) {
                TvCanvas {
                    CompositionLocalProvider(LocalPrefs provides (application as MavioApplication).prefs) {
                        Root(model)
                    }
                }
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
