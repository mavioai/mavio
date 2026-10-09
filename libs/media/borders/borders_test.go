package borders

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
)

func TestCombine(t *testing.T) {
	tests := []struct {
		name   string
		frames []imaging.BlackBorders
		want   core.Crop
	}{
		{"none", nil, core.Crop{}},
		{"letterbox in every frame", []imaging.BlackBorders{{Top: 141, Bottom: 140}, {Top: 139, Bottom: 141}}, core.Crop{Top: 138, Bottom: 140}},
		{"one frame without", []imaging.BlackBorders{{Top: 140, Bottom: 140}, {}}, core.Crop{}},
		{"thin border dropped", []imaging.BlackBorders{{Top: 10, Bottom: 10, Left: 240, Right: 240}}, core.Crop{Left: 240, Right: 240}},
		{"never the whole picture", []imaging.BlackBorders{{Top: 300, Bottom: 300}}, core.Crop{}},
	}
	for _, tt := range tests {
		if got := Combine(tt.frames, 1920, 1080); got != tt.want {
			t.Errorf("%s: got = %+v, want = %+v", tt.name, got, tt.want)
		}
	}
}

// TestDetect needs ffmpeg: a 2.39:1 picture letterboxed in 16:9.
func TestDetect(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	file := filepath.Join(t.TempDir(), "letterboxed.mkv")
	gen := exec.CommandContext(t.Context(), ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x268:rate=24",
		"-t", "3", "-vf", "pad=640:360:0:46:black", "-c:v", "libx264", "-pix_fmt", "yuv420p", file)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the video: %v\n%s", err, out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	crop, err := (&Detector{FFmpeg: ffmpeg, Samples: 3}).Detect(t.Context(), file, 3*time.Second, 640, 360)
	if err != nil {
		t.Fatal(err)
	}
	// 46 pixels above and below, up to the rounding of the detection.
	if crop.Left != 0 || crop.Right != 0 || crop.Top < 40 || crop.Top > 46 || crop.Bottom < 40 || crop.Bottom > 46 {
		t.Errorf("crop = %+v, want about 46 above and below", crop)
	}
}
