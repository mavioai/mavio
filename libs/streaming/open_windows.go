//go:build windows

package streaming

import (
	"errors"
	"syscall"
)

// errorSharingViolation is ERROR_SHARING_VIOLATION.
const errorSharingViolation = syscall.Errno(32)

// sharingViolation reports whether err is Windows refusing to open a file
// another process holds, as it briefly does around ffmpeg renaming a
// finished segment into place, or an antivirus scanning it.
func sharingViolation(err error) bool {
	return errors.Is(err, errorSharingViolation)
}
