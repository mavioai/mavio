package dev.mavio.player

import android.app.Activity
import android.os.Bundle
import android.view.SurfaceView

/** A window filled with the [SurfaceView] a player renders into. */
class SurfaceActivity : Activity() {
    lateinit var surface: SurfaceView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        surface = SurfaceView(this)
        setContentView(surface)
    }
}
