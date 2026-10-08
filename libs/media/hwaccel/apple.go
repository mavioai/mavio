package hwaccel

import "strings"

// HasAV1HardwareDecode reports whether an Apple CPU decodes AV1 in hardware:
// Apple silicon from M3 on does; M1 and M2 do not.
func HasAV1HardwareDecode(goarch, cpuBrand string) bool {
	if goarch != "arm64" {
		return false
	}
	brand := strings.ToLower(cpuBrand)
	return !strings.Contains(brand, "m1") && !strings.Contains(brand, "m2")
}
