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
	dockerfile, mise := pins(t)
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

// TestRclonePins checks that the container image verifies mavioai/rclone
// against the same release and SHA-256 digests mise.toml pins, and that
// every platform mise.toml pins downloads that release.
func TestRclonePins(t *testing.T) {
	dockerfile, mise := pins(t)
	version := regexp.MustCompile(`ARG RCLONE_VERSION=(\S+)`).FindSubmatch(dockerfile)
	pinned := regexp.MustCompile(`(?s)\[tools\."http:rclone"\]\s*version = "([^"]+)"`).FindSubmatch(mise)
	if version == nil || pinned == nil || string(version[1]) != string(pinned[1]) {
		t.Fatalf("rclone versions: Dockerfile %q, mise.toml %q", version, pinned)
	}
	urls := regexp.MustCompile(`url = "https://github\.com/mavioai/rclone/releases/download/v([^/]+)/rclone-v([^-]+-[^-]+)-`).FindAllSubmatch(mise, -1)
	if len(urls) == 0 {
		t.Fatal("mise.toml pins no rclone downloads")
	}
	for _, u := range urls {
		if string(u[1]) != string(pinned[1]) || string(u[2]) != string(pinned[1]) {
			t.Errorf("mise.toml downloads rclone %s, want = %s", u[0], pinned[1])
		}
	}
	for arch, asset := range map[string]string{"amd64": "linux-amd64", "arm64": "linux-arm64"} {
		digest := regexp.MustCompile(`ARG RCLONE_SHA256_` + arch + `=([0-9a-f]{64})`).FindSubmatch(dockerfile)
		entry := regexp.MustCompile(`rclone-v[^"]*-` + asset + `\.zip", checksum = "sha256:([0-9a-f]{64})"`).FindSubmatch(mise)
		switch {
		case digest == nil:
			t.Errorf("the Dockerfile pins no rclone digest for %s", arch)
		case entry == nil:
			t.Errorf("mise.toml pins no rclone checksum for %s", asset)
		case string(entry[1]) != string(digest[1]):
			t.Errorf("rclone %s: the Dockerfile verifies %s, mise.toml pins %s", asset, digest[1], entry[1])
		}
	}
}

// TestBunPins checks that the Dockerfile compiles the agent plugin with the
// Bun mise.toml pins.
func TestBunPins(t *testing.T) {
	dockerfile, mise := pins(t)
	version := regexp.MustCompile(`ARG BUN_VERSION=(\S+)`).FindSubmatch(dockerfile)
	pinned := regexp.MustCompile(`(?m)^bun = "([^"]+)"`).FindSubmatch(mise)
	if version == nil || pinned == nil || string(version[1]) != string(pinned[1]) {
		t.Errorf("Bun versions: Dockerfile %q, mise.toml %q", version, pinned)
	}
}

// pins reads the Dockerfile and mise.toml.
func pins(t *testing.T) (dockerfile, mise []byte) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	dockerfile, err := os.ReadFile(filepath.Join(root, "apps", "server", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	mise, err = os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return dockerfile, mise
}
