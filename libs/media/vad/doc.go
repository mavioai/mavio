// Package vad detects speech in audio with Silero VAD's 16 kHz model
// (github.com/snakers4/silero-vad, MIT), its inference written in Go: a
// short-time Fourier transform as a convolution, four convolutions, an LSTM
// cell and a sigmoid, over windows of 512 samples (32 ms) each preceded by
// the 64 samples before it.
//
// The weights are read from Silero's ONNX file by internal/cmd/silerogen and
// embedded (weights.bin); ModelSource names the file they come from. A
// Detector keeps the LSTM's state across the windows of one stream;
// Model.Detect computes a whole stream's probabilities with the convolutions
// of many windows in parallel.
package vad
