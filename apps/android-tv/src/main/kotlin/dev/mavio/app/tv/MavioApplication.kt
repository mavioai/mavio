package dev.mavio.app.tv

import android.app.Application
import android.content.pm.ApplicationInfo
import android.system.Os
import dev.mavio.core.AppModel
import dev.mavio.ui.installImages

/** The app, which owns the core for as long as its process lives. */
class MavioApplication : Application() {
    /** The app's state over the core. */
    lateinit var model: AppModel
        private set

    override fun onCreate() {
        super.onCreate()
        installImages(this)
        // A debug build trusts the certificate authority of a development
        // mavio.dev, put in the app's files.
        val ca = filesDir.resolve("dev-ca.pem")
        if (applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0 && ca.exists()) {
            Os.setenv("MAVIO_EXTRA_CA_CERTS", ca.path, true)
        }
        model = AppModel(this, "Mavio for Android TV")
    }
}
