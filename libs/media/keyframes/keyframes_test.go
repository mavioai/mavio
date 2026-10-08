package keyframes

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/tools/fixtures"
)

func TestFFprobeKeyframeExtractorCases(t *testing.T) {
	portedCases(t, "ff_probe/ff_probe_keyframe_extractor.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ParseStream_Valid_Success": func(t *testing.T, a args) {
				f, err := os.Open(filepath.Join("testdata", "ffprobe", a.str(t, "testDataFileName")))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				got, err := Parse(f)
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join("testdata", "ffprobe", a.str(t, "resultFileName")))
				if err != nil {
					t.Fatal(err)
				}
				var want struct {
					TotalDuration int64
					KeyframeTicks []int64
				}
				if err := json.Unmarshal(data, &want); err != nil {
					t.Fatal(err)
				}
				if g := int64(got.Duration / tick); g != want.TotalDuration {
					t.Errorf("duration: got = %d, want = %d", g, want.TotalDuration)
				}
				ticks := make([]int64, len(got.Keyframes))
				for i, k := range got.Keyframes {
					ticks[i] = int64(k / tick)
				}
				if !slices.Equal(ticks, want.KeyframeTicks) {
					t.Errorf("keyframes: got = %v, want = %v", ticks, want.KeyframeTicks)
				}
			},
		},
	})
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Data
	}{
		{"empty", "", Data{}},
		{"malformed lines", "garbage\npacket\npacket,1.0\n\nformat,-3\n", Data{}},
		{"not available", "packet,N/A,K_\nstream,N/A\nformat,2.5\n", Data{Duration: 2500 * time.Millisecond}},
		{"stream preferred", "stream,1.5\nformat,2.5\n", Data{Duration: 1500 * time.Millisecond}},
		{"zero stream", "stream,0\nformat,2.5\n", Data{Duration: 2500 * time.Millisecond}},
		{"exponent rejected", "packet,1e3,K_\n", Data{}},
		{"case insensitive", "PACKET,0.5,K__\npacket,1.0,__\npacket,2.0,KD\n", Data{Keyframes: []time.Duration{500 * time.Millisecond}}},
		{"half to even", "packet,0.00000005,K_\npacket,0.00000015,K_\n", Data{Keyframes: []time.Duration{0, 200 * time.Nanosecond}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if got.Duration != tt.want.Duration || !slices.Equal(got.Keyframes, tt.want.Keyframes) {
				t.Errorf("got = %+v, want = %+v", got, tt.want)
			}
		})
	}
}

func TestExtractFixtures(t *testing.T) {
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	e := &Extractor{FFprobe: path}
	for _, name := range []string{"h264_aac.mp4", "hevc_main10_hdr10.mkv", "mpeg2_interlaced.ts", "vp9_opus.webm"} {
		t.Run(name, func(t *testing.T) {
			d, err := e.Extract(t.Context(), fixtures.Require(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if d.Duration <= 0 || len(d.Keyframes) == 0 || d.Keyframes[0] > d.Duration {
				t.Errorf("got = %+v", d)
			}
			if !slices.IsSorted(d.Keyframes) {
				t.Errorf("keyframes not in order: got = %v", d.Keyframes)
			}
		})
	}
}
