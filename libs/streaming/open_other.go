//go:build !windows

package streaming

// sharingViolation reports whether err is Windows refusing to open a file
// another process holds; other systems do not refuse.
func sharingViolation(error) bool { return false }
