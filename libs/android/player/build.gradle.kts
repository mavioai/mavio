// One player interface over two engines, Media3 and libmpv, behind a media
// session. libmpv and its Kotlin API come from scripts/build-mpv.sh, in
// build/mpv, and Media3's FFmpeg decoder extension from
// scripts/build-ffmpeg.sh, in build/ffmpeg.
plugins {
    alias(libs.plugins.android.library)
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
