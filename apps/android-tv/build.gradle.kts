// Mavio for Android TV and Google TV.
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "dev.mavio.app.tv"
    compileSdk = 37
    defaultConfig {
        // One Play listing with the phone and tablet app, which shares the ID.
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
    implementation("dev.mavio.android:admin:0.0.0")
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.core.ktx)
    implementation(libs.media3.ui)
    implementation(libs.media3.ui.compose)
    implementation(libs.tv.material)
}
