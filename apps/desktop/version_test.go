package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionOf(t *testing.T) {
	built := func(settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Settings: settings}, true }
	}
	commit := debug.BuildSetting{Key: "vcs.revision", Value: "e0e7beb4a9e9557ad3b53818d433cb23523cd99f"}
	tests := []struct {
		name   string
		linked string
		info   func() (*debug.BuildInfo, bool)
		want   string
	}{
		{"release with v", "v0.1.0", built(commit), "0.1.0"},
		{"release without v", "0.2.0-rc.1", built(), "0.2.0-rc.1"},
		{"commit", "", built(commit), "0.0.0-e0e7beb4a9e9"},
		{"no commit", "", built(debug.BuildSetting{Key: "vcs", Value: "git"}), "dev"},
		{"no build information", "", func() (*debug.BuildInfo, bool) { return nil, false }, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionOf(tt.linked, tt.info); got != tt.want {
				t.Errorf("versionOf(%q) = %q, want = %q", tt.linked, got, tt.want)
			}
		})
	}
}
