package procs

import (
	"errors"
	"strconv"

	"golang.org/x/sys/windows"
)

// stillActive is the exit code of a process that has not exited.
const stillActive = 259

// identify returns a process's creation time and its executable.
func identify(pid int) (start, path string, err error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return "", "", errGone
	}
	if err != nil {
		return "", "", err
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return "", "", err
	}
	if code != stillActive {
		return "", "", errGone
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", "", err
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", "", err
	}
	return strconv.FormatInt(created.Nanoseconds(), 10), windows.UTF16ToString(buf[:n]), nil
}
