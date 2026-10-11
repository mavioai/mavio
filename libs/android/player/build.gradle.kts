// One player interface over two engines, Media3 and libmpv, behind a media
// session.
plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "dev.mavio.player"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
    }
}

dependencies {
    api(libs.media3.exoplayer)
    api(libs.media3.session)
}
