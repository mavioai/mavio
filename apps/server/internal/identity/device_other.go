//go:build !linux && !darwin && !windows

package identity

// machineID returns empty: the server's ID stands for the machine.
func machineID() string { return "" }
