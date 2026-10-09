package storage

import (
	"testing"
)

func TestDetectDevice(t *testing.T) {
	tempDir := t.TempDir()

	info, err := DetectDevice(tempDir)
	if err != nil {
		t.Fatalf("DetectDevice(%q) failed: %v", tempDir, err)
	}

	if info.ID == "" {
		t.Errorf("got empty ID, want non-empty device/volume ID")
	}
	if info.MountPoint == "" {
		t.Errorf("got empty MountPoint, want non-empty mount point")
	}
	if info.Kind == "" || info.Kind == KindUnknown {
		t.Errorf("got Kind %q, want valid detected Kind", info.Kind)
	}

	t.Logf("Detected storage for %s: ID=%s, Kind=%s, FSType=%s, Rotational=%v, Remote=%v",
		tempDir, info.ID, info.Kind, info.FSType, info.Rotational, info.Remote)
}

func TestDetectDevice_EmptyPath(t *testing.T) {
	_, err := DetectDevice("")
	if err == nil {
		t.Errorf("DetectDevice(\"\") got nil error, want error")
	}
}

func TestDetectDevice_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := tempDir + "/sub/path/that/does/not/exist"

	info, err := DetectDevice(nonExistent)
	// It should either resolve parent mount or fail cleanly without panic
	if err == nil && info.ID == "" {
		t.Errorf("got empty ID for non-existent path")
	}
}

func TestCloudPathDetection(t *testing.T) {
	cloud1 := "/Users/test/Library/Mobile Documents/com~apple~CloudDocs/file.mkv"
	cloud2 := "/Users/test/Library/CloudStorage/OneDrive-Personal/file.mkv"
	local := "/Users/test/Movies/file.mkv"

	if !isCloudPath(cloud1) {
		t.Errorf("isCloudPath(%q) = false, want true", cloud1)
	}
	if !isCloudPath(cloud2) {
		t.Errorf("isCloudPath(%q) = false, want true", cloud2)
	}
	if isCloudPath(local) {
		t.Errorf("isCloudPath(%q) = true, want false", local)
	}
}
