//go:build !linux

package load

// wholeDisk tells whether a disk's counters are counted: all of them, as
// the operating system lists whole disks, or on Windows its volumes.
func wholeDisk(string) bool { return true }

// virtualInterface tells whether a network interface is software's; known
// only on Linux.
func virtualInterface(string) bool { return false }
