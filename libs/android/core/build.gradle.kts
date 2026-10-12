// The shared core's Kotlin API with the app services above it. The API and
// the core's libraries come from scripts/build-core.sh, in build/rust.
plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "dev.mavio.core"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    testOptions.unitTests.all {
        // The core built for the machine running the tests.
        it.systemProperty("jna.library.path", file("build/rust/host").absolutePath)
    }
}

androidComponents {
    onVariants { variant ->
        variant.sources.java?.addStaticSourceDirectory("build/rust/kotlin")
        variant.sources.jniLibs?.addStaticSourceDirectory("build/rust/jniLibs")
    }
}

dependencies {
    implementation("${libs.jna.get()}@aar")
    implementation(libs.androidx.annotation)
    // The API's async functions are suspend functions.
    api(libs.kotlinx.coroutines.core)
    testImplementation(libs.jna)
    testImplementation(libs.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}
