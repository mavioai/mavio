package procs

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// identify returns a process's start time, in clock ticks after boot, and
// its executable.
func identify(pid int) (start, path string, err error) {
	dir := "/proc/" + strconv.Itoa(pid)
	b, err := os.ReadFile(dir + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return "", "", errGone
	}
	if err != nil {
		return "", "", err
	}
	// The command name, in parentheses, may hold spaces and parentheses.
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return "", "", fmt.Errorf("%s/stat: %q", dir, b)
	}
	// After it: state (field 3) … starttime (field 22).
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 20 {
		return "", "", fmt.Errorf("%s/stat: %q", dir, b)
	}
	if f[0] == "Z" || f[0] == "X" {
		return "", "", errGone
	}
	path, err = os.Readlink(dir + "/exe")
	if errors.Is(err, os.ErrNotExist) {
		return "", "", errGone
	}
	if err != nil {
		return "", "", err
	}
	return f[19], path, nil
}
