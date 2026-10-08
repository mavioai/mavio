package hwaccel

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// CPUBrand returns the CPU brand string, such as "Apple M3 Pro".
func CPUBrand() (string, error) {
	brand, err := unix.Sysctl("machdep.cpu.brand_string")
	if err != nil {
		return "", fmt.Errorf("sysctl machdep.cpu.brand_string: %w", err)
	}
	return brand, nil
}
