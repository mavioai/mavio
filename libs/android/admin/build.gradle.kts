// The server's administration in the settings, which both apps show: the
// Server group's sections and menus, and the pages within them.
plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "dev.mavio.admin"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
    }
}

dependencies {
    api(project(":ui"))
    implementation(libs.androidx.activity.compose)
}
