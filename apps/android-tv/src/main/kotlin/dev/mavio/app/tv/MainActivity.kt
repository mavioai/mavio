package dev.mavio.app.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import dev.mavio.design.TvLayout
import dev.mavio.design.TvTypography
import dev.mavio.ui.MavioTheme

/** The app's only activity. */
class MainActivity : ComponentActivity() {
    private val model get() = (application as MavioApplication).model

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            // A television starts dark, as it is watched in dim rooms (docs/tv.md §1).
            MavioTheme(dark = true, layout = TvLayout, type = TvTypography) {
                TvCanvas {
                    Root(model)
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
