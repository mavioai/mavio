// Package borders finds the black borders of a video by sampling frames
// with ffmpeg and measuring their luma with imaging.DetectBlackBordersY.
// Borders count only where every sampled frame has them, so that a dark
// scene does not pass for a letterbox.
package borders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
)

// Defaults of a Detector.
const (
	DefaultSamples = 5
	// MinFraction is the least share of a dimension a border must cover to
	// be cropped; thinner ones are encoder noise or overscan.
	MinFraction = 0.02
)

// Detector samples frames with ffmpeg.
type Detector struct {
	FFmpeg string
	// Samples is the number of frames sampled, spread over the video;
	// zero means DefaultSamples.
	Samples int
	// Threshold is the luma below which a line is black; zero means
	// imaging.DefaultBlackThreshold.
	Threshold uint8
}

// Args returns the ffmpeg arguments writing the frame at offset of the
// first video stream as raw 8-bit luma of width × height.
func Args(path string, offset time.Duration, width, height int) []string {
	return []string{
		"-v", "error", "-nostdin",
		"-ss", strconv.FormatFloat(offset.Seconds(), 'f', 3, 64),
		"-i", path, "-map", "0:v:0", "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d,format=gray", width, height),
		"-f", "rawvideo", "-",
	}
}

// Detect returns the borders of a video of the given duration and frame
// size: zero when it has none.
func (d *Detector) Detect(ctx context.Context, path string, duration time.Duration, width, height int) (core.Crop, error) {
	if duration <= 0 || width < 16 || height < 16 {
		return core.Crop{}, errors.New("borders: unknown duration or frame size")
	}
	n := d.Samples
	if n <= 0 {
		n = DefaultSamples
	}
	var frames []imaging.BlackBorders
	for i := range n {
		offset := duration * time.Duration(i+1) / time.Duration(n+1)
		var out, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, d.FFmpeg, Args(path, offset, width, height)...)
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			return core.Crop{}, fmt.Errorf("borders: frame at %v: %w: %s", offset, err, bytes.TrimSpace(stderr.Bytes()))
		}
		if out.Len() < width*height {
			return core.Crop{}, fmt.Errorf("borders: frame at %v has %d bytes, want %d", offset, out.Len(), width*height)
		}
		b, ok := imaging.DetectBlackBordersY(out.Bytes(), width, height, width, d.Threshold)
		if !ok && b.IsZero() {
			// No border, or a frame too dark to tell.
			if isBlack(out.Bytes()[:width*height], d.threshold()) {
				continue
			}
		}
		frames = append(frames, b)
	}
	return Combine(frames, width, height), nil
}

func (d *Detector) threshold() uint8 {
	if d.Threshold == 0 {
		return imaging.DefaultBlackThreshold
	}
	return d.Threshold
}

// isBlack reports whether a luma plane is black throughout, as a fade
// is, which tells nothing about borders.
func isBlack(luma []byte, threshold uint8) bool {
	for i := 0; i < len(luma); i += 97 {
		if luma[i] > threshold {
			return false
		}
	}
	return true
}

// Combine returns the borders all frames share, dropping those thinner
// than MinFraction of the frame and rounding to even pixels, as chroma
// subsampling needs; no frames give no borders.
func Combine(frames []imaging.BlackBorders, width, height int) core.Crop {
	if len(frames) == 0 {
		return core.Crop{}
	}
	c := core.Crop{Top: frames[0].Top, Bottom: frames[0].Bottom, Left: frames[0].Left, Right: frames[0].Right}
	for _, f := range frames[1:] {
		c.Top, c.Bottom = min(c.Top, f.Top), min(c.Bottom, f.Bottom)
		c.Left, c.Right = min(c.Left, f.Left), min(c.Right, f.Right)
	}
	keep := func(px, size int) int {
		if float64(px) < MinFraction*float64(size) {
			return 0
		}
		return px &^ 1
	}
	c.Top, c.Bottom = keep(c.Top, height), keep(c.Bottom, height)
	c.Left, c.Right = keep(c.Left, width), keep(c.Right, width)
	// Never crop away the picture.
	if c.Top+c.Bottom >= height/2 || c.Left+c.Right >= width/2 {
		return core.Crop{}
	}
	return c
}
