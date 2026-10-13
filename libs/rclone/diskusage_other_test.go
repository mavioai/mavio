//go:build !darwin && !linux

package rclone

// diskUsage reports that the disk usage of files is not known here.
func diskUsage(string) (int64, bool) { return 0, false }
