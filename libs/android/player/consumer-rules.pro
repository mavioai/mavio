# libmpv and Media3's FFmpeg extension call back into their Java classes
# from native code, by name.
-keep class is.xyz.mpv.** { *; }
-keep class androidx.media3.decoder.ffmpeg.** { *; }
