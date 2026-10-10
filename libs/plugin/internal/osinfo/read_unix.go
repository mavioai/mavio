//go:build !windows && !darwin

package osinfo

func read() (string, string, bool) { return readRelease("/etc") }
