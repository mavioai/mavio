// Package thumbnails makes the images and measurements of media that ffmpeg
// reads off it: trickplay thumbnail sheets, which the fps, scale and tile
// filters produce in one pass, chapter images, and the integrated
// loudness of audio (EBU R128).
package thumbnails

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Maker runs ffmpeg.
type Maker struct {
	FFmpeg string
}

// Video describes the video a thumbnail is taken of.
type Video struct {
	Duration      time.Duration
	Width, Height int
	// Crop is cut off before scaling.
	Crop core.Crop
}

// size returns the height of a thumbnail width pixels wide of the cropped
// picture, even.
func (v Video) height(width int) int {
	w, h := v.Width-v.Crop.Left-v.Crop.Right, v.Height-v.Crop.Top-v.Crop.Bottom
	if w <= 0 || h <= 0 {
		return width * 9 / 16 &^ 1
	}
	return max(2, int(math.Round(float64(width)*float64(h)/float64(w)/2))*2)
}

// filters returns the crop and scale filters to a thumbnail width.
func (v Video) filters(width int) []string {
	var out []string
	if c := v.Crop; !c.IsZero() {
		out = append(out, fmt.Sprintf("crop=%d:%d:%d:%d", v.Width-c.Left-c.Right, v.Height-c.Top-c.Bottom, c.Left, c.Top))
	}
	return append(out, fmt.Sprintf("scale=%d:%d", width, v.height(width)))
}

// Options shape trickplay sheets.
type Options struct {
	// Width is a thumbnail's width; Interval the time between thumbnails;
	// TileWidth and TileHeight the thumbnails across and down a sheet.
	Width                 int
	Interval              time.Duration
	TileWidth, TileHeight int
	// Quality is the JPEG quality scale of ffmpeg, 2 (best) to 31.
	Quality int
}

// DefaultOptions are Jellyfin's: 320 pixels wide, every ten seconds, ten
// by ten to a sheet.
func DefaultOptions() Options {
	return Options{Width: 320, Interval: 10 * time.Second, TileWidth: 10, TileHeight: 10, Quality: 4}
}

// TrickplayArgs returns the ffmpeg arguments writing the sheets of a
// video into dir as 0.jpg, 1.jpg, ….
func TrickplayArgs(path string, v Video, o Options, dir string) []string {
	vf := append([]string{"fps=1/" + strconv.FormatFloat(o.Interval.Seconds(), 'f', -1, 64)}, v.filters(o.Width)...)
	vf = append(vf, fmt.Sprintf("tile=%dx%d", o.TileWidth, o.TileHeight))
	return []string{
		"-v", "error", "-nostdin", "-i", path, "-map", "0:v:0", "-an", "-sn", "-dn",
		"-vf", strings.Join(vf, ","), "-q:v", strconv.Itoa(o.Quality), "-f", "image2", "-start_number", "0",
		filepath.Join(dir, "%d.jpg"),
	}
}

// Trickplay writes the sheets of a video into dir, which must exist, and
// describes them.
func (m *Maker) Trickplay(ctx context.Context, path string, v Video, o Options, dir string) (core.Trickplay, error) {
	if v.Duration <= 0 {
		return core.Trickplay{}, errors.New("thumbnails: video of unknown duration")
	}
	if err := m.run(ctx, TrickplayArgs(path, v, o, dir)); err != nil {
		return core.Trickplay{}, fmt.Errorf("trickplay of %s: %w", path, err)
	}
	count := int(math.Ceil(v.Duration.Seconds() / o.Interval.Seconds()))
	t := core.Trickplay{
		Width: o.Width, Height: v.height(o.Width), TileWidth: o.TileWidth, TileHeight: o.TileHeight,
		ThumbnailCount: count, Interval: o.Interval,
	}
	// The sheets ffmpeg wrote decide the count: the last may be partial.
	sheets := 0
	largest := int64(0)
	for ; ; sheets++ {
		info, err := os.Stat(filepath.Join(dir, strconv.Itoa(sheets)+".jpg"))
		if err != nil {
			break
		}
		largest = max(largest, info.Size())
	}
	if sheets == 0 {
		return core.Trickplay{}, fmt.Errorf("trickplay of %s: ffmpeg wrote no sheets", path)
	}
	per := o.TileWidth * o.TileHeight
	t.ThumbnailCount = min(count, sheets*per)
	if t.ThumbnailCount <= (sheets-1)*per {
		t.ThumbnailCount = (sheets-1)*per + 1
	}
	// A sheet serves per thumbnails' worth of playback.
	t.Bandwidth = int(math.Ceil(float64(largest*8) / (float64(per) * o.Interval.Seconds())))
	return t, nil
}

// ChapterImageArgs returns the ffmpeg arguments writing the frame at a
// time as a JPEG file width pixels wide.
func ChapterImageArgs(path string, at time.Duration, v Video, width int, file string) []string {
	return []string{
		"-v", "error", "-nostdin", "-ss", strconv.FormatFloat(at.Seconds(), 'f', 3, 64), "-i", path,
		"-map", "0:v:0", "-an", "-sn", "-dn", "-frames:v", "1", "-vf", strings.Join(v.filters(width), ","),
		"-q:v", "3", "-f", "image2", "-update", "1", file,
	}
}

// ChapterImage writes the frame of a video at a time into file.
func (m *Maker) ChapterImage(ctx context.Context, path string, at time.Duration, v Video, width int, file string) error {
	if err := m.run(ctx, ChapterImageArgs(path, at, v, width, file)); err != nil {
		return fmt.Errorf("chapter image of %s at %v: %w", path, at, err)
	}
	if info, err := os.Stat(file); err != nil || info.Size() == 0 {
		return fmt.Errorf("chapter image of %s at %v: ffmpeg wrote no frame", path, at)
	}
	return nil
}

// LoudnessArgs returns the ffmpeg arguments measuring the first audio
// stream of a file.
func LoudnessArgs(path string) []string {
	return []string{"-hide_banner", "-nostats", "-nostdin", "-v", "info", "-i", path, "-map", "0:a:0", "-af", "ebur128=framelog=quiet", "-f", "null", "-"}
}

// integrated finds the summary's integrated loudness.
var integrated = regexp.MustCompile(`(?m)^\s*I:\s+(-?[0-9.]+|-inf) LUFS`)

// ParseLoudness reads the integrated loudness from ffmpeg's ebur128
// summary; silence has none.
func ParseLoudness(stderr string) (float64, bool) {
	all := integrated.FindAllStringSubmatch(stderr, -1)
	if len(all) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(all[len(all)-1][1], 64)
	if err != nil || math.IsInf(v, 0) || v < -70 {
		return 0, false
	}
	return v, true
}

// Loudness measures the integrated loudness of a file's first audio
// stream, in LUFS.
func (m *Maker) Loudness(ctx context.Context, path string) (float64, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, m.FFmpeg, LoudnessArgs(path)...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("loudness of %s: %w: %s", path, err, lastLines(stderr.String()))
	}
	v, ok := ParseLoudness(stderr.String())
	if !ok {
		return 0, fmt.Errorf("loudness of %s: no integrated loudness", path)
	}
	return v, nil
}

func (m *Maker) run(ctx context.Context, args []string) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, m.FFmpeg, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, lastLines(stderr.String()))
	}
	return nil
}

// lastLines keeps the end of ffmpeg's output, where its errors are.
func lastLines(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1000 {
		s = s[len(s)-1000:]
	}
	return s
}
