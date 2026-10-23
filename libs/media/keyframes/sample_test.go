package keyframes

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/tools/fixtures"
)

func TestWriteSamples(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed: the samples are decoded with ffprobe")
	}
	at := []time.Duration{0, 3 * time.Second, 5 * time.Second, 10 * time.Second, 19 * time.Second}
	for _, tt := range []struct {
		name    string
		fixture string
		// scan forgets where blocks are in their clusters.
		scan bool
	}{
		{"cued blocks", "keyframes.mkv", false},
		{"scanned clusters", "keyframes.mkv", true},
		{"webm", "vp9_opus.webm", false},
		{"hevc", "hevc_main10_hdr10.mkv", false},
		{"mp4", "keyframes.mp4", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
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
			m, err := ReadSampler(cr, fi.Size())
			if err != nil {
				t.Fatal(err)
			}
			if tt.scan {
				mkv := m.(*Matroska)
				for i := range mkv.Cues {
					mkv.Cues[i].block = -1
				}
			}
			var out bytes.Buffer
			if err := m.WriteSamples(t.Context(), &out, cr, fi.Size(), at); err != nil {
				t.Fatalf("WriteSamples() error = %v", err)
			}
			if strings.HasPrefix(tt.fixture, "keyframes.") {
				// Five pictures from keyframes 2 s apart: four distinct ones.
				if n := cr.n.Load(); n*4 > fi.Size() {
					t.Errorf("WriteSamples() read %d bytes of %d, want = under a quarter", n, fi.Size())
				}
			}
			stream := filepath.Join(t.TempDir(), "samples.mkv")
			if err := os.WriteFile(stream, out.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-select_streams", "v:0", "-count_frames",
				"-show_entries", "stream=nb_read_frames:frame=pts_time,key_frame", "-of", "csv=p=0", stream)
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ffprobe: %v: %s", err, got)
			}
			lines := strings.Fields(string(got))
			for i, l := range lines {
				// Frames with side data end in an empty column.
				lines[i] = strings.TrimRight(l, ",")
			}
			want := []string{"1,0.000000", "1,3.000000", "1,5.000000", "1,10.000000", "1,19.000000", "5"}
			if strings.Join(lines, " ") != strings.Join(want, " ") {
				t.Errorf("decoded frames = %v, want = %v", lines, want)
			}
		})
	}
}

func TestReadSamplerFragmented(t *testing.T) {
	// The index of a fragmented file points to fragments, not samples.
	for _, fixture := range []string{"keyframes_fragmented.mp4", "keyframes_sidx.mp4"} {
		t.Run(fixture, func(t *testing.T) {
			f, err := os.Open(fixtures.Require(t, fixture))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			fi, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadSampler(f, fi.Size()); !errors.Is(err, ErrNoIndex) {
				t.Errorf("ReadSampler() error = %v, want = %v", err, ErrNoIndex)
			}
		})
	}
}
