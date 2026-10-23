// Package borders finds the black borders of a video by sampling frames,
// keyframes read through the container's index or else seeked to, with
// ffmpeg and measuring their luma with imaging.DetectBlackBordersY.
// Borders count only where every sampled frame has them, so that a dark
// scene does not pass for a letterbox.
package borders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
	"github.com/mavioai/mavio/libs/media/keyframes"
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
func Args(input string, offset time.Duration, width, height int) []string {
	return []string{
		"-v", "error", "-nostdin",
		"-ss", strconv.FormatFloat(offset.Seconds(), 'f', 3, 64),
		"-i", input, "-map", "0:v:0", "-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d,format=gray", width, height),
		"-f", "rawvideo", "-",
	}
}

// Input is a video as a Detector reads it.
type Input struct {
	// URL is what ffmpeg reads to seek: a file: URL, or an HTTP URL.
	URL string
	// Open opens the video for reads at offsets, which sampling through
	// its index takes.
	Open func() (keyframes.File, error)
}

// SampledArgs returns the ffmpeg arguments writing each picture on stdin
// as raw 8-bit luma of width × height.
func SampledArgs(width, height int) []string {
	return []string{
		"-v", "error", "-threads", "1", "-f", "matroska", "-i", "pipe:0",
		"-map", "0:v:0", "-fps_mode", "passthrough",
		"-vf", fmt.Sprintf("scale=%d:%d,format=gray", width, height),
		"-f", "rawvideo", "-",
	}
}

// Detect returns the borders of a video of the given duration and frame
// size: zero when it has none. Its frames are the keyframes at or after
// times spread over it, each read on its own through the container's
// index; when seek is set and that fails, as for a file without an index,
// ffmpeg seeks to each time instead, and when it is not, such files give
// keyframes.ErrNoIndex.
func (d *Detector) Detect(ctx context.Context, in Input, duration time.Duration, width, height int, seek bool) (core.Crop, error) {
	if duration <= 0 || width < 16 || height < 16 {
		return core.Crop{}, errors.New("borders: unknown duration or frame size")
	}
	n := d.Samples
	if n <= 0 {
		n = DefaultSamples
	}
	at := make([]time.Duration, n)
	for i := range at {
		at[i] = duration * time.Duration(i+1) / time.Duration(n+1)
	}
	frames, err := d.sampled(ctx, in, at, width, height)
	if err != nil && seek && ctx.Err() == nil {
		frames, err = d.seeked(ctx, in.URL, at, width, height)
	}
	if err != nil {
		return core.Crop{}, err
	}
	var found []imaging.BlackBorders
	for _, luma := range frames {
		b, ok := imaging.DetectBlackBordersY(luma, width, height, width, d.Threshold)
		if !ok && b.IsZero() {
			// No border, or a frame too dark to tell.
			if isBlack(luma, d.threshold()) {
				continue
			}
		}
		found = append(found, b)
	}
	return Combine(found, width, height), nil
}

// sampled returns the luma of the keyframes at or after the times, read
// through the file's index and decoded by one ffmpeg run.
func (d *Detector) sampled(ctx context.Context, in Input, at []time.Duration, width, height int) ([][]byte, error) {
	f, size, s, err := keyframes.OpenSampler(in.Open)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	times, _ := keyframes.AtOrAfter(s.Keyframes(), at)
	if len(times) == 0 {
		return nil, keyframes.ErrNoIndex
	}
	pr, pw := io.Pipe()
	written := make(chan error, 1)
	go func() {
		err := s.WriteSamples(ctx, pw, f, size, times)
		pw.CloseWithError(err)
		written <- err
	}()
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, d.FFmpeg, SampledArgs(width, height)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = pr, &out, &stderr
	err = cmd.Run()
	// ffmpeg stopping early ends the writer.
	pr.CloseWithError(io.ErrClosedPipe)
	if werr := <-written; werr != nil && !errors.Is(werr, io.ErrClosedPipe) {
		return nil, fmt.Errorf("borders: %w", werr)
	}
	if err != nil {
		return nil, fmt.Errorf("borders: sampled frames: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	frame := width * height
	if out.Len() < frame {
		return nil, fmt.Errorf("borders: sampled %d bytes, want %d a frame", out.Len(), frame)
	}
	var frames [][]byte
	for b := out.Bytes(); len(b) >= frame; b = b[frame:] {
		frames = append(frames, b[:frame])
	}
	return frames, nil
}

// seeked returns the luma of the frames at the times, an ffmpeg seek
// each.
func (d *Detector) seeked(ctx context.Context, input string, at []time.Duration, width, height int) ([][]byte, error) {
	var frames [][]byte
	for _, offset := range at {
		var out, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, d.FFmpeg, Args(input, offset, width, height)...)
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("borders: frame at %v: %w: %s", offset, err, bytes.TrimSpace(stderr.Bytes()))
		}
		if out.Len() < width*height {
			return nil, fmt.Errorf("borders: frame at %v has %d bytes, want %d", offset, out.Len(), width*height)
		}
		frames = append(frames, out.Bytes()[:width*height])
	}
	return frames, nil
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
