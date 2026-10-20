// Mavio for Android TV and Google TV.
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.baselineprofile)
    // For baselineprofile/, on the classpath of the build.
    alias(libs.plugins.android.test) apply false
}

// The version, as the server's and the core's: a release's from
// $MAVIO_VERSION (v0.1.0 or 0.1.0), else 0.0.0 and the commit it is built
// at, else dev. A release's code is x.y.z as x*10000+y*100+z, else the
// number of commits.
val release = providers.environmentVariable("MAVIO_VERSION").orNull?.removePrefix("v")?.takeIf { it.isNotEmpty() }

fun git(vararg args: String): String? = runCatching {
    providers.exec {
        commandLine("git", *args)
        isIgnoreExitValue = true
    }.standardOutput.asText.get().trim()
}.getOrNull()?.takeIf { it.isNotEmpty() }

val mavioVersion: Pair<String, Int> = when {
    release != null -> {
        val (x, y, z) = requireNotNull(Regex("""(\d+)\.(\d+)\.(\d+)""").matchEntire(release)) { "MAVIO_VERSION $release is not x.y.z" }
            .destructured.toList().map(String::toInt)
        require(y < 100 && z < 100) { "MAVIO_VERSION $release has a part over 99" }
        release to x * 10000 + y * 100 + z
    }
    else -> git("rev-parse", "--short=12", "HEAD")?.let { "0.0.0-$it" to (git("rev-list", "--count", "HEAD")?.toInt() ?: 1) } ?: ("dev" to 1)
}

android {
    namespace = "dev.mavio.app.tv"
    compileSdk = 37
    defaultConfig {
        // One Play listing with the phone and tablet app, which shares the ID.
        applicationId = "dev.mavio.app"
        minSdk = 26
        targetSdk = 37
        versionCode = mavioVersion.second
        versionName = mavioVersion.first
        // The ABIs the core is built for.
        ndk { abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64") }
        // The mavio.dev to sign a Mavio account in at; a development build
        // points it at a local site with -Pmavio.devApi=https://…/api.
        val api = providers.gradleProperty("mavio.devApi").getOrElse("https://mavio.dev/api")
        buildConfigField("String", "MAVIO_DEV_API", "\"$api\"")
    }
    buildFeatures { buildConfig = true }
    // A release build signs with the key the properties mavio.signing.storeFile,
    // .storePassword, .keyAlias and .keyPassword name, else with the debug key,
    // for trying a release build on a device.
    val store = providers.gradleProperty("mavio.signing.storeFile").orNull
    signingConfigs {
        if (store != null) {
            create("release") {
                storeFile = file(store)
                storePassword = providers.gradleProperty("mavio.signing.storePassword").get()
                keyAlias = providers.gradleProperty("mavio.signing.keyAlias").get()
                keyPassword = providers.gradleProperty("mavio.signing.keyPassword").get()
            }
        }
    }
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            signingConfig = signingConfigs.getByName(if (store != null) "release" else "debug")
        }
    }
}

dependencies {
    // Installs the baseline profile of the app's start and Home (baselineprofile/).
    implementation(libs.profileinstaller)
    baselineProfile(project(":baselineprofile"))
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
