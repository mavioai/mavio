plugins {
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.kotlin.compose) apply false
}

// The coordinates the apps depend on, which the included build substitutes.
subprojects {
    group = "dev.mavio.android"
    version = "0.0.0"
}
