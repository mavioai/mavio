package load

import (
	"os"
	"strings"
)

// wholeDisk tells whether a block device is a whole disk, not a partition
// of one, nor a device stacked on others, as LVM's, RAID's, loops and
// memory are, whose bytes the disks below count already.
func wholeDisk(name string) bool {
	for _, p := range []string{"loop", "ram", "zram", "dm-", "md", "sr", "nbd"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	_, err := os.Stat("/sys/block/" + name)
	return err == nil
}

// virtualInterface tells whether a network interface is software's, as
// bridges and veth pairs are.
func virtualInterface(name string) bool {
	_, err := os.Stat("/sys/devices/virtual/net/" + name)
	return err == nil
}
