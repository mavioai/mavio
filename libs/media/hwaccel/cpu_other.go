//go:build !darwin

package hwaccel

import "errors"

// CPUBrand returns the CPU brand string; it is only implemented on macOS.
func CPUBrand() (string, error) {
	return "", errors.New("CPU brand: not supported on this platform")
}
