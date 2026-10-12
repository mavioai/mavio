// One player interface over two engines, Media3 and libmpv, behind a media
// session. libmpv with its Kotlin API, in build/mpv, and Media3's FFmpeg
// decoder extension, in build/ffmpeg, come from mavio-deps (tools/deps).
plugins {
    alias(libs.plugins.android.library)
}

// The extension is built for one Media3, the one the version catalog names.
val media3Ffmpeg = rootDir.resolve("../../tools/deps/deps.txt").readLines()
    .map { it.split(" ") }.single { it[0] == "media3-ffmpeg" }[1]
check(media3Ffmpeg.startsWith("media3-ffmpeg-${libs.versions.media3.get()}-")) {
    "tools/deps/deps.txt pins $media3Ffmpeg, not the extension of Media3 ${libs.versions.media3.get()}"
}

android {
    namespace = "dev.mavio.player"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
}

androidComponents {
    onVariants { variant ->
        variant.sources.java?.addStaticSourceDirectory("build/mpv/kotlin")
        variant.sources.jniLibs?.addStaticSourceDirectory("build/mpv/jniLibs")
        variant.sources.java?.addStaticSourceDirectory("build/ffmpeg/java")
        variant.sources.jniLibs?.addStaticSourceDirectory("build/ffmpeg/jniLibs")
    }
}

dependencies {
    api(libs.media3.exoplayer)
    api(libs.media3.exoplayer.hls)
    api(libs.media3.session)
    compileOnly(libs.checkerframework.qual)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}
