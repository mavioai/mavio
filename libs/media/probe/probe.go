package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Prober runs ffprobe.
type Prober struct {
	// FFprobe is the ffprobe executable.
	FFprobe string
	// Threads limits ffprobe's decoding threads; zero lets it choose.
	Threads int
	// AnalyzeDuration and ProbeSize are ffprobe's -analyzeduration (in
	// microseconds) and -probesize; empty uses ffprobe's defaults.
	AnalyzeDuration string
	ProbeSize       string
	// FirstVideoFrame enables reading the first video frame for HDR10+
	// detection (-show_frames -only_first_vframe, a jellyfin-ffmpeg
	// option).
	FirstVideoFrame bool
}

// Request describes what to probe.
type Request struct {
	// Path is a file path or a URL.
	Path string
	// Remote is set for URLs, RTSP for rtsp:// URLs.
	Remote, RTSP bool
	// UserAgent is sent with remote requests.
	UserAgent string
	// AnalyzeDuration overrides the prober's for this request.
	AnalyzeDuration time.Duration
	// Audio selects audio-file handling.
	Audio bool
	// Chapters requests chapters.
	Chapters bool
}

// Args returns ffprobe's arguments for a request.
func (p *Prober) Args(r Request) []string {
	var args []string
	switch {
	case r.AnalyzeDuration > 0:
		args = append(args, "-analyzeduration", strconv.FormatInt(r.AnalyzeDuration.Microseconds(), 10))
	case p.AnalyzeDuration != "":
		args = append(args, "-analyzeduration", p.AnalyzeDuration)
	}
	if p.ProbeSize != "" {
		args = append(args, "-probesize", p.ProbeSize)
	}
	if r.UserAgent != "" {
		args = append(args, "-user_agent", r.UserAgent)
	}
	if r.RTSP {
		args = append(args, "-rtsp_transport", "tcp+udp", "-rtsp_flags", "prefer_tcp")
	}
	input := r.Path
	if !r.Remote {
		input = "file:" + r.Path
	}
	args = append(args, "-i", input, "-threads", strconv.Itoa(p.Threads), "-v", "warning",
		"-print_format", "json", "-show_streams", "-show_format")
	if r.Chapters {
		args = append(args, "-show_chapters")
	}
	if !r.Remote && !r.Audio && p.FirstVideoFrame {
		args = append(args, "-show_frames", "-only_first_vframe")
	}
	return args
}

// Probe runs ffprobe on a file or URL and normalizes its output.
func (p *Prober) Probe(ctx context.Context, r Request) (Result, error) {
	cmd := exec.CommandContext(ctx, p.FFprobe, p.Args(r)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("ffprobe %s: %w: %s", r.Path, err, strings.TrimSpace(stderr.String()))
	}
	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return Result{}, fmt.Errorf("ffprobe %s: decode output: %w", r.Path, err)
	}
	return Normalize(&out, r.Path, r.Audio), nil
}
