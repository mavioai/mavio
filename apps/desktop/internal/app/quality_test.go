package app

import (
	"testing"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

func TestBitrate(t *testing.T) {
	a := &App{servers: []*capiv1.ServerRecord{
		capiv1.ServerRecord_builder{Id: "home", Local: true}.Build(),
		capiv1.ServerRecord_builder{Id: "away"}.Build(),
	}}
	for _, tt := range []struct {
		quality, server string
		want            int64
	}{
		{"auto", "home", 100_000_000},
		{"auto", "away", 40_000_000},
		{"auto", "gone", 40_000_000},
		{"", "home", 100_000_000},
		{"1080p", "home", 10_000_000},
		{"original", "away", 200_000_000},
	} {
		a.prefs.Quality = tt.quality
		if got := a.bitrate(tt.server); got != tt.want {
			t.Errorf("bitrate(%q) with %q = %d, want = %d", tt.server, tt.quality, got, tt.want)
		}
	}
}
