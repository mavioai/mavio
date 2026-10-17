//go:build !linux && !darwin && !windows

package procs

// identify tells nothing on this system: processes are not recorded.
func identify(int) (start, path string, err error) { return "", "", errUnsupported }
