package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mavioai/mavio/apps/desktop/internal/app"
)

// systemFacts tells the system's name and version and the device's maker
// and model, as far as the system tells them.
func systemFacts(ctx context.Context) app.System {
	s := app.System{Version: platforms[runtime.GOOS]}
	switch runtime.GOOS {
	case "darwin":
		if v := output(ctx, "sw_vers", "-productVersion"); v != "" {
			s.Version = "macOS " + v
		}
		s.Manufacturer, s.Model = "Apple", output(ctx, "sysctl", "-n", "hw.model")
	case "linux":
		if v := osRelease("PRETTY_NAME"); v != "" {
			s.Version = v
		}
		s.Manufacturer, s.Model = readTrimmed("/sys/class/dmi/id/sys_vendor"), readTrimmed("/sys/class/dmi/id/product_name")
	}
	return s
}

// output runs a command and returns what it printed, trimmed, or nothing
// when it failed.
func output(ctx context.Context, name string, args ...string) string {
	b, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// readTrimmed returns a file's contents, trimmed, or nothing.
func readTrimmed(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// osRelease returns a field of /etc/os-release, unquoted.
func osRelease(key string) string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), key+"="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}
