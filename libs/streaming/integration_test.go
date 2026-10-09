package streaming

import (
	"bytes"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

// TestStreamTranscodes plays a 40 s file with keyframes every 1.5 s
// through ffmpeg, copied and encoded, seeking ahead and back, and checks
// that every segment holds exactly its stretch of the playlist.
func TestStreamTranscodes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	gen := exec.Command(ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440", "-t", "40",
		"-c:v", "libx264", "-g", "36", "-keyint_min", "36", "-sc_threshold", "0", "-c:a", "aac", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("integration test: cannot generate the source: %v\n%s", err, out)
	}
	res, err := (&probe.Prober{FFprobe: ffprobe}).Probe(t.Context(), probe.Request{Path: src})
	if err != nil {
		t.Fatal(err)
	}
	kf, err := (&keyframes.Extractor{FFprobe: ffprobe}).Extract(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	ms := res.Source
	ms.Path = src
	p := &planner.Planner{Options: planner.DefaultOptions()}

	for _, mode := range []string{"copied", "encoded"} {
		t.Run(mode, func(t *testing.T) {
			source := &decision.Source{MediaSource: &ms}
			job := &planner.Job{
				Delivery: planner.HLS, Source: source, Video: &ms.Streams[0], Audio: &ms.Streams[1],
				Request: &decision.Decision{}, VideoCodec: planner.Copy, AudioCodec: planner.Copy,
				AllowVideoCopy: true, AllowAudioCopy: true, SegmentContainer: "mp4", SegmentLength: 4 * time.Second,
			}
			layout := CopiedLayout(kf.Keyframes, kf.Duration, 4*time.Second)
			if mode == "encoded" {
				job.VideoCodec, job.AudioCodec, job.AllowVideoCopy = "h264", planner.Copy, false
				if layout, err = EncodedLayout(4*time.Second, ms.Duration); err != nil {
					t.Fatal(err)
				}
			}
			s, err := NewStream(Config{
				Dir: filepath.Join(t.TempDir(), "stream"), Layout: layout,
				Args: func(start time.Duration, out planner.HLSOutput) []string {
					j := *job
					j.Start = start
					return p.HLSArgs(&j, out)
				},
				Start:  supervisor.Exec(ffmpeg),
				MaxGap: 8 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			init := segmentBytes(t, s, InitSegment)
			// ffmpeg shifts the whole timeline by the reordering delay of
			// the B-frames, so that no timestamp is negative; segments
			// must line up with the playlist after that shift, whichever
			// run wrote them.
			var shift time.Duration
			for n, i := range []int{0, 1, 2, 8, 9, 3} {
				data := append(bytes.Clone(init), segmentBytes(t, s, i)...)
				first, last := videoSpan(t, ffprobe, filepath.Join(t.TempDir(), "seg.mp4"), data)
				if n == 0 {
					shift = first
				}
				first, last = first-shift, last-shift
				const frame = 42 * time.Millisecond
				if d := first - layout.Start(i); d.Abs() > time.Millisecond {
					t.Errorf("segment %d starts at %v, want = %v", i, first, layout.Start(i))
				}
				// The last segment ends with the container, after the audio.
				tail := frame + time.Millisecond
				if i == len(layout.Segments)-1 {
					tail = 100 * time.Millisecond
				}
				if last >= layout.End(i) || layout.End(i)-last > tail {
					t.Errorf("segment %d ends with a frame at %v, want = one frame before %v", i, last, layout.End(i))
				}
			}
			if shift > 100*time.Millisecond {
				t.Errorf("timeline shift = %v, want = the reordering delay", shift)
			}
		})
	}
}

func segmentBytes(t *testing.T, s *Stream, i int) []byte {
	t.Helper()
	r, _, err := s.Segment(t.Context(), i)
	if err != nil {
		t.Fatalf("Segment(%d): %v", i, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// videoSpan returns the first and last video presentation times of an
// fMP4 segment with its initialization segment.
func videoSpan(t *testing.T, ffprobe, path string, data []byte) (time.Duration, time.Duration) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v", "-show_entries", "packet=pts_time",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var times []time.Duration
	for line := range strings.Lines(string(out)) {
		if v, err := strconv.ParseFloat(strings.TrimSpace(line), 64); err == nil {
			times = append(times, time.Duration(math.Round(v*1e3))*time.Millisecond)
		}
	}
	if len(times) == 0 {
		t.Fatal("segment without video")
	}
	lo, hi := times[0], times[0]
	for _, v := range times {
		lo, hi = min(lo, v), max(hi, v)
	}
	return lo, hi
}
