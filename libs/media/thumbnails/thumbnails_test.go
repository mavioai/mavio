package thumbnails

import (
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func ffmpegOrSkip(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	return path
}

// letterboxed makes a 25-second 320x240 video whose picture is 320x180
// between black bars, with a tone.
func letterboxed(t *testing.T, ffmpeg string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "film.mkv")
	cmd := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "25", "-vf", "pad=320:240:0:30:black",
		"-c:v", "libx264", "-g", "10", "-c:a", "aac", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot make the video: %v\n%s", err, out)
	}
	return file
}

func dims(t *testing.T, file string) (int, int) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestTrickplayAndChapterImages(t *testing.T) {
	ffmpeg := ffmpegOrSkip(t)
	film := letterboxed(t, ffmpeg)
	m := &Maker{FFmpeg: ffmpeg}
	v := Video{Duration: 25 * time.Second, Width: 320, Height: 240, Crop: core.Crop{Top: 30, Bottom: 30}}
	o := DefaultOptions()
	o.Width, o.TileWidth, o.TileHeight = 160, 2, 1
	dir := t.TempDir()
	tp, err := m.Trickplay(t.Context(), film, v, o, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Three thumbnails, at 0, 10 and 20 seconds, two to a sheet; the
	// borders are cropped away, so thumbnails are 16:9.
	if tp.ThumbnailCount != 3 || tp.Width != 160 || tp.Height != 90 || tp.Sheets() != 2 || tp.Bandwidth <= 0 {
		t.Errorf("trickplay = %+v", tp)
	}
	if w, h := dims(t, filepath.Join(dir, "0.jpg")); w != 320 || h != 90 {
		t.Errorf("sheet = %dx%d, want 320x90", w, h)
	}
	if _, err := os.Stat(filepath.Join(dir, "1.jpg")); err != nil {
		t.Errorf("second sheet: %v", err)
	}

	file := filepath.Join(t.TempDir(), "chapter.jpg")
	if err := m.ChapterImage(t.Context(), film, 12*time.Second, v, 320, file); err != nil {
		t.Fatal(err)
	}
	if w, h := dims(t, file); w != 320 || h != 180 {
		t.Errorf("chapter image = %dx%d, want 320x180", w, h)
	}
	if err := m.ChapterImage(t.Context(), film, time.Hour, v, 320, filepath.Join(t.TempDir(), "late.jpg")); err == nil {
		t.Error("an image past the end: no error")
	}

	lufs, err := m.Loudness(t.Context(), film)
	if err != nil || lufs > -10 || lufs < -40 {
		t.Errorf("Loudness = %v, %v", lufs, err)
	}
}

func TestParseLoudness(t *testing.T) {
	summary := `[Parsed_ebur128_0 @ 0x1] Summary:

  Integrated loudness:
    I:         -16.2 LUFS
    Threshold: -26.5 LUFS
`
	if v, ok := ParseLoudness(summary); !ok || v != -16.2 {
		t.Errorf("ParseLoudness = %v, %v", v, ok)
	}
	if _, ok := ParseLoudness("    I:         -inf LUFS\n"); ok {
		t.Error("silence measured")
	}
	if _, ok := ParseLoudness("nothing"); ok {
		t.Error("no summary measured")
	}
}
