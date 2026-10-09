package planner

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/media/decision"
)

func TestH264LevelArgs(t *testing.T) {
	tests := []struct {
		encoder, level string
		want           []string // the -level argument; nil for none
	}{
		{"libx264", "41", []string{"-level", "4.1"}},
		{"libx264", "12", []string{"-level", "1.2"}},
		{"libx264", "62", []string{"-level", "5.1"}},
		{"libx264", "", nil},
		{"h264_videotoolbox", "41", []string{"-level", "4.1"}},
		{"h264_videotoolbox", "30", []string{"-level", "3.0"}},
		{"h264_videotoolbox", "12", nil},
		{"h264_videotoolbox", "60", []string{"-level", "5.1"}},
	}
	p := &Planner{Options: DefaultOptions()}
	for _, tt := range tests {
		d := &decision.Decision{}
		if tt.level != "" {
			d.SetOption("h264", "level", tt.level)
		}
		args := p.profileArgs(&Job{Request: d}, tt.encoder)
		var got []string
		if i := slices.Index(args, "-level"); i >= 0 {
			got = args[i : i+2]
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s level %q: got = %v, want = %v", tt.encoder, tt.level, got, tt.want)
		}
	}
}
