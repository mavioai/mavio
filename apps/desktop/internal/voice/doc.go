// Package voice is the audio of the assistant's voice sessions
// (docs/assistant.md §8, §9.5): the microphone captured with the
// platform's echo cancellation, as 16 kHz mono 16-bit PCM in 20 ms frames,
// and the answers' speech played through the same voice processing, whose
// echo it removes. On macOS the voice processing I/O unit captures the
// system's default input; a microphone chosen in the settings is captured
// by a unit of its own without it. Other platforms have no capture yet.
package voice
