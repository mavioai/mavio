package borders

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
	"github.com/mavioai/mavio/libs/media/keyframes"
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
	for _, tt := range []struct {
		name, ext string
		seek      bool
		// noIndex is set when the file is not read without an index.
		noIndex bool
	}{
		{"sampled through the Matroska index", "mkv", false, false},
		{"sampled through the MP4 index", "mp4", false, false},
		{"seeks without an index", "ts", true, false},
		{"no index", "ts", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A second longer than the duration given: a seek near the end
			// of a transport stream finds no frame.
			file := filepath.Join(t.TempDir(), "letterboxed."+tt.ext)
			gen := exec.CommandContext(t.Context(), ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x268:rate=24",
				"-t", "4", "-vf", "pad=640:360:0:46:black", "-c:v", "libx264", "-g", "24", "-pix_fmt", "yuv420p", file)
			if out, err := gen.CombinedOutput(); err != nil {
				t.Skipf("integration test: cannot generate the video: %v\n%s", err, out)
			}
			in := Input{URL: "file:" + file, Open: func() (keyframes.File, error) { return os.Open(file) }}
			crop, err := (&Detector{FFmpeg: ffmpeg, Samples: 3}).Detect(t.Context(), in, 3*time.Second, 640, 360, tt.seek)
			if tt.noIndex {
				if !errors.Is(err, keyframes.ErrNoIndex) {
					t.Errorf("Detect() error = %v, want = %v", err, keyframes.ErrNoIndex)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// 46 pixels above and below, up to the rounding of the detection.
			if crop.Left != 0 || crop.Right != 0 || crop.Top < 40 || crop.Top > 46 || crop.Bottom < 40 || crop.Bottom > 46 {
				t.Errorf("crop = %+v, want about 46 above and below", crop)
			}
		})
	}
}
