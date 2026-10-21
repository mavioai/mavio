//go:build !darwin

package voice

// System returns the platform's audio, nil where it has no capture yet.
func System() Audio { return nil }
