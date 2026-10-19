package identity

import (
	"context"
	"os/exec"
	"regexp"
	"time"
)

var platformUUID = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([0-9A-Fa-f-]+)"`)

// machineID returns the Mac's IOPlatformUUID, read from ioreg, or empty
// when it cannot be read.
func machineID() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	if m := platformUUID.FindSubmatch(out); m != nil && usable(string(m[1])) {
		return string(m[1])
	}
	return ""
}
