// Mavio for Android phones, tablets and foldables.
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "dev.mavio.app"
    compileSdk = 37
    defaultConfig {
        // One Play listing with the TV app, which shares the ID.
        applicationId = "dev.mavio.app"
        minSdk = 26
        targetSdk = 37
        versionCode = 1
        versionName = "0.0.0"
        // The ABIs the core is built for.
        ndk { abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64") }
        // The mavio.dev to sign a Mavio account in at; a development build
        // points it at a local site with -Pmavio.devApi=https://…/api.
        val api = providers.gradleProperty("mavio.devApi").getOrElse("https://mavio.dev/api")
        buildConfigField("String", "MAVIO_DEV_API", "\"$api\"")
    }
    buildFeatures { buildConfig = true }
}

dependencies {
    implementation("dev.mavio.android:core:0.0.0")
    implementation("dev.mavio.android:player:0.0.0")
    implementation("dev.mavio.android:ui:0.0.0")
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.core.ktx)
    implementation(libs.browser)
    implementation(libs.haze.blur)
    implementation(libs.navigation3.ui)
}
