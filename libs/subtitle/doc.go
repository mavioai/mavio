// Package subtitle reads SRT, SSA, ASS and WebVTT subtitles, filters them to
// a time window and writes SRT, SSA, ASS, WebVTT, TTML or Jellyfin's JSON
// track format. ToUTF8 converts files in legacy character sets, detected
// from a byte order mark or statistically. It is a pure library with no
// I/O; formats it cannot read are converted to SRT with ffmpeg by the media
// pipeline.
//
// Subtitle.ApplyMods applies the subtitle modifications of Bazarr's
// Subzero by their Bazarr identifiers and in Subzero's order: common
// fixes, style tag removal, hearing-impaired removal, OCR fixes with
// Subzero's dictionaries, colors, emoji removal, punctuation reversal in
// right-to-left languages, upper-case fixes, time offsets, frame rate
// changes and two-point fits. DetectHearingImpaired tells hearing-impaired
// subtitles from their text as Bazarr does, DetectLanguage guesses the
// language of subtitles without one as Bazarr's guess_language does, and
// DetectChineseScript tells Simplified from Traditional Chinese by their
// characters. FixText repairs text as ftfy's fix_text does for Bazarr:
// mojibake, HTML entities, control characters and line breaks. The data
// these use is generated from its sources by internal/cmd/subzerogen and
// internal/cmd/ftfygen into data/, which records each origin.
package subtitle
