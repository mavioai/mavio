package playback

import (
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/streaming"
	"github.com/mavioai/mavio/libs/subtitle"
)

func TestSegmentCues(t *testing.T) {
	sec := func(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
	cue := func(i int, start, end float64) subtitle.Cue {
		return subtitle.Cue{Index: i, Start: sec(start), End: sec(end), Text: "cue"}
	}
	// Segments 0-4 s, 4-8 s and 8-10 s.
	l, err := streaming.EncodedLayout(4*time.Second, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s := &subtitle.Subtitle{Cues: []subtitle.Cue{
		cue(1, 1, 2), cue(2, 3, 5), cue(3, 6, 7), cue(4, 8, 9), cue(5, 11, 12),
	}}

	tests := []struct {
		segment int
		want    []int
	}{
		{0, []int{1, 2}},
		// A cue across a boundary is in both segments, timed as in the
		// source; one starting at the boundary is in both too.
		{1, []int{2, 3, 4}},
		// The last segment takes cues past the end of the media.
		{2, []int{4, 5}},
	}
	for _, tt := range tests {
		got := segmentCues(s, l, tt.segment)
		var indexes []int
		for _, c := range got.Cues {
			indexes = append(indexes, c.Index)
			if want := s.Cues[c.Index-1]; c != want {
				t.Errorf("segment %d: cue = %+v, want = %+v", tt.segment, c, want)
			}
		}
		if !slices.Equal(indexes, tt.want) {
			t.Errorf("segment %d: cues = %v, want = %v", tt.segment, indexes, tt.want)
		}
	}
	if len(s.Cues) != 5 {
		t.Errorf("source cues = %d, want = 5", len(s.Cues))
	}
}
