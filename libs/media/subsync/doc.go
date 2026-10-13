// Package subsync synchronizes subtitles with a video's speech or with
// another subtitle, porting ffsubsync's algorithm
// (github.com/smacke/ffsubsync, MIT), which Bazarr runs.
//
// Both sides become speech signals at SampleRate: the subtitle's cues mark
// speech where they show text, the reference's come from its cues or from
// Silero VAD's probabilities over its audio (AudioSpeech). Align finds the
// offset maximizing their cross-correlation, computed by FFT, for the
// subtitle as timed and, as options ask, scaled by the ratios between common
// frame rates, by ratios a golden-section search finds and by the ratio of
// the two signals' spans. Result.Problems is ffsubsync's quality gate, and
// Result.Apply retimes cues.
package subsync
