package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestFFmpegPins checks that the container image verifies jellyfin-ffmpeg
// against the same version and SHA-256 digests mise.toml pins for
// development and CI, so that the two pins cannot drift apart.
func TestFFmpegPins(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	dockerfile, err := os.ReadFile(filepath.Join(root, "apps", "server", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	mise, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	version := regexp.MustCompile(`ARG JELLYFIN_FFMPEG_VERSION=(\S+)`).FindSubmatch(dockerfile)
	pinned := regexp.MustCompile(`(?s)\[tools\."github:jellyfin/jellyfin-ffmpeg"\]\s*version = "([^"]+)"`).FindSubmatch(mise)
	if version == nil || pinned == nil || string(version[1]) != string(pinned[1]) {
		t.Fatalf("jellyfin-ffmpeg versions: Dockerfile %q, mise.toml %q", version, pinned)
	}
	for arch, platform := range map[string]string{"amd64": "linux-x64", "arm64": "linux-arm64"} {
		digest := regexp.MustCompile(`ARG SHA256_` + arch + `=([0-9a-f]{64})`).FindSubmatch(dockerfile)
		entry := regexp.MustCompile(`(?m)^` + platform + ` = \{[^}]*checksum = "sha256:([0-9a-f]{64})"`).FindSubmatch(mise)
		switch {
		case digest == nil:
			t.Errorf("the Dockerfile pins no digest for %s", arch)
		case entry == nil:
			t.Errorf("mise.toml pins no checksum for %s", platform)
		case string(entry[1]) != string(digest[1]):
			t.Errorf("%s: the Dockerfile verifies %s, mise.toml pins %s", platform, digest[1], entry[1])
		}
	}
}
