// Compose components the apps share, and the design tokens, icons and
// strings libs/design generates.
plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.roborazzi)
}

android {
    namespace = "dev.mavio.ui"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
    }
    // The snapshot tests draw with Robolectric's native graphics.
    testOptions.unitTests {
        isIncludeAndroidResources = true
        all {
            // The references of the snapshot tests (docs/clients.md §6.8).
            it.inputs.dir("snapshots").withPathSensitivity(PathSensitivity.RELATIVE)
            // Robolectric reaches into the file descriptors of the JDK.
            it.jvmArgs(
                "--add-opens=java.base/java.io=ALL-UNNAMED",
                "--add-exports=java.base/jdk.internal.access=ALL-UNNAMED",
            )
        }
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
    debugImplementation(libs.compose.ui.test.manifest)
    testImplementation(libs.junit)
    testImplementation(libs.robolectric)
    testImplementation(libs.compose.ui.test.junit4)
    testImplementation(libs.androidx.test.espresso.core)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
}
