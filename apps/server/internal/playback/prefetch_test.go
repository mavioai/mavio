package playback

import (
	"testing"
	"time"
)

func TestSourcePrefetchSpan(t *testing.T) {
	const s = time.Second
	indexed := &sourcePrefetch{
		size: 1000, duration: 100 * s,
		keyframes: []time.Duration{0, 10 * s, 20 * s, 30 * s},
		offsets:   []int64{100, 300, 500, 700},
	}
	estimated := &sourcePrefetch{size: 100 << 20, duration: 100 * s}
	for _, tt := range []struct {
		name             string
		s                *sourcePrefetch
		from, to         time.Duration
		wantStart, wantE int64
	}{
		{"start", indexed, 0, 15 * s, 100, 500},
		{"between keyframes", indexed, 12 * s, 25 * s, 300, 700},
		{"on a keyframe", indexed, 20 * s, 20 * s, 500, 700},
		{"past the last keyframe", indexed, 35 * s, 65 * s, 700, 1000},
		{"estimated", estimated, 50 * s, 60 * s, 50<<20 - resumeLead, 60<<20 + resumeLead},
		{"estimated from the start", estimated, 0, 10 * s, 0, 10<<20 + resumeLead},
		{"at most maxPrefetch", &sourcePrefetch{size: 1 << 40, duration: 100 * s}, 0, 50 * s, 0, maxPrefetch},
		{"unknown", &sourcePrefetch{size: 1000}, 10 * s, 20 * s, 0, 0},
	} {
		start, end := tt.s.span(tt.from, tt.to)
		if start != tt.wantStart || end != tt.wantE {
			t.Errorf("%s: span() = %d, %d, want = %d, %d", tt.name, start, end, tt.wantStart, tt.wantE)
		}
	}
}
