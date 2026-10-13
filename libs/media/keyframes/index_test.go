package keyframes

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/tools/fixtures"
)

// countingReader counts the bytes read through it.
type countingReader struct {
	r interface {
		ReadAt(p []byte, off int64) (int, error)
	}
	n atomic.Int64
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.n.Add(int64(n))
	return n, err
}

func TestFromIndexFixtures(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed: the index is compared with what ffprobe reads")
	}
	want := make([]time.Duration, 10)
	for i := range want {
		want[i] = time.Duration(i) * 2 * time.Second
	}
	// Fragmented files carry no edit list, so their presentation starts
	// after the two frames B-frames delay.
	delayed := make([]time.Duration, len(want))
	for i, w := range want {
		delayed[i] = w + ticks(float64(time.Second)/12)
	}
	for _, tt := range []struct {
		fixture string
		want    []time.Duration
	}{
		{"keyframes.mkv", want},
		{"keyframes.mp4", want},
		{"keyframes_fragmented.mp4", delayed},
		{"keyframes_sidx.mp4", delayed},
		{"chapters.mkv", nil},
		{"vp9_opus.webm", nil},
		{"h264_aac.mp4", nil},
		{"ntsc_film_odd_duration.mp4", nil},
		{"multi_track.mkv", nil},
		{"hevc_main10_hdr10.mkv", nil},
		{"av1_opus.mkv", nil},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			path := fixtures.Require(t, tt.fixture)
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			fi, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			cr := &countingReader{r: f}
			got, err := FromIndex(cr, fi.Size())
			if err != nil {
				t.Fatalf("FromIndex() error = %v", err)
			}
			full, err := (&Extractor{FFprobe: ffprobe}).Extract(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Keyframes, full.Keyframes) {
				t.Errorf("FromIndex() keyframes = %v, want = %v (ffprobe)", got.Keyframes, full.Keyframes)
			}
			if tt.want != nil && !slices.Equal(got.Keyframes, tt.want) {
				t.Errorf("FromIndex() keyframes = %v, want = %v", got.Keyframes, tt.want)
			}
			if diff := got.Duration - full.Duration; got.Duration != 0 && (diff < -100*time.Millisecond || diff > 100*time.Millisecond) {
				t.Errorf("FromIndex() duration = %v, want = %v (ffprobe)", got.Duration, full.Duration)
			}
			// Each offset is at or before its keyframe's packet, and past
			// the keyframe before.
			pos := keyframePositions(t, ffprobe, path)
			if len(got.Offsets) != len(pos) {
				t.Fatalf("FromIndex() offsets = %d, want = %d (ffprobe)", len(got.Offsets), len(pos))
			}
			for i, off := range got.Offsets {
				if off > pos[i] || (i > 0 && off <= pos[i-1]) {
					t.Errorf("FromIndex() offset %d = %d, want = in (%d, %d] (ffprobe)", i, off, prev(pos, i), pos[i])
				}
			}
			if tt.want != nil {
				if n := cr.n.Load(); n*50 > fi.Size() {
					t.Errorf("FromIndex() read %d bytes of %d, want = under 2%%", n, fi.Size())
				}
			}
		})
	}
}

// keyframePositions returns the file offsets of the keyframe packets of
// the first video stream, as ffprobe reads them.
func keyframePositions(t *testing.T, ffprobe, path string) []int64 {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pos,flags", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var pos []int64
	for line := range strings.Lines(string(out)) {
		p, flags, _ := strings.Cut(strings.TrimSpace(line), ",")
		if !strings.HasPrefix(flags, "K") {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			t.Fatalf("ffprobe packet position %q: %v", p, err)
		}
		pos = append(pos, n)
	}
	return pos
}

// prev returns the position before the i-th, -1 for the first.
func prev(pos []int64, i int) int64 {
	if i == 0 {
		return -1
	}
	return pos[i-1]
}

func TestFromIndexNoIndex(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"text", []byte("WEBVTT\n\n00:00.000 --> 00:01.000\nhello\n")},
		{"ebml without segment", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x80}},
		{"ftyp without moov", []byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}},
		{"truncated box", []byte{0, 0, 1, 0, 'f', 't', 'y', 'p'}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromIndex(bytes.NewReader(tt.data), int64(len(tt.data)))
			if !errors.Is(err, ErrNoIndex) {
				t.Errorf("FromIndex() error = %v, want = %v", err, ErrNoIndex)
			}
		})
	}
}

func TestFromIndexTruncated(t *testing.T) {
	for _, name := range []string{"keyframes.mkv", "keyframes.mp4", "keyframes_fragmented.mp4", "keyframes_sidx.mp4"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(fixtures.Require(t, name))
			if err != nil {
				t.Fatal(err)
			}
			// No cut may panic; most leave no index.
			for cut := 0; cut < len(data); cut += max(1, len(data)/997) {
				_, _ = FromIndex(bytes.NewReader(data[:cut]), int64(cut))
			}
		})
	}
}
