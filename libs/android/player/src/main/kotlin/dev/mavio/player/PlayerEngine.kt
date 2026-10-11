package dev.mavio.player

/** The engine that plays a source. */
enum class PlayerEngine {
    /** Media3 ExoPlayer, for what it plays and every HLS stream of the server. */
    MEDIA3,

    /** libmpv, for what Media3 does not play. */
    MPV,
}
