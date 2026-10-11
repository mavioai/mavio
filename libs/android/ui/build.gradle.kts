// Compose components the apps share, and the design tokens, icons and
// strings libs/design generates.
plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "dev.mavio.ui"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
    }
}

androidComponents {
    onVariants { variant ->
        variant.sources.java?.addStaticSourceDirectory("../../design/gen/kotlin")
        variant.sources.res?.addStaticSourceDirectory("../../design/gen/kotlin/res")
    }
}

dependencies {
    api(platform(libs.compose.bom))
    api(libs.compose.ui)
    api(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)
}
