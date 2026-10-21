package dev.mavio.app

import android.app.Application
import android.content.pm.ApplicationInfo
import android.system.Os
import dev.mavio.core.AppModel
import dev.mavio.ui.Assistant
import dev.mavio.ui.Prefs
import dev.mavio.ui.installImages
import kotlinx.coroutines.flow.MutableStateFlow

/** The app, which owns the core for as long as its process lives. */
class MavioApplication : Application() {
    /** The app's state over the core. */
    lateinit var model: AppModel
        private set

    /**
     * Where mavio.dev sent the browser back to from a Mavio account's
     * sign-in, until the sign-in takes it.
     */
    val signInRedirect = MutableStateFlow<String?>(null)

    /** This device's settings. */
    val prefs by lazy { Prefs(this) }

    /** The assistant over the app. */
    val assistant by lazy { Assistant(model, this) }

    override fun onCreate() {
        super.onCreate()
        installImages(this)
        // A debug build trusts the certificate authority of a development
        // mavio.dev, put in the app's files.
        val ca = filesDir.resolve("dev-ca.pem")
        if (applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0 && ca.exists()) {
            Os.setenv("MAVIO_EXTRA_CA_CERTS", ca.path, true)
        }
        model = AppModel(this, "Mavio for Android")
    }
}
