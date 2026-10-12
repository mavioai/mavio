// Package thumbnails makes the images and measurements of media that ffmpeg
// reads off it: trickplay thumbnail sheets, which the scale and tile
// filters produce in one pass from a keyframe a thumbnail, chapter images,
// and the integrated loudness of audio (EBU R128).
package thumbnails

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Maker runs ffmpeg.
type Maker struct {
	FFmpeg string
}

// Input is a video as Maker reads it.
type Input struct {
	// URL is what ffmpeg reads: a file: URL, or an HTTP URL.
	URL string
	// Open opens the video for reads at offsets, which sampling through
	// its index takes.
	Open func() (File, error)
}

// File is an open video.
type File interface {
	io.ReaderAt
	io.Closer
	Stat() (fs.FileInfo, error)
}

// LocalInput returns the input of the local file at path.
func LocalInput(path string) Input {
	return Input{URL: "file:" + path, Open: func() (File, error) { return os.Open(path) }}
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

// Mode is how trickplay reads a video.
type Mode int

const (
	// Sampled decodes a keyframe a thumbnail, each read on its own through
	// the Matroska index, from a stream on stdin.
	Sampled Mode = iota
	// KeyframesOnly decodes the keyframes alone, reading the whole file.
	KeyframesOnly
	// AllFrames decodes every frame.
	AllFrames
)

// TrickplayArgs returns the ffmpeg arguments writing the sheets of a
// video into dir as 0.jpg, 1.jpg, …: in Sampled mode from the pictures on
// stdin, a thumbnail each, and otherwise from input, the URL ffmpeg reads,
// at the interval. ffmpeg decodes and filters on one thread, as a background
// task.
func TrickplayArgs(input string, mode Mode, v Video, o Options, dir string) []string {
	args := []string{"-v", "error", "-filter_threads", "1", "-threads", "1"}
	var vf []string
	switch mode {
	case Sampled:
		args = append(args, "-f", "matroska", "-i", "pipe:0")
	case KeyframesOnly:
		args = append(args, "-nostdin", "-skip_frame", "nokey", "-i", input)
	default:
		args = append(args, "-nostdin", "-i", input)
	}
	// A thumbnail is the first picture at or after its time, so that the
	// last is there when the keyframes end before the next time.
	vf = append(vf, "fps=1/"+strconv.FormatFloat(o.Interval.Seconds(), 'f', -1, 64)+":round=down:eof_action=pass")
	vf = append(vf, v.filters(o.Width)...)
	vf = append(vf, fmt.Sprintf("tile=%dx%d", o.TileWidth, o.TileHeight))
	return append(args, "-map", "0:v:0", "-an", "-sn", "-dn",
		"-vf", strings.Join(vf, ","), "-q:v", strconv.Itoa(o.Quality), "-f", "image2", "-start_number", "0",
		filepath.Join(dir, "%d.jpg"))
}

// Trickplay writes the sheets of a video into dir, which must exist and be
// empty, and describes them. It samples a Matroska file's keyframes
// through its index; when whole is set, it otherwise decodes the keyframes
// of the whole file, or failing that every frame, and when it is not,
// files without an index give keyframes.ErrNoIndex.
func (m *Maker) Trickplay(ctx context.Context, in Input, v Video, o Options, dir string, whole bool) (core.Trickplay, error) {
	if v.Duration <= 0 {
		return core.Trickplay{}, errors.New("thumbnails: video of unknown duration")
	}
	count := int(math.Ceil(v.Duration.Seconds() / o.Interval.Seconds()))
	err := m.sampled(ctx, in, v, o, dir, count)
	if err != nil && whole && ctx.Err() == nil {
		for _, mode := range []Mode{KeyframesOnly, AllFrames} {
			if err = empty(dir); err != nil {
				break
			}
			if err = m.run(ctx, TrickplayArgs(in.URL, mode, v, o, dir), nil); err == nil && sheets(dir) > 0 {
				break
			}
		}
	}
	if err != nil {
		return core.Trickplay{}, fmt.Errorf("trickplay of %s: %w", in.URL, err)
	}
	t := core.Trickplay{
		Width: o.Width, Height: v.height(o.Width), TileWidth: o.TileWidth, TileHeight: o.TileHeight,
		ThumbnailCount: count, Interval: o.Interval,
	}
	// The sheets ffmpeg wrote decide the count: the last may be partial.
	n := sheets(dir)
	if n == 0 {
		return core.Trickplay{}, fmt.Errorf("trickplay of %s: ffmpeg wrote no sheets", in.URL)
	}
	largest := int64(0)
	for i := range n {
		if info, err := os.Stat(filepath.Join(dir, strconv.Itoa(i)+".jpg")); err == nil {
			largest = max(largest, info.Size())
		}
	}
	per := o.TileWidth * o.TileHeight
	t.ThumbnailCount = min(count, n*per)
	if t.ThumbnailCount <= (n-1)*per {
		t.ThumbnailCount = (n-1)*per + 1
	}
	// A sheet serves per thumbnails' worth of playback.
	t.Bandwidth = int(math.Ceil(float64(largest*8) / (float64(per) * o.Interval.Seconds())))
	return t, nil
}

// sampled writes the sheets from the keyframe at or before each
// thumbnail's time, read through the file's Matroska index.
func (m *Maker) sampled(ctx context.Context, in Input, v Video, o Options, dir string, count int) error {
	if in.Open == nil {
		return keyframes.ErrNoIndex
	}
	f, err := in.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	idx, err := keyframes.ReadMatroska(f, fi.Size())
	if err != nil {
		return err
	}
	at := make([]time.Duration, count)
	for i := range at {
		at[i] = time.Duration(i) * o.Interval
	}
	pr, pw := io.Pipe()
	written := make(chan error, 1)
	go func() {
		err := idx.WriteSamples(ctx, pw, f, fi.Size(), at)
		pw.CloseWithError(err)
		written <- err
	}()
	err = m.run(ctx, TrickplayArgs("", Sampled, v, o, dir), pr)
	// ffmpeg stopping early ends the writer.
	pr.CloseWithError(io.ErrClosedPipe)
	if werr := <-written; werr != nil && !errors.Is(werr, io.ErrClosedPipe) {
		return werr
	}
	return err
}

// sheets counts the sheets in dir, 0.jpg, 1.jpg, ….
func sheets(dir string) int {
	n := 0
	for ; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(n)+".jpg")); err != nil {
			return n
		}
	}
}

// empty removes what a failed run left in dir.
func empty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// ChapterImageArgs returns the ffmpeg arguments writing the frame at a
// time as a JPEG file width pixels wide, reading input, the URL ffmpeg
// reads.
func ChapterImageArgs(input string, at time.Duration, v Video, width int, file string) []string {
	return []string{
		"-v", "error", "-nostdin", "-ss", strconv.FormatFloat(at.Seconds(), 'f', 3, 64), "-i", input,
		"-map", "0:v:0", "-an", "-sn", "-dn", "-frames:v", "1", "-vf", strings.Join(v.filters(width), ","),
		"-q:v", "3", "-f", "image2", "-update", "1", file,
	}
}

// ChapterImage writes the frame at a time of the video ffmpeg reads at
// input, a URL, into file.
func (m *Maker) ChapterImage(ctx context.Context, input string, at time.Duration, v Video, width int, file string) error {
	if err := m.run(ctx, ChapterImageArgs(input, at, v, width, file), nil); err != nil {
		return fmt.Errorf("chapter image of %s at %v: %w", input, at, err)
	}
	if info, err := os.Stat(file); err != nil || info.Size() == 0 {
		return fmt.Errorf("chapter image of %s at %v: ffmpeg wrote no frame", input, at)
	}
	return nil
}

// LoudnessArgs returns the ffmpeg arguments measuring the first audio
// stream of the file ffmpeg reads at input, a URL.
func LoudnessArgs(input string) []string {
	return []string{"-hide_banner", "-nostats", "-nostdin", "-v", "info", "-i", input, "-map", "0:a:0", "-af", "ebur128=framelog=quiet", "-f", "null", "-"}
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
// stream, in LUFS; input is the URL ffmpeg reads.
func (m *Maker) Loudness(ctx context.Context, input string) (float64, error) {
	var stderr bytes.Buffer
	if err := m.start(ctx, LoudnessArgs(input), nil, &stderr); err != nil {
		return 0, fmt.Errorf("loudness of %s: %w: %s", input, err, lastLines(stderr.String()))
	}
	v, ok := ParseLoudness(stderr.String())
	if !ok {
		return 0, fmt.Errorf("loudness of %s: no integrated loudness", input)
	}
	return v, nil
}

func (m *Maker) run(ctx context.Context, args []string, stdin io.Reader) error {
	var stderr bytes.Buffer
	if err := m.start(ctx, args, stdin, &stderr); err != nil {
		return fmt.Errorf("%w: %s", err, lastLines(stderr.String()))
	}
	return nil
}

// start runs ffmpeg at a low priority, as its runs are background tasks
// beside playbacks.
func (m *Maker) start(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, m.FFmpeg, args...)
	cmd.Stdin, cmd.Stderr = stdin, stderr
	started := lowPriority(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	started()
	return cmd.Wait()
}

// lastLines keeps the end of ffmpeg's output, where its errors are.
func lastLines(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1000 {
		s = s[len(s)-1000:]
	}
	return s
}
