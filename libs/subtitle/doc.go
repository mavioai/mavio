// Package subtitle reads SRT, SSA, ASS and WebVTT subtitles, filters them to
// a time window and writes SRT, SSA, ASS, WebVTT, TTML or Jellyfin's JSON
// track format. ToUTF8 converts files in legacy character sets, detected
// from a byte order mark or statistically. It is a pure library with no
// I/O; formats it cannot read are converted to SRT with ffmpeg by the media
// pipeline.
package subtitle
