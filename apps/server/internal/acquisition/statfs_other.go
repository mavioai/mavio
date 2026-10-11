//go:build !(darwin || linux || freebsd)

package acquisition

// freeSpace reports free space unknown where it is not read.
func freeSpace(string) (int64, bool) { return 0, false }
