package keyframes

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Data is the keyframe layout of a file's first video stream.
type Data struct {
	// Duration is the stream duration, or the container duration when the
	// stream reports none.
	Duration time.Duration
	// Keyframes holds the presentation times of the keyframes in packet
	// order.
	Keyframes []time.Duration
}

// tick is the 100 ns unit ffprobe times are rounded to, matching the
// precision playback positions are stored with.
const tick = 100 * time.Nanosecond

// Extractor reads keyframes with ffprobe.
type Extractor struct {
	// FFprobe is the path of the ffprobe binary.
	FFprobe string
}

// Args returns the ffprobe arguments that list the keyframe packets of
// path's video streams together with the stream and format durations.
func Args(path string) []string {
	return []string{
		"-fflags", "+genpts", "-v", "error", "-skip_frame", "nokey",
		"-show_entries", "format=duration", "-show_entries", "stream=duration",
		"-show_entries", "packet=pts_time,flags",
		"-select_streams", "v", "-of", "csv", "file:" + path,
	}
}

// Extract runs ffprobe on the local file at path and parses its output.
func (e *Extractor) Extract(ctx context.Context, path string) (Data, error) {
	cmd := exec.CommandContext(ctx, e.FFprobe, Args(path)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Data{}, fmt.Errorf("ffprobe keyframes %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	d, err := Parse(&stdout)
	if err != nil {
		return Data{}, fmt.Errorf("ffprobe keyframes %s: %w", path, err)
	}
	return d, nil
}

// Parse reads the CSV ffprobe writes for [Args]: "packet,<pts>,<flags>"
// lines, of which those flagged "K_" are keyframes, and "stream,<duration>"
// and "format,<duration>" lines. The stream duration is preferred as the
// more accurate one. Malformed lines and values such as "N/A" are ignored.
func Parse(r io.Reader) (Data, error) {
	var (
		d              Data
		stream, format float64
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		kind, rest, ok := strings.Cut(sc.Text(), ",")
		if !ok {
			continue
		}
		switch {
		case strings.EqualFold(kind, "packet"):
			pts, flags, ok := strings.Cut(rest, ",")
			if !ok || !strings.HasPrefix(flags, "K_") {
				continue
			}
			if s, ok := seconds(pts); ok {
				// Rounded half to even, as .NET's Convert.ToInt64 does.
				d.Keyframes = append(d.Keyframes, time.Duration(math.RoundToEven(s*float64(time.Second/tick)))*tick)
			}
		case strings.EqualFold(kind, "stream"):
			if s, ok := seconds(rest); ok {
				stream = s
			}
		case strings.EqualFold(kind, "format"):
			if s, ok := seconds(rest); ok {
				format = s
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Data{}, fmt.Errorf("read ffprobe output: %w", err)
	}
	duration := format
	if stream > 0 {
		duration = stream
	}
	d.Duration = time.Duration(duration*float64(time.Second/tick)) * tick
	return d, nil
}

// seconds parses an unsigned decimal number of seconds without exponent,
// the only form ffprobe writes for times.
func seconds(s string) (float64, bool) {
	if s == "" || strings.Trim(s, "0123456789.") != "" || strings.Count(s, ".") > 1 || s == "." {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
